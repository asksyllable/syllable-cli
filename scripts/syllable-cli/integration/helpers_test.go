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
	baseURL   string
	apiKey    string // direct key (no org lookup)
	org       string // org name (key looked up from config file)
)

// initConfig populates the test config from environment variables.
//
// Auth: prefers --org (reads key from ~/.syllable/config.yaml) over --api-key,
// so set SYLLABLE_ORG to pick the right org. SYLLABLE_API_KEY is used only
// when SYLLABLE_ORG is not set.
//
// Base URL: defaults to https://api.syllable.cloud. Override with SYLLABLE_TEST_BASE_URL
// (NOT SYLLABLE_BASE_URL, to avoid accidentally picking up shell env pointing to staging).
func initConfig() {
	cliBinary = envOr("SYLLABLE_CLI_BINARY", "../syllable")
	baseURL = envOr("SYLLABLE_TEST_BASE_URL", "https://api.syllable.cloud")
	apiKey = os.Getenv("SYLLABLE_API_KEY")
	org = os.Getenv("SYLLABLE_ORG")
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

// runCLI executes the CLI binary with the given args, always injecting
// --base-url and -o json. Auth is either --api-key (if SYLLABLE_API_KEY is
// set) or --org (if SYLLABLE_ORG is set, reads key from ~/.syllable/config.yaml).
func runCLI(args ...string) ([]byte, error) {
	base := []string{"--base-url", baseURL, "-o", "json"}
	if apiKey != "" {
		base = append(base, "--api-key", apiKey)
	} else {
		base = append(base, "--org", org)
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
