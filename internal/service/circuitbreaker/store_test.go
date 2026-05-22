package circuitbreaker

import (
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// These tests exercise the backward-compatible package-level API in store.go.
// They mirror the original tests to ensure no regressions.

// TestIsOpen tests circuit breaker open state.
func TestIsOpen(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	if IsOpen(provider) {
		t.Error("Circuit breaker should be closed initially")
	}

	SetOpenForTest(1, time.Now().Add(1*time.Hour))

	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open")
	}
}

// TestIsOpen_Expired tests that breakers transition to half-open after expiry.
func TestIsOpen_Expired(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	SetOpenForTest(1, time.Now().Add(-1*time.Second))

	if IsOpen(provider) {
		t.Error("Circuit breaker should transition to half-open after expiry")
	}

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open after expiry")
	}
}

// TestRecordFailure tests failure recording.
func TestRecordFailure(t *testing.T) {
	ResetForTest()

	threshold := 5
	provider := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	for i := 0; i < 4; i++ {
		RecordFailure(provider, false)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open before threshold")
	}

	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open after threshold")
	}
}

// TestRecordFailure_NetworkError tests network error handling.
func TestRecordFailure_NetworkError(t *testing.T) {
	ResetForTest()

	threshold := 3
	provider := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	Configure(false)
	for i := 0; i < 5; i++ {
		RecordFailure(provider, true)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open for network errors when disabled")
	}

	ResetForTest()
	Configure(true)

	for i := 0; i < 3; i++ {
		RecordFailure(provider, true)
	}

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open for network errors when enabled")
	}
}

// TestRecordSuccess tests success recording.
func TestRecordSuccess(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	SetHalfOpenForTest(1)

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open")
	}

	RecordSuccess(provider)

	if IsOpen(provider) {
		t.Error("Circuit breaker should be closed after success")
	}

	if IsHalfOpen(provider) {
		t.Error("Circuit breaker should not be half-open after success")
	}
}

// TestRecordFailure_CustomThreshold tests custom failure threshold.
func TestRecordFailure_CustomThreshold(t *testing.T) {
	ResetForTest()

	threshold := 10
	provider := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	for i := 0; i < 9; i++ {
		RecordFailure(provider, false)
	}

	if IsOpen(provider) {
		t.Error("Circuit breaker should not open before custom threshold")
	}

	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should open after custom threshold")
	}
}

// TestRecordFailure_CustomDuration tests custom open duration.
func TestRecordFailure_CustomDuration(t *testing.T) {
	ResetForTest()

	threshold := 1
	duration := 100 // 100ms
	provider := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
		CircuitBreakerOpenDuration:     &duration,
	}

	RecordFailure(provider, false)

	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open")
	}

	time.Sleep(150 * time.Millisecond)

	if IsOpen(provider) {
		t.Error("Circuit breaker should transition to half-open after custom duration")
	}

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open after custom duration")
	}
}

// TestIsOpen_NilProvider tests nil provider.
func TestIsOpen_NilProvider(t *testing.T) {
	ResetForTest()

	if IsOpen(nil) {
		t.Error("IsOpen should return false for nil provider")
	}
}

// TestIsOpen_InvalidID tests invalid ID.
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

// TestRecordFailure_NilProvider tests nil provider.
func TestRecordFailure_NilProvider(t *testing.T) {
	ResetForTest()
	RecordFailure(nil, false)
}

// TestRecordSuccess_NilProvider tests nil provider.
func TestRecordSuccess_NilProvider(t *testing.T) {
	ResetForTest()
	RecordSuccess(nil)
}

// TestIsHalfOpen tests half-open state.
func TestIsHalfOpen(t *testing.T) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	if IsHalfOpen(provider) {
		t.Error("Circuit breaker should not be half-open initially")
	}

	SetHalfOpenForTest(1)

	if !IsHalfOpen(provider) {
		t.Error("Circuit breaker should be half-open")
	}
}

// TestConcurrentAccess tests concurrent access.
func TestConcurrentAccess(t *testing.T) {
	ResetForTest()

	threshold := 100
	provider := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			for j := 0; j < 20; j++ {
				RecordFailure(provider, false)
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if !IsOpen(provider) {
		t.Error("Circuit breaker should be open after concurrent failures")
	}
}

// BenchmarkIsOpen performance benchmark.
func BenchmarkIsOpen(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsOpen(provider)
	}
}

// BenchmarkRecordFailure performance benchmark.
func BenchmarkRecordFailure(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordFailure(provider, false)
	}
}

// BenchmarkRecordSuccess performance benchmark.
func BenchmarkRecordSuccess(b *testing.B) {
	ResetForTest()

	provider := &model.Provider{ID: 1, Name: "test"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		RecordSuccess(provider)
	}
}
