package auth

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestOIDCPasswordAuth_ImplementsAuthenticator(t *testing.T) {
	// Verify the interface is satisfied
	var _ Authenticator = &OIDCPasswordAuth{}
}

func TestOIDCPasswordAuth_WrapTransport(t *testing.T) {
	// Set up a token server that returns a valid token
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "fresh-token",
		})
	}))
	defer tokenServer.Close()

	// Set up an API server that checks for the bearer token
	var capturedAuth string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	defer apiServer.Close()

	oidcAuth := NewOIDCPasswordAuth(OIDCPasswordConfig{
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Username:     "admin",
		Password:     "admin",
		InitialToken: "initial-token",
	}, http.DefaultTransport)

	// The transport should inject the bearer token
	client := &http.Client{Transport: oidcAuth.WrapTransport()}

	resp, err := client.Get(apiServer.URL + "/api/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if capturedAuth != "Bearer initial-token" {
		t.Errorf("Authorization = %q, want %q", capturedAuth, "Bearer initial-token")
	}
}

func TestOIDCPasswordAuth_RefreshesOn401(t *testing.T) {
	// Token server
	tokenServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		t.Logf("token request body: %s", string(body))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "refreshed-token",
		})
	}))
	defer tokenServer.Close()

	// API server: first call returns 401, second call (after refresh) succeeds
	var calls int32
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		call := atomic.AddInt32(&calls, 1)
		auth := r.Header.Get("Authorization")
		if call == 1 && auth == "Bearer expired-token" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte("token expired"))
			return
		}
		if auth == "Bearer refreshed-token" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("success"))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer apiServer.Close()

	oidcAuth := NewOIDCPasswordAuth(OIDCPasswordConfig{
		TokenURL:     tokenServer.URL,
		ClientID:     "test-client",
		ClientSecret: "test-secret",
		Username:     "admin",
		Password:     "admin",
		InitialToken: "expired-token",
	}, http.DefaultTransport)

	client := &http.Client{Transport: oidcAuth.WrapTransport()}

	resp, err := client.Get(apiServer.URL + "/api/test")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("StatusCode = %d, want 200 (should have refreshed and retried)", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Errorf("body = %q, want %q", string(body), "success")
	}
}

func TestOIDCPasswordAuth_Authenticate_IsNoop(t *testing.T) {
	oidcAuth := NewOIDCPasswordAuth(OIDCPasswordConfig{
		TokenURL: "http://localhost/token",
		ClientID: "test",
		Username: "user",
		Password: "pass",
	}, http.DefaultTransport)

	req, _ := http.NewRequest("GET", "http://example.com/test", nil)
	err := oidcAuth.Authenticate(req)
	if err != nil {
		t.Fatalf("Authenticate should be a no-op, got error: %v", err)
	}

	// Should not have modified the request
	if req.Header.Get("Authorization") != "" {
		t.Error("Authenticate should not set Authorization header (transport handles it)")
	}
}
