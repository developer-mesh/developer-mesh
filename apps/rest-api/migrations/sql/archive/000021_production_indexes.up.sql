-- Migration: 000021_production_indexes.up.sql
-- Purpose: Add missing indexes and columns for production performance

-- Add missing indexes for task queries
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status ON tasks(tenant_id, status);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX IF NOT EXISTS idx_tasks_created_by ON tasks(created_by);
CREATE INDEX IF NOT EXISTS idx_tasks_priority_status ON tasks(priority, status);
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);

-- Add missing indexes for agent queries
CREATE INDEX IF NOT EXISTS idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX IF NOT EXISTS idx_agents_status ON agents(status);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_id ON agents(tenant_id);

-- Add missing indexes for workflow queries
CREATE INDEX IF NOT EXISTS idx_workflow_executions_status ON workflow_executions(workflow_id, status);
CREATE INDEX IF NOT EXISTS idx_workflow_executions_started_at ON workflow_executions(started_at DESC);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_type ON workflows(tenant_id, type);
CREATE INDEX IF NOT EXISTS idx_workflows_is_active ON workflows(is_active) WHERE is_active = true;

-- Add missing indexes for document queries
CREATE INDEX IF NOT EXISTS idx_documents_workspace_id ON documents(workspace_id);
CREATE INDEX IF NOT EXISTS idx_documents_created_by ON documents(created_by);
CREATE INDEX IF NOT EXISTS idx_documents_type ON documents(type);

-- Add missing indexes for workspace queries
CREATE INDEX IF NOT EXISTS idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_type ON workspaces(type);
CREATE INDEX IF NOT EXISTS idx_workspace_members_agent_id ON workspace_members(agent_id);

-- Add missing columns for task delegation tracking
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS assigned_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS accepted_at TIMESTAMP;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegated_from VARCHAR(255);
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegation_count INTEGER DEFAULT 0;

-- Add missing columns for agent workload tracking
ALTER TABLE agents ADD COLUMN IF NOT EXISTS current_workload INTEGER DEFAULT 0;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS max_workload INTEGER DEFAULT 10;
ALTER TABLE agents ADD COLUMN IF NOT EXISTS last_task_assigned_at TIMESTAMP;

-- Add missing columns for workflow execution tracking
ALTER TABLE workflow_executions ADD COLUMN IF NOT EXISTS retry_count INTEGER DEFAULT 0;
ALTER TABLE workflow_executions ADD COLUMN IF NOT EXISTS parent_execution_id UUID;
ALTER TABLE workflow_executions ADD COLUMN IF NOT EXISTS error_details JSONB;

-- Add composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_created_by_status ON tasks(tenant_id, created_by, status);
CREATE INDEX IF NOT EXISTS idx_agents_tenant_status_workload ON agents(tenant_id, status, current_workload);

-- Add partial indexes for performance
CREATE INDEX IF NOT EXISTS idx_tasks_pending ON tasks(status) WHERE status = 'pending';
CREATE INDEX IF NOT EXISTS idx_tasks_in_progress ON tasks(status, assigned_to) WHERE status = 'in_progress';
CREATE INDEX IF NOT EXISTS idx_agents_available ON agents(status, current_workload) WHERE status = 'available' AND current_workload < max_workload;

-- Update statistics for query planner
ANALYZE tasks;
ANALYZE agents;
ANALYZE workflows;
ANALYZE workflow_executions;
ANALYZE documents;
ANALYZE workspaces;