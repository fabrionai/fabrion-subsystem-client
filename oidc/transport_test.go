package oidc

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestAutoRefreshTransport_InjectsToken(t *testing.T) {
	var capturedAuth string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	transport := NewAutoRefreshTransport(
		"http://unused/token",
		"client", "", "user", "pass",
		"my-token",
		http.DefaultTransport,
	)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedAuth != "Bearer my-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer my-token")
	}
}

func TestAutoRefreshTransport_RefreshesOn401(t *testing.T) {
	// Token endpoint
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "new-token",
		})
	}))
	defer tokenServer.Close()

	// API endpoint: 401 on first call, 200 on retry with new token
	var calls int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		auth := r.Header.Get("Authorization")
		if call == 1 {
			if auth != "Bearer old-token" {
				t.Errorf("first call auth = %q, want Bearer old-token", auth)
			}
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("expired"))
			return
		}
		if auth != "Bearer new-token" {
			t.Errorf("retry call auth = %q, want Bearer new-token", auth)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer apiServer.Close()

	transport := NewAutoRefreshTransport(
		tokenServer.URL,
		"client", "secret", "user", "pass",
		"old-token",
		http.DefaultTransport,
	)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", string(body), "ok")
	}
}

func TestAutoRefreshTransport_Returns401IfRefreshFails(t *testing.T) {
	// Token endpoint that always fails
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("token server down"))
	}))
	defer tokenServer.Close()

	// API endpoint that always returns 401
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("unauthorized"))
	}))
	defer apiServer.Close()

	transport := NewAutoRefreshTransport(
		tokenServer.URL,
		"client", "secret", "user", "pass",
		"bad-token",
		http.DefaultTransport,
	)

	client := &http.Client{Transport: transport}
	resp, err := client.Get(apiServer.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Should return the original 401 response when refresh fails
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("StatusCode = %d, want 401", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "unauthorized" {
		t.Errorf("body = %q, want %q", string(body), "unauthorized")
	}
}
