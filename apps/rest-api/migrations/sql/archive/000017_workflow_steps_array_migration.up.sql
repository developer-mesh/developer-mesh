-- Migration to convert workflow steps from object format to array format

BEGIN;

SET search_path TO mcp, public;

-- First, update any existing workflows that have steps stored as an object with a "steps" key
UPDATE mcp.workflows 
SET steps = steps->'steps'
WHERE jsonb_typeof(steps) = 'object' 
  AND steps ? 'steps'
  AND jsonb_typeof(steps->'steps') = 'array';

-- Convert any remaining object-type steps to empty arrays (for templates or special cases)
UPDATE mcp.workflows 
SET steps = '[]'::jsonb
WHERE jsonb_typeof(steps) = 'object';

-- Ensure all steps are now arrays
UPDATE mcp.workflows 
SET steps = '[]'::jsonb
WHERE steps IS NULL;

-- Drop the old constraint if it exists
ALTER TABLE mcp.workflows 
DROP CONSTRAINT IF EXISTS valid_steps_jsonb;

-- Drop any other step validation constraints
ALTER TABLE mcp.workflows 
DROP CONSTRAINT IF EXISTS valid_workflow_steps;

-- Add a new constraint that ensures steps is always an array
ALTER TABLE mcp.workflows 
ADD CONSTRAINT workflows_steps_must_be_array 
CHECK (jsonb_typeof(steps) = 'array');

-- Add validation to ensure each step has required fields
CREATE OR REPLACE FUNCTION validate_workflow_steps() RETURNS trigger AS $$
BEGIN
  -- Check if steps is an array
  IF jsonb_typeof(NEW.steps) != 'array' THEN
    RAISE EXCEPTION 'Workflow steps must be an array';
  END IF;
  
  -- Validate each step in the array
  IF NEW.steps IS NOT NULL AND jsonb_array_length(NEW.steps) > 0 THEN
    DECLARE
      step jsonb;
      i int;
    BEGIN
      FOR i IN 0..jsonb_array_length(NEW.steps)-1 LOOP
        step := NEW.steps->i;
        
        -- Check required fields
        IF NOT (step ? 'id' AND step ? 'name' AND step ? 'type') THEN
          RAISE EXCEPTION 'Each workflow step must have id, name, and type fields';
        END IF;
        
        -- Validate ID is not empty
        IF length(step->>'id') = 0 THEN
          RAISE EXCEPTION 'Step ID cannot be empty';
        END IF;
        
        -- Validate name is not empty
        IF length(step->>'name') = 0 THEN
          RAISE EXCEPTION 'Step name cannot be empty';
        END IF;
        
        -- Validate type is not empty
        IF length(step->>'type') = 0 THEN
          RAISE EXCEPTION 'Step type cannot be empty';
        END IF;
      END LOOP;
    END;
  END IF;
  
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for validation
DROP TRIGGER IF EXISTS validate_workflow_steps_trigger ON mcp.workflows;
CREATE TRIGGER validate_workflow_steps_trigger
BEFORE INSERT OR UPDATE ON mcp.workflows
FOR EACH ROW
EXECUTE FUNCTION validate_workflow_steps();

-- Create an index on steps for better query performance
DROP INDEX IF EXISTS idx_workflows_steps;
CREATE INDEX idx_workflows_steps ON mcp.workflows USING gin(steps);

COMMIT;