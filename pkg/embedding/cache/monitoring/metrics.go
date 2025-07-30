package monitoring

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/cache/eviction"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CacheMetricsExporter exports cache metrics to Prometheus
type CacheMetricsExporter struct {
	metrics     observability.MetricsClient
	vectorStore eviction.VectorStore
	interval    time.Duration
	stopCh      chan struct{}
}

// NewCacheMetricsExporter creates a new cache metrics exporter
func NewCacheMetricsExporter(
	metrics observability.MetricsClient,
	vectorStore eviction.VectorStore,
	interval time.Duration,
) *CacheMetricsExporter {
	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}
	if interval == 0 {
		interval = 30 * time.Second
	}

	return &CacheMetricsExporter{
		metrics:     metrics,
		vectorStore: vectorStore,
		interval:    interval,
		stopCh:      make(chan struct{}),
	}
}

// GetMetrics returns the metrics client
func (e *CacheMetricsExporter) GetMetrics() observability.MetricsClient {
	return e.metrics
}

// Start begins exporting metrics
func (e *CacheMetricsExporter) Start(ctx context.Context) {
	ticker := time.NewTicker(e.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-e.stopCh:
			return
		case <-ticker.C:
			e.exportMetrics(ctx)
		}
	}
}

// Stop stops the metrics exporter
func (e *CacheMetricsExporter) Stop() {
	close(e.stopCh)
}

// exportMetrics exports current cache metrics
func (e *CacheMetricsExporter) exportMetrics(ctx context.Context) {
	// Export global cache stats
	if e.vectorStore != nil {
		globalStats, err := e.vectorStore.GetGlobalCacheStats(ctx)
		if err == nil {
			e.exportGlobalStats(globalStats)
		}

		// Export per-tenant stats
		tenants, err := e.vectorStore.GetTenantsWithCache(ctx)
		if err == nil {
			for _, tenantID := range tenants {
				e.exportTenantStats(ctx, tenantID)
			}
		}
	}
}

// exportGlobalStats exports global cache statistics
func (e *CacheMetricsExporter) exportGlobalStats(stats map[string]interface{}) {
	if tenantCount, ok := stats["tenant_count"].(int); ok {
		e.metrics.RecordGauge("cache.global.tenant_count", float64(tenantCount), nil)
	}

	if totalEntries, ok := stats["total_entries"].(int); ok {
		e.metrics.RecordGauge("cache.global.total_entries", float64(totalEntries), nil)
	}

	if totalHits, ok := stats["total_hits"].(int64); ok {
		e.metrics.RecordGauge("cache.global.total_hits", float64(totalHits), nil)
	}

	if avgHitsPerEntry, ok := stats["avg_hits_per_entry"].(float64); ok {
		e.metrics.RecordGauge("cache.global.avg_hits_per_entry", avgHitsPerEntry, nil)
	}
}

// exportTenantStats exports per-tenant cache statistics
func (e *CacheMetricsExporter) exportTenantStats(ctx context.Context, tenantID uuid.UUID) {
	stats, err := e.vectorStore.GetTenantCacheStats(ctx, tenantID)
	if err != nil {
		return
	}

	labels := map[string]string{
		"tenant_id": tenantID.String(),
	}

	e.metrics.RecordGauge("cache.entries", float64(stats.EntryCount), labels)
	e.metrics.RecordGauge("cache.hits", float64(stats.TotalHits), labels)

	// Calculate hit rate if we have the data
	if stats.EntryCount > 0 {
		// This is a simplified calculation - in production you'd track attempts
		hitRate := float64(stats.TotalHits) / float64(stats.EntryCount)
		e.metrics.RecordGauge("cache.hit_rate", hitRate, labels)
	}
}

// MetricsMiddleware records cache operation metrics
func MetricsMiddleware(metrics observability.MetricsClient) func(next func() error) error {
	return func(next func() error) error {
		startTime := time.Now()
		err := next()
		duration := time.Since(startTime).Seconds()

		status := "success"
		if err != nil {
			status = "error"
		}

		metrics.RecordHistogram("cache.operation.duration", duration, map[string]string{
			"status": status,
		})

		return err
	}
}

// TrackCacheOperation tracks a cache operation with metrics
func TrackCacheOperation(
	ctx context.Context,
	metrics observability.MetricsClient,
	operation string,
	tenantID uuid.UUID,
	fn func() error,
) error {
	startTime := time.Now()
	err := fn()
	duration := time.Since(startTime).Seconds()

	labels := map[string]string{
		"operation": operation,
		"tenant_id": tenantID.String(),
		"status":    "success",
	}

	if err != nil {
		labels["status"] = "error"
		labels["error_type"] = getErrorType(err)
	}

	metrics.RecordHistogram("cache.operation.duration", duration, labels)
	metrics.IncrementCounterWithLabels("cache.operation.count", 1, labels)

	return err
}

// getErrorType categorizes errors for metrics
func getErrorType(err error) string {
	if err == nil {
		return "none"
	}

	// Add specific error type detection
	switch err.Error() {
	case "cache miss":
		return "miss"
	case "rate limit exceeded":
		return "rate_limit"
	case "feature disabled for tenant":
		return "feature_disabled"
	case "no tenant ID in context":
		return "no_tenant"
	default:
		return "other"
	}
}
