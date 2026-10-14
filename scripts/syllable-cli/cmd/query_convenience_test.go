package cmd

import (
	"encoding/json"
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/asksyllable/syllable-cli/internal/client"
)

func TestPluralSessionFiltersRetainIncludeTest(t *testing.T) {
	old := apiClient
	t.Cleanup(func() { apiClient = old })
	apiClient = client.New("https://api.example.invalid", "unit-test")
	apiClient.DryRun = true
	cmd := sessionsCmd()
	if err := bindSDKQueryParameters(cmd); err != nil {
		t.Fatal(err)
	}
	leaf, rest, err := cmd.Find([]string{"list", "--include-test", "--search-fields", "agent_name", "--search-values", "A & B"})
	if err != nil {
		t.Fatal(err)
	}
	if err = leaf.ParseFlags(rest); err != nil {
		t.Fatal(err)
	}
	var dry *client.DryRunResult
	if err = leaf.RunE(leaf, leaf.Flags().Args()); !errors.As(err, &dry) {
		t.Fatal(err)
	}
	var request struct {
		URL string `json:"url"`
	}
	if err = json.Unmarshal(dry.Output, &request); err != nil {
		t.Fatal(err)
	}
	u, err := url.Parse(request.URL)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(u.Query()["search_fields"], []string{"agent_name", "is_test"}) || !reflect.DeepEqual(u.Query()["search_field_values"], []string{"A & B", "true"}) {
		t.Fatalf("include-test lost: %s", u)
	}
}
