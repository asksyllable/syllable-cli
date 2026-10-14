package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"time"
)

// httpClientFor keeps the caller's transport, cookie jar, and redirect policy.
// Per-operation defaults and explicit timeout overrides use a shallow copy so
// neither a stream nor a concurrent request changes the shared HTTP client.
func (c *Client) httpClientFor(operationTimeout *time.Duration) *http.Client {
	base := c.HTTPClient
	if base == nil {
		base = newHTTPClient(30 * time.Second)
	}
	if operationTimeout == nil && c.RequestTimeout == nil {
		return base
	}
	copy := *base
	if operationTimeout != nil {
		copy.Timeout = *operationTimeout
	}
	if c.RequestTimeout != nil {
		copy.Timeout = *c.RequestTimeout
	}
	return &copy
}

// execute is shared by JSON, form, upload, and streaming transports. Retries
// are opt-in and replay the already encoded request body without re-reading a
// file or stdin. A mutating method also requires RetryWrites.
func (c *Client) execute(req *http.Request, operationTimeout *time.Duration) (*http.Response, error) {
	httpClient := c.httpClientFor(operationTimeout)
	retries := c.Retries
	if retries < 0 || !c.retryMethod(req.Method) || req.Body != nil && req.GetBody == nil {
		retries = 0
	}
	started := time.Now()
	for attempt := 0; ; attempt++ {
		current := req.Clone(req.Context())
		if attempt > 0 && req.Body != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, fmt.Errorf("replaying request body: %w", err)
			}
			current.Body = body
		}
		resp, err := httpClient.Do(current)
		if contextErr := req.Context().Err(); contextErr != nil {
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			return nil, contextErr
		}
		if attempt >= retries || !c.retryResponse(resp, err) {
			return resp, err
		}
		delay := c.retryDelay(attempt, resp, time.Now())
		if c.RetryMaxElapsedTime > 0 {
			elapsed := time.Since(started)
			if elapsed >= c.RetryMaxElapsedTime || delay >= c.RetryMaxElapsedTime-elapsed {
				return resp, err
			}
		}
		if resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))
			_ = resp.Body.Close()
		}
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "retrying %s request (%d/%d) in %s\n", req.Method, attempt+1, retries, delay)
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

func (c *Client) retryMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	default:
		return c.RetryWrites
	}
}

func (c *Client) retryResponse(resp *http.Response, err error) bool {
	if err != nil {
		return c.RetryConnectionErrors && !errors.Is(err, context.Canceled)
	}
	if resp == nil {
		return false
	}
	status := strconv.Itoa(resp.StatusCode)
	for _, match := range c.RetryStatuses {
		if match == status || len(match) == 3 && strings.EqualFold(match[1:], "xx") && match[0] == status[0] {
			return true
		}
	}
	return false
}

func (c *Client) retryDelay(attempt int, resp *http.Response, now time.Time) time.Duration {
	backoff := float64(c.RetryDelay) * math.Pow(float64(attempt+1), c.RetryExponent)
	delay := time.Duration(backoff)
	if backoff >= float64(math.MaxInt64) || math.IsInf(backoff, 1) {
		delay = time.Duration(math.MaxInt64)
	} else if delay < 0 || math.IsNaN(backoff) {
		delay = 0
	}
	if resp != nil {
		if serverDelay, ok := retryAfter(resp.Header, now); ok {
			delay = serverDelay
		}
	}
	if c.RetryMaxDelay > 0 && delay > c.RetryMaxDelay {
		delay = c.RetryMaxDelay
	}
	return delay
}

func retryAfter(headers http.Header, now time.Time) (time.Duration, bool) {
	if milliseconds := headers.Get("Retry-After-Ms"); milliseconds != "" {
		if n, err := strconv.ParseFloat(milliseconds, 64); err == nil && n >= 0 && !math.IsInf(n, 0) && !math.IsNaN(n) && n <= float64(math.MaxInt64)/float64(time.Millisecond) {
			return time.Duration(n * float64(time.Millisecond)), true
		}
	}
	value := headers.Get("Retry-After")
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil && seconds >= 0 && seconds <= math.MaxInt64/int64(time.Second) {
		return time.Duration(seconds) * time.Second, true
	}
	if date, err := http.ParseTime(value); err == nil {
		delay := date.Sub(now)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

func (c *Client) requestAccept(method, path, fallback string) string {
	if c.RequestAccept != nil {
		if accept := c.RequestAccept(method, path); accept != "" {
			return accept
		}
	}
	return fallback
}

func (c *Client) setRequestHeaders(req *http.Request, path, accept string) {
	if c.APIKey != "" {
		req.Header.Set("Syllable-API-Key", c.APIKey)
	}
	req.Header.Set("Accept", c.requestAccept(req.Method, path, accept))
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
}

func (c *Client) createFormFile(writer *multipart.Writer, fieldName, fileName string) (io.Writer, error) {
	if c.UploadFilename != "" {
		fileName = c.UploadFilename
	}
	contentType := c.UploadContentType
	if contentType == "" {
		contentType = sdkFileContentType(fileName)
	}
	headers := textproto.MIMEHeader{}
	headers.Set("Content-Disposition", mime.FormatMediaType("form-data", map[string]string{"name": fieldName, "filename": fileName}))
	headers.Set("Content-Type", contentType)
	return writer.CreatePart(headers)
}

// Match the pinned TypeScript SDK's lib/files.ts MIME inference rather than an
// OS MIME database whose output (notably charsets and JavaScript types) varies.
func sdkFileContentType(fileName string) string {
	types := map[string]string{
		"json": "application/json", "xml": "application/xml", "html": "text/html", "htm": "text/html",
		"txt": "text/plain", "csv": "text/csv", "pdf": "application/pdf", "png": "image/png",
		"jpg": "image/jpeg", "jpeg": "image/jpeg", "gif": "image/gif", "svg": "image/svg+xml",
		"js": "application/javascript", "css": "text/css", "zip": "application/zip", "tar": "application/x-tar",
		"gz": "application/gzip", "mp4": "video/mp4", "mp3": "audio/mpeg", "wav": "audio/wav",
		"webp": "image/webp", "ico": "image/x-icon", "woff": "font/woff", "woff2": "font/woff2",
		"ttf": "font/ttf", "otf": "font/otf",
	}
	parts := strings.Split(strings.ToLower(fileName), ".")
	ext := parts[len(parts)-1]
	if contentType := types[ext]; contentType != "" {
		return contentType
	}
	return "application/octet-stream"
}
