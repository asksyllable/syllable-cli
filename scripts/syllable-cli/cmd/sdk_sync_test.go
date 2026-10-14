package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func TestExperimentsRequests(t *testing.T) {
	body := `{"name":"Greeting","target_id":7,"variants":[{"name":"Control","agent_id":10,"weight":50,"is_control":true},{"name":"Short","agent_id":11,"weight":50}]}`
	for _, tc := range []struct {
		name, method, path, response string
		cmd                          func() *cobra.Command
		args                         []string
		body                         string
	}{
		{"list", "GET", "/api/v1/experiments/", `{"items":[],"total_count":0}`, experimentsListCmd, nil, ""},
		{"get", "GET", "/api/v1/experiments/42", `{"id":42,"name":"Greeting","variants":[]}`, experimentsGetCmd, []string{"42"}, ""},
		{"create", "POST", "/api/v1/experiments/", `{"id":42}`, func() *cobra.Command { return experimentsWriteCmd(false) }, nil, body},
		{"update", "PUT", "/api/v1/experiments/42", `{"id":42}`, func() *cobra.Command { return experimentsWriteCmd(true) }, []string{"42"}, `{"name":"Updated","variants":[]}`},
		{"start", "POST", "/api/v1/experiments/42/start", `{"experiment":{"id":42,"status":"running"},"warnings":["Check the agent"]}`, func() *cobra.Command { return experimentsActionCmd("start") }, []string{"42"}, ""},
		{"stop", "POST", "/api/v1/experiments/42/stop", `{"id":42,"status":"stopped"}`, func() *cobra.Command { return experimentsActionCmd("stop") }, []string{"42"}, ""},
		{"results", "GET", "/api/v1/experiments/42/results", `{"experiment_id":42,"variants":[{"variant_id":1,"session_count":2}]}`, experimentsResultsCmd, []string{"42"}, ""},
		{"delete", "DELETE", "/api/v1/experiments/42", `{"deleted":true}`, experimentsCmd, []string{"delete", "42"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var called bool
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				called = true
				if r.Method != tc.method || r.URL.Path != tc.path {
					t.Errorf("request = %s %s, want %s %s", r.Method, r.URL.Path, tc.method, tc.path)
				}
				data, _ := io.ReadAll(r.Body)
				if tc.body != "" {
					var want, got interface{}
					json.Unmarshal([]byte(tc.body), &want)
					if err := json.Unmarshal(data, &got); err != nil || !reflect.DeepEqual(got, want) {
						t.Errorf("body = %s, want %s", data, tc.body)
					}
				} else if len(data) != 0 {
					t.Errorf("unexpected body: %s", data)
				}
				if tc.name == "list" && (r.URL.Query().Get("page") != "0" || r.URL.Query().Get("limit") != "25") {
					t.Errorf("unexpected pagination: %s", r.URL.RawQuery)
				}
				io.WriteString(w, tc.response)
			})
			defer server.Close()
			cmd := tc.cmd()
			args := append([]string{}, tc.args...)
			if tc.body != "" {
				args = append(args, "--file", writeTempJSON(t, tc.body))
			}
			cmd.SetArgs(args)
			var err error
			out := captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("command made no request")
			}
			var got, want interface{}
			if json.Unmarshal([]byte(out), &got) != nil {
				t.Fatalf("invalid JSON output: %s", out)
			}
			json.Unmarshal([]byte(tc.response), &want)
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("response changed: %s", out)
			}
		})
	}
}

func TestExperimentsCreateStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = old; r.Close() }()
	io.WriteString(w, `{"name":"stdin experiment","target_id":7}`)
	w.Close()
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["name"] != "stdin experiment" {
			t.Errorf("unexpected body: %v (%v)", body, err)
		}
		io.WriteString(w, `{"id":42}`)
	})
	defer server.Close()
	cmd := experimentsWriteCmd(false)
	cmd.SetArgs([]string{"--file", "-"})
	var execErr error
	captureStdout(func() { execErr = cmd.Execute() })
	if execErr != nil {
		t.Fatal(execErr)
	}
}

func TestSDKListQueries(t *testing.T) {
	for _, resource := range []string{"experiments", "executions", "sessions", "files"} {
		t.Run(resource, func(t *testing.T) {
			path := "/api/v1/experiments/"
			var cmd *cobra.Command
			var args []string
			if resource == "experiments" {
				cmd = experimentsListCmd()
			} else {
				path = "/api/v1/insights/workflows/42/" + resource
				cmd = insightsWorkflowRowsCmd(resource)
				args = []string{"42"}
			}
			field := "status"
			if resource == "sessions" {
				field = "session_id"
			}
			if resource == "files" {
				field = "filename"
			}
			args = append(args, "--page", "2", "--limit", "10", "--search-field", field, "--search", "A & B+", "--order-by", "created_at", "--order-by-direction", "desc", "--start-date", "2026-10-01T00:00:00+00:00", "--end-date", "2026-10-02T00:00:00Z")
			want := url.Values{"page": {"2"}, "limit": {"10"}, "search_fields": {field}, "search_field_values": {"A & B+"}, "order_by": {"created_at"}, "order_by_direction": {"desc"}, "start_datetime": {"2026-10-01T00:00:00+00:00"}, "end_datetime": {"2026-10-02T00:00:00Z"}}
			if resource == "experiments" || resource == "executions" {
				args = append(args, "--api-fields", "id,status")
				want["fields"] = []string{"id", "status"}
			}
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != path || !reflect.DeepEqual(r.URL.Query(), want) {
					t.Errorf("unexpected request: %s %s; want %s?%s", r.Method, r.URL, path, want.Encode())
				}
				io.WriteString(w, `{"items":[],"total_count":0}`)
			})
			defer server.Close()
			cmd.SetArgs(args)
			var err error
			captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWorkflowExecutionSummary(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/insights/workflows/42/executions/summary" || r.URL.Query().Get("start_datetime") != "2026-10-01T00:00:00+00:00" || r.URL.Query().Get("end_datetime") != "2026-10-02T00:00:00Z" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL)
		}
		io.WriteString(w, `{"pending":1,"processing":2,"completed":3,"failed":4,"total":10}`)
	})
	defer server.Close()
	viper.Set("output", "table")
	defer viper.Set("output", "json")
	cmd := insightsWorkflowExecutionSummaryCmd()
	cmd.SetArgs([]string{"42", "--start-date", "2026-10-01T00:00:00+00:00", "--end-date", "2026-10-02T00:00:00Z"})
	var err error
	out := captureStdout(func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Completed") || !strings.Contains(out, "10") {
		t.Fatalf("missing counts: %s", out)
	}
}

func TestCatalogSelectionQueries(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		cmd        func() *cobra.Command
		args       []string
		query      url.Values
	}{
		{"voices", "/api/v1/agents/voices/available", agentsVoicesCmd, []string{"--selected-voice", "a:b + c", "--selected-voice", "d:e"}, url.Values{"selected_voice": {"a:b + c", "d:e"}}},
		{"stt", "/api/v1/agents/stt/available", agentsSTTProvidersCmd, []string{"--selected-provider", "Deepgram Nova 3 (On-Prem)"}, url.Values{"selected_provider": {"Deepgram Nova 3 (On-Prem)"}}},
		{"llms", "/api/v1/prompts/llms/supported", promptsSupportedLLMsCmd, []string{"--selected-model", "retired/model +"}, url.Values{"selected_model": {"retired/model +"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "GET" || r.URL.Path != tc.path || !reflect.DeepEqual(r.URL.Query(), tc.query) {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				io.WriteString(w, `[]`)
			})
			defer server.Close()
			cmd := tc.cmd()
			cmd.SetArgs(tc.args)
			var err error
			captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCardClick(t *testing.T) {
	for _, fileMode := range []bool{false, true} {
		t.Run(map[bool]string{false: "flags", true: "file"}[fileMode], func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.EscapedPath() != "/api/v1/agents/test/test%2Fid/card-click" {
					t.Errorf("unexpected request: %s %s", r.Method, r.URL)
				}
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body["action_id"] != "book" || body["action_type"] != "cta" {
					t.Errorf("unexpected body: %v (%v)", body, err)
				}
				io.WriteString(w, `{"status":"ok"}`)
			})
			defer server.Close()
			args := []string{"test/id", "--action-id", "book", "--action-type", "cta"}
			if fileMode {
				args = []string{"test/id", "--file", writeTempJSON(t, `{"action_id":"book","action_type":"cta"}`)}
			}
			cmd := agentsCardClickCmd()
			cmd.SetArgs(args)
			var err error
			captureStdout(func() { err = cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestDirectoryDeletionFilters(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/directory_members/" || r.URL.Query().Get("include_deleted") != "true" || r.URL.Query().Get("status") != "deleted" || r.URL.Query().Get("response_format") != "raw" {
			t.Errorf("unexpected request: %s", r.URL)
		}
		io.WriteString(w, `{"items":[],"total_count":0}`)
	})
	defer server.Close()
	cmd := directoryListCmd()
	cmd.SetArgs([]string{"--include-deleted", "--status", "deleted", "--format", "raw"})
	var err error
	captureStdout(func() { err = cmd.Execute() })
	if err != nil {
		t.Fatal(err)
	}
}

func TestSyncCommandValidation(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("invalid input made an API request") })
	defer server.Close()
	for _, tc := range []struct {
		cmd  *cobra.Command
		args []string
	}{
		{experimentsWriteCmd(false), nil}, {experimentsWriteCmd(true), []string{"42"}},
		{agentsCardClickCmd(), []string{"test"}}, {agentsCardClickCmd(), []string{"test", "--action-id", "book", "--action-type", "invalid"}},
		{directoryListCmd(), []string{"--status", "invalid"}}, {directoryListCmd(), []string{"--format", "invalid"}},
	} {
		tc.cmd.SilenceErrors = true
		tc.cmd.SilenceUsage = true
		tc.cmd.SetArgs(tc.args)
		if err := tc.cmd.Execute(); err == nil {
			t.Errorf("%s %v should fail", tc.cmd.Use, tc.args)
		}
	}
}

func TestSyncTables(t *testing.T) {
	for _, tc := range []struct {
		name, response, contains string
		cmd                      *cobra.Command
		args                     []string
	}{
		{"experiment-list", `{"items":[{"id":42,"name":"Greeting","target_id":null,"variants":[],"status":"stopped"}],"total_count":1}`, "Greeting", experimentsListCmd(), nil},
		{"experiment-get", `{"id":42,"name":"Greeting","target_id":null,"variants":[{"id":1,"name":"Base","weight":100,"is_control":true,"agent_id":5,"agent_name":"Support"}]}`, "(control)", experimentsGetCmd(), []string{"42"}},
		{"experiment-results", `{"experiment_id":42,"variants":[{"variant_id":55,"session_count":123}]}`, "123", experimentsResultsCmd(), []string{"42"}},
		{"executions", `{"items":[{"id":1,"session_id":null,"upload_file_id":3,"priority":2,"retry_count":4,"status":"FAILED","error_message":"LLM timeout"}],"total_count":1}`, "LLM timeout", insightsWorkflowRowsCmd("executions"), []string{"42"}},
		{"sessions", `{"items":[{"session_id":15702,"status":"COMPLETED","results":{"tool":{"key":[true]}}}],"total_count":1}`, "15702", insightsWorkflowRowsCmd("sessions"), []string{"42"}},
		{"files", `{"items":[{"upload_file_id":88,"filename":null,"status":"PENDING","analyzed_at":null}],"total_count":1}`, "PENDING", insightsWorkflowRowsCmd("files"), []string{"42"}},
		{"stt", `[{"provider":"deepgram","display_name":"Deepgram","status":"retired","sunset_date":"2026-10-01","fallback":"new-provider"}]`, "new-provider", agentsSTTProvidersCmd(), nil},
		{"voices", `[{"provider":"OpenAI","display_name":"Alloy","status":"deprecated","sunset_date":"2026-12-01","fallback":"new-voice"}]`, "new-voice", agentsVoicesCmd(), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) { io.WriteString(w, tc.response) })
			defer server.Close()
			viper.Set("output", "table")
			defer viper.Set("output", "json")
			tc.cmd.SetArgs(tc.args)
			var err error
			out := captureStdout(func() { err = tc.cmd.Execute() })
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(out, tc.contains) {
				t.Fatalf("missing %q: %s", tc.contains, out)
			}
		})
	}
}
