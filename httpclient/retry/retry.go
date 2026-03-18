// Package retry provides config and utils for retry.
package retry

import (
	"bytes"
	"errors"
	"io"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"
)

// IsRetryableStatusCode returns true for HTTP status codes worth retrying.
func IsRetryableStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests, // 429
		http.StatusInternalServerError, // 500
		http.StatusBadGateway,          // 502
		http.StatusServiceUnavailable,  // 503
		http.StatusGatewayTimeout:      // 504
		return true
	default:
		return false
	}
}

// IsRetryableError checks if a network error is worth retrying.
// It properly identifies transient errors like timeouts, connection resets,
// and EOF while avoiding retries for permanent errors like invalid URLs
// or TLS certificate errors.
func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	// Check for io.EOF (connection reset)
	if errors.Is(err, io.EOF) {
		return true
	}

	// Check for network operation errors (timeouts, connection refused, etc.)
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}

	// Check for temporary errors
	var tempErr interface{ Temporary() bool }
	if errors.As(err, &tempErr) && tempErr.Temporary() {
		return true
	}

	return false
}

// Config holds retry configuration.
type Config struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
	Multiplier     float64
	Jitter         float64 // 0.0 to 1.0, e.g., 0.5 means +/- 50%
}

// DefaultConfig returns sensible defaults for retry configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2, // 20% jitter
	}
}

// CalculateBackoff computes the backoff duration for a given attempt.
func CalculateBackoff(cfg *Config, attempt int) time.Duration {
	// Exponential: initial * multiplier^attempt
	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.Multiplier, float64(attempt))

	// Cap at max
	if backoff > float64(cfg.MaxBackoff) {
		backoff = float64(cfg.MaxBackoff)
	}

	// Apply jitter
	if cfg.Jitter > 0 {
		jitterRange := backoff * cfg.Jitter
		// #nosec G404 -- jitter does not require cryptographically secure randomness
		backoff = backoff - jitterRange + (rand.Float64() * 2 * jitterRange)
	}

	return time.Duration(backoff)
}

// RoundTripper wraps an http.RoundTripper with retry logic.
type RoundTripper struct {
	transport http.RoundTripper
	config    *Config
}

// NewRoundTripper creates a new retry RoundTripper.
func NewRoundTripper(transport http.RoundTripper, config *Config) *RoundTripper {
	if transport == nil {
		transport = http.DefaultTransport
	}
	if config == nil {
		config = DefaultConfig()
	}
	return &RoundTripper{
		transport: transport,
		config:    config,
	}
}

// RoundTrip implements http.RoundTripper with retry logic.
func (rt *RoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	var err error

	// Buffer request body for retries if present
	if req.Body != nil && req.Body != http.NoBody {
		bodyBytes, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		defer func() { _ = req.Body.Close() }()
	}

	var resp *http.Response
	for attempt := 0; attempt < rt.config.MaxAttempts; attempt++ {
		// Restore body for this attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err = rt.transport.RoundTrip(req)

		// If request succeeded or error is not retryable, return
		if err != nil {
			if !IsRetryableError(err) || attempt == rt.config.MaxAttempts-1 {
				return nil, err
			}
		} else {
			// Check if status code is retryable
			if !IsRetryableStatusCode(resp.StatusCode) {
				return resp, nil
			}
			// On last attempt, return response even if retryable
			if attempt == rt.config.MaxAttempts-1 {
				return resp, nil
			}
			// Drain and close body before retry
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}

		// Wait before retry, respecting context cancellation
		backoff := CalculateBackoff(rt.config, attempt)
		select {
		case <-time.After(backoff):
			// Continue with next attempt
		case <-req.Context().Done():
			return nil, req.Context().Err()
		}
	}

	return resp, err
}
