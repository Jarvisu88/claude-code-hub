package guard

import (
	"context"

	"github.com/ding113/claude-code-hub/internal/service/ratelimit"
	"github.com/ding113/claude-code-hub/internal/pkg/errors"
	"github.com/quagmt/udecimal"
)

// RateLimitGuard 限流守卫
// 负责检查用户级别的限流
type RateLimitGuard struct {
	rateLimitService ratelimit.Service
}

// NewRateLimitGuard 创建限流守卫
func NewRateLimitGuard(rateLimitService ratelimit.Service) *RateLimitGuard {
	return &RateLimitGuard{
		rateLimitService: rateLimitService,
	}
}

// Name 返回 Guard 名称
func (g *RateLimitGuard) Name() string {
	return "RateLimitGuard"
}

// Check 执行限流检查
func (g *RateLimitGuard) Check(ctx context.Context, req *Request) error {
	if req.User == nil {
		return nil
	}

	user := req.User

	// 1. 检查 RPM 限流
	if user.RPMLimit != nil && *user.RPMLimit > 0 {
		allowed, err := g.rateLimitService.CheckRPM(ctx, user.ID, *user.RPMLimit)
		if err != nil {
			return errors.NewInternalError("Failed to check RPM limit")
		}
		if !allowed {
			return errors.NewRateLimitExceeded(
				"RPM limit exceeded",
				errors.CodeRPMLimitExceeded,
			)
		}
	}

	// 2. 检查并发会话限流
	if user.LimitConcurrentSessions != nil && *user.LimitConcurrentSessions > 0 {
		allowed, err := g.rateLimitService.CheckConcurrentSessions(ctx, user.ID, *user.LimitConcurrentSessions)
		if err != nil {
			return errors.NewInternalError("Failed to check concurrent sessions")
		}
		if !allowed {
			return errors.NewRateLimitExceeded(
				"Concurrent sessions limit exceeded",
				errors.CodeConcurrentSessionsExceeded,
			)
		}
	}

	// 3. 检查金额限流（需要从 Context 中获取预估成本）
	if estimatedCost, ok := req.Context["estimatedCost"].(udecimal.Decimal); ok {
		// 检查 5h 限流
		if user.Limit5hUSD != nil && !user.Limit5hUSD.IsZero() {
			if err := g.checkAmountLimit(ctx, user.ID, estimatedCost, *user.Limit5hUSD, ratelimit.Period5H); err != nil {
				return err
			}
		}

		// 检查 daily 限流
		if user.DailyLimitUSD != nil && !user.DailyLimitUSD.IsZero() {
			if err := g.checkAmountLimit(ctx, user.ID, estimatedCost, *user.DailyLimitUSD, ratelimit.PeriodDaily); err != nil {
				return err
			}
		}

		// 检查 weekly 限流
		if user.LimitWeeklyUSD != nil && !user.LimitWeeklyUSD.IsZero() {
			if err := g.checkAmountLimit(ctx, user.ID, estimatedCost, *user.LimitWeeklyUSD, ratelimit.PeriodWeekly); err != nil {
				return err
			}
		}

		// 检查 monthly 限流
		if user.LimitMonthlyUSD != nil && !user.LimitMonthlyUSD.IsZero() {
			if err := g.checkAmountLimit(ctx, user.ID, estimatedCost, *user.LimitMonthlyUSD, ratelimit.PeriodMonthly); err != nil {
				return err
			}
		}

		// 检查 total 限流
		if user.LimitTotalUSD != nil && !user.LimitTotalUSD.IsZero() {
			if err := g.checkAmountLimit(ctx, user.ID, estimatedCost, *user.LimitTotalUSD, ratelimit.PeriodTotal); err != nil {
				return err
			}
		}
	}

	return nil
}

// checkAmountLimit 检查金额限流
func (g *RateLimitGuard) checkAmountLimit(ctx context.Context, userID int, estimatedCost, limit udecimal.Decimal, period ratelimit.Period) error {
	usage, err := g.rateLimitService.GetAmountUsage(ctx, userID, period)
	if err != nil {
		return errors.NewInternalError("Failed to get amount usage")
	}

	// 检查是否会超限
	if usage.Add(estimatedCost).GreaterThan(limit) {
		var code errors.ErrorCode
		switch period {
		case ratelimit.Period5H:
			code = errors.Code5HLimitExceeded
		case ratelimit.PeriodDaily:
			code = errors.CodeDailyLimitExceeded
		case ratelimit.PeriodWeekly:
			code = errors.CodeWeeklyLimitExceeded
		case ratelimit.PeriodMonthly:
			code = errors.CodeMonthlyLimitExceeded
		case ratelimit.PeriodTotal:
			code = errors.CodeTotalLimitExceeded
		default:
			code = errors.CodeRateLimitExceeded
		}

		return errors.NewRateLimitExceeded(
			string(period)+" limit exceeded",
			code,
		).WithDetails(map[string]interface{}{
			"period":        period,
			"usage":         usage.String(),
			"limit":         limit.String(),
			"estimatedCost": estimatedCost.String(),
		})
	}

	return nil
}
