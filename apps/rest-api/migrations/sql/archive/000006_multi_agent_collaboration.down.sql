-- Rollback: Multi-Agent Collaboration Schema

-- Drop all tables in reverse order of creation
DROP TABLE IF EXISTS conflict_resolutions CASCADE;
DROP TABLE IF EXISTS audit_log CASCADE;
DROP TABLE IF EXISTS document_operations CASCADE;
DROP TABLE IF EXISTS shared_documents CASCADE;
DROP TABLE IF EXISTS workspace_members CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;
DROP TABLE IF EXISTS workflow_executions CASCADE;
DROP TABLE IF EXISTS workflows CASCADE;
DROP TABLE IF EXISTS task_delegations CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;

-- Drop sequences
DROP SEQUENCE IF EXISTS document_operation_seq;

-- Drop types
DROP TYPE IF EXISTS member_role;
DROP TYPE IF EXISTS workspace_visibility;
DROP TYPE IF EXISTS delegation_type;
DROP TYPE IF EXISTS workflow_status;
DROP TYPE IF EXISTS workflow_type;
DROP TYPE IF EXISTS task_priority;
DROP TYPE IF EXISTS task_status;

-- Drop functions
DROP FUNCTION IF EXISTS jsonb_merge_recursive(JSONB, JSONB);
DROP FUNCTION IF EXISTS validate_jsonb_keys(JSONB, TEXT[]);
DROP FUNCTION IF EXISTS current_tenant_id();
DROP FUNCTION IF EXISTS update_updated_at_column();