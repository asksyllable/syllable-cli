package cmd

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestOperationsDump(t *testing.T) {
	var out bytes.Buffer
	rootCmd.SetOut(&out)
	rootCmd.SetArgs([]string{"__operations"})
	t.Cleanup(func() {
		rootCmd.SetOut(nil)
		rootCmd.SetArgs(nil)
	})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("__operations: %v", err)
	}
	var entries map[string]operationEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("parse dump: %v\n%s", err, out.String())
	}
	manifest, err := parseGeneratedManifest(generatedCommandsJSON)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(sdkOperationCommands) + len(manifest.Commands); len(entries) != want {
		t.Fatalf("dump has %d operations, want %d hand-written plus generated", len(entries), want)
	}

	// A command that builds its body from inline flags lists them...
	agentCreate := entries["agent_create"]
	if agentCreate.Command != "agents create" || agentCreate.Method != "POST" || !contains(agentCreate.Flags, "name") {
		t.Errorf("agent_create = %+v, want agents create with inline --name", agentCreate)
	}
	// ...while a pass-through (--file only) command lists none, and spec-bound
	// query flags never count.
	if flags := entries["insights_tool_test"].Flags; len(flags) != 0 {
		t.Errorf("insights_tool_test flags = %v, want none (--file only)", flags)
	}
	for id, entry := range entries {
		for _, flag := range entry.Flags {
			if flag == "file" || flag == "api-fields" {
				t.Errorf("%s lists %q, which is not an inline body flag", id, flag)
			}
		}
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
