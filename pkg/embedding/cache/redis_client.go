package cache

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/retry"
	"github.com/redis/go-redis/v9"
)

// ResilientRedisClient wraps Redis client with circuit breaker and retry logic
type ResilientRedisClient struct {
	client         *redis.Client
	circuitBreaker *resilience.CircuitBreaker
	logger         observability.Logger
	metrics        observability.MetricsClient
	retryPolicy    retry.Policy
}

// NewResilientRedisClient creates a new resilient Redis client
func NewResilientRedisClient(
	client *redis.Client,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *ResilientRedisClient {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.redis")
	}

	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	// Use project's circuit breaker configuration
	cbConfig := resilience.CircuitBreakerConfig{
		FailureThreshold:    5,
		FailureRatio:        0.6,
		ResetTimeout:        30 * time.Second,
		SuccessThreshold:    2,
		TimeoutThreshold:    5 * time.Second,
		MaxRequestsHalfOpen: 5,
		MinimumRequestCount: 10,
	}

	// Use project's retry policy configuration
	retryConfig := retry.Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxRetries:      3,
		Multiplier:      2.0,
		MaxElapsedTime:  30 * time.Second,
	}

	return &ResilientRedisClient{
		client:         client,
		circuitBreaker: resilience.NewCircuitBreaker("redis_cache", cbConfig, logger, metrics),
		logger:         logger,
		metrics:        metrics,
		retryPolicy:    retry.NewExponentialBackoff(retryConfig),
	}
}

// NewResilientRedisClientWithConfig creates a new resilient Redis client with performance config
func NewResilientRedisClientWithConfig(
	client *redis.Client,
	perfConfig *PerformanceConfig,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *ResilientRedisClient {
	if logger == nil {
		logger = observability.NewLogger("embedding.cache.redis")
	}

	if metrics == nil {
		metrics = observability.NewMetricsClient()
	}

	if perfConfig == nil {
		perfConfig = GetPerformanceProfile(ProfileBalanced)
	}

	// Configure circuit breaker based on performance config
	var cbConfig resilience.CircuitBreakerConfig
	if perfConfig.CircuitBreakerEnabled {
		cbConfig = resilience.CircuitBreakerConfig{
			FailureThreshold:    perfConfig.CircuitBreakerThreshold,
			FailureRatio:        0.6,
			ResetTimeout:        perfConfig.CircuitBreakerResetTimeout,
			SuccessThreshold:    2,
			TimeoutThreshold:    perfConfig.CircuitBreakerTimeout,
			MaxRequestsHalfOpen: 5,
			MinimumRequestCount: 10,
		}
	} else {
		// Disabled circuit breaker
		cbConfig = resilience.CircuitBreakerConfig{
			FailureThreshold: 999999, // Effectively disabled
		}
	}

	// Configure retry policy based on performance config
	var retryConfig retry.Config
	if perfConfig.RetryEnabled {
		retryConfig = retry.Config{
			InitialInterval: perfConfig.RetryInitialDelay,
			MaxInterval:     perfConfig.RetryMaxDelay,
			MaxRetries:      perfConfig.RetryMaxAttempts,
			Multiplier:      perfConfig.RetryMultiplier,
			MaxElapsedTime:  perfConfig.RetryMaxDelay * time.Duration(perfConfig.RetryMaxAttempts),
		}
	} else {
		retryConfig = retry.Config{
			MaxRetries: 0, // No retries
		}
	}

	return &ResilientRedisClient{
		client:         client,
		circuitBreaker: resilience.NewCircuitBreaker("redis_cache", cbConfig, logger, metrics),
		logger:         logger,
		metrics:        metrics,
		retryPolicy:    retry.NewExponentialBackoff(retryConfig),
	}
}

// Get retrieves a value from Redis with circuit breaker protection
func (r *ResilientRedisClient) Get(ctx context.Context, key string) (string, error) {
	result, err := r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		var val string
		err := r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			v, err := r.client.Get(ctx, key).Result()
			if err != nil {
				return err
			}
			val = v
			return nil
		})
		return val, err
	})

	if err != nil {
		return "", err
	}

	return result.(string), nil
}

// Set stores a value in Redis with circuit breaker protection
func (r *ResilientRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	_, err := r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		return nil, r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			return r.client.Set(ctx, key, value, expiration).Err()
		})
	})
	return err
}

// Del deletes a key from Redis with circuit breaker protection
func (r *ResilientRedisClient) Del(ctx context.Context, keys ...string) error {
	_, err := r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		return nil, r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			return r.client.Del(ctx, keys...).Err()
		})
	})
	return err
}

// Scan performs a scan operation with circuit breaker protection
func (r *ResilientRedisClient) Scan(ctx context.Context, cursor uint64, match string, count int64) *redis.ScanCmd {
	// For scan operations, we need to handle it differently as it returns an iterator
	return r.client.Scan(ctx, cursor, match, count)
}

// Eval executes a Lua script with circuit breaker protection
func (r *ResilientRedisClient) Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error) {
	return r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		var result interface{}
		err := r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			val, err := r.client.Eval(ctx, script, keys, args...).Result()
			if err != nil {
				return err
			}
			result = val
			return nil
		})
		return result, err
	})
}

// MemoryUsage gets memory usage of a key with circuit breaker protection
func (r *ResilientRedisClient) MemoryUsage(ctx context.Context, key string) (int64, error) {
	result, err := r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		var val int64
		err := r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			v, err := r.client.MemoryUsage(ctx, key).Result()
			if err != nil {
				return err
			}
			val = v
			return nil
		})
		return val, err
	})

	if err != nil {
		return 0, err
	}

	return result.(int64), nil
}

// Close closes the Redis client
func (r *ResilientRedisClient) Close() error {
	return r.client.Close()
}

// Execute wraps Redis operations with circuit breaker and retry
func (r *ResilientRedisClient) Execute(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	return r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		var result interface{}
		err := r.retryPolicy.Execute(ctx, func(ctx context.Context) error {
			var opErr error
			result, opErr = operation()
			return opErr
		})
		return result, err
	})
}

// GetClient returns the underlying Redis client for operations not covered by the wrapper
func (r *ResilientRedisClient) GetClient() *redis.Client {
	return r.client
}

// Health checks if Redis is healthy
func (r *ResilientRedisClient) Health(ctx context.Context) error {
	_, err := r.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		return nil, r.client.Ping(ctx).Err()
	})
	return err
}
