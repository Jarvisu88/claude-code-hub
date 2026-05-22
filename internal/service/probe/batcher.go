package probe

import (
	"context"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/model"
	"github.com/ding113/claude-code-hub/internal/pkg/logger"
)

const (
	defaultBatchSize  = 64
	defaultFlushEvery = 10 * time.Second
)

// ProbeLogRepo is the subset of repository.ProviderEndpointProbeLogRepository
// needed by the batcher.
type ProbeLogRepo interface {
	Create(ctx context.Context, log *model.ProviderEndpointProbeLog) (*model.ProviderEndpointProbeLog, error)
}

// LogBatcher collects probe log entries and flushes them in bulk.
type LogBatcher struct {
	logRepo   ProbeLogRepo
	buffer    []*model.ProviderEndpointProbeLog
	mu        sync.Mutex
	batchSize int
}

// NewLogBatcher creates a LogBatcher that flushes when the buffer reaches batchSize.
// If batchSize <= 0, defaultBatchSize is used.
func NewLogBatcher(repo ProbeLogRepo, batchSize int) *LogBatcher {
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	return &LogBatcher{
		logRepo:   repo,
		buffer:    make([]*model.ProviderEndpointProbeLog, 0, batchSize),
		batchSize: batchSize,
	}
}

// Add appends a log entry to the buffer. If the buffer reaches batchSize it
// is flushed synchronously.
func (b *LogBatcher) Add(log *model.ProviderEndpointProbeLog) {
	if log == nil {
		return
	}
	b.mu.Lock()
	b.buffer = append(b.buffer, log)
	shouldFlush := len(b.buffer) >= b.batchSize
	b.mu.Unlock()

	if shouldFlush {
		if err := b.Flush(context.Background()); err != nil {
			logger.Error().Err(err).Msg("probe log batcher: flush on full buffer failed")
		}
	}
}

// Flush persists all buffered entries and clears the buffer.
func (b *LogBatcher) Flush(ctx context.Context) error {
	b.mu.Lock()
	if len(b.buffer) == 0 {
		b.mu.Unlock()
		return nil
	}
	entries := b.buffer
	b.buffer = make([]*model.ProviderEndpointProbeLog, 0, b.batchSize)
	b.mu.Unlock()

	var firstErr error
	for _, entry := range entries {
		if _, err := b.logRepo.Create(ctx, entry); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			logger.Error().Err(err).Int("endpointId", entry.EndpointID).Msg("probe log batcher: failed to persist log")
		}
	}
	return firstErr
}

// Len returns the current number of buffered entries.
func (b *LogBatcher) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.buffer)
}
