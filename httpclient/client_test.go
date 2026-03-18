package httpclient

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fabrionai/fabrion-subsystem-client/httpclient/auth"
	"github.com/fabrionai/fabrion-subsystem-client/httpclient/retry"
)

func TestClient_Do_BasicRequest(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("hello"))
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
	)

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "hello" {
		t.Errorf("body = %q, want %q", string(body), "hello")
	}
}

func TestClient_Do_WithDefaultHeaders(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithHeader("X-Custom", "value"),
		WithHeader("Accept", "application/json"),
	)

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedHeaders.Get("X-Custom") != "value" {
		t.Errorf("X-Custom header = %q, want %q", capturedHeaders.Get("X-Custom"), "value")
	}
	if capturedHeaders.Get("Accept") != "application/json" {
		t.Errorf("Accept header = %q, want %q", capturedHeaders.Get("Accept"), "application/json")
	}
}

func TestClient_Do_WithAuth(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithAuth(auth.NewBearerAuth("test-token")),
	)

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedAuth != "Bearer test-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer test-token")
	}
}

func TestClient_Do_WithRetry(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		count := atomic.AddInt32(&attempts, 1)
		if count < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryCfg := &retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	client := NewClient(
		WithBaseURL(server.URL),
		WithRetry(retryCfg),
	)

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
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

func TestClient_Do_WithAuthAndRetry(t *testing.T) {
	var attempts int32
	var lastAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lastAuth = r.Header.Get("Authorization")
		count := atomic.AddInt32(&attempts, 1)
		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	retryCfg := &retry.Config{
		MaxAttempts:    3,
		InitialBackoff: 1 * time.Millisecond,
		MaxBackoff:     10 * time.Millisecond,
		Multiplier:     2.0,
		Jitter:         0,
	}

	client := NewClient(
		WithBaseURL(server.URL),
		WithAuth(auth.NewBearerAuth("my-token")),
		WithRetry(retryCfg),
	)

	req, _ := http.NewRequest("GET", server.URL+"/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
	if lastAuth != "Bearer my-token" {
		t.Errorf("Authorization on retry = %q, want %q", lastAuth, "Bearer my-token")
	}
}

func TestClient_HTTPClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
	)

	// HTTPClient returns an *http.Client that can be used with oapi-codegen
	httpClient := client.HTTPClient()

	if httpClient == nil {
		t.Fatal("HTTPClient() returned nil")
	}

	// Should work as regular http.Client
	resp, err := httpClient.Get(server.URL + "/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}
}

func TestClient_ImplementsHTTPRequestDoer(_ *testing.T) {
	// This test verifies the Client satisfies oapi-codegen's HttpRequestDoer interface
	var _ HTTPRequestDoer = NewClient()
}
