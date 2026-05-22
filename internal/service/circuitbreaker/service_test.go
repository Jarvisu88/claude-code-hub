package circuitbreaker

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestService(t *testing.T) (*Service, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		client.Close()
		mr.Close()
	})

	svc := NewService(ServiceConfig{
		RedisClient:        client,
		CountNetworkErrors: true,
	})
	return svc, mr
}

func newTestServiceNoRedis() *Service {
	return NewService(ServiceConfig{
		CountNetworkErrors: true,
	})
}

// --- Provider level ---

func TestService_ProviderOpenClose(t *testing.T) {
	svc, _ := newTestService(t)

	threshold := 2
	p := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	assert.False(t, svc.IsProviderOpenForModel(p))

	svc.RecordProviderFailureForModel(p, false, nil)
	svc.RecordProviderFailureForModel(p, false, nil)

	assert.True(t, svc.IsProviderOpen(1))

	s := svc.GetProviderState(1)
	assert.NotNil(t, s)
	assert.Equal(t, StateOpen, s.State)
}

func TestService_ProviderReset(t *testing.T) {
	svc, _ := newTestService(t)

	threshold := 1
	p := &model.Provider{
		ID:                             1,
		Name:                           "test",
		CircuitBreakerFailureThreshold: &threshold,
	}

	svc.RecordProviderFailureForModel(p, false, nil)
	assert.True(t, svc.IsProviderOpen(1))

	svc.ResetProvider(1)
	assert.False(t, svc.IsProviderOpen(1))
}

func TestService_ProviderSuccess(t *testing.T) {
	svc := newTestServiceNoRedis()

	threshold := 1
	halfOpen := 1
	dur := 50 // ms
	p := &model.Provider{
		ID:                                     1,
		Name:                                   "test",
		CircuitBreakerFailureThreshold:         &threshold,
		CircuitBreakerOpenDuration:             &dur,
		CircuitBreakerHalfOpenSuccessThreshold: &halfOpen,
	}

	svc.RecordProviderFailureForModel(p, false, nil)
	assert.True(t, svc.IsProviderOpen(1))

	// Manipulate internal breaker time to go half-open.
	b := svc.ProviderBreakers().GetBreaker(1)
	b.mu.Lock()
	b.nowFunc = func() time.Time { return time.Now().Add(100 * time.Millisecond) }
	b.mu.Unlock()

	// ShouldAllow transitions to half-open.
	assert.False(t, svc.IsProviderOpen(1))

	svc.RecordProviderSuccess(1)
	assert.False(t, svc.IsProviderOpen(1))

	s := svc.GetProviderState(1)
	assert.Equal(t, StateClosed, s.State)
}

// --- Endpoint level ---

func TestService_EndpointOpenClose(t *testing.T) {
	svc := newTestServiceNoRedis()

	assert.False(t, svc.IsEndpointOpen(1))

	// Default threshold is 3.
	svc.RecordEndpointFailure(1)
	svc.RecordEndpointFailure(1)
	svc.RecordEndpointFailure(1)

	assert.True(t, svc.IsEndpointOpen(1))

	svc.ResetEndpoint(1)
	assert.False(t, svc.IsEndpointOpen(1))
}

func TestService_EndpointSuccess(t *testing.T) {
	svc := newTestServiceNoRedis()

	svc.RecordEndpointFailure(1)
	svc.RecordEndpointFailure(1)
	svc.RecordEndpointSuccess(1)

	// Should reset the failure count.
	svc.RecordEndpointFailure(1)
	svc.RecordEndpointFailure(1)
	assert.False(t, svc.IsEndpointOpen(1))
}

func TestService_GetEndpointState(t *testing.T) {
	svc := newTestServiceNoRedis()
	assert.Nil(t, svc.GetEndpointState(999))

	svc.RecordEndpointFailure(1)
	s := svc.GetEndpointState(1)
	assert.NotNil(t, s)
	assert.Equal(t, "closed", s.State)
}

// --- Vendor+Type level ---

func TestService_VendorTypeOpenClose(t *testing.T) {
	svc := newTestServiceNoRedis()

	assert.False(t, svc.IsVendorTypeOpen(1, "claude"))

	svc.TriggerVendorTypeBreaker(1, "claude")
	assert.True(t, svc.IsVendorTypeOpen(1, "claude"))

	svc.ResetVendorType(1, "claude")
	assert.False(t, svc.IsVendorTypeOpen(1, "claude"))
}

func TestService_GetVendorTypeState(t *testing.T) {
	svc := newTestServiceNoRedis()
	assert.Nil(t, svc.GetVendorTypeState(1, "claude"))

	svc.TriggerVendorTypeBreaker(1, "claude")
	s := svc.GetVendorTypeState(1, "claude")
	assert.NotNil(t, s)
	assert.Equal(t, "open", s.State)
}

// --- Redis sync / loader ---

func TestService_LoadFromRedis(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Pre-populate Redis with an open breaker state.
	state := BreakerState{
		ProviderID:   42,
		State:        StateOpen,
		FailureCount: 10,
		OpenedAt:     time.Now(),
	}
	svc.RedisSync().SaveProviderState(ctx, state, 10*time.Minute)

	// Create a fresh service and load.
	svc2, _ := newTestService(t)
	// Copy the Redis client to the new service.
	svc2.redisSync = svc.redisSync

	err := svc2.LoadFromRedis(ctx)
	require.NoError(t, err)

	// The breaker should be loaded.
	s := svc2.GetProviderState(42)
	require.NotNil(t, s)
	assert.Equal(t, StateOpen, s.State)
	assert.Equal(t, 10, s.FailureCount)
}

func TestService_LoadFromRedis_NoRedis(t *testing.T) {
	svc := newTestServiceNoRedis()
	err := svc.LoadFromRedis(context.Background())
	assert.NoError(t, err) // Should be a no-op.
}

func TestService_LoadFromRedis_SkipsClosed(t *testing.T) {
	svc, _ := newTestService(t)
	ctx := context.Background()

	// Save a closed state.
	state := BreakerState{
		ProviderID:   1,
		State:        StateClosed,
		FailureCount: 0,
	}
	svc.RedisSync().SaveProviderState(ctx, state, 10*time.Minute)

	svc2 := newTestServiceNoRedis()
	svc2.redisSync = svc.redisSync
	err := svc2.LoadFromRedis(ctx)
	require.NoError(t, err)

	// No breaker should be created for a closed state.
	assert.Nil(t, svc2.GetProviderState(1))
}

// --- SubManager access ---

func TestService_SubManagersNotNil(t *testing.T) {
	svc := newTestServiceNoRedis()
	assert.NotNil(t, svc.ProviderBreakers())
	assert.NotNil(t, svc.EndpointBreakers())
	assert.NotNil(t, svc.VendorTypeBreakers())
	assert.NotNil(t, svc.RedisSync())
}

// --- Concurrent usage of service ---

func TestService_ConcurrentOperations(t *testing.T) {
	svc := newTestServiceNoRedis()

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				svc.RecordProviderFailure(id%3+1, nil)
				svc.IsProviderOpen(id%3 + 1)
				svc.RecordProviderSuccess(id%3 + 1)
				svc.RecordEndpointFailure(id%5 + 1)
				svc.IsEndpointOpen(id%5 + 1)
				svc.RecordEndpointSuccess(id%5 + 1)
				svc.TriggerVendorTypeBreaker(id%2+1, "claude")
				svc.IsVendorTypeOpen(id%2+1, "claude")
			}
		}(i)
	}
	wg.Wait()
}

// --- Nil provider safety ---

func TestService_NilProviderSafety(t *testing.T) {
	svc := newTestServiceNoRedis()

	// None of these should panic.
	assert.False(t, svc.IsProviderOpenForModel(nil))
	svc.RecordProviderFailureForModel(nil, false, nil)
	svc.RecordProviderSuccessForModel(nil)
}
