package guard

import (
	"context"
	"errors"
	"strings"
)

// ErrWarmupIntercepted is a sentinel error indicating a warmup request was intercepted.
// This is not a failure - it signals the proxy to return a synthetic warmup response.
var ErrWarmupIntercepted = errors.New("warmup request intercepted")

// WarmupGuard intercepts Anthropic warmup requests.
type WarmupGuard struct {
	settingsRepo SystemSettingsRepo
}

// NewWarmupGuard creates a new WarmupGuard.
func NewWarmupGuard(settingsRepo SystemSettingsRepo) *WarmupGuard {
	return &WarmupGuard{settingsRepo: settingsRepo}
}

// Name returns the guard name.
func (g *WarmupGuard) Name() string {
	return "WarmupGuard"
}

// Check detects and intercepts Anthropic warmup requests.
// A warmup request targets /v1/messages, has a single message with role=user,
// text content "Warmup", and cache_control.type="ephemeral".
func (g *WarmupGuard) Check(ctx context.Context, req *Request) error {
	// Quick checks first
	if !strings.HasSuffix(req.Endpoint, "/v1/messages") && req.Endpoint != "/v1/messages" {
		return nil
	}

	if len(req.Messages) != 1 {
		return nil
	}

	msg := req.Messages[0]
	if !strings.EqualFold(msg.Role, "user") {
		return nil
	}

	// Check for warmup content
	if !isWarmupContent(msg.Content) {
		return nil
	}

	// Check if interception is enabled
	settings, err := g.settingsRepo.Get(ctx)
	if err != nil {
		// If we cannot read settings, do not intercept
		return nil
	}
	if !settings.InterceptAnthropicWarmupRequests {
		return nil
	}

	return ErrWarmupIntercepted
}

// isWarmupContent checks if the content matches the warmup pattern.
func isWarmupContent(content interface{}) bool {
	// Check for content blocks with text="Warmup" and cache_control
	blocks, ok := content.([]ContentBlock)
	if !ok {
		return false
	}

	if len(blocks) != 1 {
		return false
	}

	block := blocks[0]
	if block.Type != "text" || block.Text != "Warmup" {
		return false
	}

	// Check cache_control.type == "ephemeral"
	if block.CacheControl == nil {
		return false
	}
	return block.CacheControl["type"] == "ephemeral"
}
