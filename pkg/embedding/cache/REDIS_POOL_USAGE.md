# Redis Connection Pool Configuration

The semantic cache now includes configurable Redis connection pool settings to optimize performance and prevent connection exhaustion.

## Usage

### Using Default Configuration
```go
// Create Redis client with default pool configuration
redisClient := cache.NewRedisClientWithPool("localhost:6379", 0, nil)

// Create semantic cache with the pooled client
config := cache.DefaultConfig() // Includes default pool config
semanticCache, err := cache.NewSemanticCache(redisClient, config, logger)
```

### Using Custom Configuration
```go
// Create custom pool configuration
poolConfig := &cache.RedisPoolConfig{
    PoolSize:     200,
    MinIdleConns: 20,
    MaxRetries:   5,
    DialTimeout:  3 * time.Second,
    ReadTimeout:  2 * time.Second,
    WriteTimeout: 2 * time.Second,
    PoolTimeout:  3 * time.Second,
    IdleTimeout:  10 * time.Minute,
}

// Create Redis client with custom pool
redisClient := cache.NewRedisClientWithPool("localhost:6379", 0, poolConfig)

// Or update cache config
config := cache.DefaultConfig()
config.RedisPoolConfig = poolConfig
```

### Using Pre-configured Profiles

```go
// For high-load scenarios
config := cache.DefaultConfig()
config.RedisPoolConfig = cache.HighLoadRedisPoolConfig()

// For low-latency requirements
config := cache.DefaultConfig()
config.RedisPoolConfig = cache.LowLatencyRedisPoolConfig()
```

## Configuration Parameters

- **PoolSize**: Maximum number of socket connections (default: 100)
- **MinIdleConns**: Minimum number of idle connections (default: 10)
- **MaxRetries**: Maximum retries before giving up (default: 3)
- **DialTimeout**: Timeout for establishing new connections (default: 5s)
- **ReadTimeout**: Timeout for socket reads (default: 3s)
- **WriteTimeout**: Timeout for socket writes (default: 3s)
- **PoolTimeout**: Time to wait for connection from pool (default: 4s)
- **IdleTimeout**: Time after which idle connections are closed (default: 5m)
- **IdleCheckFreq**: Frequency of idle checks (default: 1m)
- **MaxConnAge**: Maximum age of a connection (default: 30m)

## Monitoring

Monitor these Redis metrics:
- `redis.pool.hits` - Successful connection acquisitions from pool
- `redis.pool.misses` - New connections created
- `redis.pool.timeouts` - Pool timeout errors
- `redis.pool.total_conns` - Total connections in pool
- `redis.pool.idle_conns` - Current idle connections
- `redis.pool.stale_conns` - Connections closed due to age

## Best Practices

1. **Start with defaults**: The default configuration works well for most use cases
2. **Monitor pool metrics**: Adjust pool size based on actual usage patterns
3. **Set appropriate timeouts**: Balance between failing fast and allowing for network hiccups
4. **Use profiles**: Choose high-load or low-latency profiles based on your requirements
5. **Test under load**: Verify pool behavior under expected peak loads