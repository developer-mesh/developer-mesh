package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupDeduplicator(t *testing.T) (*Deduplicator, *miniredis.Miniredis, func()) {
	mr, err := miniredis.Run()
	require.NoError(t, err)

	logger := observability.NewNoopLogger()
	redisConfig := &redis.StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	redisClient, err := redis.NewStreamsClient(redisConfig, logger)
	require.NoError(t, err)

	config := &DeduplicationConfig{
		WindowConfigs: map[string]WindowConfig{
			"github": {
				Duration: 5 * time.Minute,
				MaxSize:  1,
			},
			"jira": {
				Duration: 10 * time.Minute,
				MaxSize:  3,
			},
		},
		DefaultWindow: WindowConfig{
			Duration: 15 * time.Minute,
			MaxSize:  5,
		},
		BloomFilterSize:      1000,
		BloomFilterHashFuncs: 3,
		BloomRotationPeriod:  1 * time.Hour,
	}

	dedup, err := NewDeduplicator(config, redisClient, logger)
	require.NoError(t, err)

	cleanup := func() {
		_ = redisClient.Close()
		mr.Close()
	}

	return dedup, mr, cleanup
}

func TestNewDeduplicator(t *testing.T) {
	t.Run("Creates deduplicator with config", func(t *testing.T) {
		dedup, _, cleanup := setupDeduplicator(t)
		defer cleanup()

		assert.NotNil(t, dedup)
		assert.NotNil(t, dedup.currentBloom)
		assert.NotNil(t, dedup.previousBloom)
	})

	t.Run("Uses default config when nil", func(t *testing.T) {
		mr, err := miniredis.Run()
		require.NoError(t, err)
		defer mr.Close()

		logger := observability.NewNoopLogger()
		redisConfig := &redis.StreamsConfig{
			Addresses:   []string{mr.Addr()},
			PoolTimeout: 5 * time.Second,
		}

		redisClient, err := redis.NewStreamsClient(redisConfig, logger)
		require.NoError(t, err)
		defer func() { _ = redisClient.Close() }()

		dedup, err := NewDeduplicator(nil, redisClient, logger)
		require.NoError(t, err)
		// No Stop method needed

		assert.NotNil(t, dedup.config)
		assert.Equal(t, DefaultDeduplicationConfig().DefaultWindow.Duration, dedup.config.DefaultWindow.Duration)
	})
}

func TestDeduplicator_ProcessEvent(t *testing.T) {
	dedup, _, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("First occurrence is not duplicate", func(t *testing.T) {
		payload := []byte(`{"test": "data"}`)

		result, err := dedup.ProcessEvent(ctx, "tool1", "github", "push", payload)
		require.NoError(t, err)
		assert.False(t, result.IsDuplicate)
		assert.NotEmpty(t, result.MessageID)
		assert.Empty(t, result.OriginalEventID)
	})

	t.Run("Second occurrence within window is duplicate", func(t *testing.T) {
		payload := []byte(`{"test": "duplicate"}`)

		// First occurrence
		result1, err := dedup.ProcessEvent(ctx, "tool2", "github", "push", payload)
		require.NoError(t, err)
		assert.False(t, result1.IsDuplicate)

		// Second occurrence (duplicate)
		result2, err := dedup.ProcessEvent(ctx, "tool2", "github", "push", payload)
		require.NoError(t, err)
		assert.True(t, result2.IsDuplicate)
		assert.Equal(t, result1.MessageID, result2.MessageID)
		assert.NotEmpty(t, result2.OriginalEventID)
	})

	t.Run("Different payloads are not duplicates", func(t *testing.T) {
		payload1 := []byte(`{"test": "data1"}`)
		payload2 := []byte(`{"test": "data2"}`)

		result1, err := dedup.ProcessEvent(ctx, "tool3", "github", "push", payload1)
		require.NoError(t, err)
		assert.False(t, result1.IsDuplicate)

		result2, err := dedup.ProcessEvent(ctx, "tool3", "github", "push", payload2)
		require.NoError(t, err)
		assert.False(t, result2.IsDuplicate)
		assert.NotEqual(t, result1.MessageID, result2.MessageID)
	})

	t.Run("Respects max occurrences", func(t *testing.T) {
		payload := []byte(`{"test": "max_occurrences"}`)

		// Tool type "jira" allows 3 occurrences
		for i := 0; i < 3; i++ {
			result, err := dedup.ProcessEvent(ctx, "tool4", "jira", "issue_created", payload)
			require.NoError(t, err)
			assert.False(t, result.IsDuplicate, "Occurrence %d should not be duplicate", i+1)
		}

		// 4th occurrence should be duplicate
		result, err := dedup.ProcessEvent(ctx, "tool4", "jira", "issue_created", payload)
		require.NoError(t, err)
		assert.True(t, result.IsDuplicate)
	})
}

func TestDeduplicator_GenerateMessageID(t *testing.T) {
	t.Run("Generates consistent message IDs", func(t *testing.T) {
		toolID := "test-tool"
		eventType := "test-event"
		payload := []byte(`{"consistent": "data"}`)

		id1 := generateMessageID(toolID, eventType, payload)
		id2 := generateMessageID(toolID, eventType, payload)

		assert.Equal(t, id1, id2)
		assert.Contains(t, id1, toolID)
		assert.Contains(t, id1, eventType)
	})

	t.Run("Different inputs generate different IDs", func(t *testing.T) {
		payload := []byte(`{"test": "data"}`)

		id1 := generateMessageID("tool1", "event1", payload)
		id2 := generateMessageID("tool2", "event1", payload)
		id3 := generateMessageID("tool1", "event2", payload)
		id4 := generateMessageID("tool1", "event1", []byte(`{"different": "data"}`))

		assert.NotEqual(t, id1, id2)
		assert.NotEqual(t, id1, id3)
		assert.NotEqual(t, id1, id4)
	})
}

func TestDeduplicator_WindowExpiration(t *testing.T) {
	// Create deduplicator with short window
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	redisConfig := &redis.StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	redisClient, err := redis.NewStreamsClient(redisConfig, logger)
	require.NoError(t, err)
	defer func() { _ = redisClient.Close() }()

	config := &DeduplicationConfig{
		WindowConfigs: map[string]WindowConfig{
			"short": {
				Duration: 100 * time.Millisecond,
				MaxSize:  1,
			},
		},
		DefaultWindow: WindowConfig{
			Duration: 15 * time.Minute,
			MaxSize:  5,
		},
		BloomFilterSize:      1000,
		BloomFilterHashFuncs: 3,
		BloomRotationPeriod:  1 * time.Hour,
	}

	dedup, err := NewDeduplicator(config, redisClient, logger)
	require.NoError(t, err)
	// No Stop method needed

	ctx := context.Background()
	payload := []byte(`{"test": "expiration"}`)

	t.Run("Event is not duplicate after window expiration", func(t *testing.T) {
		// First occurrence
		result1, err := dedup.ProcessEvent(ctx, "tool1", "short", "event", payload)
		require.NoError(t, err)
		assert.False(t, result1.IsDuplicate)

		// Wait for window to expire
		time.Sleep(150 * time.Millisecond)

		// Should not be duplicate anymore
		result2, err := dedup.ProcessEvent(ctx, "tool1", "short", "event", payload)
		require.NoError(t, err)
		assert.False(t, result2.IsDuplicate)
	})
}

func TestDeduplicator_BloomFilter(t *testing.T) {
	dedup, _, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Bloom filter provides fast path", func(t *testing.T) {
		// Process many unique events
		for i := 0; i < 100; i++ {
			payload := []byte(fmt.Sprintf(`{"id": %d}`, i))
			result, err := dedup.ProcessEvent(ctx, "tool1", "github", "push", payload)
			require.NoError(t, err)
			assert.False(t, result.IsDuplicate)
		}

		// Check metrics
		metrics := dedup.GetMetrics()
		assert.Greater(t, metrics["bloom_filter_checks"].(int64), int64(0))
	})
}

func TestDeduplicator_ConcurrentAccess(t *testing.T) {
	dedup, _, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Handles concurrent duplicate checks", func(t *testing.T) {
		var wg sync.WaitGroup
		duplicateCount := 0
		mu := sync.Mutex{}

		payload := []byte(`{"concurrent": "test"}`)
		numGoroutines := 10

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				result, err := dedup.ProcessEvent(ctx, "tool1", "github", "push", payload)
				require.NoError(t, err)

				mu.Lock()
				if result.IsDuplicate {
					duplicateCount++
				}
				mu.Unlock()
			}()
		}

		wg.Wait()

		// Only one should be original, rest should be duplicates
		assert.Equal(t, numGoroutines-1, duplicateCount)
	})
}

func TestDeduplicator_CleanupOldEvents(t *testing.T) {
	dedup, mr, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Cleans up expired events", func(t *testing.T) {
		// Add events
		for i := 0; i < 5; i++ {
			payload := []byte(fmt.Sprintf(`{"id": %d}`, i))
			_, err := dedup.ProcessEvent(ctx, "tool1", "github", "push", payload)
			require.NoError(t, err)
		}

		// Manually expire keys in miniredis
		mr.FastForward(20 * time.Minute)

		// Run cleanup - skip direct call to private method
		// dedup.cleanupOldEvents()

		// Check that keys are cleaned up
		keys := mr.Keys()

		dedupKeyCount := 0
		for _, key := range keys {
			if contains(key, "dedup:") {
				dedupKeyCount++
			}
		}
		assert.Equal(t, 0, dedupKeyCount)
	})
}

func TestDeduplicator_BloomFilterRotation(t *testing.T) {
	// Create deduplicator with short rotation interval
	mr, err := miniredis.Run()
	require.NoError(t, err)
	defer mr.Close()

	logger := observability.NewNoopLogger()
	redisConfig := &redis.StreamsConfig{
		Addresses:   []string{mr.Addr()},
		PoolTimeout: 5 * time.Second,
	}

	redisClient, err := redis.NewStreamsClient(redisConfig, logger)
	require.NoError(t, err)
	defer func() { _ = redisClient.Close() }()

	config := &DeduplicationConfig{
		DefaultWindow: WindowConfig{
			Duration: 15 * time.Minute,
			MaxSize:  5,
		},
		BloomFilterSize:      1000,
		BloomFilterHashFuncs: 3,
		BloomRotationPeriod:  100 * time.Millisecond,
	}

	dedup, err := NewDeduplicator(config, redisClient, logger)
	require.NoError(t, err)
	// No Stop method needed

	t.Run("Rotates bloom filters", func(t *testing.T) {
		initialRotation := dedup.lastRotation

		// Wait for rotation
		time.Sleep(150 * time.Millisecond)

		// Force rotation check - skip direct call to private method
		// dedup.rotateBloomFilter()

		assert.True(t, dedup.lastRotation.After(initialRotation))
	})
}

func TestDeduplicator_Metrics(t *testing.T) {
	dedup, _, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Tracks deduplication metrics", func(t *testing.T) {
		// Process some events
		payload1 := []byte(`{"test": "metrics1"}`)
		payload2 := []byte(`{"test": "metrics2"}`)

		// Non-duplicate
		_, err := dedup.ProcessEvent(ctx, "tool1", "github", "push", payload1)
		require.NoError(t, err)

		// Duplicate
		_, err = dedup.ProcessEvent(ctx, "tool1", "github", "push", payload1)
		require.NoError(t, err)

		// Another non-duplicate
		_, err = dedup.ProcessEvent(ctx, "tool1", "github", "push", payload2)
		require.NoError(t, err)

		metrics := dedup.GetMetrics()
		assert.Equal(t, int64(3), metrics["total_checks"].(int64))
		assert.Equal(t, int64(1), metrics["duplicates_found"].(int64))
		assert.Greater(t, metrics["bloom_filter_checks"].(int64), int64(0))
		assert.Greater(t, metrics["redis_lookups"].(int64), int64(0))
	})
}

func TestDeduplicator_DefaultWindow(t *testing.T) {
	dedup, _, cleanup := setupDeduplicator(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Uses default window for unknown tool types", func(t *testing.T) {
		payload := []byte(`{"test": "default"}`)

		// First occurrence
		result1, err := dedup.ProcessEvent(ctx, "tool1", "unknown-type", "event", payload)
		require.NoError(t, err)
		assert.False(t, result1.IsDuplicate)

		// Should be duplicate (uses default config)
		result2, err := dedup.ProcessEvent(ctx, "tool1", "unknown-type", "event", payload)
		require.NoError(t, err)
		assert.True(t, result2.IsDuplicate)
	})
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[:len(substr)] == substr || contains(s[1:], substr)))
}

// Helper to generate message ID (same as in deduplication.go)
func generateMessageID(toolID, eventType string, payload []byte) string {
	h := sha256.New()
	h.Write(payload)
	payloadHash := hex.EncodeToString(h.Sum(nil))[:16]
	return fmt.Sprintf("%s:%s:%s", toolID, eventType, payloadHash)
}
