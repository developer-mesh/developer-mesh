-- Migration: Rollback MCP Tenants Table
-- This migration removes the mcp.tenants table

-- Drop trigger
DROP TRIGGER IF EXISTS update_tenants_updated_at ON mcp.tenants;

-- Drop function
DROP FUNCTION IF EXISTS mcp.update_tenants_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_tenants_organization;
DROP INDEX IF EXISTS mcp.idx_tenants_active;
DROP INDEX IF EXISTS mcp.idx_tenants_name;

-- Drop table
DROP TABLE IF EXISTS mcp.tenants;
