package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	lru "github.com/hashicorp/golang-lru/v2"
)

// CacheManager provides advanced caching capabilities for the REST client
type CacheManager struct {
	// Multi-level cache
	l1Cache *lru.Cache[string, *cacheEntry]
	l2Cache cache.Cache

	// Configuration
	config CacheConfig
	logger observability.Logger

	// Cache warming
	warmupQueue    chan warmupRequest
	warmupWorkers  int
	warmupInterval time.Duration

	// Request coalescing
	inflightRequests map[string]*coalescedRequest
	inflightMutex    sync.RWMutex

	// Metrics
	metrics *CacheMetrics

	// Invalidation tracking
	dependencies map[string][]string // Track cache key dependencies
	depMutex     sync.RWMutex

	// Shutdown management
	shutdown chan struct{}
	wg       sync.WaitGroup
}

// CacheConfig defines cache configuration
type CacheConfig struct {
	// L1 Cache (in-memory)
	L1MaxSize int           `json:"l1_max_size"`
	L1TTL     time.Duration `json:"l1_ttl"`

	// L2 Cache (Redis)
	L2Enabled bool          `json:"l2_enabled"`
	L2TTL     time.Duration `json:"l2_ttl"`

	// Cache warming
	WarmupEnabled  bool          `json:"warmup_enabled"`
	WarmupInterval time.Duration `json:"warmup_interval"`
	WarmupWorkers  int           `json:"warmup_workers"`

	// Request coalescing
	CoalescingEnabled bool `json:"coalescing_enabled"`

	// Compression
	CompressionEnabled bool `json:"compression_enabled"`
	CompressionLevel   int  `json:"compression_level"`
	CompressionMinSize int  `json:"compression_min_size"`
}

// cacheEntry represents a cached item
type cacheEntry struct {
	Data         []byte
	Compressed   bool
	CachedAt     time.Time
	TTL          time.Duration
	AccessCount  int64
	LastAccessed time.Time
	Dependencies []string // Keys this entry depends on
}

// warmupRequest represents a cache warmup request
type warmupRequest struct {
	Key      string
	TenantID string
	Priority int
	Callback func() (interface{}, error)
}

// coalescedRequest represents a coalesced request
type coalescedRequest struct {
	key     string
	result  interface{}
	err     error
	done    chan struct{}
	waiters int
}

// CacheMetrics tracks cache performance metrics
type CacheMetrics struct {
	mu sync.RWMutex

	// Hit/Miss metrics
	L1Hits   int64
	L1Misses int64
	L2Hits   int64
	L2Misses int64

	// Performance metrics
	AvgL1Latency     time.Duration
	AvgL2Latency     time.Duration
	CompressionRatio float64

	// Coalescing metrics
	CoalescedRequests int64
	CoalescingSaved   int64

	// Warmup metrics
	WarmupRequests  int64
	WarmupSuccesses int64
	WarmupFailures  int64

	// Invalidation metrics
	InvalidationCount    int64
	CascadeInvalidations int64
}

// DefaultCacheConfig returns default cache configuration
func DefaultCacheConfig() CacheConfig {
	return CacheConfig{
		L1MaxSize:          1000,
		L1TTL:              5 * time.Minute,
		L2Enabled:          true,
		L2TTL:              15 * time.Minute,
		WarmupEnabled:      true,
		WarmupInterval:     5 * time.Minute,
		WarmupWorkers:      2,
		CoalescingEnabled:  true,
		CompressionEnabled: true,
		CompressionLevel:   6,
		CompressionMinSize: 1024, // 1KB minimum for compression
	}
}

// NewCacheManager creates a new cache manager
func NewCacheManager(l2Cache cache.Cache, config CacheConfig, logger observability.Logger) (*CacheManager, error) {
	// Create L1 cache
	l1Cache, err := lru.New[string, *cacheEntry](config.L1MaxSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache: %w", err)
	}

	manager := &CacheManager{
		l1Cache:          l1Cache,
		l2Cache:          l2Cache,
		config:           config,
		logger:           logger,
		warmupQueue:      make(chan warmupRequest, 100),
		warmupWorkers:    config.WarmupWorkers,
		warmupInterval:   config.WarmupInterval,
		inflightRequests: make(map[string]*coalescedRequest),
		metrics:          &CacheMetrics{},
		dependencies:     make(map[string][]string),
		shutdown:         make(chan struct{}),
	}

	// Start warmup workers
	if config.WarmupEnabled {
		for i := 0; i < config.WarmupWorkers; i++ {
			manager.wg.Add(1)
			go manager.warmupWorker()
		}

		// Start periodic warmup
		manager.wg.Add(1)
		go manager.periodicWarmup()
	}

	return manager, nil
}

// Get retrieves a value from cache with multi-level lookup
func (m *CacheManager) Get(ctx context.Context, key string) (interface{}, bool, error) {
	startTime := time.Now()

	// Check L1 cache
	if entry := m.getL1(key); entry != nil {
		m.recordL1Hit(time.Since(startTime))

		// Decompress if needed
		data := entry.Data
		if entry.Compressed {
			data = m.decompress(data)
		}

		// Unmarshal
		var value interface{}
		if err := json.Unmarshal(data, &value); err != nil {
			return nil, false, fmt.Errorf("failed to unmarshal cached data: %w", err)
		}

		// Update access metrics
		entry.AccessCount++
		entry.LastAccessed = time.Now()

		return value, true, nil
	}

	m.recordL1Miss()

	// Check L2 cache if enabled
	if m.config.L2Enabled && m.l2Cache != nil {
		l2Start := time.Now()

		var data []byte
		err := m.l2Cache.Get(ctx, key, &data)
		if err == nil {
			m.recordL2Hit(time.Since(l2Start))

			// Decompress if needed
			if m.isCompressed(data) {
				data = m.decompress(data)
			}

			// Unmarshal
			var value interface{}
			if err := json.Unmarshal(data, &value); err != nil {
				return nil, false, fmt.Errorf("failed to unmarshal L2 data: %w", err)
			}

			// Promote to L1
			m.setL1(key, data, false, m.config.L1TTL)

			return value, true, nil
		}

		m.recordL2Miss()
	}

	return nil, false, nil
}

// Set stores a value in cache with multi-level write-through
func (m *CacheManager) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	// Marshal value
	data, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("failed to marshal value: %w", err)
	}

	// Compress if beneficial
	compressed := false
	if m.config.CompressionEnabled && len(data) >= m.config.CompressionMinSize {
		compressedData := m.compress(data)
		if len(compressedData) < len(data) {
			data = compressedData
			compressed = true
			m.updateCompressionRatio(len(compressedData), len(data))
		}
	}

	// Set in L1
	if ttl == 0 {
		ttl = m.config.L1TTL
	}
	m.setL1(key, data, compressed, ttl)

	// Set in L2 if enabled
	if m.config.L2Enabled && m.l2Cache != nil {
		l2TTL := ttl
		if l2TTL < m.config.L2TTL {
			l2TTL = m.config.L2TTL
		}
		if err := m.l2Cache.Set(ctx, key, data, l2TTL); err != nil {
			m.logger.Warn("Failed to set L2 cache", map[string]interface{}{
				"key":   key,
				"error": err.Error(),
			})
		}
	}

	return nil
}

// GetOrSet performs a cache-aside pattern with request coalescing
func (m *CacheManager) GetOrSet(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	// Try to get from cache first
	if value, found, err := m.Get(ctx, key); found {
		return value, err
	}

	// Check if coalescing is enabled
	if !m.config.CoalescingEnabled {
		// No coalescing, just load and cache
		value, err := loader()
		if err != nil {
			return nil, err
		}

		if err := m.Set(ctx, key, value, ttl); err != nil {
			m.logger.Warn("Failed to cache value", map[string]interface{}{
				"key":   key,
				"error": err.Error(),
			})
		}

		return value, nil
	}

	// Request coalescing
	return m.coalesceRequest(ctx, key, loader, ttl)
}

// coalesceRequest implements request coalescing for duplicate cache misses
func (m *CacheManager) coalesceRequest(ctx context.Context, key string, loader func() (interface{}, error), ttl time.Duration) (interface{}, error) {
	m.inflightMutex.Lock()

	// Check if request is already in flight
	if req, exists := m.inflightRequests[key]; exists {
		req.waiters++
		m.inflightMutex.Unlock()

		// Wait for the result
		select {
		case <-req.done:
			m.recordCoalescingSaved()
			return req.result, req.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	// Create new inflight request
	req := &coalescedRequest{
		key:     key,
		done:    make(chan struct{}),
		waiters: 0,
	}
	m.inflightRequests[key] = req
	m.inflightMutex.Unlock()

	// Execute the loader
	value, err := loader()

	// Store result
	req.result = value
	req.err = err

	// Cache the result if successful
	if err == nil {
		if cacheErr := m.Set(ctx, key, value, ttl); cacheErr != nil {
			m.logger.Warn("Failed to cache coalesced result", map[string]interface{}{
				"key":   key,
				"error": cacheErr.Error(),
			})
		}
	}

	// Clean up and notify waiters
	m.inflightMutex.Lock()
	delete(m.inflightRequests, key)
	m.inflightMutex.Unlock()

	close(req.done)

	if req.waiters > 0 {
		m.recordCoalescedRequest(req.waiters)
	}

	return value, err
}

// Invalidate removes a key and its dependencies from cache
func (m *CacheManager) Invalidate(ctx context.Context, key string) error {
	// Remove from L1
	m.l1Cache.Remove(key)

	// Remove from L2
	if m.config.L2Enabled && m.l2Cache != nil {
		if err := m.l2Cache.Delete(ctx, key); err != nil {
			m.logger.Warn("Failed to delete from L2 cache", map[string]interface{}{
				"key":   key,
				"error": err.Error(),
			})
		}
	}

	// Cascade invalidation to dependencies
	m.invalidateDependencies(ctx, key)

	m.recordInvalidation()

	return nil
}

// InvalidatePattern removes all keys matching a pattern
func (m *CacheManager) InvalidatePattern(ctx context.Context, pattern string) error {
	invalidated := 0

	// Invalidate L1 cache entries
	keys := m.l1Cache.Keys()
	for _, key := range keys {
		if m.matchesPattern(key, pattern) {
			m.l1Cache.Remove(key)
			invalidated++

			// Also invalidate from L2
			if m.config.L2Enabled && m.l2Cache != nil {
				_ = m.l2Cache.Delete(ctx, key)
			}
		}
	}

	m.logger.Info("Pattern invalidation completed", map[string]interface{}{
		"pattern":     pattern,
		"invalidated": invalidated,
	})

	return nil
}

// AddDependency adds a dependency between cache keys
func (m *CacheManager) AddDependency(parent, child string) {
	m.depMutex.Lock()
	defer m.depMutex.Unlock()

	if m.dependencies[parent] == nil {
		m.dependencies[parent] = []string{}
	}

	// Avoid duplicates
	for _, dep := range m.dependencies[parent] {
		if dep == child {
			return
		}
	}

	m.dependencies[parent] = append(m.dependencies[parent], child)
}

// invalidateDependencies cascades invalidation to dependent keys
func (m *CacheManager) invalidateDependencies(ctx context.Context, key string) {
	m.depMutex.RLock()
	deps := m.dependencies[key]
	m.depMutex.RUnlock()

	if len(deps) == 0 {
		return
	}

	for _, dep := range deps {
		m.l1Cache.Remove(dep)

		if m.config.L2Enabled && m.l2Cache != nil {
			_ = m.l2Cache.Delete(ctx, dep)
		}

		m.recordCascadeInvalidation()

		// Recursive invalidation
		m.invalidateDependencies(ctx, dep)
	}

	// Clean up dependency tracking
	m.depMutex.Lock()
	delete(m.dependencies, key)
	m.depMutex.Unlock()
}

// WarmCache queues a cache warming request
func (m *CacheManager) WarmCache(key, tenantID string, loader func() (interface{}, error), priority int) {
	if !m.config.WarmupEnabled {
		return
	}

	select {
	case m.warmupQueue <- warmupRequest{
		Key:      key,
		TenantID: tenantID,
		Priority: priority,
		Callback: loader,
	}:
		m.recordWarmupRequest()
	default:
		// Queue is full, skip warming
		m.logger.Debug("Warmup queue full, skipping", map[string]interface{}{
			"key": key,
		})
	}
}

// warmupWorker processes cache warming requests
func (m *CacheManager) warmupWorker() {
	defer m.wg.Done()

	for {
		select {
		case req := <-m.warmupQueue:
			m.processWarmupRequest(req)
		case <-m.shutdown:
			return
		}
	}
}

// processWarmupRequest handles a single warmup request
func (m *CacheManager) processWarmupRequest(req warmupRequest) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Check if already cached
	if _, found, _ := m.Get(ctx, req.Key); found {
		return
	}

	// Load the data
	value, err := req.Callback()
	if err != nil {
		m.recordWarmupFailure()
		m.logger.Debug("Warmup failed", map[string]interface{}{
			"key":   req.Key,
			"error": err.Error(),
		})
		return
	}

	// Cache the result
	if err := m.Set(ctx, req.Key, value, m.config.L1TTL); err != nil {
		m.logger.Warn("Failed to cache warmup result", map[string]interface{}{
			"key":   req.Key,
			"error": err.Error(),
		})
	}

	m.recordWarmupSuccess()
}

// periodicWarmup performs periodic cache warming
func (m *CacheManager) periodicWarmup() {
	defer m.wg.Done()

	ticker := time.NewTicker(m.warmupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			m.performPeriodicWarmup()
		case <-m.shutdown:
			return
		}
	}
}

// performPeriodicWarmup warms frequently accessed cache entries
func (m *CacheManager) performPeriodicWarmup() {
	// Get frequently accessed keys from L1 cache
	keys := m.l1Cache.Keys()

	for _, key := range keys {
		if entry, ok := m.l1Cache.Peek(key); ok {
			// Warm if accessed frequently and nearing expiration
			if entry.AccessCount > 5 && time.Until(entry.CachedAt.Add(entry.TTL)) < time.Minute {
				// Queue for warming with high priority
				m.logger.Debug("Queuing frequently accessed key for warmup", map[string]interface{}{
					"key":          key,
					"access_count": entry.AccessCount,
				})
			}
		}
	}
}

// Helper methods

func (m *CacheManager) getL1(key string) *cacheEntry {
	if entry, ok := m.l1Cache.Get(key); ok {
		// Check if expired
		if time.Since(entry.CachedAt) < entry.TTL {
			return entry
		}
		// Remove expired entry
		m.l1Cache.Remove(key)
	}
	return nil
}

func (m *CacheManager) setL1(key string, data []byte, compressed bool, ttl time.Duration) {
	entry := &cacheEntry{
		Data:         data,
		Compressed:   compressed,
		CachedAt:     time.Now(),
		TTL:          ttl,
		AccessCount:  0,
		LastAccessed: time.Now(),
	}
	m.l1Cache.Add(key, entry)
}

func (m *CacheManager) compress(data []byte) []byte {
	// Implementation would use gzip or similar
	// For now, return as-is
	return data
}

func (m *CacheManager) decompress(data []byte) []byte {
	// Implementation would use gzip or similar
	// For now, return as-is
	return data
}

func (m *CacheManager) isCompressed(data []byte) bool {
	// Check for compression magic bytes
	// For now, return false
	return false
}

func (m *CacheManager) matchesPattern(key, pattern string) bool {
	// Simple pattern matching
	// Could be enhanced with glob patterns
	return key == pattern || (pattern == "*" || key[:len(pattern)-1] == pattern[:len(pattern)-1])
}

// Metrics recording methods

func (m *CacheManager) recordL1Hit(latency time.Duration) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.L1Hits++
	m.metrics.AvgL1Latency = (m.metrics.AvgL1Latency + latency) / 2
}

func (m *CacheManager) recordL1Miss() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.L1Misses++
}

func (m *CacheManager) recordL2Hit(latency time.Duration) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.L2Hits++
	m.metrics.AvgL2Latency = (m.metrics.AvgL2Latency + latency) / 2
}

func (m *CacheManager) recordL2Miss() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.L2Misses++
}

func (m *CacheManager) recordCoalescedRequest(waiters int) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.CoalescedRequests++
	m.metrics.CoalescingSaved += int64(waiters)
}

func (m *CacheManager) recordCoalescingSaved() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.CoalescingSaved++
}

func (m *CacheManager) recordWarmupRequest() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.WarmupRequests++
}

func (m *CacheManager) recordWarmupSuccess() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.WarmupSuccesses++
}

func (m *CacheManager) recordWarmupFailure() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.WarmupFailures++
}

func (m *CacheManager) recordInvalidation() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.InvalidationCount++
}

func (m *CacheManager) recordCascadeInvalidation() {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	m.metrics.CascadeInvalidations++
}

func (m *CacheManager) updateCompressionRatio(compressed, original int) {
	m.metrics.mu.Lock()
	defer m.metrics.mu.Unlock()
	ratio := float64(compressed) / float64(original)
	if m.metrics.CompressionRatio == 0 {
		m.metrics.CompressionRatio = ratio
	} else {
		m.metrics.CompressionRatio = (m.metrics.CompressionRatio + ratio) / 2
	}
}

// GetMetrics returns cache performance metrics
func (m *CacheManager) GetMetrics() map[string]interface{} {
	m.metrics.mu.RLock()
	defer m.metrics.mu.RUnlock()

	l1HitRate := float64(0)
	if total := m.metrics.L1Hits + m.metrics.L1Misses; total > 0 {
		l1HitRate = float64(m.metrics.L1Hits) / float64(total)
	}

	l2HitRate := float64(0)
	if total := m.metrics.L2Hits + m.metrics.L2Misses; total > 0 {
		l2HitRate = float64(m.metrics.L2Hits) / float64(total)
	}

	warmupSuccessRate := float64(0)
	if m.metrics.WarmupRequests > 0 {
		warmupSuccessRate = float64(m.metrics.WarmupSuccesses) / float64(m.metrics.WarmupRequests)
	}

	return map[string]interface{}{
		"l1_hits":               m.metrics.L1Hits,
		"l1_misses":             m.metrics.L1Misses,
		"l1_hit_rate":           l1HitRate,
		"l1_avg_latency":        m.metrics.AvgL1Latency.String(),
		"l2_hits":               m.metrics.L2Hits,
		"l2_misses":             m.metrics.L2Misses,
		"l2_hit_rate":           l2HitRate,
		"l2_avg_latency":        m.metrics.AvgL2Latency.String(),
		"compression_ratio":     m.metrics.CompressionRatio,
		"coalesced_requests":    m.metrics.CoalescedRequests,
		"coalescing_saved":      m.metrics.CoalescingSaved,
		"warmup_requests":       m.metrics.WarmupRequests,
		"warmup_successes":      m.metrics.WarmupSuccesses,
		"warmup_success_rate":   warmupSuccessRate,
		"invalidations":         m.metrics.InvalidationCount,
		"cascade_invalidations": m.metrics.CascadeInvalidations,
	}
}

// Close shuts down the cache manager
func (m *CacheManager) Close() error {
	close(m.shutdown)
	m.wg.Wait()

	if m.l2Cache != nil {
		return m.l2Cache.Close()
	}

	return nil
}
