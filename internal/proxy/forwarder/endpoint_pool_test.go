package forwarder

import (
	"testing"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewEndpointPool_FiltersInactive(t *testing.T) {
	disabled := false
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 1},
		{ID: 2, URL: "http://ep2.test", IsEnabled: disabled}, // disabled
		{ID: 3, URL: "http://ep3.test", IsEnabled: true, SortOrder: 0},
	}

	pool := NewEndpointPool(endpoints)
	assert.Equal(t, 2, pool.Len())
}

func TestNewEndpointPool_SortsBySortOrder(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 3},
		{ID: 2, URL: "http://ep2.test", IsEnabled: true, SortOrder: 1},
		{ID: 3, URL: "http://ep3.test", IsEnabled: true, SortOrder: 2},
	}

	pool := NewEndpointPool(endpoints)
	require.Equal(t, 3, pool.Len())

	// First Next() should return the lowest sort_order endpoint
	ep := pool.Next()
	require.NotNil(t, ep)
	assert.Equal(t, 2, ep.ID)
}

func TestEndpointPool_RoundRobin(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
		{ID: 2, URL: "http://ep2.test", IsEnabled: true, SortOrder: 1},
		{ID: 3, URL: "http://ep3.test", IsEnabled: true, SortOrder: 2},
	}

	pool := NewEndpointPool(endpoints)

	// Get all three in sequence
	seen := make(map[int]int)
	for i := 0; i < 6; i++ {
		ep := pool.Next()
		require.NotNil(t, ep)
		seen[ep.ID]++
	}

	// Each should be seen exactly twice
	assert.Equal(t, 2, seen[1])
	assert.Equal(t, 2, seen[2])
	assert.Equal(t, 2, seen[3])
}

func TestEndpointPool_MarkFailed(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
		{ID: 2, URL: "http://ep2.test", IsEnabled: true, SortOrder: 1},
	}

	pool := NewEndpointPool(endpoints)

	// Mark ep1 as failed
	pool.MarkFailed(1)
	assert.Equal(t, 1, pool.AvailableCount())

	// All calls should return ep2
	for i := 0; i < 5; i++ {
		ep := pool.Next()
		require.NotNil(t, ep)
		assert.Equal(t, 2, ep.ID)
	}
}

func TestEndpointPool_AllFailed(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
		{ID: 2, URL: "http://ep2.test", IsEnabled: true, SortOrder: 1},
	}

	pool := NewEndpointPool(endpoints)
	pool.MarkFailed(1)
	pool.MarkFailed(2)

	ep := pool.Next()
	assert.Nil(t, ep)
	assert.Equal(t, 0, pool.AvailableCount())
}

func TestEndpointPool_MarkHealthy(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
	}

	pool := NewEndpointPool(endpoints)
	pool.MarkFailed(1)
	assert.Nil(t, pool.Next())

	pool.MarkHealthy(1)
	ep := pool.Next()
	require.NotNil(t, ep)
	assert.Equal(t, 1, ep.ID)
}

func TestEndpointPool_ResetFailed(t *testing.T) {
	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
		{ID: 2, URL: "http://ep2.test", IsEnabled: true, SortOrder: 1},
	}

	pool := NewEndpointPool(endpoints)
	pool.MarkFailed(1)
	pool.MarkFailed(2)
	assert.Equal(t, 0, pool.AvailableCount())

	pool.ResetFailed()
	assert.Equal(t, 2, pool.AvailableCount())
}

func TestEndpointPool_Empty(t *testing.T) {
	pool := NewEndpointPool(nil)
	assert.Equal(t, 0, pool.Len())
	assert.Nil(t, pool.Next())
}

func TestEndpointPoolManager_GetSet(t *testing.T) {
	mgr := NewEndpointPoolManager()

	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
	}

	pool := mgr.LoadEndpoints(1, "claude", endpoints)
	assert.NotNil(t, pool)

	got := mgr.GetPool(1, "claude")
	assert.Equal(t, pool, got)

	// Different vendor should be nil
	assert.Nil(t, mgr.GetPool(2, "claude"))
}

func TestEndpointPoolManager_GetEndpointURL(t *testing.T) {
	mgr := NewEndpointPoolManager()

	endpoints := []*model.ProviderEndpoint{
		{ID: 1, URL: "http://ep1.test", IsEnabled: true, SortOrder: 0},
	}
	mgr.LoadEndpoints(1, "claude", endpoints)

	url := mgr.GetEndpointURL(1, "claude")
	assert.Equal(t, "http://ep1.test", url)

	// Non-existent pool
	url2 := mgr.GetEndpointURL(99, "openai")
	assert.Equal(t, "", url2)
}

func TestPoolKey(t *testing.T) {
	k1 := poolKey(1, "claude")
	k2 := poolKey(1, "openai")
	k3 := poolKey(2, "claude")

	assert.NotEqual(t, k1, k2)
	assert.NotEqual(t, k1, k3)
	assert.NotEqual(t, k2, k3)
}

func TestAppendInt(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{12345, "12345"},
	}
	for _, tt := range tests {
		got := string(appendInt(nil, tt.n))
		assert.Equal(t, tt.want, got)
	}
}
