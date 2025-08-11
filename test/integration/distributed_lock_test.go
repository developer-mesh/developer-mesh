package integration

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/services"
)

func TestDistributedDocumentLocking(t *testing.T) {
	// Skip if not running integration tests
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Setup Redis client
	redisAddr := os.Getenv("TEST_REDIS_ADDR")
	if redisAddr == "" {
		redisAddr = "127.0.0.1:6379" // Default Redis port
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
		DB:   1, // Use DB 1 for tests
	})
	defer func() { _ = redisClient.Close() }()

	// Clear test data
	ctx := context.Background()
	redisClient.FlushDB(ctx)

	// Create test dependencies
	logger := observability.NewLogger("test-distributed-lock")
	metrics := observability.NewMetricsClient()

	config := services.ServiceConfig{
		Logger:  logger,
		Metrics: metrics,
		Tracer:  observability.NoopStartSpan,
	}

	// Create lock service
	lockService := services.NewDocumentLockService(config, redisClient)

	t.Run("Single Agent Lock and Unlock", func(t *testing.T) {
		docID := uuid.New()
		agentID := "agent-1"

		// Acquire lock
		lock, err := lockService.LockDocument(ctx, docID, agentID, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, lock)
		assert.Equal(t, agentID, lock.AgentID)
		assert.Equal(t, docID, lock.DocumentID)

		// Verify lock is held
		isLocked, lockInfo, err := lockService.IsDocumentLocked(ctx, docID)
		require.NoError(t, err)
		assert.True(t, isLocked)
		assert.Equal(t, agentID, lockInfo.AgentID)

		// Release lock
		err = lockService.UnlockDocument(ctx, docID, agentID)
		require.NoError(t, err)

		// Verify lock is released
		isLocked, _, err = lockService.IsDocumentLocked(ctx, docID)
		require.NoError(t, err)
		assert.False(t, isLocked)
	})

	t.Run("Lock Conflict Between Agents", func(t *testing.T) {
		docID := uuid.New()
		agent1 := "agent-1"
		agent2 := "agent-2"

		// Agent 1 acquires lock
		lock1, err := lockService.LockDocument(ctx, docID, agent1, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, lock1)

		// Agent 2 tries to acquire lock (should fail)
		lock2, err := lockService.LockDocument(ctx, docID, agent2, 5*time.Minute)
		assert.Error(t, err)
		assert.Nil(t, lock2)
		assert.Contains(t, err.Error(), "locked by")

		// Clean up
		err = lockService.UnlockDocument(ctx, docID, agent1)
		assert.NoError(t, err)
	})

	t.Run("Lock Extension", func(t *testing.T) {
		docID := uuid.New()
		agentID := "agent-1"

		// Acquire lock with short duration
		lock, err := lockService.LockDocument(ctx, docID, agentID, 2*time.Second)
		require.NoError(t, err)
		originalExpiry := lock.ExpiresAt

		// Wait a moment
		time.Sleep(1 * time.Second)

		// Extend lock
		err = lockService.ExtendLock(ctx, docID, agentID, 5*time.Minute)
		require.NoError(t, err)

		// Verify extension
		isLocked, lockInfo, err := lockService.IsDocumentLocked(ctx, docID)
		require.NoError(t, err)
		assert.True(t, isLocked)
		assert.True(t, lockInfo.ExpiresAt.After(originalExpiry))

		// Clean up
		err = lockService.UnlockDocument(ctx, docID, agentID)
		assert.NoError(t, err)
	})

	t.Run("Section Level Locking", func(t *testing.T) {
		docID := uuid.New()
		agent1 := "agent-1"
		agent2 := "agent-2"
		section1 := "intro"
		section2 := "conclusion"

		// Agent 1 locks section 1
		sectionLock1, err := lockService.LockSection(ctx, docID, section1, agent1, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, sectionLock1)
		assert.Equal(t, section1, sectionLock1.SectionID)

		// Agent 2 can lock section 2
		sectionLock2, err := lockService.LockSection(ctx, docID, section2, agent2, 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, sectionLock2)
		assert.Equal(t, section2, sectionLock2.SectionID)

		// Agent 2 cannot lock section 1
		sectionLock3, err := lockService.LockSection(ctx, docID, section1, agent2, 5*time.Minute)
		assert.Error(t, err)
		assert.Nil(t, sectionLock3)
		assert.Contains(t, err.Error(), "section")

		// Get all section locks
		locks, err := lockService.GetSectionLocks(ctx, docID)
		require.NoError(t, err)
		assert.Len(t, locks, 2)

		// Clean up
		_ = lockService.UnlockSection(ctx, docID, section1, agent1)
		_ = lockService.UnlockSection(ctx, docID, section2, agent2)
	})

	t.Run("Concurrent Lock Attempts - Performance Test", func(t *testing.T) {
		docID := uuid.New()
		numAgents := 100
		successCount := 0
		var mu sync.Mutex
		var wg sync.WaitGroup

		start := time.Now()

		// Launch concurrent lock attempts
		for i := 0; i < numAgents; i++ {
			wg.Add(1)
			go func(agentNum int) {
				defer wg.Done()
				agentID := fmt.Sprintf("agent-%d", agentNum)

				// Try to acquire lock
				_, err := lockService.LockDocument(ctx, docID, agentID, 5*time.Minute)
				if err == nil {
					mu.Lock()
					successCount++
					mu.Unlock()

					// Hold lock briefly
					time.Sleep(10 * time.Millisecond)

					// Release lock
					_ = lockService.UnlockDocument(ctx, docID, agentID)
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		// Only one agent should succeed
		assert.Equal(t, 1, successCount, "Only one agent should acquire the lock")

		// Performance check - should handle 100 concurrent attempts quickly
		assert.Less(t, elapsed, 1*time.Second, "Should handle 100 concurrent lock attempts in under 1 second")

		// Average latency per attempt
		avgLatency := elapsed / time.Duration(numAgents)
		assert.Less(t, avgLatency, 10*time.Millisecond, "Average latency should be under 10ms")

		logger.Info("Concurrent lock performance", map[string]interface{}{
			"total_agents":   numAgents,
			"success_count":  successCount,
			"total_duration": elapsed.String(),
			"avg_latency":    avgLatency.String(),
		})
	})

	t.Run("Auto Lock Refresh", func(t *testing.T) {
		docID := uuid.New()
		agentID := "agent-1"

		// Acquire lock with very short duration
		initialLock, err := lockService.LockDocument(ctx, docID, agentID, 35*time.Second)
		require.NoError(t, err)
		originalExpiry := initialLock.ExpiresAt

		// Wait for auto-refresh to kick in (refresh interval is 30s)
		time.Sleep(32 * time.Second)

		// Check if lock is still valid and refreshed
		isLocked, lockInfo, err := lockService.IsDocumentLocked(ctx, docID)
		require.NoError(t, err)
		assert.True(t, isLocked)

		// The lock should have been auto-refreshed if expiry was approaching
		if time.Until(originalExpiry) < 30*time.Second {
			assert.True(t, lockInfo.ExpiresAt.After(originalExpiry), "Lock should have been auto-refreshed")
		}

		// Clean up
		_ = lockService.UnlockDocument(ctx, docID, agentID)
	})

	t.Run("Lock Expiration Cleanup", func(t *testing.T) {
		docID := uuid.New()
		agentID := "agent-1"

		// Acquire lock with very short duration
		lock, err := lockService.LockDocument(ctx, docID, agentID, 1*time.Second)
		require.NoError(t, err)
		assert.NotNil(t, lock)

		// Wait for lock to expire
		time.Sleep(2 * time.Second)

		// Check if lock is expired and cleaned up
		isLocked, _, err := lockService.IsDocumentLocked(ctx, docID)
		require.NoError(t, err)
		assert.False(t, isLocked, "Expired lock should be cleaned up")

		// Another agent should be able to acquire the lock
		lock2, err := lockService.LockDocument(ctx, docID, "agent-2", 5*time.Minute)
		require.NoError(t, err)
		assert.NotNil(t, lock2)

		// Clean up
		_ = lockService.UnlockDocument(ctx, docID, "agent-2")
	})

	t.Run("Performance Benchmark - 1000 Concurrent Locks", func(t *testing.T) {
		// This tests the requirement for handling 1000+ concurrent locks
		numDocs := 1000
		var wg sync.WaitGroup
		errors := 0
		var errorMu sync.Mutex

		start := time.Now()

		// Create 1000 concurrent locks on different documents
		for i := 0; i < numDocs; i++ {
			wg.Add(1)
			go func(docNum int) {
				defer wg.Done()

				docID := uuid.New()
				agentID := fmt.Sprintf("agent-%d", docNum)

				// Acquire lock
				_, err := lockService.LockDocument(ctx, docID, agentID, 5*time.Minute)
				if err != nil {
					errorMu.Lock()
					errors++
					errorMu.Unlock()
				}
			}(i)
		}

		wg.Wait()
		elapsed := time.Since(start)

		// All locks should succeed (different documents)
		assert.Equal(t, 0, errors, "All lock attempts should succeed")

		// Performance requirements
		assert.Less(t, elapsed, 10*time.Second, "Should handle 1000 locks in under 10 seconds")

		// Average latency per lock
		avgLatency := elapsed / time.Duration(numDocs)
		assert.Less(t, avgLatency, 10*time.Millisecond, "Average latency should be under 10ms")

		logger.Info("1000 concurrent locks performance", map[string]interface{}{
			"total_locks":    numDocs,
			"errors":         errors,
			"total_duration": elapsed.String(),
			"avg_latency":    avgLatency.String(),
		})
	})
}

// Benchmark test for lock acquisition
func BenchmarkLockAcquisition(b *testing.B) {
	// Setup
	redisClient := redis.NewClient(&redis.Options{
		Addr: "127.0.0.1:6379",
		DB:   2, // Use different DB for benchmarks
	})
	defer func() { _ = redisClient.Close() }()

	ctx := context.Background()
	redisClient.FlushDB(ctx)

	logger := observability.NewLogger("benchmark-lock")
	metrics := observability.NewMetricsClient()

	config := services.ServiceConfig{
		Logger:  logger,
		Metrics: metrics,
		Tracer:  observability.NoopStartSpan,
	}

	lockService := services.NewDocumentLockService(config, redisClient)

	b.ResetTimer()

	// Run benchmark
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			docID := uuid.New()
			agentID := uuid.New().String()

			// Acquire and release lock
			_, err := lockService.LockDocument(ctx, docID, agentID, 1*time.Minute)
			if err != nil {
				b.Fatal(err)
			}

			err = lockService.UnlockDocument(ctx, docID, agentID)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
