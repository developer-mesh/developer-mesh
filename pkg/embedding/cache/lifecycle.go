package cache

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Lifecycle manages cache lifecycle operations
type Lifecycle struct {
	cache      *SemanticCache
	warmers    []*CacheWarmer
	shutdownMu sync.Mutex
	shutdown   bool
	wg         sync.WaitGroup
	logger     observability.Logger
}

// NewLifecycle creates a new lifecycle manager for the cache
func NewLifecycle(cache *SemanticCache, logger observability.Logger) *Lifecycle {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.lifecycle")
	}

	return &Lifecycle{
		cache:  cache,
		logger: logger,
	}
}

// AddWarmer adds a cache warmer to be managed
func (l *Lifecycle) AddWarmer(warmer *CacheWarmer) {
	l.warmers = append(l.warmers, warmer)
}

// Start initializes cache operations
func (l *Lifecycle) Start(ctx context.Context) error {
	l.logger.Info("Starting semantic cache", map[string]interface{}{})

	// Start background operations
	l.wg.Add(1)
	go l.metricsExporter(ctx)

	// Note: Cache warmers don't have a Start method
	// They are typically used on-demand via Warm() or WarmWithPriority()
	// If automatic warming is needed, it should be implemented separately

	// Start eviction monitoring if configured
	if l.cache.config.MaxCacheSize > 0 {
		l.wg.Add(1)
		go l.evictionMonitor(ctx)
	}

	return nil
}

// Shutdown gracefully stops cache operations
func (l *Lifecycle) Shutdown(ctx context.Context) error {
	l.shutdownMu.Lock()
	if l.shutdown {
		l.shutdownMu.Unlock()
		return nil
	}
	l.shutdown = true
	l.shutdownMu.Unlock()

	l.logger.Info("Shutting down semantic cache", map[string]interface{}{})

	// Create shutdown context with timeout
	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Stop accepting new operations
	l.cache.mu.Lock()
	l.cache.shuttingDown = true
	l.cache.mu.Unlock()

	// Wait for ongoing operations
	done := make(chan struct{})
	go func() {
		l.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		l.logger.Info("All cache operations completed", map[string]interface{}{})
	case <-shutdownCtx.Done():
		l.logger.Warn("Shutdown timeout, some operations may be incomplete", map[string]interface{}{})
	}

	// Export final metrics
	l.exportMetrics()

	// Flush metrics if available
	if l.cache.metrics != nil {
		// Note: The observability.MetricsClient interface doesn't have a Flush method
		// We'll export final metrics instead
		l.logger.Info("Final metrics exported", map[string]interface{}{
			"total_hits":   l.cache.hitCount.Load(),
			"total_misses": l.cache.missCount.Load(),
		})
	}

	// Close Redis connection
	if l.cache.redis != nil {
		if err := l.cache.redis.Close(); err != nil {
			return fmt.Errorf("error closing Redis connection: %w", err)
		}
	}

	l.logger.Info("Semantic cache shutdown complete", map[string]interface{}{})
	return nil
}

// metricsExporter periodically exports cache metrics
func (l *Lifecycle) metricsExporter(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			l.exportMetrics()
		case <-ctx.Done():
			return
		}
	}
}

// evictionMonitor monitors cache size and triggers eviction when needed
func (l *Lifecycle) evictionMonitor(ctx context.Context) {
	defer l.wg.Done()

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			// Trigger eviction check
			go l.cache.evictIfNecessary(context.Background())
		case <-ctx.Done():
			return
		}
	}
}

func (l *Lifecycle) exportMetrics() {
	stats := l.cache.GetStats()

	if l.cache.metrics == nil {
		return
	}

	labels := map[string]string{
		"cache_type": "semantic",
	}

	// Export gauge metrics
	l.cache.metrics.RecordGauge("cache.entries", float64(stats.TotalEntries), labels)
	l.cache.metrics.RecordGauge("cache.hit_rate", stats.HitRate, labels)

	// Export counter metrics
	l.cache.metrics.RecordGauge("cache.total_hits", float64(stats.TotalHits), labels)
	l.cache.metrics.RecordGauge("cache.total_misses", float64(stats.TotalMisses), labels)

	// Log current stats
	l.logger.Info("Cache metrics exported", map[string]interface{}{
		"total_entries": stats.TotalEntries,
		"hit_rate":      stats.HitRate,
		"total_hits":    stats.TotalHits,
		"total_misses":  stats.TotalMisses,
	})
}

// IsShuttingDown returns true if the lifecycle manager is shutting down
func (l *Lifecycle) IsShuttingDown() bool {
	l.shutdownMu.Lock()
	defer l.shutdownMu.Unlock()
	return l.shutdown
}

// GetStats returns current lifecycle statistics
func (l *Lifecycle) GetStats() map[string]interface{} {
	stats := map[string]interface{}{
		"is_shutting_down": l.IsShuttingDown(),
		"warmer_count":     len(l.warmers),
		"cache_stats":      l.cache.GetStats(),
	}

	// Note: CacheWarmer doesn't have config fields
	// Just return warmer count for now
	stats["warmer_stats"] = "Warmers are used on-demand"

	return stats
}
