<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:28:26
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Configuration Guide

This guide explains the Developer Mesh configuration system and best practices for managing configurations across different environments.

## Table of Contents

1. [Configuration Overview](#configuration-overview)
2. [Environment-Specific Configurations](#environment-specific-configurations)
3. [Configuration Loading](#configuration-loading)
4. [Environment Variables](#environment-variables)
5. [Security Best Practices](#security-best-practices)
6. [Configuration Reference](#configuration-reference)
7. [Migration Guide](#migration-guide)

## Configuration Overview

Developer Mesh uses a hierarchical configuration system that supports:

- **Base configuration** with sensible defaults
- **Environment-specific overrides** (development, staging, production)
- **Local overrides** for developer customization
- **Environment variable substitution**
- **Secret management integration**

### Configuration Files Structure

```
configs/
├── config.base.yaml           # Base configuration (defaults)
├── config.development.yaml    # Development environment
├── config.staging.yaml        # Staging environment
├── config.production.yaml     # Production environment
├── config.test.yaml          # Test environment
└── config.*.local.yaml       # Local overrides (git-ignored)
```

### Configuration Hierarchy

Configurations are loaded and merged in the following order:

1. `config.base.yaml` - Base defaults
2. `config.{environment}.yaml` - Environment-specific settings
3. `config.{environment}.local.yaml` - Local overrides (optional)
4. Environment variables - Highest priority

## Environment-Specific Configurations

### Development Environment

**File**: `configs/config.development.yaml`

Optimized for local development:
- Authentication can be disabled
- Static API keys for testing
- Local PostgreSQL and Redis
- Debug logging enabled
- Mock services available
- Hot reload enabled

```bash
# Start with development config
export ENVIRONMENT=development
./developer-mesh server
```

### Staging Environment

**File**: `configs/config.staging.yaml`

Similar to production with relaxations:
- Test API keys allowed
- Higher rate limits
- Full request logging
- Tracing enabled at 100%
- Chaos testing available

```bash
# Start with staging config
export ENVIRONMENT=staging
./developer-mesh server
```

### Production Environment

**File**: `configs/config.production.yaml`

Hardened for production use:
- All secrets from environment/vault
- TLS required
- Strict rate limiting
- Minimal logging
- High availability features
- Compliance features enabled

```bash
# Start with production config
export ENVIRONMENT=production
./developer-mesh server
```

### Test Environment

**File**: `configs/config.test.yaml`

Optimized for automated testing:
- In-memory databases
- No external dependencies
- Deterministic behavior
- Fast timeouts
- All mocks enabled

## Configuration Loading

### Using the Configuration Loader

```go
import "github.com/developer-mesh/developer-mesh/pkg/config"

// Load configuration
loader, err := config.LoadConfig("./configs", "production")
if err != nil {
    log.Fatal(err)
}

// Validate configuration
if err := config.ValidateConfig(loader, "production"); err != nil {
    log.Fatal(err)
}

// Access configuration values
port := loader.GetString("api.listen_address")
dbHost := loader.GetString("database.host")
```

### Configuration Inheritance

Use the `_base` directive to inherit from another configuration:

```yaml
# config.staging.yaml
_base: config.base.yaml  # Inherit from base

# Override specific values
api:
  enable_swagger: true
```

### Local Overrides

Create `config.{environment}.local.yaml` for local customization:

```yaml
# config.development.local.yaml
database:
  host: my-local-postgres
  
adapters:
  github:
    token: my-personal-token
```

## Environment Variables

### Variable Substitution

Configuration files support environment variable substitution:

```yaml
database:
  host: "${DB_HOST}"
  password: "${DB_PASSWORD}"
  port: ${DB_PORT:-5432}  # With default value
```

### Override via Environment

Any configuration value can be overridden via environment variables:

```bash
# Override api.listen_address
export API_LISTEN_ADDRESS=:8081

# Override database.host
export DATABASE_HOST=my-database.example.com

# Override nested values (dots become underscores)
export API_AUTH_JWT_SECRET=my-secret
```

### Environment Variable Files

#### Development (.env)

```bash
cp .env.example .env
# Edit .env with your values
```

#### Production (.env.production.example)

Production environments should use proper secret management:

- AWS Secrets Manager
- HashiCorp Vault
- Kubernetes Secrets
- Azure Key Vault

## Security Best Practices

### 1. Never Commit Secrets

```yaml
# BAD - Never do this
auth:
  jwt:
    secret: "actual-secret-value"

# GOOD - Use environment variables
auth:
  jwt:
    secret: "${JWT_SECRET}"
```

### 2. Use Secret Management

Production configuration for AWS Secrets Manager:

```yaml
security:
  secrets:
    provider: "aws-secrets-manager"
    aws_region: "${AWS_REGION}"
    key_prefix: "developer-mesh/"
```

### 3. Rotate Secrets Regularly

```bash
# Rotate JWT secret
aws secretsmanager put-secret-value \
  --secret-id developer-mesh/jwt-secret \
  --secret-string "$(openssl rand -base64 32)"
```

### 4. Restrict Configuration Access

```bash
# Set proper file permissions
chmod 600 configs/config.production.local.yaml

# Use RBAC for Kubernetes ConfigMaps
kubectl create role config-reader \
  --verb=get,list \
  --resource=configmaps
```

### 5. Audit Configuration Changes

```yaml
# Enable configuration audit
audit:
  config_changes:
    enabled: true
    storage: "database"
    retention_days: 90
```

## Configuration Reference

### Core Configuration Sections

#### API Configuration

```yaml
api:
  listen_address: ":8080"      # Server listen address
  base_path: "/api/v1"         # API base path
  enable_swagger: true         # Enable Swagger UI
  enable_pprof: false          # Enable profiling
  
  # Timeouts
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  
  # TLS
  tls:
    enabled: true
    cert_file: "/path/to/cert"
    key_file: "/path/to/key"
```

#### Authentication Configuration

```yaml
auth:
  # JWT settings
  jwt:
    secret: "${JWT_SECRET}"
    expiration: 24h
    algorithm: "HS256"
    
  # API key settings
  api_keys:
    enable_database: true
    min_length: 16
    rotation_period: 90d
    
  # Rate limiting
  rate_limiting:
    enabled: true
    default:
      max_attempts: 100
      window: 1m
```

#### Database Configuration

```yaml
database:
  driver: "postgres"
  host: "${DB_HOST}"
  port: 5432
  username: "${DB_USER}"
  password: "${DB_PASSWORD}"
  database: "${DB_NAME}"
  
  # Connection pool
  max_open_conns: 100
  max_idle_conns: 25
  conn_max_lifetime: 30m
  
  # SSL/TLS
  ssl_mode: "require"
  
  # Vector extension (pgvector)
  vector:
    enabled: true
    index_type: "ivfflat"
    lists: 100
    probes: 10
```

#### Cache Configuration

```yaml
cache:
  # Local cache
  local:
    enabled: true
    size: 10000
    ttl: 60s
    
  # Redis configuration (actual)
  type: "redis"
  address: "${REDIS_ADDR:-localhost:6379}"
  password: "${REDIS_PASSWORD}"
  database: 0
  pool_size: 50
  
  # TLS
  tls:
    enabled: ${CACHE_TLS_ENABLED:-false}
    insecure_skip_verify: false
```

## Embedding Configuration

The embedding system requires at least one provider to be configured:

### OpenAI Configuration
```yaml
embedding:
  providers:
    openai:
      enabled: true
      api_key: ${OPENAI_API_KEY}
```

### AWS Bedrock Configuration
```yaml
embedding:
  providers:
    bedrock:
      enabled: true
      region: us-east-1
      # Uses standard AWS credential chain
```

### Google AI Configuration
```yaml
embedding:
  providers:
    google:
      enabled: true
      api_key: ${GOOGLE_API_KEY}
```

### Agent Configuration Example
```yaml
# Create via API
POST /api/embeddings/agents
{
  "agent_id": "production-claude",
  "embedding_strategy": "quality",
  "model_preferences": [
    {
      "task_type": "general_qa",
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    },
    {
      "task_type": "code_analysis",
      "primary_models": ["voyage-code-2"],
      "fallback_models": ["text-embedding-3-large"]
    }
  ],
  "constraints": {
    "max_cost_per_month_usd": 500.0
  }
}
```

## Docker Deployment Configuration

### Using Pre-built Images

When deploying with pre-built Docker images from GitHub Container Registry:

#### Docker Compose Configuration

```yaml
# docker-compose.prod.yml
services:
  mcp-server:
    image: ghcr.io/${GITHUB_USERNAME}/developer-mesh-mcp-server:${VERSION:-latest}
    environment:
      MCP_CONFIG_FILE: /app/configs/config.docker.yaml
      DATABASE_HOST: database
      DATABASE_PORT: 5432
      DATABASE_USER: ${DATABASE_USER}
      DATABASE_PASSWORD: ${DATABASE_PASSWORD}
      # ... other environment variables
```

#### Environment File (.env)

Create a `.env` file for Docker Compose:

```bash
# Registry configuration
GITHUB_USERNAME=your-github-username
VERSION=v1.2.3  # Or 'latest' for latest stable

# Database configuration
DATABASE_USER=mcp
DATABASE_PASSWORD=secure-password
DATABASE_NAME=devops_mcp
DATABASE_SSL_MODE=disable  # Use 'require' in production

# Redis configuration
REDIS_ADDR=redis:6379

# Security
JWT_SECRET=your-jwt-secret
ADMIN_API_KEY=your-admin-api-key

# AWS (optional)
AWS_REGION=us-east-1
AWS_ACCESS_KEY_ID=your-access-key
AWS_SECRET_ACCESS_KEY=your-secret-key
S3_BUCKET=mcp-storage

# GitHub integration
GITHUB_TOKEN=your-github-token
GITHUB_APP_ID=your-app-id
GITHUB_APP_PRIVATE_KEY=your-private-key
GITHUB_WEBHOOK_SECRET=your-webhook-secret
```

#### Kubernetes ConfigMap

For Kubernetes deployments:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: mcp-config
  namespace: mcp-prod
data:
  config.yaml: |
    api:
      listen_address: ":8080"
      base_path: "/api/v1"
    
    database:
      host: postgres-service
      port: 5432
      database: devops_mcp
      
    cache:
      type: redis
      address: redis-service:6379
```

#### Secrets Management

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mcp-secrets
  namespace: mcp-prod
type: Opaque
data:
  database-password: <base64-encoded-password>
  jwt-secret: <base64-encoded-secret>
  github-token: <base64-encoded-token>
```

### Image Version Selection

Choose the appropriate image tag:

- **latest**: Latest stable release
- **v1.2.3**: Specific version (recommended for production)
- **main-abc1234**: Specific commit
- **pr-123**: Pull request build

## Migration Guide

### From Old Configuration Format

If migrating from the old configuration format:

1. **Identify environment-specific values**
   ```yaml
   # Old: Single config with conditionals
   database:
     host: ${DB_HOST:-localhost}
   
   # New: Separate configs
   # config.development.yaml
   database:
     host: localhost
   
   # config.production.yaml
   database:
     host: "${DB_HOST}"
   ```

2. **Extract common values to base**
   ```yaml
   # config.base.yaml
   api:
     base_path: "/api/v1"  # Same across environments
   ```

3. **Move secrets to environment variables**
   ```yaml
   # Remove from config
   auth:
     jwt:
       secret: "hardcoded-secret"
   
   # Use environment variable
   auth:
     jwt:
       secret: "${JWT_SECRET}"
   ```

4. **Update deployment scripts**
   ```bash
   # Old
   ./developer-mesh --config config.yaml
   
   # New
   export ENVIRONMENT=production
   ./developer-mesh server
   ```

### Configuration Validation

Use the built-in validation:

```go
// Validate before starting
if err := config.ValidateConfig(loader, environment); err != nil {
    log.Fatalf("Invalid configuration: %v", err)
}
```

### Health Checks

Configure health checks to verify configuration:

```yaml
monitoring:
  health:
    checks:
      - name: "config"
        critical: true
        check: "validate_required_vars"
```

## Troubleshooting

### Common Issues

1. **Missing environment variables**
   ```
   Error: missing required configuration fields: [auth.jwt.secret]
   Solution: Ensure JWT_SECRET is set in environment
   ```

2. **Configuration not loading**
   ```
   Error: failed to load environment config
   Solution: Check file exists and has correct syntax
   ```

3. **Variable substitution not working**
   ```
   Symptom: Seeing ${VAR} in config values
   Solution: Ensure variable is exported in environment
   ```

### Debug Configuration Loading

```bash
# Enable debug logging
export LOG_LEVEL=debug

# Show loaded configuration
./developer-mesh config show

# Validate configuration
./developer-mesh config validate
```

## Performance Configuration

### Performance Tuning Settings

Configure application performance characteristics:

```yaml
performance:
  # Goroutine settings
  go:
    max_procs: 0  # 0 = use all CPUs
    gc_percent: 100  # GC aggressiveness

  # HTTP server performance
  http:
    max_connections: 10000
    max_concurrent_streams: 1000
    enable_http2: true
    enable_compression: true
    compression_level: 6
    
  # Connection pooling
  pools:
    database:
      max_open: 100
      max_idle: 25
      lifetime: 30m
    redis:
      pool_size: 200
      min_idle: 10
      
  # Worker pools
  workers:
    pool_size: 50
    queue_size: 1000
    prefetch_count: 10
```

### Caching Configuration (Actual Implementation)

Developer Mesh implements multi-level caching with L1 (in-memory) and L2 (Redis):

```yaml
cache:
  # Multi-level cache configuration
  multilevel:
    l1_size: 10000          # L1 LRU cache size
    l1_ttl: 300s           # L1 TTL
    l2_ttl: 3600s          # L2 (Redis) TTL
    compression_threshold: 1024  # Compress values > 1KB
    prefetch_threshold: 0.8      # Prefetch when 80% through TTL
    
  # Redis configuration
  type: "redis"
  address: "${REDIS_ADDR:-localhost:6379}"
  pool_size: 50
```

**Note**: S3 caching and cache warming are not implemented.

### Resource Limits

Prevent resource exhaustion:

```yaml
limits:
  # Request limits
  request:
    max_size: "50MB"
    timeout: 30s
    max_headers: 100
    
  # Rate limiting
  rate:
    enabled: true
    default_rps: 100
    burst: 200
    
  # Memory limits
  memory:
    heap_limit: "4GB"
    stack_size: "10MB"
    
  # Concurrent operations
  concurrency:
    max_websocket_connections: 10000 <!-- Source: pkg/models/websocket/binary.go -->
    max_db_connections: 500
    max_parallel_tasks: 100
```

## Cost Management Configuration

### Cost Control Settings

Manage and limit costs across services:

```yaml
cost_management:
  # Global limits
  limits:
    daily_limit: 100.00
    monthly_limit: 3000.00
    session_limit: 1.00
    
  # Service-specific limits
  services:
    bedrock:
      enabled: true
      session_limit: 0.10
      daily_limit: 50.00
      models:
        - name: "claude-3-opus"
          max_cost_per_day: 20.00
        - name: "claude-3-sonnet"
          max_cost_per_day: 10.00
          
  # Cost tracking
  tracking:
    enabled: true
    interval: 1m
    storage: "database"
    retention_days: 90
    
  # Alerts
  alerts:
    enabled: true
    thresholds:
      - level: "warning"
        percent: 80
      - level: "critical"
        percent: 95
```

### Model Selection Strategy

Configure how models are selected based on cost and performance:

```yaml
ai_models:
  # Selection strategy
  selection:
    strategy: "balanced"  # cheapest, balanced, performance
    
  # Model preferences
  preferences:
    embeddings:
      primary: "amazon.titan-embed-text-v1"
      fallback: ["text-embedding-3-small"]
      
    inference:
      simple_tasks: "claude-3-haiku"
      complex_tasks: "claude-3-sonnet"
      critical_tasks: "claude-3-opus"
      
  # Cost optimization
  optimization:
    cache_embeddings: true
    batch_requests: true
    use_cheaper_models_off_peak: true
    off_peak_hours: "22:00-06:00"
```

### Resource Optimization

Configure resource usage for cost efficiency:

```yaml
resource_optimization:
  # Auto-scaling
  scaling:
    enabled: true
    min_instances: 1
    max_instances: 10
    target_cpu: 70
    scale_down_cooldown: 300s
    
  # Scheduled scaling
  schedules:
    - name: "business_hours"
      schedule: "0 8 * * MON-FRI"
      min_instances: 3
      max_instances: 20
      
    - name: "off_hours"
      schedule: "0 18 * * MON-FRI"
      min_instances: 1
      max_instances: 5
      
    - name: "weekend"
      schedule: "0 0 * * SAT,SUN"
      min_instances: 0
      max_instances: 3
      
  # Spot instances
  spot:
    enabled: true
    max_price: 0.05
    percentage: 80
```

## Monitoring Configuration

### Metrics and Observability

Configure comprehensive monitoring:

```yaml
monitoring:
  # Metrics
  metrics:
    enabled: true
    provider: "prometheus"
    interval: 60s
    detailed: true
    
    # Custom metrics
    custom:
      - name: "cost_per_request"
        type: "histogram"
      - name: "model_latency"
        type: "summary"
        
  # Tracing
  tracing:
    enabled: true
    provider: "jaeger"
    sampling_rate: 0.01  # 1% in production
    
    # Span attributes
    attributes:
      - "user_id"
      - "agent_id"
      - "model_name"
      - "cost"
      
  # Logging
  logging:
    level: "info"
    format: "json"
    sampling:
      initial: 100
      thereafter: 100
```

### Health Checks

Configure comprehensive health monitoring:

```yaml
health:
  # Liveness probe
  liveness:
    endpoint: "/health/live"
    interval: 10s
    timeout: 5s
    
  # Readiness probe
  readiness:
    endpoint: "/health/ready"
    interval: 5s
    timeout: 3s
    checks:
      - database
      - redis
      - s3
      
  # Startup probe
  startup:
    endpoint: "/health/startup"
    initial_delay: 10s
    period: 5s
    failure_threshold: 30
```

## Environment-Specific Performance Tuning

### Development Performance Config

```yaml
# config.development.yaml
performance:
  go:
    gc_percent: 100  # Default GC
  http:
    max_connections: 100
  pools:
    database:
      max_open: 10
      
cost_management:
  limits:
    daily_limit: 1.00  # $1 limit for dev
```

### Production Performance Config

```yaml
# config.production.yaml
performance:
  go:
    max_procs: 0  # Use all CPUs
    gc_percent: 200  # Less frequent GC
  http:
    max_connections: 50000
    enable_http2: true
  pools:
    database:
      max_open: 500
      max_idle: 100
      
cost_management:
  limits:
    daily_limit: 5000.00
    alerts:
      enabled: true
      channels: ["slack", "pagerduty"]
```

## Configuration Scenarios

For detailed configuration examples for different use cases, see:
- [Configuration Scenarios Guide](../guides/configuration-scenarios.md)

### Quick Reference

- **Small Team**: Focus on simplicity and cost
- **Enterprise**: High availability and compliance
- **High Performance**: Maximum speed, cost secondary
- **Cost Optimized**: Minimum cost, acceptable performance
- **Multi-Region**: Global deployment with data residency

## Performance Monitoring

Monitor configuration effectiveness:

```yaml
performance_monitoring:
  # Track config changes impact
  track_changes: true
  
  # Baseline metrics
  baselines:
    response_time_p99: 100ms
    throughput_rps: 1000
    error_rate: 0.01
    
  # Alert on degradation
  alerts:
    response_time_increase: 50%
    throughput_decrease: 30%
    error_rate_increase: 100%
```

## Cost Monitoring

Track cost impact of configuration:

```yaml
cost_monitoring:
  # Budget tracking
  budgets:
    - name: "ai_models"
      monthly_limit: 1000.00
      alert_threshold: 80%
      
    - name: "infrastructure"
      monthly_limit: 2000.00
      alert_threshold: 90%
      
  # Cost attribution
  tags:
    required:
      - environment
      - team
      - project
      - cost_center
```

## Best Practices Summary

1. **Use hierarchical configuration** - Base → Environment → Local
2. **Never commit secrets** - Use environment variables or secret managers
3. **Validate configuration** - Fail fast on missing required values
4. **Document all settings** - Include descriptions and examples
5. **Use consistent naming** - Follow the established patterns
6. **Implement health checks** - Verify configuration at runtime
7. **Audit configuration changes** - Track who changed what and when
8. **Test configuration changes** - Especially for production
9. **Use feature flags** - For gradual rollouts
10. **Monitor configuration drift** - Ensure deployed config matches expected
11. **Optimize for your use case** - Balance performance vs cost
12. **Set resource limits** - Prevent runaway costs and resource exhaustion
13. **Enable monitoring** - Track the impact of configuration changes
14. **Use caching strategically** - Reduce costs and improve performance
