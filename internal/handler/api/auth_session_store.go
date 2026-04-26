package api

import (
	"context"
	"os"
	"strings"

	"github.com/ding113/claude-code-hub/internal/database"
	"github.com/redis/go-redis/v9"
)

const authSessionKeyPrefix = "cch:session:"

type authSessionRevoker interface {
	Revoke(ctx context.Context, sessionID string) error
}

type redisAuthSessionStore struct {
	client *redis.Client
}

func NewRedisAuthSessionStore(rdb *database.RedisClient) authSessionRevoker {
	if rdb == nil {
		return nil
	}
	client := rdb.GetRedisClient()
	if client == nil {
		return nil
	}
	return &redisAuthSessionStore{client: client}
}

func (s *redisAuthSessionStore) Revoke(ctx context.Context, sessionID string) error {
	if s == nil || s.client == nil {
		return nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil
	}
	return s.client.Del(ctx, authSessionKeyPrefix+sessionID).Err()
}

type sessionTokenMode string

const (
	sessionTokenModeLegacy sessionTokenMode = "legacy"
	sessionTokenModeDual   sessionTokenMode = "dual"
	sessionTokenModeOpaque sessionTokenMode = "opaque"
)

func resolveSessionTokenModeFromEnv() sessionTokenMode {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("SESSION_TOKEN_MODE"))) {
	case string(sessionTokenModeDual):
		return sessionTokenModeDual
	case string(sessionTokenModeOpaque):
		return sessionTokenModeOpaque
	default:
		return sessionTokenModeLegacy
	}
}
