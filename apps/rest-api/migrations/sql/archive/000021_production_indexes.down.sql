-- Migration: 000021_production_indexes.down.sql
-- Purpose: Rollback indexes and columns added in 000021_production_indexes.up.sql

-- Drop partial indexes
DROP INDEX IF EXISTS idx_agents_available;
DROP INDEX IF EXISTS idx_tasks_in_progress;
DROP INDEX IF EXISTS idx_tasks_pending;

-- Drop composite indexes
DROP INDEX IF EXISTS idx_agents_tenant_status_workload;
DROP INDEX IF EXISTS idx_tasks_tenant_created_by_status;

-- Drop workflow execution columns
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS error_details;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS parent_execution_id;
ALTER TABLE workflow_executions DROP COLUMN IF EXISTS retry_count;

-- Drop agent workload columns
ALTER TABLE agents DROP COLUMN IF EXISTS last_task_assigned_at;
ALTER TABLE agents DROP COLUMN IF EXISTS max_workload;
ALTER TABLE agents DROP COLUMN IF EXISTS current_workload;

-- Drop task delegation columns
ALTER TABLE tasks DROP COLUMN IF EXISTS delegation_count;
ALTER TABLE tasks DROP COLUMN IF EXISTS delegated_from;
ALTER TABLE tasks DROP COLUMN IF EXISTS accepted_at;
ALTER TABLE tasks DROP COLUMN IF EXISTS assigned_at;

-- Drop workspace indexes
DROP INDEX IF EXISTS idx_workspace_members_agent_id;
DROP INDEX IF EXISTS idx_workspaces_type;
DROP INDEX IF EXISTS idx_workspaces_tenant_id;

-- Drop document indexes
DROP INDEX IF EXISTS idx_documents_type;
DROP INDEX IF EXISTS idx_documents_created_by;
DROP INDEX IF EXISTS idx_documents_workspace_id;

-- Drop workflow indexes
DROP INDEX IF EXISTS idx_workflows_is_active;
DROP INDEX IF EXISTS idx_workflows_tenant_type;
DROP INDEX IF EXISTS idx_workflow_executions_started_at;
DROP INDEX IF EXISTS idx_workflow_executions_status;

-- Drop agent indexes
DROP INDEX IF EXISTS idx_agents_tenant_id;
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_capabilities;

-- Drop task indexes
DROP INDEX IF EXISTS idx_tasks_created_at;
DROP INDEX IF EXISTS idx_tasks_priority_status;
DROP INDEX IF EXISTS idx_tasks_created_by;
DROP INDEX IF EXISTS idx_tasks_assigned_to;
DROP INDEX IF EXISTS idx_tasks_tenant_status;