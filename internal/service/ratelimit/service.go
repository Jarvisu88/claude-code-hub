package ratelimit

import (
	"context"
	"time"

	"github.com/quagmt/udecimal"
)

// Service 限流服务接口
type Service interface {
	// CheckRPM 检查 RPM 限流
	CheckRPM(ctx context.Context, userID int, limit int) (bool, error)

	// CheckAmount 检查金额限流
	CheckAmount(ctx context.Context, userID int, amount udecimal.Decimal, period Period) (bool, error)

	// CheckConcurrentSessions 检查并发会话限流
	CheckConcurrentSessions(ctx context.Context, userID int, limit int) (bool, error)

	// IncrementRPM 增加 RPM 计数
	IncrementRPM(ctx context.Context, userID int) error

	// IncrementAmount 增加金额计数
	IncrementAmount(ctx context.Context, userID int, amount udecimal.Decimal, period Period) error

	// AcquireSession 获取会话
	AcquireSession(ctx context.Context, userID int, sessionID string) error

	// ReleaseSession 释放会话
	ReleaseSession(ctx context.Context, userID int, sessionID string) error

	// GetRPMUsage 获取 RPM 使用情况
	GetRPMUsage(ctx context.Context, userID int) (int, error)

	// GetAmountUsage 获取金额使用情况
	GetAmountUsage(ctx context.Context, userID int, period Period) (udecimal.Decimal, error)

	// GetSessionCount 获取会话数量
	GetSessionCount(ctx context.Context, userID int) (int, error)

	// ResetRPM 重置 RPM 计数
	ResetRPM(ctx context.Context, userID int) error

	// ResetAmount 重置金额计数
	ResetAmount(ctx context.Context, userID int, period Period) error
}

// Period 时间周期
type Period string

const (
	Period5H      Period = "5h"
	PeriodDaily   Period = "daily"
	PeriodWeekly  Period = "weekly"
	PeriodMonthly Period = "monthly"
	PeriodTotal   Period = "total"
)

// GetDuration 获取周期时长
func (p Period) GetDuration() time.Duration {
	switch p {
	case Period5H:
		return 5 * time.Hour
	case PeriodDaily:
		return 24 * time.Hour
	case PeriodWeekly:
		return 7 * 24 * time.Hour
	case PeriodMonthly:
		return 30 * 24 * time.Hour
	case PeriodTotal:
		return 0 // 永久
	default:
		return 0
	}
}

// IsRolling 是否是滚动窗口
func (p Period) IsRolling() bool {
	return p == Period5H
}

// Result 限流检查结果
type Result struct {
	// Allowed 是否允许
	Allowed bool

	// Remaining 剩余配额
	Remaining int

	// ResetAt 重置时间
	ResetAt time.Time

	// RetryAfter 重试等待时间（秒）
	RetryAfter int
}
