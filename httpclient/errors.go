// Package httpclient provides an HTTP client with authentication, retry, and middleware support.
package httpclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/fabrionai/fabrion-subsystem-client/httpclient/retry"
)

// APIError represents a structured error from an HTTP API call.
type APIError struct {
	StatusCode int    // HTTP status code
	Code       string // API-specific error code (if any)
	Message    string // Human-readable error message
	Cause      error  // Underlying error (timeout, connection, etc.)
}

// Error implements the error interface.
func (e *APIError) Error() string {
	var b strings.Builder
	b.WriteString("api error: status=")
	b.WriteString(fmt.Sprintf("%d", e.StatusCode))

	if e.Code != "" {
		b.WriteString(", code=")
		b.WriteString(e.Code)
	}

	if e.Message != "" {
		b.WriteString(", message=")
		b.WriteString(e.Message)
	}

	return b.String()
}

// Unwrap returns the underlying cause error for use with errors.Is/As.
func (e *APIError) Unwrap() error {
	return e.Cause
}

// IsRetryable determines if an error is worth retrying.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for io.EOF (connection reset)
	if errors.Is(err, io.EOF) {
		return true
	}

	// Check for network errors
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		return true
	}

	// Check for API errors with retryable status codes
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return retry.IsRetryableStatusCode(apiErr.StatusCode)
	}

	return false
}

// IsRetryableStatusCode delegates to retry.IsRetryableStatusCode for backwards compatibility.
// Deprecated: Use retry.IsRetryableStatusCode directly.
func IsRetryableStatusCode(code int) bool {
	return retry.IsRetryableStatusCode(code)
}

// ParseErrorResponse extracts error information from an HTTP response.
// It attempts to parse JSON error bodies, falling back to plain text.
func ParseErrorResponse(resp *http.Response) *APIError {
	apiErr := &APIError{
		StatusCode: resp.StatusCode,
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		apiErr.Message = http.StatusText(resp.StatusCode)
		return apiErr
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/json") {
		apiErr.Message = string(body)
		return apiErr
	}

	// Try to parse JSON error response
	if !parseJSONError(body, apiErr) {
		// Fall back to raw body if JSON parsing fails
		apiErr.Message = string(body)
	}

	return apiErr
}

// parseJSONError attempts to extract code and message from JSON error body.
// Returns true if parsing succeeded.
func parseJSONError(body []byte, apiErr *APIError) bool {
	var data map[string]any
	if err := json.Unmarshal(body, &data); err != nil {
		return false
	}

	// Try "code" field
	if code, ok := data["code"].(string); ok {
		apiErr.Code = code
	}

	// Try "message" field first, then "error" field
	if msg, ok := data["message"].(string); ok {
		apiErr.Message = msg
	} else if errVal, ok := data["error"]; ok {
		switch v := errVal.(type) {
		case string:
			apiErr.Message = v
		case map[string]any:
			// Nested error object: {"error": {"code": "...", "message": "..."}}
			if code, ok := v["code"].(string); ok {
				apiErr.Code = code
			}
			if msg, ok := v["message"].(string); ok {
				apiErr.Message = msg
			}
		}
	}

	return apiErr.Message != "" || apiErr.Code != ""
}
