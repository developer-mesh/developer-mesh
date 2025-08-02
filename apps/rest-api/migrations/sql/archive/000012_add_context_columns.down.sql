-- Rollback: Remove added columns from contexts table
SET search_path TO mcp, public;

-- Drop indexes first
DROP INDEX IF EXISTS idx_contexts_agent_id;
DROP INDEX IF EXISTS idx_contexts_session_id;

-- Drop the columns
ALTER TABLE mcp.contexts 
DROP COLUMN IF EXISTS agent_id,
DROP COLUMN IF EXISTS model_id,
DROP COLUMN IF EXISTS session_id,
DROP COLUMN IF EXISTS current_tokens,
DROP COLUMN IF EXISTS max_tokens,
DROP COLUMN IF EXISTS expires_at;