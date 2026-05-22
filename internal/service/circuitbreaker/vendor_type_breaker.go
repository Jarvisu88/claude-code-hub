package circuitbreaker

import (
	"fmt"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

const (
	DefaultVendorTypeOpenDuration = 30 * time.Second
)

// VendorTypeState represents the state of a vendor+type breaker.
type VendorTypeState struct {
	VendorID     int       `json:"vendorId"`
	ProviderType string    `json:"providerType"`
	State        string    `json:"state"` // "closed" or "open"
	OpenUntil    time.Time `json:"openUntil,omitempty"`
	LastFailure  time.Time `json:"lastFailure,omitempty"`
	ManualOpen   bool      `json:"manualOpen"`
}

// vendorTypeEntry is the internal mutable state.
type vendorTypeEntry struct {
	state       string // "closed" or "open"
	openUntil   time.Time
	lastFailure time.Time
	manualOpen  bool
}

// VendorTypeBreakerManager manages vendor+type level circuit breakers.
// Triggered when ALL endpoints of a vendor+type combination timeout simultaneously.
// Short-lived (30s default), auto-recovers. Prevents cascading failures.
type VendorTypeBreakerManager struct {
	mu      sync.RWMutex
	entries map[string]*vendorTypeEntry

	defaultOpenDuration time.Duration
	nowFunc             func() time.Time
}

// NewVendorTypeBreakerManager creates a new vendor+type breaker manager.
func NewVendorTypeBreakerManager() *VendorTypeBreakerManager {
	return &VendorTypeBreakerManager{
		entries:             make(map[string]*vendorTypeEntry),
		defaultOpenDuration: DefaultVendorTypeOpenDuration,
		nowFunc:             time.Now,
	}
}

// IsOpen reports whether the vendor+type circuit is open.
func (m *VendorTypeBreakerManager) IsOpen(vendorID int, providerType string) bool {
	key := vendorTypeKey(vendorID, providerType)

	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()

	if !ok {
		return false
	}

	if e.manualOpen {
		return true
	}

	if e.state != "open" {
		return false
	}

	now := m.nowFunc()
	if !e.openUntil.IsZero() && now.After(e.openUntil) {
		// Auto-recover.
		m.mu.Lock()
		if e2, ok2 := m.entries[key]; ok2 && e2.state == "open" && !e2.manualOpen &&
			!e2.openUntil.IsZero() && now.After(e2.openUntil) {
			e2.state = "closed"
			e2.openUntil = time.Time{}
		}
		m.mu.Unlock()
		return false
	}

	return true
}

// TriggerOpen opens the vendor+type breaker. Called when all endpoints of
// a vendor+type timeout simultaneously.
func (m *VendorTypeBreakerManager) TriggerOpen(vendorID int, providerType string, openDuration ...time.Duration) {
	dur := m.defaultOpenDuration
	if len(openDuration) > 0 && openDuration[0] > 0 {
		dur = openDuration[0]
	}

	key := vendorTypeKey(vendorID, providerType)
	now := m.nowFunc()

	m.mu.Lock()
	defer m.mu.Unlock()

	e := m.getOrCreateLocked(key)
	if e.manualOpen {
		return
	}

	e.state = "open"
	e.lastFailure = now
	e.openUntil = now.Add(dur)

	logger.Warn().
		Int("vendorId", vendorID).
		Str("providerType", providerType).
		Str("openUntil", e.openUntil.Format(time.RFC3339)).
		Msg("[VendorTypeBreaker] Vendor+type circuit opened")
}

// SetManualOpen manually opens or closes the vendor+type breaker.
func (m *VendorTypeBreakerManager) SetManualOpen(vendorID int, providerType string, manualOpen bool) {
	key := vendorTypeKey(vendorID, providerType)
	now := m.nowFunc()

	m.mu.Lock()
	defer m.mu.Unlock()

	e := m.getOrCreateLocked(key)
	e.manualOpen = manualOpen
	if manualOpen {
		e.state = "open"
		e.openUntil = time.Time{} // Manual open has no auto-expire.
		e.lastFailure = now
	} else {
		e.state = "closed"
		e.openUntil = time.Time{}
	}
}

// Reset removes the vendor+type breaker state.
func (m *VendorTypeBreakerManager) Reset(vendorID int, providerType string) {
	key := vendorTypeKey(vendorID, providerType)
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.entries, key)
}

// GetState returns a snapshot of the vendor+type breaker.
func (m *VendorTypeBreakerManager) GetState(vendorID int, providerType string) *VendorTypeState {
	key := vendorTypeKey(vendorID, providerType)
	m.mu.RLock()
	e, ok := m.entries[key]
	m.mu.RUnlock()
	if !ok {
		return nil
	}

	state := e.state
	now := m.nowFunc()
	if state == "open" && !e.manualOpen && !e.openUntil.IsZero() && now.After(e.openUntil) {
		state = "closed"
	}

	return &VendorTypeState{
		VendorID:     vendorID,
		ProviderType: providerType,
		State:        state,
		OpenUntil:    e.openUntil,
		LastFailure:  e.lastFailure,
		ManualOpen:   e.manualOpen,
	}
}

// ResetAll clears all state (for testing).
func (m *VendorTypeBreakerManager) ResetAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.entries = make(map[string]*vendorTypeEntry)
}

func (m *VendorTypeBreakerManager) getOrCreateLocked(key string) *vendorTypeEntry {
	e, ok := m.entries[key]
	if !ok {
		e = &vendorTypeEntry{state: "closed"}
		m.entries[key] = e
	}
	return e
}

func vendorTypeKey(vendorID int, providerType string) string {
	return fmt.Sprintf("vendor_%d_type_%s", vendorID, providerType)
}
