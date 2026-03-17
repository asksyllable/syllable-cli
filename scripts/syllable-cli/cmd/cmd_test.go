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
	expected := []string{"list", "get", "create", "update", "delete"}
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
	expected := []string{"list", "get", "transcript", "summary", "latency", "recording"}
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
	expected := []string{"workflows", "folders", "tool-configs", "tool-definitions"}
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

	expectedFlags := []string{"config", "api-key", "base-url", "org", "output"}
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
	expected := []string{"list", "get", "create", "update", "delete"}
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
	expected := []string{"list", "get-csv", "metadata"}
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
