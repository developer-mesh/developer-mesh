# Environment Variables Reference

This document provides a comprehensive reference for all environment variables used in the DevOps MCP platform.

## Table of Contents
- [Core Configuration](#core-configuration)
- [Database Configuration](#database-configuration)
- [Redis Configuration](#redis-configuration)
- [Authentication & Security](#authentication--security)
- [AWS Configuration](#aws-configuration)
- [Service-Specific Variables](#service-specific-variables)
- [Docker Compose Overrides](#docker-compose-overrides)

## Core Configuration

### General Settings
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `ENVIRONMENT` | Environment name (development, staging, production) | `development` | No | All |
| `LOG_LEVEL` | Logging level (debug, info, warn, error) | `info` | No | All |
| `MCP_CONFIG_FILE` | Path to configuration file | `/app/configs/config.docker.yaml` | No | All |

### Service Addresses
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `MCP_SERVER_URL` | MCP Server endpoint | `http://localhost:8080` | Yes | REST API, Worker |
| `REST_API_URL` | REST API endpoint | `http://localhost:8081` | Yes | MCP Server, Worker |
| `API_HOST` | API bind address | `0.0.0.0` | No | All APIs |
| `API_PORT` | API port | `8080` | No | All APIs |

## Database Configuration

### PostgreSQL Settings
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `DATABASE_HOST` | PostgreSQL host | `localhost` | Yes | All |
| `DATABASE_PORT` | PostgreSQL port | `5432` | No | All |
| `DATABASE_NAME` | Database name | `devmesh_development` | Yes | All |
| `DATABASE_USER` | Database username | `devmesh` | Yes | All |
| `DATABASE_PASSWORD` | Database password | - | Yes | All |
| `DATABASE_SSL_MODE` | SSL mode (disable, require, verify-full) | `disable` | No | All |
| `DATABASE_DSN` | Full connection string (overrides individual settings) | - | No | All |
| `DATABASE_SEARCH_PATH` | PostgreSQL search path | `mcp,public` | No | All |

### Alias Variables (Legacy Support)
These variables are aliases for the above and work identically:
- `DB_HOST` → `DATABASE_HOST`
- `DB_PORT` → `DATABASE_PORT`
- `DB_NAME` → `DATABASE_NAME`
- `DB_USER` → `DATABASE_USER`
- `DB_PASSWORD` → `DATABASE_PASSWORD`
- `DB_SSLMODE` → `DATABASE_SSL_MODE`

### PostgreSQL (Docker)
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `POSTGRES_USER` | PostgreSQL superuser | `devmesh` | Yes | Database |
| `POSTGRES_PASSWORD` | PostgreSQL password | `devmesh` | Yes | Database |
| `POSTGRES_DB` | Default database | `devmesh_development` | Yes | Database |

## Redis Configuration

### Redis Connection
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `REDIS_ADDR` | Redis address (host:port) | `localhost:6379` | Yes | All |
| `REDIS_HOST` | Redis hostname | `localhost` | No | All |
| `REDIS_PORT` | Redis port | `6379` | No | All |
| `REDIS_PASSWORD` | Redis password | - | No | All |
| `REDIS_ADDRESS` | Alias for REDIS_ADDR | - | No | All |

### ElastiCache (AWS)
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `ELASTICACHE_ENDPOINT` | ElastiCache endpoint | - | No | All |
| `ELASTICACHE_PORT` | ElastiCache port | `6379` | No | All |
| `ELASTICACHE_AUTH_TOKEN` | ElastiCache auth token | - | No | All |

## Authentication & Security

### Encryption Keys
**IMPORTANT**: All encryption keys must be at least 32 characters long for AES-256 encryption.

| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `CREDENTIAL_ENCRYPTION_KEY` | Key for encrypting tool credentials | - | Yes* | REST API, Worker |
| `DEVMESH_ENCRYPTION_KEY` | REST API encryption key | - | Yes* | REST API |
| `ENCRYPTION_MASTER_KEY` | MCP Server encryption key | - | Yes* | MCP Server |
| `ENCRYPTION_KEY` | Legacy encryption key | - | No | All |

*Required for production. Development environments will generate temporary keys with warnings.

### API Keys
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `ADMIN_API_KEY` | Admin API key | `dev-admin-key-1234567890` | Yes | All |
| `READER_API_KEY` | Read-only API key | `dev-readonly-key-1234567890` | No | REST API |
| `MCP_API_KEY` | MCP service API key | Same as ADMIN_API_KEY | Yes | MCP Server |
| `API_KEY` | Generic API key (testing) | - | No | Tests |

### JWT Configuration
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `JWT_SECRET` | JWT signing secret | - | Yes* | All APIs |
| `JWT_EXPIRATION` | JWT token expiration | `24h` | No | All APIs |

### GitHub Authentication
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `GITHUB_ACCESS_TOKEN` | GitHub personal access token | - | No | All |
| `GITHUB_WEBHOOK_SECRET` | GitHub webhook validation secret | - | No | REST API |

## AWS Configuration

### General AWS Settings
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `AWS_REGION` | AWS region | `us-east-1` | Yes | All |
| `AWS_ACCESS_KEY_ID` | AWS access key | - | Yes* | All |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | - | Yes* | All |
| `AWS_ENDPOINT_URL` | Custom endpoint (LocalStack) | - | No | All |

*Required when using real AWS services. Not needed for LocalStack.

### S3 Configuration
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `S3_BUCKET` | S3 bucket name | `mcp-contexts` | Yes | MCP Server |
| `S3_ENDPOINT` | S3 endpoint override | - | No | MCP Server |
| `S3_REGION` | S3 region (if different) | AWS_REGION | No | MCP Server |

### Bedrock Configuration
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `BEDROCK_ENABLED` | Enable AWS Bedrock | `true` | No | MCP Server |
| `BEDROCK_REGION` | Bedrock region | AWS_REGION | No | MCP Server |
| `BEDROCK_ENDPOINT` | Bedrock endpoint override | - | No | MCP Server |

### LocalStack Configuration
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `USE_LOCALSTACK` | Use LocalStack instead of AWS | `false` | No | All |
| `USE_REAL_AWS` | Explicitly use real AWS | `true` | No | All |
| `LOCALSTACK_ENDPOINT` | LocalStack endpoint | `http://localstack:4566` | No | All |

## Service-Specific Variables

### MCP Server
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `MCP_WEBHOOK_ENABLED` | Enable webhook endpoints | `true` | No | MCP Server |
| `WS_MAX_CONNECTIONS` | Max WebSocket connections | `10000` | No | MCP Server |
| `WS_ALLOWED_ORIGINS` | WebSocket allowed origins | `*` | No | MCP Server |

### REST API
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `ENABLE_SWAGGER` | Enable Swagger UI | `true` | No | REST API |
| `RATE_LIMIT_ENABLED` | Enable rate limiting | `true` | No | REST API |
| `RATE_LIMIT_RPS` | Requests per second | `100` | No | REST API |

### Worker
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `WORKER_CONSUMER_NAME` | Redis consumer name | `worker-1` | Yes | Worker |
| `WORKER_IDEMPOTENCY_TTL` | Idempotency key TTL | `24h` | No | Worker |
| `WORKER_BATCH_SIZE` | Event batch size | `100` | No | Worker |
| `HEALTH_ENDPOINT` | Health check port | `:8088` | No | Worker |

### Embedding Services
| Variable | Description | Default | Required | Services |
|----------|-------------|---------|----------|----------|
| `OPENAI_API_KEY` | OpenAI API key | - | No | REST API |
| `OPENAI_ENABLED` | Enable OpenAI provider | `false` | No | REST API |
| `GOOGLE_AI_API_KEY` | Google AI API key | - | No | REST API |
| `GOOGLE_AI_ENABLED` | Enable Google AI provider | `false` | No | REST API |

## Docker Compose Overrides

### Development Overrides (.env.local)
```bash
# Local Docker environment
ENVIRONMENT=local
DATABASE_HOST=database
REDIS_ADDR=redis:6379
USE_LOCALSTACK=true
USE_REAL_AWS=false
AWS_ENDPOINT_URL=http://localstack:4566
```

### Production Overrides (.env.production)
```bash
# Production environment
ENVIRONMENT=production
LOG_LEVEL=warn
DATABASE_SSL_MODE=require
USE_LOCALSTACK=false
USE_REAL_AWS=true
ENABLE_SWAGGER=false
```

## SSH Tunnel Variables (Development)

For local development against AWS resources:

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `SSH_KEY_PATH` | Path to SSH key | `~/.ssh/bastion-key.pem` | Yes |
| `NAT_INSTANCE_IP` | NAT/Bastion IP | - | Yes |
| `RDS_ENDPOINT` | RDS endpoint for tunnel | - | Yes |
| `ELASTICACHE_ENDPOINT` | ElastiCache endpoint | - | Yes |

## Best Practices

### Security
1. **Never commit secrets** - Use `.env` files locally, environment variables in production
2. **Rotate keys regularly** - Especially encryption keys and API keys
3. **Use strong keys** - Minimum 32 characters for encryption keys
4. **Separate by environment** - Different keys for dev/staging/prod

### Configuration Hierarchy
1. Environment variables (highest priority)
2. `.env.local` file (local overrides)
3. `.env` file (base configuration)
4. Configuration files (lowest priority)

### Required vs Optional
- **Production**: All security-related variables are required
- **Development**: Many variables have safe defaults
- **Testing**: Minimal configuration needed with defaults

### Naming Conventions
- Use `UPPERCASE_WITH_UNDERSCORES`
- Prefix with service name for service-specific vars
- Use consistent names across services
- Provide aliases for backward compatibility

## Troubleshooting

### Missing Required Variables
```bash
# Check all environment variables
env | grep -E "(DATABASE|REDIS|AWS|API)" | sort

# Validate configuration
make validate-env
```

### Connection Issues
```bash
# Test database connection
psql $DATABASE_DSN -c "SELECT 1"

# Test Redis connection
redis-cli -h $REDIS_HOST -p $REDIS_PORT ping
```

### Encryption Key Issues
```bash
# Generate new encryption keys
./scripts/generate-encryption-keys.sh

# Test encryption
echo "test" | openssl enc -aes-256-cbc -base64 -pass pass:$ENCRYPTION_KEY
```

## Examples

### Minimal Development Setup
```bash
export DATABASE_PASSWORD=devmesh
export ADMIN_API_KEY=dev-key
export JWT_SECRET=dev-secret
```

### Full Production Setup
```bash
# Security
export CREDENTIAL_ENCRYPTION_KEY=$(openssl rand -base64 32)
export DEVMESH_ENCRYPTION_KEY=$(openssl rand -base64 32)
export ENCRYPTION_MASTER_KEY=$(openssl rand -base64 32)
export JWT_SECRET=$(openssl rand -base64 32)

# Database
export DATABASE_HOST=prod-db.region.rds.amazonaws.com
export DATABASE_PASSWORD=secure-password
export DATABASE_SSL_MODE=verify-full

# AWS
export AWS_REGION=us-east-1
export S3_BUCKET=prod-mcp-contexts

# APIs
export ADMIN_API_KEY=prod-admin-key
export GITHUB_WEBHOOK_SECRET=webhook-secret
```

## Migration Guide

### From Legacy Variables
```bash
# Old → New
DB_HOST → DATABASE_HOST
DB_PASS → DATABASE_PASSWORD
REDIS_URL → REDIS_ADDR
S3_BUCKET_NAME → S3_BUCKET
```

### From Hardcoded Values
1. Extract all hardcoded values to environment variables
2. Update configuration files to reference variables
3. Document new variables in this guide
4. Update deployment scripts