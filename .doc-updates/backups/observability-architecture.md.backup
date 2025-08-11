# Observability Architecture Guide

> **Purpose**: Comprehensive guide to the Developer Mesh observability architecture
> **Audience**: Platform engineers, SREs, and developers implementing monitoring
> **Scope**: Metrics, logs, traces integration and best practices

## Table of Contents

1. [Overview](#overview)
2. [Observability Pillars](#observability-pillars)
3. [Architecture Design](#architecture-design)
4. [Metrics Architecture](#metrics-architecture)
5. [Logging Architecture](#logging-architecture)
6. [Tracing Architecture](#tracing-architecture)
7. [Correlation and Context](#correlation-and-context)
8. [Data Flow](#data-flow)
9. [Storage and Retention](#storage-and-retention)
10. [Query and Analysis](#query-and-analysis)
11. [Alerting Architecture](#alerting-architecture)
12. [Implementation Guidelines](#implementation-guidelines)

## Overview

The Developer Mesh platform implements a comprehensive observability architecture based on the three pillars: metrics, logs, and traces. This unified approach provides complete visibility into system behavior, performance, and issues.

### Key Principles

1. **Correlation**: All telemetry data is correlated through trace IDs
2. **Context Propagation**: Context flows through all service boundaries
3. **Structured Data**: All telemetry uses structured formats
4. **Low Overhead**: Minimal performance impact
5. **Actionable Insights**: Focus on meaningful data

### Technology Stack

- **Metrics**: Prometheus + Grafana
- **Logging**: Structured JSON + Loki
- **Tracing**: OpenTelemetry + Jaeger
- **Alerting**: AlertManager + PagerDuty
- **Storage**: Time-series databases + S3

## Observability Pillars

### Metrics (What's Happening)

Metrics provide quantitative data about system behavior:
- Request rates and error rates
- Latency percentiles (p50, p95, p99)
- Resource utilization (CPU, memory, disk)
- Business metrics (tasks processed, costs)
- WebSocket connections and message rates

### Logs (Why It Happened)

Logs provide detailed context about specific events:
- Structured JSON with consistent fields
- Trace ID correlation
- Error details and stack traces
- Business event tracking
- Audit trails

### Traces (How It Happened)

Traces show the complete journey of requests:
- End-to-end request flow
- Service dependencies
- Performance bottlenecks
- Error propagation
- Distributed debugging

## Architecture Design

```
┌─────────────────────────────────────────────────────────────┐
│                     Application Layer                        │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │   Metrics   │  │    Logs     │  │   Traces    │       │
│  │ Prometheus  │  │ Structured  │  │OpenTelemetry│       │
│  │  Client     │  │    JSON     │  │     SDK     │       │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘       │
│         │                 │                 │               │
├─────────┼─────────────────┼─────────────────┼───────────────┤
│         │                 │                 │               │
│         ▼                 ▼                 ▼               │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐       │
│  │ Prometheus  │  │    Loki     │  │   Jaeger    │       │
│  │   Server    │  │             │  │  Collector  │       │
│  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘       │
│         │                 │                 │               │
├─────────┼─────────────────┼─────────────────┼───────────────┤
│         │                 │                 │               │
│         ▼                 ▼                 ▼               │
│  ┌────────────────────────────────────────────────┐       │
│  │           Unified Query Layer (Grafana)         │       │
│  └────────────────────────────────────────────────┘       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Metrics Architecture

### Metric Types and Usage

```go
// Counter: Cumulative values that only increase
var requestsTotal = prometheus.NewCounterVec(
    prometheus.CounterOpts{
        Name: "mcp_requests_total",
        Help: "Total number of requests",
    },
    []string{"service", "method", "status"},
)

// Gauge: Values that can go up or down
var connectionsActive = prometheus.NewGaugeVec(
    prometheus.GaugeOpts{
        Name: "mcp_connections_active",
        Help: "Number of active connections",
    },
    []string{"type", "protocol"},
)

// Histogram: Distribution of values
var requestDuration = prometheus.NewHistogramVec(
    prometheus.HistogramOpts{
        Name: "mcp_request_duration_seconds",
        Help: "Request duration in seconds",
        Buckets: prometheus.ExponentialBuckets(0.001, 2, 10),
    },
    []string{"service", "method"},
)

// Summary: Similar to histogram with quantiles
var taskProcessingTime = prometheus.NewSummaryVec(
    prometheus.SummaryOpts{
        Name: "mcp_task_processing_seconds",
        Help: "Task processing time",
        Objectives: map[float64]float64{
            0.5:  0.05,  // 50th percentile
            0.9:  0.01,  // 90th percentile
            0.99: 0.001, // 99th percentile
        },
    },
    []string{"task_type", "agent_type"},
)
```

### Metric Collection Pipeline

```yaml
# Prometheus configuration
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'production'
    region: 'us-east-1'

scrape_configs:
  - job_name: 'mcp-services'
    kubernetes_sd_configs:
      - role: pod
        namespaces:
          names: ['mcp-prod']
    
    relabel_configs:
      # Only scrape pods with annotation
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
        action: keep
        regex: true
      
      # Use pod annotation for metrics path
      - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_path]
        action: replace
        target_label: __metrics_path__
        regex: (.+)
      
      # Extract service name from pod labels
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: replace
        target_label: service

# Remote storage for long-term retention
remote_write:
  - url: https://prometheus-storage.example.com/api/v1/write
    queue_config:
      capacity: 10000
      max_shards: 30
      max_samples_per_send: 5000
```

### Business Metrics Implementation

```go
// Track business-specific metrics
type BusinessMetrics struct {
    tasksProcessed   *prometheus.CounterVec
    taskValue        *prometheus.HistogramVec
    costPerTask      *prometheus.HistogramVec
    agentUtilization *prometheus.GaugeVec
}

func (m *BusinessMetrics) RecordTaskCompletion(task *Task, agent *Agent) {
    labels := prometheus.Labels{
        "task_type":  task.Type,
        "agent_type": agent.Type,
        "priority":   task.Priority.String(),
    }
    
    m.tasksProcessed.With(labels).Inc()
    m.taskValue.With(labels).Observe(task.BusinessValue)
    m.costPerTask.With(labels).Observe(task.Cost)
    
    // Update agent utilization
    m.agentUtilization.WithLabelValues(
        agent.ID,
        agent.Type,
    ).Set(agent.CurrentWorkload)
}
```

## Logging Architecture

### Structured Logging Design

```go
// Logger configuration
type Logger interface {
    Info(msg string, fields ...Field)
    Error(msg string, fields ...Field)
    Debug(msg string, fields ...Field)
    With(fields ...Field) Logger
}

// Consistent field structure
type Field struct {
    Key   string
    Value interface{}
}

// Common fields for correlation
func CommonFields(ctx context.Context) []Field {
    return []Field{
        {"trace_id", trace.SpanFromContext(ctx).SpanContext().TraceID()},
        {"span_id", trace.SpanFromContext(ctx).SpanContext().SpanID()},
        {"user_id", ctx.Value("user_id")},
        {"tenant_id", ctx.Value("tenant_id")},
        {"service", ServiceName},
        {"version", Version},
        {"environment", Environment},
    }
}

// Usage example
func ProcessRequest(ctx context.Context, req *Request) error {
    logger := log.With(CommonFields(ctx)...)
    logger.Info("Processing request", 
        Field{"request_id", req.ID},
        Field{"method", req.Method},
    )
    
    // Process request...
    
    if err != nil {
        logger.Error("Request failed",
            Field{"error", err.Error()},
            Field{"stack_trace", debug.Stack()},
        )
        return err
    }
    
    return nil
}
```

### Log Aggregation Pipeline

```yaml
# Loki configuration
auth_enabled: false

server:
  http_listen_port: 3100
  grpc_listen_port: 9096

ingester:
  wal:
    enabled: true
    dir: /loki/wal
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

schema_config:
  configs:
    - from: 2024-01-01
      store: boltdb-shipper
      object_store: s3
      schema: v11
      index:
        prefix: index_
        period: 24h

storage_config:
  boltdb_shipper:
    active_index_directory: /loki/boltdb-shipper-active
    cache_location: /loki/boltdb-shipper-cache
    shared_store: s3
  aws:
    bucketnames: mcp-logs
    region: us-east-1

limits_config:
  enforce_metric_name: false
  reject_old_samples: true
  reject_old_samples_max_age: 168h
  ingestion_rate_mb: 16
  ingestion_burst_size_mb: 32
```

### Log Processing Rules

```yaml
# Promtail configuration for log shipping
clients:
  - url: http://loki:3100/loki/api/v1/push

positions:
  filename: /tmp/positions.yaml

scrape_configs:
  - job_name: kubernetes-pods
    kubernetes_sd_configs:
      - role: pod
    
    pipeline_stages:
      # Parse JSON logs
      - json:
          expressions:
            timestamp: timestamp
            level: level
            message: message
            trace_id: trace_id
            span_id: span_id
            service: service
            
      # Extract additional labels
      - labels:
          level:
          service:
          trace_id:
          
      # Parse specific patterns
      - regex:
          expression: '(?P<duration>\d+\.?\d*)ms'
      
      # Add derived fields
      - template:
          source: duration
          template: '{{ .Value }}'
      
      # Filter sensitive data
      - replace:
          expression: '([0-9]{4}-){3}[0-9]{4}'
          replace: '****'
```

## Tracing Architecture

### OpenTelemetry Implementation

```go
// Tracer initialization
func InitTracing(serviceName string) (*trace.TracerProvider, error) {
    // Create Jaeger exporter
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
        ),
    )
    if err != nil {
        return nil, err
    }
    
    // Create resource
    resource := resource.NewWithAttributes(
        semconv.SchemaURL,
        semconv.ServiceNameKey.String(serviceName),
        semconv.ServiceVersionKey.String(version),
        semconv.DeploymentEnvironmentKey.String(environment),
        attribute.String("region", region),
        attribute.String("cluster", cluster),
    )
    
    // Create tracer provider
    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource),
        trace.WithSampler(trace.TraceIDRatioBased(0.1)), // 10% sampling
        trace.WithSpanProcessor(
            // Custom processor for adding attributes
            &CustomSpanProcessor{},
        ),
    )
    
    // Set global provider
    otel.SetTracerProvider(tp)
    
    // Configure propagation
    otel.SetTextMapPropagator(
        propagation.NewCompositeTextMapPropagator(
            propagation.TraceContext{},
            propagation.Baggage{},
        ),
    )
    
    return tp, nil
}
```

### Span Creation and Management

```go
// HTTP middleware for automatic tracing
func TracingMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        tracer := otel.Tracer("http-server")
        
        // Extract parent context
        ctx := otel.GetTextMapPropagator().Extract(
            c.Request.Context(),
            propagation.HeaderCarrier(c.Request.Header),
        )
        
        // Start span
        ctx, span := tracer.Start(ctx, 
            fmt.Sprintf("%s %s", c.Request.Method, c.Request.URL.Path),
            trace.WithSpanKind(trace.SpanKindServer),
        )
        defer span.End()
        
        // Add standard attributes
        span.SetAttributes(
            semconv.HTTPMethodKey.String(c.Request.Method),
            semconv.HTTPURLKey.String(c.Request.URL.String()),
            semconv.HTTPUserAgentKey.String(c.Request.UserAgent()),
            semconv.NetPeerIPKey.String(c.ClientIP()),
        )
        
        // Store span in context
        c.Request = c.Request.WithContext(ctx)
        
        // Process request
        c.Next()
        
        // Add response attributes
        span.SetAttributes(
            semconv.HTTPStatusCodeKey.Int(c.Writer.Status()),
            attribute.Int("response.size", c.Writer.Size()),
        )
        
        // Record error if any
        if c.Writer.Status() >= 400 {
            span.RecordError(fmt.Errorf("HTTP %d", c.Writer.Status()))
            span.SetStatus(codes.Error, http.StatusText(c.Writer.Status()))
        }
    }
}
```

### Custom Span Processors

```go
// Custom processor for adding platform-specific attributes
type CustomSpanProcessor struct{}

func (p *CustomSpanProcessor) OnStart(ctx context.Context, s trace.ReadWriteSpan) {
    // Add custom attributes at span start
    if userID := ctx.Value("user_id"); userID != nil {
        s.SetAttributes(attribute.String("user.id", userID.(string)))
    }
    
    if tenantID := ctx.Value("tenant_id"); tenantID != nil {
        s.SetAttributes(attribute.String("tenant.id", tenantID.(string)))
    }
    
    // Add cost tracking
    if costLimit := ctx.Value("cost_limit"); costLimit != nil {
        s.SetAttributes(attribute.Float64("cost.limit", costLimit.(float64)))
    }
}

func (p *CustomSpanProcessor) OnEnd(s trace.ReadOnlySpan) {
    // Calculate and record costs at span end
    if cost, ok := s.Attributes()["actual_cost"]; ok {
        // Record cost metric
        costMetric.Record(s.SpanContext().TraceID().String(), cost.AsFloat64())
    }
}

func (p *CustomSpanProcessor) Shutdown(ctx context.Context) error {
    return nil
}

func (p *CustomSpanProcessor) ForceFlush(ctx context.Context) error {
    return nil
}
```

## Correlation and Context

### Trace Context Propagation

```go
// Context enrichment middleware
func ContextEnrichmentMiddleware() gin.HandlerFunc {
    return func(c *gin.Context) {
        ctx := c.Request.Context()
        
        // Extract or generate trace ID
        span := trace.SpanFromContext(ctx)
        traceID := span.SpanContext().TraceID()
        
        // Add to all telemetry
        logger := log.With(Field{"trace_id", traceID.String()})
        ctx = context.WithValue(ctx, "logger", logger)
        
        // Add to response headers for client correlation
        c.Header("X-Trace-ID", traceID.String())
        
        // Add to metrics labels
        metrics.WithTraceID(traceID.String())
        
        c.Request = c.Request.WithContext(ctx)
        c.Next()
    }
}
```

### Cross-Service Correlation

```go
// HTTP client with automatic context propagation
type TracedHTTPClient struct {
    client *http.Client
    tracer trace.Tracer
}

func (c *TracedHTTPClient) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
    // Start client span
    ctx, span := c.tracer.Start(ctx, 
        fmt.Sprintf("HTTP %s %s", req.Method, req.URL.Path),
        trace.WithSpanKind(trace.SpanKindClient),
    )
    defer span.End()
    
    // Inject trace context
    otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
    
    // Add trace ID to logs
    logger := log.With(Field{"trace_id", span.SpanContext().TraceID().String()})
    logger.Info("Making HTTP request", Field{"url", req.URL.String()})
    
    // Execute request
    resp, err := c.client.Do(req)
    
    if err != nil {
        span.RecordError(err)
        span.SetStatus(codes.Error, err.Error())
        return nil, err
    }
    
    // Record response
    span.SetAttributes(
        semconv.HTTPStatusCodeKey.Int(resp.StatusCode),
    )
    
    return resp, nil
}
```

## Data Flow

### Telemetry Collection Pipeline

```
Application → Instrumentation → Collectors → Storage → Query → Visualization

1. Application emits telemetry
2. SDK/agents collect and batch data
3. Collectors process and forward
4. Time-series databases store
5. Query engines aggregate
6. Dashboards visualize
```

### Data Processing Rules

```yaml
# Processor configuration
processors:
  batch:
    send_batch_size: 10000
    timeout: 10s
    send_batch_max_size: 11000

  memory_limiter:
    check_interval: 5s
    limit_mib: 512
    spike_limit_mib: 128

  attributes:
    actions:
      - key: environment
        value: production
        action: upsert
      - key: cost.estimated
        from_attribute: estimated_cost
        action: rename
      - key: sensitive_data
        action: delete

  resource:
    attributes:
      - key: service.namespace
        value: mcp
        action: insert
      - key: cloud.provider
        value: aws
        action: insert

  filter:
    metrics:
      exclude:
        match_type: regexp
        metric_names:
          - prefix/.*
          - .*_temp$
```

## Storage and Retention

### Retention Policies

```yaml
# Metrics retention (Prometheus)
metrics:
  raw_data: 15d        # High resolution
  5m_aggregates: 90d   # 5-minute aggregates
  1h_aggregates: 2y    # Hourly aggregates
  
# Logs retention (Loki)
logs:
  hot_tier: 7d         # Recent logs in fast storage
  warm_tier: 30d       # Compressed in slower storage
  cold_tier: 90d       # Archived in S3
  
# Traces retention (Jaeger)
traces:
  sampling_rate: 0.1   # 10% sampling
  retention: 7d        # Full trace data
  index_retention: 30d # Search index
```

### Storage Optimization

```go
// Metric downsampling for long-term storage
func DownsampleMetrics(ctx context.Context) error {
    // Query raw metrics
    query := `avg_over_time(mcp_request_duration_seconds[5m])`
    
    result, err := prometheus.Query(ctx, query, time.Now())
    if err != nil {
        return err
    }
    
    // Write to long-term storage
    for _, sample := range result {
        err := longTermDB.Write(
            "mcp_request_duration_seconds_5m",
            sample.Labels,
            sample.Value,
            sample.Timestamp,
        )
        if err != nil {
            return err
        }
    }
    
    return nil
}
```

## Query and Analysis

### Unified Query Interface

```go
// Query builder for cross-pillar queries
type ObservabilityQuery struct {
    TraceID    string
    TimeRange  TimeRange
    Service    string
    UserID     string
    Filters    map[string]string
}

func (q *ObservabilityQuery) Execute(ctx context.Context) (*ObservabilityResult, error) {
    result := &ObservabilityResult{}
    
    // Get trace data
    if q.TraceID != "" {
        trace, err := jaeger.GetTrace(ctx, q.TraceID)
        if err == nil {
            result.Trace = trace
        }
    }
    
    // Get related logs
    logQuery := fmt.Sprintf(`{trace_id="%s"}`, q.TraceID)
    logs, err := loki.Query(ctx, logQuery, q.TimeRange)
    if err == nil {
        result.Logs = logs
    }
    
    // Get metrics for time window
    metricQuery := fmt.Sprintf(
        `mcp_request_duration_seconds{service="%s"}[%s]`,
        q.Service,
        q.TimeRange.Duration(),
    )
    metrics, err := prometheus.Query(ctx, metricQuery, q.TimeRange.End)
    if err == nil {
        result.Metrics = metrics
    }
    
    return result, nil
}
```

### Analysis Patterns

```sql
-- Find slow traces with high error rates
WITH trace_stats AS (
  SELECT 
    trace_id,
    service_name,
    MAX(duration) as max_duration,
    COUNT(CASE WHEN error = true THEN 1 END) as error_count,
    COUNT(*) as span_count
  FROM spans
  WHERE timestamp > NOW() - INTERVAL '1 hour'
  GROUP BY trace_id, service_name
)
SELECT 
  trace_id,
  service_name,
  max_duration,
  error_count::float / span_count as error_rate
FROM trace_stats
WHERE max_duration > 1000 -- 1 second
  AND error_count > 0
ORDER BY max_duration DESC
LIMIT 100;
```

## Alerting Architecture

### Alert Rule Definition

```yaml
# Prometheus alerting rules
groups:
  - name: performance
    rules:
      - alert: HighLatency
        expr: |
          histogram_quantile(0.95,
            sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)
          ) > 0.5
        for: 5m
        labels:
          severity: warning
          team: platform
        annotations:
          summary: "High latency detected for {{ $labels.service }}"
          description: "P95 latency is {{ $value | humanizeDuration }}"
          runbook_url: "https://wiki.example.com/runbooks/high-latency"
          
      - alert: ErrorRateHigh
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m])) by (service)
          /
          sum(rate(http_requests_total[5m])) by (service)
          > 0.05
        for: 5m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "High error rate for {{ $labels.service }}"
          description: "Error rate is {{ $value | humanizePercentage }}"
          
  - name: business
    rules:
      - alert: CostLimitExceeded
        expr: |
          sum(rate(mcp_cost_total[1h])) * 3600 > 100
        for: 5m
        labels:
          severity: critical
          team: finance
        annotations:
          summary: "Hourly cost limit exceeded"
          description: "Current rate: ${{ $value | humanize }} per hour"
```

### Alert Routing

```yaml
# AlertManager configuration
route:
  receiver: 'default'
  group_by: ['alertname', 'cluster', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  routes:
    - match:
        severity: critical
      receiver: pagerduty
      continue: true
      
    - match:
        severity: warning
      receiver: slack
      
    - match:
        team: finance
      receiver: finance-team

receivers:
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: '${PAGERDUTY_KEY}'
        description: '{{ range .Alerts }}{{ .Annotations.summary }}{{ end }}'
        
  - name: 'slack'
    slack_configs:
      - api_url: '${SLACK_WEBHOOK}'
        channel: '#alerts'
        title: 'Alert: {{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
```

## Implementation Guidelines

### 1. Instrumentation Checklist

```go
// Service instrumentation checklist
type ServiceInstrumentation struct {
    // Metrics
    RequestCounter   *prometheus.CounterVec
    RequestDuration  *prometheus.HistogramVec
    ActiveGauge      prometheus.Gauge
    
    // Logging
    Logger structured.Logger
    
    // Tracing
    Tracer trace.Tracer
}

// Initialize all instrumentation
func NewServiceInstrumentation(name string) *ServiceInstrumentation {
    return &ServiceInstrumentation{
        RequestCounter: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: fmt.Sprintf("%s_requests_total", name),
                Help: "Total requests processed",
            },
            []string{"method", "status"},
        ),
        RequestDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name: fmt.Sprintf("%s_request_duration_seconds", name),
                Help: "Request duration",
                Buckets: prometheus.DefBuckets,
            },
            []string{"method"},
        ),
        ActiveGauge: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Name: fmt.Sprintf("%s_active_requests", name),
                Help: "Active requests",
            },
        ),
        Logger: structured.NewLogger(name),
        Tracer: otel.Tracer(name),
    }
}
```

### 2. Context Propagation Pattern

```go
// Ensure context flows through all layers
func (s *Service) HandleRequest(ctx context.Context, req *Request) (*Response, error) {
    // Start span
    ctx, span := s.tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // Add to logs
    logger := s.logger.With(Field{"trace_id", span.SpanContext().TraceID()})
    
    // Track metrics
    timer := prometheus.NewTimer(s.requestDuration.WithLabelValues(req.Method))
    defer timer.ObserveDuration()
    
    s.activeGauge.Inc()
    defer s.activeGauge.Dec()
    
    // Business logic with context
    result, err := s.processRequest(ctx, req)
    
    // Record outcome
    status := "success"
    if err != nil {
        status = "error"
        span.RecordError(err)
        logger.Error("Request failed", Field{"error", err})
    }
    
    s.requestCounter.WithLabelValues(req.Method, status).Inc()
    
    return result, err
}
```

### 3. Dashboard Design Principles

```yaml
# Effective dashboard design
dashboards:
  - name: "Service Overview"
    rows:
      - title: "Golden Signals"
        panels:
          - metric: "rate(requests_total[5m])"
            title: "Request Rate"
          - metric: "histogram_quantile(0.99, rate(request_duration_bucket[5m]))"
            title: "P99 Latency"
          - metric: "rate(requests_total{status=~'5..'}[5m]) / rate(requests_total[5m])"
            title: "Error Rate"
          - metric: "sum(rate(requests_total[5m])) by (instance)"
            title: "Saturation"
            
      - title: "Business Metrics"
        panels:
          - metric: "sum(rate(tasks_completed[5m])) by (type)"
            title: "Task Throughput"
          - metric: "sum(cost_per_hour)"
            title: "Cost Rate"
            
  - name: "Troubleshooting"
    rows:
      - title: "Error Analysis"
        panels:
          - query: "topk(10, sum by (error_type) (rate(errors_total[5m])))"
            title: "Top Errors"
          - query: "sum by (service) (rate(errors_total[5m]))"
            title: "Errors by Service"
```

## Best Practices

1. **Start with SLIs/SLOs**: Define what matters before instrumenting
2. **Use structured data**: Consistent fields across all telemetry
3. **Sample intelligently**: Balance visibility with cost
4. **Correlate everything**: Use trace IDs to connect all data
5. **Automate analysis**: Build runbooks into alerts
6. **Test observability**: Include in integration tests
7. **Review regularly**: Dashboards and alerts need maintenance

## Next Steps

1. Review [Monitoring Guide](../operations/MONITORING.md) for implementation details
2. See [Trace-Based Debugging](./trace-based-debugging.md) for troubleshooting
3. Check [Performance Tuning Guide](./performance-tuning-guide.md) for optimization
4. Read [Cost Management Guide](./cost-management.md) for cost tracking

## Resources

- [OpenTelemetry Documentation](https://opentelemetry.io/docs/)
- [Prometheus Best Practices](https://prometheus.io/docs/practices/)
- [Grafana Dashboard Guide](https://grafana.com/docs/grafana/latest/dashboards/)
- [Distributed Tracing Explained](https://opentracing.io/docs/overview/)