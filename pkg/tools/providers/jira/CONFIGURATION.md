# Jira Provider Configuration Guide

## Overview

This guide provides comprehensive documentation for configuring the Jira provider, including all available options, best practices, and deployment scenarios.

## Table of Contents

- [Configuration Methods](#configuration-methods)
- [Authentication Configuration](#authentication-configuration)
- [Provider Settings](#provider-settings)
- [Security Configuration](#security-configuration)
- [Performance Configuration](#performance-configuration)
- [Observability Configuration](#observability-configuration)
- [Advanced Configuration](#advanced-configuration)
- [Configuration Examples](#configuration-examples)
- [Best Practices](#best-practices)

## Configuration Methods

The Jira provider supports multiple configuration methods, applied in the following precedence order:

1. **Context Values** (highest priority)
2. **Environment Variables**
3. **Configuration Files**
4. **Default Values** (lowest priority)

### 1. Context-Based Configuration

Pass configuration through context for per-request customization:

```go
ctx := context.Background()

// Authentication
ctx = context.WithValue(ctx, "auth_type", "basic")
ctx = context.WithValue(ctx, "email", "user@example.com")
ctx = context.WithValue(ctx, "api_token", "ATATT3xFfGF0...")

// Provider settings
ctx = context.WithValue(ctx, "tenant_id", "tenant-123")
ctx = context.WithValue(ctx, "domain", "https://company.atlassian.net")

// Optional settings
ctx = context.WithValue(ctx, "cache_ttl", "10m")
ctx = context.WithValue(ctx, "debug_mode", true)
ctx = context.WithValue(ctx, "read_only", false)
```

### 2. Environment Variables

Configure via environment variables for containerized deployments:

```bash
# Authentication
export JIRA_DOMAIN="https://company.atlassian.net"
export JIRA_EMAIL="user@example.com"
export JIRA_API_TOKEN="ATATT3xFfGF0..."
export JIRA_AUTH_TYPE="basic"

# Cache settings
export JIRA_CACHE_ENABLED="true"
export JIRA_CACHE_TTL="5m"
export JIRA_CACHE_MAX_SIZE="100MB"

# Security settings
export JIRA_ENCRYPT_CREDENTIALS="true"
export JIRA_SANITIZE_PII="true"
export JIRA_ALLOWED_PROJECTS="PROJ1,PROJ2"

# Performance settings
export JIRA_TIMEOUT="30s"
export JIRA_MAX_RETRIES="3"
export JIRA_RATE_LIMIT="100"

# Observability
export JIRA_DEBUG_MODE="false"
export JIRA_ENABLE_METRICS="true"
export JIRA_LOG_LEVEL="info"
```

### 3. Configuration File (YAML)

```yaml
jira:
  # Core settings
  domain: https://company.atlassian.net
  tenant_id: tenant-123
  deployment_type: cloud  # cloud, server, datacenter

  # Authentication
  auth:
    type: basic  # basic, oauth, pat
    email: ${JIRA_EMAIL}
    api_token: ${JIRA_API_TOKEN}
    # OAuth settings (if type: oauth)
    client_id: ${OAUTH_CLIENT_ID}
    client_secret: ${OAUTH_CLIENT_SECRET}
    access_token: ${OAUTH_ACCESS_TOKEN}
    refresh_token: ${OAUTH_REFRESH_TOKEN}

  # Cache configuration
  cache:
    enabled: true
    ttl: 5m
    max_size: 100MB
    max_entries: 10000
    invalidation:
      on_write: true
      on_error: false
    storage:
      type: memory  # memory, redis
      redis:
        host: localhost:6379
        password: ${REDIS_PASSWORD}
        db: 0

  # Security configuration
  security:
    encrypt_credentials: true
    encryption_key: ${ENCRYPTION_KEY}
    sanitize_pii: true
    pii_patterns:
      - email
      - ssn
      - credit_card
    allowed_projects:
      - PROJ1
      - PROJ2
    denied_operations:
      - issues/delete
      - projects/delete
    audit:
      enabled: true
      log_path: /var/log/jira-audit.log

  # Performance configuration
  performance:
    timeout: 30s
    connect_timeout: 10s
    max_idle_conns: 100
    max_conns_per_host: 20
    idle_conn_timeout: 90s
    max_retries: 3
    retry_wait: 1s
    retry_max_wait: 30s
    rate_limit:
      enabled: true
      requests_per_second: 10
      burst: 20

  # Observability configuration
  observability:
    debug_mode: false
    log_level: info  # debug, info, warn, error
    metrics:
      enabled: true
      namespace: jira_provider
      labels:
        environment: production
        service: api
    tracing:
      enabled: true
      sample_rate: 0.1
    health_check:
      enabled: true
      interval: 60s
      timeout: 5s
      endpoint: /rest/api/3/myself

  # Feature flags
  features:
    batch_operations: true
    webhook_support: false
    async_processing: true
    auto_retry: true
    smart_caching: true
```

## Authentication Configuration

### Basic Authentication (API Token)

Most common for Jira Cloud:

```go
// Via context
ctx = context.WithValue(ctx, "auth_type", "basic")
ctx = context.WithValue(ctx, "email", "user@example.com")
ctx = context.WithValue(ctx, "api_token", "ATATT3xFfGF0...")

// Via environment
export JIRA_AUTH_TYPE="basic"
export JIRA_EMAIL="user@example.com"
export JIRA_API_TOKEN="ATATT3xFfGF0..."
```

### OAuth 2.0

For Jira apps and integrations:

```go
// Via context
ctx = context.WithValue(ctx, "auth_type", "oauth")
ctx = context.WithValue(ctx, "client_id", "your-client-id")
ctx = context.WithValue(ctx, "client_secret", "your-client-secret")
ctx = context.WithValue(ctx, "access_token", "access-token")
ctx = context.WithValue(ctx, "refresh_token", "refresh-token")

// Token refresh configuration
ctx = context.WithValue(ctx, "oauth_token_url", "https://auth.atlassian.com/oauth/token")
ctx = context.WithValue(ctx, "oauth_auto_refresh", true)
```

### Personal Access Token (PAT)

For Jira Server/Data Center:

```go
// Via context
ctx = context.WithValue(ctx, "auth_type", "pat")
ctx = context.WithValue(ctx, "pat_token", "personal-access-token")
ctx = context.WithValue(ctx, "pat_bearer", true) // Use Bearer instead of Basic

// Via environment
export JIRA_AUTH_TYPE="pat"
export JIRA_PAT_TOKEN="your-personal-access-token"
```

### Encrypted Credentials

Store encrypted credentials for enhanced security:

```go
// Credentials are automatically decrypted when prefixed
ctx = context.WithValue(ctx, "api_token", "encrypted:base64-encrypted-token")

// Configure encryption key
ctx = context.WithValue(ctx, "encryption_key", "32-byte-encryption-key")

// Or use system encryption service
encryptedToken, _ := encryptionService.Encrypt([]byte("plain-token"))
ctx = context.WithValue(ctx, "api_token", fmt.Sprintf("encrypted:%s", encryptedToken))
```

## Provider Settings

### Domain Configuration

```go
// Cloud instance
ctx = context.WithValue(ctx, "domain", "https://company.atlassian.net")

// Server/Data Center
ctx = context.WithValue(ctx, "domain", "https://jira.company.com:8443")

// With custom base path
ctx = context.WithValue(ctx, "domain", "https://company.com/jira")
ctx = context.WithValue(ctx, "base_path", "/jira")
```

### Deployment Type

```go
// Specify deployment type for API compatibility
ctx = context.WithValue(ctx, "deployment_type", "cloud")    // Default
ctx = context.WithValue(ctx, "deployment_type", "server")   // Server 8.x+
ctx = context.WithValue(ctx, "deployment_type", "datacenter") // Data Center
```

### Project Filtering

Limit operations to specific projects:

```go
// Single project
ctx = context.WithValue(ctx, "project_filter", "PROJ1")

// Multiple projects
ctx = context.WithValue(ctx, "project_filter", "PROJ1,PROJ2,PROJ3")

// Via environment
export JIRA_PROJECT_FILTER="PROJ1,PROJ2"
```

### Read-Only Mode

Prevent write operations:

```go
// Enable read-only mode
ctx = context.WithValue(ctx, "read_only", true)

// Check if operation is allowed
if provider.IsReadOnlyMode(ctx) && provider.IsWriteOperation(operation) {
    return fmt.Errorf("write operations not allowed in read-only mode")
}
```

## Security Configuration

### PII Detection and Sanitization

```go
// Enable PII detection
ctx = context.WithValue(ctx, "sanitize_pii", true)

// Custom PII patterns
ctx = context.WithValue(ctx, "pii_patterns", []string{
    `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`, // Email
    `\b\d{3}-\d{2}-\d{4}\b`, // SSN
    `\b4[0-9]{12}(?:[0-9]{3})?\b`, // Credit card
})

// Sanitize specific fields
ctx = context.WithValue(ctx, "sanitize_fields", []string{
    "emailAddress",
    "displayName",
    "accountId",
})
```

### Request/Response Sanitization

```go
// Headers to sanitize
ctx = context.WithValue(ctx, "sanitize_headers", []string{
    "Authorization",
    "X-Atlassian-Token",
    "Cookie",
})

// Response fields to redact
ctx = context.WithValue(ctx, "redact_fields", []string{
    "password",
    "token",
    "secret",
})
```

### Audit Configuration

```go
// Enable audit logging
ctx = context.WithValue(ctx, "audit_enabled", true)
ctx = context.WithValue(ctx, "audit_log_path", "/var/log/jira-audit.log")

// Audit specific operations
ctx = context.WithValue(ctx, "audit_operations", []string{
    "issues/delete",
    "projects/create",
    "users/update",
})

// Include request/response in audit
ctx = context.WithValue(ctx, "audit_include_payload", true)
ctx = context.WithValue(ctx, "audit_include_response", false)
```

## Performance Configuration

### Timeouts

```go
// Operation timeout
ctx = context.WithValue(ctx, "timeout", "30s")

// Connection timeout
ctx = context.WithValue(ctx, "connect_timeout", "10s")

// Idle connection timeout
ctx = context.WithValue(ctx, "idle_timeout", "90s")

// Via environment
export JIRA_TIMEOUT="30s"
export JIRA_CONNECT_TIMEOUT="10s"
```

### Connection Pooling

```go
// Connection pool settings
ctx = context.WithValue(ctx, "max_idle_conns", 100)
ctx = context.WithValue(ctx, "max_conns_per_host", 20)
ctx = context.WithValue(ctx, "max_idle_conns_per_host", 10)

// Via configuration
performance:
  max_idle_conns: 100
  max_conns_per_host: 20
  idle_conn_timeout: 90s
```

### Rate Limiting

```go
// Client-side rate limiting
ctx = context.WithValue(ctx, "rate_limit", 100) // requests per minute
ctx = context.WithValue(ctx, "rate_limit_burst", 20) // burst allowance

// Advanced rate limiting
rateLimiter := &RateLimitConfig{
    RequestsPerSecond: 10,
    Burst:            20,
    WaitTimeout:      5 * time.Second,
}
ctx = context.WithValue(ctx, "rate_limiter", rateLimiter)
```

### Retry Configuration

```go
// Basic retry settings
ctx = context.WithValue(ctx, "max_retries", 3)
ctx = context.WithValue(ctx, "retry_wait", "1s")
ctx = context.WithValue(ctx, "retry_max_wait", "30s")

// Advanced retry with backoff
retryConfig := &RetryConfig{
    MaxAttempts:  3,
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    Jitter:       0.1,
}
ctx = context.WithValue(ctx, "retry_config", retryConfig)

// Retry on specific status codes
ctx = context.WithValue(ctx, "retry_status_codes", []int{429, 502, 503, 504})
```

### Caching Configuration

```go
// Basic cache settings
ctx = context.WithValue(ctx, "cache_enabled", true)
ctx = context.WithValue(ctx, "cache_ttl", "5m")

// Advanced cache configuration
cacheConfig := &CacheConfig{
    Enabled:    true,
    TTL:        5 * time.Minute,
    MaxSize:    100 * 1024 * 1024, // 100MB
    MaxEntries: 10000,
    InvalidateOnWrite: true,
    InvalidateOnError: false,
}
ctx = context.WithValue(ctx, "cache_config", cacheConfig)

// Cache key strategy
ctx = context.WithValue(ctx, "cache_key_strategy", "url_params_auth") // or "url_only"

// Force cache refresh
ctx = context.WithValue(ctx, "force_refresh", true)

// Disable cache for request
ctx = context.WithValue(ctx, "no_cache", true)
```

## Observability Configuration

### Logging

```go
// Log level
ctx = context.WithValue(ctx, "log_level", "info") // debug, info, warn, error

// Debug mode
ctx = context.WithValue(ctx, "debug_mode", true)

// Custom logger
logger := &CustomLogger{}
ctx = context.WithValue(ctx, "logger", logger)

// Log specific operations
ctx = context.WithValue(ctx, "log_operations", []string{
    "issues/create",
    "issues/delete",
})
```

### Metrics

```go
// Enable metrics
ctx = context.WithValue(ctx, "metrics_enabled", true)
ctx = context.WithValue(ctx, "metrics_namespace", "jira_provider")

// Custom metrics labels
ctx = context.WithValue(ctx, "metrics_labels", map[string]string{
    "environment": "production",
    "service":     "api",
    "version":     "1.0.0",
})

// Metrics to collect
ctx = context.WithValue(ctx, "collect_metrics", []string{
    "request_duration",
    "request_count",
    "error_count",
    "cache_hits",
    "cache_misses",
})
```

### Tracing

```go
// Enable tracing
ctx = context.WithValue(ctx, "tracing_enabled", true)
ctx = context.WithValue(ctx, "trace_sample_rate", 0.1) // 10% sampling

// Add trace context
ctx = context.WithValue(ctx, "trace_id", "abc123")
ctx = context.WithValue(ctx, "span_id", "def456")
ctx = context.WithValue(ctx, "parent_span_id", "parent123")

// Custom trace attributes
ctx = context.WithValue(ctx, "trace_attributes", map[string]interface{}{
    "user.id":     "user-123",
    "project.key": "PROJ",
    "operation":   "issues/create",
})
```

### Health Checks

```go
// Health check configuration
healthConfig := &HealthCheckConfig{
    Enabled:  true,
    Interval: 60 * time.Second,
    Timeout:  5 * time.Second,
    Endpoint: "/rest/api/3/myself",
}
ctx = context.WithValue(ctx, "health_check_config", healthConfig)

// Custom health check function
healthFunc := func(ctx context.Context) error {
    // Custom health check logic
    return nil
}
ctx = context.WithValue(ctx, "health_check_func", healthFunc)
```

## Advanced Configuration

### Webhook Configuration

```go
// Webhook settings (coming soon)
webhookConfig := &WebhookConfig{
    Enabled:      true,
    CallbackURL:  "https://api.company.com/webhooks/jira",
    Secret:       "webhook-secret",
    Events: []string{
        "jira:issue_created",
        "jira:issue_updated",
        "jira:issue_deleted",
    },
}
ctx = context.WithValue(ctx, "webhook_config", webhookConfig)
```

### Batch Operations

```go
// Enable batch operations
ctx = context.WithValue(ctx, "batch_enabled", true)
ctx = context.WithValue(ctx, "batch_size", 50)
ctx = context.WithValue(ctx, "batch_timeout", "30s")

// Batch configuration
batchConfig := &BatchConfig{
    Enabled:         true,
    MaxBatchSize:    100,
    FlushInterval:   5 * time.Second,
    MaxConcurrency:  5,
}
ctx = context.WithValue(ctx, "batch_config", batchConfig)
```

### Circuit Breaker

```go
// Circuit breaker configuration
circuitBreaker := &CircuitBreakerConfig{
    Enabled:           true,
    Threshold:         5,     // failures before opening
    Timeout:          30 * time.Second, // time before half-open
    MaxHalfOpen:       3,     // requests in half-open state
}
ctx = context.WithValue(ctx, "circuit_breaker", circuitBreaker)
```

### Custom Middleware

```go
// Add custom middleware
middleware := func(next http.RoundTripper) http.RoundTripper {
    return &CustomTransport{
        Base: next,
        // Custom logic
    }
}
ctx = context.WithValue(ctx, "http_middleware", middleware)
```

## Configuration Examples

### Development Environment

```yaml
jira:
  domain: https://dev.atlassian.net
  auth:
    type: basic
    email: ${DEV_JIRA_EMAIL}
    api_token: ${DEV_JIRA_TOKEN}
  cache:
    enabled: true
    ttl: 1m  # Short TTL for development
  observability:
    debug_mode: true
    log_level: debug
  performance:
    timeout: 60s  # Longer timeout for debugging
    max_retries: 1
```

### Production Environment

```yaml
jira:
  domain: https://company.atlassian.net
  auth:
    type: basic
    email: ${JIRA_EMAIL}
    api_token: ${JIRA_API_TOKEN}
  cache:
    enabled: true
    ttl: 5m
    storage:
      type: redis
      redis:
        host: redis-cluster:6379
  security:
    encrypt_credentials: true
    sanitize_pii: true
    audit:
      enabled: true
  observability:
    debug_mode: false
    log_level: info
    metrics:
      enabled: true
    tracing:
      enabled: true
      sample_rate: 0.01  # 1% sampling
  performance:
    timeout: 30s
    max_retries: 3
    rate_limit:
      enabled: true
      requests_per_second: 100
```

### High-Security Environment

```yaml
jira:
  domain: https://secure.jira.internal
  auth:
    type: oauth
    client_id: ${OAUTH_CLIENT_ID}
    client_secret: ${OAUTH_CLIENT_SECRET}
  security:
    encrypt_credentials: true
    encryption_key: ${MASTER_ENCRYPTION_KEY}
    sanitize_pii: true
    allowed_projects:
      - SECURE
      - INTERNAL
    denied_operations:
      - issues/delete
      - projects/delete
      - users/create
    audit:
      enabled: true
      log_path: /secure/audit/jira.log
      include_payload: true
  observability:
    log_level: warn  # Minimal logging
    metrics:
      enabled: false  # No external metrics
```

### Multi-Tenant Configuration

```go
// Tenant-specific configuration
func configureTenant(ctx context.Context, tenantID string) context.Context {
    tenantConfig := getTenantConfig(tenantID)

    ctx = context.WithValue(ctx, "tenant_id", tenantID)
    ctx = context.WithValue(ctx, "domain", tenantConfig.Domain)
    ctx = context.WithValue(ctx, "api_token", tenantConfig.APIToken)
    ctx = context.WithValue(ctx, "project_filter", tenantConfig.AllowedProjects)
    ctx = context.WithValue(ctx, "rate_limit", tenantConfig.RateLimit)

    return ctx
}
```

## Best Practices

### 1. Security Best Practices

- **Never hardcode credentials** - Use environment variables or secure vaults
- **Enable credential encryption** for stored tokens
- **Use PII sanitization** in production environments
- **Implement audit logging** for compliance
- **Rotate API tokens** regularly
- **Use OAuth** for user-facing applications

### 2. Performance Best Practices

- **Enable caching** for read-heavy workloads
- **Set appropriate timeouts** based on network conditions
- **Use connection pooling** for concurrent requests
- **Implement rate limiting** to avoid API throttling
- **Enable circuit breakers** for resilience
- **Use batch operations** for bulk updates

### 3. Monitoring Best Practices

- **Enable metrics** in production
- **Use appropriate log levels** (info/warn in production)
- **Implement health checks** for service monitoring
- **Enable tracing** for distributed systems
- **Monitor error rates** and response times
- **Set up alerts** for critical failures

### 4. Configuration Management

- **Use environment-specific configs** (dev, staging, prod)
- **Externalize sensitive configuration**
- **Version control non-sensitive configs**
- **Document all custom configuration**
- **Validate configuration on startup**
- **Implement configuration hot-reloading** where appropriate

### 5. Error Handling Configuration

```go
// Configure error behavior
errorConfig := &ErrorConfig{
    RetryableErrors: []string{
        "rate_limit",
        "server_error",
        "timeout",
    },
    FatalErrors: []string{
        "authentication",
        "authorization",
    },
    LogErrors:      true,
    IncludeStack:   false, // Only in debug mode
}
ctx = context.WithValue(ctx, "error_config", errorConfig)
```

## Validation

### Configuration Validation

```go
// Validate configuration on startup
func validateConfig(config *JiraConfig) error {
    // Required fields
    if config.Domain == "" {
        return fmt.Errorf("domain is required")
    }

    // URL validation
    if _, err := url.Parse(config.Domain); err != nil {
        return fmt.Errorf("invalid domain URL: %w", err)
    }

    // Auth validation
    switch config.Auth.Type {
    case "basic":
        if config.Auth.Email == "" || config.Auth.APIToken == "" {
            return fmt.Errorf("email and api_token required for basic auth")
        }
    case "oauth":
        if config.Auth.AccessToken == "" {
            return fmt.Errorf("access_token required for oauth")
        }
    default:
        return fmt.Errorf("unsupported auth type: %s", config.Auth.Type)
    }

    // Performance validation
    if config.Performance.Timeout < time.Second {
        return fmt.Errorf("timeout must be at least 1 second")
    }

    return nil
}
```

### Runtime Validation

```go
// Validate at runtime
provider.ValidateCredentials(ctx, creds)
provider.HealthCheck(ctx)
```

## Troubleshooting Configuration Issues

### Common Configuration Problems

1. **Missing Environment Variables**
   - Check variable names match exactly
   - Verify variables are exported
   - Use `printenv | grep JIRA` to debug

2. **Invalid YAML Format**
   - Validate YAML syntax
   - Check indentation (spaces, not tabs)
   - Verify data types match expected

3. **Authentication Failures**
   - Verify API token format
   - Check email matches token owner
   - Ensure proper auth type configured

4. **Performance Issues**
   - Increase cache TTL
   - Enable connection pooling
   - Adjust timeout values
   - Check rate limiting settings

5. **Debugging Configuration**
   ```go
   // Print effective configuration
   config := provider.GetEffectiveConfig(ctx)
   log.Printf("Effective config: %+v", config)
   ```

## Configuration Reference

For a complete list of all configuration options, see the [API Reference](./API_REFERENCE.md).

## Support

For configuration assistance:
1. Review this guide and examples
2. Check environment variable settings
3. Validate YAML configuration
4. Enable debug mode for detailed logging
5. Contact support with configuration details