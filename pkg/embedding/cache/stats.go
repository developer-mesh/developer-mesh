package cache

import (
	"container/heap"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Stats returns comprehensive cache statistics
func (c *SemanticCache) Stats(ctx context.Context) (*CacheStats, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "semantic_cache.stats")
	defer span.End()

	stats := &CacheStats{
		Timestamp: time.Now(),
	}

	// Get hit/miss stats using atomic counters
	stats.TotalHits = int(c.hitCount.Load())
	stats.TotalMisses = int(c.missCount.Load())

	// Calculate hit rate
	total := stats.TotalHits + stats.TotalMisses
	if total > 0 {
		stats.HitRate = float64(stats.TotalHits) / float64(total)
	}

	// Get cache entries
	pattern := fmt.Sprintf("%s:query:*", c.config.Prefix)
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	stats.TotalEntries = len(keys)

	// Analyze cache entries
	if len(keys) > 0 {
		err = c.analyzeEntries(ctx, keys, stats)
		if err != nil {
			c.logger.Warn("Failed to analyze all entries", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Estimate memory usage
	stats.MemoryUsageBytes = c.estimateMemoryUsage(ctx)

	return stats, nil
}

// GetEntryStats returns statistics for a specific cache entry
func (c *SemanticCache) GetEntryStats(ctx context.Context, query string) (*CacheEntry, error) {
	normalized := c.normalizer.Normalize(query)
	return c.getExactMatch(ctx, normalized)
}

// GetTopQueries returns the most frequently accessed queries using efficient heap for top-K selection
func (c *SemanticCache) GetTopQueries(ctx context.Context, limit int) ([]*CacheEntry, error) {
	// First check local cache for efficiency
	// sync.Map always exists, so check if it has entries
	hasEntries := false
	c.entries.Range(func(key, value interface{}) bool {
		hasEntries = true
		return false // Stop after finding first entry
	})

	if hasEntries {
		return c.getTopQueriesFromLocalCache(ctx, limit)
	}

	// Fallback to Redis scan
	pattern := fmt.Sprintf("%s:query:*", c.config.Prefix)
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	// Use min heap for efficient top-K selection
	h := &entryHeap{}
	heap.Init(h)

	// Process entries
	for _, key := range keys {
		entry, err := c.getCacheEntry(ctx, key)
		if err != nil {
			continue
		}

		if h.Len() < limit {
			heap.Push(h, entry)
		} else if entry.HitCount > (*h)[0].HitCount {
			heap.Pop(h)
			heap.Push(h, entry)
		}
	}

	// Extract results in descending order
	results := make([]*CacheEntry, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(*CacheEntry)
	}

	return results, nil
}

// getTopQueriesFromLocalCache uses local cache for efficiency
func (c *SemanticCache) getTopQueriesFromLocalCache(ctx context.Context, limit int) ([]*CacheEntry, error) {
	// Create min heap for top K entries
	h := &entryHeap{}
	heap.Init(h)

	// Iterate through entries
	c.entries.Range(func(key, value interface{}) bool {
		entry := value.(*CacheEntry)

		if h.Len() < limit {
			heap.Push(h, entry)
		} else if entry.HitCount > (*h)[0].HitCount {
			heap.Pop(h)
			heap.Push(h, entry)
		}

		return true
	})

	// Extract results in descending order
	results := make([]*CacheEntry, h.Len())
	for i := len(results) - 1; i >= 0; i-- {
		results[i] = heap.Pop(h).(*CacheEntry)
	}

	return results, nil
}

// GetStaleEntries returns entries that haven't been accessed recently
func (c *SemanticCache) GetStaleEntries(ctx context.Context, staleDuration time.Duration) ([]*CacheEntry, error) {
	pattern := fmt.Sprintf("%s:query:*", c.config.Prefix)
	keys, err := c.scanKeys(ctx, pattern)
	if err != nil {
		return nil, fmt.Errorf("failed to scan keys: %w", err)
	}

	staleTime := time.Now().Add(-staleDuration)
	staleEntries := make([]*CacheEntry, 0)

	for _, key := range keys {
		entry, err := c.getCacheEntry(ctx, key)
		if err != nil {
			continue
		}

		if entry.LastAccessedAt.Before(staleTime) {
			staleEntries = append(staleEntries, entry)
		}
	}

	return staleEntries, nil
}

// ExportStats exports cache statistics in various formats
func (c *SemanticCache) ExportStats(ctx context.Context, format string) ([]byte, error) {
	stats, err := c.Stats(ctx)
	if err != nil {
		return nil, err
	}

	switch format {
	case "json":
		return json.MarshalIndent(stats, "", "  ")
	case "prometheus":
		return c.exportPrometheusMetrics(stats), nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// Helper methods

func (c *SemanticCache) scanKeys(ctx context.Context, pattern string) ([]string, error) {
	var keys []string

	iter := c.redis.Scan(ctx, 0, pattern, 100).Iterator()
	for iter.Next(ctx) {
		keys = append(keys, iter.Val())
	}

	if err := iter.Err(); err != nil {
		return nil, err
	}

	return keys, nil
}

func (c *SemanticCache) analyzeEntries(ctx context.Context, keys []string, stats *CacheStats) error {
	var (
		totalHits    int
		totalResults int
		oldestEntry  time.Time
	)

	// Process in batches to avoid timeout
	batchSize := 100
	for i := 0; i < len(keys); i += batchSize {
		end := i + batchSize
		if end > len(keys) {
			end = len(keys)
		}

		batch := keys[i:end]
		for _, key := range batch {
			entry, err := c.getCacheEntry(ctx, key)
			if err != nil {
				continue
			}

			totalHits += entry.HitCount
			totalResults += len(entry.Results)

			if oldestEntry.IsZero() || entry.CachedAt.Before(oldestEntry) {
				oldestEntry = entry.CachedAt
			}
		}

		// Check context cancellation
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if stats.TotalEntries > 0 {
		stats.AverageHitsPerEntry = float64(totalHits) / float64(stats.TotalEntries)
		stats.AverageResultsPerEntry = float64(totalResults) / float64(stats.TotalEntries)
	}

	if !oldestEntry.IsZero() {
		stats.OldestEntry = time.Since(oldestEntry)
	}

	return nil
}

func (c *SemanticCache) estimateMemoryUsage(ctx context.Context) int64 {
	// Get Redis memory usage for our keys
	pattern := fmt.Sprintf("%s:*", c.config.Prefix)

	// Use Redis MEMORY USAGE command if available
	var totalBytes int64

	keys, _ := c.scanKeys(ctx, pattern)
	for _, key := range keys {
		// Redis MEMORY USAGE is available in Redis 4.0+
		usage, err := c.redis.MemoryUsage(ctx, key)
		if err == nil {
			totalBytes += usage
		}
	}

	return totalBytes
}

func (c *SemanticCache) exportPrometheusMetrics(stats *CacheStats) []byte {
	metrics := fmt.Sprintf(`# HELP semantic_cache_entries_total Total number of cache entries
# TYPE semantic_cache_entries_total gauge
semantic_cache_entries_total %d

# HELP semantic_cache_hits_total Total number of cache hits
# TYPE semantic_cache_hits_total counter
semantic_cache_hits_total %d

# HELP semantic_cache_misses_total Total number of cache misses
# TYPE semantic_cache_misses_total counter
semantic_cache_misses_total %d

# HELP semantic_cache_hit_rate Cache hit rate
# TYPE semantic_cache_hit_rate gauge
semantic_cache_hit_rate %f

# HELP semantic_cache_memory_bytes Memory usage in bytes
# TYPE semantic_cache_memory_bytes gauge
semantic_cache_memory_bytes %d

# HELP semantic_cache_average_hits_per_entry Average hits per cache entry
# TYPE semantic_cache_average_hits_per_entry gauge
semantic_cache_average_hits_per_entry %f

# HELP semantic_cache_average_results_per_entry Average results per cache entry
# TYPE semantic_cache_average_results_per_entry gauge
semantic_cache_average_results_per_entry %f

# HELP semantic_cache_oldest_entry_seconds Age of oldest cache entry in seconds
# TYPE semantic_cache_oldest_entry_seconds gauge
semantic_cache_oldest_entry_seconds %f
`,
		stats.TotalEntries,
		stats.TotalHits,
		stats.TotalMisses,
		stats.HitRate,
		stats.MemoryUsageBytes,
		stats.AverageHitsPerEntry,
		stats.AverageResultsPerEntry,
		stats.OldestEntry.Seconds(),
	)

	return []byte(metrics)
}

// CacheAnalytics provides advanced analytics for cache performance
type CacheAnalytics struct {
	cache  *SemanticCache
	logger observability.Logger
}

// NewCacheAnalytics creates a new cache analytics instance
func NewCacheAnalytics(cache *SemanticCache, logger observability.Logger) *CacheAnalytics {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.analytics")
	}

	return &CacheAnalytics{
		cache:  cache,
		logger: logger,
	}
}

// AnalyzeQueryPatterns analyzes common query patterns
func (ca *CacheAnalytics) AnalyzeQueryPatterns(ctx context.Context) (map[string]int, error) {
	entries, err := ca.cache.GetTopQueries(ctx, 100)
	if err != nil {
		return nil, err
	}

	// Extract common terms
	termFrequency := make(map[string]int)

	for _, entry := range entries {
		// Tokenize normalized query
		tokens := tokenize(entry.NormalizedQuery)
		for _, token := range tokens {
			termFrequency[token] += entry.HitCount
		}
	}

	return termFrequency, nil
}

// AnalyzeCacheEfficiency analyzes cache efficiency metrics
func (ca *CacheAnalytics) AnalyzeCacheEfficiency(ctx context.Context) (*CacheEfficiencyReport, error) {
	stats, err := ca.cache.Stats(ctx)
	if err != nil {
		return nil, err
	}

	report := &CacheEfficiencyReport{
		HitRate:             stats.HitRate,
		TotalQueries:        stats.TotalHits + stats.TotalMisses,
		CacheSize:           stats.TotalEntries,
		MemoryUsageBytes:    stats.MemoryUsageBytes,
		AverageHitsPerEntry: stats.AverageHitsPerEntry,
		Timestamp:           time.Now(),
	}

	// Calculate memory efficiency
	if stats.TotalEntries > 0 {
		report.BytesPerEntry = stats.MemoryUsageBytes / int64(stats.TotalEntries)
	}

	// Check for inefficiencies
	if stats.HitRate < 0.5 {
		report.Recommendations = append(report.Recommendations,
			"Low hit rate detected. Consider warming cache with more common queries.")
	}

	if stats.AverageHitsPerEntry < 2 {
		report.Recommendations = append(report.Recommendations,
			"Many entries have low hit counts. Consider reducing cache size or TTL.")
	}

	staleEntries, _ := ca.cache.GetStaleEntries(ctx, 7*24*time.Hour)
	if len(staleEntries) > stats.TotalEntries/2 {
		report.Recommendations = append(report.Recommendations,
			"Over 50% of entries are stale. Consider implementing more aggressive eviction.")
	}

	return report, nil
}

// CacheEfficiencyReport contains cache efficiency analysis
type CacheEfficiencyReport struct {
	HitRate             float64   `json:"hit_rate"`
	TotalQueries        int       `json:"total_queries"`
	CacheSize           int       `json:"cache_size"`
	MemoryUsageBytes    int64     `json:"memory_usage_bytes"`
	BytesPerEntry       int64     `json:"bytes_per_entry"`
	AverageHitsPerEntry float64   `json:"average_hits_per_entry"`
	Recommendations     []string  `json:"recommendations"`
	Timestamp           time.Time `json:"timestamp"`
}

// Helper function to tokenize queries
func tokenize(text string) []string {
	// Simple whitespace tokenization
	return strings.Fields(text)
}

// Min heap implementation for top-K selection
type entryHeap []*CacheEntry

func (h entryHeap) Len() int           { return len(h) }
func (h entryHeap) Less(i, j int) bool { return h[i].HitCount < h[j].HitCount }
func (h entryHeap) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func (h *entryHeap) Push(x interface{}) {
	*h = append(*h, x.(*CacheEntry))
}

func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[0 : n-1]
	return x
}
