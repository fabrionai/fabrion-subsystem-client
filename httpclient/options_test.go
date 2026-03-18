package httpclient

import (
	"net/http"
	"testing"
	"time"
)

func TestClientConfig_Defaults(t *testing.T) {
	cfg := NewConfig()

	if cfg.Timeout != 30*time.Second {
		t.Errorf("default Timeout = %v, want 30s", cfg.Timeout)
	}

	if cfg.BaseURL != "" {
		t.Errorf("default BaseURL = %q, want empty", cfg.BaseURL)
	}
}

func TestWithBaseURL(t *testing.T) {
	cfg := NewConfig(
		WithBaseURL("https://api.example.com"),
	)

	if cfg.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://api.example.com")
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := NewConfig(
		WithTimeout(60 * time.Second),
	)

	if cfg.Timeout != 60*time.Second {
		t.Errorf("Timeout = %v, want 60s", cfg.Timeout)
	}
}

func TestWithHeaders(t *testing.T) {
	const appJSON = "application/json"
	cfg := NewConfig(
		WithHeaders(http.Header{
			"X-Custom-Header": []string{"value1"},
			"Accept":          []string{appJSON},
		}),
	)

	if cfg.Headers.Get("X-Custom-Header") != "value1" {
		t.Errorf("Header X-Custom-Header = %q, want %q", cfg.Headers.Get("X-Custom-Header"), "value1")
	}
	if cfg.Headers.Get("Accept") != appJSON {
		t.Errorf("Header Accept = %q, want %q", cfg.Headers.Get("Accept"), appJSON)
	}
}

func TestWithHeader(t *testing.T) {
	cfg := NewConfig(
		WithHeader("Authorization", "Bearer token123"),
	)

	if cfg.Headers.Get("Authorization") != "Bearer token123" {
		t.Errorf("Header Authorization = %q, want %q", cfg.Headers.Get("Authorization"), "Bearer token123")
	}
}

func TestWithHTTPClient(t *testing.T) {
	customClient := &http.Client{Timeout: 120 * time.Second}

	cfg := NewConfig(
		WithHTTPClient(customClient),
	)

	if cfg.HTTPClient != customClient {
		t.Error("HTTPClient should be the custom client")
	}
}

func TestOptions_Chaining(t *testing.T) {
	appJSON := "application/json"
	cfg := NewConfig(
		WithBaseURL("https://api.example.com"),
		WithTimeout(45*time.Second),
		WithHeader("X-API-Key", "secret"),
		WithHeader("Accept", appJSON),
	)

	if cfg.BaseURL != "https://api.example.com" {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, "https://api.example.com")
	}
	if cfg.Timeout != 45*time.Second {
		t.Errorf("Timeout = %v, want 45s", cfg.Timeout)
	}
	if cfg.Headers.Get("X-API-Key") != "secret" {
		t.Errorf("Header X-API-Key = %q, want %q", cfg.Headers.Get("X-API-Key"), "secret")
	}
	if cfg.Headers.Get("Accept") != appJSON {
		t.Errorf("Header Accept = %q, want %q", cfg.Headers.Get("Accept"), appJSON)
	}
}
