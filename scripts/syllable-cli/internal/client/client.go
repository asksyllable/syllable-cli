package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// APIError represents an HTTP error response from the Syllable API.
type APIError struct {
	StatusCode int
	Body       []byte
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error (status %d): %s", e.StatusCode, string(e.Body))
}

// DryRunResult is returned instead of making a real HTTP call when DryRun is enabled.
// It carries the JSON-encoded request details that would have been sent.
type DryRunResult struct {
	Output []byte
}

func (e *DryRunResult) Error() string { return "dry-run" }

// Client is the HTTP client for the Syllable API.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	DryRun     bool
	Verbose    bool
}

// New creates a new Client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL: baseURL,
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Do performs an HTTP request and returns the response body, status code, and error.
func (c *Client) Do(method, path string, body interface{}) ([]byte, int, error) {
	var bodyBytes []byte
	if body != nil {
		var err error
		bodyBytes, err = json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshaling request body: %w", err)
		}
	}

	if c.DryRun {
		out := map[string]interface{}{
			"dry_run": true,
			"method":  method,
			"url":     c.BaseURL + path,
		}
		if bodyBytes != nil {
			var bodyJSON json.RawMessage = bodyBytes
			out["body"] = bodyJSON
		}
		data, _ := json.Marshal(out)
		return nil, 0, &DryRunResult{Output: data}
	}

	var bodyReader io.Reader
	if bodyBytes != nil {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	url := c.BaseURL + path
	req, err := http.NewRequest(method, url, bodyReader)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Syllable-API-Key", c.APIKey)
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		if bodyBytes != nil {
			fmt.Fprintf(os.Stderr, "> Content-Type: application/json\n")
			var pretty bytes.Buffer
			if json.Indent(&pretty, bodyBytes, "> ", "  ") == nil {
				fmt.Fprintf(os.Stderr, ">\n> %s\n", pretty.String())
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		fmt.Fprintf(os.Stderr, "< Content-Type: %s\n", resp.Header.Get("Content-Type"))
		var pretty bytes.Buffer
		if json.Indent(&pretty, data, "< ", "  ") == nil {
			fmt.Fprintf(os.Stderr, "<\n< %s\n", pretty.String())
		}
		fmt.Fprintln(os.Stderr)
	}

	if resp.StatusCode >= 400 {
		return data, resp.StatusCode, &APIError{StatusCode: resp.StatusCode, Body: data}
	}

	return data, resp.StatusCode, nil
}

// maskKey returns the API key with the middle portion replaced by asterisks.
func maskKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// Get performs a GET request.
func (c *Client) Get(path string) ([]byte, int, error) {
	return c.Do(http.MethodGet, path, nil)
}

// Post performs a POST request.
func (c *Client) Post(path string, body interface{}) ([]byte, int, error) {
	return c.Do(http.MethodPost, path, body)
}

// Put performs a PUT request.
func (c *Client) Put(path string, body interface{}) ([]byte, int, error) {
	return c.Do(http.MethodPut, path, body)
}

// Delete performs a DELETE request. The Syllable API requires a delete-reason
// query param on most resources; "reason=deleted+via+cli" is appended
// automatically unless the path already contains a query string (in which case
// the caller is responsible for supplying the required param).
func (c *Client) Delete(path string) ([]byte, int, error) {
	if !strings.Contains(path, "?") {
		path += "?reason=" + url.QueryEscape("deleted via cli")
	}
	return c.Do(http.MethodDelete, path, nil)
}

// DeleteWithBody performs a DELETE request with a JSON body.
func (c *Client) DeleteWithBody(path string, body interface{}) ([]byte, int, error) {
	return c.Do(http.MethodDelete, path, body)
}

// DeleteWithForm performs a DELETE request with an application/x-www-form-urlencoded body.
func (c *Client) DeleteWithForm(path string, fields map[string]string) ([]byte, int, error) {
	form := url.Values{}
	for k, v := range fields {
		form.Set(k, v)
	}
	encoded := form.Encode()

	if c.DryRun {
		out := map[string]interface{}{
			"dry_run": true,
			"method":  http.MethodDelete,
			"url":     c.BaseURL + path,
			"body":    encoded,
		}
		data, _ := json.Marshal(out)
		return nil, 0, &DryRunResult{Output: data}
	}

	req, err := http.NewRequest(http.MethodDelete, c.BaseURL+path, strings.NewReader(encoded))
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	req.Header.Set("Syllable-API-Key", c.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", http.MethodDelete, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		fmt.Fprintf(os.Stderr, "> Content-Type: application/x-www-form-urlencoded\n>\n> %s\n\n", encoded)
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		fmt.Fprintf(os.Stderr, "< Content-Type: %s\n", resp.Header.Get("Content-Type"))
		var pretty bytes.Buffer
		if json.Indent(&pretty, data, "< ", "  ") == nil {
			fmt.Fprintf(os.Stderr, "<\n< %s\n", pretty.String())
		}
		fmt.Fprintln(os.Stderr)
	}

	if resp.StatusCode >= 400 {
		return data, resp.StatusCode, &APIError{StatusCode: resp.StatusCode, Body: data}
	}
	return data, resp.StatusCode, nil
}
