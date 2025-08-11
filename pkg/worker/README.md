# Worker Module Documentation

## Overview

The Worker Module provides a standardized way to process Redis Stream events within the developer-mesh system. This package uses Redis Streams with consumer groups for reliable, distributed event processing.

## Key Components

### Redis Stream Event Processing

The `worker` package provides the following key components:

1. **ProcessStreamEvent**: Core function to process a Redis stream event.
   ```go
   func ProcessStreamEvent(ctx context.Context, event StreamEvent) error
   ```

2. **StreamConsumer**: Interface defining operations for consuming Redis stream messages.
   ```go
   type StreamConsumer interface {
       ReadMessages(ctx context.Context) ([]StreamMessage, error)
       AckMessage(ctx context.Context, messageID string) error
       NackMessage(ctx context.Context, messageID string) error
   }
   ```

3. **RedisIdempotency**: Interface for implementing idempotent event processing.
   ```go
   type RedisIdempotency interface {
       Exists(ctx context.Context, key string) (int64, error)
       Set(ctx context.Context, key string, value string, ttl time.Duration) error
   }
   ```

4. **RunWorker**: Function to start a worker process that continuously consumes stream events.
   ```go
   func RunWorker(ctx context.Context, consumer StreamConsumer, redisClient RedisIdempotency, processFunc func(StreamEvent) error) error
   ```

## Usage Examples

### Processing Events with RunWorker

```go
package main

import (
    "context"
    "log"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/worker"
    "github.com/developer-mesh/developer-mesh/pkg/redis"
    "github.com/developer-mesh/developer-mesh/pkg/queue"
)

func main() {
    // Create Redis client
    redisClient := redis.NewClient(&redis.Options{
        Addr: "localhost:6379",
    })
    
    // Create stream consumer
    consumer := queue.NewConsumer(redisClient, &queue.ConsumerConfig{
        Stream:        "webhook_events",
        Group:         "webhook_workers",
        Consumer:      "worker-1",
        BlockDuration: 5 * time.Second,
        BatchSize:     10,
    })
    
    // Define event processing function
    processFunc := func(event worker.StreamEvent) error {
        log.Printf("Processing event: %s", event.ID)
        
        // Your event processing logic here
        switch event.EventType {
        case "webhook.github.push":
            return processPushEvent(event)
        case "webhook.github.pull_request":
            return processPullRequestEvent(event)
        default:
            log.Printf("Unknown event type: %s", event.EventType)
            return nil
        }
    }
    
    // Start worker
    ctx := context.Background()
    if err := worker.RunWorker(ctx, consumer, redisClient, processFunc); err != nil {
        log.Fatal("Worker failed:", err)
    }
}
```

### Implementing Custom Event Processor

```go
type WebhookProcessor struct {
    logger      *log.Logger
    redisClient redis.Client
    repository  Repository
}

func (p *WebhookProcessor) ProcessEvent(event worker.StreamEvent) error {
    // Check idempotency
    idempotencyKey := fmt.Sprintf("processed:%s", event.ID)
    exists, err := p.redisClient.Exists(context.Background(), idempotencyKey)
    if err != nil {
        return fmt.Errorf("failed to check idempotency: %w", err)
    }
    if exists > 0 {
        p.logger.Printf("Event %s already processed, skipping", event.ID)
        return nil
    }
    
    // Process the event
    switch event.EventType {
    case "webhook.github.push":
        if err := p.processPushWebhook(event); err != nil {
            return err
        }
    case "webhook.github.issue":
        if err := p.processIssueWebhook(event); err != nil {
            return err
        }
    }
    
    // Mark as processed
    err = p.redisClient.Set(
        context.Background(),
        idempotencyKey,
        "processed",
        24*time.Hour,
    )
    if err != nil {
        return fmt.Errorf("failed to set idempotency key: %w", err)
    }
    
    return nil
}
```

## Configuration

The worker can be configured via environment variables or config file:

```yaml
worker:
  # Redis configuration
  redis:
    addr: "localhost:6379"
    password: ""
    db: 0
  
  # Stream configuration
  stream:
    name: "webhook_events"
    consumer_group: "webhook_workers"
    consumer_name: "worker-${HOSTNAME}"
    block_duration: "5s"
    batch_size: 10
    
  # Processing configuration
  processing:
    max_retries: 3
    retry_delay: "1s"
    idempotency_ttl: "24h"
    
  # Monitoring
  metrics:
    enabled: true
    port: 9090
```

## Error Handling

### Retry Strategy

The worker implements exponential backoff for failed events:

```go
type RetryStrategy struct {
    MaxAttempts int
    InitialDelay time.Duration
    MaxDelay time.Duration
    Multiplier float64
}

func DefaultRetryStrategy() *RetryStrategy {
    return &RetryStrategy{
        MaxAttempts:  3,
        InitialDelay: 1 * time.Second,
        MaxDelay:     30 * time.Second,
        Multiplier:   2.0,
    }
}
```

### Dead Letter Queue

Events that fail after max retries are moved to a DLQ:

```go
func (w *Worker) moveToDeadLetterQueue(event StreamEvent, err error) error {
    dlqEvent := DeadLetterEvent{
        OriginalEvent: event,
        Error:         err.Error(),
        FailedAt:      time.Now(),
        RetryCount:    event.RetryCount,
    }
    
    return w.streamsClient.AddToStream(
        context.Background(),
        "webhook_events_dlq",
        dlqEvent,
    )
}
```

## Monitoring

### Metrics

The worker exposes the following Prometheus metrics:

- `worker_events_processed_total` - Total events processed
- `worker_events_failed_total` - Total events failed
- `worker_events_retried_total` - Total events retried
- `worker_processing_duration_seconds` - Event processing duration
- `worker_consumer_lag` - Consumer group lag

### Health Check

```go
// Health check endpoint
http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    status := worker.GetHealth()
    if status.Healthy {
        w.WriteHeader(http.StatusOK)
    } else {
        w.WriteHeader(http.StatusServiceUnavailable)
    }
    json.NewEncoder(w).Encode(status)
})
```

## Best Practices

1. **Idempotency**: Always implement idempotent event processing
2. **Error Handling**: Distinguish between retryable and non-retryable errors
3. **Monitoring**: Set up alerts for consumer lag and DLQ growth
4. **Graceful Shutdown**: Handle SIGTERM/SIGINT for clean shutdown
5. **Resource Management**: Close connections properly in defer blocks

## Testing

```go
func TestWorkerProcessing(t *testing.T) {
    // Create mock consumer
    mockConsumer := &MockStreamConsumer{}
    mockRedis := &MockRedisClient{}
    
    // Set up test event
    testEvent := StreamEvent{
        ID:        "test-123",
        EventType: "webhook.github.push",
        Payload:   json.RawMessage(`{"ref":"refs/heads/main"}`),
    }
    
    mockConsumer.On("ReadMessages", mock.Anything).
        Return([]StreamMessage{{Event: testEvent}}, nil)
    mockConsumer.On("AckMessage", mock.Anything, "test-123").
        Return(nil)
    
    // Process event
    processFunc := func(event StreamEvent) error {
        assert.Equal(t, "test-123", event.ID)
        return nil
    }
    
    // Run worker for one iteration
    ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
    defer cancel()
    
    err := RunWorkerOnce(ctx, mockConsumer, mockRedis, processFunc)
    assert.NoError(t, err)
    
    mockConsumer.AssertExpectations(t)
}
```

## Migration from SQS

If migrating from SQS-based workers:

1. **Event Format**: Update event structure from SQS to Redis Streams format
2. **Consumer Groups**: Replace SQS visibility timeout with Redis consumer groups
3. **Acknowledgment**: Change from DeleteMessage to AckMessage
4. **Configuration**: Update environment variables and config files
5. **Monitoring**: Switch from CloudWatch to Prometheus metrics

### Migration Example

**Before (SQS):**
```go
func ProcessSQSEvent(event queue.SQSEvent) error {
    // Process SQS event
    return nil
}
```

**After (Redis Streams):**
```go
func ProcessStreamEvent(event worker.StreamEvent) error {
    // Process Redis stream event
    return nil
}
```

## Troubleshooting

### Common Issues

1. **Worker Not Processing Messages**
   - Check Redis connectivity
   - Verify consumer group exists
   - Check for pending messages: `redis-cli xpending webhook_events webhook_workers`

2. **High Error Rate**
   - Check application logs
   - Review DLQ for error patterns
   - Verify downstream service health

3. **Consumer Lag Growing**
   - Scale up worker instances
   - Increase batch size
   - Optimize processing logic

### Debug Commands

```bash
# Check worker health
curl http://localhost:9090/health

# View worker metrics
curl http://localhost:9090/metrics | grep worker_

# Check Redis stream status
redis-cli xinfo stream webhook_events

# View consumer group details
redis-cli xinfo consumers webhook_events webhook_workers
```