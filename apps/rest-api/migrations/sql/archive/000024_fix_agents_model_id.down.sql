-- Revert agents.model_id back to UUID
-- Note: This is a destructive operation as we lose the string model IDs

BEGIN;

-- Remove the check constraint
ALTER TABLE agents DROP CONSTRAINT IF EXISTS agents_model_id_not_empty;

-- Drop the varchar index
DROP INDEX IF EXISTS idx_agents_model_id_varchar;

-- Change back to UUID (this will fail if there are non-UUID values)
-- First set to NULL for all rows to avoid conversion errors
ALTER TABLE agents ALTER COLUMN model_id DROP NOT NULL;
UPDATE agents SET model_id = NULL;

-- Change column type back to UUID
ALTER TABLE agents ALTER COLUMN model_id TYPE UUID USING NULL::UUID;

-- Re-add the foreign key constraint (assuming models table still exists)
ALTER TABLE agents ADD CONSTRAINT agents_model_fk FOREIGN KEY (model_id) REFERENCES models(id) ON DELETE RESTRICT;

COMMIT;