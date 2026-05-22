package circuitbreaker

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestBreaker(threshold int, openDur time.Duration, halfOpenSucc int) *Breaker {
	return NewBreaker(1, BreakerConfig{
		FailureThreshold:         threshold,
		OpenDuration:             openDur,
		HalfOpenSuccessThreshold: halfOpenSucc,
	})
}

func TestBreaker_InitialState(t *testing.T) {
	b := newTestBreaker(5, time.Minute, 2)
	assert.Equal(t, StateClosed, b.CurrentState())
	assert.True(t, b.ShouldAllow())
	assert.False(t, b.IsOpen())
}

func TestBreaker_ClosedToOpen(t *testing.T) {
	b := newTestBreaker(3, time.Minute, 2)

	// Below threshold: still closed.
	for i := 0; i < 2; i++ {
		b.RecordFailure(nil)
	}
	assert.Equal(t, StateClosed, b.CurrentState())
	assert.True(t, b.ShouldAllow())

	// Reach threshold: trips to open.
	b.RecordFailure(nil)
	assert.Equal(t, StateOpen, b.CurrentState())
	assert.False(t, b.ShouldAllow())
	assert.True(t, b.IsOpen())
}

func TestBreaker_OpenToHalfOpen(t *testing.T) {
	b := newTestBreaker(1, 50*time.Millisecond, 1)

	// Trip open.
	b.RecordFailure(nil)
	assert.Equal(t, StateOpen, b.CurrentState())
	assert.False(t, b.ShouldAllow())

	// Advance past open duration using nowFunc.
	b.mu.Lock()
	b.nowFunc = func() time.Time {
		return time.Now().Add(100 * time.Millisecond)
	}
	b.mu.Unlock()

	// ShouldAllow transitions to half-open.
	assert.True(t, b.ShouldAllow())
	assert.Equal(t, StateHalfOpen, b.CurrentState())
}

func TestBreaker_HalfOpenToClosed(t *testing.T) {
	b := newTestBreaker(1, 50*time.Millisecond, 2)

	b.RecordFailure(nil)

	// Advance time to go half-open.
	b.mu.Lock()
	b.nowFunc = func() time.Time {
		return time.Now().Add(100 * time.Millisecond)
	}
	b.mu.Unlock()
	b.ShouldAllow() // triggers transition

	assert.Equal(t, StateHalfOpen, b.CurrentState())

	// First success: still half-open.
	b.RecordSuccess()
	assert.Equal(t, StateHalfOpen, b.CurrentState())

	// Second success: transitions to closed.
	b.RecordSuccess()
	assert.Equal(t, StateClosed, b.CurrentState())
}

func TestBreaker_HalfOpenFailure_ReopensCircuit(t *testing.T) {
	b := newTestBreaker(1, 50*time.Millisecond, 2)

	b.RecordFailure(nil)

	b.mu.Lock()
	b.nowFunc = func() time.Time {
		return time.Now().Add(100 * time.Millisecond)
	}
	b.mu.Unlock()
	b.ShouldAllow()

	assert.Equal(t, StateHalfOpen, b.CurrentState())

	// Failure in half-open re-opens.
	b.RecordFailure(nil)
	assert.Equal(t, StateOpen, b.CurrentState())
	assert.False(t, b.ShouldAllow())
}

func TestBreaker_SuccessInClosed_ResetsFailures(t *testing.T) {
	b := newTestBreaker(5, time.Minute, 2)

	b.RecordFailure(nil)
	b.RecordFailure(nil)

	// Success resets failure count.
	b.RecordSuccess()

	// Now needs 5 more failures to open.
	for i := 0; i < 4; i++ {
		b.RecordFailure(nil)
	}
	assert.Equal(t, StateClosed, b.CurrentState())

	b.RecordFailure(nil)
	assert.Equal(t, StateOpen, b.CurrentState())
}

func TestBreaker_Reset(t *testing.T) {
	b := newTestBreaker(1, time.Minute, 1)

	b.RecordFailure(nil)
	assert.Equal(t, StateOpen, b.CurrentState())

	b.Reset()
	assert.Equal(t, StateClosed, b.CurrentState())
	assert.True(t, b.ShouldAllow())
}

func TestBreaker_GetState_Snapshot(t *testing.T) {
	b := newTestBreaker(2, time.Minute, 1)
	b.RecordFailure(nil)
	b.RecordFailure(nil)

	s := b.GetState()
	assert.Equal(t, StateOpen, s.State)
	assert.Equal(t, 2, s.FailureCount)
	assert.Equal(t, 1, s.ProviderID)
	assert.False(t, s.OpenedAt.IsZero())
}

func TestBreaker_LoadState(t *testing.T) {
	b := newTestBreaker(5, time.Minute, 2)
	openedAt := time.Now().Add(-30 * time.Second)
	b.LoadState(BreakerState{
		ProviderID:   1,
		State:        StateOpen,
		FailureCount: 10,
		SuccessCount: 0,
		OpenedAt:     openedAt,
	})

	assert.Equal(t, StateOpen, b.CurrentState())
	s := b.GetState()
	assert.Equal(t, 10, s.FailureCount)
}

func TestBreaker_DefaultConfig_Normalization(t *testing.T) {
	// Zero/negative values should use defaults.
	b := NewBreaker(1, BreakerConfig{
		FailureThreshold:         0,
		OpenDuration:             -1,
		HalfOpenSuccessThreshold: 0,
	})
	require.NotNil(t, b)
	assert.Equal(t, DefaultFailureThreshold, b.config.FailureThreshold)
	assert.Equal(t, DefaultOpenDuration, b.config.OpenDuration)
	assert.Equal(t, DefaultHalfOpenSuccessThreshold, b.config.HalfOpenSuccessThreshold)
}

func TestBreaker_GetState_ShowsHalfOpenWhenTimerExpired(t *testing.T) {
	b := newTestBreaker(1, 50*time.Millisecond, 1)
	b.RecordFailure(nil)

	b.mu.Lock()
	b.nowFunc = func() time.Time {
		return time.Now().Add(100 * time.Millisecond)
	}
	b.mu.Unlock()

	// GetState should show half-open even without explicit ShouldAllow call.
	s := b.GetState()
	assert.Equal(t, StateHalfOpen, s.State)
}

// TestBreaker_ConcurrentAccess verifies no data races under concurrent load.
func TestBreaker_ConcurrentAccess(t *testing.T) {
	b := newTestBreaker(100, time.Minute, 10)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(3)
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				b.RecordFailure(nil)
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				b.RecordSuccess()
			}
		}()
		go func() {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				b.ShouldAllow()
				b.IsOpen()
				b.GetState()
			}
		}()
	}
	wg.Wait()
	// If we get here without -race complaints, concurrency is safe.
}
