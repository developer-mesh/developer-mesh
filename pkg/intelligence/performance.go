package intelligence

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/redis/go-redis/v9"
)

// PerformanceOptimizer manages caching, batching, and connection pooling
type PerformanceOptimizer struct {
	// Caching layers
	l1Cache     *lru.Cache[string, CachedItem] // In-memory LRU cache
	l2Cache     *redis.Client                  // Redis distributed cache
	cacheConfig CacheConfig

	// Batching
	batchProcessor *BatchProcessor
	batchQueue     chan BatchItem

	// Connection pools
	dbPool    *sql.DB
	redisPool *redis.Client
	httpPool  *HTTPConnectionPool

	// Prefetching
	prefetcher    *Prefetcher
	prefetchQueue chan PrefetchRequest

	// Metrics
	cacheHits   uint64
	cacheMisses uint64
	batchCount  uint64
	// poolMetrics sync.Map // Reserved for future use

	// Configuration
	config PerformanceConfig
	logger Logger
}

// PerformanceConfig contains performance optimization settings
type PerformanceConfig struct {
	// Cache settings
	L1CacheSize int
	L2CacheTTL  time.Duration
	CacheWarmup bool

	// Batching settings
	BatchSize    int
	BatchTimeout time.Duration
	MaxBatchWait time.Duration

	// Connection pool settings
	DBMaxConns     int
	DBMaxIdleConns int
	RedisPoolSize  int
	HTTPMaxConns   int

	// Prefetch settings
	PrefetchEnabled   bool
	PrefetchThreshold float64 // 0.8 = prefetch when 80% likely
	PrefetchWorkers   int
}

// CacheConfig defines cache behavior
type CacheConfig struct {
	EnableL1         bool
	EnableL2         bool
	TTL              time.Duration
	MaxSize          int
	EvictionPolicy   string // "lru", "lfu", "arc"
	CompressionLevel int    // 0-9
}

// NewPerformanceOptimizer creates a performance optimization layer
func NewPerformanceOptimizer(config PerformanceConfig, deps OptimizationDependencies) (*PerformanceOptimizer, error) {
	// Create L1 cache
	l1Cache, err := lru.New[string, CachedItem](config.L1CacheSize)
	if err != nil {
		return nil, fmt.Errorf("failed to create L1 cache: %w", err)
	}

	// Create batch processor
	batchProcessor := NewBatchProcessor(BatchProcessorConfig{
		BatchSize:    config.BatchSize,
		BatchTimeout: config.BatchTimeout,
		MaxWait:      config.MaxBatchWait,
		Logger:       deps.Logger,
	})

	// Create prefetcher
	prefetcher := NewPrefetcher(PrefetcherConfig{
		Workers:   config.PrefetchWorkers,
		Threshold: config.PrefetchThreshold,
		Logger:    deps.Logger,
	})

	optimizer := &PerformanceOptimizer{
		l1Cache:        l1Cache,
		l2Cache:        deps.RedisClient,
		cacheConfig:    deps.CacheConfig,
		batchProcessor: batchProcessor,
		batchQueue:     make(chan BatchItem, config.BatchSize*10),
		dbPool:         deps.DBPool,
		redisPool:      deps.RedisClient,
		httpPool:       NewHTTPConnectionPool(config.HTTPMaxConns),
		prefetcher:     prefetcher,
		prefetchQueue:  make(chan PrefetchRequest, 100),
		config:         config,
		logger:         deps.Logger,
	}

	// Start background workers
	optimizer.startWorkers()

	// Warm up cache if enabled
	if config.CacheWarmup {
		go optimizer.warmupCache(context.Background())
	}

	return optimizer, nil
}

// GetWithCache retrieves data with multi-level caching
func (p *PerformanceOptimizer) GetWithCache(ctx context.Context, key string, loader func() (interface{}, error)) (interface{}, error) {
	// Check L1 cache
	if p.cacheConfig.EnableL1 {
		if item, ok := p.l1Cache.Get(key); ok {
			if !item.IsExpired() {
				p.recordCacheHit("l1")
				return item.Value, nil
			}
			p.l1Cache.Remove(key)
		}
	}

	// Check L2 cache
	if p.cacheConfig.EnableL2 {
		cacheKey := p.buildCacheKey(key)
		val, err := p.l2Cache.Get(ctx, cacheKey).Result()
		if err == nil {
			var item CachedItem
			if err := json.Unmarshal([]byte(val), &item); err == nil {
				if !item.IsExpired() {
					// Promote to L1
					if p.cacheConfig.EnableL1 {
						p.l1Cache.Add(key, item)
					}
					p.recordCacheHit("l2")
					return item.Value, nil
				}
			}
			// Remove expired item
			p.l2Cache.Del(ctx, cacheKey)
		}
	}

	p.recordCacheMiss()

	// Load data
	value, err := loader()
	if err != nil {
		return nil, err
	}

	// Cache the result
	item := CachedItem{
		Value:      value,
		Expiration: time.Now().Add(p.cacheConfig.TTL),
		Version:    1,
		Metadata: map[string]interface{}{
			"created": time.Now(),
			"key":     key,
		},
	}

	// Add to L1
	if p.cacheConfig.EnableL1 {
		p.l1Cache.Add(key, item)
	}

	// Add to L2
	if p.cacheConfig.EnableL2 {
		go p.setL2Cache(context.Background(), key, item)
	}

	// Trigger prefetch analysis
	if p.config.PrefetchEnabled {
		p.analyzePrefetchOpportunity(key, value)
	}

	return value, nil
}

// BatchExecute processes operations in batches
func (p *PerformanceOptimizer) BatchExecute(ctx context.Context, items []BatchItem) ([]BatchResult, error) {
	if len(items) <= p.config.BatchSize {
		// Single batch
		return p.batchProcessor.ProcessBatch(ctx, items)
	}

	// Split into multiple batches
	var results []BatchResult
	var wg sync.WaitGroup
	resultsChan := make(chan []BatchResult, len(items)/p.config.BatchSize+1)
	errorsChan := make(chan error, len(items)/p.config.BatchSize+1)

	for i := 0; i < len(items); i += p.config.BatchSize {
		end := i + p.config.BatchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		wg.Add(1)

		go func(b []BatchItem) {
			defer wg.Done()

			batchResults, err := p.batchProcessor.ProcessBatch(ctx, b)
			if err != nil {
				errorsChan <- err
				return
			}
			resultsChan <- batchResults
		}(batch)
	}

	// Wait for all batches
	wg.Wait()
	close(resultsChan)
	close(errorsChan)

	// Check for errors
	if err := <-errorsChan; err != nil {
		return nil, fmt.Errorf("batch execution failed: %w", err)
	}

	// Collect results
	for batchResults := range resultsChan {
		results = append(results, batchResults...)
	}

	p.recordBatchExecution(len(items))
	return results, nil
}

// OptimizeQuery optimizes database queries with caching and pooling
func (p *PerformanceOptimizer) OptimizeQuery(ctx context.Context, query QueryRequest) (*QueryResult, error) {
	// Check if query can be cached
	if query.Cacheable {
		cacheKey := p.buildQueryCacheKey(query)
		if cached, err := p.GetWithCache(ctx, cacheKey, func() (interface{}, error) {
			return p.executeQuery(ctx, query)
		}); err == nil {
			return cached.(*QueryResult), nil
		}
	}

	// Execute with connection from pool
	return p.executeQuery(ctx, query)
}

// executeQuery executes a database query with connection pooling
func (p *PerformanceOptimizer) executeQuery(ctx context.Context, query QueryRequest) (*QueryResult, error) {
	// Get connection from pool
	conn, err := p.dbPool.Conn(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}
	defer func() {
		if err := conn.Close(); err != nil {
			p.logger.Warn("Failed to close connection", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Prepare statement if frequently used
	if query.Prepared {
		stmt, err := conn.PrepareContext(ctx, query.SQL)
		if err != nil {
			return nil, fmt.Errorf("failed to prepare statement: %w", err)
		}
		defer func() {
			if err := stmt.Close(); err != nil {
				p.logger.Warn("Failed to close statement", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()

		rows, err := stmt.QueryContext(ctx, query.Args...)
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		defer func() {
			if err := rows.Close(); err != nil {
				p.logger.Warn("Failed to close rows", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}()

		return p.scanRows(rows)
	}

	// Regular query
	rows, err := conn.QueryContext(ctx, query.SQL, query.Args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			p.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return p.scanRows(rows)
}

// scanRows scans database rows into result
func (p *PerformanceOptimizer) scanRows(rows *sql.Rows) (*QueryResult, error) {
	var results []map[string]interface{}

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			row[col] = values[i]
		}
		results = append(results, row)
	}

	return &QueryResult{
		Rows:     results,
		RowCount: len(results),
	}, nil
}

// warmupCache pre-loads frequently accessed data
func (p *PerformanceOptimizer) warmupCache(ctx context.Context) {
	p.logger.Info("Starting cache warmup", map[string]interface{}{})

	// Load frequent patterns
	patterns, err := p.loadFrequentPatterns(ctx)
	if err != nil {
		p.logger.Error("Failed to load patterns for warmup", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Pre-load each pattern
	for _, pattern := range patterns {
		key := pattern.Key
		loader := p.getLoaderForPattern(pattern)

		if _, err := p.GetWithCache(ctx, key, loader); err != nil {
			p.logger.Warn("Failed to warmup cache entry", map[string]interface{}{
				"key":   key,
				"error": err.Error(),
			})
		}
	}

	p.logger.Info("Cache warmup completed", map[string]interface{}{
		"entries": len(patterns),
	})
}

// analyzePrefetchOpportunity determines if related data should be prefetched
func (p *PerformanceOptimizer) analyzePrefetchOpportunity(key string, value interface{}) {
	// Analyze access patterns
	pattern := p.analyzeAccessPattern(key)

	// Check if prefetch threshold is met
	if pattern.Probability >= p.config.PrefetchThreshold {
		// Queue prefetch request
		select {
		case p.prefetchQueue <- PrefetchRequest{
			Key:         key,
			RelatedKeys: pattern.RelatedKeys,
			Priority:    pattern.Priority,
		}:
		default:
			// Queue full, skip prefetch
		}
	}
}

// startWorkers starts background optimization workers
func (p *PerformanceOptimizer) startWorkers() {
	// Batch processing worker
	go p.batchWorker()

	// Prefetch worker
	if p.config.PrefetchEnabled {
		for i := 0; i < p.config.PrefetchWorkers; i++ {
			go p.prefetchWorker()
		}
	}

	// Metrics collector
	go p.metricsWorker()
}

// batchWorker processes batched operations
func (p *PerformanceOptimizer) batchWorker() {
	ticker := time.NewTicker(p.config.BatchTimeout)
	defer ticker.Stop()

	var batch []BatchItem

	for {
		select {
		case item := <-p.batchQueue:
			batch = append(batch, item)

			if len(batch) >= p.config.BatchSize {
				p.processBatch(batch)
				batch = nil
			}

		case <-ticker.C:
			if len(batch) > 0 {
				p.processBatch(batch)
				batch = nil
			}
		}
	}
}

// processBatch processes a batch of items
func (p *PerformanceOptimizer) processBatch(batch []BatchItem) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if _, err := p.batchProcessor.ProcessBatch(ctx, batch); err != nil {
		p.logger.Error("Batch processing failed", map[string]interface{}{
			"size":  len(batch),
			"error": err.Error(),
		})
	}
}

// prefetchWorker processes prefetch requests
func (p *PerformanceOptimizer) prefetchWorker() {
	for req := range p.prefetchQueue {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)

		for _, key := range req.RelatedKeys {
			// Check if already cached
			if p.isCached(key) {
				continue
			}

			// Prefetch the data
			loader := p.getDefaultLoader(key)
			if _, err := p.GetWithCache(ctx, key, loader); err != nil {
				p.logger.Debug("Prefetch failed", map[string]interface{}{
					"key":   key,
					"error": err.Error(),
				})
			}
		}

		cancel()
	}
}

// metricsWorker collects and reports performance metrics
func (p *PerformanceOptimizer) metricsWorker() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		p.reportMetrics()
	}
}

// Helper methods

func (p *PerformanceOptimizer) buildCacheKey(key string) string {
	return fmt.Sprintf("perf:cache:%s", key)
}

func (p *PerformanceOptimizer) buildQueryCacheKey(query QueryRequest) string {
	h := sha256.New()
	_, _ = fmt.Fprintf(h, "%s:%v", query.SQL, query.Args)
	hash := hex.EncodeToString(h.Sum(nil))
	return fmt.Sprintf("query:%s", hash)
}

func (p *PerformanceOptimizer) setL2Cache(ctx context.Context, key string, item CachedItem) {
	data, err := json.Marshal(item)
	if err != nil {
		p.logger.Error("Failed to marshal cache item", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	cacheKey := p.buildCacheKey(key)
	if err := p.l2Cache.Set(ctx, cacheKey, data, p.config.L2CacheTTL).Err(); err != nil {
		p.logger.Error("Failed to set L2 cache", map[string]interface{}{
			"error": err.Error(),
		})
	}
}

func (p *PerformanceOptimizer) isCached(key string) bool {
	// Check L1
	if p.cacheConfig.EnableL1 {
		if _, ok := p.l1Cache.Get(key); ok {
			return true
		}
	}

	// Check L2
	if p.cacheConfig.EnableL2 {
		ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
		defer cancel()

		cacheKey := p.buildCacheKey(key)
		if exists, _ := p.l2Cache.Exists(ctx, cacheKey).Result(); exists > 0 {
			return true
		}
	}

	return false
}

func (p *PerformanceOptimizer) recordCacheHit(level string) {
	atomic.AddUint64(&p.cacheHits, 1)
	p.logger.Debug("Cache hit", map[string]interface{}{
		"level": level,
	})
}

func (p *PerformanceOptimizer) recordCacheMiss() {
	atomic.AddUint64(&p.cacheMisses, 1)
}

func (p *PerformanceOptimizer) recordBatchExecution(size int) {
	atomic.AddUint64(&p.batchCount, 1)
	p.logger.Debug("Batch executed", map[string]interface{}{
		"size": size,
	})
}

func (p *PerformanceOptimizer) reportMetrics() {
	hits := atomic.LoadUint64(&p.cacheHits)
	misses := atomic.LoadUint64(&p.cacheMisses)
	batches := atomic.LoadUint64(&p.batchCount)

	hitRate := float64(0)
	if total := hits + misses; total > 0 {
		hitRate = float64(hits) / float64(total) * 100
	}

	p.logger.Info("Performance metrics", map[string]interface{}{
		"cache_hits":   hits,
		"cache_misses": misses,
		"hit_rate":     hitRate,
		"batches":      batches,
		"l1_size":      p.l1Cache.Len(),
	})
}

func (p *PerformanceOptimizer) loadFrequentPatterns(ctx context.Context) ([]AccessPattern, error) {
	// Load from database or configuration
	// This would typically query historical access patterns
	return []AccessPattern{}, nil
}

func (p *PerformanceOptimizer) getLoaderForPattern(pattern AccessPattern) func() (interface{}, error) {
	return func() (interface{}, error) {
		// Pattern-specific loading logic
		return nil, nil
	}
}

func (p *PerformanceOptimizer) getDefaultLoader(key string) func() (interface{}, error) {
	return func() (interface{}, error) {
		// Default loading logic
		return nil, nil
	}
}

func (p *PerformanceOptimizer) analyzeAccessPattern(key string) AccessPattern {
	// Analyze historical access patterns
	// This would use ML or statistical analysis
	return AccessPattern{
		Key:         key,
		Probability: 0.5,
		Priority:    1,
	}
}

// Supporting types

type CachedItem struct {
	Value      interface{}
	Expiration time.Time
	Version    int
	Metadata   map[string]interface{}
}

func (c CachedItem) IsExpired() bool {
	return time.Now().After(c.Expiration)
}

type BatchItem struct {
	ID        uuid.UUID
	Operation string
	Data      interface{}
	Callback  func(result interface{}, err error)
}

type BatchResult struct {
	ID     uuid.UUID
	Result interface{}
	Error  error
}

type QueryRequest struct {
	SQL       string
	Args      []interface{}
	Cacheable bool
	Prepared  bool
	Timeout   time.Duration
}

type QueryResult struct {
	Rows     []map[string]interface{}
	RowCount int
}

type PrefetchRequest struct {
	Key         string
	RelatedKeys []string
	Priority    int
}

type AccessPattern struct {
	Key         string
	RelatedKeys []string
	Probability float64
	Priority    int
}

type HTTPConnectionPool struct {
	maxConns int
	conns    chan struct{}
}

func NewHTTPConnectionPool(maxConns int) *HTTPConnectionPool {
	pool := &HTTPConnectionPool{
		maxConns: maxConns,
		conns:    make(chan struct{}, maxConns),
	}

	// Pre-fill the pool
	for i := 0; i < maxConns; i++ {
		pool.conns <- struct{}{}
	}

	return pool
}

func (p *HTTPConnectionPool) Acquire() {
	<-p.conns
}

func (p *HTTPConnectionPool) Release() {
	p.conns <- struct{}{}
}
