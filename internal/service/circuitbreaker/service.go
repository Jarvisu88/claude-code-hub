package circuitbreaker

import (
	"context"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

// Service is the unified circuit breaker facade combining all three levels
// (provider, endpoint, vendor+type) with optional Redis cross-process sync.
type Service struct {
	providerBreakers   *ProviderBreakerManager
	endpointBreakers   *EndpointBreakerManager
	vendorTypeBreakers *VendorTypeBreakerManager
	redisSync          *RedisStateSync
}

// ServiceConfig holds configuration for the unified circuit breaker service.
type ServiceConfig struct {
	// Redis client for cross-process sync. Nil disables sync (in-memory only).
	RedisClient *redis.Client

	// Endpoint breaker config overrides.
	EndpointFailureThreshold int
	EndpointSkipDuration     time.Duration

	// Whether to count network-level errors as circuit breaker failures.
	CountNetworkErrors bool
}

// NewService creates a new unified circuit breaker service.
func NewService(cfg ServiceConfig) *Service {
	pm := NewProviderBreakerManager()
	pm.SetCountNetworkErrors(cfg.CountNetworkErrors)

	em := NewEndpointBreakerManager(EndpointBreakerConfig{
		FailureThreshold: cfg.EndpointFailureThreshold,
		SkipDuration:     cfg.EndpointSkipDuration,
	})

	vm := NewVendorTypeBreakerManager()
	rs := NewRedisStateSync(cfg.RedisClient)

	return &Service{
		providerBreakers:   pm,
		endpointBreakers:   em,
		vendorTypeBreakers: vm,
		redisSync:          rs,
	}
}

// --- Provider-level operations ---

// IsProviderOpen checks whether a provider's circuit breaker blocks requests.
func (s *Service) IsProviderOpen(providerID int) bool {
	return s.providerBreakers.IsOpen(providerID)
}

// IsProviderOpenForModel checks using a provider model (creates breaker on demand).
func (s *Service) IsProviderOpenForModel(provider *model.Provider) bool {
	return s.providerBreakers.IsOpenForProvider(provider)
}

// RecordProviderSuccess records a successful request for a provider.
func (s *Service) RecordProviderSuccess(providerID int) {
	s.providerBreakers.RecordSuccess(providerID)
	s.syncProviderStateAsync(providerID)
}

// RecordProviderSuccessForModel records success using the model.
func (s *Service) RecordProviderSuccessForModel(provider *model.Provider) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	s.providerBreakers.RecordSuccessForProvider(provider)
	s.syncProviderStateAsync(provider.ID)
}

// RecordProviderFailure records a failure for a provider.
func (s *Service) RecordProviderFailure(providerID int, err error) {
	s.providerBreakers.RecordFailure(providerID, false, err)
	s.syncProviderStateAsync(providerID)
}

// RecordProviderFailureForModel records a failure using the model.
func (s *Service) RecordProviderFailureForModel(provider *model.Provider, networkError bool, err error) {
	if provider == nil || provider.ID <= 0 {
		return
	}
	s.providerBreakers.RecordFailureForProvider(provider, networkError, err)
	s.syncProviderStateAsync(provider.ID)
}

// ResetProvider forces a provider's breaker to closed state.
func (s *Service) ResetProvider(providerID int) {
	s.providerBreakers.ResetProvider(providerID)
	if s.redisSync.Available() {
		s.redisSync.DeleteProviderState(context.Background(), providerID)
	}
}

// GetProviderState returns the current state of a provider's breaker.
func (s *Service) GetProviderState(providerID int) *BreakerState {
	return s.providerBreakers.GetState(providerID)
}

// --- Endpoint-level operations ---

// IsEndpointOpen checks whether an endpoint should be skipped.
func (s *Service) IsEndpointOpen(endpointID int) bool {
	return s.endpointBreakers.IsOpen(endpointID)
}

// RecordEndpointSuccess records a success for an endpoint.
func (s *Service) RecordEndpointSuccess(endpointID int) {
	s.endpointBreakers.RecordSuccess(endpointID)
}

// RecordEndpointFailure records a failure for an endpoint.
func (s *Service) RecordEndpointFailure(endpointID int) {
	s.endpointBreakers.RecordFailure(endpointID)
}

// ResetEndpoint resets an endpoint's breaker state.
func (s *Service) ResetEndpoint(endpointID int) {
	s.endpointBreakers.Reset(endpointID)
	if s.redisSync.Available() {
		s.redisSync.DeleteEndpointState(context.Background(), endpointID)
	}
}

// GetEndpointState returns the current state of an endpoint breaker.
func (s *Service) GetEndpointState(endpointID int) *EndpointBreakerState {
	return s.endpointBreakers.GetState(endpointID)
}

// --- Vendor+Type level operations ---

// IsVendorTypeOpen checks whether a vendor+type circuit is open.
func (s *Service) IsVendorTypeOpen(vendorID int, providerType string) bool {
	return s.vendorTypeBreakers.IsOpen(vendorID, providerType)
}

// TriggerVendorTypeBreaker opens the vendor+type breaker. Called when all
// endpoints of a vendor+type combination timeout simultaneously.
func (s *Service) TriggerVendorTypeBreaker(vendorID int, providerType string) {
	s.vendorTypeBreakers.TriggerOpen(vendorID, providerType)
	s.syncVendorTypeStateAsync(vendorID, providerType)
}

// ResetVendorType resets a vendor+type breaker.
func (s *Service) ResetVendorType(vendorID int, providerType string) {
	s.vendorTypeBreakers.Reset(vendorID, providerType)
	if s.redisSync.Available() {
		s.redisSync.DeleteVendorTypeState(context.Background(), vendorID, providerType)
	}
}

// GetVendorTypeState returns the current state of a vendor+type breaker.
func (s *Service) GetVendorTypeState(vendorID int, providerType string) *VendorTypeState {
	return s.vendorTypeBreakers.GetState(vendorID, providerType)
}

// --- Loader: initialize from Redis on startup ---

// LoadFromRedis loads all persisted breaker states from Redis into memory.
// Should be called once during application startup.
func (s *Service) LoadFromRedis(ctx context.Context) error {
	if !s.redisSync.Available() {
		return nil
	}

	states, err := s.redisSync.LoadAllProviderStates(ctx)
	if err != nil {
		logger.Warn().Err(err).Msg("[CircuitBreaker] Failed to load provider states from Redis")
		return err
	}

	loaded := 0
	for _, state := range states {
		if state.State == StateClosed {
			continue // No need to load closed states.
		}
		b := s.providerBreakers.GetOrCreateBreaker(state.ProviderID)
		b.LoadState(state)
		loaded++
	}

	if loaded > 0 {
		logger.Info().Int("count", loaded).
			Msg("[CircuitBreaker] Loaded provider states from Redis")
	}

	return nil
}

// --- Access to sub-managers (for advanced usage) ---

// ProviderBreakers returns the underlying provider breaker manager.
func (s *Service) ProviderBreakers() *ProviderBreakerManager {
	return s.providerBreakers
}

// EndpointBreakers returns the underlying endpoint breaker manager.
func (s *Service) EndpointBreakers() *EndpointBreakerManager {
	return s.endpointBreakers
}

// VendorTypeBreakers returns the underlying vendor+type breaker manager.
func (s *Service) VendorTypeBreakers() *VendorTypeBreakerManager {
	return s.vendorTypeBreakers
}

// RedisSync returns the underlying Redis state synchroniser.
func (s *Service) RedisSync() *RedisStateSync {
	return s.redisSync
}

// --- Async Redis sync helpers (fire-and-forget, fail-open) ---

func (s *Service) syncProviderStateAsync(providerID int) {
	if !s.redisSync.Available() {
		return
	}

	state := s.providerBreakers.GetState(providerID)
	if state == nil {
		return
	}

	// Determine TTL from the breaker's config.
	openDuration := DefaultOpenDuration
	b := s.providerBreakers.GetBreaker(providerID)
	if b != nil {
		b.mu.RLock()
		openDuration = b.config.OpenDuration
		b.mu.RUnlock()
	}

	// Delete closed state from Redis to keep it clean.
	if state.State == StateClosed {
		go func() {
			s.redisSync.DeleteProviderState(context.Background(), providerID)
		}()
		return
	}

	go func() {
		s.redisSync.SaveProviderState(context.Background(), *state, openDuration)
	}()
}

func (s *Service) syncVendorTypeStateAsync(vendorID int, providerType string) {
	if !s.redisSync.Available() {
		return
	}

	state := s.vendorTypeBreakers.GetState(vendorID, providerType)
	if state == nil {
		return
	}

	if state.State == "closed" {
		go func() {
			s.redisSync.DeleteVendorTypeState(context.Background(), vendorID, providerType)
		}()
		return
	}

	go func() {
		s.redisSync.SaveVendorTypeState(context.Background(), *state, DefaultVendorTypeOpenDuration*2)
	}()
}
