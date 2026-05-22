package forwarder

import (
	"sync"
	"sync/atomic"

	"github.com/ding113/claude-code-hub/internal/model"
)

// EndpointPool manages a pool of endpoints for a given vendor+type combination.
// It provides round-robin selection and skipping of failed endpoints.
type EndpointPool struct {
	mu        sync.RWMutex
	endpoints []*model.ProviderEndpoint
	counter   atomic.Uint64
	failed    map[int]struct{} // set of failed endpoint IDs
}

// NewEndpointPool creates a pool from a list of endpoints.
// Endpoints are filtered to only include active ones, sorted by SortOrder.
func NewEndpointPool(endpoints []*model.ProviderEndpoint) *EndpointPool {
	active := make([]*model.ProviderEndpoint, 0, len(endpoints))
	for _, ep := range endpoints {
		if ep.IsActive() {
			active = append(active, ep)
		}
	}

	// Sort by sort_order (ascending)
	sortEndpoints(active)

	return &EndpointPool{
		endpoints: active,
		failed:    make(map[int]struct{}),
	}
}

// Next returns the next available endpoint using round-robin.
// Returns nil if all endpoints are failed or the pool is empty.
func (p *EndpointPool) Next() *model.ProviderEndpoint {
	p.mu.RLock()
	defer p.mu.RUnlock()

	total := len(p.endpoints)
	if total == 0 {
		return nil
	}

	// Try up to len(endpoints) times to find a non-failed one
	start := p.counter.Add(1) - 1
	for i := 0; i < total; i++ {
		idx := int((start + uint64(i)) % uint64(total))
		ep := p.endpoints[idx]
		if _, failed := p.failed[ep.ID]; !failed {
			return ep
		}
	}

	return nil // all failed
}

// MarkFailed marks an endpoint as failed, so it is skipped in future selections.
func (p *EndpointPool) MarkFailed(endpointID int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failed[endpointID] = struct{}{}
}

// MarkHealthy removes an endpoint from the failed set.
func (p *EndpointPool) MarkHealthy(endpointID int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.failed, endpointID)
}

// ResetFailed clears all failed marks.
func (p *EndpointPool) ResetFailed() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.failed = make(map[int]struct{})
}

// Len returns the total number of endpoints (including failed ones).
func (p *EndpointPool) Len() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.endpoints)
}

// AvailableCount returns the number of non-failed endpoints.
func (p *EndpointPool) AvailableCount() int {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return len(p.endpoints) - len(p.failed)
}

// sortEndpoints sorts endpoints by SortOrder ascending (stable).
func sortEndpoints(eps []*model.ProviderEndpoint) {
	// Simple insertion sort - endpoint lists are typically small
	for i := 1; i < len(eps); i++ {
		key := eps[i]
		j := i - 1
		for j >= 0 && eps[j].SortOrder > key.SortOrder {
			eps[j+1] = eps[j]
			j--
		}
		eps[j+1] = key
	}
}

// --------------------------------------------------------------------------
// EndpointPoolManager manages pools per vendor+type key
// --------------------------------------------------------------------------

// EndpointPoolManager manages multiple endpoint pools keyed by vendorID+providerType.
type EndpointPoolManager struct {
	mu    sync.RWMutex
	pools map[string]*EndpointPool
}

// NewEndpointPoolManager creates a new pool manager.
func NewEndpointPoolManager() *EndpointPoolManager {
	return &EndpointPoolManager{
		pools: make(map[string]*EndpointPool),
	}
}

// poolKey generates a unique key for a vendor+type combination.
func poolKey(vendorID int, providerType string) string {
	// Use a simple string key. We avoid fmt.Sprintf for performance.
	buf := make([]byte, 0, 32)
	buf = appendInt(buf, vendorID)
	buf = append(buf, ':')
	buf = append(buf, providerType...)
	return string(buf)
}

// appendInt appends the decimal representation of n to buf.
func appendInt(buf []byte, n int) []byte {
	if n == 0 {
		return append(buf, '0')
	}
	if n < 0 {
		buf = append(buf, '-')
		n = -n
	}
	// Stack digits then reverse
	start := len(buf)
	for n > 0 {
		buf = append(buf, byte('0'+n%10))
		n /= 10
	}
	// Reverse the digits
	for i, j := start, len(buf)-1; i < j; i, j = i+1, j-1 {
		buf[i], buf[j] = buf[j], buf[i]
	}
	return buf
}

// GetPool returns the endpoint pool for a vendor+type. Returns nil if not loaded.
func (m *EndpointPoolManager) GetPool(vendorID int, providerType string) *EndpointPool {
	key := poolKey(vendorID, providerType)
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.pools[key]
}

// SetPool sets the endpoint pool for a vendor+type.
func (m *EndpointPoolManager) SetPool(vendorID int, providerType string, pool *EndpointPool) {
	key := poolKey(vendorID, providerType)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.pools[key] = pool
}

// LoadEndpoints creates a pool from endpoints and stores it.
func (m *EndpointPoolManager) LoadEndpoints(vendorID int, providerType string, endpoints []*model.ProviderEndpoint) *EndpointPool {
	pool := NewEndpointPool(endpoints)
	m.SetPool(vendorID, providerType, pool)
	return pool
}

// GetEndpointURL returns the URL from the next available endpoint in the pool,
// or empty string if no pool or no available endpoint.
func (m *EndpointPoolManager) GetEndpointURL(vendorID int, providerType string) string {
	pool := m.GetPool(vendorID, providerType)
	if pool == nil {
		return ""
	}
	ep := pool.Next()
	if ep == nil {
		return ""
	}
	return ep.URL
}
