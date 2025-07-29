package cache

import (
	"context"
	"time"
)

// CacheEntry represents a cached query result
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

// CachedSearchResult represents a simplified search result for caching
type CachedSearchResult struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type,omitempty"`
	Score       float32                `json:"score"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// Config configures the semantic cache
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
}

// DefaultConfig returns default cache configuration
func DefaultConfig() *Config {
	return &Config{
		SimilarityThreshold: 0.95,
		TTL:                 24 * time.Hour,
		MaxCandidates:       10,
		MaxCacheSize:        10000,
		Prefix:              "semantic_cache",
		EnableMetrics:       true,
		EnableCompression:   false,
	}
}

// CacheStats represents cache statistics
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

// SimilarQuery represents a query similarity match
type SimilarQuery struct {
	CacheKey   string  `json:"cache_key"`
	Query      string  `json:"query"`
	Similarity float32 `json:"similarity"`
}

// SearchExecutor is a function type for executing searches
type SearchExecutor func(ctx context.Context, query string) ([]CachedSearchResult, error)
