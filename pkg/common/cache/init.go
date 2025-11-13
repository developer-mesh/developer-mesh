package cache

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	securitytls "github.com/developer-mesh/developer-mesh/pkg/security/tls"
)

// ErrNotFound is returned when a key is not found in the cache
var ErrNotFound = errors.New("key not found in cache")

// RedisConfig holds configuration for Redis
type RedisConfig struct {
	Type         string        `mapstructure:"type"`           // "redis" or "redis_cluster"
	Address      string        `mapstructure:"address"`        // Redis address (single instance)
	Addresses    []string      `mapstructure:"addresses"`      // Redis addresses (cluster mode)
	Username     string        `mapstructure:"username"`       // Redis username
	Password     string        `mapstructure:"password"`       // Redis password
	Database     int           `mapstructure:"database"`       // Redis database number (single mode only)
	MaxRetries   int           `mapstructure:"max_retries"`    // Max retries on failure
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`   // Dial timeout
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`   // Read timeout
	WriteTimeout time.Duration `mapstructure:"write_timeout"`  // Write timeout
	PoolSize     int           `mapstructure:"pool_size"`      // Connection pool size
	MinIdleConns int           `mapstructure:"min_idle_conns"` // Min idle connections
	PoolTimeout  time.Duration `mapstructure:"pool_timeout"`   // Pool timeout
	UseIAMAuth   bool          `mapstructure:"use_iam_auth"`   // Use IAM authentication for Redis

	// AWS ElastiCache specific configuration
	UseAWS            bool                   `mapstructure:"use_aws"`      // Use AWS ElastiCache
	ClusterMode       bool                   `mapstructure:"cluster_mode"` // Use ElastiCache in cluster mode
	ElastiCacheConfig *aws.ElastiCacheConfig `mapstructure:"elasticache"`  // ElastiCache configuration

	// TLS configuration
	TLS *TLSConfig `mapstructure:"tls"` // TLS configuration
}

// TLSConfig holds TLS configuration
type TLSConfig struct {
	securitytls.Config `mapstructure:",squash"` // Embed secure TLS configuration
}

// NewCache creates a new cache based on the configuration
func NewCache(ctx context.Context, cfg interface{}) (Cache, error) {
	switch config := cfg.(type) {
	case RedisConfig:
		// Default to AWS ElastiCache with IAM auth in production environments
		isLocalEnv := os.Getenv("MCP_ENV") == "local" || os.Getenv("MCP_ENVIRONMENT") == "local"

		// Determine if we should use AWS ElastiCache
		useAWS := config.UseAWS
		if !isLocalEnv && config.ElastiCacheConfig != nil {
			// In non-local environments, prefer AWS ElastiCache unless explicitly disabled
			useAWS = true
		}

		// If we should use AWS ElastiCache
		if useAWS && config.ElastiCacheConfig != nil {
			return newAWSElastiCacheClient(ctx, config)
		}

		// Check if we should use cluster mode
		if config.Type == "redis_cluster" || len(config.Addresses) > 0 {
			return newRedisClusterClient(config)
		}

		// Standard Redis client
		return NewRedisCache(RedisConfig{
			Address:      config.Address,
			Username:     config.Username,
			Password:     config.Password,
			Database:     config.Database,
			MaxRetries:   config.MaxRetries,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			PoolTimeout:  config.PoolTimeout,
			TLS:          config.TLS,
		})
	default:
		return nil, fmt.Errorf("unsupported cache type: %T", cfg)
	}
}

// newRedisClusterClient creates a new Redis cluster client
func newRedisClusterClient(config RedisConfig) (Cache, error) {
	// Create cluster configuration
	clusterConfig := RedisClusterConfig{
		Addrs:          config.Addresses,
		Username:       config.Username,
		Password:       config.Password,
		MaxRetries:     config.MaxRetries,
		MinIdleConns:   config.MinIdleConns,
		PoolSize:       config.PoolSize,
		DialTimeout:    config.DialTimeout,
		ReadTimeout:    config.ReadTimeout,
		WriteTimeout:   config.WriteTimeout,
		PoolTimeout:    config.PoolTimeout,
		RouteRandomly:  true,
		RouteByLatency: true,
	}

	// Add TLS if IAM auth is enabled
	if config.UseIAMAuth {
		clusterConfig.UseTLS = true
		clusterConfig.TLSConfig = &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
	}

	return NewRedisClusterCache(clusterConfig)
}

// newAWSElastiCacheClient creates a new AWS ElastiCache client
func newAWSElastiCacheClient(ctx context.Context, config RedisConfig) (Cache, error) {
	// Initialize the AWS ElastiCache client
	elastiCacheClient, err := aws.NewElastiCacheClient(ctx, *config.ElastiCacheConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize ElastiCache client: %w", err)
	}

	// Get Redis options from the ElastiCache client
	options, err := elastiCacheClient.BuildRedisOptions(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to build Redis options: %w", err)
	}

	// Check if we should use cluster mode
	if clusterMode, ok := options["clusterMode"].(bool); ok && clusterMode {
		// Get addresses for cluster mode
		addrs, ok := options["addrs"].([]string)
		if !ok || len(addrs) == 0 {
			return nil, fmt.Errorf("no addresses available for Redis cluster mode")
		}

		// Create cluster configuration
		clusterConfig := RedisClusterConfig{
			Addrs:          addrs,
			MaxRetries:     config.MaxRetries,
			MinIdleConns:   config.MinIdleConns,
			PoolSize:       config.PoolSize,
			DialTimeout:    config.DialTimeout,
			ReadTimeout:    config.ReadTimeout,
			WriteTimeout:   config.WriteTimeout,
			PoolTimeout:    config.PoolTimeout,
			RouteRandomly:  true,
			RouteByLatency: true,
		}

		// Add authentication if available
		if username, ok := options["username"].(string); ok {
			clusterConfig.Username = username
		}

		if password, ok := options["password"].(string); ok {
			clusterConfig.Password = password
		}

		// Add TLS if enabled
		if tlsConfig, ok := options["tls"].(*tls.Config); ok {
			clusterConfig.UseTLS = true
			clusterConfig.TLSConfig = tlsConfig
		}

		return NewRedisClusterCache(clusterConfig)
	} else {
		// Standard Redis client
		addr, ok := options["addr"].(string)
		if !ok || addr == "" {
			return nil, fmt.Errorf("no address available for Redis")
		}

		// Create Redis configuration
		redisConfig := RedisConfig{
			Address:      addr,
			MaxRetries:   config.MaxRetries,
			DialTimeout:  config.DialTimeout,
			ReadTimeout:  config.ReadTimeout,
			WriteTimeout: config.WriteTimeout,
			PoolSize:     config.PoolSize,
			MinIdleConns: config.MinIdleConns,
			PoolTimeout:  config.PoolTimeout,
		}

		// Add authentication if available
		if username, ok := options["username"].(string); ok {
			redisConfig.Username = username
		}

		if password, ok := options["password"].(string); ok {
			redisConfig.Password = password
		}

		// Add TLS if enabled
		if tlsConfig, ok := options["tls"].(*tls.Config); ok && tlsConfig != nil {
			redisConfig.UseIAMAuth = true // If TLS is present, enable it
		}

		return NewRedisCache(redisConfig)
	}
}
