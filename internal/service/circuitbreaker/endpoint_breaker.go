package circuitbreaker

import (
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

const (
	DefaultEndpointSkipDuration = 5 * time.Minute
	DefaultEndpointFailureThreshold = 3
)

// EndpointBreakerConfig holds per-endpoint circuit breaker configuration.
type EndpointBreakerConfig struct {
	FailureThreshold int
	SkipDuration     time.Duration
}

// endpointState is a simplified breaker for individual endpoints.
// It tracks failure count and a skip-until timestamp. Once failures reach
// the threshold, the endpoint is skipped for SkipDuration.
type endpointState struct {
	failureCount int
	lastFailure  time.Time
	skipUntil    time.Time
	successCount int
}

// EndpointBreakerManager manages per-endpoint circuit breakers.
type EndpointBreakerManager struct {
	mu     sync.RWMutex
	states map[int]*endpointState
	config EndpointBreakerConfig

	nowFunc func() time.Time
}

// NewEndpointBreakerManager creates a new endpoint breaker manager.
func NewEndpointBreakerManager(cfg ...EndpointBreakerConfig) *EndpointBreakerManager {
	c := EndpointBreakerConfig{
		FailureThreshold: DefaultEndpointFailureThreshold,
		SkipDuration:     DefaultEndpointSkipDuration,
	}
	if len(cfg) > 0 {
		if cfg[0].FailureThreshold > 0 {
			c.FailureThreshold = cfg[0].FailureThreshold
		}
		if cfg[0].SkipDuration > 0 {
			c.SkipDuration = cfg[0].SkipDuration
		}
	}
	return &EndpointBreakerManager{
		states:  make(map[int]*endpointState),
		config:  c,
		nowFunc: time.Now,
	}
}

// IsOpen reports whether an endpoint should be skipped.
func (m *EndpointBreakerManager) IsOpen(endpointID int) bool {
	m.mu.RLock()
	s, ok := m.states[endpointID]
	m.mu.RUnlock()
	if !ok {
		return false
	}

	now := m.nowFunc()
	if !s.skipUntil.IsZero() && now.Before(s.skipUntil) {
		return true
	}

	// If skip has expired, auto-recover on next check.
	if !s.skipUntil.IsZero() && now.After(s.skipUntil) {
		m.mu.Lock()
		if s2, ok2 := m.states[endpointID]; ok2 && !s2.skipUntil.IsZero() && now.After(s2.skipUntil) {
			// Transition to recovery: reset skip but keep failure info for monitoring.
			s2.skipUntil = time.Time{}
			s2.successCount = 0
		}
		m.mu.Unlock()
		return false
	}

	return false
}

// RecordFailure records a failure for an endpoint. When the threshold is
// reached the endpoint is marked as skipped for the configured duration.
func (m *EndpointBreakerManager) RecordFailure(endpointID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s := m.getOrCreateLocked(endpointID)
	now := m.nowFunc()
	s.failureCount++
	s.lastFailure = now
	s.successCount = 0

	if s.failureCount >= m.config.FailureThreshold {
		s.skipUntil = now.Add(m.config.SkipDuration)
		logger.Warn().Int("endpointId", endpointID).
			Int("failures", s.failureCount).
			Msg("[EndpointBreaker] Endpoint circuit opened")
	}
}

// RecordSuccess records a success for an endpoint. Resets the failure counter.
func (m *EndpointBreakerManager) RecordSuccess(endpointID int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	s, ok := m.states[endpointID]
	if !ok {
		return
	}

	s.successCount++
	s.failureCount = 0
	s.skipUntil = time.Time{}
}

// Reset resets the endpoint breaker state.
func (m *EndpointBreakerManager) Reset(endpointID int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.states, endpointID)
}

// GetState returns a serializable snapshot of endpoint breaker state.
func (m *EndpointBreakerManager) GetState(endpointID int) *EndpointBreakerState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	s, ok := m.states[endpointID]
	if !ok {
		return nil
	}

	state := "closed"
	now := m.nowFunc()
	if !s.skipUntil.IsZero() && now.Before(s.skipUntil) {
		state = "open"
	}

	return &EndpointBreakerState{
		EndpointID:   endpointID,
		State:        state,
		FailureCount: s.failureCount,
		SkipUntil:    s.skipUntil,
		LastFailure:  s.lastFailure,
	}
}

// EndpointBreakerState is a JSON-serializable snapshot for Redis sync / status API.
type EndpointBreakerState struct {
	EndpointID   int       `json:"endpointId"`
	State        string    `json:"state"`
	FailureCount int       `json:"failureCount"`
	SkipUntil    time.Time `json:"skipUntil,omitempty"`
	LastFailure  time.Time `json:"lastFailure,omitempty"`
}

// ResetAll clears all state (for testing).
func (m *EndpointBreakerManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states = make(map[int]*endpointState)
}

func (m *EndpointBreakerManager) getOrCreateLocked(endpointID int) *endpointState {
	s, ok := m.states[endpointID]
	if !ok {
		s = &endpointState{}
		m.states[endpointID] = s
	}
	return s
}
