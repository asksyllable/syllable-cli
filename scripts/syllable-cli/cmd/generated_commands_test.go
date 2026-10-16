package cmd

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
	"github.com/spf13/cobra"
)

var updateGenerated = flag.Bool("update", false, "rewrite generated_commands.json and the generated docs blocks (go generate ./cmd)")

// generatedDocs are the files holding a generated-commands block, relative to
// this package.
var generatedDocs = []string{
	"../../../AGENTS.md",
	"../../../.claude/skills/syllable-cli/references/commands.md",
}

func handWrittenRoot() *cobra.Command {
	root := &cobra.Command{Use: "syllable", SilenceUsage: true, SilenceErrors: true}
	addHandWrittenCommands(root)
	return root
}

func generatedCommandFor(root *cobra.Command, id string) *cobra.Command {
	var found *cobra.Command
	walkCommands(root, func(c *cobra.Command) {
		if c.Annotations[generatedAnnotation] == "true" && c.Annotations["syllable.sdk.operation"] == id {
			found = c
		}
	})
	return found
}

// TestGeneratedCommands is also the generator: `go generate ./cmd` runs it
// with -update to name new operations and rewrite the docs blocks. Without
// -update it fails while either is stale, or while an operation can't be
// generated and has no hand-written command.
func TestGeneratedCommands(t *testing.T) {
	doc, err := embeddedSpecDocument()
	if err != nil {
		t.Fatal(err)
	}
	current, err := os.ReadFile("generated_commands.json")
	if err != nil {
		t.Fatal(err)
	}
	previous, err := parseGeneratedManifest(current)
	if err != nil {
		t.Fatal(err)
	}
	root := handWrittenRoot()
	entries, problems := planGeneratedCommands(root, doc, previous.Commands)
	for _, p := range problems {
		t.Errorf("needs a hand-written command (sdk_operations.go) or a console-only exclusion: %s", p)
	}
	write := func(path string, old, want []byte) {
		if string(old) == string(want) {
			return
		}
		if !*updateGenerated {
			t.Errorf("%s is stale; run `go generate ./cmd` in scripts/syllable-cli", path)
			return
		}
		if err := os.WriteFile(path, want, 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("updated %s", path)
	}
	write("generated_commands.json", current, renderGeneratedManifest(entries))
	block := generatedCommandsMarkdown(root, doc)
	for _, path := range generatedDocs {
		old, err := os.ReadFile(path)
		if errors.Is(err, os.ErrNotExist) {
			t.Logf("skipping %s: not in this checkout", path)
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		want, err := replaceGeneratedDocsBlock(string(old), block)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		write(path, old, []byte(want))
	}
}

// The real CLI registers exactly the manifest's commands.
func TestGeneratedCommandsRegistered(t *testing.T) {
	manifest, err := parseGeneratedManifest(generatedCommandsJSON)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range manifest.Commands {
		c := generatedCommandFor(rootCmd, entry.OperationID)
		if c == nil {
			t.Errorf("generated command %q (%s) is not registered", entry.Command, entry.OperationID)
			continue
		}
		if got := strings.TrimPrefix(c.CommandPath(), "syllable "); got != entry.Command {
			t.Errorf("%s is registered as %q, want %q", entry.OperationID, got, entry.Command)
		}
	}
}

func TestGeneratedAcceptMatchesSDK(t *testing.T) {
	doc, err := embeddedSpecDocument()
	if err != nil {
		t.Fatal(err)
	}
	operations, err := loadSDKOperations()
	if err != nil {
		t.Fatal(err)
	}
	for _, op := range doc.Operations {
		if got, want := op.accept(), operations[op.OperationID].Accept; got != want {
			t.Errorf("%s: accept %q, want %q", op.OperationID, got, want)
		}
	}
}

// --- a made-up API to exercise generation ---

const generatedFixtureSpec = `{
  "openapi": "3.1.0",
  "tags": [{"name": "widgets", "description": "Operations related to widgets.           A widget is a test fixture."}],
  "components": {
    "securitySchemes": {"APIKeyHeader": {"type": "apiKey", "in": "header", "name": "Syllable-API-Key"}},
    "schemas": {
      "WidgetKind": {"type": "string", "enum": ["round", "square"], "description": "Shape of a widget."},
      "WidgetCreate": {"type": "object", "required": ["name", "kind", "parts"], "properties": {
        "name": {"type": "string", "description": "Display name. Shown in lists."},
        "kind": {"$ref": "#/components/schemas/WidgetKind"},
        "size": {"type": "integer", "minimum": 1},
        "notes": {"anyOf": [{"type": "string"}, {"type": "null"}], "description": "Free text \u001b[31mred\u001b[0m \u202eevil"},
        "parts": {"type": "array", "items": {"$ref": "#/components/schemas/WidgetPart"}},
        "made_at": {"type": "string", "format": "date-time"}
      }},
      "WidgetPart": {"type": "object", "required": ["label"], "properties": {"label": {"type": "string"}}},
      "Widget": {"type": "object", "properties": {"id": {"type": "integer"}, "name": {"type": "string"}}}
    }
  },
  "paths": {
    "/api/v1/widgets/": {
      "get": {"tags": ["widgets"], "operationId": "widget_list", "summary": "List widgets", "security": [{"APIKeyHeader": []}],
        "parameters": [
          {"name": "kind", "in": "query", "required": true, "schema": {"type": "string"}, "description": "Only this kind"},
          {"name": "page", "in": "query", "schema": {"type": "integer"}},
          {"name": "search_fields", "in": "query", "schema": {"type": "array", "items": {"type": "string"}}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "array", "items": {"$ref": "#/components/schemas/Widget"}}}}}}},
      "post": {"tags": ["widgets"], "operationId": "widget_create", "summary": "Create a widget | fast",
        "description": "Creates a widget.\n\n        Indented docstring line.", "security": [{"APIKeyHeader": []}],
        "requestBody": {"required": true, "content": {"application/json": {"schema": {"$ref": "#/components/schemas/WidgetCreate"}}}},
        "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}}}}
    },
    "/api/v1/widgets/{widget_id}": {
      "get": {"tags": ["widgets"], "operationId": "widget_get_by_id", "summary": "Get a widget", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "widget_id", "in": "path", "required": true, "schema": {"type": "integer"}, "description": "The widget's ID."}],
        "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}}}},
      "delete": {"tags": ["widgets"], "operationId": "widget_delete", "summary": "Delete a widget", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "widget_id", "in": "path", "required": true, "schema": {"type": "integer"}},
          {"name": "reason", "in": "query", "schema": {"type": "string"}}],
        "responses": {"204": {"description": "No content"}}}
    },
    "/api/v1/widgets/{widget_id}/archive": {
      "post": {"tags": ["widgets"], "operationId": "widgets_archive", "summary": "Archive a widget", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "widget_id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"$ref": "#/components/schemas/Widget"}}}}}}
    },
    "/api/v1/widgets/{widget_id}/parts": {
      "get": {"tags": ["widgets.parts"], "operationId": "widget_parts_list", "summary": "List a widget's parts", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "widget_id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "array", "items": {"$ref": "#/components/schemas/WidgetPart"}}}}}}}
    },
    "/api/v1/agents/{agent_id}/notes": {
      "put": {"tags": ["agents"], "operationId": "agent_notes_update", "summary": "Replace an agent's notes", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "agent_id", "in": "path", "required": true, "schema": {"type": "integer"}}],
        "requestBody": {"content": {"application/json": {"schema": {"type": "object", "properties": {"notes": {"type": "string"}}}}}},
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/agents/by-name/{name}": {
      "get": {"tags": ["agents"], "operationId": "agent_get", "summary": "Get an agent by name", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "name", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/sessions/timeline/{session_id}/export": {
      "get": {"tags": ["sessions.timeline"], "operationId": "session_timeline_export", "summary": "Export a timeline", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "session_id", "in": "path", "required": true, "schema": {"type": "string"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/widgets/upload": {
      "post": {"tags": ["widgets"], "operationId": "widget_upload", "summary": "Upload", "security": [{"APIKeyHeader": []}],
        "requestBody": {"content": {"multipart/form-data": {"schema": {"type": "object"}}}}, "responses": {"200": {}}}
    },
    "/api/v1/widgets/export": {
      "get": {"tags": ["widgets"], "operationId": "widget_export", "summary": "Export", "security": [{"APIKeyHeader": []}],
        "responses": {"200": {"content": {"text/csv": {"schema": {"type": "string"}}}}}}
    },
    "/api/v1/widgets/public": {
      "get": {"tags": ["widgets"], "operationId": "widget_public", "summary": "Public", "security": [],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/widgets/traced": {
      "get": {"tags": ["widgets"], "operationId": "widget_traced", "summary": "Traced", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "X-Trace", "in": "header", "schema": {"type": "string"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/widgets/collide": {
      "get": {"tags": ["widgets"], "operationId": "widget_collide", "summary": "Collide", "security": [{"APIKeyHeader": []}],
        "parameters": [{"name": "output", "in": "query", "schema": {"type": "string"}}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    },
    "/api/v1/widgets/status": {
      "get": {"tags": ["widgets"], "operationId": "status", "summary": "Reserved name", "security": [{"APIKeyHeader": []}],
        "responses": {"200": {"content": {"application/json": {"schema": {"type": "object"}}}}}}
    }
  }
}`

func fixtureSpec(t *testing.T, edit func(raw map[string]interface{})) *specDocument {
	t.Helper()
	data := []byte(generatedFixtureSpec)
	if edit != nil {
		var raw map[string]interface{}
		if err := json.Unmarshal(data, &raw); err != nil {
			t.Fatal(err)
		}
		edit(raw)
		data, _ = json.Marshal(raw)
	}
	doc, err := parseSpecDocument(data)
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

// fixtureRoot is the hand-written CLI plus the fixture's generated commands.
func fixtureRoot(t *testing.T) (*cobra.Command, []generatedEntry, []string) {
	t.Helper()
	root := handWrittenRoot()
	entries, problems := planGeneratedCommands(root, fixtureSpec(t, nil), nil)
	return root, entries, problems
}

func TestGeneratedCommandNames(t *testing.T) {
	_, entries, problems := fixtureRoot(t)
	got := map[string]string{}
	for _, e := range entries {
		got[e.Method+" "+e.Path] = e.Command
	}
	want := map[string]string{
		"GET /api/v1/widgets/":                              "widgets list",
		"POST /api/v1/widgets/":                             "widgets create",
		"GET /api/v1/widgets/{widget_id}":                   "widgets get",
		"DELETE /api/v1/widgets/{widget_id}":                "widgets delete",
		"POST /api/v1/widgets/{widget_id}/archive":          "widgets archive",
		"GET /api/v1/widgets/{widget_id}/parts":             "widgets parts list",
		"PUT /api/v1/agents/{agent_id}/notes":               "agents notes-update",
		"GET /api/v1/agents/by-name/{name}":                 "agents agent-get",         // `agents get` is hand-written
		"GET /api/v1/sessions/timeline/{session_id}/export": "sessions timeline-export", // `sessions timeline` is a leaf
	}
	for route, command := range want {
		if got[route] != command {
			t.Errorf("%s: named %q, want %q", route, got[route], command)
		}
	}
	if len(got) != len(want) {
		t.Errorf("generated %d commands, want %d: %v", len(got), len(want), got)
	}
	blocked := map[string]string{
		"widget_upload":  "takes a multipart/form-data request body",
		"widget_export":  "returns text/csv",
		"widget_public":  "does not authenticate",
		"widget_traced":  "takes a header parameter",
		"widget_collide": "collide with the --output flag",
		"(status)":       `"status" is reserved`,
	}
	for id, reason := range blocked {
		found := false
		for _, p := range problems {
			if strings.Contains(p, id) && strings.Contains(p, reason) {
				found = true
			}
		}
		if !found {
			t.Errorf("no problem reported for %s (%s) in %q", id, reason, problems)
		}
	}
	if len(problems) != len(blocked) {
		t.Errorf("got %d problems, want %d: %q", len(problems), len(blocked), problems)
	}
}

// A shipped name survives upstream retagging, and an operation moving to a
// hand-written command retires its generated one.
func TestGeneratedCommandNamesAreFrozen(t *testing.T) {
	_, entries, _ := fixtureRoot(t)
	retagged := fixtureSpec(t, func(raw map[string]interface{}) {
		op := raw["paths"].(map[string]interface{})["/api/v1/widgets/{widget_id}"].(map[string]interface{})["get"].(map[string]interface{})
		op["tags"] = []interface{}{"gadgets"}
		op["operationId"] = "gadget_fetch"
	})
	again, _ := planGeneratedCommands(handWrittenRoot(), retagged, entries)
	for _, e := range again {
		if e.Path == "/api/v1/widgets/{widget_id}" && e.Method == "GET" {
			if e.Command != "widgets get" || e.OperationID != "gadget_fetch" {
				t.Errorf("retagged operation became %+v", e)
			}
		}
	}

	sdkOperationCommands["widget_get_by_id"] = []string{"widgets", "get", "17"}
	defer delete(sdkOperationCommands, "widget_get_by_id")
	retired, _ := planGeneratedCommands(handWrittenRoot(), fixtureSpec(t, nil), entries)
	for _, e := range retired {
		if e.OperationID == "widget_get_by_id" {
			t.Errorf("hand-mapped operation kept its generated command %q", e.Command)
		}
	}
	if len(retired) != len(entries)-1 {
		t.Errorf("got %d entries after retiring one of %d", len(retired), len(entries))
	}
}

// runGenerated executes a command line against a fresh fixture CLI.
func runGenerated(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root, _, _ := fixtureRoot(t)
	root.SetArgs(args)
	stderr = captureStderr(func() {
		stdout = captureStdout(func() { _, err = root.ExecuteC() })
	})
	return stdout, stderr, err
}

type capturedRequest struct {
	Method, Path, Query, Body string
}

func generatedServer(t *testing.T, status int, response string) *[]capturedRequest {
	t.Helper()
	var requests []capturedRequest
	server := setupTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		requests = append(requests, capturedRequest{r.Method, r.URL.EscapedPath(), r.URL.RawQuery, string(body)})
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		io.WriteString(w, response)
	})
	t.Cleanup(server.Close)
	previous := assumeYes
	t.Cleanup(func() { assumeYes = previous })
	return &requests
}

func TestGeneratedCommandRequests(t *testing.T) {
	body := writeTempJSON(t, `{"name":"w","kind":"round","parts":[],"size":12345678901234567890}`)
	cases := []struct {
		name string
		args []string
		want capturedRequest
	}{
		{"get escapes the path", []string{"widgets", "get", "a/b"}, capturedRequest{"GET", "/api/v1/widgets/a%2Fb", "", ""}},
		{"list sends query flags", []string{"widgets", "list", "--kind", "round", "--page", "2", "--search-fields", "a,b"},
			capturedRequest{"GET", "/api/v1/widgets/", "kind=round&page=2&search_fields=a%2Cb", ""}},
		{"create passes the body through", []string{"widgets", "create", "--file", body},
			capturedRequest{"POST", "/api/v1/widgets/", "", `{"kind":"round","name":"w","parts":[],"size":12345678901234567890}`}},
		{"delete adds the default reason", []string{"widgets", "delete", "7"}, capturedRequest{"DELETE", "/api/v1/widgets/7", "reason=deleted+via+cli", ""}},
		{"delete reason is a flag", []string{"widgets", "delete", "7", "--reason", "cleanup"}, capturedRequest{"DELETE", "/api/v1/widgets/7", "reason=cleanup", ""}},
		{"body-less action sends no body", []string{"widgets", "archive", "7"}, capturedRequest{"POST", "/api/v1/widgets/7/archive", "", ""}},
		{"optional body may be omitted", []string{"agents", "notes-update", "3"}, capturedRequest{"PUT", "/api/v1/agents/3/notes", "", ""}},
		{"nested group", []string{"widgets", "parts", "list", "7"}, capturedRequest{"GET", "/api/v1/widgets/7/parts", "", ""}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			requests := generatedServer(t, 200, `{"id":7}`)
			stdout, _, err := runGenerated(t, tc.args...)
			if err != nil {
				t.Fatal(err)
			}
			if len(*requests) != 1 || (*requests)[0] != tc.want {
				t.Fatalf("requests %+v, want %+v", *requests, tc.want)
			}
			if !strings.Contains(stdout, `"id": 7`) {
				t.Errorf("stdout %q lacks the JSON response", stdout)
			}
		})
	}
}

func TestGeneratedCommandGuards(t *testing.T) {
	t.Run("required body", func(t *testing.T) {
		requests := generatedServer(t, 200, `{}`)
		_, _, err := runGenerated(t, "widgets", "create")
		if err == nil || !strings.Contains(err.Error(), "--file") || len(*requests) != 0 {
			t.Fatalf("err %v, requests %v", err, *requests)
		}
	})
	t.Run("required query flag", func(t *testing.T) {
		requests := generatedServer(t, 200, `{}`)
		_, _, err := runGenerated(t, "widgets", "list")
		if err == nil || !strings.Contains(err.Error(), `"kind"`) || len(*requests) != 0 {
			t.Fatalf("err %v, requests %v", err, *requests)
		}
	})
	t.Run("empty argument", func(t *testing.T) {
		requests := generatedServer(t, 200, `{}`)
		_, _, err := runGenerated(t, "widgets", "get", "")
		if err == nil || len(*requests) != 0 {
			t.Fatalf("err %v, requests %v", err, *requests)
		}
	})
	for _, args := range [][]string{{"widgets", "delete", "7"}, {"widgets", "archive", "7"}} {
		t.Run("confirms "+args[1], func(t *testing.T) {
			requests := generatedServer(t, 200, `{}`)
			assumeYes = false
			// A pipe, not a terminal: the command must refuse rather than prompt.
			r, w, _ := os.Pipe()
			w.Close()
			stdin := os.Stdin
			os.Stdin = r
			defer func() { os.Stdin = stdin }()
			_, _, err := runGenerated(t, args...)
			if err == nil || !strings.Contains(err.Error(), "--yes") || len(*requests) != 0 {
				t.Fatalf("err %v, requests %v", err, *requests)
			}
		})
	}
	t.Run("no content", func(t *testing.T) {
		generatedServer(t, 204, "")
		stdout, stderr, err := runGenerated(t, "widgets", "delete", "7")
		if err != nil || stdout != "" || !strings.Contains(stderr, "no content") {
			t.Fatalf("err %v, stdout %q, stderr %q", err, stdout, stderr)
		}
	})
}

func TestGeneratedCommandDryRun(t *testing.T) {
	body := writeTempJSON(t, `{"name":"w"}`)
	previous, previousDryRun, previousYes := apiClient, dryRun, assumeYes
	t.Cleanup(func() { apiClient, dryRun, assumeYes = previous, previousDryRun, previousYes })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun, dryRun, assumeYes = true, true, false
	// Dry runs never ask for confirmation.
	_, _, err := runGenerated(t, "widgets", "create", "--file", body)
	var result *client.DryRunResult
	if !errors.As(err, &result) {
		t.Fatalf("not a dry run: %v", err)
	}
	var request struct {
		Method string          `json:"method"`
		URL    string          `json:"url"`
		Body   json.RawMessage `json:"body"`
	}
	if err := json.Unmarshal(result.Output, &request); err != nil {
		t.Fatal(err)
	}
	if request.Method != "POST" || request.URL != "https://api.example.invalid/api/v1/widgets/" || string(request.Body) != `{"name":"w"}` {
		t.Errorf("dry run %s", result.Output)
	}
	if _, _, err := runGenerated(t, "widgets", "archive", "7"); !errors.As(err, &result) {
		t.Errorf("archive dry run: %v", err)
	}
}

func TestGeneratedCommandHelp(t *testing.T) {
	root, _, _ := fixtureRoot(t)
	find := func(args ...string) *cobra.Command {
		c, _, err := root.Find(args)
		if err != nil {
			t.Fatal(err)
		}
		return c
	}
	create := find("widgets", "create")
	for _, want := range []string{
		"Create a widget | fast\n\nCreates a widget.\n\nIndented docstring line.",
		"API: POST /api/v1/widgets/",
		"Request body (JSON via --file FILE, or --file - for stdin, required): WidgetCreate",
		"Required fields:",
		`kind     WidgetKind (one of: "round", "square") - Shape of a widget.`,
		"name     string - Display name.",
		"parts    array of WidgetPart",
		"Optional fields:",
		"made_at  string (date-time)",
		"notes    string | null - Free text [31mred[0m evil",
		`"kind": "round"`,
		`"label": "string"`,
		"Full schema: syllable schema get WidgetCreate",
		"Returns: Widget (JSON). Full schema: syllable schema get Widget",
		"and any missing required body fields",
	} {
		if !strings.Contains(create.Long, want) {
			t.Errorf("create help lacks %q:\n%s", want, create.Long)
		}
	}
	if strings.ContainsAny(create.Long, "\x1b\u202e") {
		t.Errorf("create help keeps unsafe characters: %q", create.Long)
	}
	if create.Short != "Create a widget | fast [generated]" {
		t.Errorf("short %q", create.Short)
	}
	get := find("widgets", "get")
	for _, want := range []string{"Arguments:\n  <widget-id>  integer - The widget's ID.", "Returns: Widget (JSON)."} {
		if !strings.Contains(get.Long, want) {
			t.Errorf("get help lacks %q:\n%s", want, get.Long)
		}
	}
	if !strings.Contains(find("widgets", "archive").Long, "pass --yes") || strings.Contains(get.Long, "pass --yes") {
		t.Error("only changes made without a body should mention --yes")
	}
	if usage := find("widgets", "list").Flags().Lookup("kind").Usage; usage != "Only this kind (required)" {
		t.Errorf("kind usage %q", usage)
	}
	if group := find("widgets"); group.Short != "Operations related to widgets." {
		t.Errorf("group short %q", group.Short)
	}
}

func TestGeneratedCommandsMarkdown(t *testing.T) {
	root, _, _ := fixtureRoot(t)
	md := generatedCommandsMarkdown(root, fixtureSpec(t, nil))
	for _, want := range []string{
		"| `syllable widgets create` | `POST /api/v1/widgets/` | Create a widget \\| fast |",
		"| `syllable widgets get <widget-id>` | `GET /api/v1/widgets/{widget_id}` | Get a widget |",
		"| `syllable agents agent-get <name>` | `GET /api/v1/agents/by-name/{name}` | Get an agent by name |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	empty := generatedCommandsMarkdown(handWrittenRoot(), fixtureSpec(t, nil))
	if !strings.Contains(empty, "_None right now") {
		t.Errorf("empty markdown: %s", empty)
	}
	text := "intro\n" + generatedDocsBegin + "\nold\n" + generatedDocsEnd + "\noutro\n"
	got, err := replaceGeneratedDocsBlock(text, "new\n")
	if err != nil || got != "intro\n"+generatedDocsBegin+"\nnew\n"+generatedDocsEnd+"\noutro\n" {
		t.Errorf("replaced block %q, %v", got, err)
	}
	if _, err := replaceGeneratedDocsBlock("no markers", "x"); err == nil {
		t.Error("missing markers should fail")
	}
}

func TestOperationsDumpIncludesGenerated(t *testing.T) {
	root, _, _ := fixtureRoot(t)
	entries, err := operationEntries(root)
	if err != nil {
		t.Fatal(err)
	}
	got := entries["widget_create"]
	if !got.Generated || got.Command != "widgets create" || got.Method != "POST" || got.Path != "/api/v1/widgets/" || len(got.Flags) != 0 {
		t.Errorf("dump entry %+v", got)
	}
	if entries["agent_list"].Generated {
		t.Error("hand-written command marked generated")
	}
}

func TestGeneratedDocsHaveMarkers(t *testing.T) {
	for _, path := range generatedDocs {
		data, err := os.ReadFile(filepath.Clean(path))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if _, err := replaceGeneratedDocsBlock(string(data), ""); err != nil {
			t.Errorf("%s: %v", path, err)
		}
	}
}
