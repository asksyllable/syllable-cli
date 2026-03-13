package spec

import (
	"encoding/json"
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
