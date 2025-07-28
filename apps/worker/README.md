# Webhook Worker

A generic, provider-agnostic webhook event processor that supports dynamic tool configuration, retry logic, dead letter queues, and comprehensive observability.

## Overview

The webhook worker processes events from any webhook provider (GitHub, GitLab, Jira, etc.) based on dynamic tool configurations. It provides:

- **Dynamic Tool Support**: Process webhooks from any provider without code changes
- **Flexible Processing Modes**: Store only, store and forward, or transform and store
- **Robust Error Handling**: Exponential backoff retries with configurable limits
- **Dead Letter Queue**: Failed events are stored for manual inspection and retry
- **Comprehensive Observability**: Metrics, distributed tracing, and health checks

## Architecture

```
┌─────────────────┐     ┌──────────────┐     ┌─────────────────┐
│   REST API      │────▶│ Redis Queue  │────▶│  Worker Process │
└─────────────────┘     └──────────────┘     └─────────────────┘
                                                      │
                                                      ▼
                                              ┌───────────────┐
                                              │   Database    │
                                              │  (DLQ, Logs)  │
                                              └───────────────┘
```

## Features

### 1. Dynamic Tool Processing

The worker extracts tool configuration from event metadata and processes events according to the tool's webhook configuration:

```go
// Tool configuration determines:
- Authentication validation
- Processing mode (store, forward, transform)
- Event transformation rules
- Retry policies
```

### 2. Processing Modes

- **Store Only**: Store the event in the database (default)
- **Store and Forward**: Store the event and forward to another service
- **Transform and Store**: Apply transformation rules before storing

### 3. Error Handling

#### Retry Logic
- Exponential backoff with configurable parameters
- Smart error classification (retryable vs non-retryable)
- Maximum retry attempts before sending to DLQ

#### Dead Letter Queue
- Failed events stored with error details
- Automatic retry attempts for DLQ entries
- Manual retry API for specific events

### 4. Observability

#### Metrics
- Event processing rate and duration
- Queue depth monitoring
- Retry attempt tracking
- DLQ activity monitoring
- Memory and performance metrics

#### Health Checks
- Database connectivity
- Redis connectivity
- Queue health
- Worker process health

#### Distributed Tracing
- OpenTelemetry integration
- End-to-end request tracing
- Performance profiling

## Configuration

### Environment Variables

```bash
# Database Configuration
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=devops_mcp
DATABASE_USER=dbuser
DATABASE_PASSWORD=password
DATABASE_SSL_MODE=disable

# Redis Configuration
REDIS_ADDR=localhost:6379
REDIS_TLS_ENABLED=false

# Worker Configuration
WORKER_CONSUMER_NAME=worker-1
WORKER_IDEMPOTENCY_TTL=24h

# Health Check
HEALTH_ENDPOINT=:8088

# Logging
LOG_LEVEL=info
```

### Retry Configuration

```go
type RetryConfig struct {
    MaxRetries      int           // Default: 5
    InitialInterval time.Duration // Default: 1s
    MaxInterval     time.Duration // Default: 5m
    Multiplier      float64       // Default: 2.0
    MaxElapsedTime  time.Duration // Default: 30m
}
```

## API Reference

### Health Check Endpoint

```bash
GET /health
```

Response:
```json
{
  "status": "healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "components": {
    "database": {
      "status": "healthy",
      "details": {
        "open_connections": 5,
        "query_duration_ms": 2
      }
    },
    "redis": {
      "status": "healthy",
      "details": {
        "ping_duration_ms": 1
      }
    },
    "queue": {
      "status": "healthy",
      "details": {
        "queue_depth": 42
      }
    },
    "worker": {
      "status": "healthy",
      "details": {
        "runtime": {
          "goroutines": 25,
          "memory_alloc_mb": 64
        }
      }
    }
  }
}
```

## Metrics

### Prometheus Metrics

```
# Event processing
webhook_events_received_total{event_type, tool_id}
webhook_events_processed_total{event_type, tool_id, status}
webhook_event_processing_duration_seconds{event_type, tool_id, status}

# Retries and DLQ
webhook_retry_attempts_total{attempt, reason}
webhook_dlq_entries_total{event_type, reason}
webhook_dlq_retries_total{status}

# Queue monitoring
webhook_queue_depth
webhook_dlq_depth

# Performance
webhook_memory_allocated_bytes
webhook_goroutines_total
webhook_gc_runs_total
webhook_gc_pause_duration_seconds

# Health
webhook_health_checks_total{component, status}
webhook_health_check_duration_seconds{component}
```

## Development

### Building

```bash
# Build the worker
go build -o worker ./cmd/worker

# Run tests
go test ./...

# Run with race detector
go test -race ./...
```

### Running Locally

```bash
# Start dependencies
docker-compose up -d postgres redis

# Run database migrations
migrate -path migrations -database postgres://... up

# Start the worker
./worker
```

### Testing

The worker includes comprehensive unit tests for all components:

- Generic processor tests
- Retry handler tests
- DLQ handler tests
- Health check tests
- Performance monitor tests

## Deployment

### Docker

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o worker ./cmd/worker

FROM alpine:latest
RUN apk --no-cache add ca-certificates
COPY --from=builder /app/worker /worker
ENTRYPOINT ["/worker"]
```

### Kubernetes

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: webhook-worker
spec:
  replicas: 3
  template:
    spec:
      containers:
      - name: worker
        image: webhook-worker:latest
        env:
        - name: DATABASE_HOST
          value: postgres-service
        - name: REDIS_ADDR
          value: redis-service:6379
        livenessProbe:
          httpGet:
            path: /health/live
            port: 8088
        readinessProbe:
          httpGet:
            path: /health
            port: 8088
```

## Monitoring

### Grafana Dashboard

Import the provided dashboard JSON for:
- Event processing rates
- Error rates by event type
- Processing duration percentiles
- Queue depth trends
- Memory and CPU usage
- GC activity

### Alerts

Recommended alerts:
- High error rate (> 5%)
- Queue depth > 1000
- DLQ depth increasing
- Memory usage > 80%
- Database connection pool exhaustion

## Troubleshooting

### Common Issues

1. **High Memory Usage**
   - Check for memory leaks in event handlers
   - Review GC metrics
   - Increase GOMAXPROCS if CPU bound

2. **Events Stuck in DLQ**
   - Check error messages in DLQ entries
   - Verify tool configuration
   - Manual retry specific events

3. **Slow Processing**
   - Check database query performance
   - Review transformation rules complexity
   - Monitor Redis latency

### Debug Mode

Enable debug logging:
```bash
LOG_LEVEL=debug ./worker
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

See LICENSE file in the repository root.