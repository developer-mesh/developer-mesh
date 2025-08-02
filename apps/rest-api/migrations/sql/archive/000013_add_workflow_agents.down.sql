-- Rollback: Remove agents column from workflows table
SET search_path TO mcp, public;

-- Drop index first
DROP INDEX IF EXISTS idx_workflows_agents;

-- Drop the column
ALTER TABLE mcp.workflows 
DROP COLUMN IF EXISTS agents;