package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/service/auth"
	appErrors "github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// AuthService 认证服务接口
type AuthService interface {
	AuthenticateProxy(ctx context.Context, input auth.ProxyAuthInput) (*auth.AuthResult, error)
}

// AuthGuard 认证守卫
// 负责验证 API Key 和用户身份
type AuthGuard struct {
	authService AuthService
}

// NewAuthGuard 创建认证守卫
func NewAuthGuard(authService AuthService) *AuthGuard {
	return &AuthGuard{
		authService: authService,
	}
}

// Name 返回 Guard 名称
func (g *AuthGuard) Name() string {
	return "AuthGuard"
}

// Check 执行认证检查
func (g *AuthGuard) Check(ctx context.Context, req *Request) error {
	// 检查 API Key 是否存在
	if req.APIKey == nil || req.APIKey.Key == "" {
		return appErrors.NewAuthenticationError(
			"未提供认证凭据。请在 Authorization 头部、x-api-key 头部或 x-goog-api-key 头部中包含 API 密钥。",
			appErrors.CodeTokenRequired,
		)
	}

	// 构造认证输入
	input := auth.ProxyAuthInput{
		AuthorizationHeader: "Bearer " + req.APIKey.Key,
		AllowSessionToken:   false,
	}

	// 调用认证服务进行验证
	// AuthService 内部会检查：
	// - API Key 是否有效
	// - API Key 是否被禁用或过期
	// - 用户是否被禁用或过期
	result, err := g.authService.AuthenticateProxy(ctx, input)
	if err != nil {
		return err
	}

	// 填充用户信息和 API Key 信息到请求上下文
	req.User = result.User
	req.APIKey = result.Key

	return nil
}
