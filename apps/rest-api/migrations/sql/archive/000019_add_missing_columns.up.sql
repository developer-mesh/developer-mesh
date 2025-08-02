-- Add missing columns required by functional tests

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

-- Add version column to tasks table (for optimistic locking)
ALTER TABLE tasks 
ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1;

-- Add version column to workflows table (for optimistic locking)
ALTER TABLE workflows 
ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1;

-- Add version column to workspaces table (for optimistic locking)
-- Note: workspaces already has state_version, but may need version for record versioning
ALTER TABLE workspaces 
ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1;

-- Add indexes for version columns
CREATE INDEX IF NOT EXISTS idx_tasks_version ON tasks(version);
CREATE INDEX IF NOT EXISTS idx_workflows_version ON workflows(version);
CREATE INDEX IF NOT EXISTS idx_workspaces_version ON workspaces(version);

COMMIT;