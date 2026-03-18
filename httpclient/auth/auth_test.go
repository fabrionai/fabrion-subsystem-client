package auth

import (
	"bytes"
	"io"
	"net/http"
	"testing"
)

func TestBearerAuth_Authenticate(t *testing.T) {
	token := "my-secret-token"
	a := NewBearerAuth(token)

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := req.Header.Get("Authorization")
	want := "Bearer my-secret-token"
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

func TestBearerAuth_EmptyToken(t *testing.T) {
	a := NewBearerAuth("")

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	err := a.Authenticate(req)

	// Empty token should return an error to catch configuration issues early
	if err == nil {
		t.Fatal("expected error for empty token, got nil")
	}

	// Header should not be set
	got := req.Header.Get("Authorization")
	if got != "" {
		t.Errorf("Authorization header should be empty for empty token, got %q", got)
	}
}

func TestAPIKeyAuth_Header(t *testing.T) {
	a := NewAPIKeyAuth("X-API-Key", "secret123", InHeader)

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := req.Header.Get("X-API-Key")
	if got != "secret123" {
		t.Errorf("X-API-Key header = %q, want %q", got, "secret123")
	}
}

func TestAPIKeyAuth_Query(t *testing.T) {
	a := NewAPIKeyAuth("api_key", "secret123", InQuery)

	req, _ := http.NewRequest("GET", "https://api.example.com/test?existing=param", nil)
	err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := req.URL.Query().Get("api_key")
	if got != "secret123" {
		t.Errorf("api_key query param = %q, want %q", got, "secret123")
	}

	// Verify existing params preserved
	if req.URL.Query().Get("existing") != "param" {
		t.Error("existing query param was not preserved")
	}
}

func TestAPIKeyAuth_CustomHeader(t *testing.T) {
	a := NewAPIKeyAuth("Authorization", "ApiKey my-key", InHeader)

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := req.Header.Get("Authorization")
	if got != "ApiKey my-key" {
		t.Errorf("Authorization header = %q, want %q", got, "ApiKey my-key")
	}
}

func TestNoAuth_DoesNothing(t *testing.T) {
	a := NoAuth()

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	req.Header.Set("Existing", "header")
	originalURL := req.URL.String()

	err := a.Authenticate(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Headers unchanged (except existing)
	if req.Header.Get("Authorization") != "" {
		t.Error("NoAuth should not set Authorization header")
	}
	if req.Header.Get("Existing") != "header" {
		t.Error("NoAuth should preserve existing headers")
	}
	if req.URL.String() != originalURL {
		t.Error("NoAuth should not modify URL")
	}
}

func TestAuthRoundTripper(t *testing.T) {
	token := "test-token"
	a := NewBearerAuth(token)

	// Create a mock transport that captures the request
	var capturedReq *http.Request
	mockTransport := &mockRoundTripper{
		fn: func(req *http.Request) (*http.Response, error) {
			capturedReq = req
			return &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(bytes.NewReader(nil)),
			}, nil
		},
	}

	rt := NewRoundTripper(mockTransport, a)
	client := &http.Client{Transport: rt}

	req, _ := http.NewRequest("GET", "https://api.example.com/test", nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedReq == nil {
		t.Fatal("request was not captured")
	}

	got := capturedReq.Header.Get("Authorization")
	want := "Bearer test-token"
	if got != want {
		t.Errorf("Authorization header = %q, want %q", got, want)
	}
}

// mockRoundTripper is a test helper
type mockRoundTripper struct {
	fn func(*http.Request) (*http.Response, error)
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req)
}
