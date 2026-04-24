package sessiontracker

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ding113/claude-code-hub/internal/database"
)

var ErrNotConfigured = errors.New("session tracker not configured")

type finder interface {
	activeSessionIDs(ctx context.Context, limit int) ([]string, error)
}

type noopFinder struct{}

type redisFinder struct {
	client *database.RedisClient
	ttl    time.Duration
}

type staticFinder struct {
	ids []string
	err error
}

var currentFinder finder = noopFinder{}

func Configure(client *database.RedisClient, ttl time.Duration) {
	if client == nil {
		currentFinder = noopFinder{}
		return
	}
	if ttl <= 0 {
		ttl = 300 * time.Second
	}
	currentFinder = redisFinder{client: client, ttl: ttl}
}

func ResetForTest() {
	currentFinder = noopFinder{}
}

func SetIDsForTest(ids []string) {
	copied := append([]string(nil), ids...)
	currentFinder = staticFinder{ids: copied}
}

func ActiveSessionIDs(ctx context.Context, limit int) ([]string, error) {
	return currentFinder.activeSessionIDs(ctx, limit)
}

func (noopFinder) activeSessionIDs(context.Context, int) ([]string, error) {
	return nil, ErrNotConfigured
}

func (f staticFinder) activeSessionIDs(context.Context, int) ([]string, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]string(nil), f.ids...), nil
}

func (f redisFinder) activeSessionIDs(ctx context.Context, limit int) ([]string, error) {
	if f.client == nil {
		return nil, ErrNotConfigured
	}
	lastSeenBySession := map[string]int64{}
	var cursor uint64
	for {
		keys, nextCursor, err := f.client.Scan(ctx, cursor, "session:*:last_seen", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			sessionID := parseSessionID(key)
			if sessionID == "" {
				continue
			}
			raw, err := f.client.Get(ctx, key).Result()
			if err != nil || strings.TrimSpace(raw) == "" {
				continue
			}
			lastSeen, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64)
			if err != nil {
				continue
			}
			if existing, ok := lastSeenBySession[sessionID]; !ok || lastSeen > existing {
				lastSeenBySession[sessionID] = lastSeen
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	return sortSessionIDsByLastSeen(lastSeenBySession, limit), nil
}

func parseSessionID(key string) string {
	if !strings.HasPrefix(key, "session:") || !strings.HasSuffix(key, ":last_seen") {
		return ""
	}
	trimmed := strings.TrimPrefix(key, "session:")
	trimmed = strings.TrimSuffix(trimmed, ":last_seen")
	return strings.TrimSpace(trimmed)
}

func sortSessionIDsByLastSeen(lastSeenBySession map[string]int64, limit int) []string {
	type pair struct {
		sessionID string
		lastSeen  int64
	}
	pairs := make([]pair, 0, len(lastSeenBySession))
	for sessionID, lastSeen := range lastSeenBySession {
		pairs = append(pairs, pair{sessionID: sessionID, lastSeen: lastSeen})
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].lastSeen != pairs[j].lastSeen {
			return pairs[i].lastSeen > pairs[j].lastSeen
		}
		return pairs[i].sessionID < pairs[j].sessionID
	})
	if limit > 0 && len(pairs) > limit {
		pairs = pairs[:limit]
	}
	ids := make([]string, 0, len(pairs))
	for _, item := range pairs {
		ids = append(ids, item.sessionID)
	}
	return ids
}
