package worker

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/queue"
)

type mockQueueClient struct {
	recvFunc   func(context.Context, int32, int32) ([]queue.Event, []string, error)
	deleteFunc func(context.Context, string) error
}

func (m *mockQueueClient) ReceiveEvents(ctx context.Context, maxMessages int32, waitSeconds int32) ([]queue.Event, []string, error) {
	return m.recvFunc(ctx, maxMessages, waitSeconds)
}
func (m *mockQueueClient) DeleteMessage(ctx context.Context, receiptHandle string) error {
	return m.deleteFunc(ctx, receiptHandle)
}

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

func TestRunWorker_ProcessesEvents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	queueCalled := int32(0)
	redisCalled := int32(0)
	processCalled := int32(0)

	queueClient := &mockQueueClient{
		recvFunc: func(ctx context.Context, max, wait int32) ([]queue.Event, []string, error) {
			atomic.AddInt32(&queueCalled, 1)
			event := queue.Event{
				EventID:   "d1",
				EventType: "pull_request",
				Payload:   json.RawMessage(`{"foo":1}`),
				Timestamp: time.Now(),
			}
			return []queue.Event{event}, []string{"h1"}, nil
		},
		deleteFunc: func(ctx context.Context, handle string) error {
			return nil
		},
	}
	redis := &mockRedisClient{
		existsFunc: func(ctx context.Context, key string) (int64, error) {
			atomic.AddInt32(&redisCalled, 1)
			return 0, nil
		},
		setFunc: func(ctx context.Context, key, value string, ttl time.Duration) error {
			return nil
		},
	}
	processFunc := func(ev queue.Event) error {
		atomic.AddInt32(&processCalled, 1)
		return nil
	}
	// Patch RunWorker to use our mocks
	go func() {
		_ = RunWorker(ctx, queueClient, redis, processFunc)
	}()
	time.Sleep(200 * time.Millisecond)
	if atomic.LoadInt32(&queueCalled) == 0 || atomic.LoadInt32(&redisCalled) == 0 || atomic.LoadInt32(&processCalled) == 0 {
		t.Errorf("Expected all components to be called at least once")
	}
}

// Additional tests for error paths, idempotency, and processFunc failure can be added similarly.
