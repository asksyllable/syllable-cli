package client

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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
