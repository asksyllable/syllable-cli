//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
)

func TestAgentSTTProviders(t *testing.T) {
	out := mustRunCLI(t, "agents", "stt-providers")
	var providers []map[string]interface{}
	if err := json.Unmarshal(out, &providers); err != nil {
		t.Fatalf("parse STT providers: %v", err)
	}
}

func TestExperimentsList(t *testing.T) {
	out := mustRunCLI(t, "experiments", "list", "--limit", "1")
	assertContains(t, string(out), "items")
}

func TestExperimentsCRUD(t *testing.T) {
	// These must be dedicated test resources: start/stop changes how the target
	// routes traffic. Never set these variables to customer target/agent IDs.
	ids := map[string]int{}
	for _, key := range []string{"SYLLABLE_EXPERIMENT_TARGET_ID", "SYLLABLE_EXPERIMENT_CONTROL_AGENT_ID", "SYLLABLE_EXPERIMENT_VARIANT_AGENT_ID"} {
		value := os.Getenv(key)
		if value == "" {
			t.Skipf("%s not set; skipping experiment lifecycle test", key)
		}
		id, err := strconv.Atoi(value)
		if err != nil || id <= 0 {
			t.Fatalf("%s must be a positive integer", key)
		}
		ids[key] = id
	}
	name := testName("Experiment")
	variants := []map[string]interface{}{
		{"name": "Control", "agent_id": ids["SYLLABLE_EXPERIMENT_CONTROL_AGENT_ID"], "weight": 50, "is_control": true},
		{"name": "Variant", "agent_id": ids["SYLLABLE_EXPERIMENT_VARIANT_AGENT_ID"], "weight": 50},
	}
	createFile := writeTempJSON(t, map[string]interface{}{"name": name, "target_id": ids["SYLLABLE_EXPERIMENT_TARGET_ID"], "variants": variants})
	out := mustRunCLI(t, "experiments", "create", "--file", createFile)
	id := mustExtractField(t, out, "id")
	registerDelete("experiments", "delete", id)
	// LIFO cleanup stops a running experiment before the registered deletion.
	registerCleanup(func() {
		out, err := runCLI("experiments", "get", id)
		if err != nil {
			return
		}
		status, err := extractField(out, "status")
		if err == nil && status == "running" {
			if out, err = runCLI("experiments", "stop", id); err != nil {
				fmt.Fprintf(os.Stderr, "cleanup stop experiment %s: %v\n%s\n", id, err, out)
			}
		}
	})
	assertContains(t, string(mustRunCLI(t, "experiments", "get", id)), name)
	assertContains(t, string(mustRunCLI(t, "experiments", "list", "--search", name)), name)
	updatedName := name + " Updated"
	updateFile := writeTempJSON(t, map[string]interface{}{"name": updatedName, "variants": variants})
	assertContains(t, string(mustRunCLI(t, "experiments", "update", id, "--file", updateFile)), updatedName)
	assertContains(t, string(mustRunCLI(t, "experiments", "results", id)), "session_count")
	assertContains(t, string(mustRunCLI(t, "experiments", "start", id)), "running")
	assertContains(t, string(mustRunCLI(t, "experiments", "stop", id)), "stopped")
	mustRunCLI(t, "experiments", "delete", id)
}

func TestInsightWorkflowExecutionViews(t *testing.T) {
	out := mustRunCLI(t, "insights", "workflows", "list", "--limit", "1")
	var listed struct {
		Items []struct {
			ID json.Number `json:"id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		t.Fatalf("parse workflow list: %v", err)
	}
	if len(listed.Items) == 0 {
		t.Skip("no workflows in this org")
	}
	id := listed.Items[0].ID.String()
	for _, view := range []string{"executions", "sessions", "files", "executions-summary"} {
		t.Run(view, func(t *testing.T) {
			out := mustRunCLI(t, "insights", "workflows", view, id)
			if !json.Valid(out) {
				t.Fatalf("invalid JSON: %s", out)
			}
			if view != "executions-summary" {
				assertContains(t, string(out), "items")
			} else {
				assertContains(t, string(out), "total")
			}
		})
	}
}

func TestAgentCardClick(t *testing.T) {
	agentID := os.Getenv("SYLLABLE_TEST_AGENT_ID")
	if agentID == "" {
		t.Skip("SYLLABLE_TEST_AGENT_ID not set; skipping card-click test")
	}
	// Prod answers 404 for conversation-test sessions (#192), so this test can't
	// pass yet. Set SYLLABLE_RUN_KNOWN_BROKEN=1 to re-check it; drop this gate
	// once #192 is fixed.
	if os.Getenv("SYLLABLE_RUN_KNOWN_BROKEN") == "" {
		t.Skip("known server-side failure (#192); set SYLLABLE_RUN_KNOWN_BROKEN=1 to run")
	}
	testID := fmt.Sprintf("cli-card-click-%d", os.Getpid())
	mustRunCLI(t, "agents", "send-test-message", agentID, "--test-id", testID, "--session-start", "--text", "")
	out := mustRunCLI(t, "agents", "card-click", testID, "--action-id", "test-action", "--action-type", "cta")
	assertContains(t, string(out), "status")
}
