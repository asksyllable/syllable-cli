package cmd

// Generated commands
//
// An API operation with no hand-written command (sdk_operations.go) and no
// console-only exclusion gets a command generated from the embedded spec at
// startup, so a new endpoint is usable as soon as the CLI syncs to it, with no
// hand-written code. They are plain and built for agents: path parameters are
// positional arguments, query parameters are flags, a JSON request body comes
// from --file, the raw JSON response is printed, and --help spells out the
// body's fields with an example. Deletes and other body-less changes ask for
// confirmation like the hand-written deletes.
//
// Names are frozen in generated_commands.json, so a change to the naming rules
// or to an upstream tag can't rename a shipped command. `go generate ./cmd`
// names new operations and refreshes the command reference docs; the test
// suite fails while either is stale. Mapping an operation to a hand-written
// command retires its generated one.

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode/utf8"

	"github.com/asksyllable/syllable-cli/internal/output"
	apispec "github.com/asksyllable/syllable-cli/internal/spec"
	"github.com/spf13/cobra"
)

//go:generate go test -count=1 -run ^TestGeneratedCommands$ . -update

//go:embed generated_commands.json
var generatedCommandsJSON []byte

// generatedAnnotation marks a generated command; its operation ID is under the
// usual syllable.sdk.operation annotation and its route under
// generatedRouteAnnotation ("GET /api/v1/...").
const (
	generatedAnnotation      = "syllable.generated"
	generatedRouteAnnotation = "syllable.generated.route"
)

const generatedManifestComment = "Frozen names of the commands generated from the API spec. Written by `go generate ./cmd`; do not edit by hand."

type generatedManifest struct {
	Comment  string           `json:"_comment"`
	Commands []generatedEntry `json:"commands"`
}

type generatedEntry struct {
	Command     string `json:"command"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	OperationID string `json:"operation_id"`
}

func parseGeneratedManifest(data []byte) (generatedManifest, error) {
	var m generatedManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return m, fmt.Errorf("parsing generated_commands.json: %w", err)
	}
	return m, nil
}

func renderGeneratedManifest(entries []generatedEntry) []byte {
	if entries == nil {
		entries = []generatedEntry{}
	}
	data, _ := json.MarshalIndent(generatedManifest{Comment: generatedManifestComment, Commands: entries}, "", "  ")
	return append(data, '\n')
}

// --- the spec, as generation sees it ---

type specContent struct {
	Schema map[string]interface{} `json:"schema"`
}

type specOperation struct {
	Method      string
	Path        string
	OperationID string                 `json:"operationId"`
	Tags        []string               `json:"tags"`
	Summary     string                 `json:"summary"`
	Description string                 `json:"description"`
	Deprecated  bool                   `json:"deprecated"`
	Security    *[]map[string][]string `json:"security"`
	Parameters  []sdkQueryParameter    `json:"parameters"`
	RequestBody *struct {
		Required bool                   `json:"required"`
		Content  map[string]specContent `json:"content"`
	} `json:"requestBody"`
	Responses map[string]struct {
		Content map[string]specContent `json:"content"`
	} `json:"responses"`
}

type specDocument struct {
	Operations      []specOperation // sorted by path, then method
	Schemas         map[string]map[string]interface{}
	Tags            map[string]string
	Security        *[]map[string][]string
	SecuritySchemes map[string]struct {
		Type string `json:"type"`
		In   string `json:"in"`
		Name string `json:"name"`
	}
	index map[string]int
}

func parseSpecDocument(data []byte) (*specDocument, error) {
	var raw struct {
		Paths      map[string]map[string]json.RawMessage `json:"paths"`
		Components struct {
			Schemas         map[string]map[string]interface{} `json:"schemas"`
			SecuritySchemes map[string]struct {
				Type string `json:"type"`
				In   string `json:"in"`
				Name string `json:"name"`
			} `json:"securitySchemes"`
		} `json:"components"`
		Tags []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tags"`
		Security *[]map[string][]string `json:"security"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("parsing the OpenAPI spec: %w", err)
	}
	doc := &specDocument{
		Schemas:         raw.Components.Schemas,
		Tags:            map[string]string{},
		Security:        raw.Security,
		SecuritySchemes: raw.Components.SecuritySchemes,
		index:           map[string]int{},
	}
	for _, tag := range raw.Tags {
		doc.Tags[tag.Name] = tag.Description
	}
	for path, methods := range raw.Paths {
		for method, body := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options", "trace":
			default:
				continue
			}
			var op specOperation
			if err := json.Unmarshal(body, &op); err != nil {
				return nil, fmt.Errorf("parsing %s %s: %w", strings.ToUpper(method), path, err)
			}
			op.Method, op.Path = strings.ToUpper(method), path
			doc.Operations = append(doc.Operations, op)
		}
	}
	sort.Slice(doc.Operations, func(i, j int) bool {
		a, b := doc.Operations[i], doc.Operations[j]
		if a.Path != b.Path {
			return a.Path < b.Path
		}
		return a.Method < b.Method
	})
	for i, op := range doc.Operations {
		doc.index[op.Method+" "+op.Path] = i
	}
	return doc, nil
}

func (d *specDocument) operation(method, path string) (specOperation, bool) {
	i, ok := d.index[method+" "+path]
	if !ok {
		return specOperation{}, false
	}
	return d.Operations[i], true
}

var (
	embeddedSpecOnce sync.Once
	embeddedSpec     *specDocument
	embeddedSpecErr  error
)

func embeddedSpecDocument() (*specDocument, error) {
	embeddedSpecOnce.Do(func() { embeddedSpec, embeddedSpecErr = parseSpecDocument(apispec.OpenAPI) })
	return embeddedSpec, embeddedSpecErr
}

// accept matches loadSDKOperations: the success media types, else any
// response's, else */*.
func (op specOperation) accept() string {
	var accepts []string
	for status, response := range op.Responses {
		if strings.HasPrefix(status, "2") {
			for contentType := range response.Content {
				accepts = append(accepts, contentType)
			}
		}
	}
	if len(accepts) == 0 {
		for _, response := range op.Responses {
			for contentType := range response.Content {
				accepts = append(accepts, contentType)
			}
		}
	}
	if len(accepts) == 0 {
		return "*/*"
	}
	sort.Strings(accepts)
	return strings.Join(accepts, ", ")
}

func (op specOperation) sdk() sdkOperation {
	return sdkOperation{ID: op.OperationID, Parameters: op.Parameters, Method: op.Method, Path: op.Path, Accept: op.accept()}
}

var specPathParameter = regexp.MustCompile(`\{([^{}]*)\}`)

// pathParameters lists the path template's parameters in order, with their
// declarations where the spec has them.
func (op specOperation) pathParameters() []sdkQueryParameter {
	declared := map[string]sdkQueryParameter{}
	for _, p := range op.Parameters {
		if p.In == "path" {
			declared[p.Name] = p
		}
	}
	var out []sdkQueryParameter
	for _, m := range specPathParameter.FindAllStringSubmatch(op.Path, -1) {
		p, ok := declared[m[1]]
		if !ok {
			p = sdkQueryParameter{Name: m[1], In: "path", Required: true}
		}
		out = append(out, p)
	}
	return out
}

// --- which operations can be generated ---

var (
	specPathPattern      = regexp.MustCompile(`^/[A-Za-z0-9_.~{}/-]*$`)
	specParameterPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	generatedWordPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(-[a-z0-9]+)*$`)
)

// generationBlocker explains why an operation can't get a generated command,
// or returns "" when it can. Generation covers JSON-in, JSON-out operations
// that authenticate with the API key. Uploads, downloads, form bodies, header
// parameters, and anonymous operations still need a hand-written command.
func generationBlocker(op specOperation, doc *specDocument) string {
	switch op.Method {
	case "GET", "POST", "PUT", "PATCH", "DELETE":
	default:
		return "uses HTTP " + op.Method
	}
	if op.OperationID == "" {
		return "has no operationId"
	}
	if !specPathPattern.MatchString(op.Path) {
		return "has an unusual path"
	}
	if !usesAPIKey(op, doc) {
		return "does not authenticate with the Syllable API key"
	}
	for _, p := range op.pathParameters() {
		if !specParameterPattern.MatchString(p.Name) {
			return fmt.Sprintf("has an unusual path parameter %q", p.Name)
		}
	}
	flags := map[string]string{}
	probe := &cobra.Command{}
	for _, p := range op.Parameters {
		switch p.In {
		case "path":
		case "query":
			if !specParameterPattern.MatchString(p.Name) {
				return fmt.Sprintf("has an unusual query parameter %q", p.Name)
			}
			name := sdkQueryFlagName(probe, p.Name)
			if name == "file" || name == "help" || rootCmd.PersistentFlags().Lookup(name) != nil {
				return fmt.Sprintf("has a query parameter %q that would collide with the --%s flag", p.Name, name)
			}
			if other, taken := flags[name]; taken {
				return fmt.Sprintf("has query parameters %q and %q that would share the --%s flag", other, p.Name, name)
			}
			flags[name] = p.Name
		default:
			return fmt.Sprintf("takes a %s parameter (%s)", p.In, p.Name)
		}
	}
	if body := op.RequestBody; body != nil {
		if _, ok := body.Content["application/json"]; !ok || len(body.Content) != 1 {
			return "takes a " + mediaTypes(body.Content) + " request body"
		}
	}
	for status, response := range op.Responses {
		if !strings.HasPrefix(status, "2") {
			continue
		}
		for contentType := range response.Content {
			if contentType != "application/json" {
				return "returns " + mediaTypes(response.Content)
			}
		}
	}
	return ""
}

func mediaTypes(content map[string]specContent) string {
	var types []string
	for t := range content {
		types = append(types, t)
	}
	sort.Strings(types)
	if len(types) == 0 {
		return "an empty"
	}
	return strings.Join(types, ", ")
}

// usesAPIKey reports whether every way to call the operation includes the
// Syllable-API-Key header, which is what the CLI sends.
func usesAPIKey(op specOperation, doc *specDocument) bool {
	security := op.Security
	if security == nil {
		security = doc.Security
	}
	if security == nil || len(*security) == 0 {
		return false
	}
	for _, requirement := range *security {
		found := false
		for name := range requirement {
			scheme, ok := doc.SecuritySchemes[name]
			if ok && scheme.Type == "apiKey" && scheme.In == "header" && strings.EqualFold(scheme.Name, "Syllable-API-Key") {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// --- naming ---

// generatedNameTokens lowercases s and splits it into alphanumeric runs.
func generatedNameTokens(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9')
	})
}

func childCommand(parent *cobra.Command, name string) *cobra.Command {
	if parent == nil {
		return nil
	}
	for _, c := range parent.Commands() {
		if c.Name() == name || c.HasAlias(name) {
			return c
		}
	}
	return nil
}

func isCommandGroup(c *cobra.Command) bool {
	return c.HasSubCommands() || (c.RunE == nil && c.Run == nil)
}

func commandExists(root *cobra.Command, words []string) bool {
	c := root
	for _, w := range words {
		if c = childCommand(c, w); c == nil {
			return false
		}
	}
	return true
}

// validGeneratedWord rejects names that aren't plain kebab-case, and names the
// CLI treats specially: any command with one of these names (or under one)
// runs without credentials.
func validGeneratedWord(word string) error {
	if !generatedWordPattern.MatchString(word) || len(word) > 64 {
		return fmt.Errorf("%q is not a usable command name", word)
	}
	if _, reserved := noAuthCommandNames[word]; reserved {
		return fmt.Errorf("%q is reserved", word)
	}
	return nil
}

// generatedCommandWords names a new operation: its first tag gives the group
// (agents.test is `agents test`), and its operationId, minus the words that
// repeat the group, gives the verb (agent_list_active_labels under agents is
// `list-active-labels`). A tag segment that is already a hand-written leaf
// command becomes part of the verb instead (`sessions timeline-export`). On a
// name collision the verb falls back to the whole operationId.
func generatedCommandWords(op specOperation, root *cobra.Command) ([]string, error) {
	if len(op.Tags) == 0 {
		return nil, fmt.Errorf("has no tag to group it under")
	}
	var groups, prefix []string
	vocabulary := map[string]bool{}
	parent, fresh := root, false
	for _, segment := range strings.Split(op.Tags[0], ".") {
		tokens := generatedNameTokens(segment)
		if len(tokens) == 0 {
			return nil, fmt.Errorf("has an unusable tag %q", op.Tags[0])
		}
		for _, t := range tokens {
			vocabulary[t], vocabulary[strings.TrimSuffix(t, "s")] = true, true
		}
		name := strings.Join(tokens, "-")
		if len(prefix) > 0 {
			prefix = append(prefix, tokens...)
			continue
		}
		if !fresh {
			child := childCommand(parent, name)
			if child != nil && !isCommandGroup(child) {
				prefix = append(prefix, tokens...)
				continue
			}
			if child == nil {
				fresh = true
			}
			parent = child
		}
		groups = append(groups, name)
	}
	idTokens := generatedNameTokens(op.OperationID)
	i := 0
	for i < len(idTokens) && vocabulary[idTokens[i]] {
		i++
	}
	verb := idTokens[i:]
	if strings.Join(verb, " ") == "get by id" {
		verb = []string{"get"}
	}
	if len(verb) == 0 {
		verb = []string{defaultGeneratedVerb(op)}
	}
	words := append(append([]string(nil), groups...), strings.Join(append(append([]string(nil), prefix...), verb...), "-"))
	if commandExists(root, words) {
		words[len(words)-1] = strings.Join(append(append([]string(nil), prefix...), idTokens...), "-")
		if commandExists(root, words) {
			return nil, fmt.Errorf("would be named `syllable %s`, which is taken", strings.Join(words, " "))
		}
	}
	for _, w := range words {
		if err := validGeneratedWord(w); err != nil {
			return nil, err
		}
	}
	return words, nil
}

func defaultGeneratedVerb(op specOperation) string {
	switch op.Method {
	case "GET":
		if strings.HasSuffix(op.Path, "}") {
			return "get"
		}
		return "list"
	case "POST":
		return "create"
	case "DELETE":
		return "delete"
	default:
		return "update"
	}
}

// planGeneratedCommands decides the generated commands for a spec. It keeps
// every frozen name that still fits and names new operations, adding each
// command to root (which starts with only the hand-written commands) so later
// names see earlier ones. problems lists the operations that need a
// hand-written command or a console-only exclusion instead.
func planGeneratedCommands(root *cobra.Command, doc *specDocument, previous []generatedEntry) (entries []generatedEntry, problems []string) {
	needsCommand := func(op specOperation) bool {
		_, mapped := sdkOperationCommands[op.OperationID]
		_, excluded := consoleOnlyOperations[op.OperationID]
		return !mapped && !excluded
	}
	frozen := map[string]bool{}
	sorted := append([]generatedEntry(nil), previous...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Command < sorted[j].Command })
	for _, e := range sorted {
		op, ok := doc.operation(e.Method, e.Path)
		if !ok || !needsCommand(op) {
			continue // removed upstream, or replaced by a hand-written command
		}
		frozen[e.Method+" "+e.Path] = true
		e.OperationID = op.OperationID
		if err := addGeneratedCommand(root, doc, e); err != nil {
			problems = append(problems, fmt.Sprintf("`syllable %s` (%s %s) can't be kept: %v", e.Command, e.Method, e.Path, err))
			continue
		}
		entries = append(entries, e)
	}
	for _, op := range doc.Operations {
		if frozen[op.Method+" "+op.Path] || !needsCommand(op) {
			continue
		}
		label := fmt.Sprintf("%s %s (%s)", op.Method, op.Path, op.OperationID)
		if reason := generationBlocker(op, doc); reason != "" {
			problems = append(problems, label+" "+reason)
			continue
		}
		words, err := generatedCommandWords(op, root)
		if err != nil {
			problems = append(problems, label+" "+err.Error())
			continue
		}
		e := generatedEntry{Command: strings.Join(words, " "), Method: op.Method, Path: op.Path, OperationID: op.OperationID}
		if err := addGeneratedCommand(root, doc, e); err != nil {
			problems = append(problems, label+": "+err.Error())
			continue
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Command < entries[j].Command })
	return entries, problems
}

// --- building and registering commands ---

// registerGeneratedCommands adds the frozen generated commands to the CLI. It
// never stops the CLI from starting: an entry that no longer fits the spec is
// skipped here, and the test suite reports it.
func registerGeneratedCommands(root *cobra.Command) {
	doc, err := embeddedSpecDocument()
	if err != nil {
		return
	}
	manifest, err := parseGeneratedManifest(generatedCommandsJSON)
	if err != nil {
		return
	}
	for _, entry := range manifest.Commands {
		_ = addGeneratedCommand(root, doc, entry)
	}
}

func addGeneratedCommand(root *cobra.Command, doc *specDocument, entry generatedEntry) error {
	op, ok := doc.operation(entry.Method, entry.Path)
	if !ok {
		return fmt.Errorf("%s %s is no longer in the spec", entry.Method, entry.Path)
	}
	if _, mapped := sdkOperationCommands[op.OperationID]; mapped {
		return fmt.Errorf("%s has a hand-written command", op.OperationID)
	}
	if _, excluded := consoleOnlyOperations[op.OperationID]; excluded {
		return fmt.Errorf("%s is console-only", op.OperationID)
	}
	if reason := generationBlocker(op, doc); reason != "" {
		return fmt.Errorf("%s %s", op.OperationID, reason)
	}
	words := strings.Fields(entry.Command)
	if len(words) < 2 {
		return fmt.Errorf("%q needs a group and a verb", entry.Command)
	}
	for _, w := range words {
		if err := validGeneratedWord(w); err != nil {
			return err
		}
	}
	parent := root
	for i, w := range words[:len(words)-1] {
		child := childCommand(parent, w)
		if child == nil {
			short := "Commands generated from the API spec"
			if i == len(words)-2 && len(op.Tags) > 0 {
				if d := cleanSpecLine(firstSentence(doc.Tags[op.Tags[0]]), 160); d != "" {
					short = d
				}
			}
			child = &cobra.Command{Use: w, Short: short, Annotations: map[string]string{generatedAnnotation: "group"}}
			parent.AddCommand(child)
		} else if !isCommandGroup(child) {
			return fmt.Errorf("`syllable %s` is a command, not a group", strings.Join(words[:i+1], " "))
		}
		parent = child
	}
	if childCommand(parent, words[len(words)-1]) != nil {
		return fmt.Errorf("`syllable %s` is taken", entry.Command)
	}
	cmd, err := newGeneratedCommand(op, words[len(words)-1], doc)
	if err != nil {
		return err
	}
	parent.AddCommand(cmd)
	return nil
}

func argName(parameter string) string {
	return strings.Join(generatedNameTokens(parameter), "-")
}

func newGeneratedCommand(op specOperation, verb string, doc *specDocument) (*cobra.Command, error) {
	params := op.pathParameters()
	use := verb
	for _, p := range params {
		use += " <" + argName(p.Name) + ">"
	}
	hasBody := op.RequestBody != nil
	// Deletes, and changes made without a body (start, stop, send, ...), get
	// the same confirmation as hand-written deletes.
	confirm := op.Method == "DELETE" || op.Method != "GET" && !hasBody
	var file string
	cmd := &cobra.Command{
		Use:         use,
		Short:       generatedShort(op),
		Long:        generatedLong(op, params, confirm, doc),
		Args:        cobra.ExactArgs(len(params)),
		Annotations: map[string]string{generatedAnnotation: "true", generatedRouteAnnotation: op.Method + " " + op.Path},
	}
	cmd.RunE = func(cmd *cobra.Command, args []string) error {
		path := op.Path
		for i, p := range params {
			if args[i] == "" {
				return fmt.Errorf("<%s> must not be empty", argName(p.Name))
			}
			path = strings.ReplaceAll(path, "{"+p.Name+"}", url.PathEscape(args[i]))
		}
		var body interface{}
		if file != "" {
			var err error
			if body, err = readJSONBody(file); err != nil {
				return err
			}
		} else if hasBody && op.RequestBody.Required {
			return fmt.Errorf("use --file to provide the JSON request body (run with --help to see its fields)")
		}
		if confirm {
			warning := "This is destructive and cannot be undone."
			if op.Method != "DELETE" {
				warning = "This changes data on the platform."
			}
			if err := confirmAction(cmd, args, warning); err != nil {
				return err
			}
		}
		var data []byte
		var err error
		if op.Method == "DELETE" && body == nil {
			data, _, err = apiClient.Delete(path)
		} else {
			data, _, err = apiClient.Do(op.Method, path, body)
		}
		if err != nil {
			return err
		}
		if len(bytes.TrimSpace(data)) == 0 {
			fmt.Fprintln(os.Stderr, "Done; the API returned no content.")
			return nil
		}
		output.PrintJSON(data)
		return nil
	}
	if hasBody {
		cmd.Flags().StringVar(&file, "file", "", "JSON request body file, or - for stdin (fields are listed above)")
	}
	if err := bindSDKOperation(cmd, op.OperationID, op.sdk()); err != nil {
		return nil, err
	}
	for _, p := range op.Parameters {
		if p.In != "query" || !p.Required {
			continue
		}
		name := sdkQueryFlagName(cmd, p.Name)
		if f := cmd.Flags().Lookup(name); f != nil {
			f.Usage += " (required)"
			if err := cmd.MarkFlagRequired(name); err != nil {
				return nil, err
			}
		}
	}
	return cmd, nil
}

// --- help text ---

func generatedShort(op specOperation) string {
	short := cleanSpecLine(op.Summary, 100)
	if short == "" {
		short = op.Method + " " + op.Path
	}
	if op.Deprecated {
		short += " (deprecated)"
	}
	return short + " [generated]"
}

func generatedLong(op specOperation, params []sdkQueryParameter, confirm bool, doc *specDocument) string {
	var b strings.Builder
	summary := cleanSpecLine(op.Summary, 200)
	if summary != "" {
		b.WriteString(summary + "\n\n")
	}
	if d := cleanSpecText(op.Description, 1200); d != "" && d != summary {
		b.WriteString(d + "\n\n")
	}
	if op.Deprecated {
		b.WriteString("The API marks this operation deprecated.\n\n")
	}
	fmt.Fprintf(&b, "API: %s %s\n", op.Method, op.Path)
	if len(params) > 0 {
		b.WriteString("\nArguments:\n")
		width := 0
		for _, p := range params {
			width = max(width, len(argName(p.Name))+2)
		}
		for _, p := range params {
			fmt.Fprintf(&b, "  %-*s  %s%s\n", width, "<"+argName(p.Name)+">", schemaSummary(p.Schema, doc, 0), describe(p.Description))
		}
	}
	if op.RequestBody != nil {
		writeBodyHelp(&b, op, doc)
	}
	if schema := successSchema(op); schema != nil {
		fmt.Fprintf(&b, "\nReturns: %s (JSON).", schemaSummary(schema, doc, 0))
		if ref := refName(schema); ref != "" {
			fmt.Fprintf(&b, " Full schema: syllable schema get %s", ref)
		} else if items, ok := schema["items"].(map[string]interface{}); ok && refName(items) != "" {
			fmt.Fprintf(&b, " Full schema: syllable schema get %s", refName(items))
		}
		b.WriteString("\n")
	}
	b.WriteString("\nGenerated from the API spec; prints the raw JSON response. --dry-run shows the exact request")
	if op.RequestBody != nil {
		b.WriteString(" and any missing required body fields")
	}
	b.WriteString(".")
	if confirm {
		b.WriteString("\nAsks for confirmation before it runs; pass --yes when running non-interactively.")
	}
	return stripUnsafeRunes(b.String())
}

func writeBodyHelp(b *strings.Builder, op specOperation, doc *specDocument) {
	schema := op.RequestBody.Content["application/json"].Schema
	required := ""
	if op.RequestBody.Required {
		required = ", required"
	}
	fmt.Fprintf(b, "\nRequest body (JSON via --file FILE, or --file - for stdin%s): %s\n", required, schemaSummary(schema, doc, 0))
	if fields, ok := objectFields(schema, doc); ok {
		var req, opt []string
		for name, field := range fields {
			if field.required {
				req = append(req, name)
			} else {
				opt = append(opt, name)
			}
		}
		sort.Strings(req)
		sort.Strings(opt)
		width := 0
		for name := range fields {
			width = max(width, len(name))
		}
		width = min(width, 32)
		list := func(title string, names []string, limit int) {
			if len(names) == 0 {
				return
			}
			fmt.Fprintf(b, "  %s:\n", title)
			for i, name := range names {
				if i == limit {
					fmt.Fprintf(b, "    ... and %d more (see the full schema)\n", len(names)-limit)
					break
				}
				f := fields[name]
				fmt.Fprintf(b, "    %-*s  %s%s\n", width, cleanSpecLine(name, 64), schemaSummary(f.schema, doc, 0), describe(description(f.schema, doc)))
			}
		}
		list("Required fields", req, 40)
		list("Optional fields", opt, 30)
	}
	if example := schemaExample(schema, doc, 0, map[string]bool{}); example != nil {
		data, err := json.MarshalIndent(example, "    ", "  ")
		if err == nil {
			fmt.Fprintf(b, "  Example:\n    %s\n", data)
		}
	}
	if ref := refName(schema); ref != "" {
		fmt.Fprintf(b, "  Full schema: syllable schema get %s\n", ref)
	}
}

func successSchema(op specOperation) map[string]interface{} {
	var codes []string
	for code := range op.Responses {
		if strings.HasPrefix(code, "2") {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	for _, code := range codes {
		if content, ok := op.Responses[code].Content["application/json"]; ok && content.Schema != nil {
			return content.Schema
		}
	}
	return nil
}

func refName(schema map[string]interface{}) string {
	ref, _ := schema["$ref"].(string)
	if !strings.HasPrefix(ref, "#/components/schemas/") {
		return ""
	}
	return strings.TrimPrefix(ref, "#/components/schemas/")
}

func describe(text string) string {
	if d := cleanSpecLine(firstSentence(text), 100); d != "" {
		return " - " + d
	}
	return ""
}

// description is a field's own description, or its referenced schema's.
func description(schema map[string]interface{}, doc *specDocument) string {
	if d, _ := schema["description"].(string); d != "" {
		return d
	}
	if ref := refName(schema); ref != "" {
		d, _ := doc.Schemas[ref]["description"].(string)
		return d
	}
	return ""
}

type objectField struct {
	schema   map[string]interface{}
	required bool
}

// objectFields lists a body's top-level fields, following $ref and allOf. ok
// is false for a body that isn't an object with declared properties.
func objectFields(schema map[string]interface{}, doc *specDocument) (map[string]objectField, bool) {
	fields := map[string]objectField{}
	var walk func(s map[string]interface{}, depth int)
	walk = func(s map[string]interface{}, depth int) {
		if s == nil || depth > 8 {
			return
		}
		if ref := refName(s); ref != "" {
			walk(doc.Schemas[ref], depth+1)
			return
		}
		if parts, ok := s["allOf"].([]interface{}); ok {
			for _, part := range parts {
				if m, ok := part.(map[string]interface{}); ok {
					walk(m, depth+1)
				}
			}
		}
		props, _ := s["properties"].(map[string]interface{})
		for name, p := range props {
			if m, ok := p.(map[string]interface{}); ok {
				f := fields[name]
				f.schema = m
				fields[name] = f
			}
		}
		required, _ := s["required"].([]interface{})
		for _, r := range required {
			if name, ok := r.(string); ok {
				f := fields[name]
				f.required = true
				fields[name] = f
			}
		}
	}
	walk(schema, 0)
	return fields, len(fields) > 0
}

// schemaSummary describes a schema in a few words: a schema name, a type, an
// enum's values, or a union of those.
func schemaSummary(schema map[string]interface{}, doc *specDocument, depth int) string {
	if schema == nil {
		return "any"
	}
	if depth > 3 {
		return "..."
	}
	if ref := refName(schema); ref != "" {
		if values, ok := doc.Schemas[ref]["enum"].([]interface{}); ok {
			return cleanSpecLine(ref, 64) + " (" + enumSummary(values) + ")"
		}
		return cleanSpecLine(ref, 64)
	}
	if value, ok := schema["const"]; ok {
		return jsonText(value)
	}
	if values, ok := schema["enum"].([]interface{}); ok {
		return enumSummary(values)
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		choices, ok := schema[key].([]interface{})
		if !ok {
			continue
		}
		var parts []string
		seen, nullable := map[string]bool{}, false
		for _, c := range choices {
			m, _ := c.(map[string]interface{})
			if m["type"] == "null" {
				nullable = true
				continue
			}
			s := schemaSummary(m, doc, depth+1)
			if !seen[s] {
				seen[s] = true
				parts = append(parts, s)
			}
		}
		if nullable {
			parts = append(parts, "null")
		}
		return strings.Join(parts, " | ")
	}
	if parts, ok := schema["allOf"].([]interface{}); ok && len(parts) == 1 {
		m, _ := parts[0].(map[string]interface{})
		return schemaSummary(m, doc, depth+1)
	}
	switch t := schema["type"].(type) {
	case string:
		switch t {
		case "array":
			items, _ := schema["items"].(map[string]interface{})
			return "array of " + schemaSummary(items, doc, depth+1)
		case "object":
			if extra, ok := schema["additionalProperties"].(map[string]interface{}); ok {
				return "object (map of " + schemaSummary(extra, doc, depth+1) + ")"
			}
			return "object"
		case "string":
			if format, _ := schema["format"].(string); format != "" {
				return "string (" + cleanSpecLine(format, 32) + ")"
			}
		}
		return cleanSpecLine(t, 32)
	case []interface{}:
		var parts []string
		for _, v := range t {
			if s, ok := v.(string); ok {
				parts = append(parts, cleanSpecLine(s, 32))
			}
		}
		return strings.Join(parts, " | ")
	}
	return "any"
}

func enumSummary(values []interface{}) string {
	const limit = 12
	var parts []string
	for i, v := range values {
		if i == limit {
			parts = append(parts, fmt.Sprintf("... %d more", len(values)-limit))
			break
		}
		parts = append(parts, jsonText(v))
	}
	return "one of: " + strings.Join(parts, ", ")
}

func jsonText(v interface{}) string {
	data, _ := json.Marshal(v)
	return cleanSpecLine(string(data), 80)
}

// schemaExample builds an example value: the spec's own example or default
// where it has one, otherwise an object with just its required fields, filled
// with placeholders.
func schemaExample(schema map[string]interface{}, doc *specDocument, depth int, seen map[string]bool) interface{} {
	if schema == nil || depth > 5 {
		return nil
	}
	for _, key := range []string{"default", "example"} {
		if v, ok := schema[key]; ok && v != nil {
			return v
		}
	}
	if examples, ok := schema["examples"].([]interface{}); ok && len(examples) > 0 {
		return examples[0]
	}
	if ref := refName(schema); ref != "" {
		if seen[ref] {
			return map[string]interface{}{}
		}
		next := map[string]bool{ref: true}
		for k := range seen {
			next[k] = true
		}
		return schemaExample(doc.Schemas[ref], doc, depth+1, next)
	}
	if v, ok := schema["const"]; ok {
		return v
	}
	if values, ok := schema["enum"].([]interface{}); ok && len(values) > 0 {
		return values[0]
	}
	for _, key := range []string{"anyOf", "oneOf"} {
		if choices, ok := schema[key].([]interface{}); ok {
			for _, c := range choices {
				if m, _ := c.(map[string]interface{}); m != nil && m["type"] != "null" {
					return schemaExample(m, doc, depth+1, seen)
				}
			}
			return nil
		}
	}
	if fields, ok := objectFields(schema, doc); ok {
		out := map[string]interface{}{}
		for name, f := range fields {
			if f.required {
				out[name] = schemaExample(f.schema, doc, depth+1, seen)
			}
		}
		return out
	}
	typ, _ := schema["type"].(string)
	switch typ {
	case "object":
		return map[string]interface{}{}
	case "array":
		items, _ := schema["items"].(map[string]interface{})
		if item := schemaExample(items, doc, depth+1, seen); item != nil {
			return []interface{}{item}
		}
		return []interface{}{}
	case "integer", "number":
		if m, ok := schema["minimum"].(float64); ok {
			return m
		}
		return 0
	case "boolean":
		return false
	case "string":
		switch schema["format"] {
		case "date-time":
			return "2026-01-01T00:00:00Z"
		case "date":
			return "2026-01-01"
		case "email":
			return "user@example.com"
		case "uri", "url":
			return "https://example.com"
		case "uuid":
			return "00000000-0000-0000-0000-000000000000"
		}
		return "string"
	}
	return nil
}

// --- text from the spec ---

// The upstream spec is untrusted input, so its text is cleaned before it
// reaches a terminal or the docs: no control characters (escape sequences
// included) or bidirectional overrides, bounded length.

var specTextSpaces = regexp.MustCompile(`[ \t]+`)

func stripUnsafeRunes(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\n' || r == '\t':
			return r
		case r < 0x20, r == 0x7f, r >= 0x80 && r <= 0x9f, r == utf8.RuneError:
			return -1
		case r >= 0x202a && r <= 0x202e, r >= 0x2066 && r <= 0x2069, r == 0x200e, r == 0x200f, r == 0x061c:
			return -1
		}
		return r
	}, s)
}

func cleanSpecText(s string, limit int) string {
	lines := strings.Split(stripUnsafeRunes(s), "\n")
	var out []string
	blank := false
	for _, line := range lines {
		line = strings.TrimSpace(specTextSpaces.ReplaceAllString(line, " "))
		if line == "" {
			if !blank && len(out) > 0 {
				out = append(out, "")
			}
			blank = true
			continue
		}
		out = append(out, line)
		blank = false
	}
	return truncateRunes(strings.TrimSpace(strings.Join(out, "\n")), limit)
}

func cleanSpecLine(s string, limit int) string {
	return truncateRunes(strings.TrimSpace(specTextSpaces.ReplaceAllString(strings.ReplaceAll(stripUnsafeRunes(s), "\n", " "), " ")), limit)
}

func truncateRunes(s string, limit int) string {
	if utf8.RuneCountInString(s) <= limit {
		return s
	}
	runes := []rune(s)
	return strings.TrimSpace(string(runes[:limit-1])) + "…"
}

func firstSentence(s string) string {
	s = cleanSpecLine(s, 400)
	if i := strings.Index(s, ". "); i >= 0 {
		return s[:i+1]
	}
	return s
}

// --- the docs block ---

const (
	generatedDocsBegin = "<!-- BEGIN GENERATED COMMANDS: written by `go generate ./cmd`; do not edit by hand -->"
	generatedDocsEnd   = "<!-- END GENERATED COMMANDS -->"
)

// generatedCommandsMarkdown renders the docs block listing the generated
// commands registered under root.
func generatedCommandsMarkdown(root *cobra.Command, doc *specDocument) string {
	var b strings.Builder
	b.WriteString("Commands for API operations that don't have a hand-written command yet are generated from the API spec, so a new endpoint is usable as soon as the CLI syncs to it. ")
	b.WriteString("They take a JSON request body through `--file` (`--file -` reads stdin), print the raw JSON response, and ask for confirmation on deletes and other changes made without a body (pass `--yes` when non-interactive). ")
	b.WriteString("Run one with `--help` for its arguments, query flags, and request fields with an example body.\n\n")
	byID := map[string]specOperation{}
	for _, op := range doc.Operations {
		byID[op.OperationID] = op
	}
	var rows []string
	walkCommands(root, func(c *cobra.Command) {
		if c.Annotations[generatedAnnotation] != "true" {
			return
		}
		op := byID[c.Annotations["syllable.sdk.operation"]]
		summary := strings.NewReplacer("|", `\|`, "`", "'").Replace(cleanSpecLine(op.Summary, 120))
		usage := strings.TrimPrefix(c.Parent().CommandPath()+" "+c.Use, root.Name()+" ")
		rows = append(rows, fmt.Sprintf("| `syllable %s` | `%s %s` | %s |", usage, op.Method, op.Path, summary))
	})
	if len(rows) == 0 {
		b.WriteString("_None right now: every API operation has a hand-written command._\n")
		return b.String()
	}
	sort.Strings(rows)
	b.WriteString("| Command | API | What it does |\n|---|---|---|\n")
	b.WriteString(strings.Join(rows, "\n") + "\n")
	return b.String()
}

// replaceGeneratedDocsBlock swaps the text between the markers in a docs file.
func replaceGeneratedDocsBlock(text, block string) (string, error) {
	start := strings.Index(text, generatedDocsBegin)
	end := strings.Index(text, generatedDocsEnd)
	if start < 0 || end < start {
		return "", fmt.Errorf("missing the generated-commands markers")
	}
	return text[:start+len(generatedDocsBegin)] + "\n" + block + text[end:], nil
}

func walkCommands(c *cobra.Command, visit func(*cobra.Command)) {
	visit(c)
	for _, child := range c.Commands() {
		walkCommands(child, visit)
	}
}
