package lru

import (
	"context"
	"fmt"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// LRUStats represents LRU manager statistics
type LRUStats struct {
	TotalKeys        int64         `json:"total_keys"`
	TotalBytes       int64         `json:"total_bytes"`
	EvictionCount    int64         `json:"eviction_count"`
	LastEviction     time.Time     `json:"last_eviction"`
	AverageAccessAge time.Duration `json:"average_access_age"`
}

// Manager handles LRU eviction for tenant cache entries
type Manager struct {
	redis   RedisClient
	config  *Config
	logger  observability.Logger
	metrics observability.MetricsClient
	prefix  string

	// Async tracking
	tracker *AsyncTracker

	// Policies
	policies map[string]EvictionPolicy
}

// Config defines LRU manager configuration
type Config struct {
	// Global limits
	MaxGlobalEntries int
	MaxGlobalBytes   int64

	// Per-tenant limits (defaults)
	MaxTenantEntries int
	MaxTenantBytes   int64

	// Eviction settings
	EvictionBatchSize int
	EvictionInterval  time.Duration

	// Tracking settings
	TrackingBatchSize  int
	FlushInterval      time.Duration
	TrackingBufferSize int // Channel buffer size for async tracking
}

// DefaultConfig returns default LRU configuration
func DefaultConfig() *Config {
	return &Config{
		MaxGlobalEntries:   1000000,
		MaxGlobalBytes:     10 * 1024 * 1024 * 1024, // 10GB
		MaxTenantEntries:   10000,
		MaxTenantBytes:     100 * 1024 * 1024, // 100MB
		EvictionBatchSize:  100,
		EvictionInterval:   5 * time.Minute,
		TrackingBatchSize:  1000,
		FlushInterval:      10 * time.Second,
		TrackingBufferSize: 1000, // Reduced from hardcoded 10000
	}
}

// NewManager creates a new LRU manager
func NewManager(redis RedisClient, config *Config, prefix string, logger observability.Logger, metrics observability.MetricsClient) *Manager {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.lru")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	m := &Manager{
		redis:    redis,
		config:   config,
		logger:   logger,
		metrics:  metrics,
		prefix:   prefix,
		policies: make(map[string]EvictionPolicy),
	}

	// Create async tracker
	m.tracker = NewAsyncTracker(redis, config, logger, metrics)

	// Register default policies
	m.policies["size_based"] = &SizeBasedPolicy{
		maxEntries: config.MaxTenantEntries,
		maxBytes:   config.MaxTenantBytes,
	}

	m.policies["adaptive"] = &AdaptivePolicy{
		base:       m.policies["size_based"],
		minHitRate: 0.5,
		config:     config,
	}

	return m
}

// TrackAccess records cache access for LRU tracking
func (m *Manager) TrackAccess(tenantID uuid.UUID, key string) {
	m.tracker.Track(tenantID, key)
}

// GetAccessScore returns the access score for a cache key
func (m *Manager) GetAccessScore(ctx context.Context, tenantID uuid.UUID, key string) (float64, error) {
	scoreKey := m.getScoreKey(tenantID)

	score, err := m.redis.GetClient().ZScore(ctx, scoreKey, key).Result()
	if err != nil {
		if err == redis.Nil {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get access score: %w", err)
	}

	return score, nil
}

// GetLRUKeys returns the least recently used keys for a tenant
func (m *Manager) GetLRUKeys(ctx context.Context, tenantID uuid.UUID, limit int) ([]string, error) {
	scoreKey := m.getScoreKey(tenantID)

	// Get keys with lowest scores (oldest access times)
	results, err := m.redis.GetClient().ZRangeWithScores(ctx, scoreKey, 0, int64(limit-1)).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get LRU keys: %w", err)
	}

	keys := make([]string, 0, len(results))
	for _, result := range results {
		keys = append(keys, result.Member.(string))
	}

	return keys, nil
}

// GetStats returns global LRU manager statistics
func (m *Manager) GetStats() map[string]interface{} {
	// Get tracker stats
	trackerStats := m.tracker.GetStats()

	return map[string]interface{}{
		"config": map[string]interface{}{
			"max_global_entries": m.config.MaxGlobalEntries,
			"max_global_bytes":   m.config.MaxGlobalBytes,
			"eviction_interval":  m.config.EvictionInterval.String(),
		},
		"tracker":  trackerStats,
		"policies": len(m.policies),
	}
}

// GetTenantStats returns LRU statistics for a specific tenant
func (m *Manager) GetTenantStats(ctx context.Context, tenantID uuid.UUID) (*LRUStats, error) {
	scoreKey := m.getScoreKey(tenantID)

	// Get total keys
	totalKeys, err := m.redis.GetClient().ZCard(ctx, scoreKey).Result()
	if err != nil {
		return nil, fmt.Errorf("failed to get key count: %w", err)
	}

	// Get average access age
	now := time.Now()
	oldestScore, err := m.redis.GetClient().ZRangeWithScores(ctx, scoreKey, 0, 0).Result()
	var avgAge time.Duration
	if err == nil && len(oldestScore) > 0 {
		oldestTime := time.Unix(0, int64(oldestScore[0].Score))
		avgAge = now.Sub(oldestTime)
	}

	return &LRUStats{
		TotalKeys:        totalKeys,
		TotalBytes:       totalKeys * 1024, // Estimate
		EvictionCount:    0,                // TODO: Track this
		LastEviction:     time.Time{},      // TODO: Track this
		AverageAccessAge: avgAge,
	}, nil
}

// Start starts the LRU manager background processes
func (m *Manager) Start(ctx context.Context) error {
	// Start async tracker
	go m.tracker.Run(ctx)

	// Start eviction process
	go m.Run(ctx, nil)

	return nil
}

// EvictForTenant performs LRU eviction for a specific tenant
func (m *Manager) EvictForTenant(ctx context.Context, tenantID uuid.UUID, targetBytes int64) error {
	ctx, span := observability.StartSpan(ctx, "lru.evict_for_tenant")
	defer span.End()

	pattern := fmt.Sprintf("%s:{%s}:q:*", m.prefix, tenantID.String())
	scoreKey := m.getScoreKey(tenantID)

	// Convert target bytes to entry count (estimate 1KB per entry)
	targetCount := int(targetBytes / 1024)
	if targetCount <= 0 {
		targetCount = 1
	}

	// Get current count
	currentCount, err := m.getKeyCount(ctx, pattern)
	if err != nil {
		return fmt.Errorf("failed to get key count: %w", err)
	}

	if currentCount <= targetCount {
		return nil // No eviction needed
	}

	toEvict := currentCount - targetCount

	// Get LRU candidates from sorted set
	candidates, err := m.getLRUCandidates(ctx, scoreKey, toEvict)
	if err != nil {
		return fmt.Errorf("failed to get LRU candidates: %w", err)
	}

	// Batch delete with circuit breaker
	evicted := 0
	for i := 0; i < len(candidates); i += m.config.EvictionBatchSize {
		batch := candidates[i:min(i+m.config.EvictionBatchSize, len(candidates))]

		_, err := m.redis.Execute(ctx, func() (interface{}, error) {
			pipe := m.redis.GetClient().Pipeline()

			// Delete cache entries
			for _, key := range batch {
				pipe.Del(ctx, key)
			}

			// Remove from score set
			members := make([]interface{}, len(batch))
			for i, key := range batch {
				members[i] = key
			}
			pipe.ZRem(ctx, scoreKey, members...)

			_, err := pipe.Exec(ctx)
			return nil, err
		})

		if err != nil {
			m.logger.Error("Failed to evict batch", map[string]interface{}{
				"error":      err.Error(),
				"tenant_id":  tenantID.String(),
				"batch_size": len(batch),
			})
			// Continue with next batch
		} else {
			evicted += len(batch)
		}

		m.metrics.IncrementCounterWithLabels("cache.evicted", float64(len(batch)), map[string]string{
			"tenant_id": tenantID.String(),
		})
	}

	m.logger.Info("Completed eviction", map[string]interface{}{
		"tenant_id": tenantID.String(),
		"evicted":   evicted,
		"target":    toEvict,
	})

	return nil
}

// EvictTenantEntries performs eviction based on the configured policy
func (m *Manager) EvictTenantEntries(ctx context.Context, tenantID uuid.UUID, vectorStore eviction.VectorStore) error {
	// Get tenant stats
	stats, err := vectorStore.GetTenantCacheStats(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant stats: %w", err)
	}

	// Calculate total bytes from Redis
	totalBytes, err := m.calculateTenantBytes(ctx, tenantID)
	if err != nil {
		m.logger.Warn("Failed to calculate tenant bytes", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
		totalBytes = 0
	}

	// Calculate hit rate from metrics
	hitRate := m.calculateHitRate(ctx, tenantID)

	// Convert to policy stats
	policyStats := TenantStats{
		EntryCount:   stats.EntryCount,
		TotalBytes:   totalBytes,
		LastEviction: time.Now(),
		HitRate:      hitRate,
	}

	// Check if eviction is needed
	policy := m.policies["adaptive"]
	if !policy.ShouldEvict(ctx, tenantID, policyStats) {
		return nil
	}

	// Get target count from policy
	targetCount := policy.GetEvictionTarget(ctx, tenantID, policyStats)

	// Perform eviction (convert count to bytes estimate)
	return m.EvictForTenant(ctx, tenantID, int64(targetCount)*1024)
}

// EvictGlobal performs global LRU eviction across all tenants
func (m *Manager) EvictGlobal(ctx context.Context, targetBytes int64) error {
	ctx, span := observability.StartSpan(ctx, "lru.evict_global")
	defer span.End()

	// Convert target bytes to entry count (estimate 1KB per entry)
	targetCount := int(targetBytes / 1024)
	if targetCount <= 0 {
		targetCount = 1
	}

	// Get all tenant IDs
	tenantPattern := fmt.Sprintf("%s:{*}:q:*", m.prefix)
	currentCount, err := m.getKeyCount(ctx, tenantPattern)
	if err != nil {
		return fmt.Errorf("failed to get global key count: %w", err)
	}

	if currentCount <= targetCount {
		return nil // No eviction needed
	}

	toEvict := currentCount - targetCount

	// Get global LRU candidates
	globalScoreKey := fmt.Sprintf("%s:global:lru_scores", m.prefix)
	candidates, err := m.getLRUCandidates(ctx, globalScoreKey, toEvict)
	if err != nil {
		return fmt.Errorf("failed to get global LRU candidates: %w", err)
	}

	// Batch delete
	evicted := 0
	for i := 0; i < len(candidates); i += m.config.EvictionBatchSize {
		batch := candidates[i:min(i+m.config.EvictionBatchSize, len(candidates))]

		_, err := m.redis.Execute(ctx, func() (interface{}, error) {
			pipe := m.redis.GetClient().Pipeline()

			// Delete cache entries
			for _, key := range batch {
				pipe.Del(ctx, key)
			}

			// Remove from global score set
			pipe.ZRem(ctx, globalScoreKey, batch)

			_, execErr := pipe.Exec(ctx)
			return nil, execErr
		})

		if err != nil {
			m.logger.Error("Failed to evict batch", map[string]interface{}{
				"error":      err.Error(),
				"batch_size": len(batch),
			})
			continue
		}

		evicted += len(batch)
	}

	// Record metrics
	if m.metrics != nil {
		m.metrics.IncrementCounterWithLabels("cache.eviction.global", float64(evicted), map[string]string{
			"type": "lru",
		})
	}

	m.logger.Info("Global eviction completed", map[string]interface{}{
		"evicted":      evicted,
		"target_count": targetCount,
	})

	return nil
}

// Run starts the background eviction process
func (m *Manager) Run(ctx context.Context, vectorStore eviction.VectorStore) {
	ticker := time.NewTicker(m.config.EvictionInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			m.logger.Info("LRU manager stopping", nil)
			m.tracker.Stop()
			return
		case <-ticker.C:
			m.runEvictionCycle(ctx, vectorStore)
		}
	}
}

// Stop gracefully stops the LRU manager
func (m *Manager) Stop(ctx context.Context) error {
	m.tracker.Stop()
	return nil
}

func (m *Manager) runEvictionCycle(ctx context.Context, vectorStore eviction.VectorStore) {
	startTime := time.Now()

	// Get all tenants with cache entries
	tenants, err := vectorStore.GetTenantsWithCache(ctx)
	if err != nil {
		m.logger.Error("Failed to get tenants", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	evictedTotal := 0
	for _, tenantID := range tenants {
		if err := m.EvictTenantEntries(ctx, tenantID, vectorStore); err != nil {
			m.logger.Error("Failed to evict for tenant", map[string]interface{}{
				"error":     err.Error(),
				"tenant_id": tenantID.String(),
			})
		}
	}

	m.metrics.RecordHistogram("lru.eviction_cycle.duration", time.Since(startTime).Seconds(), map[string]string{
		"evicted": fmt.Sprintf("%d", evictedTotal),
	})
}

// getKeyCount uses Lua script for accurate counting
func (m *Manager) getKeyCount(ctx context.Context, pattern string) (int, error) {
	const countScript = `
		local count = 0
		local cursor = "0"
		repeat
			local result = redis.call("SCAN", cursor, "MATCH", ARGV[1], "COUNT", 100)
			cursor = result[1]
			count = count + #result[2]
		until cursor == "0"
		return count
	`

	result, err := m.redis.Execute(ctx, func() (interface{}, error) {
		return m.redis.GetClient().Eval(ctx, countScript, []string{}, pattern).Result()
	})

	if err != nil {
		return 0, err
	}

	count, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}

	return int(count), nil
}

// getLRUCandidates retrieves least recently used keys
func (m *Manager) getLRUCandidates(ctx context.Context, scoreKey string, count int) ([]string, error) {
	result, err := m.redis.Execute(ctx, func() (interface{}, error) {
		// Get oldest entries (lowest scores)
		return m.redis.GetClient().ZRange(ctx, scoreKey, 0, int64(count-1)).Result()
	})

	if err != nil {
		return nil, err
	}

	candidates, ok := result.([]string)
	if !ok {
		return nil, fmt.Errorf("unexpected result type: %T", result)
	}

	return candidates, nil
}

func (m *Manager) getScoreKey(tenantID uuid.UUID) string {
	return fmt.Sprintf("%s:lru:{%s}", m.prefix, tenantID.String())
}

// RegisterPolicy registers a custom eviction policy
func (m *Manager) RegisterPolicy(name string, policy EvictionPolicy) {
	m.policies[name] = policy
}

// GetPolicy returns the eviction policy by name
func (m *Manager) GetPolicy(name string) (EvictionPolicy, bool) {
	policy, ok := m.policies[name]
	return policy, ok
}

// GetConfig returns the current configuration (for testing)
func (m *Manager) GetConfig() *Config {
	return m.config
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// calculateTenantBytes calculates the total memory usage for a tenant's cache entries
func (m *Manager) calculateTenantBytes(ctx context.Context, tenantID uuid.UUID) (int64, error) {
	pattern := fmt.Sprintf("%s:{%s}:q:*", m.prefix, tenantID.String())

	// Use Lua script for efficiency
	script := `
		local total = 0
		local cursor = "0"
		repeat
			local result = redis.call("SCAN", cursor, "MATCH", ARGV[1], "COUNT", 100)
			cursor = result[1]
			for _, key in ipairs(result[2]) do
				local size = redis.call("MEMORY", "USAGE", key)
				if size then
					total = total + size
				end
			end
		until cursor == "0"
		return total
	`

	result, err := m.redis.Execute(ctx, func() (interface{}, error) {
		return m.redis.GetClient().Eval(ctx, script, []string{}, pattern).Result()
	})

	if err != nil {
		return 0, err
	}

	bytes, ok := result.(int64)
	if !ok {
		return 0, fmt.Errorf("unexpected result type: %T", result)
	}

	return bytes, nil
}

// calculateHitRate calculates the cache hit rate for a tenant
func (m *Manager) calculateHitRate(ctx context.Context, tenantID uuid.UUID) float64 {
	// This should integrate with the metrics system
	// For now, return a default until metrics integration is complete
	if m.metrics != nil {
		// Get hit/miss counts from Prometheus metrics
		// Calculate rate = hits / (hits + misses)
		// This requires access to the metrics backend
		// For now, we'll log that this needs implementation
		m.logger.Debug("Hit rate calculation not fully implemented", map[string]interface{}{
			"tenant_id": tenantID.String(),
			"note":      "Requires metrics backend integration",
		})
	}
	return 0.5 // Default 50% hit rate
}
