package client

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A CLI query option must survive every transport, including dry-run. These
// tests stay in-process and never contact a platform or require credentials.
func TestRequestPathAllTransports(t *testing.T) {
	file := filepath.Join(t.TempDir(), "upload.csv")
	if err := os.WriteFile(file, []byte("a,b\n1,2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, method string
		run          func(*Client) error
	}{
		{"json", "POST", func(c *Client) error { _, _, err := c.Post("/resource", map[string]int{"n": 0}); return err }},
		{"get", "GET", func(c *Client) error { _, _, err := c.Get("/resource"); return err }},
		{"stream", "GET", func(c *Client) error { _, _, err := c.GetStream("/resource"); return err }},
		{"multipart-post", "POST", func(c *Client) error { _, _, err := c.PostMultipart("/resource", "file", file); return err }},
		{"multipart-put", "PUT", func(c *Client) error { _, _, err := c.PutMultipart("/resource", "file", file); return err }},
		{"multipart-form", "PUT", func(c *Client) error {
			_, _, err := c.PutMultipartForm("/resource", map[string]string{"empty": ""}, "", "")
			return err
		}},
		{"form", "DELETE", func(c *Client) error {
			_, _, err := c.DeleteWithForm("/resource", map[string]string{"empty": ""})
			return err
		}},
		{"delete", "DELETE", func(c *Client) error { _, _, err := c.Delete("/resource"); return err }},
		{"delete-body", "DELETE", func(c *Client) error { _, _, err := c.DeleteWithBody("/resource", map[string]int{"n": 0}); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://api.example.invalid", "unit-test")
			c.DryRun = true
			calls := 0
			c.RequestPath = func(method, path string) (string, error) {
				calls++
				if method != tc.method {
					t.Fatalf("hook method=%s, want %s", method, tc.method)
				}
				u, _ := url.Parse(path)
				q := u.Query()
				q.Set("optional", "0 & false+")
				u.RawQuery = q.Encode()
				return u.String(), nil
			}
			var result *DryRunResult
			if err := tc.run(c); !errors.As(err, &result) {
				t.Fatalf("expected dry-run, got %v", err)
			}
			var out struct{ URL string }
			if err := json.Unmarshal(result.Output, &out); err != nil {
				t.Fatal(err)
			}
			u, _ := url.Parse(out.URL)
			if calls != 1 || u.Query().Get("optional") != "0 & false+" {
				t.Fatalf("hook called %d times; output query=%s", calls, u.RawQuery)
			}
			// An override failure must surface before an HTTP request or file read.
			want := errors.New("invalid SDK query")
			c.RequestPath = func(string, string) (string, error) { return "", want }
			if err := tc.run(c); !errors.Is(err, want) {
				t.Fatalf("hook error lost: %v", err)
			}
		})
	}
}

type queryPathTransport func(*http.Request) (*http.Response, error)

func (f queryPathTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRequestPathChangesActualJSONRequest(t *testing.T) {
	c := New("https://api.example.invalid", "unit-test")
	c.RequestPath = func(method, path string) (string, error) {
		return path + "?search_fields=name&search_field_values=foo%2Cbar&search_field_values=", nil
	}
	hit := false
	c.HTTPClient = &http.Client{Transport: queryPathTransport(func(r *http.Request) (*http.Response, error) {
		hit = true
		values := r.URL.Query()["search_field_values"]
		if len(values) != 2 || values[0] != "foo,bar" || values[1] != "" {
			t.Fatalf("actual request changed query values: %#v", values)
		}
		if r.Header.Get("Syllable-API-Key") != "unit-test" || r.Header.Get("Content-Type") != "application/json" {
			t.Fatalf("query hook altered headers: %v", r.Header)
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}
	if _, _, err := c.Post("/resource", map[string]int{"n": 0}); err != nil {
		t.Fatal(err)
	}
	if !hit {
		t.Fatal("request was not sent")
	}
}
