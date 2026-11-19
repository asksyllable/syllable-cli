package cmd

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/spf13/cobra"
)

// The four insights workflow operations take an optional
// X-Insight-Workflow-Format-Version header (#225). The flag sends it only when
// set; otherwise the API applies its default.
func TestInsightsWorkflowFormatVersionHeader(t *testing.T) {
	previous, previousYes := apiClient, assumeYes
	t.Cleanup(func() { apiClient, assumeYes = previous, previousYes })
	assumeYes = true
	for _, id := range []string{
		"insights_workflow_inactivate",
		"insights_workflow_sessions",
		"insights_workflow_files",
		"insights_workflow_executions_summary",
	} {
		t.Run(id, func(t *testing.T) {
			for _, tc := range []struct {
				name  string
				flags []string
				want  string
			}{
				{"set", []string{"--insight-workflow-format-version", "2"}, "2"},
				{"omitted", nil, ""},
			} {
				t.Run(tc.name, func(t *testing.T) {
					var requests []*http.Request
					apiClient = client.New("https://api.example.invalid", "unit-test")
					apiClient.HTTPClient = &http.Client{Transport: sdkAuditTransport(func(r *http.Request) (*http.Response, error) {
						requests = append(requests, r)
						return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
					})}
					root := newSDKQueryTestRoot(t)
					cmd, _, err := root.Find(sdkOperationCommands[id])
					if err != nil {
						t.Fatal(err)
					}
					if err := cmd.ParseFlags(append([]string{"17"}, tc.flags...)); err != nil {
						t.Fatal(err)
					}
					captureStdout(func() {
						if err := cmd.RunE(cmd, cmd.Flags().Args()); err != nil {
							t.Fatal(err)
						}
					})
					if len(requests) != 1 {
						t.Fatalf("sent %d requests, want 1", len(requests))
					}
					values := requests[0].Header.Values("X-Insight-Workflow-Format-Version")
					if tc.want == "" && len(values) != 0 || tc.want != "" && (len(values) != 1 || values[0] != tc.want) {
						t.Errorf("X-Insight-Workflow-Format-Version = %v, want %q", values, tc.want)
					}
					if requests[0].Header.Get("Syllable-API-Key") != "unit-test" {
						t.Error("API key header missing")
					}
					if apiClient.RequestHeaders != nil {
						t.Error("header hook leaked outside the command")
					}
				})
			}
		})
	}
}

// Every optional header parameter in the embedded spec gets a flag, on
// hand-written and generated commands alike.
func TestSDKOptionalHeadersHaveFlags(t *testing.T) {
	operations, err := loadSDKOperations()
	if err != nil {
		t.Fatal(err)
	}
	entries, err := operationEntries(rootCmd)
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for id, operation := range operations {
		entry, ok := entries[id]
		if !ok {
			continue // console-only operations have no command
		}
		for _, parameter := range operation.Parameters {
			if parameter.In != "header" || parameter.Required || client.ReservedHeader(parameter.Name) {
				continue
			}
			checked++
			flag := entry.HeaderFlags[parameter.Name]
			if flag == "" {
				t.Errorf("%s: optional header %s has no flag", id, parameter.Name)
				continue
			}
			cmd, _, err := rootCmd.Find(strings.Fields(entry.Command))
			if err != nil || cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%s: %s has no --%s flag", id, entry.Command, flag)
			}
			for _, inline := range entry.Flags {
				if inline == flag {
					t.Errorf("%s: header flag --%s is listed as a body flag", id, flag)
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no optional header parameters checked; the spec has some")
	}
}

func TestSDKHeaderFlagNamesAndValues(t *testing.T) {
	if got := sdkHeaderFlagName("X-Insight-Workflow-Format-Version"); got != "insight-workflow-format-version" {
		t.Errorf("flag name = %q", got)
	}
	if got := sdkHeaderFlagName("Trace_Id"); got != "trace-id" {
		t.Errorf("flag name = %q", got)
	}

	cmd := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error {
		_, _, err := apiClient.Get("/api/v1/probe")
		return err
	}}
	cmd.Flags().String("format-version", "", "an existing flag")
	operation := sdkOperation{ID: "probe", Method: "GET", Path: "/api/v1/probe", Parameters: []sdkQueryParameter{
		{Name: "X-Format-Version", In: "header", Schema: map[string]interface{}{"type": "integer"}},
		{Name: "X-Required", In: "header", Required: true, Schema: map[string]interface{}{"type": "string"}},
		{Name: "Accept", In: "header", Schema: map[string]interface{}{"type": "string"}},
	}}
	if err := bindSDKOperation(cmd, "probe", operation); err != nil {
		t.Fatal(err)
	}
	if cmd.Flags().Lookup("header-format-version") == nil {
		t.Error("a colliding header flag should get the header- prefix")
	}
	if cmd.Flags().Lookup("required") != nil || cmd.Flags().Lookup("accept") != nil {
		t.Error("required and reserved headers must not get flags")
	}

	previous := apiClient
	t.Cleanup(func() { apiClient = previous })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun = true
	if err := cmd.ParseFlags([]string{"--header-format-version", "3"}); err != nil {
		t.Fatal(err)
	}
	var result *client.DryRunResult
	if err := cmd.RunE(cmd, nil); !errors.As(err, &result) {
		t.Fatalf("want a dry-run result, got %v", err)
	}
	if !strings.Contains(string(result.Output), `"headers":{"X-Format-Version":"3"}`) {
		t.Errorf("dry run doesn't show the header: %s", result.Output)
	}
}

// A generated command gets a flag for an optional header too.
func TestGeneratedCommandSendsOptionalHeaders(t *testing.T) {
	previous, previousDryRun := apiClient, dryRun
	t.Cleanup(func() { apiClient, dryRun = previous, previousDryRun })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun, dryRun = true, true
	var result *client.DryRunResult
	if _, _, err := runGenerated(t, "widgets", "either-auth", "--format-version", "2"); !errors.As(err, &result) {
		t.Fatalf("not a dry run: %v", err)
	}
	if !strings.Contains(string(result.Output), `"headers":{"X-Format-Version":"2"}`) {
		t.Errorf("dry run doesn't show the header: %s", result.Output)
	}
	if _, _, err := runGenerated(t, "widgets", "either-auth"); !errors.As(err, &result) {
		t.Fatalf("not a dry run: %v", err)
	}
	if strings.Contains(string(result.Output), `"headers"`) {
		t.Errorf("an unset header flag sent the header: %s", result.Output)
	}
}

func TestSDKHeaderFlagRejectsLineBreaks(t *testing.T) {
	cmd := &cobra.Command{Use: "probe", RunE: func(*cobra.Command, []string) error { return nil }}
	operation := sdkOperation{ID: "probe", Method: "GET", Path: "/api/v1/probe", Parameters: []sdkQueryParameter{
		{Name: "X-Note", In: "header", Schema: map[string]interface{}{"type": "string"}},
	}}
	if err := bindSDKOperation(cmd, "probe", operation); err != nil {
		t.Fatal(err)
	}
	if err := cmd.ParseFlags([]string{"--note", "a\r\nX-Injected: 1"}); err != nil {
		t.Fatal(err)
	}
	if err := cmd.RunE(cmd, nil); err == nil || !strings.Contains(err.Error(), "line break") {
		t.Errorf("want a line-break error, got %v", err)
	}
}
