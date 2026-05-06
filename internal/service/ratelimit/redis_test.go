package ratelimit

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/quagmt/udecimal"
	"github.com/redis/go-redis/v9"
)

// setupTestRedis 设置测试 Redis
func setupTestRedis(t *testing.T) (*RedisService, *miniredis.Miniredis) {
	mr, err := miniredis.Run()
	if err != nil {
		t.Fatalf("Failed to start miniredis: %v", err)
	}

	client := redis.NewClient(&redis.Options{
		Addr: mr.Addr(),
	})

	service := NewRedisService(client)
	return service, mr
}

// TestCheckRPM 测试 RPM 检查
func TestCheckRPM(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	limit := 10

	// 初始状态应该允许
	allowed, err := service.CheckRPM(ctx, userID, limit)
	if err != nil {
		t.Fatalf("CheckRPM() error = %v", err)
	}
	if !allowed {
		t.Error("Expected allowed = true")
	}

	// 增加计数到限制
	for i := 0; i < limit; i++ {
		service.IncrementRPM(ctx, userID)
	}

	// 应该不允许
	allowed, err = service.CheckRPM(ctx, userID, limit)
	if err != nil {
		t.Fatalf("CheckRPM() error = %v", err)
	}
	if allowed {
		t.Error("Expected allowed = false after reaching limit")
	}
}

// TestIncrementRPM 测试 RPM 增加
func TestIncrementRPM(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1

	// 增加计数
	err := service.IncrementRPM(ctx, userID)
	if err != nil {
		t.Fatalf("IncrementRPM() error = %v", err)
	}

	// 检查计数
	count, err := service.GetRPMUsage(ctx, userID)
	if err != nil {
		t.Fatalf("GetRPMUsage() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count = 1, got %d", count)
	}

	// 再次增加
	service.IncrementRPM(ctx, userID)
	count, _ = service.GetRPMUsage(ctx, userID)
	if count != 2 {
		t.Errorf("Expected count = 2, got %d", count)
	}
}

// TestRPMExpiration 测试 RPM 过期
func TestRPMExpiration(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1

	// 增加计数
	service.IncrementRPM(ctx, userID)

	// 快进 61 秒
	mr.FastForward(61 * time.Second)

	// 应该已过期
	count, err := service.GetRPMUsage(ctx, userID)
	if err != nil {
		t.Fatalf("GetRPMUsage() error = %v", err)
	}
	if count != 0 {
		t.Errorf("Expected count = 0 after expiration, got %d", count)
	}
}

// TestIncrementAmount 测试金额增加
func TestIncrementAmount(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	amount := udecimal.MustParse("10.50")

	// 增加金额
	err := service.IncrementAmount(ctx, userID, amount, PeriodDaily)
	if err != nil {
		t.Fatalf("IncrementAmount() error = %v", err)
	}

	// 检查金额
	usage, err := service.GetAmountUsage(ctx, userID, PeriodDaily)
	if err != nil {
		t.Fatalf("GetAmountUsage() error = %v", err)
	}
	if !usage.Equal(amount) {
		t.Errorf("Expected usage = %s, got %s", amount, usage)
	}

	// 再次增加
	service.IncrementAmount(ctx, userID, amount, PeriodDaily)
	usage, _ = service.GetAmountUsage(ctx, userID, PeriodDaily)
	expected := udecimal.MustParse("21.00")
	if !usage.Equal(expected) {
		t.Errorf("Expected usage = %s, got %s", expected, usage)
	}
}

// TestAcquireReleaseSession 测试会话获取和释放
func TestAcquireReleaseSession(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	sessionID := "session-123"

	// 获取会话
	err := service.AcquireSession(ctx, userID, sessionID)
	if err != nil {
		t.Fatalf("AcquireSession() error = %v", err)
	}

	// 检查会话数
	count, err := service.GetSessionCount(ctx, userID)
	if err != nil {
		t.Fatalf("GetSessionCount() error = %v", err)
	}
	if count != 1 {
		t.Errorf("Expected count = 1, got %d", count)
	}

	// 释放会话
	err = service.ReleaseSession(ctx, userID, sessionID)
	if err != nil {
		t.Fatalf("ReleaseSession() error = %v", err)
	}

	// 检查会话数
	count, _ = service.GetSessionCount(ctx, userID)
	if count != 0 {
		t.Errorf("Expected count = 0 after release, got %d", count)
	}
}

// TestCheckConcurrentSessions 测试并发会话检查
func TestCheckConcurrentSessions(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	limit := 3

	// 初始状态应该允许
	allowed, err := service.CheckConcurrentSessions(ctx, userID, limit)
	if err != nil {
		t.Fatalf("CheckConcurrentSessions() error = %v", err)
	}
	if !allowed {
		t.Error("Expected allowed = true")
	}

	// 获取 3 个会话
	for i := 0; i < limit; i++ {
		service.AcquireSession(ctx, userID, "session-"+string(rune('1'+i)))
	}

	// 应该不允许
	allowed, err = service.CheckConcurrentSessions(ctx, userID, limit)
	if err != nil {
		t.Fatalf("CheckConcurrentSessions() error = %v", err)
	}
	if allowed {
		t.Error("Expected allowed = false after reaching limit")
	}
}

// TestResetRPM 测试重置 RPM
func TestResetRPM(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1

	// 增加计数
	service.IncrementRPM(ctx, userID)
	service.IncrementRPM(ctx, userID)

	// 重置
	err := service.ResetRPM(ctx, userID)
	if err != nil {
		t.Fatalf("ResetRPM() error = %v", err)
	}

	// 检查计数
	count, _ := service.GetRPMUsage(ctx, userID)
	if count != 0 {
		t.Errorf("Expected count = 0 after reset, got %d", count)
	}
}

// TestResetAmount 测试重置金额
func TestResetAmount(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	amount := udecimal.MustParse("10.00")

	// 增加金额
	service.IncrementAmount(ctx, userID, amount, PeriodDaily)

	// 重置
	err := service.ResetAmount(ctx, userID, PeriodDaily)
	if err != nil {
		t.Fatalf("ResetAmount() error = %v", err)
	}

	// 检查金额
	usage, _ := service.GetAmountUsage(ctx, userID, PeriodDaily)
	if !usage.Equal(udecimal.Zero) {
		t.Errorf("Expected usage = 0, got %s", usage)
	}
}

// TestPeriodGetDuration 测试周期时长
func TestPeriodGetDuration(t *testing.T) {
	tests := []struct {
		period   Period
		expected time.Duration
	}{
		{Period5H, 5 * time.Hour},
		{PeriodDaily, 24 * time.Hour},
		{PeriodWeekly, 7 * 24 * time.Hour},
		{PeriodMonthly, 30 * 24 * time.Hour},
		{PeriodTotal, 0},
	}

	for _, tt := range tests {
		t.Run(string(tt.period), func(t *testing.T) {
			duration := tt.period.GetDuration()
			if duration != tt.expected {
				t.Errorf("Expected duration = %v, got %v", tt.expected, duration)
			}
		})
	}
}

// TestPeriodIsRolling 测试是否滚动窗口
func TestPeriodIsRolling(t *testing.T) {
	if !Period5H.IsRolling() {
		t.Error("Expected Period5H to be rolling")
	}

	if PeriodDaily.IsRolling() {
		t.Error("Expected PeriodDaily to not be rolling")
	}
}

// TestCheckRPMWithZeroLimit 测试零限制
func TestCheckRPMWithZeroLimit(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()
	userID := 1

	// 零限制应该总是允许
	allowed, err := service.CheckRPM(ctx, userID, 0)
	if err != nil {
		t.Fatalf("CheckRPM() error = %v", err)
	}
	if !allowed {
		t.Error("Expected allowed = true with zero limit")
	}
}

// TestMultipleUsers 测试多用户
func TestMultipleUsers(t *testing.T) {
	service, mr := setupTestRedis(t)
	defer mr.Close()

	ctx := context.Background()

	// 用户 1
	service.IncrementRPM(ctx, 1)
	service.IncrementRPM(ctx, 1)

	// 用户 2
	service.IncrementRPM(ctx, 2)

	// 检查计数
	count1, _ := service.GetRPMUsage(ctx, 1)
	count2, _ := service.GetRPMUsage(ctx, 2)

	if count1 != 2 {
		t.Errorf("User 1: expected count = 2, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("User 2: expected count = 1, got %d", count2)
	}
}
