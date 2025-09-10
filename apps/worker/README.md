# Worker - Redis-Based Event Processor

## Service Overview
The Worker is a robust, scalable event processor that handles:
- Webhook event processing from Redis streams
- Dynamic tool configuration extraction
- Retry logic with exponential backoff
- Dead Letter Queue (DLQ) management
- Health monitoring and metrics
- Generic event transformation

## Architecture
- **Queue**: Redis Streams with consumer groups
- **Processing**: Generic processor pattern
- **Storage**: PostgreSQL for DLQ and audit logs
- **Monitoring**: Prometheus metrics, health endpoints

## Key Components

### Core Worker (`internal/worker/`)
- `Worker`: Main processing loop
- `Processor`: Message handling logic
- `GenericProcessor`: Provider-agnostic processing
- `RetryHandler`: Exponential backoff implementation
- `DLQHandler`: Failed message handling
- `DLQWorker`: DLQ retry processor

### Supporting Components
- `HealthServer`: HTTP health endpoints
- `PerformanceMonitor`: Resource tracking
- `EventTransformer`: Event normalization
- `ToolConfigExtractor`: Dynamic config parsing

## Redis Streams Configuration
```bash
# Stream names
webhook_events          # Main event stream
webhook_events_dlq      # Dead letter queue

# Consumer group
webhook_workers         # Worker consumer group

# Keys
webhook:idempotency:*   # Deduplication keys
```

## Processing Flow
1. **Receive**: Read from Redis stream
2. **Extract**: Get tool config from metadata
3. **Validate**: Check idempotency
4. **Process**: Apply transformations
5. **Store**: Save to database
6. **Acknowledge**: Mark as processed

## Error Handling
```go
// Retry configuration
type RetryConfig struct {
    MaxRetries:      5
    InitialInterval: 1s
    MaxInterval:     5m
    Multiplier:      2.0
    MaxElapsedTime:  30m
}

// Error classification
- Retryable: Network, timeout, rate limit
- Non-retryable: Validation, auth, not found
```

## Health Endpoints
- `GET /health` - Overall health
- `GET /health/live` - Liveness probe
- `GET /health/ready` - Readiness probe

Health response includes:
- Database connectivity
- Redis connectivity
- Queue depth
- Worker runtime stats

## Configuration
```yaml
# Environment variables
DATABASE_HOST: localhost
DATABASE_PORT: 5432
REDIS_ADDR: localhost:6379
WORKER_CONSUMER_NAME: worker-1
WORKER_IDEMPOTENCY_TTL: 24h
HEALTH_ENDPOINT: :8088
LOG_LEVEL: info
```

## Metrics
```
# Event processing
webhook_events_received_total
webhook_events_processed_total
webhook_event_processing_duration_seconds

# Retries and DLQ
webhook_retry_attempts_total
webhook_dlq_entries_total
webhook_dlq_retries_total

# Performance
webhook_memory_allocated_bytes
webhook_goroutines_total
webhook_gc_runs_total
```

## Testing
```bash
# Run all tests
cd apps/worker && go test ./...

# Integration tests
go test -tags=integration ./...

# Race detection
go test -race ./...

# Specific component
go test ./internal/worker/...
```

## Common Issues

### High Memory Usage
- Check for goroutine leaks
- Review event payload sizes
- Monitor GC activity
- Adjust GOMAXPROCS

### Events Stuck in Queue
- Check consumer group status
- Verify Redis connectivity
- Review processing errors
- Check idempotency keys

### DLQ Growing
- Review error types
- Check tool configurations
- Verify external dependencies
- Manual retry if needed

## Debugging
```bash
# Monitor Redis stream
redis-cli xinfo stream webhook_events

# Check consumer group
redis-cli xinfo groups webhook_events

# View pending messages
redis-cli xpending webhook_events webhook_workers

# Check DLQ
redis-cli xlen webhook_events_dlq
```

## Development Workflow
1. Make changes in `internal/worker/`
2. Run unit tests
3. Test with local Redis
4. Verify metrics output
5. Check health endpoints

## Important Files
- `cmd/worker/main.go` - Entry point
- `internal/worker/worker.go` - Main loop
- `internal/worker/processor.go` - Core logic
- `internal/worker/generic_processor.go` - Event handling
- `internal/worker/retry_handler.go` - Retry logic
- `internal/worker/dlq_handler.go` - DLQ management

## Performance Tuning
- Batch size: 100 messages
- Processing timeout: 5 minutes
- Max concurrent: 10 workers
- Redis pool size: 20 connections
- DB pool size: 10 connections

## Security
- No credentials in logs
- Validate webhook signatures
- Sanitize event data
- Use prepared statements
- Encrypt sensitive fields

## Integration Points
- **REST API**: Produces events to Redis
- **Redis**: Message queue and cache
- **PostgreSQL**: DLQ and audit logs
- **Prometheus**: Metrics export

## Deployment
```yaml
# Kubernetes example
replicas: 3
resources:
  requests:
    memory: "256Mi"
    cpu: "250m"
  limits:
    memory: "512Mi"
    cpu: "1000m"
```

## Monitoring Alerts
- Error rate > 5%
- Queue depth > 1000
- DLQ depth increasing
- Memory > 80% limit
- Processing latency > 30s

## Best Practices
- Process events idempotently
- Log correlation IDs
- Use structured logging
- Monitor queue depth
- Handle graceful shutdown
- Test retry scenarios

## Never Do
- Don't log sensitive data
- Don't skip idempotency checks
- Don't ignore health checks
- Don't process synchronously
- Don't retry forever