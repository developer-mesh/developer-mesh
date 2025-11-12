//go:build integration

package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDistributedLockingMultiInstance tests distributed locking across multiple instances
func TestDistributedLockingMultiInstance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis client
	redisClient := setupTestRedis(t)
	defer cleanupTestRedis(redisClient)

	logger := observability.NewLogger("test")

	// Create mock storage and compression services
	storage := &mockStorageBackend{
		data: make(map[string][]byte),
	}
	compression := &mockCompressionService{}

	// Create multiple lifecycle managers (simulating multiple instances)
	managers := make([]*ContextLifecycleManager, 3)
	for i := 0; i < 3; i++ {
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

	// Test concurrent lock acquisition
	var wg sync.WaitGroup
	successCount := 0
	var successMu sync.Mutex

	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			lock, err := managers[idx].AcquireContextLock(ctx, tenantID, contextID)
			if err == nil {
				successMu.Lock()
				successCount++
				successMu.Unlock()

				// Hold lock for a bit
				time.Sleep(100 * time.Millisecond)

				// Release lock
				err = lock.Release(ctx)
				assert.NoError(t, err)
			}
		}(i)
	}

	wg.Wait()

	// All three should eventually succeed (with retries)
	assert.Equal(t, 3, successCount, "All instances should eventually acquire lock")
}

// TestBatchProcessingEfficiency tests batch processing performance
func TestBatchProcessingEfficiency(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	redisClient := setupTestRedis(t)
	defer cleanupTestRedis(redisClient)

	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{
		data: make(map[string][]byte),
	}
	compression := &mockCompressionService{}

	config := DefaultLifecycleConfig()
	config.TransitionBatchSize = 50
	config.TransitionInterval = 100 * time.Millisecond

	manager := NewContextLifecycleManager(
		config,
		redisClient,
		storage,
		compression,
		logger,
	)
	defer manager.Stop()

	ctx := context.Background()
	tenantID := "batch-test-tenant"

	// Create 100 contexts
	for i := 0; i < 100; i++ {
		data := map[string]interface{}{
			"index": i,
			"data":  fmt.Sprintf("test-data-%d", i),
		}

		metadata := &ContextMetadata{
			ID:         fmt.Sprintf("context-%d", i),
			TenantID:   tenantID,
			Importance: 0.3, // Low importance to ensure transition
		}

		err := manager.StoreContext(ctx, tenantID, data, metadata)
		require.NoError(t, err)
	}

	// Fast-forward time to trigger transitions
	// In real test, we would mock time or use shorter durations
	// For now, we'll manually trigger batch transitions
	batchProcessor := manager.batchProcessor

	// Add transition requests
	for i := 0; i < 100; i++ {
		req := TransitionRequest{
			TenantID:  tenantID,
			ContextID: fmt.Sprintf("context-%d", i),
			FromState: StateHot,
			ToState:   StateWarm,
			Priority:  1,
		}
		err := batchProcessor.AddTransition(req)
		require.NoError(t, err)
	}

	// Wait for batch processing
	time.Sleep(200 * time.Millisecond)

	// Verify transitions completed
	client := redisClient.GetClient()
	for i := 0; i < 100; i++ {
		warmKey := fmt.Sprintf("context:warm:%s:context-%d", tenantID, i)
		exists, err := client.Exists(ctx, warmKey).Result()
		require.NoError(t, err)
		assert.Equal(t, int64(1), exists, "Context should be in warm storage")
	}
}

// TestSearchContextsWithSortedSets tests the new sorted set-based search
func TestSearchContextsWithSortedSets(t *testing.T) {
	// Setup
	redisClient := setupTestRedis(t)
	defer cleanupTestRedis(redisClient)

	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{
		data: make(map[string][]byte),
	}
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

	// Create contexts for different tenants with different timestamps
	baseTime := time.Now()
	tenants := []string{"tenant-a", "tenant-b", "tenant-c"}

	for i, tenantID := range tenants {
		for j := 0; j < 5; j++ {
			data := map[string]interface{}{
				"tool_id": fmt.Sprintf("tool-%d", j%2),
				"data":    fmt.Sprintf("test-data-%d-%d", i, j),
			}

			metadata := &ContextMetadata{
				ID:        fmt.Sprintf("ctx-%d-%d", i, j),
				TenantID:  tenantID,
				CreatedAt: baseTime.Add(time.Duration(i*5+j) * time.Hour),
			}

			err := manager.StoreContext(ctx, tenantID, data, metadata)
			require.NoError(t, err)
		}
	}

	// Test 1: Search all contexts
	t.Run("SearchAll", func(t *testing.T) {
		criteria := &ContextSearchCriteria{}
		results, err := manager.SearchContexts(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, 15, len(results), "Should find all 15 contexts")
	})

	// Test 2: Search by tenant
	t.Run("SearchByTenant", func(t *testing.T) {
		criteria := &ContextSearchCriteria{
			TenantID: "tenant-b",
		}
		results, err := manager.SearchContexts(ctx, criteria)
		require.NoError(t, err)
		assert.Equal(t, 5, len(results), "Should find 5 contexts for tenant-b")

		for _, result := range results {
			assert.Equal(t, "tenant-b", result.TenantID)
		}
	})

	// Test 3: Search by time range
	t.Run("SearchByTimeRange", func(t *testing.T) {
		criteria := &ContextSearchCriteria{
			StartTime: baseTime.Add(5 * time.Hour),
			EndTime:   baseTime.Add(10 * time.Hour),
		}
		results, err := manager.SearchContexts(ctx, criteria)
		require.NoError(t, err)
		assert.Greater(t, len(results), 0, "Should find contexts in time range")

		for _, result := range results {
			assert.True(t, result.CreatedAt.After(criteria.StartTime) || result.CreatedAt.Equal(criteria.StartTime))
			assert.True(t, result.CreatedAt.Before(criteria.EndTime) || result.CreatedAt.Equal(criteria.EndTime))
		}
	})

	// Test 4: Search by tool ID
	t.Run("SearchByToolID", func(t *testing.T) {
		criteria := &ContextSearchCriteria{
			ToolID: "tool-0",
		}
		results, err := manager.SearchContexts(ctx, criteria)
		require.NoError(t, err)
		assert.Greater(t, len(results), 0, "Should find contexts with tool-0")
	})
}

// TestConcurrentStateTransitions tests race conditions in state transitions
func TestConcurrentStateTransitions(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup
	redisClient := setupTestRedis(t)
	defer cleanupTestRedis(redisClient)

	logger := observability.NewLogger("test")
	storage := &mockStorageBackend{
		data: make(map[string][]byte),
	}
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
	tenantID := "concurrent-test"
	contextID := "ctx-concurrent"

	// Store initial context
	data := map[string]interface{}{
		"test": "concurrent access",
	}
	err := manager.StoreContext(ctx, tenantID, data, nil)
	require.NoError(t, err)

	// Simulate concurrent operations
	var wg sync.WaitGroup
	errChan := make(chan error, 10)

	// Multiple goroutines trying to transition the same context
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := manager.TransitionToWarm(ctx, tenantID, contextID); err != nil {
				errChan <- err
			}
		}()
	}

	// Multiple goroutines trying to read the same context
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := manager.GetContext(ctx, tenantID, contextID); err != nil {
				errChan <- err
			}
		}()
	}

	wg.Wait()
	close(errChan)

	// Check for errors - some operations might fail due to state changes
	errorCount := 0
	for err := range errChan {
		if err != nil {
			errorCount++
			t.Logf("Expected error during concurrent access: %v", err)
		}
	}

	// Verify final state is consistent
	contextData, err := manager.GetContext(ctx, tenantID, contextID)
	assert.NoError(t, err)
	assert.NotNil(t, contextData)
}

// Helper functions for tests
func setupTestRedis(t *testing.T) *redis.StreamsClient {
	// For unit tests running in short mode, return nil
	// The integration tests should only run with the integration build tag
	if testing.Short() {
		t.Skip("Skipping Redis setup in short mode")
		return nil
	}

	// This function is only called in integration tests with the build tag
	// In real integration tests, this would use testcontainers to start Redis
	t.Skip("Real Redis integration tests require testcontainers setup")
	return nil
}

func cleanupTestRedis(client *redis.StreamsClient) {
	// Cleanup logic for test Redis
	if client != nil && client.GetClient() != nil {
		_ = client.GetClient().FlushDB(context.Background())
		_ = client.Close()
	}
}

// mockCompressionService implements CompressionService for testing
type mockCompressionService struct{}

func (m *mockCompressionService) Compress(data interface{}) ([]byte, error) {
	// Simple mock - just marshal to JSON
	return json.Marshal(data)
}

func (m *mockCompressionService) CompressWithSemantics(data []byte) ([]byte, float64, error) {
	// Mock compression - just return original with 1.0 ratio
	return data, 1.0, nil
}

func (m *mockCompressionService) Decompress(data []byte) ([]byte, error) {
	// Mock decompression - just return as is
	return data, nil
}
