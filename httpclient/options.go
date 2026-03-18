// Package httpclient provides an HTTP client with authentication, retry, and middleware support.
package httpclient

import (
	"net/http"
	"time"

	"github.com/fabrionai/fabrion-subsystem-client/httpclient/auth"
	"github.com/fabrionai/fabrion-subsystem-client/httpclient/retry"
)

// Config holds HTTP client configuration.
type Config struct {
	BaseURL        string
	Timeout        time.Duration
	Headers        http.Header
	HTTPClient     *http.Client
	authenticator  auth.Authenticator
	retryConfig    *retry.Config
	oidcConfig     *auth.OIDCPasswordConfig
}

// Option is a functional option for configuring the HTTP client.
type Option func(*Config)

// NewConfig creates a new Config with defaults and applies options.
func NewConfig(opts ...Option) *Config {
	cfg := &Config{
		Timeout: 30 * time.Second,
		Headers: make(http.Header),
	}

	for _, opt := range opts {
		opt(cfg)
	}

	return cfg
}

// WithBaseURL sets the base URL for all requests.
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithTimeout sets the HTTP client timeout.
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// WithHeaders sets multiple headers at once.
func WithHeaders(headers http.Header) Option {
	return func(c *Config) {
		for k, v := range headers {
			c.Headers[k] = v
		}
	}
}

// WithHeader sets a single header.
func WithHeader(key, value string) Option {
	return func(c *Config) {
		c.Headers.Set(key, value)
	}
}

// WithHTTPClient sets a custom http.Client.
func WithHTTPClient(client *http.Client) Option {
	return func(c *Config) {
		c.HTTPClient = client
	}
}
