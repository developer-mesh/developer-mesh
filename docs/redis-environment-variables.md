# Redis Queue Environment Variables

## Required Environment Variables

The DevOps MCP now uses Redis Streams for all queue operations. Configure these environment variables:

### Redis Connection
```bash
# Redis server address (required)
REDIS_ADDRESS=localhost:6379

# Redis password (optional)
REDIS_PASSWORD=your-password

# Redis database number (optional, default: 0)
REDIS_DB=0
```

### Redis TLS Configuration
```bash
# Enable TLS for Redis connections
REDIS_TLS_ENABLED=true

# TLS certificate files (required if TLS enabled)
REDIS_TLS_CERT_FILE=/path/to/client.crt
REDIS_TLS_KEY_FILE=/path/to/client.key
REDIS_TLS_CA_FILE=/path/to/ca.crt

# Skip TLS verification (development only)
REDIS_TLS_SKIP_VERIFY=false
```

### Redis Sentinel (High Availability)
```bash
# Enable Redis Sentinel
REDIS_SENTINEL_ENABLED=true

# Sentinel addresses (comma-separated)
REDIS_SENTINEL_ADDRESSES=sentinel1:26379,sentinel2:26379,sentinel3:26379

# Sentinel master name
REDIS_SENTINEL_MASTER_NAME=webhook-master
```

### Redis Streams Configuration
```bash
# Stream name for webhook events
REDIS_STREAM_NAME=webhook-events

# Consumer group name
REDIS_CONSUMER_GROUP=webhook-processors

# Maximum stream length (0 = unlimited)
REDIS_MAX_LEN=1000000

# Block duration for XREAD (seconds)
REDIS_BLOCK_DURATION=5
```

### Worker Configuration
```bash
# Number of concurrent workers
WORKER_COUNT=10

# Batch size for message processing
WORKER_BATCH_SIZE=100

# Prefetch count for better throughput
WORKER_PREFETCH_COUNT=1000

# Idempotency TTL (hours)
WORKER_IDEMPOTENCY_TTL=24
```

## Removed Environment Variables

The following AWS SQS-related variables are no longer needed:

- ~~`SQS_QUEUE_URL`~~ - Replaced by Redis Streams
- ~~`AWS_REGION`~~ - Not needed for Redis
- ~~`AWS_ACCESS_KEY_ID`~~ - Not needed for Redis
- ~~`AWS_SECRET_ACCESS_KEY`~~ - Not needed for Redis
- ~~`SQS_ENABLED`~~ - Redis is always enabled
- ~~`WORKER_QUEUE_TYPE`~~ - Always uses Redis now

## Example Configuration

### Development
```bash
export REDIS_ADDRESS=localhost:6379
export REDIS_STREAM_NAME=webhook-events-dev
export WORKER_COUNT=2
```

### Production
```bash
export REDIS_ADDRESS=redis-cluster.internal:6379
export REDIS_PASSWORD=${REDIS_PASSWORD_FROM_SECRETS}
export REDIS_TLS_ENABLED=true
export REDIS_TLS_CERT_FILE=/etc/redis/certs/client.crt
export REDIS_TLS_KEY_FILE=/etc/redis/certs/client.key
export REDIS_TLS_CA_FILE=/etc/redis/certs/ca.crt
export REDIS_SENTINEL_ENABLED=true
export REDIS_SENTINEL_ADDRESSES=redis-sentinel1:26379,redis-sentinel2:26379,redis-sentinel3:26379
export REDIS_STREAM_NAME=webhook-events
export REDIS_CONSUMER_GROUP=webhook-processors
export WORKER_COUNT=20
export WORKER_BATCH_SIZE=100
```

## Docker Compose Example

```yaml
version: '3.8'
services:
  webhook-handler:
    image: devops-mcp/webhook-handler:latest
    environment:
      - REDIS_ADDRESS=redis:6379
      - REDIS_STREAM_NAME=webhook-events
      - REDIS_CONSUMER_GROUP=webhook-processors
    depends_on:
      - redis

  worker:
    image: devops-mcp/worker:latest
    environment:
      - REDIS_ADDRESS=redis:6379
      - REDIS_STREAM_NAME=webhook-events
      - REDIS_CONSUMER_GROUP=webhook-processors
      - WORKER_COUNT=5
    depends_on:
      - redis

  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data

volumes:
  redis-data:
```

## Kubernetes ConfigMap Example

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: webhook-config
  namespace: devops-mcp
data:
  REDIS_ADDRESS: "redis-service.devops-mcp.svc.cluster.local:6379"
  REDIS_STREAM_NAME: "webhook-events"
  REDIS_CONSUMER_GROUP: "webhook-processors"
  REDIS_TLS_ENABLED: "true"
  WORKER_COUNT: "10"
  WORKER_BATCH_SIZE: "100"
```