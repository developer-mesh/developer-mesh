# Semantic Cache with Tenant Isolation

## Overview

The Developer Mesh semantic cache provides a high-performance, tenant-isolated caching layer for embedding-based search results. It includes advanced features like LRU eviction, encryption for sensitive data, vector similarity search, and comprehensive monitoring.

## Features

- **Tenant Isolation**: Complete data isolation between tenants using UUID-based identification
- **LRU Eviction**: Configurable eviction policies based on access patterns and size limits
- **Encryption**: Automatic encryption of sensitive data using AES-256-GCM
- **Vector Search**: Integration with pgvector for similarity-based cache lookups
- **Compression**: Optional gzip compression for large cache entries
- **Rate Limiting**: Per-tenant and per-operation rate limiting
- **Monitoring**: Prometheus metrics and OpenTelemetry tracing
- **Circuit Breaker**: Resilient Redis operations with automatic failover

## Architecture

```
┌─────────────────────┐
│   HTTP Request      │
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│  Auth Middleware    │ ◄── Extract Tenant ID
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│  Rate Limiter       │ ◄── Per-tenant limits
└──────────┬──────────┘
           │
┌──────────▼──────────┐
│ TenantAwareCache    │ ◄── Main cache interface
└──────────┬──────────┘
           │
      ┌────┴────┬─────────┬──────────┐
      │         │         │          │
┌─────▼───┐ ┌──▼───┐ ┌───▼───┐ ┌────▼────┐
│  Redis  │ │ LRU  │ │Vector │ │Encrypt  │
│  Cache  │ │ Mgr  │ │Store  │ │Service  │
└─────────┘ └──────┘ └───────┘ └─────────┘
```

## Usage

### Basic Setup

```go
import (
    "github.com/developer-mesh/developer-mesh/pkg/embedding/cache"
    "github.com/developer-mesh/developer-mesh/pkg/repository"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Create base cache
config := cache.DefaultConfig()
baseCache, err := cache.NewSemanticCache(redisClient, config, nil)

// Create tenant-aware cache
tenantCache := cache.NewTenantAwareCache(
    baseCache,
    tenantConfigRepo,
    rateLimiter,
    "encryption-key",
    logger,
    metrics,
)

// Start LRU eviction (optional)
tenantCache.StartLRUEviction(ctx, vectorStore)
defer tenantCache.StopLRUEviction()
```

### Setting Cache Entries

```go
// Context must contain tenant ID
ctx := auth.WithTenantID(context.Background(), tenantID)

// Cache search results
results := []cache.CachedSearchResult{
    {
        ID:      "doc1",
        Content: "Document content",
        Score:   0.95,
        Metadata: map[string]interface{}{
            "source": "knowledge_base",
        },
    },
}

err := tenantCache.Set(ctx, query, embedding, results)
```

### Getting Cache Entries

```go
// Retrieve from cache
entry, err := tenantCache.Get(ctx, query, embedding)
if err == cache.ErrNoTenantID {
    // No tenant ID in context
} else if err == cache.ErrFeatureDisabled {
    // Cache disabled for this tenant
} else if entry != nil {
    // Cache hit
    fmt.Printf("Results: %v\n", entry.Results)
}
```

### Integration with Gin

```go
// Setup routes with all middleware
router := integration.NewCacheRouter(
    tenantCache,
    baseRateLimiter,
    vectorStore,
    logger,
    metrics,
)

api := gin.New()
router.SetupRoutes(api.Group("/api/v1"))
```

## Configuration

### Tenant Configuration

Each tenant can have custom cache settings:

```go
type CacheTenantConfig struct {
    MaxCacheEntries  int               // Max entries per tenant
    MaxCacheBytes    int64             // Max bytes per tenant
    CacheTTLOverride time.Duration     // Custom TTL
    EnabledFeatures  CacheFeatureFlags // Feature toggles
}

type CacheFeatureFlags struct {
    EnableSemanticCache  bool // Enable/disable cache
    EnableCacheWarming   bool // Pre-populate cache
    EnableAsyncEviction  bool // Background eviction
    EnableMetrics        bool // Tenant metrics
}
```

### LRU Configuration

```go
type Config struct {
    MaxTenantEntries  int           // Default max entries
    MaxTenantBytes    int64         // Default max bytes
    EvictionBatchSize int           // Batch size for eviction
    EvictionInterval  time.Duration // How often to run eviction
    TrackingBatchSize int           // Access tracking batch
    FlushInterval     time.Duration // Tracking flush interval
}
```

## Advanced Features

### Vector Store Integration

The cache integrates with pgvector for similarity search:

```go
vectorStore := cache.NewVectorStore(db, logger, metrics)

// Find similar cached queries
similar, err := vectorStore.FindSimilarQueries(
    ctx, 
    tenantID, 
    embedding, 
    0.8,    // similarity threshold
    10,     // max results
)
```

### Encryption

Sensitive data is automatically encrypted:

```go
// Fields named 'api_key', 'secret', etc. are encrypted
results := []cache.CachedSearchResult{
    {
        ID: "1",
        Metadata: map[string]interface{}{
            "api_key": "sk-12345", // Automatically encrypted
        },
    },
}
```

### Monitoring

Prometheus metrics are automatically exported:

```
# Cache operations
devmesh_cache_operation_duration_seconds{operation="get",tenant_id="...",status="hit"}
devmesh_cache_operation_count{operation="set",tenant_id="..."}

# LRU eviction
devmesh_cache_eviction_count{tenant_id="..."}
devmesh_cache_eviction_duration_seconds{tenant_id="..."}

# Rate limiting
devmesh_cache_rate_limit_exceeded{tenant_id="...",operation="read"}
```

## Testing

### Unit Tests

```bash
cd pkg/embedding/cache
go test ./...
```

### Test Suite

Use the provided test suite for comprehensive testing:

```go
import "github.com/developer-mesh/developer-mesh/pkg/embedding/cache/testing"

func TestMyCacheIntegration(t *testing.T) {
    suite.Run(t, new(testing.CacheTestSuite))
}
```

## Tenant Requirements

All cache operations require a tenant ID. There is no legacy or non-tenant mode:

1. All requests must include tenant identification via auth context
2. Auth middleware must extract and validate tenant IDs
3. Configure tenant-specific limits and features
4. Requests without tenant ID will be rejected with `ErrNoTenantID`

## Performance Considerations

1. **Redis Cluster**: Use hash tags `{tenant_id}` for cluster compatibility
2. **Batch Operations**: LRU eviction processes entries in batches
3. **Async Tracking**: Access tracking is non-blocking
4. **Circuit Breaker**: Prevents cascade failures
5. **Local Config Cache**: Tenant configs cached for 5 minutes

## Error Handling

Common errors and their meanings:

- `ErrNoTenantID`: No tenant ID found in context
- `ErrFeatureDisabled`: Cache disabled for tenant
- `ErrRateLimitExceeded`: Too many requests
- `ErrQuotaExceeded`: Tenant cache quota exceeded
- `ErrEvictionFailed`: Failed to evict entries

## Security

- All tenant data is isolated using UUID-based keys
- Sensitive fields are encrypted with AES-256-GCM
- API keys are validated against injection patterns
- Rate limiting prevents abuse
- No cross-tenant data access possible

## Future Enhancements

- [ ] Distributed cache warming
- [ ] ML-based eviction policies
- [ ] Cross-region replication
- [ ] GraphQL API support
- [ ] Cache analytics dashboard