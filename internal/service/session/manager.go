package session

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/config"
	"github.com/ding113/claude-code-hub/internal/database"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

type ClientSessionIDSource string

const (
	ClientSessionIDSourceClaudeMetadataUserIDJSON   ClientSessionIDSource = "claude_metadata_user_id_json"
	ClientSessionIDSourceClaudeMetadataUserIDLegacy ClientSessionIDSource = "claude_metadata_user_id_legacy"
	ClientSessionIDSourceClaudeMetadataSessionID    ClientSessionIDSource = "claude_metadata_session_id"
)

const (
	defaultSessionTTL        = 300 * time.Second
	sessionHashPrefix        = "hash:"
	sessionHashSuffix        = ":session"
	sessionKeyPrefix         = "session:"
	sessionKeySuffixKey      = ":key"
	sessionKeySuffixLastSeen = ":last_seen"
	sessionKeySuffixProvider = ":provider"
)

var claudeMetadataUserIDLegacyPattern = regexp.MustCompile(`^user_(.+?)_account__session_(.+)$`)

type ClientSessionExtractionResult struct {
	SessionID string
	Source    ClientSessionIDSource
}

type ClaudeMetadataUserIDFormat string

const (
	ClaudeMetadataUserIDFormatLegacy ClaudeMetadataUserIDFormat = "legacy"
	ClaudeMetadataUserIDFormatJSON   ClaudeMetadataUserIDFormat = "json"
)

type ClaudeMetadataUserIDParseResult struct {
	SessionID   string
	Format      ClaudeMetadataUserIDFormat
	DeviceID    string
	AccountUUID string
}

type sessionStore interface {
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	Expire(ctx context.Context, key string, ttl time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	SetEX(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, key string) error
}

type redisSessionStore struct {
	client *database.RedisClient
}

func (s redisSessionStore) Incr(ctx context.Context, key string) (int64, error) {
	return s.client.Incr(ctx, key).Result()
}

func (s redisSessionStore) Decr(ctx context.Context, key string) (int64, error) {
	return s.client.Decr(ctx, key).Result()
}

func (s redisSessionStore) Expire(ctx context.Context, key string, ttl time.Duration) error {
	return s.client.Expire(ctx, key, ttl).Err()
}

func (s redisSessionStore) Get(ctx context.Context, key string) (string, error) {
	return s.client.Get(ctx, key).Result()
}

func (s redisSessionStore) SetEX(ctx context.Context, key, value string, ttl time.Duration) error {
	return s.client.SetEx(ctx, key, value, ttl).Err()
}

func (s redisSessionStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, key).Err()
}

type Manager struct {
	ttl                         time.Duration
	shortContextThreshold       int
	enableShortContextDetection bool
	store                       sessionStore
	tracker                     *Tracker
	now                         func() time.Time
	randomIntn                  func(n int) int
	randomBytes                 func([]byte) error
}

func NewManager(cfg config.SessionConfig, client *database.RedisClient) *Manager {
	ttl := time.Duration(cfg.TTL) * time.Second
	if ttl <= 0 {
		ttl = defaultSessionTTL
	}

	var store sessionStore
	if client != nil {
		store = redisSessionStore{client: client}
	}

	return &Manager{
		ttl:                         ttl,
		shortContextThreshold:       defaultShortContextThreshold(cfg.ShortContextThreshold),
		enableShortContextDetection: defaultShortContextDetectionEnabled(cfg),
		store:                       store,
		tracker:                     NewTracker(ttl, store),
		now:                         time.Now,
		randomIntn: func(n int) int {
			if n <= 0 {
				return 0
			}

			var buf [1]byte
			if _, err := rand.Read(buf[:]); err != nil {
				return 0
			}
			return int(buf[0]) % n
		},
		randomBytes: func(dst []byte) error {
			_, err := rand.Read(dst)
			return err
		},
	}
}

func (m *Manager) ExtractClientSessionID(requestBody map[string]any, headers http.Header) ClientSessionExtractionResult {
	if headers != nil && isCodexRequest(requestBody) {
		result := ExtractCodexSessionID(headers, requestBody)
		if result.SessionID != "" {
			return ClientSessionExtractionResult{
				SessionID: result.SessionID,
				Source:    ClientSessionIDSource(result.Source),
			}
		}
		return ClientSessionExtractionResult{}
	}

	metadata := metadataMap(requestBody)
	if metadata == nil {
		return ClientSessionExtractionResult{}
	}

	parsedUserID := ParseClaudeMetadataUserID(metadata["user_id"])
	if parsedUserID.SessionID != "" {
		source := ClientSessionIDSourceClaudeMetadataUserIDLegacy
		if parsedUserID.Format == ClaudeMetadataUserIDFormatJSON {
			source = ClientSessionIDSourceClaudeMetadataUserIDJSON
		}

		return ClientSessionExtractionResult{
			SessionID: parsedUserID.SessionID,
			Source:    source,
		}
	}

	sessionID := strings.TrimSpace(stringValue(metadata["session_id"]))
	if sessionID == "" {
		return ClientSessionExtractionResult{}
	}

	return ClientSessionExtractionResult{
		SessionID: sessionID,
		Source:    ClientSessionIDSourceClaudeMetadataSessionID,
	}
}

func ParseClaudeMetadataUserID(userID any) ClaudeMetadataUserIDParseResult {
	text, ok := userID.(string)
	if !ok {
		return ClaudeMetadataUserIDParseResult{}
	}

	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ClaudeMetadataUserIDParseResult{}
	}

	var parsed map[string]any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		sessionID := strings.TrimSpace(stringValue(parsed["session_id"]))
		if sessionID != "" {
			return ClaudeMetadataUserIDParseResult{
				SessionID:   sessionID,
				Format:      ClaudeMetadataUserIDFormatJSON,
				DeviceID:    stringValue(parsed["device_id"]),
				AccountUUID: stringValue(parsed["account_uuid"]),
			}
		}
	}

	matches := claudeMetadataUserIDLegacyPattern.FindStringSubmatch(trimmed)
	if len(matches) != 3 {
		return ClaudeMetadataUserIDParseResult{}
	}

	sessionID := strings.TrimSpace(matches[2])
	if sessionID == "" {
		return ClaudeMetadataUserIDParseResult{}
	}

	return ClaudeMetadataUserIDParseResult{
		SessionID: sessionID,
		Format:    ClaudeMetadataUserIDFormatLegacy,
		DeviceID:  matches[1],
	}
}

func (m *Manager) GenerateSessionID() string {
	var randomPart [6]byte
	if err := m.randomBytes(randomPart[:]); err != nil {
		logger.Error().Err(err).Msg("SessionManager: failed to generate random session suffix")
		return "sess_" + strconv.FormatInt(m.now().UnixMilli(), 36)
	}

	return "sess_" + strconv.FormatInt(m.now().UnixMilli(), 36) + "_" + hex.EncodeToString(randomPart[:])
}

func (m *Manager) CalculateMessagesHash(messages any) string {
	messageItems, ok := messages.([]any)
	if !ok || len(messageItems) == 0 {
		return ""
	}

	count := minInt(len(messageItems), 3)
	contents := make([]string, 0, count)
	for i := 0; i < count; i++ {
		messageMap, ok := messageItems[i].(map[string]any)
		if !ok {
			continue
		}

		switch content := messageMap["content"].(type) {
		case string:
			contents = append(contents, content)
		case []any:
			textParts := make([]string, 0, len(content))
			for _, item := range content {
				part, ok := item.(map[string]any)
				if !ok || stringValue(part["type"]) != "text" {
					continue
				}
				if text := stringValue(part["text"]); text != "" {
					textParts = append(textParts, text)
				}
			}
			if len(textParts) > 0 {
				contents = append(contents, strings.Join(textParts, ""))
			}
		}
	}

	if len(contents) == 0 {
		return ""
	}

	sum := sha256.Sum256([]byte(strings.Join(contents, "|")))
	return hex.EncodeToString(sum[:])[:16]
}

func (m *Manager) GetOrCreateSessionID(ctx context.Context, keyID int, messages any, clientSessionID string) string {
	if sessionID := strings.TrimSpace(clientSessionID); sessionID != "" {
		if m.enableShortContextDetection && messageCount(messages) <= m.shortContextThreshold {
			if concurrentCount := m.tracker.GetConcurrentCount(ctx, sessionID); concurrentCount > 0 {
				newSessionID := m.GenerateSessionID()
				logger.Info().
					Str("originalSessionId", sessionID).
					Str("newSessionId", newSessionID).
					Int("messagesLength", messageCount(messages)).
					Int("existingConcurrentCount", concurrentCount).
					Msg("SessionManager: detected concurrent short-context request, forcing new session")
				return newSessionID
			}
		}

		m.refreshSessionTTL(ctx, sessionID)
		return sessionID
	}

	contentHash := m.CalculateMessagesHash(messages)
	if contentHash == "" {
		logger.Warn().Int("keyId", keyID).Msg("SessionManager: no client session ID and message hash unavailable, generating new session")
		return m.GenerateSessionID()
	}

	if m.store == nil {
		return m.GenerateSessionID()
	}

	hashKey := sessionHashPrefix + contentHash + sessionHashSuffix
	existingSessionID, err := m.store.Get(ctx, hashKey)
	if err == nil && strings.TrimSpace(existingSessionID) != "" {
		m.refreshSessionTTL(ctx, existingSessionID)
		return existingSessionID
	}
	if err != nil {
		logger.Error().Err(err).Str("hashKey", hashKey).Msg("SessionManager: failed to load session by content hash")
		return m.GenerateSessionID()
	}

	newSessionID := m.GenerateSessionID()
	if err := m.storeSessionMapping(ctx, contentHash, newSessionID, keyID); err != nil {
		logger.Error().Err(err).Str("sessionId", newSessionID).Str("contentHash", contentHash).Msg("SessionManager: failed to persist session mapping")
	}

	return newSessionID
}

func (m *Manager) BindProvider(ctx context.Context, sessionID string, providerID int) {
	if m.store == nil || strings.TrimSpace(sessionID) == "" || providerID <= 0 {
		return
	}
	if err := m.store.SetEX(ctx, sessionKeyPrefix+sessionID+sessionKeySuffixProvider, strconv.Itoa(providerID), m.ttl); err != nil {
		logger.Warn().Err(err).Str("sessionId", sessionID).Int("providerId", providerID).Msg("SessionManager: failed to bind provider")
	}
}

func (m *Manager) GetNextRequestSequence(ctx context.Context, sessionID string) int {
	if strings.TrimSpace(sessionID) == "" {
		return m.fallbackSequence("")
	}
	if m.store == nil {
		return m.fallbackSequence(sessionID)
	}

	key := sessionKeyPrefix + sessionID + ":seq"
	sequence, err := m.store.Incr(ctx, key)
	if err != nil {
		logger.Warn().Err(err).Str("sessionId", sessionID).Msg("SessionManager: fallback request sequence because Redis INCR failed")
		return m.fallbackSequence(sessionID)
	}

	if sequence == 1 {
		if err := m.store.Expire(ctx, key, m.ttl); err != nil {
			logger.Warn().Err(err).Str("sessionId", sessionID).Msg("SessionManager: failed to set request sequence TTL")
		}
	}

	return int(sequence)
}

func (m *Manager) GetSessionRequestCount(ctx context.Context, sessionID string) int {
	if strings.TrimSpace(sessionID) == "" || m.store == nil {
		return 0
	}

	value, err := m.store.Get(ctx, sessionKeyPrefix+sessionID+":seq")
	if err != nil || value == "" {
		return 0
	}

	count, err := strconv.Atoi(value)
	if err != nil {
		logger.Warn().Err(err).Str("sessionId", sessionID).Str("value", value).Msg("SessionManager: invalid request count value")
		return 0
	}

	return count
}

func (m *Manager) IncrementConcurrentCount(ctx context.Context, sessionID string) {
	if m == nil || m.tracker == nil {
		return
	}
	m.tracker.IncrementConcurrentCount(ctx, sessionID)
}

func (m *Manager) DecrementConcurrentCount(ctx context.Context, sessionID string) {
	if m == nil || m.tracker == nil {
		return
	}
	m.tracker.DecrementConcurrentCount(ctx, sessionID)
}

func (m *Manager) storeSessionMapping(ctx context.Context, contentHash, sessionID string, keyID int) error {
	if m.store == nil {
		return nil
	}

	hashKey := sessionHashPrefix + contentHash + sessionHashSuffix
	if err := m.store.SetEX(ctx, hashKey, sessionID, m.ttl); err != nil {
		return err
	}
	if err := m.store.SetEX(ctx, sessionKeyPrefix+sessionID+sessionKeySuffixKey, strconv.Itoa(keyID), m.ttl); err != nil {
		return err
	}
	if err := m.store.SetEX(ctx, sessionKeyPrefix+sessionID+sessionKeySuffixLastSeen, strconv.FormatInt(m.now().UnixMilli(), 10), m.ttl); err != nil {
		return err
	}
	return nil
}

func (m *Manager) refreshSessionTTL(ctx context.Context, sessionID string) {
	if m.store == nil || strings.TrimSpace(sessionID) == "" {
		return
	}

	for _, key := range []string{
		sessionKeyPrefix + sessionID + sessionKeySuffixKey,
		sessionKeyPrefix + sessionID + sessionKeySuffixProvider,
	} {
		if err := m.store.Expire(ctx, key, m.ttl); err != nil {
			logger.Warn().Err(err).Str("sessionId", sessionID).Str("key", key).Msg("SessionManager: failed to refresh session TTL")
		}
	}

	if err := m.store.SetEX(ctx, sessionKeyPrefix+sessionID+sessionKeySuffixLastSeen, strconv.FormatInt(m.now().UnixMilli(), 10), m.ttl); err != nil {
		logger.Warn().Err(err).Str("sessionId", sessionID).Msg("SessionManager: failed to refresh last_seen TTL")
	}
}

func (m *Manager) fallbackSequence(sessionID string) int {
	base := int(m.now().UnixMilli() % 1000000)
	sequence := base + m.randomIntn(1000)

	logger.Warn().
		Str("sessionId", sessionID).
		Int("fallbackSequence", sequence).
		Msg("SessionManager: Redis unavailable, using fallback request sequence")

	return sequence
}

func isCodexRequest(requestBody map[string]any) bool {
	if requestBody == nil {
		return false
	}
	_, ok := requestBody["input"].([]any)
	return ok
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func messageCount(messages any) int {
	items, ok := messages.([]any)
	if !ok {
		return 0
	}
	return len(items)
}

func defaultShortContextThreshold(value int) int {
	if value > 0 {
		return value
	}
	return 2
}

func defaultShortContextDetectionEnabled(cfg config.SessionConfig) bool {
	if cfg.ShortContextThreshold == 0 && !cfg.EnableShortContextDetection {
		return true
	}
	return cfg.EnableShortContextDetection
}
