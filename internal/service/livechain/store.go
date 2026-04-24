package livechain

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/database"
	"github.com/ding113/claude-code-hub/internal/model"
)

const defaultTTL = 300 * time.Second

type Snapshot struct {
	Chain     []model.ProviderChainItem `json:"chain"`
	Phase     string                    `json:"phase"`
	UpdatedAt int64                     `json:"updatedAt"`
}

type Store interface {
	Write(ctx context.Context, sessionID string, requestSequence int, chain []model.ProviderChainItem) error
	ReadBatch(ctx context.Context, keys []Key) (map[string]Snapshot, error)
	Delete(ctx context.Context, sessionID string, requestSequence int) error
}

type Key struct {
	SessionID       string
	RequestSequence int
}

type memoryStore struct {
	mu    sync.Mutex
	ttl   time.Duration
	items map[string]memoryItem
}

type memoryItem struct {
	snapshot  Snapshot
	expiresAt time.Time
}

type redisStore struct {
	client *database.RedisClient
	ttl    time.Duration
}

var (
	now                = time.Now
	currentStore Store = newMemoryStore(defaultTTL)
)

func Configure(client *database.RedisClient, ttl time.Duration) {
	if ttl <= 0 {
		ttl = defaultTTL
	}
	if client == nil {
		currentStore = newMemoryStore(ttl)
		return
	}
	currentStore = &redisStore{client: client, ttl: ttl}
}

func ResetForTest() {
	currentStore = newMemoryStore(defaultTTL)
}

func Write(ctx context.Context, sessionID string, requestSequence int, chain []model.ProviderChainItem) error {
	if strings.TrimSpace(sessionID) == "" || requestSequence <= 0 {
		return nil
	}
	return currentStore.Write(ctx, sessionID, requestSequence, chain)
}

func ReadBatch(ctx context.Context, keys []Key) (map[string]Snapshot, error) {
	if len(keys) == 0 {
		return map[string]Snapshot{}, nil
	}
	return currentStore.ReadBatch(ctx, keys)
}

func Delete(ctx context.Context, sessionID string, requestSequence int) error {
	if strings.TrimSpace(sessionID) == "" || requestSequence <= 0 {
		return nil
	}
	return currentStore.Delete(ctx, sessionID, requestSequence)
}

func InferPhase(chain []model.ProviderChainItem) string {
	if len(chain) == 0 {
		return "queued"
	}
	last := chain[len(chain)-1]
	reason := ""
	if last.Reason != nil {
		reason = strings.TrimSpace(*last.Reason)
	}
	switch reason {
	case "initial_selection":
		return "provider_selected"
	case "session_reuse":
		return "session_reused"
	case "retry_failed", "system_error", "resource_not_found":
		return "retrying"
	case "hedge_triggered", "hedge_launched":
		return "hedge_racing"
	case "hedge_winner", "hedge_loser_cancelled":
		return "hedge_resolved"
	case "request_success", "retry_success":
		return "streaming"
	case "client_abort":
		return "aborted"
	default:
		return "forwarding"
	}
}

func newSnapshot(chain []model.ProviderChainItem) Snapshot {
	copied := make([]model.ProviderChainItem, len(chain))
	copy(copied, chain)
	return Snapshot{
		Chain:     copied,
		Phase:     InferPhase(copied),
		UpdatedAt: now().UnixMilli(),
	}
}

func buildStorageKey(sessionID string, requestSequence int) string {
	return "cch:live-chain:" + sessionID + ":" + strconv.Itoa(requestSequence)
}

func buildLookupKey(sessionID string, requestSequence int) string {
	return sessionID + ":" + strconv.Itoa(requestSequence)
}

func newMemoryStore(ttl time.Duration) *memoryStore {
	return &memoryStore{
		ttl:   ttl,
		items: map[string]memoryItem{},
	}
}

func (s *memoryStore) Write(_ context.Context, sessionID string, requestSequence int, chain []model.ProviderChainItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	s.items[buildLookupKey(sessionID, requestSequence)] = memoryItem{
		snapshot:  newSnapshot(chain),
		expiresAt: now().Add(s.ttl),
	}
	return nil
}

func (s *memoryStore) ReadBatch(_ context.Context, keys []Key) (map[string]Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.cleanupLocked()
	results := make(map[string]Snapshot, len(keys))
	for _, key := range keys {
		if item, ok := s.items[buildLookupKey(key.SessionID, key.RequestSequence)]; ok {
			results[buildLookupKey(key.SessionID, key.RequestSequence)] = item.snapshot
		}
	}
	return results, nil
}

func (s *memoryStore) Delete(_ context.Context, sessionID string, requestSequence int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, buildLookupKey(sessionID, requestSequence))
	return nil
}

func (s *memoryStore) cleanupLocked() {
	current := now()
	for key, item := range s.items {
		if !item.expiresAt.After(current) {
			delete(s.items, key)
		}
	}
}

func (s *redisStore) Write(ctx context.Context, sessionID string, requestSequence int, chain []model.ProviderChainItem) error {
	payload, err := json.Marshal(newSnapshot(chain))
	if err != nil {
		return err
	}
	return s.client.Set(ctx, buildStorageKey(sessionID, requestSequence), payload, s.ttl).Err()
}

func (s *redisStore) ReadBatch(ctx context.Context, keys []Key) (map[string]Snapshot, error) {
	results := make(map[string]Snapshot, len(keys))
	for _, key := range keys {
		raw, err := s.client.Get(ctx, buildStorageKey(key.SessionID, key.RequestSequence)).Result()
		if err != nil || raw == "" {
			continue
		}
		var snapshot Snapshot
		if err := json.Unmarshal([]byte(raw), &snapshot); err != nil {
			continue
		}
		results[buildLookupKey(key.SessionID, key.RequestSequence)] = snapshot
	}
	return results, nil
}

func (s *redisStore) Delete(ctx context.Context, sessionID string, requestSequence int) error {
	return s.client.Del(ctx, buildStorageKey(sessionID, requestSequence)).Err()
}
