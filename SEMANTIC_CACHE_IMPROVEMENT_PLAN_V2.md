# Semantic Cache Improvement Plan - Version 2

## Executive Summary
This plan implements a production-ready semantic cache with full tenant isolation from day one. Since Developer Mesh is a greenfield project with no existing users, we implement the ideal architecture directly without any migration complexity or backward compatibility concerns.

## Timeline Overview
- **Phase 1 (Immediate)**: 2-3 days - Critical fixes with proper package integration
- **Phase 2 (Short-term)**: 1 week - Production readiness using existing patterns
- **Phase 3 (Medium-term)**: 1 week - Testing and performance optimization
- **Phase 4 (Long-term)**: 2 weeks - Advanced features with security

## Phase 1: Immediate Critical Fixes (2-3 days)

### 1.1 Fix Race Condition in updateAccessStats
**Priority**: CRITICAL
**Time**: 4 hours

#### Solution Using Project Patterns
```go
// pkg/embedding/cache/semantic_cache.go
import (
    "sync"
    "sync/atomic"
)

type SemanticCache struct {
    // Use sync.Map for concurrent access (project pattern)
    entries     sync.Map // map[string]*CacheEntry
    
    // Atomic counters for stats
    hitCount    atomic.Int64
    missCount   atomic.Int64
    
    // ... other fields
}

// Thread-safe update using copy-on-write pattern
func (c *SemanticCache) updateAccessStats(ctx context.Context, key string, entry *CacheEntry) (*CacheEntry, error) {
    // Create a copy to avoid race conditions
    updatedEntry := &CacheEntry{
        Query:           entry.Query,
        NormalizedQuery: entry.NormalizedQuery,
        Embedding:       entry.Embedding,
        Results:         entry.Results,
        CreatedAt:       entry.CreatedAt,
        LastAccessedAt:  time.Now(),
        HitCount:        entry.HitCount + 1,
        TTL:             entry.TTL,
    }
    
    // Update in Redis atomically
    data, err := json.Marshal(updatedEntry)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal entry: %w", err)
    }
    
    // Use resilient client with retry
    err = c.redis.Execute(ctx, func() (interface{}, error) {
        return nil, c.redis.GetClient().Set(ctx, key, data, entry.TTL).Err()
    })
    
    if err != nil {
        return nil, fmt.Errorf("failed to update access stats: %w", err)
    }
    
    // Update local cache
    c.entries.Store(key, updatedEntry)
    
    return updatedEntry, nil
}
```

### 1.2 Add Input Validation with Security Integration
**Priority**: CRITICAL
**Time**: 6 hours

#### Implementation Using Existing Packages
```go
// pkg/embedding/cache/validator.go
package cache

import (
    "context"
    "errors"
    "fmt"
    "regexp"
    "strings"
    "unicode/utf8"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/middleware"
)

var (
    // Use project error patterns
    ErrQueryTooLong      = errors.New("query exceeds maximum length")
    ErrInvalidCharacters = errors.New("query contains invalid characters")
    ErrEmptyQuery        = errors.New("query cannot be empty")
    ErrNoTenantID        = errors.New("no tenant ID in context")
    ErrRateLimitExceeded = errors.New("rate limit exceeded")
)

type QueryValidator struct {
    maxLength       int
    allowedPattern  *regexp.Regexp
    sanitizePattern *regexp.Regexp
    rateLimiter     *middleware.RateLimiter
    logger          observability.Logger
}

func NewQueryValidator(rateLimiter *middleware.RateLimiter, logger observability.Logger) *QueryValidator {
    return &QueryValidator{
        maxLength:       1000,
        // Use project's standard validation pattern
        allowedPattern:  regexp.MustCompile(`^[\p{L}\p{N}\s\-_.,!?'"@#$%^&*()+=/\\<>{}[\]|~` + "`" + `]+$`),
        sanitizePattern: regexp.MustCompile(`[^\p{L}\p{N}\s\-_.,!?'"@#$%^&*()+=/\\<>{}[\]|~` + "`" + `]+`),
        rateLimiter:     rateLimiter,
        logger:          logger,
    }
}

func (v *QueryValidator) ValidateWithContext(ctx context.Context, query string) error {
    // Extract tenant ID using auth package
    tenantID := auth.GetTenantID(ctx)
    if tenantID == uuid.Nil {
        return ErrNoTenantID
    }
    
    // Apply rate limiting
    limiterKey := fmt.Sprintf("cache:validate:%s", tenantID.String())
    if !v.rateLimiter.Allow(limiterKey) {
        v.logger.Warn("Query validation rate limited", map[string]interface{}{
            "tenant_id": tenantID.String(),
        })
        return ErrRateLimitExceeded
    }
    
    // Validate query
    if query == "" {
        return ErrEmptyQuery
    }
    
    if !utf8.ValidString(query) {
        return ErrInvalidCharacters
    }
    
    if len(query) > v.maxLength {
        return ErrQueryTooLong
    }
    
    if !v.allowedPattern.MatchString(query) {
        return ErrInvalidCharacters
    }
    
    return nil
}

// SanitizeRedisKey ensures Redis key safety following project patterns
func SanitizeRedisKey(key string) string {
    // Use project's standard key sanitization
    replacer := strings.NewReplacer(
        " ", "_",
        ":", "-",
        "*", "-",
        "?", "-",
        "[", "-",
        "]", "-",
        "{", "-",
        "}", "-",
        "\\", "-",
        "\n", "-",
        "\r", "-",
        "\x00", "-",
    )
    return replacer.Replace(key)
}
```

### 1.3 Fix Sorting with Efficient Algorithms
**Priority**: HIGH
**Time**: 2 hours

```go
// pkg/embedding/cache/stats.go
import (
    "sort"
    "container/heap"
)

// GetTopQueries using efficient heap for top-K selection
func (c *SemanticCache) GetTopQueries(ctx context.Context, limit int) ([]*CacheEntry, error) {
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
```

### 1.4 Add Panic Recovery Following Project Patterns
**Priority**: HIGH
**Time**: 2 hours

```go
// pkg/embedding/cache/recovery.go
package cache

import (
    "runtime/debug"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RecoverMiddleware wraps functions with panic recovery
func RecoverMiddleware(logger observability.Logger, operation string) func(func()) {
    return func(fn func()) {
        defer func() {
            if r := recover(); r != nil {
                logger.Error("Panic recovered", map[string]interface{}{
                    "operation": operation,
                    "panic":     r,
                    "stack":     string(debug.Stack()),
                })
                
                // Record metric
                metrics.IncrementCounterWithLabels("cache.panic_recovered", 1, map[string]string{
                    "operation": operation,
                })
            }
        }()
        
        fn()
    }
}

// Usage in semantic_cache.go
func (c *SemanticCache) evictIfNecessary(ctx context.Context) {
    RecoverMiddleware(c.logger, "evict_if_necessary")(func() {
        // Existing eviction logic
    })
}
```

## Phase 2: Production Readiness (1 week)

### 2.1 Implement Resilient Redis Client with Circuit Breaker
**Time**: 1 day

```go
// pkg/embedding/cache/redis_client.go
package cache

import (
    "context"
    "time"
    
    "github.com/go-redis/redis/v8"
    "github.com/developer-mesh/developer-mesh/pkg/resilience"
    "github.com/developer-mesh/developer-mesh/pkg/retry"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

type ResilientRedisClient struct {
    client         *redis.Client
    circuitBreaker *resilience.CircuitBreaker
    retryPolicy    retry.Policy
    logger         observability.Logger
    metrics        observability.MetricsClient
}

func NewResilientRedisClient(
    client *redis.Client,
    logger observability.Logger,
    metrics observability.MetricsClient,
) *ResilientRedisClient {
    // Use project's circuit breaker
    cbConfig := resilience.CircuitBreakerConfig{
        FailureThreshold:    5,
        FailureRatio:        0.6,
        ResetTimeout:        30 * time.Second,
        SuccessThreshold:    2,
        TimeoutThreshold:    5 * time.Second,
        MaxRequestsHalfOpen: 5,
    }
    
    // Use project's retry policy
    retryConfig := retry.Config{
        InitialInterval: 100 * time.Millisecond,
        MaxInterval:     5 * time.Second,
        MaxRetries:      3,
        Multiplier:      2.0,
    }
    
    return &ResilientRedisClient{
        client:         client,
        circuitBreaker: resilience.NewCircuitBreaker("redis_cache", cbConfig, logger, metrics),
        retryPolicy:    retry.NewExponentialBackoff(retryConfig),
        logger:         logger,
        metrics:        metrics,
    }
}

// Execute wraps Redis operations with circuit breaker and retry
func (r *ResilientRedisClient) Execute(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
    return r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
        var result interface{}
        err := r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
            var opErr error
            result, opErr = operation()
            return opErr
        })
        return result, err
    })
}

// Get with resilience
func (r *ResilientRedisClient) Get(ctx context.Context, key string) (string, error) {
    result, err := r.Execute(ctx, func() (interface{}, error) {
        return r.client.Get(ctx, key).Result()
    })
    
    if err != nil {
        return "", err
    }
    
    return result.(string), nil
}

// Set with resilience
func (r *ResilientRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
    _, err := r.Execute(ctx, func() (interface{}, error) {
        return nil, r.client.Set(ctx, key, value, expiration).Err()
    })
    return err
}

// GetClient returns the underlying Redis client for advanced operations
func (r *ResilientRedisClient) GetClient() *redis.Client {
    return r.client
}

// Close gracefully shuts down the client
func (r *ResilientRedisClient) Close() error {
    return r.client.Close()
}
```

### 2.2 Implement Tenant Isolation with UUID Support
**Time**: 1 day

```go
// pkg/embedding/cache/tenant_cache.go
package cache

import (
    "context"
    "fmt"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/middleware"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
    "github.com/developer-mesh/developer-mesh/pkg/security"
)

type TenantAwareCache struct {
    baseCache          *SemanticCache
    tenantConfigRepo   repository.TenantConfigRepository
    rateLimiter        *middleware.RateLimiter
    encryptionService  *security.EncryptionService
    logger             observability.Logger
    metrics            observability.MetricsClient
}

func NewTenantAwareCache(
    baseCache *SemanticCache,
    configRepo repository.TenantConfigRepository,
    rateLimiter *middleware.RateLimiter,
    encryptionKey string,
    logger observability.Logger,
    metrics observability.MetricsClient,
) *TenantAwareCache {
    return &TenantAwareCache{
        baseCache:          baseCache,
        tenantConfigRepo:   configRepo,
        rateLimiter:        rateLimiter,
        encryptionService:  security.NewEncryptionService(encryptionKey),
        logger:             logger,
        metrics:            metrics,
    }
}

// Get retrieves from cache with tenant isolation
func (tc *TenantAwareCache) Get(ctx context.Context, query string, embedding []float32) (*CacheEntry, error) {
    // Extract tenant ID using auth package
    tenantID := auth.GetTenantID(ctx)
    if tenantID == uuid.Nil {
        return nil, ErrNoTenantID
    }
    
    // Apply rate limiting
    limiterKey := fmt.Sprintf("cache:%s", tenantID.String())
    if !tc.rateLimiter.Allow(limiterKey) {
        tc.metrics.IncrementCounterWithLabels("cache.rate_limited", 1, map[string]string{
            "tenant_id": tenantID.String(),
        })
        return nil, ErrRateLimitExceeded
    }
    
    // Get tenant config
    config, err := tc.getTenantConfig(ctx, tenantID)
    if err != nil {
        return nil, fmt.Errorf("failed to get tenant config: %w", err)
    }
    
    // Check if cache is enabled for tenant
    if !config.IsFeatureEnabled("semantic_cache") {
        return nil, ErrFeatureDisabled
    }
    
    // Build tenant-specific key
    key := tc.getCacheKey(tenantID, query)
    
    // Get from cache
    entry, err := tc.baseCache.getWithTenantKey(ctx, key, query, embedding)
    if err != nil {
        return nil, err
    }
    
    // Decrypt sensitive data if needed
    if entry != nil && len(entry.EncryptedData) > 0 {
        decrypted, err := tc.encryptionService.DecryptCredential(entry.EncryptedData, tenantID.String())
        if err != nil {
            tc.logger.Error("Failed to decrypt cache entry", map[string]interface{}{
                "error":     err.Error(),
                "tenant_id": tenantID.String(),
            })
            return nil, err
        }
        entry.DecryptedData = decrypted
    }
    
    return entry, nil
}

// Set stores in cache with tenant isolation and encryption
func (tc *TenantAwareCache) Set(ctx context.Context, query string, embedding []float32, results []CachedSearchResult) error {
    tenantID := auth.GetTenantID(ctx)
    if tenantID == uuid.Nil {
        return ErrNoTenantID
    }
    
    // Check if results contain sensitive data
    sensitiveData := extractSensitiveData(results)
    var encryptedData []byte
    
    if sensitiveData != nil {
        encrypted, err := tc.encryptionService.EncryptJSON(sensitiveData, tenantID.String())
        if err != nil {
            return fmt.Errorf("failed to encrypt sensitive data: %w", err)
        }
        encryptedData = []byte(encrypted)
    }
    
    key := tc.getCacheKey(tenantID, query)
    return tc.baseCache.setWithEncryption(ctx, key, query, embedding, results, encryptedData)
}

// getCacheKey generates tenant-isolated Redis key
func (tc *TenantAwareCache) getCacheKey(tenantID uuid.UUID, query string) string {
    normalized := tc.baseCache.normalizer.Normalize(query)
    sanitized := SanitizeRedisKey(normalized)
    
    // Use Redis hash tags for cluster support
    return fmt.Sprintf("%s:{%s}:q:%s", 
        tc.baseCache.config.Prefix,
        tenantID.String(),
        sanitized)
}

// getTenantConfig retrieves tenant configuration with caching
func (tc *TenantAwareCache) getTenantConfig(ctx context.Context, tenantID uuid.UUID) (*models.TenantConfig, error) {
    // Try local cache first
    cacheKey := fmt.Sprintf("tenant_config:%s", tenantID.String())
    if cached, found := tc.baseCache.localCache.Get(cacheKey); found {
        return cached.(*models.TenantConfig), nil
    }
    
    // Load from repository
    config, err := tc.tenantConfigRepo.GetByTenantID(ctx, tenantID.String())
    if err != nil {
        return nil, err
    }
    
    // Cache for 5 minutes
    tc.baseCache.localCache.Set(cacheKey, config, 5*time.Minute)
    
    return config, nil
}
```

### 2.3 Implement Graceful Shutdown
**Time**: 4 hours

```go
// pkg/embedding/cache/lifecycle.go
package cache

import (
    "context"
    "sync"
    "time"
)

// Lifecycle manages cache lifecycle operations
type Lifecycle struct {
    cache      *SemanticCache
    warmers    []*CacheWarmer
    shutdownMu sync.Mutex
    shutdown   bool
    wg         sync.WaitGroup
    logger     observability.Logger
}

func NewLifecycle(cache *SemanticCache, logger observability.Logger) *Lifecycle {
    return &Lifecycle{
        cache:  cache,
        logger: logger,
    }
}

// Start initializes cache operations
func (l *Lifecycle) Start(ctx context.Context) error {
    l.logger.Info("Starting semantic cache", map[string]interface{}{})
    
    // Start background operations
    l.wg.Add(1)
    go l.metricsExporter(ctx)
    
    // Start cache warmers
    for _, warmer := range l.warmers {
        l.wg.Add(1)
        go func(w *CacheWarmer) {
            defer l.wg.Done()
            w.Start(ctx)
        }(warmer)
    }
    
    return nil
}

// Shutdown gracefully stops cache operations
func (l *Lifecycle) Shutdown(ctx context.Context) error {
    l.shutdownMu.Lock()
    if l.shutdown {
        l.shutdownMu.Unlock()
        return nil
    }
    l.shutdown = true
    l.shutdownMu.Unlock()
    
    l.logger.Info("Shutting down semantic cache", map[string]interface{}{})
    
    // Create shutdown context with timeout
    shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    
    // Stop accepting new operations
    l.cache.mu.Lock()
    l.cache.shuttingDown = true
    l.cache.mu.Unlock()
    
    // Wait for ongoing operations
    done := make(chan struct{})
    go func() {
        l.wg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        l.logger.Info("All cache operations completed", map[string]interface{}{})
    case <-shutdownCtx.Done():
        l.logger.Warn("Shutdown timeout, some operations may be incomplete", map[string]interface{}{})
    }
    
    // Flush metrics
    if l.cache.metrics != nil {
        l.cache.metrics.Flush()
    }
    
    // Close Redis connection
    if l.cache.redis != nil {
        if err := l.cache.redis.Close(); err != nil {
            return fmt.Errorf("error closing Redis connection: %w", err)
        }
    }
    
    l.logger.Info("Semantic cache shutdown complete", map[string]interface{}{})
    return nil
}

// metricsExporter periodically exports cache metrics
func (l *Lifecycle) metricsExporter(ctx context.Context) {
    defer l.wg.Done()
    
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            l.exportMetrics()
        case <-ctx.Done():
            return
        }
    }
}

func (l *Lifecycle) exportMetrics() {
    stats := l.cache.GetStats()
    
    labels := map[string]string{
        "cache_type": "semantic",
    }
    
    l.cache.metrics.SetGaugeWithLabels("cache.entries", float64(stats.TotalEntries), labels)
    l.cache.metrics.SetGaugeWithLabels("cache.hit_rate", stats.HitRate, labels)
    l.cache.metrics.SetGaugeWithLabels("cache.memory_bytes", float64(stats.MemoryUsage), labels)
}
```

## Phase 3: Performance and Testing (1 week)

### 3.1 Add Database Schema (Initial Setup)
**Time**: 4 hours

```sql
-- migrations/001_initial_cache_metadata.up.sql
CREATE TABLE IF NOT EXISTS cache_metadata (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    cache_key VARCHAR(255) NOT NULL,
    query_hash VARCHAR(64) NOT NULL,
    embedding vector(1536),
    hit_count INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_tenant_cache_key UNIQUE(tenant_id, cache_key)
);

-- Indexes for performance
CREATE INDEX idx_cache_metadata_tenant_access ON cache_metadata(tenant_id, last_accessed_at DESC);
CREATE INDEX idx_cache_metadata_hit_count ON cache_metadata(tenant_id, hit_count DESC);
CREATE INDEX idx_cache_metadata_embedding ON cache_metadata USING ivfflat (embedding vector_cosine_ops);

-- Function to update updated_at
CREATE OR REPLACE FUNCTION update_cache_metadata_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Trigger for updated_at
CREATE TRIGGER cache_metadata_updated_at_trigger
BEFORE UPDATE ON cache_metadata
FOR EACH ROW
EXECUTE FUNCTION update_cache_metadata_updated_at();
```

```sql
-- migrations/001_initial_cache_metadata.down.sql
DROP TRIGGER IF EXISTS cache_metadata_updated_at_trigger ON cache_metadata;
DROP FUNCTION IF EXISTS update_cache_metadata_updated_at();
DROP TABLE IF EXISTS cache_metadata;
```

### 3.2 Configuration Integration
**Time**: 2 hours

```yaml
# configs/config.base.yaml additions
cache:
  semantic:
    enabled: true
    # No mode configuration - always tenant isolated
    
    redis:
      prefix: "devmesh:cache"
      ttl: 3600
      max_entries: 10000
      max_memory_mb: 1024
      compression_enabled: true
      
    circuit_breaker:
      failure_threshold: 5
      failure_ratio: 0.6
      reset_timeout: 30s
      max_requests_half_open: 5
      
    retry:
      max_attempts: 3
      initial_interval: 100ms
      max_interval: 5s
      multiplier: 2.0
      
    validation:
      max_query_length: 1000
      rate_limit_rps: 100
      rate_limit_burst: 200
      
    tenant:
      default_max_entries: 1000
      default_ttl: 3600
      encryption_enabled: true
      
    warmup:
      enabled: true
      schedule: "0 */6 * * *"  # Every 6 hours
      batch_size: 100
      concurrent_requests: 5
      
    monitoring:
      metrics_interval: 30s
      slow_query_threshold: 100ms
      
    eviction:
      strategy: "lru"  # lru, lfu, ttl
      check_interval: 300s
      batch_size: 100
```

### 3.3 Comprehensive Test Suite
**Time**: 2 days

```go
// pkg/embedding/cache/semantic_cache_test.go
package cache_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
    "github.com/developer-mesh/developer-mesh/pkg/testutil"
)

func TestTenantIsolation(t *testing.T) {
    // Setup
    redis := testutil.SetupRedis(t)
    defer redis.Close()
    
    cache := setupTestCache(t, redis)
    
    tenant1 := uuid.New()
    tenant2 := uuid.New()
    
    ctx1 := auth.WithTenantID(context.Background(), tenant1)
    ctx2 := auth.WithTenantID(context.Background(), tenant2)
    
    query := "test query"
    embedding := []float32{0.1, 0.2, 0.3}
    results := []cache.CachedSearchResult{{ID: "1", Score: 0.9}}
    
    // Test isolation
    err := cache.Set(ctx1, query, embedding, results)
    require.NoError(t, err)
    
    // Should find in tenant 1
    entry1, err := cache.Get(ctx1, query, embedding)
    require.NoError(t, err)
    require.NotNil(t, entry1)
    
    // Should not find in tenant 2
    entry2, err := cache.Get(ctx2, query, embedding)
    require.Error(t, err)
    require.Nil(t, entry2)
}

func TestRaceConditionSafety(t *testing.T) {
    cache := setupTestCache(t, nil)
    ctx := auth.WithTenantID(context.Background(), uuid.New())
    
    // Run concurrent operations
    done := make(chan bool)
    
    // Writers
    for i := 0; i < 10; i++ {
        go func(id int) {
            query := fmt.Sprintf("query %d", id)
            err := cache.Set(ctx, query, nil, nil)
            assert.NoError(t, err)
            done <- true
        }(i)
    }
    
    // Readers
    for i := 0; i < 10; i++ {
        go func(id int) {
            query := fmt.Sprintf("query %d", id)
            _, _ = cache.Get(ctx, query, nil)
            done <- true
        }(i)
    }
    
    // Wait for completion
    for i := 0; i < 20; i++ {
        <-done
    }
    
    // No race detector warnings should occur
}

func TestCircuitBreakerIntegration(t *testing.T) {
    // Setup failing Redis
    failingRedis := testutil.NewFailingRedis(t)
    cache := setupTestCache(t, failingRedis)
    
    ctx := auth.WithTenantID(context.Background(), uuid.New())
    
    // Trigger circuit breaker
    for i := 0; i < 10; i++ {
        err := cache.Set(ctx, "test", nil, nil)
        assert.Error(t, err)
    }
    
    // Circuit should be open
    err := cache.Set(ctx, "test", nil, nil)
    assert.Error(t, err)
    assert.Contains(t, err.Error(), "circuit breaker is open")
}
```

### 3.4 Performance Benchmarks
**Time**: 1 day

```go
// pkg/embedding/cache/benchmark_test.go
package cache_test

import (
    "context"
    "testing"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/auth"
)

func BenchmarkCacheSet(b *testing.B) {
    cache := setupBenchmarkCache(b)
    ctx := auth.WithTenantID(context.Background(), uuid.New())
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            query := fmt.Sprintf("benchmark query %d", i)
            _ = cache.Set(ctx, query, generateEmbedding(), nil)
            i++
        }
    })
}

func BenchmarkCacheGet(b *testing.B) {
    cache := setupBenchmarkCache(b)
    ctx := auth.WithTenantID(context.Background(), uuid.New())
    
    // Pre-populate cache
    for i := 0; i < 1000; i++ {
        query := fmt.Sprintf("query %d", i)
        _ = cache.Set(ctx, query, generateEmbedding(), nil)
    }
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        i := 0
        for pb.Next() {
            query := fmt.Sprintf("query %d", i%1000)
            _, _ = cache.Get(ctx, query, generateEmbedding())
            i++
        }
    })
}
```

## Phase 4: Advanced Features (2 weeks)

### 4.1 Vector Similarity Search with pgvector
**Time**: 3 days

```go
// pkg/embedding/cache/vector_store.go
package cache

import (
    "context"
    "database/sql"
    
    "github.com/google/uuid"
    "github.com/pgvector/pgvector-go"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
)

type VectorStore struct {
    db      *sql.DB
    logger  observability.Logger
    metrics observability.MetricsClient
}

func NewVectorStore(db *sql.DB, logger observability.Logger, metrics observability.MetricsClient) *VectorStore {
    return &VectorStore{
        db:      db,
        logger:  logger,
        metrics: metrics,
    }
}

// StoreCacheEmbedding stores query embedding in pgvector
func (v *VectorStore) StoreCacheEmbedding(ctx context.Context, tenantID uuid.UUID, cacheKey, queryHash string, embedding []float32) error {
    query := `
        INSERT INTO cache_metadata (tenant_id, cache_key, query_hash, embedding)
        VALUES ($1, $2, $3, $4)
        ON CONFLICT (tenant_id, cache_key) 
        DO UPDATE SET 
            embedding = EXCLUDED.embedding,
            hit_count = cache_metadata.hit_count + 1,
            last_accessed_at = CURRENT_TIMESTAMP
    `
    
    _, err := v.db.ExecContext(ctx, query, tenantID, cacheKey, queryHash, pgvector.NewVector(embedding))
    if err != nil {
        return fmt.Errorf("failed to store embedding: %w", err)
    }
    
    return nil
}

// FindSimilarQueries finds cached queries with similar embeddings
func (v *VectorStore) FindSimilarQueries(ctx context.Context, tenantID uuid.UUID, embedding []float32, threshold float32, limit int) ([]SimilarQuery, error) {
    query := `
        SELECT 
            cache_key,
            query_hash,
            1 - (embedding <=> $2) as similarity,
            hit_count,
            last_accessed_at
        FROM cache_metadata
        WHERE tenant_id = $1
            AND 1 - (embedding <=> $2) >= $3
        ORDER BY embedding <=> $2
        LIMIT $4
    `
    
    rows, err := v.db.QueryContext(ctx, query, tenantID, pgvector.NewVector(embedding), threshold, limit)
    if err != nil {
        return nil, fmt.Errorf("vector search failed: %w", err)
    }
    defer rows.Close()
    
    var results []SimilarQuery
    for rows.Next() {
        var sq SimilarQuery
        err := rows.Scan(&sq.CacheKey, &sq.QueryHash, &sq.Similarity, &sq.HitCount, &sq.LastAccessedAt)
        if err != nil {
            return nil, err
        }
        results = append(results, sq)
    }
    
    return results, nil
}

// UpdateAccessStats updates hit count and access time
func (v *VectorStore) UpdateAccessStats(ctx context.Context, tenantID uuid.UUID, cacheKey string) error {
    query := `
        UPDATE cache_metadata 
        SET hit_count = hit_count + 1,
            last_accessed_at = CURRENT_TIMESTAMP
        WHERE tenant_id = $1 AND cache_key = $2
    `
    
    _, err := v.db.ExecContext(ctx, query, tenantID, cacheKey)
    return err
}
```

### 4.2 LRU Eviction with Tenant Awareness
**Time**: 2 days

```go
// pkg/embedding/cache/eviction/lru.go
package eviction

import (
    "context"
    "time"
    
    "github.com/google/uuid"
    "github.com/developer-mesh/developer-mesh/pkg/common/cache"
)

type LRUEvictor struct {
    cache       CacheInterface
    vectorStore *cache.VectorStore
    config      Config
    logger      observability.Logger
    metrics     observability.MetricsClient
}

type Config struct {
    MaxEntriesPerTenant int
    MaxGlobalEntries    int
    CheckInterval       time.Duration
    BatchSize           int
}

// EvictTenantEntries evicts LRU entries for a specific tenant
func (e *LRUEvictor) EvictTenantEntries(ctx context.Context, tenantID uuid.UUID) error {
    // Get tenant's cache stats
    stats, err := e.vectorStore.GetTenantCacheStats(ctx, tenantID)
    if err != nil {
        return fmt.Errorf("failed to get tenant stats: %w", err)
    }
    
    if stats.EntryCount <= e.config.MaxEntriesPerTenant {
        return nil // No eviction needed
    }
    
    // Calculate entries to evict (10% buffer)
    toEvict := stats.EntryCount - int(float64(e.config.MaxEntriesPerTenant)*0.9)
    
    // Get LRU entries from database
    entries, err := e.vectorStore.GetLRUEntries(ctx, tenantID, toEvict)
    if err != nil {
        return fmt.Errorf("failed to get LRU entries: %w", err)
    }
    
    // Batch evict
    for i := 0; i < len(entries); i += e.config.BatchSize {
        batch := entries[i:min(i+e.config.BatchSize, len(entries))]
        
        if err := e.evictBatch(ctx, tenantID, batch); err != nil {
            e.logger.Error("Failed to evict batch", map[string]interface{}{
                "error":      err.Error(),
                "tenant_id":  tenantID.String(),
                "batch_size": len(batch),
            })
        }
    }
    
    e.metrics.IncrementCounterWithLabels("cache.evictions", float64(len(entries)), map[string]string{
        "tenant_id": tenantID.String(),
        "strategy":  "lru",
    })
    
    return nil
}

func (e *LRUEvictor) evictBatch(ctx context.Context, tenantID uuid.UUID, entries []LRUEntry) error {
    // Start transaction for consistency
    tx, err := e.cache.BeginTx(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    for _, entry := range entries {
        // Delete from Redis
        if err := e.cache.DeleteWithTx(tx, entry.CacheKey); err != nil {
            return err
        }
        
        // Delete from database
        if err := e.vectorStore.DeleteCacheEntry(ctx, tenantID, entry.CacheKey); err != nil {
            return err
        }
    }
    
    return tx.Commit()
}

// Run starts the background eviction process
func (e *LRUEvictor) Run(ctx context.Context) {
    ticker := time.NewTicker(e.config.CheckInterval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            e.runEvictionCycle(ctx)
        case <-ctx.Done():
            return
        }
    }
}

func (e *LRUEvictor) runEvictionCycle(ctx context.Context) {
    RecoverMiddleware(e.logger, "eviction_cycle")(func() {
        // Get all tenants with cache entries
        tenants, err := e.vectorStore.GetTenantsWithCache(ctx)
        if err != nil {
            e.logger.Error("Failed to get tenants", map[string]interface{}{
                "error": err.Error(),
            })
            return
        }
        
        // Evict for each tenant
        for _, tenantID := range tenants {
            if err := e.EvictTenantEntries(ctx, tenantID); err != nil {
                e.logger.Error("Failed to evict for tenant", map[string]interface{}{
                    "error":     err.Error(),
                    "tenant_id": tenantID.String(),
                })
            }
        }
    })
}
```

### 4.3 Compression with Security
**Time**: 1 day

```go
// pkg/embedding/cache/compression.go
package cache

import (
    "bytes"
    "compress/gzip"
    "encoding/base64"
    "io"
    
    "github.com/developer-mesh/developer-mesh/pkg/security"
)

type CompressionService struct {
    encryptionService *security.EncryptionService
    compressionLevel  int
    minSizeBytes      int
}

func NewCompressionService(encryptionKey string) *CompressionService {
    return &CompressionService{
        encryptionService: security.NewEncryptionService(encryptionKey),
        compressionLevel:  gzip.BestSpeed,
        minSizeBytes:      1024, // Only compress if > 1KB
    }
}

// CompressAndEncrypt compresses then encrypts data
func (c *CompressionService) CompressAndEncrypt(data []byte, tenantID string) (string, error) {
    // Skip compression for small data
    if len(data) < c.minSizeBytes {
        encrypted, err := c.encryptionService.EncryptCredential(string(data), tenantID)
        if err != nil {
            return "", err
        }
        return base64.StdEncoding.EncodeToString(encrypted), nil
    }
    
    // Compress
    compressed, err := c.compress(data)
    if err != nil {
        return "", fmt.Errorf("compression failed: %w", err)
    }
    
    // Encrypt compressed data
    encrypted, err := c.encryptionService.EncryptCredential(string(compressed), tenantID)
    if err != nil {
        return "", fmt.Errorf("encryption failed: %w", err)
    }
    
    return base64.StdEncoding.EncodeToString(encrypted), nil
}

// DecryptAndDecompress decrypts then decompresses data
func (c *CompressionService) DecryptAndDecompress(encryptedBase64, tenantID string) ([]byte, error) {
    // Decode base64
    encrypted, err := base64.StdEncoding.DecodeString(encryptedBase64)
    if err != nil {
        return nil, fmt.Errorf("base64 decode failed: %w", err)
    }
    
    // Decrypt
    decrypted, err := c.encryptionService.DecryptCredential(encrypted, tenantID)
    if err != nil {
        return nil, fmt.Errorf("decryption failed: %w", err)
    }
    
    // Check if compressed (gzip magic bytes)
    data := []byte(decrypted)
    if !c.isCompressed(data) {
        return data, nil
    }
    
    // Decompress
    return c.decompress(data)
}

func (c *CompressionService) compress(data []byte) ([]byte, error) {
    var buf bytes.Buffer
    gz, err := gzip.NewWriterLevel(&buf, c.compressionLevel)
    if err != nil {
        return nil, err
    }
    
    if _, err := gz.Write(data); err != nil {
        gz.Close()
        return nil, err
    }
    
    if err := gz.Close(); err != nil {
        return nil, err
    }
    
    return buf.Bytes(), nil
}

func (c *CompressionService) decompress(data []byte) ([]byte, error) {
    gz, err := gzip.NewReader(bytes.NewReader(data))
    if err != nil {
        return nil, err
    }
    defer gz.Close()
    
    return io.ReadAll(gz)
}

func (c *CompressionService) isCompressed(data []byte) bool {
    return len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b
}
```

## Monitoring and Observability

### Prometheus Metrics
```go
// pkg/embedding/cache/metrics.go
package cache

import (
    "github.com/prometheus/client_golang/prometheus"
)

var (
    cacheHits = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "semantic_cache_hits_total",
            Help: "Total number of cache hits",
        },
        []string{"tenant_id", "cache_type"},
    )
    
    cacheMisses = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "semantic_cache_misses_total",
            Help: "Total number of cache misses",
        },
        []string{"tenant_id", "cache_type"},
    )
    
    cacheLatency = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "semantic_cache_operation_duration_seconds",
            Help:    "Cache operation latency",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
        },
        []string{"operation", "status"},
    )
    
    cacheSize = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "semantic_cache_size_bytes",
            Help: "Current cache size in bytes",
        },
        []string{"tenant_id"},
    )
    
    cacheEvictions = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "semantic_cache_evictions_total",
            Help: "Total number of cache evictions",
        },
        []string{"tenant_id", "reason"},
    )
)

func RegisterMetrics(registry prometheus.Registerer) {
    registry.MustRegister(
        cacheHits,
        cacheMisses,
        cacheLatency,
        cacheSize,
        cacheEvictions,
    )
}
```

### Grafana Dashboard
```json
{
  "dashboard": {
    "title": "Semantic Cache Performance",
    "panels": [
      {
        "title": "Cache Hit Rate",
        "targets": [
          {
            "expr": "rate(semantic_cache_hits_total[5m]) / (rate(semantic_cache_hits_total[5m]) + rate(semantic_cache_misses_total[5m]))"
          }
        ]
      },
      {
        "title": "Operation Latency (p99)",
        "targets": [
          {
            "expr": "histogram_quantile(0.99, rate(semantic_cache_operation_duration_seconds_bucket[5m]))"
          }
        ]
      },
      {
        "title": "Cache Size by Tenant",
        "targets": [
          {
            "expr": "semantic_cache_size_bytes"
          }
        ]
      },
      {
        "title": "Eviction Rate",
        "targets": [
          {
            "expr": "rate(semantic_cache_evictions_total[5m])"
          }
        ]
      }
    ]
  }
}
```

## Deployment Plan

### 1. Pre-deployment Checklist
- [ ] Run database migrations
- [ ] Update configuration files
- [ ] Deploy monitoring dashboards
- [ ] Prepare rollback plan
- [ ] Update documentation

### 2. Direct Deployment (Greenfield)
```yaml
# Since this is a new application with no existing users:

# Phase 1: Development (Week 1)
- Deploy complete tenant-isolated cache
- Run full test suite
- Establish performance baselines

# Phase 2: Staging (Week 2)
- Full feature deployment (no feature flags needed)
- Load testing with tenant isolation
- Security scanning

# Phase 3: Production (Week 3)
- Direct production deployment
- Monitor metrics
- No migration or gradual rollout needed
```

### 3. Rollback Plan
```bash
# Quick rollback script
#!/bin/bash
kubectl rollout undo deployment/semantic-cache -n production
kubectl rollout status deployment/semantic-cache -n production

# Verify rollback
curl -s http://semantic-cache.production.svc.cluster.local/health | jq .
```

## Success Metrics

1. **Performance**
   - P99 latency < 10ms for cache hits
   - Cache hit rate > 60%
   - Zero memory leaks over 7 days

2. **Reliability**
   - Zero race conditions
   - Circuit breaker prevents cascading failures
   - 99.9% uptime

3. **Security**
   - All sensitive data encrypted
   - No SQL injection vulnerabilities
   - Proper tenant isolation verified

4. **Operations**
   - Automated monitoring alerts
   - Clear troubleshooting documentation
   - Efficient resource usage

## Summary

This updated plan properly integrates with the Developer Mesh codebase by:
- Using existing packages (resilience, retry, security, auth)
- Following project patterns (UUID types, error handling, metrics)
- Adding missing components (migrations, config, monitoring)
- Implementing proper security measures
- Providing comprehensive testing and deployment strategies

The implementation is now ready for development with minimal risk of integration issues.