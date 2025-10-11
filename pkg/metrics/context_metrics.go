// Story 5.2: Context Management Metrics
// LOCATION: pkg/metrics/context_metrics.go

package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// ContextMetrics holds all context-related metrics
type ContextMetrics struct {
	// Embedding metrics
	EmbeddingGenerationDuration prometheus.Histogram
	EmbeddingGenerationErrors   prometheus.Counter

	// Retrieval metrics
	ContextRetrievalMethod   *prometheus.CounterVec
	ContextRetrievalDuration prometheus.Histogram

	// Compaction metrics
	CompactionExecutions *prometheus.CounterVec
	CompactionDuration   prometheus.Histogram
	TokensSaved          prometheus.Counter

	// Token utilization
	TokenUtilization prometheus.Histogram

	// Security metrics
	SecurityViolations *prometheus.CounterVec
	AuditEvents        *prometheus.CounterVec
}

var (
	contextMetricsInstance *ContextMetrics
	contextMetricsOnce     sync.Once
)

// NewContextMetrics creates and registers context metrics (singleton)
func NewContextMetrics() *ContextMetrics {
	contextMetricsOnce.Do(func() {
		contextMetricsInstance = initContextMetrics()
	})
	return contextMetricsInstance
}

// initContextMetrics initializes the metrics (called only once)
func initContextMetrics() *ContextMetrics {
	return &ContextMetrics{
		EmbeddingGenerationDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "context_embedding_generation_duration_seconds",
			Help:    "Time to generate embeddings for context items",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to ~1s
		}),

		EmbeddingGenerationErrors: promauto.NewCounter(prometheus.CounterOpts{
			Name: "context_embedding_generation_errors_total",
			Help: "Total number of embedding generation errors",
		}),

		ContextRetrievalMethod: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "context_retrieval_method_total",
			Help: "Count of context retrievals by method",
		}, []string{"method"}), // "full", "semantic", "windowed"

		ContextRetrievalDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "context_retrieval_duration_seconds",
			Help:    "Time to retrieve context",
			Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
		}),

		CompactionExecutions: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "context_compaction_executions_total",
			Help: "Count of context compactions by strategy",
		}, []string{"strategy", "status"}), // strategy: summarize/prune/sliding, status: success/failure

		CompactionDuration: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "context_compaction_duration_seconds",
			Help:    "Time to compact context",
			Buckets: prometheus.ExponentialBuckets(0.01, 2, 10), // 10ms to ~10s
		}),

		TokensSaved: promauto.NewCounter(prometheus.CounterOpts{
			Name: "context_tokens_saved_total",
			Help: "Total tokens saved through compaction",
		}),

		TokenUtilization: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "context_token_utilization_ratio",
			Help:    "Ratio of tokens used vs max tokens in context window",
			Buckets: prometheus.LinearBuckets(0, 0.1, 11), // 0.0 to 1.0
		}),

		SecurityViolations: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "context_security_violations_total",
			Help: "Count of security violations detected",
		}, []string{"type"}), // "injection", "cross_tenant", "replay"

		AuditEvents: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "context_audit_events_total",
			Help: "Count of audit events by operation",
		}, []string{"operation", "tenant_id"}),
	}
}

// RecordEmbeddingGeneration records embedding generation metrics
func (m *ContextMetrics) RecordEmbeddingGeneration(duration float64, success bool) {
	m.EmbeddingGenerationDuration.Observe(duration)
	if !success {
		m.EmbeddingGenerationErrors.Inc()
	}
}

// RecordRetrieval records context retrieval metrics
func (m *ContextMetrics) RecordRetrieval(method string, duration float64) {
	m.ContextRetrievalMethod.WithLabelValues(method).Inc()
	m.ContextRetrievalDuration.Observe(duration)
}

// RecordCompaction records compaction metrics
func (m *ContextMetrics) RecordCompaction(strategy string, duration float64, tokensSaved int, success bool) {
	status := "success"
	if !success {
		status = "failure"
	}

	m.CompactionExecutions.WithLabelValues(strategy, status).Inc()
	m.CompactionDuration.Observe(duration)

	if tokensSaved > 0 {
		m.TokensSaved.Add(float64(tokensSaved))
	}
}

// RecordTokenUtilization records token usage ratio
func (m *ContextMetrics) RecordTokenUtilization(usedTokens, maxTokens int) {
	if maxTokens > 0 {
		ratio := float64(usedTokens) / float64(maxTokens)
		m.TokenUtilization.Observe(ratio)
	}
}

// RecordSecurityViolation records security issues
func (m *ContextMetrics) RecordSecurityViolation(violationType string) {
	m.SecurityViolations.WithLabelValues(violationType).Inc()
}

// RecordAuditEvent records audit trail events
func (m *ContextMetrics) RecordAuditEvent(operation, tenantID string) {
	m.AuditEvents.WithLabelValues(operation, tenantID).Inc()
}
