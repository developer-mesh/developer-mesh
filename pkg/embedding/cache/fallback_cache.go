package cache

import (
	"context"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// contextKey is a type for context keys to avoid collisions
type contextKey string

const (
	// contextKeyFromDegraded is used to prevent infinite recursion in degraded mode
	contextKeyFromDegraded contextKey = "from_degraded_mode"
)

// FallbackCache provides in-memory caching when Redis is unavailable.
// It implements a simple LRU cache with TTL support, designed to maintain
// basic functionality during Redis outages or network issues.
//
// The cache has a fixed capacity and evicts least recently used entries
// when full. All entries have a TTL and are periodically cleaned up.
//
// FallbackCache is safe for concurrent use.
type FallbackCache struct {
	entries    sync.Map // map[string]*FallbackEntry
	maxEntries int
	ttl        time.Duration
	logger     observability.Logger
	metrics    observability.MetricsClient
	mu         sync.RWMutex

	// LRU tracking
	accessList *accessList

	// Stats
	hits   int64
	misses int64
}

// FallbackEntry represents an in-memory cache entry
type FallbackEntry struct {
	Entry      *CacheEntry
	ExpiresAt  time.Time
	AccessTime time.Time
}

// accessNode represents a node in the LRU access list
type accessNode struct {
	key  string
	prev *accessNode
	next *accessNode
}

// accessList maintains LRU order
type accessList struct {
	head *accessNode
	tail *accessNode
	size int
	mu   sync.Mutex
}

// NewFallbackCache creates a new in-memory fallback cache.
// It starts a background goroutine for periodic cleanup of expired entries.
//
// Parameters:
//   - maxEntries: Maximum number of entries (defaults to 1000 if <= 0)
//   - ttl: Time-to-live for entries (defaults to 15 minutes if <= 0)
//   - logger: Logger for debugging (creates default if nil)
//   - metrics: Metrics client for monitoring (optional)
//
// The returned cache must not be modified after creation.
func NewFallbackCache(maxEntries int, ttl time.Duration, logger observability.Logger, metrics observability.MetricsClient) *FallbackCache {
	if maxEntries <= 0 {
		maxEntries = DefaultMaxCacheSize // Default entries
	}
	if ttl <= 0 {
		ttl = DefaultFallbackTTL // Default to 15 minutes
	}
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.fallback")
	}

	cache := &FallbackCache{
		maxEntries: maxEntries,
		ttl:        ttl,
		logger:     logger,
		metrics:    metrics,
		accessList: &accessList{},
	}

	// Start cleanup routine
	go cache.cleanupRoutine()

	return cache
}

// Get retrieves an entry from the fallback cache
func (fc *FallbackCache) Get(ctx context.Context, key string) (*CacheEntry, error) {
	// Check if entry exists
	if stored, ok := fc.entries.Load(key); ok {
		entry := stored.(*FallbackEntry)

		// Check if expired
		if time.Now().After(entry.ExpiresAt) {
			fc.entries.Delete(key)
			fc.accessList.remove(key)
			fc.recordMiss()
			return nil, nil
		}

		// Update access time
		entry.AccessTime = time.Now()
		fc.accessList.moveToFront(key)

		// Update hit count
		entry.Entry.HitCount++
		entry.Entry.LastAccessedAt = time.Now()

		fc.recordHit()

		fc.logger.Debug("Fallback cache hit", map[string]interface{}{
			"key": key,
		})

		return entry.Entry, nil
	}

	fc.recordMiss()
	return nil, nil
}

// Set stores an entry in the fallback cache
func (fc *FallbackCache) Set(ctx context.Context, key string, entry *CacheEntry) error {
	// Check if we need to evict
	fc.mu.Lock()
	currentSize := fc.accessList.size
	fc.mu.Unlock()

	if currentSize >= fc.maxEntries {
		// Evict LRU entry
		if evictKey := fc.accessList.removeTail(); evictKey != "" {
			fc.entries.Delete(evictKey)
			fc.logger.Debug("Evicted entry from fallback cache", map[string]interface{}{
				"key": evictKey,
			})
		}
	}

	// Create fallback entry
	fallbackEntry := &FallbackEntry{
		Entry:      entry,
		ExpiresAt:  time.Now().Add(fc.ttl),
		AccessTime: time.Now(),
	}

	// Store entry
	fc.entries.Store(key, fallbackEntry)
	fc.accessList.add(key)

	fc.logger.Debug("Stored in fallback cache", map[string]interface{}{
		"key": key,
		"ttl": fc.ttl,
	})

	return nil
}

// Delete removes an entry from the fallback cache
func (fc *FallbackCache) Delete(ctx context.Context, key string) error {
	fc.entries.Delete(key)
	fc.accessList.remove(key)
	return nil
}

// Clear removes all entries from the fallback cache
func (fc *FallbackCache) Clear(ctx context.Context) error {
	fc.entries.Range(func(key, value interface{}) bool {
		fc.entries.Delete(key)
		return true
	})
	fc.accessList.clear()
	return nil
}

// GetStats returns fallback cache statistics
func (fc *FallbackCache) GetStats() map[string]interface{} {
	fc.mu.RLock()
	defer fc.mu.RUnlock()

	count := 0
	fc.entries.Range(func(key, value interface{}) bool {
		count++
		return true
	})

	total := fc.hits + fc.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(fc.hits) / float64(total)
	}

	return map[string]interface{}{
		"type":        "in-memory",
		"entries":     count,
		"max_entries": fc.maxEntries,
		"hits":        fc.hits,
		"misses":      fc.misses,
		"hit_rate":    hitRate,
		"ttl_seconds": fc.ttl.Seconds(),
	}
}

// cleanupRoutine periodically removes expired entries
func (fc *FallbackCache) cleanupRoutine() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		now := time.Now()
		var expiredKeys []string

		fc.entries.Range(func(key, value interface{}) bool {
			entry := value.(*FallbackEntry)
			if now.After(entry.ExpiresAt) {
				expiredKeys = append(expiredKeys, key.(string))
			}
			return true
		})

		for _, key := range expiredKeys {
			fc.entries.Delete(key)
			fc.accessList.remove(key)
		}

		if len(expiredKeys) > 0 {
			fc.logger.Debug("Cleaned up expired entries", map[string]interface{}{
				"count": len(expiredKeys),
			})
		}
	}
}

func (fc *FallbackCache) recordHit() {
	fc.mu.Lock()
	fc.hits++
	fc.mu.Unlock()

	if fc.metrics != nil {
		fc.metrics.IncrementCounterWithLabels("cache.fallback.hit", 1, nil)
	}
}

func (fc *FallbackCache) recordMiss() {
	fc.mu.Lock()
	fc.misses++
	fc.mu.Unlock()

	if fc.metrics != nil {
		fc.metrics.IncrementCounterWithLabels("cache.fallback.miss", 1, nil)
	}
}

// LRU access list implementation

func (al *accessList) add(key string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	node := &accessNode{key: key}

	if al.head == nil {
		al.head = node
		al.tail = node
	} else {
		node.next = al.head
		al.head.prev = node
		al.head = node
	}

	al.size++
}

func (al *accessList) remove(key string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Simple implementation - in production, use a map for O(1) lookup
	current := al.head
	for current != nil {
		if current.key == key {
			if current.prev != nil {
				current.prev.next = current.next
			} else {
				al.head = current.next
			}

			if current.next != nil {
				current.next.prev = current.prev
			} else {
				al.tail = current.prev
			}

			al.size--
			break
		}
		current = current.next
	}
}

func (al *accessList) moveToFront(key string) {
	al.mu.Lock()
	defer al.mu.Unlock()

	// Find and move node to front
	current := al.head
	for current != nil {
		if current.key == key {
			// Already at front
			if current == al.head {
				return
			}

			// Remove from current position
			if current.prev != nil {
				current.prev.next = current.next
			}
			if current.next != nil {
				current.next.prev = current.prev
			} else {
				al.tail = current.prev
			}

			// Move to front
			current.prev = nil
			current.next = al.head
			al.head.prev = current
			al.head = current

			break
		}
		current = current.next
	}
}

func (al *accessList) removeTail() string {
	al.mu.Lock()
	defer al.mu.Unlock()

	if al.tail == nil {
		return ""
	}

	key := al.tail.key

	if al.tail.prev != nil {
		al.tail.prev.next = nil
		al.tail = al.tail.prev
	} else {
		al.head = nil
		al.tail = nil
	}

	al.size--
	return key
}

func (al *accessList) clear() {
	al.mu.Lock()
	defer al.mu.Unlock()

	al.head = nil
	al.tail = nil
	al.size = 0
}

// DegradedModeCache wraps the main cache with automatic fallback support.
// It monitors the primary cache health and automatically switches to a
// fallback cache when the primary becomes unavailable.
//
// The cache periodically checks if the primary has recovered and switches
// back automatically, providing seamless degraded mode operation.
//
// DegradedModeCache is safe for concurrent use.
type DegradedModeCache struct {
	primary     *SemanticCache
	fallback    *FallbackCache
	degraded    bool
	mu          sync.RWMutex
	logger      observability.Logger
	lastCheckAt time.Time
	checkPeriod time.Duration
}

// NewDegradedModeCache creates a cache with automatic fallback capabilities.
// It configures the fallback cache based on the primary cache settings,
// using 10% of the capacity and 25% of the TTL.
//
// Parameters:
//   - primary: The primary SemanticCache (can be nil for testing)
//   - logger: Logger for monitoring mode switches
//
// The cache starts in degraded mode if primary is nil.
func NewDegradedModeCache(primary *SemanticCache, logger observability.Logger) *DegradedModeCache {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.degraded")
	}

	// Create fallback cache with configuration from primary
	maxEntries := 1000
	ttl := 15 * time.Minute
	var metrics observability.MetricsClient

	if primary != nil {
		if primary.config != nil {
			if primary.config.MaxCacheSize > 0 {
				maxEntries = primary.config.MaxCacheSize / 10 // Use 10% of primary size
			}
			if primary.config.TTL > 0 {
				ttl = primary.config.TTL / 4 // Use 25% of primary TTL
			}
		}
		metrics = primary.metrics
	}

	// Start in degraded mode if primary is nil
	degraded := primary == nil

	return &DegradedModeCache{
		primary:     primary,
		fallback:    NewFallbackCache(maxEntries, ttl, logger, metrics),
		logger:      logger,
		checkPeriod: 5 * time.Second,
		degraded:    degraded,
	}
}

// Get retrieves from primary or fallback
func (dc *DegradedModeCache) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
	// Check if we should test primary
	if dc.shouldCheckPrimary() {
		dc.checkPrimaryHealth(ctx)
	}

	dc.mu.RLock()
	degraded := dc.degraded
	dc.mu.RUnlock()

	if !degraded && dc.primary != nil {
		// Add context to prevent infinite recursion
		ctxWithFlag := context.WithValue(ctx, contextKeyFromDegraded, true)

		// Try primary
		entry, err := dc.primary.Get(ctxWithFlag, query, embedding)
		if err == nil {
			return entry, nil
		}

		// Primary failed, switch to degraded mode
		dc.enterDegradedMode()
	}

	// Use fallback
	key := dc.getCacheKey(query)
	return dc.fallback.Get(ctx, key)
}

// Set stores in primary or fallback
func (dc *DegradedModeCache) Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error {
	dc.mu.RLock()
	degraded := dc.degraded
	dc.mu.RUnlock()

	if !degraded && dc.primary != nil {
		// Add context to prevent infinite recursion
		ctxWithFlag := context.WithValue(ctx, contextKeyFromDegraded, true)

		// Try primary
		err := dc.primary.Set(ctxWithFlag, query, embedding, results)
		if err == nil {
			return nil
		}

		// Primary failed, switch to degraded mode
		dc.enterDegradedMode()
	}

	// Use fallback
	key := dc.getCacheKey(query)
	entry := &CacheEntry{
		Query:           query,
		NormalizedQuery: query, // Simple normalization for fallback
		Embedding:       embedding,
		Results:         results,
		CachedAt:        time.Now(),
		HitCount:        0,
		LastAccessedAt:  time.Now(),
		TTL:             dc.fallback.ttl,
	}

	return dc.fallback.Set(ctx, key, entry)
}

// Helper methods

func (dc *DegradedModeCache) shouldCheckPrimary() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()

	return dc.degraded && time.Since(dc.lastCheckAt) > dc.checkPeriod
}

func (dc *DegradedModeCache) checkPrimaryHealth(ctx context.Context) {
	if dc.primary == nil || dc.primary.redis == nil {
		return
	}

	// Simple health check
	checkCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	err := dc.primary.redis.Health(checkCtx)

	dc.mu.Lock()
	dc.lastCheckAt = time.Now()

	if err == nil {
		// Primary is healthy again
		if dc.degraded {
			dc.logger.Info("Primary cache recovered, exiting degraded mode", nil)
			dc.degraded = false
		}
	}
	dc.mu.Unlock()
}

func (dc *DegradedModeCache) enterDegradedMode() {
	dc.mu.Lock()
	defer dc.mu.Unlock()

	if !dc.degraded {
		dc.degraded = true
		dc.logger.Warn("Primary cache failed, entering degraded mode", nil)
	}
}

func (dc *DegradedModeCache) getCacheKey(query string) string {
	// Simple key generation for fallback
	return "fallback:" + query
}

// IsInDegradedMode returns true if operating in degraded mode
func (dc *DegradedModeCache) IsInDegradedMode() bool {
	dc.mu.RLock()
	defer dc.mu.RUnlock()
	return dc.degraded
}

// GetStats returns combined statistics
func (dc *DegradedModeCache) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"degraded_mode": dc.IsInDegradedMode(),
		"fallback":      dc.fallback.GetStats(),
	}

	if dc.primary != nil {
		primaryStats := dc.primary.GetStats()
		stats["primary"] = map[string]interface{}{
			"total_hits":    primaryStats.TotalHits,
			"total_misses":  primaryStats.TotalMisses,
			"hit_rate":      primaryStats.HitRate,
			"total_entries": primaryStats.TotalEntries,
		}
	}

	return stats
}
