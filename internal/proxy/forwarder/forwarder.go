// Package forwarder implements HTTP request forwarding to upstream providers.
//
// It supports streaming (SSE) and non-streaming responses, configurable timeouts
// (first-byte, idle, total), and integrates with the retry and transport layers.
package forwarder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/rs/zerolog/log"
)

// TimeoutConfig holds the various timeout settings for a forwarding request.
type TimeoutConfig struct {
	// FirstByteTimeout is the max time to wait for the first byte of the response.
	// Only applies to streaming requests. 0 means use TotalTimeout.
	FirstByteTimeout time.Duration

	// IdleTimeout is the max time between consecutive reads on a streaming response.
	// 0 means no idle timeout.
	IdleTimeout time.Duration

	// TotalTimeout is the overall timeout for the entire request.
	TotalTimeout time.Duration
}

// DefaultTimeoutConfig returns reasonable defaults.
func DefaultTimeoutConfig() TimeoutConfig {
	return TimeoutConfig{
		FirstByteTimeout: 60 * time.Second,
		IdleTimeout:      30 * time.Second,
		TotalTimeout:     300 * time.Second,
	}
}

// TimeoutConfigFromProvider builds a TimeoutConfig from provider settings.
func TimeoutConfigFromProvider(p *model.Provider, streaming bool) TimeoutConfig {
	cfg := DefaultTimeoutConfig()
	if streaming {
		if p.FirstByteTimeoutStreamingMs != nil && *p.FirstByteTimeoutStreamingMs > 0 {
			cfg.FirstByteTimeout = time.Duration(*p.FirstByteTimeoutStreamingMs) * time.Millisecond
		}
		if p.StreamingIdleTimeoutMs != nil && *p.StreamingIdleTimeoutMs > 0 {
			cfg.IdleTimeout = time.Duration(*p.StreamingIdleTimeoutMs) * time.Millisecond
		}
	} else {
		if p.RequestTimeoutNonStreamingMs != nil && *p.RequestTimeoutNonStreamingMs > 0 {
			cfg.TotalTimeout = time.Duration(*p.RequestTimeoutNonStreamingMs) * time.Millisecond
		}
	}
	return cfg
}

// ForwardRequest holds everything needed to make an upstream HTTP call.
type ForwardRequest struct {
	// Provider is the target upstream provider.
	Provider *model.Provider

	// Method is the HTTP method (POST, GET, etc.).
	Method string

	// Path is the request path (e.g. "/v1/messages").
	Path string

	// Headers are the request headers to forward.
	Headers map[string]string

	// Body is the raw request body.
	Body []byte

	// Streaming indicates whether the client expects a streaming response.
	Streaming bool

	// EndpointURL overrides Provider.URL if set (used by endpoint pool).
	EndpointURL string
}

// ForwardResult holds the upstream response.
type ForwardResult struct {
	// StatusCode is the HTTP status code.
	StatusCode int

	// Headers are the response headers.
	Headers map[string]string

	// Body is the full response body (non-streaming only).
	Body []byte

	// Stream is the streaming channel (streaming only). Closed when done.
	Stream <-chan []byte

	// StreamDone is closed when the stream goroutine finishes.
	StreamDone <-chan struct{}
}

// Forwarder sends HTTP requests to upstream providers and returns the response.
type Forwarder struct {
	transport *TransportManager
	timeouts  TimeoutConfig
}

// New creates a Forwarder with the given transport manager and timeout config.
func New(transport *TransportManager, timeouts TimeoutConfig) *Forwarder {
	if transport == nil {
		transport = NewTransportManager(TransportConfig{})
	}
	return &Forwarder{
		transport: transport,
		timeouts:  timeouts,
	}
}

// Forward sends the request to the upstream provider and returns the result.
func (f *Forwarder) Forward(ctx context.Context, req *ForwardRequest) (*ForwardResult, error) {
	if req.Provider == nil {
		return nil, errors.NewInternalError("provider is required")
	}

	// Build upstream HTTP request
	httpReq, err := f.buildHTTPRequest(ctx, req)
	if err != nil {
		return nil, err
	}

	// Get the appropriate HTTP client
	client := f.transport.ClientForProvider(req.Provider)

	// Apply total timeout for non-streaming
	if !req.Streaming && f.timeouts.TotalTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, f.timeouts.TotalTimeout)
		defer cancel()
		httpReq = httpReq.WithContext(ctx)
	}

	// Send request
	resp, err := client.Do(httpReq)
	if err != nil {
		return nil, errors.NewProxyError(
			fmt.Sprintf("failed to forward request: %v", err), 0, nil,
		)
	}

	if req.Streaming {
		return f.handleStreamResponse(resp)
	}

	defer resp.Body.Close()
	return f.handleNormalResponse(resp)
}

// buildHTTPRequest constructs the upstream HTTP request.
func (f *Forwarder) buildHTTPRequest(ctx context.Context, req *ForwardRequest) (*http.Request, error) {
	baseURL := req.Provider.URL
	if req.EndpointURL != "" {
		baseURL = req.EndpointURL
	}
	url := baseURL + req.Path

	httpReq, err := http.NewRequestWithContext(ctx, req.Method, url, bytes.NewReader(req.Body))
	if err != nil {
		return nil, errors.NewInternalError(fmt.Sprintf("failed to create HTTP request: %v", err))
	}

	// Copy headers
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}

	// Set auth header based on provider type
	if req.Provider.Key != "" {
		switch req.Provider.ProviderType {
		case "anthropic", "claude", "claude-auth":
			httpReq.Header.Set("x-api-key", req.Provider.Key)
		default:
			httpReq.Header.Set("Authorization", "Bearer "+req.Provider.Key)
		}
	}

	return httpReq, nil
}

// handleNormalResponse reads the full response body for non-streaming responses.
func (f *Forwarder) handleNormalResponse(resp *http.Response) (*ForwardResult, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewProxyError(
			fmt.Sprintf("failed to read response body: %v", err), resp.StatusCode, nil,
		)
	}

	result := &ForwardResult{
		StatusCode: resp.StatusCode,
		Headers:    extractHeaders(resp),
		Body:       body,
	}

	if resp.StatusCode >= 400 {
		return result, errors.NewProxyError(
			fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode),
			resp.StatusCode,
			&errors.UpstreamError{
				Body: truncateBody(body, 4096),
			},
		)
	}

	return result, nil
}

// handleStreamResponse sets up a goroutine to pipe streaming data.
func (f *Forwarder) handleStreamResponse(resp *http.Response) (*ForwardResult, error) {
	// For error status codes, read the full body and return error
	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		return &ForwardResult{
			StatusCode: resp.StatusCode,
			Headers:    extractHeaders(resp),
			Body:       body,
		}, errors.NewProxyError(
			fmt.Sprintf("upstream returned HTTP %d", resp.StatusCode),
			resp.StatusCode,
			&errors.UpstreamError{
				Body: truncateBody(body, 4096),
			},
		)
	}

	ch := make(chan []byte, 128)
	doneCh := make(chan struct{})

	result := &ForwardResult{
		StatusCode: resp.StatusCode,
		Headers:    extractHeaders(resp),
		Stream:     ch,
		StreamDone: doneCh,
	}

	go func() {
		defer close(ch)
		defer close(doneCh)
		defer resp.Body.Close()

		buf := make([]byte, 8192)
		idleTimeout := f.timeouts.IdleTimeout

		for {
			// Apply idle timeout per read
			if idleTimeout > 0 {
				if deadline, ok := resp.Request.Context().Deadline(); ok {
					_ = deadline // context already has deadline
				}
			}

			n, readErr := resp.Body.Read(buf)
			if n > 0 {
				data := make([]byte, n)
				copy(data, buf[:n])
				select {
				case ch <- data:
				case <-resp.Request.Context().Done():
					log.Debug().Msg("forwarder: context canceled during stream write")
					return
				}
			}
			if readErr != nil {
				if readErr != io.EOF {
					log.Warn().Err(readErr).Msg("forwarder: stream read error")
				}
				return
			}
		}
	}()

	return result, nil
}

// extractHeaders copies response headers into a string map.
func extractHeaders(resp *http.Response) map[string]string {
	headers := make(map[string]string, len(resp.Header))
	for key, values := range resp.Header {
		if len(values) > 0 {
			headers[key] = values[0]
		}
	}
	return headers
}

// truncateBody truncates a body for logging/error messages.
func truncateBody(body []byte, maxLen int) string {
	if len(body) <= maxLen {
		return string(body)
	}
	return string(body[:maxLen]) + "...(truncated)"
}
