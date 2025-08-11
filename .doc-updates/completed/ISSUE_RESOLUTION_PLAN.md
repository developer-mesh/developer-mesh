<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:56:34
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Issue Resolution Plan - DevOps MCP

## Priority P0 - Critical Service Startup Issues

### 1. Fix Service Startup Dependencies
**Problem**: Services try to access tables before migrations complete
**Root Cause**: All services start simultaneously when database is healthy, but REST API runs migrations AFTER startup

**Solution Options**:
1. **Option A (Recommended)**: Add initialization container for migrations
2. **Option B**: Add startup delay to MCP Server and Worker
3. **Option C**: Create health check that verifies migrations completed

**Implementation (Option A)**:
```yaml
# docker-compose.local.yml
  migration-runner:
    build:
      context: .
      dockerfile: apps/rest-api/Dockerfile
    command: ["/app/rest-api", "--migrate"]
    volumes:
      - ./apps/rest-api/migrations:/app/migrations:ro
    depends_on:
      database:
        condition: service_healthy
    environment:
      DATABASE_DSN: postgres://devmesh:devmesh@database:5432/devmesh_development?sslmode=disable&search_path=mcp,public

  mcp-server:
    depends_on:
      migration-runner:
        condition: service_completed_successfully
      redis:
        condition: service_healthy
        
  worker:
    depends_on:
      migration-runner:
        condition: service_completed_successfully
      redis:
        condition: service_healthy
```

### 2. Fix Worker DLQ Schema References
**File**: `apps/worker/internal/worker/dlq_handler.go`
**Changes Required**: Add `mcp.` prefix to all queries

```go
// Line 109: INSERT query
"INSERT INTO mcp.webhook_dlq ("

// Line 169: SELECT query
"FROM mcp.webhook_dlq"

// Line 205: SELECT for retry
"FROM mcp.webhook_dlq"

// Line 226: UPDATE for retry
"UPDATE mcp.webhook_dlq"

// Line 284: UPDATE status
"UPDATE mcp.webhook_dlq SET"
```

## Priority P1 - Schema/Code Mismatches

### 3. Fix Document Repository Deleted_At Issue
**Problem**: Code expects `deleted_at` column in `shared_documents` table that doesn't exist
**File**: `pkg/repository/postgres/document_repository.go`
**Issue**: All queries reference `deleted_at` column which is not in the current schema

**Solution Options**:
1. **Option A**: Add `deleted_at` column to schema via migration (RECOMMENDED)
2. **Option B**: Remove `deleted_at` references from code (requires refactoring soft delete logic)

**Implementation (Option A - Add column via migration)**:
Create new migration file: `000004_add_deleted_at_columns.up.sql`
```sql
-- Add deleted_at column to shared_documents for soft deletes
ALTER TABLE mcp.shared_documents 
ADD COLUMN IF NOT EXISTS deleted_at TIMESTAMP WITH TIME ZONE;

-- Add index for performance
CREATE INDEX IF NOT EXISTS idx_shared_documents_deleted_at 
ON mcp.shared_documents(deleted_at) 
WHERE deleted_at IS NULL;
```

### 4. Fix Missing Schema Prefixes
**Check all repository files for missing `mcp.` prefix**:
- `pkg/services/task_service_enhanced.go:691` - `task_delegation_history` → `mcp.task_delegation_history`
- `pkg/services/workspace_member_service.go:496` - `workspace_members` → `mcp.workspace_members`

## Priority P2 - Configuration Issues

### 5. Set Proper Encryption Keys
**Files**: `docker-compose.local.yml`
**Add to environment**:
```yaml
environment:
  - ENCRYPTION_KEY=${ENCRYPTION_KEY:-dev_encryption_key_32_chars_long}
  - ENCRYPTION_MASTER_KEY=${ENCRYPTION_MASTER_KEY:-dev_master_key_32_chars_long}
  - DEVMESH_ENCRYPTION_KEY=${DEVMESH_ENCRYPTION_KEY:-dev_mesh_key_32_chars_long}
```

**Create `.env.local`**:
```bash
ENCRYPTION_KEY=your_32_character_encryption_key_here
ENCRYPTION_MASTER_KEY=your_32_character_master_key_here
DEVMESH_ENCRYPTION_KEY=your_32_character_mesh_key_here
```

## Priority P3 - Non-Critical Improvements

### 6. Redis Memory Overcommit (Optional)
**Solution**: Add to `docker-compose.local.yml` under redis service:
```yaml
redis:
  sysctls:
    - vm.overcommit_memory=1
```

Or document as known issue for local development.

## Implementation Checklist

### Immediate Actions (P0)
- [ ] Create migration runner service in docker-compose
- [ ] Update service dependencies to wait for migrations
- [ ] Fix all `webhook_dlq` queries to use `mcp.webhook_dlq`
- [ ] Test service startup sequence

### Short-term Actions (P1)
- [ ] Audit all SQL queries for missing schema prefixes
- [ ] Fix document repository deleted_at references
- [ ] Add schema prefix to task and workspace queries
- [ ] Run full integration test suite

### Configuration Actions (P2)
- [ ] Create `.env.local` with proper encryption keys
- [ ] Update docker-compose to use env file
- [ ] Document key generation process
- [ ] Add key rotation documentation

### Testing & Validation
- [ ] Clean restart: `docker-compose down -v && docker-compose up`
- [ ] Verify no "relation does not exist" errors
- [ ] Check all services reach healthy state
- [ ] Test webhook processing through worker
- [ ] Validate task operations work correctly

## Additional Issues Found

### 7. Task Rebalancing Deleted_At Error
**Problem**: Task rebalancing fails with `column "deleted_at" does not exist`
**Investigation Needed**: Check if tasks table also needs deleted_at column or if code needs fixing

### 8. Initial Statement Preparation Failures
**Problem**: Services try to prepare statements before migrations run
**Solution**: Services should handle initial preparation failures gracefully and retry after migrations

## Code Files to Modify

1. **apps/worker/internal/worker/dlq_handler.go**
   - Lines: 109, 169, 205, 226, 284
   - Add `mcp.` prefix to all webhook_dlq references

2. **apps/rest-api/migrations/sql/000004_add_deleted_at_columns.up.sql** (NEW)
   - Create migration to add deleted_at column to shared_documents

3. **pkg/services/task_service_enhanced.go**
   - Line 691: Add `mcp.` prefix

4. **pkg/services/workspace_member_service.go**
   - Line 496: Add `mcp.` prefix

5. **docker-compose.local.yml**
   - Add migration-runner service
   - Update service dependencies
   - Add encryption key environment variables

## Monitoring After Fix

Watch for these in logs after implementing fixes:
- No "relation does not exist" errors
- All services report healthy
- Worker processes webhooks successfully
- No encryption key warnings
- Task operations complete without errors

## Rollback Plan

If issues persist after fixes:
1. Revert code changes
2. Use REST API's auto-migration feature
3. Add manual delays in docker-compose with `command: sh -c "sleep 30 && /app/binary"`
4. Document as known issue for local development

## Success Criteria

- [ ] All services start without database errors
- [ ] Worker processes DLQ entries successfully
- [ ] No missing table/column errors in logs
- [ ] Health checks pass for all services
