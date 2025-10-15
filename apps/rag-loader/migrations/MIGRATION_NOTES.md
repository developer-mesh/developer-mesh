# RAG Loader Migration Notes

## Step 1: Multi-Tenant Database Schema (Completed)

### Migrations Applied
- **000041_create_tenant_tables**: Created multi-tenant table structure
- **000042_add_row_level_security**: Implemented RLS policies with FORCE

### Tables Created
1. `rag.tenant_sources` - Tenant-specific source configurations
2. `rag.tenant_source_credentials` - Encrypted credentials storage
3. `rag.tenant_documents` - Document storage with embeddings support
4. `rag.tenant_sync_jobs` - Sync job tracking and metrics

### Security Features Implemented
- ✅ Row Level Security (RLS) enabled on all tenant tables
- ✅ FORCE ROW LEVEL SECURITY to prevent owner bypass
- ✅ Tenant isolation policies on all tables
- ✅ Helper functions: `rag.set_current_tenant()` and `rag.get_current_tenant()`
- ✅ Foreign key constraints for referential integrity
- ✅ Cascade deletes for data cleanup

### Critical Security Note: BYPASSRLS in Development

**Current State:**
The `devmesh` database user has `BYPASSRLS=true` (superuser privilege), which allows it to bypass Row Level Security even with `FORCE ROW LEVEL SECURITY` enabled. This is PostgreSQL's expected behavior for superusers.

**Impact:**
- ✅ RLS policies are correctly configured and will work in production
- ⚠️  In development, the superuser can see all tenant data (expected for admin access)
- ✅ Application code MUST call `rag.set_current_tenant()` before queries
- ✅ Non-superuser application users will be properly isolated

**Production Deployment Requirements:**
1. Create a dedicated application user WITHOUT superuser privileges
2. Grant only necessary permissions to the application user
3. The application user will then be properly restricted by RLS
4. Keep superuser access for admin/maintenance operations only

**Example Production Setup:**
```sql
-- Create dedicated application user (run as postgres superuser)
CREATE ROLE rag_loader_app WITH LOGIN PASSWORD 'secure_password_here';

-- Grant schema access
GRANT USAGE ON SCHEMA rag, mcp TO rag_loader_app;

-- Grant table permissions
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA rag TO rag_loader_app;
GRANT SELECT ON mcp.tenants TO rag_loader_app;

-- Grant sequence access
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA rag TO rag_loader_app;

-- Grant function execution
GRANT EXECUTE ON FUNCTION rag.set_current_tenant(UUID) TO rag_loader_app;
GRANT EXECUTE ON FUNCTION rag.get_current_tenant() TO rag_loader_app;

-- Verify no BYPASSRLS
SELECT rolname, rolsuper, rolbypassrls FROM pg_roles WHERE rolname = 'rag_loader_app';
-- Should show: rolsuper=f, rolbypassrls=f
```

### Verification Tests Completed
- ✅ All migrations run successfully (version 42)
- ✅ All tables created with correct schema
- ✅ RLS enabled on all tenant tables
- ✅ RLS FORCED on all tenant tables
- ✅ All policies created and active
- ✅ Helper functions created and accessible
- ✅ Indexes created for performance
- ✅ Triggers created for updated_at columns
- ✅ Foreign key constraints working

### Database Schema Status
```
Migration Version: 42
Tables Created: 4 tenant tables
Policies Active: 4 RLS policies
Functions: 2 (set_current_tenant, get_current_tenant)
Indexes: 15 (performance-optimized)
Triggers: 3 (auto-update timestamps)
```

### Next Steps (Step 2)
Now that the database foundation is complete, the next phase involves:
1. Implement CredentialManager service (pkg/rag/security/)
2. Implement JWT validation (apps/rag-loader/internal/auth/)
3. Implement TenantMiddleware (apps/rag-loader/internal/middleware/)
4. Implement API handlers (apps/rag-loader/internal/api/)
5. Implement Repository layer (apps/rag-loader/internal/repository/)

### Rollback Procedure
If needed, rollback with:
```bash
migrate -database "postgresql://devmesh:devmesh@localhost:5432/devmesh_development?sslmode=disable" \
  -path migrations down 2
```

This will safely remove both the RLS migration and tenant tables migration.
