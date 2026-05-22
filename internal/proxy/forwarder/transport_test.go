package forwarder

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestNewTransportManager(t *testing.T) {
	tm := NewTransportManager(TransportConfig{})
	assert.NotNil(t, tm)
	assert.NotNil(t, tm.defaultClient)
}

func TestClientForProvider_Direct(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	p := &model.Provider{ID: 1}
	client := tm.ClientForProvider(p)
	assert.NotNil(t, client)
	// Should return the default client
	assert.Equal(t, tm.defaultClient, client)
}

func TestClientForProvider_WithProxy(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	proxyURL := "http://proxy.example.com:8080"
	p := &model.Provider{
		ID:       1,
		ProxyUrl: &proxyURL,
	}

	client1 := tm.ClientForProvider(p)
	assert.NotNil(t, client1)
	assert.NotEqual(t, tm.defaultClient, client1)

	// Second call should return cached client
	client2 := tm.ClientForProvider(p)
	assert.Equal(t, client1, client2)
}

func TestClientForProvider_InvalidProxy(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	badProxy := "://invalid-proxy"
	p := &model.Provider{
		ID:       1,
		ProxyUrl: &badProxy,
	}

	client := tm.ClientForProvider(p)
	assert.NotNil(t, client)
	// Should fall back to default client
	assert.Equal(t, tm.defaultClient, client)
}

func TestClientForProvider_SOCKS5Proxy(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	socksProxy := "socks5://user:pass@proxy.example.com:1080"
	p := &model.Provider{
		ID:       1,
		ProxyUrl: &socksProxy,
	}

	client := tm.ClientForProvider(p)
	assert.NotNil(t, client)
	assert.NotEqual(t, tm.defaultClient, client)
}

func TestClientForProvider_EmptyProxy(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	emptyProxy := ""
	p := &model.Provider{
		ID:       1,
		ProxyUrl: &emptyProxy,
	}

	client := tm.ClientForProvider(p)
	assert.Equal(t, tm.defaultClient, client)
}

func TestCloseIdleConnections(t *testing.T) {
	tm := NewTransportManager(DefaultTransportConfig())
	// Should not panic
	tm.CloseIdleConnections()
}

func TestDefaultTransportConfig(t *testing.T) {
	cfg := DefaultTransportConfig()
	assert.Equal(t, 100, cfg.MaxIdleConnsPerHost)
	assert.False(t, cfg.DisableHTTP2)
	assert.False(t, cfg.InsecureSkipVerify)
}
