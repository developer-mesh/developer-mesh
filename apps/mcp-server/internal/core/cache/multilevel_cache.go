package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/config"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	lru "github.com/hashicorp/golang-lru/v2"
	redis "github.com/redis/go-redis/v9"
)

// MultiLevelCache implements a tiered caching strategy for optimal performance
type MultiLevelCache struct {
	// L1: In-memory LRU cache for ultra-fast access
	l1Cache *lru.Cache[string, *CacheItem]
	l1TTL   time.Duration
	l1Mutex sync.RWMutex

	// L2: Redis distributed cache for shared state
	l2Client *redis.Client
	l2TTL    time.Duration

	// Configuration
	config *config.CacheConfig

	// Metrics and monitoring
	logger  observability.Logger
	metrics observability.MetricsClient

	// Cache warming
	warmupTicker *time.Ticker
	warmupStop   chan bool
}

// CacheItem wraps cached data with metadata
type CacheItem struct {
	Data      interface{} `json:"data"`
	CreatedAt time.Time   `json:"created_at"`
	ExpiresAt time.Time   `json:"expires_at"`
	Version   int         `json:"version"`
}

// NewMultiLevelCache creates an optimized multi-level cache
func NewMultiLevelCache(
	cfg *config.CacheConfig,
	redisClient *redis.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) (*MultiLevelCache, error) {
	// Initialize L1 cache
	l1Cache, err := lru.New[string, *CacheItem](cfg.InMemory.MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache: %w", err)
	}

	mlc := &MultiLevelCache{
		l1Cache:  l1Cache,
		l1TTL:    cfg.InMemory.TTL,
		l2Client: redisClient,
		l2TTL:    cfg.Redis.TTL,
		config:   cfg,
		logger:   logger,
		metrics:  metrics,
	}

	// Start cache warming if enabled
	if cfg.Warming.Enabled {
		mlc.startCacheWarming()
	}

	return mlc, nil
}

// Get retrieves a context from cache with fallback strategy
func (mlc *MultiLevelCache) Get(ctx context.Context, key string) (*models.Context, error) {
	startTime := time.Now()
	defer func() {
		mlc.metrics.RecordLatency("cache_get", time.Since(startTime))
	}()

	// Try L1 cache first
	if item := mlc.getFromL1(key); item != nil {
		mlc.metrics.IncrementCounter("cache_l1_hit", 1)
		return item.Data.(*models.Context), nil
	}

	// Try L2 cache
	item, err := mlc.getFromL2(ctx, key)
	if err == nil && item != nil {
		mlc.metrics.IncrementCounter("cache_l2_hit", 1)
		// Promote to L1
		mlc.setInL1(key, item)
		return item.Data.(*models.Context), nil
	}

	mlc.metrics.IncrementCounter("cache_miss", 1)
	return nil, nil
}

// Set stores a context in both cache levels
func (mlc *MultiLevelCache) Set(ctx context.Context, key string, value *models.Context) error {
	startTime := time.Now()
	defer func() {
		mlc.metrics.RecordLatency("cache_set", time.Since(startTime))
	}()

	item := &CacheItem{
		Data:      value,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(mlc.l2TTL),
		Version:   1,
	}

	// Set in L1 (synchronous)
	mlc.setInL1(key, item)

	// Set in L2 (asynchronous for performance)
	go func() {
		if err := mlc.setInL2(context.Background(), key, item); err != nil {
			mlc.logger.Error("Failed to set in L2 cache", map[string]interface{}{"key": key, "error": err})
		}
	}()

	return nil
}

// Delete removes a context from all cache levels
func (mlc *MultiLevelCache) Delete(ctx context.Context, key string) error {
	// Delete from L1
	mlc.l1Mutex.Lock()
	mlc.l1Cache.Remove(key)
	mlc.l1Mutex.Unlock()

	// Delete from L2
	if err := mlc.l2Client.Del(ctx, mlc.l2Key(key)).Err(); err != nil {
		mlc.logger.Error("Failed to delete from L2 cache", map[string]interface{}{"key": key, "error": err})
		return err
	}

	return nil
}

// InvalidatePattern removes all keys matching a pattern
func (mlc *MultiLevelCache) InvalidatePattern(ctx context.Context, pattern string) error {
	// Clear L1 cache (simpler to clear all for patterns)
	mlc.l1Mutex.Lock()
	mlc.l1Cache.Purge()
	mlc.l1Mutex.Unlock()

	// Clear matching keys from L2
	iter := mlc.l2Client.Scan(ctx, 0, mlc.l2Key(pattern), 0).Iterator()
	for iter.Next(ctx) {
		if err := mlc.l2Client.Del(ctx, iter.Val()).Err(); err != nil {
			mlc.logger.Error("Failed to delete key from L2", map[string]interface{}{"key": iter.Val(), "error": err})
		}
	}

	return iter.Err()
}

// WarmCache preloads frequently accessed contexts
func (mlc *MultiLevelCache) WarmCache(ctx context.Context, contextIDs []string, loader func(string) (*models.Context, error)) error {
	for _, id := range contextIDs {
		// Check if already cached
		if cached, err := mlc.Get(ctx, id); err == nil && cached != nil {
			continue
		}

		// Load and cache
		context, err := loader(id)
		if err != nil {
			mlc.logger.Warn("Failed to warm cache for context", map[string]interface{}{"id": id, "error": err})
			continue
		}

		if err := mlc.Set(ctx, id, context); err != nil {
			mlc.logger.Warn("Failed to cache warmed context", map[string]interface{}{"id": id, "error": err})
		}
	}

	return nil
}

// L1 cache operations
func (mlc *MultiLevelCache) getFromL1(key string) *CacheItem {
	mlc.l1Mutex.RLock()
	defer mlc.l1Mutex.RUnlock()

	if item, ok := mlc.l1Cache.Get(key); ok {
		// Check expiration
		if time.Now().Before(item.ExpiresAt) {
			return item
		}
		// Remove expired item
		mlc.l1Cache.Remove(key)
	}
	return nil
}

func (mlc *MultiLevelCache) setInL1(key string, item *CacheItem) {
	mlc.l1Mutex.Lock()
	defer mlc.l1Mutex.Unlock()

	// Update expiration for L1
	l1Item := *item
	l1Item.ExpiresAt = time.Now().Add(mlc.l1TTL)
	mlc.l1Cache.Add(key, &l1Item)
}

// L2 cache operations
func (mlc *MultiLevelCache) getFromL2(ctx context.Context, key string) (*CacheItem, error) {
	data, err := mlc.l2Client.Get(ctx, mlc.l2Key(key)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	var item CacheItem
	if err := json.Unmarshal([]byte(data), &item); err != nil {
		return nil, err
	}

	// Check expiration
	if time.Now().After(item.ExpiresAt) {
		return nil, nil
	}

	return &item, nil
}

func (mlc *MultiLevelCache) setInL2(ctx context.Context, key string, item *CacheItem) error {
	data, err := json.Marshal(item)
	if err != nil {
		return err
	}

	return mlc.l2Client.Set(ctx, mlc.l2Key(key), data, mlc.l2TTL).Err()
}

func (mlc *MultiLevelCache) l2Key(key string) string {
	return fmt.Sprintf("mcp:context:%s", key)
}

// Cache warming
func (mlc *MultiLevelCache) startCacheWarming() {
	mlc.warmupTicker = time.NewTicker(mlc.config.Warming.WarmupInterval)
	mlc.warmupStop = make(chan bool)

	go func() {
		for {
			select {
			case <-mlc.warmupTicker.C:
				// This would be implemented to warm cache with recent/popular contexts
				mlc.logger.Debug("Cache warming triggered", map[string]interface{}{})
			case <-mlc.warmupStop:
				return
			}
		}
	}()
}

// Close gracefully shuts down the cache
func (mlc *MultiLevelCache) Close() error {
	if mlc.warmupTicker != nil {
		mlc.warmupTicker.Stop()
		close(mlc.warmupStop)
	}
	return nil
}
