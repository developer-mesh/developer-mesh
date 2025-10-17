<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:56:49
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Local Development Guide

## Quick Start

```bash
# 1. Start all services
docker-compose -f docker-compose.local.yml up -d

# 2. Verify everything is working
./scripts/local/quick-check.sh

# 3. View logs
docker-compose -f docker-compose.local.yml logs -f
```

## Available Scripts

### 🔍 Quick System Check
```bash
./scripts/local/quick-check.sh
```
Performs a comprehensive health check of all services and database state.

### 🔄 Reset Database
```bash
./scripts/local/reset-db.sh
```
Completely resets the database, removes volumes, and runs fresh migrations.

### ✅ Verify Migrations
```bash
./scripts/local/verify-migrations.sh
```
Checks that all required database tables exist with detailed output.

## Common Issues & Solutions

### Issue: `pq: relation "mcp.*" does not exist`

**Symptom**: Services report missing tables even though migrations have run.

**Cause**: PostgreSQL search_path not including the `mcp` schema.

**Solution**:
1. Ensure `DATABASE_DSN` includes `search_path=mcp,public` parameter
2. Rebuild services: `docker-compose -f docker-compose.local.yml build`
3. Restart services: `docker-compose -f docker-compose.local.yml restart`

### Issue: Services Can't Connect to Database

**Symptom**: Connection refused or timeout errors.

**Solution**:
```bash
# Check if database is running
docker-compose -f docker-compose.local.yml ps database

# Check database logs
docker-compose -f docker-compose.local.yml logs database

# Restart database
docker-compose -f docker-compose.local.yml restart database
```

### Issue: Redis Memory Overcommit Warning

**Symptom**: Redis warning about memory overcommit in logs.

**Solution** (Optional - just a warning):
```bash
# On host machine (macOS/Linux)
sudo sysctl vm.overcommit_memory=1

# Or add to docker-compose.yml under redis service:
sysctls:
  - vm.overcommit_memory=1
```

### Issue: Migrations Not Running

**Symptom**: Tables don't exist even after starting services.

**Solution**:
```bash
# REST API runs migrations on startup
# Check if it started successfully
docker-compose -f docker-compose.local.yml logs rest-api | grep -i migration

# Force migration run
docker-compose -f docker-compose.local.yml restart rest-api
```

### Issue: API Keys Not Working

**Symptom**: Authentication failures with dev API keys.

**Solution**:
The default development API keys are:
- Admin: `dev-admin-key-1234567890`
- Reader: `dev-readonly-key-1234567890`
- MCP: `dev-admin-key-1234567890`

Use in requests:
```bash
curl -H "X-API-Key: dev-admin-key-1234567890" http://localhost:8081/api/v1/tools
```

## Service Endpoints

| Service | Port | Health Check | Purpose |
|---------|------|--------------|---------|
| MCP Server | 8080 | http://localhost:8080 (MCP Server)/health | WebSocket & API |
| REST API | 8081 | http://localhost:8081 (REST API)/health | REST endpoints |
| Database | 5432 | `pg_isready -h localhost -p 5432` | PostgreSQL |
| Redis | 6379 | `redis-cli ping` | Cache & Streams | <!-- Source: pkg/redis/streams_client.go -->
| MockServer | 8082 | http://localhost:8082/health | Testing |

## Database Access

```bash
# Connect to database
psql -h localhost -U devmesh -d devmesh_development
# Password: devmesh

# Or via Docker
docker-compose -f docker-compose.local.yml exec database \
  psql -U devmesh -d devmesh_development

# Common queries
\dn                    # List schemas
\dt mcp.*             # List tables in mcp schema
SELECT current_schema(); # Check current schema
```

## Redis Access

```bash
# Connect to Redis
redis-cli

# Or via Docker
docker-compose -f docker-compose.local.yml exec redis redis-cli

# Common commands
PING                   # Check connection
XINFO STREAM webhook_events  # Check webhook stream
MONITOR               # Watch all commands (debug)
```

## Debugging Tips

### Enable Verbose Logging

Add to docker-compose.local.yml environment:
```yaml
services:
  edge-mcp:
    environment:
      - LOG_LEVEL=debug
      - DEBUG_SQL=true
```

### Check Service Health with Details

```bash
# MCP Server with table checks
curl "http://localhost:8080/health?details=true"

# REST API health
curl http://localhost:8081/health
```

### Monitor All Logs

```bash
# All services
docker-compose -f docker-compose.local.yml logs -f

# Specific service
docker-compose -f docker-compose.local.yml logs -f edge-mcp

# Filter for errors
docker-compose -f docker-compose.local.yml logs -f 2>&1 | grep -i error
```

### Check Resource Usage

```bash
docker stats
```

## Rebuilding After Code Changes

```bash
# 1. Rebuild Docker images
docker-compose -f docker-compose.local.yml build

# 2. Restart services
docker-compose -f docker-compose.local.yml up -d

# 3. Verify
./scripts/local/quick-check.sh
```

## Clean Slate Reset

```bash
# Stop everything and remove volumes
docker-compose -f docker-compose.local.yml down -v

# Remove all containers and images
docker-compose -f docker-compose.local.yml down --rmi all

# Start fresh
docker-compose -f docker-compose.local.yml up -d
```

## Testing

```bash
# Run unit tests (outside Docker)
make test

# Run integration tests
make test-integration

# Test specific service
cd apps/edge-mcp && go test ./...
```

## Pre-commit Checks

Before committing:
```bash
make pre-commit
```

This runs:
- `make fmt` - Format code
- `make lint` - Check linting
- `make test` - Run tests

## Environment Variables

Key environment variables are set in `docker-compose.local.yml`. For local overrides, create a `.env` file:

```bash
# .env (git ignored)
DATABASE_HOST=localhost
DATABASE_PORT=5432
REDIS_HOST=localhost
LOG_LEVEL=debug
```

## Troubleshooting Workflow

1. **Quick Check**: `./scripts/local/quick-check.sh`
2. **Verify Tables**: `./scripts/local/verify-migrations.sh`
3. **Check Logs**: `docker-compose -f docker-compose.local.yml logs --tail=50`
4. **Reset if Needed**: `./scripts/local/reset-db.sh`
5. **Rebuild Images**: `docker-compose -f docker-compose.local.yml build`

## Known Issues

- **Task Rebalancing Error**: The error `column "deleted_at" does not exist` in task rebalancing is non-critical and doesn't affect local development
- **Redis Memory Warning**: Can be ignored for local development
- **Encryption Key Warnings**: Expected in local dev; random keys are fine

## Getting Help

- Check logs first: `docker-compose -f docker-compose.local.yml logs`
- Run diagnostics: `./scripts/local/quick-check.sh`
- Verify migrations: `./scripts/local/verify-migrations.sh`
