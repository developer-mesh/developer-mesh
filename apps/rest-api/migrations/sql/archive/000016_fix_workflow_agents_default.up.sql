-- Fix the default value for agents column to match check constraint

BEGIN;

SET search_path TO mcp, public;

-- Update all existing workflows to have agents as an object instead of array
UPDATE mcp.workflows 
SET agents = '{}'::jsonb 
WHERE jsonb_typeof(agents) = 'array' OR agents = '[]'::jsonb;

-- Alter the default to be an object
ALTER TABLE mcp.workflows 
ALTER COLUMN agents SET DEFAULT '{}'::jsonb;

COMMIT;