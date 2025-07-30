// Package cache provides a high-performance semantic caching solution for embeddings
// with support for multi-tenancy, LRU eviction, and degraded mode operation.
//
// # Overview
//
// The cache package implements a distributed caching system optimized for storing
// and retrieving embedding-based search results. It uses Redis as the primary
// storage backend with pgvector for similarity search, and includes fallback
// mechanisms for resilient operation.
//
// Key Features:
//   - Semantic similarity matching using vector embeddings
//   - Multi-tenant isolation with per-tenant configuration
//   - LRU eviction with configurable policies
//   - Automatic degraded mode with in-memory fallback
//   - Compression and encryption support
//   - Circuit breaker pattern for Redis resilience
//   - Comprehensive metrics and observability
//
// # Architecture
//
// The cache system consists of several layers:
//
//  1. SemanticCache: Core caching logic with similarity matching
//  2. TenantAwareCache: Multi-tenant wrapper with isolation
//  3. ResilientRedisClient: Redis client with circuit breaker
//  4. VectorStore: pgvector integration for similarity search
//  5. FallbackCache: In-memory LRU cache for degraded mode
//
// Basic Usage
//
//	// Create a semantic cache
//	redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
//	cache, err := cache.NewSemanticCache(redisClient, cache.DefaultConfig(), logger)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer cache.Shutdown(ctx)
//
//	// Store results
//	query := "What is the capital of France?"
//	embedding := []float32{0.1, 0.2, 0.3, ...} // From embedding model
//	results := []cache.CachedSearchResult{
//	    {ID: "1", Score: 0.95, Content: "Paris is the capital of France"},
//	}
//	err = cache.Set(ctx, query, embedding, results)
//
//	// Retrieve results
//	entry, err := cache.Get(ctx, query, embedding)
//	if entry != nil {
//	    // Cache hit - use cached results
//	    fmt.Printf("Found %d cached results\n", len(entry.Results))
//	}
//
// Multi-Tenant Usage
//
//	// Create tenant-aware cache
//	tenantCache := cache.NewTenantAwareCache(baseCache, configCache, keyManager, logger)
//
//	// Set tenant ID in context
//	ctx = auth.WithTenantID(ctx, tenantID)
//
//	// All operations are now tenant-isolated
//	err = tenantCache.Set(ctx, query, embedding, results)
//
// # Configuration
//
// Cache behavior can be customized through the Config struct:
//
//	config := &cache.Config{
//	    Prefix:              "myapp",      // Redis key prefix
//	    TTL:                 24 * time.Hour,
//	    SimilarityThreshold: 0.95,         // Minimum similarity for cache hits
//	    MaxCandidates:       10,           // Max candidates for similarity search
//	    MaxCacheSize:        10000,        // Max entries before eviction
//	    EnableCompression:   true,         // Compress large entries
//	    EnableMetrics:       true,         // Prometheus metrics
//	}
//
// # Degraded Mode
//
// When Redis becomes unavailable, the cache automatically switches to degraded
// mode using an in-memory fallback cache. This ensures continued operation with
// reduced capacity:
//
//	// Degraded mode is automatic - no code changes needed
//	// The cache will periodically check Redis health and recover automatically
//
// # LRU Eviction
//
// The cache supports multiple eviction policies through the LRU manager:
//
//	// Start LRU eviction with vector store
//	tenantCache.StartLRUEviction(ctx, vectorStore)
//	defer tenantCache.StopLRUEviction()
//
// Performance Considerations
//
//   - Use batch operations when possible (GetBatch)
//   - Enable compression for large result sets
//   - Configure appropriate TTL values
//   - Monitor metrics for cache hit rates
//   - Use circuit breaker timeouts for resilience
//
// Security
//
//   - All tenant data is isolated
//   - Sensitive data can be encrypted at rest
//   - API keys are validated with strict regex patterns
//   - SQL queries use parameterized statements
//
// # Monitoring
//
// The cache exposes Prometheus metrics:
//   - semantic_cache_hit_total: Cache hits by type
//   - semantic_cache_miss_total: Cache misses by type
//   - semantic_cache_degraded_mode: Degraded mode activations
//   - cache_eviction_total: Evictions by policy
//
// For more examples and advanced usage, see the test files and PRODUCTION_READY_IMPLEMENTATION_PLAN.md.
package cache
