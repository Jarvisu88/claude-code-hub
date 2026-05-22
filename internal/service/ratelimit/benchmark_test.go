package ratelimit

import (
	"context"
	"testing"

	"github.com/quagmt/udecimal"
)

// BenchmarkCheckRPM 性能基准测试
func BenchmarkCheckRPM(b *testing.B) {
	service, mr := setupTestRedis(&testing.T{})
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	limit := 100

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.CheckRPM(ctx, userID, limit)
	}
}

// BenchmarkIncrementRPM 性能基准测试
func BenchmarkIncrementRPM(b *testing.B) {
	service, mr := setupTestRedis(&testing.T{})
	defer mr.Close()

	ctx := context.Background()
	userID := 1

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.IncrementRPM(ctx, userID)
	}
}

// BenchmarkIncrementAmount 性能基准测试
func BenchmarkIncrementAmount(b *testing.B) {
	service, mr := setupTestRedis(&testing.T{})
	defer mr.Close()

	ctx := context.Background()
	userID := 1
	amount := udecimal.MustParse("10.50")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.IncrementAmount(ctx, userID, amount, PeriodDaily)
	}
}
