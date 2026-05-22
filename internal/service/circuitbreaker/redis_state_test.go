package circuitbreaker

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupMiniredis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})
	return client, mr
}

// --- Provider state tests ---

func TestRedisSync_SaveAndLoadProviderState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	state := BreakerState{
		ProviderID:   42,
		State:        StateOpen,
		FailureCount: 5,
		SuccessCount: 0,
		OpenedAt:     time.Now().Truncate(time.Millisecond),
		LastFailure:  time.Now().Truncate(time.Millisecond),
	}

	r.SaveProviderState(ctx, state, time.Minute)

	loaded, err := r.LoadProviderState(ctx, 42)
	require.NoError(t, err)
	require.NotNil(t, loaded)

	assert.Equal(t, state.ProviderID, loaded.ProviderID)
	assert.Equal(t, state.State, loaded.State)
	assert.Equal(t, state.FailureCount, loaded.FailureCount)
}

func TestRedisSync_LoadProviderState_NotFound(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	loaded, err := r.LoadProviderState(ctx, 999)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestRedisSync_DeleteProviderState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	state := BreakerState{ProviderID: 1, State: StateOpen, FailureCount: 3}
	r.SaveProviderState(ctx, state, time.Minute)

	r.DeleteProviderState(ctx, 1)

	loaded, err := r.LoadProviderState(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

func TestRedisSync_LoadAllProviderStates(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	for i := 1; i <= 3; i++ {
		r.SaveProviderState(ctx, BreakerState{
			ProviderID:   i,
			State:        StateOpen,
			FailureCount: i,
		}, time.Minute)
	}

	states, err := r.LoadAllProviderStates(ctx)
	require.NoError(t, err)
	assert.Len(t, states, 3)
}

func TestRedisSync_ProviderState_TTLExpiry(t *testing.T) {
	client, mr := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	// With minimum TTL of 5 minutes applied internally.
	r.SaveProviderState(ctx, BreakerState{
		ProviderID:   1,
		State:        StateOpen,
		FailureCount: 5,
	}, time.Second) // will be bumped to 5 min

	// FastForward past the TTL.
	mr.FastForward(6 * time.Minute)

	loaded, err := r.LoadProviderState(ctx, 1)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// --- Endpoint state tests ---

func TestRedisSync_SaveAndLoadEndpointState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	state := EndpointBreakerState{
		EndpointID:   10,
		State:        "open",
		FailureCount: 3,
		SkipUntil:    time.Now().Add(time.Minute).Truncate(time.Millisecond),
	}

	r.SaveEndpointState(ctx, state, 10*time.Minute)

	loaded, err := r.LoadEndpointState(ctx, 10)
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, 10, loaded.EndpointID)
	assert.Equal(t, "open", loaded.State)
	assert.Equal(t, 3, loaded.FailureCount)
}

func TestRedisSync_DeleteEndpointState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	r.SaveEndpointState(ctx, EndpointBreakerState{
		EndpointID: 5, State: "open", FailureCount: 1,
	}, 10*time.Minute)

	r.DeleteEndpointState(ctx, 5)

	loaded, err := r.LoadEndpointState(ctx, 5)
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// --- Vendor+Type state tests ---

func TestRedisSync_SaveAndLoadVendorTypeState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	state := VendorTypeState{
		VendorID:     1,
		ProviderType: "claude",
		State:        "open",
		OpenUntil:    time.Now().Add(30 * time.Second).Truncate(time.Millisecond),
		ManualOpen:   false,
	}

	r.SaveVendorTypeState(ctx, state, 2*time.Minute)

	loaded, err := r.LoadVendorTypeState(ctx, 1, "claude")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	assert.Equal(t, 1, loaded.VendorID)
	assert.Equal(t, "claude", loaded.ProviderType)
	assert.Equal(t, "open", loaded.State)
}

func TestRedisSync_DeleteVendorTypeState(t *testing.T) {
	client, _ := setupMiniredis(t)
	r := NewRedisStateSync(client)
	ctx := context.Background()

	r.SaveVendorTypeState(ctx, VendorTypeState{
		VendorID: 1, ProviderType: "claude", State: "open",
	}, 2*time.Minute)

	r.DeleteVendorTypeState(ctx, 1, "claude")

	loaded, err := r.LoadVendorTypeState(ctx, 1, "claude")
	require.NoError(t, err)
	assert.Nil(t, loaded)
}

// --- Nil client (fail-open) tests ---

func TestRedisSync_NilClient_FailOpen(t *testing.T) {
	r := NewRedisStateSync(nil)
	ctx := context.Background()

	assert.False(t, r.Available())

	// All operations should be no-ops without panics.
	r.SaveProviderState(ctx, BreakerState{ProviderID: 1}, time.Minute)
	loaded, err := r.LoadProviderState(ctx, 1)
	assert.NoError(t, err)
	assert.Nil(t, loaded)

	r.DeleteProviderState(ctx, 1)

	states, err := r.LoadAllProviderStates(ctx)
	assert.NoError(t, err)
	assert.Nil(t, states)

	r.SaveEndpointState(ctx, EndpointBreakerState{EndpointID: 1}, time.Minute)
	eLoaded, err := r.LoadEndpointState(ctx, 1)
	assert.NoError(t, err)
	assert.Nil(t, eLoaded)

	r.DeleteEndpointState(ctx, 1)

	r.SaveVendorTypeState(ctx, VendorTypeState{VendorID: 1, ProviderType: "claude"}, time.Minute)
	vtLoaded, err := r.LoadVendorTypeState(ctx, 1, "claude")
	assert.NoError(t, err)
	assert.Nil(t, vtLoaded)

	r.DeleteVendorTypeState(ctx, 1, "claude")
}
