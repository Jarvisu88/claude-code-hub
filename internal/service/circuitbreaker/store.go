package circuitbreaker

// store.go provides backward-compatible package-level functions used by the
// existing proxy handler (internal/handler/v1/proxy.go). These delegate to a
// shared package-level Service instance.
//
// New callers should prefer injecting *Service directly.

import (
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
)

// defaultService is the singleton used by the legacy package-level API.
var (
	defaultService     *Service
	defaultServiceOnce sync.Once
)

// getDefaultService lazily initialises the package-level service.
// It runs without Redis (pure in-memory) to match the old behaviour.
func getDefaultService() *Service {
	defaultServiceOnce.Do(func() {
		defaultService = NewService(ServiceConfig{})
	})
	return defaultService
}

// SetDefaultService replaces the package-level service instance. Call this at
// application startup if you have a Redis client available.
func SetDefaultService(svc *Service) {
	defaultService = svc
	// Mark once as done so getDefaultService won't overwrite.
	defaultServiceOnce.Do(func() {})
}

// Configure controls whether network-level errors are counted as failures.
func Configure(enableNetworkErrors bool) {
	getDefaultService().ProviderBreakers().SetCountNetworkErrors(enableNetworkErrors)
}

// IsOpen reports whether the provider's circuit breaker is open.
// Backward-compatible: creates breaker on demand from provider config.
func IsOpen(provider *model.Provider) bool {
	if provider == nil || provider.ID <= 0 {
		return false
	}
	return getDefaultService().IsProviderOpenForModel(provider)
}

// RecordFailure records a failed request for the provider.
func RecordFailure(provider *model.Provider, networkError bool) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	getDefaultService().RecordProviderFailureForModel(provider, networkError, nil)
}

// RecordSuccess records a successful request for the provider.
// For backward compatibility, a single success in half-open state immediately
// resets the breaker (the old code used delete(states, id)).
func RecordSuccess(provider *model.Provider) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	svc := getDefaultService()
	b := svc.ProviderBreakers().GetBreaker(provider.ID)
	if b != nil && b.CurrentState() == StateHalfOpen {
		// Legacy behaviour: any success in half-open = full reset.
		b.Reset()
		return
	}
	svc.RecordProviderSuccessForModel(provider)
}

// IsHalfOpen reports whether the provider is in half-open state.
func IsHalfOpen(provider *model.Provider) bool {
	if provider == nil || provider.ID <= 0 {
		return false
	}
	s := getDefaultService().GetProviderState(provider.ID)
	if s == nil {
		return false
	}
	if s.State == StateHalfOpen {
		return true
	}
	// Also check if open-duration has elapsed (logically half-open).
	if s.State == StateOpen {
		b := getDefaultService().ProviderBreakers().GetBreaker(provider.ID)
		if b != nil {
			b.mu.RLock()
			elapsed := time.Now().After(b.openedAt.Add(b.config.OpenDuration))
			b.mu.RUnlock()
			return elapsed
		}
	}
	return false
}

// --- Test helpers (preserve the existing test API) ---

// ResetForTest resets all breaker state. For testing only.
func ResetForTest() {
	// Reset the singleton so tests get a fresh instance.
	defaultService = NewService(ServiceConfig{})
	defaultServiceOnce.Do(func() {})
}

// SetOpenForTest sets a provider breaker to open state until the given time.
func SetOpenForTest(providerID int, openUntil time.Time) {
	svc := getDefaultService()
	b := svc.ProviderBreakers().GetOrCreateBreaker(providerID)
	b.LoadState(BreakerState{
		ProviderID:   providerID,
		State:        StateOpen,
		FailureCount: 999,
		OpenedAt:     time.Now(),
	})
	// Override nowFunc to control when the breaker transitions.
	b.mu.Lock()
	savedOpenedAt := b.openedAt
	b.config.OpenDuration = openUntil.Sub(savedOpenedAt)
	if b.config.OpenDuration <= 0 {
		// If openUntil is in the past, use a very small positive duration.
		b.config.OpenDuration = time.Millisecond
		b.openedAt = openUntil.Add(-time.Millisecond)
	}
	b.mu.Unlock()
}

// SetHalfOpenForTest sets a provider breaker to half-open state.
func SetHalfOpenForTest(providerID int) {
	svc := getDefaultService()
	b := svc.ProviderBreakers().GetOrCreateBreaker(providerID)
	b.LoadState(BreakerState{
		ProviderID:   providerID,
		State:        StateHalfOpen,
		FailureCount: 0,
	})
}
