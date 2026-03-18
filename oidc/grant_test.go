package oidc

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPasswordGrant_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/x-www-form-urlencoded" {
			t.Errorf("Content-Type = %q, want application/x-www-form-urlencoded", r.Header.Get("Content-Type"))
		}

		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		if r.Form.Get("grant_type") != "password" {
			t.Errorf("grant_type = %q, want password", r.Form.Get("grant_type"))
		}
		if r.Form.Get("client_id") != "my-client" {
			t.Errorf("client_id = %q, want my-client", r.Form.Get("client_id"))
		}
		if r.Form.Get("client_secret") != "my-secret" {
			t.Errorf("client_secret = %q, want my-secret", r.Form.Get("client_secret"))
		}
		if r.Form.Get("username") != "admin" {
			t.Errorf("username = %q, want admin", r.Form.Get("username"))
		}
		if r.Form.Get("password") != "admin123" {
			t.Errorf("password = %q, want admin123", r.Form.Get("password"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "test-token-123",
		})
	}))
	defer server.Close()

	token, err := PasswordGrant(context.Background(), server.URL, "my-client", "my-secret", "admin", "admin123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "test-token-123" {
		t.Errorf("token = %q, want %q", token, "test-token-123")
	}
}

func TestPasswordGrant_NoClientSecret(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Fatal(err)
		}
		// client_secret should not be present
		if r.Form.Get("client_secret") != "" {
			t.Errorf("client_secret should be empty when not provided, got %q", r.Form.Get("client_secret"))
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "public-token",
		})
	}))
	defer server.Close()

	token, err := PasswordGrant(context.Background(), server.URL, "public-client", "", "user", "pass")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if token != "public-token" {
		t.Errorf("token = %q, want %q", token, "public-token")
	}
}

func TestPasswordGrant_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "invalid_grant"}`))
	}))
	defer server.Close()

	_, err := PasswordGrant(context.Background(), server.URL, "client", "secret", "user", "wrong-pass")
	if err == nil {
		t.Fatal("expected error for 401 response")
	}
}

func TestPasswordGrant_EmptyToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"access_token": "",
		})
	}))
	defer server.Close()

	_, err := PasswordGrant(context.Background(), server.URL, "client", "secret", "user", "pass")
	if err == nil {
		t.Fatal("expected error for empty access_token")
	}
}
