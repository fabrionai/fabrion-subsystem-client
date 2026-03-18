package httpclient

import (
	"errors"
	"io"
	"net"
	"net/http"
	"strings"
	"testing"
)

func TestAPIError_Error(t *testing.T) {
	tests := []struct {
		name    string
		err     *APIError
		wantMsg string
	}{
		{
			name: "basic error with status and message",
			err: &APIError{
				StatusCode: 404,
				Message:    "resource not found",
			},
			wantMsg: "api error: status=404, message=resource not found",
		},
		{
			name: "error with code",
			err: &APIError{
				StatusCode: 400,
				Code:       "INVALID_INPUT",
				Message:    "invalid request body",
			},
			wantMsg: "api error: status=400, code=INVALID_INPUT, message=invalid request body",
		},
		{
			name: "error with empty message",
			err: &APIError{
				StatusCode: 500,
			},
			wantMsg: "api error: status=500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
		})
	}
}

func TestAPIError_Unwrap(t *testing.T) {
	cause := errors.New("connection refused")
	apiErr := &APIError{
		StatusCode: 503,
		Message:    "service unavailable",
		Cause:      cause,
	}

	if !errors.Is(apiErr, cause) {
		t.Error("Unwrap() should return the cause error")
	}

	// Test errors.Is works through the chain
	if !errors.Is(apiErr, cause) {
		t.Error("errors.Is should find cause through Unwrap")
	}
}

func TestIsRetryable(t *testing.T) {
	tests := []struct {
		name      string
		err       error
		wantRetry bool
	}{
		{
			name:      "nil error is not retryable",
			err:       nil,
			wantRetry: false,
		},
		{
			name:      "429 Too Many Requests is retryable",
			err:       &APIError{StatusCode: 429},
			wantRetry: true,
		},
		{
			name:      "500 Internal Server Error is retryable",
			err:       &APIError{StatusCode: 500},
			wantRetry: true,
		},
		{
			name:      "502 Bad Gateway is retryable",
			err:       &APIError{StatusCode: 502},
			wantRetry: true,
		},
		{
			name:      "503 Service Unavailable is retryable",
			err:       &APIError{StatusCode: 503},
			wantRetry: true,
		},
		{
			name:      "504 Gateway Timeout is retryable",
			err:       &APIError{StatusCode: 504},
			wantRetry: true,
		},
		{
			name:      "400 Bad Request is not retryable",
			err:       &APIError{StatusCode: 400},
			wantRetry: false,
		},
		{
			name:      "401 Unauthorized is not retryable",
			err:       &APIError{StatusCode: 401},
			wantRetry: false,
		},
		{
			name:      "403 Forbidden is not retryable",
			err:       &APIError{StatusCode: 403},
			wantRetry: false,
		},
		{
			name:      "404 Not Found is not retryable",
			err:       &APIError{StatusCode: 404},
			wantRetry: false,
		},
		{
			name:      "network timeout is retryable",
			err:       &net.OpError{Op: "dial", Err: errors.New("timeout")},
			wantRetry: true,
		},
		{
			name:      "connection refused is retryable",
			err:       &net.OpError{Op: "dial", Err: errors.New("connection refused")},
			wantRetry: true,
		},
		{
			name:      "io.EOF is retryable (connection reset)",
			err:       io.EOF,
			wantRetry: true,
		},
		{
			name:      "wrapped retryable error is retryable",
			err:       &APIError{StatusCode: 503, Cause: errors.New("upstream")},
			wantRetry: true,
		},
		{
			name:      "generic error is not retryable",
			err:       errors.New("something went wrong"),
			wantRetry: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsRetryable(tt.err)
			if got != tt.wantRetry {
				t.Errorf("IsRetryable() = %v, want %v", got, tt.wantRetry)
			}
		})
	}
}

func TestParseErrorResponse(t *testing.T) {
	tests := []struct {
		name           string
		statusCode     int
		body           string
		contentType    string
		wantStatusCode int
		wantCode       string
		wantMessage    string
	}{
		{
			name:           "JSON error with code and message",
			statusCode:     400,
			body:           `{"code": "INVALID_INPUT", "message": "field 'name' is required"}`,
			contentType:    "application/json",
			wantStatusCode: 400,
			wantCode:       "INVALID_INPUT",
			wantMessage:    "field 'name' is required",
		},
		{
			name:           "JSON error with error field instead of message",
			statusCode:     500,
			body:           `{"error": "internal server error"}`,
			contentType:    "application/json",
			wantStatusCode: 500,
			wantCode:       "",
			wantMessage:    "internal server error",
		},
		{
			name:           "JSON error with nested error object",
			statusCode:     422,
			body:           `{"error": {"code": "VALIDATION_ERROR", "message": "invalid data"}}`,
			contentType:    "application/json; charset=utf-8",
			wantStatusCode: 422,
			wantCode:       "VALIDATION_ERROR",
			wantMessage:    "invalid data",
		},
		{
			name:           "plain text error",
			statusCode:     503,
			body:           "Service Unavailable",
			contentType:    "text/plain",
			wantStatusCode: 503,
			wantCode:       "",
			wantMessage:    "Service Unavailable",
		},
		{
			name:           "empty body uses status text",
			statusCode:     404,
			body:           "",
			contentType:    "application/json",
			wantStatusCode: 404,
			wantCode:       "",
			wantMessage:    "Not Found",
		},
		{
			name:           "invalid JSON falls back to raw body",
			statusCode:     500,
			body:           "not valid json {",
			contentType:    "application/json",
			wantStatusCode: 500,
			wantCode:       "",
			wantMessage:    "not valid json {",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &http.Response{
				StatusCode: tt.statusCode,
				Body:       io.NopCloser(strings.NewReader(tt.body)),
				Header:     http.Header{"Content-Type": []string{tt.contentType}},
			}

			apiErr := ParseErrorResponse(resp)

			if apiErr.StatusCode != tt.wantStatusCode {
				t.Errorf("StatusCode = %d, want %d", apiErr.StatusCode, tt.wantStatusCode)
			}
			if apiErr.Code != tt.wantCode {
				t.Errorf("Code = %q, want %q", apiErr.Code, tt.wantCode)
			}
			if apiErr.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", apiErr.Message, tt.wantMessage)
			}
		})
	}
}
