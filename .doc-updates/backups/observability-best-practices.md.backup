# Observability Best Practices

> **Purpose**: Guidelines for implementing effective observability in the Developer Mesh platform
> **Audience**: Developers adding instrumentation to services
> **Scope**: When and how to add spans, metrics, and logs for maximum insight with minimal overhead

## Table of Contents

1. [Overview](#overview)
2. [The Three Pillars](#the-three-pillars)
3. [When to Add Observability](#when-to-add-observability)
4. [Spans Best Practices](#spans-best-practices)
5. [Metrics Best Practices](#metrics-best-practices)
6. [Logging Best Practices](#logging-best-practices)
7. [Correlation and Context](#correlation-and-context)
8. [Performance Considerations](#performance-considerations)
9. [Cost Management](#cost-management)
10. [Common Patterns](#common-patterns)

## Overview

Effective observability is crucial for understanding system behavior, debugging issues, and optimizing performance. This guide provides practical guidelines for implementing observability in the Developer Mesh platform.

### Core Principles

1. **Purpose-Driven**: Every span, metric, and log should serve a specific purpose
2. **Context-Rich**: Include enough context to understand what happened
3. **Performance-Aware**: Minimize overhead on production systems
4. **Cost-Conscious**: Balance visibility needs with operational costs
5. **Actionable**: Focus on data that leads to insights and actions

## The Three Pillars

### Understanding When to Use Each

```
┌─────────────────────────────────────────────────────────────┐
│                   Observability Decision Tree                │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Need to track request flow? ──────► Use Traces/Spans      │
│                                                             │
│  Need to measure trends? ──────────► Use Metrics           │
│                                                             │
│  Need detailed context? ───────────► Use Logs              │
│                                                             │
│  Need all three? ──────────────────► Correlate them!       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### Quick Reference

| Use Case | Traces | Metrics | Logs |
|----------|--------|---------|------|
| Debug slow requests | ✓✓✓ | ✓ | ✓ |
| Monitor error rates | ✓ | ✓✓✓ | ✓ |
| Track business KPIs | | ✓✓✓ | |
| Audit user actions | ✓ | | ✓✓✓ |
| Capacity planning | | ✓✓✓ | |
| Root cause analysis | ✓✓✓ | ✓ | ✓✓ |

## When to Add Observability

### Always Instrument

```go
// 1. Service boundaries
func (h *Handler) HandleRequest(w http.ResponseWriter, r *http.Request) {
    ctx, span := tracer.Start(r.Context(), "HandleRequest",
        trace.WithSpanKind(trace.SpanKindServer))
    defer span.End()
    
    logger.InfoContext(ctx, "request started",
        slog.String("method", r.Method),
        slog.String("path", r.URL.Path))
    
    requestsTotal.Inc()
    // ... handle request
}

// 2. External calls
func (c *Client) CallExternalAPI(ctx context.Context, request *Request) (*Response, error) {
    ctx, span := tracer.Start(ctx, "CallExternalAPI",
        trace.WithSpanKind(trace.SpanKindClient))
    defer span.End()
    
    start := time.Now()
    response, err := c.doCall(ctx, request)
    
    externalCallDuration.WithLabelValues("api", statusFromError(err)).Observe(time.Since(start).Seconds())
    
    if err != nil {
        span.RecordError(err)
        logger.ErrorContext(ctx, "external API call failed",
            slog.String("api", c.apiName),
            slog.Error(err))
    }
    
    return response, err
}

// 3. Critical business operations
func (s *Service) ProcessPayment(ctx context.Context, payment *Payment) error {
    ctx, span := tracer.Start(ctx, "ProcessPayment")
    defer span.End()
    
    span.SetAttributes(
        attribute.String("payment.id", payment.ID),
        attribute.Float64("payment.amount", payment.Amount),
        attribute.String("payment.currency", payment.Currency),
    )
    
    logger.InfoContext(ctx, "processing payment",
        slog.String("payment_id", payment.ID),
        slog.Float64("amount", payment.Amount))
    
    paymentProcessed.WithLabelValues(payment.Currency).Inc()
    paymentAmount.WithLabelValues(payment.Currency).Add(payment.Amount)
    
    return s.process(ctx, payment)
}

// 4. Async operations
func (w *Worker) ProcessTask(ctx context.Context, task *Task) error {
    // Link to parent trace
    ctx, span := tracer.Start(ctx, "ProcessTask",
        trace.WithLinks(trace.LinkFromContext(task.ParentContext)))
    defer span.End()
    
    taskProcessingTotal.Inc()
    timer := prometheus.NewTimer(taskProcessingDuration.WithLabelValues(task.Type))
    defer timer.ObserveDuration()
    
    return w.process(ctx, task)
}
```

### Consider Instrumenting

```go
// 1. High-frequency operations (sample or aggregate)
func (c *Cache) Get(ctx context.Context, key string) (interface{}, error) {
    // Only trace sampled requests
    if shouldTrace(ctx) {
        ctx, span := tracer.Start(ctx, "cache.Get")
        defer span.End()
        span.SetAttributes(attribute.String("cache.key", hashKey(key)))
    }
    
    // Always update metrics
    cacheRequests.WithLabelValues("get").Inc()
    
    // Only log errors
    value, err := c.backend.Get(key)
    if err != nil && err != ErrNotFound {
        logger.ErrorContext(ctx, "cache get failed",
            slog.String("key_hash", hashKey(key)),
            slog.Error(err))
    }
    
    return value, err
}

// 2. Internal computations (only if slow)
func (s *Service) calculateEmbedding(ctx context.Context, text string) ([]float32, error) {
    start := time.Now()
    embedding, err := s.embed(text)
    duration := time.Since(start)
    
    // Only create span if slow
    if duration > 100*time.Millisecond {
        _, span := tracer.Start(ctx, "calculateEmbedding",
            trace.WithTimestamp(start))
        span.End(trace.WithTimestamp(start.Add(duration)))
        span.SetAttributes(
            attribute.Int("text.length", len(text)),
            attribute.Float64("duration.ms", duration.Seconds()*1000),
        )
    }
    
    return embedding, err
}
```

### Avoid Over-Instrumenting

```go
// Bad: Too granular
func processItem(item Item) {
    _, span := tracer.Start(ctx, "processItem") // Don't trace every item
    defer span.End()
    
    validateItem(item)  // Don't create spans for these
    transformItem(item) // micro-operations
    saveItem(item)
}

// Good: Aggregate at appropriate level
func processBatch(ctx context.Context, items []Item) error {
    ctx, span := tracer.Start(ctx, "processBatch")
    defer span.End()
    
    span.SetAttributes(
        attribute.Int("batch.size", len(items)),
    )
    
    validCount := 0
    errorCount := 0
    
    for _, item := range items {
        if err := processItem(item); err != nil {
            errorCount++
        } else {
            validCount++
        }
    }
    
    span.SetAttributes(
        attribute.Int("items.processed", validCount),
        attribute.Int("items.errors", errorCount),
    )
    
    batchProcessed.Add(float64(validCount))
    batchErrors.Add(float64(errorCount))
    
    return nil
}
```

## Spans Best Practices

### Span Naming

```go
// Use consistent hierarchical naming
"service.component.operation"

// Good examples:
"mcp-server.handler.CreateContext"
"mcp-server.repository.SaveContext"
"mcp-server.cache.Get"
"rest-api.auth.ValidateToken"
"worker.bedrock.InvokeModel"

// Bad examples:
"save"           // Too generic
"CreateContext"  // Missing service context
"HTTP POST"      // Not descriptive enough
```

### Span Attributes

```go
// Essential attributes for every span
span.SetAttributes(
    // Resource identifiers
    attribute.String("service.name", "mcp-server"),
    attribute.String("service.version", "1.2.3"),
    attribute.String("service.instance", instanceID),
    
    // Request context
    attribute.String("request.id", requestID),
    attribute.String("tenant.id", tenantID),
    attribute.String("user.id", userID),
)

// Operation-specific attributes
span.SetAttributes(
    // HTTP operations
    semconv.HTTPMethodKey.String(method),
    semconv.HTTPTargetKey.String(path),
    semconv.HTTPStatusCodeKey.Int(statusCode),
    
    // Database operations
    attribute.String("db.system", "postgresql"),
    attribute.String("db.operation", "SELECT"),
    attribute.String("db.table", "contexts"),
    attribute.Int("db.rows_affected", rowCount),
    
    // AI/ML operations
    attribute.String("ai.model", "claude-3-opus"),
    attribute.Int("ai.input_tokens", inputTokens),
    attribute.Int("ai.output_tokens", outputTokens),
    attribute.Float64("ai.cost_usd", cost),
    
    // Business metrics
    attribute.String("task.type", taskType),
    attribute.String("task.status", status),
    attribute.Float64("payment.amount", amount),
)
```

### Span Events

```go
// Add events for significant moments
span.AddEvent("cache_miss", trace.WithAttributes(
    attribute.String("cache.key", key),
    attribute.String("cache.tier", "L1"),
))

span.AddEvent("retry_attempt", trace.WithAttributes(
    attribute.Int("retry.count", retryCount),
    attribute.String("retry.reason", err.Error()),
))

span.AddEvent("rate_limited", trace.WithAttributes(
    attribute.String("limit.type", "api"),
    attribute.Float64("limit.remaining", remaining),
))

// Don't add events for routine operations
// Bad: span.AddEvent("function_called")
// Bad: span.AddEvent("loop_iteration")
```

### Error Handling

```go
// Comprehensive error recording
func recordError(span trace.Span, err error, operation string) {
    span.RecordError(err, trace.WithAttributes(
        attribute.String("error.operation", operation),
        attribute.String("error.type", fmt.Sprintf("%T", err)),
    ))
    
    // Set status based on error type
    switch {
    case errors.Is(err, context.Canceled):
        span.SetStatus(codes.Error, "Cancelled")
    case errors.Is(err, context.DeadlineExceeded):
        span.SetStatus(codes.Error, "DeadlineExceeded")
    case errors.Is(err, ErrNotFound):
        span.SetStatus(codes.Error, "NotFound")
    case errors.Is(err, ErrUnauthorized):
        span.SetStatus(codes.Error, "Unauthenticated")
    case errors.Is(err, ErrForbidden):
        span.SetStatus(codes.Error, "PermissionDenied")
    case errors.Is(err, ErrRateLimit):
        span.SetStatus(codes.Error, "ResourceExhausted")
    default:
        span.SetStatus(codes.Error, "Internal")
    }
}
```

## Metrics Best Practices

### Metric Types and Usage

```go
// Counters - for rates and totals
var (
    requestsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_requests_total",
            Help: "Total number of requests processed",
        },
        []string{"method", "endpoint", "status"},
    )
    
    tasksProcessed = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_tasks_processed_total",
            Help: "Total number of tasks processed",
        },
        []string{"type", "status"},
    )
)

// Gauges - for current state
var (
    activeConnections = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_websocket_connections_active",
            Help: "Number of active WebSocket connections",
        },
        []string{"protocol_version"},
    )
    
    queueDepth = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_queue_depth",
            Help: "Current depth of task queue",
        },
        []string{"queue_name", "priority"},
    )
)

// Histograms - for distributions
var (
    requestDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 15), // 1ms to 16s
        },
        []string{"method", "endpoint"},
    )
    
    embeddingCost = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "mcp_embedding_cost_usd",
            Help:    "Cost of embedding generation in USD",
            Buckets: prometheus.ExponentialBuckets(0.0001, 10, 8), // $0.0001 to $10
        },
        []string{"model", "provider"},
    )
)

// Summaries - for percentiles (use sparingly)
var (
    taskProcessingTime = promauto.NewSummaryVec(
        prometheus.SummaryOpts{
            Name:       "mcp_task_processing_seconds",
            Help:       "Task processing time in seconds",
            Objectives: map[float64]float64{0.5: 0.05, 0.9: 0.01, 0.99: 0.001},
            MaxAge:     10 * time.Minute,
        },
        []string{"task_type"},
    )
)
```

### Metric Naming

```go
// Follow Prometheus naming conventions
// Format: namespace_subsystem_name_unit

// Good examples:
"mcp_api_requests_total"
"mcp_api_request_duration_seconds"
"mcp_websocket_connections_active"
"mcp_embedding_tokens_processed_total"
"mcp_cache_hit_ratio"
"mcp_cost_by_service_usd"

// Bad examples:
"requests"              // Missing namespace
"mcpAPIRequests"       // Wrong case
"request_time"         // Missing unit
"total_cost_in_dollars" // Unit should be suffix
```

### Label Best Practices

```go
// Keep cardinality under control
// Good: bounded, categorical labels
requestsTotal.WithLabelValues(
    r.Method,              // GET, POST, etc. (limited)
    endpoint,              // /api/v1/contexts (bounded)
    strconv.Itoa(status),  // 200, 404, 500 (limited)
).Inc()

// Bad: high cardinality labels
requestsTotal.WithLabelValues(
    userID,      // Unbounded!
    requestID,   // Unique per request!
    timestamp,   // Continuous!
).Inc()

// Use buckets for continuous values
func bucketDuration(d time.Duration) string {
    switch {
    case d < 10*time.Millisecond:
        return "10ms"
    case d < 100*time.Millisecond:
        return "100ms"
    case d < 1*time.Second:
        return "1s"
    case d < 10*time.Second:
        return "10s"
    default:
        return "slow"
    }
}
```

### Business Metrics

```go
// Track business KPIs alongside technical metrics
var (
    // Revenue metrics
    revenueTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_revenue_usd_total",
            Help: "Total revenue in USD",
        },
        []string{"product", "tier", "region"},
    )
    
    // Usage metrics
    apiCallsByTier = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_api_calls_by_tier_total",
            Help: "API calls by customer tier",
        },
        []string{"tier", "endpoint"},
    )
    
    // Cost metrics
    aiModelCosts = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_ai_model_costs_usd_total",
            Help: "Total costs for AI model usage in USD",
        },
        []string{"model", "provider", "tenant"},
    )
)
```

## Logging Best Practices

### Structured Logging

```go
// Use slog for structured logging
logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelInfo,
    ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
        // Add trace context to all logs
        if a.Key == slog.TimeKey {
            return slog.Attr{Key: "timestamp", Value: a.Value}
        }
        return a
    },
}))

// Log with context for trace correlation
func logWithContext(ctx context.Context, logger *slog.Logger) *slog.Logger {
    span := trace.SpanFromContext(ctx)
    if span.SpanContext().IsValid() {
        return logger.With(
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("span_id", span.SpanContext().SpanID().String()),
        )
    }
    return logger
}

// Usage
logger := logWithContext(ctx, baseLogger)
logger.Info("processing request",
    slog.String("method", r.Method),
    slog.String("path", r.URL.Path),
    slog.String("user_id", userID),
    slog.Group("request_details",
        slog.Int("size", r.ContentLength),
        slog.String("content_type", r.Header.Get("Content-Type")),
    ),
)
```

### Log Levels

```go
// Debug - Detailed information for debugging
logger.Debug("cache lookup",
    slog.String("key", key),
    slog.String("tier", "L1"),
    slog.Bool("found", found),
)

// Info - Normal operational events
logger.Info("task completed",
    slog.String("task_id", taskID),
    slog.Duration("duration", duration),
    slog.String("status", "success"),
)

// Warn - Warning conditions that might need attention
logger.Warn("rate limit approaching",
    slog.String("api", "bedrock"),
    slog.Float64("usage_percent", 85.5),
    slog.Int("remaining_calls", 150),
)

// Error - Error conditions that need investigation
logger.Error("payment processing failed",
    slog.String("payment_id", paymentID),
    slog.String("error", err.Error()),
    slog.String("provider", "stripe"),
    slog.Float64("amount", amount),
)
```

### What to Log

```go
// Always log:
// 1. Service lifecycle events
logger.Info("service starting",
    slog.String("version", version),
    slog.Any("config", sanitizedConfig),
)

// 2. External service interactions
logger.Info("calling external API",
    slog.String("service", "github"),
    slog.String("operation", "create_issue"),
    slog.String("request_id", requestID),
)

// 3. Business events
logger.Info("order processed",
    slog.String("order_id", orderID),
    slog.Float64("total", total),
    slog.String("customer_tier", tier),
)

// 4. Errors with context
logger.Error("database query failed",
    slog.String("query", sanitizeQuery(query)),
    slog.Duration("duration", duration),
    slog.String("error", err.Error()),
)

// Consider logging:
// 1. Performance anomalies
if duration > expectedDuration*2 {
    logger.Warn("slow operation detected",
        slog.String("operation", "embedding_search"),
        slog.Duration("duration", duration),
        slog.Duration("expected", expectedDuration),
    )
}

// 2. Security events
logger.Warn("authentication failed",
    slog.String("user", username),
    slog.String("ip", clientIP),
    slog.String("reason", "invalid_password"),
)

// Avoid logging:
// 1. Sensitive data
// Bad: logger.Info("user login", slog.String("password", password))
// Good: logger.Info("user login", slog.String("username", username))

// 2. High-frequency events without sampling
// Bad: logger.Debug("cache hit", slog.String("key", key)) // Every request
// Good: Sample or use metrics instead
```

## Correlation and Context

### Trace-Metric-Log Correlation

```go
// Correlation middleware
func ObservabilityMiddleware(logger *slog.Logger, metrics *Metrics) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Generate request ID
        requestID := c.GetHeader("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }
        
        // Start trace
        ctx, span := tracer.Start(c.Request.Context(), 
            fmt.Sprintf("%s %s", c.Request.Method, c.FullPath()),
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()
        
        // Add to context
        ctx = context.WithValue(ctx, "request_id", requestID)
        c.Request = c.Request.WithContext(ctx)
        
        // Create correlated logger
        reqLogger := logger.With(
            slog.String("request_id", requestID),
            slog.String("trace_id", span.SpanContext().TraceID().String()),
            slog.String("method", c.Request.Method),
            slog.String("path", c.Request.URL.Path),
        )
        
        // Log request start
        reqLogger.Info("request started")
        
        // Record metrics
        timer := prometheus.NewTimer(metrics.RequestDuration.WithLabelValues(
            c.Request.Method,
            c.FullPath(),
        ))
        
        // Process request
        c.Next()
        
        // Complete observations
        duration := timer.ObserveDuration()
        
        // Update span
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
            attribute.Float64("duration_ms", duration.Seconds()*1000),
        )
        
        // Log completion
        reqLogger.Info("request completed",
            slog.Int("status", c.Writer.Status()),
            slog.Duration("duration", duration),
        )
        
        // Update metrics
        metrics.RequestsTotal.WithLabelValues(
            c.Request.Method,
            c.FullPath(),
            strconv.Itoa(c.Writer.Status()),
        ).Inc()
    }
}
```

### Context Propagation

```go
// MCP-specific context
type MCPContext struct {
    RequestID   string
    TenantID    string
    UserID      string
    SessionID   string
    CostBudget  float64
    CostUsed    float64
}

// Add to all observability
func EnrichObservability(ctx context.Context, mcpCtx *MCPContext) {
    // Add to current span
    span := trace.SpanFromContext(ctx)
    span.SetAttributes(
        attribute.String("mcp.request_id", mcpCtx.RequestID),
        attribute.String("mcp.tenant_id", mcpCtx.TenantID),
        attribute.String("mcp.user_id", mcpCtx.UserID),
        attribute.String("mcp.session_id", mcpCtx.SessionID),
        attribute.Float64("mcp.cost_budget", mcpCtx.CostBudget),
        attribute.Float64("mcp.cost_used", mcpCtx.CostUsed),
    )
    
    // Add to logs
    logger := loggerFromContext(ctx)
    logger = logger.With(
        slog.String("tenant_id", mcpCtx.TenantID),
        slog.String("user_id", mcpCtx.UserID),
        slog.String("session_id", mcpCtx.SessionID),
    )
    
    // Add to metrics labels (be careful with cardinality)
    tenantLabel := mcpCtx.TenantID
    if !isHighValueTenant(mcpCtx.TenantID) {
        tenantLabel = "standard" // Group small tenants
    }
}
```

## Performance Considerations

### Sampling Strategies

```go
// Dynamic sampling based on operation
func shouldSample(operation string, error bool) bool {
    // Always sample errors
    if error {
        return true
    }
    
    // Sample based on operation importance
    switch operation {
    case "ProcessPayment", "CreateOrder":
        return true // Always sample critical operations
    case "HealthCheck", "Metrics":
        return rand.Float64() < 0.001 // 0.1% sampling
    case "GetCache", "SetCache":
        return rand.Float64() < 0.01 // 1% sampling
    default:
        return rand.Float64() < 0.1 // 10% default
    }
}

// Adaptive sampling based on load
type AdaptiveSampler struct {
    targetTracesPerSecond float64
    currentRate          atomic.Value // float64
}

func (s *AdaptiveSampler) ShouldSample() bool {
    rate := s.currentRate.Load().(float64)
    return rand.Float64() < rate
}

func (s *AdaptiveSampler) adjust(currentTPS float64) {
    newRate := s.targetTracesPerSecond / currentTPS
    newRate = math.Max(0.001, math.Min(1.0, newRate)) // Clamp between 0.1% and 100%
    s.currentRate.Store(newRate)
}
```

### Batching and Buffering

```go
// Batch metric updates
type BatchedMetrics struct {
    mu       sync.Mutex
    counters map[string]float64
    interval time.Duration
}

func (b *BatchedMetrics) Inc(name string, labels ...string) {
    key := fmt.Sprintf("%s{%s}", name, strings.Join(labels, ","))
    
    b.mu.Lock()
    b.counters[key]++
    b.mu.Unlock()
}

func (b *BatchedMetrics) flush() {
    b.mu.Lock()
    counters := b.counters
    b.counters = make(map[string]float64)
    b.mu.Unlock()
    
    for key, value := range counters {
        // Parse and update actual metrics
        updateMetric(key, value)
    }
}

// Async logging
type AsyncLogger struct {
    logger *slog.Logger
    buffer chan logEntry
}

func (l *AsyncLogger) Info(msg string, attrs ...slog.Attr) {
    select {
    case l.buffer <- logEntry{Level: slog.LevelInfo, Msg: msg, Attrs: attrs}:
    default:
        // Buffer full, log synchronously
        l.logger.Info(msg, attrs...)
    }
}
```

### Lazy Evaluation

```go
// Only compute expensive attributes if needed
func traceExpensiveOperation(ctx context.Context, data []byte) {
    ctx, span := tracer.Start(ctx, "ExpensiveOperation")
    defer span.End()
    
    // Always add cheap attributes
    span.SetAttributes(
        attribute.Int("data.size", len(data)),
    )
    
    // Only compute expensive attributes if sampled
    if span.IsRecording() {
        span.SetAttributes(
            attribute.String("data.hash", computeHash(data)),
            attribute.String("data.preview", truncate(string(data), 100)),
        )
    }
}

// Defer expensive log fields
logger.Info("processing large dataset",
    slog.Int("size", len(data)),
    slog.Any("stats", slog.GroupValue(
        lazyAttr("mean", func() interface{} { return calculateMean(data) }),
        lazyAttr("stddev", func() interface{} { return calculateStdDev(data) }),
    )),
)
```

## Cost Management

### Cost-Aware Instrumentation

```go
// Track observability costs
type ObservabilityCosts struct {
    tracesPerDollar   float64
    metricsPerDollar  float64
    logsPerDollar     float64
    
    dailyBudget       float64
    currentSpend      atomic.Value // float64
}

func (o *ObservabilityCosts) ShouldTrace(priority TracePriority) bool {
    spend := o.currentSpend.Load().(float64)
    budgetRemaining := o.dailyBudget - spend
    
    switch priority {
    case TracePriorityHigh:
        return true // Always trace critical operations
    case TracePriorityMedium:
        return budgetRemaining > o.dailyBudget*0.2 // 20% budget remaining
    case TracePriorityLow:
        return budgetRemaining > o.dailyBudget*0.5 // 50% budget remaining
    default:
        return false
    }
}
```

### Retention Policies

```yaml
# observability-retention.yaml
retention_policies:
  traces:
    default: 3d
    errors: 7d
    slow_requests: 7d
    high_cost_operations: 30d
    
  metrics:
    raw: 15d
    aggregated_5m: 30d
    aggregated_1h: 90d
    aggregated_1d: 365d
    
  logs:
    debug: 1d
    info: 7d
    warn: 30d
    error: 90d
```

## Common Patterns

### Request Tracing Pattern

```go
func TraceableHandler(operation string, handler HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Extract or create request ID
        requestID := r.Header.Get("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
        }
        
        // Start trace
        ctx, span := tracer.Start(r.Context(), operation,
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                attribute.String("request.id", requestID),
            ),
        )
        defer span.End()
        
        // Create wrapped response writer
        wrapped := &responseWriter{ResponseWriter: w, statusCode: 200}
        
        // Create logger
        logger := slog.With(
            slog.String("request_id", requestID),
            slog.String("trace_id", span.SpanContext().TraceID().String()),
        )
        
        // Add to context
        ctx = context.WithValue(ctx, "logger", logger)
        
        // Time the request
        start := time.Now()
        
        // Execute handler
        handler(wrapped, r.WithContext(ctx))
        
        // Record results
        duration := time.Since(start)
        
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(wrapped.statusCode),
            attribute.Float64("duration_ms", float64(duration.Milliseconds())),
        )
        
        // Log completion
        logger.Info("request completed",
            slog.Int("status", wrapped.statusCode),
            slog.Duration("duration", duration),
        )
        
        // Update metrics
        httpRequestsTotal.WithLabelValues(
            r.Method,
            operation,
            strconv.Itoa(wrapped.statusCode),
        ).Inc()
        
        httpRequestDuration.WithLabelValues(
            r.Method,
            operation,
        ).Observe(duration.Seconds())
    }
}
```

### Background Task Pattern

```go
func TraceableTask(name string, task TaskFunc) TaskFunc {
    return func(ctx context.Context, payload interface{}) error {
        // Start new trace with link to parent
        parentSpan := trace.SpanFromContext(ctx)
        
        ctx, span := tracer.Start(
            context.Background(), // New trace
            name,
            trace.WithSpanKind(trace.SpanKindConsumer),
            trace.WithLinks(trace.Link{
                SpanContext: parentSpan.SpanContext(),
            }),
        )
        defer span.End()
        
        // Create task logger
        logger := slog.With(
            slog.String("task", name),
            slog.String("trace_id", span.SpanContext().TraceID().String()),
        )
        
        logger.Info("task started")
        
        // Record task metrics
        taskStarted.WithLabelValues(name).Inc()
        timer := prometheus.NewTimer(taskDuration.WithLabelValues(name))
        
        // Execute task
        err := task(ctx, payload)
        
        // Record completion
        timer.ObserveDuration()
        
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
            logger.Error("task failed", slog.Error(err))
            tasksFailed.WithLabelValues(name).Inc()
        } else {
            logger.Info("task completed")
            tasksSucceeded.WithLabelValues(name).Inc()
        }
        
        return err
    }
}
```

### Circuit Breaker Pattern

```go
func ObservableCircuitBreaker(name string, cb *gobreaker.CircuitBreaker) *gobreaker.CircuitBreaker {
    // Wrap state change callback
    originalOnStateChange := cb.OnStateChange
    cb.OnStateChange = func(name string, from, to gobreaker.State) {
        // Log state change
        logger.Warn("circuit breaker state changed",
            slog.String("breaker", name),
            slog.String("from", from.String()),
            slog.String("to", to.String()),
        )
        
        // Update metrics
        circuitBreakerState.WithLabelValues(name).Set(float64(to))
        
        // Create trace event
        if span := trace.SpanFromContext(context.Background()); span.IsRecording() {
            span.AddEvent("circuit_breaker_state_change", trace.WithAttributes(
                attribute.String("breaker.name", name),
                attribute.String("state.from", from.String()),
                attribute.String("state.to", to.String()),
            ))
        }
        
        if originalOnStateChange != nil {
            originalOnStateChange(name, from, to)
        }
    }
    
    return cb
}
```

## Checklist

### Adding New Service

- [ ] Initialize tracer with service name and version
- [ ] Add request tracing middleware
- [ ] Set up structured logging with trace correlation
- [ ] Define and register service-specific metrics
- [ ] Implement health check endpoint with metrics
- [ ] Add trace propagation to all external calls
- [ ] Configure appropriate sampling rates
- [ ] Set up dashboards and alerts

### Adding New Operation

- [ ] Decide if operation needs tracing (use decision tree)
- [ ] Choose appropriate span name (hierarchical)
- [ ] Add relevant span attributes (avoid high cardinality)
- [ ] Record errors with context
- [ ] Add operation-specific metrics
- [ ] Log significant events at appropriate level
- [ ] Consider performance impact
- [ ] Test observability in load conditions

### Production Readiness

- [ ] Sampling rates configured for expected load
- [ ] Retention policies defined and implemented
- [ ] Dashboards created for key metrics
- [ ] Alerts configured for error rates and SLOs
- [ ] Runbooks reference trace/metric queries
- [ ] Cost monitoring in place
- [ ] Performance impact measured
- [ ] Emergency kill switches for observability

## Summary

Effective observability requires thoughtful instrumentation that balances visibility needs with performance and cost constraints. Focus on instrumenting service boundaries, critical operations, and error paths while avoiding over-instrumentation of internal details. Always correlate traces, metrics, and logs for complete system understanding.

---

Last Updated: 2024-01-10
Version: 1.0.0