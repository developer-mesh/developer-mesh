package aws

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	securitytls "github.com/developer-mesh/developer-mesh/pkg/security/tls"
)

// ElastiCacheConfig holds configuration for ElastiCache
type ElastiCacheConfig struct {
	AuthConfig         AuthConfig          `mapstructure:"auth"`
	PrimaryEndpoint    string              `mapstructure:"primary_endpoint"`
	Port               int                 `mapstructure:"port"`
	Username           string              `mapstructure:"username"` // For Redis auth
	Password           string              `mapstructure:"password"` // For Redis auth
	UseIAMAuth         bool                `mapstructure:"use_iam_auth"`
	ClusterMode        bool                `mapstructure:"cluster_mode"`
	ReaderEndpoint     string              `mapstructure:"reader_endpoint"`   // Used for cluster mode
	CacheNodes         []string            `mapstructure:"cache_nodes"`       // List of nodes for cluster mode
	ClusterDiscovery   bool                `mapstructure:"cluster_discovery"` // Use API to discover nodes
	ClusterName        string              `mapstructure:"cluster_name"`
	TLS                *securitytls.Config `mapstructure:"tls"` // TLS configuration
	MaxRetries         int                 `mapstructure:"max_retries"`
	MinIdleConnections int                 `mapstructure:"min_idle_connections"`
	PoolSize           int                 `mapstructure:"pool_size"`
	DialTimeout        time.Duration       `mapstructure:"dial_timeout"`
	ReadTimeout        time.Duration       `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration       `mapstructure:"write_timeout"`
	PoolTimeout        time.Duration       `mapstructure:"pool_timeout"`
	TokenExpiration    int                 `mapstructure:"token_expiration"`
}

// ElastiCacheClient is a client for AWS ElastiCache
type ElastiCacheClient struct {
	config ElastiCacheConfig
}

// NewElastiCacheClient creates a new ElastiCache client
func NewElastiCacheClient(ctx context.Context, cfg ElastiCacheConfig) (*ElastiCacheClient, error) {
	// Always use IAM authentication if enabled, and pass AssumeRole if set
	if cfg.UseIAMAuth {
		_, err := GetAWSConfig(ctx, cfg.AuthConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to load AWS config with IAM auth: %w", err)
		}
	}
	return &ElastiCacheClient{
		config: cfg,
	}, nil
}

// GetAuthToken generates a temporary IAM auth token for ElastiCache
// To implement IAM auth for ElastiCache in production, you'll need to:
// 1. Import the AWS SDK ElastiCache package: github.com/aws/aws-sdk-go-v2/service/elasticache
// 2. Use the API to generate or authenticate with IAM
func (c *ElastiCacheClient) GetAuthToken(ctx context.Context) (string, error) {
	// For testing in local development environments
	if c.config.PrimaryEndpoint == "localhost" || c.config.PrimaryEndpoint == "127.0.0.1" ||
		c.config.PrimaryEndpoint == "redis" || c.config.PrimaryEndpoint == "" ||
		strings.HasPrefix(c.config.PrimaryEndpoint, "host.docker.internal") {
		log.Println("Using mock ElastiCache auth token for local development")
		return "mock-auth-token-local", nil
	}

	// Get AWS configuration - just validate it works
	_, err := GetAWSConfig(ctx, c.config.AuthConfig)
	if err != nil {
		return "", fmt.Errorf("failed to get AWS config: %w", err)
	}

	// For now, this is a placeholder implementation
	// In a production environment, this would make AWS SDK calls
	log.Println("Using mock ElastiCache auth token for IAM authentication")

	// If IAM auth fails and no password is configured, return an error
	if c.config.Password == "" && !c.config.UseIAMAuth {
		return "", fmt.Errorf("no authentication method available: IAM auth disabled and no password configured")
	}

	if !c.config.UseIAMAuth {
		// If IAM auth is disabled, use the password
		return c.config.Password, nil
	}

	// Return a mock token (in production, this would be a real IAM token)
	return "iam-auth-token", nil
}

// DiscoverClusterNodes discovers all nodes in a Redis cluster
func (c *ElastiCacheClient) DiscoverClusterNodes(ctx context.Context) ([]string, error) {
	// Mock implementation for testing
	return []string{"localhost:6379"}, nil
}

// BuildRedisOptions builds options for connecting to Redis
func (c *ElastiCacheClient) BuildRedisOptions(ctx context.Context) (map[string]any, error) {
	options := make(map[string]any)

	// Determine if we're using cluster mode
	if c.config.ClusterMode {
		// In cluster mode, we need a list of nodes
		var nodes []string

		// If cluster discovery is enabled, use the AWS API to discover nodes
		if c.config.ClusterDiscovery {
			discoveredNodes, err := c.DiscoverClusterNodes(ctx)
			if err != nil {
				return nil, err
			}
			nodes = discoveredNodes
		} else if len(c.config.CacheNodes) > 0 {
			// Use explicitly configured nodes
			nodes = c.config.CacheNodes
		} else if c.config.PrimaryEndpoint != "" {
			// Use primary endpoint as fallback
			primaryAddr := fmt.Sprintf("%s:%d", c.config.PrimaryEndpoint, c.config.Port)
			nodes = []string{primaryAddr}

			// If we have a reader endpoint, add it too
			if c.config.ReaderEndpoint != "" {
				readerAddr := fmt.Sprintf("%s:%d", c.config.ReaderEndpoint, c.config.Port)
				nodes = append(nodes, readerAddr)
			}
		} else {
			return nil, fmt.Errorf("no nodes available for cluster mode - either configure cache_nodes, enable cluster_discovery, or provide primary_endpoint")
		}

		options["clusterMode"] = true
		options["addrs"] = nodes
	} else {
		// Standard mode - just use the primary endpoint
		if c.config.PrimaryEndpoint == "" {
			return nil, fmt.Errorf("primary endpoint is required for non-cluster mode")
		}

		options["addr"] = fmt.Sprintf("%s:%d", c.config.PrimaryEndpoint, c.config.Port)
		options["clusterMode"] = false
	}

	// Authentication - prioritize IAM auth but have a fallback
	if c.config.UseIAMAuth {
		token, err := c.GetAuthToken(ctx)
		if err != nil {
			log.Printf("Warning: Failed to get IAM auth token for ElastiCache: %v", err)

			// If IAM auth fails and no password is configured, return an error
			if c.config.Password == "" {
				return nil, fmt.Errorf("IAM authentication failed and no password fallback is configured: %w", err)
			}

			// Otherwise fall back to password auth
			log.Println("Falling back to password authentication for ElastiCache")
			if c.config.Username != "" {
				options["username"] = c.config.Username
			}
			options["password"] = c.config.Password
		} else {
			// Use the IAM token
			options["username"] = c.config.Username
			options["password"] = token
		}
	} else if c.config.Password != "" {
		// IAM auth is disabled, use standard password auth if available
		if c.config.Username != "" {
			options["username"] = c.config.Username
		}
		options["password"] = c.config.Password
	} else {
		// No authentication configured - this may be okay for dev/test environments
		log.Println("Warning: No authentication configured for ElastiCache")
	}

	// TLS configuration
	if c.config.TLS != nil && c.config.TLS.Enabled {
		// Use TLS configuration
		tlsConfig, err := c.config.TLS.BuildTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("failed to build TLS config: %w", err)
		}
		options["tls"] = tlsConfig
	}

	// Connection pool settings
	options["poolSize"] = c.config.PoolSize
	options["minIdleConns"] = c.config.MinIdleConnections
	options["maxRetries"] = c.config.MaxRetries
	options["dialTimeout"] = c.config.DialTimeout
	options["readTimeout"] = c.config.ReadTimeout
	options["writeTimeout"] = c.config.WriteTimeout
	options["poolTimeout"] = c.config.PoolTimeout

	return options, nil
}

// GetClusters gets information about ElastiCache Redis clusters
func (c *ElastiCacheClient) GetClusters(ctx context.Context) ([]any, error) {
	// Mock implementation for testing
	return []any{}, nil
}

// ParseRedisURL parses a Redis URL into separate components
func ParseRedisURL(url string) (host string, port int, err error) {
	// Remove protocol prefix if present
	url = strings.TrimPrefix(url, "redis://")
	url = strings.TrimPrefix(url, "rediss://")

	// Split host and port
	parts := strings.Split(url, ":")
	if len(parts) != 2 {
		return "", 0, fmt.Errorf("invalid Redis URL format: %s", url)
	}

	host = parts[0]
	if _, err := fmt.Sscanf(parts[1], "%d", &port); err != nil {
		// If parsing fails, use default port
		port = 6379
	}

	if port == 0 {
		port = 6379 // Default Redis port
	}

	return host, port, nil
}
