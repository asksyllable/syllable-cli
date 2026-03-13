// Package integration_test runs black-box CRUD tests against a live Syllable
// instance by invoking the CLI binary as a subprocess.
//
// Required env vars:
//
//	SYLLABLE_API_KEY   — API key for the test org
//
// Optional env vars:
//
//	SYLLABLE_ORG              — org slug (default: sandbox)
//	SYLLABLE_BASE_URL         — base URL (default: https://api.syllable.cloud)
//	SYLLABLE_CLI_BINARY       — path to CLI binary (default: ../syllable)
//	SYLLABLE_TOOL_SERVICE_ID  — integer service ID; enables TestToolsCRUD
//	SYLLABLE_TEST_CALLER_ID   — phone number; enables TestOutboundCampaignsCRUD
package integration_test

import (
	"fmt"
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// TestMain — build binary, run tests, always clean up
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	initConfig()

	// Require at least one auth method.
	// Prefer SYLLABLE_ORG (looks up key from ~/.syllable/config.yaml).
	// Fall back to SYLLABLE_API_KEY for keyless-config environments.
	if apiKey == "" && org == "" {
		fmt.Fprintln(os.Stderr, "Set SYLLABLE_ORG or SYLLABLE_API_KEY to run integration tests")
		os.Exit(0)
	}

	code := m.Run()
	runCleanup()
	os.Exit(code)
}

// ---------------------------------------------------------------------------
// Language Groups
// ---------------------------------------------------------------------------

func TestLanguageGroupsCRUD(t *testing.T) {
	name := testName("LangGroup")

	// Create
	out := mustRunCLI(t, "language-groups", "create", "--name", name)
	id := mustExtractField(t, out, "id")
	t.Logf("created language group id=%s", id)

	registerCleanup(func() {
		runCLI("language-groups", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "language-groups", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "language-groups", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":                               id,
		"name":                             updName,
		"language_configs":                 []interface{}{},
		"skip_current_language_in_message": false,
	}
	f := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "language-groups", "update", id, "--file", f)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "language-groups", "delete", id)
}

// ---------------------------------------------------------------------------
// Custom Messages (Greetings)
// ---------------------------------------------------------------------------

func TestCustomMessagesCRUD(t *testing.T) {
	name := testName("Greeting")

	// Create
	out := mustRunCLI(t, "custom-messages", "create",
		"--name", name,
		"--text", "Hello, this is a test greeting.",
	)
	id := mustExtractField(t, out, "id")
	t.Logf("created custom message id=%s", id)

	registerCleanup(func() {
		runCLI("custom-messages", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "custom-messages", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "custom-messages", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":      id,
		"name":    updName,
		"text":    "Hello, this is an updated test greeting.",
		"type":    "greeting",
		"label":   "",
		"rules":   []interface{}{},
		"preamble": nil,
		"subject":  nil,
	}
	f := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "custom-messages", "update", id, "--file", f)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "custom-messages", "delete", id)
}

// ---------------------------------------------------------------------------
// Prompts
// ---------------------------------------------------------------------------

func TestPromptsCRUD(t *testing.T) {
	name := testName("Prompt")

	createBody := map[string]interface{}{
		"name":        name,
		"description": "Integration test prompt",
		"type":        "prompt_v1",
		"context":     "You are a test assistant.",
		"tools":       []interface{}{},
		"llm_config": map[string]interface{}{
			"provider": "openai",
			"model":    "gpt-4.1",
			"version":  "2025-04-14",
		},
		"session_end_enabled": false,
	}
	f := writeTempJSON(t, createBody)

	// Create
	out := mustRunCLI(t, "prompts", "create", "--file", f)
	id := mustExtractField(t, out, "id")
	t.Logf("created prompt id=%s", id)

	registerCleanup(func() {
		runCLI("prompts", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "prompts", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "prompts", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":          id,
		"name":        updName,
		"description": "Updated integration test prompt",
		"type":        "prompt_v1",
		"context":     "You are an updated test assistant.",
		"tools":       []interface{}{},
		"llm_config": map[string]interface{}{
			"provider": "openai",
			"model":    "gpt-4.1",
			"version":  "2025-04-14",
		},
		"session_end_enabled": false,
	}
	uf := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "prompts", "update", id, "--file", uf)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "prompts", "delete", id)
}

// ---------------------------------------------------------------------------
// Agents
// ---------------------------------------------------------------------------

func TestAgentsCRUD(t *testing.T) {
	// First create a prompt and greeting to attach to the agent
	greetingBody := map[string]interface{}{
		"name": testName("Agent Greeting"),
		"text": "Test agent greeting.",
		"type": "greeting",
	}
	gf := writeTempJSON(t, greetingBody)
	gOut := mustRunCLI(t, "custom-messages", "create", "--file", gf)
	greetingID := mustExtractField(t, gOut, "id")
	registerCleanup(func() { runCLI("custom-messages", "delete", greetingID) })

	promptBody := map[string]interface{}{
		"name":    testName("Agent Prompt"),
		"type":    "prompt_v1",
		"context": "You are a test agent assistant.",
		"tools":   []interface{}{},
		"llm_config": map[string]interface{}{
			"provider": "openai",
			"model":    "gpt-4.1",
			"version":  "2025-04-14",
		},
		"session_end_enabled": false,
	}
	pf := writeTempJSON(t, promptBody)
	pOut := mustRunCLI(t, "prompts", "create", "--file", pf)
	promptID := mustExtractField(t, pOut, "id")
	registerCleanup(func() { runCLI("prompts", "delete", promptID) })

	name := testName("Agent")
	createBody := map[string]interface{}{
		"name":              name,
		"description":       "Integration test agent",
		"type":              "ca_v1",
		"prompt_id":         promptID,
		"custom_message_id": greetingID,
		"language_group_id": nil,
		"timezone":          "America/New_York",
		"stt_provider":      "Deepgram Nova 3",
		"wait_sound":        "Keyboard 1",
		"agent_initiated":   false,
		"variables":         map[string]interface{}{},
		"tool_headers":      map[string]interface{}{},
		"prompt_tool_defaults": []interface{}{},
		"labels":            []interface{}{},
	}
	af := writeTempJSON(t, createBody)

	// Create
	out := mustRunCLI(t, "agents", "create", "--file", af)
	id := mustExtractField(t, out, "id")
	t.Logf("created agent id=%s", id)

	registerCleanup(func() {
		runCLI("agents", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "agents", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "agents", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":                id,
		"name":              updName,
		"description":       "Updated integration test agent",
		"type":              "ca_v1",
		"prompt_id":         promptID,
		"custom_message_id": greetingID,
		"language_group_id": nil,
		"timezone":          "America/New_York",
		"stt_provider":      "Deepgram Nova 3",
		"wait_sound":        "Keyboard 1",
		"agent_initiated":   false,
		"variables":         map[string]interface{}{},
		"tool_headers":      map[string]interface{}{},
		"prompt_tool_defaults": []interface{}{},
		"labels":            []interface{}{},
	}
	uf := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "agents", "update", id, "--file", uf)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "agents", "delete", id)
}

// ---------------------------------------------------------------------------
// Directory Members
// ---------------------------------------------------------------------------

func TestDirectoryCRUD(t *testing.T) {
	name := testName("DirMember")

	// Create
	out := mustRunCLI(t, "directory", "create",
		"--name", name,
		"--type", "individual",
	)
	id := mustExtractField(t, out, "id")
	t.Logf("created directory member id=%s", id)

	registerCleanup(func() {
		runCLI("directory", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "directory", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "directory", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update — directory update PUTs to /{id} but the API also requires id in body
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":       id,
		"name":     updName,
		"type":     "individual",
		"comments": "updated by integration test",
	}
	f := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "directory", "update", id, "--file", f)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "directory", "delete", id)
}

// ---------------------------------------------------------------------------
// Tools (gated on SYLLABLE_TOOL_SERVICE_ID)
// ---------------------------------------------------------------------------

func TestToolsCRUD(t *testing.T) {
	serviceID := os.Getenv("SYLLABLE_TOOL_SERVICE_ID")
	if serviceID == "" {
		t.Skip("SYLLABLE_TOOL_SERVICE_ID not set; skipping tool tests")
	}

	toolName := fmt.Sprintf("test-integ-tool-%d", os.Getpid())

	createBody := map[string]interface{}{
		"name":       toolName,
		"service_id": serviceID,
		"definition": map[string]interface{}{
			"type": "tool",
			"tool": map[string]interface{}{
				"name":        toolName,
				"description": "Integration test tool",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []interface{}{},
				},
			},
			"endpoint": map[string]interface{}{
				"url":    "https://httpbin.org/get",
				"method": "GET",
			},
			"static_parameters": []interface{}{},
		},
	}
	f := writeTempJSON(t, createBody)

	// Create
	out := mustRunCLI(t, "tools", "create", "--file", f)
	// tools use name as identifier, not numeric ID
	name := mustExtractField(t, out, "name")
	t.Logf("created tool name=%s", name)

	registerCleanup(func() {
		runCLI("tools", "delete", name)
	})

	// Get (uses name, not ID)
	out = mustRunCLI(t, "tools", "get", name)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "tools", "list", "--search", "test-integ-tool")
	assertContains(t, string(out), name)

	// Update
	updateBody := map[string]interface{}{
		"name":       name,
		"service_id": serviceID,
		"definition": map[string]interface{}{
			"type": "tool",
			"tool": map[string]interface{}{
				"name":        name,
				"description": "Updated integration test tool",
				"parameters": map[string]interface{}{
					"type":       "object",
					"properties": map[string]interface{}{},
					"required":   []interface{}{},
				},
			},
			"endpoint": map[string]interface{}{
				"url":    "https://httpbin.org/get",
				"method": "GET",
			},
			"static_parameters": []interface{}{},
		},
	}
	uf := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "tools", "update", name, "--file", uf)
	assertContains(t, string(out), name)

	// Delete
	mustRunCLI(t, "tools", "delete", name)
}

// ---------------------------------------------------------------------------
// Outbound Campaigns (gated on SYLLABLE_TEST_CALLER_ID)
// ---------------------------------------------------------------------------

func TestOutboundCampaignsCRUD(t *testing.T) {
	callerID := os.Getenv("SYLLABLE_TEST_CALLER_ID")
	if callerID == "" {
		t.Skip("SYLLABLE_TEST_CALLER_ID not set; skipping outbound campaign tests")
	}

	name := testName("Campaign")

	// Create
	out := mustRunCLI(t, "outbound", "campaigns", "create",
		"--name", name,
		"--caller-id", callerID,
	)
	id := mustExtractField(t, out, "id")
	t.Logf("created outbound campaign id=%s", id)

	registerCleanup(func() {
		runCLI("outbound", "campaigns", "delete", id)
	})

	// Get
	out = mustRunCLI(t, "outbound", "campaigns", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "outbound", "campaigns", "list")
	assertContains(t, string(out), name)

	// Update — outbound campaigns PUT to /{id}
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"campaign_name":      updName,
		"caller_id":          callerID,
		"description":        "Updated by integration test",
		"campaign_variables": map[string]interface{}{},
		"active_days":        []interface{}{},
	}
	uf := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "outbound", "campaigns", "update", id, "--file", uf)
	assertContains(t, string(out), updName)

	// Delete
	mustRunCLI(t, "outbound", "campaigns", "delete", id)
}
