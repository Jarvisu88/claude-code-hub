package circuitbreaker

import (
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// TestIsOpen 测试熔断器打开状态
func TestIsOpen(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	// 初始状态：关闭
	if IsOpen(provider) {
		t.Error("Circuit breaker should be closed initially")
	}

	// 设置为打开状态
	SetOpenForTest(1, time.Now().Add(1*time.Hour))

	// 应该是打开状态
	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open")
	}
}

// TestIsOpen_Expired 测试熔断器过期后自动转为半开
func TestIsOpen_Expired(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	// 设置为打开状态，但已过期
	SetOpenForTest(1, time.Now().Add(-1*time.Second))

	// 应该自动转为半开状态
	if IsOpen(provider) {
		t.Error("Circuit breaker should transition to half-open after expiry")
	}

	// 验证是半开状态
	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open after expiry")
	}
}

// TestRecordFailure 测试记录失败
func TestRecordFailure(t *testing.T) {
	ResetForTest()

	threshold := 5
	provider := &model.Provider{
		ID:                             1,
		CircuitBreakerFailureThreshold: &threshold,
	}

	// 记录 4 次失败，不应该打开
	for i := 0; i < 4; i++ {
		RecordFailure(provider, false)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open before threshold")
	}

	// 记录第 5 次失败，应该打开
	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open after threshold")
	}
}

// TestRecordFailure_NetworkError 测试网络错误
func TestRecordFailure_NetworkError(t *testing.T) {
	// 保存原始设置
	originalCountNetworkErrors := countNetworkErrors
	defer func() {
		mu.Lock()
		countNetworkErrors = originalCountNetworkErrors
		mu.Unlock()
	}()

	ResetForTest()

	threshold := 3
	provider := &model.Provider{
		ID:                             1,
		CircuitBreakerFailureThreshold: &threshold,
	}

	// 默认不计数网络错误
	Configure(false)
	for i := 0; i < 5; i++ {
		RecordFailure(provider, true)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open for network errors when disabled")
	}

	// 启用网络错误计数并重置
	ResetForTest()
	Configure(true)

	// 记录 3 次网络错误，应该打开
	for i := 0; i < 3; i++ {
		RecordFailure(provider, true)
	}

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open for network errors when enabled")
	}
}

// TestRecordSuccess 测试记录成功
func TestRecordSuccess(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	// 设置为半开状态
	SetHalfOpenForTest(1)

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open")
	}

	// 记录成功，应该关闭
	RecordSuccess(provider)

	if IsOpen(provider) {
		t.Error("Circuit breaker should be closed after success")
	}

	if IsHalfOpen(provider) {
		t.Error("Circuit breaker should not be half-open after success")
	}
}

// TestRecordFailure_CustomThreshold 测试自定义阈值
func TestRecordFailure_CustomThreshold(t *testing.T) {
	ResetForTest()

	threshold := 10
	provider := &model.Provider{
		ID:                             1,
		CircuitBreakerFailureThreshold: &threshold,
	}

	// 记录 9 次失败
	for i := 0; i < 9; i++ {
		RecordFailure(provider, false)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open before custom threshold")
	}

	// 记录第 10 次失败
	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open after custom threshold")
	}
}

// TestRecordFailure_CustomDuration 测试自定义打开时长
func TestRecordFailure_CustomDuration(t *testing.T) {
	ResetForTest()

	threshold := 1
	duration := 100 // 100ms
	provider := &model.Provider{
		ID:                             1,
		CircuitBreakerFailureThreshold: &threshold,
		CircuitBreakerOpenDuration:     &duration,
	}

	// 记录失败，打开熔断器
	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open")
	}

	// 等待过期
	time.Sleep(150 * time.Millisecond)

	// 应该转为半开状态
	if IsOpen(provider) {
		t.Error("Circuit breaker should transition to half-open after custom duration")
	}

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open after custom duration")
	}
}

// TestIsOpen_NilProvider 测试 nil provider
func TestIsOpen_NilProvider(t *testing.T) {
	ResetForTest()

	if IsOpen(nil) {
		t.Error("IsOpen should return false for nil provider")
	}
}

// TestIsOpen_InvalidID 测试无效 ID
func TestIsOpen_InvalidID(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 0}

	if IsOpen(provider) {
		t.Error("IsOpen should return false for invalid ID")
	}

	provider.ID = -1

	if IsOpen(provider) {
		t.Error("IsOpen should return false for negative ID")
	}
}

// TestRecordFailure_NilProvider 测试 nil provider
func TestRecordFailure_NilProvider(t *testing.T) {
	ResetForTest()

	// 不应该 panic
	RecordFailure(nil, false)
}

// TestRecordSuccess_NilProvider 测试 nil provider
func TestRecordSuccess_NilProvider(t *testing.T) {
	ResetForTest()

	// 不应该 panic
	RecordSuccess(nil)
}

// TestIsHalfOpen 测试半开状态
func TestIsHalfOpen(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	// 初始状态：不是半开
	if IsHalfOpen(provider) {
		t.Error("Circuit breaker should not be half-open initially")
	}

	// 设置为半开状态
	SetHalfOpenForTest(1)

	// 应该是半开状态
	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open")
	}
}

// TestConcurrentAccess 测试并发访问
func TestConcurrentAccess(t *testing.T) {
	ResetForTest()

	threshold := 100
	provider := &model.Provider{
		ID:                             1,
		CircuitBreakerFailureThreshold: &threshold,
	}

	// 并发记录失败
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				RecordFailure(provider, false)
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 应该打开（200 次失败 > 100 阈值）
	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open after concurrent failures")
	}
}

// BenchmarkIsOpen 性能基准测试
func BenchmarkIsOpen(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsOpen(provider)
	}
}

// BenchmarkRecordFailure 性能基准测试
func BenchmarkRecordFailure(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordFailure(provider, false)
	}
}

// BenchmarkRecordSuccess 性能基准测试
func BenchmarkRecordSuccess(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordSuccess(provider)
	}
}
