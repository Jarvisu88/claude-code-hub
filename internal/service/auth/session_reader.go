package auth

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"os"
	"strings"

	"github.com/ding113/claude-code-hub/internal/database"
	"github.com/redis/go-redis/v9"
)

const authSessionKeyPrefix = "cch:session:"

type sessionTokenMode string

const (
	sessionTokenModeLegacy sessionTokenMode = "legacy"
	sessionTokenModeDual   sessionTokenMode = "dual"
	sessionTokenModeOpaque sessionTokenMode = "opaque"
)

type SessionTokenData struct {
	SessionID      string `json:"sessionId"`
	KeyFingerprint string `json:"keyFingerprint"`
	UserID         int    `json:"userId"`
	UserRole       string `json:"userRole"`
	CreatedAt      int64  `json:"createdAt"`
	ExpiresAt      int64  `json:"expiresAt"`
}

type sessionTokenReader interface {
	Read(ctx context.Context, sessionID string) (*SessionTokenData, error)
}

type redisSessionTokenReader struct {
	client *redis.Client
}

func NewRedisSessionReader(rdb *database.RedisClient) sessionTokenReader {
	if rdb == nil {
		return nil
	}
	client := rdb.GetRedisClient()
	if client == nil {
		return nil
	}
	return &redisSessionTokenReader{client: client}
}

func (r *redisSessionTokenReader) Read(ctx context.Context, sessionID string) (*SessionTokenData, error) {
	if r == nil || r.client == nil {
		return nil, nil
	}
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return nil, nil
	}
	raw, err := r.client.Get(ctx, authSessionKeyPrefix+sessionID).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var payload SessionTokenData
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return nil, err
	}
	return &payload, nil
}

func keyFingerprint(apiKey string) string {
	sum := sha256.Sum256([]byte(apiKey))
	return "sha256:" + hex.EncodeToString(sum[:])
}

func normalizeKeyFingerprint(fingerprint string) string {
	if strings.HasPrefix(fingerprint, "sha256:") {
		return fingerprint
	}
	return "sha256:" + fingerprint
}

func constantTimeEqualString(left, right string) bool {
	return subtle.ConstantTimeCompare([]byte(left), []byte(right)) == 1
}

func resolveSessionTokenMode() sessionTokenMode {
	switch strings.TrimSpace(strings.ToLower(os.Getenv("SESSION_TOKEN_MODE"))) {
	case string(sessionTokenModeDual):
		return sessionTokenModeDual
	case string(sessionTokenModeOpaque):
		return sessionTokenModeOpaque
	default:
		return sessionTokenModeLegacy
	}
}

func looksLikeSessionToken(token string) bool {
	return strings.HasPrefix(strings.TrimSpace(token), "sid_")
}
