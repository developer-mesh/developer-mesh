//go:build integration && testcontainers
// +build integration,testcontainers

package webhook

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestWithRealRedis tests the context lifecycle with a real Redis instance
func TestWithRealRedis(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Start Redis container
	ctx := context.Background()
	redisContainer, redisAddr := startRedisContainer(t, ctx)
	defer redisContainer.Terminate(ctx)

	// Create Redis client
	redisClient := createRedisClient(t, redisAddr)
	defer redisClient.Close()

	// Run test suites
	t.Run("DistributedLocking", func(t *testing.T) {
		testDistributedLockingWithRealRedis(t, redisClient)
	})

	t.Run("BatchProcessing", func(t *testing.T) {
		testBatchProcessingWithRealRedis(t, redisClient)
	})

	t.Run("SearchPerformance", func(t *testing.T) {
		testSearchPerformanceWithRealRedis(t, redisClient)
	})

	t.Run("ConcurrentTransitions", func(t *testing.T) {
		testConcurrentTransitionsWithRealRedis(t, redisClient)
	})
}

// startRedisContainer starts a Redis container using testcontainers
func startRedisContainer(t *testing.T, ctx context.Context) (testcontainers.Container, string) {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)

	mappedPort, err := redisContainer.MappedPort(ctx, "6379")
	require.NoError(t, err)

	hostIP, err := redisContainer.Host(ctx)
	require.NoError(t, err)

	redisAddr := fmt.Sprintf("%s:%s", hostIP, mappedPort.Port())
	return redisContainer, redisAddr
}

// createRedisClient creates a Redis client for testing
func createRedisClient(t *testing.T, addr string) *redis.StreamsClient {
	logger := observability.NewLogger("test-redis")

	// Wait for Redis to be ready
	var client *redis.StreamsClient
	var err error

	for i := 0; i < 10; i++ {
		client, err = redis.NewStreamsClient(&redis.StreamsConfig{
			Addr:         addr,
			Password:     "",
			DB:           0,
			MaxRetries:   3,
			MinIdleConns: 1,
			PoolSize:     10,
		}, logger)

		if err == nil {
			// Test connection
			if err := client.GetClient().Ping(context.Background()).Err(); err == nil {
				break
			}
		}

		time.Sleep(500 * time.Millisecond)
	}

	require.NoError(t, err, "Failed to connect to Redis")
	return client
}

// testDistributedLockingWithRealRedis tests distributed locking with real Redis
func testDistributedLockingWithRealRedis(t *testing.T, redisClient *redis.StreamsClient) {
	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{data: make(map[string][]byte)}
	compression := &mockCompressionService{}

	// Create multiple lifecycle managers
	managers := make([]*ContextLifecycleManager, 5)
	for i := 0; i < 5; i++ {
		managers[i] = NewContextLifecycleManager(
			DefaultLifecycleConfig(),
			redisClient,
			storage,
			compression,
			logger,
		)
		defer managers[i].Stop()
	}

	ctx := context.Background()
	tenantID := "test-tenant"
	contextID := "test-context"

	// Test 1: Concurrent lock acquisition
	t.Run("ConcurrentAcquisition", func(t *testing.T) {
		var wg sync.WaitGroup
		successCount := 0
		var successMu sync.Mutex
		acquiredLocks := make([]*ContextLock, 0)
		var locksMu sync.Mutex

		// Try to acquire the same lock from multiple managers concurrently
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				lock, err := managers[idx].TryAcquireContextLock(ctx, tenantID, contextID)
				if err == nil {
					successMu.Lock()
					successCount++
					successMu.Unlock()

					locksMu.Lock()
					acquiredLocks = append(acquiredLocks, lock)
					locksMu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// Only one should succeed with TryAcquire
		assert.Equal(t, 1, successCount, "Only one manager should acquire the lock")

		// Release the lock
		for _, lock := range acquiredLocks {
			err := lock.Release(ctx)
			assert.NoError(t, err)
		}
	})

	// Test 2: Lock retry mechanism
	t.Run("LockRetryMechanism", func(t *testing.T) {
		// First manager acquires the lock
		lock1, err := managers[0].AcquireContextLock(ctx, tenantID, "retry-test")
		require.NoError(t, err)

		// Second manager tries to acquire with timeout
		done := make(chan bool)
		go func() {
			time.Sleep(500 * time.Millisecond)
			lock1.Release(ctx)
		}()

		go func() {
			lock2, err := managers[1].AcquireContextLock(ctx, tenantID, "retry-test")
			assert.NoError(t, err)
			if err == nil {
				lock2.Release(ctx)
			}
			done <- true
		}()

		select {
		case <-done:
			// Success
		case <-time.After(2 * time.Second):
			t.Fatal("Lock acquisition timed out")
		}
	})

	// Test 3: Lock expiration
	t.Run("LockExpiration", func(t *testing.T) {
		// Acquire lock but don't release it
		lock, err := managers[0].AcquireContextLock(ctx, tenantID, "expire-test")
		require.NoError(t, err)

		// Wait for lock to expire (30s is too long for test, so we'll extend instead)
		err = lock.ExtendLock(ctx, 1*time.Second)
		assert.NoError(t, err)

		// Wait for extended lock to expire
		time.Sleep(1500 * time.Millisecond)

		// Another manager should be able to acquire it now
		lock2, err := managers[1].TryAcquireContextLock(ctx, tenantID, "expire-test")
		assert.NoError(t, err)
		if err == nil {
			lock2.Release(ctx)
		}
	})
}

// testBatchProcessingWithRealRedis tests batch processing with real Redis
func testBatchProcessingWithRealRedis(t *testing.T, redisClient *redis.StreamsClient) {
	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{data: make(map[string][]byte)}
	compression := &mockCompressionService{}

	config := DefaultLifecycleConfig()
	config.TransitionBatchSize = 50
	config.TransitionInterval = 500 * time.Millisecond

	manager := NewContextLifecycleManager(
		config,
		redisClient,
		storage,
		compression,
		logger,
	)
	defer manager.Stop()

	ctx := context.Background()
	tenantID := "batch-tenant"

	// Create many contexts
	contextCount := 200
	for i := 0; i < contextCount; i++ {
		data := map[string]interface{}{
			"index": i,
			"data":  fmt.Sprintf("test-data-%d", i),
		}

		metadata := &ContextMetadata{
			ID:         fmt.Sprintf("ctx-%d", i),
			TenantID:   tenantID,
			Importance: 0.3,
			CreatedAt:  time.Now(),
		}

		err := manager.StoreContext(ctx, tenantID, data, metadata)
		require.NoError(t, err)
	}

	// Verify all contexts are in hot storage
	client := redisClient.GetClient()
	hotCount := 0
	for i := 0; i < contextCount; i++ {
		hotKey := fmt.Sprintf("context:hot:%s:ctx-%d", tenantID, i)
		exists, _ := client.Exists(ctx, hotKey).Result()
		if exists > 0 {
			hotCount++
		}
	}
	assert.Equal(t, contextCount, hotCount, "All contexts should be in hot storage")

	// Trigger batch transitions
	batchProcessor := manager.batchProcessor
	for i := 0; i < contextCount; i++ {
		req := TransitionRequest{
			TenantID:  tenantID,
			ContextID: fmt.Sprintf("ctx-%d", i),
			FromState: StateHot,
			ToState:   StateWarm,
			Priority:  1,
		}
		err := batchProcessor.AddTransition(req)
		require.NoError(t, err)
	}

	// Wait for batch processing to complete
	time.Sleep(2 * time.Second)

	// Verify contexts moved to warm storage
	warmCount := 0
	for i := 0; i < contextCount; i++ {
		warmKey := fmt.Sprintf("context:warm:%s:ctx-%d", tenantID, i)
		exists, _ := client.Exists(ctx, warmKey).Result()
		if exists > 0 {
			warmCount++
		}
	}

	// Should have transitioned most contexts
	assert.Greater(t, warmCount, contextCount*8/10, "At least 80% should be in warm storage")
}

// testSearchPerformanceWithRealRedis tests search performance with real Redis
func testSearchPerformanceWithRealRedis(t *testing.T, redisClient *redis.StreamsClient) {
	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{data: make(map[string][]byte)}
	compression := &mockCompressionService{}

	manager := NewContextLifecycleManager(
		DefaultLifecycleConfig(),
		redisClient,
		storage,
		compression,
		logger,
	)
	defer manager.Stop()

	ctx := context.Background()

	// Create contexts across multiple tenants
	tenants := []string{"tenant-a", "tenant-b", "tenant-c"}
	baseTime := time.Now()
	totalContexts := 0

	for i, tenantID := range tenants {
		for j := 0; j < 100; j++ {
			data := map[string]interface{}{
				"tool_id": fmt.Sprintf("tool-%d", j%5),
				"data":    fmt.Sprintf("test-data-%d-%d", i, j),
			}

			metadata := &ContextMetadata{
				ID:        fmt.Sprintf("ctx-%d-%d", i, j),
				TenantID:  tenantID,
				CreatedAt: baseTime.Add(time.Duration(i*100+j) * time.Minute),
			}

			err := manager.StoreContext(ctx, tenantID, data, metadata)
			require.NoError(t, err)
			totalContexts++
		}
	}

	// Test search performance
	testCases := []struct {
		name     string
		criteria *ContextSearchCriteria
		minCount int
	}{
		{
			name:     "SearchAll",
			criteria: &ContextSearchCriteria{},
			minCount: totalContexts,
		},
		{
			name: "SearchByTenant",
			criteria: &ContextSearchCriteria{
				TenantID: "tenant-b",
			},
			minCount: 100,
		},
		{
			name: "SearchByTimeRange",
			criteria: &ContextSearchCriteria{
				StartTime: baseTime.Add(50 * time.Minute),
				EndTime:   baseTime.Add(150 * time.Minute),
			},
			minCount: 50,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			results, err := manager.SearchContexts(ctx, tc.criteria)
			duration := time.Since(start)

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(results), tc.minCount)
			assert.Less(t, duration, 100*time.Millisecond, "Search should be fast")

			t.Logf("%s: Found %d results in %v", tc.name, len(results), duration)
		})
	}
}

// testConcurrentTransitionsWithRealRedis tests concurrent state transitions
func testConcurrentTransitionsWithRealRedis(t *testing.T, redisClient *redis.StreamsClient) {
	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{data: make(map[string][]byte)}
	compression := &mockCompressionService{}

	// Create multiple managers
	managerCount := 3
	managers := make([]*ContextLifecycleManager, managerCount)
	for i := 0; i < managerCount; i++ {
		managers[i] = NewContextLifecycleManager(
			DefaultLifecycleConfig(),
			redisClient,
			storage,
			compression,
			logger,
		)
		defer managers[i].Stop()
	}

	ctx := context.Background()
	tenantID := "concurrent-tenant"

	// Create shared contexts
	contextCount := 50
	for i := 0; i < contextCount; i++ {
		data := map[string]interface{}{
			"index": i,
		}

		err := managers[0].StoreContext(ctx, tenantID, data, &ContextMetadata{
			ID:       fmt.Sprintf("shared-ctx-%d", i),
			TenantID: tenantID,
		})
		require.NoError(t, err)
	}

	// Concurrent operations from different managers
	var wg sync.WaitGroup
	errChan := make(chan error, contextCount*managerCount)

	// Each manager tries to transition different contexts
	for m := 0; m < managerCount; m++ {
		for c := 0; c < contextCount; c++ {
			if c%managerCount == m { // Distribute work
				wg.Add(1)
				go func(managerIdx, ctxIdx int) {
					defer wg.Done()

					contextID := fmt.Sprintf("shared-ctx-%d", ctxIdx)

					// Try to transition
					err := managers[managerIdx].TransitionToWarm(ctx, tenantID, contextID)
					if err != nil {
						errChan <- err
					}

					// Try to read
					_, err = managers[managerIdx].GetContext(ctx, tenantID, contextID)
					if err != nil {
						errChan <- err
					}
				}(m, c)
			}
		}
	}

	wg.Wait()
	close(errChan)

	// Check errors - some are expected due to race conditions
	errorCount := 0
	for err := range errChan {
		if err != nil {
			errorCount++
			t.Logf("Expected error: %v", err)
		}
	}

	// Verify final state consistency
	client := redisClient.GetClient()
	hotCount := 0
	warmCount := 0

	for i := 0; i < contextCount; i++ {
		contextID := fmt.Sprintf("shared-ctx-%d", i)

		hotKey := fmt.Sprintf("context:hot:%s:%s", tenantID, contextID)
		hotExists, _ := client.Exists(ctx, hotKey).Result()

		warmKey := fmt.Sprintf("context:warm:%s:%s", tenantID, contextID)
		warmExists, _ := client.Exists(ctx, warmKey).Result()

		// Context should be in exactly one storage tier
		assert.True(t, (hotExists > 0) != (warmExists > 0),
			"Context %s should be in exactly one tier", contextID)

		if hotExists > 0 {
			hotCount++
		}
		if warmExists > 0 {
			warmCount++
		}
	}

	assert.Equal(t, contextCount, hotCount+warmCount, "All contexts should be accounted for")
	t.Logf("Final state: %d hot, %d warm", hotCount, warmCount)
}

// Benchmark tests
func BenchmarkDistributedLocking(b *testing.B) {
	ctx := context.Background()
	redisContainer, redisAddr := startRedisContainer(&testing.T{}, ctx)
	defer redisContainer.Terminate(ctx)

	redisClient := createRedisClient(&testing.T{}, redisAddr)
	defer redisClient.Close()

	logger := observability.NewLogger("bench")
	storage := &mockStorageBackend{data: make(map[string][]byte)}
	compression := &mockCompressionService{}

	manager := NewContextLifecycleManager(
		DefaultLifecycleConfig(),
		redisClient,
		storage,
		compression,
		logger,
	)
	defer manager.Stop()

	b.ResetTimer()

	b.Run("LockAcquisition", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			lock, err := manager.AcquireContextLock(ctx, "bench-tenant", fmt.Sprintf("ctx-%d", i))
			if err == nil {
				lock.Release(ctx)
			}
		}
	})

	b.Run("ConcurrentLocking", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				lock, err := manager.TryAcquireContextLock(ctx, "bench-tenant", fmt.Sprintf("ctx-%d", i%100))
				if err == nil {
					lock.Release(ctx)
				}
				i++
			}
		})
	})
}
