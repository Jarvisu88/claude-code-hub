package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/quagmt/udecimal"
	"github.com/redis/go-redis/v9"
)

// RedisService Redis 实现的限流服务
type RedisService struct {
	client *redis.Client
}

// NewRedisService 创建 Redis 限流服务
func NewRedisService(client *redis.Client) *RedisService {
	return &RedisService{
		client: client,
	}
}

// CheckRPM 检查 RPM 限流
func (s *RedisService) CheckRPM(ctx context.Context, userID int, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	key := fmt.Sprintf("ratelimit:rpm:%d", userID)
	count, err := s.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	return count < limit, nil
}

// IncrementRPM 增加 RPM 计数
func (s *RedisService) IncrementRPM(ctx context.Context, userID int) error {
	key := fmt.Sprintf("ratelimit:rpm:%d", userID)

	pipe := s.client.Pipeline()
	pipe.Incr(ctx, key)
	pipe.Expire(ctx, key, 60*time.Second)
	_, err := pipe.Exec(ctx)

	return err
}

// CheckAmount 检查金额限流
func (s *RedisService) CheckAmount(ctx context.Context, userID int, amount udecimal.Decimal, period Period) (bool, error) {
	// 获取当前使用量
	_, err := s.GetAmountUsage(ctx, userID, period)
	if err != nil {
		return false, err
	}

	// 检查是否超限（这里需要外部传入 limit，暂时返回 true）
	// 实际使用时需要从用户配置中获取 limit
	return true, nil
}

// IncrementAmount 增加金额计数
func (s *RedisService) IncrementAmount(ctx context.Context, userID int, amount udecimal.Decimal, period Period) error {
	key := s.getAmountKey(userID, period)

	// 使用 INCRBYFLOAT 增加金额
	amountStr := amount.String()
	var amountFloat float64
	fmt.Sscanf(amountStr, "%f", &amountFloat)

	err := s.client.IncrByFloat(ctx, key, amountFloat).Err()
	if err != nil {
		return err
	}

	// 设置过期时间（除了 total）
	if period != PeriodTotal {
		duration := period.GetDuration()
		if duration > 0 {
			s.client.Expire(ctx, key, duration)
		}
	}

	return nil
}

// CheckConcurrentSessions 检查并发会话限流
func (s *RedisService) CheckConcurrentSessions(ctx context.Context, userID int, limit int) (bool, error) {
	if limit <= 0 {
		return true, nil
	}

	count, err := s.GetSessionCount(ctx, userID)
	if err != nil {
		return false, err
	}

	return count < limit, nil
}

// AcquireSession 获取会话
func (s *RedisService) AcquireSession(ctx context.Context, userID int, sessionID string) error {
	key := fmt.Sprintf("ratelimit:sessions:%d", userID)

	// 添加到集合，设置 1 小时过期
	err := s.client.SAdd(ctx, key, sessionID).Err()
	if err != nil {
		return err
	}

	// 设置过期时间
	return s.client.Expire(ctx, key, 1*time.Hour).Err()
}

// ReleaseSession 释放会话
func (s *RedisService) ReleaseSession(ctx context.Context, userID int, sessionID string) error {
	key := fmt.Sprintf("ratelimit:sessions:%d", userID)
	return s.client.SRem(ctx, key, sessionID).Err()
}

// GetRPMUsage 获取 RPM 使用情况
func (s *RedisService) GetRPMUsage(ctx context.Context, userID int) (int, error) {
	key := fmt.Sprintf("ratelimit:rpm:%d", userID)
	count, err := s.client.Get(ctx, key).Int()
	if err == redis.Nil {
		return 0, nil
	}
	return count, err
}

// GetAmountUsage 获取金额使用情况
func (s *RedisService) GetAmountUsage(ctx context.Context, userID int, period Period) (udecimal.Decimal, error) {
	key := s.getAmountKey(userID, period)
	amount, err := s.client.Get(ctx, key).Float64()
	if err == redis.Nil {
		return udecimal.Zero, nil
	}
	if err != nil {
		return udecimal.Zero, err
	}

	return udecimal.MustParse(fmt.Sprintf("%.2f", amount)), nil
}

// GetSessionCount 获取会话数量
func (s *RedisService) GetSessionCount(ctx context.Context, userID int) (int, error) {
	key := fmt.Sprintf("ratelimit:sessions:%d", userID)
	count, err := s.client.SCard(ctx, key).Result()
	if err == redis.Nil {
		return 0, nil
	}
	return int(count), err
}

// ResetRPM 重置 RPM 计数
func (s *RedisService) ResetRPM(ctx context.Context, userID int) error {
	key := fmt.Sprintf("ratelimit:rpm:%d", userID)
	return s.client.Del(ctx, key).Err()
}

// ResetAmount 重置金额计数
func (s *RedisService) ResetAmount(ctx context.Context, userID int, period Period) error {
	key := s.getAmountKey(userID, period)
	return s.client.Del(ctx, key).Err()
}

// getAmountKey 获取金额限流的 key
func (s *RedisService) getAmountKey(userID int, period Period) string {
	return fmt.Sprintf("ratelimit:amount:%s:%d", period, userID)
}
