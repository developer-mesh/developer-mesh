<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:40:39
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Debugging Guide

Comprehensive debugging strategies for the Developer Mesh platform, from development to production.

## Quick Debugging Commands

```bash
# Health check all services
make health-check

# View real-time logs
make logs-follow service=mcp-server

# Debug mode with verbose logging
LOG_LEVEL=debug make run service=mcp-server

# Interactive debugging
make debug service=rest-api
```

## Debugging Setup

### 1. Enable Debug Mode

```yaml
# configs/config.yaml
logging:
  level: debug
  format: json
  output: stdout
  
debugging:
  enabled: true
  pprof: true
  trace: true
  metrics: true
```

### 2. Observability Stack

Note: Full observability stack (Grafana, Prometheus, Jaeger) is not currently configured in the project. For production monitoring, these tools would need to be set up separately.

### 3. Debug Containers

```dockerfile
# Dockerfile.debug (example, not currently in project)
FROM golang:1.24-alpine AS debug
RUN go install github.com/go-delve/delve/cmd/dlv@latest
WORKDIR /app
COPY . .
RUN go build -gcflags="all=-N -l" -o app ./cmd/server
EXPOSE 8080 2345
CMD ["dlv", "--listen=:2345", "--headless=true", "--api-version=2", "exec", "./app"]
```

## Common Issues & Solutions

### 1. Go Workspace Issues

#### Symptoms
- `module not found` errors
- `ambiguous import` errors
- `cannot find package` in workspace

#### Solutions

```bash
# Fix 1: Sync workspace
go work sync

# Fix 2: Clean module cache
go clean -modcache
go mod download

# Fix 3: Verify workspace
go work edit -json | jq '.Use[].DiskPath'

# Fix 4: Rebuild workspace
rm go.work go.work.sum
go work init
go work use -r .
```

### 2. Interface & Type Issues

#### Symptoms
- `type does not implement interface`
- `cannot use X as Y in argument`
- Method signature mismatches

#### Debugging

```go
// Use compiler to show missing methods
var _ interfaces.Adapter = (*MyAdapter)(nil)

// Print actual vs expected types
fmt.Printf("Expected: %T, Got: %T\n", expected, actual)

// Use reflection for runtime checks
if !reflect.TypeOf(actual).Implements(reflect.TypeOf((*Expected)(nil)).Elem()) {
    panic("type does not implement interface")
}
```

#### Common Fixes

```go
// Fix 1: Pointer vs value receivers
func (a *Adapter) Method() {} // Pointer receiver
func (a Adapter) Method() {}  // Value receiver

// Fix 2: Return types
func (a *Adapter) Get() (interface{}, error) { // Wrong
    return &Model{}, nil
}

func (a *Adapter) Get() (*Model, error) { // Correct
    return &Model{}, nil
}

// Fix 3: Context parameter
func (a *Adapter) Execute(req Request) error {} // Wrong
func (a *Adapter) Execute(ctx context.Context, req Request) error {} // Correct
```

### 3. Testing & Mock Issues

#### Symptoms
- `mock: Unexpected Method Call`
- `argument does not match`
- Race conditions in tests

#### Advanced Mock Techniques

```go
// Custom matcher for complex types
type vectorMatcher struct {
    expected []float32
    tolerance float32
}

func (m vectorMatcher) Match(actual interface{}) bool {
    vec, ok := actual.([]float32)
    if !ok || len(vec) != len(m.expected) {
        return false
    }
    for i := range vec {
        if math.Abs(float64(vec[i]-m.expected[i])) > float64(m.tolerance) {
            return false
        }
    }
    return true
}

// Use in test
mockRepo.On("Store", mock.Anything, vectorMatcher{
    expected: []float32{0.1, 0.2, 0.3},
    tolerance: 0.001,
}).Return(nil)

// Debug mock calls
mockRepo.On("Method", mock.Anything).Run(func(args mock.Arguments) {
    // Print actual arguments
    fmt.Printf("Called with: %+v\n", args)
}).Return(nil)

// Assert specific calls
mockRepo.AssertCalled(t, "Method", mock.MatchedBy(func(req interface{}) bool {
    r, ok := req.(*Request)
    return ok && r.ID == "expected-id"
}))
```

#### Race Condition Detection

```bash
# Run tests with race detector
go test -race -count=10 ./...

# Fix common race conditions
# 1. Shared state in tests
t.Parallel() // Use carefully

# 2. Goroutine leaks
defer goleak.VerifyNone(t)

# 3. Context cancellation
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()
```

### 4. Vector Search & pgvector Issues

#### Symptoms
- `operator does not exist: vector <-> unknown`
- Slow vector searches
- Incorrect similarity scores
- Dimension mismatch errors

#### Debugging pgvector

```sql
-- Check pgvector installation
SELECT * FROM pg_extension WHERE extname = 'vector';

-- Verify vector dimensions
SELECT 
    id,
    array_length(embedding, 1) as dimensions,
    model_id
FROM embeddings
GROUP BY model_id, array_length(embedding, 1), id;

-- Debug similarity search
EXPLAIN ANALYZE
SELECT 
    id,
    1 - (embedding <=> '[0.1, 0.2, ...]'::vector) as similarity
FROM embeddings
WHERE 1 - (embedding <=> '[0.1, 0.2, ...]'::vector) > 0.7
ORDER BY embedding <=> '[0.1, 0.2, ...]'::vector
LIMIT 10;

-- Check index usage
SELECT 
    schemaname,
    tablename,
    indexname,
    idx_scan,
    idx_tup_read
FROM pg_stat_user_indexes
WHERE tablename = 'embeddings';
```

#### Performance Optimization

```go
// Enable query logging
db.LogMode(true)

// Profile slow queries
start := time.Now()
results, err := repo.Search(ctx, query)
logger.Info("search completed",
    "duration", time.Since(start),
    "results", len(results),
)

// Batch operations
const batchSize = 100
for i := 0; i < len(embeddings); i += batchSize {
    end := min(i+batchSize, len(embeddings))
    batch := embeddings[i:end]
    if err := repo.BatchStore(ctx, batch); err != nil {
        return fmt.Errorf("batch %d-%d: %w", i, end, err)
    }
}
```

### 5. Container & Networking Issues

#### Symptoms
- `connection refused` errors
- `no such host` errors
- Services can't communicate
- Health checks failing

#### Docker Debugging

```bash
# 1. Check container networking
docker network ls
docker network inspect developer-mesh_default

# 2. Test connectivity
docker exec mcp-server ping postgres
docker exec mcp-server nc -zv postgres 5432

# 3. View container details
docker inspect mcp-server | jq '.[0].NetworkSettings'

# 4. Debug DNS
docker exec mcp-server nslookup postgres
docker exec mcp-server cat /etc/resolv.conf

# 5. Port mapping issues
docker port mcp-server
netstat -tlnp | grep 8080
```

#### Health Check Debugging

```yaml
# docker-compose.yml
healthcheck:
  test: ["CMD", "curl", "-f", "http://localhost:8080/health"]
  interval: 30s
  timeout: 10s
  retries: 3
  start_period: 40s
```

```bash
# Check health status
docker inspect mcp-server | jq '.[0].State.Health'

# View health check logs
docker inspect mcp-server | jq '.[0].State.Health.Log'

# Manual health check
docker exec mcp-server curl -v http://localhost:8080/health
```

## Advanced Debugging Tools

### 1. Delve Debugger

```bash
# Remote debugging setup
dlv debug --headless --listen=:2345 --api-version=2 ./cmd/server

# Connect from VS Code
# launch.json
{
    "name": "Connect to server",
    "type": "go",
    "request": "attach",
    "mode": "remote",
    "remotePath": "${workspaceFolder}",
    "port": 2345,
    "host": "localhost"
}

# Debug tests
dlv test -- -test.run TestSpecific

# Conditional breakpoints
(dlv) break main.go:42 if user.ID == "123"

# Watch expressions
(dlv) watch -x (*s).config.URL

# Print goroutines
(dlv) goroutines
(dlv) goroutine 12
(dlv) stack
```

### 2. Performance Profiling

```bash
# CPU profiling
go test -cpuprofile=cpu.prof -bench=.
go tool pprof -http=:8080 cpu.prof

# Memory profiling
go test -memprofile=mem.prof -bench=.
go tool pprof -alloc_space -http=:8080 mem.prof

# Live profiling via pprof
curl http://localhost:6060/debug/pprof/profile?seconds=30 > cpu.prof
curl http://localhost:6060/debug/pprof/heap > heap.prof
curl http://localhost:6060/debug/pprof/goroutine > goroutine.prof

# Trace analysis
go test -trace=trace.out
go tool trace trace.out
```

### 3. Distributed Tracing

```go
// Add tracing to requests
ctx, span := tracer.Start(ctx, "operation.name",
    trace.WithAttributes(
        attribute.String("user.id", userID),
        attribute.Int("request.size", len(data)),
    ),
)
defer span.End()

// Add events
span.AddEvent("processing started")

// Record errors
if err != nil {
    span.RecordError(err)
    span.SetStatus(codes.Error, err.Error())
}
```

## API Testing & Debugging

### 1. HTTP Request Debugging

```bash
# Verbose curl with timing
curl -w "@curl-format.txt" -v -X POST http://localhost:8080/api/v1/tools/execute \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -H "X-Request-ID: $(uuidgen)" \
  -d @request.json

# curl-format.txt
time_namelookup:  %{time_namelookup}\n
time_connect:     %{time_connect}\n
time_appconnect:  %{time_appconnect}\n
time_pretransfer: %{time_pretransfer}\n
time_redirect:    %{time_redirect}\n
time_starttransfer: %{time_starttransfer}\n
                    ----------\n
time_total:       %{time_total}\n

# Using httpie
http -v POST localhost:8081/api/embeddings \
  Authorization:"Bearer $API_KEY" \
  agent_id=test-agent \
  text="Sample text to embed"

# Test WebSocket connection with wscat <!-- Source: pkg/models/websocket/binary.go -->
wscat -c ws://localhost:8080/v1/mcp -s mcp.v1 \
  -H "Authorization: Bearer $API_KEY"

# Test with cURL (WebSocket upgrade) <!-- Source: pkg/models/websocket/binary.go -->
curl -i -N \
  -H "Connection: Upgrade" \
  -H "Upgrade: websocket" \ <!-- Source: pkg/models/websocket/binary.go -->
  -H "Sec-WebSocket-Version: 13" \ <!-- Source: pkg/models/websocket/binary.go -->
  -H "Sec-WebSocket-Key: SGVsbG8sIHdvcmxkIQ==" \ <!-- Source: pkg/models/websocket/binary.go -->
  -H "Sec-WebSocket-Protocol: mcp.v1" \ <!-- Source: pkg/models/websocket/binary.go -->
  http://localhost:8080/v1/mcp
```

### 2. Load Testing

```bash
# Using vegeta
echo "POST http://localhost:8081/api/embeddings/search" | \
  vegeta attack -duration=30s -rate=100 -body=search.json | \
  vegeta report

# Using k6
k6 run --vus=10 --duration=30s load-test.js

# load-test.js
import http from 'k6/http';
import { check } from 'k6';

export default function() {
  const res = http.post('http://localhost:8081/api/embeddings/search', 
    JSON.stringify({
      agent_id: 'test-agent',
      query: 'Sample search query',
      limit: 10
    }),
    { headers: { 'Content-Type': 'application/json' }}
  );
  
  check(res, {
    'status is 200': (r) => r.status === 200,
    'response time < 500ms': (r) => r.timings.duration < 500,
  });
}
```

## Production Debugging

### 1. Structured Logging

```go
// Use structured logging for better debugging
logger.Error("operation failed",
    zap.String("operation", "create_build"),
    zap.String("project_id", projectID),
    zap.Error(err),
    zap.Duration("duration", time.Since(start)),
    zap.Stack("stacktrace"),
)

// Query logs in production
jq 'select(.level=="error" and .operation=="create_build")' logs.json
```

### 2. Debug Endpoints

```go
// Add debug endpoints (protect in production!)
if config.Debug.Enabled {
    router.GET("/debug/config", handleDebugConfig)
    router.GET("/debug/connections", handleDebugConnections)
    router.GET("/debug/goroutines", handleDebugGoroutines)
}

// Graceful degradation
func handleDebugConnections(c *gin.Context) {
    status := map[string]interface{}{
        "database": checkDatabase(),
        "redis": checkRedis(),
        "github": checkGitHubAPI(),
        "timestamp": time.Now(),
    }
    c.JSON(200, status)
}
```

### 3. Circuit Breaker Debugging

```go
// Monitor circuit breaker state
metrics.Gauge("circuit_breaker_state", float64(cb.State()), map[string]string{
    "service": "github",
    "operation": "create_issue",
})

// Log state changes
cb.OnStateChange = func(from, to gobreaker.State) {
    logger.Warn("circuit breaker state changed",
        "from", from.String(),
        "to", to.String(),
        "counts", cb.Counts(),
    )
}
```

## Troubleshooting Checklist

### Service Won't Start
- [ ] Check port availability: `lsof -i :8080 (MCP Server)`
- [ ] Verify config file: `make validate-config`
- [ ] Check permissions: `ls -la configs/`
- [ ] Review logs: `journalctl -u mcp-server -f`

### Database Issues
- [ ] Test connection: `pg_isready -h localhost -p 5432`
- [ ] Check migrations: `make migrate-status`
- [ ] Verify credentials: `make db-test-connection`
- [ ] Review slow queries: `make db-slow-queries`

### Memory Leaks
- [ ] Enable pprof: `go tool pprof http://localhost:6060/debug/pprof/heap`
- [ ] Check goroutines: `curl http://localhost:6060/debug/pprof/goroutine?debug=1`
- [ ] Monitor metrics: `make metrics | grep memory`
- [ ] Use leak detector: `go test -run=^$ -bench=. -benchmem`

## Reporting Issues

### Issue Template

```markdown
### Environment
- OS: [e.g., macOS 13.0]
- Go version: [go version]
- Docker version: [docker --version]
- Commit: [git rev-parse HEAD]

### Description
[Clear description of the issue]

### Steps to Reproduce
1. [First step]
2. [Second step]
3. [Expected vs actual result]

### Logs
```
[Relevant error messages]
```

### Additional Context
[Any other relevant information]
```

## Advanced Debugging Techniques

### Correlation IDs Across Services

```go
// Generate correlation ID at edge
func GenerateCorrelationID(ctx context.Context) context.Context {
    correlationID := uuid.New().String()
    ctx = context.WithValue(ctx, "correlation_id", correlationID)
    
    // Add to trace baggage
    bag, _ := baggage.Parse(fmt.Sprintf("correlation_id=%s", correlationID))
    ctx = baggage.ContextWithBaggage(ctx, bag)
    
    return ctx
}

// Extract in downstream services
func GetCorrelationID(ctx context.Context) string {
    // Try context value first
    if id, ok := ctx.Value("correlation_id").(string); ok {
        return id
    }
    
    // Try baggage
    if bag := baggage.FromContext(ctx); bag != nil {
        if member := bag.Member("correlation_id"); member.Key() != "" {
            return member.Value()
        }
    }
    
    // Try trace ID as fallback
    span := trace.SpanFromContext(ctx)
    return span.SpanContext().TraceID().String()
}
```

### Debug Endpoints

```go
// Add comprehensive debug endpoints
func RegisterDebugEndpoints(router *gin.Engine) {
    debug := router.Group("/debug")
    debug.Use(RequireDebugMode())
    
    // WebSocket debugging <!-- Source: pkg/models/websocket/binary.go -->
    debug.GET("/websocket/connections", handleWebSocketConnections) <!-- Source: pkg/models/websocket/binary.go -->
    debug.GET("/websocket/stats", handleWebSocketStats) <!-- Source: pkg/models/websocket/binary.go -->
    debug.GET("/websocket/messages/:agent_id", handleWebSocketMessages) <!-- Source: pkg/models/websocket/binary.go -->
    
    // Trace debugging
    debug.GET("/traces/current", handleCurrentTraces)
    debug.GET("/traces/slow", handleSlowTraces)
    debug.POST("/traces/analyze", handleTraceAnalysis)
    
    // AWS service debugging
    debug.GET("/aws/costs", handleAWSCosts)
    debug.GET("/aws/quotas", handleAWSQuotas)
    debug.GET("/aws/errors", handleAWSErrors)
    
    // System debugging
    debug.GET("/goroutines", handleGoroutines)
    debug.GET("/connections", handleConnections)
    debug.GET("/cache/stats", handleCacheStats)
}
```

---

Last Updated: 2024-01-10
Version: 2.0.0
