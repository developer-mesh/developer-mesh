package lru

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	
	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Manager handles LRU eviction for tenant cache entries
type Manager struct {
	redis      RedisClient
	config     *Config
	logger     observability.Logger
	metrics    observability.MetricsClient
	prefix     string
	
	// Async tracking
	tracker    *AsyncTracker
	
	// Policies
	policies   map[string]EvictionPolicy
}

// Config defines LRU manager configuration
type Config struct {
	// Global limits
	MaxGlobalEntries  int
	MaxGlobalBytes    int64
	
	// Per-tenant limits (defaults)
	MaxTenantEntries  int
	MaxTenantBytes    int64
	
	// Eviction settings
	EvictionBatchSize int
	EvictionInterval  time.Duration
	
	// Tracking settings
	TrackingBatchSize int
	FlushInterval     time.Duration
}

// DefaultConfig returns default LRU configuration
func DefaultConfig() *Config {
	return &Config{
		MaxGlobalEntries:  1000000,
		MaxGlobalBytes:    10 * 1024 * 1024 * 1024, // 10GB
		MaxTenantEntries:  10000,
		MaxTenantBytes:    100 * 1024 * 1024, // 100MB
		EvictionBatchSize: 100,
		EvictionInterval:  5 * time.Minute,
		TrackingBatchSize: 1000,
		FlushInterval:     10 * time.Second,
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
		base: m.policies["size_based"],
		minHitRate: 0.5,
		config: config,
	}

	return m
}

// TrackAccess records cache access for LRU tracking
func (m *Manager) TrackAccess(ctx context.Context, tenantID uuid.UUID, key string) {
	m.tracker.Track(tenantID, key)
}

// EvictForTenant performs LRU eviction for a specific tenant
func (m *Manager) EvictForTenant(ctx context.Context, tenantID uuid.UUID, targetCount int) error {
	ctx, span := observability.StartSpan(ctx, "lru.evict_for_tenant")
	defer span.End()

	pattern := fmt.Sprintf("%s:{%s}:q:*", m.prefix, tenantID.String())
	scoreKey := m.getScoreKey(tenantID)
	
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
		
		err := m.redis.Execute(ctx, func() (interface{}, error) {
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
				"error":     err.Error(),
				"tenant_id": tenantID.String(),
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
		"evicted": evicted,
		"target": toEvict,
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

	// Convert to policy stats
	policyStats := TenantStats{
		EntryCount:   stats.EntryCount,
		TotalBytes:   0, // Would need to calculate from Redis
		LastEviction: time.Now(),
		HitRate:      0, // Would need to calculate from metrics
	}

	// Check if eviction is needed
	policy := m.policies["adaptive"]
	if !policy.ShouldEvict(ctx, tenantID, policyStats) {
		return nil
	}

	// Get target count from policy
	targetCount := policy.GetEvictionTarget(ctx, tenantID, policyStats)
	
	// Perform eviction
	return m.EvictForTenant(ctx, tenantID, targetCount)
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
func (m *Manager) Stop() {
	m.tracker.Stop()
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}