package cache

import (
	"context"
	"time"
)

// CacheEntry represents a cached query result with associated metadata.
// It stores the original query, its normalized form, embedding vector,
// search results, and usage statistics for cache management.
//
// CacheEntry instances are immutable after creation and safe for
// concurrent access.
type CacheEntry struct {
	Query           string                 `json:"query"`
	NormalizedQuery string                 `json:"normalized_query"`
	Embedding       []float32              `json:"embedding"`
	Results         []CachedSearchResult   `json:"results"`
	Metadata        map[string]interface{} `json:"metadata"`
	CachedAt        time.Time              `json:"cached_at"`
	HitCount        int                    `json:"hit_count"`
	LastAccessedAt  time.Time              `json:"last_accessed_at"`
	TTL             time.Duration          `json:"ttl"`
}

// CachedSearchResult represents a simplified search result for caching.
// It contains the essential information from a search result including
// relevance score and content, with optional metadata for filtering
// and additional context.
type CachedSearchResult struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type,omitempty"`
	Score       float32                `json:"score"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Config configures the semantic cache behavior.
// It controls cache behavior including TTL, similarity thresholds,
// storage limits, and performance optimizations.
//
// Use DefaultConfig() to get a configuration with sensible defaults,
// then customize specific fields as needed.
type Config struct {
	// SimilarityThreshold is the minimum similarity for cache hit (0.0 to 1.0)
	SimilarityThreshold float32 `json:"similarity_threshold"`
	// TTL is the default cache entry time-to-live
	TTL time.Duration `json:"ttl"`
	// MaxCandidates is the maximum number of candidates to check for similarity
	MaxCandidates int `json:"max_candidates"`
	// MaxCacheSize is the maximum number of entries to keep in cache
	MaxCacheSize int `json:"max_cache_size"`
	// Prefix is the Redis key prefix for cache entries
	Prefix string `json:"prefix"`
	// WarmupQueries are queries to pre-warm the cache with
	WarmupQueries []string `json:"warmup_queries"`
	// EnableMetrics enables metrics collection
	EnableMetrics bool `json:"enable_metrics"`
	// EnableCompression enables compression of cached results
	EnableCompression bool `json:"enable_compression"`
	// EnableAuditLogging enables audit logging for compliance
	EnableAuditLogging bool `json:"enable_audit_logging"`
	// RedisPoolConfig contains Redis connection pool settings
	RedisPoolConfig *RedisPoolConfig `json:"redis_pool_config,omitempty"`
	// PerformanceConfig contains performance tuning parameters
	PerformanceConfig *PerformanceConfig `json:"performance_config,omitempty"`
}

// DefaultConfig returns default cache configuration with production-ready settings.
// The defaults provide a good balance between performance and resource usage:
//   - 95% similarity threshold for high-quality matches
//   - 24-hour TTL for cached entries
//   - Support for up to 10,000 cached queries
//   - Metrics enabled for observability
//
// Adjust these values based on your specific use case and requirements.
func DefaultConfig() *Config {
	return &Config{
		SimilarityThreshold: 0.95,
		TTL:                 24 * time.Hour,
		MaxCandidates:       10,
		MaxCacheSize:        10000,
		Prefix:              "semantic_cache",
		EnableMetrics:       true,
		EnableCompression:   false,
		RedisPoolConfig:     DefaultRedisPoolConfig(),
		PerformanceConfig:   GetPerformanceProfile(ProfileBalanced),
	}
}

// CacheStats represents cache statistics at a point in time.
// It provides metrics for monitoring cache performance and effectiveness,
// including hit rates, memory usage, and entry counts.
//
// These statistics are used for observability, capacity planning,
// and identifying optimization opportunities.
type CacheStats struct {
	TotalEntries           int           `json:"total_entries"`
	TotalHits              int           `json:"total_hits"`
	TotalMisses            int           `json:"total_misses"`
	AverageHitsPerEntry    float64       `json:"average_hits_per_entry"`
	AverageResultsPerEntry float64       `json:"average_results_per_entry"`
	OldestEntry            time.Duration `json:"oldest_entry"`
	HitRate                float64       `json:"hit_rate"`
	MemoryUsageBytes       int64         `json:"memory_usage_bytes"`
	Timestamp              time.Time     `json:"timestamp"`
}

// SimilarQuery represents a query similarity match found during vector search.
// It contains the matched query text, its cache key for retrieval,
// and similarity score indicating how closely it matches the search query.
//
// Results are typically ordered by descending similarity score.
type SimilarQuery struct {
	CacheKey   string  `json:"cache_key"`
	Query      string  `json:"query"`
	Similarity float32 `json:"similarity"`
}

// SearchExecutor is a function type for executing searches when cache misses occur.
// It takes a query and returns search results that can be cached.
//
// Implementations should handle errors gracefully and may include
// retry logic or fallback mechanisms.
type SearchExecutor func(ctx context.Context, query string) ([]CachedSearchResult, error)
