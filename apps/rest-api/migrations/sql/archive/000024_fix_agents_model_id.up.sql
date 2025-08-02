-- Fix agents.model_id to be VARCHAR instead of UUID
-- This aligns with the Go model and allows using well-known model IDs like "gpt-4", "claude-3-opus", etc.

BEGIN;

-- Drop the foreign key constraint first
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_model_fk;

-- Change the column type from UUID to VARCHAR
ALTER TABLE agents ALTER COLUMN model_id TYPE VARCHAR(255) USING model_id::VARCHAR;

-- Add a default value for existing records without model_id
UPDATE agents SET model_id = 'gpt-4' WHERE model_id IS NULL OR model_id = '';

-- Make the column NOT NULL
ALTER TABLE agents ALTER COLUMN model_id SET NOT NULL;

-- Add a check constraint to ensure model_id is not empty
ALTER TABLE agents ADD CONSTRAINT agents_model_id_not_empty CHECK (length(trim(model_id)) > 0);

-- Create an index on model_id for performance
CREATE INDEX IF NOT EXISTS idx_agents_model_id_varchar ON agents(model_id) WHERE deleted_at IS NULL;

COMMIT;