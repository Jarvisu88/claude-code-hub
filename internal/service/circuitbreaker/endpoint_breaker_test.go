package circuitbreaker

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestEndpointBreaker_InitiallyClosed(t *testing.T) {
	m := NewEndpointBreakerManager()
	assert.False(t, m.IsOpen(1))
	assert.Nil(t, m.GetState(1))
}

func TestEndpointBreaker_TripsOnThreshold(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 3,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	m.RecordFailure(1)
	assert.False(t, m.IsOpen(1))

	m.RecordFailure(1)
	assert.True(t, m.IsOpen(1))

	s := m.GetState(1)
	assert.NotNil(t, s)
	assert.Equal(t, "open", s.State)
	assert.Equal(t, 3, s.FailureCount)
}

func TestEndpointBreaker_AutoRecovery(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 1,
		SkipDuration:     50 * time.Millisecond,
	})

	m.RecordFailure(1)
	assert.True(t, m.IsOpen(1))

	// Advance time.
	m.mu.Lock()
	m.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	m.mu.Unlock()

	assert.False(t, m.IsOpen(1))
}

func TestEndpointBreaker_SuccessResets(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 3,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	m.RecordFailure(1)
	m.RecordSuccess(1)

	// Failure counter should be reset.
	m.RecordFailure(1)
	m.RecordFailure(1)
	assert.False(t, m.IsOpen(1))

	m.RecordFailure(1)
	assert.True(t, m.IsOpen(1))
}

func TestEndpointBreaker_Reset(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 1,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	assert.True(t, m.IsOpen(1))

	m.Reset(1)
	assert.False(t, m.IsOpen(1))
	assert.Nil(t, m.GetState(1))
}

func TestEndpointBreaker_MultipleEndpoints(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 2,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	m.RecordFailure(1)
	m.RecordFailure(2)

	assert.True(t, m.IsOpen(1))
	assert.False(t, m.IsOpen(2))
}

func TestEndpointBreaker_ResetAll(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 1,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	m.RecordFailure(2)
	m.ResetAll()

	assert.False(t, m.IsOpen(1))
	assert.False(t, m.IsOpen(2))
}

func TestEndpointBreaker_DefaultConfig(t *testing.T) {
	m := NewEndpointBreakerManager()
	assert.Equal(t, DefaultEndpointFailureThreshold, m.config.FailureThreshold)
	assert.Equal(t, DefaultEndpointSkipDuration, m.config.SkipDuration)
}

func TestEndpointBreaker_GetState_Closed(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 5,
		SkipDuration:     time.Minute,
	})

	m.RecordFailure(1)
	s := m.GetState(1)
	assert.NotNil(t, s)
	assert.Equal(t, "closed", s.State)
	assert.Equal(t, 1, s.FailureCount)
}

func TestEndpointBreaker_ConcurrentAccess(t *testing.T) {
	m := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: 50,
		SkipDuration:     time.Minute,
	})

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			eid := id%5 + 1
			for j := 0; j < 100; j++ {
				m.RecordFailure(eid)
				m.IsOpen(eid)
				m.RecordSuccess(eid)
				m.GetState(eid)
			}
		}(i)
	}
	wg.Wait()
}

func TestEndpointBreaker_SuccessOnUnknown_NoOp(t *testing.T) {
	m := NewEndpointBreakerManager()
	// Should not panic on unknown endpoint.
	m.RecordSuccess(999)
}
