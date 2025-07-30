package cache

import (
	"context"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// ObservabilityManager manages metrics, logging, and tracing for the cache
type ObservabilityManager struct {
	logger        observability.Logger
	metrics       *MetricsCollector
	globalMetrics *GlobalMetrics
	tracer        trace.Tracer
}

// NewObservabilityManager creates a new observability manager
func NewObservabilityManager(tenantID string, logger observability.Logger) *ObservabilityManager {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.observability")
	}

	return &ObservabilityManager{
		logger:        logger,
		metrics:       NewMetricsCollector(tenantID),
		globalMetrics: &GlobalMetrics{},
		tracer:        observability.GetTracer(),
	}
}

// TrackCacheOperation tracks a cache operation with metrics and tracing
func (o *ObservabilityManager) TrackCacheOperation(ctx context.Context, operation string, fn func() error) error {
	// Start span
	_, span := trace.SpanFromContext(ctx).TracerProvider().Tracer("semantic-cache").Start(ctx, fmt.Sprintf("cache.%s", operation))
	defer span.End()

	startTime := time.Now()
	err := fn()
	duration := time.Since(startTime).Seconds()

	// Record metrics
	status := "success"
	if err != nil {
		status = "error"
		span.RecordError(err)
	}
	o.metrics.RecordLatency(operation, status, duration)

	// Add span attributes
	span.SetAttributes(
		attribute.String("operation", operation),
		attribute.String("status", status),
		attribute.Float64("duration_seconds", duration),
	)

	return err
}

// LogCacheHit logs and records metrics for a cache hit
func (o *ObservabilityManager) LogCacheHit(ctx context.Context, query string, similarity float32) {
	o.metrics.RecordHit("semantic")

	o.logger.Debug("Cache hit", map[string]interface{}{
		"query":      query,
		"similarity": similarity,
	})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.String("cache.result", "hit"),
			attribute.Float64("cache.similarity", float64(similarity)),
		)
	}
}

// LogCacheMiss logs and records metrics for a cache miss
func (o *ObservabilityManager) LogCacheMiss(ctx context.Context, query string, reason string) {
	o.metrics.RecordMiss("semantic")

	o.logger.Debug("Cache miss", map[string]interface{}{
		"query":  query,
		"reason": reason,
	})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.String("cache.result", "miss"),
			attribute.String("cache.miss_reason", reason),
		)
	}
}

// TrackSimilaritySearch tracks similarity search metrics
func (o *ObservabilityManager) TrackSimilaritySearch(ctx context.Context, resultCount int, threshold float32, duration time.Duration) {
	o.metrics.RecordSimilaritySearch(resultCount, float64(threshold))

	o.logger.Debug("Similarity search completed", map[string]interface{}{
		"result_count": resultCount,
		"threshold":    threshold,
		"duration_ms":  duration.Milliseconds(),
	})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.Int("search.result_count", resultCount),
			attribute.Float64("search.threshold", float64(threshold)),
			attribute.Int64("search.duration_ms", duration.Milliseconds()),
		)
	}
}

// TrackCompression tracks compression metrics
func (o *ObservabilityManager) TrackCompression(ctx context.Context, originalSize, compressedSize int, duration time.Duration, operation string) {
	ratio := 0.0
	if originalSize > 0 {
		ratio = 1.0 - (float64(compressedSize) / float64(originalSize))
	}

	o.metrics.RecordCompression(ratio, duration.Seconds(), operation)

	o.logger.Debug("Compression completed", map[string]interface{}{
		"operation":       operation,
		"original_size":   originalSize,
		"compressed_size": compressedSize,
		"ratio":           ratio,
		"duration_ms":     duration.Milliseconds(),
	})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.String("compression.operation", operation),
			attribute.Int("compression.original_size", originalSize),
			attribute.Int("compression.compressed_size", compressedSize),
			attribute.Float64("compression.ratio", ratio),
		)
	}
}

// TrackEviction tracks cache eviction
func (o *ObservabilityManager) TrackEviction(ctx context.Context, count int, reason, strategy string) {
	for i := 0; i < count; i++ {
		o.metrics.RecordEviction(reason, strategy)
	}

	o.logger.Info("Cache eviction completed", map[string]interface{}{
		"count":    count,
		"reason":   reason,
		"strategy": strategy,
	})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.Int("eviction.count", count),
			attribute.String("eviction.reason", reason),
			attribute.String("eviction.strategy", strategy),
		)
	}
}

// UpdateCacheStats updates cache statistics gauges
func (o *ObservabilityManager) UpdateCacheStats(entries int, sizeBytes int64) {
	o.metrics.UpdateCacheEntries(float64(entries))
	o.metrics.UpdateCacheSize(float64(sizeBytes))
}

// TrackRateLimitExceeded tracks rate limit exceeded events
func (o *ObservabilityManager) TrackRateLimitExceeded(ctx context.Context) {
	o.metrics.RecordRateLimitExceeded()

	o.logger.Warn("Rate limit exceeded", map[string]interface{}{})

	if span := trace.SpanFromContext(ctx); span != nil {
		span.SetAttributes(
			attribute.Bool("rate_limit.exceeded", true),
		)
	}
}

// CacheHealthCheck performs a health check on cache components
type CacheHealthCheck struct {
	cache       *SemanticCache
	vectorStore *VectorStore
}

// NewCacheHealthCheck creates a new cache health check
func NewCacheHealthCheck(cache *SemanticCache, vectorStore *VectorStore) *CacheHealthCheck {
	return &CacheHealthCheck{
		cache:       cache,
		vectorStore: vectorStore,
	}
}

// Check performs the health check
func (h *CacheHealthCheck) Check(ctx context.Context) error {
	// Check Redis connectivity
	if err := h.cache.redis.Health(ctx); err != nil {
		return fmt.Errorf("redis health check failed: %w", err)
	}

	// Check database connectivity via vector store
	stats, err := h.vectorStore.GetGlobalCacheStats(ctx)
	if err != nil {
		return fmt.Errorf("vector store health check failed: %w", err)
	}

	// Log current stats
	if logger := h.cache.logger; logger != nil {
		logger.Debug("Cache health check passed", map[string]interface{}{
			"stats": stats,
		})
	}

	return nil
}

// Name returns the health check name
func (h *CacheHealthCheck) Name() string {
	return "semantic_cache"
}

// DashboardConfig provides configuration for Grafana dashboards
type DashboardConfig struct {
	Title       string
	Description string
	Panels      []PanelConfig
}

// PanelConfig defines a dashboard panel
type PanelConfig struct {
	Title  string
	Query  string
	Type   string // graph, gauge, table, etc.
	Unit   string
	Legend []string
}

// GetDefaultDashboardConfig returns the default Grafana dashboard configuration
func GetDefaultDashboardConfig() DashboardConfig {
	return DashboardConfig{
		Title:       "Semantic Cache Performance",
		Description: "Monitor semantic cache performance, hit rates, and resource usage",
		Panels: []PanelConfig{
			{
				Title:  "Cache Hit Rate",
				Query:  `rate(devmesh_semantic_cache_hits_total[5m]) / (rate(devmesh_semantic_cache_hits_total[5m]) + rate(devmesh_semantic_cache_misses_total[5m]))`,
				Type:   "graph",
				Unit:   "percentunit",
				Legend: []string{"tenant_id"},
			},
			{
				Title:  "Cache Operations/sec",
				Query:  `sum(rate(devmesh_semantic_cache_operation_duration_seconds_count[5m])) by (operation)`,
				Type:   "graph",
				Unit:   "ops",
				Legend: []string{"operation"},
			},
			{
				Title:  "P95 Latency by Operation",
				Query:  `histogram_quantile(0.95, rate(devmesh_semantic_cache_operation_duration_seconds_bucket[5m])) by (operation)`,
				Type:   "graph",
				Unit:   "s",
				Legend: []string{"operation"},
			},
			{
				Title:  "Cache Size (MB)",
				Query:  `devmesh_semantic_cache_size_bytes / 1024 / 1024`,
				Type:   "gauge",
				Unit:   "decmbytes",
				Legend: []string{"tenant_id"},
			},
			{
				Title:  "Evictions/min",
				Query:  `sum(rate(devmesh_semantic_cache_evictions_total[1m])) by (reason)`,
				Type:   "graph",
				Unit:   "short",
				Legend: []string{"reason"},
			},
			{
				Title:  "Compression Ratio",
				Query:  `histogram_quantile(0.5, rate(devmesh_semantic_cache_compression_ratio_bucket[5m]))`,
				Type:   "gauge",
				Unit:   "percentunit",
				Legend: []string{},
			},
			{
				Title:  "Vector Search Results",
				Query:  `histogram_quantile(0.5, rate(devmesh_semantic_cache_similarity_search_results_bucket[5m]))`,
				Type:   "graph",
				Unit:   "short",
				Legend: []string{"tenant_id"},
			},
			{
				Title:  "Circuit Breaker State",
				Query:  `devmesh_semantic_cache_circuit_breaker_state`,
				Type:   "table",
				Unit:   "short",
				Legend: []string{"name"},
			},
		},
	}
}

// AlertConfig defines Prometheus alerting rules
type AlertConfig struct {
	Name        string
	Expression  string
	Duration    string
	Severity    string
	Annotations map[string]string
}

// GetDefaultAlerts returns default alerting rules
func GetDefaultAlerts() []AlertConfig {
	return []AlertConfig{
		{
			Name:       "HighCacheMissRate",
			Expression: `rate(devmesh_semantic_cache_misses_total[5m]) / (rate(devmesh_semantic_cache_hits_total[5m]) + rate(devmesh_semantic_cache_misses_total[5m])) > 0.8`,
			Duration:   "5m",
			Severity:   "warning",
			Annotations: map[string]string{
				"summary":     "High cache miss rate detected",
				"description": "Cache miss rate is above 80% for {{ $labels.tenant_id }}",
			},
		},
		{
			Name:       "CacheEvictionHigh",
			Expression: `sum(rate(devmesh_semantic_cache_evictions_total[5m])) by (tenant_id) > 10`,
			Duration:   "5m",
			Severity:   "warning",
			Annotations: map[string]string{
				"summary":     "High cache eviction rate",
				"description": "Cache evicting more than 10 entries per second for {{ $labels.tenant_id }}",
			},
		},
		{
			Name:       "CacheLatencyHigh",
			Expression: `histogram_quantile(0.99, rate(devmesh_semantic_cache_operation_duration_seconds_bucket[5m])) > 1`,
			Duration:   "5m",
			Severity:   "critical",
			Annotations: map[string]string{
				"summary":     "High cache operation latency",
				"description": "P99 cache operation latency is above 1 second",
			},
		},
		{
			Name:       "CircuitBreakerOpen",
			Expression: `devmesh_semantic_cache_circuit_breaker_state == 1`,
			Duration:   "1m",
			Severity:   "critical",
			Annotations: map[string]string{
				"summary":     "Circuit breaker is open",
				"description": "Circuit breaker {{ $labels.name }} is in open state",
			},
		},
	}
}
