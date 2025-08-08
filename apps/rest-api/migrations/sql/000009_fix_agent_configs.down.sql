-- =====================================================================
-- Rollback Fix Agent Configs Table
-- =====================================================================

-- Restore created_by to NOT NULL (will fail if any NULLs exist)
ALTER TABLE mcp.agent_configs 
ALTER COLUMN created_by SET NOT NULL;

-- Restore original trigger function
-- This would need the original function definition from before the fix
-- For now, we'll leave the fixed version in place

-- Remove the unique index
DROP INDEX IF EXISTS mcp.idx_agent_configs_agent_id;