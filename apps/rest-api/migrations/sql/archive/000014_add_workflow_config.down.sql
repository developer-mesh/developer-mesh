-- Rollback: Remove config column from workflows table
SET search_path TO mcp, public;

-- Drop index first
DROP INDEX IF EXISTS idx_workflows_config;

-- Drop the column
ALTER TABLE mcp.workflows 
DROP COLUMN IF EXISTS config;