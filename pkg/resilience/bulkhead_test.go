package resilience

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBulkhead(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	tests := []struct {
		name          string
		config        BulkheadConfig
		expectedCalls int
		expectedQueue int
	}{
		{
			name:          "with defaults",
			config:        BulkheadConfig{},
			expectedCalls: 10,
			expectedQueue: 0,
		},
		{
			name: "custom config",
			config: BulkheadConfig{
				MaxConcurrentCalls: 5,
				MaxQueueDepth:      20,
			},
			expectedCalls: 5,
			expectedQueue: 20,
		},
		{
			name: "with rate limiting",
			config: BulkheadConfig{
				MaxConcurrentCalls: 10,
				RateLimitConfig: &RateLimiterConfig{
					Limit:  100,
					Period: time.Minute,
				},
			},
			expectedCalls: 10,
			expectedQueue: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := NewBulkhead("test", tt.config, logger, metrics)
			require.NotNil(t, b)
			assert.Equal(t, "test", b.name)
			assert.Equal(t, tt.expectedCalls, cap(b.semaphore))

			if tt.expectedQueue > 0 {
				assert.NotNil(t, b.queue)
				assert.Equal(t, tt.expectedQueue, cap(b.queue))
			}

			if tt.config.RateLimitConfig != nil {
				assert.NotNil(t, b.rateLimiter)
			}

			err := b.Close()
			assert.NoError(t, err)
		})
	}
}

func TestBulkhead_Execute_Success(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 5,
		MaxQueueDepth:      0,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	ctx := context.Background()
	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	result, err := b.Execute(ctx, operation)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	stats := b.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.CompletedRequests)
	assert.Equal(t, int64(0), stats.RejectedRequests)
}

func TestBulkhead_Execute_Error(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 5,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	ctx := context.Background()
	expectedErr := errors.New("operation failed")
	operation := func(ctx context.Context) (interface{}, error) {
		return nil, expectedErr
	}

	result, err := b.Execute(ctx, operation)
	assert.Error(t, err)
	assert.Equal(t, expectedErr, err)
	assert.Nil(t, result)

	stats := b.GetStats()
	assert.Equal(t, int64(1), stats.TotalRequests)
	assert.Equal(t, int64(1), stats.CompletedRequests)
}

func TestBulkhead_ConcurrentLimit(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 3,
		MaxQueueDepth:      0, // No queueing
		EnableBackpressure: true,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	var activeCount atomic.Int32
	var maxActive atomic.Int32

	operation := func(ctx context.Context) (interface{}, error) {
		current := activeCount.Add(1)

		// Update max if this is higher
		for {
			max := maxActive.Load()
			if current <= max || maxActive.CompareAndSwap(max, current) {
				break
			}
		}

		time.Sleep(50 * time.Millisecond)
		activeCount.Add(-1)
		return "success", nil
	}

	// Start 10 concurrent operations
	var wg sync.WaitGroup
	results := make([]error, 10)

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			ctx := context.Background()
			_, err := b.Execute(ctx, operation)
			results[idx] = err
		}(i)
	}

	wg.Wait()

	// Should never have more than 3 concurrent executions
	assert.LessOrEqual(t, maxActive.Load(), int32(3))

	// Some requests should be rejected due to no queueing
	rejectedCount := 0
	for _, err := range results {
		if err == ErrBulkheadFull {
			rejectedCount++
		}
	}
	assert.Greater(t, rejectedCount, 0, "some requests should be rejected when bulkhead is full")

	stats := b.GetStats()
	assert.Greater(t, stats.RejectedRequests, int64(0))
}

func TestBulkhead_Queueing(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 2,
		MaxQueueDepth:      5,
		QueueTimeout:       1 * time.Second,
		EnableBackpressure: false,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(100 * time.Millisecond)
		return "success", nil
	}

	// Start 5 concurrent operations (2 active, 3 queued)
	var wg sync.WaitGroup
	successCount := atomic.Int32{}

	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_, err := b.Execute(ctx, operation)
			if err == nil {
				successCount.Add(1)
			}
		}()
	}

	wg.Wait()

	// All should succeed (2 immediate + 3 queued)
	assert.Equal(t, int32(5), successCount.Load())

	stats := b.GetStats()
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.Equal(t, int64(5), stats.CompletedRequests)
}

func TestBulkhead_QueueBackpressure(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 2,
		MaxQueueDepth:      2,
		QueueTimeout:       1 * time.Second,
		EnableBackpressure: true,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(200 * time.Millisecond)
		return "success", nil
	}

	// Start 10 concurrent operations
	var wg sync.WaitGroup
	rejectedCount := atomic.Int32{}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			_, err := b.Execute(ctx, operation)
			if err == ErrBulkheadQueueFull {
				rejectedCount.Add(1)
			}
		}()
	}

	// Give some time for operations to queue
	time.Sleep(50 * time.Millisecond)

	wg.Wait()

	// Should have some rejections due to full queue
	assert.Greater(t, rejectedCount.Load(), int32(0))

	stats := b.GetStats()
	assert.Greater(t, stats.RejectedRequests, int64(0))
}

func TestBulkhead_QueueTimeout(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 1,
		MaxQueueDepth:      5,
		QueueTimeout:       100 * time.Millisecond,
		EnableBackpressure: false,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	// Long-running operation
	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(500 * time.Millisecond)
		return "success", nil
	}

	// Start operation that will block
	go func() { _, _ = b.Execute(context.Background(), operation) }()
	time.Sleep(50 * time.Millisecond)

	// This should timeout waiting in queue
	ctx := context.Background()
	_, err := b.Execute(ctx, operation)
	assert.Equal(t, ErrBulkheadTimeout, err)

	stats := b.GetStats()
	assert.Greater(t, stats.TimedOutRequests, int64(0))
}

func TestBulkhead_ContextCancellation(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 1,
		MaxQueueDepth:      5,
		QueueTimeout:       10 * time.Second,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	// Block the bulkhead
	blocker := func(ctx context.Context) (interface{}, error) {
		time.Sleep(500 * time.Millisecond)
		return "success", nil
	}

	go func() { _, _ = b.Execute(context.Background(), blocker) }()
	time.Sleep(50 * time.Millisecond)

	// Create context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())

	// Start operation in goroutine
	var err error
	done := make(chan struct{})
	go func() {
		_, err = b.Execute(ctx, blocker)
		close(done)
	}()

	// Cancel context while operation is queued
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Wait for operation to complete
	<-done

	assert.Error(t, err)
	// Should be context canceled error
	assert.True(t, errors.Is(err, context.Canceled) || err == ErrBulkheadContextCanceled)
}

func TestBulkhead_RateLimiting(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 10,
		RateLimitConfig: &RateLimiterConfig{
			Limit:       5,
			Period:      1 * time.Second,
			BurstFactor: 1,
		},
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	// Try to execute 10 operations quickly
	successCount := 0
	rateLimitedCount := 0

	for i := 0; i < 10; i++ {
		ctx := context.Background()
		_, err := b.Execute(ctx, operation)
		if err == nil {
			successCount++
		} else if err.Error() == "rate limit exceeded for bulkhead test" {
			rateLimitedCount++
		}
	}

	// Should have some rate limited
	assert.LessOrEqual(t, successCount, 5, "should not exceed rate limit")
	assert.Greater(t, rateLimitedCount, 0, "should have rate limited requests")
}

func TestBulkhead_Metrics(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 5,
		MaxQueueDepth:      10,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(10 * time.Millisecond)
		return "success", nil
	}

	ctx := context.Background()
	_, err := b.Execute(ctx, operation)
	assert.NoError(t, err)

	// Check that metrics were recorded
	assert.Greater(t, metrics.getGauge("bulkhead_requests_total"), 0.0)
}

func TestBulkhead_Close(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 5,
		MaxQueueDepth:      10,
	}

	b := NewBulkhead("test", config, logger, metrics)

	err := b.Close()
	assert.NoError(t, err)

	// Second close should error
	err = b.Close()
	assert.Error(t, err)

	// Operations after close should error
	ctx := context.Background()
	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	_, err = b.Execute(ctx, operation)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "closed")
}

func TestBulkheadManager(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	configs := map[string]BulkheadConfig{
		"service1": {
			MaxConcurrentCalls: 5,
			MaxQueueDepth:      10,
		},
		"service2": {
			MaxConcurrentCalls: 3,
			MaxQueueDepth:      5,
		},
	}

	manager := NewBulkheadManager(configs, logger, metrics)
	require.NotNil(t, manager)

	// Test getting existing bulkhead
	b1 := manager.GetBulkhead("service1")
	assert.NotNil(t, b1)
	assert.Equal(t, "service1", b1.name)

	// Test getting same bulkhead again
	b1_again := manager.GetBulkhead("service1")
	assert.Same(t, b1, b1_again)

	// Test creating new bulkhead with default config
	b3 := manager.GetBulkhead("service3")
	assert.NotNil(t, b3)
	assert.Equal(t, "service3", b3.name)

	// Test Execute through manager
	ctx := context.Background()
	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	result, err := manager.Execute(ctx, "service1", operation)
	assert.NoError(t, err)
	assert.Equal(t, "success", result)

	// Test GetAllStats
	stats := manager.GetAllStats()
	assert.Len(t, stats, 3) // service1, service2, service3
	assert.Contains(t, stats, "service1")
	assert.Contains(t, stats, "service2")
	assert.Contains(t, stats, "service3")

	// Test Close
	err = manager.Close()
	assert.NoError(t, err)
}

func TestBulkheadManager_ConcurrentAccess(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	manager := NewBulkheadManager(nil, logger, metrics)

	// Concurrently get/create bulkheads
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "service"
			if idx%2 == 0 {
				name = "service1"
			}
			b := manager.GetBulkhead(name)
			assert.NotNil(t, b)
		}(i)
	}

	wg.Wait()

	// Should have exactly 2 bulkheads created
	stats := manager.GetAllStats()
	assert.Len(t, stats, 2)

	err := manager.Close()
	assert.NoError(t, err)
}

func TestBulkheadStats(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 5,
		MaxQueueDepth:      10,
	}

	b := NewBulkhead("test", config, logger, metrics)
	defer func() { _ = b.Close() }()

	// Execute some operations
	ctx := context.Background()
	successOp := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	for i := 0; i < 5; i++ {
		_, err := b.Execute(ctx, successOp)
		assert.NoError(t, err)
	}

	stats := b.GetStats()
	assert.Equal(t, "test", stats.Name)
	assert.Equal(t, int64(5), stats.TotalRequests)
	assert.Equal(t, int64(5), stats.CompletedRequests)
	assert.Equal(t, int64(0), stats.RejectedRequests)
	assert.Equal(t, int64(0), stats.TimedOutRequests)
	assert.Equal(t, int64(5), stats.MaxConcurrent)
	assert.Equal(t, int64(10), stats.MaxQueueDepth)
}

func TestDefaultBulkheadConfigs(t *testing.T) {
	// Verify default configs exist for expected services
	expectedServices := []string{
		"github_api",
		"harness_api",
		"database",
		"cache",
		"agent_execution",
		"workflow_execution",
	}

	for _, service := range expectedServices {
		config, exists := DefaultBulkheadConfigs[service]
		assert.True(t, exists, "default config should exist for %s", service)
		assert.Greater(t, config.MaxConcurrentCalls, 0, "max concurrent calls should be positive for %s", service)
		assert.GreaterOrEqual(t, config.MaxQueueDepth, 0, "max queue depth should be non-negative for %s", service)
		assert.Greater(t, config.QueueTimeout, time.Duration(0), "queue timeout should be positive for %s", service)
	}

	// Verify rate limiting is configured for API services
	apiServices := []string{"github_api", "harness_api", "agent_execution", "workflow_execution"}
	for _, service := range apiServices {
		config := DefaultBulkheadConfigs[service]
		assert.NotNil(t, config.RateLimitConfig, "rate limit config should exist for %s", service)
	}
}

func BenchmarkBulkhead_Execute(b *testing.B) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 100,
		MaxQueueDepth:      0,
	}

	bulkhead := NewBulkhead("bench", config, logger, metrics)
	defer func() { _ = bulkhead.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		return "success", nil
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = bulkhead.Execute(ctx, operation)
		}
	})
}

func BenchmarkBulkhead_ExecuteWithQueue(b *testing.B) {
	logger := observability.NewStandardLogger("test")
	metrics := newMockMetricsClient()

	config := BulkheadConfig{
		MaxConcurrentCalls: 10,
		MaxQueueDepth:      1000,
		QueueTimeout:       10 * time.Second,
	}

	bulkhead := NewBulkhead("bench", config, logger, metrics)
	defer func() { _ = bulkhead.Close() }()

	operation := func(ctx context.Context) (interface{}, error) {
		time.Sleep(time.Microsecond)
		return "success", nil
	}

	ctx := context.Background()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, _ = bulkhead.Execute(ctx, operation)
		}
	})
}
