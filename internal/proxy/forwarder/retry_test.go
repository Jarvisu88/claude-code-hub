package forwarder

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRetryStrategy_NoRetry_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, DefaultRetryConfig())

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Result.StatusCode)
	assert.Equal(t, 1, result.TotalAttempts)
	assert.Equal(t, 0, result.ProviderSwitches)
}

func TestRetryStrategy_SameProviderRetry(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 1 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`error`))
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	cfg := RetryConfig{
		MaxSameProviderRetries: 2,
		MaxProviderSwitches:    5,
		RetryDelay:             10 * time.Millisecond,
	}
	rs := NewRetryStrategy(f, cfg)

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Result.StatusCode)
	assert.Equal(t, 2, result.TotalAttempts)
}

func TestRetryStrategy_ClientError_NoRetry(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`bad request`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, DefaultRetryConfig())

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nil)

	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, 1, result.TotalAttempts)
	assert.Equal(t, 0, result.ProviderSwitches)
}

func TestRetryStrategy_CrossProviderFallback(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`server1 error`))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`server2 ok`))
	}))
	defer server2.Close()

	provider2 := &model.Provider{ID: 2, URL: server2.URL, Key: "k"}

	f := New(nil, DefaultTimeoutConfig())
	cfg := RetryConfig{
		MaxSameProviderRetries: 0, // no same-provider retry
		MaxProviderSwitches:    5,
		RetryDelay:             10 * time.Millisecond,
	}
	rs := NewRetryStrategy(f, cfg)

	nextProvider := func(excludeIDs []int) *model.Provider {
		for _, id := range excludeIDs {
			if id == provider2.ID {
				return nil
			}
		}
		return provider2
	}

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server1.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nextProvider)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Result.StatusCode)
	assert.Equal(t, 2, result.Provider.ID)
	assert.Equal(t, 1, result.ProviderSwitches)
}

func TestRetryStrategy_RateLimit_SwitchesProvider(t *testing.T) {
	server1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`rate limited`))
	}))
	defer server1.Close()

	server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server2.Close()

	provider2 := &model.Provider{ID: 2, URL: server2.URL, Key: "k"}

	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, RetryConfig{
		MaxSameProviderRetries: 2,
		MaxProviderSwitches:    5,
		RetryDelay:             10 * time.Millisecond,
	})

	nextProvider := func(excludeIDs []int) *model.Provider {
		for _, id := range excludeIDs {
			if id == provider2.ID {
				return nil
			}
		}
		return provider2
	}

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server1.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nextProvider)

	require.NoError(t, err)
	assert.Equal(t, 2, result.Provider.ID)
}

func TestRetryStrategy_AllProvidersFail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`error`))
	}))
	defer server.Close()

	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, RetryConfig{
		MaxSameProviderRetries: 0,
		MaxProviderSwitches:    3,
		RetryDelay:             10 * time.Millisecond,
	})

	// No fallback providers
	nextProvider := func(excludeIDs []int) *model.Provider {
		return nil
	}

	_, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nextProvider)

	assert.Error(t, err)
}

func TestRetryStrategy_ProviderMaxRetryAttemptsOverride(t *testing.T) {
	var attempts atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n <= 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`ok`))
	}))
	defer server.Close()

	maxRetries := 5
	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, RetryConfig{
		MaxSameProviderRetries: 1, // Default is 1, but provider overrides to 5
		MaxProviderSwitches:    5,
		RetryDelay:             10 * time.Millisecond,
	})

	result, err := rs.ForwardWithRetry(context.Background(), &ForwardRequest{
		Provider: &model.Provider{
			ID:               1,
			URL:              server.URL,
			Key:              "k",
			MaxRetryAttempts: &maxRetries,
		},
		Method: "POST",
		Path:   "/v1/messages",
		Body:   []byte(`{}`),
	}, nil)

	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, result.Result.StatusCode)
	assert.Equal(t, 4, result.TotalAttempts)
}

func TestRetryStrategy_ContextCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	f := New(nil, DefaultTimeoutConfig())
	rs := NewRetryStrategy(f, DefaultRetryConfig())

	_, err := rs.ForwardWithRetry(ctx, &ForwardRequest{
		Provider: &model.Provider{ID: 1, URL: server.URL, Key: "k"},
		Method:   "POST",
		Path:     "/v1/messages",
		Body:     []byte(`{}`),
	}, nil)

	assert.Error(t, err)
}

func TestClassifyHTTPError(t *testing.T) {
	tests := []struct {
		status int
		want   string
	}{
		{400, "non_retryable_client"},
		{401, "non_retryable_client"},
		{403, "non_retryable_client"},
		{429, "provider"},
		{500, "provider"},
		{502, "provider"},
		{503, "provider"},
		{200, "system"},
	}
	for _, tt := range tests {
		got := ClassifyHTTPError(tt.status)
		switch tt.want {
		case "non_retryable_client":
			assert.Equal(t, got, errCategoryNonRetryableClient, "status %d", tt.status)
		case "provider":
			assert.Equal(t, got, errCategoryProvider, "status %d", tt.status)
		case "system":
			assert.Equal(t, got, errCategorySystem, "status %d", tt.status)
		}
	}
}

// error category aliases for test assertions
var (
	errCategoryNonRetryableClient = ClassifyHTTPError(400)
	errCategoryProvider           = ClassifyHTTPError(429)
	errCategorySystem             = ClassifyHTTPError(200)
)

func TestNewRetryStrategy_CapMaxSwitches(t *testing.T) {
	f := New(nil, DefaultTimeoutConfig())

	// Exceeding max should be capped
	rs := NewRetryStrategy(f, RetryConfig{
		MaxProviderSwitches: 100,
	})
	assert.Equal(t, MaxProviderSwitches, rs.config.MaxProviderSwitches)

	// Zero should use default
	rs2 := NewRetryStrategy(f, RetryConfig{
		MaxProviderSwitches: 0,
	})
	assert.Equal(t, MaxProviderSwitches, rs2.config.MaxProviderSwitches)
}
