package cmd

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/asksyllable/syllable-cli/internal/client"
)

// setupTestServer creates a test HTTP server and configures the global apiClient.
// Returns the server (caller must defer server.Close()) and a request log channel.
func setupTestServer(t *testing.T, handler http.HandlerFunc) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(handler)
	apiClient = client.New(server.URL, "test-key")
	viper.Set("output", "json")
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
	expected := []string{"list", "get", "transcript", "summary", "latency", "recording", "recording-stream"}
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
	expected := []string{"list", "create", "update", "delete", "targets", "available-targets", "twilio"}
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
	cmd.SetArgs([]string{"--name", "flag-agent", "--type", "inbound", "--prompt-id", "p1", "--timezone", "UTC"})
	captureStdout(func() {
		if err := cmd.Execute(); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	if receivedBody["name"] != "flag-agent" {
		t.Errorf("expected name=flag-agent, got %v", receivedBody["name"])
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
	cmd.SetArgs([]string{"--name", "my_tool", "--service-id", "svc-1"})
	captureStdout(func() {
		cmd.Execute()
	})

	if receivedBody["name"] != "my_tool" {
		t.Errorf("expected name=my_tool, got %v", receivedBody["name"])
	}
	if receivedBody["definition"] == nil {
		t.Error("expected empty definition map in body")
	}
}

// --- Sessions functional tests ---

func TestSessionsList(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.String()
		json.NewEncoder(w).Encode(map[string]interface{}{
			"items":       []interface{}{},
			"total_count": 0,
		})
	})
	defer server.Close()

	cmd := sessionsListCmd()
	cmd.SetArgs([]string{"--start-date", "2024-01-01T00:00:00Z", "--end-date", "2024-01-31T23:59:59Z"})
	captureStdout(func() {
		cmd.Execute()
	})

	if !strings.Contains(requestPath, "start_datetime=2024-01-01T00:00:00Z") {
		t.Errorf("expected start_datetime in path, got: %s", requestPath)
	}
	if !strings.Contains(requestPath, "end_datetime=2024-01-31T23:59:59Z") {
		t.Errorf("expected end_datetime in path, got: %s", requestPath)
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
				{"id": "1", "name": "FAQ Data", "description": "FAQ content", "last_updated": "2024-01-01"},
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
}

func TestDataSourcesGet(t *testing.T) {
	var requestPath string
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "5", "name": "FAQ Data", "content": "Some FAQ content",
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
