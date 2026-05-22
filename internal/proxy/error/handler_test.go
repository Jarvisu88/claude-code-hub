package proxyerror

import (
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/ding113/claude-code-hub/internal/proxy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------
// HandleProxyError
// ---------------------------------------------------------------

func TestHandleProxyError_NilError(t *testing.T) {
	statusCode, body := HandleProxyError(nil, nil)
	assert.Equal(t, http.StatusInternalServerError, statusCode)
	assert.Contains(t, string(body), "Unknown error")
}

func TestHandleProxyError_GenericError(t *testing.T) {
	session := proxy.NewSession(nil)
	statusCode, body := HandleProxyError(session, errors.New("upstream failed"))
	assert.Equal(t, http.StatusBadGateway, statusCode)

	var resp ErrorResponse
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp.Type)
	assert.Contains(t, resp.Error.Message, "upstream failed")
}

func TestHandleProxyError_ClientAbort(t *testing.T) {
	session := proxy.NewSession(nil)
	statusCode, body := HandleProxyError(session, errors.New("context canceled"))
	assert.Equal(t, 499, statusCode)
	assert.Contains(t, string(body), "Client closed connection")
}

func TestHandleProxyError_RateLimit(t *testing.T) {
	session := proxy.NewSession(nil)
	statusCode, body := HandleProxyError(session, errors.New("rate limit exceeded"))
	assert.Equal(t, http.StatusTooManyRequests, statusCode)
	assert.Contains(t, string(body), "rate limit")
}

func TestHandleProxyError_TransportError(t *testing.T) {
	session := proxy.NewSession(nil)
	statusCode, body := HandleProxyError(session, errors.New("dial tcp: connection refused"))
	assert.Equal(t, http.StatusBadGateway, statusCode)
	assert.Contains(t, string(body), "Upstream connection error")
}

func TestHandleProxyError_UsesUpstreamStatusCode(t *testing.T) {
	session := proxy.NewSession(nil)
	session.SetResponse(&proxy.Response{
		StatusCode: 503,
	})
	statusCode, _ := HandleProxyError(session, errors.New("service unavailable"))
	assert.Equal(t, 503, statusCode)
}

// ---------------------------------------------------------------
// BuildUpstreamErrorResponse
// ---------------------------------------------------------------

func TestBuildUpstreamErrorResponse_WithOverrideBody(t *testing.T) {
	override := []byte(`{"error":{"type":"custom","message":"custom error"}}`)
	statusCode, body := BuildUpstreamErrorResponse(503, "ignored", "req-123", override)
	assert.Equal(t, 503, statusCode)
	assert.Equal(t, override, body)
}

func TestBuildUpstreamErrorResponse_WithMessage(t *testing.T) {
	statusCode, body := BuildUpstreamErrorResponse(500, "Internal server error", "", nil)
	assert.Equal(t, 500, statusCode)

	var resp map[string]any
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "Internal server error", errObj["message"])
}

func TestBuildUpstreamErrorResponse_WithRequestID(t *testing.T) {
	statusCode, body := BuildUpstreamErrorResponse(502, "Bad gateway", "req-abc", nil)
	assert.Equal(t, 502, statusCode)

	var resp map[string]any
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "req-abc", errObj["request_id"])
}

func TestBuildUpstreamErrorResponse_EmptyMessage(t *testing.T) {
	statusCode, body := BuildUpstreamErrorResponse(404, "", "", nil)
	assert.Equal(t, 404, statusCode)

	var resp map[string]any
	err := json.Unmarshal(body, &resp)
	require.NoError(t, err)
	errObj := resp["error"].(map[string]any)
	assert.Equal(t, "Not Found", errObj["message"]) // http.StatusText fallback
}

// ---------------------------------------------------------------
// Error categorization helpers
// ---------------------------------------------------------------

func TestIsClientAbortMessage(t *testing.T) {
	assert.True(t, isClientAbortMessage("context canceled"))
	assert.True(t, isClientAbortMessage("Context Deadline Exceeded"))
	assert.True(t, isClientAbortMessage("client disconnected"))
	assert.True(t, isClientAbortMessage("connection reset by peer"))
	assert.False(t, isClientAbortMessage("internal server error"))
}

func TestIsRateLimitMessage(t *testing.T) {
	assert.True(t, isRateLimitMessage("rate limit exceeded"))
	assert.True(t, isRateLimitMessage("too many requests"))
	assert.True(t, isRateLimitMessage("rate_limit_error"))
	assert.False(t, isRateLimitMessage("model not found"))
}

func TestIsTransportError(t *testing.T) {
	assert.True(t, isTransportError("dial tcp 10.0.0.1:443: connection refused"))
	assert.True(t, isTransportError("TLS handshake error"))
	assert.True(t, isTransportError("DNS lookup failed: no such host"))
	assert.True(t, isTransportError("read: i/o timeout"))
	assert.True(t, isTransportError("unexpected EOF"))
	assert.False(t, isTransportError("invalid api key"))
}

// ---------------------------------------------------------------
// buildErrorJSON
// ---------------------------------------------------------------

func TestBuildErrorJSON(t *testing.T) {
	data := buildErrorJSON("server_error", "Internal error occurred")
	var resp ErrorResponse
	err := json.Unmarshal(data, &resp)
	require.NoError(t, err)
	assert.Equal(t, "error", resp.Type)
	assert.Equal(t, "server_error", resp.Error.Type)
	assert.Equal(t, "Internal error occurred", resp.Error.Message)
}
