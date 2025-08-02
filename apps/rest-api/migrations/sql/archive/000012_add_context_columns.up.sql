-- Add missing columns to contexts table for MCP server compatibility

BEGIN;

SET search_path TO mcp, public;

-- Add columns that the MCP server expects
ALTER TABLE mcp.contexts 
ADD COLUMN IF NOT EXISTS agent_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS model_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS session_id VARCHAR(255),
ADD COLUMN IF NOT EXISTS current_tokens INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS max_tokens INTEGER DEFAULT 4000,
ADD COLUMN IF NOT EXISTS expires_at TIMESTAMP WITH TIME ZONE;

-- Create indexes for the new columns
CREATE INDEX IF NOT EXISTS idx_contexts_agent_id ON mcp.contexts(agent_id);
CREATE INDEX IF NOT EXISTS idx_contexts_session_id ON mcp.contexts(session_id);

-- Update the contexts table to ensure agent_id is not null for new records
-- For existing records, we'll set a default value
UPDATE mcp.contexts 
SET agent_id = 'system' 
WHERE agent_id IS NULL;

-- Now make agent_id NOT NULL
ALTER TABLE mcp.contexts 
ALTER COLUMN agent_id SET NOT NULL;

-- Also ensure model_id has a value
UPDATE mcp.contexts 
SET model_id = 'default' 
WHERE model_id IS NULL;

ALTER TABLE mcp.contexts 
ALTER COLUMN model_id SET NOT NULL;

COMMIT;