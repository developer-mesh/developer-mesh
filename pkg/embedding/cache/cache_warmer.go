package cache

import (
	"context"
	"fmt"
	"runtime/debug"
	"sort"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CacheWarmer handles pre-loading cache with common queries
type CacheWarmer struct {
	cache          *SemanticCache
	searchExecutor SearchExecutor
	logger         observability.Logger
}

// NewCacheWarmer creates a new cache warmer
func NewCacheWarmer(cache *SemanticCache, executor SearchExecutor, logger observability.Logger) *CacheWarmer {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.warmer")
	}

	return &CacheWarmer{
		cache:          cache,
		searchExecutor: executor,
		logger:         logger,
	}
}

// WarmupQuery represents a query to warm the cache with
type WarmupQuery struct {
	Query     string    `json:"query"`
	Embedding []float32 `json:"embedding,omitempty"`
	Priority  int       `json:"priority"` // Higher priority queries are warmed first
}

// WarmupResult represents the result of a warmup operation
type WarmupResult struct {
	Query       string
	Success     bool
	Error       error
	ResultCount int
	Duration    time.Duration
	FromCache   bool
}

// Warm pre-loads cache with the provided queries
func (w *CacheWarmer) Warm(ctx context.Context, queries []string) ([]WarmupResult, error) {
	// Convert to WarmupQuery with default priority
	warmupQueries := make([]WarmupQuery, len(queries))
	for i, q := range queries {
		warmupQueries[i] = WarmupQuery{Query: q, Priority: 0}
	}

	return w.WarmWithPriority(ctx, warmupQueries)
}

// WarmWithPriority pre-loads cache with prioritized queries
func (w *CacheWarmer) WarmWithPriority(ctx context.Context, queries []WarmupQuery) ([]WarmupResult, error) {
	startTime := time.Now()

	// Sort by priority (higher first)
	sortedQueries := make([]WarmupQuery, len(queries))
	copy(sortedQueries, queries)
	sort.Slice(sortedQueries, func(i, j int) bool {
		return sortedQueries[i].Priority > sortedQueries[j].Priority
	})

	// Process queries with controlled concurrency
	results := make([]WarmupResult, len(queries))
	var wg sync.WaitGroup

	// Use semaphore to limit concurrency
	concurrency := 10
	sem := make(chan struct{}, concurrency)

	for i, wq := range sortedQueries {
		wg.Add(1)
		go func(idx int, warmupQuery WarmupQuery) {
			defer wg.Done()

			// Acquire semaphore
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				results[idx] = WarmupResult{
					Query:   warmupQuery.Query,
					Success: false,
					Error:   ctx.Err(),
				}
				return
			}

			result := w.warmSingleQuery(ctx, warmupQuery)
			results[idx] = result
		}(i, wq)
	}

	// Wait for all queries to complete
	wg.Wait()

	// Log summary
	successCount := 0
	failureCount := 0
	fromCacheCount := 0
	totalResults := 0

	for _, r := range results {
		if r.Success {
			successCount++
			totalResults += r.ResultCount
			if r.FromCache {
				fromCacheCount++
			}
		} else {
			failureCount++
		}
	}

	w.logger.Info("Cache warming completed", map[string]interface{}{
		"total_queries":    len(queries),
		"successful":       successCount,
		"failed":           failureCount,
		"from_cache":       fromCacheCount,
		"total_results":    totalResults,
		"duration_seconds": time.Since(startTime).Seconds(),
	})

	return results, nil
}

// warmSingleQuery warms a single query
func (w *CacheWarmer) warmSingleQuery(ctx context.Context, wq WarmupQuery) WarmupResult {
	startTime := time.Now()

	// Check if already cached
	entry, err := w.cache.Get(ctx, wq.Query, wq.Embedding)
	if err == nil && entry != nil {
		return WarmupResult{
			Query:       wq.Query,
			Success:     true,
			ResultCount: len(entry.Results),
			Duration:    time.Since(startTime),
			FromCache:   true,
		}
	}

	// Execute search
	results, err := w.searchExecutor(ctx, wq.Query)
	if err != nil {
		w.logger.Error("Failed to execute warmup query", map[string]interface{}{
			"query": wq.Query,
			"error": err.Error(),
		})
		return WarmupResult{
			Query:    wq.Query,
			Success:  false,
			Error:    err,
			Duration: time.Since(startTime),
		}
	}

	// Cache results
	err = w.cache.Set(ctx, wq.Query, wq.Embedding, results)
	if err != nil {
		w.logger.Error("Failed to cache warmup results", map[string]interface{}{
			"query": wq.Query,
			"error": err.Error(),
		})
		// Don't fail the warmup if caching fails
	}

	return WarmupResult{
		Query:       wq.Query,
		Success:     true,
		ResultCount: len(results),
		Duration:    time.Since(startTime),
		FromCache:   false,
	}
}

// ScheduledWarmer runs cache warming on a schedule
type ScheduledWarmer struct {
	warmer   *CacheWarmer
	queries  []WarmupQuery
	interval time.Duration
	logger   observability.Logger
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewScheduledWarmer creates a scheduled cache warmer
func NewScheduledWarmer(
	warmer *CacheWarmer,
	queries []WarmupQuery,
	interval time.Duration,
	logger observability.Logger,
) *ScheduledWarmer {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.scheduled_warmer")
	}

	return &ScheduledWarmer{
		warmer:   warmer,
		queries:  queries,
		interval: interval,
		logger:   logger,
		stopCh:   make(chan struct{}),
	}
}

// Start begins the scheduled warming process
func (sw *ScheduledWarmer) Start(ctx context.Context) {
	sw.wg.Add(1)
	go func() {
		defer sw.wg.Done()

		// Run immediately
		sw.runWarmup(ctx)

		ticker := time.NewTicker(sw.interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				sw.runWarmup(ctx)
			case <-ctx.Done():
				sw.logger.Info("Scheduled warmer stopped due to context cancellation", map[string]interface{}{})
				return
			case <-sw.stopCh:
				sw.logger.Info("Scheduled warmer stopped", map[string]interface{}{})
				return
			}
		}
	}()
}

// Stop stops the scheduled warming
func (sw *ScheduledWarmer) Stop() {
	close(sw.stopCh)
	sw.wg.Wait()
}

// runWarmup executes a warmup cycle
func (sw *ScheduledWarmer) runWarmup(ctx context.Context) {
	defer func() {
		if r := recover(); r != nil {
			sw.logger.Error("Panic in scheduled warmup", map[string]interface{}{
				"panic": r,
				"stack": string(debug.Stack()),
			})
		}
	}()

	sw.logger.Info("Starting scheduled cache warming", map[string]interface{}{
		"query_count": len(sw.queries),
	})

	// Create a timeout context for the warmup
	warmupCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	results, err := sw.warmer.WarmWithPriority(warmupCtx, sw.queries)
	if err != nil {
		sw.logger.Error("Scheduled warmup failed", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Log any individual failures
	for _, r := range results {
		if !r.Success && r.Error != nil {
			sw.logger.Warn("Failed to warm query", map[string]interface{}{
				"query": r.Query,
				"error": r.Error.Error(),
			})
		}
	}
}

// LoadWarmupQueries loads warmup queries from various sources
func LoadWarmupQueries(ctx context.Context, sources ...WarmupQuerySource) ([]WarmupQuery, error) {
	var allQueries []WarmupQuery

	for _, source := range sources {
		queries, err := source.LoadQueries(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to load from source %s: %w", source.Name(), err)
		}
		allQueries = append(allQueries, queries...)
	}

	// Deduplicate queries
	seen := make(map[string]bool)
	unique := make([]WarmupQuery, 0, len(allQueries))

	for _, q := range allQueries {
		if !seen[q.Query] {
			seen[q.Query] = true
			unique = append(unique, q)
		}
	}

	return unique, nil
}

// WarmupQuerySource represents a source of warmup queries
type WarmupQuerySource interface {
	Name() string
	LoadQueries(ctx context.Context) ([]WarmupQuery, error)
}

// StaticWarmupSource provides static warmup queries
type StaticWarmupSource struct {
	name    string
	queries []WarmupQuery
}

// NewStaticWarmupSource creates a static warmup source
func NewStaticWarmupSource(name string, queries []string) *StaticWarmupSource {
	wq := make([]WarmupQuery, len(queries))
	for i, q := range queries {
		wq[i] = WarmupQuery{Query: q, Priority: 0}
	}

	return &StaticWarmupSource{
		name:    name,
		queries: wq,
	}
}

func (s *StaticWarmupSource) Name() string {
	return s.name
}

func (s *StaticWarmupSource) LoadQueries(ctx context.Context) ([]WarmupQuery, error) {
	return s.queries, nil
}
