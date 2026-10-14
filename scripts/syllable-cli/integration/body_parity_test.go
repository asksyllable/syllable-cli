//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"testing"
)

// Requires a dedicated test agent; this opens a conversation-test session.
func TestAgentTestMessageFullBody(t *testing.T) {
	agentID := os.Getenv("SYLLABLE_TEST_AGENT_ID")
	if agentID == "" {
		t.Skip("SYLLABLE_TEST_AGENT_ID not set; full test-message body needs a dedicated test agent")
	}
	if id, err := strconv.Atoi(agentID); err != nil || id <= 0 {
		t.Fatal("SYLLABLE_TEST_AGENT_ID must be a positive integer")
	}
	body := map[string]interface{}{
		"agent_id":           agentID,
		"test_id":            fmt.Sprintf("cli-full-body-%d", os.Getpid()),
		"service_name":       "test",
		"source":             "cli-body-parity-test",
		"session_start":      true,
		"text":               "",
		"org_name":           nil,
		"override_timestamp": nil,
	}
	out := mustRunCLI(t, "agents", "send-test-message", agentID, "--file", writeTempJSON(t, body))
	if !json.Valid(out) {
		t.Fatalf("full-body test-message response is not JSON: %s", out)
	}
}

// The fixture must be an inactive, dedicated workflow with no backfill. This
// test activates it briefly using a complete body, then inactivates it.
func TestWorkflowActivationFullBody(t *testing.T) {
	id := os.Getenv("SYLLABLE_TEST_WORKFLOW_ACTIVATION_ID")
	if id == "" {
		t.Skip("SYLLABLE_TEST_WORKFLOW_ACTIVATION_ID not set; activation needs a dedicated inactive workflow")
	}
	out := mustRunCLI(t, "insights", "workflows", "get", id)
	var workflow struct {
		Status   string          `json:"status"`
		Estimate json.RawMessage `json:"estimate"`
	}
	if err := json.Unmarshal(out, &workflow); err != nil {
		t.Fatalf("parse workflow: %v", err)
	}
	if workflow.Status != "INACTIVE" {
		t.Fatalf("activation fixture must be INACTIVE, got %q", workflow.Status)
	}
	var estimate struct {
		BackfillCount int64 `json:"backfill_count"`
	}
	if err := json.Unmarshal(workflow.Estimate, &estimate); err != nil {
		t.Fatalf("parse workflow estimate: %v", err)
	}
	if estimate.BackfillCount != 0 {
		t.Fatalf("activation fixture must have no backfill, got %d calls", estimate.BackfillCount)
	}
	activation := writeTempJSON(t, map[string]interface{}{"is_acknowledged": true, "estimate": workflow.Estimate})
	t.Cleanup(func() {
		if out, err := runCLI("insights", "workflows", "inactivate", id); err != nil {
			t.Errorf("inactivate fixture %s: %v\n%s", id, err, out)
		}
	})
	out = mustRunCLI(t, "insights", "workflows", "activate", id, "--file", activation)
	assertContains(t, string(out), "ACTIVE")
	out = mustRunCLI(t, "insights", "workflows", "get", id)
	if err := json.Unmarshal(out, &workflow); err != nil {
		t.Fatal(err)
	}
	if workflow.Status != "ACTIVE" {
		t.Fatalf("expected activated fixture, got %q", workflow.Status)
	}
}

// Sample synthesis may incur usage; supplying this file opts into both sample
// endpoints. Use a saved config for an available test voice.
func TestVoiceSampleEndpoints(t *testing.T) {
	file := os.Getenv("SYLLABLE_VOICE_SAMPLE_BODY_FILE")
	if file == "" {
		t.Skip("SYLLABLE_VOICE_SAMPLE_BODY_FILE not set; sample endpoints need a valid voice config")
	}
	for _, resource := range []string{"voice-groups", "language-groups"} {
		t.Run(resource, func(t *testing.T) {
			out := mustRunCLI(t, resource, "sample", "--file", file)
			if len(out) == 0 || json.Valid(out) {
				t.Fatalf("expected binary sample response, got %d bytes", len(out))
			}
		})
	}
}
