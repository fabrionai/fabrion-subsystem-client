package retry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func TestConfig_Defaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("MaxAttempts = %d, want 3", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff != 100*time.Millisecond {
		t.Errorf("InitialBackoff = %v, want 100ms", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 5*time.Second {
		t.Errorf("MaxBackoff = %v, want 5s", cfg.MaxBackoff)
	}
	if cfg.Multiplier != 2.0 {
		t.Errorf("Multiplier = %v, want 2.0", cfg.Multiplier)
	}
}

func TestBackoff_ExponentialGrowth(t *testing.T) {
	cfg := &Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0, // No jitter for predictable tests
	}

	tests := []struct {
		attempt int
		want    time.Duration
	}{
		{0, 100 * time.Millisecond},  // 100ms * 2^0
		{1, 200 * time.Millisecond},  // 100ms * 2^1
		{2, 400 * time.Millisecond},  // 100ms * 2^2
		{3, 800 * time.Millisecond},  // 100ms * 2^3
		{4, 1600 * time.Millisecond}, // 100ms * 2^4
	}

	for _, tt := range tests {
		got := CalculateBackoff(cfg, tt.attempt)
		if got != tt.want {
			t.Errorf("CalculateBackoff(attempt=%d) = %v, want %v", tt.attempt, got, tt.want)
		}
	}
}

func TestBackoff_CappedAtMax(t *testing.T) {
	cfg := &Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     500 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	// Attempt 10 would be 100ms * 2^10 = 102400ms without cap
	got := CalculateBackoff(cfg, 10)
	if got != 500*time.Millisecond {
		t.Errorf("CalculateBackoff(attempt=10) = %v, want 500ms (capped)", got)
	}
}

func TestBackoff_WithJitter(t *testing.T) {
	cfg := &Config{
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.5, // 50% jitter
	}

	// With 50% jitter, value should be between 50ms and 150ms for attempt 0
	minExpected := 50 * time.Millisecond
	maxExpected := 150 * time.Millisecond

	// Run multiple times to account for randomness
	for i := 0; i < 100; i++ {
		got := CalculateBackoff(cfg, 0)
		if got < minExpected || got > maxExpected {
			t.Errorf("CalculateBackoff with jitter = %v, want between %v and %v", got, minExpected, maxExpected)
		}
	}
}

func TestRoundTripper_SuccessOnFirstAttempt(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	rt := NewRoundTripper(http.DefaultTransport, DefaultConfig())
	client := &http.Client{Transport: rt}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 1 {
		t.Errorf("attempts = %d, want 1", attempts)
	}
}

func TestRoundTripper_RetriesOn500(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("success on third attempt"))
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond, // Fast for tests
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	rt := NewRoundTripper(http.DefaultTransport, cfg)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3", attempts)
	}
}

func TestRoundTripper_RetriesOn429(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	rt := NewRoundTripper(http.DefaultTransport, cfg)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("attempts = %d, want 2", attempts)
	}
}

func TestRoundTripper_RetryBehavior(t *testing.T) {
	tests := []struct {
		name            string
		statusCode      int
		wantAttempts    int32
		wantDescription string
	}{
		{
			name:            "no retry on 400",
			statusCode:      http.StatusBadRequest,
			wantAttempts:    1,
			wantDescription: "client errors should not be retried",
		},
		{
			name:            "exhausts retries on 503",
			statusCode:      http.StatusServiceUnavailable,
			wantAttempts:    3,
			wantDescription: "server errors should exhaust all retries",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(tt.statusCode)
			}))
			defer server.Close()

			cfg := &Config{
				MaxAttempts:    3,
				InitialBackoff: 1 * time.Millisecond,
				MaxBackoff:     10 * time.Millisecond,
				Multiplier:     2.0,
				Jitter:         0,
			}

			rt := NewRoundTripper(http.DefaultTransport, cfg)
			client := &http.Client{Transport: rt}

			resp, err := client.Get(server.URL)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()

			if resp.StatusCode != tt.statusCode {
				t.Errorf("StatusCode = %d, want %d", resp.StatusCode, tt.statusCode)
			}
			if atomic.LoadInt32(&attempts) != tt.wantAttempts {
				t.Errorf("attempts = %d, want %d (%s)", attempts, tt.wantAttempts, tt.wantDescription)
			}
		})
	}
}

func TestRoundTripper_PreservesRequestBody(t *testing.T) {
	var attempts int32
	var lastBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)
		if count == 1 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts:    2,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	rt := NewRoundTripper(http.DefaultTransport, cfg)
	client := &http.Client{Transport: rt}

	reqBody := `{"key": "value"}`
	req, _ := http.NewRequest("POST", server.URL, bytes.NewBufferString(reqBody))
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if lastBody != reqBody {
		t.Errorf("body on retry = %q, want %q", lastBody, reqBody)
	}
}

func TestIsRetryableError(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name:      "nil error is not retryable",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "io.EOF is retryable",
			err:       io.EOF,
			wantRetry: true,
		},
		{
			name:      "generic error is not retryable",
			err:       errors.New("something went wrong"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryableError(tt.err)
			if got != tt.wantRetry {
				t.Errorf("IsRetryableError() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestIsRetryableStatusCode(t *testing.T) {
	tests := []struct {
		code      int
		wantRetry bool
	}{
		{429, true},  // Too Many Requests
		{500, true},  // Internal Server Error
		{502, true},  // Bad Gateway
		{503, true},  // Service Unavailable
		{504, true},  // Gateway Timeout
		{400, false}, // Bad Request
		{401, false}, // Unauthorized
		{403, false}, // Forbidden
		{404, false}, // Not Found
		{200, false}, // OK
	}

	for _, tt := range tests {
		t.Run(http.StatusText(tt.code), func(t *testing.T) {
			got := IsRetryableStatusCode(tt.code)
			if got != tt.wantRetry {
				t.Errorf("IsRetryableStatusCode(%d) = %v, want %v", tt.code, got, tt.wantRetry)
			}
		})
	}
}

func TestRoundTripper_RespectsContextCancellation(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts:    10,
		InitialBackoff: 500 * time.Millisecond, // Long backoff to ensure we cancel during sleep
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0,
	}

	rt := NewRoundTripper(http.DefaultTransport, cfg)
	client := &http.Client{Transport: rt}

	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)

	done := make(chan struct{})
	var resultErr error

	go func() {
		resp, err := client.Do(req)
		if resp != nil && resp.Body != nil {
			defer func() { _ = resp.Body.Close() }()
		}
		resultErr = err
		close(done)
	}()

	// Wait for first attempt, then cancel during backoff sleep
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Wait for result with timeout
	select {
	case <-done:
		// Request completed
	case <-time.After(2 * time.Second):
		t.Fatal("request should have been cancelled quickly, but timed out")
	}

	// Should return context error, not exhaust all retries
	if resultErr == nil {
		t.Fatal("expected error from cancelled context")
	}
	if !errors.Is(resultErr, context.Canceled) {
		t.Errorf("expected context.Canceled error, got: %v", resultErr)
	}
	// Should have stopped after first attempt (cancelled during backoff)
	if atomic.LoadInt32(&attempts) > 2 {
		t.Errorf("should stop quickly when context cancelled during backoff, got %d attempts", attempts)
	}
}

func TestRoundTripper_ResponseBodyReadableOnLastRetryableAttempt(t *testing.T) {
	const expectedBody = `{"error": "internal server error", "code": 500}`
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(expectedBody))
	}))
	defer server.Close()

	cfg := &Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	rt := NewRoundTripper(http.DefaultTransport, cfg)
	client := &http.Client{Transport: rt}

	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("attempts = %d, want 3 (should exhaust all retries)", attempts)
	}

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("StatusCode = %d, want 500", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body (bug detected): %v", err)
	}

	gotBody := string(body)
	if gotBody != expectedBody {
		t.Errorf("response body = %q, want %q", gotBody, expectedBody)
	}
}
