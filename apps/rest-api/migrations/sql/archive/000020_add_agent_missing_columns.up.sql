-- Add missing columns to agents table

-- Add type column
ALTER TABLE agents 
ADD COLUMN IF NOT EXISTS type VARCHAR(50) NOT NULL DEFAULT 'standard';

-- Add status column
ALTER TABLE agents 
ADD COLUMN IF NOT EXISTS status VARCHAR(50) NOT NULL DEFAULT 'offline';

-- Add last_seen_at column
ALTER TABLE agents 
ADD COLUMN IF NOT EXISTS last_seen_at TIMESTAMP WITH TIME ZONE;

-- Add indexes for new columns
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agents_type ON agents(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_agents_last_seen_at ON agents(last_seen_at DESC) WHERE deleted_at IS NULL;

-- Update existing rows to have reasonable defaults
UPDATE agents 
SET last_seen_at = updated_at 
WHERE last_seen_at IS NULL;

-- Add comments for documentation
COMMENT ON COLUMN agents.type IS 'Agent type: standard, specialized, etc.';
COMMENT ON COLUMN agents.status IS 'Agent status: available, busy, offline, etc.';
COMMENT ON COLUMN agents.last_seen_at IS 'Last time the agent sent a heartbeat';