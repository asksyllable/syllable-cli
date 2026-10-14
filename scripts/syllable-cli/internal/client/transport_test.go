package client

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/cookiejar"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type closeTrackingBody struct {
	io.Reader
	closed bool
}

func (b *closeTrackingBody) Close() error { b.closed = true; return nil }

func TestRetriesRequireExplicitMethodAuthorization(t *testing.T) {
	for _, tc := range []struct {
		method      string
		retries     int
		writes      bool
		wantCalls   int
		wantSuccess bool
	}{
		{"GET", 0, false, 1, false},
		{"GET", 2, false, 2, true},
		{"POST", 2, false, 1, false},
		{"POST", 2, true, 2, true},
		{"PUT", 2, false, 1, false},
		{"PUT", 2, true, 2, true},
		{"DELETE", 2, false, 1, false},
		{"DELETE", 2, true, 2, true},
	} {
		t.Run(tc.method+"/"+string(rune('0'+tc.retries))+"/"+map[bool]string{true: "writes", false: "reads"}[tc.writes], func(t *testing.T) {
			c := New("https://api.example.invalid", "unit-test")
			c.Retries, c.RetryWrites, c.RetryDelay = tc.retries, tc.writes, 0
			calls := 0
			first := &closeTrackingBody{Reader: strings.NewReader(`{"detail":"temporary"}`)}
			var requestBodies []string
			c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if r.Body != nil {
					body, _ := io.ReadAll(r.Body)
					_ = r.Body.Close()
					requestBodies = append(requestBodies, string(body))
				}
				if calls == 1 {
					return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
				}
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
			})
			body, status, err := c.Do(tc.method, "/resource", map[string]interface{}{"enabled": false, "count": 0})
			if calls != tc.wantCalls || tc.wantSuccess != (err == nil) {
				t.Fatalf("calls=%d, status=%d, err=%v", calls, status, err)
			}
			if !first.closed {
				t.Fatal("failed attempt body was not closed")
			}
			if calls == 2 && (requestBodies[0] != requestBodies[1] || string(body) != `{"ok":true}`) {
				t.Fatalf("request body was not replayed: %v", requestBodies)
			}
		})
	}
}

func TestRetryExhaustionAndSelectedStatuses(t *testing.T) {
	for _, tc := range []struct {
		status   int
		statuses []string
		want     int
	}{
		{503, []string{"503"}, 3},
		{501, []string{"5xx"}, 3},
		{401, []string{"429", "5xx"}, 1},
		{429, []string{"500"}, 1},
	} {
		c := New("https://api.example.invalid", "unit-test")
		c.Retries, c.RetryDelay, c.RetryStatuses = 2, 0, tc.statuses
		calls := 0
		c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"detail":"last attempt"}`))}, nil
		})
		body, status, err := c.Get("/resource")
		var apiErr *APIError
		if calls != tc.want || status != tc.status || !errors.As(err, &apiErr) || string(body) != `{"detail":"last attempt"}` {
			t.Fatalf("status=%d: calls=%d status=%d body=%q err=%v", tc.status, calls, status, body, err)
		}
	}
}

func TestRetryConnectionErrorsAndCancellation(t *testing.T) {
	for _, connectionRetries := range []bool{false, true} {
		c := New("https://api.example.invalid", "unit-test")
		c.Retries, c.RetryDelay, c.RetryConnectionErrors = 2, 0, connectionRetries
		calls := 0
		c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			return nil, errors.New("connection reset")
		})
		_, _, err := c.Get("/resource")
		want := 1
		if connectionRetries {
			want = 3
		}
		if calls != want || err == nil {
			t.Fatalf("connectionRetries=%v calls=%d err=%v", connectionRetries, calls, err)
		}
	}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryConnectionErrors = 2, time.Hour, true
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.invalid/resource", nil)
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		cancel()
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	if _, err := c.execute(req, nil); !errors.Is(err, context.Canceled) || calls != 1 {
		t.Fatalf("cancellation failed to interrupt backoff: calls=%d err=%v", calls, err)
	}
}

func TestRetryAfterAndBackoff(t *testing.T) {
	now := time.Date(2026, 10, 13, 12, 0, 0, 0, time.UTC)
	c := New("https://api.example.invalid", "unit-test")
	c.RetryDelay, c.RetryExponent, c.RetryMaxDelay = time.Second, 2, 10*time.Second
	for _, tc := range []struct {
		attempt int
		headers http.Header
		want    time.Duration
	}{
		{0, nil, time.Second},
		{1, nil, 4 * time.Second},
		{4, nil, 10 * time.Second},
		{1, http.Header{"Retry-After": {"3"}}, 3 * time.Second},
		{1, http.Header{"Retry-After": {"0"}}, 0},
		{1, http.Header{"Retry-After": {"invalid"}}, 4 * time.Second},
		{1, http.Header{"Retry-After": {"1000"}}, 10 * time.Second},
		{1, http.Header{"Retry-After": {now.Add(8 * time.Second).Format(http.TimeFormat)}}, 8 * time.Second},
		{1, http.Header{"Retry-After": {now.Add(-time.Hour).Format(http.TimeFormat)}}, 0},
		{1, http.Header{"Retry-After-Ms": {"125"}, "Retry-After": {"3"}}, 125 * time.Millisecond},
		{1, http.Header{"Retry-After-Ms": {"NaN"}, "Retry-After": {"3"}}, 3 * time.Second},
	} {
		if got := c.retryDelay(tc.attempt, &http.Response{Header: tc.headers}, now); got != tc.want {
			t.Errorf("attempt=%d headers=%v got=%s want=%s", tc.attempt, tc.headers, got, tc.want)
		}
	}
	c.Retries, c.RetryMaxElapsedTime = 5, time.Millisecond
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	if _, _, err := c.Get("/resource"); err == nil || calls != 1 {
		t.Fatalf("elapsed budget should prevent another attempt: calls=%d err=%v", calls, err)
	}
}

func TestMultipartReplayAndFileMediaType(t *testing.T) {
	file := filepath.Join(t.TempDir(), "contacts.CSV")
	content := []byte("name,phone\nSomebody,+12025550123\n")
	if err := os.WriteFile(file, content, 0600); err != nil {
		t.Fatal(err)
	}
	for _, form := range []bool{false, true} {
		c := New("https://api.example.invalid", "unit-test")
		c.Retries, c.RetryWrites, c.RetryDelay = 1, true, 0
		var bodies [][]byte
		c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
			raw, _ := io.ReadAll(r.Body)
			_ = r.Body.Close()
			bodies = append(bodies, raw)
			_, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil {
				t.Fatal(err)
			}
			reader := multipart.NewReader(bytes.NewReader(raw), params["boundary"])
			found := false
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				if part.FormName() == "file" {
					found = true
					data, _ := io.ReadAll(part)
					if part.FileName() != "contacts.CSV" || part.Header.Get("Content-Type") != "text/csv" || !bytes.Equal(data, content) {
						t.Fatalf("file upload differs: name=%s type=%s data=%q", part.FileName(), part.Header.Get("Content-Type"), data)
					}
				}
			}
			if !found {
				t.Fatal("missing multipart file")
			}
			status := 200
			if len(bodies) == 1 {
				status = 503
			}
			return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
		})
		var err error
		if form {
			_, _, err = c.PutMultipartForm("/upload", map[string]string{"empty": ""}, "file", file)
		} else {
			_, _, err = c.PostMultipart("/upload", "file", file)
		}
		if err != nil || len(bodies) != 2 || !bytes.Equal(bodies[0], bodies[1]) {
			t.Fatalf("multipart replay failed: attempts=%d err=%v", len(bodies), err)
		}
	}
}

func TestFormAndStreamRetriesUseSharedTransport(t *testing.T) {
	for _, stream := range []bool{false, true} {
		c := New("https://api.example.invalid", "unit-test")
		c.Retries, c.RetryWrites, c.RetryDelay = 1, true, 0
		calls := 0
		var requestBodies []string
		first := &closeTrackingBody{Reader: strings.NewReader(`{"detail":"temporary"}`)}
		c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
			calls++
			if r.Body != nil {
				body, _ := io.ReadAll(r.Body)
				_ = r.Body.Close()
				requestBodies = append(requestBodies, string(body))
			}
			if calls == 1 {
				return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("success"))}, nil
		})
		if stream {
			body, _, err := c.GetStream("/resource")
			if err != nil {
				t.Fatal(err)
			}
			data, _ := io.ReadAll(body)
			_ = body.Close()
			if string(data) != "success" {
				t.Fatalf("stream body changed: %q", data)
			}
		} else {
			if _, _, err := c.DeleteWithForm("/resource", map[string]string{"reason": "why & now", "empty": ""}); err != nil {
				t.Fatal(err)
			}
			if len(requestBodies) != 2 || requestBodies[0] != requestBodies[1] || requestBodies[0] != "empty=&reason=why+%26+now" {
				t.Fatalf("form body replay changed: %v", requestBodies)
			}
		}
		if calls != 2 || !first.closed {
			t.Fatalf("stream=%v calls=%d first body closed=%v", stream, calls, first.closed)
		}
	}
}

func TestTimeoutOverridesPreserveCustomClient(t *testing.T) {
	c := New("https://api.example.invalid", "unit-test")
	jar, _ := cookiejar.New(nil)
	c.HTTPClient.Jar = jar
	c.HTTPClient.Timeout = 13 * time.Second
	redirectErr := errors.New("custom redirect policy")
	c.HTTPClient.CheckRedirect = func(*http.Request, []*http.Request) error { return redirectErr }
	transport := queryPathTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	c.HTTPClient.Transport = transport
	original := c.HTTPClient
	for _, override := range []*time.Duration{nil, durationPointer(0), durationPointer(25 * time.Second)} {
		c.RequestTimeout = override
		for _, operation := range []*time.Duration{nil, durationPointer(5 * time.Minute), durationPointer(30 * time.Minute), durationPointer(45 * time.Second)} {
			copy := c.httpClientFor(operation)
			want := 13 * time.Second
			if operation != nil {
				want = *operation
			}
			if override != nil {
				want = *override
			}
			if copy.Timeout != want || copy.Jar != jar || reflect.ValueOf(copy.Transport).Pointer() != reflect.ValueOf(transport).Pointer() || copy.CheckRedirect(nil, nil) != redirectErr {
				t.Fatalf("client options were lost: timeout=%s want=%s", copy.Timeout, want)
			}
		}
		if _, _, err := c.PostWithTimeout("/resource", map[string]int{"n": 0}, 45*time.Second); err != nil {
			t.Fatal(err)
		}
		stream, _, err := c.GetStream("/resource")
		if err != nil {
			t.Fatal(err)
		}
		_ = stream.Close()
		if c.HTTPClient != original || original.Timeout != 13*time.Second {
			t.Fatal("request changed the shared HTTP client")
		}
	}
}

func durationPointer(value time.Duration) *time.Duration { return &value }

func TestActualRequestTimeoutAndHeaderOverrides(t *testing.T) {
	c := New("https://api.example.invalid/base", "unit-test")
	c.RequestTimeout, c.UserAgent = durationPointer(25*time.Millisecond), "test-user-agent"
	c.RequestAccept = func(method, path string) string {
		if method != "POST" || path != "/sample" {
			t.Fatalf("wrong Accept selector input: %s %s", method, path)
		}
		return "application/octet-stream"
	}
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 25*time.Millisecond || r.Header.Get("Accept") != "application/octet-stream" || r.Header.Get("User-Agent") != "test-user-agent" {
			t.Fatalf("request controls were not applied: deadline=%v headers=%v", deadline, r.Header)
		}
		<-r.Context().Done()
		return nil, r.Context().Err()
	})
	if _, _, err := c.PostWithTimeout("/sample", map[string]int{"n": 0}, 45*time.Second); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected explicit timeout to override operation default: %v", err)
	}
}

func TestConcurrentTimeoutRequestsDoNotMutateSharedClient(t *testing.T) {
	c := New("https://api.example.invalid", "unit-test")
	var calls atomic.Int64
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	var wait sync.WaitGroup
	for i := 0; i < 10; i++ {
		wait.Add(1)
		go func(n int) {
			defer wait.Done()
			if _, _, err := c.PostWithTimeout("/resource", map[string]int{"n": n}, time.Duration(n+1)*time.Second); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wait.Wait()
	if calls.Load() != 10 || c.HTTPClient.Timeout != 30*time.Second {
		t.Fatalf("calls=%d shared timeout=%s", calls.Load(), c.HTTPClient.Timeout)
	}
}

func TestSDKUploadMediaTypeOverrides(t *testing.T) {
	for _, tc := range []struct{ name, want string }{
		{"a.csv", "text/csv"}, {"csv", "text/csv"}, {"A.WAV", "audio/wav"}, {"script.js", "application/javascript"}, {"stdin", "application/octet-stream"},
	} {
		if got := sdkFileContentType(tc.name); got != tc.want {
			t.Errorf("%s MIME=%s want=%s", tc.name, got, tc.want)
		}
	}
	c := New("https://api.example.invalid", "unit-test")
	c.UploadFilename, c.UploadContentType = "custom.data", "application/x-custom"
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := c.createFormFile(writer, "file", "stdin")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write([]byte("fixture"))
	_ = writer.Close()
	reader := multipart.NewReader(&body, writer.Boundary())
	upload, err := reader.NextPart()
	if err != nil || upload.FileName() != "custom.data" || upload.Header.Get("Content-Type") != "application/x-custom" {
		t.Fatalf("upload overrides missing: part=%v err=%v", upload, err)
	}
}
