package client

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// An operation's optional headers are sent, but they can never replace a
// header the client sets itself.
func TestRequestHeadersCannotOverrideReservedHeaders(t *testing.T) {
	var got http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Header.Clone()
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	c := New(server.URL, "real-key")
	c.UserAgent = "syllable-cli/test"
	c.RequestHeaders = func(method, path string) map[string]string {
		return map[string]string{
			"X-Format-Version": "2",
			"Syllable-API-Key": "stolen",
			"accept":           "text/evil",
			"Content-Type":     "text/evil",
			"User-Agent":       "evil",
			"Authorization":    "Bearer evil",
		}
	}
	if _, _, err := c.Post("/api/v1/probe", map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if got.Get("X-Format-Version") != "2" {
		t.Errorf("X-Format-Version = %q, want 2", got.Get("X-Format-Version"))
	}
	if got.Get("Syllable-API-Key") != "real-key" || got.Get("Content-Type") != "application/json" ||
		got.Get("User-Agent") != "syllable-cli/test" || got.Get("Authorization") != "" || got.Get("Accept") == "text/evil" {
		t.Errorf("a reserved header was overridden: %v", got)
	}
}

func TestReservedHeader(t *testing.T) {
	for _, name := range []string{"accept", "Content-Type", "syllable-api-key", "AUTHORIZATION", "Host", "Cookie"} {
		if !ReservedHeader(name) {
			t.Errorf("%s should be reserved", name)
		}
	}
	if ReservedHeader("X-Insight-Workflow-Format-Version") {
		t.Error("an ordinary header must not be reserved")
	}
}
