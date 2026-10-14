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
	"net/http/httptest"
	"net/url"
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

type delayedRetryBody struct {
	reader    io.Reader
	delay     time.Duration
	started   chan struct{}
	finished  chan struct{}
	closed    chan struct{}
	readOnce  sync.Once
	closeOnce sync.Once
}

func newDelayedRetryBody(body string, delay time.Duration) *delayedRetryBody {
	return &delayedRetryBody{reader: strings.NewReader(body), delay: delay, started: make(chan struct{}), finished: make(chan struct{}), closed: make(chan struct{})}
}

func (b *delayedRetryBody) Read(p []byte) (int, error) {
	var stopped bool
	b.readOnce.Do(func() {
		close(b.started)
		defer close(b.finished)
		timer := time.NewTimer(b.delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-b.closed:
			stopped = true
		}
	})
	if stopped {
		return 0, io.ErrClosedPipe
	}
	return b.reader.Read(p)
}

func (b *delayedRetryBody) Close() error {
	b.closeOnce.Do(func() { close(b.closed) })
	return nil
}

func TestRetryBudgetIncludesFailedResponseDrain(t *testing.T) {
	const failure = `{"detail":"original failure"}`
	first := newDelayedRetryBody(failure, 80*time.Millisecond)
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	started := time.Now()
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 1 || err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("retry started after cleanup exhausted budget: calls=%d elapsed=%s response=%v err=%v", calls, time.Since(started), resp, err)
	}
	if elapsed := time.Since(started); elapsed >= 60*time.Millisecond {
		t.Fatalf("retry cleanup was not bounded: elapsed=%s", elapsed)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != failure {
		t.Fatalf("final error response body was corrupted: body=%q err=%v", data, err)
	}
}

func TestRetryBudgetRecheckedAfterBodyReplay(t *testing.T) {
	const failure = `{"detail":"preserve this failure"}`
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryWrites, c.RetryDelay, c.RetryMaxElapsedTime = 1, true, 0, 20*time.Millisecond
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		_ = r.Body.Close()
		status, body := 503, failure
		if calls > 1 {
			status, body = 200, `{}`
		}
		return &http.Response{StatusCode: status, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	req, _ := http.NewRequest("POST", "https://api.example.invalid/resource", strings.NewReader(`{}`))
	replayed := &closeTrackingBody{Reader: strings.NewReader(`{}`)}
	req.GetBody = func() (io.ReadCloser, error) {
		time.Sleep(80 * time.Millisecond)
		return replayed, nil
	}
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 1 || err != nil || resp == nil || resp.StatusCode != 503 || !replayed.closed {
		t.Fatalf("retry started after replay exhausted budget: calls=%d replay closed=%v response=%v err=%v", calls, replayed.closed, resp, err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != failure {
		t.Fatalf("final error response body was corrupted: body=%q err=%v", data, err)
	}
}

type delayedCloseRetryBody struct {
	io.Reader
	delay     time.Duration
	calls     atomic.Int64
	readBytes atomic.Int64
	closed    atomic.Bool
	closeDone chan struct{}
}

func (b *delayedCloseRetryBody) Read(p []byte) (int, error) {
	if b.closed.Load() {
		return 0, io.ErrClosedPipe
	}
	n, err := b.Reader.Read(p)
	b.readBytes.Add(int64(n))
	return n, err
}

func (b *delayedCloseRetryBody) Close() error {
	b.calls.Add(1)
	b.closed.Store(true)
	time.Sleep(b.delay)
	if b.closeDone != nil {
		close(b.closeDone)
	}
	return nil
}

func TestRetryCommitsBeforeRetiringIncompleteSlowCloseBody(t *testing.T) {
	failure := strings.Repeat("original failure data\n", 8*1024)
	first := &delayedCloseRetryBody{Reader: strings.NewReader(failure), delay: 80 * time.Millisecond, closeDone: make(chan struct{})}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	calls := 0
	started := time.Now()
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
		}
		if elapsed := time.Since(started); elapsed >= c.RetryMaxElapsedTime {
			t.Errorf("retry committed after budget expired: %s", elapsed)
		}
		if first.readBytes.Load() != retryDrainLimit {
			t.Errorf("cleanup exceeded cap: bytes=%d", first.readBytes.Load())
		}
		return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"ok":true}`))}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 2 || err != nil || resp == nil || resp.StatusCode != 200 || first.calls.Load() != 1 {
		t.Fatalf("committed retry returned retired failure: calls=%d closes=%d response=%v err=%v", calls, first.calls.Load(), resp, err)
	}
	select {
	case <-first.closeDone:
	default:
		t.Fatal("response retirement was not joined")
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != `{"ok":true}` {
		t.Fatalf("retry result changed: body=%q err=%v", data, err)
	}
}

func TestRetryBudgetCompleteEOFSlowClosePreservesFailure(t *testing.T) {
	const failure = `{"detail":"complete original failure"}`
	first := &delayedCloseRetryBody{Reader: strings.NewReader(failure), delay: 80 * time.Millisecond}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 1 || err != nil || resp == nil || resp.StatusCode != 503 || first.calls.Load() != 1 {
		t.Fatalf("EOF close exhausted budget but retry began: calls=%d closes=%d response=%v err=%v", calls, first.calls.Load(), resp, err)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != failure {
		t.Fatalf("complete cached error body changed: body=%q err=%v", data, err)
	}
}

func TestRetryBudgetLeavesInitialAndFinalResponseTimeoutsAlone(t *testing.T) {
	const failure = `{"detail":"slow final failure"}`
	for _, tc := range []struct {
		name    string
		retries int
		status  int
	}{
		{"no-retries", 0, 503},
		{"successful-initial-response", 1, 200},
		{"budget-spent-by-initial-response", 1, 503},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := New("https://api.example.invalid", "unit-test")
			c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = tc.retries, 0, 10*time.Millisecond
			calls := 0
			c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
				calls++
				if deadline, ok := r.Context().Deadline(); !ok || time.Until(deadline) < time.Second {
					t.Errorf("retry budget changed initial request timeout: deadline=%v", deadline)
				}
				time.Sleep(30 * time.Millisecond)
				return &http.Response{StatusCode: tc.status, Header: http.Header{}, Body: newDelayedRetryBody(failure, 30*time.Millisecond)}, nil
			})
			body, status, err := c.Get("/resource")
			var apiErr *APIError
			if calls != 1 || status != tc.status || string(body) != failure || tc.status == 200 && err != nil || tc.status == 503 && !errors.As(err, &apiErr) {
				t.Fatalf("initial/final response was cut short: calls=%d status=%d body=%q err=%v", calls, status, body, err)
			}
		})
	}
}

func TestRetryCleanupCancellationJoinsBodyReader(t *testing.T) {
	first := newDelayedRetryBody("failure", time.Hour)
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, time.Hour
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "https://api.example.invalid/resource", nil)
	result := make(chan error, 1)
	go func() { _, err := c.execute(req, nil); result <- err }()
	select {
	case <-first.started:
	case <-time.After(time.Second):
		t.Fatal("response cleanup did not begin")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation was lost: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("response cleanup did not stop after cancellation")
	}
	select {
	case <-first.finished:
	default:
		t.Fatal("response reader was not joined")
	}
	select {
	case <-first.closed:
	default:
		t.Fatal("cancelled response was not closed")
	}
}

func TestRetryBudgetReturnedBodyCloseJoinsPendingRead(t *testing.T) {
	first := newDelayedRetryBody("failure", time.Hour)
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("original failure was not returned: response=%v err=%v", resp, err)
	}
	defer resp.Body.Close()
	read := make(chan error, 1)
	go func() { _, err := io.ReadAll(resp.Body); read <- err }()
	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-first.finished:
	default:
		t.Fatal("closing returned body left the drain reader running")
	}
	select {
	case err := <-read:
		if !errors.Is(err, io.ErrClosedPipe) {
			t.Fatalf("original body read error changed: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("returned body read raced or remained blocked after Close")
	}
}

func TestRetryBudgetWithSlowHTTPErrorBody(t *testing.T) {
	const failure = `{"detail":"server failure"}`
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(503)
		w.(http.Flusher).Flush()
		select {
		case <-release:
			_, _ = io.WriteString(w, failure)
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer unblock()
	c := New(server.URL, "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 30*time.Millisecond
	req, _ := http.NewRequest("GET", server.URL+"/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls.Load() != 1 || err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("retry crossed HTTP body cleanup deadline: calls=%d response=%v err=%v", calls.Load(), resp, err)
	}
	unblock()
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != failure {
		t.Fatalf("real HTTP error body was cut short: body=%q err=%v", data, err)
	}
}

func TestRetryCleanupStopsAtByteLimitBeforeStreamingBodyStalls(t *testing.T) {
	var calls atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 128*1024))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	defer close(release)
	c := New(server.URL, "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 100*time.Millisecond
	c.HTTPClient.Timeout = 500 * time.Millisecond
	body, status, err := c.Get("/resource")
	if err != nil || status != http.StatusOK || string(body) != `{"ok":true}` || calls.Load() != 2 {
		t.Fatalf("bounded cleanup should retry without awaiting EOF: calls=%d status=%d body bytes=%d err=%v", calls.Load(), status, len(body), err)
	}
}

func TestRetryCappedFallbackReplaysPrefixAndUnbufferedRemainder(t *testing.T) {
	failure := strings.Repeat("original body with unread tail\n", 8*1024)
	counter := &delayedCloseRetryBody{Reader: strings.NewReader(failure)}
	first := newDelayedRetryBody("", 80*time.Millisecond)
	first.reader = counter
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 1 || err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("original failure not retained: calls=%d response=%v err=%v", calls, resp, err)
	}
	body, ok := resp.Body.(*retryResponseBody)
	if !ok {
		t.Fatalf("missing prefix replay body: %T", resp.Body)
	}
	select {
	case <-body.done:
	case <-time.After(time.Second):
		t.Fatal("capped reader did not finish")
	}
	if len(body.data) != retryDrainLimit || cap(body.data) != retryDrainLimit || body.complete || counter.readBytes.Load() != retryDrainLimit {
		t.Fatalf("cleanup read/buffer exceeded cap: len=%d cap=%d complete=%v reads=%d", len(body.data), cap(body.data), body.complete, counter.readBytes.Load())
	}
	data, err := io.ReadAll(body)
	if err != nil || string(data) != failure || counter.readBytes.Load() != int64(len(failure)) || cap(body.data) != retryDrainLimit {
		t.Fatalf("prefix/tail replay changed bytes or buffered tail: len=%d reads=%d cap=%d err=%v", len(data), counter.readBytes.Load(), cap(body.data), err)
	}
}

type terminalRetryReadError struct {
	data []byte
	err  error
}

func (r *terminalRetryReadError) Read(p []byte) (int, error) {
	n := copy(p, r.data)
	r.data = r.data[n:]
	if len(r.data) == 0 {
		return n, r.err
	}
	return n, nil
}

func TestRetryFallbackPreservesActualUnexpectedEOF(t *testing.T) {
	const failure = `{"detail":"valid-looking error prefix"}`
	first := newDelayedRetryBody("", 80*time.Millisecond)
	first.reader = &terminalRetryReadError{data: []byte(failure), err: io.ErrUnexpectedEOF}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil || resp == nil || resp.StatusCode != 503 {
		t.Fatalf("original response not retained: response=%v err=%v", resp, err)
	}
	data, err := io.ReadAll(resp.Body)
	if !errors.Is(err, io.ErrUnexpectedEOF) || string(data) != failure {
		t.Fatalf("transport read error was suppressed: body=%q err=%v", data, err)
	}
}

type retryPreparationJar struct {
	calls atomic.Int64
	delay time.Duration
}

func (j *retryPreparationJar) Cookies(*url.URL) []*http.Cookie {
	if j.calls.Add(1) == 2 {
		time.Sleep(j.delay)
	}
	return nil
}

func (*retryPreparationJar) SetCookies(*url.URL, []*http.Cookie) {}

func TestRetryGateExpirationPreservesOpenCappedFailure(t *testing.T) {
	failure := strings.Repeat("original incomplete failure\n", 8*1024)
	first := &delayedCloseRetryBody{Reader: strings.NewReader(failure)}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	jar := &retryPreparationJar{delay: 80 * time.Millisecond}
	c.HTTPClient.Jar = jar
	calls := 0
	c.HTTPClient.Transport = queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: first}, nil
	})
	req, _ := http.NewRequest("GET", "https://api.example.invalid/resource", nil)
	resp, err := c.execute(req, nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if calls != 1 || err != nil || resp == nil || resp.StatusCode != 503 || jar.calls.Load() != 2 || first.calls.Load() != 0 {
		t.Fatalf("pre-start expiry retired original response: calls=%d cookie calls=%d closes=%d response=%v err=%v", calls, jar.calls.Load(), first.calls.Load(), resp, err)
	}
	if first.readBytes.Load() != retryDrainLimit {
		t.Fatalf("unexpected cleanup read size: %d", first.readBytes.Load())
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil || string(data) != failure {
		t.Fatalf("pre-start expiry lost unread tail: body bytes=%d err=%v", len(data), err)
	}
}

func TestRetryAttemptRedirectKeepsTransportJarAndPolicy(t *testing.T) {
	first := &delayedCloseRetryBody{Reader: strings.NewReader(strings.Repeat("failure", 32*1024))}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 20*time.Millisecond
	jar, _ := cookiejar.New(nil)
	c.HTTPClient.Jar = jar
	redirects, calls := 0, 0
	c.HTTPClient.CheckRedirect = func(r *http.Request, via []*http.Request) error {
		redirects++
		return nil
	}
	transport := queryPathTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		switch calls {
		case 1:
			return &http.Response{StatusCode: 503, Header: http.Header{"Set-Cookie": {"initial=kept"}}, Body: first}, nil
		case 2:
			if cookie, err := r.Cookie("initial"); err != nil || cookie.Value != "kept" {
				t.Errorf("retry discarded cookie jar: cookie=%v err=%v", cookie, err)
			}
			time.Sleep(80 * time.Millisecond)
			return &http.Response{StatusCode: 302, Header: http.Header{"Location": {"/final"}, "Set-Cookie": {"redirect=kept"}}, Body: io.NopCloser(strings.NewReader("redirect"))}, nil
		case 3:
			if cookie, err := r.Cookie("redirect"); err != nil || cookie.Value != "kept" {
				t.Errorf("redirect discarded cookie jar: cookie=%v err=%v", cookie, err)
			}
			return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader("complete"))}, nil
		default:
			t.Errorf("unexpected request %d", calls)
			return nil, errors.New("unexpected request")
		}
	})
	c.HTTPClient.Transport = transport
	body, status, err := c.Get("/resource")
	if err != nil || status != 200 || string(body) != "complete" || calls != 3 || redirects != 1 || first.calls.Load() != 1 || c.HTTPClient.Jar != jar || reflect.ValueOf(c.HTTPClient.Transport).Pointer() != reflect.ValueOf(transport).Pointer() {
		t.Fatalf("redirect re-gated or lost client options: calls=%d redirects=%d status=%d body=%q err=%v", calls, redirects, status, body, err)
	}
}

func TestRetryCappedRetirementWithSingleConnection(t *testing.T) {
	var calls atomic.Int64
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write(bytes.Repeat([]byte("x"), 128*1024))
			w.(http.Flusher).Flush()
			select {
			case <-r.Context().Done():
			case <-release:
			}
			return
		}
		_, _ = io.WriteString(w, `{"ok":true}`)
	}))
	defer server.Close()
	defer close(release)
	c := New(server.URL, "unit-test")
	c.Retries, c.RetryDelay, c.RetryMaxElapsedTime = 1, 0, 100*time.Millisecond
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.MaxConnsPerHost = 1
	defer transport.CloseIdleConnections()
	c.HTTPClient.Transport, c.HTTPClient.Timeout = transport, time.Second
	body, status, err := c.Get("/resource")
	if err != nil || status != 200 || string(body) != `{"ok":true}` || calls.Load() != 2 || c.HTTPClient.Transport != transport {
		t.Fatalf("retirement deadlocked connection acquisition: calls=%d status=%d body bytes=%d err=%v", calls.Load(), status, len(body), err)
	}
}

type legacyRetryTransport struct {
	first   io.ReadCloser
	calls   atomic.Int64
	second  atomic.Pointer[http.Request]
	cancel  chan struct{}
	cancels atomic.Int64
	once    sync.Once
}

func (t *legacyRetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.calls.Add(1) == 1 {
		return &http.Response{StatusCode: 503, Header: http.Header{}, Body: t.first}, nil
	}
	t.second.Store(req)
	// This legacy transport requires CancelRequest instead of using Context.
	select {
	case <-t.cancel:
		return nil, context.DeadlineExceeded
	case <-time.After(time.Second):
		return nil, errors.New("legacy transport cancellation was not forwarded")
	}
}

func (t *legacyRetryTransport) CancelRequest(req *http.Request) {
	if req == t.second.Load() {
		t.once.Do(func() {
			t.cancels.Add(1)
			close(t.cancel)
		})
	}
}

func TestRetryGateForwardsLegacyTransportCancellation(t *testing.T) {
	first := &delayedCloseRetryBody{Reader: strings.NewReader(strings.Repeat("failure", 32*1024))}
	transport := &legacyRetryTransport{first: first, cancel: make(chan struct{})}
	c := New("https://api.example.invalid", "unit-test")
	c.Retries, c.RetryDelay = 1, 0
	c.HTTPClient.Transport, c.HTTPClient.Timeout = transport, 20*time.Millisecond
	_, _, err := c.Get("/resource")
	if !errors.Is(err, context.DeadlineExceeded) || transport.calls.Load() != 2 || transport.cancels.Load() != 1 || first.calls.Load() != 1 {
		t.Fatalf("retry lost legacy cancellation: calls=%d cancellations=%d closes=%d err=%v", transport.calls.Load(), transport.cancels.Load(), first.calls.Load(), err)
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
