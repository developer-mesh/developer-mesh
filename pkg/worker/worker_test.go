package worker

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockQueueClient mocks the Redis queue client
type mockQueueClient struct {
	receiveFunc func(context.Context, int32, int32) ([]queue.Event, []string, error)
	deleteFunc  func(context.Context, string) error
}

func (m *mockQueueClient) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.Event, []string, error) {
	return m.receiveFunc(ctx, maxMessages, waitSeconds)
}

func (m *mockQueueClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
	return m.deleteFunc(ctx, receiptHandle)
}

func (m *mockQueueClient) EnqueueEvent(ctx context.Context, event queue.Event) error {
	return nil
}

func (m *mockQueueClient) Close() error {
	return nil
}

func (m *mockQueueClient) Health(ctx context.Context) error {
	return nil
}

func (m *mockQueueClient) GetQueueDepth(ctx context.Context) (int64, error) {
	return 0, nil
}

// mockRedisClient for idempotency
type mockRedisClient struct {
	existsFunc func(context.Context, string) (int64, error)
	setFunc    func(context.Context, string, string, time.Duration) error
}

func (m *mockRedisClient) Exists(ctx context.Context, key string) (int64, error) {
	return m.existsFunc(ctx, key)
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	return m.setFunc(ctx, key, value, ttl)
}

func TestRedisWorker_ProcessesEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	queueCalled := int32(0)
	redisCalled := int32(0)
	processCalled := int32(0)

	// Create test event
	testEvent := queue.Event{
		EventID:    "test-event-1",
		EventType:  "pull_request",
		RepoName:   "test-repo",
		SenderName: "test-user",
		Payload:    json.RawMessage(`{"action": "opened"}`),
		AuthContext: &queue.EventAuthContext{
			TenantID:    "tenant-1",
			PrincipalID: "user-1",
		},
		Timestamp: time.Now(),
	}

	queueClient := &mockQueueClient{
		receiveFunc: func(ctx context.Context, max, wait int32) ([]queue.Event, []string, error) {
			atomic.AddInt32(&queueCalled, 1)
			return []queue.Event{testEvent}, []string{"handle-1"}, nil
		},
		deleteFunc: func(ctx context.Context, handle string) error {
			return nil
		},
	}

	redisClient := &mockRedisClient{
		existsFunc: func(ctx context.Context, key string) (int64, error) {
			atomic.AddInt32(&redisCalled, 1)
			return 0, nil // Not exists
		},
		setFunc: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	processFunc := func(event queue.Event) error {
		atomic.AddInt32(&processCalled, 1)
		assert.Equal(t, testEvent.EventID, event.EventID)
		return nil
	}

	// Create worker
	worker, err := NewRedisWorker(&Config{
		QueueClient:    queueClient,
		RedisClient:    redisClient,
		Processor:      processFunc,
		ConsumerName:   "test-worker",
		IdempotencyTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// Run worker
	err = worker.Run(ctx)

	// Context timeout is expected
	assert.ErrorIs(t, err, context.DeadlineExceeded)

	// Verify calls
	assert.Greater(t, atomic.LoadInt32(&queueCalled), int32(0))
	assert.Greater(t, atomic.LoadInt32(&redisCalled), int32(0))
	assert.Greater(t, atomic.LoadInt32(&processCalled), int32(0))
}

func TestRedisWorker_SkipsDuplicates(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	testEvent := queue.Event{
		EventID:   "duplicate-event",
		EventType: "push",
		Timestamp: time.Now(),
	}

	queueClient := &mockQueueClient{
		receiveFunc: func(ctx context.Context, max, wait int32) ([]queue.Event, []string, error) {
			return []queue.Event{testEvent}, []string{"handle-1"}, nil
		},
		deleteFunc: func(ctx context.Context, handle string) error {
			return nil
		},
	}

	redisClient := &mockRedisClient{
		existsFunc: func(ctx context.Context, key string) (int64, error) {
			return 1, nil // Already exists - duplicate
		},
		setFunc: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			t.Fatal("Set should not be called for duplicates")
			return nil
		},
	}

	processCalled := false
	processFunc := func(event queue.Event) error {
		processCalled = true
		t.Fatal("Process should not be called for duplicates")
		return nil
	}

	// Create worker
	worker, err := NewRedisWorker(&Config{
		QueueClient:    queueClient,
		RedisClient:    redisClient,
		Processor:      processFunc,
		ConsumerName:   "test-worker",
		IdempotencyTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// Run worker
	_ = worker.Run(ctx)

	// Process should not have been called
	assert.False(t, processCalled)
}

func TestRedisWorker_HandlesProcessingError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	deleteCalled := false

	testEvent := queue.Event{
		EventID:   "error-event",
		EventType: "issue",
		Timestamp: time.Now(),
	}

	queueClient := &mockQueueClient{
		receiveFunc: func(ctx context.Context, max, wait int32) ([]queue.Event, []string, error) {
			return []queue.Event{testEvent}, []string{"handle-1"}, nil
		},
		deleteFunc: func(ctx context.Context, handle string) error {
			deleteCalled = true
			return nil
		},
	}

	redisClient := &mockRedisClient{
		existsFunc: func(ctx context.Context, key string) (int64, error) {
			return 0, nil
		},
		setFunc: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			t.Fatal("Set should not be called on processing error")
			return nil
		},
	}

	processFunc := func(event queue.Event) error {
		return assert.AnError // Processing fails
	}

	// Create worker
	worker, err := NewRedisWorker(&Config{
		QueueClient:    queueClient,
		RedisClient:    redisClient,
		Processor:      processFunc,
		ConsumerName:   "test-worker",
		IdempotencyTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// Run worker
	_ = worker.Run(ctx)

	// Message should NOT be deleted on error
	assert.False(t, deleteCalled)
}

// Test backward compatibility with ProcessSQSEvent
func TestProcessSQSEvent_BackwardCompatibility(t *testing.T) {
	// Create SQS event (legacy format)
	sqsEvent := queue.SQSEvent{
		DeliveryID: "legacy-event-1",
		EventType:  "push",
		RepoName:   "test-repo",
		SenderName: "test-user",
		Payload:    json.RawMessage(`{"commits": 1}`),
		AuthContext: &queue.EventAuthContext{
			TenantID: "tenant-1",
		},
	}

	// ProcessSQSEvent should convert and process successfully
	err := ProcessSQSEvent(sqsEvent)
	// Push events are simulated to fail in the processor
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "simulated failure")
}
