// Package httpclient provides interface for oapi-codegen generated clients.
package httpclient

import (
	"net/http"

	"github.com/fabrionai/fabrion-subsystem-client/httpclient/auth"
	"github.com/fabrionai/fabrion-subsystem-client/httpclient/retry"
)

// HTTPRequestDoer is the interface that oapi-codegen generated clients expect.
// This matches github.com/oapi-codegen/oapi-codegen/v2/pkg/runtime.HttpRequestDoer
type HTTPRequestDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Client is an HTTP client with auth, retry, and default headers.
type Client struct {
	config     *Config
	httpClient *http.Client
}

// NewClient creates a new HTTP client with the given options.
func NewClient(opts ...Option) *Client {
	cfg := NewConfig(opts...)

	// Build transport chain: base → oidc → retry → auth → headers
	var transport = http.DefaultTransport

	if cfg.HTTPClient != nil && cfg.HTTPClient.Transport != nil {
		transport = cfg.HTTPClient.Transport
	}

	// Add OIDC auto-refresh transport (wraps base, handles 401 refresh)
	if cfg.oidcConfig != nil {
		oidcAuth := auth.NewOIDCPasswordAuth(*cfg.oidcConfig, transport)
		transport = oidcAuth.WrapTransport()
	}

	// Add retry middleware
	if cfg.retryConfig != nil {
		transport = retry.NewRoundTripper(transport, cfg.retryConfig)
	}

	// Add auth middleware (for non-OIDC auth like static bearer/API key)
	if cfg.authenticator != nil {
		transport = auth.NewRoundTripper(transport, cfg.authenticator)
	}

	// Add default headers middleware
	if len(cfg.Headers) > 0 {
		transport = &headerRoundTripper{
			transport: transport,
			headers:   cfg.Headers,
		}
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   cfg.Timeout,
	}

	return &Client{
		config:     cfg,
		httpClient: httpClient,
	}
}

// Do sends an HTTP request and returns an HTTP response.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	return c.httpClient.Do(req)
}

// HTTPClient returns the underlying *http.Client.
// Use this when passing to oapi-codegen generated clients.
func (c *Client) HTTPClient() *http.Client {
	return c.httpClient
}

// headerRoundTripper adds default headers to requests.
type headerRoundTripper struct {
	transport http.RoundTripper
	headers   http.Header
}

// RoundTrip implements http.RoundTripper.
func (rt *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range rt.headers {
		// Don't override headers already set on the request
		if req.Header.Get(k) == "" {
			for _, val := range v {
				req.Header.Add(k, val)
			}
		}
	}
	return rt.transport.RoundTrip(req)
}

// WithAuth adds an authenticator to the client config.
func WithAuth(a auth.Authenticator) Option {
	return func(c *Config) {
		c.authenticator = a
	}
}

// WithRetry adds retry configuration to the client config.
func WithRetry(cfg *retry.Config) Option {
	return func(c *Config) {
		c.retryConfig = cfg
	}
}

// WithOIDCPasswordAuth configures the client to use OIDC resource owner password grant
// with automatic token refresh on 401 responses. This is mutually exclusive with WithAuth —
// if both are set, OIDC handles the base auth and WithAuth wraps above it.
func WithOIDCPasswordAuth(cfg auth.OIDCPasswordConfig) Option {
	return func(c *Config) {
		c.oidcConfig = &cfg
	}
}
