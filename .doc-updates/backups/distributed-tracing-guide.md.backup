# Distributed Tracing Guide

> **Purpose**: Complete guide for implementing and using distributed tracing in the Developer Mesh platform
> **Audience**: Developers and SREs implementing observability and debugging distributed systems
> **Scope**: OpenTelemetry setup, trace analysis, debugging patterns, and production best practices

## Table of Contents

1. [Overview](#overview)
2. [Core Concepts](#core-concepts)
3. [Implementation Architecture](#implementation-architecture)
4. [Setting Up Distributed Tracing](#setting-up-distributed-tracing)
5. [Instrumenting Services](#instrumenting-services)
6. [Trace Analysis](#trace-analysis)
7. [Debugging with Traces](#debugging-with-traces)
8. [Performance Optimization](#performance-optimization)
9. [Production Operations](#production-operations)
10. [Best Practices](#best-practices)

## Overview

Distributed tracing provides end-to-end visibility into request flows across all services in the Developer Mesh platform, enabling debugging of complex multi-service interactions, performance bottlenecks, and error propagation.

### Why Distributed Tracing?

- **End-to-End Visibility**: Follow requests across services, databases, and external APIs
- **Performance Analysis**: Identify bottlenecks and optimize critical paths
- **Error Diagnosis**: Understand error propagation and root causes
- **Service Dependencies**: Visualize and understand service interactions
- **Cost Attribution**: Track AI model usage and costs across requests

### Key Components

```
┌─────────────┐     ┌─────────────┐     ┌─────────────┐
│   Client    │────▶│ MCP Server  │────▶│  REST API   │
└─────────────┘     └─────────────┘     └─────────────┘
       │                    │                    │
       └────────────────────┴────────────────────┘
                            │
                     ┌──────▼──────┐
                     │   Jaeger    │
                     │  Collector  │
                     └──────┬──────┘
                            │
                     ┌──────▼──────┐
                     │   Storage   │
                     │ (Cassandra) │
                     └─────────────┘
```

## Core Concepts

### Traces, Spans, and Context

```go
// Trace: End-to-end request flow
type Trace struct {
    TraceID     string
    Spans       []Span
    StartTime   time.Time
    EndTime     time.Time
    Services    []string
}

// Span: Single operation within a trace
type Span struct {
    SpanID      string
    TraceID     string
    ParentID    string
    Operation   string
    Service     string
    StartTime   time.Time
    Duration    time.Duration
    Status      SpanStatus
    Attributes  map[string]interface{}
    Events      []SpanEvent
    Links       []SpanLink
}

// Context: Propagated across services
type TraceContext struct {
    TraceID     TraceID
    SpanID      SpanID
    TraceFlags  TraceFlags
    TraceState  TraceState
    Baggage     map[string]string
}
```

### Trace Propagation

```
Client Request
    │
    ├─ HTTP Headers: traceparent, tracestate
    │
    ▼
MCP Server (Parent Span)
    │
    ├─ WebSocket: Embed trace in binary protocol
    ├─ HTTP: W3C Trace Context headers
    ├─ SQS: Message attributes
    └─ gRPC: Metadata
    │
    ▼
Worker Service (Child Span)
    │
    ├─ Bedrock API: AWS X-Ray integration
    ├─ Database: Query annotation
    └─ Cache: Key tagging
```

## Implementation Architecture

### OpenTelemetry Setup

```go
// pkg/observability/tracing.go
package observability

import (
    "context"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/resource"
    "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type TracingConfig struct {
    ServiceName     string
    ServiceVersion  string
    Environment     string
    JaegerEndpoint  string
    SamplingRate    float64
    MaxSpansPerTrace int
}

func InitTracing(cfg TracingConfig) (*trace.TracerProvider, error) {
    // Create Jaeger exporter
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint(cfg.JaegerEndpoint),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("create jaeger exporter: %w", err)
    }
    
    // Create resource
    res, err := resource.Merge(
        resource.Default(),
        resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(cfg.ServiceName),
            semconv.ServiceVersionKey.String(cfg.ServiceVersion),
            semconv.DeploymentEnvironmentKey.String(cfg.Environment),
            attribute.String("service.namespace", "developer-mesh"),
        ),
    )
    if err != nil {
        return nil, fmt.Errorf("create resource: %w", err)
    }
    
    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(res),
        trace.WithSampler(NewAdaptiveSampler(cfg.SamplingRate)),
        trace.WithSpanLimits(trace.SpanLimits{
            AttributeCountLimit: 128,
            EventCountLimit:     128,
            LinkCountLimit:      128,
        }),
    )
    
    // Set global provider
    otel.SetTracerProvider(tp)
    
    // Set global propagator
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},
            propagation.Baggage{},
            &AWSPropagator{}, // For X-Ray compatibility
            &MCPPropagator{}, // Custom MCP context
        ),
    )
    
    return tp, nil
}
```

### Custom Propagators

```go
// MCP-specific context propagation
type MCPPropagator struct{}

func (p *MCPPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
    // Extract MCP context
    if mcpCtx := GetMCPContext(ctx); mcpCtx != nil {
        carrier.Set("X-MCP-Tenant-ID", mcpCtx.TenantID)
        carrier.Set("X-MCP-User-ID", mcpCtx.UserID)
        carrier.Set("X-MCP-Session-ID", mcpCtx.SessionID)
        carrier.Set("X-MCP-Agent-ID", mcpCtx.AgentID)
        carrier.Set("X-MCP-Cost-Budget", fmt.Sprintf("%.4f", mcpCtx.CostBudget))
    }
}

func (p *MCPPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
    mcpCtx := &MCPContext{
        TenantID:   carrier.Get("X-MCP-Tenant-ID"),
        UserID:     carrier.Get("X-MCP-User-ID"),
        SessionID:  carrier.Get("X-MCP-Session-ID"),
        AgentID:    carrier.Get("X-MCP-Agent-ID"),
    }
    
    if budget := carrier.Get("X-MCP-Cost-Budget"); budget != "" {
        mcpCtx.CostBudget, _ = strconv.ParseFloat(budget, 64)
    }
    
    return WithMCPContext(ctx, mcpCtx)
}

func (p *MCPPropagator) Fields() []string {
    return []string{
        "X-MCP-Tenant-ID",
        "X-MCP-User-ID",
        "X-MCP-Session-ID",
        "X-MCP-Agent-ID",
        "X-MCP-Cost-Budget",
    }
}
```

## Setting Up Distributed Tracing

### 1. Deploy Jaeger

```yaml
# docker-compose.jaeger.yml
version: '3.8'

services:
  jaeger:
    image: jaegertracing/all-in-one:1.50
    environment:
      - COLLECTOR_OTLP_ENABLED=true
      - SPAN_STORAGE_TYPE=cassandra
      - CASSANDRA_SERVERS=cassandra
      - CASSANDRA_KEYSPACE=jaeger_v1_dc1
    ports:
      - "16686:16686"  # UI
      - "14268:14268"  # Collector HTTP
      - "14250:14250"  # Collector gRPC
      - "4317:4317"    # OTLP gRPC
      - "4318:4318"    # OTLP HTTP
    depends_on:
      - cassandra
      
  cassandra:
    image: cassandra:4.1
    environment:
      - CASSANDRA_DC=dc1
      - CASSANDRA_ENDPOINT_SNITCH=GossipingPropertyFileSnitch
    volumes:
      - cassandra_data:/var/lib/cassandra
      
  jaeger-spark-dependencies:
    image: jaegertracing/spark-dependencies:latest
    environment:
      - STORAGE=cassandra
      - CASSANDRA_CONTACT_POINTS=cassandra
    depends_on:
      - cassandra

volumes:
  cassandra_data:
```

### 2. Configure Services

```yaml
# configs/tracing.yaml
tracing:
  enabled: true
  service_name: ${SERVICE_NAME}
  service_version: ${SERVICE_VERSION}
  environment: ${ENVIRONMENT}
  
  jaeger:
    endpoint: http://jaeger:14268/api/traces
    
  sampling:
    default_rate: 0.01  # 1%
    rules:
      - name: errors
        condition: "error == true"
        rate: 1.0  # 100% for errors
      - name: slow_requests
        condition: "duration > 1000"
        rate: 0.5  # 50% for slow requests
      - name: vip_users
        condition: "user.tier == 'enterprise'"
        rate: 0.1  # 10% for VIP users
        
  propagation:
    formats:
      - w3c
      - b3
      - aws-xray
      - mcp
      
  span_processors:
    - type: batch
      max_queue_size: 2048
      batch_timeout: 5s
      max_batch_size: 512
    - type: memory_limiter
      check_interval: 1s
      limit_mib: 512
```

### 3. Initialize in Services

```go
// apps/mcp-server/cmd/server/main.go
func main() {
    // Initialize tracing
    tp, err := observability.InitTracing(observability.TracingConfig{
        ServiceName:    "mcp-server",
        ServiceVersion: version.Version,
        Environment:    config.Environment,
        JaegerEndpoint: config.Tracing.JaegerEndpoint,
        SamplingRate:   config.Tracing.SamplingRate,
    })
    if err != nil {
        log.Fatal("init tracing:", err)
    }
    defer func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := tp.Shutdown(ctx); err != nil {
            log.Printf("error shutting down tracer provider: %v", err)
        }
    }()
    
    // Create tracer
    tracer := otel.Tracer("mcp-server",
        trace.WithInstrumentationVersion(version.Version),
    )
    
    // Start server with tracing
    server := NewServer(config, tracer)
    server.Run()
}
```

## Instrumenting Services

### HTTP Middleware

```go
// pkg/api/middleware/tracing.go
func TracingMiddleware(tracer trace.Tracer) gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract trace context
        ctx := otel.GetTextMapPropagator().Extract(
            c.Request.Context(),
            propagation.HeaderCarrier(c.Request.Header),
        )
        
        // Start span
        spanName := fmt.Sprintf("%s %s", c.Request.Method, c.FullPath())
        ctx, span := tracer.Start(ctx, spanName,
            trace.WithSpanKind(trace.SpanKindServer),
            trace.WithAttributes(
                semconv.HTTPMethodKey.String(c.Request.Method),
                semconv.HTTPTargetKey.String(c.Request.URL.Path),
                semconv.HTTPSchemeKey.String(c.Request.URL.Scheme),
                semconv.NetHostNameKey.String(c.Request.Host),
                semconv.NetTransportTCP,
            ),
        )
        defer span.End()
        
        // Add request ID
        requestID := c.GetHeader("X-Request-ID")
        if requestID == "" {
            requestID = uuid.New().String()
            c.Header("X-Request-ID", requestID)
        }
        span.SetAttributes(attribute.String("request.id", requestID))
        
        // Store in context
        c.Request = c.Request.WithContext(ctx)
        
        // Process request
        c.Next()
        
        // Record response
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
            attribute.Int("response.size", c.Writer.Size()),
        )
        
        // Record error if any
        if len(c.Errors) > 0 {
            span.RecordError(c.Errors.Last())
            span.SetStatus(codes.Error, c.Errors.Last().Error())
        } else if c.Writer.Status() >= 400 {
            span.SetStatus(codes.Error, http.StatusText(c.Writer.Status()))
        }
        
        // Add trace ID to response
        span.SpanContext().TraceID().String()
        c.Header("X-Trace-ID", span.SpanContext().TraceID().String())
    }
}
```

### WebSocket Instrumentation

```go
// pkg/api/websocket/instrumented_handler.go
func (h *Handler) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from HTTP upgrade request
    ctx := otel.GetTextMapPropagator().Extract(
        r.Context(),
        propagation.HeaderCarrier(r.Header),
    )
    
    // Start connection span
    ctx, connSpan := h.tracer.Start(ctx, "websocket.connection",
        trace.WithSpanKind(trace.SpanKindServer),
        trace.WithAttributes(
            attribute.String("ws.agent_id", r.Header.Get("X-Agent-ID")),
            attribute.String("ws.protocol_version", r.Header.Get("Sec-WebSocket-Protocol")),
        ),
    )
    
    // Upgrade connection
    conn, err := h.upgrader.Upgrade(w, r, nil)
    if err != nil {
        connSpan.RecordError(err)
        connSpan.End()
        return
    }
    
    // Create connection context
    connCtx := &ConnectionContext{
        Context:   ctx,
        Conn:      conn,
        AgentID:   r.Header.Get("X-Agent-ID"),
        ConnSpan:  connSpan,
        MessageCh: make(chan []byte, 100),
    }
    
    // Handle connection lifecycle
    go h.handleMessages(connCtx)
}

func (h *Handler) handleMessages(connCtx *ConnectionContext) {
    defer connCtx.ConnSpan.End()
    
    for {
        // Read message
        _, msg, err := connCtx.Conn.ReadMessage()
        if err != nil {
            connCtx.ConnSpan.RecordError(err)
            return
        }
        
        // Start message span with link to connection
        msgCtx, msgSpan := h.tracer.Start(connCtx.Context, "websocket.message",
            trace.WithLinks(trace.Link{
                SpanContext: connCtx.ConnSpan.SpanContext(),
            }),
            trace.WithAttributes(
                attribute.Int("message.size", len(msg)),
                attribute.Bool("message.binary", msg[0] == 0x00),
            ),
        )
        
        // Process message
        if err := h.processMessage(msgCtx, connCtx, msg); err != nil {
            msgSpan.RecordError(err)
            msgSpan.SetStatus(codes.Error, err.Error())
        }
        
        msgSpan.End()
    }
}

// Embed trace context in binary messages
func (h *Handler) sendMessage(ctx context.Context, conn *websocket.Conn, msg *Message) error {
    span := trace.SpanFromContext(ctx)
    
    // Add trace context to message
    if msg.TraceContext == nil {
        msg.TraceContext = &TraceContext{}
    }
    msg.TraceContext.TraceID = span.SpanContext().TraceID().String()
    msg.TraceContext.SpanID = span.SpanContext().SpanID().String()
    msg.TraceContext.TraceFlags = uint8(span.SpanContext().TraceFlags())
    
    // Serialize and send
    data, err := h.encodeMessage(msg)
    if err != nil {
        return err
    }
    
    return conn.WriteMessage(websocket.BinaryMessage, data)
}
```

### Database Instrumentation

```go
// pkg/repository/instrumented_repository.go
type InstrumentedRepository struct {
    *Repository
    tracer trace.Tracer
}

func (r *InstrumentedRepository) CreateContext(ctx context.Context, context *models.Context) error {
    ctx, span := r.tracer.Start(ctx, "repository.CreateContext",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("db.system", "postgresql"),
            attribute.String("db.operation", "INSERT"),
            attribute.String("db.table", "contexts"),
        ),
    )
    defer span.End()
    
    // Add trace ID as comment for slow query log correlation
    query := fmt.Sprintf("/* trace_id=%s */ INSERT INTO contexts...", 
        span.SpanContext().TraceID().String())
    
    start := time.Now()
    err := r.Repository.CreateContext(ctx, context)
    duration := time.Since(start)
    
    span.SetAttributes(
        attribute.Int64("db.rows_affected", 1),
        attribute.Float64("db.duration_ms", float64(duration.Milliseconds())),
    )
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
    }
    
    return err
}

// Vector search with tracing
func (r *InstrumentedRepository) SearchEmbeddings(ctx context.Context, query []float32, limit int) ([]*Embedding, error) {
    ctx, span := r.tracer.Start(ctx, "repository.SearchEmbeddings",
        trace.WithAttributes(
            attribute.Int("embedding.dimensions", len(query)),
            attribute.Int("search.limit", limit),
            attribute.String("db.operation", "vector_search"),
        ),
    )
    defer span.End()
    
    results, err := r.Repository.SearchEmbeddings(ctx, query, limit)
    
    span.SetAttributes(
        attribute.Int("search.results", len(results)),
    )
    
    return results, err
}
```

### External Service Instrumentation

```go
// pkg/adapters/bedrock/instrumented_client.go
func (c *InstrumentedBedrockClient) InvokeModel(ctx context.Context, input *InvokeModelInput) (*InvokeModelOutput, error) {
    ctx, span := c.tracer.Start(ctx, "bedrock.InvokeModel",
        trace.WithSpanKind(trace.SpanKindClient),
        trace.WithAttributes(
            attribute.String("bedrock.model_id", input.ModelID),
            attribute.Int("bedrock.input_tokens", estimateTokens(input.Body)),
            attribute.String("aws.service", "bedrock"),
            attribute.String("aws.operation", "InvokeModel"),
        ),
    )
    defer span.End()
    
    // Inject trace context for AWS X-Ray
    carrier := make(propagation.MapCarrier)
    otel.GetTextMapPropagator().Inject(ctx, carrier)
    
    // Add to request
    for k, v := range carrier {
        input.Headers[k] = v
    }
    
    start := time.Now()
    output, err := c.client.InvokeModel(ctx, input)
    duration := time.Since(start)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        
        // Check for specific errors
        var tre *types.ThrottlingException
        if errors.As(err, &tre) {
            span.SetAttributes(
                attribute.Bool("bedrock.throttled", true),
                attribute.String("error.type", "ThrottlingException"),
            )
        }
    } else {
        // Record success metrics
        outputTokens := estimateTokens(output.Body)
        cost := calculateCost(input.ModelID, len(input.Body), outputTokens)
        
        span.SetAttributes(
            attribute.Int("bedrock.output_tokens", outputTokens),
            attribute.Float64("bedrock.latency_ms", float64(duration.Milliseconds())),
            attribute.Float64("bedrock.cost_usd", cost),
        )
        
        span.AddEvent("model_invoked", trace.WithAttributes(
            attribute.String("response.content_type", output.ContentType),
            attribute.Int("response.size", len(output.Body)),
        ))
    }
    
    return output, err
}
```

## Trace Analysis

### Finding Performance Bottlenecks

```go
// tools/trace-analyzer/main.go
func analyzeTraces(traces []Trace) *PerformanceReport {
    report := &PerformanceReport{
        SlowOperations: make(map[string]*OperationStats),
        ServiceLatencies: make(map[string]*LatencyStats),
    }
    
    for _, trace := range traces {
        // Find critical path
        criticalPath := findCriticalPath(trace.Spans)
        
        // Analyze each span
        for _, span := range trace.Spans {
            op := fmt.Sprintf("%s.%s", span.Service, span.Operation)
            
            if stats, ok := report.SlowOperations[op]; ok {
                stats.Count++
                stats.TotalDuration += span.Duration
                if span.Duration > stats.MaxDuration {
                    stats.MaxDuration = span.Duration
                    stats.ExampleTraceID = trace.TraceID
                }
            } else {
                report.SlowOperations[op] = &OperationStats{
                    Operation:      op,
                    Count:          1,
                    TotalDuration:  span.Duration,
                    MaxDuration:    span.Duration,
                    ExampleTraceID: trace.TraceID,
                }
            }
        }
    }
    
    // Calculate percentiles
    for op, stats := range report.SlowOperations {
        stats.AvgDuration = stats.TotalDuration / time.Duration(stats.Count)
        stats.ImpactScore = float64(stats.TotalDuration) / float64(totalTraceDuration) * 100
    }
    
    return report
}

func findCriticalPath(spans []Span) []Span {
    // Build span tree
    spanMap := make(map[string]*Span)
    children := make(map[string][]*Span)
    
    for i := range spans {
        span := &spans[i]
        spanMap[span.SpanID] = span
        if span.ParentID != "" {
            children[span.ParentID] = append(children[span.ParentID], span)
        }
    }
    
    // Find root span
    var root *Span
    for _, span := range spans {
        if span.ParentID == "" {
            root = &span
            break
        }
    }
    
    // DFS to find longest path
    var criticalPath []Span
    var maxDuration time.Duration
    
    var dfs func(*Span, []Span, time.Duration)
    dfs = func(span *Span, path []Span, duration time.Duration) {
        path = append(path, *span)
        duration += span.Duration
        
        childSpans := children[span.SpanID]
        if len(childSpans) == 0 {
            if duration > maxDuration {
                maxDuration = duration
                criticalPath = make([]Span, len(path))
                copy(criticalPath, path)
            }
        } else {
            for _, child := range childSpans {
                dfs(child, path, duration)
            }
        }
    }
    
    dfs(root, nil, 0)
    return criticalPath
}
```

### Error Analysis

```go
// Analyze error propagation patterns
func analyzeErrors(traces []Trace) *ErrorReport {
    report := &ErrorReport{
        ErrorPatterns: make(map[string]*ErrorPattern),
        ServiceErrors: make(map[string]int),
    }
    
    for _, trace := range traces {
        var errorSpans []Span
        
        for _, span := range trace.Spans {
            if span.Status.Code == codes.Error {
                errorSpans = append(errorSpans, span)
                report.ServiceErrors[span.Service]++
            }
        }
        
        if len(errorSpans) > 0 {
            // Find root cause (first error in time)
            sort.Slice(errorSpans, func(i, j int) bool {
                return errorSpans[i].StartTime.Before(errorSpans[j].StartTime)
            })
            
            rootCause := errorSpans[0]
            pattern := fmt.Sprintf("%s.%s: %s", 
                rootCause.Service, 
                rootCause.Operation,
                rootCause.Status.Description,
            )
            
            if p, ok := report.ErrorPatterns[pattern]; ok {
                p.Count++
                p.ExampleTraces = append(p.ExampleTraces, trace.TraceID)
            } else {
                report.ErrorPatterns[pattern] = &ErrorPattern{
                    Pattern:       pattern,
                    Count:         1,
                    RootService:   rootCause.Service,
                    ExampleTraces: []string{trace.TraceID},
                }
            }
        }
    }
    
    return report
}
```

## Debugging with Traces

### Common Debugging Scenarios

#### 1. Slow Request Investigation

```bash
# Find slow traces
curl "http://localhost:16686/api/traces?service=mcp-server&minDuration=5s&limit=20"

# Analyze specific slow trace
TRACE_ID="4bf92f3577b34da6a3ce929d0e0e4736"
curl "http://localhost:16686/api/traces/${TRACE_ID}" | jq '.data[0].spans | map({operation: .operationName, duration: .duration, service: .process.serviceName}) | sort_by(.duration) | reverse'

# Find operations taking >1s
curl "http://localhost:16686/api/traces/${TRACE_ID}" | jq '.data[0].spans | map(select(.duration > 1000000)) | map({operation: .operationName, duration_ms: (.duration / 1000), tags: .tags})'
```

#### 2. Error Root Cause Analysis

```go
// Helper to analyze error traces
func FindErrorRootCause(traceID string) (*ErrorAnalysis, error) {
    trace, err := jaegerClient.GetTrace(traceID)
    if err != nil {
        return nil, err
    }
    
    analysis := &ErrorAnalysis{
        TraceID: traceID,
        Errors:  []ErrorDetail{},
    }
    
    // Find all error spans
    var errorSpans []Span
    for _, span := range trace.Spans {
        for _, tag := range span.Tags {
            if tag.Key == "error" && tag.Value == true {
                errorSpans = append(errorSpans, span)
            }
        }
    }
    
    // Sort by time to find first error
    sort.Slice(errorSpans, func(i, j int) bool {
        return errorSpans[i].StartTime < errorSpans[j].StartTime
    })
    
    if len(errorSpans) > 0 {
        rootCause := errorSpans[0]
        analysis.RootCause = &ErrorDetail{
            Service:   rootCause.Process.ServiceName,
            Operation: rootCause.OperationName,
            Timestamp: time.Unix(0, rootCause.StartTime*1000),
            Message:   getErrorMessage(rootCause),
        }
        
        // Build error chain
        for _, span := range errorSpans[1:] {
            analysis.ErrorChain = append(analysis.ErrorChain, ErrorDetail{
                Service:   span.Process.ServiceName,
                Operation: span.OperationName,
                Timestamp: time.Unix(0, span.StartTime*1000),
                Message:   getErrorMessage(span),
            })
        }
    }
    
    return analysis, nil
}
```

#### 3. Service Dependency Analysis

```go
// Generate service dependency graph from traces
func BuildDependencyGraph(traces []Trace) *DependencyGraph {
    graph := &DependencyGraph{
        Nodes: make(map[string]*ServiceNode),
        Edges: make(map[string]*ServiceEdge),
    }
    
    for _, trace := range traces {
        spanMap := make(map[string]*Span)
        
        // Index spans by ID
        for i := range trace.Spans {
            span := &trace.Spans[i]
            spanMap[span.SpanID] = span
            
            // Add service node
            if _, ok := graph.Nodes[span.Service]; !ok {
                graph.Nodes[span.Service] = &ServiceNode{
                    Service: span.Service,
                    Operations: make(map[string]int),
                }
            }
            graph.Nodes[span.Service].Operations[span.Operation]++
        }
        
        // Build edges from parent-child relationships
        for _, span := range trace.Spans {
            if span.ParentID != "" {
                parent, ok := spanMap[span.ParentID]
                if ok && parent.Service != span.Service {
                    edgeKey := fmt.Sprintf("%s->%s", parent.Service, span.Service)
                    
                    if edge, ok := graph.Edges[edgeKey]; ok {
                        edge.CallCount++
                        edge.TotalDuration += span.Duration
                    } else {
                        graph.Edges[edgeKey] = &ServiceEdge{
                            From:          parent.Service,
                            To:            span.Service,
                            CallCount:     1,
                            TotalDuration: span.Duration,
                        }
                    }
                }
            }
        }
    }
    
    return graph
}
```

## Performance Optimization

### Reducing Tracing Overhead

```go
// Conditional instrumentation
func (h *Handler) shouldTrace(r *http.Request) bool {
    // Always trace errors
    if r.Context().Err() != nil {
        return true
    }
    
    // Skip health checks
    if r.URL.Path == "/health" || r.URL.Path == "/metrics" {
        return false
    }
    
    // Force trace with header
    if r.Header.Get("X-Force-Trace") == "true" {
        return true
    }
    
    // Use configured sampling
    return h.sampler.ShouldSample(r)
}

// Lazy span attributes
func (s *Service) ProcessRequest(ctx context.Context, req *Request) error {
    ctx, span := s.tracer.Start(ctx, "ProcessRequest")
    defer span.End()
    
    // Add basic attributes immediately
    span.SetAttributes(
        attribute.String("request.id", req.ID),
        attribute.String("request.type", req.Type),
    )
    
    // Add expensive attributes only if sampled
    if span.IsRecording() {
        span.SetAttributes(
            attribute.Int("request.size", len(req.Data)),
            attribute.String("request.hash", calculateHash(req.Data)),
        )
    }
    
    return s.process(ctx, req)
}
```

### Batch Span Processing

```go
// Configure batch processor for efficiency
func configureBatchProcessor() trace.SpanProcessor {
    return trace.NewBatchSpanProcessor(
        exporter,
        trace.WithMaxQueueSize(2048),
        trace.WithBatchTimeout(5*time.Second),
        trace.WithMaxExportBatchSize(512),
        trace.WithExportTimeout(30*time.Second),
    )
}

// Buffer span events
type BufferedSpanRecorder struct {
    span   trace.Span
    events []trace.Event
    mu     sync.Mutex
    ticker *time.Ticker
}

func (r *BufferedSpanRecorder) AddEvent(name string, opts ...trace.EventOption) {
    r.mu.Lock()
    r.events = append(r.events, trace.Event{Name: name, Options: opts})
    r.mu.Unlock()
}

func (r *BufferedSpanRecorder) flush() {
    r.mu.Lock()
    events := r.events
    r.events = nil
    r.mu.Unlock()
    
    for _, event := range events {
        r.span.AddEvent(event.Name, event.Options...)
    }
}
```

## Production Operations

### Trace Retention Policies

```yaml
# jaeger-retention.yaml
retention:
  default:
    max_age: 72h  # 3 days default
    
  policies:
    - name: errors
      filter: 'error=true'
      max_age: 168h  # 7 days for errors
      
    - name: slow_requests
      filter: 'duration>5000'
      max_age: 168h  # 7 days for slow requests
      
    - name: vip_users
      filter: 'user.tier="enterprise"'
      max_age: 720h  # 30 days for VIP
      
    - name: high_cost
      filter: 'cost.total>10.0'
      max_age: 2160h  # 90 days for expensive operations
```

### Monitoring Trace Health

```go
// Health check for tracing pipeline
func CheckTracingHealth(ctx context.Context) (*TracingHealth, error) {
    health := &TracingHealth{
        Timestamp: time.Now(),
        Checks:    make(map[string]HealthCheck),
    }
    
    // Check exporter
    exporterCheck := HealthCheck{Name: "exporter"}
    if err := testExporter(ctx); err != nil {
        exporterCheck.Status = "unhealthy"
        exporterCheck.Error = err.Error()
    } else {
        exporterCheck.Status = "healthy"
    }
    health.Checks["exporter"] = exporterCheck
    
    // Check sampling rate
    samplingCheck := HealthCheck{Name: "sampling"}
    actualRate := calculateActualSamplingRate()
    if math.Abs(actualRate-expectedRate) > 0.1 {
        samplingCheck.Status = "warning"
        samplingCheck.Message = fmt.Sprintf("Actual rate %.2f%% differs from expected %.2f%%", 
            actualRate*100, expectedRate*100)
    } else {
        samplingCheck.Status = "healthy"
    }
    health.Checks["sampling"] = samplingCheck
    
    // Check trace completeness
    completenessCheck := HealthCheck{Name: "completeness"}
    orphanRate := calculateOrphanSpanRate()
    if orphanRate > 0.05 { // >5% orphan spans
        completenessCheck.Status = "warning"
        completenessCheck.Message = fmt.Sprintf("%.1f%% orphan spans detected", orphanRate*100)
    } else {
        completenessCheck.Status = "healthy"
    }
    health.Checks["completeness"] = completenessCheck
    
    return health, nil
}
```

### Cost Management

```go
// Track tracing costs
type TracingCostTracker struct {
    spanCount     atomic.Int64
    bytesExported atomic.Int64
    costPerSpan   float64
    costPerGB     float64
}

func (t *TracingCostTracker) RecordSpan(span ReadOnlySpan) {
    t.spanCount.Add(1)
    
    // Estimate span size
    size := estimateSpanSize(span)
    t.bytesExported.Add(int64(size))
}

func (t *TracingCostTracker) GetMonthlyCost() float64 {
    spans := t.spanCount.Load()
    bytes := t.bytesExported.Load()
    
    spanCost := float64(spans) * t.costPerSpan
    storageCost := float64(bytes) / 1e9 * t.costPerGB * 30 // Monthly
    
    return spanCost + storageCost
}

// Implement cost-aware sampling
type CostAwareSampler struct {
    budgetPerHour float64
    costTracker   *TracingCostTracker
    baseSampler   trace.Sampler
}

func (s *CostAwareSampler) ShouldSample(parameters trace.SamplingParameters) trace.SamplingResult {
    // Always sample errors
    for _, attr := range parameters.Attributes {
        if attr.Key == "error" && attr.Value.AsBool() {
            return trace.AlwaysSample().ShouldSample(parameters)
        }
    }
    
    // Check budget
    currentCost := s.costTracker.GetHourlyCost()
    if currentCost >= s.budgetPerHour {
        // Over budget, only sample critical
        return trace.NeverSample().ShouldSample(parameters)
    }
    
    // Under budget, use base sampler
    return s.baseSampler.ShouldSample(parameters)
}
```

## Best Practices

### 1. Span Naming Conventions

```go
// Use hierarchical naming
"service.component.operation"
"mcp-server.handler.CreateContext"
"rest-api.repository.SearchEmbeddings"
"worker.bedrock.GenerateEmbedding"

// Include important context in span name
fmt.Sprintf("cache.redis.Get[%s]", cacheType)
fmt.Sprintf("db.query[%s]", tableName)
fmt.Sprintf("http.client[%s]", service)
```

### 2. Attribute Standards

```go
// Standard attributes for all spans
span.SetAttributes(
    // Service context
    attribute.String("service.name", serviceName),
    attribute.String("service.version", version),
    attribute.String("service.instance", instanceID),
    
    // Request context
    attribute.String("request.id", requestID),
    attribute.String("tenant.id", tenantID),
    attribute.String("user.id", userID),
    
    // Operation details
    attribute.String("operation.type", opType),
    attribute.Int("operation.retry", retryCount),
    
    // Cost tracking
    attribute.Float64("cost.estimated", estimated),
    attribute.Float64("cost.actual", actual),
)
```

### 3. Error Recording

```go
// Comprehensive error recording
func RecordError(span trace.Span, err error) {
    span.RecordError(err, trace.WithTimestamp(time.Now()))
    
    // Add error details
    span.SetAttributes(
        attribute.String("error.type", fmt.Sprintf("%T", err)),
        attribute.String("error.message", err.Error()),
    )
    
    // Add stack trace for unexpected errors
    if !errors.Is(err, context.Canceled) && !errors.Is(err, sql.ErrNoRows) {
        span.SetAttributes(
            attribute.String("error.stack", string(debug.Stack())),
        )
    }
    
    // Set appropriate status
    switch {
    case errors.Is(err, context.Canceled):
        span.SetStatus(codes.Error, "Cancelled")
    case errors.Is(err, context.DeadlineExceeded):
        span.SetStatus(codes.Error, "DeadlineExceeded")
    default:
        span.SetStatus(codes.Error, "Internal Error")
    }
}
```

### 4. Context Propagation

```go
// Always propagate context
func (s *Service) ProcessAsync(ctx context.Context, task *Task) {
    // Bad: Creates new root trace
    go s.process(context.Background(), task)
    
    // Good: Links to parent trace
    go func() {
        ctx, span := s.tracer.Start(
            context.Background(), 
            "ProcessAsync",
            trace.WithLinks(trace.LinkFromContext(ctx)),
        )
        defer span.End()
        
        s.process(ctx, task)
    }()
}
```

### 5. Performance Guidelines

- Keep span count under control (aim for <100 spans per trace)
- Use sampling for high-volume, low-value operations
- Batch span exports to reduce network overhead
- Set appropriate span limits to prevent memory issues
- Use async span processors for better performance

## Troubleshooting

### Missing Traces

```bash
# Check sampling decisions
curl http://localhost:8080/metrics | grep tracing_sampling_decision

# Verify exporter is working
curl http://localhost:8080/debug/tracing/export

# Check for dropped spans
curl http://localhost:8080/metrics | grep tracing_spans_dropped
```

### Incomplete Traces

```go
// Debug trace completeness
func DebugTraceCompleteness(traceID string) {
    trace, _ := jaegerClient.GetTrace(traceID)
    
    // Check for orphan spans
    parentIDs := make(map[string]bool)
    spanIDs := make(map[string]bool)
    
    for _, span := range trace.Spans {
        spanIDs[span.SpanID] = true
        if span.ParentID != "" {
            parentIDs[span.ParentID] = true
        }
    }
    
    // Find missing parents
    for parentID := range parentIDs {
        if !spanIDs[parentID] {
            fmt.Printf("Missing parent span: %s\n", parentID)
        }
    }
}
```

### High Cardinality Issues

```go
// Avoid high cardinality attributes
// Bad: Creates too many unique time series
span.SetAttributes(
    attribute.String("user.email", email),        // Unbounded
    attribute.String("request.body", body),       // Large
    attribute.Float64("timestamp", time.Now().Unix()), // Unique
)

// Good: Use bounded, categorical values
span.SetAttributes(
    attribute.String("user.tier", tier),          // Limited values
    attribute.Int("request.size_bucket", sizeBucket), // Bucketed
    attribute.String("time.hour", hourString),    // Grouped
)
```

## Next Steps

1. Review [Trace-Based Debugging](./trace-based-debugging.md) for debugging techniques
2. See [Cross-Service Tracing](./cross-service-tracing.md) for propagation details
3. Check [Trace Sampling Guide](./trace-sampling-guide.md) for sampling strategies
4. Read [Observability Architecture](./observability-architecture.md) for system design
5. Study [Observability Best Practices](./observability-best-practices.md) for guidelines

---

Last Updated: 2024-01-10
Version: 1.0.0