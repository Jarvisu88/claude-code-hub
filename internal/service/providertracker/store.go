package providertracker

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/ding113/claude-code-hub/internal/database"
)

var ErrNotConfigured = errors.New("provider tracker not configured")

type counter interface {
	count(ctx context.Context) (map[int]int, error)
}

type noopCounter struct{}

type redisCounter struct {
	client *database.RedisClient
}

type staticCounter struct {
	counts map[int]int
	err    error
}

var currentCounter counter = noopCounter{}

func Configure(client *database.RedisClient) {
	if client == nil {
		currentCounter = noopCounter{}
		return
	}
	currentCounter = redisCounter{client: client}
}

func ResetForTest() {
	currentCounter = noopCounter{}
}

func SetCounterForTest(next counter) {
	if next == nil {
		currentCounter = noopCounter{}
		return
	}
	currentCounter = next
}

func SetCountsForTest(counts map[int]int) {
	copied := map[int]int{}
	for providerID, count := range counts {
		copied[providerID] = count
	}
	currentCounter = staticCounter{counts: copied}
}

func Count(ctx context.Context) (map[int]int, error) {
	return currentCounter.count(ctx)
}

func (noopCounter) count(context.Context) (map[int]int, error) {
	return nil, ErrNotConfigured
}

func (c redisCounter) count(ctx context.Context) (map[int]int, error) {
	results := map[int]int{}
	if c.client == nil {
		return nil, ErrNotConfigured
	}

	var cursor uint64
	for {
		keys, nextCursor, err := c.client.Scan(ctx, cursor, "session:*:provider", 100).Result()
		if err != nil {
			return nil, err
		}
		for _, key := range keys {
			raw, err := c.client.Get(ctx, key).Result()
			if err != nil || strings.TrimSpace(raw) == "" {
				continue
			}
			providerID, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || providerID <= 0 {
				continue
			}
			results[providerID]++
		}
		cursor = nextCursor
		if cursor == 0 {
			break
		}
	}

	return results, nil
}

func (c staticCounter) count(context.Context) (map[int]int, error) {
	if c.err != nil {
		return nil, c.err
	}
	results := map[int]int{}
	for providerID, count := range c.counts {
		results[providerID] = count
	}
	return results, nil
}
