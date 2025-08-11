# Queue Package

> **Purpose**: Redis Streams integration for event processing in the Developer Mesh platform
> **Status**: Production Ready
> **Dependencies**: Redis 7+, Consumer Groups, JSON serialization

## Overview

The queue package provides Redis Streams integration for processing webhook events and other asynchronous tasks. It uses Redis consumer groups for distributed processing with automatic retries and dead letter queue support.

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Queue Architecture                      │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Event Producers ──► Redis Client ──► Redis Streams ──► Workers │
│                          │                                   │
│                          ├── Stream Operations              │
│                          ├── Consumer Groups                │
│                          └── Dead Letter Queue              │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

## Core Components

### 1. StreamsClient Interface

```go
// StreamsClient interface represents Redis Streams operations
type StreamsClient interface {
    // AddToStream adds a message to a Redis stream
    AddToStream(ctx context.Context, stream string, event interface{}) (string, error)
    
    // ReadFromStream reads messages from a stream using consumer groups
    ReadFromStream(ctx context.Context, group, consumer string, count int64) ([]StreamMessage, error)
    
    // AckMessage acknowledges a processed message
    AckMessage(ctx context.Context, stream, group string, messageID string) error
    
    // GetStreamInfo returns stream statistics
    GetStreamInfo(ctx context.Context, stream string) (*StreamInfo, error)
}
```

### 2. Event Definition

```go
// StreamEvent represents an event in the Redis stream with auth context
type StreamEvent struct {
    ID          string          `json:"id"`
    EventType   string          `json:"event_type"`
    Repository  string          `json:"repository"`
    TenantID    string          `json:"tenant_id"`
    Payload     json.RawMessage `json:"payload"`
    AuthContext AuthContext     `json:"auth_context,omitempty"`
    Timestamp   time.Time       `json:"timestamp"`
    RetryCount  int             `json:"retry_count"`
}
```

### 3. Consumer Group Configuration

```go
type ConsumerConfig struct {
    Stream         string        // Stream name
    Group          string        // Consumer group name
    Consumer       string        // Consumer identifier
    BlockDuration  time.Duration // Block timeout for XREADGROUP
    BatchSize      int           // Number of messages to read
    MaxRetries     int           // Maximum retry attempts
    RetryDelay     time.Duration // Delay between retries
}
```

## Usage Examples

### Producer

```go
import (
    "github.com/developer-mesh/developer-mesh/pkg/queue"
    "github.com/developer-mesh/developer-mesh/pkg/redis"
)

// Create Redis client
redisClient := redis.NewClient(&redis.Options{
    Addr: "localhost:6379",
})

// Create streams client
streamsClient := queue.NewRedisStreamsClient(redisClient)

// Produce an event
event := &StreamEvent{
    EventType:  "webhook.github.push",
    Repository: "owner/repo",
    TenantID:   "tenant-123",
    Payload:    rawPayload,
    Timestamp:  time.Now(),
}

messageID, err := streamsClient.AddToStream(ctx, "webhook_events", event)
if err != nil {
    log.Fatal("Failed to add event:", err)
}
```

### Consumer

```go
// Configure consumer
config := &ConsumerConfig{
    Stream:        "webhook_events",
    Group:         "webhook_workers",
    Consumer:      "worker-1",
    BlockDuration: 5 * time.Second,
    BatchSize:     10,
    MaxRetries:    3,
}

// Create consumer
consumer := queue.NewConsumer(streamsClient, config)

// Process messages
for {
    messages, err := consumer.ReadMessages(ctx)
    if err != nil {
        log.Error("Failed to read messages:", err)
        continue
    }
    
    for _, msg := range messages {
        // Process message
        err := processMessage(msg)
        if err != nil {
            // Message will be retried
            consumer.NackMessage(ctx, msg.ID)
        } else {
            // Acknowledge successful processing
            consumer.AckMessage(ctx, msg.ID)
        }
    }
}
```

## Configuration

Configure via environment variables or config file:

```yaml
redis:
  addr: "localhost:6379"
  password: ""
  db: 0
  
streams:
  webhook_events:
    max_length: 10000
    consumer_group: "webhook_workers"
    block_duration: "5s"
    batch_size: 10
    max_retries: 3
    
  dlq:
    stream_name: "webhook_events_dlq"
    max_length: 1000
    retention: "168h"  # 7 days
```

## Monitoring

### Stream Info
```bash
# Check stream length
redis-cli xlen webhook_events

# Check consumer group info
redis-cli xinfo groups webhook_events

# Check consumer lag
redis-cli xpending webhook_events webhook_workers
```

### Metrics

The package exposes the following Prometheus metrics:

- `queue_messages_produced_total` - Total messages produced
- `queue_messages_consumed_total` - Total messages consumed
- `queue_messages_failed_total` - Total failed messages
- `queue_consumer_lag` - Consumer group lag
- `queue_processing_duration_seconds` - Message processing duration

## Error Handling

### Retry Logic

Failed messages are automatically retried with exponential backoff:
1. First retry: immediate
2. Second retry: after 1 second
3. Third retry: after 2 seconds

After max retries, messages are moved to the dead letter queue.

### Dead Letter Queue

Messages that fail processing after max retries are moved to a DLQ stream:

```go
// Check DLQ
dlqMessages, err := streamsClient.ReadFromStream(
    ctx, 
    "webhook_events_dlq",
    "$",  // Read all messages
    10,   // Limit
)
```

## Best Practices

1. **Consumer Groups**: Always use consumer groups for distributed processing
2. **Acknowledgment**: Explicitly acknowledge processed messages
3. **Idempotency**: Ensure message processing is idempotent
4. **Monitoring**: Monitor consumer lag and DLQ size
5. **Trimming**: Set max_length to prevent unbounded growth
6. **Error Handling**: Implement proper retry and DLQ strategies

## Testing

```go
// Use mock streams client for testing
mockClient := queue.NewMockStreamsClient()

// Set up expectations
mockClient.On("AddToStream", ctx, "test_stream", mock.Anything).
    Return("1234567890-0", nil)

// Use in tests
producer := queue.NewProducer(mockClient)
err := producer.SendEvent(ctx, event)
assert.NoError(t, err)
```

## Migration from SQS

If migrating from AWS SQS:

1. **Message Format**: Convert SQS JSON to Redis Streams format
2. **Consumer Groups**: Replace SQS visibility timeout with Redis consumer groups
3. **DLQ**: Replace SQS DLQ with Redis DLQ stream
4. **Metrics**: Update CloudWatch metrics to Prometheus metrics
5. **IAM**: Remove SQS IAM policies, use Redis ACLs if needed

## Troubleshooting

### Common Issues

1. **Consumer Lag Growing**
   - Check processing errors
   - Scale up consumers
   - Increase batch size

2. **Messages Not Acknowledged**
   - Verify ACK is called after successful processing
   - Check for processing timeouts

3. **DLQ Growing**
   - Review error logs
   - Check message format
   - Verify downstream services

### Debug Commands

```bash
# Read last 10 messages
redis-cli xread count 10 streams webhook_events $

# Check pending messages
redis-cli xpending webhook_events webhook_workers - + 10

# Claim stuck messages
redis-cli xclaim webhook_events webhook_workers consumer-1 3600000 1234567890-0
```