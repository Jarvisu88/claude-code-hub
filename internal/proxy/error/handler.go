package proxyerror

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ding113/claude-code-hub/internal/proxy"
)

// ErrorResponse is the structured error envelope returned to the client.
// It mirrors the Anthropic API error format: {"type":"error","error":{...}}.
type ErrorResponse struct {
	Type  string          `json:"type"`
	Error ErrorResponseBody `json:"error"`
}

// ErrorResponseBody is the inner error object.
type ErrorResponseBody struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// HandleProxyError builds a structured HTTP error response from a proxy session
// and error. It categorizes the error and produces a JSON body following the
// Anthropic error format.
func HandleProxyError(session *proxy.Session, err error) (int, []byte) {
	if err == nil {
		return http.StatusInternalServerError, buildErrorJSON("internal_error", "Unknown error")
	}

	statusCode, category, message := categorizeProxyError(err)

	// If we have upstream response info, try to use that status code.
	if session != nil && session.Response != nil && session.Response.StatusCode >= 400 {
		statusCode = session.Response.StatusCode
	}

	_ = category // available for future use (e.g. metrics tagging)

	return statusCode, buildErrorJSON(string(CategorizeUpstreamStatus(statusCode)), message)
}

// categorizeProxyError extracts the status code, category, and message from an error.
func categorizeProxyError(err error) (int, Category, string) {
	errMsg := err.Error()

	// Check for client abort patterns first.
	if isClientAbortMessage(errMsg) {
		return 499, CategoryClientAbort, "Client closed connection"
	}

	// Check for rate limit patterns.
	if isRateLimitMessage(errMsg) {
		return http.StatusTooManyRequests, CategoryRateLimit, errMsg
	}

	// Check for transport/network errors.
	if isTransportError(errMsg) {
		return http.StatusBadGateway, CategoryTransportError, "Upstream connection error"
	}

	// Default to server error.
	return http.StatusBadGateway, CategoryServerError, errMsg
}

// isClientAbortMessage checks if the error message indicates a client-initiated disconnect.
func isClientAbortMessage(msg string) bool {
	lower := strings.ToLower(msg)
	patterns := []string{
		"context canceled",
		"context deadline exceeded",
		"client disconnected",
		"connection reset by peer",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isRateLimitMessage checks if the error message indicates a rate limit.
func isRateLimitMessage(msg string) bool {
	lower := strings.ToLower(msg)
	patterns := []string{
		"rate limit",
		"rate_limit",
		"too many requests",
		"429",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// isTransportError checks if the error message indicates a network/transport failure.
func isTransportError(msg string) bool {
	lower := strings.ToLower(msg)
	patterns := []string{
		"dial tcp",
		"tls handshake",
		"dns lookup",
		"no such host",
		"connection refused",
		"i/o timeout",
		"network is unreachable",
		"eof",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

// buildErrorJSON produces the standard Anthropic-format error JSON.
func buildErrorJSON(errorType, message string) []byte {
	resp := ErrorResponse{
		Type: "error",
		Error: ErrorResponseBody{
			Type:    errorType,
			Message: message,
		},
	}
	data, err := json.Marshal(resp)
	if err != nil {
		// Fallback to hand-crafted JSON.
		return []byte(`{"type":"error","error":{"type":"internal_error","message":"Failed to encode error response"}}`)
	}
	return data
}

// BuildUpstreamErrorResponse creates a structured error response from upstream
// error details. This is used when the upstream returns an error that may need
// to be transformed by error rules before being returned to the client.
func BuildUpstreamErrorResponse(
	statusCode int,
	errorMessage string,
	requestID string,
	overrideBody []byte,
) (int, []byte) {
	if len(overrideBody) > 0 {
		return statusCode, overrideBody
	}

	if errorMessage == "" {
		errorMessage = http.StatusText(statusCode)
	}

	cat := CategorizeUpstreamStatus(statusCode)
	body := buildErrorJSON(string(cat), errorMessage)

	// Inject request_id if available.
	if requestID != "" {
		var parsed map[string]any
		if json.Unmarshal(body, &parsed) == nil {
			if errObj, ok := parsed["error"].(map[string]any); ok {
				errObj["request_id"] = requestID
				if updated, err := json.Marshal(parsed); err == nil {
					body = updated
				}
			}
		}
	}

	return statusCode, body
}
