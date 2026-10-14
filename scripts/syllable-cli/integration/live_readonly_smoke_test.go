//go:build integration

package integration_test

import (
	"encoding/json"
	"reflect"
	"strconv"
	"testing"
)

// TestSDKParityLiveReadOnlySmoke exercises populated fixtures without creating
// sessions, synthesizing audio, sending email, or changing platform resources.
// Missing fixtures skip their subtest; they do not count as live verification.
func TestSDKParityLiveReadOnlySmoke(t *testing.T) {
	t.Run("agents_search_projection_pagination", func(t *testing.T) {
		type agent struct {
			ID   json.Number `json:"id"`
			Name string      `json:"name"`
		}
		page := liveSmokeJSON[liveSmokePage[agent]](t, "agents", "list", "--limit", "2", "--order-by", "id", "--order-by-direction", "asc")
		if len(page.Items) == 0 {
			t.Skip("no agent fixture")
		}
		for i := 1; i < len(page.Items); i++ {
			previous, previousErr := page.Items[i-1].ID.Int64()
			current, currentErr := page.Items[i].ID.Int64()
			if previousErr != nil || currentErr != nil || previous >= current {
				t.Fatal("agent results are not ordered by ascending numeric ID")
			}
		}
		fixture := page.Items[0]
		got := liveSmokeJSON[agent](t, "agents", "get", fixture.ID.String())
		if got != fixture {
			t.Fatal("agent get identity differs from list")
		}
		filtered := liveSmokeJSON[liveSmokePage[agent]](t, "agents", "list", "--search-fields", "id", "--search-values", fixture.ID.String(), "--search-fields", "name_exact", "--search-values", fixture.Name)
		if len(filtered.Items) != 1 || filtered.Items[0] != fixture {
			t.Fatal("paired ID/name search did not return exactly the fixture")
		}
		projected := liveSmokeJSON[liveSmokePage[agent]](t, "agents", "list", "--search-fields", "id", "--search-values", fixture.ID.String(), "--api-fields", "id,name")
		// The response model can retain other keys; requested identity fields
		// must survive projection with the same values.
		if len(projected.Items) != 1 || projected.Items[0] != fixture {
			t.Fatal("requested identity fields differ after projection")
		}
		t.Run("pagination", func(t *testing.T) {
			if len(page.Items) < 2 {
				t.Skip("two agents required for pagination")
			}
			for i := range page.Items {
				args := []string{"agents", "list", "--limit", "1", "--page", strconv.Itoa(i), "--order-by", "id", "--order-by-direction", "asc"}
				one := liveSmokeJSON[liveSmokePage[agent]](t, args...)
				if len(one.Items) != 1 || one.Items[0] != page.Items[i] {
					t.Fatalf("page %d does not match the corresponding sorted row", i)
				}
			}
		})
	})
	t.Run("prompt_get_history", func(t *testing.T) {
		type prompt struct {
			ID            json.Number `json:"id"`
			Name          string      `json:"name"`
			VersionNumber int         `json:"version_number"`
		}
		page := liveSmokeJSON[liveSmokePage[prompt]](t, "prompts", "list", "--limit", "1")
		if len(page.Items) == 0 {
			t.Skip("no prompt fixture")
		}
		fixture := page.Items[0]
		got := liveSmokeJSON[prompt](t, "prompts", "get", fixture.ID.String())
		if got != fixture {
			t.Fatal("prompt get identity/version differs from list")
		}
		history := liveSmokeJSON[[]struct {
			PromptID      json.Number `json:"prompt_id"`
			VersionNumber int         `json:"version_number"`
		}](t, "prompts", "history", fixture.ID.String())
		if len(history) == 0 {
			t.Skip("no prompt history fixture")
		}
		for _, version := range history {
			if version.PromptID != fixture.ID || version.VersionNumber <= 0 {
				t.Fatal("history returned a different prompt or invalid version")
			}
		}
	})
	t.Run("test_session_search_details", func(t *testing.T) {
		type session struct {
			SessionID string `json:"session_id"`
			IsTest    bool   `json:"is_test"`
		}
		page := liveSmokeJSON[liveSmokePage[session]](t, "sessions", "list", "--include-test", "--limit", "1")
		if len(page.Items) == 0 {
			t.Skip("no test-session fixture")
		}
		fixture := page.Items[0]
		if !fixture.IsTest {
			t.Skip("first included session is not a test fixture")
		}
		filtered := liveSmokeJSON[liveSmokePage[session]](t, "sessions", "list", "--include-test", "--search-fields", "session_id", "--search-values", fixture.SessionID)
		if len(filtered.Items) != 1 || filtered.Items[0] != fixture {
			t.Fatal("session-ID search combined with include-test did not return the fixture")
		}
		for _, action := range []string{"get", "transcript", "timeline"} {
			got := liveSmokeJSON[session](t, "sessions", action, fixture.SessionID)
			if got.SessionID != fixture.SessionID {
				t.Fatalf("%s returned a different session", action)
			}
		}
	})
	t.Run("directory_status_and_formats", func(t *testing.T) {
		type member struct {
			ID        json.Number `json:"id"`
			DeletedAt *string     `json:"deleted_at"`
		}
		args := []string{"directory", "list", "--status", "deleted", "--include-deleted=false", "--limit", "3", "--order-by", "id", "--order-by-direction", "asc"}
		normalized := liveSmokeJSON[liveSmokePage[member]](t, append(args, "--format", "normalized")...)
		if len(normalized.Items) == 0 {
			t.Skip("no deleted-directory fixture")
		}
		for _, item := range normalized.Items {
			if item.DeletedAt == nil {
				t.Fatal("deleted status returned an active member")
			}
		}
		raw := liveSmokeJSON[liveSmokePage[member]](t, append(args, "--format", "raw")...)
		if !reflect.DeepEqual(raw.Items, normalized.Items) {
			t.Fatal("raw and normalized formats returned different member identities/statuses")
		}
		active := liveSmokeJSON[liveSmokePage[member]](t, "directory", "list", "--status", "active", "--include-deleted", "--limit", "3")
		for _, item := range active.Items {
			if item.DeletedAt != nil {
				t.Fatal("explicit active status failed to override include-deleted")
			}
		}
	})
	t.Run("structured_not_found", func(t *testing.T) {
		// The dedicated smoke org has no agent with this sentinel ID. If a
		// different org does, report the missing error fixture as a skip.
		out, err := runCLI("agents", "get", "1000000000")
		if err == nil {
			t.Skip("sentinel agent exists; no not-found fixture")
		}
		var response struct {
			Error struct {
				StatusCode int             `json:"status_code"`
				Detail     json.RawMessage `json:"detail"`
				Hint       string          `json:"hint"`
			} `json:"error"`
		}
		if err := json.Unmarshal(out, &response); err != nil {
			t.Fatalf("error output is not valid structured JSON: %v", err)
		}
		if response.Error.StatusCode != 404 || len(response.Error.Detail) == 0 || response.Error.Hint == "" {
			t.Fatal("not-found response lacks HTTP 404, API detail, or CLI hint")
		}
	})
}

type liveSmokePage[T any] struct {
	Items []T `json:"items"`
}

func liveSmokeJSON[T any](t *testing.T, args ...string) T {
	t.Helper()
	var result T
	if err := json.Unmarshal(mustRunCLI(t, args...), &result); err != nil {
		t.Fatalf("invalid response JSON: %v", err)
	}
	return result
}
