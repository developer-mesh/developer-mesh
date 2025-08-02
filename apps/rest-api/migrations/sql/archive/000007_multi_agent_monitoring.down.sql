-- Rollback: Multi-Agent Monitoring Views and Audit Triggers

-- Drop audit triggers
DROP TRIGGER IF EXISTS audit_tasks ON tasks;
DROP TRIGGER IF EXISTS audit_workflows ON workflows;
DROP TRIGGER IF EXISTS audit_workspaces ON workspaces;
DROP TRIGGER IF EXISTS audit_documents ON shared_documents;
DROP FUNCTION IF EXISTS audit_trigger_function();

-- Drop monitoring views
DROP VIEW IF EXISTS v_workspace_activity;
DROP VIEW IF EXISTS v_agent_workload;
DROP VIEW IF EXISTS v_active_workflows;
DROP VIEW IF EXISTS v_task_statistics;