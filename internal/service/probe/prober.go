package probe

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// DefaultProbeTimeout is the default timeout for a single probe request.
const DefaultProbeTimeout = 5 * time.Second

// ProbeResult holds the outcome of a single endpoint health probe.
type ProbeResult struct {
	OK         bool
	StatusCode int
	LatencyMs  int64
	ErrorType  string
	ErrorMsg   string
}

// Prober sends lightweight HTTP requests to check endpoint health.
type Prober struct {
	httpClient *http.Client
	timeout    time.Duration
}

// NewProber creates a Prober with the given timeout.
// If timeout <= 0, DefaultProbeTimeout is used.
func NewProber(timeout time.Duration) *Prober {
	if timeout <= 0 {
		timeout = DefaultProbeTimeout
	}
	return &Prober{
		httpClient: &http.Client{
			Timeout: timeout,
			// Do not follow redirects -- we want the raw status.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
		timeout: timeout,
	}
}

// Probe sends an HTTP HEAD (falling back to GET) request to the endpoint URL
// and measures the round-trip latency. It classifies errors into well-known
// categories: timeout, connection_refused, dns_error, tls_error, http_error.
func (p *Prober) Probe(ctx context.Context, endpoint *model.ProviderEndpoint) *ProbeResult {
	if endpoint == nil || endpoint.URL == "" {
		return &ProbeResult{
			OK:        false,
			ErrorType: "invalid_endpoint",
			ErrorMsg:  "endpoint or URL is empty",
		}
	}

	url := normalizeURL(endpoint.URL)

	start := time.Now()
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return &ProbeResult{
			OK:        false,
			LatencyMs: millisSince(start),
			ErrorType: "request_build_error",
			ErrorMsg:  err.Error(),
		}
	}
	req.Header.Set("User-Agent", "cch-probe/1.0")

	resp, err := p.httpClient.Do(req)
	latencyMs := millisSince(start)

	if err != nil {
		return &ProbeResult{
			OK:        false,
			LatencyMs: latencyMs,
			ErrorType: classifyError(err),
			ErrorMsg:  truncate(err.Error(), 500),
		}
	}
	defer resp.Body.Close()

	ok := resp.StatusCode >= 200 && resp.StatusCode < 500
	result := &ProbeResult{
		OK:         ok,
		StatusCode: resp.StatusCode,
		LatencyMs:  latencyMs,
	}
	if !ok {
		result.ErrorType = "http_error"
		result.ErrorMsg = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return result
}

// classifyError inspects the error chain and returns a short category string.
func classifyError(err error) string {
	if err == nil {
		return ""
	}

	// Check context deadline / cancellation first.
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	if errors.Is(err, context.Canceled) {
		return "canceled"
	}

	// Unwrap net errors.
	var netErr *net.OpError
	if errors.As(err, &netErr) {
		if netErr.Timeout() {
			return "timeout"
		}
		// DNS lookup failures.
		var dnsErr *net.DNSError
		if errors.As(err, &dnsErr) {
			return "dns_error"
		}
		// Connection refused typically surfaces as a syscall error.
		msg := netErr.Error()
		if strings.Contains(msg, "connection refused") || strings.Contains(msg, "connectex") {
			return "connection_refused"
		}
		return "connection_error"
	}

	// TLS errors.
	var tlsErr *tls.CertificateVerificationError
	if errors.As(err, &tlsErr) {
		return "tls_error"
	}
	if isTLSError(err) {
		return "tls_error"
	}

	// Timeout indicated by the net/http package.
	if isTimeoutError(err) {
		return "timeout"
	}

	return "unknown_error"
}

// isTimeoutError checks for the Timeout() interface on the error.
func isTimeoutError(err error) bool {
	type timeout interface{ Timeout() bool }
	var t timeout
	if errors.As(err, &t) {
		return t.Timeout()
	}
	return false
}

// isTLSError does a best-effort string check for TLS-related errors
// that may not be typed.
func isTLSError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "tls:") ||
		strings.Contains(msg, "x509:") ||
		strings.Contains(msg, "certificate")
}

// normalizeURL ensures the URL has a scheme.
func normalizeURL(rawURL string) string {
	if !strings.HasPrefix(rawURL, "http://") && !strings.HasPrefix(rawURL, "https://") {
		return "https://" + rawURL
	}
	return rawURL
}

func millisSince(start time.Time) int64 {
	return time.Since(start).Milliseconds()
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
