<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:36:46
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Redis Migration Guide: From SQS to Redis Streams <!-- Source: pkg/redis/streams_client.go -->

## Overview

This guide covers the complete migration from AWS SQS to Redis Streams for the DevOps MCP webhook processing system. <!-- Source: pkg/redis/streams_client.go -->

## Architecture Changes

### Before (SQS-based)
```
GitHub Webhook → REST API → SQS → Worker → Process Event
                                ↓
                          Redis (Idempotency)
```

### After (Redis-based)
```
GitHub Webhook → REST API → Redis Streams → Worker → Process Event <!-- Source: pkg/redis/streams_client.go -->
                                    ↓
                            AI Intelligence Layer
                                    ↓
                          Context Management (Hot/Warm/Cold)
```

## Migration Steps

### 1. Pre-Migration Checklist

- [ ] Redis cluster deployed and accessible
- [ ] Redis Sentinel configured for HA
- [ ] TLS certificates generated (if using TLS)
- [ ] Monitoring dashboards prepared
- [ ] Team notified of migration window

### 2. Environment Variables

#### Remove/Comment Out SQS Variables
```bash
# SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/123456789/webhooks
# AWS_REGION=us-east-1
# AWS_ACCESS_KEY_ID=xxx
# AWS_SECRET_ACCESS_KEY=xxx
```

#### Add Redis Variables
```bash
# Queue Type Selection
QUEUE_TYPE=redis  # Options: sqs, redis

# Redis Connection
REDIS_ADDRESS=redis-cluster.example.com:6379
REDIS_PASSWORD=your-redis-password
REDIS_TLS_ENABLED=true
REDIS_TLS_CERT_FILE=/etc/redis/certs/client.crt
REDIS_TLS_KEY_FILE=/etc/redis/certs/client.key
REDIS_TLS_CA_FILE=/etc/redis/certs/ca.crt

# Redis Sentinel (for HA)
REDIS_SENTINEL_ENABLED=true
REDIS_SENTINEL_MASTER_NAME=webhook-master
REDIS_SENTINEL_ADDRESSES=sentinel1:26379,sentinel2:26379,sentinel3:26379

# Redis Streams Configuration <!-- Source: pkg/redis/streams_client.go -->
REDIS_STREAM_NAME=webhook-events <!-- Source: pkg/redis/streams_client.go -->
REDIS_CONSUMER_GROUP=webhook-processors
REDIS_MAX_LEN=1000000  # Maximum stream length <!-- Source: pkg/redis/streams_client.go -->
REDIS_BLOCK_DURATION=5s  # How long to block on reads

# Worker Configuration
WORKER_COUNT=10
WORKER_BATCH_SIZE=100
WORKER_PREFETCH_COUNT=1000
```

### 3. Code Changes Applied

#### Queue Factory Pattern
The codebase now uses a factory pattern to select between SQS and Redis:

```go
// pkg/queue/factory.go
queueAdapter, err := queue.NewQueueAdapter(ctx, &queue.QueueConfig{
    Type: queue.GetQueueType(), // Reads QUEUE_TYPE env var
}, logger)
```

#### Webhook Handler Update
```go
// apps/rest-api/internal/api/webhooks/webhooks.go
// Changed from:
sqsClient, err := queue.NewSQSClient(ctx)

// To:
queueAdapter, err := queue.NewQueueAdapter(ctx, &queue.QueueConfig{
    Type: queue.GetQueueType(),
}, logger)
```

#### Worker Service Update
```go
// apps/worker/cmd/worker/main.go
// Now uses the queue factory instead of direct SQS client
queueAdapter, err := queue.NewQueueAdapter(ctx, queueConfig, logger)
```

### 4. Deployment Steps

#### Step 1: Deploy with Dual Configuration (Safe)
```bash
# Keep SQS running but prepare Redis
export QUEUE_TYPE=sqs
export REDIS_ADDRESS=redis-cluster:6379
# Deploy and verify Redis connectivity
```

#### Step 2: Test Redis in Staging
```bash
# In staging environment
export QUEUE_TYPE=redis
# Deploy and run comprehensive tests
```

#### Step 3: Gradual Production Rollout
```bash
# Start with one instance using Redis
kubectl set env deployment/webhook-handler QUEUE_TYPE=redis --dry-run=client -o yaml | kubectl apply -f -

# Monitor for 30 minutes
# If successful, roll out to more instances
```

#### Step 4: Complete Migration
```bash
# Update all instances
export QUEUE_TYPE=redis
kubectl set env deployment/webhook-handler QUEUE_TYPE=redis
kubectl set env deployment/worker QUEUE_TYPE=redis
```

### 5. Verification

#### Check Redis Stream <!-- Source: pkg/redis/streams_client.go -->
```bash
# View stream info
redis-cli XINFO STREAM webhook-events <!-- Source: pkg/redis/streams_client.go -->

# Monitor new messages
redis-cli XREAD BLOCK 0 STREAMS webhook-events $ <!-- Source: pkg/redis/streams_client.go -->

# Check consumer groups
redis-cli XINFO GROUPS webhook-events

# Check pending messages
redis-cli XPENDING webhook-events webhook-processors
```

#### Monitor Application Logs
```bash
# Webhook handler logs
kubectl logs -f deployment/webhook-handler | grep -E "(queue|redis)"

# Worker logs
kubectl logs -f deployment/worker | grep -E "(Received|Processing|redis)"
```

### 6. Rollback Plan

If issues occur, rollback is simple:

```bash
# Immediate rollback
export QUEUE_TYPE=sqs
kubectl set env deployment/webhook-handler QUEUE_TYPE=sqs
kubectl set env deployment/worker QUEUE_TYPE=sqs

# Services will automatically switch back to SQS
```

### 7. Post-Migration Cleanup

After successful migration and stability period (recommended: 1 week):

1. **Remove SQS IAM Policies**
   ```bash
   aws iam detach-role-policy --role-name webhook-processor-role --policy-arn arn:aws:iam::xxx:policy/sqs-access
   ```

2. **Delete SQS Queue** (after backing up any remaining messages)
   ```bash
   aws sqs delete-queue --queue-url https://sqs.us-east-1.amazonaws.com/xxx/webhooks
   ```

3. **Remove SQS Code** (Phase 2 - after 30 days)
   - Delete `/pkg/queue/sqs.go`
   - Delete `/pkg/queue/sqsadapter.go`
   - Remove SQS dependencies from `go.mod`
   - Remove SQS health checks

## Monitoring and Alerts

### Key Metrics to Monitor

1. **Queue Depth**
   ```promql
   redis_stream_length{stream="webhook-events"} <!-- Source: pkg/redis/streams_client.go -->
   ```

2. **Processing Latency**
   ```promql
   webhook_event_processing_duration_seconds{quantile="0.99"}
   ```

3. **Error Rate**
   ```promql
   rate(webhook_events_failed_total[5m])
   ```

4. **Consumer Lag**
   ```promql
   redis_stream_consumer_lag{group="webhook-processors"} <!-- Source: pkg/redis/streams_client.go -->
   ```

### Alerts to Configure

```yaml
- alert: HighWebhookQueueDepth
  expr: redis_stream_length{stream="webhook-events"} > 10000 <!-- Source: pkg/redis/streams_client.go -->
  for: 5m
  
- alert: WebhookProcessingErrors
  expr: rate(webhook_events_failed_total[5m]) > 0.01
  for: 5m
  
- alert: RedisConnectionFailure
  expr: up{job="redis"} == 0
  for: 1m
```

## Benefits of Redis Over SQS

1. **Cost Savings**: No per-message charges
2. **Lower Latency**: Sub-millisecond vs SQS's network latency
3. **Better Visibility**: Can inspect messages without consuming
4. **Advanced Features**: 
   - Consumer groups with automatic rebalancing
   - Stream trimming policies
   - Exact-once delivery semantics
5. **AI Integration**: Built-in support for embeddings and context management
6. **Cloud Agnostic**: Works on any cloud or on-premises

## Troubleshooting

### Issue: High Memory Usage on Redis
```bash
# Check memory usage
redis-cli INFO memory

# Trim stream if needed
redis-cli XTRIM webhook-events MAXLEN ~ 100000
```

### Issue: Stuck Consumer
```bash
# Check pending messages
redis-cli XPENDING webhook-events webhook-processors

# Claim old messages
redis-cli XCLAIM webhook-events webhook-processors consumer-new 3600000 message-id
```

### Issue: Connection Timeouts
- Check Redis cluster health
- Verify network connectivity
- Check TLS certificate validity
- Review connection pool settings

## Performance Tuning

### Redis Configuration
```conf
# redis.conf
maxmemory 8gb
maxmemory-policy allkeys-lru
stream-node-max-bytes 4096
stream-node-max-entries 100
```

### Application Tuning
```yaml
# Optimal settings based on load testing
consumer:
  num_workers: 20  # 2x CPU cores
  batch_size: 100
  prefetch_count: 1000
  block_timeout: 5s
```

## Security Considerations

1. **Encryption in Transit**: Always use TLS for Redis connections
2. **Encryption at Rest**: Enable Redis persistence encryption
3. **Access Control**: Use Redis ACLs to limit permissions
4. **Network Security**: Keep Redis in private subnet
5. **Audit Logging**: Enable Redis command logging

## Migration Timeline

- **Week 1**: Deploy Redis infrastructure
- **Week 2**: Test in development/staging
- **Week 3**: Gradual production rollout
- **Week 4**: Monitor and optimize
- **Week 5**: Remove SQS dependencies

## Support

For issues during migration:
- Slack: #devops-mcp-migration
- Wiki: internal.wiki/redis-migration
