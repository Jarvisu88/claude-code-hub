package circuitbreaker

import (
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

// ProviderBreakerManager manages per-provider circuit breakers.
// Thread-safe; config is read from provider model fields.
type ProviderBreakerManager struct {
	mu       sync.RWMutex
	breakers map[int]*Breaker

	// countNetworkErrors controls whether network-level errors contribute
	// to the failure count. Mirrors the old Configure(enableNetworkErrors) behaviour.
	countNetworkErrors bool
}

// NewProviderBreakerManager creates a new manager.
func NewProviderBreakerManager() *ProviderBreakerManager {
	return &ProviderBreakerManager{
		breakers: make(map[int]*Breaker),
	}
}

// SetCountNetworkErrors controls whether transport-level errors (DNS, TLS,
// connection refused, etc.) are counted as circuit breaker failures.
func (m *ProviderBreakerManager) SetCountNetworkErrors(v bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.countNetworkErrors = v
}

// CountNetworkErrors reports the current setting.
func (m *ProviderBreakerManager) CountNetworkErrors() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.countNetworkErrors
}

// getOrCreate returns an existing breaker or creates one from provider config.
// Caller must NOT hold m.mu.
func (m *ProviderBreakerManager) getOrCreate(providerID int) *Breaker {
	m.mu.RLock()
	b, ok := m.breakers[providerID]
	m.mu.RUnlock()
	if ok {
		return b
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	// Double-check after acquiring write lock.
	if b, ok = m.breakers[providerID]; ok {
		return b
	}
	b = NewBreaker(providerID, DefaultBreakerConfig())
	m.breakers[providerID] = b
	return b
}

// getOrCreateWithConfig returns a breaker configured from the provider's DB fields.
func (m *ProviderBreakerManager) getOrCreateWithConfig(provider *model.Provider) *Breaker {
	if provider == nil || provider.ID <= 0 {
		return nil
	}
	m.mu.RLock()
	b, ok := m.breakers[provider.ID]
	m.mu.RUnlock()
	if ok {
		return b
	}

	cfg := configFromProvider(provider)

	m.mu.Lock()
	defer m.mu.Unlock()
	if b, ok = m.breakers[provider.ID]; ok {
		return b
	}
	b = NewBreaker(provider.ID, cfg)
	m.breakers[provider.ID] = b
	return b
}

// IsOpen reports whether the provider's circuit breaker is in the open state.
func (m *ProviderBreakerManager) IsOpen(providerID int) bool {
	m.mu.RLock()
	b, ok := m.breakers[providerID]
	m.mu.RUnlock()
	if !ok {
		return false
	}
	return !b.ShouldAllow()
}

// IsOpenForProvider reports whether the provider's circuit breaker is open.
// This is the model-aware variant that creates a breaker on demand using
// provider config if one doesn't exist yet.
func (m *ProviderBreakerManager) IsOpenForProvider(provider *model.Provider) bool {
	if provider == nil || provider.ID <= 0 {
		return false
	}
	b := m.getOrCreateWithConfig(provider)
	return !b.ShouldAllow()
}

// RecordSuccess resets the failure counter (and transitions from half-open to closed).
func (m *ProviderBreakerManager) RecordSuccess(providerID int) {
	m.mu.RLock()
	b, ok := m.breakers[providerID]
	m.mu.RUnlock()
	if !ok {
		return
	}
	b.RecordSuccess()
}

// RecordSuccessForProvider is the model-aware variant.
func (m *ProviderBreakerManager) RecordSuccessForProvider(provider *model.Provider) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	b := m.getOrCreateWithConfig(provider)
	b.RecordSuccess()
}

// RecordFailure records a failure. If networkError is true and countNetworkErrors
// is false, the failure is ignored.
func (m *ProviderBreakerManager) RecordFailure(providerID int, networkError bool, err error) {
	if networkError && !m.CountNetworkErrors() {
		return
	}
	b := m.getOrCreate(providerID)
	prevState := b.CurrentState()
	b.RecordFailure(err)
	newState := b.CurrentState()
	if prevState != StateOpen && newState == StateOpen {
		logger.Warn().Int("providerId", providerID).Msg("[CircuitBreaker] Provider circuit opened")
	}
}

// RecordFailureForProvider is the model-aware variant.
func (m *ProviderBreakerManager) RecordFailureForProvider(provider *model.Provider, networkError bool, err error) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	if networkError && !m.CountNetworkErrors() {
		return
	}
	b := m.getOrCreateWithConfig(provider)
	prevState := b.CurrentState()
	b.RecordFailure(err)
	newState := b.CurrentState()
	if prevState != StateOpen && newState == StateOpen {
		logger.Warn().Int("providerId", provider.ID).Str("name", provider.Name).
			Msg("[CircuitBreaker] Provider circuit opened")
	}
}

// ResetProvider forces a provider's breaker back to closed.
func (m *ProviderBreakerManager) ResetProvider(providerID int) {
	m.mu.RLock()
	b, ok := m.breakers[providerID]
	m.mu.RUnlock()
	if ok {
		b.Reset()
	}
}

// GetState returns the breaker state snapshot for a provider.
// Returns nil if no breaker exists for the provider.
func (m *ProviderBreakerManager) GetState(providerID int) *BreakerState {
	m.mu.RLock()
	b, ok := m.breakers[providerID]
	m.mu.RUnlock()
	if !ok {
		return nil
	}
	s := b.GetState()
	return &s
}

// GetBreaker returns the raw Breaker for a provider (for Redis sync, etc.).
func (m *ProviderBreakerManager) GetBreaker(providerID int) *Breaker {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.breakers[providerID]
}

// GetOrCreateBreaker returns or creates a breaker by ID (for Redis loader).
func (m *ProviderBreakerManager) GetOrCreateBreaker(providerID int) *Breaker {
	return m.getOrCreate(providerID)
}

// ResetAll clears all breakers (for testing).
func (m *ProviderBreakerManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.breakers = make(map[int]*Breaker)
	m.countNetworkErrors = false
}

// AllStates returns a snapshot of all breaker states.
func (m *ProviderBreakerManager) AllStates() map[int]BreakerState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[int]BreakerState, len(m.breakers))
	for id, b := range m.breakers {
		result[id] = b.GetState()
	}
	return result
}

// configFromProvider extracts breaker config from provider model fields.
func configFromProvider(p *model.Provider) BreakerConfig {
	cfg := DefaultBreakerConfig()
	if p.CircuitBreakerFailureThreshold != nil && *p.CircuitBreakerFailureThreshold > 0 {
		cfg.FailureThreshold = *p.CircuitBreakerFailureThreshold
	}
	if p.CircuitBreakerOpenDuration != nil && *p.CircuitBreakerOpenDuration > 0 {
		cfg.OpenDuration = time.Duration(*p.CircuitBreakerOpenDuration) * time.Millisecond
	}
	if p.CircuitBreakerHalfOpenSuccessThreshold != nil && *p.CircuitBreakerHalfOpenSuccessThreshold > 0 {
		cfg.HalfOpenSuccessThreshold = *p.CircuitBreakerHalfOpenSuccessThreshold
	}
	return cfg
}
