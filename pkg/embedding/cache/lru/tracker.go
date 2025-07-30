package lru

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// AsyncTracker handles asynchronous LRU access tracking
type AsyncTracker struct {
	updates chan accessUpdate
	batch   map[string][]accessUpdate
	batchMu sync.Mutex

	flushInterval time.Duration
	batchSize     int

	redis   RedisClient
	logger  observability.Logger
	metrics observability.MetricsClient

	stopCh chan struct{}
	wg     sync.WaitGroup
}

// accessUpdate represents a cache access event
type accessUpdate struct {
	TenantID  uuid.UUID
	Key       string
	Timestamp time.Time
}

// NewAsyncTracker creates a new async tracker
func NewAsyncTracker(redis RedisClient, config *Config, logger observability.Logger, metrics observability.MetricsClient) *AsyncTracker {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.lru.tracker")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	// Use configured buffer size, default to 1000 if not set
	bufferSize := config.TrackingBufferSize
	if bufferSize <= 0 {
		bufferSize = 1000
	}

	t := &AsyncTracker{
		updates:       make(chan accessUpdate, bufferSize),
		batch:         make(map[string][]accessUpdate),
		flushInterval: config.FlushInterval,
		batchSize:     config.TrackingBatchSize,
		redis:         redis,
		logger:        logger,
		metrics:       metrics,
		stopCh:        make(chan struct{}),
	}

	// Start processing loops
	t.wg.Add(2)
	go t.processLoop()
	go t.flushLoop()

	return t
}

// Track records an access (non-blocking)
func (t *AsyncTracker) Track(tenantID uuid.UUID, key string) {
	select {
	case t.updates <- accessUpdate{
		TenantID:  tenantID,
		Key:       key,
		Timestamp: time.Now(),
	}:
	default:
		// Drop if channel full - tracking is best effort
		t.metrics.IncrementCounterWithLabels("lru.tracker.dropped", 1, nil)
	}
}

// Stop gracefully stops the tracker
func (t *AsyncTracker) Stop() {
	close(t.stopCh)
	close(t.updates)
	t.wg.Wait()

	// Final flush
	t.flushAll()
}

func (t *AsyncTracker) processLoop() {
	defer t.wg.Done()

	for {
		select {
		case <-t.stopCh:
			return
		case update, ok := <-t.updates:
			if !ok {
				return
			}

			t.batchMu.Lock()

			scoreKey := fmt.Sprintf("cache:lru:{%s}", update.TenantID.String())
			t.batch[scoreKey] = append(t.batch[scoreKey], update)

			// Flush if batch is large enough
			if len(t.batch[scoreKey]) >= t.batchSize {
				updates := t.batch[scoreKey]
				delete(t.batch, scoreKey)
				t.batchMu.Unlock()

				t.flush(context.Background(), scoreKey, updates)
			} else {
				t.batchMu.Unlock()
			}
		}
	}
}

func (t *AsyncTracker) flushLoop() {
	defer t.wg.Done()

	ticker := time.NewTicker(t.flushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.flushAll()
		}
	}
}

func (t *AsyncTracker) flushAll() {
	t.batchMu.Lock()

	// Copy and clear batch
	toFlush := make(map[string][]accessUpdate)
	for k, v := range t.batch {
		toFlush[k] = v
		delete(t.batch, k)
	}

	t.batchMu.Unlock()

	// Flush all batches
	for scoreKey, updates := range toFlush {
		t.flush(context.Background(), scoreKey, updates)
	}
}

func (t *AsyncTracker) flush(ctx context.Context, scoreKey string, updates []accessUpdate) {
	startTime := time.Now()

	_, err := t.redis.Execute(ctx, func() (interface{}, error) {
		pipe := t.redis.GetClient().Pipeline()

		for _, update := range updates {
			// Update score with timestamp
			score := float64(update.Timestamp.Unix())
			pipe.ZAdd(ctx, scoreKey, &redis.Z{
				Score:  score,
				Member: update.Key,
			})
		}

		_, err := pipe.Exec(ctx)
		return nil, err
	})

	duration := time.Since(startTime).Seconds()

	if err != nil {
		t.logger.Error("Failed to flush LRU updates", map[string]interface{}{
			"error":      err.Error(),
			"score_key":  scoreKey,
			"batch_size": len(updates),
		})
		t.metrics.IncrementCounterWithLabels("lru.tracker.flush_failed", 1, map[string]string{
			"reason": "redis_error",
		})
	} else {
		t.metrics.IncrementCounterWithLabels("lru.tracker.flush_success", 1, nil)
		t.metrics.RecordHistogram("lru.tracker.flush_duration", duration, map[string]string{
			"batch_size": fmt.Sprintf("%d", len(updates)),
		})

		t.logger.Debug("Flushed LRU updates", map[string]interface{}{
			"score_key":  scoreKey,
			"batch_size": len(updates),
			"duration":   duration,
		})
	}
}

// GetAccessScore retrieves the access score for a cache key
func (t *AsyncTracker) GetAccessScore(ctx context.Context, tenantID uuid.UUID, key string) (float64, error) {
	scoreKey := fmt.Sprintf("cache:lru:{%s}", tenantID.String())

	result, err := t.redis.Execute(ctx, func() (interface{}, error) {
		return t.redis.GetClient().ZScore(ctx, scoreKey, key).Result()
	})

	if err != nil {
		if err == redis.Nil {
			return 0, nil // Not found, return 0 score
		}
		return 0, fmt.Errorf("failed to get access score: %w", err)
	}

	score, ok := result.(float64)
	if !ok {
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}

	return score, nil
}

// GetLRUKeys retrieves keys in LRU order for a tenant
func (t *AsyncTracker) GetLRUKeys(ctx context.Context, tenantID uuid.UUID, limit int) ([]string, error) {
	scoreKey := fmt.Sprintf("cache:lru:{%s}", tenantID.String())

	result, err := t.redis.Execute(ctx, func() (interface{}, error) {
		// Get oldest entries (lowest scores)
		return t.redis.GetClient().ZRange(ctx, scoreKey, 0, int64(limit-1)).Result()
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get LRU keys: %w", err)
	}

	keys, ok := result.([]string)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return keys, nil
}

// RemoveKeys removes keys from the LRU tracking
func (t *AsyncTracker) RemoveKeys(ctx context.Context, tenantID uuid.UUID, keys []string) error {
	if len(keys) == 0 {
		return nil
	}

	scoreKey := fmt.Sprintf("cache:lru:{%s}", tenantID.String())

	members := make([]interface{}, len(keys))
	for i, key := range keys {
		members[i] = key
	}

	_, err := t.redis.Execute(ctx, func() (interface{}, error) {
		return t.redis.GetClient().ZRem(ctx, scoreKey, members...).Result()
	})

	if err != nil {
		return fmt.Errorf("failed to remove keys from LRU: %w", err)
	}

	return nil
}

// GetStats returns tracker statistics
func (t *AsyncTracker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"buffer_size":    len(t.updates),
		"batch_size":     t.batchSize,
		"flush_interval": t.flushInterval.String(),
		"total_tracked":  0, // TODO: Add counter
	}
}

// Run starts the tracker background process
func (t *AsyncTracker) Run(ctx context.Context) {
	// Already started in NewAsyncTracker
	// This method exists for interface compatibility
}
