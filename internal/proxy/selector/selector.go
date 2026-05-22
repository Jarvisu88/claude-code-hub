// Package selector implements a multi-stage provider selection pipeline.
//
// The pipeline consists of 9 stages:
//  1. Session Reuse - Check Redis for provider bound to this session
//  2. Group Pre-filtering - Filter by user/key providerGroup
//  3. Client Restriction Filtering - Provider allowedClients/blockedClients vs User-Agent
//  4. Basic Filtering - Enabled, schedule (time window), format-type match, model support
//  5. Health Filtering - Circuit breaker state, cost limits
//  6. Priority Tiers - Select highest priority tier, support groupPriorities overrides
//  7. Weighted Random - Cost-sorted, then weighted random selection
//  8. Concurrent Session Check - Atomic Redis check (fail -> try next, max 20 switches)
//  9. Result - Set selected provider and metadata
package selector

import (
	"context"
	"fmt"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/quagmt/udecimal"
)

// MaxProviderSwitches is the maximum number of attempts for concurrent session check fallback.
const MaxProviderSwitches = 20

// ProviderRepo provides access to the provider list.
type ProviderRepo interface {
	GetActiveProviders(ctx context.Context) ([]*model.Provider, error)
	GetByID(ctx context.Context, id int) (*model.Provider, error)
}

// ProviderGroupRepo looks up group cost multiplier.
type ProviderGroupRepo interface {
	GetCostMultiplier(ctx context.Context, group string) (udecimal.Decimal, error)
}

// CircuitBreakerService checks provider health.
type CircuitBreakerService interface {
	IsOpen(provider *model.Provider) bool
}

// SessionService manages session-to-provider bindings.
type SessionService interface {
	// GetBoundProvider returns the provider ID bound to the session, or 0 if none.
	GetBoundProvider(ctx context.Context, sessionID string) (int, error)
	// BindProvider binds a session to a provider.
	BindProvider(ctx context.Context, sessionID string, providerID int)
}

// CostLimitChecker checks whether a provider has exceeded its cost limits.
type CostLimitChecker interface {
	// CheckCostLimits returns true if the provider is within cost limits (allowed to serve).
	CheckCostLimits(ctx context.Context, provider *model.Provider) (bool, error)
}

// ConcurrencyChecker atomically checks and tracks concurrent sessions on a provider.
type ConcurrencyChecker interface {
	// CheckAndTrack atomically checks if the provider can accept a new session.
	// Returns (allowed, currentCount, error).
	CheckAndTrack(ctx context.Context, providerID int, sessionID string, limit int) (bool, int, error)
}

// SelectRequest holds the input parameters for a provider selection.
type SelectRequest struct {
	Model         string   // requested model name
	UserAgent     string   // client User-Agent header
	SessionID     string   // client session ID (for session reuse and concurrent check)
	KeyID         int      // API key ID
	UserID        int      // user ID
	ProviderGroup string   // from key or user, comma-separated group names
	Format        string   // request format: claude, openai, codex, gemini, gemini-cli
	ExcludeIDs    []int    // already-failed provider IDs to skip
	Providers     []*model.Provider // optional: pre-fetched provider list (skips repo call)
}

// SelectResult holds the output of a provider selection.
type SelectResult struct {
	Provider      *model.Provider
	GroupCostMult udecimal.Decimal
	SessionReused bool
	AttemptCount  int
}

// Selector orchestrates the multi-stage provider selection pipeline.
type Selector struct {
	providerRepo    ProviderRepo
	groupRepo       ProviderGroupRepo
	circuitBreaker  CircuitBreakerService
	sessionService  SessionService
	costChecker     CostLimitChecker
	concurrency     ConcurrencyChecker
	nowFunc         func() TimeOfDay // injectable for testing
	timezoneFunc    func() string    // returns system timezone, injectable for testing
}

// Option configures a Selector.
type Option func(*Selector)

// WithNowFunc overrides the current time function (for testing).
func WithNowFunc(f func() TimeOfDay) Option {
	return func(s *Selector) { s.nowFunc = f }
}

// WithTimezoneFunc overrides the timezone resolver (for testing).
func WithTimezoneFunc(f func() string) Option {
	return func(s *Selector) { s.timezoneFunc = f }
}

// New creates a Selector with the given dependencies.
// All repo/service parameters may be nil; the selector degrades gracefully.
func New(
	providerRepo ProviderRepo,
	groupRepo ProviderGroupRepo,
	circuitBreaker CircuitBreakerService,
	sessionService SessionService,
	costChecker CostLimitChecker,
	concurrency ConcurrencyChecker,
	opts ...Option,
) *Selector {
	s := &Selector{
		providerRepo:   providerRepo,
		groupRepo:      groupRepo,
		circuitBreaker: circuitBreaker,
		sessionService: sessionService,
		costChecker:    costChecker,
		concurrency:    concurrency,
		nowFunc:        defaultNowFunc,
		timezoneFunc:   func() string { return "UTC" },
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Select runs the full 9-stage selection pipeline.
func (s *Selector) Select(ctx context.Context, req SelectRequest) (*SelectResult, error) {
	// Stage 1: Session reuse
	if reused, err := s.trySessionReuse(ctx, req); err == nil && reused != nil {
		groupCostMult := s.resolveGroupCostMultiplier(ctx, req.ProviderGroup)
		return &SelectResult{
			Provider:      reused,
			GroupCostMult: groupCostMult,
			SessionReused: true,
			AttemptCount:  1,
		}, nil
	}

	// Load providers
	providers, err := s.loadProviders(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("selector: failed to load providers: %w", err)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: no providers available")
	}

	// Stage 2: Group pre-filtering
	providers = FilterByGroup(providers, req.ProviderGroup)
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: no providers match group %q", req.ProviderGroup)
	}

	// Stage 3: Client restriction filtering
	providers = FilterByClientRestriction(providers, req.UserAgent)
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: no providers available after client restriction filtering")
	}

	// Stage 4: Basic filtering (enabled, schedule, format-type, model, exclude)
	tz := s.timezoneFunc()
	now := s.nowFunc()
	providers = FilterBasic(providers, req.Model, req.Format, req.ExcludeIDs, now, tz)
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: no providers available after basic filtering")
	}

	// Stage 5: Health filtering (circuit breaker + cost limits)
	providers, err = s.filterHealth(ctx, providers)
	if err != nil {
		return nil, fmt.Errorf("selector: health filtering error: %w", err)
	}
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: all providers unhealthy (circuit breaker or cost limits)")
	}

	// Stage 6: Priority tiers
	providers = SelectTopPriority(providers, req.ProviderGroup)
	if len(providers) == 0 {
		return nil, fmt.Errorf("selector: no providers after priority selection")
	}

	// Stage 7 + 8: Weighted random + concurrent session check with retry
	groupCostMult := s.resolveGroupCostMultiplier(ctx, req.ProviderGroup)
	result, err := s.selectWithConcurrencyCheck(ctx, req, providers, groupCostMult)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// trySessionReuse implements Stage 1.
func (s *Selector) trySessionReuse(ctx context.Context, req SelectRequest) (*model.Provider, error) {
	if s.sessionService == nil || req.SessionID == "" {
		return nil, nil
	}

	providerID, err := s.sessionService.GetBoundProvider(ctx, req.SessionID)
	if err != nil || providerID == 0 {
		return nil, nil
	}

	// Look up provider
	if s.providerRepo == nil {
		return nil, nil
	}
	provider, err := s.providerRepo.GetByID(ctx, providerID)
	if err != nil || provider == nil {
		return nil, nil
	}

	// Validate: enabled
	if provider.IsEnabled == nil || !*provider.IsEnabled {
		return nil, nil
	}

	// Validate: not opted out of session reuse
	if provider.DisableSessionReuse {
		return nil, nil
	}

	// Validate: schedule
	tz := s.timezoneFunc()
	now := s.nowFunc()
	if !IsProviderActiveNow(provider.ActiveTimeStart, provider.ActiveTimeEnd, now, tz) {
		return nil, nil
	}

	// Validate: circuit breaker
	if s.circuitBreaker != nil && s.circuitBreaker.IsOpen(provider) {
		return nil, nil
	}

	// Validate: model support
	if req.Model != "" && !provider.SupportsModel(req.Model) {
		return nil, nil
	}

	// Validate: format compatibility
	if req.Format != "" && !CheckFormatCompatibility(req.Format, provider.ProviderType) {
		return nil, nil
	}

	// Validate: group match
	if req.ProviderGroup != "" && !CheckProviderGroupMatch(provider.GroupTag, req.ProviderGroup) {
		return nil, nil
	}

	// Validate: client restriction
	if !IsClientAllowed(provider.AllowedClients, provider.BlockedClients, req.UserAgent) {
		return nil, nil
	}

	// Validate: cost limits
	if s.costChecker != nil {
		allowed, err := s.costChecker.CheckCostLimits(ctx, provider)
		if err != nil || !allowed {
			return nil, nil
		}
	}

	// Validate: not in exclude list
	for _, id := range req.ExcludeIDs {
		if id == provider.ID {
			return nil, nil
		}
	}

	return provider, nil
}

// loadProviders fetches providers from request or repo.
func (s *Selector) loadProviders(ctx context.Context, req SelectRequest) ([]*model.Provider, error) {
	if len(req.Providers) > 0 {
		return req.Providers, nil
	}
	if s.providerRepo == nil {
		return nil, fmt.Errorf("no providers supplied and no provider repo configured")
	}
	return s.providerRepo.GetActiveProviders(ctx)
}

// filterHealth implements Stage 5.
func (s *Selector) filterHealth(ctx context.Context, providers []*model.Provider) ([]*model.Provider, error) {
	var result []*model.Provider
	for _, p := range providers {
		// Circuit breaker
		if s.circuitBreaker != nil && s.circuitBreaker.IsOpen(p) {
			continue
		}
		// Cost limits
		if s.costChecker != nil {
			allowed, err := s.costChecker.CheckCostLimits(ctx, p)
			if err != nil {
				continue
			}
			if !allowed {
				continue
			}
		}
		result = append(result, p)
	}
	return result, nil
}

// selectWithConcurrencyCheck implements Stages 7+8.
func (s *Selector) selectWithConcurrencyCheck(
	ctx context.Context,
	req SelectRequest,
	providers []*model.Provider,
	groupCostMult udecimal.Decimal,
) (*SelectResult, error) {
	// If no concurrency checker or no session ID, just do weighted random
	if s.concurrency == nil || req.SessionID == "" {
		selected := SelectWeightedRandom(providers)
		if selected == nil {
			return nil, fmt.Errorf("selector: weighted random returned nil")
		}
		return &SelectResult{
			Provider:      selected,
			GroupCostMult: groupCostMult,
			SessionReused: false,
			AttemptCount:  1,
		}, nil
	}

	remaining := make([]*model.Provider, len(providers))
	copy(remaining, providers)

	for attempt := 1; attempt <= MaxProviderSwitches && len(remaining) > 0; attempt++ {
		selected := SelectWeightedRandom(remaining)
		if selected == nil {
			break
		}

		limit := 0
		if selected.LimitConcurrentSessions != nil {
			limit = *selected.LimitConcurrentSessions
		}

		// If no limit, accept immediately
		if limit <= 0 {
			return &SelectResult{
				Provider:      selected,
				GroupCostMult: groupCostMult,
				SessionReused: false,
				AttemptCount:  attempt,
			}, nil
		}

		allowed, _, err := s.concurrency.CheckAndTrack(ctx, selected.ID, req.SessionID, limit)
		if err != nil || !allowed {
			// Remove from candidates and retry
			remaining = removeProvider(remaining, selected.ID)
			continue
		}

		return &SelectResult{
			Provider:      selected,
			GroupCostMult: groupCostMult,
			SessionReused: false,
			AttemptCount:  attempt,
		}, nil
	}

	return nil, fmt.Errorf("selector: all providers exceeded concurrent session limit after %d attempts", MaxProviderSwitches)
}

// resolveGroupCostMultiplier looks up the cost multiplier for a provider group.
func (s *Selector) resolveGroupCostMultiplier(ctx context.Context, providerGroup string) udecimal.Decimal {
	one := udecimal.MustParse("1.0")
	if s.groupRepo == nil || providerGroup == "" {
		return one
	}

	mult, err := s.groupRepo.GetCostMultiplier(ctx, providerGroup)
	if err != nil {
		return one
	}
	if mult.IsZero() {
		return one
	}
	return mult
}

// removeProvider returns a new slice excluding the provider with the given ID.
func removeProvider(providers []*model.Provider, id int) []*model.Provider {
	result := make([]*model.Provider, 0, len(providers)-1)
	for _, p := range providers {
		if p.ID != id {
			result = append(result, p)
		}
	}
	return result
}
