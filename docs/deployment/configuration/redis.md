# Redis Configuration

## Overview

Developer Mesh uses Redis for multiple purposes:
- **Redis Streams**: Event processing and webhook handling
- **Caching**: Response and embedding caching
- **Session Management**: WebSocket session state
- **Rate Limiting**: API rate limit counters
- **Distributed Locks**: Coordination between services

## Redis Requirements

- **Version**: Redis 7.0 or higher
- **Memory**: Minimum 1GB recommended
- **Persistence**: AOF recommended for production
- **Modules**: No additional modules required

## Connection Configuration

### Environment Variables

```bash
# Basic connection
REDIS_ADDR=localhost:6379        # Redis address (host:port)
REDIS_PASSWORD=your_password     # Redis password (optional)
REDIS_DB=0                       # Database number (0-15)

# Alternative format
REDIS_HOST=localhost             # Host only
REDIS_PORT=6379                 # Port only

# Connection pool settings
REDIS_POOL_SIZE=50              # Maximum connections
REDIS_MIN_IDLE_CONNS=10         # Minimum idle connections
REDIS_MAX_RETRIES=3             # Maximum retry attempts
```

### Configuration File (config.yaml)

```yaml
redis:
  address: ${REDIS_ADDR:-localhost:6379}
  password: ${REDIS_PASSWORD}
  db: ${REDIS_DB:-0}
  
  # Connection pool
  pool:
    size: 50                    # Max connections
    min_idle: 10                # Min idle connections
    max_retries: 3              # Retry attempts
    retry_backoff: 100ms        # Backoff between retries
    
  # Timeouts
  timeouts:
    connect: 5s                 # Connection timeout
    read: 3s                    # Read timeout
    write: 3s                   # Write timeout
    idle: 300s                  # Idle connection timeout
```

## Redis Streams Configuration

Redis Streams are used for event processing and webhook handling.

### Stream Settings

```yaml
redis:
  streams:
    # Stream names
    webhook_events: "webhook_events"
    task_events: "task_events"
    agent_messages: "agent_messages"
    
    # Consumer group settings
    consumer_groups:
      webhook_workers:
        stream: webhook_events
        start_id: ">"           # Start from new messages
        
      task_workers:
        stream: task_events
        start_id: "0"           # Start from beginning
        
    # Stream configuration
    config:
      max_len: 100000           # Maximum stream length
      block_duration: 5s        # XREAD block duration
      batch_size: 100           # Messages per batch
      claim_min_idle: 60s       # Claim idle messages after
      
    # Dead letter queue
    dlq:
      enabled: true
      max_retries: 3
      ttl: 7d                   # Keep failed messages for 7 days
```

### Worker Configuration

```yaml
worker:
  # Consumer settings
  consumer_name: ${WORKER_CONSUMER_NAME:-worker-1}
  consumer_group: webhook_workers
  
  # Processing settings
  batch_size: 100
  poll_interval: 100ms
  max_retries: 3
  
  # Idempotency
  idempotency:
    enabled: true
    ttl: 24h                    # Idempotency key TTL
    key_prefix: "processed:"
```

## Caching Configuration

### Cache Settings

```yaml
cache:
  # Default TTL for different cache types
  ttl:
    embeddings: 15m             # Embedding cache
    api_responses: 5m           # API response cache
    tool_specs: 10m             # Tool specification cache
    sessions: 30m               # Session data
    
  # Cache key prefixes
  prefixes:
    embedding: "emb:"
    response: "resp:"
    tool: "tool:"
    session: "sess:"
    
  # Eviction policy
  eviction:
    policy: allkeys-lru         # LRU eviction
    max_memory: 1gb             # Maximum memory usage
```

### Cache Implementation

```go
// Cache key patterns
embedding:{tenant_id}:{model}:{hash}
response:{endpoint}:{params_hash}
tool:{tool_id}:spec
session:{session_id}
rate_limit:{api_key}:{window}
```

## High Availability Configuration

### Redis Sentinel

For production high availability, use Redis Sentinel:

```yaml
redis:
  sentinel:
    enabled: true
    master_name: webhook-master
    addresses:
      - sentinel1:26379
      - sentinel2:26379
      - sentinel3:26379
    
    # Sentinel options
    options:
      password: ${SENTINEL_PASSWORD}
      dial_timeout: 5s
      failover_timeout: 60s
```

### Redis Cluster

For horizontal scaling, use Redis Cluster:

```yaml
redis:
  cluster:
    enabled: true
    addresses:
      - redis-node1:6379
      - redis-node2:6379
      - redis-node3:6379
      - redis-node4:6379
      - redis-node5:6379
      - redis-node6:6379
    
    # Cluster options
    options:
      max_redirects: 3
      read_only: false          # Allow reads from replicas
      route_by_latency: true    # Route to fastest node
```

## TLS/SSL Configuration

### TLS Settings

```yaml
redis:
  tls:
    enabled: true
    cert_file: /path/to/client.crt
    key_file: /path/to/client.key
    ca_file: /path/to/ca.crt
    
    # TLS options
    options:
      skip_verify: false        # Verify server certificate
      server_name: redis.example.com
      min_version: "1.2"        # Minimum TLS version
```

### Environment Variables for TLS

```bash
REDIS_TLS_ENABLED=true
REDIS_TLS_CERT_FILE=/path/to/client.crt
REDIS_TLS_KEY_FILE=/path/to/client.key
REDIS_TLS_CA_FILE=/path/to/ca.crt
REDIS_TLS_SKIP_VERIFY=false
```

## Performance Tuning

### Redis Server Configuration (redis.conf)

```conf
# Memory settings
maxmemory 2gb
maxmemory-policy allkeys-lru

# Persistence
appendonly yes
appendfsync everysec
no-appendfsync-on-rewrite no

# Performance
tcp-keepalive 300
tcp-backlog 511
timeout 0

# Slow log
slowlog-log-slower-than 10000
slowlog-max-len 128

# Client connections
maxclients 10000

# Threading (Redis 6+)
io-threads 4
io-threads-do-reads yes
```

### Application-Level Optimizations

```yaml
redis:
  # Pipeline settings
  pipeline:
    enabled: true
    max_commands: 100          # Commands per pipeline
    flush_interval: 10ms       # Auto-flush interval
    
  # Connection pooling
  pool:
    size: 50
    min_idle: 10
    max_conn_age: 30m          # Maximum connection age
    
  # Circuit breaker
  circuit_breaker:
    enabled: true
    failure_threshold: 5       # Failures to open
    success_threshold: 2       # Successes to close
    timeout: 30s               # Open state timeout
```

## Monitoring

### Health Checks

```bash
# Check Redis health
redis-cli ping

# Check memory usage
redis-cli info memory

# Check connected clients
redis-cli client list

# Check slow queries
redis-cli slowlog get 10
```

### Metrics to Monitor

```yaml
metrics:
  # Connection metrics
  - redis_connected_clients
  - redis_blocked_clients
  - redis_rejected_connections
  
  # Memory metrics
  - redis_used_memory
  - redis_used_memory_rss
  - redis_mem_fragmentation_ratio
  
  # Performance metrics
  - redis_instantaneous_ops_per_sec
  - redis_hit_rate
  - redis_evicted_keys
  
  # Stream metrics
  - redis_stream_length
  - redis_stream_consumer_lag
  - redis_stream_pending_messages
```

## Backup and Recovery

### Backup Strategies

```bash
# RDB snapshot
redis-cli BGSAVE

# AOF backup
cp /var/lib/redis/appendonly.aof backup-$(date +%Y%m%d).aof

# Redis dump
redis-cli --rdb dump.rdb

# Backup with compression
redis-cli --rdb - | gzip > backup-$(date +%Y%m%d).rdb.gz
```

### Restore Procedures

```bash
# Restore from RDB
cp backup.rdb /var/lib/redis/dump.rdb
redis-server

# Restore from AOF
cp backup.aof /var/lib/redis/appendonly.aof
redis-server --appendonly yes

# Import from dump
cat dump.rdb | redis-cli --pipe
```

## Security Configuration

### Authentication

```yaml
redis:
  # Password authentication
  password: ${REDIS_PASSWORD}
  
  # ACL configuration (Redis 6+)
  acl:
    enabled: true
    users:
      - name: app_user
        password: ${APP_PASSWORD}
        commands: ["+@all", "-flushdb", "-flushall", "-config"]
        keys: ["*"]
        
      - name: readonly_user
        password: ${READONLY_PASSWORD}
        commands: ["+@read"]
        keys: ["*"]
```

### Network Security

```yaml
redis:
  # Bind to specific interfaces
  bind: 127.0.0.1 ::1
  
  # Protected mode
  protected_mode: yes
  
  # Rename dangerous commands
  rename_commands:
    FLUSHDB: ""
    FLUSHALL: ""
    CONFIG: "CONFIG_e8f9c6d5"
```

## Multi-Environment Setup

### Development

```yaml
redis:
  address: localhost:6379
  password: ""                  # No password in dev
  db: 0
  pool:
    size: 10
```

### Staging

```yaml
redis:
  address: staging-redis.example.com:6379
  password: ${REDIS_PASSWORD}
  db: 0
  tls:
    enabled: true
  pool:
    size: 25
```

### Production

```yaml
redis:
  sentinel:
    enabled: true
    master_name: production-master
    addresses:
      - sentinel1.prod:26379
      - sentinel2.prod:26379
      - sentinel3.prod:26379
  password: ${REDIS_PASSWORD}
  tls:
    enabled: true
  pool:
    size: 50
```

## AWS ElastiCache Configuration

### ElastiCache Settings

```yaml
redis:
  elasticache:
    enabled: true
    endpoint: ${ELASTICACHE_ENDPOINT}
    port: 6379
    auth_token: ${ELASTICACHE_AUTH_TOKEN}
    
    # Cluster mode
    cluster_mode:
      enabled: true
      configuration_endpoint: ${ELASTICACHE_CONFIG_ENDPOINT}
    
    # Encryption
    encryption:
      at_rest: true
      in_transit: true
```

### ElastiCache Parameter Group

```ini
# Custom parameter group
timeout = 300
tcp-keepalive = 300
maxmemory-policy = allkeys-lru
notify-keyspace-events = AKE
```

## Troubleshooting

### Connection Issues

```bash
# Test connection
redis-cli -h $REDIS_HOST -p $REDIS_PORT ping

# Test with password
redis-cli -h $REDIS_HOST -a $REDIS_PASSWORD ping

# Check connectivity
nc -zv $REDIS_HOST $REDIS_PORT
```

### Performance Issues

```bash
# Monitor commands in real-time
redis-cli monitor

# Check slow log
redis-cli slowlog get 20

# Analyze memory usage
redis-cli --bigkeys

# Check stream info
redis-cli xinfo stream webhook_events
```

### Stream Issues

```bash
# Check consumer groups
redis-cli xinfo groups webhook_events

# Check pending messages
redis-cli xpending webhook_events webhook_workers

# Claim idle messages
redis-cli xclaim webhook_events webhook_workers consumer-1 60000 message-id
```

## Best Practices

1. **Use connection pooling** to reduce connection overhead
2. **Set appropriate TTLs** for cached data
3. **Monitor memory usage** and configure eviction policies
4. **Use pipelining** for batch operations
5. **Enable persistence** in production (AOF recommended)
6. **Implement circuit breakers** for resilience
7. **Use Redis Streams** for reliable event processing
8. **Configure TLS/SSL** for production
9. **Set up monitoring** and alerting
10. **Regular backups** with point-in-time recovery

## Related Documentation

- [Environment Variables Reference](../ENVIRONMENT_VARIABLES.md)
- [Configuration Overview](configuration-overview.md)
- [Production Deployment](../guides/production-deployment.md)