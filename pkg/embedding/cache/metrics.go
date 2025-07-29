package cache

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Cache hit/miss metrics
	cacheHits = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "hits_total",
			Help:      "Total number of cache hits",
		},
		[]string{"tenant_id", "cache_type"},
	)

	cacheMisses = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "misses_total",
			Help:      "Total number of cache misses",
		},
		[]string{"tenant_id", "cache_type"},
	)

	// Operation latency metrics
	cacheLatency = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "operation_duration_seconds",
			Help:      "Cache operation latency",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		},
		[]string{"operation", "status"},
	)

	// Cache size metrics
	cacheSize = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "size_bytes",
			Help:      "Current cache size in bytes",
		},
		[]string{"tenant_id"},
	)

	cacheEntries = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "entries",
			Help:      "Number of entries in cache",
		},
		[]string{"tenant_id"},
	)

	// Eviction metrics
	cacheEvictions = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "evictions_total",
			Help:      "Total number of cache evictions",
		},
		[]string{"tenant_id", "reason", "strategy"},
	)

	// Similarity search metrics
	similaritySearchResults = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "similarity_search_results",
			Help:      "Number of results returned by similarity search",
			Buckets:   prometheus.LinearBuckets(0, 1, 20), // 0 to 20 results
		},
		[]string{"tenant_id"},
	)

	similarityThreshold = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "similarity_threshold",
			Help:      "Similarity threshold distribution",
			Buckets:   prometheus.LinearBuckets(0, 0.1, 11), // 0.0 to 1.0
		},
		[]string{"tenant_id"},
	)

	// Compression metrics
	compressionRatio = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "compression_ratio",
			Help:      "Compression ratio achieved",
			Buckets:   prometheus.LinearBuckets(0, 0.1, 11), // 0% to 100%
		},
		[]string{"tenant_id"},
	)

	compressionTime = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "compression_duration_seconds",
			Help:      "Time taken to compress/decompress",
			Buckets:   prometheus.ExponentialBuckets(0.0001, 2, 10), // 0.1ms to ~100ms
		},
		[]string{"operation"},
	)

	// Circuit breaker metrics
	circuitBreakerState = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "circuit_breaker_state",
			Help:      "Current state of circuit breaker (0=closed, 1=open, 2=half-open)",
		},
		[]string{"name"},
	)

	// Rate limiting metrics
	rateLimitExceeded = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "rate_limit_exceeded_total",
			Help:      "Total number of rate limit exceeded events",
		},
		[]string{"tenant_id"},
	)

	// Warmup metrics
	warmupDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "warmup_duration_seconds",
			Help:      "Time taken to warm cache",
			Buckets:   prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~1000s
		},
		[]string{"source"},
	)

	warmupQueries = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "warmup_queries_total",
			Help:      "Total number of queries warmed",
		},
		[]string{"source", "status"},
	)

	// Vector store metrics
	vectorStoreOperations = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "vector_store_operations_total",
			Help:      "Total number of vector store operations",
		},
		[]string{"operation", "status"},
	)

	vectorStoreDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: "devmesh",
			Subsystem: "semantic_cache",
			Name:      "vector_store_duration_seconds",
			Help:      "Duration of vector store operations",
			Buckets:   prometheus.ExponentialBuckets(0.001, 2, 10),
		},
		[]string{"operation"},
	)
)

// MetricsCollector provides methods to update cache metrics
type MetricsCollector struct {
	tenantID string
}

// NewMetricsCollector creates a new metrics collector for a tenant
func NewMetricsCollector(tenantID string) *MetricsCollector {
	return &MetricsCollector{
		tenantID: tenantID,
	}
}

// RecordHit records a cache hit
func (m *MetricsCollector) RecordHit(cacheType string) {
	cacheHits.WithLabelValues(m.tenantID, cacheType).Inc()
}

// RecordMiss records a cache miss
func (m *MetricsCollector) RecordMiss(cacheType string) {
	cacheMisses.WithLabelValues(m.tenantID, cacheType).Inc()
}

// RecordLatency records operation latency
func (m *MetricsCollector) RecordLatency(operation string, status string, seconds float64) {
	cacheLatency.WithLabelValues(operation, status).Observe(seconds)
}

// UpdateCacheSize updates the current cache size
func (m *MetricsCollector) UpdateCacheSize(bytes float64) {
	cacheSize.WithLabelValues(m.tenantID).Set(bytes)
}

// UpdateCacheEntries updates the number of cache entries
func (m *MetricsCollector) UpdateCacheEntries(count float64) {
	cacheEntries.WithLabelValues(m.tenantID).Set(count)
}

// RecordEviction records a cache eviction
func (m *MetricsCollector) RecordEviction(reason, strategy string) {
	cacheEvictions.WithLabelValues(m.tenantID, reason, strategy).Inc()
}

// RecordSimilaritySearch records similarity search metrics
func (m *MetricsCollector) RecordSimilaritySearch(resultCount int, threshold float64) {
	similaritySearchResults.WithLabelValues(m.tenantID).Observe(float64(resultCount))
	similarityThreshold.WithLabelValues(m.tenantID).Observe(threshold)
}

// RecordCompression records compression metrics
func (m *MetricsCollector) RecordCompression(ratio float64, duration float64, operation string) {
	compressionRatio.WithLabelValues(m.tenantID).Observe(ratio)
	compressionTime.WithLabelValues(operation).Observe(duration)
}

// RecordRateLimitExceeded records rate limit exceeded event
func (m *MetricsCollector) RecordRateLimitExceeded() {
	rateLimitExceeded.WithLabelValues(m.tenantID).Inc()
}

// GlobalMetrics provides global metrics functions
type GlobalMetrics struct{}

// UpdateCircuitBreakerState updates circuit breaker state
func (g *GlobalMetrics) UpdateCircuitBreakerState(name string, state int) {
	circuitBreakerState.WithLabelValues(name).Set(float64(state))
}

// RecordWarmup records warmup metrics
func (g *GlobalMetrics) RecordWarmup(source string, duration float64, queryCount int, status string) {
	warmupDuration.WithLabelValues(source).Observe(duration)
	warmupQueries.WithLabelValues(source, status).Add(float64(queryCount))
}

// RecordVectorStoreOperation records vector store operation metrics
func (g *GlobalMetrics) RecordVectorStoreOperation(operation, status string, duration float64) {
	vectorStoreOperations.WithLabelValues(operation, status).Inc()
	vectorStoreDuration.WithLabelValues(operation).Observe(duration)
}

// RegisterMetrics registers all metrics with the provided registry
func RegisterMetrics(registry prometheus.Registerer) {
	// Metrics are already registered via promauto
	// This function is kept for compatibility
}

// GetMetricsHandler returns a handler for Prometheus metrics
func GetMetricsHandler() func() []byte {
	return func() []byte {
		// This is a simplified version
		// In production, use promhttp.Handler()
		return []byte("# Metrics endpoint - use promhttp.Handler() for production")
	}
}
