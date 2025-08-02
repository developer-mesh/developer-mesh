-- Revert workflow steps array migration
SET search_path TO mcp, public;

-- Drop the trigger and function
DROP TRIGGER IF EXISTS validate_workflow_steps_trigger ON mcp.workflows;
DROP FUNCTION IF EXISTS validate_workflow_steps();

-- Drop the array constraint
ALTER TABLE mcp.workflows 
DROP CONSTRAINT IF EXISTS workflows_steps_must_be_array;

-- Re-add the original constraint
ALTER TABLE mcp.workflows 
ADD CONSTRAINT valid_steps_jsonb 
CHECK (jsonb_typeof(steps) = 'array');

-- Note: We don't revert the data transformation as that would be destructive