package session

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/ding113/claude-code-hub/internal/config"
)

type fakeSetCall struct {
	key   string
	value string
	ttl   time.Duration
}

type fakeSessionStore struct {
	incrValue  int64
	incrErr    error
	decrValue  int64
	decrErr    error
	expireErr  error
	getValue   string
	getErr     error
	setErr     error
	delErr     error
	incrKeys   []string
	decrKeys   []string
	expireKeys []string
	getKeys    []string
	delKeys    []string
	setCalls   []fakeSetCall
	expireTTL  time.Duration
}

func (f *fakeSessionStore) Incr(_ context.Context, key string) (int64, error) {
	f.incrKeys = append(f.incrKeys, key)
	if f.incrErr != nil {
		return 0, f.incrErr
	}
	return f.incrValue, nil
}

func (f *fakeSessionStore) Decr(_ context.Context, key string) (int64, error) {
	f.decrKeys = append(f.decrKeys, key)
	if f.decrErr != nil {
		return 0, f.decrErr
	}
	return f.decrValue, nil
}

func (f *fakeSessionStore) Expire(_ context.Context, key string, ttl time.Duration) error {
	f.expireKeys = append(f.expireKeys, key)
	f.expireTTL = ttl
	return f.expireErr
}

func (f *fakeSessionStore) Get(_ context.Context, key string) (string, error) {
	f.getKeys = append(f.getKeys, key)
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.getValue, nil
}

func (f *fakeSessionStore) SetEX(_ context.Context, key, value string, ttl time.Duration) error {
	f.setCalls = append(f.setCalls, fakeSetCall{key: key, value: value, ttl: ttl})
	return f.setErr
}

func (f *fakeSessionStore) Del(_ context.Context, key string) error {
	f.delKeys = append(f.delKeys, key)
	return f.delErr
}

func TestExtractClientSessionIDCodex(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	headers := http.Header{}
	headers.Set("session_id", "sess_header_priority_1234567890")

	result := manager.ExtractClientSessionID(map[string]any{
		"input": []any{map[string]any{"role": "user"}},
	}, headers)

	if result.SessionID != "sess_header_priority_1234567890" {
		t.Fatalf("expected codex session id, got %q", result.SessionID)
	}
	if result.Source != ClientSessionIDSource(CodexSessionIDSourceHeaderSessionID) {
		t.Fatalf("expected codex header source, got %q", result.Source)
	}
}

func TestExtractClientSessionIDClaudeJSONMetadata(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)

	result := manager.ExtractClientSessionID(map[string]any{
		"metadata": map[string]any{
			"user_id": `{"device_id":"dev-1","account_uuid":"","session_id":"sess_json_123"}`,
		},
	}, nil)

	if result.SessionID != "sess_json_123" {
		t.Fatalf("expected session id from json metadata, got %q", result.SessionID)
	}
	if result.Source != ClientSessionIDSourceClaudeMetadataUserIDJSON {
		t.Fatalf("expected json metadata source, got %q", result.Source)
	}
}

func TestExtractClientSessionIDClaudeLegacyMetadata(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)

	result := manager.ExtractClientSessionID(map[string]any{
		"metadata": map[string]any{
			"user_id": "user_device_hash_account__session_sess_legacy_123",
		},
	}, nil)

	if result.SessionID != "sess_legacy_123" {
		t.Fatalf("expected session id from legacy metadata, got %q", result.SessionID)
	}
	if result.Source != ClientSessionIDSourceClaudeMetadataUserIDLegacy {
		t.Fatalf("expected legacy metadata source, got %q", result.Source)
	}
}

func TestExtractClientSessionIDClaudeMetadataSessionFallback(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)

	result := manager.ExtractClientSessionID(map[string]any{
		"metadata": map[string]any{
			"session_id": "sess_metadata_direct",
		},
	}, nil)

	if result.SessionID != "sess_metadata_direct" {
		t.Fatalf("expected metadata.session_id fallback, got %q", result.SessionID)
	}
	if result.Source != ClientSessionIDSourceClaudeMetadataSessionID {
		t.Fatalf("expected metadata.session_id source, got %q", result.Source)
	}
}

func TestNewManagerUsesNodeCompatibleShortContextDefaults(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)

	if !manager.enableShortContextDetection {
		t.Fatalf("expected short-context detection to default to enabled")
	}
	if manager.shortContextThreshold != 2 {
		t.Fatalf("expected short-context threshold default 2, got %d", manager.shortContextThreshold)
	}
}

func TestCalculateMessagesHash(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)

	tests := []struct {
		name     string
		messages any
		want     string
	}{
		{
			name: "uses first three string messages",
			messages: []any{
				map[string]any{"content": "alpha"},
				map[string]any{"content": "beta"},
				map[string]any{"content": "gamma"},
				map[string]any{"content": "delta"},
			},
			want: "27e1a9b200ae113f",
		},
		{
			name: "joins multimodal text parts only",
			messages: []any{
				map[string]any{"content": []any{
					map[string]any{"type": "text", "text": "hello "},
					map[string]any{"type": "input_image", "image_url": "ignored"},
					map[string]any{"type": "text", "text": "world"},
				}},
			},
			want: "b94d27b9934d3e08",
		},
		{
			name: "returns empty when no usable contents",
			messages: []any{
				map[string]any{"content": []any{map[string]any{"type": "image", "text": "ignored"}}},
			},
			want: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := manager.CalculateMessagesHash(tc.messages); got != tc.want {
				t.Fatalf("expected hash %q, got %q", tc.want, got)
			}
		})
	}
}

func TestGetOrCreateSessionIDUsesClientSessionAndRefreshesTTL(t *testing.T) {
	store := &fakeSessionStore{}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 42, nil, "sess_client_123")
	if sessionID != "sess_client_123" {
		t.Fatalf("expected client session id, got %q", sessionID)
	}
	if len(store.expireKeys) != 2 {
		t.Fatalf("expected 2 expire calls, got %+v", store.expireKeys)
	}
	if store.expireKeys[0] != "session:sess_client_123:key" || store.expireKeys[1] != "session:sess_client_123:provider" {
		t.Fatalf("unexpected expire keys: %+v", store.expireKeys)
	}
	if len(store.setCalls) != 1 || store.setCalls[0].key != "session:sess_client_123:last_seen" || store.setCalls[0].value != "1713744000123" {
		t.Fatalf("unexpected set calls: %+v", store.setCalls)
	}
}

func TestGetOrCreateSessionIDCreatesNewSessionForConcurrentShortContext(t *testing.T) {
	store := &fakeSessionStore{getValue: "2"}
	manager := NewManager(config.SessionConfig{TTL: 300, ShortContextThreshold: 2, EnableShortContextDetection: true}, nil)
	manager.store = store
	manager.tracker = NewTracker(manager.ttl, store)
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}
	manager.randomBytes = func(dst []byte) error {
		copy(dst, []byte{1, 2, 3, 4, 5, 6})
		return nil
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 42, []any{
		map[string]any{"content": "short"},
	}, "sess_client_123")

	if sessionID != "sess_lva6xhff_010203040506" {
		t.Fatalf("expected new session for concurrent short context, got %q", sessionID)
	}
	if len(store.getKeys) != 1 || store.getKeys[0] != "session:sess_client_123:concurrent_count" {
		t.Fatalf("unexpected get keys: %+v", store.getKeys)
	}
	if len(store.expireKeys) != 0 || len(store.setCalls) != 0 {
		t.Fatalf("expected no ttl refresh for replaced session, expire=%+v set=%+v", store.expireKeys, store.setCalls)
	}
}

func TestGetOrCreateSessionIDSkipsShortContextDetectionForLongContext(t *testing.T) {
	store := &fakeSessionStore{getValue: "5"}
	manager := NewManager(config.SessionConfig{TTL: 300, ShortContextThreshold: 2, EnableShortContextDetection: true}, nil)
	manager.store = store
	manager.tracker = NewTracker(manager.ttl, store)
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 42, []any{
		map[string]any{"content": "one"},
		map[string]any{"content": "two"},
		map[string]any{"content": "three"},
	}, "sess_client_123")

	if sessionID != "sess_client_123" {
		t.Fatalf("expected client session to be reused, got %q", sessionID)
	}
	if len(store.getKeys) != 0 {
		t.Fatalf("expected no concurrent count lookup for long context, got %+v", store.getKeys)
	}
}

func TestGetOrCreateSessionIDReusesHashMappedSession(t *testing.T) {
	store := &fakeSessionStore{getValue: "sess_existing_123"}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 7, []any{
		map[string]any{"content": "hello"},
		map[string]any{"content": "world"},
	}, "")
	if sessionID != "sess_existing_123" {
		t.Fatalf("expected existing session id, got %q", sessionID)
	}
	if len(store.getKeys) != 1 || store.getKeys[0] != "hash:55a3db6314a88ae7:session" {
		t.Fatalf("unexpected get keys: %+v", store.getKeys)
	}
	if len(store.setCalls) != 1 || store.setCalls[0].key != "session:sess_existing_123:last_seen" {
		t.Fatalf("unexpected set calls: %+v", store.setCalls)
	}
}

func TestGetOrCreateSessionIDCreatesSessionMappingWhenHashMisses(t *testing.T) {
	store := &fakeSessionStore{}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}
	manager.randomBytes = func(dst []byte) error {
		copy(dst, []byte{1, 2, 3, 4, 5, 6})
		return nil
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 99, []any{
		map[string]any{"content": "hello"},
		map[string]any{"content": "world"},
	}, "")
	if sessionID != "sess_lva6xhff_010203040506" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
	if len(store.setCalls) != 3 {
		t.Fatalf("expected 3 set calls, got %+v", store.setCalls)
	}
	wantCalls := []fakeSetCall{
		{key: "hash:55a3db6314a88ae7:session", value: "sess_lva6xhff_010203040506", ttl: 300 * time.Second},
		{key: "session:sess_lva6xhff_010203040506:key", value: "99", ttl: 300 * time.Second},
		{key: "session:sess_lva6xhff_010203040506:last_seen", value: "1713744000123", ttl: 300 * time.Second},
	}
	for i, want := range wantCalls {
		if store.setCalls[i] != want {
			t.Fatalf("set call %d: expected %+v, got %+v", i, want, store.setCalls[i])
		}
	}
}

func TestGetOrCreateSessionIDFallsBackToNewSessionOnStoreError(t *testing.T) {
	store := &fakeSessionStore{getErr: errors.New("redis down")}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}
	manager.randomBytes = func(dst []byte) error {
		copy(dst, []byte{1, 2, 3, 4, 5, 6})
		return nil
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 99, []any{
		map[string]any{"content": "hello"},
		map[string]any{"content": "world"},
	}, "")
	if sessionID != "sess_lva6xhff_010203040506" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
	if len(store.setCalls) != 0 {
		t.Fatalf("expected no mapping writes on store error, got %+v", store.setCalls)
	}
}

func TestGetOrCreateSessionIDGeneratesNewSessionWhenMessagesCannotHash(t *testing.T) {
	store := &fakeSessionStore{}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}
	manager.randomBytes = func(dst []byte) error {
		copy(dst, []byte{1, 2, 3, 4, 5, 6})
		return nil
	}

	sessionID := manager.GetOrCreateSessionID(context.Background(), 99, []any{
		map[string]any{"content": []any{map[string]any{"type": "image", "text": "ignored"}}},
	}, "")
	if sessionID != "sess_lva6xhff_010203040506" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
	if len(store.getKeys) != 0 || len(store.setCalls) != 0 {
		t.Fatalf("expected no redis access when hash unavailable, get=%+v set=%+v", store.getKeys, store.setCalls)
	}
}

func TestGenerateSessionID(t *testing.T) {
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}
	manager.randomBytes = func(dst []byte) error {
		copy(dst, []byte{1, 2, 3, 4, 5, 6})
		return nil
	}

	sessionID := manager.GenerateSessionID()
	if sessionID != "sess_lva6xhff_010203040506" {
		t.Fatalf("unexpected session id: %q", sessionID)
	}
}

func TestGetNextRequestSequenceFromRedis(t *testing.T) {
	store := &fakeSessionStore{incrValue: 1}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store

	sequence := manager.GetNextRequestSequence(context.Background(), "sess_123")
	if sequence != 1 {
		t.Fatalf("expected sequence 1, got %d", sequence)
	}
	if len(store.incrKeys) != 1 || store.incrKeys[0] != "session:sess_123:seq" {
		t.Fatalf("unexpected INCR keys: %+v", store.incrKeys)
	}
	if len(store.expireKeys) != 1 || store.expireKeys[0] != "session:sess_123:seq" {
		t.Fatalf("unexpected EXPIRE keys: %+v", store.expireKeys)
	}
	if store.expireTTL != 300*time.Second {
		t.Fatalf("expected ttl 300s, got %v", store.expireTTL)
	}
}

func TestGetNextRequestSequenceFallsBackWhenRedisUnavailable(t *testing.T) {
	store := &fakeSessionStore{incrErr: errors.New("redis down")}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(123456)
	}
	manager.randomIntn = func(_ int) int {
		return 7
	}

	sequence := manager.GetNextRequestSequence(context.Background(), "sess_123")
	if sequence != 123463 {
		t.Fatalf("expected fallback sequence 123463, got %d", sequence)
	}
}

func TestGetSessionRequestCount(t *testing.T) {
	store := &fakeSessionStore{getValue: "42"}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store

	count := manager.GetSessionRequestCount(context.Background(), "sess_123")
	if count != 42 {
		t.Fatalf("expected count 42, got %d", count)
	}
}

func TestGetSessionRequestCountReturnsZeroForInvalidValue(t *testing.T) {
	store := &fakeSessionStore{getValue: "not-a-number"}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store

	count := manager.GetSessionRequestCount(context.Background(), "sess_123")
	if count != 0 {
		t.Fatalf("expected count 0, got %d", count)
	}
}

func TestTrackerIncrementConcurrentCount(t *testing.T) {
	store := &fakeSessionStore{incrValue: 1}
	tracker := NewTracker(300*time.Second, store)

	tracker.IncrementConcurrentCount(context.Background(), "sess_123")

	if len(store.incrKeys) != 1 || store.incrKeys[0] != "session:sess_123:concurrent_count" {
		t.Fatalf("unexpected incr keys: %+v", store.incrKeys)
	}
	if len(store.expireKeys) != 1 || store.expireKeys[0] != "session:sess_123:concurrent_count" {
		t.Fatalf("unexpected expire keys: %+v", store.expireKeys)
	}
	if store.expireTTL != 600*time.Second {
		t.Fatalf("expected 600s concurrent ttl, got %v", store.expireTTL)
	}
}

func TestTrackerDecrementConcurrentCountDeletesZeroValue(t *testing.T) {
	store := &fakeSessionStore{decrValue: 0}
	tracker := NewTracker(300*time.Second, store)

	tracker.DecrementConcurrentCount(context.Background(), "sess_123")

	if len(store.decrKeys) != 1 || store.decrKeys[0] != "session:sess_123:concurrent_count" {
		t.Fatalf("unexpected decr keys: %+v", store.decrKeys)
	}
	if len(store.delKeys) != 1 || store.delKeys[0] != "session:sess_123:concurrent_count" {
		t.Fatalf("unexpected del keys: %+v", store.delKeys)
	}
}

func TestUpdateCodexSessionWithPromptCacheKeyCopiesBindings(t *testing.T) {
	store := &fakeSessionStore{getValue: "42"}
	manager := NewManager(config.SessionConfig{TTL: 300}, nil)
	manager.store = store
	manager.now = func() time.Time {
		return time.UnixMilli(1713744000123)
	}

	updated := manager.UpdateCodexSessionWithPromptCacheKey(
		context.Background(),
		"sess_generated_123",
		"019b82ff-08ff-75a3-a203-7e10274fdbd8",
		99,
	)

	if updated != "codex_019b82ff-08ff-75a3-a203-7e10274fdbd8" {
		t.Fatalf("expected codex session id, got %q", updated)
	}
	if len(store.setCalls) != 3 {
		t.Fatalf("expected 3 set calls, got %+v", store.setCalls)
	}
	if store.setCalls[0].key != "session:codex_019b82ff-08ff-75a3-a203-7e10274fdbd8:provider" || store.setCalls[0].value != "99" {
		t.Fatalf("unexpected provider set call: %+v", store.setCalls[0])
	}
	if store.setCalls[1].key != "session:codex_019b82ff-08ff-75a3-a203-7e10274fdbd8:key" || store.setCalls[1].value != "42" {
		t.Fatalf("unexpected key copy call: %+v", store.setCalls[1])
	}
	if store.setCalls[2].key != "session:codex_019b82ff-08ff-75a3-a203-7e10274fdbd8:last_seen" || store.setCalls[2].value != "1713744000123" {
		t.Fatalf("unexpected last_seen set call: %+v", store.setCalls[2])
	}
}
