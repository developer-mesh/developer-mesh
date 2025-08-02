-- Remove columns added in up migration

-- Drop indexes first
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_type;
DROP INDEX IF EXISTS idx_agents_last_seen_at;

-- Drop columns
ALTER TABLE agents 
DROP COLUMN IF EXISTS type,
DROP COLUMN IF EXISTS status,
DROP COLUMN IF EXISTS last_seen_at;