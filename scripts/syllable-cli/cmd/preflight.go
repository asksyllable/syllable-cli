package cmd

import (
	"encoding/json"
	"strings"

	apispec "github.com/asksyllable/syllable-cli/internal/spec"
)

// specPreflight returns the names of top-level required fields — per the
// embedded OpenAPI request schema for this path+method — that are absent from
// the JSON body. It is wired into the client only to annotate --dry-run output
// (#143), so it never blocks a real request: the spec and prod can legitimately
// differ, and the server's 422 plus hint422 already handle deep validation.
//
// Best-effort and nil-safe: an unknown path/method, a missing requestBody, or a
// non-object body yields no findings.
func specPreflight(method, path string, body []byte) []string {
	if i := strings.IndexByte(path, '?'); i >= 0 {
		path = path[:i]
	}

	var doc map[string]interface{}
	if json.Unmarshal(apispec.OpenAPI, &doc) != nil {
		return nil
	}
	paths, _ := doc["paths"].(map[string]interface{})
	pathItem, _ := paths[path].(map[string]interface{})
	op, _ := pathItem[strings.ToLower(method)].(map[string]interface{})
	rb, _ := op["requestBody"].(map[string]interface{})
	content, _ := rb["content"].(map[string]interface{})
	appJSON, _ := content["application/json"].(map[string]interface{})
	schema, _ := appJSON["schema"].(map[string]interface{})
	if schema == nil {
		return nil
	}
	// Resolve a $ref to its named component schema.
	if ref, ok := schema["$ref"].(string); ok {
		name := ref[strings.LastIndex(ref, "/")+1:]
		comps, _ := doc["components"].(map[string]interface{})
		schemas, _ := comps["schemas"].(map[string]interface{})
		schema, _ = schemas[name].(map[string]interface{})
	}
	required, _ := schema["required"].([]interface{})
	if len(required) == 0 {
		return nil
	}

	var parsed map[string]interface{}
	if json.Unmarshal(body, &parsed) != nil {
		return nil // not a JSON object — nothing to check
	}
	var missing []string
	for _, r := range required {
		key, _ := r.(string)
		if key == "" {
			continue
		}
		if v, present := parsed[key]; !present || v == nil {
			missing = append(missing, key)
		}
	}
	return missing
}
