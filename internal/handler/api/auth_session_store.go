package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/database"
	authsvc "github.com/ding113/claude-code-hub/internal/service/auth"
	"github.com/redis/go-redis/v9"
)

const authSessionKeyPrefix = "cch:session:"
const authSessionTTL = 7 * 24 * time.Hour

type authSessionRevoker interface {
	Create(ctx context.Context, apiKey string, userID int, userRole string) (string, error)
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

func (s *redisAuthSessionStore) Create(ctx context.Context, apiKey string, userID int, userRole string) (string, error) {
	if s == nil || s.client == nil {
		return "", nil
	}
	sessionID, err := generateOpaqueSessionID()
	if err != nil {
		return "", err
	}
	now := time.Now()
	payload := authsvc.SessionTokenData{
		SessionID:      sessionID,
		KeyFingerprint: authsvcKeyFingerprint(apiKey),
		UserID:         userID,
		UserRole:       userRole,
		CreatedAt:      now.UnixMilli(),
		ExpiresAt:      now.Add(authSessionTTL).UnixMilli(),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := s.client.Set(ctx, authSessionKeyPrefix+sessionID, encoded, authSessionTTL).Err(); err != nil {
		return "", err
	}
	return sessionID, nil
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

func generateOpaqueSessionID() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return "sid_" + hex.EncodeToString(buf), nil
}

func authsvcKeyFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return "sha256:" + hex.EncodeToString(sum[:])
}
