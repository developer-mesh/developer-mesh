-- Remove task delegation tracking columns
ALTER TABLE tasks DROP COLUMN IF EXISTS delegation_count;
ALTER TABLE tasks DROP COLUMN IF EXISTS max_delegations;
ALTER TABLE tasks DROP COLUMN IF EXISTS auto_escalate;
ALTER TABLE tasks DROP COLUMN IF EXISTS escalation_timeout;

-- Remove workspace quota tracking columns
ALTER TABLE workspaces DROP COLUMN IF EXISTS max_members;
ALTER TABLE workspaces DROP COLUMN IF EXISTS max_storage_bytes;
ALTER TABLE workspaces DROP COLUMN IF EXISTS current_storage_bytes;
ALTER TABLE workspaces DROP COLUMN IF EXISTS max_documents;
ALTER TABLE workspaces DROP COLUMN IF EXISTS current_documents;

-- Drop tables in reverse order
DROP TABLE IF EXISTS task_idempotency_keys;
DROP TABLE IF EXISTS task_state_transitions;
DROP TABLE IF EXISTS task_delegation_history;
DROP TABLE IF EXISTS workspace_activities;
DROP TABLE IF EXISTS workspace_members;