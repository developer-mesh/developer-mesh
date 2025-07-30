package lru

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestAsyncTracker_Track(t *testing.T) {

	mockRedis := &MockRedisClient{}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil)

	config := &Config{
		TrackingBatchSize: 5,
		FlushInterval:     100 * time.Millisecond,
	}

	tracker := NewAsyncTracker(mockRedis, config, nil, nil)
	defer tracker.Stop()

	// Track multiple accesses
	tenantID := uuid.New()
	for i := 0; i < 10; i++ {
		tracker.Track(tenantID, fmt.Sprintf("key%d", i))
	}

	// Wait for processing
	time.Sleep(200 * time.Millisecond)

	// Verify flushes occurred
	mockRedis.AssertCalled(t, "Execute", mock.Anything, mock.Anything)
}

func TestAsyncTracker_BatchProcessing(t *testing.T) {

	var flushCount int
	var mu sync.Mutex

	mockRedis := &MockRedisClient{}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Run(func(args mock.Arguments) {
		mu.Lock()
		flushCount++
		mu.Unlock()
	})

	config := &Config{
		TrackingBatchSize: 3,
		FlushInterval:     1 * time.Second, // Long interval to test batch size trigger
	}

	tracker := NewAsyncTracker(mockRedis, config, nil, nil)
	defer tracker.Stop()

	tenantID := uuid.New()

	// Track exactly batch size
	for i := 0; i < 3; i++ {
		tracker.Track(tenantID, fmt.Sprintf("key%d", i))
	}

	// Wait for batch processing
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 1, flushCount, "Should flush once when batch size is reached")
	mu.Unlock()

	// Track more to trigger another batch
	for i := 3; i < 6; i++ {
		tracker.Track(tenantID, fmt.Sprintf("key%d", i))
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	assert.Equal(t, 2, flushCount, "Should flush twice")
	mu.Unlock()
}

func TestAsyncTracker_FlushInterval(t *testing.T) {

	var flushCount int
	var mu sync.Mutex

	mockRedis := &MockRedisClient{}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Run(func(args mock.Arguments) {
		mu.Lock()
		flushCount++
		mu.Unlock()
	})

	config := &Config{
		TrackingBatchSize: 100, // High batch size to test interval flush
		FlushInterval:     100 * time.Millisecond,
	}

	tracker := NewAsyncTracker(mockRedis, config, nil, nil)
	defer tracker.Stop()

	tenantID := uuid.New()

	// Track fewer than batch size
	tracker.Track(tenantID, "key1")
	tracker.Track(tenantID, "key2")

	// Wait for interval flush
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	assert.GreaterOrEqual(t, flushCount, 1, "Should flush at least once due to interval")
	mu.Unlock()
}

func TestAsyncTracker_ChannelFull(t *testing.T) {
	mockRedis := &MockRedisClient{}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil)

	config := &Config{
		TrackingBatchSize: 1000,
		FlushInterval:     10 * time.Second,
	}

	metrics := &mockMetricsClient{}
	tracker := NewAsyncTracker(mockRedis, config, nil, metrics)

	// Stop processing to fill channel
	close(tracker.stopCh)
	time.Sleep(10 * time.Millisecond)

	// Fill the channel
	tenantID := uuid.New()
	dropped := 0
	for i := 0; i < 20000; i++ {
		tracker.Track(tenantID, fmt.Sprintf("key%d", i))
		if metrics.droppedCount > 0 {
			dropped = metrics.droppedCount
			break
		}
	}

	assert.Greater(t, dropped, 0, "Should drop some updates when channel is full")
}

func TestAsyncTracker_GetAccessScore(t *testing.T) {
	ctx := context.Background()

	// Setup Redis client for real operations
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	mockRedis := &MockRedisClient{client: redisClient}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func() (interface{}, error))
		_, _ = fn()
	})

	config := &Config{
		TrackingBatchSize: 1,
		FlushInterval:     10 * time.Millisecond,
	}

	tracker := NewAsyncTracker(mockRedis, config, nil, nil)
	defer tracker.Stop()

	tenantID := uuid.New()
	key := "test_key"

	// Track access
	tracker.Track(tenantID, key)

	// Wait for flush
	time.Sleep(50 * time.Millisecond)

	// Get score
	score, err := tracker.GetAccessScore(ctx, tenantID, key)
	require.NoError(t, err)
	assert.Greater(t, score, float64(0))

	// Non-existent key
	score, err = tracker.GetAccessScore(ctx, tenantID, "non_existent")
	require.NoError(t, err)
	assert.Equal(t, float64(0), score)
}

func TestAsyncTracker_GetLRUKeys(t *testing.T) {
	ctx := context.Background()

	// Setup Redis client for real operations
	redisClient := GetTestRedisClient(t)
	defer func() { _ = redisClient.Close() }()

	if err := redisClient.Ping(ctx).Err(); err != nil {
		t.Skip("Redis not available")
	}

	if err := redisClient.FlushDB(ctx).Err(); err != nil {
		t.Fatalf("Failed to flush Redis: %v", err)
	}

	mockRedis := &MockRedisClient{client: redisClient}
	mockRedis.On("Execute", mock.Anything, mock.Anything).Return(nil, nil).Run(func(args mock.Arguments) {
		fn := args.Get(1).(func() (interface{}, error))
		_, _ = fn()
	})

	config := &Config{
		TrackingBatchSize: 5,
		FlushInterval:     10 * time.Millisecond,
	}

	tracker := NewAsyncTracker(mockRedis, config, nil, nil)
	defer tracker.Stop()

	tenantID := uuid.New()

	// Track accesses with different timestamps
	for i := 0; i < 5; i++ {
		tracker.Track(tenantID, fmt.Sprintf("key%d", i))
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for flush
	time.Sleep(50 * time.Millisecond)

	// Get LRU keys
	keys, err := tracker.GetLRUKeys(ctx, tenantID, 3)
	require.NoError(t, err)
	assert.Len(t, keys, 3)

	// Should return oldest keys first
	assert.Equal(t, "key0", keys[0])
	assert.Equal(t, "key1", keys[1])
	assert.Equal(t, "key2", keys[2])
}

// mockMetricsClient is a simple mock for testing metrics
type mockMetricsClient struct {
	droppedCount int
	mu           sync.Mutex
}

func (m *mockMetricsClient) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	if name == "lru.tracker.dropped" {
		m.mu.Lock()
		m.droppedCount++
		m.mu.Unlock()
	}
}

func (m *mockMetricsClient) IncrementCounter(name string, value float64)                          {}
func (m *mockMetricsClient) RecordHistogram(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) SetGauge(name string, value float64)                                  {}
func (m *mockMetricsClient) SetGaugeWithLabels(name string, value float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordGauge(name string, value float64, labels map[string]string)   {}
func (m *mockMetricsClient) RecordEvent(source, eventType string)                               {}
func (m *mockMetricsClient) RecordLatency(operation string, duration time.Duration)             {}
func (m *mockMetricsClient) RecordCounter(name string, value float64, labels map[string]string) {}
func (m *mockMetricsClient) RecordTimer(name string, duration time.Duration, labels map[string]string) {
}
func (m *mockMetricsClient) RecordCacheOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordOperation(component string, operation string, success bool, durationSeconds float64, labels map[string]string) {
}
func (m *mockMetricsClient) RecordAPIOperation(api string, operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) RecordDatabaseOperation(operation string, success bool, durationSeconds float64) {
}
func (m *mockMetricsClient) StartTimer(name string, labels map[string]string) func() {
	return func() {}
}
func (m *mockMetricsClient) RecordDuration(name string, duration time.Duration) {}
func (m *mockMetricsClient) Close() error                                       { return nil }
