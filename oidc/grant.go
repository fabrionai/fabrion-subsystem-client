package oidc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

type tokenResponse struct {
	AccessToken string `json:"access_token"`
}

// PasswordGrant performs an OIDC resource owner password grant and returns the access token.
func PasswordGrant(ctx context.Context, tokenURL, clientID, clientSecret, user, pass string) (string, error) {
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {clientID},
		"username":   {user},
		"password":   {pass},
	}
	if clientSecret != "" {
		form.Set("client_secret", clientSecret)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, bytes.NewReader([]byte(form.Encode())))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("token request failed (status %d): %s", resp.StatusCode, string(body))
	}
	var tok tokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("returned empty access_token")
	}
	return tok.AccessToken, nil
}
