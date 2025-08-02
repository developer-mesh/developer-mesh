-- Add missing agents column to workflows table for MCP server compatibility

BEGIN;

SET search_path TO mcp, public;

-- Add agents column to workflows table
ALTER TABLE mcp.workflows 
ADD COLUMN IF NOT EXISTS agents JSONB DEFAULT '[]';

-- Create index for agents column
CREATE INDEX IF NOT EXISTS idx_workflows_agents ON mcp.workflows USING gin(agents);

COMMIT;

-- Update existing workflows to have empty agents array
UPDATE mcp.workflows 
SET agents = '[]' 
WHERE agents IS NULL;