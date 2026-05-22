package guard

import (
	"context"
	"errors"
	"strings"
)

// ErrProbeDetected is a sentinel error indicating the request is a probe.
// This is not a failure - it signals the proxy to short-circuit with a probe response.
var ErrProbeDetected = errors.New("probe request detected")

// ProbeGuard detects probe/health-check requests and short-circuits them.
type ProbeGuard struct{}

// NewProbeGuard creates a new ProbeGuard.
func NewProbeGuard() *ProbeGuard {
	return &ProbeGuard{}
}

// Name returns the guard name.
func (g *ProbeGuard) Name() string {
	return "ProbeGuard"
}

// Check detects probe requests.
// A probe request has a single message with string content "foo" or "count"
// (case-insensitive, trimmed). Returns ErrProbeDetected sentinel.
func (g *ProbeGuard) Check(ctx context.Context, req *Request) error {
	if len(req.Messages) != 1 {
		return nil
	}

	msg := req.Messages[0]
	text, ok := msg.Content.(string)
	if !ok {
		return nil
	}

	trimmed := strings.TrimSpace(strings.ToLower(text))
	if trimmed == "foo" || trimmed == "count" {
		return ErrProbeDetected
	}

	return nil
}
