package eviction

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CacheInterface defines the interface for cache operations
type CacheInterface interface {
	Delete(ctx context.Context, key string) error
	BeginTx(ctx context.Context) (Transaction, error)
}

// Transaction defines cache transaction operations
type Transaction interface {
	Delete(key string) error
	Commit() error
	Rollback() error
}

// LRUEvictor implements LRU eviction with tenant awareness
type LRUEvictor struct {
	cache       CacheInterface
	vectorStore VectorStore
	config      Config
	logger      observability.Logger
	metrics     observability.MetricsClient
}

// Config defines eviction configuration
type Config struct {
	MaxEntriesPerTenant int
	MaxGlobalEntries    int
	CheckInterval       time.Duration
	BatchSize           int
	StaleThreshold      time.Duration // Entries not accessed for this duration are candidates
}

// DefaultConfig returns default eviction configuration
func DefaultConfig() Config {
	return Config{
		MaxEntriesPerTenant: 1000,
		MaxGlobalEntries:    10000,
		CheckInterval:       5 * time.Minute,
		BatchSize:           100,
		StaleThreshold:      7 * 24 * time.Hour, // 7 days
	}
}

// NewLRUEvictor creates a new LRU evictor
func NewLRUEvictor(
	cache CacheInterface,
	vectorStore VectorStore,
	config Config,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *LRUEvictor {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.eviction.lru")
	}
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	return &LRUEvictor{
		cache:       cache,
		vectorStore: vectorStore,
		config:      config,
		logger:      logger,
		metrics:     metrics,
	}
}

// EvictTenantEntries evicts LRU entries for a specific tenant
func (e *LRUEvictor) EvictTenantEntries(ctx context.Context, tenantID uuid.UUID) error {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "lru_evictor.evict_tenant_entries")
	defer span.End()

	// Get tenant's cache stats
	stats, err := e.vectorStore.GetTenantCacheStats(ctx, tenantID)
	if err != nil {
		return fmt.Errorf("failed to get tenant stats: %w", err)
	}

	if stats.EntryCount <= e.config.MaxEntriesPerTenant {
		return nil // No eviction needed
	}

	// Calculate entries to evict (10% buffer)
	toEvict := stats.EntryCount - int(float64(e.config.MaxEntriesPerTenant)*0.9)

	e.logger.Info("Evicting LRU entries for tenant", map[string]interface{}{
		"tenant_id":     tenantID.String(),
		"current_count": stats.EntryCount,
		"max_allowed":   e.config.MaxEntriesPerTenant,
		"to_evict":      toEvict,
	})

	// Get LRU entries from database
	entries, err := e.vectorStore.GetLRUEntries(ctx, tenantID, toEvict)
	if err != nil {
		return fmt.Errorf("failed to get LRU entries: %w", err)
	}

	evicted := 0
	// Batch evict
	for i := 0; i < len(entries); i += e.config.BatchSize {
		batch := entries[i:min(i+e.config.BatchSize, len(entries))]

		if err := e.evictBatch(ctx, tenantID, batch); err != nil {
			e.logger.Error("Failed to evict batch", map[string]interface{}{
				"error":      err.Error(),
				"tenant_id":  tenantID.String(),
				"batch_size": len(batch),
			})
			// Continue with next batch even if one fails
			continue
		}
		evicted += len(batch)
	}

	e.metrics.IncrementCounterWithLabels("cache.evictions", float64(evicted), map[string]string{
		"tenant_id": tenantID.String(),
		"strategy":  "lru",
		"reason":    "size_limit",
	})

	e.logger.Info("Completed LRU eviction for tenant", map[string]interface{}{
		"tenant_id": tenantID.String(),
		"evicted":   evicted,
	})

	return nil
}

func (e *LRUEvictor) evictBatch(ctx context.Context, tenantID uuid.UUID, entries []LRUEntry) error {
	// For each entry, delete from both Redis and database
	for _, entry := range entries {
		// Delete from Redis cache
		if err := e.cache.Delete(ctx, entry.CacheKey); err != nil {
			e.logger.Warn("Failed to delete from Redis", map[string]interface{}{
				"error":     err.Error(),
				"cache_key": entry.CacheKey,
			})
			// Continue even if Redis delete fails
		}

		// Delete from database
		if err := e.vectorStore.DeleteCacheEntry(ctx, tenantID, entry.CacheKey); err != nil {
			return fmt.Errorf("failed to delete from database: %w", err)
		}
	}

	return nil
}

// Run starts the background eviction process
func (e *LRUEvictor) Run(ctx context.Context) {
	e.logger.Info("Starting LRU eviction background process", map[string]interface{}{
		"check_interval": e.config.CheckInterval,
		"batch_size":     e.config.BatchSize,
	})

	ticker := time.NewTicker(e.config.CheckInterval)
	defer ticker.Stop()

	// Run initial eviction cycle
	e.runEvictionCycle(ctx)

	for {
		select {
		case <-ticker.C:
			e.runEvictionCycle(ctx)
		case <-ctx.Done():
			e.logger.Info("Stopping LRU eviction process", map[string]interface{}{})
			return
		}
	}
}

func (e *LRUEvictor) runEvictionCycle(ctx context.Context) {
	// Use recovery middleware to handle panics
	RecoverMiddleware(e.logger, "eviction_cycle")(func() {
		startTime := time.Now()

		// First, cleanup globally stale entries
		if e.config.StaleThreshold > 0 {
			deleted, err := e.vectorStore.CleanupStaleEntries(ctx, e.config.StaleThreshold)
			if err != nil {
				e.logger.Error("Failed to cleanup stale entries", map[string]interface{}{
					"error": err.Error(),
				})
			} else if deleted > 0 {
				e.metrics.IncrementCounterWithLabels("cache.evictions", float64(deleted), map[string]string{
					"strategy": "lru",
					"reason":   "stale",
				})
			}
		}

		// Get all tenants with cache entries
		tenants, err := e.vectorStore.GetTenantsWithCache(ctx)
		if err != nil {
			e.logger.Error("Failed to get tenants", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}

		tenantsProcessed := 0
		totalEvicted := 0

		// Evict for each tenant
		for _, tenantID := range tenants {
			// Check if context is cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			if err := e.EvictTenantEntries(ctx, tenantID); err != nil {
				e.logger.Error("Failed to evict for tenant", map[string]interface{}{
					"error":     err.Error(),
					"tenant_id": tenantID.String(),
				})
			} else {
				tenantsProcessed++
			}
		}

		// Check global limit
		globalStats, err := e.vectorStore.GetGlobalCacheStats(ctx)
		if err != nil {
			e.logger.Error("Failed to get global stats", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			totalEntries, _ := globalStats["total_entries"].(int)
			if totalEntries > e.config.MaxGlobalEntries {
				e.evictGlobalLRU(ctx, totalEntries-e.config.MaxGlobalEntries)
			}
		}

		duration := time.Since(startTime).Seconds()
		e.metrics.RecordHistogram("cache.eviction.cycle_duration", duration, map[string]string{
			"strategy": "lru",
		})

		e.logger.Info("Completed eviction cycle", map[string]interface{}{
			"duration_seconds":  duration,
			"tenants_processed": tenantsProcessed,
			"total_evicted":     totalEvicted,
		})
	})
}

// evictGlobalLRU evicts entries globally when total limit is exceeded
func (e *LRUEvictor) evictGlobalLRU(ctx context.Context, toEvict int) {
	e.logger.Warn("Global cache limit exceeded, evicting entries", map[string]interface{}{
		"to_evict": toEvict,
		"limit":    e.config.MaxGlobalEntries,
	})

	// For global eviction, we need to get LRU entries across all tenants
	// This would require a different query that doesn't filter by tenant
	// For now, log the warning

	e.metrics.IncrementCounterWithLabels("cache.evictions.global", float64(toEvict), map[string]string{
		"reason": "global_limit",
	})
}

// Stop gracefully stops the eviction process
func (e *LRUEvictor) Stop() {
	e.logger.Info("Stopping LRU evictor", map[string]interface{}{})
}

// GetConfig returns the current eviction configuration
func (e *LRUEvictor) GetConfig() Config {
	return e.config
}

// UpdateConfig updates the eviction configuration
func (e *LRUEvictor) UpdateConfig(config Config) {
	e.config = config
	e.logger.Info("Updated eviction configuration", map[string]interface{}{
		"max_entries_per_tenant": config.MaxEntriesPerTenant,
		"max_global_entries":     config.MaxGlobalEntries,
		"check_interval":         config.CheckInterval,
		"batch_size":             config.BatchSize,
		"stale_threshold":        config.StaleThreshold,
	})
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
