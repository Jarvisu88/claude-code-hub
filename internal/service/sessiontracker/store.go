package sessiontracker

import (
	"context"
	"errors"
	"sort"
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
	if limit <= 0 {
		limit = 20
	}
	ids := make([]string, 0, limit)
	seen := map[string]struct{}{}
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
			if _, ok := seen[sessionID]; ok {
				continue
			}
			seen[sessionID] = struct{}{}
			ids = append(ids, sessionID)
			if len(ids) >= limit {
				sort.Strings(ids)
				return ids, nil
			}
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func parseSessionID(key string) string {
	if !strings.HasPrefix(key, "session:") || !strings.HasSuffix(key, ":last_seen") {
		return ""
	}
	trimmed := strings.TrimPrefix(key, "session:")
	trimmed = strings.TrimSuffix(trimmed, ":last_seen")
	return strings.TrimSpace(trimmed)
}
