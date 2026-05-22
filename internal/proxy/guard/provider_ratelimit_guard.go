package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/service/ratelimit"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
)

// ProviderRateLimitGuard 供应商限流守卫
// 负责检查供应商级别的限流
type ProviderRateLimitGuard struct {
	rateLimitService ratelimit.Service
}

// NewProviderRateLimitGuard 创建供应商限流守卫
func NewProviderRateLimitGuard(rateLimitService ratelimit.Service) *ProviderRateLimitGuard {
	return &ProviderRateLimitGuard{
		rateLimitService: rateLimitService,
	}
}

// Name 返回 Guard 名称
func (g *ProviderRateLimitGuard) Name() string {
	return "ProviderRateLimitGuard"
}

// Check 执行供应商限流检查
func (g *ProviderRateLimitGuard) Check(ctx context.Context, req *Request) error {
	if req.Provider == nil {
		return nil
	}

	provider := req.Provider

	// 1. 检查供应商并发会话限流
	if provider.LimitConcurrentSessions != nil && *provider.LimitConcurrentSessions > 0 {
		allowed, err := g.rateLimitService.CheckConcurrentSessions(ctx, provider.ID, *provider.LimitConcurrentSessions)
		if err != nil {
			return errors.NewInternalError("Failed to check provider concurrent sessions")
		}
		if !allowed {
			return errors.NewRateLimitExceeded(
				"Provider concurrent sessions limit exceeded",
				errors.CodeRateLimitExceeded,
			)
		}
	}

	return nil
}
