package cmd

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/asksyllable/syllable-cli/internal/client"
	apispec "github.com/asksyllable/syllable-cli/internal/spec"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type sdkQueryParameter struct {
	Name        string                 `json:"name"`
	In          string                 `json:"in"`
	Required    bool                   `json:"required"`
	Description string                 `json:"description"`
	Schema      map[string]interface{} `json:"schema"`
}

type sdkOperation struct {
	ID         string              `json:"operationId"`
	Parameters []sdkQueryParameter `json:"parameters"`
	Responses  map[string]struct {
		Content map[string]json.RawMessage `json:"content"`
	} `json:"responses"`
	Method string
	Path   string
	Accept string
}

var (
	sdkOperationsOnce sync.Once
	sdkOperations     map[string]sdkOperation
	sdkSchemas        map[string]map[string]interface{}
	sdkOperationsErr  error
)

// loadSDKOperations parses the embedded, pinned SDK contract once. The command
// registry remains hand-reviewed so a new SDK endpoint cannot silently acquire
// an unintended CLI command.
func loadSDKOperations() (map[string]sdkOperation, error) {
	sdkOperationsOnce.Do(func() {
		var doc struct {
			Paths      map[string]map[string]json.RawMessage `json:"paths"`
			Components struct {
				Schemas map[string]map[string]interface{} `json:"schemas"`
			} `json:"components"`
		}
		if err := json.Unmarshal(apispec.OpenAPI, &doc); err != nil {
			sdkOperationsErr = fmt.Errorf("loading SDK query contract: %w", err)
			return
		}
		sdkSchemas = doc.Components.Schemas
		sdkOperations = make(map[string]sdkOperation)
		for path, methods := range doc.Paths {
			for method, raw := range methods {
				switch method {
				case "get", "post", "put", "patch", "delete", "head", "options", "trace":
				default:
					continue
				}
				var operation sdkOperation
				if err := json.Unmarshal(raw, &operation); err != nil {
					sdkOperationsErr = err
					return
				}
				operation.Method, operation.Path = strings.ToUpper(method), path
				var accepts []string
				for status, response := range operation.Responses {
					if strings.HasPrefix(status, "2") {
						for contentType := range response.Content {
							accepts = append(accepts, contentType)
						}
					}
				}
				if len(accepts) == 0 {
					for _, response := range operation.Responses {
						for contentType := range response.Content {
							accepts = append(accepts, contentType)
						}
					}
				}
				sort.Strings(accepts)
				operation.Accept = strings.Join(accepts, ", ")
				if operation.Accept == "" {
					operation.Accept = "*/*"
				}
				sdkOperations[operation.ID] = operation
			}
		}
	})
	return sdkOperations, sdkOperationsErr
}

// bindSDKQueryParameters works for both the complete CLI and fresh resource
// trees in contract tests. Existing convenience flags and their defaults are
// retained; only explicitly supplied SDK query options override their requests.
func bindSDKQueryParameters(root *cobra.Command) error {
	operations, err := loadSDKOperations()
	if err != nil {
		return err
	}
	ids := make([]string, 0, len(sdkOperationCommands))
	for id := range sdkOperationCommands {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		argv := sdkOperationCommands[id]
		if root.Name() != "syllable" {
			if argv[0] != root.Name() {
				continue
			}
			argv = argv[1:]
		}
		cmd, _, err := root.Find(argv)
		if err != nil || cmd.RunE == nil {
			return fmt.Errorf("SDK operation %s has no registered command: %v", id, err)
		}
		operation, ok := operations[id]
		if !ok {
			return fmt.Errorf("CLI mapping references unknown SDK operation %s", id)
		}
		if err := bindSDKOperation(cmd, id, operation); err != nil {
			return err
		}
	}
	return nil
}

// headerFlagsAnnotation records the flags bound to an operation's optional
// header parameters, as a JSON object of header name to flag name, so the
// __operations dump can tell them from flags that build a request body.
const headerFlagsAnnotation = "syllable.sdk.header-flags"

var sdkHeaderFlagPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)

// sdkHeaderFlagName is the flag for an optional header parameter: lowercased,
// without a leading X-, with underscores as dashes, so
// X-Insight-Workflow-Format-Version becomes insight-workflow-format-version.
// bindSDKHeaderFlags puts header- in front when that name is already taken.
func sdkHeaderFlagName(header string) string {
	name := strings.TrimPrefix(strings.ToLower(header), "x-")
	return strings.ReplaceAll(name, "_", "-")
}

// sdkFlagTaken reports whether a flag name is already in use on the command,
// including the flags it inherits.
func sdkFlagTaken(cmd *cobra.Command, name string) bool {
	return name == "help" || cmd.Flags().Lookup(name) != nil || cmd.InheritedFlags().Lookup(name) != nil ||
		cmd.Root().PersistentFlags().Lookup(name) != nil || rootCmd.PersistentFlags().Lookup(name) != nil
}

type sdkHeaderFlag struct {
	header, flag string
}

// bindSDKHeaderFlags adds a flag for each of an operation's optional header
// parameters. A flag sends its header only when it is set, so by default the
// API applies its own default. Required headers need hand-written code (the
// generator won't generate an operation that takes one).
func bindSDKHeaderFlags(cmd *cobra.Command, id string, operation sdkOperation) ([]sdkHeaderFlag, error) {
	var headers []sdkHeaderFlag
	names := map[string]string{}
	for _, parameter := range operation.Parameters {
		if parameter.In != "header" || parameter.Required || client.ReservedHeader(parameter.Name) {
			continue
		}
		name := sdkHeaderFlagName(parameter.Name)
		if !sdkHeaderFlagPattern.MatchString(name) {
			return nil, fmt.Errorf("SDK operation %s: header parameter %q has no usable flag name", id, parameter.Name)
		}
		if sdkFlagTaken(cmd, name) {
			name = "header-" + name
			if sdkFlagTaken(cmd, name) {
				return nil, fmt.Errorf("SDK operation %s: header parameter %q would collide with the --%s flag", id, parameter.Name, name)
			}
		}
		// The spec is untrusted input, so its text is cleaned before it reaches help.
		help := cleanSpecLine(parameter.Description, 200)
		if help == "" {
			help = "Sends the " + parameter.Name + " request header"
		}
		switch sdkSchemaType(parameter.Schema) {
		case "boolean":
			cmd.Flags().Bool(name, false, help)
		case "integer":
			cmd.Flags().Int64(name, 0, help)
		case "number":
			cmd.Flags().Float64(name, 0, help)
		default:
			cmd.Flags().String(name, "", help)
		}
		headers = append(headers, sdkHeaderFlag{header: parameter.Name, flag: name})
		names[parameter.Name] = name
	}
	if len(names) > 0 {
		encoded, err := json.Marshal(names)
		if err != nil {
			return nil, err
		}
		cmd.Annotations[headerFlagsAnnotation] = string(encoded)
	}
	return headers, nil
}

// sdkHeaderFlagsOf reads the header flags bindSDKHeaderFlags recorded.
func sdkHeaderFlagsOf(cmd *cobra.Command) map[string]string {
	names := map[string]string{}
	if raw := cmd.Annotations[headerFlagsAnnotation]; raw != "" {
		_ = json.Unmarshal([]byte(raw), &names)
	}
	return names
}

// sdkHeaderValues returns the headers whose flags were set on this run.
func sdkHeaderValues(cmd *cobra.Command, headers []sdkHeaderFlag) (map[string]string, error) {
	values := map[string]string{}
	for _, h := range headers {
		flag := cmd.Flags().Lookup(h.flag)
		if flag == nil || !flag.Changed {
			continue
		}
		value := flag.Value.String()
		if strings.ContainsAny(value, "\r\n\x00") {
			return nil, fmt.Errorf("--%s can't contain a line break", h.flag)
		}
		values[h.header] = value
	}
	return values, nil
}

// bindSDKOperation adds an operation's query and optional header parameters to
// its command as flags and scopes the request's Accept header, extra headers,
// delete reason, and query overrides to that operation. Hand-written and
// generated commands share it.
func bindSDKOperation(cmd *cobra.Command, id string, operation sdkOperation) error {
	if cmd.Annotations == nil {
		cmd.Annotations = make(map[string]string)
	}
	if previous := cmd.Annotations["syllable.sdk.operation"]; previous != "" {
		if previous != id {
			return fmt.Errorf("command %s maps to both %s and %s", cmd.CommandPath(), previous, id)
		}
		return nil
	}
	cmd.Annotations["syllable.sdk.operation"] = id
	var parameters []sdkQueryParameter
	for _, parameter := range operation.Parameters {
		if parameter.In != "query" || sdkQueryIsPositional(operation.ID, parameter.Name) {
			continue
		}
		parameters = append(parameters, parameter)
		name := sdkQueryFlagName(cmd, parameter.Name)
		if cmd.Flags().Lookup(name) != nil {
			continue
		}
		help := parameter.Description
		if help == "" {
			help = "SDK query parameter " + parameter.Name
		}
		switch sdkSchemaType(parameter.Schema) {
		case "array":
			if parameter.Name == "fields" {
				cmd.Flags().StringSlice(name, nil, help+" (comma-separated; repeatable)")
			} else {
				cmd.Flags().StringArray(name, nil, help+" (repeat once per value; commas are literal)")
			}
		case "boolean":
			cmd.Flags().Bool(name, false, help)
		case "integer":
			cmd.Flags().Int64(name, 0, help)
		case "number":
			cmd.Flags().Float64(name, 0, help)
		default:
			cmd.Flags().String(name, "", help)
		}
	}
	headers, err := bindSDKHeaderFlags(cmd, id, operation)
	if err != nil {
		return err
	}
	run := cmd.RunE
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		overrides, err := sdkQueryOverrides(cmd, parameters)
		if err != nil {
			return err
		}
		headerValues, err := sdkHeaderValues(cmd, headers)
		if err != nil {
			return err
		}
		if apiClient == nil {
			return run(cmd, args)
		}
		previous := apiClient
		scoped := *previous
		scoped.DeleteReasonRequired = func(path string) bool {
			if operation.Method == "DELETE" {
				if u, err := url.Parse(path); err == nil && sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
					return sdkOperationHasQuery(operation, "reason")
				}
			}
			if previous.DeleteReasonRequired != nil {
				return previous.DeleteReasonRequired(path)
			}
			return true
		}
		scoped.RequestAccept = func(method, path string) string {
			if operation.Method == method {
				if u, err := url.Parse(path); err == nil && sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
					return operation.Accept
				}
			}
			if previous.RequestAccept != nil {
				return previous.RequestAccept(method, path)
			}
			return ""
		}
		scoped.RequestHeaders = func(method, path string) map[string]string {
			out := map[string]string{}
			if previous.RequestHeaders != nil {
				for name, value := range previous.RequestHeaders(method, path) {
					out[name] = value
				}
			}
			if len(headerValues) > 0 && operation.Method == method {
				if u, err := url.Parse(path); err == nil && sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
					for name, value := range headerValues {
						out[name] = value
					}
				}
			}
			return out
		}
		scoped.RequestPath = func(method, path string) (string, error) {
			if previous.RequestPath != nil {
				var err error
				path, err = previous.RequestPath(method, path)
				if err != nil {
					return "", err
				}
			}
			return applySDKQueryOverrides(operation, method, path, overrides)
		}
		apiClient = &scoped
		defer func() { apiClient = previous }()
		return run(cmd, args)
	}
	return nil
}

// target_id on channel-target deletion is already represented by the second
// positional argument. An additional overriding flag could delete a different
// target from the one named in the confirmation prompt.
func sdkQueryIsPositional(operationID, name string) bool {
	return operationID == "channel_targets_delete" && name == "target_id"
}

func sdkQueryFlagName(cmd *cobra.Command, name string) string {
	switch name {
	case "fields":
		return "api-fields"
	case "search_fields":
		return "search-fields"
	case "search_field_values":
		return "search-values"
	case "start_datetime":
		return "start-date"
	case "end_datetime":
		return "end-date"
	case "dashboard_name":
		if cmd.Flags().Lookup("name") != nil {
			return "name"
		}
	case "response_format":
		if cmd.Flags().Lookup("format") != nil {
			return "format"
		}
	}
	return strings.ReplaceAll(name, "_", "-")
}

func sdkSchemaType(schema map[string]interface{}) string {
	if ref, ok := schema["$ref"].(string); ok {
		return sdkSchemaType(sdkSchemas[strings.TrimPrefix(ref, "#/components/schemas/")])
	}
	if typ, ok := schema["type"].(string); ok && typ != "null" {
		return typ
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if choices, ok := schema[key].([]interface{}); ok {
			for _, choice := range choices {
				if candidate, ok := choice.(map[string]interface{}); ok {
					if typ := sdkSchemaType(candidate); typ != "" {
						return typ
					}
				}
			}
		}
	}
	return ""
}

func sdkFlagValues(flag *pflag.Flag) []string {
	if value, ok := flag.Value.(pflag.SliceValue); ok {
		return value.GetSlice()
	}
	return []string{flag.Value.String()}
}

func sdkQueryOverrides(cmd *cobra.Command, parameters []sdkQueryParameter) (url.Values, error) {
	values := url.Values{}
	for _, parameter := range parameters {
		flag := cmd.Flags().Lookup(sdkQueryFlagName(cmd, parameter.Name))
		if flag != nil && flag.Changed {
			values[parameter.Name] = sdkFlagValues(flag)
		}
	}
	// Preserve --search/--search-field while allowing explicit empty searches.
	// The plural forms replace convenience values and preserve commas literally.
	search, field := cmd.Flags().Lookup("search"), cmd.Flags().Lookup("search-field")
	if _, plural := values["search_field_values"]; !plural && search != nil && search.Changed {
		values["search_field_values"] = sdkFlagValues(search)
	}
	if _, plural := values["search_fields"]; !plural && field != nil {
		if search != nil && search.Changed || field.Changed && search != nil && search.Value.String() != "" {
			values["search_fields"] = sdkFlagValues(field)
		}
	}
	// Including test sessions is independent of the convenience search pair.
	// Preserve it when plural SDK filters replace the command's original query.
	if include := cmd.Flags().Lookup("include-test"); include != nil && include.Value.String() == "true" {
		_, hasFields := values["search_fields"]
		_, hasValues := values["search_field_values"]
		if hasFields || hasValues {
			alreadyExplicit := false
			for _, name := range values["search_fields"] {
				if name == "is_test" {
					alreadyExplicit = true
				}
			}
			if !alreadyExplicit {
				values["search_fields"] = append(values["search_fields"], "is_test")
				values["search_field_values"] = append(values["search_field_values"], "true")
			}
		}
	}
	if fields, hasFields := values["search_fields"]; hasFields {
		if searchValues, hasValues := values["search_field_values"]; hasValues && len(fields) != len(searchValues) {
			return nil, fmt.Errorf("--search-fields and --search-values must have equal numbers of values")
		}
	}
	return values, nil
}

func applySDKQueryOverrides(operation sdkOperation, method, path string, overrides url.Values) (string, error) {
	if method != operation.Method {
		return path, nil
	}
	u, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("parsing request path: %w", err)
	}
	if !sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
		return path, nil
	}
	q := u.Query()
	for name, values := range overrides {
		q[name] = append([]string(nil), values...)
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func sdkOperationPathMatches(template, escapedPath string) bool {
	want, got := strings.Split(template, "/"), strings.Split(escapedPath, "/")
	if len(want) != len(got) {
		return false
	}
	for i := range want {
		if strings.HasPrefix(want[i], "{") && strings.HasSuffix(want[i], "}") {
			if got[i] == "" {
				return false
			}
			continue
		}
		if want[i] != got[i] {
			return false
		}
	}
	return true
}

// sdkRequestAccept is also used outside registered leaf commands (for helper
// lookups). A static route wins over a competing {resource_id} route.
func sdkRequestAccept(method, path string) string {
	operation, _ := findSDKOperation(method, path)
	return operation.Accept
}

func sdkDeleteReasonRequired(path string) bool {
	operation, found := findSDKOperation("DELETE", path)
	return found && sdkOperationHasQuery(operation, "reason")
}

func sdkOperationHasQuery(operation sdkOperation, name string) bool {
	for _, parameter := range operation.Parameters {
		if parameter.In == "query" && parameter.Name == name {
			return true
		}
	}
	return false
}

func findSDKOperation(method, path string) (sdkOperation, bool) {
	operations, err := loadSDKOperations()
	if err != nil {
		return sdkOperation{}, false
	}
	u, err := url.Parse(path)
	if err != nil {
		return sdkOperation{}, false
	}
	best, specificity := sdkOperation{}, -1
	for _, operation := range operations {
		if operation.Method != method || !sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
			continue
		}
		score := 0
		for _, segment := range strings.Split(operation.Path, "/") {
			if !strings.HasPrefix(segment, "{") {
				score += len(segment)
			}
		}
		if score > specificity {
			best, specificity = operation, score
		}
	}
	return best, specificity >= 0
}
