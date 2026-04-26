package circuitbreaker

import (
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

type providerState struct {
	failureCount int
	openUntil    time.Time
}

var (
	mu                 sync.Mutex
	states             = map[int]providerState{}
	now                = time.Now
	countNetworkErrors = false
)

func Configure(enableNetworkErrors bool) {
	mu.Lock()
	defer mu.Unlock()
	countNetworkErrors = enableNetworkErrors
}

func ResetForTest() {
	mu.Lock()
	defer mu.Unlock()
	states = map[int]providerState{}
	countNetworkErrors = false
}

func SetOpenForTest(providerID int, openUntil time.Time) {
	mu.Lock()
	defer mu.Unlock()
	states[providerID] = providerState{failureCount: 999, openUntil: openUntil}
}

func IsOpen(provider *model.Provider) bool {
	if provider == nil || provider.ID <= 0 {
		return false
	}
	mu.Lock()
	defer mu.Unlock()
	state, ok := states[provider.ID]
	if !ok {
		return false
	}
	if state.openUntil.IsZero() {
		return false
	}
	if now().After(state.openUntil) {
		delete(states, provider.ID)
		return false
	}
	return true
}

func RecordFailure(provider *model.Provider, networkError bool) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	if networkError && !countNetworkErrors {
		return
	}
	threshold := 5
	if provider.CircuitBreakerFailureThreshold != nil && *provider.CircuitBreakerFailureThreshold > 0 {
		threshold = *provider.CircuitBreakerFailureThreshold
	}
	openDuration := 1800000 * time.Millisecond
	if provider.CircuitBreakerOpenDuration != nil && *provider.CircuitBreakerOpenDuration > 0 {
		openDuration = time.Duration(*provider.CircuitBreakerOpenDuration) * time.Millisecond
	}

	mu.Lock()
	defer mu.Unlock()
	state := states[provider.ID]
	state.failureCount++
	if state.failureCount >= threshold {
		state.openUntil = now().Add(openDuration)
	}
	states[provider.ID] = state
}

func RecordSuccess(provider *model.Provider) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	delete(states, provider.ID)
}
