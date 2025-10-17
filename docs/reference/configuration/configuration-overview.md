# Configuration Overview

## Configuration System

Developer Mesh uses a hierarchical configuration system that supports multiple environments and configuration sources.

## Configuration Hierarchy

Configuration is loaded in the following priority order (highest to lowest):

1. **Environment Variables** - Override any configuration setting
2. **Environment-specific YAML** - e.g., `config.development.yaml`, `config.production.yaml`
3. **Base Configuration** - `config.base.yaml` with default values
4. **Hardcoded Defaults** - Built into the application

## Configuration Files

### Location
All configuration files are stored in the `/configs` directory:

```
configs/
├── config.base.yaml           # Base configuration with defaults
├── config.development.yaml    # Development environment
├── config.production.yaml     # Production environment
├── config.staging.yaml        # Staging environment
├── config.test.yaml          # Test environment
├── config.docker.yaml        # Docker-specific configuration
├── config.rest-api.yaml      # REST API specific settings
├── auth.development.yaml     # Development auth configuration
└── auth.production.yaml      # Production auth configuration
```

### Loading Configuration

The system automatically loads the appropriate configuration based on the `ENVIRONMENT` variable:

```bash
# Development
ENVIRONMENT=development  # Loads config.development.yaml

# Production
ENVIRONMENT=production   # Loads config.production.yaml

# Custom config file
MCP_CONFIG_FILE=/path/to/custom/config.yaml
```

## Core Configuration Sections

### 1. API Server Configuration

```yaml
api:
  listen_address: ":8081"       # REST API port
  base_path: "/api/v1"          # API base path
  enable_swagger: true          # Swagger UI
  enable_metrics: true          # Prometheus metrics
  
  # TLS Configuration
  tls:
    enabled: false              # Enable HTTPS
    cert_file: ""               # TLS certificate
    key_file: ""                # TLS private key
    
  # CORS Settings
  cors:
    enabled: true
    allowed_origins: ["*"]      # Restrict in production
    
  # Rate Limiting
  rate_limit:
    enabled: true
    limit: 100                  # Requests per period
    period: 60s                 # Time period
```

### 2. WebSocket Configuration

```yaml
websocket:
  enabled: true
  max_connections: 10000
  ping_interval: 30s
  pong_timeout: 60s
  max_message_size: 1048576     # 1MB
  
  security:
    require_auth: true
    allowed_origins: ["*"]       # Restrict in production
```

### 3. Database Configuration

```yaml
database:
  host: localhost
  port: 5432
  name: devmesh_development
  user: devmesh
  password: ${DATABASE_PASSWORD}  # From environment variable
  ssl_mode: disable              # Use 'require' in production
  search_path: "mcp,public"
  
  # Connection pool settings
  pool:
    max_open: 100
    max_idle: 10
    max_lifetime: 1h
```

### 4. Redis Configuration

```yaml
redis:
  address: localhost:6379
  password: ${REDIS_PASSWORD}
  db: 0
  
  # Connection pool
  pool:
    size: 50
    min_idle: 10
    max_retries: 3
    
  # Redis Streams settings
  streams:
    webhook_events: "webhook_events"
    consumer_group: "webhook_workers"
    max_len: 100000              # Max stream length
    block_duration: 5s           # Block duration for XREAD
```

### 5. Authentication Configuration

```yaml
auth:
  # JWT Settings
  jwt:
    secret: ${JWT_SECRET}        # Must be set in production
    algorithm: "HS256"
    expiration: 24h
    refresh_enabled: true
    refresh_expiration: 7d
    
  # API Keys
  api_keys:
    header: "X-API-Key"
    enable_database: true        # Store keys in database
    cache_ttl: 5m
    
    # Static keys (development only)
    static_keys:
      "dev-admin-key-1234567890":
        role: "admin"
        scopes: ["read", "write", "admin"]
        tenant_id: "00000000-0000-0000-0000-000000000001"
```

### 6. AWS Configuration

```yaml
aws:
  region: ${AWS_REGION:-us-east-1}
  
  # S3 Settings
  s3:
    bucket: ${S3_BUCKET:-mcp-contexts}
    endpoint: ${S3_ENDPOINT}     # Optional custom endpoint
    
  # Bedrock Settings
  bedrock:
    enabled: true
    region: ${BEDROCK_REGION:-us-east-1}
    endpoint: ${BEDROCK_ENDPOINT} # Optional custom endpoint
```

### 7. Encryption Configuration

```yaml
encryption:
  # Single master key for all services
  master_key: ${ENCRYPTION_MASTER_KEY}
  
  # Key requirements
  min_key_length: 32             # Minimum 32 characters recommended
  algorithm: "AES-256-GCM"       # Encryption algorithm
  key_derivation: "PBKDF2"      # Per-tenant key derivation
```

## Environment-Specific Configurations

### Development Environment

- Authentication optional (`require_auth: false`)
- Swagger UI enabled
- Profiling enabled (`enable_pprof: true`)
- CORS allows all origins
- Static API keys for testing
- LocalStack for AWS services

### Production Environment

- Authentication required
- Swagger UI disabled
- Strict CORS policies
- TLS/SSL enabled
- Real AWS services
- No static API keys
- Database SSL required

### Staging Environment

- Similar to production
- Enhanced logging
- Test API keys allowed
- Performance monitoring enabled

## Configuration Validation

The system validates configuration on startup:

1. **Required Fields**: Ensures all required fields are present
2. **Type Checking**: Validates field types
3. **Range Validation**: Checks numeric ranges
4. **Format Validation**: Validates formats (URLs, emails, etc.)
5. **Dependency Checks**: Ensures dependent services are configured

## Best Practices

### 1. Security

- Never commit secrets to version control
- Use environment variables for sensitive data
- Different keys per environment
- Rotate keys regularly
- Use strong encryption keys (32+ characters)

### 2. Environment Variables

- Use descriptive names
- Document all variables
- Provide sensible defaults
- Validate at startup
- Use `.env` files locally

### 3. Configuration Files

- Keep base configuration minimal
- Override only what's needed
- Document all settings
- Use YAML anchors for repeated values
- Version control configuration templates

### 4. Development vs Production

```yaml
# Development
api:
  enable_swagger: true
  enable_pprof: true
auth:
  require_auth: false

# Production  
api:
  enable_swagger: false
  enable_pprof: false
auth:
  require_auth: true
database:
  ssl_mode: require
```

## Configuration Templates

The project includes several configuration templates:

- `config.yaml.template` - Basic template
- `config.yaml.template.dynamic` - Dynamic tools configuration
- `config.yaml.template.embedding` - Embedding service configuration

Copy and customize these templates for your environment:

```bash
cp configs/config.yaml.template configs/config.custom.yaml
# Edit config.custom.yaml
export MCP_CONFIG_FILE=configs/config.custom.yaml
```

## Monitoring Configuration

### Prometheus Configuration
Located at `configs/prometheus.yml`:
- Scrape configurations for all services
- Alert rules
- Service discovery

### Grafana Configuration
Located at `configs/grafana/`:
- Dashboard definitions
- Data source provisioning
- Alert configurations

## Redis Cluster Configuration

For production Redis cluster setup, see `configs/redis-cluster/`:
- Node configurations (redis-0.conf through redis-5.conf)
- Cluster topology settings
- Replication configuration

## Troubleshooting

### Configuration Not Loading

```bash
# Check which config file is being used
echo $MCP_CONFIG_FILE
echo $ENVIRONMENT

# Validate YAML syntax
yamllint configs/config.development.yaml

# Check for missing environment variables
grep '\${' configs/config.development.yaml
```

### Environment Variable Not Working

```bash
# Verify variable is exported
env | grep YOUR_VARIABLE

# Check if variable is referenced correctly in config
grep YOUR_VARIABLE configs/*.yaml
```

### Service-Specific Issues

```bash
# MCP Server
export LOG_LEVEL=debug
./edge-mcp 2>&1 | grep -i config

# REST API
export LOG_LEVEL=debug
./rest-api 2>&1 | grep -i config
```

## Related Documentation

- [Environment Variables Reference](../ENVIRONMENT_VARIABLES.md)
- [Encryption Key Configuration](encryption-keys.md)
- [Redis Configuration](redis-configuration.md)
- [Production Deployment Guide](../guides/production-deployment.md)