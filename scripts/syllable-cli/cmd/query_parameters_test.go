package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/spf13/cobra"
)

func newSDKQueryTestRoot(t *testing.T) *cobra.Command {
	t.Helper()
	root := &cobra.Command{Use: "syllable"}
	for _, factory := range []func() *cobra.Command{
		agentsCmd, bridgePhrasesCmd, channelsCmd, conversationsCmd, conversationConfigCmd,
		dataSourcesCmd, eventsCmd, experimentsCmd, incidentsCmd, insightsCmd, customMessagesCmd,
		permissionsCmd, promptsCmd, pronunciationsCmd, rolesCmd, servicesCmd, sessionLabelsCmd,
		sessionsCmd, sessionDebugCmd, toolsCmd, directoryCmd, dashboardsCmd, organizationsCmd,
		outboundCmd, languageGroupsCmd, voiceGroupsCmd, takeoutsCmd, usersCmd,
	} {
		root.AddCommand(factory())
	}
	if err := bindSDKQueryParameters(root); err != nil {
		t.Fatal(err)
	}
	return root
}

// Every SDK query occurrence is exercised through its real command and client,
// including upload/stream commands. Checking help text alone cannot detect a
// flag that is accepted but omitted from the request.
func TestSDKQueryParameterSerialization(t *testing.T) {
	operations, err := loadSDKOperations()
	if err != nil {
		t.Fatal(err)
	}
	body := writeTempJSON(t, `{"id":17,"name":"audit","email":"audit@example.invalid"}`)
	previous, previousDryRun := apiClient, dryRun
	t.Cleanup(func() { apiClient, dryRun = previous, previousDryRun })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun, dryRun = true, true
	checked := 0
	for id, operation := range operations {
		argv, mapped := sdkOperationCommands[id]
		if !mapped {
			continue // console-only organization provisioning/deletion
		}
		var parameters []sdkQueryParameter
		for _, parameter := range operation.Parameters {
			if parameter.In == "query" {
				parameters = append(parameters, parameter)
			}
		}
		if len(parameters) == 0 {
			continue
		}
		t.Run(id, func(t *testing.T) {
			root := newSDKQueryTestRoot(t)
			cmd, _, err := root.Find(argv)
			if err != nil {
				t.Fatal(err)
			}
			_, remaining, err := root.Find(argv)
			if err != nil {
				t.Fatal(err)
			}
			args := append([]string(nil), remaining...)
			for i := range args {
				if args[i] == "$body" {
					args[i] = body
				}
			}
			want := url.Values{}
			for _, parameter := range parameters {
				checked++
				if sdkQueryIsPositional(id, parameter.Name) {
					want[parameter.Name] = []string{"17"}
					continue
				}
				flagName := sdkQueryFlagName(cmd, parameter.Name)
				if cmd.Flags().Lookup(flagName) == nil {
					t.Fatalf("SDK query %s has no flag --%s", parameter.Name, flagName)
				}
				values := sdkQueryTestValues(parameter)
				want[parameter.Name] = values
				for _, value := range values {
					args = append(args, "--"+flagName+"="+value)
				}
			}
			if err := cmd.ParseFlags(args); err != nil {
				t.Fatal(err)
			}
			if err := cmd.ValidateRequiredFlags(); err != nil {
				t.Fatal(err)
			}
			positionals := cmd.Flags().Args()
			if cmd.Args != nil {
				if err := cmd.Args(cmd, positionals); err != nil {
					t.Fatal(err)
				}
			}
			var result *client.DryRunResult
			if err := cmd.RunE(cmd, positionals); !errors.As(err, &result) {
				t.Fatalf("expected request capture: %v", err)
			}
			var request struct{ Method, URL string }
			if err := json.Unmarshal(result.Output, &request); err != nil {
				t.Fatal(err)
			}
			u, err := url.Parse(request.URL)
			if err != nil {
				t.Fatal(err)
			}
			if request.Method != operation.Method || !sdkOperationPathMatches(operation.Path, u.EscapedPath()) {
				t.Fatalf("captured prerequisite request instead of SDK operation: %s %s", request.Method, u.EscapedPath())
			}
			for name, expected := range want {
				if actual := u.Query()[name]; !reflect.DeepEqual(actual, expected) {
					t.Errorf("query %s = %#v, want %#v", name, actual, expected)
				}
			}
		})
	}
	if checked != 339 {
		t.Errorf("checked %d query occurrences; update the reviewed SDK total if the contract changes", checked)
	}
}

func sdkQueryTestValues(parameter sdkQueryParameter) []string {
	switch sdkSchemaType(parameter.Schema) {
	case "array":
		if parameter.Name == "search_field_values" {
			return []string{"value,with,commas & plus+", ""}
		}
		items := sdkTestSchema(parameter.Schema)["items"].(map[string]interface{})
		item := sdkTestSchema(items)
		if choices, ok := item["enum"].([]interface{}); ok && len(choices) > 0 {
			first := choices[0].(string)
			second := first
			if len(choices) > 1 {
				second = choices[1].(string)
			}
			return []string{first, second}
		}
		return []string{"selected,retired", "saved & option+"}
	case "boolean":
		return []string{"false"}
	case "integer", "number":
		return []string{"0"}
	default:
		schema := sdkTestSchema(parameter.Schema)
		if choices, ok := schema["enum"].([]interface{}); ok && len(choices) > 0 {
			return []string{choices[0].(string)}
		}
		if parameter.Name == "reason" || parameter.Name == "comment" || parameter.Name == "metadata" {
			return []string{""}
		}
		if parameter.Name == "timestamp" || strings.Contains(parameter.Name, "datetime") || strings.HasSuffix(parameter.Name, "_time") {
			return []string{"2026-10-13T12:34:56+05:30"}
		}
		return []string{"value & plus+ unicode-é"}
	}
}

func sdkTestSchema(schema map[string]interface{}) map[string]interface{} {
	if ref, ok := schema["$ref"].(string); ok {
		return sdkTestSchema(sdkSchemas[strings.TrimPrefix(ref, "#/components/schemas/")])
	}
	if variants, ok := schema["anyOf"].([]interface{}); ok {
		for _, variant := range variants {
			candidate := variant.(map[string]interface{})
			if candidate["type"] != "null" {
				return sdkTestSchema(candidate)
			}
		}
	}
	return schema
}

func TestSDKQueryConvenienceAndScoping(t *testing.T) {
	previous := apiClient
	t.Cleanup(func() { apiClient = previous })
	var requests []*http.Request
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.HTTPClient = &http.Client{Transport: sdkAuditTransport(func(r *http.Request) (*http.Response, error) {
		requests = append(requests, r)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"items":[]}`))}, nil
	})}
	resource := agentsCmd()
	if err := bindSDKQueryParameters(resource); err != nil {
		t.Fatal(err)
	}
	cmd, _, _ := resource.Find([]string{"list"})
	if err := cmd.ParseFlags([]string{"--search", "", "--search-field", "name & other", "--start-date", "2026-01-01T00:00:00+05:30"}); err != nil {
		t.Fatal(err)
	}
	captureStdout(func() {
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatal(err)
		}
	})
	q := requests[0].URL.Query()
	if !reflect.DeepEqual(q["search_field_values"], []string{""}) || q.Get("search_fields") != "name & other" || q.Get("start_datetime") != "2026-01-01T00:00:00+05:30" {
		t.Fatalf("convenience query values did not round-trip: %v", q)
	}
	if _, _, err := apiClient.Get("/api/v1/agents/17"); err != nil {
		t.Fatal(err)
	}
	if requests[1].URL.RawQuery != "" || apiClient.RequestPath != nil {
		t.Fatal("query overrides leaked outside the command")
	}
}

func TestSDKQueryOverridesMatchOnlyExactOperation(t *testing.T) {
	operation := sdkOperation{Method: "PUT", Path: "/api/v1/tools/{tool_name}"}
	overrides := url.Values{"reason": {"why & now"}}
	for _, tc := range []struct {
		method, path string
		match        bool
	}{
		{"PUT", "/api/v1/tools/saved%2Fname", true},
		{"GET", "/api/v1/tools/saved%2Fname", false},
		{"PUT", "/api/v1/tools/", false},
		{"PUT", "/api/v1/tools/saved/name", false},
		{"PUT", "/api/v1/tools/name/history", false},
	} {
		got, err := applySDKQueryOverrides(operation, tc.method, tc.path, overrides)
		if err != nil {
			t.Fatal(err)
		}
		u, _ := url.Parse(got)
		if tc.match != (u.Query().Get("reason") == "why & now") || !tc.match && got != tc.path {
			t.Errorf("%s %s transformed to %s; match=%v", tc.method, tc.path, got, tc.match)
		}
	}
}

func TestSDKQuerySearchPairsRejectMismatch(t *testing.T) {
	resource := agentsCmd()
	if err := bindSDKQueryParameters(resource); err != nil {
		t.Fatal(err)
	}
	cmd, _, _ := resource.Find([]string{"list"})
	if err := cmd.ParseFlags([]string{"--search-fields", "name", "--search-fields", "description", "--search-values", "value"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "equal numbers") {
		t.Fatalf("expected mismatched search fields/values to be rejected, got %v", err)
	}
}
