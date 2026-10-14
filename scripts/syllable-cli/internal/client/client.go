package client

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
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
	// Preflight, when set, returns the names of required request-body fields
	// missing from a JSON body. It's used only to annotate --dry-run output, so
	// it never blocks a real request (spec and prod can legitimately differ). The
	// validator is injected by the cmd layer so this package stays spec-free (#143).
	Preflight func(method, path string, body []byte) []string
	// RequestPath applies operation-scoped CLI query options before any request
	// is sent or rendered by --dry-run. It is set on a command-local client copy.
	RequestPath func(method, path string) (string, error)
	// RequestAccept selects the SDK operation's response media type.
	RequestAccept func(method, path string) string
	// DeleteReasonRequired limits the legacy default delete reason to operations
	// that declare it. Nil preserves compatibility for direct client callers.
	DeleteReasonRequired func(path string) bool
	// A nil RequestTimeout preserves each operation's timeout; explicit zero
	// disables the timeout. Retry controls are disabled unless Retries is set.
	RequestTimeout        *time.Duration
	Retries               int
	RetryWrites           bool
	RetryDelay            time.Duration
	RetryMaxDelay         time.Duration
	RetryExponent         float64
	RetryMaxElapsedTime   time.Duration
	RetryConnectionErrors bool
	RetryStatuses         []string
	UserAgent             string
	UploadFilename        string
	UploadContentType     string
}

// New creates a new Client.
func New(baseURL, apiKey string) *Client {
	return &Client{
		BaseURL:             baseURL,
		APIKey:              apiKey,
		HTTPClient:          newHTTPClient(30 * time.Second),
		RetryDelay:          500 * time.Millisecond,
		RetryMaxDelay:       time.Minute,
		RetryExponent:       1.5,
		RetryMaxElapsedTime: time.Hour,
		RetryStatuses:       []string{"429", "500", "502", "503", "504"},
	}
}

// newHTTPClient builds an http.Client with the credential-safe redirect policy.
func newHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout:       timeout,
		CheckRedirect: checkRedirect,
	}
}

// checkRedirect strips the Syllable-API-Key header when a redirect crosses to a
// different host. Go's default policy only strips standard auth headers
// (Authorization, Cookie), not custom ones, so without this the API key would
// be forwarded to whatever host a 3xx points at (#125).
func checkRedirect(req *http.Request, via []*http.Request) error {
	if len(via) >= 10 {
		return errors.New("stopped after 10 redirects")
	}
	if len(via) > 0 && req.URL.Host != via[0].URL.Host {
		req.Header.Del("Syllable-API-Key")
	}
	return nil
}

const redactedPlaceholder = "***REDACTED***"

// isSensitiveKey reports whether a body/field key's value is a credential that
// must be masked in --debug output (e.g. service auth_values) (#124).
func isSensitiveKey(key string) bool {
	k := strings.ToLower(key)
	for _, s := range []string{"password", "secret", "token", "api_key", "apikey", "auth_values", "authorization", "hmac", "credential", "private_key"} {
		if strings.Contains(k, s) {
			return true
		}
	}
	return false
}

// redactSensitive parses JSON and replaces the values of sensitive keys with a
// placeholder, returning compact JSON for display. Non-JSON or marshal failures
// return the input unchanged (the caller only uses this for debug output).
func redactSensitive(raw []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	redactValue(v)
	out, err := json.Marshal(v)
	if err != nil {
		return raw
	}
	return out
}

func redactValue(v interface{}) {
	switch t := v.(type) {
	case map[string]interface{}:
		for k, val := range t {
			if isSensitiveKey(k) {
				t[k] = redactedPlaceholder
			} else {
				redactValue(val)
			}
		}
	case []interface{}:
		for _, item := range t {
			redactValue(item)
		}
	}
}

// Do performs an HTTP request and returns the response body, status code, and error.
func (c *Client) Do(method, path string, body interface{}) ([]byte, int, error) {
	path, err := c.requestPath(method, path)
	if err != nil {
		return nil, 0, err
	}
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
			if c.Preflight != nil {
				if missing := c.Preflight(method, path, bodyBytes); len(missing) > 0 {
					out["missing_required_fields"] = missing
				}
			}
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

	c.setRequestHeaders(req, path, "application/json")
	if bodyBytes != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		if bodyBytes != nil {
			fmt.Fprintf(os.Stderr, "> Content-Type: application/json\n")
			var pretty bytes.Buffer
			if json.Indent(&pretty, redactSensitive(bodyBytes), "> ", "  ") == nil {
				fmt.Fprintf(os.Stderr, ">\n> %s\n", pretty.String())
			}
		}
		fmt.Fprintln(os.Stderr)
	}

	resp, err := c.execute(req, nil)
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
	// Reveal only a short prefix for recognizability — not the suffix, and not
	// the exact length (a fixed-width mask), so --debug output is safer to share (#127).
	return key[:4] + "****"
}

// Get performs a GET request.
func (c *Client) Get(path string) ([]byte, int, error) {
	return c.Do(http.MethodGet, path, nil)
}

// GetStream performs a GET and returns the response body as an io.ReadCloser
// without buffering it. The caller must Close the returned reader.
//
// Use this for endpoints that return potentially large binary payloads (audio
// recordings, file downloads) where buffering the whole response into memory
// is wasteful. A 30-minute timeout is used since recordings can be long; the
// regular Get's 30-second timeout would cut off mid-stream.
//
// On non-2xx responses the body is read into memory and returned as an
// APIError, since errors are typically small JSON payloads.
func (c *Client) GetStream(path string) (io.ReadCloser, int, error) {
	path, err := c.requestPath(http.MethodGet, path)
	if err != nil {
		return nil, 0, err
	}
	if c.DryRun {
		out := map[string]interface{}{
			"dry_run": true,
			"method":  http.MethodGet,
			"url":     c.BaseURL + path,
			"stream":  true,
		}
		data, _ := json.Marshal(out)
		return nil, 0, &DryRunResult{Output: data}
	}

	req, err := http.NewRequest(http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	c.setRequestHeaders(req, path, "application/octet-stream")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", http.MethodGet, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		fmt.Fprintln(os.Stderr)
	}

	streamTimeout := 30 * time.Minute
	resp, err := c.execute(req, &streamTimeout)
	if err != nil {
		return nil, 0, fmt.Errorf("executing request: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "< %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
			fmt.Fprintf(os.Stderr, "< %s\n\n", string(data))
		}
		return nil, resp.StatusCode, &APIError{StatusCode: resp.StatusCode, Body: data}
	}

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "< %d %s\n", resp.StatusCode, http.StatusText(resp.StatusCode))
		fmt.Fprintf(os.Stderr, "< Content-Type: %s\n", resp.Header.Get("Content-Type"))
		fmt.Fprintln(os.Stderr, "< (streaming body to stdout)")
		fmt.Fprintln(os.Stderr)
	}

	return resp.Body, resp.StatusCode, nil
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
	if !strings.Contains(path, "?") && (c.DeleteReasonRequired == nil || c.DeleteReasonRequired(path)) {
		path += "?reason=" + url.QueryEscape("deleted via cli")
	}
	return c.Do(http.MethodDelete, path, nil)
}

// DeleteWithBody performs a DELETE request with a JSON body.
func (c *Client) DeleteWithBody(path string, body interface{}) ([]byte, int, error) {
	return c.Do(http.MethodDelete, path, body)
}

// PostWithTimeout performs a POST request with a custom timeout. It runs the
// request through a shallow copy of the client that has its own http.Client, so
// it never mutates the shared client's timeout and is safe to call concurrently (#132).
func (c *Client) PostWithTimeout(path string, body interface{}, timeout time.Duration) ([]byte, int, error) {
	tmp := *c
	tmp.HTTPClient = c.httpClientFor(&timeout)
	return tmp.Do(http.MethodPost, path, body)
}

// PostMultipart performs a multipart/form-data POST, uploading a local file as the named field.
// Large files may require more than the default 30s client timeout — callers should be aware.
func (c *Client) PostMultipart(path, fieldName, filePath string) ([]byte, int, error) {
	return c.doMultipart(http.MethodPost, path, fieldName, filePath)
}

// PutMultipart performs a multipart/form-data PUT, uploading a local file as the named field.
func (c *Client) PutMultipart(path, fieldName, filePath string) ([]byte, int, error) {
	return c.doMultipart(http.MethodPut, path, fieldName, filePath)
}

func (c *Client) doMultipart(method, path, fieldName, filePath string) ([]byte, int, error) {
	path, err := c.requestPath(method, path)
	if err != nil {
		return nil, 0, err
	}
	var src io.Reader
	var formName string
	if filePath == "-" {
		src = os.Stdin
		formName = "stdin"
	} else {
		f, err := os.Open(filePath)
		if err != nil {
			return nil, 0, fmt.Errorf("opening file: %w", err)
		}
		defer f.Close()
		src = f
		formName = filepath.Base(filePath)
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	part, err := c.createFormFile(w, fieldName, formName)
	if err != nil {
		return nil, 0, fmt.Errorf("creating form file: %w", err)
	}
	if _, err := io.Copy(part, src); err != nil {
		return nil, 0, fmt.Errorf("copying file to form: %w", err)
	}
	w.Close()

	if c.DryRun {
		out := map[string]interface{}{
			"dry_run":      true,
			"method":       method,
			"url":          c.BaseURL + path,
			"field":        fieldName,
			"file":         filePath,
			"content_type": w.FormDataContentType(),
		}
		data, _ := json.Marshal(out)
		return nil, 0, &DryRunResult{Output: data}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &buf)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	c.setRequestHeaders(req, path, "application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		fmt.Fprintf(os.Stderr, "> Content-Type: %s\n", w.FormDataContentType())
		fmt.Fprintf(os.Stderr, "> (multipart file: %s, field: %s)\n\n", filePath, fieldName)
	}

	// Use a longer timeout for file uploads
	uploadTimeout := 5 * time.Minute
	resp, err := c.execute(req, &uploadTimeout)
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

// PutMultipartForm performs a multipart/form-data PUT that mixes text fields
// and an optional file. Pass fileField="" or filePath="" to send fields only.
func (c *Client) PutMultipartForm(path string, fields map[string]string, fileField, filePath string) ([]byte, int, error) {
	return c.doMultipartForm(http.MethodPut, path, fields, fileField, filePath)
}

func (c *Client) doMultipartForm(method, path string, fields map[string]string, fileField, filePath string) ([]byte, int, error) {
	path, err := c.requestPath(method, path)
	if err != nil {
		return nil, 0, err
	}
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for k, v := range fields {
		if err := w.WriteField(k, v); err != nil {
			return nil, 0, fmt.Errorf("writing form field %s: %w", k, err)
		}
	}

	if fileField != "" && filePath != "" {
		var src io.Reader
		var formName string
		if filePath == "-" {
			src = os.Stdin
			formName = "stdin"
		} else {
			f, err := os.Open(filePath)
			if err != nil {
				return nil, 0, fmt.Errorf("opening file: %w", err)
			}
			defer f.Close()
			src = f
			formName = filepath.Base(filePath)
		}
		part, err := c.createFormFile(w, fileField, formName)
		if err != nil {
			return nil, 0, fmt.Errorf("creating form file: %w", err)
		}
		if _, err := io.Copy(part, src); err != nil {
			return nil, 0, fmt.Errorf("copying file to form: %w", err)
		}
	}
	w.Close()

	if c.DryRun {
		out := map[string]interface{}{
			"dry_run":      true,
			"method":       method,
			"url":          c.BaseURL + path,
			"fields":       fields,
			"content_type": w.FormDataContentType(),
		}
		if fileField != "" && filePath != "" {
			out["file_field"] = fileField
			out["file"] = filePath
		}
		data, _ := json.Marshal(out)
		return nil, 0, &DryRunResult{Output: data}
	}

	req, err := http.NewRequest(method, c.BaseURL+path, &buf)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}
	c.setRequestHeaders(req, path, "application/json")
	req.Header.Set("Content-Type", w.FormDataContentType())

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", method, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		fmt.Fprintf(os.Stderr, "> Content-Type: %s\n", w.FormDataContentType())
		for k, v := range fields {
			if isSensitiveKey(k) {
				v = redactedPlaceholder
			}
			fmt.Fprintf(os.Stderr, "> (form field) %s=%s\n", k, v)
		}
		if fileField != "" && filePath != "" {
			fmt.Fprintf(os.Stderr, "> (multipart file: %s, field: %s)\n", filePath, fileField)
		}
		fmt.Fprintln(os.Stderr)
	}

	uploadTimeout := 5 * time.Minute
	resp, err := c.execute(req, &uploadTimeout)
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

// DeleteWithForm performs a DELETE request with an application/x-www-form-urlencoded body.
func (c *Client) DeleteWithForm(path string, fields map[string]string) ([]byte, int, error) {
	path, err := c.requestPath(http.MethodDelete, path)
	if err != nil {
		return nil, 0, err
	}
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
	c.setRequestHeaders(req, path, "application/json")
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if c.Verbose {
		fmt.Fprintf(os.Stderr, "> %s %s\n", http.MethodDelete, c.BaseURL+path)
		fmt.Fprintf(os.Stderr, "> Syllable-API-Key: %s\n", maskKey(c.APIKey))
		fmt.Fprintf(os.Stderr, "> Content-Type: application/x-www-form-urlencoded\n>\n> %s\n\n", encoded)
	}

	resp, err := c.execute(req, nil)
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

func (c *Client) requestPath(method, path string) (string, error) {
	if c.RequestPath == nil {
		return path, nil
	}
	return c.RequestPath(method, path)
}
