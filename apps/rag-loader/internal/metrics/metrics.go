// Package metrics provides Prometheus metrics for the RAG loader service
package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the RAG loader
type Metrics struct {
	// Ingestion metrics
	DocumentsProcessed  prometheus.Counter
	ChunksCreated       prometheus.Counter
	EmbeddingsGenerated prometheus.Counter
	IngestionDuration   prometheus.Histogram
	IngestionErrors     prometheus.Counter
	ActiveIngestions    prometheus.Gauge

	// Processing metrics
	ChunkingDuration  prometheus.Histogram
	EmbeddingDuration prometheus.Histogram
	StorageDuration   prometheus.Histogram
	BatchProcessed    prometheus.Counter
	ProcessingErrors  prometheus.CounterVec

	// Search metrics
	SearchRequests    prometheus.Counter
	SearchDuration    prometheus.Histogram
	SearchErrors      prometheus.Counter
	SearchResultCount prometheus.Histogram

	// Quality metrics
	ChunkCoherence     prometheus.Gauge
	RetrievalPrecision prometheus.Gauge
	RetrievalRecall    prometheus.Gauge
	MRR                prometheus.Gauge
	NDCG               prometheus.Gauge

	// Performance metrics
	CacheHitRate       prometheus.Gauge
	CacheMissRate      prometheus.Gauge
	CircuitBreakerOpen prometheus.GaugeVec
	RateLimitHits      prometheus.Counter

	// Resource metrics
	DatabaseConnections prometheus.Gauge
	RedisConnections    prometheus.Gauge
	GoroutineCount      prometheus.Gauge
	MemoryUsage         prometheus.Gauge

	// Cost tracking
	TokensProcessed  prometheus.Counter
	APICallsCount    prometheus.CounterVec
	EstimatedCostUSD prometheus.Gauge
}

// NewMetrics creates and registers all RAG loader metrics
func NewMetrics() *Metrics {
	return &Metrics{
		// Ingestion metrics
		DocumentsProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_documents_processed_total",
			Help: "Total number of documents processed",
		}),
		ChunksCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_chunks_created_total",
			Help: "Total number of chunks created",
		}),
		EmbeddingsGenerated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_embeddings_generated_total",
			Help: "Total number of embeddings generated",
		}),
		IngestionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_ingestion_duration_seconds",
			Help:    "Duration of ingestion jobs in seconds",
			Buckets: prometheus.ExponentialBuckets(1, 2, 10), // 1s to ~17min
		}),
		IngestionErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_ingestion_errors_total",
			Help: "Total number of ingestion errors",
		}),
		ActiveIngestions: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_active_ingestions",
			Help: "Number of currently active ingestion jobs",
		}),

		// Processing metrics
		ChunkingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_chunking_duration_seconds",
			Help:    "Duration of document chunking in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		}),
		EmbeddingDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_embedding_duration_seconds",
			Help:    "Duration of embedding generation in seconds",
			Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to ~100s
		}),
		StorageDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_storage_duration_seconds",
			Help:    "Duration of storage operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		}),
		BatchProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_batches_processed_total",
			Help: "Total number of batches processed",
		}),
		ProcessingErrors: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "rag_processing_errors_total",
			Help: "Total number of processing errors by type",
		}, []string{"error_type", "source_type"}),

		// Search metrics
		SearchRequests: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_search_requests_total",
			Help: "Total number of search requests",
		}),
		SearchDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_search_duration_seconds",
			Help:    "Duration of search operations in seconds",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		}),
		SearchErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_search_errors_total",
			Help: "Total number of search errors",
		}),
		SearchResultCount: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "rag_search_results_count",
			Help:    "Number of results returned per search",
			Buckets: prometheus.LinearBuckets(0, 5, 20), // 0 to 100 results
		}),

		// Quality metrics
		ChunkCoherence: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_chunk_coherence_score",
			Help: "Average semantic coherence score of chunks",
		}),
		RetrievalPrecision: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_retrieval_precision",
			Help: "Precision at K for retrieval",
		}),
		RetrievalRecall: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_retrieval_recall",
			Help: "Recall at K for retrieval",
		}),
		MRR: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_mean_reciprocal_rank",
			Help: "Mean Reciprocal Rank for retrieval",
		}),
		NDCG: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_normalized_discounted_cumulative_gain",
			Help: "Normalized Discounted Cumulative Gain",
		}),

		// Performance metrics
		CacheHitRate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_cache_hit_rate",
			Help: "Cache hit rate percentage",
		}),
		CacheMissRate: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_cache_miss_rate",
			Help: "Cache miss rate percentage",
		}),
		CircuitBreakerOpen: *promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "rag_circuit_breaker_open",
			Help: "Circuit breaker state (1 = open, 0 = closed)",
		}, []string{"service"}),
		RateLimitHits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_rate_limit_hits_total",
			Help: "Total number of rate limit hits",
		}),

		// Resource metrics
		DatabaseConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_database_connections",
			Help: "Number of active database connections",
		}),
		RedisConnections: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_redis_connections",
			Help: "Number of active Redis connections",
		}),
		GoroutineCount: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_goroutines",
			Help: "Number of goroutines",
		}),
		MemoryUsage: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_memory_usage_bytes",
			Help: "Memory usage in bytes",
		}),

		// Cost tracking
		TokensProcessed: promauto.NewCounter(prometheus.CounterOpts{
			Name: "rag_tokens_processed_total",
			Help: "Total number of tokens processed",
		}),
		APICallsCount: *promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "rag_api_calls_total",
			Help: "Total number of API calls by provider",
		}, []string{"provider", "operation"}),
		EstimatedCostUSD: promauto.NewGauge(prometheus.GaugeOpts{
			Name: "rag_estimated_cost_usd",
			Help: "Estimated cost in USD for the current month",
		}),
	}
}

// RecordIngestionMetrics records metrics for an ingestion job
func (m *Metrics) RecordIngestionMetrics(docsProcessed, chunksCreated, embeddingsGenerated int, duration float64, err error) {
	m.DocumentsProcessed.Add(float64(docsProcessed))
	m.ChunksCreated.Add(float64(chunksCreated))
	m.EmbeddingsGenerated.Add(float64(embeddingsGenerated))
	m.IngestionDuration.Observe(duration)

	if err != nil {
		m.IngestionErrors.Inc()
	}
}

// RecordSearchMetrics records metrics for a search operation
func (m *Metrics) RecordSearchMetrics(resultCount int, duration float64, err error) {
	m.SearchRequests.Inc()
	m.SearchDuration.Observe(duration)
	m.SearchResultCount.Observe(float64(resultCount))

	if err != nil {
		m.SearchErrors.Inc()
	}
}

// RecordProcessingError records a processing error with context
func (m *Metrics) RecordProcessingError(errorType, sourceType string) {
	m.ProcessingErrors.WithLabelValues(errorType, sourceType).Inc()
}

// RecordAPICall records an API call with provider and operation
func (m *Metrics) RecordAPICall(provider, operation string) {
	m.APICallsCount.WithLabelValues(provider, operation).Inc()
}

// SetCircuitBreakerState sets the circuit breaker state for a service
func (m *Metrics) SetCircuitBreakerState(service string, open bool) {
	value := 0.0
	if open {
		value = 1.0
	}
	m.CircuitBreakerOpen.WithLabelValues(service).Set(value)
}

// UpdateResourceMetrics updates resource usage metrics
func (m *Metrics) UpdateResourceMetrics(dbConns, redisConns, goroutines int, memoryBytes uint64) {
	m.DatabaseConnections.Set(float64(dbConns))
	m.RedisConnections.Set(float64(redisConns))
	m.GoroutineCount.Set(float64(goroutines))
	m.MemoryUsage.Set(float64(memoryBytes))
}

// UpdateQualityMetrics updates quality assessment metrics
func (m *Metrics) UpdateQualityMetrics(coherence, precision, recall, mrr, ndcg float64) {
	m.ChunkCoherence.Set(coherence)
	m.RetrievalPrecision.Set(precision)
	m.RetrievalRecall.Set(recall)
	m.MRR.Set(mrr)
	m.NDCG.Set(ndcg)
}
