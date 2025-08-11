# Performance Tuning Guide

> **Purpose**: Guide for optimizing Developer Mesh platform performance
> **Audience**: Platform engineers and DevOps teams
> **Scope**: Application optimization, database performance, caching, and monitoring
> **Status**: CONCEPTUAL - Many optimizations shown are not yet implemented

## Table of Contents

1. [Overview](#overview)
2. [Current Architecture](#current-architecture)
3. [Application Performance](#application-performance)
4. [Database Optimization](#database-optimization)
5. [Caching Strategy](#caching-strategy)
6. [Network Optimization](#network-optimization)
7. [AI Model Performance](#ai-model-performance)
8. [Infrastructure Scaling](#infrastructure-scaling)
9. [Monitoring and Profiling](#monitoring-and-profiling)
10. [Performance Testing](#performance-testing)
11. [Best Practices](#best-practices)

## Overview

**IMPORTANT**: This guide contains both implemented optimizations and theoretical performance improvements. The actual deployment is a single EC2 instance with Docker Compose, not a highly-optimized distributed system.

### Current Reality
- Single EC2 instance deployment
- Docker Compose orchestration
- Basic connection handling
- No CDN, ALB, or auto-scaling
- Limited performance optimizations implemented

### Theoretical Performance Goals (Not Measured)

| Metric | Target | Current | Status |
|--------|--------|---------|--------|
| API Response Time (p50) | < 50ms | Unknown | ❓ |
| API Response Time (p99) | < 200ms | Unknown | ❓ |
| WebSocket Latency | < 10ms | Unknown | ❓ |
| Task Processing | < 5s | Unknown | ❓ |
| Embedding Generation | < 500ms | Varies by model | ⚠️ |
| Database Query Time | < 10ms | Not optimized | ❌ |

## Current Architecture

### Actual Deployment Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    EC2 Instance                          │
│                 (Single t3.large)                        │
└───────────────────────┬─────────────────────────────────┘
                        │
┌───────────────────────┴─────────────────────────────────┐
│                 Docker Compose                           │
│        (All services on single host)                     │
├──────────────────────────────────────────────────────────┤
│  - mcp-server (port 8080)                                │
│  - rest-api (port 8081)                                  │
│  - worker (background tasks)                             │
└───────────────────────┬─────────────────────────────────┘
                        │
    ┌───────────────────┴────────────────────┐
    │                                        │
┌───┴─────────┐  ┌──────────────┐  ┌────────┴───┐
│ElastiCache  │  │RDS PostgreSQL│  │    SQS     │
│(SSH tunnel) │  │ + pgvector   │  │            │
└─────────────┘  └──────────────┘  └────────────┘
```

### Theoretical High-Performance Architecture (NOT IMPLEMENTED)

The following architecture represents potential future improvements:

### Performance-Critical Paths

1. **WebSocket Message Processing**: Binary protocol, connection pooling
2. **Database Queries**: Query optimization, connection pooling, read replicas
3. **Cache Layer**: Multi-level caching, cache warming
4. **Task Distribution**: Efficient routing algorithms
5. **AI Model Calls**: Request batching, model caching

## Application Performance

### Current Implementation Status

**What's Actually Implemented:**
- Basic WebSocket binary protocol (24-byte header)
- Simple connection pooling in `apps/mcp-server/internal/api/websocket/pool.go`
- Standard Go HTTP server
- Basic database queries without optimization

**What's NOT Implemented:**
- Worker pools for controlled concurrency
- Request batching
- Advanced memory management
- Performance profiling endpoints
- Optimized caching layers

### 1. Go Performance Optimization (EXAMPLES ONLY)

The following are examples of optimizations that COULD be implemented:

#### Memory Management (NOT IMPLEMENTED)

```go
// EXAMPLE: Use sync.Pool for frequent allocations
// This pattern is NOT currently used in MCP
var bufferPool = sync.Pool{
    New: func() interface{} {
        return make([]byte, 0, 4096)
    },
}

func processMessage(data []byte) {
    // Get buffer from pool
    buf := bufferPool.Get().([]byte)
    defer func() {
        buf = buf[:0] // Reset buffer
        bufferPool.Put(buf)
    }()
    
    // Use buffer for processing
    buf = append(buf, data...)
    // Process...
}

// Avoid string concatenation in loops
func buildResponse(items []string) string {
    var builder strings.Builder
    builder.Grow(len(items) * 50) // Pre-allocate
    
    for _, item := range items {
        builder.WriteString(item)
        builder.WriteByte('\n')
    }
    
    return builder.String()
}
```

#### Goroutine Management (NOT IMPLEMENTED)

```go
// EXAMPLE: Worker pool pattern for controlled concurrency
// MCP does NOT have a WorkerPool implementation
type WorkerPool struct {
    workers   int
    taskQueue chan Task
    wg        sync.WaitGroup
}

func NewWorkerPool(workers int, queueSize int) *WorkerPool {
    return &WorkerPool{
        workers:   workers,
        taskQueue: make(chan Task, queueSize),
    }
}

func (p *WorkerPool) Start() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

func (p *WorkerPool) worker() {
    defer p.wg.Done()
    
    for task := range p.taskQueue {
        processTask(task)
    }
}

// Use context for cancellation
func (p *WorkerPool) Submit(ctx context.Context, task Task) error {
    select {
    case p.taskQueue <- task:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

#### CPU Profiling

```go
import _ "net/http/pprof"

func init() {
    // Enable profiling endpoint
    go func() {
        log.Println(http.ListenAndServe("localhost:6060", nil))
    }()
}

// Profile CPU usage
// go tool pprof http://localhost:6060/debug/pprof/profile?seconds=30
// go tool pprof -http=:8080 profile.out
```

### 2. WebSocket Performance

#### Binary Protocol Optimization

```go
// Efficient binary message encoding
func (m *Message) MarshalBinary() ([]byte, error) {
    buf := make([]byte, 0, m.EstimatedSize())
    
    // Write header (24 bytes)
    buf = binary.BigEndian.AppendUint32(buf, MagicNumber)
    buf = append(buf, m.Version)
    buf = append(buf, m.Type)
    buf = binary.BigEndian.AppendUint16(buf, m.Flags)
    buf = binary.BigEndian.AppendUint64(buf, m.SequenceID)
    buf = binary.BigEndian.AppendUint16(buf, m.Method)
    buf = binary.BigEndian.AppendUint16(buf, 0) // Reserved
    
    // Compress payload if beneficial
    payload := m.Payload
    if len(payload) > CompressionThreshold {
        compressed := compress(payload)
        if len(compressed) < len(payload) {
            payload = compressed
            m.Flags |= FlagCompressed
        }
    }
    
    buf = binary.BigEndian.AppendUint32(buf, uint32(len(payload)))
    buf = append(buf, payload...)
    
    return buf, nil
}

// Zero-copy message reading
func ReadMessage(r io.Reader) (*Message, error) {
    // Read header into fixed buffer
    header := make([]byte, 24)
    if _, err := io.ReadFull(r, header); err != nil {
        return nil, err
    }
    
    // Parse header without allocations
    msg := &Message{
        Magic:      binary.BigEndian.Uint32(header[0:4]),
        Version:    header[4],
        Type:       header[5],
        Flags:      binary.BigEndian.Uint16(header[6:8]),
        SequenceID: binary.BigEndian.Uint64(header[8:16]),
        Method:     binary.BigEndian.Uint16(header[16:18]),
        DataSize:   binary.BigEndian.Uint32(header[20:24]),
    }
    
    // Read payload directly
    msg.Payload = make([]byte, msg.DataSize)
    if _, err := io.ReadFull(r, msg.Payload); err != nil {
        return nil, err
    }
    
    return msg, nil
}
```

#### Connection Management

```go
// Optimized WebSocket connection settings
func setupWebSocket(conn *websocket.Conn) {
    // Set buffer sizes
    conn.SetReadBufferSize(65536)  // 64KB
    conn.SetWriteBufferSize(65536) // 64KB
    
    // Configure compression
    conn.EnableWriteCompression(true)
    conn.SetCompressionLevel(6) // Balance speed/ratio
    
    // Set timeouts
    conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    conn.SetPongHandler(func(appData string) error {
        conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    
    // Enable TCP keepalive
    if tcpConn, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
        tcpConn.SetKeepAlive(true)
        tcpConn.SetKeepAlivePeriod(30 * time.Second)
        tcpConn.SetNoDelay(true) // Disable Nagle's algorithm
    }
}
```

### 3. HTTP API Performance

#### Request Handling

```go
// Middleware for request optimization
func PerformanceMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Pre-allocate response buffer
        c.Writer.Header().Set("X-Content-Type-Options", "nosniff")
        
        // Enable gzip compression
        if strings.Contains(c.GetHeader("Accept-Encoding"), "gzip") {
            c.Writer.Header().Set("Content-Encoding", "gzip")
            gz := gzip.NewWriter(c.Writer)
            defer gz.Close()
            c.Writer = &gzipWriter{ResponseWriter: c.Writer, Writer: gz}
        }
        
        c.Next()
    }
}

// Batch API endpoint
func BatchHandler(svc *Service) gin.HandlerFunc {
    return func(c *gin.Context) {
        var requests []BatchRequest
        if err := c.ShouldBindJSON(&requests); err != nil {
            c.JSON(400, gin.H{"error": err.Error()})
            return
        }
        
        // Process in parallel with controlled concurrency
        results := make([]BatchResult, len(requests))
        var wg sync.WaitGroup
        sem := make(chan struct{}, 10) // Max 10 concurrent
        
        for i, req := range requests {
            wg.Add(1)
            go func(idx int, r BatchRequest) {
                defer wg.Done()
                
                sem <- struct{}{}        // Acquire
                defer func() { <-sem }() // Release
                
                results[idx] = svc.ProcessRequest(r)
            }(i, req)
        }
        
        wg.Wait()
        c.JSON(200, results)
    }
}
```

## Database Optimization

### 1. PostgreSQL Performance

#### Connection Pooling

```go
// Optimized database configuration
func NewDBPool(config Config) (*pgxpool.Pool, error) {
    poolConfig, err := pgxpool.ParseConfig(config.DatabaseURL)
    if err != nil {
        return nil, err
    }
    
    // Connection pool settings
    poolConfig.MaxConns = 100
    poolConfig.MinConns = 10
    poolConfig.MaxConnLifetime = time.Hour
    poolConfig.MaxConnIdleTime = time.Minute * 30
    
    // Performance settings
    poolConfig.ConnConfig.RuntimeParams["application_name"] = "mcp-platform"
    poolConfig.ConnConfig.RuntimeParams["jit"] = "off" // Disable JIT for consistent performance
    poolConfig.ConnConfig.RuntimeParams["statement_timeout"] = "30s"
    
    // Connection lifecycle hooks
    poolConfig.BeforeAcquire = func(ctx context.Context, conn *pgx.Conn) bool {
        // Check connection health
        return conn.Ping(ctx) == nil
    }
    
    poolConfig.AfterRelease = func(conn *pgx.Conn) bool {
        // Reset connection state
        conn.Exec(context.Background(), "DISCARD ALL")
        return true
    }
    
    return pgxpool.ConnectConfig(context.Background(), poolConfig)
}
```

#### Query Optimization

```sql
-- Optimize agent queries with proper indexes
CREATE INDEX CONCURRENTLY idx_agents_status_workload 
ON agents(status, current_workload) 
WHERE status = 'active';

CREATE INDEX CONCURRENTLY idx_agents_capabilities_gin 
ON agents USING GIN (capabilities jsonb_path_ops);

-- Optimize task queries
CREATE INDEX CONCURRENTLY idx_tasks_status_priority_created 
ON tasks(status, priority DESC, created_at) 
WHERE status IN ('pending', 'assigned');

-- Partial index for active sessions
CREATE INDEX CONCURRENTLY idx_sessions_active 
ON sessions(user_id, last_activity) 
WHERE expires_at > NOW();

-- Optimize vector searches
CREATE INDEX CONCURRENTLY idx_embeddings_vector 
ON embeddings USING ivfflat (vector vector_l2_ops)
WITH (lists = 100);

-- Analyze tables regularly
ANALYZE agents, tasks, sessions, embeddings;
```

#### Prepared Statements

```go
// Cache prepared statements
type QueryCache struct {
    stmts map[string]*pgconn.StatementDescription
    mu    sync.RWMutex
}

func (db *Database) GetAgentByID(ctx context.Context, id string) (*Agent, error) {
    const query = `
        SELECT id, name, type, status, capabilities, 
               current_workload, max_workload, created_at
        FROM agents 
        WHERE id = $1`
    
    // Use cached prepared statement
    row := db.pool.QueryRow(ctx, "get_agent_by_id", id)
    
    var agent Agent
    err := row.Scan(
        &agent.ID, &agent.Name, &agent.Type, 
        &agent.Status, &agent.Capabilities,
        &agent.CurrentWorkload, &agent.MaxWorkload, 
        &agent.CreatedAt,
    )
    
    return &agent, err
}
```

### 2. pgvector Optimization

```sql
-- Optimize vector similarity searches
CREATE OR REPLACE FUNCTION search_similar_embeddings(
    query_vector vector(1536),
    limit_count int = 10,
    threshold float = 0.8
) RETURNS TABLE (
    id uuid,
    content text,
    similarity float
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        e.id,
        e.content,
        1 - (e.vector <=> query_vector) as similarity
    FROM embeddings e
    WHERE 1 - (e.vector <=> query_vector) > threshold
    ORDER BY e.vector <=> query_vector
    LIMIT limit_count;
END;
$$ LANGUAGE plpgsql;

-- Create materialized view for frequently accessed vectors
CREATE MATERIALIZED VIEW mv_document_embeddings AS
SELECT 
    d.id,
    d.title,
    d.content,
    e.vector,
    e.model,
    e.created_at
FROM documents d
JOIN embeddings e ON e.document_id = d.id
WHERE d.status = 'published';

CREATE INDEX ON mv_document_embeddings USING ivfflat (vector vector_l2_ops);
```

## Caching Strategy

### Current Implementation

MCP HAS a multi-level caching implementation in `pkg/common/cache/`:
- **Multi-level cache**: `multilevel_cache.go` with L1 (in-memory LRU) and L2 (Redis)
- **Redis cache**: Full Redis integration with connection pooling
- **Memory cache**: Simple in-memory cache implementation
- **Prefetch workers**: Background workers for cache warming

### 1. Actual Multi-Level Cache Architecture

```go
// From pkg/common/cache/multilevel_cache.go
type MultiLevelCache struct {
    // L1 cache (in-memory using hashicorp/golang-lru)
    l1Cache *lru.Cache[string, []byte]
    
    // L2 cache (Redis)
    l2Cache Cache
    
    // Configuration
    ttl           time.Duration
    metricsClient observability.MetricsClient
    
    // Prefetch queue for cache warming
    prefetchQueue   chan prefetchRequest
    prefetchWorkers int
}

// Actual implementation from multilevel_cache.go
func (c *MultiLevelCache) Get(ctx context.Context, key string, value interface{}) (bool, error) {
    startTime := time.Now()
    
    // Try L1 cache first (LRU in-memory)
    if data, ok := c.l1Cache.Get(key); ok {
        // Unmarshal value
        err := json.Unmarshal(data, value)
        if err != nil {
            return false, fmt.Errorf("failed to unmarshal value from L1 cache: %w", err)
        }
        
        // Record metrics
        duration := time.Since(startTime)
        c.metricsClient.RecordCacheOperation("get", true, duration.Seconds())
        
        return true, nil
    }
    
    // Try L2 cache (Redis)
    var data []byte
    err := c.l2Cache.Get(ctx, key, &data)
    if err != nil {
        // Record metrics
        duration := time.Since(startTime)
        c.metricsClient.RecordCacheOperation("get", false, duration.Seconds())
        
        // If it's a not found error, return false with no error
        if err == ErrNotFound {
            return false, nil
        }
        
        return false, err
    }
    
    // Add to L1 cache for future hits
    c.l1Cache.Add(key, data)
    
    // Unmarshal value
    err = json.Unmarshal(data, value)
    
    return true, nil
}
```

### 2. Cache Configuration

```go
// Actual configuration from multilevel_cache.go
type MultiLevelCacheConfig struct {
    L1MaxSize         int           `mapstructure:"l1_max_size"`
    DefaultTTL        time.Duration `mapstructure:"default_ttl"`
    PrefetchWorkers   int           `mapstructure:"prefetch_workers"`
    PrefetchQueueSize int           `mapstructure:"prefetch_queue_size"`
}

// Default configuration values
func NewMultiLevelCache(l2Cache Cache, config MultiLevelCacheConfig) (*MultiLevelCache, error) {
    // Apply defaults
    if config.L1MaxSize <= 0 {
        config.L1MaxSize = 1000 // Default to 1000 entries
    }
    if config.DefaultTTL <= 0 {
        config.DefaultTTL = 15 * time.Minute // Default to 15 minutes
    }
    if config.PrefetchWorkers <= 0 {
        config.PrefetchWorkers = 2 // Default to 2 workers
    }
    
    // Create L1 cache using LRU
    l1Cache, err := lru.New[string, []byte](config.L1MaxSize)
    // ...
}

// Redis configuration for performance
func NewRedisCache() *redis.Client {
    return redis.NewClient(&redis.Options{
        Addr:         os.Getenv("REDIS_ADDR"),
        Password:     os.Getenv("REDIS_PASSWORD"),
        DB:           0,
        PoolSize:     100,
        MinIdleConns: 10,
        MaxRetries:   3,
        DialTimeout:  5 * time.Second,
        ReadTimeout:  3 * time.Second,
        WriteTimeout: 3 * time.Second,
        PoolTimeout:  4 * time.Second,
        IdleTimeout:  5 * time.Minute,
        // Enable pipelining
        OnConnect: func(ctx context.Context, cn *redis.Conn) error {
            return cn.Ping(ctx).Err()
        },
    })
}
```

### 3. Cache Warming and Prefetching

```go
// Actual implementation uses prefetch workers
func (c *MultiLevelCache) prefetchWorker() {
    for req := range c.prefetchQueue {
        // Skip if already in L1 cache to avoid unnecessary work
        if _, ok := c.l1Cache.Get(req.key); ok {
            continue
        }
        
        // Get from L2 cache
        _, _ = c.GetContext(context.Background(), req.key)
        
        // Don't block the worker if there's an error
    }
}

// Automatic prefetching of related contexts
func (c *MultiLevelCache) prefetchRelatedContexts(context models.Context) {
    // Prefetch contexts from the same agent
    if context.AgentID != "" {
        key := fmt.Sprintf("contexts:agent:%s", context.AgentID)
        c.queuePrefetch(key, &context, map[string]interface{}{
            "type":     "agent_contexts",
            "agent_id": context.AgentID,
        })
    }
    
    // Prefetch contexts from the same session
    if context.SessionID != "" {
        key := fmt.Sprintf("contexts:session:%s", context.SessionID)
        c.queuePrefetch(key, &context, map[string]interface{}{
            "type":       "session_contexts",
            "session_id": context.SessionID,
        })
    }
}

func (w *CacheWarmer) warmCache(ctx context.Context) {
    // Warm frequently accessed data
    queries := []string{
        "SELECT * FROM agents WHERE status = 'active'",
        "SELECT * FROM models WHERE enabled = true",
        "SELECT * FROM configurations WHERE active = true",
    }
    
    for _, query := range queries {
        rows, err := w.db.Query(ctx, query)
        if err != nil {
            continue
        }
        
        for rows.Next() {
            var data map[string]interface{}
            rows.Scan(&data)
            
            key := generateCacheKey(query, data["id"])
            w.cache.Set(ctx, key, data)
        }
        rows.Close()
    }
}
```

## Network Optimization

### 1. HTTP/2 and Keep-Alive

```go
// Configure HTTP/2 server
server := &http.Server{
    Addr:    ":8080",
    Handler: router,
    TLSConfig: &tls.Config{
        MinVersion: tls.VersionTLS12,
        CurvePreferences: []tls.CurveID{
            tls.X25519,
            tls.CurveP256,
        },
        PreferServerCipherSuites: true,
        CipherSuites: []uint16{
            tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
            tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
            tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
        },
    },
    ReadTimeout:    10 * time.Second,
    WriteTimeout:   10 * time.Second,
    IdleTimeout:    120 * time.Second,
    MaxHeaderBytes: 1 << 20, // 1 MB
}

// Enable HTTP/2
http2.ConfigureServer(server, &http2.Server{
    MaxConcurrentStreams: 250,
    IdleTimeout:          120 * time.Second,
})
```

### 2. Load Balancer Configuration

```yaml
# ALB configuration for optimal performance
apiVersion: v1
kind: Service
metadata:
  name: mcp-server
  annotations:
    service.beta.kubernetes.io/aws-load-balancer-type: "nlb"
    service.beta.kubernetes.io/aws-load-balancer-cross-zone-load-balancing-enabled: "true"
    service.beta.kubernetes.io/aws-load-balancer-connection-draining-enabled: "true"
    service.beta.kubernetes.io/aws-load-balancer-connection-draining-timeout: "30"
spec:
  type: LoadBalancer
  sessionAffinity: ClientIP  # For WebSocket
  sessionAffinityConfig:
    clientIP:
      timeoutSeconds: 10800  # 3 hours
```

## AI Model Performance

### Current Implementation

MCP HAS batch processing for embeddings:
- **Batch API endpoint**: `/api/v2/embeddings/batch`
- **Service implementation**: `BatchGenerateEmbeddings` in `service_v2.go`
- **Provider support**: OpenAI, Bedrock, and Google providers support batching
- **Pipeline batching**: `BatchProcessContent` with configurable batch size

### 1. Actual Request Batching Implementation

```go
// From pkg/embedding/service_v2.go
func (s *ServiceV2) BatchGenerateEmbeddings(ctx context.Context, reqs []GenerateEmbeddingRequest) ([]*GenerateEmbeddingResponse, error) {
    // Groups requests by tenant and model for efficient processing
    requestsByAgent := make(map[string][]int)
    
    for i, req := range reqs {
        agentKey := fmt.Sprintf("%s:%s", req.TenantID, req.AgentID)
        requestsByAgent[agentKey] = append(requestsByAgent[agentKey], i)
    }
    
    // Process each group with the appropriate provider
    for agentKey, reqIndices := range requestsByAgent {
        // ... batch processing logic
        batchResp, err := provider.BatchGenerateEmbeddings(ctx, batchReq)
        // ...
    }
}

func (b *EmbeddingBatcher) AddRequest(text string) <-chan EmbeddingResult {
    b.mu.Lock()
    defer b.mu.Unlock()
    
    idx := len(b.batch)
    ch := make(chan EmbeddingResult, 1)
    
    b.batch = append(b.batch, text)
    b.results[idx] = ch
    
    // Process if batch is full
    if len(b.batch) >= b.maxBatch {
        b.processBatch()
    } else if b.timer == nil {
        // Start timer for partial batch
        b.timer = time.AfterFunc(b.maxWait, func() {
            b.mu.Lock()
            b.processBatch()
            b.mu.Unlock()
        })
    }
    
    return ch
}

func (b *EmbeddingBatcher) processBatch() {
    if len(b.batch) == 0 {
        return
    }
    
    // Process entire batch in one API call
    embeddings, err := b.generateBatchEmbeddings(b.batch)
    
    // Distribute results
    for idx, ch := range b.results {
        if err != nil {
            ch <- EmbeddingResult{Err: err}
        } else {
            ch <- EmbeddingResult{Embedding: embeddings[idx]}
        }
        close(ch)
    }
    
    // Reset
    b.batch = b.batch[:0]
    b.results = make(map[int]chan EmbeddingResult)
    if b.timer != nil {
        b.timer.Stop()
        b.timer = nil
    }
}
```

### 2. Model Response Caching

```go
// Cache AI model responses
type ModelCache struct {
    cache *ristretto.Cache
    ttl   time.Duration
}

func (mc *ModelCache) GetOrGenerate(
    ctx context.Context,
    prompt string,
    generate func() (string, error),
) (string, error) {
    // Generate cache key
    key := fmt.Sprintf("model:%x", sha256.Sum256([]byte(prompt)))
    
    // Check cache
    if val, found := mc.cache.Get(key); found {
        metrics.ModelCacheHit()
        return val.(string), nil
    }
    
    // Generate response
    response, err := generate()
    if err != nil {
        return "", err
    }
    
    // Cache successful responses
    mc.cache.SetWithTTL(key, response, 1, mc.ttl)
    metrics.ModelCacheMiss()
    
    return response, nil
}
```

## Infrastructure Scaling

### 1. Auto-Scaling Configuration

```hcl
# ECS auto-scaling
resource "aws_appautoscaling_target" "ecs_target" {
  service_namespace  = "ecs"
  resource_id        = "service/${var.cluster_name}/${var.service_name}"
  scalable_dimension = "ecs:service:DesiredCount"
  min_capacity       = 3
  max_capacity       = 50
}

# CPU-based scaling
resource "aws_appautoscaling_policy" "cpu_scaling" {
  name               = "${var.service_name}-cpu-scaling"
  service_namespace  = "ecs"
  resource_id        = aws_appautoscaling_target.ecs_target.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_target.scalable_dimension
  
  target_tracking_scaling_policy_configuration {
    predefined_metric_specification {
      predefined_metric_type = "ECSServiceAverageCPUUtilization"
    }
    target_value       = 70.0
    scale_in_cooldown  = 300
    scale_out_cooldown = 60
  }
}

# Custom metric scaling (requests per second)
resource "aws_appautoscaling_policy" "rps_scaling" {
  name               = "${var.service_name}-rps-scaling"
  service_namespace  = "ecs"
  resource_id        = aws_appautoscaling_target.ecs_target.resource_id
  scalable_dimension = aws_appautoscaling_target.ecs_target.scalable_dimension
  
  target_tracking_scaling_policy_configuration {
    customized_metric_specification {
      metric_name = "RequestsPerSecond"
      namespace   = "MCP/Performance"
      statistic   = "Average"
      unit        = "Count/Second"
    }
    target_value = 100.0
  }
}
```

### 2. Database Read Replicas

```go
// Read/write splitting
type DBManager struct {
    primary  *pgxpool.Pool
    replicas []*pgxpool.Pool
    mu       sync.RWMutex
    current  int
}

func (db *DBManager) Primary() *pgxpool.Pool {
    return db.primary
}

func (db *DBManager) Replica() *pgxpool.Pool {
    db.mu.Lock()
    defer db.mu.Unlock()
    
    // Round-robin selection
    replica := db.replicas[db.current]
    db.current = (db.current + 1) % len(db.replicas)
    
    return replica
}

// Use appropriate connection for query type
func (db *DBManager) Query(ctx context.Context, query string, args ...interface{}) (pgx.Rows, error) {
    // Route read queries to replicas
    if isReadQuery(query) {
        return db.Replica().Query(ctx, query, args...)
    }
    
    // Write queries go to primary
    return db.Primary().Query(ctx, query, args...)
}
```

## Monitoring and Profiling

### Current Implementation

MCP HAS comprehensive monitoring:
- **Prometheus metrics**: Full implementation in `pkg/observability/prometheus_metrics.go`
- **Metrics client interface**: Abstraction for different metric backends
- **OpenTelemetry tracing**: Support for distributed tracing
- **Structured logging**: JSON-formatted logs with context

### 1. Actual Application Metrics

```go
// From pkg/observability/prometheus_metrics.go
type PrometheusMetricsClient struct {
    namespace string
    subsystem string
    
    // Metric collectors
    counters   map[string]*prometheus.CounterVec
    gauges     map[string]*prometheus.GaugeVec
    histograms map[string]*prometheus.HistogramVec
    summaries  map[string]*prometheus.SummaryVec
}

// Actual metrics being recorded (from multilevel_cache.go)
c.metricsClient.RecordCacheOperation("get", true, duration.Seconds())
c.metricsClient.RecordCacheOperation("set", true, duration.Seconds())
    
    dbQueryDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "db_query_duration_seconds",
            Help:    "Database query latencies",
            Buckets: []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1},
        },
        []string{"query_type", "table"},
    )
    
    cacheHitRate = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "cache_hit_total",
            Help: "Cache hit counts",
        },
        []string{"cache_level", "cache_type"},
    )
)

// Middleware to track metrics
func MetricsMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        
        c.Next()
        
        duration := time.Since(start).Seconds()
        status := strconv.Itoa(c.Writer.Status())
        
        requestDuration.WithLabelValues(
            c.Request.Method,
            c.FullPath(),
            status,
        ).Observe(duration)
    }
}
```

### 2. Distributed Tracing

```go
// OpenTelemetry tracing for performance analysis
func InitTracing() {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
        ),
    )
    if err != nil {
        log.Fatal(err)
    }
    
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithSampler(trace.TraceIDRatioBased(0.1)), // Sample 10%
        trace.WithResource(resource.NewWithAttributes(
            semconv.ServiceNameKey.String("mcp-platform"),
        )),
    )
    
    otel.SetTracerProvider(tp)
}

// Trace expensive operations
func (s *Service) ProcessTask(ctx context.Context, task Task) error {
    ctx, span := tracer.Start(ctx, "ProcessTask",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    // Database operation
    dbCtx, dbSpan := tracer.Start(ctx, "db.query")
    agent, err := s.db.GetAgent(dbCtx, task.AgentID)
    dbSpan.End()
    
    if err != nil {
        span.RecordError(err)
        return err
    }
    
    // AI model call
    modelCtx, modelSpan := tracer.Start(ctx, "model.inference")
    result, err := s.model.Process(modelCtx, task.Input)
    modelSpan.SetAttributes(
        attribute.Int("tokens.input", result.InputTokens),
        attribute.Int("tokens.output", result.OutputTokens),
    )
    modelSpan.End()
    
    return nil
}
```

## Performance Testing

### 1. Load Testing

```go
// k6 load test script
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export const options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp up
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 200 },  // Ramp up
    { duration: '5m', target: 200 },  // Stay at 200 users
    { duration: '2m', target: 0 },    // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'], // 95% of requests under 500ms
    errors: ['rate<0.01'],            // Error rate under 1%
  },
};

export default function() {
  const payload = JSON.stringify({
    text: 'Sample text for embedding',
    model: 'titan-embed-v1',
  });
  
  const params = {
    headers: {
      'Content-Type': 'application/json',
      'Authorization': `Bearer ${__ENV.API_KEY}`,
    },
  };
  
  const res = http.post('https://api.mcp.example.com/v1/embeddings', payload, params);
  
  const success = check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
  
  errorRate.add(!success);
  sleep(1);
}
```

### 2. Benchmark Tests

```go
// Benchmark critical paths
func BenchmarkEmbeddingGeneration(b *testing.B) {
    svc := NewEmbeddingService(testConfig)
    ctx := context.Background()
    text := "This is a sample text for embedding generation benchmark"
    
    b.ResetTimer()
    b.RunParallel(func(pb *testing.PB) {
        for pb.Next() {
            _, err := svc.GenerateEmbedding(ctx, text)
            if err != nil {
                b.Fatal(err)
            }
        }
    })
    
    b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "embeddings/sec")
}

func BenchmarkDatabaseQuery(b *testing.B) {
    db := setupTestDB(b)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        var agent Agent
        err := db.QueryRow(ctx, 
            "SELECT * FROM agents WHERE id = $1", 
            "test-agent-001",
        ).Scan(&agent)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## What's Actually Implemented vs Theoretical

### Actually Implemented Performance Features ✅

1. **Multi-level caching** with LRU (L1) and Redis (L2)
2. **Cache prefetching** with background workers
3. **Batch embedding processing** via API and pipeline
4. **Prometheus metrics** for monitoring
5. **WebSocket binary protocol** with compression
6. **Basic connection pooling** for WebSocket
7. **Structured logging** with JSON format
8. **OpenTelemetry tracing** support

### NOT Implemented (Theoretical Only) ❌

1. **Worker pools** for controlled goroutine concurrency
2. **Advanced memory management** with sync.Pool
3. **CDN, ALB, Auto-scaling** - single EC2 instance only
4. **Database query optimization** - no prepared statements or read replicas
5. **Ristretto cache** - uses simpler LRU instead
6. **HTTP/2 server configuration**
7. **Performance profiling endpoints** (pprof)
8. **Load testing infrastructure**

## Best Practices

### 1. Code-Level Optimization (EXAMPLES - Not All Implemented)

```go
// These are EXAMPLES of optimizations that COULD be done
// Not all are implemented in MCP

// Pre-allocate slices (good practice)
agents := make([]Agent, 0, expectedCount)

// Reuse buffers (NOT implemented in MCP)
var bufferPool = sync.Pool{
    New: func() interface{} {
        return new(bytes.Buffer)
    },
}

// Batch database operations (NOT implemented)
func BatchInsert(items []Item) error {
    // This is an example - MCP doesn't use CopyFrom
    copyCount, err := db.CopyFrom(
        context.Background(),
        pgx.Identifier{"items"},
        []string{"id", "name", "value"},
        pgx.CopyFromSlice(len(items), func(i int) ([]interface{}, error) {
            return []interface{}{
                items[i].ID,
                items[i].Name,
                items[i].Value,
            }, nil
        }),
    )
    return err
}
```

### 2. Current Infrastructure Reality

1. **Single EC2 instance** - no CDN or load balancing
2. **Docker Compose** - not Kubernetes or ECS
3. **Basic monitoring** - not comprehensive APM
4. **Manual scaling** - no auto-scaling
5. **SSH tunnel for Redis** - not direct VPC access

### 3. Monitoring Available

- [x] Cache operation metrics (hit/miss)
- [x] Basic Prometheus metrics
- [x] Structured JSON logs
- [ ] Response time percentiles
- [ ] Database query performance
- [ ] Detailed error tracking
- [ ] Infrastructure metrics

## Realistic Performance Expectations

### Current Setup (Single EC2)

| Component | Realistic Expectation | Notes |
|-----------|---------------------|-------|
| API Response | 50-200ms | Depends on operation |
| WebSocket Latency | 10-50ms | Network dependent |
| Database Queries | 10-100ms | No optimization |
| Cache Hit Rate | 70-90% | With multi-level cache |
| Concurrent Users | 100-500 | Single instance limit |
| Availability | 95-99% | No redundancy |

## Next Steps

1. Review [Monitoring Guide](../monitoring/MONITORING.md) for observability setup
2. Check [Cost Optimization Guide](./cost-optimization-guide.md) for cost-efficient performance
3. See [Scaling Guide](./scaling-guide.md) for growth strategies
4. Explore [Database Optimization](./database-optimization.md) for deeper DB tuning

## Resources

- [Go Performance Best Practices](https://go.dev/doc/perf)
- [PostgreSQL Performance Tuning](https://www.postgresql.org/docs/current/performance-tips.html)
- [AWS Performance Efficiency Pillar](https://docs.aws.amazon.com/wellarchitected/latest/performance-efficiency-pillar/welcome.html)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)