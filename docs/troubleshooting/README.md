# Troubleshooting Guide

## Overview

This guide provides solutions for common issues, debugging procedures, and error resolution strategies for Developer Mesh. It's organized by symptoms to help you quickly identify and resolve problems.

## Table of Contents

1. [Common Issues](#common-issues)
2. [Service-Specific Issues](#service-specific-issues)
3. [Authentication & Authorization](#authentication--authorization)
4. [Database Issues](#database-issues)
5. [Performance Problems](#performance-problems)
6. [Network & Connectivity](#network--connectivity)
7. [Integration Issues](#integration-issues)
8. [Debugging Tools & Techniques](#debugging-tools--techniques)
9. [Error Reference](#error-reference)

## Common Issues

### Service Won't Start

**Symptoms:**
- Service fails to start
- Container crash loop
- Exit code 1 or 2

**Diagnosis:**
```bash
# Check container status
docker-compose ps

# Check logs
docker-compose logs -f <service-name> --tail=100

# Check specific service
docker inspect <container-name>
```

**Common Causes & Solutions:**

1. **Configuration Error**
   ```bash
   # Check configuration
   cat configs/config.development.yaml
   
   # Test configuration by starting with verbose logging
   LOG_LEVEL=debug ./edge-mcp --config=configs/config.development.yaml
   ```

2. **Database Connection Failed**
   ```bash
   # Test database connectivity
   docker-compose exec edge-mcp \
     psql -h postgres -U devmesh -d devmesh_development -c "SELECT 1"
   
   # Check database credentials in environment
   docker-compose config | grep DATABASE_URL
   ```

3. **Missing Environment Variables**
   ```bash
   # List all env vars
   docker-compose exec edge-mcp env | sort
   
   # Add missing vars in docker-compose.yml or .env file
   echo "NEW_VAR=value" >> .env.development
   docker-compose up -d
   ```

### High Memory Usage

**Symptoms:**
- OOMKilled errors
- Slow response times
- Memory alerts

**Diagnosis:**
```bash
# Check memory usage
docker stats --no-stream

# Get memory profile
docker-compose exec edge-mcp \
  curl http://localhost:6060/debug/pprof/heap > heap.prof

# Analyze profile
go tool pprof -http=:8080 heap.prof
```

**Solutions:**

1. **Increase Memory Limits**
   ```yaml
   # In docker-compose.yml
   services:
     edge-mcp:
       mem_limit: 2g
       mem_reservation: 1g
   ```

2. **Fix Memory Leaks**
   ```go
   // Common leak: not closing resources
   defer func() {
       if err := resource.Close(); err != nil {
           log.Printf("Failed to close resource: %v", err)
       }
   }()
   ```

3. **Tune Garbage Collection**
   ```bash
   # In docker-compose.yml or .env file
   environment:
     - GOGC=50
   # Then restart
   docker-compose restart edge-mcp
   ```

### API Timeout Errors

**Symptoms:**
- 504 Gateway Timeout
- Context deadline exceeded
- Client timeout errors

**Diagnosis:**
```bash
# Check slow queries
curl http://localhost:8080/debug/pprof/trace?seconds=30 > trace.out
go tool trace trace.out

# Check database slow queries
psql -h localhost -U mcp_user -d mcp -c "
  SELECT query, mean_exec_time, calls 
  FROM pg_stat_statements 
  WHERE mean_exec_time > 1000 
  ORDER BY mean_exec_time DESC 
  LIMIT 10;"
```

**Solutions:**

1. **Increase Timeouts**
   ```yaml
   # In config.yaml
   http:
     read_timeout: 60s
     write_timeout: 60s
   
   database:
     query_timeout: 30s
   ```

2. **Add Indexes**
   ```sql
   -- Find missing indexes
   SELECT schemaname, tablename, attname, n_distinct, correlation
   FROM pg_stats
   WHERE correlation < 0.1
   ORDER BY n_distinct DESC;
   
   -- Create index
   CREATE INDEX CONCURRENTLY idx_contexts_tenant_created 
   ON contexts(tenant_id, created_at DESC);
   ```

## Service-Specific Issues

### MCP Server Issues

#### WebSocket Connection Drops

**Symptoms:**
- "WebSocket connection closed" errors
- Intermittent disconnections
- Reconnection loops

**Solutions:**

1. **Increase Keepalive Settings**
   ```yaml
   websocket:
     ping_interval: 30s
     pong_timeout: 10s
     write_timeout: 10s
   ```

2. **Configure Proxy Settings**
   ```nginx
   # nginx.conf
   location /ws {
       proxy_pass http://edge-mcp:8080;
       proxy_http_version 1.1;
       proxy_set_header Upgrade $http_upgrade;
       proxy_set_header Connection "upgrade";
       proxy_read_timeout 3600s;
       proxy_send_timeout 3600s;
   }
   ```

#### Tool Execution Failures

**Symptoms:**
- "Tool not found" errors
- "Permission denied" errors
- Tool timeouts

**Diagnosis:**
```bash
# List available tools
curl http://localhost:8080/api/v1/tools

# Check tool permissions
curl http://localhost:8080/api/v1/tools/github/permissions \
  -H "Authorization: Bearer $TOKEN"

# Check tool configuration
kubectl get configmap tool-config -n mcp-prod -o yaml
```

**Solutions:**

1. **Register Missing Tools**
   ```go
   // In tool initialization
   toolRegistry.Register("github", githubToolProvider)
   toolRegistry.Register("aws", awsToolProvider)
   ```

2. **Fix Permission Issues**
   ```sql
   -- Grant tool permissions
   INSERT INTO tool_permissions (user_id, tool_name, permission)
   VALUES ('user-123', 'github', 'execute');
   ```

### REST API Issues

#### Rate Limiting Errors

**Symptoms:**
- 429 Too Many Requests
- X-RateLimit-Remaining: 0
- Rate limit exceeded messages

**Solutions:**

1. **Check Current Limits**
   ```bash
   curl -I http://localhost:8081/api/v1/contexts \
     -H "X-API-Key: $API_KEY" | grep -i ratelimit
   ```

2. **Increase Rate Limits**
   ```yaml
   rate_limiting:
     default:
       requests_per_minute: 60
       burst: 120
     
     api_keys:
       premium:
         requests_per_minute: 1000
         burst: 2000
   ```

3. **Implement Client-Side Retry**
   ```python
   import time
   from typing import Optional
   
   def with_rate_limit_retry(func):
       def wrapper(*args, **kwargs):
           max_retries = 3
           for attempt in range(max_retries):
               try:
                   response = func(*args, **kwargs)
                   if response.status_code == 429:
                       retry_after = int(response.headers.get('Retry-After', 60))
                       time.sleep(retry_after)
                       continue
                   return response
               except Exception as e:
                   if attempt == max_retries - 1:
                       raise
           return None
       return wrapper
   ```

### Worker Service Issues

#### Queue Processing Delays

**Symptoms:**
- High queue depth
- Delayed event processing
- Worker backlogs

**Diagnosis:**
```bash
# Check Redis stream depth
redis-cli xlen webhook_events

# Check consumer group info
redis-cli xinfo groups webhook_events

# Check worker metrics
curl http://localhost:8082/metrics | grep worker_
```

**Solutions:**

1. **Scale Workers**
   ```bash
   # In docker-compose.yml, add more worker replicas
   docker-compose up -d --scale worker=10
   ```

2. **Optimize Processing**
   ```go
   // Increase concurrency
   workerPool := worker.NewPool(worker.Config{
       Concurrency: 50,  // Increased from 10
       BatchSize:   10,  // Process in batches
   })
   ```

3. **Add Circuit Breaker**
   ```go
   cb := circuitbreaker.New(circuitbreaker.Config{
       Threshold:   5,
       Timeout:     60 * time.Second,
       MaxRequests: 100,
   })
   ```

## Authentication & Authorization

### Invalid API Key Errors

**Symptoms:**
- 401 Unauthorized
- "Invalid API key" message
- "API key expired" error

**Diagnosis:**
```bash
# Check API key status
psql -h localhost -U mcp_user -d mcp -c "
  SELECT key_id, name, expires_at, revoked_at 
  FROM api_keys 
  WHERE key_hash = crypt('$API_KEY', key_hash);"

# Check API key scopes
curl http://localhost:8081/api/v1/auth/keys/validate \
  -H "X-API-Key: $API_KEY"
```

**Solutions:**

1. **Regenerate API Key**
   ```bash
   curl -X POST http://localhost:8081/api/v1/auth/keys \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "name": "new-key",
       "scopes": ["contexts:read", "contexts:write"],
       "expires_at": "2025-12-31T23:59:59Z"
     }'
   ```

2. **Fix Clock Skew**
   ```bash
   # Sync time on all nodes
   timedatectl set-ntp true
   systemctl restart systemd-timesyncd
   ```

### JWT Token Issues

**Symptoms:**
- "Token expired" errors
- "Invalid signature" errors
- "Token not yet valid" errors

**Solutions:**

1. **Refresh Token**
   ```javascript
   async function refreshAccessToken(refreshToken) {
     const response = await fetch('/api/v1/auth/refresh', {
       method: 'POST',
       headers: { 'Content-Type': 'application/json' },
       body: JSON.stringify({ refresh_token: refreshToken })
     });
     
     if (!response.ok) {
       throw new Error('Failed to refresh token');
     }
     
     const data = await response.json();
     return data.access_token;
   }
   ```

2. **Validate Token Configuration**
   ```bash
   # Check JWT secret
   kubectl get secret jwt-secret -n mcp-prod -o jsonpath='{.data.secret}' | base64 -d
   
   # Verify issuer
   jwt decode $TOKEN | jq '.iss'
   ```

### Permission Denied Errors

**Symptoms:**
- 403 Forbidden
- "Insufficient permissions" error
- "Access denied" message

**Diagnosis:**
```sql
-- Check user permissions
SELECT u.email, r.name as role, p.resource, p.action
FROM users u
JOIN user_roles ur ON u.id = ur.user_id
JOIN roles r ON ur.role_id = r.id
JOIN role_permissions rp ON r.id = rp.role_id
JOIN permissions p ON rp.permission_id = p.id
WHERE u.id = 'user-123';
```

**Solutions:**

1. **Grant Required Permissions**
   ```sql
   -- Add permission to role
   INSERT INTO role_permissions (role_id, permission_id)
   SELECT r.id, p.id
   FROM roles r, permissions p
   WHERE r.name = 'developer'
   AND p.resource = 'contexts'
   AND p.action = 'write';
   ```

2. **Update API Key Scopes**
   ```bash
   curl -X PATCH http://localhost:8081/api/v1/auth/keys/{key_id} \
     -H "Authorization: Bearer $ADMIN_TOKEN" \
     -H "Content-Type: application/json" \
     -d '{
       "scopes": ["contexts:read", "contexts:write", "tools:execute"]
     }'
   ```

## Database Issues

### Connection Pool Exhausted

**Symptoms:**
- "Too many connections" error
- "Connection pool timeout" error
- Database connection failures

**Diagnosis:**
```sql
-- Check active connections
SELECT datname, usename, count(*) 
FROM pg_stat_activity 
GROUP BY datname, usename 
ORDER BY count DESC;

-- Find long-running connections
SELECT pid, usename, datname, state, query_start, state_change, query
FROM pg_stat_activity
WHERE state != 'idle'
AND query_start < now() - interval '5 minutes'
ORDER BY query_start;
```

**Solutions:**

1. **Increase Connection Limits**
   ```yaml
   database:
     max_connections: 200
     pool_size: 50
     max_idle_connections: 10
   ```

2. **Kill Idle Connections**
   ```sql
   -- Terminate idle connections older than 1 hour
   SELECT pg_terminate_backend(pid)
   FROM pg_stat_activity
   WHERE state = 'idle'
   AND state_change < now() - interval '1 hour';
   ```

3. **Implement Connection Pooling**
   ```go
   db.SetMaxOpenConns(25)
   db.SetMaxIdleConns(5)
   db.SetConnMaxLifetime(5 * time.Minute)
   ```

### Database Performance Issues

**Symptoms:**
- Slow queries
- High CPU usage on database
- Lock contention

**Diagnosis:**
```sql
-- Enable query logging
ALTER SYSTEM SET log_min_duration_statement = 1000;
SELECT pg_reload_conf();

-- Check for missing indexes
SELECT 
  schemaname,
  tablename,
  attname,
  n_distinct,
  correlation
FROM pg_stats
WHERE schemaname NOT IN ('pg_catalog', 'information_schema')
  AND n_distinct > 100
  AND correlation < 0.1
ORDER BY n_distinct DESC;

-- Find blocking queries
SELECT 
  blocked_locks.pid AS blocked_pid,
  blocked_activity.usename AS blocked_user,
  blocking_locks.pid AS blocking_pid,
  blocking_activity.usename AS blocking_user,
  blocked_activity.query AS blocked_query,
  blocking_activity.query AS blocking_query
FROM pg_catalog.pg_locks blocked_locks
JOIN pg_catalog.pg_stat_activity blocked_activity ON blocked_activity.pid = blocked_locks.pid
JOIN pg_catalog.pg_locks blocking_locks ON blocking_locks.locktype = blocked_locks.locktype
  AND blocking_locks.database IS NOT DISTINCT FROM blocked_locks.database
  AND blocking_locks.relation IS NOT DISTINCT FROM blocked_locks.relation
  AND blocking_locks.page IS NOT DISTINCT FROM blocked_locks.page
  AND blocking_locks.tuple IS NOT DISTINCT FROM blocked_locks.tuple
  AND blocking_locks.virtualxid IS NOT DISTINCT FROM blocked_locks.virtualxid
  AND blocking_locks.transactionid IS NOT DISTINCT FROM blocked_locks.transactionid
  AND blocking_locks.classid IS NOT DISTINCT FROM blocked_locks.classid
  AND blocking_locks.objid IS NOT DISTINCT FROM blocked_locks.objid
  AND blocking_locks.objsubid IS NOT DISTINCT FROM blocked_locks.objsubid
  AND blocking_locks.pid != blocked_locks.pid
JOIN pg_catalog.pg_stat_activity blocking_activity ON blocking_activity.pid = blocking_locks.pid
WHERE NOT blocked_locks.granted;
```

**Solutions:**

1. **Add Missing Indexes**
   ```sql
   -- Create index for common queries
   CREATE INDEX CONCURRENTLY idx_contexts_tenant_created 
   ON contexts(tenant_id, created_at DESC);
   
   CREATE INDEX CONCURRENTLY idx_vector_embeddings_similarity 
   ON vector_embeddings USING ivfflat (embedding vector_cosine_ops);
   ```

2. **Optimize Queries**
   ```sql
   -- Use EXPLAIN ANALYZE
   EXPLAIN (ANALYZE, BUFFERS) 
   SELECT * FROM contexts 
   WHERE tenant_id = 'tenant-123' 
   ORDER BY created_at DESC 
   LIMIT 100;
   ```

3. **Vacuum and Analyze**
   ```sql
   -- Manual vacuum
   VACUUM ANALYZE contexts;
   VACUUM ANALYZE vector_embeddings;
   
   -- Update statistics
   ANALYZE;
   ```

## Performance Problems

### High CPU Usage

**Symptoms:**
- CPU constantly above 80%
- Slow response times
- System feels sluggish

**Diagnosis:**
```bash
# CPU profiling
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
go tool pprof -http=:8080 cpu.prof

# Check goroutines
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof
go tool pprof -http=:8080 goroutine.prof
```

**Solutions:**

1. **Optimize Hot Paths**
   ```go
   // Use sync.Pool for frequently allocated objects
   var bufferPool = sync.Pool{
       New: func() interface{} {
           return new(bytes.Buffer)
       },
   }
   
   func processRequest(data []byte) {
       buf := bufferPool.Get().(*bytes.Buffer)
       defer func() {
           buf.Reset()
           bufferPool.Put(buf)
       }()
       // Use buffer
   }
   ```

2. **Reduce Lock Contention**
   ```go
   // Use sharded maps for high-concurrency scenarios
   type ShardedMap struct {
       shards []*sync.Map
       hash   func(key string) uint32
   }
   
   func (sm *ShardedMap) Get(key string) (interface{}, bool) {
       shard := sm.hash(key) % uint32(len(sm.shards))
       return sm.shards[shard].Load(key)
   }
   ```

### Memory Leaks

**Symptoms:**
- Gradually increasing memory usage
- OOM kills after running for a while
- Memory not released after load decreases

**Diagnosis:**
```bash
# Compare heap profiles
curl http://localhost:6060/debug/pprof/heap > heap1.prof
# Wait 10 minutes
curl http://localhost:6060/debug/pprof/heap > heap2.prof

# Compare
go tool pprof -base heap1.prof heap2.prof
```

**Common Leak Patterns:**

1. **Goroutine Leaks**
   ```go
   // BAD: Goroutine leak
   func processAsync(ch chan data) {
       go func() {
           for item := range ch {
               process(item)
           }
       }()
   }
   
   // GOOD: Proper cleanup
   func processAsync(ctx context.Context, ch chan data) {
       go func() {
           for {
               select {
               case item := <-ch:
                   process(item)
               case <-ctx.Done():
                   return
               }
           }
       }()
   }
   ```

2. **Unclosed Resources**
   ```go
   // Always use defer for cleanup
   resp, err := http.Get(url)
   if err != nil {
       return err
   }
   defer resp.Body.Close()
   ```

## Network & Connectivity

### DNS Resolution Issues

**Symptoms:**
- "No such host" errors
- Service discovery failures
- Intermittent connection errors

**Diagnosis:**
```bash
# Check DNS resolution
docker-compose exec edge-mcp nslookup postgres
docker-compose exec edge-mcp nslookup redis

# Check Docker network
docker network ls
docker network inspect devops-mcp_default
```

**Solutions:**

1. **Fix Docker Network**
   ```bash
   # Recreate network if needed
   docker-compose down
   docker network prune
   docker-compose up -d
   ```

2. **Use Container Names**
   ```yaml
   # In configuration, use service names from docker-compose.yml
   database:
     host: postgres  # Service name from docker-compose
   redis:
     addr: redis:6379
   ```

### Load Balancer Issues (Production with Nginx)

**Symptoms:**
- Uneven traffic distribution
- Some containers not receiving traffic
- Connection refused errors

**Diagnosis:**
```bash
# Check container health
docker-compose ps

# Check nginx configuration (if using)
cat deployments/nginx/mcp.conf

# Test load balancing
for i in {1..20}; do
  curl -s http://localhost:8080/health | jq -r '.instance'
done | sort | uniq -c
```

**Solutions:**

1. **Fix Nginx Configuration**
   ```nginx
   # In nginx.conf
   upstream mcp_backend {
       server edge-mcp:8080;
       keepalive 32;
   }
   
   location / {
       proxy_pass http://mcp_backend;
       proxy_http_version 1.1;
       proxy_set_header Connection "";
   }
   ```

2. **Scale Services**
   ```bash
   # Scale up services
   docker-compose up -d --scale edge-mcp=3
   ```

## Integration Issues

### GitHub Integration Failures

**Symptoms:**
- Webhook delivery failures
- API rate limit exceeded
- Authentication errors

**Diagnosis:**
```bash
# Check webhook deliveries
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/repos/owner/repo/hooks/123/deliveries

# Check rate limits
curl -H "Authorization: token $GITHUB_TOKEN" \
  https://api.github.com/rate_limit
```

**Solutions:**

1. **Fix Webhook Secret**
   ```bash
   # Update webhook secret
   kubectl create secret generic github-webhook \
     --from-literal=secret=$WEBHOOK_SECRET \
     -n mcp-prod --dry-run=client -o yaml | kubectl apply -f -
   ```

2. **Implement Exponential Backoff**
   ```go
   func callGitHubAPI(ctx context.Context, endpoint string) (*Response, error) {
       backoff := 1 * time.Second
       maxBackoff := 5 * time.Minute
       
       for i := 0; i < 5; i++ {
           resp, err := makeRequest(ctx, endpoint)
           if err == nil {
               return resp, nil
           }
           
           if resp != nil && resp.StatusCode == 429 {
               retryAfter := resp.Header.Get("Retry-After")
               if seconds, err := strconv.Atoi(retryAfter); err == nil {
                   backoff = time.Duration(seconds) * time.Second
               }
           }
           
           select {
           case <-time.After(backoff):
               backoff = min(backoff*2, maxBackoff)
           case <-ctx.Done():
               return nil, ctx.Err()
           }
       }
       
       return nil, fmt.Errorf("max retries exceeded")
   }
   ```

### AWS Integration Issues

**Symptoms:**
- S3 access denied
- Bedrock API failures
- IAM permission errors

**Solutions:**

1. **Check AWS Credentials**
   ```bash
   # Verify AWS credentials are set
   docker-compose exec edge-mcp env | grep AWS
   
   # Test AWS access
   docker-compose exec edge-mcp \
     aws s3 ls --debug
   ```

2. **Fix Environment Variables**
   ```bash
   # Add to .env file
   AWS_ACCESS_KEY_ID=your-key
   AWS_SECRET_ACCESS_KEY=your-secret
   AWS_REGION=us-east-1
   
   # Restart services
   docker-compose restart
   ```

## Debugging Tools & Techniques

### Enable Debug Logging

```bash
# Runtime log level change
curl -X PUT http://localhost:8080/admin/log-level \
  -H "Content-Type: application/json" \
  -d '{"level": "debug"}'

# Environment variable
kubectl set env deployment/edge-mcp LOG_LEVEL=debug -n mcp-prod
```

### Distributed Tracing (If Configured)

```bash
# NOTE: Jaeger is not deployed by default
# If you have enabled OpenTelemetry tracing:

# Find slow traces (requires Jaeger)
curl "http://localhost:16686/api/traces?service=edge-mcp&minDuration=1s&limit=20"

# Get specific trace
curl "http://localhost:16686/api/traces/{traceID}"
```

### Performance Analysis

```bash
#!/bin/bash
# Performance debugging script

# CPU profile
echo "Collecting CPU profile..."
curl -s http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof

# Memory profile
echo "Collecting memory profile..."
curl -s http://localhost:6060/debug/pprof/heap > heap.prof

# Goroutine profile
echo "Collecting goroutine profile..."
curl -s http://localhost:6060/debug/pprof/goroutine > goroutine.prof

# Block profile
echo "Collecting block profile..."
curl -s http://localhost:6060/debug/pprof/block > block.prof

# Analyze
echo "Starting analysis server on :8080"
go tool pprof -http=:8080 cpu.prof
```

### Request Tracing

```go
// Add request tracing middleware
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        traceID := c.GetHeader("X-Trace-ID")
        if traceID == "" {
            traceID = uuid.New().String()
        }
        
        logger := log.With().
            Str("trace_id", traceID).
            Str("method", c.Request.Method).
            Str("path", c.Request.URL.Path).
            Logger()
        
        c.Set("logger", logger)
        c.Set("trace_id", traceID)
        c.Header("X-Trace-ID", traceID)
        
        start := time.Now()
        c.Next()
        
        logger.Info().
            Int("status", c.Writer.Status()).
            Dur("latency", time.Since(start)).
            Msg("Request completed")
    }
}
```

## Error Reference

### Common Error Categories

**Note**: The error codes below are documentation references for categorizing issues. The actual application returns standard HTTP status codes (401, 403, 500, etc.) and descriptive error messages in logs.

| Category | Description | Solution |
|------------|-------------|----------|
| Database Connection | Database connection failed | Check database credentials and connectivity |
| Redis Connection | Redis connection failed | Verify Redis is running and accessible |
| Configuration | Configuration invalid | Validate configuration file syntax |
| Tool Not Found | Tool not found | Ensure tool is registered and enabled |
| Authentication | Authentication failed | Check credentials and token validity |
| Rate Limiting | Rate limit exceeded | Wait for rate limit reset or upgrade plan |
| Context Not Found | Context not found | Verify context ID and tenant access |
| Webhook Validation | Webhook validation failed | Check webhook secret configuration |
| Embedding Failure | Vector embedding failed | Verify embedding service is available |
| Queue Processing | Queue processing error | Check Redis Streams and worker connectivity |

### Error Message Patterns

```bash
# Search for specific errors in logs
docker-compose logs edge-mcp | grep -E "ERROR|FATAL|panic"

# Count error types
docker-compose logs --since=1h edge-mcp | \
  grep ERROR | \
  awk -F'"error":"' '{print $2}' | \
  awk -F'"' '{print $1}' | \
  sort | uniq -c | sort -nr
```

### Panic Recovery

```go
// Global panic recovery
func RecoveryMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        defer func() {
            if err := recover(); err != nil {
                // Log panic
                log.Error().
                    Interface("error", err).
                    Str("stack", string(debug.Stack())).
                    Msg("Panic recovered")
                
                // Return error response
                c.JSON(500, gin.H{
                    "error": "Internal server error",
                    "code": "MCP-PANIC",
                    "trace_id": c.GetString("trace_id"),
                })
                
                c.Abort()
            }
        }()
        
        c.Next()
    }
}
```

## Getting Help

If you can't resolve an issue using this guide:

1. **Check Logs**: Always check logs first for error details
2. **Search Issues**: Look for similar issues on GitHub
3. **Community Support**: Join our Discord channel
4. **Create Issue**: File a detailed bug report with:
   - Error messages
   - Steps to reproduce
   - Environment details
   - Relevant logs

Remember to sanitize any sensitive information before sharing logs or configuration files.

Last Updated: $(date)
Version: 1.0.0