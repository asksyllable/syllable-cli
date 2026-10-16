package cmd

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	apispec "github.com/asksyllable/syllable-cli/internal/spec"
	"github.com/spf13/cobra"
)

// TestSDKOperationCoverage fails when a spec operation has no reviewed CLI
// mapping, or a registered command constructs a different method/path. It uses
// the real command constructors and a dry-run client; no API/config credentials
// are used, and no platform calls or mutations occur.
func TestSDKOperationCoverage(t *testing.T) {
	var doc struct {
		Paths map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(apispec.OpenAPI, &doc); err != nil {
		t.Fatal(err)
	}
	body := writeTempJSON(t, `{"id":17,"name":"audit","email":"audit@example.invalid"}`)
	oldClient, oldDryRun := apiClient, dryRun
	t.Cleanup(func() { apiClient, dryRun = oldClient, oldDryRun })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun, dryRun = true, true
	factories := map[string]func() *cobra.Command{
		"agents": agentsCmd, "bridge-phrases": bridgePhrasesCmd, "channels": channelsCmd,
		"conversations": conversationsCmd, "conversation-config": conversationConfigCmd,
		"data-sources": dataSourcesCmd, "events": eventsCmd, "experiments": experimentsCmd,
		"incidents": incidentsCmd, "insights": insightsCmd, "custom-messages": customMessagesCmd,
		"permissions": permissionsCmd, "prompts": promptsCmd, "pronunciations": pronunciationsCmd,
		"roles": rolesCmd, "services": servicesCmd, "session-labels": sessionLabelsCmd,
		"sessions": sessionsCmd, "session-debug": sessionDebugCmd, "tools": toolsCmd,
		"directory": directoryCmd, "dashboards": dashboardsCmd, "organizations": organizationsCmd,
		"outbound": outboundCmd, "language-groups": languageGroupsCmd, "voice-groups": voiceGroupsCmd,
		"takeouts": takeoutsCmd, "users": usersCmd,
	}
	excluded := consoleOnlyOperations
	values := map[string]string{
		"tool_name": "audit", "test_id": "audit-test", "job_id": "audit-job", "file_name": "audit.bin",
		"user_email": "audit@example.invalid", "channel_manager_service": "twilio", "channel_manager_sid": "audit-sid", "tool_call_id": "audit-call",
	}
	seen := map[string]bool{}
	covered := 0
	for path, methods := range doc.Paths {
		for method, raw := range methods {
			switch method {
			case "get", "post", "put", "patch", "delete", "head", "options", "trace":
			default:
				continue
			}
			var operation struct {
				OperationID string `json:"operationId"`
			}
			if err := json.Unmarshal(raw, &operation); err != nil {
				t.Fatal(err)
			}
			id := operation.OperationID
			if id == "" || seen[id] {
				t.Fatalf("missing or duplicate SDK operationId at %s %s", method, path)
			}
			seen[id] = true
			if reason, ok := excluded[id]; ok {
				t.Logf("excluded %s: %s", id, reason)
				continue
			}
			argv, ok := sdkOperationCommands[id]
			if !ok {
				// Generated commands get their own dry-run checks in
				// generated_commands_test.go.
				if generatedCommandFor(rootCmd, id) == nil {
					t.Errorf("SDK operation has neither a hand-written nor a generated command: %s %s (%s)", method, path, id)
				}
				covered++
				continue
			}
			covered++
			t.Run(id, func(t *testing.T) {
				if id == "insights_workflow_activate" {
					// Activation reads the current estimate before its PUT. Supply
					// that prerequisite with an in-process transport, then intercept
					// the mutation as a dry-run result. No network access is possible.
					previous := apiClient
					stub := *apiClient
					stub.DryRun = false
					stub.HTTPClient = &http.Client{Transport: sdkAuditTransport(func(r *http.Request) (*http.Response, error) {
						if r.Method == "GET" && r.URL.Path == "/api/v1/insights/workflows/17" {
							return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"estimate":{}}`))}, nil
						}
						data, _ := json.Marshal(map[string]string{"method": r.Method, "url": r.URL.String()})
						return nil, &client.DryRunResult{Output: data}
					})}
					apiClient = &stub
					defer func() { apiClient = previous }()
				}
				args := append([]string(nil), argv...)
				for i := range args {
					if args[i] == "$body" {
						args[i] = body
					}
				}
				registered, _, err := rootCmd.Find(args)
				if err != nil || registered.RunE == nil {
					t.Fatalf("CLI command is not registered: %v (%v)", args, err)
				}
				factory, ok := factories[args[0]]
				if !ok {
					t.Fatalf("missing command factory: %s", args[0])
				}
				cmd, remaining, err := factory().Find(args[1:])
				if err != nil {
					t.Fatal(err)
				}
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				if err := cmd.ParseFlags(remaining); err != nil {
					t.Fatal(err)
				}
				if err := cmd.ValidateRequiredFlags(); err != nil {
					t.Fatal(err)
				}
				positional := cmd.Flags().Args()
				if cmd.Args != nil {
					if err := cmd.Args(cmd, positional); err != nil {
						t.Fatal(err)
					}
				}
				var result *client.DryRunResult
				if err := cmd.RunE(cmd, positional); !errors.As(err, &result) {
					t.Fatalf("command did not produce a dry-run request: %v", err)
				}
				var request struct {
					Method string `json:"method"`
					URL    string `json:"url"`
				}
				if err := json.Unmarshal(result.Output, &request); err != nil {
					t.Fatal(err)
				}
				u, err := url.Parse(request.URL)
				if err != nil {
					t.Fatal(err)
				}
				want := path
				for _, parameter := range strings.Split(path, "/") {
					if strings.HasPrefix(parameter, "{") {
						name := strings.Trim(parameter, "{}")
						value := values[name]
						if value == "" {
							value = "17"
						}
						want = strings.ReplaceAll(want, parameter, value)
					}
				}
				if request.Method != strings.ToUpper(method) || u.Path != want {
					t.Errorf("%v constructs %s %s, SDK expects %s %s", args, request.Method, u.Path, strings.ToUpper(method), want)
				}
			})
		}
	}
	for id := range sdkOperationCommands {
		if !seen[id] {
			t.Errorf("stale CLI mapping for removed SDK operation: %s", id)
		}
	}
	for id := range excluded {
		if !seen[id] {
			t.Errorf("stale SDK exclusion: %s", id)
		}
	}
	t.Logf("covered %d SDK operations; %d deliberate exclusions", covered, len(excluded))
}

type sdkAuditTransport func(*http.Request) (*http.Response, error)

func (f sdkAuditTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
