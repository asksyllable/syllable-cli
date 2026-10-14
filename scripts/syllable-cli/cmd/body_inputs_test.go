package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func bodyFile(t *testing.T, body string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(file, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	return file
}

func executeBodyCommand(t *testing.T, cmd *cobra.Command, args ...string) string {
	t.Helper()
	cmd.SetArgs(args)
	var commandErr error
	out := captureStdout(func() { commandErr = cmd.Execute() })
	if commandErr != nil {
		t.Fatalf("%s: %v", cmd.Name(), commandErr)
	}
	return out
}

func assertRequestJSON(t *testing.T, r *http.Request, want string) {
	t.Helper()
	if got := r.Header.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	var actual, expected interface{}
	requestDecoder := json.NewDecoder(r.Body)
	requestDecoder.UseNumber()
	if err := requestDecoder.Decode(&actual); err != nil {
		t.Errorf("decode request: %v", err)
		return
	}
	wantDecoder := json.NewDecoder(strings.NewReader(want))
	wantDecoder.UseNumber()
	if err := wantDecoder.Decode(&expected); err != nil {
		t.Errorf("decode expected: %v", err)
		return
	}
	if !reflect.DeepEqual(actual, expected) {
		t.Errorf("request body = %#v, want %#v", actual, expected)
	}
}

func TestBodyInputPreservesJSONValues(t *testing.T) {
	const payload = `{"id":9007199254740993,"nested":{"number":0,"flag":false,"text":"","null":null,"items":[]},"number":1.234567890123456789}`
	body, err := readJSONBody(bodyFile(t, payload))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, fragment := range []string{`9007199254740993`, `1.234567890123456789`, `"flag":false`, `"number":0`, `"text":""`, `"null":null`, `"items":[]`} {
		if !bytes.Contains(encoded, []byte(fragment)) {
			t.Errorf("serialized body %s lost %s", encoded, fragment)
		}
	}
}

func TestBodyInputRejectsTrailingValues(t *testing.T) {
	for _, payload := range []string{`{} {}`, `{} false`, `{} trailing`, ``} {
		t.Run(payload, func(t *testing.T) {
			if _, err := readJSONBody(bodyFile(t, payload)); err == nil {
				t.Errorf("accepted invalid full-body JSON %q", payload)
			}
		})
	}
}

func TestAgentsTestMessageFullBody(t *testing.T) {
	const payload = `{"agent_id":"42","test_id":"full-body","service_name":"","source":"","session_start":false,"text":null,"org_name":null,"override_timestamp":null,"extra":{"id":9007199254740993,"count":0,"items":[],"flag":false}}`
	var calls int
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "POST" || r.URL.Path != "/api/v1/agents/test/messages" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		assertRequestJSON(t, r, payload)
		w.Write([]byte(`{"test_id":"full-body","agent_id":"42"}`))
	})
	defer server.Close()
	executeBodyCommand(t, agentsSendTestMessageCmd(), "42", "--file", bodyFile(t, payload), "--timeout", "10")
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestAgentsTestMessageFullBodyStdin(t *testing.T) {
	const payload = `{"test_id":"stdin","service_name":"test","source":"tester","text":"","session_start":false}`
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(payload); err != nil {
		t.Fatal(err)
	}
	w.Close()
	originalStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = originalStdin; r.Close() }()
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		assertRequestJSON(t, r, `{"agent_id":"42","test_id":"stdin","service_name":"test","source":"tester","text":"","session_start":false}`)
		w.Write([]byte(`{}`))
	})
	defer server.Close()
	executeBodyCommand(t, agentsSendTestMessageCmd(), "42", "--file", "-")
}

func TestAgentsTestMessageInlineOrgName(t *testing.T) {
	for _, orgName := range []string{"sdk-body-org", ""} {
		t.Run(orgName, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				var body map[string]interface{}
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Error(err)
				}
				if got, exists := body["org_name"]; !exists || got != orgName {
					t.Errorf("org_name = %v, exists=%v", got, exists)
				}
				w.Write([]byte(`{}`))
			})
			defer server.Close()
			executeBodyCommand(t, agentsSendTestMessageCmd(), "42", "--test-id", "test", "--org-name", orgName, "--session-start=false")
		})
	}
}

func TestFullBodyConflictsFailBeforeRequests(t *testing.T) {
	cases := []struct {
		name    string
		command func() *cobra.Command
		body    string
		args    []string
	}{
		{"agent identifier", agentsSendTestMessageCmd, `{"agent_id":"43"}`, []string{"42"}},
		{"agent inline text", agentsSendTestMessageCmd, `{}`, []string{"42", "--text", ""}},
		{"agent explicit false", agentsSendTestMessageCmd, `{}`, []string{"42", "--session-start=false"}},
		{"user identifier", usersDeleteCmd, `{"email":"other@example.com","reason":"custom"}`, []string{"test@example.com"}},
		{"user reason", usersDeleteCmd, `{}`, []string{"test@example.com", "--reason", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) { t.Error("request sent for conflicting body") })
			defer server.Close()
			cmd := tc.command()
			cmd.SetArgs(append(tc.args, "--file", bodyFile(t, tc.body)))
			captureStdout(func() {
				if err := cmd.Execute(); err == nil {
					t.Error("conflicting body was accepted")
				}
			})
		})
	}
}

func TestWorkflowActivationFullBodySkipsEstimateFetch(t *testing.T) {
	const payload = `{"is_acknowledged":false,"estimate":{"backfill_count":0,"backfill_duration":0,"estimated_daily_count":0,"estimated_daily_duration":0,"estimated_daily_cost":0,"estimated_backfill_cost":0},"extra":{"text":"","null":null,"id":9007199254740993}}`
	var calls int
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Method != "PUT" || r.URL.Path != "/api/v1/insights/workflows/42/activate" {
			t.Errorf("request = %s %s, want activation PUT without GET", r.Method, r.URL.Path)
		}
		assertRequestJSON(t, r, payload)
		w.Write([]byte(`{}`))
	})
	defer server.Close()
	executeBodyCommand(t, insightsWorkflowsActivateCmd(), "42", "--file", bodyFile(t, payload))
	if calls != 1 {
		t.Errorf("calls = %d, want 1", calls)
	}
}

func TestWorkflowActivationConvenienceFetchesEstimate(t *testing.T) {
	const estimate = `{"backfill_count":9007199254740993,"backfill_duration":0,"estimated_daily_count":0,"estimated_daily_duration":0,"estimated_daily_cost":0,"estimated_backfill_cost":0}`
	var methods []string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		if r.Method == "GET" {
			if r.URL.Path != "/api/v1/insights/workflows/42" {
				t.Errorf("GET path = %s", r.URL.Path)
			}
			io.WriteString(w, `{"estimate":`+estimate+`}`)
			return
		}
		if r.URL.Path != "/api/v1/insights/workflows/42/activate" {
			t.Errorf("PUT path = %s", r.URL.Path)
		}
		assertRequestJSON(t, r, `{"is_acknowledged":true,"estimate":`+estimate+`}`)
		w.Write([]byte(`{}`))
	})
	defer server.Close()
	executeBodyCommand(t, insightsWorkflowsActivateCmd(), "42")
	if !reflect.DeepEqual(methods, []string{"GET", "PUT"}) {
		t.Errorf("methods = %v", methods)
	}
}

func TestUsersDeleteCustomBodyAndReason(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		args []string
	}{
		{"custom reason", `{"email":"test@example.com","reason":"User left the organization"}`, []string{"--reason", "User left the organization"}},
		{"empty reason", `{"email":"test@example.com","reason":""}`, []string{"--reason", ""}},
		{"file body", `{"email":"test@example.com","reason":"file reason","extra":{"flag":false,"text":"","id":9007199254740993,"null":null}}`, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "DELETE" || r.URL.Path != "/api/v1/users/" {
					t.Errorf("request = %s %s", r.Method, r.URL.Path)
				}
				assertRequestJSON(t, r, tc.body)
				w.Write([]byte(`{}`))
			})
			defer server.Close()
			args := append([]string{"test@example.com"}, tc.args...)
			if tc.args == nil {
				args = append(args, "--file", bodyFile(t, tc.body))
			}
			executeBodyCommand(t, usersDeleteCmd(), args...)
		})
	}
}

func TestOrganizationMultipartExplicitEmptyValues(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" || r.URL.Path != "/api/v1/organizations/" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		for _, field := range []string{"display_name", "description", "domains", "saml_provider_id", "update_comments"} {
			if !reflect.DeepEqual(r.MultipartForm.Value[field], []string{""}) {
				t.Errorf("multipart %s = %#v, want explicit empty field", field, r.MultipartForm.Value[field])
			}
		}
		if len(r.MultipartForm.File) != 0 {
			t.Errorf("unexpected file fields: %v", r.MultipartForm.File)
		}
		w.Write([]byte(`{}`))
	})
	defer server.Close()
	executeBodyCommand(t, organizationsUpdateCmd(), "--display-name=", "--description=", "--domains=", "--saml-provider-id=", "--update-comments=")
}

func TestOrganizationMultipartOmitsUntouchedFields(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Error(err)
			return
		}
		defer r.MultipartForm.RemoveAll()
		want := map[string][]string{"display_name": {"CLI test"}}
		if !reflect.DeepEqual(r.MultipartForm.Value, want) {
			t.Errorf("multipart fields = %v, want %v", r.MultipartForm.Value, want)
		}
		w.Write([]byte(`{}`))
	})
	defer server.Close()
	executeBodyCommand(t, organizationsUpdateCmd(), "--display-name", "CLI test")
}

func TestVoiceSampleEndpointsReturnUnchangedBinary(t *testing.T) {
	const payload = `{"language_code":"en-US","voice_provider":"OpenAI","voice_display_name":"Alloy","voice_speed":null,"voice_pitch":0,"text":"","apply_pronunciation_overrides":false}`
	for _, tc := range []struct {
		name    string
		command func() *cobra.Command
		path    string
	}{
		{"voice", voiceGroupsSampleCmd, "/api/v1/voice_groups/voices/sample"},
		{"legacy language", languageGroupsSampleCmd, "/api/v1/language_groups/voices/sample"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			audio := []byte{'I', 'D', '3', 0, 0xff, '\n'}
			server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != "POST" || r.URL.Path != tc.path {
					t.Errorf("request = %s %s, want POST %s", r.Method, r.URL.Path, tc.path)
				}
				assertRequestJSON(t, r, payload)
				w.Header().Set("Content-Type", "audio/mpeg")
				w.Write(audio)
			})
			defer server.Close()
			out := executeBodyCommand(t, tc.command(), "--file", bodyFile(t, payload))
			if !bytes.Equal([]byte(out), audio) {
				t.Errorf("stdout = %v, want unchanged audio %v", []byte(out), audio)
			}
		})
	}
}
