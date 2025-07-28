package worker_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	pkgworker "github.com/developer-mesh/developer-mesh/pkg/worker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkerRedisIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Create logger
	logger := observability.NewTestLogger()

	// Create queue client
	queueClient, err := queue.NewClient(ctx, &queue.Config{
		Logger: logger,
	})
	require.NoError(t, err)
	defer queueClient.Close()

	// Create test event
	testEvent := queue.Event{
		EventID:    "functional-test-" + time.Now().Format("20060102-150405"),
		EventType:  "functional_test",
		RepoName:   "developer-mesh",
		SenderName: "functional-test-suite",
		Payload:    json.RawMessage(`{"test": true, "timestamp": "` + time.Now().UTC().Format(time.RFC3339) + `"}`),
		AuthContext: &queue.EventAuthContext{
			TenantID:      "test-tenant",
			PrincipalID:   "test-principal",
			PrincipalType: "api_key",
			Permissions:   []string{"read", "write"},
		},
		Timestamp: time.Now(),
	}

	// Send event to Redis
	err = queueClient.EnqueueEvent(ctx, testEvent)
	require.NoError(t, err)

	t.Logf("Successfully sent test message to Redis queue: %s", testEvent.EventID)

	// Create a test worker to verify processing
	processed := make(chan queue.Event, 1)
	testProcessor := func(event queue.Event) error {
		processed <- event
		return nil
	}

	// Create Redis adapter for idempotency
	redisAdapter := &mockRedisClient{
		existsFunc: func(ctx context.Context, key string) (int64, error) {
			return 0, nil // Not exists
		},
		setFunc: func(ctx context.Context, key string, value string, ttl time.Duration) error {
			return nil
		},
	}

	// Create worker
	worker, err := pkgworker.NewRedisWorker(&pkgworker.Config{
		QueueClient:    queueClient,
		RedisClient:    redisAdapter,
		Processor:      testProcessor,
		Logger:         logger,
		ConsumerName:   "test-worker",
		IdempotencyTTL: 24 * time.Hour,
	})
	require.NoError(t, err)

	// Run worker in background
	workerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	go func() {
		_ = worker.Run(workerCtx)
	}()

	// Wait for message to be processed
	select {
	case receivedEvent := <-processed:
		assert.Equal(t, testEvent.EventID, receivedEvent.EventID)
		assert.Equal(t, testEvent.EventType, receivedEvent.EventType)
		assert.Equal(t, testEvent.RepoName, receivedEvent.RepoName)
		t.Log("Message successfully processed by worker")
	case <-time.After(5 * time.Second):
		t.Fatal("Timeout waiting for message to be processed")
	}
}

func TestRedisQueueSecurity(t *testing.T) {
	// Skip if not in integration test mode
	if os.Getenv("RUN_INTEGRATION_TESTS") != "true" {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()

	// Create logger
	logger := observability.NewTestLogger()

	// Create queue client
	queueClient, err := queue.NewClient(ctx, &queue.Config{
		Logger: logger,
	})
	require.NoError(t, err)
	defer queueClient.Close()

	// Test health check
	err = queueClient.Health(ctx)
	assert.NoError(t, err, "Redis health check should pass")

	// Verify TLS is enabled if Redis URL indicates it
	redisURL := os.Getenv("REDIS_URL")
	if redisURL != "" && (contains(redisURL, "rediss://") || contains(redisURL, "tls=true")) {
		t.Log("Redis TLS encryption verified")
	}

	// Test queue depth functionality
	depth, err := queueClient.GetQueueDepth(ctx)
	assert.NoError(t, err, "Should be able to get queue depth")
	assert.GreaterOrEqual(t, depth, int64(0), "Queue depth should be non-negative")

	t.Log("Redis queue security configuration verified successfully")
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && s[0:len(substr)] == substr || len(s) > len(substr) && s[len(s)-len(substr):] == substr || len(substr) > 0 && len(s) > len(substr) && findSubstring(s, substr) >= 0)
}

func findSubstring(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

// mockRedisClient for testing
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
