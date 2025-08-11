<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:31:44
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Monitoring and Observability Guide

## Overview
This guide covers the monitoring capabilities of Developer Mesh. While the codebase has comprehensive metrics and tracing instrumentation, the full observability stack deployment is optional.

## Current Implementation Status

### ✅ Implemented
- Prometheus metrics collection in code
- OpenTelemetry tracing (disabled by default)
- Structured JSON logging
- Metrics endpoints at `/metrics`

### ⚠️ Optional/Manual Setup
- Prometheus server deployment
- Grafana dashboards
- Jaeger tracing backend

### ❌ Not Implemented
- docker-compose.monitoring.yml (referenced but doesn't exist)
- AlertManager integration
- Loki/Promtail log aggregation
- pprof profiling endpoints
- SLO/SLI tracking

## Quick Start (Manual Setup)

### 1. Enable Monitoring in Production
```bash
# Edit docker-compose.production.yml to uncomment monitoring services
# Look for the "Optional monitoring" section

# Start with monitoring enabled
docker-compose -f docker-compose.production.yml up -d prometheus grafana
```

### 2. Access Dashboards (if deployed)
- Grafana: http://localhost:3000 (admin/admin)
- Prometheus: http://localhost:9090
- Jaeger: Not configured (would need manual setup)

## Metrics Collection

### Available Metrics

#### HTTP Metrics
- `http_requests_total` - Total HTTP requests by method, path, status
- `http_request_duration_seconds` - Request latency histogram
- `http_requests_in_flight` - Current in-flight requests

#### Business Metrics
- `mcp_contexts_created_total` - Total contexts created
- `mcp_tool_executions_total` - Tool execution count by tool name
- `mcp_auth_attempts_total` - Authentication attempts by method
- `mcp_api_keys_active` - Number of active API keys

#### WebSocket Metrics <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_connections_active` - Current active WebSocket connections <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_connections_total` - Total WebSocket connections established <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_messages_sent_total` - Messages sent by type and compression <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_messages_received_total` - Messages received by type <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_message_size_bytes` - Message size distribution <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_compression_ratio` - Compression effectiveness <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_protocol_version` - Protocol version in use <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_errors_total` - WebSocket errors by type <!-- Source: pkg/models/websocket/binary.go -->
- `mcp_websocket_ping_latency_seconds` - Ping/pong latency <!-- Source: pkg/models/websocket/binary.go -->

#### Agent Metrics (Universal Agent System)
- `mcp_agents_registered` - Number of registered agents by type (ide, slack, monitoring, cicd, custom)
- `mcp_agent_workload_current` - Current workload per agent
- `mcp_agent_tasks_completed_total` - Tasks completed by agent
- `mcp_agent_task_duration_seconds` - Task processing time by agent
- `mcp_agent_capabilities` - Agent capabilities matrix
- `mcp_agent_manifests_total` - Total agent manifests by organization
- `mcp_agent_registrations_active` - Active agent registrations by type
- `mcp_agent_health_status` - Agent health (healthy, unhealthy, unknown)
- `mcp_cross_agent_messages_total` - Cross-agent messages by source/target type
- `mcp_agent_discovery_requests_total` - Agent discovery requests by capability

#### Tenant Isolation Metrics
- `mcp_tenant_agents_total` - Agents per tenant/organization
- `mcp_cross_org_attempts_blocked` - Blocked cross-organization attempts
- `mcp_tenant_isolation_mode` - Organizations with strict isolation enabled
- `mcp_tenant_rate_limit_hits` - Rate limit hits per tenant
- `mcp_tenant_circuit_breaker_state` - Circuit breaker state by tenant

#### Rate Limiting Metrics
- `mcp_rate_limit_agent_exceeded` - Agent rate limit exceeded events
- `mcp_rate_limit_tenant_exceeded` - Tenant rate limit exceeded events
- `mcp_rate_limit_capability_exceeded` - Capability rate limit exceeded events
- `mcp_rate_limit_current_rps` - Current requests per second by type
- `mcp_rate_limit_burst_used` - Burst capacity utilization

#### Circuit Breaker Metrics
- `mcp_circuit_breaker_state` - Circuit breaker state (open, closed, half-open)
- `mcp_circuit_breaker_failures` - Failure count per breaker
- `mcp_circuit_breaker_success` - Success count per breaker
- `mcp_circuit_breaker_trips` - Total circuit breaker trips
- `mcp_agent_marked_unhealthy` - Agents marked unhealthy by circuit breaker

#### System Metrics
- `go_goroutines` - Number of goroutines
- `go_memstats_alloc_bytes` - Memory allocation
- `process_cpu_seconds_total` - CPU usage

### Prometheus Configuration
```yaml
# prometheus/prometheus.yml
global:
  scrape_interval: 15s
  evaluation_interval: 15s

scrape_configs:
  - job_name: 'mcp-server'
    static_configs:
      - targets: ['mcp-server:8080']
    relabel_configs:
      - source_labels: [__address__]
        target_label: instance
        regex: '([^:]+)(?::\d+)?'
        replacement: '${1}'

  - job_name: 'rest-api'
    static_configs:
      - targets: ['rest-api:8080']

  - job_name: 'worker'
    static_configs:
      - targets: ['worker:8080']
```

### Actually Implemented Metrics

The following metrics are available at the `/metrics` endpoint:

```go
// From pkg/observability/prometheus_metrics.go
// These metrics are actually implemented and exposed:

// API Metrics
- api_request_duration_seconds (histogram)
- api_request_total (counter)
- api_request_size_bytes (histogram)
- api_response_size_bytes (histogram)
- api_concurrent_requests (gauge)

// Database Metrics  
- db_query_duration_seconds (histogram)
- db_query_total (counter)
- db_connection_pool_size (gauge)
- db_connection_pool_used (gauge)

// Cache Metrics
- cache_hits_total (counter)
- cache_misses_total (counter)
- cache_evictions_total (counter)
- cache_size_bytes (gauge)

// WebSocket Metrics <!-- Source: pkg/models/websocket/binary.go -->
- websocket_connections_total (counter) <!-- Source: pkg/models/websocket/binary.go -->
- websocket_active_connections (gauge) <!-- Source: pkg/models/websocket/binary.go -->
- websocket_messages_received_total (counter) <!-- Source: pkg/models/websocket/binary.go -->
- websocket_messages_sent_total (counter) <!-- Source: pkg/models/websocket/binary.go -->
- websocket_errors_total (counter) <!-- Source: pkg/models/websocket/binary.go -->

// Health Check Metrics
- health_check_duration_seconds (histogram)
- health_check_total (counter)
```

### WebSocket Metrics Implementation <!-- Source: pkg/models/websocket/binary.go -->
```go
// WebSocket-specific metrics <!-- Source: pkg/models/websocket/binary.go -->
package metrics

import (
    "github.com/prometheus/client_golang/prometheus"
    "github.com/prometheus/client_golang/prometheus/promauto"
)

var (
    // Connection metrics
    wsConnectionsActive = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_websocket_connections_active", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Number of active WebSocket connections", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"protocol_version", "agent_type"},
    )
    
    wsConnectionsTotal = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_connections_total", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Total WebSocket connections established", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"protocol_version", "status"},
    )
    
    // Message metrics
    wsMessagesSent = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_messages_sent_total", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Total messages sent via WebSocket", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"message_type", "compressed", "protocol_version"},
    )
    
    wsMessagesReceived = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_websocket_messages_received_total", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Total messages received via WebSocket", <!-- Source: pkg/models/websocket/binary.go -->
        },
        []string{"message_type", "protocol_version"},
    )
    
    wsMessageSize = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_message_size_bytes", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Size of WebSocket messages in bytes", <!-- Source: pkg/models/websocket/binary.go -->
            Buckets: prometheus.ExponentialBuckets(128, 2, 12), // 128B to 256KB
        },
        []string{"direction", "message_type", "compressed"},
    )
    
    wsCompressionRatio = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_compression_ratio", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "Compression ratio for WebSocket messages", <!-- Source: pkg/models/websocket/binary.go -->
            Buckets: prometheus.LinearBuckets(0, 0.1, 11), // 0% to 100%
        },
    )
    
    wsPingLatency = promauto.NewHistogram(
        prometheus.HistogramOpts{
            Name: "mcp_websocket_ping_latency_seconds", <!-- Source: pkg/models/websocket/binary.go -->
            Help: "WebSocket ping/pong latency", <!-- Source: pkg/models/websocket/binary.go -->
            Buckets: prometheus.ExponentialBuckets(0.001, 2, 10), // 1ms to 1s
        },
    )
)

// Track WebSocket connection <!-- Source: pkg/models/websocket/binary.go -->
func TrackWSConnection(protocolVersion, agentType string, connected bool) {
    if connected {
        wsConnectionsActive.WithLabelValues(protocolVersion, agentType).Inc()
        wsConnectionsTotal.WithLabelValues(protocolVersion, "established").Inc()
    } else {
        wsConnectionsActive.WithLabelValues(protocolVersion, agentType).Dec()
        wsConnectionsTotal.WithLabelValues(protocolVersion, "closed").Inc()
    }
}

// Track WebSocket message <!-- Source: pkg/models/websocket/binary.go -->
func TrackWSMessage(direction string, msg *WSMessage) {
    compressed := "false"
    if msg.Flags&FlagCompressed != 0 {
        compressed = "true"
    }
    
    labels := []string{msg.Type.String(), compressed, msg.Version.String()}
    
    if direction == "sent" {
        wsMessagesSent.WithLabelValues(labels...).Inc()
    } else {
        wsMessagesReceived.WithLabelValues(msg.Type.String(), msg.Version.String()).Inc()
    }
    
    wsMessageSize.WithLabelValues(direction, msg.Type.String(), compressed).
        Observe(float64(len(msg.Payload)))
    
    // Track compression ratio if compressed
    if compressed == "true" && msg.OriginalSize > 0 {
        ratio := float64(len(msg.Payload)) / float64(msg.OriginalSize)
        wsCompressionRatio.Observe(1 - ratio) // Savings percentage
    }
}

// Track ping latency
func TrackPingLatency(latency time.Duration) {
    wsPingLatency.Observe(latency.Seconds())
}
```

### Agent Metrics Implementation
```go
// Agent-specific metrics
var (
    agentsRegistered = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agents_registered",
            Help: "Number of registered agents",
        },
        []string{"agent_type", "model", "status"},
    )
    
    agentWorkload = promauto.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "mcp_agent_workload_current",
            Help: "Current workload percentage per agent",
        },
        []string{"agent_id", "agent_type"},
    )
    
    agentTasksCompleted = promauto.NewCounterVec(
        prometheus.CounterOpts{
            Name: "mcp_agent_tasks_completed_total",
            Help: "Total tasks completed by agent",
        },
        []string{"agent_id", "agent_type", "task_type", "status"},
    )
    
    agentTaskDuration = promauto.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "mcp_agent_task_duration_seconds",
            Help: "Task processing duration by agent",
            Buckets: prometheus.ExponentialBuckets(0.1, 2, 10), // 100ms to 100s
        },
        []string{"agent_type", "task_type"},
    )
)

// Track agent registration
func TrackAgentRegistration(agent *Agent) {
    agentsRegistered.WithLabelValues(
        agent.Type, 
        agent.Model, 
        agent.Status.String(),
    ).Inc()
}

// Update agent workload
func UpdateAgentWorkload(agentID, agentType string, workload float64) {
    agentWorkload.WithLabelValues(agentID, agentType).Set(workload)
}

// Track task completion
func TrackTaskCompletion(agent *Agent, task *Task, duration time.Duration, status string) {
    agentTasksCompleted.WithLabelValues(
        agent.ID,
        agent.Type,
        task.Type,
        status,
    ).Inc()
    
    agentTaskDuration.WithLabelValues(
        agent.Type,
        task.Type,
    ).Observe(duration.Seconds())
}
```

## Logging Configuration (Implemented)

### Structured Logging
Developer Mesh uses structured JSON logging. Configure via environment variables:

```bash
# Set log level
LOG_LEVEL=debug  # debug, info, warn, error

# Logs are written to stdout in JSON format
# Example log output:
{"level":"info","ts":"2024-01-23T10:00:00Z","caller":"main.go:42","msg":"Server started","port":8080}
```

### Log Aggregation (Not Implemented)

**Note**: Loki and Promtail are mentioned in docs but not implemented. For log aggregation, you would need to:
1. Deploy Loki/Promtail separately
2. Configure them to scrape container logs
3. This is not included in Developer Mesh

```yaml
# THEORETICAL: loki/config.yaml does not exist
auth_enabled: false

server:
  http_listen_port: 3100

ingester:
  lifecycler:
    address: 127.0.0.1
    ring:
      kvstore:
        store: inmemory
      replication_factor: 1

schema_config:
  configs:
    - from: 2020-10-24
      store: boltdb-shipper
      object_store: filesystem
      schema: v11
      index:
        prefix: index_
        period: 24h
```

### Promtail Configuration
```yaml
# promtail/config.yaml
clients:
  - url: http://loki:3100/loki/api/v1/push

positions:
  filename: /tmp/positions.yaml

scrape_configs:
  - job_name: mcp-services
    static_configs:
      - targets:
          - localhost
        labels:
          job: mcp-services
          __path__: /var/log/mcp/*.log
    pipeline_stages:
      - json:
          expressions:
            level: level
            service: service
            trace_id: trace_id
      - labels:
          level:
          service:
```

## Distributed Tracing (Optional)

### OpenTelemetry Integration (Implemented but Disabled)

**Note**: Tracing code exists but is disabled by default. To enable:

1. Set environment variables:
```bash
ENABLE_TRACING=true
OTEL_EXPORTER_OTLP_ENDPOINT=http://your-jaeger:4317
```

2. Deploy Jaeger (not included):
```bash
# Example Jaeger deployment (not part of Developer Mesh)
docker run -d --name jaeger \
  -p 16686:16686 \
  -p 4317:4317 \
  jaegertracing/all-in-one:latest
```

### Actual Implementation
```go
// From pkg/observability/tracer.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/trace"
)

func InitTracing() (*trace.TracerProvider, error) {
    exporter, err := jaeger.New(
        jaeger.WithCollectorEndpoint(
            jaeger.WithEndpoint("http://jaeger:14268/api/traces"),
        ),
    )
    if err != nil {
        return nil, err
    }

    tp := trace.NewTracerProvider(
        trace.WithBatcher(exporter),
        trace.WithResource(resource.NewWithAttributes(
            semconv.SchemaURL,
            semconv.ServiceNameKey.String("mcp-server"),
            semconv.ServiceVersionKey.String("1.0.0"),
        )),
    )
    
    otel.SetTracerProvider(tp)
    return tp, nil
}

// Use in handlers
func HandleRequest(ctx context.Context) {
    tracer := otel.Tracer("mcp-server")
    ctx, span := tracer.Start(ctx, "HandleRequest")
    defer span.End()
    
    // Add attributes
    span.SetAttributes(
        attribute.String("tenant.id", tenantID),
        attribute.Int("request.size", len(body)),
    )
}

// WebSocket tracing with custom attributes <!-- Source: pkg/models/websocket/binary.go -->
func HandleWebSocketMessage(ctx context.Context, msg *Message) { <!-- Source: pkg/models/websocket/binary.go -->
    tracer := otel.Tracer("mcp-websocket") <!-- Source: pkg/models/websocket/binary.go -->
    ctx, span := tracer.Start(ctx, "HandleWebSocketMessage", <!-- Source: pkg/models/websocket/binary.go -->
        trace.WithSpanKind(trace.SpanKindServer))
    defer span.End()
    
    // Custom WebSocket attributes <!-- Source: pkg/models/websocket/binary.go -->
    span.SetAttributes(
        attribute.String("ws.connection_id", msg.ConnectionID),
        attribute.String("ws.message_type", msg.Type.String()),
        attribute.Int("ws.sequence_id", int(msg.SequenceID)),
        attribute.Int("ws.protocol_version", int(msg.Version)),
        attribute.Bool("ws.compressed", msg.IsCompressed()),
        attribute.Int("ws.message_size", len(msg.Payload)),
        attribute.String("ws.encoding", msg.Encoding),
    )
    
    // Agent-specific attributes
    if msg.AgentID != "" {
        span.SetAttributes(
            attribute.String("agent.id", msg.AgentID),
            attribute.String("agent.type", msg.AgentType),
            attribute.StringSlice("agent.capabilities", msg.Capabilities),
            attribute.Float64("agent.workload", msg.CurrentWorkload),
        )
    }
    
    // Task routing attributes <!-- Source: pkg/services/assignment_engine.go -->
    if msg.TaskID != "" {
        span.SetAttributes(
            attribute.String("task.id", msg.TaskID),
            attribute.String("task.type", msg.TaskType),
            attribute.String("task.routing_strategy", msg.RoutingStrategy), <!-- Source: pkg/services/assignment_engine.go -->
            attribute.Float64("task.priority", msg.Priority),
            attribute.Float64("task.estimated_cost", msg.EstimatedCost),
        )
    }
}

// Custom span attributes for different operations
var CustomSpanAttributes = struct {
    // WebSocket attributes <!-- Source: pkg/models/websocket/binary.go -->
    WebSocket struct { <!-- Source: pkg/models/websocket/binary.go -->
        ConnectionID    attribute.Key
        MessageType     attribute.Key
        ProtocolVersion attribute.Key
        Compressed      attribute.Key
        MessageSize     attribute.Key
        PingLatency     attribute.Key
    }
    
    // Agent attributes
    Agent struct {
        ID           attribute.Key
        Type         attribute.Key
        Model        attribute.Key
        Capabilities attribute.Key
        Workload     attribute.Key
        Status       attribute.Key
    }
    
    // Task attributes
    Task struct {
        ID               attribute.Key
        Type             attribute.Key
        RoutingStrategy  attribute.Key
        Priority         attribute.Key
        EstimatedCost    attribute.Key
        ActualCost       attribute.Key
        ProcessingTime   attribute.Key
    }
    
    // Cost tracking attributes
    Cost struct {
        Model         attribute.Key
        InputTokens   attribute.Key
        OutputTokens  attribute.Key
        TotalCost     attribute.Key
        SessionLimit  attribute.Key
        RemainingBudget attribute.Key
    }
}{
    WebSocket: struct { <!-- Source: pkg/models/websocket/binary.go -->
        ConnectionID    attribute.Key
        MessageType     attribute.Key
        ProtocolVersion attribute.Key
        Compressed      attribute.Key
        MessageSize     attribute.Key
        PingLatency     attribute.Key
    }{
        ConnectionID:    attribute.Key("ws.connection_id"),
        MessageType:     attribute.Key("ws.message_type"),
        ProtocolVersion: attribute.Key("ws.protocol_version"),
        Compressed:      attribute.Key("ws.compressed"),
        MessageSize:     attribute.Key("ws.message_size"),
        PingLatency:     attribute.Key("ws.ping_latency_ms"),
    },
    Agent: struct {
        ID           attribute.Key
        Type         attribute.Key
        Model        attribute.Key
        Capabilities attribute.Key
        Workload     attribute.Key
        Status       attribute.Key
    }{
        ID:           attribute.Key("agent.id"),
        Type:         attribute.Key("agent.type"),
        Model:        attribute.Key("agent.model"),
        Capabilities: attribute.Key("agent.capabilities"),
        Workload:     attribute.Key("agent.workload"),
        Status:       attribute.Key("agent.status"),
    },
    Task: struct {
        ID               attribute.Key
        Type             attribute.Key
        RoutingStrategy  attribute.Key
        Priority         attribute.Key
        EstimatedCost    attribute.Key
        ActualCost       attribute.Key
        ProcessingTime   attribute.Key
    }{
        ID:              attribute.Key("task.id"),
        Type:            attribute.Key("task.type"),
        RoutingStrategy: attribute.Key("task.routing_strategy"), <!-- Source: pkg/services/assignment_engine.go -->
        Priority:        attribute.Key("task.priority"),
        EstimatedCost:   attribute.Key("task.estimated_cost_usd"),
        ActualCost:      attribute.Key("task.actual_cost_usd"),
        ProcessingTime:  attribute.Key("task.processing_time_ms"),
    },
    Cost: struct {
        Model         attribute.Key
        InputTokens   attribute.Key
        OutputTokens  attribute.Key
        TotalCost     attribute.Key
        SessionLimit  attribute.Key
        RemainingBudget attribute.Key
    }{
        Model:           attribute.Key("cost.model"),
        InputTokens:     attribute.Key("cost.input_tokens"),
        OutputTokens:     attribute.Key("cost.output_tokens"),
        TotalCost:       attribute.Key("cost.total_usd"),
        SessionLimit:    attribute.Key("cost.session_limit_usd"),
        RemainingBudget: attribute.Key("cost.remaining_budget_usd"),
    },
}
```

## Grafana Dashboards (Configuration Only)

**Note**: Dashboard JSON files exist in `configs/grafana/dashboards/` but Grafana deployment is optional. To use:

1. Uncomment Grafana in docker-compose.production.yml
2. Start Grafana container
3. Import dashboards manually or configure auto-provisioning

### Available Dashboard Configuration
```json
// From configs/grafana/dashboards/mcp-dashboard.json
{
  "dashboard": {
    "title": "MCP Metrics Dashboard",
    "panels": [
      {
        "title": "Request Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(rate(http_requests_total[5m])) by (service)"
        }]
      },
      {
        "title": "Error Rate",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "sum(rate(http_requests_total{status=~\"5..\"}[5m])) by (service) / sum(rate(http_requests_total[5m])) by (service)"
        }]
      },
      {
        "title": "P95 Latency",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le))"
        }]
      },
      {
        "title": "Active Contexts",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "sum(mcp_contexts_active) by (tenant_id)"
        }]
      }
    ]
  }
}
```

### WebSocket Dashboard <!-- Source: pkg/models/websocket/binary.go -->
```json
{
  "dashboard": {
    "title": "WebSocket Monitoring", <!-- Source: pkg/models/websocket/binary.go -->
    "panels": [
      {
        "title": "Active WebSocket Connections", <!-- Source: pkg/models/websocket/binary.go -->
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(mcp_websocket_connections_active) by (protocol_version, agent_type)" <!-- Source: pkg/models/websocket/binary.go -->
        }]
      },
      {
        "title": "WebSocket Message Rate", <!-- Source: pkg/models/websocket/binary.go -->
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "sum(rate(mcp_websocket_messages_sent_total[5m])) by (message_type)", <!-- Source: pkg/models/websocket/binary.go -->
          "legendFormat": "Sent - {{message_type}}"
        }, {
          "expr": "sum(rate(mcp_websocket_messages_received_total[5m])) by (message_type)", <!-- Source: pkg/models/websocket/binary.go -->
          "legendFormat": "Received - {{message_type}}"
        }]
      },
      {
        "title": "Message Size Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(mcp_websocket_message_size_bytes_bucket[5m])) by (message_type, le))" <!-- Source: pkg/models/websocket/binary.go -->
        }]
      },
      {
        "title": "Compression Effectiveness",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.5, sum(rate(mcp_websocket_compression_ratio_bucket[5m])) by (le)) * 100", <!-- Source: pkg/models/websocket/binary.go -->
          "legendFormat": "Median Compression %"
        }]
      },
      {
        "title": "WebSocket Ping Latency", <!-- Source: pkg/models/websocket/binary.go -->
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 16},
        "targets": [{
          "expr": "histogram_quantile(0.99, sum(rate(mcp_websocket_ping_latency_seconds_bucket[5m])) by (le)) * 1000", <!-- Source: pkg/models/websocket/binary.go -->
          "legendFormat": "P99 Latency (ms)"
        }]
      },
      {
        "title": "Protocol Version Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 16},
        "targets": [{
          "expr": "sum(mcp_websocket_connections_active) by (protocol_version)" <!-- Source: pkg/models/websocket/binary.go -->
        }],
        "type": "piechart"
      }
    ]
  }
}
```

### Agent Performance Dashboard
```json
{
  "dashboard": {
    "title": "Agent Performance",
    "panels": [
      {
        "title": "Registered Agents",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 0},
        "targets": [{
          "expr": "sum(mcp_agents_registered) by (agent_type, model)"
        }]
      },
      {
        "title": "Agent Workload Distribution",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 0},
        "targets": [{
          "expr": "mcp_agent_workload_current",
          "legendFormat": "{{agent_id}} ({{agent_type}})"
        }]
      },
      {
        "title": "Task Completion Rate",
        "gridPos": {"h": 8, "w": 12, "x": 0, "y": 8},
        "targets": [{
          "expr": "sum(rate(mcp_agent_tasks_completed_total[5m])) by (agent_type, status)"
        }]
      },
      {
        "title": "Task Processing Time (P95)",
        "gridPos": {"h": 8, "w": 12, "x": 12, "y": 8},
        "targets": [{
          "expr": "histogram_quantile(0.95, sum(rate(mcp_agent_task_duration_seconds_bucket[5m])) by (agent_type, task_type, le))"
        }]
      }
    ]
  }
}
```

### Import Dashboard
```bash
# Import via API
curl -X POST http://admin:admin@localhost:3000/api/dashboards/db \
  -H "Content-Type: application/json" \
  -d @dashboards/mcp-overview.json
```

## Alerting (Not Implemented)

**Note**: No AlertManager integration exists. The following shows how you could set up alerting:

### Theoretical Prometheus Alert Rules
```yaml
# THEORETICAL: No alerts/rules.yml exists
# You would need to create this and configure AlertManager separately
groups:
  - name: mcp_critical
    interval: 30s
    rules:
      - alert: HighErrorRate
        expr: |
          sum(rate(http_requests_total{status=~"5.."}[5m])) by (service) 
          / sum(rate(http_requests_total[5m])) by (service) > 0.05
        for: 5m
        labels:
          severity: critical
          team: platform
        annotations:
          summary: "High error rate on {{ $labels.service }}"
          description: "{{ $labels.service }} has {{ $value | humanizePercentage }} error rate"

      - alert: HighLatency
        expr: |
          histogram_quantile(0.99, sum(rate(http_request_duration_seconds_bucket[5m])) by (service, le)) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High latency on {{ $labels.service }}"
          description: "P99 latency is {{ $value }}s"

      - alert: MemoryLeak
        expr: |
          rate(go_memstats_alloc_bytes[5m]) > 0 and 
          predict_linear(go_memstats_alloc_bytes[1h], 4*3600) > 8e9
        for: 30m
        labels:
          severity: warning
        annotations:
          summary: "Potential memory leak in {{ $labels.service }}"

      - alert: GoroutineLeak
        expr: |
          rate(go_goroutines[5m]) > 0 and
          predict_linear(go_goroutines[1h], 4*3600) > 10000
        for: 30m
        labels:
          severity: warning
```

### AlertManager Configuration
```yaml
# alertmanager/config.yml
global:
  resolve_timeout: 5m
  slack_api_url: ${SLACK_WEBHOOK_URL}

route:
  group_by: ['alertname', 'service']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'default'
  routes:
    - match:
        severity: critical
      receiver: pagerduty
      continue: true
    - match:
        severity: warning
      receiver: slack

receivers:
  - name: 'default'
    # No-op

  - name: 'slack'
    slack_configs:
      - channel: '#mcp-alerts'
        title: 'MCP Alert'
        text: '{{ range .Alerts }}{{ .Annotations.summary }}\n{{ end }}'

  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: ${PAGERDUTY_SERVICE_KEY}
```

## SLI/SLO Monitoring (Not Implemented)

**Note**: No SLI/SLO tracking is implemented. This section shows what could be built:

### Theoretical Service Level Indicators
```yaml
# THEORETICAL: No SLI/SLO implementation exists
slis:
  availability:
    description: "Percentage of successful requests"
    query: |
      sum(rate(http_requests_total{status!~"5.."}[5m])) 
      / sum(rate(http_requests_total[5m]))
    
  latency:
    description: "95th percentile latency"
    query: |
      histogram_quantile(0.95, 
        sum(rate(http_request_duration_seconds_bucket[5m])) by (le)
      )
  
  error_rate:
    description: "Percentage of failed requests"
    query: |
      sum(rate(http_requests_total{status=~"5.."}[5m])) 
      / sum(rate(http_requests_total[5m]))
```

### Service Level Objectives
```yaml
# SLO targets
slos:
  - name: "API Availability"
    sli: availability
    target: 99.9
    window: 30d
    
  - name: "API Latency"
    sli: latency
    target_value: 0.5  # 500ms
    window: 7d
    
  - name: "Error Budget"
    sli: error_rate
    target: 0.1  # 0.1% error rate
    window: 30d
```

### SLO Dashboard
```json
{
  "panels": [
    {
      "title": "SLO Status",
      "targets": [{
        "expr": "1 - (sum(rate(http_requests_total{status=~\"5..\"}[30d])) / sum(rate(http_requests_total[30d])))"
      }],
      "thresholds": [
        {"value": 0.999, "color": "green"},
        {"value": 0.995, "color": "yellow"},
        {"value": 0, "color": "red"}
      ]
    }
  ]
}
```

## Performance Monitoring

### Database Queries
```sql
-- Slow query monitoring
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;

-- View slow queries
SELECT 
  query,
  calls,
  mean_exec_time,
  max_exec_time,
  total_exec_time
FROM pg_stat_statements
WHERE mean_exec_time > 100  -- queries slower than 100ms
ORDER BY mean_exec_time DESC
LIMIT 20;
```

### Redis Monitoring
```bash
# Redis metrics to monitor
redis-cli INFO stats | grep -E "instantaneous_ops_per_sec|used_memory_human|connected_clients|blocked_clients"

# Slow log
redis-cli SLOWLOG GET 10
```

## Debugging Production Issues

### Enable Debug Logging
```bash
# Set environment variable and restart service
LOG_LEVEL=debug docker-compose -f docker-compose.production.yml up -d

# Note: No runtime log level API exists
```

### Trace Specific Request
```bash
# Add trace header
curl -H "X-Debug-Trace: true" \
     -H "X-Request-ID: debug-123" \
     http://localhost:8080/api/v1/contexts

# View trace in Jaeger
open http://localhost:16686/trace/debug-123
```

### Memory Profiling (Not Available)

**Note**: pprof endpoints are not exposed. To enable profiling:

1. Modify the application to add pprof handlers:
```go
import _ "net/http/pprof"
// Add to your router:
// router.PathPrefix("/debug/").Handler(http.DefaultServeMux)
```

2. Rebuild and redeploy
3. Access pprof endpoints

Currently, no profiling endpoints are available in production.

## Best Practices

1. **Use the built-in metrics** - Comprehensive metrics are already implemented
2. **Enable JSON logging** - Already configured for structured logs
3. **Monitor the /metrics endpoint** - Available on both services
4. **Set up external monitoring** - Deploy Prometheus/Grafana separately if needed
5. **Use correlation IDs** - Pass X-Request-ID headers for request tracking
6. **Check health endpoints** - Use /health for basic monitoring
7. **Consider enabling tracing** - Code exists but needs configuration

## What's Actually Available

### Without Additional Setup
- Metrics at http://localhost:8080 (MCP Server)/metrics
- Health checks at http://localhost:8080 (MCP Server)/health
- JSON structured logs to stdout
- Correlation ID tracking via X-Request-ID

### With Manual Setup
- Prometheus scraping (uncomment in docker-compose)
- Grafana dashboards (import from configs/)
- OpenTelemetry tracing (set environment variables)

### Not Available
- pprof profiling endpoints
- Log aggregation (Loki/Promtail)
- Alerting (AlertManager)
- SLO tracking
- Jaeger UI (unless manually deployed)

## Troubleshooting

### High Memory Usage
1. Check for goroutine leaks: `/debug/pprof/goroutine`
2. Analyze heap profile: `/debug/pprof/heap`
3. Review recent deployments
4. Check for increased traffic

### High Latency
1. Check database slow queries
2. Review Redis latency
3. Check external API calls
4. Analyze trace data

### Missing Metrics
1. Ensure service is running: `docker-compose ps`
2. Check metrics endpoint directly: `curl http://localhost:8080 (MCP Server)/metrics`
3. If using Prometheus, verify scrape config matches service names
4. Check logs for metric registration errors

## Summary

