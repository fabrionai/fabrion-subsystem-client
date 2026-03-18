package oidc

import (
	"bytes"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// AutoRefreshTransport wraps an http.RoundTripper and automatically refreshes
// the OIDC token on 401 responses.
type AutoRefreshTransport struct {
	TokenURL     string
	ClientID     string
	ClientSecret string
	User         string
	Pass         string

	mu          sync.RWMutex
	token       string
	lastRefresh time.Time

	Base http.RoundTripper
}

// NewAutoRefreshTransport creates a transport that auto-refreshes on 401.
func NewAutoRefreshTransport(tokenURL, clientID, clientSecret, user, pass, initialToken string, base http.RoundTripper) *AutoRefreshTransport {
	return &AutoRefreshTransport{
		TokenURL:     tokenURL,
		ClientID:     clientID,
		ClientSecret: clientSecret,
		User:         user,
		Pass:         pass,
		token:        initialToken,
		Base:         base,
	}
}

func (t *AutoRefreshTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.mu.RLock()
	token := t.token
	t.mu.RUnlock()

	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := t.Base.RoundTrip(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusUnauthorized {
		return resp, nil
	}

	// Read the 401 body so we can return it if refresh fails
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()

	t.mu.Lock()
	if time.Since(t.lastRefresh) < 5*time.Second {
		t.mu.Unlock()
		return t.retry(req)
	}

	slog.Info("token expired, refreshing")
	newToken, err := PasswordGrant(req.Context(), t.TokenURL, t.ClientID, t.ClientSecret, t.User, t.Pass)
	if err != nil {
		t.mu.Unlock()
		slog.Error("token refresh failed", "error", err)
		resp.Body = io.NopCloser(bytes.NewReader(body))
		return resp, nil
	}
	t.token = newToken
	t.lastRefresh = time.Now()
	t.mu.Unlock()

	slog.Info("token refreshed")
	return t.retry(req)
}

func (t *AutoRefreshTransport) retry(req *http.Request) (*http.Response, error) {
	t.mu.RLock()
	token := t.token
	t.mu.RUnlock()

	newReq := req.Clone(req.Context())
	newReq.Header.Set("Authorization", "Bearer "+token)
	return t.Base.RoundTrip(newReq)
}
