package notification

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRedis creates a miniredis instance and returns a *redis.Client.
func setupTestRedis(t *testing.T) (*redis.Client, *miniredis.Miniredis) {
	t.Helper()
	mr, err := miniredis.Run()
	require.NoError(t, err)

	rc := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() {
		rc.Close()
		mr.Close()
	})

	return rc, mr
}

func TestQueue_EnqueueDequeue(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	var receivedPayload []byte
	var processed atomic.Int32

	q.RegisterHandler("test_task", func(ctx context.Context, payload []byte) error {
		receivedPayload = payload
		processed.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()

	// Enqueue a task
	type testData struct {
		Message string `json:"message"`
	}
	err := q.Enqueue(ctx, "test_task", testData{Message: "hello"})
	require.NoError(t, err)

	// Wait for processing
	require.Eventually(t, func() bool {
		return processed.Load() > 0
	}, 5*time.Second, 100*time.Millisecond, "task should be processed")

	// Verify payload
	var got testData
	err = json.Unmarshal(receivedPayload, &got)
	require.NoError(t, err)
	assert.Equal(t, "hello", got.Message)
}

func TestQueue_RetryOnFailure(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	var attempts atomic.Int32

	q.RegisterHandler("retry_task", func(ctx context.Context, payload []byte) error {
		n := attempts.Add(1)
		if n < 3 {
			return fmt.Errorf("transient failure attempt %d", n)
		}
		return nil // succeed on 3rd attempt
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()

	err := q.Enqueue(ctx, "retry_task", map[string]string{"key": "value"}, WithMaxRetries(3))
	require.NoError(t, err)

	// Wait for retries (exponential backoff: 2s, 4s, so ~6s worst case)
	require.Eventually(t, func() bool {
		return attempts.Load() >= 3
	}, 30*time.Second, 200*time.Millisecond, "handler should be called at least 3 times")
}

func TestQueue_DeadLetter(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	q.RegisterHandler("fail_task", func(ctx context.Context, payload []byte) error {
		return fmt.Errorf("permanent failure")
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()

	// Enqueue with max 1 retry so it fails quickly (1 initial attempt = goes to DLQ)
	err := q.Enqueue(ctx, "fail_task", map[string]string{"data": "test"}, WithMaxRetries(1))
	require.NoError(t, err)

	// Wait for the task to hit dead letter
	require.Eventually(t, func() bool {
		size, err := q.DeadLetterSize(ctx)
		return err == nil && size > 0
	}, 10*time.Second, 200*time.Millisecond, "dead letter queue should have entries")

	// Verify dead letter content
	entries, err := q.DrainDeadLetter(ctx, 10)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Parse the dead letter entry
	var dlEntry map[string]interface{}
	err = json.Unmarshal([]byte(entries[0]), &dlEntry)
	require.NoError(t, err)
	assert.Equal(t, "permanent failure", dlEntry["reason"])
}

func TestQueue_NoHandlerGoesToDeadLetter(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	// Register a handler so Start captures the key, then remove it
	q.RegisterHandler("vanished_type", func(ctx context.Context, payload []byte) error {
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)

	// Remove the handler after start so the worker finds no handler when processing
	q.mu.Lock()
	delete(q.handlers, "vanished_type")
	q.mu.Unlock()

	// Push a task directly to the queue key
	task := queueTask{
		ID:         "task_vanished_1",
		TaskType:   "vanished_type",
		Payload:    json.RawMessage(`{"foo":"bar"}`),
		MaxRetries: 1,
		CreatedAt:  time.Now(),
	}
	taskBytes, _ := json.Marshal(task)
	err := rc.LPush(ctx, queueKeyPrefix+"vanished_type", taskBytes).Err()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		size, err := q.DeadLetterSize(ctx)
		return err == nil && size > 0
	}, 10*time.Second, 200*time.Millisecond, "unhandled task should go to dead letter")

	q.Stop()
}

func TestQueue_GracefulShutdown(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	q.RegisterHandler("shutdown_task", func(ctx context.Context, payload []byte) error {
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	q.Start(ctx)

	// Cancel the context and stop
	cancel()
	q.Stop()

	// After stop, Done channel should be closed
	select {
	case <-q.Done():
		// expected
	case <-time.After(5 * time.Second):
		t.Fatal("queue did not stop in time")
	}
}

func TestQueue_EnqueueMultipleTasks(t *testing.T) {
	rc, _ := setupTestRedis(t)
	q := NewQueue(rc)

	var count atomic.Int32

	q.RegisterHandler("count_task", func(ctx context.Context, payload []byte) error {
		count.Add(1)
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	q.Start(ctx)
	defer q.Stop()

	// Enqueue 10 tasks
	for i := 0; i < 10; i++ {
		err := q.Enqueue(ctx, "count_task", map[string]int{"i": i})
		require.NoError(t, err)
	}

	require.Eventually(t, func() bool {
		return count.Load() == 10
	}, 10*time.Second, 100*time.Millisecond, "all 10 tasks should be processed")
}
