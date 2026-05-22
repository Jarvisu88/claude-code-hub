package circuitbreaker

import (
	"sync"
	"time"
)

// State represents the circuit breaker state.
type State string

const (
	StateClosed   State = "closed"
	StateOpen     State = "open"
	StateHalfOpen State = "half_open"
)

const (
	DefaultFailureThreshold         = 5
	DefaultOpenDuration             = 60 * time.Second
	DefaultHalfOpenSuccessThreshold = 2
)

// BreakerConfig holds the configuration for a single breaker instance.
type BreakerConfig struct {
	FailureThreshold         int
	OpenDuration             time.Duration
	HalfOpenSuccessThreshold int
}

// DefaultBreakerConfig returns default configuration values.
func DefaultBreakerConfig() BreakerConfig {
	return BreakerConfig{
		FailureThreshold:         DefaultFailureThreshold,
		OpenDuration:             DefaultOpenDuration,
		HalfOpenSuccessThreshold: DefaultHalfOpenSuccessThreshold,
	}
}

// BreakerState is a JSON-serializable snapshot of a breaker's runtime state.
// Used for Redis sync and status reporting.
type BreakerState struct {
	ProviderID   int       `json:"providerId"`
	State        State     `json:"state"`
	FailureCount int       `json:"failureCount"`
	SuccessCount int       `json:"successCount"`
	LastFailure  time.Time `json:"lastFailure,omitempty"`
	OpenedAt     time.Time `json:"openedAt,omitempty"`
}

// Breaker implements a thread-safe circuit breaker state machine.
//
// State transitions:
//   - Closed -> Open:      when FailureCount >= FailureThreshold
//   - Open -> HalfOpen:    when OpenDuration has elapsed
//   - HalfOpen -> Closed:  when SuccessCount >= HalfOpenSuccessThreshold
//   - HalfOpen -> Open:    when a failure is recorded
type Breaker struct {
	mu sync.RWMutex

	providerID   int
	state        State
	failureCount int
	successCount int // tracked in half-open state
	lastFailure  time.Time
	openedAt     time.Time

	config BreakerConfig

	// nowFunc is overridable for testing.
	nowFunc func() time.Time
}

// NewBreaker creates a new Breaker with the given provider ID and config.
func NewBreaker(providerID int, cfg BreakerConfig) *Breaker {
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = DefaultFailureThreshold
	}
	if cfg.OpenDuration <= 0 {
		cfg.OpenDuration = DefaultOpenDuration
	}
	if cfg.HalfOpenSuccessThreshold <= 0 {
		cfg.HalfOpenSuccessThreshold = DefaultHalfOpenSuccessThreshold
	}
	return &Breaker{
		providerID: providerID,
		state:      StateClosed,
		config:     cfg,
		nowFunc:    time.Now,
	}
}

// RecordSuccess records a successful request.
// In half-open state, once enough consecutive successes accumulate, transitions to closed.
// In closed state, resets the failure counter.
func (b *Breaker) RecordSuccess() {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		b.failureCount = 0
	case StateHalfOpen:
		b.successCount++
		if b.successCount >= b.config.HalfOpenSuccessThreshold {
			b.reset()
		}
	case StateOpen:
		// Ignore successes while open (shouldn't normally happen).
	}
}

// RecordFailure records a failed request.
// In closed state, increments failure count and trips open if threshold is reached.
// In half-open state, immediately trips back to open.
func (b *Breaker) RecordFailure(err error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := b.nowFunc()
	b.lastFailure = now

	switch b.state {
	case StateClosed:
		b.failureCount++
		if b.failureCount >= b.config.FailureThreshold {
			b.tripOpen(now)
		}
	case StateHalfOpen:
		// Any failure in half-open re-opens the circuit.
		b.tripOpen(now)
	case StateOpen:
		// Already open; update failure count for monitoring.
		b.failureCount++
	}
}

// IsOpen returns true if the breaker is in the open state and the open
// duration has not yet elapsed.
func (b *Breaker) IsOpen() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state == StateOpen && !b.shouldTransitionToHalfOpen()
}

// ShouldAllow reports whether a request should be allowed through.
//   - Closed: always allows
//   - Open: blocks unless open duration has elapsed (transitions to half-open)
//   - HalfOpen: allows (limited probing traffic)
func (b *Breaker) ShouldAllow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()

	switch b.state {
	case StateClosed:
		return true
	case StateOpen:
		if b.shouldTransitionToHalfOpen() {
			b.state = StateHalfOpen
			b.successCount = 0
			return true
		}
		return false
	case StateHalfOpen:
		return true
	default:
		return true
	}
}

// Reset forces the breaker back to closed state.
func (b *Breaker) Reset() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.reset()
}

// GetState returns a snapshot of the breaker's current state.
func (b *Breaker) GetState() BreakerState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Check if we should logically be in half-open (read-only snapshot).
	state := b.state
	if state == StateOpen && b.shouldTransitionToHalfOpen() {
		state = StateHalfOpen
	}

	return BreakerState{
		ProviderID:   b.providerID,
		State:        state,
		FailureCount: b.failureCount,
		SuccessCount: b.successCount,
		LastFailure:  b.lastFailure,
		OpenedAt:     b.openedAt,
	}
}

// LoadState restores a breaker's state from a snapshot (e.g. from Redis).
func (b *Breaker) LoadState(s BreakerState) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.state = s.State
	b.failureCount = s.FailureCount
	b.successCount = s.SuccessCount
	b.lastFailure = s.LastFailure
	b.openedAt = s.OpenedAt
}

// CurrentState returns the raw state enum (for external checks without snapshot).
func (b *Breaker) CurrentState() State {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.state
}

// --- internal helpers ---

func (b *Breaker) tripOpen(now time.Time) {
	b.state = StateOpen
	b.openedAt = now
	b.successCount = 0
}

func (b *Breaker) reset() {
	b.state = StateClosed
	b.failureCount = 0
	b.successCount = 0
	b.lastFailure = time.Time{}
	b.openedAt = time.Time{}
}

// shouldTransitionToHalfOpen checks whether the open duration has elapsed.
// Must be called while holding at least a read lock.
func (b *Breaker) shouldTransitionToHalfOpen() bool {
	if b.state != StateOpen {
		return false
	}
	if b.openedAt.IsZero() {
		return true
	}
	return b.nowFunc().After(b.openedAt.Add(b.config.OpenDuration))
}
