<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:28:43
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Environment Switching Guide

Developer Mesh supports multiple environment configurations to accommodate different development scenarios. This guide explains how to switch between environments effectively.

## Available Environments

### 1. Local Docker Environment
- **Purpose**: Fully isolated local development
- **Services**: All run in Docker containers
- **Dependencies**: Docker Desktop only
- **AWS Services**: Mocked with LocalStack
- **Best for**: Quick development, testing new features, CI/CD

### 2. Local AWS Environment
- **Purpose**: Local development with real AWS services
- **Services**: Run locally with `go run`
- **Dependencies**: AWS credentials, SSH access
- **AWS Services**: Real RDS, ElastiCache, S3 via SSH tunnels
- **Best for**: Testing AWS integrations, production-like environment

### 3. Production Environment
- **Purpose**: Deployed services in AWS
- **Services**: Running in ECS/EKS
- **Dependencies**: Full AWS infrastructure
- **AWS Services**: Direct connections
- **Best for**: Production deployments

## Quick Switching Commands

```bash
# Switch to Local Docker
make env-local
make local-docker

# Switch to Local AWS
make env-aws
make local-aws

# Check current environment
make env-check
```

## Detailed Switching Procedures

### Switching to Local Docker Environment

1. **Stop any running AWS connections:**
   ```bash
   make tunnel-kill
   make stop-all
   ```

2. **Configure for Docker:**
   ```bash
   make env-local
   ```
   This creates `.env.local` with Docker-specific settings.

3. **Start Docker environment:**
   ```bash
   make local-docker
   ```
   This will:
   - Start all Docker containers
   - Wait for services to be healthy
   - Seed test data
   - Configure E2E tests

4. **Verify setup:**
   ```bash
   make validate-services
   make health
   ```

### Switching to Local AWS Environment

1. **Stop Docker services:**
   ```bash
   make down
   ```

2. **Configure for AWS:**
   ```bash
   make env-aws
   ```
   This removes `.env.local` to use your main `.env` file.

3. **Verify AWS credentials:**
   ```bash
   aws sts get-caller-identity
   make env-check | grep AWS
   ```

4. **Create SSH tunnels:**
   ```bash
   make tunnel-all
   make tunnel-status
   ```

5. **Start services locally:**
   ```bash
   # Terminal 1
   make run-edge-mcp

   # Terminal 2
   make run-rest-api

   # Terminal 3
   make run-worker
   ```

6. **Verify setup:**
   ```bash
   make health
   make validate-services
   ```

## Environment Configuration Files

### `.env` (Main Configuration)
Your primary configuration file containing:
- AWS credentials
- Production endpoints
- SSH tunnel configuration
- API keys and tokens

### `.env.local` (Docker Override)
Created by `make env-local`, overrides settings for Docker:
```bash
ENVIRONMENT=local
DATABASE_HOST=database     # Docker container name
REDIS_HOST=redis          # Docker container name
USE_LOCALSTACK=true
USE_REAL_AWS=false
```

### `test/e2e/.env.local` (E2E Test Config)
E2E test configuration for local testing:
```bash
E2E_ENVIRONMENT=local
MCP_BASE_URL=http://localhost:8080
API_BASE_URL=http://localhost:8081
E2E_API_KEY=dev-admin-key-1234567890
```

## Environment Variables by Context

### Database Configuration

**Docker Environment:**
```bash
DATABASE_HOST=database
DATABASE_PORT=5432
DATABASE_USER=dev
DATABASE_PASSWORD=dev
DATABASE_NAME=dev
DATABASE_SSL_MODE=disable
```

**AWS Environment (via tunnel):**
```bash
DATABASE_HOST=localhost    # SSH tunnel endpoint
DATABASE_PORT=5432        # Local tunnel port
DATABASE_USER=devmesh
DATABASE_PASSWORD=${from_secrets}
DATABASE_NAME=devmesh_development
DATABASE_SSL_MODE=require
```

### Redis Configuration

**Docker Environment:**
```bash
REDIS_HOST=redis
REDIS_PORT=6379
REDIS_ADDR=redis:6379
# No password required
```

**AWS Environment (via tunnel):**
```bash
REDIS_HOST=localhost      # SSH tunnel endpoint
REDIS_PORT=6379          # Local tunnel port
REDIS_ADDR=localhost:6379
```

### AWS Services

**Docker Environment:**
```bash
USE_LOCALSTACK=true
USE_REAL_AWS=false
AWS_ENDPOINT_URL=http://localstack:4566
AWS_REGION=us-east-1
# Dummy credentials for LocalStack
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
```

**AWS Environment:**
```bash
USE_REAL_AWS=true
USE_LOCALSTACK=false
AWS_REGION=us-west-2
AWS_ACCESS_KEY_ID=${real_key}
AWS_SECRET_ACCESS_KEY=${real_secret}
# No endpoint URL - use real AWS
```

## Common Switching Scenarios

### Scenario 1: Start Fresh Development
```bash
# Clean slate with Docker
make docker-reset
make local-docker
make test-e2e-local
```

### Scenario 2: Test AWS Integration
```bash
# Switch from Docker to AWS
make env-aws
make down
make tunnel-all
make run-edge-mcp  # In separate terminal
make test-e2e-local
```

### Scenario 3: Debug Production Issue Locally
```bash
# Use AWS data but local code
make env-aws
make tunnel-all
# Restore production data snapshot
make run-edge-mcp  # With debugger attached
```

### Scenario 4: Switch Back to Docker
```bash
# From AWS to Docker
make tunnel-kill
make stop-all
make env-local
make local-docker
```

## Troubleshooting Environment Issues

### Services Using Wrong Database
- Check `DATABASE_HOST` in current environment
- Ensure `.env.local` exists for Docker mode
- Restart services after switching

### SSH Tunnels Not Working
```bash
# Check tunnel processes
ps aux | grep "ssh.*-L"

# Restart tunnels
make tunnel-kill
make tunnel-all
make tunnel-status
```

### Tests Failing After Switch
```bash
# Regenerate test config
rm test/e2e/.env.local
make test-e2e-setup

# Reseed test data
make seed-test-data
```

### Port Conflicts
```bash
# Find conflicting processes
lsof -i :8080
lsof -i :5432

# Kill specific port
kill -9 $(lsof -t -i :8080)
```

## Best Practices

1. **Always check current environment**: `make env-check`
2. **Clean up before switching**: `make down` or `make stop-all`
3. **Verify after switching**: `make validate-services`
4. **Keep environments isolated**: Don't mix Docker and AWS services
5. **Use appropriate data**: Docker has test data, AWS may have real data

## Environment Feature Matrix

| Feature | Docker | AWS Local | Production |
|---------|---------|-----------|------------|
| Quick Setup | ✅ Yes | ⚠️ Requires SSH | ❌ No |
| Real AWS Services | ❌ No | ✅ Yes | ✅ Yes |
| Isolation | ✅ Full | ⚠️ Partial | ❌ No |
| Cost | ✅ Free | 💰 AWS costs | 💰 AWS costs |
| Internet Required | ❌ No | ✅ Yes | ✅ Yes |
| Performance | ✅ Fast | ⚠️ Tunnel latency | ✅ Fast |
| Debugging | ✅ Easy | ✅ Easy | ❌ Hard |

## Quick Reference Card

```bash
# Current environment
make env-check

# Docker development
make env-local && make local-docker

# AWS development  
make env-aws && make local-aws

# Clean switch
make down && make tunnel-kill && make env-local && make local-docker

# Run tests
make test-e2e-local

# Check health
make health

# View logs
make logs
