# Edge MCP Cache Configuration Guide

## Overview

Edge MCP implements a two-tier caching system designed for flexibility across different deployment environments:

- **L1 Cache**: In-memory cache (always enabled)
- **L2 Cache**: Redis cache (optional, for distributed deployments)

This architecture allows Edge MCP to:
- Run standalone without infrastructure dependencies (local development)
- Scale horizontally with shared cache in Kubernetes (production)
- Gracefully degrade when Redis is unavailable

## Architecture

### L1 Memory Cache
- **Always enabled**: No external dependencies
- **Fast access**: Sub-microsecond latency
- **Local to each instance**: Not shared across pods
- **Default size**: 10,000 items (~100MB)
- **Default TTL**: 5 minutes
- **Automatic cleanup**: Expired items removed every minute

### L2 Redis Cache (Optional)
- **Distributed**: Shared across all Edge MCP instances
- **Longer TTL**: Default 1 hour
- **Compression**: Values >1KB compressed with gzip
- **Async writes**: Non-blocking, best-effort
- **Health monitoring**: Automatic health checks every 30 seconds
- **Graceful degradation**: Falls back to memory-only mode on failure

## Deployment Modes

### 1. Local Development (Memory-Only)

For local development without Redis:

```bash
# No Redis required
EDGE_MCP_REDIS_ENABLED=false \
./apps/edge-mcp/edge-mcp
```

**Configuration:**
```yaml
cache:
  redis_enabled: false
  l1_max_items: 10000
  l1_ttl: 5m
```

**Characteristics:**
- ✅ No infrastructure dependencies
- ✅ Fast startup
- ✅ Simple debugging
- ❌ No cache sharing across restarts
- ❌ Limited capacity

### 2. Local Development with Redis

For testing Redis integration locally:

```bash
# Start Redis locally
docker run -d -p 6379:6379 redis:7-alpine

# Run Edge MCP with Redis
EDGE_MCP_REDIS_ENABLED=true \
EDGE_MCP_REDIS_URL=redis://localhost:6379/0 \
EDGE_MCP_REDIS_FALLBACK_MODE=true \
./apps/edge-mcp/edge-mcp
```

**Configuration:**
```yaml
cache:
  redis_enabled: true
  redis_url: redis://localhost:6379/0
  redis_fallback_mode: true
  redis_connect_timeout: 5s
  l1_max_items: 10000
  l1_ttl: 5m
  l2_ttl: 1h
  enable_compression: true
```

**Characteristics:**
- ✅ Tests full cache behavior
- ✅ Falls back to memory if Redis unavailable
- ✅ Validates compression and persistence
- ⚠️ Requires local Redis instance

### 3. Kubernetes Deployment (Distributed Cache)

For production Kubernetes deployment:

**Redis Deployment:**
```yaml
# redis.yaml
apiVersion: v1
kind: Service
metadata:
  name: redis
  namespace: edge-mcp
spec:
  ports:
    - port: 6379
      targetPort: 6379
  selector:
    app: redis
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: redis
  namespace: edge-mcp
spec:
  replicas: 1
  selector:
    matchLabels:
      app: redis
  template:
    metadata:
      labels:
        app: redis
    spec:
      containers:
        - name: redis
          image: redis:7-alpine
          ports:
            - containerPort: 6379
          resources:
            requests:
              memory: "256Mi"
              cpu: "100m"
            limits:
              memory: "512Mi"
              cpu: "500m"
```

**Edge MCP Deployment:**
```yaml
# edge-mcp.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-mcp
  namespace: edge-mcp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: edge-mcp
  template:
    metadata:
      labels:
        app: edge-mcp
    spec:
      containers:
        - name: edge-mcp
          image: edge-mcp:latest
          env:
            - name: EDGE_MCP_REDIS_ENABLED
              value: "true"
            - name: EDGE_MCP_REDIS_URL
              value: "redis://redis:6379/0"
            - name: EDGE_MCP_REDIS_FALLBACK_MODE
              value: "true"
            - name: EDGE_MCP_L1_MAX_ITEMS
              value: "10000"
            - name: EDGE_MCP_L1_TTL
              value: "5m"
            - name: EDGE_MCP_L2_TTL
              value: "1h"
            - name: EDGE_MCP_ENABLE_COMPRESSION
              value: "true"
          ports:
            - containerPort: 8082
          resources:
            requests:
              memory: "256Mi"
              cpu: "200m"
            limits:
              memory: "512Mi"
              cpu: "1000m"
```

**Characteristics:**
- ✅ Shared cache across all pods
- ✅ Horizontal scaling
- ✅ Cache survives pod restarts
- ✅ Reduced external API calls
- ✅ Falls back to memory if Redis unavailable
- ⚠️ Requires Redis deployment

## Configuration Options

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `EDGE_MCP_REDIS_ENABLED` | Enable Redis L2 cache | `false` | No |
| `EDGE_MCP_REDIS_URL` | Redis connection URL | `redis://localhost:6379/0` | Yes (if enabled) |
| `EDGE_MCP_REDIS_CONNECT_TIMEOUT` | Redis connection timeout | `5s` | No |
| `EDGE_MCP_REDIS_FALLBACK_MODE` | Fall back to memory-only if Redis fails | `true` | No |
| `EDGE_MCP_L1_MAX_ITEMS` | Maximum L1 cache items | `10000` | No |
| `EDGE_MCP_L1_TTL` | L1 cache TTL | `5m` | No |
| `EDGE_MCP_L2_TTL` | L2 cache TTL (Redis) | `1h` | No |
| `EDGE_MCP_ENABLE_COMPRESSION` | Compress values >1KB | `true` | No |
| `EDGE_MCP_COMPRESSION_THRESHOLD` | Compression threshold in bytes | `1024` | No |

### Redis URL Format

```
redis://[username:password@]host:port/database

Examples:
- redis://localhost:6379/0
- redis://redis:6379/0  (Kubernetes service)
- redis://user:pass@redis-master:6379/1
- rediss://secure-redis:6380/0  (TLS)
```

## Cache Behavior

### Write Path

1. **Set Operation**:
   - Store in L1 cache immediately (blocking)
   - Store in L2 cache asynchronously (non-blocking)
   - If Redis fails, log warning and continue

```
Client → Set(key, value) → L1 (sync) → L2 (async, best-effort)
                              ↓
                           Success
```

### Read Path

1. **Get Operation**:
   - Check L1 cache first (fast path)
   - If L1 miss, check L2 cache (Redis)
   - If L2 hit, populate L1 for next access
   - If both miss, return error

```
Client → Get(key) → L1 → Hit? → Return
                      ↓ Miss
                     L2 → Hit? → Populate L1 → Return
                      ↓ Miss
                    Error
```

### Cache Warming

On startup, you can pre-populate the cache:

```go
cache := tieredcache.NewTieredCache(config)

// Warm cache with frequently accessed keys
keys := []string{"tool:github", "tool:harness", "config:default"}
_ = cache.WarmCache(ctx, keys)
```

### Cache Invalidation

**Pattern-based invalidation** (Redis only):
```go
// Invalidate all session keys for a user
_ = cache.InvalidatePattern(ctx, "session:user123:")

// Invalidate all tool configurations
_ = cache.InvalidatePattern(ctx, "tool:")
```

**TTL-based expiration** (both L1 and L2):
- L1: Items expire after configured TTL (default 5min)
- L2: Items expire after configured TTL (default 1hr)
- Automatic cleanup removes expired items

## Monitoring

### Health Checks

Edge MCP exposes cache health via `/health/ready` endpoint:

```bash
curl http://localhost:8082/health/ready
```

**Response:**
```json
{
  "status": "healthy",
  "components": {
    "cache": {
      "status": "healthy",
      "details": {
        "l1_size": 150,
        "l2_enabled": true,
        "l2_healthy": true
      }
    }
  }
}
```

### Cache Statistics

Access cache statistics at runtime:

```go
stats := cache.GetStats()

// stats contains:
// - l1_hits, l1_misses, l1_hit_rate
// - l2_hits, l2_misses, l2_hit_rate (if enabled)
// - l1_size: current L1 cache size
// - l2_enabled, l2_healthy: Redis status
// - total_requests, total_errors
// - compression_saved, compression_count
```

**Example output:**
```json
{
  "l1_hits": 1500,
  "l1_misses": 300,
  "l1_hit_rate": 0.833,
  "l1_size": 450,
  "l2_enabled": true,
  "l2_healthy": true,
  "l2_hits": 250,
  "l2_misses": 50,
  "l2_hit_rate": 0.833,
  "total_requests": 1800,
  "total_errors": 5,
  "compression_saved": 524288,
  "compression_count": 120
}
```

### Prometheus Metrics (Future Enhancement)

Cache metrics will be exposed via `/metrics` endpoint:

```
# L1 cache metrics
edge_mcp_cache_l1_hits_total
edge_mcp_cache_l1_misses_total
edge_mcp_cache_l1_size

# L2 cache metrics
edge_mcp_cache_l2_hits_total
edge_mcp_cache_l2_misses_total
edge_mcp_cache_l2_healthy

# Compression metrics
edge_mcp_cache_compression_bytes_saved_total
edge_mcp_cache_compression_operations_total
```

## Performance Characteristics

### L1 Cache (Memory)
- **Latency**: <1μs per operation
- **Throughput**: >1M ops/sec
- **Memory**: ~100MB for 10K items
- **Concurrency**: Thread-safe with RWMutex

### L2 Cache (Redis)
- **Latency**: 1-5ms per operation
- **Throughput**: 10K-100K ops/sec (depending on network)
- **Memory**: Configured in Redis
- **Concurrency**: Fully concurrent

### Compression
- **Threshold**: 1KB (configurable)
- **Ratio**: ~50-70% for JSON data
- **Overhead**: ~100μs for 10KB data
- **Benefit**: Reduced Redis memory, network bandwidth

## Best Practices

### 1. TTL Configuration

**Short TTL for L1** (5 minutes):
- Keeps memory usage bounded
- Ensures fresh data
- Acceptable for hot data

**Longer TTL for L2** (1 hour):
- Reduces external API calls
- Survives L1 cache misses
- Acceptable for cacheable data

### 2. Cache Key Design

Use hierarchical keys for better invalidation:
```
tool:{provider}:{action}
session:{user_id}:{session_id}
config:{tenant_id}:{key}
```

### 3. Compression

Enable compression for production:
- Reduces Redis memory by 50-70%
- Reduces network bandwidth
- Minimal CPU overhead
- Threshold at 1KB is optimal

### 4. Fallback Mode

Always enable fallback mode in production:
```yaml
redis_fallback_mode: true
```

This ensures Edge MCP continues operating even if Redis fails.

### 5. Connection Timeout

Keep Redis timeout short (5s) for fast failure:
```yaml
redis_connect_timeout: 5s
```

This prevents long startup delays when Redis is unavailable.

## Troubleshooting

### Redis Connection Failures

**Symptom**: Edge MCP starts but logs Redis connection errors

**Solution**:
1. Verify Redis is running: `redis-cli ping`
2. Check Redis URL configuration
3. Verify network connectivity
4. Enable fallback mode to continue operation

### Cache Misses Higher Than Expected

**Symptom**: High L1 miss rate despite recent writes

**Possible causes**:
1. L1 TTL too short
2. L1 cache size too small (evictions)
3. Multiple Edge MCP instances (no L2 sharing)

**Solution**:
1. Increase L1 TTL: `EDGE_MCP_L1_TTL=10m`
2. Increase L1 size: `EDGE_MCP_L1_MAX_ITEMS=20000`
3. Enable Redis L2 cache for sharing

### High Memory Usage

**Symptom**: Edge MCP memory usage grows over time

**Possible causes**:
1. L1 cache size too large
2. Large cached values
3. Memory leak (unlikely)

**Solution**:
1. Reduce L1 size: `EDGE_MCP_L1_MAX_ITEMS=5000`
2. Enable compression: `EDGE_MCP_ENABLE_COMPRESSION=true`
3. Reduce L1 TTL: `EDGE_MCP_L1_TTL=3m`

### Redis Memory Exhaustion

**Symptom**: Redis runs out of memory

**Solution**:
1. Enable Redis maxmemory policy:
   ```
   redis-cli CONFIG SET maxmemory 512mb
   redis-cli CONFIG SET maxmemory-policy allkeys-lru
   ```
2. Reduce L2 TTL: `EDGE_MCP_L2_TTL=30m`
3. Enable compression
4. Monitor Redis memory usage

## Migration Guide

### From Memory-Only to Redis

1. Deploy Redis in your cluster
2. Update Edge MCP configuration:
   ```yaml
   EDGE_MCP_REDIS_ENABLED=true
   EDGE_MCP_REDIS_URL=redis://redis:6379/0
   EDGE_MCP_REDIS_FALLBACK_MODE=true
   ```
3. Restart Edge MCP pods (rolling update)
4. Monitor cache hit rates and Redis health

**Expected behavior**:
- Initial cache misses as L2 warms up
- Gradual improvement in hit rates
- Shared cache across all pods

### From Redis Back to Memory-Only

1. Set `EDGE_MCP_REDIS_ENABLED=false`
2. Restart Edge MCP pods
3. Monitor memory usage (may increase)

**Expected behavior**:
- No cache sharing across pods
- Each pod maintains its own cache
- Slight increase in external API calls

## References

- [TieredCache Implementation](../../apps/edge-mcp/internal/cache/tiered_cache.go)
- [Cache Configuration](../../apps/edge-mcp/internal/config/cache_config.go)
- [Redis Best Practices](https://redis.io/docs/manual/patterns/)
