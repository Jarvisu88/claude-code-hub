package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// PermissionGuard 权限守卫
type PermissionGuard struct{}

// NewPermissionGuard 创建权限守卫
func NewPermissionGuard() *PermissionGuard {
	return &PermissionGuard{}
}

// Name 返回 Guard 名称
func (g *PermissionGuard) Name() string {
	return "PermissionGuard"
}

// Check 执行权限检查
func (g *PermissionGuard) Check(ctx context.Context, req *Request) error {
	// 1. 检查用户是否存在
	if req.User == nil {
		return errors.NewAuthenticationError("User not authenticated", errors.CodeUnauthorized)
	}

	// 2. 检查模型权限
	if req.Model != "" && len(req.User.AllowedModels) > 0 {
		allowed := false
		for _, m := range req.User.AllowedModels {
			if m == req.Model {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.NewPermissionDenied(
				"Model not allowed: "+req.Model,
				errors.CodeModelNotAllowed,
			)
		}
	}

	// 3. 检查客户端权限
	if req.ClientID != "" && len(req.User.AllowedClients) > 0 {
		allowed := false
		for _, client := range req.User.AllowedClients {
			if client == req.ClientID {
				allowed = true
				break
			}
		}
		if !allowed {
			return errors.NewPermissionDenied(
				"Client not allowed: "+req.ClientID,
				errors.CodeClientNotAllowed,
			)
		}
	}

	return nil
}
