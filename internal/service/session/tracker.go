package session

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

const defaultConcurrentCountTTL = 600 * time.Second

type Tracker struct {
	store              sessionStore
	concurrentCountTTL time.Duration
}

func NewTracker(_ time.Duration, store sessionStore) *Tracker {
	return &Tracker{
		store:              store,
		concurrentCountTTL: defaultConcurrentCountTTL,
	}
}

func (t *Tracker) IncrementConcurrentCount(ctx context.Context, sessionID string) {
	if t.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	key := sessionKeyPrefix + sessionID + ":concurrent_count"
	if _, err := t.store.Incr(ctx, key); err != nil {
		logger.Error().Err(err).Str("sessionId", sessionID).Msg("SessionTracker: failed to increment concurrent count")
		return
	}
	if err := t.store.Expire(ctx, key, t.concurrentCountTTL); err != nil {
		logger.Error().Err(err).Str("sessionId", sessionID).Msg("SessionTracker: failed to set concurrent count TTL")
	}
}

func (t *Tracker) DecrementConcurrentCount(ctx context.Context, sessionID string) {
	if t.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	key := sessionKeyPrefix + sessionID + ":concurrent_count"
	count, err := t.store.Decr(ctx, key)
	if err != nil {
		logger.Error().Err(err).Str("sessionId", sessionID).Msg("SessionTracker: failed to decrement concurrent count")
		return
	}
	if count <= 0 {
		if err := t.store.Del(ctx, key); err != nil {
			logger.Error().Err(err).Str("sessionId", sessionID).Msg("SessionTracker: failed to delete concurrent count key")
		}
	}
}

func (t *Tracker) GetConcurrentCount(ctx context.Context, sessionID string) int {
	if t.store == nil || strings.TrimSpace(sessionID) == "" {
		return 0
	}

	value, err := t.store.Get(ctx, sessionKeyPrefix+sessionID+":concurrent_count")
	if err != nil || value == "" {
		return 0
	}

	count, err := strconv.Atoi(value)
	if err != nil {
		logger.Warn().Err(err).Str("sessionId", sessionID).Str("value", value).Msg("SessionTracker: invalid concurrent count value")
		return 0
	}
	return count
}
