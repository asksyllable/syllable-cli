//go:build integration

// Package integration_test runs black-box CRUD tests against a live Syllable
// instance by invoking the CLI binary as a subprocess.
//
// Auth — set one of:
//
//	SYLLABLE_API_KEY   — API key used directly (non-interactive / CI)
//	SYLLABLE_ORG       — org whose key is read from ~/.syllable/config.yaml (local dev)
//
// Optional env vars:
//
//	SYLLABLE_ENV              — named environment (default: prod)
//	SYLLABLE_CLI_BINARY       — path to CLI binary (default: ../syllable)
//	SYLLABLE_TOOL_SERVICE_ID  — integer service ID; enables TestToolsCRUD
//	SYLLABLE_TEST_CALLER_ID   — phone number; enables TestOutboundCampaignsCRUD
package integration_test

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"testing"
)

// ---------------------------------------------------------------------------
// TestMain — build binary, run tests, always clean up
// ---------------------------------------------------------------------------

func TestMain(m *testing.M) {
	initConfig()

	// Require an auth source: SYLLABLE_API_KEY (direct) or SYLLABLE_ORG (config).
	if org == "" && os.Getenv("SYLLABLE_API_KEY") == "" {
		fmt.Fprintln(os.Stderr, "Set SYLLABLE_API_KEY or SYLLABLE_ORG to run integration tests")
		os.Exit(0)
	}

	// Run cleanup even on Ctrl+C / kill, so an aborted run doesn't leak the
	// resources it created into the test org (#139).
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Fprintln(os.Stderr, "interrupted — cleaning up created test resources")
		runCleanup()
		os.Exit(1)
	}()

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

	registerDelete("language-groups", "delete", id)

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

	registerDelete("custom-messages", "delete", id)

	// Get
	out = mustRunCLI(t, "custom-messages", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "custom-messages", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":       id,
		"name":     updName,
		"text":     "Hello, this is an updated test greeting.",
		"type":     "greeting",
		"label":    "",
		"rules":    []interface{}{},
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

	registerDelete("prompts", "delete", id)

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
	registerDelete("custom-messages", "delete", greetingID)

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
	registerDelete("prompts", "delete", promptID)

	name := testName("Agent")
	createBody := map[string]interface{}{
		"name":                 name,
		"description":          "Integration test agent",
		"type":                 "ca_v1",
		"prompt_id":            promptID,
		"custom_message_id":    greetingID,
		"language_group_id":    nil,
		"timezone":             "America/New_York",
		"stt_provider":         "Deepgram Nova 3",
		"wait_sound":           "Keyboard 1",
		"agent_initiated":      false,
		"variables":            map[string]interface{}{},
		"tool_headers":         map[string]interface{}{},
		"prompt_tool_defaults": []interface{}{},
		"labels":               []interface{}{},
	}
	af := writeTempJSON(t, createBody)

	// Create
	out := mustRunCLI(t, "agents", "create", "--file", af)
	id := mustExtractField(t, out, "id")
	t.Logf("created agent id=%s", id)

	registerDelete("agents", "delete", id)

	// Get
	out = mustRunCLI(t, "agents", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "agents", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update
	updName := name + " Updated"
	updateBody := map[string]interface{}{
		"id":                   id,
		"name":                 updName,
		"description":          "Updated integration test agent",
		"type":                 "ca_v1",
		"prompt_id":            promptID,
		"custom_message_id":    greetingID,
		"language_group_id":    nil,
		"timezone":             "America/New_York",
		"stt_provider":         "Deepgram Nova 3",
		"wait_sound":           "Keyboard 1",
		"agent_initiated":      false,
		"variables":            map[string]interface{}{},
		"tool_headers":         map[string]interface{}{},
		"prompt_tool_defaults": []interface{}{},
		"labels":               []interface{}{},
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

	registerDelete("directory", "delete", id)

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

	registerDelete("tools", "delete", name)

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

	registerDelete("outbound", "campaigns", "delete", id)

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

// ---------------------------------------------------------------------------
// Agent Labels (read-only) — spec-sync v0.0.3
// ---------------------------------------------------------------------------

func TestAgentsLabels(t *testing.T) {
	// Read-only listing; the org may have zero labels. mustRunCLI fails the test
	// on a non-zero exit, so a clean call proves the command, path, and auth all
	// work against the live API.
	mustRunCLI(t, "agents", "labels")
}

// ---------------------------------------------------------------------------
// Organization SIP IP Ranges — spec-sync v0.0.3
// ---------------------------------------------------------------------------

func TestOrganizationsSipIPRangesCRUD(t *testing.T) {
	// TEST-NET-3 (RFC 5737) — reserved for documentation, safe to create/delete.
	cidr := "203.0.113.0/24"

	// Create
	out := mustRunCLI(t, "organizations", "sip-ip-ranges", "create", "--type", "signaling", "--ip-range", cidr)
	id := mustExtractField(t, out, "id")
	t.Logf("created sip-ip-range id=%s", id)
	registerDelete("organizations", "sip-ip-ranges", "delete", id)

	// List — should include the range we created
	out = mustRunCLI(t, "organizations", "sip-ip-ranges", "list")
	assertContains(t, string(out), cidr)

	// Update the CIDR, then confirm via list
	updated := "203.0.113.0/25"
	mustRunCLI(t, "organizations", "sip-ip-ranges", "update", id, "--ip-range", updated)
	out = mustRunCLI(t, "organizations", "sip-ip-ranges", "list")
	assertContains(t, string(out), updated)

	// Delete
	mustRunCLI(t, "organizations", "sip-ip-ranges", "delete", id)
}

// ---------------------------------------------------------------------------
// Tool History (gated on SYLLABLE_TEST_TOOL_ID — needs an existing tool)
// ---------------------------------------------------------------------------

func TestToolsHistory(t *testing.T) {
	toolID := os.Getenv("SYLLABLE_TEST_TOOL_ID")
	if toolID == "" {
		t.Skip("SYLLABLE_TEST_TOOL_ID not set; skipping tool history test")
	}
	out := mustRunCLI(t, "tools", "history", toolID)
	assertContains(t, string(out), "items")
}

// ---------------------------------------------------------------------------
// Twilio A2P compliance check (gated — needs a Twilio channel + number)
// ---------------------------------------------------------------------------

func TestChannelsTwilioVerifyA2p(t *testing.T) {
	channelID := os.Getenv("SYLLABLE_TEST_CHANNEL_ID")
	phone := os.Getenv("SYLLABLE_TEST_PHONE")
	if channelID == "" || phone == "" {
		t.Skip("SYLLABLE_TEST_CHANNEL_ID / SYLLABLE_TEST_PHONE not set; skipping A2P compliance test")
	}
	out := mustRunCLI(t, "channels", "twilio", "numbers-verify-a2p-compliance", channelID, "--phone", phone)
	assertContains(t, string(out), "a2p_approved")
}

// ---------------------------------------------------------------------------
// Bridge Phrases — spec-sync v0.0.3
// ---------------------------------------------------------------------------

func TestBridgePhrasesCRUD(t *testing.T) {
	name := testName("BridgePhrases")

	// Create. is_default is left false — at most one non-deleted config per
	// suborg may be the default, so claiming it would fight the org's real one.
	out := mustRunCLI(t, "bridge-phrases", "create", "--name", name, "--description", "integration test config")
	id := mustExtractField(t, out, "id")
	t.Logf("created bridge phrases config id=%s", id)

	registerDelete("bridge-phrases", "delete", id)

	// Get
	out = mustRunCLI(t, "bridge-phrases", "get", id)
	assertContains(t, string(out), name)

	// List
	out = mustRunCLI(t, "bridge-phrases", "list", "--search", "[TEST-INTEG]")
	assertContains(t, string(out), name)

	// Update — exercises the nested config payload the inline create can't send:
	// a default phrase set, a per-language override, and a per-tool override.
	updName := name + " Updated"
	// `id` is deliberately omitted so the positional argument is injected by
	// ensureBodyIdentifier as a JSON *integer*. mustExtractField returns a
	// string, and a body id that's already present is passed through untouched —
	// so spelling it here would send "id":"7" and pass only while the backend
	// coerces it (#116). Omitting it exercises both the correct wire type and the
	// path real users hit.
	updateBody := map[string]interface{}{
		"name":        updName,
		"description": "integration test config",
		"config": map[string]interface{}{
			"phrases": map[string]interface{}{
				"messages": []string{"One moment, please.", "Let me check on that."},
				"localized": map[string]interface{}{
					"es-US": map[string]interface{}{"messages": []string{"Un momento, por favor."}},
				},
			},
			"tools":                      []interface{}{},
			"smart_turn_timeout_seconds": 1.5,
			"randomize_bridge_phrases":   true,
		},
		"edit_comments": "updated by integration test",
	}
	f := writeTempJSON(t, updateBody)
	out = mustRunCLI(t, "bridge-phrases", "update", id, "--file", f)
	assertContains(t, string(out), updName)

	// Read back — proves the nested config round-trips rather than being dropped.
	out = mustRunCLI(t, "bridge-phrases", "get", id, "--output", "json")
	for _, want := range []string{"One moment, please.", "es-US", "Un momento, por favor."} {
		assertContains(t, string(out), want)
	}

	// Delete
	mustRunCLI(t, "bridge-phrases", "delete", id)
}
