<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:39:14
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Cross-Service Tracing Guide

> **Purpose**: Comprehensive guide for implementing and maintaining trace propagation across distributed services
> **Audience**: Backend developers and platform engineers working with the Developer Mesh microservices
> **Scope**: Trace context propagation, service boundaries, and distributed debugging patterns
> **Implementation Status**: PARTIAL - OpenTelemetry SDK only, no backend storage

## ⚠️ Important Note

**Current Implementation:**
- ✅ OpenTelemetry SDK partially integrated
- ✅ Basic trace context propagation in some services
- ❌ NO Jaeger or other trace collection backend
- ❌ NO trace visualization or storage

Traces are created and propagated but not collected or viewable anywhere.

## Table of Contents

1. [Overview](#overview)
2. [Trace Context Fundamentals](#trace-context-fundamentals)
3. [Propagation Patterns](#propagation-patterns)
4. [Service Integration Points](#service-integration-points)
5. [Implementation Guide](#implementation-guide)
6. [WebSocket Trace Propagation](#websocket-trace-propagation) <!-- Source: pkg/models/websocket/binary.go -->
7. [Async Operations](#async-operations)
8. [Error Propagation](#error-propagation)
9. [Testing Trace Propagation](#testing-trace-propagation)
10. [Troubleshooting](#troubleshooting)

## Overview

Cross-service tracing enables end-to-end visibility across distributed systems by propagating trace context through service boundaries. This is essential for debugging complex interactions in the Developer Mesh platform.

### Key Benefits

- **End-to-end visibility**: Follow requests across all services
- **Performance analysis**: Identify bottlenecks across service boundaries
- **Error tracking**: Understand error propagation and root causes
- **Dependency mapping**: Visualize service interactions
- **SLA monitoring**: Track latency through the entire system

## Trace Context Fundamentals

### W3C Trace Context Standard

Developer Mesh uses the W3C Trace Context standard for propagation:

```
traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01
              │  │                                │                │
              │  │                                │                └─ Trace flags (sampled)
              │  │                                └─ Parent span ID
              │  └─ Trace ID (128-bit)
              └─ Version (00)

tracestate: mcp=tenant:acme,cost:0.05,rojo=00f067aa0ba902b7
            │                          │
            │                          └─ Other vendor data
            └─ MCP-specific data
```

### Context Components

```go
type TraceContext struct {
    TraceID    trace.TraceID  // 128-bit globally unique
    SpanID     trace.SpanID   // 64-bit unique within trace
    TraceFlags trace.Flags   // Sampling decision
    TraceState trace.State   // Vendor-specific data
}
```

## Propagation Patterns

### HTTP Services

```go
// HTTP Client - Inject context
func (c *HTTPClient) DoRequest(ctx context.Context, req *http.Request) (*http.Response, error) {
    // Inject trace context into headers
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
    
    // Add custom headers
    span := trace.SpanFromContext(ctx)
    req.Header.Set("X-Trace-ID", span.SpanContext().TraceID().String())
    req.Header.Set("X-Request-ID", middleware.GetRequestID(ctx))
    
    // Execute request
    return c.client.Do(req)
}

// HTTP Server - Extract context
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        // Extract trace context from headers
        ctx := otel.GetTextMapPropagator().Extract(
            c.Request.Context(),
            propagation.HeaderCarrier(c.Request.Header),
        )
        
        // Start server span
        tracer := otel.Tracer("edge-mcp")
        ctx, span := tracer.Start(ctx, 
            fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()
        
        // Set span attributes
        span.SetAttributes(
            semconv.HTTPMethodKey.String(c.Request.Method),
            semconv.HTTPTargetKey.String(c.Request.URL.Path),
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
        )
        
        // Continue with context
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### gRPC Services

```go
// Client Interceptor
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
    return func(ctx context.Context, method string, req, reply interface{}, 
        cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
        
        // Start client span
        tracer := otel.Tracer("mcp-grpc-client")
        ctx, span := tracer.Start(ctx, method,
            trace.WithSpanKind(trace.SpanKindClient),
        )
        defer span.End()
        
        // Inject context into metadata
        md, ok := metadata.FromOutgoingContext(ctx)
        if !ok {
            md = metadata.New(nil)
        }
        
        otel.GetTextMapPropagator().Inject(ctx, MetadataCarrier(md))
        ctx = metadata.NewOutgoingContext(ctx, md)
        
        // Call service
        err := invoker(ctx, method, req, reply, cc, opts...)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        
        return err
    }
}

// Server Interceptor
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
    return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo,
        handler grpc.UnaryHandler) (interface{}, error) {
        
        // Extract context from metadata
        md, ok := metadata.FromIncomingContext(ctx)
        if ok {
            ctx = otel.GetTextMapPropagator().Extract(ctx, MetadataCarrier(md))
        }
        
        // Start server span
        tracer := otel.Tracer("mcp-grpc-server")
        ctx, span := tracer.Start(ctx, info.FullMethod,
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()
        
        // Handle request
        resp, err := handler(ctx, req)
        if err != nil {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        
        return resp, err
    }
}
```

### Message Queue (SQS)

```go
// Producer - Add trace context to message
func (p *SQSProducer) SendMessage(ctx context.Context, message *Message) error {
    span := trace.SpanFromContext(ctx)
    
    // Create carrier for message attributes
    carrier := make(propagation.MapCarrier)
    otel.GetTextMapPropagator().Inject(ctx, carrier)
    
    // Convert to SQS message attributes
    msgAttrs := make(map[string]*sqs.MessageAttributeValue)
    for k, v := range carrier {
        msgAttrs[k] = &sqs.MessageAttributeValue{
            StringValue: aws.String(v),
            DataType:    aws.String("String"),
        }
    }
    
    // Add custom attributes
    msgAttrs["TraceID"] = &sqs.MessageAttributeValue{
        StringValue: aws.String(span.SpanContext().TraceID().String()),
        DataType:    aws.String("String"),
    }
    
    // Send message
    _, err := p.sqs.SendMessage(&sqs.SendMessageInput{
        QueueUrl:          aws.String(p.queueURL),
        MessageBody:       aws.String(message.Body),
        MessageAttributes: msgAttrs,
    })
    
    return err
}

// Consumer - Extract trace context
func (c *SQSConsumer) ProcessMessage(msg *sqs.Message) error {
    // Extract trace context from attributes
    carrier := make(propagation.MapCarrier)
    for k, v := range msg.MessageAttributes {
        if v.StringValue != nil {
            carrier[k] = *v.StringValue
        }
    }
    
    ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
    
    // Start consumer span
    tracer := otel.Tracer("mcp-worker")
    ctx, span := tracer.Start(ctx, "ProcessSQSMessage",
        trace.WithSpanKind(trace.SpanKindConsumer),
    )
    defer span.End()
    
    // Add message attributes
    span.SetAttributes(
        attribute.String("messaging.system", "sqs"),
        attribute.String("messaging.message_id", *msg.MessageId),
        attribute.String("messaging.url", c.queueURL),
    )
    
    // Process message
    return c.handler(ctx, msg)
}
```

## Service Integration Points

### MCP Server → REST API

```go
// MCP Server calling REST API
func (s *MCPServer) CallRESTAPI(ctx context.Context, endpoint string, data interface{}) error {
    // Create HTTP request
    body, _ := json.Marshal(data)
    req, err := http.NewRequestWithContext(ctx, "POST", s.restAPIURL+endpoint, bytes.NewReader(body))
    if err != nil {
        return err
    }
    
    // Trace context is automatically propagated by HTTP client
    resp, err := s.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    return nil
}
```

### REST API → Worker (via SQS)

```go
// REST API publishing to worker queue
func (h *TaskHandler) CreateTask(ctx context.Context, task *Task) error {
    span := trace.SpanFromContext(ctx)
    
    // Create task with trace context
    taskMsg := &TaskMessage{
        TaskID:    task.ID,
        Type:      task.Type,
        Payload:   task.Payload,
        TraceID:   span.SpanContext().TraceID().String(),
        SpanID:    span.SpanContext().SpanID().String(),
        Timestamp: time.Now(),
    }
    
    // Send to queue (context propagated automatically)
    return h.queue.SendMessage(ctx, taskMsg)
}
```

### Worker → External Services

```go
// Worker calling Bedrock
func (w *Worker) CallBedrock(ctx context.Context, request *BedrockRequest) (*BedrockResponse, error) {
    // Start span for external call
    ctx, span := w.tracer.Start(ctx, "CallBedrock",
        trace.WithSpanKind(trace.SpanKindClient),
    )
    defer span.End()
    
    // Add service-specific attributes
    span.SetAttributes(
        attribute.String("bedrock.model", request.Model),
        attribute.Int("bedrock.max_tokens", request.MaxTokens),
        attribute.Float64("bedrock.temperature", request.Temperature),
    )
    
    // Make request (AWS SDK handles trace propagation via X-Ray)
    result, err := w.bedrock.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String(request.Model),
        Body:        request.Body,
        ContentType: aws.String("application/json"),
    })
    
    if err != nil {
        span.RecordError(err)
        return nil, err
    }
    
    // Record cost
    cost := calculateBedrockCost(request.Model, len(request.Body), len(result.Body))
    span.SetAttributes(attribute.Float64("bedrock.cost_usd", cost))
    
    return parseBedrockResponse(result)
}
```

## Implementation Guide

### Step 1: Initialize Tracing

```go
// pkg/observability/tracing.go
package observability

import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/jaeger"
    "go.opentelemetry.io/otel/propagation"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing(serviceName string) (*trace.TracerProvider, error) {
    // Create Jaeger exporter
    exp, err := jaeger.New(
        jaeger.WithCollectorEndpoint(jaeger.WithEndpoint("http://jaeger:14268/api/traces")),
    )
    if err != nil {
        return nil, err
    }
    
    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exp),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String(serviceName),
            semconv.ServiceVersionKey.String(version),
            attribute.String("environment", environment),
        )),
        trace.WithSampler(trace.TraceIDRatioBased(samplingRate)),
    )
    
    // Set global provider
    otel.SetTracerProvider(tp)
    
    // Set global propagator
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},
            propagation.Baggage{},
            // Add custom propagator for MCP-specific data
            &MCPPropagator{},
        ),
    )
    
    return tp, nil
}
```

### Step 2: Create Custom Propagator

```go
// Custom propagator for MCP-specific context
type MCPPropagator struct{}

func (p *MCPPropagator) Inject(ctx context.Context, carrier propagation.TextMapCarrier) {
    // Extract MCP context
    if mcpCtx, ok := ctx.Value(mcpContextKey).(*MCPContext); ok {
        carrier.Set("X-MCP-Tenant-ID", mcpCtx.TenantID)
        carrier.Set("X-MCP-User-ID", mcpCtx.UserID)
        carrier.Set("X-MCP-Session-ID", mcpCtx.SessionID)
        carrier.Set("X-MCP-Cost-Limit", fmt.Sprintf("%.2f", mcpCtx.CostLimit))
    }
}

func (p *MCPPropagator) Extract(ctx context.Context, carrier propagation.TextMapCarrier) context.Context {
    mcpCtx := &MCPContext{}
    
    if tenantID := carrier.Get("X-MCP-Tenant-ID"); tenantID != "" {
        mcpCtx.TenantID = tenantID
    }
    if userID := carrier.Get("X-MCP-User-ID"); userID != "" {
        mcpCtx.UserID = userID
    }
    if sessionID := carrier.Get("X-MCP-Session-ID"); sessionID != "" {
        mcpCtx.SessionID = sessionID
    }
    if costLimit := carrier.Get("X-MCP-Cost-Limit"); costLimit != "" {
        mcpCtx.CostLimit, _ = strconv.ParseFloat(costLimit, 64)
    }
    
    return context.WithValue(ctx, mcpContextKey, mcpCtx)
}

func (p *MCPPropagator) Fields() []string {
    return []string{
        "X-MCP-Tenant-ID",
        "X-MCP-User-ID", 
        "X-MCP-Session-ID",
        "X-MCP-Cost-Limit",
    }
}
```

### Step 3: Instrument Service Boundaries

```go
// HTTP Client Factory
func NewTracedHTTPClient() *http.Client {
    return &http.Client{
        Transport: otelhttp.NewTransport(http.DefaultTransport),
        Timeout:   30 * time.Second,
    }
}

// Database Connection
func NewTracedDB(dsn string) (*sql.DB, error) {
    db, err := otelsql.Open("postgres", dsn)
    if err != nil {
        return nil, err
    }
    
    // Register metrics
    err = otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(
        semconv.DBSystemPostgreSQL,
    ))
    
    return db, err
}

// Redis Client
func NewTracedRedis(addr string) *redis.Client {
    rdb := redis.NewClient(&redis.Options{
        Addr: addr,
    })
    
    // Add tracing hook
    rdb.AddHook(redisotel.TracingHook{})
    
    return rdb
}
```

## WebSocket Trace Propagation <!-- Source: pkg/models/websocket/binary.go -->

### WebSocket Connection Establishment <!-- Source: pkg/models/websocket/binary.go -->

```go
// Client-side connection with trace context
func (c *WSClient) Connect(ctx context.Context, url string) error {
    // Create connection headers with trace context
    headers := http.Header{}
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(headers))
    
    // Add WebSocket-specific headers <!-- Source: pkg/models/websocket/binary.go -->
    span := trace.SpanFromContext(ctx)
    headers.Set("X-Agent-ID", c.agentID)
    headers.Set("X-Trace-ID", span.SpanContext().TraceID().String())
    
    // Establish connection
    conn, _, err := websocket.DefaultDialer.DialContext(ctx, url, headers) <!-- Source: pkg/models/websocket/binary.go -->
    if err != nil {
        return err
    }
    
    c.conn = conn
    return nil
}

// Server-side connection handling
func (s *WSServer) HandleConnection(w http.ResponseWriter, r *http.Request) {
    // Extract trace context from headers
    ctx := otel.GetTextMapPropagator().Extract(r.Context(), propagation.HeaderCarrier(r.Header))
    
    // Start connection span
    ctx, span := s.tracer.Start(ctx, "WebSocketConnection", <!-- Source: pkg/models/websocket/binary.go -->
        trace.WithSpanKind(trace.SpanKindServer),
    )
    defer span.End()
    
    // Upgrade connection
    conn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        span.RecordError(err)
        return
    }
    
    // Store trace context with connection
    s.connections.Store(conn, &ConnectionContext{
        Conn:    conn,
        Context: ctx,
        TraceID: span.SpanContext().TraceID().String(),
        AgentID: r.Header.Get("X-Agent-ID"),
    })
}
```

### WebSocket Message Propagation <!-- Source: pkg/models/websocket/binary.go -->

```go
// Message with trace context
type WSMessage struct {
    Type    MessageType `json:"type"`
    Payload json.RawMessage `json:"payload"`
    
    // Trace context
    TraceContext struct {
        TraceID    string            `json:"trace_id"`
        SpanID     string            `json:"span_id"`
        TraceFlags uint8             `json:"trace_flags"`
        TraceState map[string]string `json:"trace_state,omitempty"`
    } `json:"trace_context"`
}

// Send message with trace propagation
func (c *WSConnection) SendMessage(ctx context.Context, msg *WSMessage) error {
    // Inject current trace context
    span := trace.SpanFromContext(ctx)
    msg.TraceContext.TraceID = span.SpanContext().TraceID().String()
    msg.TraceContext.SpanID = span.SpanContext().SpanID().String()
    msg.TraceContext.TraceFlags = uint8(span.SpanContext().TraceFlags())
    
    // Add trace state
    msg.TraceContext.TraceState = make(map[string]string)
    if mcpCtx, ok := ctx.Value(mcpContextKey).(*MCPContext); ok {
        msg.TraceContext.TraceState["tenant_id"] = mcpCtx.TenantID
        msg.TraceContext.TraceState["cost_used"] = fmt.Sprintf("%.4f", mcpCtx.CostUsed)
    }
    
    // Send message
    return c.conn.WriteJSON(msg)
}

// Receive message with trace extraction
func (c *WSConnection) ReceiveMessage() (*WSMessage, context.Context, error) {
    var msg WSMessage
    if err := c.conn.ReadJSON(&msg); err != nil {
        return nil, nil, err
    }
    
    // Reconstruct trace context
    traceID, _ := trace.TraceIDFromHex(msg.TraceContext.TraceID)
    spanID, _ := trace.SpanIDFromHex(msg.TraceContext.SpanID)
    
    spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    traceID,
        SpanID:     spanID,
        TraceFlags: trace.TraceFlags(msg.TraceContext.TraceFlags),
        Remote:     true,
    })
    
    // Create context with span
    ctx := trace.ContextWithRemoteSpanContext(context.Background(), spanCtx)
    
    // Restore MCP context
    if tenantID, ok := msg.TraceContext.TraceState["tenant_id"]; ok {
        mcpCtx := &MCPContext{TenantID: tenantID}
        if costUsed, ok := msg.TraceContext.TraceState["cost_used"]; ok {
            mcpCtx.CostUsed, _ = strconv.ParseFloat(costUsed, 64)
        }
        ctx = context.WithValue(ctx, mcpContextKey, mcpCtx)
    }
    
    return &msg, ctx, nil
}
```

## Async Operations

### Task Queue with Trace Links

```go
// Submit async task with trace link
func (s *TaskService) SubmitAsync(ctx context.Context, task *Task) error {
    // Get current span
    span := trace.SpanFromContext(ctx)
    link := trace.Link{
        SpanContext: span.SpanContext(),
        Attributes: []attribute.KeyValue{
            attribute.String("link.type", "async_task"),
            attribute.String("task.id", task.ID),
        },
    }
    
    // Store link in task
    task.TraceLinks = []TraceLink{{
        TraceID: span.SpanContext().TraceID().String(),
        SpanID:  span.SpanContext().SpanID().String(),
    }}
    
    // Submit to queue
    return s.queue.Submit(task)
}

// Process async task with trace link
func (w *Worker) ProcessTask(task *Task) error {
    // Create new trace with link to parent
    links := make([]trace.Link, len(task.TraceLinks))
    for i, link := range task.TraceLinks {
        traceID, _ := trace.TraceIDFromHex(link.TraceID)
        spanID, _ := trace.SpanIDFromHex(link.SpanID)
        
        links[i] = trace.Link{
            SpanContext: trace.NewSpanContext(trace.SpanContextConfig{
                TraceID: traceID,
                SpanID:  spanID,
                Remote:  true,
            }),
        }
    }
    
    // Start new trace with links
    ctx, span := w.tracer.Start(context.Background(), "ProcessAsyncTask",
        trace.WithLinks(links...),
        trace.WithSpanKind(trace.SpanKindConsumer),
    )
    defer span.End()
    
    // Process task
    return w.processTask(ctx, task)
}
```

### Batch Operations

```go
// Batch processing with trace propagation
func (s *BatchService) ProcessBatch(ctx context.Context, items []Item) error {
    ctx, span := s.tracer.Start(ctx, "ProcessBatch")
    defer span.End()
    
    span.SetAttributes(
        attribute.Int("batch.size", len(items)),
        attribute.String("batch.id", generateBatchID()),
    )
    
    // Process items in parallel with trace propagation
    g, gCtx := errgroup.WithContext(ctx)
    
    for i, item := range items {
        item := item // Capture loop variable
        itemIndex := i
        
        g.Go(func() error {
            // Create child span for each item
            _, itemSpan := s.tracer.Start(gCtx, "ProcessBatchItem",
                trace.WithAttributes(
                    attribute.Int("item.index", itemIndex),
                    attribute.String("item.id", item.ID),
                ),
            )
            defer itemSpan.End()
            
            return s.processItem(gCtx, item)
        })
    }
    
    return g.Wait()
}
```

## Error Propagation

### Error Context Enrichment

```go
// Error with trace context
type TracedError struct {
    Err     error
    TraceID string
    SpanID  string
    Service string
    Details map[string]interface{}
}

func (e *TracedError) Error() string {
    return fmt.Sprintf("[%s] %s (trace:%s)", e.Service, e.Err.Error(), e.TraceID)
}

// Wrap error with trace context
func WrapErrorWithTrace(ctx context.Context, err error, details ...map[string]interface{}) error {
    if err == nil {
        return nil
    }
    
    span := trace.SpanFromContext(ctx)
    span.RecordError(err)
    
    tracedErr := &TracedError{
        Err:     err,
        TraceID: span.SpanContext().TraceID().String(),
        SpanID:  span.SpanContext().SpanID().String(),
        Service: ServiceName,
        Details: make(map[string]interface{}),
    }
    
    // Merge details
    for _, d := range details {
        for k, v := range d {
            tracedErr.Details[k] = v
        }
    }
    
    // Add to span attributes
    for k, v := range tracedErr.Details {
        span.SetAttributes(attribute.String(fmt.Sprintf("error.detail.%s", k), fmt.Sprintf("%v", v)))
    }
    
    return tracedErr
}
```

### Cross-Service Error Handling

```go
// Service A - Error origination
func (s *ServiceA) DoOperation(ctx context.Context) error {
    ctx, span := s.tracer.Start(ctx, "DoOperation")
    defer span.End()
    
    if err := s.validateInput(); err != nil {
        return WrapErrorWithTrace(ctx, err, map[string]interface{}{
            "operation": "input_validation",
            "service":   "service_a",
        })
    }
    
    // Call Service B
    if err := s.callServiceB(ctx); err != nil {
        return WrapErrorWithTrace(ctx, err, map[string]interface{}{
            "operation": "service_b_call",
            "endpoint":  "/api/v1/process",
        })
    }
    
    return nil
}

// Service B - Error propagation
func (s *ServiceB) HandleRequest(ctx context.Context, req *Request) error {
    ctx, span := s.tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // Check for traced error from upstream
    if tracedErr, ok := err.(*TracedError); ok {
        span.AddEvent("Upstream error received", trace.WithAttributes(
            attribute.String("upstream.trace_id", tracedErr.TraceID),
            attribute.String("upstream.service", tracedErr.Service),
        ))
        
        // Log with correlation
        s.logger.Error("Upstream error",
            "trace_id", span.SpanContext().TraceID().String(),
            "upstream_trace_id", tracedErr.TraceID,
            "error", tracedErr.Error(),
        )
    }
    
    return err
}
```

## Testing Trace Propagation

### Unit Tests

```go
func TestTracePropagation(t *testing.T) {
    // Create test tracer
    tp := trace.NewTracerProvider(
        trace.WithSyncer(tracetest.NewInMemoryExporter()),
    )
    
    tracer := tp.Tracer("test")
    
    // Start parent span
    ctx, parentSpan := tracer.Start(context.Background(), "parent")
    parentTraceID := parentSpan.SpanContext().TraceID()
    
    // Simulate HTTP propagation
    carrier := propagation.HeaderCarrier(http.Header{})
    otel.GetTextMapPropagator().Inject(ctx, carrier)
    
    // Extract in new context
    newCtx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
    
    // Start child span
    _, childSpan := tracer.Start(newCtx, "child")
    childTraceID := childSpan.SpanContext().TraceID()
    
    // Verify same trace ID
    assert.Equal(t, parentTraceID, childTraceID)
}
```

### Integration Tests

```go
func TestCrossServiceTracing(t *testing.T) {
    // Start test services with tracing
    serviceA := startTestService("service-a", 8080)
    serviceB := startTestService("service-b", 8081)
    defer serviceA.Stop()
    defer serviceB.Stop()
    
    // Make request with forced sampling
    req, _ := http.NewRequest("POST", "http://localhost:8080/api/task", nil)
    req.Header.Set("X-B3-Sampled", "1") // Force sampling
    
    resp, err := http.DefaultClient.Do(req)
    require.NoError(t, err)
    
    // Get trace ID from response
    traceID := resp.Header.Get("X-Trace-ID")
    require.NotEmpty(t, traceID)
    
    // Wait for trace to be collected
    time.Sleep(2 * time.Second)
    
    // Query Jaeger for trace
    trace := queryJaeger(traceID)
    require.NotNil(t, trace)
    
    // Verify spans from both services
    serviceASpans := filterSpansByService(trace.Spans, "service-a")
    serviceBSpans := filterSpansByService(trace.Spans, "service-b")
    
    assert.NotEmpty(t, serviceASpans)
    assert.NotEmpty(t, serviceBSpans)
    
    // Verify parent-child relationships
    for _, span := range serviceBSpans {
        assert.Equal(t, traceID, span.TraceID)
    }
}
```

### Load Testing with Tracing

```go
func TestTracePerformanceUnderLoad(t *testing.T) {
    // Configure low sampling for load test
    tp := trace.NewTracerProvider(
        trace.WithSampler(trace.TraceIDRatioBased(0.001)), // 0.1% sampling
    )
    
    // Run load test
    results := vegeta.Attack(
        vegeta.NewStaticTargeter(vegeta.Target{
            Method: "POST",
            URL:    "http://localhost:8080/api/tasks",
        }),
        vegeta.Rate{Freq: 1000, Per: time.Second}, // 1000 RPS
        vegeta.Duration(30*time.Second),
    )
    
    // Collect metrics
    var metrics vegeta.Metrics
    for res := range results {
        metrics.Add(res)
        
        // Check if trace was sampled
        if res.Headers.Get("X-B3-Sampled") == "1" {
            // Verify trace propagation worked
            traceID := res.Headers.Get("X-Trace-ID")
            assert.NotEmpty(t, traceID)
        }
    }
    
    // Verify performance impact
    assert.Less(t, metrics.Latencies.P99, 100*time.Millisecond)
}
```

## Troubleshooting

### Common Issues

#### 1. Missing Trace Context

**Symptom**: Child spans appear as separate traces

**Debug Steps**:
```go
// Add debug logging
func DebugTraceContext(ctx context.Context, operation string) {
    span := trace.SpanFromContext(ctx)
    spanCtx := span.SpanContext()
    
    log.Printf("[%s] Trace: %s, Span: %s, Sampled: %v",
        operation,
        spanCtx.TraceID(),
        spanCtx.SpanID(),
        spanCtx.IsSampled(),
    )
}
```

**Common Causes**:
- Context not propagated through goroutines
- Missing middleware in service
- Incorrect propagator configuration

#### 2. Trace Context Loss in Async Operations

**Solution**: Use trace links
```go
// Store trace context before async operation
type AsyncJob struct {
    ID          string
    TraceID     string
    SpanID      string
    TraceFlags  byte
}

// Restore context when processing
func RestoreTraceContext(job *AsyncJob) context.Context {
    traceID, _ := trace.TraceIDFromHex(job.TraceID)
    spanID, _ := trace.SpanIDFromHex(job.SpanID)
    
    spanCtx := trace.NewSpanContext(trace.SpanContextConfig{
        TraceID:    traceID,
        SpanID:     spanID,
        TraceFlags: trace.TraceFlags(job.TraceFlags),
        Remote:     true,
    })
    
    return trace.ContextWithRemoteSpanContext(context.Background(), spanCtx)
}
```

#### 3. Different Trace IDs Between Services

**Debug Checklist**:
- Verify propagator configuration matches
- Check header names are consistent
- Ensure middleware order is correct
- Validate trace context format

```bash
# Debug headers
curl -v -H "traceparent: 00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01" \
  http://localhost:8080/api/health

# Check response headers
< X-Trace-ID: 4bf92f3577b34da6a3ce929d0e0e4736
```

### Trace Validation

```go
// Validate trace propagation in tests
func ValidateTracePropagation(t *testing.T, spans []Span) {
    traceIDs := make(map[string]bool)
    parentChild := make(map[string]string)
    
    for _, span := range spans {
        traceIDs[span.TraceID] = true
        if span.ParentID != "" {
            parentChild[span.SpanID] = span.ParentID
        }
    }
    
    // Should have single trace
    assert.Len(t, traceIDs, 1, "Multiple trace IDs found")
    
    // Validate parent-child relationships
    for child, parent := range parentChild {
        _, found := findSpanByID(spans, parent)
        assert.True(t, found, "Parent span %s not found for child %s", parent, child)
    }
}
```

## Best Practices

1. **Always propagate context**: Never create new empty contexts in the middle of a request
2. **Use standard propagators**: Stick to W3C Trace Context for interoperability
3. **Add service boundaries**: Create spans at every service entry/exit point
4. **Include relevant attributes**: Add enough context to debug issues
5. **Handle async carefully**: Use trace links for async operations
6. **Test propagation**: Include trace propagation in integration tests
7. **Monitor sampling**: Ensure critical traces are always sampled
8. **Document trace flow**: Maintain diagrams of trace propagation paths

## Next Steps

1. Review [Trace-Based Debugging](./trace-based-debugging.md) for debugging techniques
2. See [Trace Sampling Guide](./trace-sampling-guide.md) for sampling strategies
3. Check [Observability Architecture](./observability-architecture.md) for system design
