<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:38:41
Verification Script: update-docs-parallel.sh
Batch: ad
-->

# Service Initialization Pattern

## Overview

This document describes the proper service initialization pattern for DevOps MCP that ensures services start reliably without race conditions or "relation does not exist" errors.

## Architecture

The system uses a **distributed initialization pattern** where:
1. REST API runs database migrations on startup
2. Other services wait for tables to be ready before starting
3. Health checks accurately reflect service readiness

## Key Components

### 1. Database Readiness Checker (`pkg/database/readiness.go`)

Provides table verification and waiting logic:
- `TablesExist()`: Checks if all required tables exist
- `WaitForTablesWithBackoff()`: Waits with exponential backoff for tables
- `HealthCheck()`: Performs comprehensive database health check

### 2. Migration Status Tracking (`apps/rest-api/internal/api/migration_status.go`)

Tracks migration progress globally:
- `SetInProgress()`: Marks migrations as running
- `SetCompleted()`: Marks migrations as successful
- `IsReady()`: Returns true when migrations are complete
- `GetStatus()`: Returns detailed migration status

### 3. Service Initialization with Retry

All services implement exponential backoff when connecting to the database:

```go
// Example from MCP Server
for i := 0; i < maxRetries; i++ {
    db, err = database.NewDatabase(ctx, dbConfig)
    if err == nil && db.Ping() == nil {
        break // Success!
    }
    
    delay := baseDelay * (1 << uint(i)) // Exponential backoff
    logger.Info("Database connection failed, retrying...", map[string]interface{}{
        "attempt": i + 1,
        "delay": delay.String(),
    })
    time.Sleep(delay)
}

// Wait for tables to be ready
readinessChecker := database.NewReadinessChecker(db.GetDB())
if err := readinessChecker.WaitForTablesWithBackoff(ctx); err != nil {
    return fmt.Errorf("database tables not ready: %w", err)
}
```

## Service Startup Sequence

1. **Database starts** (PostgreSQL with pgvector)
2. **Redis starts** (for caching and message queuing)
3. **REST API starts**:
   - Connects to database
   - Runs migrations if `AutoMigrate=true`
   - Tracks migration status
   - Reports ready only after migrations complete
4. **Other services start** (MCP Server, Worker):
   - Wait for REST API to be healthy
   - Connect to database with retry logic
   - Wait for required tables to exist
   - Start processing

## Health Check States

### REST API Health Endpoints

- `/health` - Comprehensive health status including migration state
- `/healthz` - Simple liveness check (is the process alive?)
- `/readyz` - Readiness check (are migrations complete and service ready?)
- `/admin/migration-status` - Detailed migration status

### Health Response Example

```json
{
  "status": "healthy",
  "ready": true,
  "time": "2024-01-15T10:30:00Z",
  "migrations": {
    "in_progress": false,
    "completed": true,
    "ready": true,
    "version": "latest",
    "completed_at": "2024-01-15T10:29:45Z",
    "duration": "2.5s"
  },
  "components": {
    "database": "healthy",
    "redis": "healthy"
  }
}
```

## Docker Compose Configuration

```yaml
services:
  rest-api:
    build:
      context: .
      dockerfile: apps/rest-api/Dockerfile
    healthcheck:
      test: ["CMD", "/app/rest-api", "--health-check"]
      interval: 5s
      timeout: 3s
      retries: 20
      start_period: 30s
    depends_on:
      database:
        condition: service_healthy
      redis:
        condition: service_healthy

  mcp-server:
    depends_on:
      rest-api:
        condition: service_healthy  # Wait for REST API (and migrations)
      database:
        condition: service_healthy
      redis:
        condition: service_healthy

  worker:
    depends_on:
      rest-api:
        condition: service_healthy  # Wait for REST API (and migrations)
      database:
        condition: service_healthy
      redis:
        condition: service_healthy
```

## Configuration

### Environment Variables

```bash
# Skip migrations (for debugging)
SKIP_MIGRATIONS=true

# Fail fast on migration errors
MIGRATIONS_FAIL_FAST=true

# Database search path (important!)
DATABASE_SEARCH_PATH=mcp,public
```

### Database Configuration

All services must include the search_path in their connection string:
```
postgres://user:pass@host:5432/db?sslmode=disable&search_path=mcp,public
```

## Retry Behavior

### Connection Retry
- **Max attempts**: 10
- **Backoff**: 1s, 2s, 4s, 8s, 16s, 30s (capped)
- **Total timeout**: ~2 minutes

### Table Readiness
- **Check interval**: 2 seconds
- **Timeout**: 120 seconds
- **Logs missing tables** for debugging

### Statement Preparation
- **Max retries**: 5
- **Backoff**: 100ms, 200ms, 400ms, 800ms, 1.6s
- **Non-fatal**: Services continue even if preparation fails

## Troubleshooting

### Common Issues

1. **"relation does not exist" errors**
   - Check REST API logs for migration status
   - Verify DATABASE_SEARCH_PATH includes "mcp,public"
   - Check /admin/migration-status endpoint

2. **Services stuck waiting**
   - Check REST API health: `curl http://localhost:8081 (REST API)/readyz`
   - Check migration status: `curl http://localhost:8081 (REST API)/admin/migration-status`
   - Look for migration errors in REST API logs

3. **Migrations not running**
   - Ensure SKIP_MIGRATIONS is not set
   - Check REST API has migrations directory mounted
   - Verify database permissions for schema creation

### Debug Commands

```bash
# Check service health
curl http://localhost:8080/health  # MCP Server
curl http://localhost:8081/health  # REST API

# Check migration status
curl http://localhost:8081/admin/migration-status

# View service logs
docker-compose logs -f rest-api
docker-compose logs -f mcp-server
docker-compose logs -f worker

# Check database tables
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT table_name FROM information_schema.tables WHERE table_schema='mcp'"

# Force service restart
docker-compose restart rest-api
```

## Best Practices

1. **Always use search_path** in database connections
2. **Log retry attempts** with clear messages
3. **Set reasonable timeouts** (not too short, not infinite)
4. **Use health checks** properly in orchestrators
5. **Monitor migration duration** for performance issues
6. **Test with artificial delays** to verify retry logic

## Migration Management

### Running Migrations Manually

```bash
# Run migrations and exit
docker-compose run --rm rest-api /app/rest-api --migrate

# Skip migrations on startup
SKIP_MIGRATIONS=true docker-compose up
```

### Adding New Migrations

1. Create migration file: `apps/rest-api/migrations/sql/00000X_description.up.sql`
2. Add rollback: `apps/rest-api/migrations/sql/00000X_description.down.sql`
3. Test locally with clean database
4. Verify all services handle new tables

## Production Considerations

### Kubernetes

Use init containers for migrations:

```yaml
initContainers:
- name: migrate
  image: your-app:latest
  command: ["/app/rest-api", "--migrate"]
  env:
    - name: DATABASE_URL
      valueFrom:
        secretKeyRef:
          name: database-secret
          key: url
```

### High Availability

- Run migrations from a single instance
- Use leader election for migration runner
- Implement migration locking in database
- Monitor migration performance

### Rollback Strategy

1. Keep rollback scripts (.down.sql files)
2. Test rollbacks in staging
3. Have database backups before major migrations
4. Use feature flags for schema changes

## Security Notes

- Never log database passwords
- Use IAM authentication in production
- Encrypt sensitive migration data
- Audit migration execution
- Restrict migration permissions

## Performance Tips

- Index new columns appropriately
- Run ANALYZE after large migrations
- Consider online schema changes for large tables
- Monitor query performance after migrations

## Conclusion

This initialization pattern ensures:
- ✅ No race conditions during startup
- ✅ Clear error messages for debugging
- ✅ Resilient to temporary failures
- ✅ Observable through health endpoints
- ✅ Works in all environments (local, Docker, Kubernetes)
