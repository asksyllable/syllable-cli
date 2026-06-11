//go:build integration

package integration_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Config (read from environment)
// ---------------------------------------------------------------------------

var (
	cliBinary string
	org       string // org name; key is resolved by the CLI from ~/.syllable/config.yaml
	env       string // optional named environment (selects base URL + per-env key)
)

// initConfig populates the test config from environment variables.
//
// Auth (either works):
//   - SYLLABLE_API_KEY — the CLI reads this key directly (non-interactive / CI).
//   - SYLLABLE_ORG     — the CLI resolves that org's key from ~/.syllable/config.yaml
//     (local dev; configure it with `syllable setup`).
//
// The CLI has no --api-key/--base-url flag. Set SYLLABLE_ENV for a non-default
// environment; base URL defaults to prod (https://api.syllable.cloud).
func initConfig() {
	cliBinary = envOr("SYLLABLE_CLI_BINARY", "../syllable")
	org = os.Getenv("SYLLABLE_ORG")
	env = os.Getenv("SYLLABLE_ENV")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// ---------------------------------------------------------------------------
// Cleanup stack (LIFO, always runs in TestMain)
// ---------------------------------------------------------------------------

var (
	cleanupMu    sync.Mutex
	cleanupStack []func()
)

func registerCleanup(fn func()) {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	cleanupStack = append(cleanupStack, fn)
}

func runCleanup() {
	cleanupMu.Lock()
	defer cleanupMu.Unlock()
	for i := len(cleanupStack) - 1; i >= 0; i-- {
		cleanupStack[i]()
	}
}

// ---------------------------------------------------------------------------
// CLI runner
// ---------------------------------------------------------------------------

// runCLI executes the CLI binary with the given args, injecting -o json (plus
// --org when SYLLABLE_ORG is set and --env when SYLLABLE_ENV is set). The CLI
// gets its key from SYLLABLE_API_KEY (inherited env) or that org's config entry.
func runCLI(args ...string) ([]byte, error) {
	base := []string{"-o", "json"}
	if org != "" {
		base = append(base, "--org", org)
	}
	if env != "" {
		base = append(base, "--env", env)
	}
	full := append(base, args...)
	cmd := exec.Command(cliBinary, full...)
	out, err := cmd.CombinedOutput()
	return out, err
}

// mustRunCLI calls runCLI and fails the test on error.
func mustRunCLI(t *testing.T, args ...string) []byte {
	t.Helper()
	out, err := runCLI(args...)
	if err != nil {
		t.Fatalf("CLI error running %v:\n%s", args, string(out))
	}
	return out
}

// ---------------------------------------------------------------------------
// JSON helpers
// ---------------------------------------------------------------------------

// extractField parses JSON and returns a string field at the top level.
func extractField(data []byte, field string) (string, error) {
	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return "", fmt.Errorf("unmarshal: %w (raw: %s)", err, string(data))
	}
	v, ok := m[field]
	if !ok {
		return "", fmt.Errorf("field %q not found in: %s", field, string(data))
	}
	switch val := v.(type) {
	case string:
		return val, nil
	case float64:
		return fmt.Sprintf("%v", int(val)), nil
	default:
		return fmt.Sprintf("%v", val), nil
	}
}

// mustExtractField calls extractField and fails the test on error.
func mustExtractField(t *testing.T, data []byte, field string) string {
	t.Helper()
	v, err := extractField(data, field)
	if err != nil {
		t.Fatalf("mustExtractField: %v", err)
	}
	return v
}

// ---------------------------------------------------------------------------
// Temp file helper
// ---------------------------------------------------------------------------

// writeTempJSON writes v as JSON to a temp file and returns its path.
// The caller is responsible for removing it (or use t.Cleanup).
func writeTempJSON(t *testing.T, v interface{}) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal JSON: %v", err)
	}
	f, err := os.CreateTemp("", "syllable-test-*.json")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := f.Write(data); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	f.Close()
	t.Cleanup(func() { os.Remove(f.Name()) })
	return f.Name()
}

// ---------------------------------------------------------------------------
// Misc
// ---------------------------------------------------------------------------

// testName returns a unique resource name with a test prefix.
func testName(suffix string) string {
	return "[TEST-INTEG] " + suffix
}

// assertContains fails the test if sub is not found in s.
func assertContains(t *testing.T, s, sub string) {
	t.Helper()
	if !strings.Contains(s, sub) {
		t.Fatalf("expected %q to contain %q", s, sub)
	}
}
