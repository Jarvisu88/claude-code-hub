package guard

import (
	"context"
	"strings"

	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// ModelGuard checks the requested model against the user's allowed models.
type ModelGuard struct{}

// NewModelGuard creates a new ModelGuard.
func NewModelGuard() *ModelGuard {
	return &ModelGuard{}
}

// Name returns the guard name.
func (g *ModelGuard) Name() string {
	return "ModelGuard"
}

// Check validates the requested model against allowedModels.
// If allowedModels is empty, all models pass. Case-insensitive exact match.
func (g *ModelGuard) Check(ctx context.Context, req *Request) error {
	if req.User == nil || req.Model == "" {
		return nil
	}

	allowedModels := req.User.AllowedModels
	if len(allowedModels) == 0 {
		return nil
	}

	reqModelLower := strings.ToLower(req.Model)
	for _, m := range allowedModels {
		if strings.ToLower(m) == reqModelLower {
			return nil
		}
	}

	return appErrors.NewPermissionDenied(
		"Model not allowed: "+req.Model,
		appErrors.CodeModelNotAllowed,
	)
}
