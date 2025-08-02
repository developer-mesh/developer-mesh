-- Rollback: Set agents default back to array (though this would violate the constraint)
SET search_path TO mcp, public;

-- This is just for completeness, though it would break the constraint
ALTER TABLE mcp.workflows 
ALTER COLUMN agents SET DEFAULT '[]'::jsonb;