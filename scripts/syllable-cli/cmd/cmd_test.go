package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// setupTestServer creates a test HTTP server and configures the global apiClient.
// Returns the server (caller must defer server.Close()) and a request log channel.
func setupTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	apiClient = client.New(server.URL, "test-key")
	viper.Set("output", "json")
	// Tests run non-interactively; skip the destructive-action confirmation
	// prompt (#118) unless a test opts back in by setting assumeYes = false.
	assumeYes = true
	return server
}

// captureStdout captures stdout output from a function call.
func captureStdout(fn func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	fn()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// captureStderr captures stderr output from a function call.
func captureStderr(fn func()) string {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	fn()
	w.Close()
	os.Stderr = old
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

// --- Command structure tests ---

func TestCmdRequiresNoAuth(t *testing.T) {
	// nil: guard avoids panic; treat as no auth required
	if cmdRequiresNoAuth(nil) {
		t.Error("cmdRequiresNoAuth(nil) should be false")
	}

	// Root name is "syllable", not in noAuthCommandNames
	if cmdRequiresNoAuth(rootCmd) {
		t.Error("root command should require auth")
	}

	// Subcommands that require auth (e.g. agents)
	var agentsCmd *cobra.Command
	for _, c := range rootCmd.Commands() {
		if c.Name() == "agents" {
			agentsCmd = c
			break
		}
	}
	if agentsCmd != nil && cmdRequiresNoAuth(agentsCmd) {
		t.Error("agents command should require auth")
	}

	// Completion subtree (Cobra adds it by default): all children should skip auth
	for _, c := range rootCmd.Commands() {
		if c.Name() != "completion" {
			continue
		}
		for _, sub := range c.Commands() {
			if !cmdRequiresNoAuth(sub) {
				t.Errorf("completion %s should not require auth", sub.Name())
			}
		}
		return
	}
}

func TestRootCommandHasSubcommands(t *testing.T) {
	expected := []string{
		"agents", "channels", "conversations", "prompts", "tools",
		"sessions", "outbound", "users", "directory", "insights",
		"custom-messages", "language-groups", "organizations", "schema",
		"data-sources", "voice-groups", "services", "roles", "incidents",
		"pronunciations", "session-labels", "session-debug", "takeouts",
		"events", "permissions", "conversation-config", "dashboards",
	}

	commands := rootCmd.Commands()
	names := make(map[string]bool)
	for _, c := range commands {
		names[c.Name()] = true
	}

	for _, exp := range expected {
		if !names[exp] {
			t.Errorf("root command missing subcommand %q", exp)
		}
	}
}

func TestAgentsCommandHasSubcommands(t *testing.T) {
	cmd := agentsCmd()
	expected := []string{"list", "get", "create", "update", "delete", "send-test-message", "voices"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("agents command missing subcommand %q", exp)
		}
	}
}

func TestPromptsCommandHasSubcommands(t *testing.T) {
	cmd := promptsCmd()
	expected := []string{"list", "get", "create", "update", "delete", "history", "supported-llms"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("prompts command missing subcommand %q", exp)
		}
	}
}

func TestToolsCommandHasSubcommands(t *testing.T) {
	cmd := toolsCmd()
	expected := []string{"list", "get", "create", "update", "delete"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("tools command missing subcommand %q", exp)
		}
	}
}

func TestSessionsCommandHasSubcommands(t *testing.T) {
	cmd := sessionsCmd()
	expected := []string{"list", "get", "transcript", "timeline", "summary", "latency", "recording", "recording-stream"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("sessions command missing subcommand %q", exp)
		}
	}
}

func TestChannelsCommandHasSubcommands(t *testing.T) {
	cmd := channelsCmd()
	// "delete" intentionally absent — the API has no delete-channel operation (#114).
	expected := []string{"list", "create", "update", "targets", "available-targets", "twilio"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("channels command missing subcommand %q", exp)
		}
	}
}

func TestOutboundCommandHasSubcommands(t *testing.T) {
	cmd := outboundCmd()
	expected := []string{"batches", "campaigns"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("outbound command missing subcommand %q", exp)
		}
	}
}

func TestInsightsCommandHasSubcommands(t *testing.T) {
	cmd := insightsCmd()
	expected := []string{"workflows", "folders", "tool-configs", "tool-definitions", "tools-test"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("insights command missing subcommand %q", exp)
		}
	}
}

func TestSchemaCommandHasSubcommands(t *testing.T) {
	cmd := schemaCmd()
	expected := []string{"list", "get"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("schema command missing subcommand %q", exp)
		}
	}
}

// --- Agents functional tests ---

func TestAgentsList(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.String()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "Test Agent", "type": "inbound", "label": "", "description": "A test", "updated_at": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := agentsListCmd()
	cmd.SetArgs([]string{"--page", "0", "--limit", "10"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(requestPath, "/api/v1/agents/?page=0&limit=10") {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(out, "Test Agent") {
		t.Errorf("output should contain agent name, got: %s", out)
	}
}

func TestAgentsListWithSearch(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.String()
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "total_count": 0})
	})
	defer server.Close()

	cmd := agentsListCmd()
	cmd.SetArgs([]string{"--search", "myagent"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(requestPath, "search_fields=name") {
		t.Errorf("search should include search_fields=name, got: %s", requestPath)
	}
	if !strings.Contains(requestPath, "search_field_values=myagent") {
		t.Errorf("search should include search_field_values=myagent, got: %s", requestPath)
	}
}

func TestAgentsGet(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "42", "name": "My Agent", "type": "inbound",
			"prompt_id": "p1", "timezone": "US/Eastern",
		})
	})
	defer server.Close()

	cmd := agentsGetCmd()
	cmd.SetArgs([]string{"42"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if requestPath != "/api/v1/agents/42" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(out, "My Agent") {
		t.Errorf("output should contain agent name, got: %s", out)
	}
}

func TestAgentsGetRequiresArg(t *testing.T) {
	cmd := agentsGetCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when no argument provided")
	}
}

func TestAgentsCreateWithFile(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new-1"}`))
	})
	defer server.Close()

	// Write a temp file
	tmpFile, _ := os.CreateTemp("", "agent-*.json")
	tmpFile.Write([]byte(`{"name":"file-agent","type":"inbound","prompt_id":"p1","timezone":"UTC"}`))
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	cmd := agentsCreateCmd()
	cmd.SetArgs([]string{"--file", tmpFile.Name()})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["name"] != "file-agent" {
		t.Errorf("expected name=file-agent, got %v", receivedBody["name"])
	}
}

func TestAgentsCreateWithFlags(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"new-2"}`))
	})
	defer server.Close()

	cmd := agentsCreateCmd()
	cmd.SetArgs([]string{"--name", "flag-agent", "--type", "inbound", "--prompt-id", "7", "--timezone", "UTC"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["name"] != "flag-agent" {
		t.Errorf("expected name=flag-agent, got %v", receivedBody["name"])
	}
	// prompt_id must be sent as a JSON number, not a string (#116).
	if receivedBody["prompt_id"] != float64(7) {
		t.Errorf("expected prompt_id=7 (number), got %#v", receivedBody["prompt_id"])
	}
	// Verify default empty maps are included
	if receivedBody["variables"] == nil {
		t.Error("expected variables map in body")
	}
	if receivedBody["tool_headers"] == nil {
		t.Error("expected tool_headers map in body")
	}
}

func TestAgentsCreateMissingFlags(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {})
	defer server.Close()

	cmd := agentsCreateCmd()
	cmd.SetArgs([]string{"--name", "partial"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when required flags are missing")
	}
}

func TestAgentsDelete(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := agentsDeleteCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE method, got %s", method)
	}
	if path != "/api/v1/agents/42" {
		t.Errorf("expected path /api/v1/agents/42, got %s", path)
	}
}

// --- Agents send-test-message tests ---

func TestAgentsSendTestMessage(t *testing.T) {
	var method, requestPath string
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		requestPath = r.URL.Path
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":       "test-123",
			"agent_id":      "42",
			"response_text": "Hello, how can I help you?",
			"text":          "Hi there",
			"response":      map[string]interface{}{"content": map[string]interface{}{"session_id": "sess-1"}},
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"42", "--test-id", "test-123", "--session-start", "--text", "Hi there"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != "POST" {
		t.Errorf("expected POST method, got %s", method)
	}
	if requestPath != "/api/v1/agents/test/messages" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if receivedBody["agent_id"] != "42" {
		t.Errorf("expected agent_id=42, got %v", receivedBody["agent_id"])
	}
	if receivedBody["test_id"] != "test-123" {
		t.Errorf("expected test_id=test-123, got %v", receivedBody["test_id"])
	}
	if receivedBody["session_start"] != true {
		t.Errorf("expected session_start=true, got %v", receivedBody["session_start"])
	}
	if receivedBody["text"] != "Hi there" {
		t.Errorf("expected text='Hi there', got %v", receivedBody["text"])
	}
	if receivedBody["service_name"] != "test" {
		t.Errorf("expected service_name=test, got %v", receivedBody["service_name"])
	}
	if receivedBody["source"] != "tester@syllable.ai" {
		t.Errorf("expected source=tester@syllable.ai, got %v", receivedBody["source"])
	}
	if !strings.Contains(out, "test-123") {
		t.Errorf("output should contain test_id, got: %s", out)
	}
}

func TestAgentsSendTestMessageJSONOutput(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":       "test-456",
			"agent_id":      "10",
			"response_text": "Welcome!",
			"response":      map[string]interface{}{"content": map[string]interface{}{"session_id": "sess-2", "action": "none"}},
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"10", "--test-id", "test-456", "--session-start"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// setupTestServer sets output to json
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("expected valid JSON output, got: %s", out)
	}
	if parsed["test_id"] != "test-456" {
		t.Errorf("expected test_id=test-456 in JSON output, got %v", parsed["test_id"])
	}
	resp, ok := parsed["response"].(map[string]interface{})
	if !ok {
		t.Fatal("expected response object in JSON output")
	}
	content, ok := resp["content"].(map[string]interface{})
	if !ok {
		t.Fatal("expected response.content object in JSON output")
	}
	if content["session_id"] != "sess-2" {
		t.Errorf("expected session_id=sess-2, got %v", content["session_id"])
	}
}

func TestAgentsSendTestMessageRequiresTestID(t *testing.T) {
	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"42"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when --test-id is missing")
	}
}

func TestAgentsSendTestMessageRequiresAgentID(t *testing.T) {
	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"--test-id", "test-1"})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error when agent-id argument is missing")
	}
}

func TestAgentsSendTestMessageNoTextOmitsField(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":  "test-789",
			"agent_id": "5",
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"5", "--test-id", "test-789"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if _, exists := receivedBody["text"]; exists {
		t.Errorf("text field should be omitted on follow-up turns when --text is not provided, got %v", receivedBody["text"])
	}
}

func TestAgentsSendTestMessageSessionStartNoTextSendsEmpty(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":  "test-greet",
			"agent_id": "5",
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"5", "--test-id", "test-greet", "--session-start"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	val, exists := receivedBody["text"]
	if !exists {
		t.Fatal("text field should be present (as empty string) when --session-start is set without --text")
	}
	if val != "" {
		t.Errorf("expected text='' on session start without --text, got %v", val)
	}
}

func TestAgentsSendTestMessageEmptyTextSendsField(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":  "test-silence",
			"agent_id": "5",
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"5", "--test-id", "test-silence", "--session-start", "--text", ""})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	val, exists := receivedBody["text"]
	if !exists {
		t.Fatal("text field should be present when --text is explicitly set to empty string")
	}
	if val != "" {
		t.Errorf("expected text='', got %v", val)
	}
}

func TestAgentsSendTestMessageOverrideTimestamp(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":  "test-ts",
			"agent_id": "42",
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"42", "--test-id", "test-ts", "--session-start", "--override-timestamp", "2030-12-25T09:30:00"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	val, exists := receivedBody["override_timestamp"]
	if !exists {
		t.Fatal("override_timestamp should be present when --override-timestamp is provided")
	}
	if val != "2030-12-25T09:30:00" {
		t.Errorf("expected override_timestamp=2030-12-25T09:30:00, got %v", val)
	}
}

// TestResolveAPIKeyFromEnv verifies the non-interactive auth path: a key in
// SYLLABLE_API_KEY is used directly, taking priority over config resolution
// (so the CLI authenticates in CI/automation with no ~/.syllable/config.yaml).
func TestResolveAPIKeyFromEnv(t *testing.T) {
	t.Setenv("SYLLABLE_API_KEY", "env-key-abc123")
	got, err := resolveAPIKey()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "env-key-abc123" {
		t.Errorf("resolveAPIKey() = %q, want %q (SYLLABLE_API_KEY should take priority)", got, "env-key-abc123")
	}
}

func TestAgentsSendTestMessageNoOverrideTimestampOmitsField(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"test_id":  "test-no-ts",
			"agent_id": "42",
		})
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	cmd.SetArgs([]string{"42", "--test-id", "test-no-ts", "--session-start", "--text", "hi"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if _, exists := receivedBody["override_timestamp"]; exists {
		t.Errorf("override_timestamp should be omitted when --override-timestamp is not provided, got %v", receivedBody["override_timestamp"])
	}
}

// --- Tools functional tests ---

func TestToolsList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "schedule_appointment", "service_name": "epic", "last_updated": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := toolsListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "schedule_appointment") {
		t.Errorf("output should contain tool name, got: %s", out)
	}
}

func TestToolsCreateWithFlags(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"t1"}`))
	})
	defer server.Close()

	cmd := toolsCreateCmd()
	cmd.SetArgs([]string{"--name", "my_tool", "--service-id", "1"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["name"] != "my_tool" {
		t.Errorf("expected name=my_tool, got %v", receivedBody["name"])
	}
	// service_id must be sent as a JSON number, not a string (#116).
	if receivedBody["service_id"] != float64(1) {
		t.Errorf("expected service_id=1 (number), got %#v", receivedBody["service_id"])
	}
	if receivedBody["definition"] == nil {
		t.Error("expected empty definition map in body")
	}
}

// --- Sessions functional tests ---

func TestSessionsList(t *testing.T) {
	var startVal, endVal string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		// Read the decoded query values so the assertion is independent of
		// escaping style and proves the value round-trips intact (#116).
		startVal = r.URL.Query().Get("start_datetime")
		endVal = r.URL.Query().Get("end_datetime")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items":       []interface{}{},
			"total_count": 0,
		})
	})
	defer server.Close()

	// A non-UTC offset contains "+", which is corrupted to a space unless the
	// value is URL-escaped — the exact bug this guards against.
	cmd := sessionsListCmd()
	cmd.SetArgs([]string{"--start-date", "2024-01-01T00:00:00+05:00", "--end-date", "2024-01-31T23:59:59Z"})
	captureStdout(func() {
		cmd.Execute()
	})

	if startVal != "2024-01-01T00:00:00+05:00" {
		t.Errorf("start_datetime not round-tripped, got: %q", startVal)
	}
	if endVal != "2024-01-31T23:59:59Z" {
		t.Errorf("end_datetime not round-tripped, got: %q", endVal)
	}
}

func TestSessionsTranscript(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": "sess-1",
			"transcription": []map[string]interface{}{
				{"role": "agent", "content": "Hello, how can I help?", "time": "00:00:01"},
				{"role": "user", "content": "I need to schedule an appointment", "time": "00:00:05"},
			},
		})
	})
	defer server.Close()

	cmd := sessionsTranscriptCmd()
	cmd.SetArgs([]string{"sess-1"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Hello, how can I help?") {
		t.Errorf("output should contain transcript content, got: %s", out)
	}
}

func TestSessionsTimeline(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("expected GET, got %s", r.Method)
		}
		if r.URL.Path != "/api/v1/sessions/timeline/sess-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"session_id": "sess-1",
			"events": []map[string]interface{}{
				{"kind": "transcript", "offset": "00:00:01", "source": "agent", "text": "Hello, how can I help?"},
				{"kind": "latency", "offset": "00:00:03", "label": "llm", "duration_ms": 420.0},
			},
		})
	})
	defer server.Close()
	viper.Set("output", "table")
	defer viper.Set("output", "json")

	cmd := sessionsTimelineCmd()
	cmd.SetArgs([]string{"sess-1"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Hello, how can I help?") {
		t.Errorf("output should contain transcript event text, got: %s", out)
	}
	if !strings.Contains(out, "420 ms") {
		t.Errorf("output should contain latency duration, got: %s", out)
	}
}

func TestSessionsSummary(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"summary": "Patient called to schedule an appointment",
			"rating":  "positive",
		})
	})
	defer server.Close()

	cmd := sessionsSummaryCmd()
	cmd.SetArgs([]string{"sess-1"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Patient called to schedule") {
		t.Errorf("output should contain summary, got: %s", out)
	}
}

// --- Schema functional tests ---

func TestSchemaList(t *testing.T) {
	viper.Set("output", "json")
	cmd := schemaListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if out == "" {
		t.Error("schema list should produce output")
	}
}

func TestSchemaListFilter(t *testing.T) {
	viper.Set("output", "json")
	cmd := schemaListCmd()
	cmd.SetArgs([]string{"--filter", "agent"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// Should only contain schema names with "agent" (case-insensitive)
	var names []string
	json.Unmarshal([]byte(out), &names)

	for _, n := range names {
		if !strings.Contains(strings.ToLower(n), "agent") {
			t.Errorf("filtered schema list contains %q which doesn't match 'agent'", n)
		}
	}
}

func TestSchemaGet(t *testing.T) {
	viper.Set("output", "json")

	// First, get a valid schema name
	listCmd := schemaListCmd()
	listCmd.SetArgs([]string{"--filter", "agent"})
	listOut := captureStdout(func() {
		listCmd.Execute()
	})
	var names []string
	json.Unmarshal([]byte(listOut), &names)

	if len(names) == 0 {
		t.Skip("no agent schemas found in embedded spec")
	}

	getCmd := schemaGetCmd()
	getCmd.SetArgs([]string{names[0]})
	out := captureStdout(func() {
		if err := getCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(out, names[0]) {
		t.Errorf("schema get output should contain schema name %q, got: %s", names[0], out[:min(len(out), 200)])
	}
}

func TestSchemaGetCaseInsensitive(t *testing.T) {
	viper.Set("output", "json")

	// First get a valid schema name
	listCmd := schemaListCmd()
	listCmd.SetArgs([]string{"--filter", "agent"})
	listOut := captureStdout(func() {
		listCmd.Execute()
	})
	var names []string
	json.Unmarshal([]byte(listOut), &names)

	if len(names) == 0 {
		t.Skip("no agent schemas found in embedded spec")
	}

	// Use lowercase version
	getCmd := schemaGetCmd()
	getCmd.SetArgs([]string{strings.ToLower(names[0])})
	out := captureStdout(func() {
		if err := getCmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if out == "" {
		t.Error("case-insensitive schema get should return output")
	}
}

// --- Global flags tests ---

func TestGlobalFlags(t *testing.T) {
	flags := rootCmd.PersistentFlags()

	expectedFlags := []string{"config", "org", "env", "output"}
	for _, name := range expectedFlags {
		f := flags.Lookup(name)
		if f == nil {
			t.Errorf("root command missing persistent flag %q", name)
		}
	}
}

func TestOutputFlagShorthand(t *testing.T) {
	f := rootCmd.PersistentFlags().Lookup("output")
	if f == nil {
		t.Fatal("output flag not found")
	}
	if f.Shorthand != "o" {
		t.Errorf("output flag shorthand = %q, want 'o'", f.Shorthand)
	}
}

// --- New resource command structure tests ---

func TestDataSourcesCommandHasSubcommands(t *testing.T) {
	cmd := dataSourcesCmd()
	expected := []string{"list", "get", "create", "update", "delete"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("data-sources command missing subcommand %q", exp)
		}
	}
}

func TestVoiceGroupsCommandHasSubcommands(t *testing.T) {
	cmd := voiceGroupsCmd()
	expected := []string{"list", "get", "create", "update", "delete", "sample"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("voice-groups command missing subcommand %q", exp)
		}
	}
}

func TestServicesCommandHasSubcommands(t *testing.T) {
	cmd := servicesCmd()
	expected := []string{"list", "get", "create", "update", "delete"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("services command missing subcommand %q", exp)
		}
	}
}

func TestRolesCommandHasSubcommands(t *testing.T) {
	cmd := rolesCmd()
	expected := []string{"list", "get", "create", "update", "delete"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("roles command missing subcommand %q", exp)
		}
	}
}

func TestIncidentsCommandHasSubcommands(t *testing.T) {
	cmd := incidentsCmd()
	expected := []string{"list", "get", "create", "update", "delete", "organizations"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("incidents command missing subcommand %q", exp)
		}
	}
}

func TestPronunciationsCommandHasSubcommands(t *testing.T) {
	cmd := pronunciationsCmd()
	expected := []string{"list", "get-csv", "upload-csv", "delete-csv", "metadata"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("pronunciations command missing subcommand %q", exp)
		}
	}
}

func TestSessionLabelsCommandHasSubcommands(t *testing.T) {
	cmd := sessionLabelsCmd()
	expected := []string{"list", "get", "create"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("session-labels command missing subcommand %q", exp)
		}
	}
}

func TestSessionDebugCommandHasSubcommands(t *testing.T) {
	cmd := sessionDebugCmd()
	expected := []string{"by-session-id", "by-sid", "tool-result"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("session-debug command missing subcommand %q", exp)
		}
	}
}

func TestTakeoutsCommandHasSubcommands(t *testing.T) {
	cmd := takeoutsCmd()
	expected := []string{"create", "get", "download"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("takeouts command missing subcommand %q", exp)
		}
	}
}

func TestDashboardsCommandHasSubcommands(t *testing.T) {
	cmd := dashboardsCmd()
	expected := []string{"list", "sessions", "session-events", "session-transfers", "session-summary", "fetch-info"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("dashboards command missing subcommand %q", exp)
		}
	}
}

func TestConversationConfigCommandHasSubcommands(t *testing.T) {
	cmd := conversationConfigCmd()
	expected := []string{"bridges", "bridges-update"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("conversation-config command missing subcommand %q", exp)
		}
	}
}

// --- New resource functional tests ---

func TestDataSourcesList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "FAQ Data", "description": "FAQ content", "updated_at": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := dataSourcesListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "FAQ Data") {
		t.Errorf("output should contain data source name, got: %s", out)
	}
	// The LAST_UPDATED column reads the API's "updated_at" field.
	if !strings.Contains(out, "2024-01-01") {
		t.Errorf("output should show the updated_at value, got: %s", out)
	}
}

func TestDataSourcesGet(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "5", "name": "FAQ Data", "text": "Some FAQ content", "updated_at": "2024-02-02",
		})
	})
	defer server.Close()

	cmd := dataSourcesGetCmd()
	cmd.SetArgs([]string{"5"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/data_sources/5" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(out, "FAQ Data") {
		t.Errorf("output should contain data source name, got: %s", out)
	}
	// The detail view reads the API's "text" field (not "content").
	if !strings.Contains(out, "Some FAQ content") {
		t.Errorf("output should contain the data source text, got: %s", out)
	}
}

func TestDataSourcesCreateWithFlags(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":1}`))
	})
	defer server.Close()

	cmd := dataSourcesCreateCmd()
	cmd.SetArgs([]string{"--name", "FAQ", "--text", "Q&A text"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	// The document body field is "text" (not "content"), and "chunk" is required.
	if receivedBody["text"] != "Q&A text" {
		t.Errorf("expected text=Q&A text, got %#v", receivedBody["text"])
	}
	if _, ok := receivedBody["chunk"]; !ok {
		t.Error("expected required chunk field in body")
	}
	if _, ok := receivedBody["content"]; ok {
		t.Errorf("body must not contain a phantom 'content' field, got %#v", receivedBody["content"])
	}
	if receivedBody["name"] != "FAQ" {
		t.Errorf("expected name=FAQ, got %#v", receivedBody["name"])
	}
}

func TestServicesList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "Epic Service", "auth_type": "bearer", "last_updated": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := servicesListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Epic Service") {
		t.Errorf("output should contain service name, got: %s", out)
	}
}

func TestVoiceGroupsList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "English Voice", "description": "Default English", "updated_at": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := voiceGroupsListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "English Voice") {
		t.Errorf("output should contain voice group name, got: %s", out)
	}
}

func TestRolesList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "name": "Admin", "description": "Administrator role", "last_updated": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := rolesListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Admin") {
		t.Errorf("output should contain role name, got: %s", out)
	}
}

func TestIncidentsList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{"id": "1", "title": "System Outage", "status": "open", "severity": "high", "created_at": "2024-01-01", "updated_at": "2024-01-01"},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	cmd := incidentsListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "System Outage") {
		t.Errorf("output should contain incident title, got: %s", out)
	}
}

func TestSessionDebugBySessionID(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"debug": "data"})
	})
	defer server.Close()

	cmd := sessionDebugBySessionIDCmd()
	cmd.SetArgs([]string{"sess-123"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/session_debug/session_id/sess-123" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestOutboundBatchesDelete(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := outboundBatchesDeleteCmd()
	cmd.SetArgs([]string{"batch-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE method, got %s", method)
	}
	if path != "/api/v1/outbound/batches/batch-1" {
		t.Errorf("expected path /api/v1/outbound/batches/batch-1, got %s", path)
	}
}

func TestOutboundBatchesResults(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"results": []interface{}{}})
	})
	defer server.Close()

	cmd := outboundBatchesResultsCmd()
	cmd.SetArgs([]string{"batch-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/outbound/batches/batch-1/results" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestDirectoryUpload(t *testing.T) {
	tmp, err := os.CreateTemp("", "members-*.csv")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("name,type\nAlice,user\n")
	tmp.Close()

	var method, path, contentType string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer server.Close()

	cmd := directoryUploadCmd()
	cmd.SetArgs([]string{"--file", tmp.Name()})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "PUT" {
		t.Errorf("expected PUT method, got %s", method)
	}
	if path != "/api/v1/directory_members/upload/" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data, got %s", contentType)
	}
}

func TestDirectoryUploadStdin(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	go func() {
		w.WriteString("name,type\nAlice,user\n")
		w.Close()
	}()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var method, path, body string
	server := setupTestServer(t, func(w http.ResponseWriter, req *http.Request) {
		method = req.Method
		path = req.URL.Path
		buf, _ := io.ReadAll(req.Body)
		body = string(buf)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer server.Close()

	cmd := directoryUploadCmd()
	cmd.SetArgs([]string{"--file", "-"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "PUT" {
		t.Errorf("expected PUT method, got %s", method)
	}
	if path != "/api/v1/directory_members/upload/" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.Contains(body, "Alice,user") {
		t.Errorf("expected stdin contents in multipart body, got: %s", body)
	}
}

func TestInsightsFoldersUploadFileStdin(t *testing.T) {
	// Locks in the side-benefit fix: doMultipart now honors --file - on every
	// caller, including the long-broken insights folders upload-file command.
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating pipe: %v", err)
	}
	go func() {
		w.WriteString("audio-bytes")
		w.Close()
	}()
	origStdin := os.Stdin
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	var method, path, body string
	server := setupTestServer(t, func(w http.ResponseWriter, req *http.Request) {
		method = req.Method
		path = req.URL.Path
		buf, _ := io.ReadAll(req.Body)
		body = string(buf)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer server.Close()

	cmd := insightsFoldersUploadFileCmd()
	cmd.SetArgs([]string{"folder-1", "--file", "-", "--call-id", "call-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "POST" {
		t.Errorf("expected POST method, got %s", method)
	}
	if path != "/api/v1/insights/folders/folder-1/upload-file" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.Contains(body, "audio-bytes") {
		t.Errorf("expected stdin contents in multipart body, got: %s", body)
	}
}

func TestDirectoryDownload(t *testing.T) {
	var method, path, query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Write([]byte("name,type\n"))
	})
	defer server.Close()

	// default --format normalized
	cmd := directoryDownloadCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET method, got %s", method)
	}
	if path != "/api/v1/directory_members/download/" {
		t.Errorf("unexpected path: %s", path)
	}
	if query != "response_format=normalized" {
		t.Errorf("unexpected default query: %s", query)
	}

	// --format raw
	cmd = directoryDownloadCmd()
	cmd.SetArgs([]string{"--format", "raw"})
	captureStdout(func() {
		cmd.Execute()
	})
	if query != "response_format=raw" {
		t.Errorf("expected raw query, got %s", query)
	}
}

func TestDirectoryHistory(t *testing.T) {
	var method, requestPath, query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		requestPath = r.URL.Path
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := directoryHistoryCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET method, got %s", method)
	}
	if requestPath != "/api/v1/directory_members/42/history" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(query, "page=0") || !strings.Contains(query, "limit=25") {
		t.Errorf("expected page+limit defaults in query, got: %s", query)
	}
}

func TestDirectoryHistoryWithFlags(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := directoryHistoryCmd()
	cmd.SetArgs([]string{"42", "--page", "2", "--limit", "10", "--order-by-direction", "desc"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(query, "page=2") {
		t.Errorf("expected page=2, got: %s", query)
	}
	if !strings.Contains(query, "limit=10") {
		t.Errorf("expected limit=10, got: %s", query)
	}
	if !strings.Contains(query, "order_by_direction=desc") {
		t.Errorf("expected order_by_direction=desc, got: %s", query)
	}
}

func TestDirectoryRestore(t *testing.T) {
	var method, requestPath, contentType string
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		requestPath = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := directoryRestoreCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "PUT" {
		t.Errorf("expected PUT method, got %s", method)
	}
	if requestPath != "/api/v1/directory_members/42/restore" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", contentType)
	}
	if receivedBody == nil {
		t.Error("expected JSON object body, got nil")
	}
}

func TestDirectoryRestoreWithComments(t *testing.T) {
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		json.Unmarshal(body, &receivedBody)
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := directoryRestoreCmd()
	cmd.SetArgs([]string{"42", "--comments", "Brought back by request"})
	captureStdout(func() {
		cmd.Execute()
	})

	if receivedBody["comments"] != "Brought back by request" {
		t.Errorf("expected comments in body, got %v", receivedBody["comments"])
	}
}

func TestDirectoryTest(t *testing.T) {
	var method, requestPath, query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		requestPath = r.URL.Path
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]interface{}{"extension": "1234"})
	})
	defer server.Close()

	cmd := directoryTestCmd()
	cmd.SetArgs([]string{"42", "--timestamp", "2025-12-04T14:29:39Z"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET method, got %s", method)
	}
	if requestPath != "/api/v1/directory_members/42/test" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(query, "timestamp=2025-12-04T14%3A29%3A39Z") {
		t.Errorf("expected URL-encoded timestamp in query, got: %s", query)
	}
}

func TestDirectoryTestDefaultsTimestampToNow(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := directoryTestCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(query, "timestamp=") {
		t.Errorf("expected timestamp query param even without --timestamp, got: %s", query)
	}
}

func TestDirectoryTestWithLanguageCode(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := directoryTestCmd()
	cmd.SetArgs([]string{"42", "--timestamp", "2025-01-01T00:00:00Z", "--language-code", "es-MX"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(query, "language_code=es-MX") {
		t.Errorf("expected language_code=es-MX in query, got: %s", query)
	}
}

func TestOutboundBatchesUpload(t *testing.T) {
	tmp, err := os.CreateTemp("", "contacts-*.csv")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name())
	tmp.WriteString("phone\n+15551234567\n")
	tmp.Close()

	var method, path, contentType string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("{}"))
	})
	defer server.Close()

	cmd := outboundBatchesUploadCmd()
	cmd.SetArgs([]string{"batch-1", "--file", tmp.Name()})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "POST" {
		t.Errorf("expected POST method, got %s", method)
	}
	if path != "/api/v1/outbound/batches/batch-1/upload_batch" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data, got %s", contentType)
	}
}

func TestInsightsWorkflowsDelete(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := insightsWorkflowsDeleteCmd()
	cmd.SetArgs([]string{"wf-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE method, got %s", method)
	}
	if path != "/api/v1/insights/workflows/wf-1" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestInsightsFoldersFiles(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := insightsFoldersFilesCmd()
	cmd.SetArgs([]string{"folder-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/insights/folders/folder-1/files" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestInsightsToolConfigsList(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := insightsToolConfigsListCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/insights/tool-configurations" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestChannelsAvailableTargets(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := channelsAvailableTargetsCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/channels/available-targets" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestChannelsTwilioGet(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "1"})
	})
	defer server.Close()

	cmd := channelsTwilioGetCmd()
	cmd.SetArgs([]string{"ch-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/channels/twilio/ch-1" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestPromptsHistory(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := promptsHistoryCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/prompts/42/history" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestSessionsLatency(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	cmd := sessionsLatencyCmd()
	cmd.SetArgs([]string{"sess-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/sessions/latency/sess-1" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestUsersSendEmail(t *testing.T) {
	var requestPath, method string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		method = r.Method
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := usersSendEmailCmd()
	cmd.SetArgs([]string{"user@example.com"})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/users/user@example.com/send_email" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if method != "POST" {
		t.Errorf("expected POST method, got %s", method)
	}
}

func TestDashboardsSessions(t *testing.T) {
	var requestPath, method string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		method = r.Method
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	cmd := dashboardsSessionsCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/dashboards/sessions" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if method != "POST" {
		t.Errorf("expected POST method, got %s", method)
	}
}

func TestPronunciationsUploadCSV(t *testing.T) {
	tmp, _ := os.CreateTemp("", "pron-*.csv")
	tmp.WriteString("word,pronunciation\nhello,heh-loh\n")
	tmp.Close()
	defer os.Remove(tmp.Name())

	var method, path, contentType string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusAccepted)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := pronunciationsUploadCSVCmd()
	cmd.SetArgs([]string{"--file", tmp.Name(), "--confirm"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "POST" {
		t.Errorf("expected POST, got %s", method)
	}
	if path != "/api/v1/pronunciations/csv" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data, got %s", contentType)
	}
}

func TestPronunciationsUploadCSVRequiresConfirm(t *testing.T) {
	tmp, _ := os.CreateTemp("", "pron-*.csv")
	tmp.WriteString("word,pronunciation\n")
	tmp.Close()
	defer os.Remove(tmp.Name())

	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be hit without --confirm")
	})
	defer server.Close()

	cmd := pronunciationsUploadCSVCmd()
	cmd.SetArgs([]string{"--file", tmp.Name()})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error without --confirm")
	}
}

func TestPronunciationsDeleteCSV(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := pronunciationsDeleteCSVCmd()
	cmd.SetArgs([]string{"--confirm"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE, got %s", method)
	}
	if path != "/api/v1/pronunciations/csv" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestPronunciationsDeleteCSVRequiresConfirm(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be hit without --confirm")
	})
	defer server.Close()

	cmd := pronunciationsDeleteCSVCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error without --confirm")
	}
}

func TestSessionsRecordingStream(t *testing.T) {
	var method, path, query string
	audio := []byte{0xff, 0xfb, 0x90, 0x44}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		query = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Write(audio)
	})
	defer server.Close()

	cmd := sessionsRecordingStreamCmd()
	cmd.SetArgs([]string{"--token", "abc-123"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET, got %s", method)
	}
	if path != "/api/v1/sessions/recording/stream" {
		t.Errorf("unexpected path: %s", path)
	}
	if query != "token=abc-123" {
		t.Errorf("expected token=abc-123 query, got %s", query)
	}
	if out != string(audio) {
		t.Errorf("expected raw audio bytes on stdout, got %q", out)
	}
}

func TestSessionsRecordingStreamRequiresToken(t *testing.T) {
	cmd := sessionsRecordingStreamCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --token missing")
	}
}

func TestOrganizationsGet(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "1", "name": "Acme", "display_name": "Acme Inc."})
	})
	defer server.Close()

	cmd := organizationsGetCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET, got %s", method)
	}
	if path != "/api/v1/organizations/" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestOrganizationsListAliasStillWorks(t *testing.T) {
	var path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"id": "1"})
	})
	defer server.Close()

	cmd := organizationsListCmd()
	if !cmd.Hidden {
		t.Error("organizations list alias should be Hidden")
	}
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if path != "/api/v1/organizations/" {
		t.Errorf("alias should hit same endpoint, got: %s", path)
	}
}

func TestOrganizationsUpdate(t *testing.T) {
	var method, path, contentType, displayName string
	var sawLogoPart bool
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Fatalf("ParseMultipartForm: %v", err)
		}
		displayName = r.MultipartForm.Value["display_name"][0]
		_, sawLogoPart = r.MultipartForm.File["logo"]
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := organizationsUpdateCmd()
	cmd.SetArgs([]string{"--display-name", "Renamed Inc."})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "PUT" {
		t.Errorf("expected PUT, got %s", method)
	}
	if path != "/api/v1/organizations/" {
		t.Errorf("unexpected path: %s", path)
	}
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected multipart/form-data, got %s", contentType)
	}
	if displayName != "Renamed Inc." {
		t.Errorf("expected display_name=Renamed Inc., got %q", displayName)
	}
	if sawLogoPart {
		t.Error("did not expect logo part when --logo omitted")
	}
}

func TestOrganizationsUpdateRequiresDisplayName(t *testing.T) {
	cmd := organizationsUpdateCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error when --display-name missing")
	}
}

func TestUsersDeleteAccountRequiresConfirm(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("server should not be hit without --confirm")
	})
	defer server.Close()

	cmd := usersDeleteAccountCmd()
	cmd.SetArgs([]string{})
	if err := cmd.Execute(); err == nil {
		t.Error("expected error without --confirm")
	}
}

func TestUsersDeleteAccountWithConfirm(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := usersDeleteAccountCmd()
	cmd.SetArgs([]string{"--confirm"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE, got %s", method)
	}
	if path != "/api/v1/users/delete_account" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestConversationConfigBridges(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	cmd := conversationConfigBridgesGetCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if requestPath != "/api/v1/conversation-config/bridges" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
}

func TestConversationConfigBridgesQueryParams(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	cmd := conversationConfigBridgesGetCmd()
	cmd.SetArgs([]string{"--agent-id", "42", "--tool-name", "transfer_call"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(query, "agent_id=42") {
		t.Errorf("expected agent_id=42 in query, got: %s", query)
	}
	if !strings.Contains(query, "tool_name=transfer_call") {
		t.Errorf("expected tool_name=transfer_call in query, got: %s", query)
	}
}

func TestConversationConfigBridgesNoQueryParams(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	// Unset --agent-id must not be sent as agent_id=0.
	cmd := conversationConfigBridgesGetCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if query != "" {
		t.Errorf("expected no query params, got: %s", query)
	}
}

func TestConversationConfigBridgesExplicitAgentIDZero(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	// An explicit --agent-id 0 is distinct from the flag being unset.
	cmd := conversationConfigBridgesGetCmd()
	cmd.SetArgs([]string{"--agent-id", "0"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(query, "agent_id=0") {
		t.Errorf("expected agent_id=0 in query, got: %s", query)
	}
}

func TestConversationConfigBridgesUpdateQueryParams(t *testing.T) {
	var method, requestPath, query string
	var receivedBody map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		requestPath = r.URL.Path
		query = r.URL.RawQuery
		json.NewDecoder(r.Body).Decode(&receivedBody)
		json.NewEncoder(w).Encode(map[string]interface{}{})
	})
	defer server.Close()

	tmp := filepath.Join(t.TempDir(), "bridges.json")
	if err := os.WriteFile(tmp, []byte(`{"messages":["one moment"]}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cmd := conversationConfigBridgesUpdateCmd()
	cmd.SetArgs([]string{"--agent-id", "42", "--file", tmp})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "PUT" {
		t.Errorf("expected PUT, got %s", method)
	}
	if requestPath != "/api/v1/conversation-config/bridges" {
		t.Errorf("unexpected request path: %s", requestPath)
	}
	if !strings.Contains(query, "agent_id=42") {
		t.Errorf("expected agent_id=42 in query, got: %s", query)
	}
	if receivedBody["messages"] == nil {
		t.Errorf("expected messages in request body, got: %v", receivedBody)
	}
}

// --- Spec-sync v0.0.3: new command coverage ---

func TestAgentsLabels(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewEncoder(w).Encode([]string{"sales", "support"})
	})
	defer server.Close()

	cmd := agentsLabelsCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET, got %s", method)
	}
	if path != "/api/v1/agents/labels" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestToolsHistory(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{"items": []interface{}{}, "total_count": 0})
	})
	defer server.Close()

	cmd := toolsHistoryCmd()
	cmd.SetArgs([]string{"5"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET, got %s", method)
	}
	if path != "/api/v1/tools/5/history" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestChannelsTwilioVerifyA2p(t *testing.T) {
	var method, path string
	var body map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		json.NewEncoder(w).Encode(map[string]interface{}{"a2p_approved": true})
	})
	defer server.Close()

	cmd := channelsTwilioNumbersVerifyA2pCmd()
	cmd.SetArgs([]string{"5", "--phone", "+18042221111"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "POST" {
		t.Errorf("expected POST, got %s", method)
	}
	if path != "/api/v1/channels/twilio/5/numbers/verify-a2p-compliance" {
		t.Errorf("unexpected path: %s", path)
	}
	if body["phone"] != "+18042221111" {
		t.Errorf("expected phone in body, got %v", body["phone"])
	}
}

func TestOrganizationsSipIPRangesList(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewEncoder(w).Encode([]interface{}{})
	})
	defer server.Close()

	cmd := organizationsSipIPRangesListCmd()
	cmd.SetArgs([]string{})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "GET" {
		t.Errorf("expected GET, got %s", method)
	}
	// Trailing slash is load-bearing — a redirect would drop the API-key header.
	if path != "/api/v1/organizations/sip_ip_ranges/" {
		t.Errorf("unexpected path: %s", path)
	}
}

func TestOrganizationsSipIPRangesCreate(t *testing.T) {
	var method, path string
	var body map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{"id": 1})
	})
	defer server.Close()

	cmd := organizationsSipIPRangesCreateCmd()
	cmd.SetArgs([]string{"--type", "signaling", "--ip-range", "192.168.1.0/24"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "POST" {
		t.Errorf("expected POST, got %s", method)
	}
	if path != "/api/v1/organizations/sip_ip_ranges/" {
		t.Errorf("unexpected path: %s", path)
	}
	if body["type"] != "signaling" || body["ip_range"] != "192.168.1.0/24" {
		t.Errorf("unexpected body: %v", body)
	}
}

func TestOrganizationsSipIPRangesDelete(t *testing.T) {
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer server.Close()

	cmd := organizationsSipIPRangesDeleteCmd()
	cmd.SetArgs([]string{"7"})
	captureStdout(func() {
		cmd.Execute()
	})

	if method != "DELETE" {
		t.Errorf("expected DELETE, got %s", method)
	}
	if path != "/api/v1/organizations/sip_ip_ranges/7" {
		t.Errorf("unexpected path: %s", path)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// --- Output purity and hint tests ---

// The bridge_phrases spec bump split BridgePhraseMessages into two qualified
// schemas. The unqualified name keeps resolving as an alias to the cortex
// (conversation-config) variant, which is field-for-field identical to what
// the old key held — so lookups that predate the split don't break.
func TestSchemaGetRenamedBridgePhraseMessages(t *testing.T) {
	viper.Set("output", "table")
	defer viper.Set("output", "json")

	cmd := schemaGetCmd()
	cmd.SetArgs([]string{"BridgePhraseMessages"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("aliased schema lookup should succeed, got: %v", err)
		}
	})

	if !strings.Contains(out, "schemas__cortex__v1__bridge_phrases__BridgePhraseMessages") {
		t.Errorf("alias should resolve to the cortex schema, got: %s", out[:min(len(out), 200)])
	}
	if !strings.Contains(out, "first_slow_messages") {
		t.Errorf("resolved schema should be the legacy conversation-config shape, got: %s", out[:min(len(out), 300)])
	}
}

func TestSchemaGetJSONOutputPure(t *testing.T) {
	prev := viper.GetString("output")
	defer viper.Set("output", prev)

	// --output json: no markdown heading, stdout must parse as JSON
	viper.Set("output", "json")
	cmd := schemaGetCmd()
	cmd.SetArgs([]string{"AgentCreate"})
	out := captureStdout(func() {
		cmd.Execute()
	})
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		t.Errorf("schema get -o json output is not pure JSON: %v\nfirst bytes: %.40q", err, out)
	}

	// table (default): heading stays
	viper.Set("output", "table")
	cmd = schemaGetCmd()
	cmd.SetArgs([]string{"AgentCreate"})
	out = captureStdout(func() {
		cmd.Execute()
	})
	if !strings.HasPrefix(out, "# AgentCreate") {
		t.Errorf("table output should keep the heading, got: %.40q", out)
	}
}

func TestTakeoutsDownloadBytes(t *testing.T) {
	// Invalid UTF-8 on purpose: takeout files may be binary (audio, archives).
	payload := []byte{0x50, 0x4b, 0x03, 0x04, 0xff, 0xfe, 0x00, 0x9f}
	var path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write(payload)
	})
	defer server.Close()

	cmd := takeoutsDownloadCmd()
	cmd.SetArgs([]string{"job-1", "export.zip"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if path != "/api/v1/takeouts/get/job-1/file/export.zip" {
		t.Errorf("unexpected path: %s", path)
	}
	if !bytes.Equal([]byte(out), payload) {
		t.Errorf("download not byte-exact: got %x, want %x", out, payload)
	}
}

// fakeCommandTree builds syllable → tools → {list, get <tool-name>, update <agent-id>}
// for hint tests without touching the real rootCmd.
func fakeCommandTree() (list, getByName, updateByID *cobra.Command) {
	root := &cobra.Command{Use: "syllable"}
	parent := &cobra.Command{Use: "tools"}
	list = &cobra.Command{Use: "list", Run: func(*cobra.Command, []string) {}}
	getByName = &cobra.Command{Use: "get <tool-name>", Run: func(*cobra.Command, []string) {}}
	updateByID = &cobra.Command{Use: "update <agent-id>", Run: func(*cobra.Command, []string) {}}
	parent.AddCommand(list, getByName, updateByID)
	root.AddCommand(parent)
	return list, getByName, updateByID
}

func TestHint404ContextAware(t *testing.T) {
	list, getByName, updateByID := fakeCommandTree()

	// list itself 404ing → no hint; the API detail says it all (issue #73)
	if h := hint404(list); h != "" {
		t.Errorf("404 on list should produce no hint, got %q", h)
	}

	// name-keyed command → steer to the NAME column, not IDs (issue #69)
	h := hint404(getByName)
	if !strings.Contains(h, "name") || !strings.Contains(h, "`syllable tools list`") {
		t.Errorf("name-keyed 404 hint should mention names and the list command path, got %q", h)
	}

	// id-keyed command → name the actual list command
	h = hint404(updateByID)
	if !strings.Contains(h, "`syllable tools list`") {
		t.Errorf("id-keyed 404 hint should name the list command path, got %q", h)
	}

	// nil command (defensive) → generic fallback
	if h := hint404(nil); !strings.Contains(h, "list") {
		t.Errorf("nil-command 404 hint should keep the generic fallback, got %q", h)
	}
}

func TestHintForErrorThreadsCommand(t *testing.T) {
	_, getByName, _ := fakeCommandTree()
	err := &client.APIError{StatusCode: 404, Body: []byte(`{"detail":"Tool with name 425 not found"}`)}
	h := hintForError(getByName, err)
	if !strings.Contains(h, "`syllable tools list`") {
		t.Errorf("hintForError should thread the command into the 404 hint, got %q", h)
	}
	if h2 := hintForError(nil, err); h2 == "" {
		t.Error("hintForError with nil command should still produce a 404 hint")
	}
}

// --- Update positional identifier tests (#68) ---

func writeTempJSON(t *testing.T, content string) string {
	t.Helper()
	f := filepath.Join(t.TempDir(), "body.json")
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return f
}

func TestEnsureBodyIdentifier(t *testing.T) {
	// inject numeric id when absent
	body := map[string]interface{}{"description": "x"}
	if err := ensureBodyIdentifier(body, "id", "574", true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["id"] != int64(574) {
		t.Errorf("expected injected id 574, got %#v", body["id"])
	}

	// matching body id passes (encoding/json decodes numbers as float64)
	if err := ensureBodyIdentifier(map[string]interface{}{"id": float64(574)}, "id", "574", true); err != nil {
		t.Errorf("matching id should pass: %v", err)
	}

	// conflicting body id is refused
	if err := ensureBodyIdentifier(map[string]interface{}{"id": float64(575)}, "id", "574", true); err == nil {
		t.Error("conflicting id should error")
	}

	// non-numeric positional for a numeric key is refused
	if err := ensureBodyIdentifier(map[string]interface{}{}, "id", "abc", true); err == nil {
		t.Error("non-numeric id should error")
	}

	// string keys inject as strings (users email, tools name)
	body = map[string]interface{}{}
	if err := ensureBodyIdentifier(body, "email", "a@b.co", false); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if body["email"] != "a@b.co" {
		t.Errorf("expected injected email, got %#v", body["email"])
	}

	// conflicting string identifier is refused
	if err := ensureBodyIdentifier(map[string]interface{}{"name": "other_tool"}, "name", "my_tool", false); err == nil {
		t.Error("conflicting name should error")
	}

	// non-object bodies pass through untouched
	if err := ensureBodyIdentifier([]interface{}{"x"}, "id", "574", true); err != nil {
		t.Errorf("non-object body should be a no-op: %v", err)
	}
}

func TestAgentsUpdateInjectsPositionalID(t *testing.T) {
	var method, path string
	var got map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := agentsUpdateCmd()
	cmd.SetArgs([]string{"574", "--file", writeTempJSON(t, `{"description":"noop"}`)})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if method != "PUT" || path != "/api/v1/agents/" {
		t.Errorf("unexpected request: %s %s", method, path)
	}
	if got["id"] != float64(574) {
		t.Errorf("expected body id 574, got %#v", got["id"])
	}
	if got["description"] != "noop" {
		t.Errorf("body fields should survive injection, got %#v", got)
	}
}

func TestAgentsUpdateConflictingBodyID(t *testing.T) {
	hit := false
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
	})
	defer server.Close()

	cmd := agentsUpdateCmd()
	cmd.SetArgs([]string{"574", "--file", writeTempJSON(t, `{"id": 575}`)})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })

	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Errorf("expected conflict error, got %v", err)
	}
	if hit {
		t.Error("request should not reach the API on identifier conflict")
	}
}

func TestUsersUpdateInjectsPositionalEmail(t *testing.T) {
	var got map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := usersUpdateCmd()
	cmd.SetArgs([]string{"a@b.co", "--file", writeTempJSON(t, `{"role_id": 2}`)})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if got["email"] != "a@b.co" {
		t.Errorf("expected injected email, got %#v", got["email"])
	}
}

func TestToolsUpdateValidatesPositionalName(t *testing.T) {
	var got map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	// name absent from body → injected from the positional
	cmd := toolsUpdateCmd()
	cmd.SetArgs([]string{"my_tool", "--file", writeTempJSON(t, `{"id": 425, "service_id": 1}`)})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	if got["name"] != "my_tool" {
		t.Errorf("expected injected name, got %#v", got["name"])
	}

	// conflicting name in body → refused
	cmd = toolsUpdateCmd()
	cmd.SetArgs([]string{"my_tool", "--file", writeTempJSON(t, `{"id": 425, "name": "other_tool"}`)})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })
	if err == nil || !strings.Contains(err.Error(), "conflicts") {
		t.Errorf("expected conflict error, got %v", err)
	}
}

// --- No-silent-wrong-answers tests (#71, #77) ---

func TestUsersMeNoEmailFailsLoudly(t *testing.T) {
	hit := false
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
	})
	defer server.Close()

	prev := viper.GetString("email")
	defer viper.Set("email", prev)
	viper.Set("email", "")

	cmd := usersMeCmd()
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })

	if err == nil || !strings.Contains(err.Error(), "syllable setup") {
		t.Errorf("expected loud failure pointing at setup, got %v", err)
	}
	if hit {
		t.Error("no request should be made when email is unconfigured")
	}
}

func TestUsersMeWithEmailLooksUpExactUser(t *testing.T) {
	var path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		w.Write([]byte(`{"id": 1, "email": "user@example.com"}`))
	})
	defer server.Close()

	prev := viper.GetString("email")
	defer viper.Set("email", prev)
	viper.Set("email", "user@example.com")

	cmd := usersMeCmd()
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if path != "/api/v1/users/user@example.com" {
		t.Errorf("expected exact email lookup, got %s", path)
	}
}

func TestOverrideTimestampWarning(t *testing.T) {
	cases := []struct {
		ts   string
		warn bool
	}{
		{"2030-12-25T09:30:00", false},        // the honored tz-naive form
		{"2030-12-25T09:30:00.123456", false}, // unconfirmed → no warning (negative checks only)
		{"not-a-real-timestamp", false},       // documented limitation: pure garbage passes silently
		{"2030-12-25", false},                 // date dashes are not an offset
		{"2030-12-25T09:30:00Z", true},        // UTC designator
		{"2030-12-25T09:30:00z", true},        // lowercase z
		{"2030-12-25T09:30:00-07:00", true},   // negative offset
		{"2030-12-25T09:30:00+05:30", true},   // positive offset
		{"2030-12-25 09:30:00", true},         // space separator
	}
	for _, c := range cases {
		got := overrideTimestampWarning(c.ts)
		if c.warn && got == "" {
			t.Errorf("%q should warn", c.ts)
		}
		if !c.warn && got != "" {
			t.Errorf("%q should not warn, got %q", c.ts, got)
		}
	}
}

func TestSendTestMessageWarnsButStillSends(t *testing.T) {
	var got map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := agentsSendTestMessageCmd()
	var stderr bytes.Buffer
	cmd.SetErr(&stderr)
	cmd.SetArgs([]string{"42", "--test-id", "t1", "--override-timestamp", "2030-12-25T09:30:00Z"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(stderr.String(), "warning: --override-timestamp") {
		t.Errorf("expected stderr warning, got %q", stderr.String())
	}
	// Warn, don't block: the value is still sent unchanged.
	if got["override_timestamp"] != "2030-12-25T09:30:00Z" {
		t.Errorf("value should still be sent unchanged, got %#v", got["override_timestamp"])
	}
}

// --- tools get ID fallback and channels get tests (#69, #70) ---

func TestToolsGetByNumericIDFallback(t *testing.T) {
	var paths []string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch {
		case r.URL.Path == "/api/v1/tools/425":
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(`{"detail":"Tool with name 425 not found"}`))
		case r.URL.Path == "/api/v1/tools/" && r.URL.Query().Get("search_fields") == "id":
			w.Write([]byte(`{"items":[{"id":425,"name":"ability_info"}],"total_count":1}`))
		case r.URL.Path == "/api/v1/tools/ability_info":
			w.Write([]byte(`{"id":425,"name":"ability_info","service_name":"pokeapi-v2","service_id":1}`))
		default:
			t.Errorf("unexpected request: %s", r.URL.String())
			w.WriteHeader(http.StatusNotFound)
		}
	})
	defer server.Close()

	cmd := toolsGetCmd()
	cmd.SetArgs([]string{"425"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if len(paths) != 3 {
		t.Errorf("expected name-miss → id-resolve → name-get, got %v", paths)
	}
	if !strings.Contains(out, "ability_info") {
		t.Errorf("expected resolved tool in output, got %q", out)
	}
}

func TestToolsGetByNameSingleRequest(t *testing.T) {
	var count int
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Write([]byte(`{"id":425,"name":"ability_info","service_name":"pokeapi-v2","service_id":1}`))
	})
	defer server.Close()

	cmd := toolsGetCmd()
	cmd.SetArgs([]string{"ability_info"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if count != 1 {
		t.Errorf("by-name get should hit the API once, got %d", count)
	}
}

func TestToolsGetUnknownIDKeepsOriginal404(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/tools/" {
			w.Write([]byte(`{"items":[],"total_count":0}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"detail":"Tool with name 999 not found"}`))
	})
	defer server.Close()

	cmd := toolsGetCmd()
	cmd.SetArgs([]string{"999"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })

	var apiErr *client.APIError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != 404 {
		t.Errorf("expected the original 404 to surface, got %v", err)
	}
}

func TestChannelsGet(t *testing.T) {
	var query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.RawQuery
		// Substring-style result on purpose: ids 4 and 40 — exact match must win.
		w.Write([]byte(`{"items":[{"id":40,"name":"other","channel_service":"twilio","is_system_channel":false},{"id":4,"name":"main-voice","channel_service":"twilio","is_system_channel":true}],"total_count":2}`))
	})
	defer server.Close()

	cmd := channelsGetCmd()
	cmd.SetArgs([]string{"4"})
	out := captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if !strings.Contains(query, "search_fields=id") {
		t.Errorf("expected id search in query, got %q", query)
	}
	if !strings.Contains(out, "main-voice") || strings.Contains(out, "other") {
		t.Errorf("expected exactly the id-4 channel, got %q", out)
	}
}

func TestChannelsGetNotFound(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"items":[],"total_count":0}`))
	})
	defer server.Close()

	cmd := channelsGetCmd()
	cmd.SetArgs([]string{"77"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })

	if err == nil || !strings.Contains(err.Error(), "syllable channels list") {
		t.Errorf("expected not-found error pointing at channels list, got %v", err)
	}
}

// --- API-contract regression tests (#114, #115) ---

func TestUsersDeleteSendsJSONBody(t *testing.T) {
	var method, path string
	var body map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := usersDeleteCmd()
	cmd.SetArgs([]string{"alice@example.com"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != http.MethodDelete || path != "/api/v1/users/" {
		t.Errorf("expected DELETE /api/v1/users/, got %s %s", method, path)
	}
	if body["email"] != "alice@example.com" {
		t.Errorf("expected email in body, got %#v", body["email"])
	}
	if body["reason"] == nil || body["reason"] == "" {
		t.Errorf("expected a non-empty reason in body, got %#v", body["reason"])
	}
}

func TestChannelsUpdateRoutesToCollection(t *testing.T) {
	var method, path string
	var body map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	// PUT must hit the collection path, and the positional id must be injected
	// into the body since the API routes by it.
	cmd := channelsUpdateCmd()
	cmd.SetArgs([]string{"5", "--file", writeTempJSON(t, `{"name":"main","channel_service":"twilio","config":{}}`)})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != http.MethodPut || path != "/api/v1/channels/" {
		t.Errorf("expected PUT /api/v1/channels/, got %s %s", method, path)
	}
	if body["id"] != float64(5) {
		t.Errorf("expected id=5 injected into body, got %#v", body["id"])
	}
}

func TestChannelsTargetsDeleteUsesChannelQueryParam(t *testing.T) {
	var method, path, target string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		path = r.URL.Path
		target = r.URL.Query().Get("target_id")
		w.Write([]byte(``))
	})
	defer server.Close()

	cmd := channelsTargetsDeleteCmd()
	cmd.SetArgs([]string{"5", "42"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != http.MethodDelete || path != "/api/v1/channels/5" {
		t.Errorf("expected DELETE /api/v1/channels/5, got %s %s", method, path)
	}
	if target != "42" {
		t.Errorf("expected target_id=42 query param, got %q", target)
	}
}

func TestDeleteRequiresConfirmation(t *testing.T) {
	hit := false
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	// Force a non-interactive stdin so the gate is deterministic regardless of
	// whether `go test` runs from a terminal.
	origStdin := os.Stdin
	r, w, _ := os.Pipe()
	w.Close() // immediate EOF
	os.Stdin = r
	defer func() { os.Stdin = origStdin }()

	prev := assumeYes
	defer func() { assumeYes = prev }()

	// Without --yes and no TTY: refuse, and do not touch the server.
	assumeYes = false
	cmd := agentsDeleteCmd()
	cmd.SetArgs([]string{"42"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	captureStdout(func() { err = cmd.Execute() })
	if err == nil || !strings.Contains(err.Error(), "without confirmation") {
		t.Errorf("expected a refusal error, got %v", err)
	}
	if hit {
		t.Error("delete must not issue a request without confirmation")
	}

	// With --yes: proceed and issue the request.
	assumeYes = true
	cmd = agentsDeleteCmd()
	cmd.SetArgs([]string{"42"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error with --yes: %v", err)
		}
	})
	if !hit {
		t.Error("delete with --yes should issue the request")
	}
}

func TestPathAndQueryEscaping(t *testing.T) {
	var escapedPath, dashName string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		escapedPath = r.URL.EscapedPath()
		dashName = r.URL.Query().Get("dashboard_name")
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	// A slash in a path segment must be percent-encoded, not split into segments
	// (which would route to a different endpoint) (#126).
	cmd := agentsGetCmd()
	cmd.SetArgs([]string{"x/y"})
	captureStdout(func() { _ = cmd.Execute() })
	if escapedPath != "/api/v1/agents/x%2Fy" {
		t.Errorf("path segment not escaped: %q", escapedPath)
	}

	// A query value with a space and & must round-trip intact, not inject params.
	cmd = dashboardsFetchInfoCmd()
	cmd.SetArgs([]string{"--name", "Quarterly Report & KPIs"})
	captureStdout(func() { _ = cmd.Execute() })
	if dashName != "Quarterly Report & KPIs" {
		t.Errorf("dashboard_name not round-tripped: %q", dashName)
	}
}

func TestSetupMergeKeysRefusesUnresolvedPlaceholder(t *testing.T) {
	stored := &setupConfig{Orgs: map[string]setupOrg{
		"acme": {APIKey: "real-key", Envs: map[string]setupOrgEnv{}},
	}}

	// Unchanged org: the placeholder resolves to the stored key.
	in := &setupConfig{Orgs: map[string]setupOrg{
		"acme": {APIKey: setupKeyPlaceholder, Envs: map[string]setupOrgEnv{}},
	}}
	merged, err := setupMergeKeys(in, stored)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if merged.Orgs["acme"].APIKey != "real-key" {
		t.Errorf("placeholder not resolved: %q", merged.Orgs["acme"].APIKey)
	}

	// Renamed org: a placeholder under a name with no stored key must be refused,
	// not written as an empty key (#130).
	renamed := &setupConfig{Orgs: map[string]setupOrg{
		"acme-renamed": {APIKey: setupKeyPlaceholder, Envs: map[string]setupOrgEnv{}},
	}}
	if _, err := setupMergeKeys(renamed, stored); err == nil {
		t.Error("expected a refusal when a renamed org's key can't be resolved")
	}
}

func TestSetupGuardRequest(t *testing.T) {
	const host = "127.0.0.1:54321"
	const token = "sekret-token"

	newReq := func(h, origin, tok string) *http.Request {
		r, _ := http.NewRequest(http.MethodPost, "http://"+h+"/save", nil)
		r.Host = h
		if origin != "" {
			r.Header.Set("Origin", origin)
		}
		if tok != "" {
			r.Header.Set("X-Syllable-CSRF", tok)
		}
		return r
	}

	// Valid: correct host, matching origin, correct token.
	if err := setupGuardRequest(newReq(host, "http://"+host, token), token, host); err != nil {
		t.Errorf("valid request rejected: %v", err)
	}
	// Valid: no Origin header (token + Host still required and present).
	if err := setupGuardRequest(newReq(host, "", token), token, host); err != nil {
		t.Errorf("valid no-origin request rejected: %v", err)
	}
	// Rejected: mismatched Host (DNS rebinding).
	if err := setupGuardRequest(newReq("evil.example.com:54321", "", token), token, host); err == nil {
		t.Error("expected rejection on mismatched Host")
	}
	// Rejected: cross-origin.
	if err := setupGuardRequest(newReq(host, "http://evil.example.com", token), token, host); err == nil {
		t.Error("expected rejection on cross-origin Origin")
	}
	// Rejected: missing token.
	if err := setupGuardRequest(newReq(host, "http://"+host, ""), token, host); err == nil {
		t.Error("expected rejection on missing CSRF token")
	}
	// Rejected: wrong token.
	if err := setupGuardRequest(newReq(host, "http://"+host, "wrong"), token, host); err == nil {
		t.Error("expected rejection on wrong CSRF token")
	}
}

func TestSetupRandomToken(t *testing.T) {
	a, err := setupRandomToken()
	if err != nil || len(a) != 64 {
		t.Fatalf("setupRandomToken() = %q, err=%v; want 64 hex chars", a, err)
	}
	if b, _ := setupRandomToken(); a == b {
		t.Error("tokens should be unique per call")
	}
}

func TestSetupPageInjectsToken(t *testing.T) {
	if !strings.Contains(setupHTMLPage, "%%CSRF_TOKEN%%") {
		t.Fatal("page template is missing the %%CSRF_TOKEN%% placeholder")
	}
	page := strings.ReplaceAll(setupHTMLPage, "%%CSRF_TOKEN%%", "TOK123")
	if strings.Contains(page, "%%CSRF_TOKEN%%") {
		t.Error("token placeholder was not replaced")
	}
	if !strings.Contains(page, "TOK123") {
		t.Error("token not present in the served page")
	}
}

func TestWarnIfInsecureBaseURL(t *testing.T) {
	cases := []struct {
		url  string
		warn bool
	}{
		{"https://api.syllable.cloud", false},
		{"http://localhost:8080", false},
		{"http://127.0.0.1:8080", false},
		{"http://api.internal.example.com", true},
	}
	for _, tc := range cases {
		out := captureStderr(func() { warnIfInsecureBaseURL(tc.url) })
		if got := strings.Contains(out, "plaintext"); got != tc.warn {
			t.Errorf("warnIfInsecureBaseURL(%q): warned=%v, want %v", tc.url, got, tc.warn)
		}
	}
}

func TestHint422NamesFields(t *testing.T) {
	body := []byte(`{"detail":[{"loc":["body","prompt_id"],"msg":"field required"},{"loc":["body","name"],"msg":"x"}]}`)
	h := hint422(body)
	if !strings.Contains(h, "prompt_id") || !strings.Contains(h, "name") {
		t.Errorf("hint422 should name the failing fields, got %q", h)
	}
}

func TestHintForErrorStatusCodes(t *testing.T) {
	cases := map[int]string{
		401: "API key",
		403: "permission",
		500: "Server error",
	}
	for code, want := range cases {
		err := &client.APIError{StatusCode: code, Body: []byte("{}")}
		if h := hintForError(nil, err); !strings.Contains(h, want) {
			t.Errorf("hintForError(%d) = %q, want substring %q", code, h, want)
		}
	}
}

func TestResolveBaseURL(t *testing.T) {
	saveEnv, saveDef := viper.GetString("env"), viper.GetString("default_env")
	defer func() {
		viper.Set("env", saveEnv)
		viper.Set("default_env", saveDef)
		viper.Set("environments", nil)
	}()
	viper.Set("default_env", "")

	viper.Set("env", "")
	if u, err := resolveBaseURL(); err != nil || u != "https://api.syllable.cloud" {
		t.Errorf("default base URL = %q, %v", u, err)
	}
	viper.Set("env", "prod")
	if u, err := resolveBaseURL(); err != nil || u != "https://api.syllable.cloud" {
		t.Errorf("prod builtin base URL = %q, %v", u, err)
	}
	viper.Set("environments", map[string]interface{}{"dev": map[string]interface{}{"base_url": "http://localhost:9999"}})
	viper.Set("env", "dev")
	if u, err := resolveBaseURL(); err != nil || u != "http://localhost:9999" {
		t.Errorf("config-defined env base URL = %q, %v", u, err)
	}
	viper.Set("env", "ghost")
	viper.Set("environments", nil)
	if _, err := resolveBaseURL(); err == nil {
		t.Error("expected error for an unknown env")
	}
}

func TestValidateOutputFmt(t *testing.T) {
	prev := viper.GetString("output")
	defer viper.Set("output", prev)

	for _, ok := range []string{"table", "json", ""} {
		viper.Set("output", ok)
		if err := validateOutputFmt(); err != nil {
			t.Errorf("validateOutputFmt(%q) = %v, want nil", ok, err)
		}
	}
	viper.Set("output", "yaml")
	if err := validateOutputFmt(); err == nil {
		t.Error(`validateOutputFmt("yaml") = nil, want error`)
	}
}

// --- Behavioral tests for previously untested command families (#136) ---

// assertBodyIDUpdate exercises a collection-style PUT that routes by the body's
// id: the positional id must be injected, and a conflicting body id refused.
func assertBodyIDUpdate(t *testing.T, newCmd func() *cobra.Command, wantPath string) {
	t.Helper()
	var got map[string]interface{}
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		json.NewDecoder(r.Body).Decode(&got)
		w.Write([]byte(`{}`))
	})
	defer server.Close()

	cmd := newCmd()
	cmd.SetArgs([]string{"7", "--file", writeTempJSON(t, `{"name":"x"}`)})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if method != http.MethodPut || path != wantPath {
		t.Errorf("expected PUT %s, got %s %s", wantPath, method, path)
	}
	if got["id"] != float64(7) {
		t.Errorf("expected id=7 injected into body, got %#v", got["id"])
	}

	cmd = newCmd()
	cmd.SetArgs([]string{"7", "--file", writeTempJSON(t, `{"id":9}`)})
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	var err error
	captureStdout(func() { err = cmd.Execute() })
	if err == nil || !strings.Contains(err.Error(), "conflict") {
		t.Errorf("expected a conflict error on mismatched body id, got %v", err)
	}
}

func TestPromptsUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, promptsUpdateCmd, "/api/v1/prompts/")
}

func TestCustomMessagesUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, customMessagesUpdateCmd, "/api/v1/custom_messages/")
}

func TestLanguageGroupsUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, languageGroupsUpdateCmd, "/api/v1/language_groups/")
}

// assertListPath checks a list command issues a GET to the expected path prefix.
func assertListPath(t *testing.T, newCmd func() *cobra.Command, wantPrefix string) {
	t.Helper()
	var method, path string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.Write([]byte(`{"items":[],"total_count":0}`))
	})
	defer server.Close()
	cmd := newCmd()
	cmd.SetArgs(nil)
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
	if method != http.MethodGet || path != wantPrefix {
		t.Errorf("expected GET %s, got %s %s", wantPrefix, method, path)
	}
}

func TestConversationsList(t *testing.T) {
	assertListPath(t, conversationsListCmd, "/api/v1/conversations/")
}
func TestEventsList(t *testing.T)      { assertListPath(t, eventsListCmd, "/api/v1/events/") }
func TestPermissionsList(t *testing.T) { assertListPath(t, permissionsListCmd, "/api/v1/permissions/") }

// --- Update-command contract consistency (#121) ---
// These five previously took no positional (id had to live in --file); they now
// accept an optional positional id reconciled via ensureBodyIdentifier.

func TestDataSourcesUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, dataSourcesUpdateCmd, "/api/v1/data_sources/")
}
func TestServicesUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, servicesUpdateCmd, "/api/v1/services/")
}
func TestVoiceGroupsUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, voiceGroupsUpdateCmd, "/api/v1/voice_groups/")
}
func TestRolesUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, rolesUpdateCmd, "/api/v1/roles/")
}
func TestIncidentsUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, incidentsUpdateCmd, "/api/v1/incidents/")
}

// --- Spec-based --dry-run preflight (#143) ---

func TestSpecPreflight(t *testing.T) {
	has := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}

	// AgentCreate requires name/type/prompt_id/timezone/variables/tool_headers.
	missing := specPreflight("POST", "/api/v1/agents/", []byte(`{"name":"x"}`))
	for _, want := range []string{"type", "prompt_id", "timezone"} {
		if !has(missing, want) {
			t.Errorf("expected %q reported missing, got %v", want, missing)
		}
	}
	if has(missing, "name") {
		t.Errorf("name was provided and must not be reported missing: %v", missing)
	}

	// A complete body reports nothing.
	if got := specPreflight("POST", "/api/v1/agents/",
		[]byte(`{"name":"x","type":"ca_v1","prompt_id":1,"timezone":"UTC","variables":{},"tool_headers":{}}`)); len(got) != 0 {
		t.Errorf("expected no missing fields, got %v", got)
	}

	// A query string is stripped before lookup.
	if got := specPreflight("POST", "/api/v1/agents/?x=1", []byte(`{}`)); len(got) == 0 {
		t.Error("expected missing fields even with a query string present")
	}

	// Unknown path / non-object body → no findings (best-effort).
	if got := specPreflight("POST", "/api/v1/nonexistent/", []byte(`{}`)); got != nil {
		t.Errorf("unknown path should yield nil, got %v", got)
	}
	if got := specPreflight("POST", "/api/v1/agents/", []byte(`["not","an","object"]`)); got != nil {
		t.Errorf("non-object body should yield nil, got %v", got)
	}
}

// --- Bridge phrases (#167) ---

func TestBridgePhrasesCommandHasSubcommands(t *testing.T) {
	cmd := bridgePhrasesCmd()
	expected := []string{"list", "get", "create", "update", "delete"}
	subs := make(map[string]bool)
	for _, c := range cmd.Commands() {
		subs[c.Name()] = true
	}
	for _, exp := range expected {
		if !subs[exp] {
			t.Errorf("bridge-phrases command missing subcommand %q", exp)
		}
	}
}

func TestBridgePhrasesList(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items": []map[string]interface{}{
				{
					"id":          1,
					"name":        "Inbound Hold",
					"description": "Standard hold phrases",
					"is_default":  true,
					"config": map[string]interface{}{
						"phrases": map[string]interface{}{
							// Three messages: no other cell in this fixture renders a "3",
							// so the count assertion below can actually fail.
							"messages": []string{"One moment, please.", "Let me check on that.", "Almost there."},
						},
					},
					"updated_at":      "2026-07-02T00:00:00Z",
					"last_updated_by": "user@mail.com",
				},
			},
			"total_count": 1,
		})
	})
	defer server.Close()

	viper.Set("output", "table")
	defer viper.Set("output", "json")

	cmd := bridgePhrasesListCmd()
	cmd.SetArgs([]string{})
	out := captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(out, "Inbound Hold") {
		t.Errorf("output should contain the config name, got: %s", out)
	}
	// The PHRASES column is a count of the default phrase set, not the phrases.
	if !strings.Contains(out, "3") {
		t.Errorf("output should contain the phrase count, got: %s", out)
	}
	if strings.Contains(out, "One moment") {
		t.Errorf("list should show a phrase count, not the phrases themselves, got: %s", out)
	}
}

func TestBridgePhrasesGetSummarizesNestedConfig(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":         7,
			"name":       "Inbound Hold",
			"is_default": false,
			"config": map[string]interface{}{
				"phrases": map[string]interface{}{
					"messages":  []string{"One moment, please."},
					"localized": map[string]interface{}{"es-US": map[string]interface{}{"messages": []string{"Un momento."}}},
				},
				"tools": []map[string]interface{}{
					{"tool_name": "lookup_order", "phrases": map[string]interface{}{"messages": []string{"Checking your order."}}},
				},
				"smart_turn_timeout_seconds": 1.5,
				"randomize_bridge_phrases":   true,
			},
			"agents_info":     []map[string]interface{}{{"id": 42, "name": "Support Agent"}},
			"updated_at":      "2026-07-02T00:00:00Z",
			"last_updated_by": "user@mail.com",
		})
	})
	defer server.Close()

	viper.Set("output", "table")
	defer viper.Set("output", "json")

	cmd := bridgePhrasesGetCmd()
	cmd.SetArgs([]string{"7"})
	out := captureStdout(func() {
		cmd.Execute()
	})

	for _, want := range []string{"Inbound Hold", "es-US", "lookup_order", "1.5", "Support Agent (42)"} {
		if !strings.Contains(out, want) {
			t.Errorf("get output should summarize %q, got: %s", want, out)
		}
	}
}

func TestBridgePhrasesUpdateInjectsPositionalID(t *testing.T) {
	assertBodyIDUpdate(t, bridgePhrasesUpdateCmd, "/api/v1/bridge_phrases/")
}

// The DELETE operation declares `reason` as a *required* query param, which the
// client appends automatically for paths that carry no query string of their own.
func TestBridgePhrasesDeleteSendsRequiredReason(t *testing.T) {
	var method, path, query string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		method, path, query = r.Method, r.URL.Path, r.URL.RawQuery
	})
	defer server.Close()

	cmd := bridgePhrasesDeleteCmd()
	cmd.SetArgs([]string{"7"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if method != http.MethodDelete || path != "/api/v1/bridge_phrases/7" {
		t.Errorf("expected DELETE /api/v1/bridge_phrases/7, got %s %s", method, path)
	}
	if !strings.Contains(query, "reason=") {
		t.Errorf("expected a reason query param, got %q", query)
	}
}

func TestBridgePhrasesCreateRequiresNameOrFile(t *testing.T) {
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		t.Error("no request should be sent when --name and --file are both absent")
	})
	defer server.Close()

	cmd := bridgePhrasesCreateCmd()
	cmd.SetArgs([]string{})
	var err error
	captureStdout(func() { err = cmd.Execute() })
	if err == nil || !strings.Contains(err.Error(), "required flags") {
		t.Errorf("expected a required-flags error, got %v", err)
	}
}

func TestBridgePhrasesCreateInlineBody(t *testing.T) {
	var body map[string]interface{}
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.Write([]byte(`{"id":1}`))
	})
	defer server.Close()

	cmd := bridgePhrasesCreateCmd()
	cmd.SetArgs([]string{"--name", "Inbound Hold", "--description", "Standard", "--default"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if body["name"] != "Inbound Hold" || body["description"] != "Standard" {
		t.Errorf("unexpected body: %#v", body)
	}
	if body["is_default"] != true {
		t.Errorf("--default should set is_default=true, got %#v", body["is_default"])
	}
	// config must be present with an empty phrase set the user can fill in.
	cfg, ok := body["config"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected a config object, got %#v", body["config"])
	}
	if _, ok := cfg["phrases"]; !ok {
		t.Errorf("expected config.phrases in inline body, got %#v", cfg)
	}
}
