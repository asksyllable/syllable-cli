package spec

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestOpenAPIEmbedded(t *testing.T) {
	if len(OpenAPI) == 0 {
		t.Fatal("OpenAPI spec is empty — embed may have failed")
	}
}

func TestOpenAPIValidJSON(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(OpenAPI, &parsed); err != nil {
		t.Fatalf("OpenAPI spec is not valid JSON: %v", err)
	}
}

func TestOpenAPIHasComponents(t *testing.T) {
	var parsed map[string]interface{}
	json.Unmarshal(OpenAPI, &parsed)

	components, ok := parsed["components"].(map[string]interface{})
	if !ok {
		t.Fatal("OpenAPI spec missing 'components' key")
	}

	schemas, ok := components["schemas"].(map[string]interface{})
	if !ok {
		t.Fatal("OpenAPI spec missing 'components.schemas' key")
	}

	if len(schemas) == 0 {
		t.Fatal("OpenAPI spec has no schemas")
	}
}

func TestOpenAPIHasPaths(t *testing.T) {
	var parsed map[string]interface{}
	json.Unmarshal(OpenAPI, &parsed)

	paths, ok := parsed["paths"].(map[string]interface{})
	if !ok {
		t.Fatal("OpenAPI spec missing 'paths' key")
	}

	if len(paths) == 0 {
		t.Fatal("OpenAPI spec has no paths")
	}
}

// TestBulkOperationPathsMatch guards the exact paths used by the bulk CLI
// commands against silent spec drift — slash-mismatch redirects drop the
// Syllable-API-Key header on some clients, so trailing slashes must match.
func TestBulkOperationPathsMatch(t *testing.T) {
	var parsed map[string]interface{}
	json.Unmarshal(OpenAPI, &parsed)
	paths := parsed["paths"].(map[string]interface{})

	want := []string{
		"/api/v1/directory_members/upload/",
		"/api/v1/directory_members/download/",
		"/api/v1/outbound/batches/{batch_id}/upload_batch",
		"/api/v1/pronunciations/csv",
		// SIP IP ranges: the collection endpoint has a trailing slash and the
		// item endpoint does not — a mismatch drops the API-key header on a 307.
		"/api/v1/organizations/sip_ip_ranges/",
		"/api/v1/organizations/sip_ip_ranges/{sip_ip_range_id}",
	}
	for _, p := range want {
		if _, ok := paths[p]; !ok {
			t.Errorf("bulk operation path %q missing from spec — CLI command will hit a slash-mismatch redirect", p)
		}
	}
}

// TestCLIPathsExistInSpec extracts every "/api/v1/..." string literal from the
// cmd/*.go sources and asserts each one's static prefix corresponds to a real
// endpoint in the embedded spec. This mechanically catches typo'd or removed
// paths (the class behind #114) in CI instead of via a live 404/405 (#117).
//
// It's a prefix check at a segment boundary: the dynamic tail of a path is built
// at runtime (concatenation or %s), so we verify the static resource prefix is
// real, not the full templated path.
func TestCLIPathsExistInSpec(t *testing.T) {
	var parsed map[string]interface{}
	if err := json.Unmarshal(OpenAPI, &parsed); err != nil {
		t.Fatalf("spec not valid JSON: %v", err)
	}
	pathsMap := parsed["paths"].(map[string]interface{})
	specPaths := make([]string, 0, len(pathsMap))
	for p := range pathsMap {
		specPaths = append(specPaths, p)
	}

	files, err := filepath.Glob("../../cmd/*.go")
	if err != nil || len(files) == 0 {
		t.Fatalf("could not list cmd sources: %v", err)
	}
	re := regexp.MustCompile(`"(/api/v1/[^"]*)"`)
	checked := 0
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range re.FindAllStringSubmatch(string(src), -1) {
			// Static prefix: up to the first format verb or query, trimmed to a segment.
			prefix := m[1]
			if i := strings.IndexAny(prefix, "%?"); i >= 0 {
				prefix = prefix[:i]
			}
			prefix = strings.TrimRight(prefix, "/")
			if prefix == "/api/v1" || prefix == "" {
				continue // too generic to be meaningful
			}
			ok := false
			for _, sp := range specPaths {
				if sp == prefix || strings.HasPrefix(sp, prefix+"/") {
					ok = true
					break
				}
			}
			if !ok {
				t.Errorf("%s: path %q has no matching endpoint in the embedded spec (static prefix %q)", filepath.Base(f), m[1], prefix)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatal("no /api/v1 path literals found in cmd/*.go — the extractor is broken")
	}
	t.Logf("checked %d CLI path literals against the spec", checked)
}
