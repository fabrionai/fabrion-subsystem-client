// Package auth provides authentication for oapi-codegen generated clients.
package auth

import (
	"errors"
	"net/http"
)

// ErrEmptyToken is returned when attempting to authenticate with an empty token.
var ErrEmptyToken = errors.New("auth: token cannot be empty")

// Authenticator adds authentication to HTTP requests.
type Authenticator interface {
	Authenticate(req *http.Request) error
}

// Location specifies where to place the API key.
type Location int

const (
	// InHeader indicates the token will be in the HTTP headers
	InHeader Location = iota
	// InQuery indicates the token will be in the query parameters
	InQuery
)

// BearerAuth implements bearer token authentication.
type BearerAuth struct {
	token string
}

// NewBearerAuth creates a new bearer token authenticator.
func NewBearerAuth(token string) *BearerAuth {
	return &BearerAuth{token: token}
}

// Authenticate adds the Authorization header with bearer token.
// Returns ErrEmptyToken if the token is empty.
func (a *BearerAuth) Authenticate(req *http.Request) error {
	if a.token == "" {
		return ErrEmptyToken
	}
	req.Header.Set("Authorization", "Bearer "+a.token)
	return nil
}

// APIKeyAuth implements API key authentication.
type APIKeyAuth struct {
	name     string
	value    string
	location Location
}

// NewAPIKeyAuth creates a new API key authenticator.
func NewAPIKeyAuth(name, value string, location Location) *APIKeyAuth {
	return &APIKeyAuth{
		name:     name,
		value:    value,
		location: location,
	}
}

// Authenticate adds the API key to the request.
func (a *APIKeyAuth) Authenticate(req *http.Request) error {
	switch a.location {
	case InHeader:
		req.Header.Set(a.name, a.value)
	case InQuery:
		q := req.URL.Query()
		q.Set(a.name, a.value)
		req.URL.RawQuery = q.Encode()
	}
	return nil
}

// noAuth is an authenticator that does nothing.
type noAuth struct{}

// NoAuth returns an authenticator that does nothing.
func NoAuth() Authenticator {
	return &noAuth{}
}

// Authenticate does nothing.
func (a *noAuth) Authenticate(_ *http.Request) error {
	return nil
}

// RoundTripper wraps an http.RoundTripper with authentication.
type RoundTripper struct {
	transport http.RoundTripper
	auth      Authenticator
}

// NewRoundTripper creates a new auth RoundTripper.
func NewRoundTripper(transport http.RoundTripper, auth Authenticator) *RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	return &RoundTripper{
		transport: transport,
		auth:      auth,
	}
}

// RoundTrip implements http.RoundTripper with authentication.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.auth != nil {
		if err := rt.auth.Authenticate(req); err != nil {
			return nil, err
		}
	}
	return rt.transport.RoundTrip(req)
}
