package client

import (
	"bytes"
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
	"sync"
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
	var retryDeadline time.Time
	if retries > 0 && c.RetryMaxElapsedTime > 0 {
		retryDeadline = time.Now().Add(c.RetryMaxElapsedTime)
	}
	current := req.Clone(req.Context())
	var previousResponse *http.Response
	var previousError error
	var retire io.Closer
	for attempt := 0; ; attempt++ {
		// The initial attempt is governed only by the request timeout. Retry
		// preparation, backoff, and cleanup must leave time for a new attempt.
		if attempt > 0 && req.Context().Err() != nil {
			closeRequestBody(current)
			closeResponseBody(previousResponse)
			return nil, req.Context().Err()
		}
		if attempt > 0 && !retryBudgetAvailable(retryDeadline, 0) {
			closeRequestBody(current)
			return previousResponse, previousError
		}
		attemptClient := httpClient
		var gate *retryAttemptTransport
		if attempt > 0 {
			base := httpClient.Transport
			if base == nil {
				base = http.DefaultTransport
			}
			gate = &retryAttemptTransport{base: base, deadline: retryDeadline, retire: retire}
			copy := *httpClient
			copy.Transport = gate
			attemptClient = &copy
		}
		resp, err := attemptClient.Do(current)
		if gate != nil {
			gate.join()
			if errors.Is(err, errRetryBeforeStart) {
				if contextErr := req.Context().Err(); contextErr != nil {
					closeResponseBody(previousResponse)
					return nil, contextErr
				}
				return previousResponse, previousError
			}
			if !gate.started {
				closeResponseBody(previousResponse)
			}
		}
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
		if !retryBudgetAvailable(retryDeadline, delay) {
			return resp, err
		}
		if c.Verbose {
			fmt.Fprintf(os.Stderr, "retrying %s request (%d/%d) in %s\n", req.Method, attempt+1, retries, delay)
		}
		timer := time.NewTimer(delay)
		select {
		case <-req.Context().Done():
			timer.Stop()
			closeResponseBody(resp)
			return nil, req.Context().Err()
		case <-timer.C:
		}
		if contextErr := req.Context().Err(); contextErr != nil {
			closeResponseBody(resp)
			return nil, contextErr
		}
		if !retryBudgetAvailable(retryDeadline, 0) {
			return resp, err
		}
		next := req.Clone(req.Context())
		if req.Body != nil {
			body, replayErr := req.GetBody()
			next.Body = body
			if contextErr := req.Context().Err(); contextErr != nil {
				closeRequestBody(next)
				closeResponseBody(resp)
				return nil, contextErr
			}
			if !retryBudgetAvailable(retryDeadline, 0) {
				closeRequestBody(next)
				return resp, err
			}
			if replayErr != nil {
				closeRequestBody(next)
				closeResponseBody(resp)
				return nil, fmt.Errorf("replaying request body: %w", replayErr)
			}
		}
		if resp != nil && resp.Body != nil {
			// Retain the response while draining: if the budget runs out, its
			// complete error body must still be available to the caller.
			body := newRetryResponseBody(resp.Body)
			resp.Body = body
			if cleanupErr := body.drain(req.Context(), retryDeadline); cleanupErr != nil {
				closeRequestBody(next)
				if req.Context().Err() != nil {
					_ = body.Close()
					return nil, req.Context().Err()
				}
				return resp, err
			}
			if body.complete {
				// EOF/error leaves a complete snapshot, so even a slow Close can
				// exhaust the budget without losing the last response's bytes.
				_ = body.Close()
				retire = nil
			} else {
				// Closing a capped body loses its unread tail. Defer retirement
				// until the next attempt commits at its first RoundTrip entry.
				retire = body
			}
		} else {
			retire = nil
		}
		if contextErr := req.Context().Err(); contextErr != nil {
			closeRequestBody(next)
			closeResponseBody(resp)
			return nil, contextErr
		}
		if !retryBudgetAvailable(retryDeadline, 0) {
			closeRequestBody(next)
			return resp, err
		}
		previousResponse, previousError, current = resp, err, next
	}
}

var errRetryBeforeStart = errors.New("retry budget expired before attempt started")

// Only the first RoundTrip starts an attempt; redirects belong to that same
// attempt. Retiring an incomplete response concurrently frees a connection
// even when MaxConnsPerHost is one. A committed attempt always returns its own
// result, never the retired response whose unread bytes are no longer valid.
type retryAttemptTransport struct {
	base     http.RoundTripper
	deadline time.Time
	retire   io.Closer
	once     sync.Once
	started  bool
	startErr error
	retired  chan struct{}
}

func (t *retryAttemptTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.once.Do(func() {
		if err := req.Context().Err(); err != nil {
			t.startErr = err
			return
		}
		if !retryBudgetAvailable(t.deadline, 0) {
			t.startErr = errRetryBeforeStart
			return
		}
		t.started = true
		if t.retire != nil {
			t.retired = make(chan struct{})
			go func() {
				_ = t.retire.Close()
				close(t.retired)
			}()
		}
	})
	if t.startErr != nil {
		closeRequestBody(req)
		return nil, t.startErr
	}
	return t.base.RoundTrip(req)
}

func (t *retryAttemptTransport) join() {
	if t.retired != nil {
		<-t.retired
	}
}

// Preserve cancellation for legacy custom transports that use CancelRequest.
func (t *retryAttemptTransport) CancelRequest(req *http.Request) {
	if base, ok := t.base.(interface{ CancelRequest(*http.Request) }); ok {
		base.CancelRequest(req)
	}
}

func retryBudgetAvailable(deadline time.Time, delay time.Duration) bool {
	return deadline.IsZero() || time.Until(deadline) > delay
}

func closeRequestBody(req *http.Request) {
	if req.Body != nil {
		_ = req.Body.Close()
	}
}

func closeResponseBody(resp *http.Response) {
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
}

const retryDrainLimit = 64 * 1024

// retryResponseBody owns one capped read worker. Subsequent reads join it and
// replay the prefix before reading the original remainder. Budget expiry keeps
// the original response open. Close must unblock Read, as required by
// http.Response.Body, and joins the worker.
type retryResponseBody struct {
	original  io.ReadCloser
	done      chan struct{}
	data      []byte
	readErr   error
	complete  bool
	reader    io.Reader
	readOnce  sync.Once
	closeOnce sync.Once
	closeErr  error
}

func newRetryResponseBody(original io.ReadCloser) *retryResponseBody {
	body := &retryResponseBody{original: original, done: make(chan struct{})}
	go func() {
		buffer := make([]byte, retryDrainLimit)
		used, emptyReads := 0, 0
		for used < len(buffer) {
			n, err := original.Read(buffer[used:])
			used += n
			if err != nil {
				body.complete = true
				if err != io.EOF {
					body.readErr = err
				}
				break
			}
			if n == 0 {
				emptyReads++
				if emptyReads >= 100 {
					body.complete, body.readErr = true, io.ErrNoProgress
					break
				}
			} else {
				emptyReads = 0
			}
		}
		body.data = buffer[:used]
		close(body.done)
	}()
	return body
}

func (b *retryResponseBody) drain(ctx context.Context, deadline time.Time) error {
	var expired <-chan time.Time
	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()
		expired = timer.C
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-expired:
		return context.DeadlineExceeded
	case <-b.done:
		return nil
	}
}

func (b *retryResponseBody) Read(p []byte) (int, error) {
	<-b.done
	b.readOnce.Do(func() {
		b.reader = bytes.NewReader(b.data)
		if !b.complete {
			b.reader = io.MultiReader(b.reader, b.original)
		}
	})
	n, err := b.reader.Read(p)
	if err == io.EOF && b.readErr != nil {
		err = b.readErr
	}
	return n, err
}

func (b *retryResponseBody) Close() error {
	b.closeOnce.Do(func() { b.closeErr = b.original.Close() })
	<-b.done
	return b.closeErr
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
