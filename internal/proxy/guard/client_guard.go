package guard

import (
	"context"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// ClientGuard checks the client User-Agent against user's allowed/blocked lists.
type ClientGuard struct{}

// NewClientGuard creates a new ClientGuard.
func NewClientGuard() *ClientGuard {
	return &ClientGuard{}
}

// Name returns the guard name.
func (g *ClientGuard) Name() string {
	return "ClientGuard"
}

// Check validates the User-Agent against the user's allowedClients list.
// The User model only has AllowedClients; if empty, all clients pass.
// Matching is case-insensitive substring on the User-Agent header.
func (g *ClientGuard) Check(ctx context.Context, req *Request) error {
	if req.User == nil {
		return nil
	}

	userAgent := req.UserAgent
	if userAgent == "" {
		return nil
	}

	allowedClients := req.User.AllowedClients

	// If allowlist is empty, pass through
	if len(allowedClients) == 0 {
		return nil
	}

	// Check allowlist: User-Agent must contain at least one allowed client string
	uaLower := strings.ToLower(userAgent)
	for _, client := range allowedClients {
		if strings.Contains(uaLower, strings.ToLower(client)) {
			return nil
		}
	}

	return appErrors.NewPermissionDenied(
		"Client not allowed",
		appErrors.CodeClientNotAllowed,
	)
}
