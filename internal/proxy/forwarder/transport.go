package forwarder

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/rs/zerolog/log"
	"golang.org/x/net/http2"
)

// TransportConfig holds configuration for HTTP transports.
type TransportConfig struct {
	// MaxIdleConnsPerHost is the maximum idle connections per host. Default 100.
	MaxIdleConnsPerHost int

	// MaxConnsPerHost is the maximum total connections per host. Default 0 (unlimited).
	MaxConnsPerHost int

	// IdleConnTimeout is how long an idle connection is kept. Default 90s.
	IdleConnTimeout time.Duration

	// TLSHandshakeTimeout is the TLS handshake timeout. Default 10s.
	TLSHandshakeTimeout time.Duration

	// DialTimeout is the TCP dial timeout. Default 10s.
	DialTimeout time.Duration

	// DisableHTTP2 disables HTTP/2 support. Default false.
	DisableHTTP2 bool

	// InsecureSkipVerify disables TLS certificate verification. Default false (verify).
	InsecureSkipVerify bool
}

// DefaultTransportConfig returns reasonable defaults for the transport.
func DefaultTransportConfig() TransportConfig {
	return TransportConfig{
		MaxIdleConnsPerHost: 100,
		MaxConnsPerHost:     0,
		IdleConnTimeout:     90 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		DialTimeout:         10 * time.Second,
		DisableHTTP2:        false,
		InsecureSkipVerify:  false,
	}
}

// TransportManager manages HTTP transports and clients for providers.
// It creates and caches per-provider transports (when a proxy is configured)
// and shares a default transport for direct connections.
type TransportManager struct {
	mu             sync.RWMutex
	config         TransportConfig
	defaultClient  *http.Client
	proxyClients   map[int]*http.Client // keyed by provider ID
}

// NewTransportManager creates a new transport manager.
func NewTransportManager(cfg TransportConfig) *TransportManager {
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = 100
	}
	if cfg.IdleConnTimeout == 0 {
		cfg.IdleConnTimeout = 90 * time.Second
	}
	if cfg.TLSHandshakeTimeout == 0 {
		cfg.TLSHandshakeTimeout = 10 * time.Second
	}
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 10 * time.Second
	}

	tm := &TransportManager{
		config:       cfg,
		proxyClients: make(map[int]*http.Client),
	}

	// Build default transport (no proxy)
	defaultTransport := tm.buildTransport(nil)
	tm.defaultClient = &http.Client{
		Transport: defaultTransport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	return tm
}

// ClientForProvider returns an HTTP client appropriate for the provider.
// If the provider has a proxy URL configured, a dedicated client is returned.
// Otherwise the shared default client is used.
func (tm *TransportManager) ClientForProvider(p *model.Provider) *http.Client {
	if p.ProxyUrl == nil || *p.ProxyUrl == "" {
		return tm.defaultClient
	}

	tm.mu.RLock()
	client, ok := tm.proxyClients[p.ID]
	tm.mu.RUnlock()
	if ok {
		return client
	}

	// Create a new proxy client
	tm.mu.Lock()
	defer tm.mu.Unlock()

	// Double-check after acquiring write lock
	if client, ok := tm.proxyClients[p.ID]; ok {
		return client
	}

	proxyURL, err := url.Parse(*p.ProxyUrl)
	if err != nil {
		log.Warn().Err(err).Int("providerID", p.ID).Msg("invalid proxy URL, using direct connection")
		return tm.defaultClient
	}

	transport := tm.buildTransport(proxyURL)
	client = &http.Client{
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	tm.proxyClients[p.ID] = client
	return client
}

// buildTransport creates an http.Transport with the given settings.
// If proxyURL is non-nil, it is used as the proxy.
func (tm *TransportManager) buildTransport(proxyURL *url.URL) http.RoundTripper {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: tm.config.InsecureSkipVerify,
	}

	dialer := &net.Dialer{
		Timeout:   tm.config.DialTimeout,
		KeepAlive: 30 * time.Second,
	}

	t := &http.Transport{
		DialContext:           dialer.DialContext,
		TLSClientConfig:      tlsConfig,
		TLSHandshakeTimeout:  tm.config.TLSHandshakeTimeout,
		MaxIdleConnsPerHost:   tm.config.MaxIdleConnsPerHost,
		MaxConnsPerHost:       tm.config.MaxConnsPerHost,
		IdleConnTimeout:       tm.config.IdleConnTimeout,
		ResponseHeaderTimeout: 0, // We manage timeouts at a higher level
		ExpectContinueTimeout: 1 * time.Second,
		ForceAttemptHTTP2:     !tm.config.DisableHTTP2,
	}

	if proxyURL != nil {
		switch proxyURL.Scheme {
		case "socks5", "socks5h":
			// For SOCKS5, we use a custom dialer
			socks5Dialer, err := newSOCKS5Dialer(proxyURL, tm.config.DialTimeout)
			if err != nil {
				log.Warn().Err(err).Str("proxy", proxyURL.String()).Msg("failed to create SOCKS5 dialer, falling back to direct")
			} else {
				t.DialContext = socks5Dialer.DialContext
			}
		default:
			// HTTP/HTTPS proxy
			t.Proxy = http.ProxyURL(proxyURL)
		}
	}

	// Configure HTTP/2 explicitly if not disabled
	if !tm.config.DisableHTTP2 {
		if err := http2.ConfigureTransport(t); err != nil {
			log.Warn().Err(err).Msg("failed to configure HTTP/2, falling back to HTTP/1.1")
		}
	}

	return t
}

// CloseIdleConnections closes all idle connections across all transports.
func (tm *TransportManager) CloseIdleConnections() {
	tm.defaultClient.CloseIdleConnections()
	tm.mu.RLock()
	defer tm.mu.RUnlock()
	for _, client := range tm.proxyClients {
		client.CloseIdleConnections()
	}
}

// --------------------------------------------------------------------------
// SOCKS5 dialer helper
// --------------------------------------------------------------------------

type socks5Dialer struct {
	proxyAddr string
	timeout   time.Duration
	auth      *url.Userinfo
}

func newSOCKS5Dialer(proxyURL *url.URL, timeout time.Duration) (*socks5Dialer, error) {
	host := proxyURL.Hostname()
	port := proxyURL.Port()
	if port == "" {
		port = "1080"
	}
	return &socks5Dialer{
		proxyAddr: net.JoinHostPort(host, port),
		timeout:   timeout,
		auth:      proxyURL.User,
	}, nil
}

// DialContext connects through the SOCKS5 proxy.
// This is a simplified implementation - for production, consider using
// golang.org/x/net/proxy for full SOCKS5 support.
func (d *socks5Dialer) DialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	// Use a basic dialer to connect to the SOCKS5 proxy
	dialer := &net.Dialer{Timeout: d.timeout}
	conn, err := dialer.DialContext(ctx, "tcp", d.proxyAddr)
	if err != nil {
		return nil, err
	}

	// Perform SOCKS5 handshake (simplified version supporting no-auth and username/password)
	if err := socks5Handshake(conn, addr, d.auth); err != nil {
		conn.Close()
		return nil, err
	}

	return conn, nil
}

// socks5Handshake performs a minimal SOCKS5 handshake.
func socks5Handshake(conn net.Conn, targetAddr string, auth *url.Userinfo) error {
	host, port, err := net.SplitHostPort(targetAddr)
	if err != nil {
		return err
	}

	portNum, err := net.LookupPort("tcp", port)
	if err != nil {
		return err
	}

	// Greeting: version 5, 1 method (no auth or user/pass)
	methods := []byte{0x00} // no auth
	if auth != nil {
		methods = []byte{0x00, 0x02} // no auth + username/password
	}
	greeting := append([]byte{0x05, byte(len(methods))}, methods...)
	if _, err := conn.Write(greeting); err != nil {
		return err
	}

	// Read method selection
	resp := make([]byte, 2)
	if _, err := conn.Read(resp); err != nil {
		return err
	}
	if resp[0] != 0x05 {
		return net.UnknownNetworkError("socks5: invalid version in response")
	}

	// Handle username/password auth if selected
	if resp[1] == 0x02 && auth != nil {
		username := auth.Username()
		password, _ := auth.Password()
		authReq := []byte{0x01, byte(len(username))}
		authReq = append(authReq, []byte(username)...)
		authReq = append(authReq, byte(len(password)))
		authReq = append(authReq, []byte(password)...)
		if _, err := conn.Write(authReq); err != nil {
			return err
		}
		authResp := make([]byte, 2)
		if _, err := conn.Read(authResp); err != nil {
			return err
		}
		if authResp[1] != 0x00 {
			return net.UnknownNetworkError("socks5: auth failed")
		}
	} else if resp[1] != 0x00 {
		return net.UnknownNetworkError("socks5: no acceptable auth method")
	}

	// Connect request
	connectReq := []byte{0x05, 0x01, 0x00} // version, connect, reserved
	// Address type: domain name
	connectReq = append(connectReq, 0x03)                        // domain name type
	connectReq = append(connectReq, byte(len(host)))             // domain length
	connectReq = append(connectReq, []byte(host)...)             // domain
	connectReq = append(connectReq, byte(portNum>>8), byte(portNum)) // port (big-endian)

	if _, err := conn.Write(connectReq); err != nil {
		return err
	}

	// Read connect response (at least 10 bytes for IPv4)
	connectResp := make([]byte, 10)
	if _, err := conn.Read(connectResp); err != nil {
		return err
	}
	if connectResp[1] != 0x00 {
		return net.UnknownNetworkError("socks5: connect failed")
	}

	return nil
}
