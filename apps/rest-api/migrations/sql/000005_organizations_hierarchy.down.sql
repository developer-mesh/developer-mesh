-- Claude Code: Rollback organization hierarchy tables
BEGIN;

DROP TABLE IF EXISTS mcp.tenant_access_matrix;
DROP TABLE IF EXISTS mcp.organization_tenants;
DROP TABLE IF EXISTS mcp.organizations;

COMMIT;