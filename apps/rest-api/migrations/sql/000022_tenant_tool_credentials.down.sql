-- Rollback migration: 000026_tenant_tool_credentials.down.sql
-- Reverses the tenant tool credentials and Edge MCP registrations tables

BEGIN;

-- Drop indexes first
DROP INDEX IF EXISTS mcp.idx_edge_mcp_active;
DROP INDEX IF EXISTS mcp.idx_tenant_tool_credentials_lookup;

-- Drop tables (in reverse order of creation due to foreign keys)
DROP TABLE IF EXISTS mcp.edge_mcp_registrations;
DROP TABLE IF EXISTS mcp.tenant_tool_credentials;

COMMIT;
