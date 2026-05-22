package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ding113/claude-code-hub/internal/pkg/logger"
	"github.com/redis/go-redis/v9"
)

const (
	// Redis key prefixes
	queueKeyPrefix = "cch:notification:queue:"
	deadLetterKey  = "cch:notification:dead_letter"

	// Default configuration
	defaultQueueMaxRetries    = 3
	defaultRetryBaseWait      = 2 * time.Second
	defaultPollInterval       = 1 * time.Second
	defaultBRPopTimeout       = 5 * time.Second
	defaultQueueWorkerCount   = 3
)

// TaskHandler processes a dequeued task payload.
type TaskHandler func(ctx context.Context, payload []byte) error

// Option configures queue behaviour on a per-enqueue basis.
type Option func(*enqueueOptions)

type enqueueOptions struct {
	maxRetries int
	delay      time.Duration
}

// WithMaxRetries overrides the default retry count for a single enqueue call.
func WithMaxRetries(n int) Option {
	return func(o *enqueueOptions) { o.maxRetries = n }
}

// WithDelay schedules the task to be processed after a delay.
func WithDelay(d time.Duration) Option {
	return func(o *enqueueOptions) { o.delay = d }
}

// queueTask is the internal representation stored in Redis.
type queueTask struct {
	ID         string          `json:"id"`
	TaskType   string          `json:"task_type"`
	Payload    json.RawMessage `json:"payload"`
	Retries    int             `json:"retries"`
	MaxRetries int             `json:"max_retries"`
	CreatedAt  time.Time       `json:"created_at"`
	LastError  string          `json:"last_error,omitempty"`
}

// Queue is a Redis-backed asynchronous task queue with retry and dead-letter support.
type Queue struct {
	redis       *redis.Client
	handlers    map[string]TaskHandler
	mu          sync.RWMutex
	workerCount int
	cancel      context.CancelFunc
	wg          sync.WaitGroup
	stopped     chan struct{}
}

// NewQueue creates a new Redis-backed notification queue.
// It accepts *redis.Client directly so callers can pass database.RedisClient.Client
// or any *redis.Client instance.
func NewQueue(redisClient *redis.Client) *Queue {
	return &Queue{
		redis:       redisClient,
		handlers:    make(map[string]TaskHandler),
		workerCount: defaultQueueWorkerCount,
		stopped:     make(chan struct{}),
	}
}

// RegisterHandler registers a handler for a specific task type.
func (q *Queue) RegisterHandler(taskType string, handler TaskHandler) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.handlers[taskType] = handler
}

// Enqueue adds a task to the queue.
func (q *Queue) Enqueue(ctx context.Context, taskType string, payload any, opts ...Option) error {
	options := &enqueueOptions{
		maxRetries: defaultQueueMaxRetries,
	}
	for _, opt := range opts {
		opt(options)
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}

	task := &queueTask{
		ID:         fmt.Sprintf("task_%s_%d", taskType, time.Now().UnixNano()),
		TaskType:   taskType,
		Payload:    payloadBytes,
		MaxRetries: options.maxRetries,
		CreatedAt:  time.Now(),
	}

	taskBytes, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("failed to marshal task: %w", err)
	}

	queueKey := queueKeyPrefix + taskType

	if options.delay > 0 {
		score := float64(time.Now().Add(options.delay).UnixMilli())
		return q.redis.ZAdd(ctx, queueKey+":delayed", redis.Z{
			Score:  score,
			Member: string(taskBytes),
		}).Err()
	}

	return q.redis.LPush(ctx, queueKey, taskBytes).Err()
}

// Start launches background workers that dequeue and process tasks.
func (q *Queue) Start(ctx context.Context) {
	ctx, q.cancel = context.WithCancel(ctx)

	q.mu.RLock()
	taskTypes := make([]string, 0, len(q.handlers))
	for tt := range q.handlers {
		taskTypes = append(taskTypes, tt)
	}
	q.mu.RUnlock()

	for i := 0; i < q.workerCount; i++ {
		q.wg.Add(1)
		go q.workerLoop(ctx, i, taskTypes)
	}

	// Delayed-task promoter goroutine
	q.wg.Add(1)
	go q.delayedPromoter(ctx, taskTypes)

	logger.Info().Int("workers", q.workerCount).Int("task_types", len(taskTypes)).
		Msg("[NotificationQueue] Started")
}

// Stop gracefully shuts down all workers and waits for in-flight tasks.
func (q *Queue) Stop() {
	if q.cancel != nil {
		q.cancel()
	}
	q.wg.Wait()
	close(q.stopped)
	logger.Info().Msg("[NotificationQueue] Stopped")
}

// Done returns a channel that is closed when the queue has fully stopped.
func (q *Queue) Done() <-chan struct{} {
	return q.stopped
}

// workerLoop is the main loop for a single worker goroutine.
func (q *Queue) workerLoop(ctx context.Context, workerID int, taskTypes []string) {
	defer q.wg.Done()

	keys := make([]string, len(taskTypes))
	for i, tt := range taskTypes {
		keys[i] = queueKeyPrefix + tt
	}

	logger.Debug().Int("worker_id", workerID).Msg("[NotificationQueue] Worker started")

	for {
		select {
		case <-ctx.Done():
			logger.Debug().Int("worker_id", workerID).Msg("[NotificationQueue] Worker stopping")
			return
		default:
		}

		result, err := q.redis.BRPop(ctx, defaultBRPopTimeout, keys...).Result()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			// Timeout (redis.Nil) is expected when the queue is empty
			continue
		}

		if len(result) < 2 {
			continue
		}

		// result[0] = key name, result[1] = value
		var task queueTask
		if err := json.Unmarshal([]byte(result[1]), &task); err != nil {
			logger.Error().Err(err).Str("raw", result[1]).
				Msg("[NotificationQueue] Failed to unmarshal task")
			continue
		}

		q.processTask(ctx, &task, workerID)
	}
}

// processTask executes the handler and manages retries / dead letter.
func (q *Queue) processTask(ctx context.Context, task *queueTask, workerID int) {
	q.mu.RLock()
	handler, ok := q.handlers[task.TaskType]
	q.mu.RUnlock()

	if !ok {
		logger.Error().Str("task_type", task.TaskType).Str("task_id", task.ID).
			Msg("[NotificationQueue] No handler registered for task type")
		q.sendToDeadLetter(ctx, task, "no handler registered")
		return
	}

	err := handler(ctx, task.Payload)
	if err == nil {
		logger.Debug().Str("task_id", task.ID).Int("worker_id", workerID).
			Msg("[NotificationQueue] Task processed successfully")
		return
	}

	task.Retries++
	task.LastError = err.Error()

	logger.Warn().Err(err).Str("task_id", task.ID).
		Int("retry", task.Retries).Int("max_retries", task.MaxRetries).
		Msg("[NotificationQueue] Task failed")

	if task.Retries >= task.MaxRetries {
		q.sendToDeadLetter(ctx, task, err.Error())
		return
	}

	// Re-enqueue with exponential backoff
	backoff := defaultRetryBaseWait * time.Duration(1<<uint(task.Retries-1))
	q.requeue(ctx, task, backoff)
}

// requeue puts a failed task back with a delay via the delayed sorted set.
func (q *Queue) requeue(ctx context.Context, task *queueTask, delay time.Duration) {
	taskBytes, err := json.Marshal(task)
	if err != nil {
		logger.Error().Err(err).Str("task_id", task.ID).
			Msg("[NotificationQueue] Failed to marshal task for requeue")
		return
	}

	delayedKey := queueKeyPrefix + task.TaskType + ":delayed"
	score := float64(time.Now().Add(delay).UnixMilli())

	if err := q.redis.ZAdd(ctx, delayedKey, redis.Z{
		Score:  score,
		Member: string(taskBytes),
	}).Err(); err != nil {
		logger.Error().Err(err).Str("task_id", task.ID).
			Msg("[NotificationQueue] Failed to requeue task")
	}
}

// sendToDeadLetter moves a permanently failed task to the dead letter queue.
func (q *Queue) sendToDeadLetter(ctx context.Context, task *queueTask, reason string) {
	entry := map[string]interface{}{
		"task":      task,
		"reason":    reason,
		"failed_at": time.Now().Format(time.RFC3339),
	}

	entryBytes, err := json.Marshal(entry)
	if err != nil {
		logger.Error().Err(err).Str("task_id", task.ID).
			Msg("[NotificationQueue] Failed to marshal dead letter entry")
		return
	}

	if err := q.redis.LPush(ctx, deadLetterKey, entryBytes).Err(); err != nil {
		logger.Error().Err(err).Str("task_id", task.ID).
			Msg("[NotificationQueue] Failed to push to dead letter queue")
		return
	}

	logger.Warn().Str("task_id", task.ID).Str("task_type", task.TaskType).
		Str("reason", reason).Msg("[NotificationQueue] Task moved to dead letter queue")
}

// delayedPromoter periodically moves due delayed tasks into the main queue.
func (q *Queue) delayedPromoter(ctx context.Context, taskTypes []string) {
	defer q.wg.Done()

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			q.promoteDelayed(ctx, taskTypes)
		}
	}
}

// promoteDelayed checks each delayed sorted set and moves ready tasks.
func (q *Queue) promoteDelayed(ctx context.Context, taskTypes []string) {
	nowStr := fmt.Sprintf("%f", float64(time.Now().UnixMilli()))

	for _, tt := range taskTypes {
		delayedKey := queueKeyPrefix + tt + ":delayed"
		mainKey := queueKeyPrefix + tt

		results, err := q.redis.ZRangeByScore(ctx, delayedKey, &redis.ZRangeBy{
			Min: "-inf",
			Max: nowStr,
		}).Result()
		if err != nil {
			logger.Error().Err(err).Str("task_type", tt).
				Msg("[NotificationQueue] Failed to fetch delayed tasks")
			continue
		}

		for _, val := range results {
			removed, err := q.redis.ZRem(ctx, delayedKey, val).Result()
			if err != nil || removed == 0 {
				continue
			}
			if err := q.redis.LPush(ctx, mainKey, val).Err(); err != nil {
				logger.Error().Err(err).Str("task_type", tt).
					Msg("[NotificationQueue] Failed to promote delayed task")
			}
		}
	}
}

// DeadLetterSize returns the current size of the dead letter queue.
func (q *Queue) DeadLetterSize(ctx context.Context) (int64, error) {
	return q.redis.LLen(ctx, deadLetterKey).Result()
}

// DrainDeadLetter reads up to `limit` entries from the dead letter queue.
func (q *Queue) DrainDeadLetter(ctx context.Context, limit int64) ([]string, error) {
	return q.redis.LRange(ctx, deadLetterKey, 0, limit-1).Result()
}
