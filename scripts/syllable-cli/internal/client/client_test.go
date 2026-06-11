package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("https://api.example.com", "test-key")

	if c.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", c.BaseURL, "https://api.example.com")
	}
	if c.APIKey != "test-key" {
		t.Errorf("APIKey = %q, want %q", c.APIKey, "test-key")
	}
	if c.HTTPClient == nil {
		t.Fatal("HTTPClient is nil")
	}
	if c.HTTPClient.Timeout.Seconds() != 30 {
		t.Errorf("Timeout = %v, want 30s", c.HTTPClient.Timeout)
	}
}

func TestDoGet(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("Method = %q, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/" {
			t.Errorf("Path = %q, want /api/v1/agents/", r.URL.Path)
		}
		if r.Header.Get("Syllable-API-Key") != "test-key" {
			t.Errorf("Syllable-API-Key = %q, want test-key", r.Header.Get("Syllable-API-Key"))
		}
		if r.Header.Get("Accept") != "application/json" {
			t.Errorf("Accept = %q, want application/json", r.Header.Get("Accept"))
		}
		if r.Header.Get("Content-Type") != "" {
			t.Errorf("Content-Type should be empty for GET, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"items":[]}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	data, status, err := c.Get("/api/v1/agents/")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if string(data) != `{"items":[]}` {
		t.Errorf("data = %q, want %q", string(data), `{"items":[]}`)
	}
}

func TestDoPost(t *testing.T) {
	body := map[string]string{"name": "test-agent"}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		reqBody, _ := io.ReadAll(r.Body)
		var parsed map[string]string
		json.Unmarshal(reqBody, &parsed)
		if parsed["name"] != "test-agent" {
			t.Errorf("body name = %q, want test-agent", parsed["name"])
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"123"}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	data, status, err := c.Post("/api/v1/agents/", body)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 201 {
		t.Errorf("status = %d, want 201", status)
	}
	if string(data) != `{"id":"123"}` {
		t.Errorf("data = %q", string(data))
	}
}

func TestDoPut(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			t.Errorf("Method = %q, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"updated":true}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	data, status, err := c.Put("/api/v1/agents/", map[string]bool{"active": true})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 200 {
		t.Errorf("status = %d, want 200", status)
	}
	if string(data) != `{"updated":true}` {
		t.Errorf("data = %q", string(data))
	}
}

func TestDoDelete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			t.Errorf("Method = %q, want DELETE", r.Method)
		}
		if r.URL.Path != "/api/v1/agents/456" {
			t.Errorf("Path = %q, want /api/v1/agents/456", r.URL.Path)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	data, status, err := c.Delete("/api/v1/agents/456")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if status != 204 {
		t.Errorf("status = %d, want 204", status)
	}
	if len(data) != 0 {
		t.Errorf("data = %q, want empty", string(data))
	}
}

func TestDoAPIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":"not found"}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	data, status, err := c.Get("/api/v1/agents/999")

	if err == nil {
		t.Fatal("expected error for 404 response")
	}
	if status != 404 {
		t.Errorf("status = %d, want 404", status)
	}
	if string(data) != `{"error":"not found"}` {
		t.Errorf("data = %q", string(data))
	}
}

func TestDoServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	_, status, err := c.Get("/api/v1/agents/")

	if err == nil {
		t.Fatal("expected error for 500 response")
	}
	if status != 500 {
		t.Errorf("status = %d, want 500", status)
	}
}

func TestDoUnreachableServer(t *testing.T) {
	c := New("http://localhost:1", "test-key")
	_, _, err := c.Get("/api/v1/agents/")

	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestDoNilBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if len(body) != 0 {
			t.Errorf("expected empty body for nil input, got %q", string(body))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}))
	defer server.Close()

	c := New(server.URL, "test-key")
	_, _, err := c.Do(http.MethodPost, "/test", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRedactSensitive(t *testing.T) {
	in := []byte(`{"name":"svc","auth_values":{"password":"p","token":"t"},"nested":{"client_secret":"s","keep":"ok"},"count":3}`)
	out := redactSensitive(in)

	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatalf("redacted output is not valid JSON: %v", err)
	}
	if m["name"] != "svc" || m["count"] != float64(3) {
		t.Errorf("non-sensitive fields altered: %#v", m)
	}
	// auth_values is itself a sensitive key → whole value replaced.
	if m["auth_values"] != redactedPlaceholder {
		t.Errorf("auth_values not redacted: %#v", m["auth_values"])
	}
	nested, _ := m["nested"].(map[string]interface{})
	if nested == nil || nested["client_secret"] != redactedPlaceholder {
		t.Errorf("nested client_secret not redacted: %#v", m["nested"])
	}
	if nested["keep"] != "ok" {
		t.Errorf("non-sensitive nested field altered: %#v", nested["keep"])
	}
	// No secret value should survive anywhere in the output.
	if strings.Contains(string(out), `"p"`) || strings.Contains(string(out), `"s"`) || strings.Contains(string(out), `"t"`) {
		t.Errorf("a secret value leaked into redacted output: %s", out)
	}
}

func TestCheckRedirectStripsKeyCrossHost(t *testing.T) {
	orig, _ := http.NewRequest(http.MethodGet, "https://api.syllable.cloud/x", nil)

	// Cross-host redirect: the API key header must be stripped.
	cross, _ := http.NewRequest(http.MethodGet, "https://evil.example.com/x", nil)
	cross.Header.Set("Syllable-API-Key", "secret")
	if err := checkRedirect(cross, []*http.Request{orig}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cross.Header.Get("Syllable-API-Key") != "" {
		t.Error("API key must be stripped on a cross-host redirect")
	}

	// Same-host redirect: the header is retained.
	same, _ := http.NewRequest(http.MethodGet, "https://api.syllable.cloud/y", nil)
	same.Header.Set("Syllable-API-Key", "secret")
	if err := checkRedirect(same, []*http.Request{orig}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if same.Header.Get("Syllable-API-Key") != "secret" {
		t.Error("API key must be retained on a same-host redirect")
	}

	// Too many redirects → error.
	via := make([]*http.Request, 10)
	for i := range via {
		via[i] = orig
	}
	if err := checkRedirect(same, via); err == nil {
		t.Error("expected an error after 10 redirects")
	}
}

func TestMaskKey(t *testing.T) {
	cases := map[string]string{
		"":               "****",     // empty
		"short":          "****",     // <= 8 chars: fully masked
		"12345678":       "****",     // exactly 8
		"123456789":      "1234****", // > 8: short prefix only, no suffix or length
		"syl_abcdef1234": "syl_****",
	}
	for in, want := range cases {
		if got := maskKey(in); got != want {
			t.Errorf("maskKey(%q) = %q, want %q", in, got, want)
		}
		// Never leak the tail of a key.
		if len(in) > 8 && strings.Contains(maskKey(in), in[len(in)-4:]) {
			t.Errorf("maskKey(%q) leaked the key suffix", in)
		}
	}
}
