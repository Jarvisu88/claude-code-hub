package circuitbreaker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes.
	redisProviderKeyPrefix   = "cb:provider:"
	redisEndpointKeyPrefix   = "cb:endpoint:"
	redisVendorTypeKeyPrefix = "cb:vendortype:"
)

// RedisStateSync synchronises circuit breaker state with Redis for
// multi-instance deployments. It follows a fail-open strategy: if Redis
// is unavailable the in-memory breakers continue to operate normally.
type RedisStateSync struct {
	client *redis.Client
}

// NewRedisStateSync creates a new Redis state synchroniser.
// If client is nil, all operations become no-ops (fail-open).
func NewRedisStateSync(client *redis.Client) *RedisStateSync {
	return &RedisStateSync{client: client}
}

// Available reports whether Redis sync is operational.
func (r *RedisStateSync) Available() bool {
	return r != nil && r.client != nil
}

// --- Provider state ---

// SaveProviderState persists a provider breaker state to Redis.
// TTL is set to openDuration * 2 (or a minimum of 5 minutes) for auto-cleanup.
func (r *RedisStateSync) SaveProviderState(ctx context.Context, state BreakerState, openDuration time.Duration) {
	if !r.Available() {
		return
	}

	data, err := json.Marshal(state)
	if err != nil {
		logger.Warn().Err(err).Int("providerId", state.ProviderID).
			Msg("[RedisSync] Failed to marshal provider state")
		return
	}

	ttl := openDuration * 2
	if ttl < 5*time.Minute {
		ttl = 5 * time.Minute
	}

	key := providerRedisKey(state.ProviderID)
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		logger.Warn().Err(err).Int("providerId", state.ProviderID).
			Msg("[RedisSync] Failed to save provider state")
	}
}

// LoadProviderState loads a single provider's state from Redis.
func (r *RedisStateSync) LoadProviderState(ctx context.Context, providerID int) (*BreakerState, error) {
	if !r.Available() {
		return nil, nil
	}

	key := providerRedisKey(providerID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var state BreakerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal provider state: %w", err)
	}
	return &state, nil
}

// DeleteProviderState removes a provider's state from Redis.
func (r *RedisStateSync) DeleteProviderState(ctx context.Context, providerID int) {
	if !r.Available() {
		return
	}
	key := providerRedisKey(providerID)
	if err := r.client.Del(ctx, key).Err(); err != nil {
		logger.Warn().Err(err).Int("providerId", providerID).
			Msg("[RedisSync] Failed to delete provider state")
	}
}

// LoadAllProviderStates scans Redis for all stored provider breaker states.
func (r *RedisStateSync) LoadAllProviderStates(ctx context.Context) ([]BreakerState, error) {
	if !r.Available() {
		return nil, nil
	}

	pattern := redisProviderKeyPrefix + "*"
	var states []BreakerState
	iter := r.client.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		data, err := r.client.Get(ctx, iter.Val()).Bytes()
		if err != nil {
			continue
		}
		var s BreakerState
		if err := json.Unmarshal(data, &s); err != nil {
			continue
		}
		states = append(states, s)
	}
	if err := iter.Err(); err != nil {
		return states, fmt.Errorf("redis scan provider states: %w", err)
	}
	return states, nil
}

// --- Endpoint state ---

// SaveEndpointState persists an endpoint breaker state to Redis.
func (r *RedisStateSync) SaveEndpointState(ctx context.Context, state EndpointBreakerState, ttl time.Duration) {
	if !r.Available() {
		return
	}

	data, err := json.Marshal(state)
	if err != nil {
		return
	}

	if ttl < 5*time.Minute {
		ttl = 5 * time.Minute
	}

	key := endpointRedisKey(state.EndpointID)
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		logger.Warn().Err(err).Int("endpointId", state.EndpointID).
			Msg("[RedisSync] Failed to save endpoint state")
	}
}

// LoadEndpointState loads a single endpoint's state from Redis.
func (r *RedisStateSync) LoadEndpointState(ctx context.Context, endpointID int) (*EndpointBreakerState, error) {
	if !r.Available() {
		return nil, nil
	}

	key := endpointRedisKey(endpointID)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var state EndpointBreakerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal endpoint state: %w", err)
	}
	return &state, nil
}

// DeleteEndpointState removes an endpoint's state from Redis.
func (r *RedisStateSync) DeleteEndpointState(ctx context.Context, endpointID int) {
	if !r.Available() {
		return
	}
	key := endpointRedisKey(endpointID)
	r.client.Del(ctx, key)
}

// --- Vendor+Type state ---

// SaveVendorTypeState persists a vendor+type breaker state to Redis.
func (r *RedisStateSync) SaveVendorTypeState(ctx context.Context, state VendorTypeState, ttl time.Duration) {
	if !r.Available() {
		return
	}

	data, err := json.Marshal(state)
	if err != nil {
		return
	}

	if ttl < time.Minute {
		ttl = time.Minute
	}

	key := vendorTypeRedisKey(state.VendorID, state.ProviderType)
	if err := r.client.Set(ctx, key, data, ttl).Err(); err != nil {
		logger.Warn().Err(err).
			Int("vendorId", state.VendorID).
			Str("providerType", state.ProviderType).
			Msg("[RedisSync] Failed to save vendor+type state")
	}
}

// LoadVendorTypeState loads a vendor+type state from Redis.
func (r *RedisStateSync) LoadVendorTypeState(ctx context.Context, vendorID int, providerType string) (*VendorTypeState, error) {
	if !r.Available() {
		return nil, nil
	}

	key := vendorTypeRedisKey(vendorID, providerType)
	data, err := r.client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("redis get %s: %w", key, err)
	}

	var state VendorTypeState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("unmarshal vendor+type state: %w", err)
	}
	return &state, nil
}

// DeleteVendorTypeState removes a vendor+type state from Redis.
func (r *RedisStateSync) DeleteVendorTypeState(ctx context.Context, vendorID int, providerType string) {
	if !r.Available() {
		return
	}
	key := vendorTypeRedisKey(vendorID, providerType)
	r.client.Del(ctx, key)
}

// --- Key helpers ---

func providerRedisKey(providerID int) string {
	return fmt.Sprintf("%s%d", redisProviderKeyPrefix, providerID)
}

func endpointRedisKey(endpointID int) string {
	return fmt.Sprintf("%s%d", redisEndpointKeyPrefix, endpointID)
}

func vendorTypeRedisKey(vendorID int, providerType string) string {
	return fmt.Sprintf("%s%d_%s", redisVendorTypeKeyPrefix, vendorID, providerType)
}
