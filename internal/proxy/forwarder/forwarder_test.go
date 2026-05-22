package forwarder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNew(t *testing.T) {
	f := New(nil, DefaultTimeoutConfig())
	assert.NotNil(t, f)
	assert.NotNil(t, f.transport)
}

func TestForward_NonStreaming_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "/v1/messages", r.URL.Path)
		assert.Equal(t, "test-key", r.Header.Get("x-api-key"))
		w.Header().Set("X-Request-Id", "req-123")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"content":"hello"}`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	result, err := f.Forward(context.Background(), &ForwardRequest{
		Provider: &model.Provider{
			ID:           1,
			URL:          server.URL,
			Key:          "test-key",
			ProviderType: "anthropic",
		},
		Method:  "POST",
		Path:    "/v1/messages",
		Headers: map[string]string{"Content-Type": "application/json"},
		Body:    []byte(`{"model":"claude-3"}`),
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.Equal(t, `{"content":"hello"}`, string(result.Body))
	assert.Equal(t, "req-123", result.Headers["X-Request-Id"])
}

func TestForward_NonStreaming_Error(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"bad request"}`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	result, err := f.Forward(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	})

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, http.StatusBadRequest, result.StatusCode)
}

func TestForward_Streaming_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 3; i++ {
			w.Write([]byte("data: chunk\n\n"))
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			time.Sleep(5 * time.Millisecond)
		}
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	result, err := f.Forward(context.Background(), &ForwardRequest{
		Provider:  &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:    "POST",
		Path:      "/v1/messages",
		Body:      []byte(`{}`),
		Streaming: true,
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
	assert.NotNil(t, result.Stream)

	chunks := 0
	for data := range result.Stream {
		if len(data) > 0 {
			chunks++
		}
	}
	assert.Greater(t, chunks, 0)
}

func TestForward_Streaming_ErrorStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error":"server error"}`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	result, err := f.Forward(context.Background(), &ForwardRequest{
		Provider:  &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:    "POST",
		Path:      "/v1/messages",
		Body:      []byte(`{}`),
		Streaming: true,
	})

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, http.StatusInternalServerError, result.StatusCode)
}

func TestForward_NilProvider(t *testing.T) {
	f := New(nil, DefaultTimeoutConfig())
	_, err := f.Forward(context.Background(), &ForwardRequest{
		Method: "POST",
		Path:   "/v1/messages",
		Body:   []byte(`{}`),
	})
	assert.Error(t, err)
}

func TestForward_AuthHeaders(t *testing.T) {
	tests := []struct {
		name         string
		providerType string
		expectHeader string
		expectValue  string
	}{
		{"anthropic", "anthropic", "X-Api-Key", "sk-test"},
		{"claude", "claude", "X-Api-Key", "sk-test"},
		{"claude-auth", "claude-auth", "X-Api-Key", "sk-test"},
		{"openai", "openai-compatible", "Authorization", "Bearer sk-test"},
		{"gemini", "gemini", "Authorization", "Bearer sk-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, tt.expectValue, r.Header.Get(tt.expectHeader))
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			f := New(nil, DefaultTimeoutConfig())
			_, _ = f.Forward(context.Background(), &ForwardRequest{
				Provider: &model.Provider{
					ID:           1,
					URL:          server.URL,
					Key:          "sk-test",
					ProviderType: tt.providerType,
				},
				Method: "POST",
				Path:   "/test",
				Body:   []byte(`{}`),
			})
		})
	}
}

func TestForward_EndpointURLOverride(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	result, err := f.Forward(context.Background(), &ForwardRequest{
		Provider:    &model.Provider{ID: 1, URL: "http://should-not-use.invalid", Key: "k"},
		Method:      "GET",
		Path:        "/health",
		Body:        nil,
		EndpointURL: server.URL,
	})

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.StatusCode)
}

func TestTimeoutConfigFromProvider(t *testing.T) {
	firstByte := 5000
	idleTimeout := 10000
	nonStreamingTimeout := 30000

	p := &model.Provider{
		FirstByteTimeoutStreamingMs:  &firstByte,
		StreamingIdleTimeoutMs:       &idleTimeout,
		RequestTimeoutNonStreamingMs: &nonStreamingTimeout,
	}

	streaming := TimeoutConfigFromProvider(p, true)
	assert.Equal(t, 5*time.Second, streaming.FirstByteTimeout)
	assert.Equal(t, 10*time.Second, streaming.IdleTimeout)

	nonStreaming := TimeoutConfigFromProvider(p, false)
	assert.Equal(t, 30*time.Second, nonStreaming.TotalTimeout)
}

func TestTimeoutConfigFromProvider_Defaults(t *testing.T) {
	p := &model.Provider{}
	cfg := TimeoutConfigFromProvider(p, true)
	defaults := DefaultTimeoutConfig()
	assert.Equal(t, defaults.FirstByteTimeout, cfg.FirstByteTimeout)
	assert.Equal(t, defaults.IdleTimeout, cfg.IdleTimeout)
}

func TestTruncateBody(t *testing.T) {
	short := "hello"
	assert.Equal(t, "hello", truncateBody([]byte(short), 100))

	long := make([]byte, 100)
	for i := range long {
		long[i] = 'a'
	}
	result := truncateBody(long, 10)
	assert.Equal(t, "aaaaaaaaaa...(truncated)", result)
}
