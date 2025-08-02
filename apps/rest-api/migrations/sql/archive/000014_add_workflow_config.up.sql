-- Add missing config column to workflows table for MCP server compatibility

BEGIN;

SET search_path TO mcp, public;

-- Add config column to workflows table
ALTER TABLE mcp.workflows 
ADD COLUMN IF NOT EXISTS config JSONB DEFAULT '{}';

-- Create index for config column
CREATE INDEX IF NOT EXISTS idx_workflows_config ON mcp.workflows USING gin(config);

COMMIT;

-- Update existing workflows to have empty config
UPDATE mcp.workflows 
SET config = '{}' 
WHERE config IS NULL;