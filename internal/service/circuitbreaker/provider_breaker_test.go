package circuitbreaker

import (
	"sync"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
)

func TestProviderBreakerManager_IsOpen_NoBreaker(t *testing.T) {
	m := NewProviderBreakerManager()
	assert.False(t, m.IsOpen(999))
}

func TestProviderBreakerManager_FailureAndTrip(t *testing.T) {
	m := NewProviderBreakerManager()

	threshold := 3
	p := &model.Provider{
		ID:                             1,
		Name:                           "test-provider",
		CircuitBreakerFailureThreshold: &threshold,
	}

	// Record failures via model-aware API.
	for i := 0; i < 2; i++ {
		m.RecordFailureForProvider(p, false, nil)
	}
	assert.False(t, m.IsOpenForProvider(p))

	m.RecordFailureForProvider(p, false, nil)
	assert.True(t, m.IsOpen(1))
}

func TestProviderBreakerManager_SuccessCloses(t *testing.T) {
	m := NewProviderBreakerManager()

	threshold := 1
	dur := 50 // 50 ms
	halfOpen := 1
	p := &model.Provider{
		ID:                                     1,
		Name:                                   "test",
		CircuitBreakerFailureThreshold:         &threshold,
		CircuitBreakerOpenDuration:             &dur,
		CircuitBreakerHalfOpenSuccessThreshold: &halfOpen,
	}

	m.RecordFailureForProvider(p, false, nil)
	assert.True(t, m.IsOpen(1))

	// Manipulate time on the internal breaker.
	b := m.GetBreaker(1)
	b.mu.Lock()
	b.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	b.mu.Unlock()

	// Half-open now. Allow one request, then success closes it.
	assert.False(t, m.IsOpen(1)) // triggers half-open
	m.RecordSuccess(1)
	assert.False(t, m.IsOpen(1))

	s := m.GetState(1)
	assert.NotNil(t, s)
	assert.Equal(t, StateClosed, s.State)
}

func TestProviderBreakerManager_NetworkErrors_Ignored(t *testing.T) {
	m := NewProviderBreakerManager()
	m.SetCountNetworkErrors(false)

	threshold := 2
	p := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	for i := 0; i < 10; i++ {
		m.RecordFailureForProvider(p, true, nil)
	}
	assert.False(t, m.IsOpen(1))
}

func TestProviderBreakerManager_NetworkErrors_Counted(t *testing.T) {
	m := NewProviderBreakerManager()
	m.SetCountNetworkErrors(true)

	threshold := 2
	p := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	m.RecordFailureForProvider(p, true, nil)
	m.RecordFailureForProvider(p, true, nil)
	assert.True(t, m.IsOpen(1))
}

func TestProviderBreakerManager_ResetProvider(t *testing.T) {
	m := NewProviderBreakerManager()

	threshold := 1
	p := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	m.RecordFailureForProvider(p, false, nil)
	assert.True(t, m.IsOpen(1))

	m.ResetProvider(1)
	assert.False(t, m.IsOpen(1))
}

func TestProviderBreakerManager_AllStates(t *testing.T) {
	m := NewProviderBreakerManager()

	threshold := 1
	for _, id := range []int{1, 2, 3} {
		p := &model.Provider{
			ID:                             id,
			Name:                           "p",
			CircuitBreakerFailureThreshold: &threshold,
		}
		m.RecordFailureForProvider(p, false, nil)
	}

	states := m.AllStates()
	assert.Len(t, states, 3)
	for _, s := range states {
		assert.Equal(t, StateOpen, s.State)
	}
}

func TestProviderBreakerManager_NilProvider(t *testing.T) {
	m := NewProviderBreakerManager()

	// Should not panic.
	m.RecordFailureForProvider(nil, false, nil)
	m.RecordSuccessForProvider(nil)
	assert.False(t, m.IsOpenForProvider(nil))
}

func TestProviderBreakerManager_ResetAll(t *testing.T) {
	m := NewProviderBreakerManager()
	m.SetCountNetworkErrors(true)

	m.RecordFailure(1, false, nil)
	m.ResetAll()

	assert.False(t, m.IsOpen(1))
	assert.False(t, m.CountNetworkErrors())
}

func TestProviderBreakerManager_Concurrent(t *testing.T) {
	m := NewProviderBreakerManager()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			threshold := 50
			p := &model.Provider{
				ID:                             id%3 + 1,
				Name:                           "p",
				CircuitBreakerFailureThreshold: &threshold,
			}
			for j := 0; j < 100; j++ {
				m.RecordFailureForProvider(p, false, nil)
				m.IsOpen(p.ID)
				m.RecordSuccessForProvider(p)
				m.GetState(p.ID)
			}
		}(i)
	}
	wg.Wait()
}

func TestProviderBreakerManager_GetState_NilForUnknown(t *testing.T) {
	m := NewProviderBreakerManager()
	assert.Nil(t, m.GetState(999))
}
