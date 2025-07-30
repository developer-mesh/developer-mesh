package monitoring

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Cache operation metrics
	cacheOperationDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "devmesh_cache_operation_duration_seconds",
		Help:    "Duration of cache operations in seconds",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"operation", "tenant_id", "status"})

	cacheOperationCount = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_operation_total",
		Help: "Total number of cache operations",
	}, []string{"operation", "tenant_id", "status"})

	// Cache hit/miss metrics
	cacheHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_hits_total",
		Help: "Total number of cache hits",
	}, []string{"tenant_id"})

	cacheMisses = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_misses_total",
		Help: "Total number of cache misses",
	}, []string{"tenant_id"})

	cacheHitRate = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "devmesh_cache_hit_rate",
		Help: "Cache hit rate per tenant",
	}, []string{"tenant_id"})

	// Cache size metrics
	cacheEntries = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "devmesh_cache_entries",
		Help: "Number of cache entries per tenant",
	}, []string{"tenant_id"})

	cacheBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "devmesh_cache_bytes",
		Help: "Total bytes in cache per tenant",
	}, []string{"tenant_id"})

	// LRU eviction metrics
	lruEvictions = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_evictions_total",
		Help: "Total number of cache evictions",
	}, []string{"tenant_id", "reason"})

	lruEvictionDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "devmesh_cache_eviction_duration_seconds",
		Help:    "Duration of cache eviction operations",
		Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"tenant_id"})

	// Rate limiting metrics
	cacheRateLimitHits = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_rate_limit_hits_total",
		Help: "Total number of cache rate limit hits",
	}, []string{"tenant_id", "operation"})

	// Migration metrics
	cacheMigrationMode = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "devmesh_cache_migration_mode",
		Help: "Current cache migration mode (0=legacy, 1=migrating, 2=tenant_only)",
	}, []string{"mode"})

	cacheMigrationCopies = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "devmesh_cache_migration_copies_total",
		Help: "Total number of cache entries copied during migration",
	}, []string{"status"})

	// Vector store metrics
	vectorSimilaritySearchDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "devmesh_cache_vector_search_duration_seconds",
		Help:    "Duration of vector similarity searches",
		Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
	}, []string{"tenant_id"})
)

// PrometheusMetricsCollector implements observability.MetricsClient using Prometheus
type PrometheusMetricsCollector struct{}

// IncrementCounter increments a counter metric
func (p *PrometheusMetricsCollector) IncrementCounter(name string, value float64) {
	// Map to specific Prometheus metrics
	switch name {
	case "cache.hit":
		cacheHits.WithLabelValues("").Add(value)
	case "cache.miss":
		cacheMisses.WithLabelValues("").Add(value)
	}
}

// IncrementCounterWithLabels increments a counter metric with labels
func (p *PrometheusMetricsCollector) IncrementCounterWithLabels(name string, value float64, labels map[string]string) {
	tenantID := labels["tenant_id"]

	switch name {
	case "cache.tenant.hit":
		cacheHits.WithLabelValues(tenantID).Add(value)
	case "cache.tenant.miss":
		cacheMisses.WithLabelValues(tenantID).Add(value)
	case "cache.evicted":
		reason := labels["reason"]
		if reason == "" {
			reason = "lru"
		}
		lruEvictions.WithLabelValues(tenantID, reason).Add(value)
	case "cache.rate_limited":
		operation := labels["operation"]
		if operation == "" {
			operation = "unknown"
		}
		cacheRateLimitHits.WithLabelValues(tenantID, operation).Add(value)
	case "cache.migration.tenant_hit", "cache.migration.legacy_hit":
		// Track migration progress
		cacheMigrationMode.WithLabelValues("migrating").Set(1)
	case "cache.migration.copy_success":
		cacheMigrationCopies.WithLabelValues("success").Add(value)
	case "cache.migration.copy_failed":
		cacheMigrationCopies.WithLabelValues("failed").Add(value)
	case "cache.operation.count":
		operation := labels["operation"]
		status := labels["status"]
		cacheOperationCount.WithLabelValues(operation, tenantID, status).Add(value)
	}
}

// RecordHistogram records a histogram metric
func (p *PrometheusMetricsCollector) RecordHistogram(name string, value float64, labels map[string]string) {
	tenantID := labels["tenant_id"]

	switch name {
	case "cache.operation.duration":
		operation := labels["operation"]
		status := labels["status"]
		cacheOperationDuration.WithLabelValues(operation, tenantID, status).Observe(value)
	case "lru.eviction_cycle.duration":
		lruEvictionDuration.WithLabelValues(tenantID).Observe(value)
	case "cache.vector_store.duration":
		vectorSimilaritySearchDuration.WithLabelValues(tenantID).Observe(value)
	}
}

// SetGauge sets a gauge metric
func (p *PrometheusMetricsCollector) SetGauge(name string, value float64) {
	// Map to specific Prometheus metrics
	switch name {
	case "cache.global.tenant_count":
		// Would need a global gauge for this
	case "cache.global.total_entries":
		// Would need a global gauge for this
	}
}

// SetGaugeWithLabels sets a gauge metric with labels
func (p *PrometheusMetricsCollector) SetGaugeWithLabels(name string, value float64, labels map[string]string) {
	tenantID := labels["tenant_id"]

	switch name {
	case "cache.entries":
		cacheEntries.WithLabelValues(tenantID).Set(value)
	case "cache.bytes":
		cacheBytes.WithLabelValues(tenantID).Set(value)
	case "cache.hit_rate":
		cacheHitRate.WithLabelValues(tenantID).Set(value)
	}
}

// RecordGauge records a gauge metric
func (p *PrometheusMetricsCollector) RecordGauge(name string, value float64, labels map[string]string) {
	p.SetGaugeWithLabels(name, value, labels)
}

// Close closes the metrics client
func (p *PrometheusMetricsCollector) Close() error {
	// Prometheus metrics don't need cleanup
	return nil
}
