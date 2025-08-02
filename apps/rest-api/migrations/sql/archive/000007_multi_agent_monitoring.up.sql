-- Migration: Multi-Agent Monitoring Views and Audit Triggers

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

-- Task statistics view
CREATE OR REPLACE VIEW v_task_statistics AS
SELECT 
    tenant_id,
    status,
    priority,
    COUNT(*) as count,
    AVG(EXTRACT(EPOCH FROM (completed_at - created_at))) as avg_duration_seconds,
    PERCENTILE_CONT(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as median_duration_seconds,
    PERCENTILE_CONT(0.95) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (completed_at - created_at))) as p95_duration_seconds
FROM tasks
WHERE deleted_at IS NULL
GROUP BY tenant_id, status, priority;

-- Active workflows view
CREATE OR REPLACE VIEW v_active_workflows AS
SELECT 
    w.id,
    w.tenant_id,
    w.name,
    w.type,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status IN ('pending', 'running')) as active_executions,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status = 'completed') as completed_executions,
    COUNT(DISTINCT we.id) FILTER (WHERE we.status = 'failed') as failed_executions,
    MAX(we.started_at) as last_execution_started
FROM workflows w
LEFT JOIN workflow_executions we ON w.id = we.workflow_id
WHERE w.is_active = true AND w.deleted_at IS NULL
GROUP BY w.id, w.tenant_id, w.name, w.type;

-- Agent workload view
CREATE OR REPLACE VIEW v_agent_workload AS
SELECT 
    tenant_id,
    assigned_to as agent_id,
    COUNT(*) FILTER (WHERE status IN ('assigned', 'accepted')) as pending_tasks,
    COUNT(*) FILTER (WHERE status = 'in_progress') as active_tasks,
    COUNT(*) FILTER (WHERE status = 'completed' AND completed_at > CURRENT_TIMESTAMP - INTERVAL '24 hours') as completed_24h,
    AVG(EXTRACT(EPOCH FROM (completed_at - started_at))) FILTER (WHERE status = 'completed') as avg_task_duration_seconds
FROM tasks
WHERE deleted_at IS NULL AND assigned_to IS NOT NULL
GROUP BY tenant_id, assigned_to;

-- Workspace activity view
CREATE OR REPLACE VIEW v_workspace_activity AS
SELECT 
    w.id,
    w.tenant_id,
    w.name,
    w.type,
    COUNT(DISTINCT wm.agent_id) as member_count,
    COUNT(DISTINCT wm.agent_id) FILTER (WHERE wm.last_seen_at > CURRENT_TIMESTAMP - INTERVAL '15 minutes') as active_members,
    w.state_version,
    w.last_activity_at,
    COUNT(DISTINCT d.id) as document_count
FROM workspaces w
LEFT JOIN workspace_members wm ON w.id = wm.workspace_id
LEFT JOIN shared_documents d ON w.id = d.workspace_id AND d.deleted_at IS NULL
WHERE w.deleted_at IS NULL
GROUP BY w.id, w.tenant_id, w.name, w.type, w.state_version, w.last_activity_at;

-- Generic audit trigger function
CREATE OR REPLACE FUNCTION audit_trigger_function()
RETURNS TRIGGER AS $$
DECLARE
    audit_row audit_log;
    changed_fields TEXT[];
BEGIN
    audit_row.id := gen_random_uuid(); -- Generate ID for audit log entry
    audit_row.tenant_id := COALESCE(NEW.tenant_id, OLD.tenant_id);
    audit_row.table_name := TG_TABLE_NAME;
    audit_row.action := TG_OP;
    audit_row.changed_by := COALESCE(current_setting('app.current_user', true), 'system');
    audit_row.ip_address := COALESCE(inet(current_setting('app.client_ip', true)), NULL);
    audit_row.user_agent := current_setting('app.user_agent', true);
    audit_row.changed_at := CURRENT_TIMESTAMP; -- Set timestamp explicitly
    
    IF TG_OP = 'DELETE' THEN
        audit_row.record_id := OLD.id;
        audit_row.old_data := to_jsonb(OLD);
    ELSIF TG_OP = 'UPDATE' THEN
        audit_row.record_id := NEW.id;
        audit_row.old_data := to_jsonb(OLD);
        audit_row.new_data := to_jsonb(NEW);
        
        -- Calculate changed fields
        SELECT array_agg(key) INTO changed_fields
        FROM jsonb_each(to_jsonb(NEW))
        WHERE to_jsonb(NEW) -> key IS DISTINCT FROM to_jsonb(OLD) -> key;
        
        audit_row.changed_fields := changed_fields;
    ELSIF TG_OP = 'INSERT' THEN
        audit_row.record_id := NEW.id;
        audit_row.new_data := to_jsonb(NEW);
    END IF;
    
    INSERT INTO audit_log VALUES (audit_row.*);
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Create audit triggers for main tables
DROP TRIGGER IF EXISTS audit_tasks ON mcp.tasks;
CREATE TRIGGER audit_tasks AFTER INSERT OR UPDATE OR DELETE ON mcp.tasks
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

DROP TRIGGER IF EXISTS audit_workflows ON mcp.workflows;
CREATE TRIGGER audit_workflows AFTER INSERT OR UPDATE OR DELETE ON mcp.workflows
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

DROP TRIGGER IF EXISTS audit_workspaces ON mcp.workspaces;
CREATE TRIGGER audit_workspaces AFTER INSERT OR UPDATE OR DELETE ON mcp.workspaces
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

DROP TRIGGER IF EXISTS audit_documents ON mcp.shared_documents;
CREATE TRIGGER audit_documents AFTER INSERT OR UPDATE OR DELETE ON mcp.shared_documents
    FOR EACH ROW EXECUTE PROCEDURE audit_trigger_function();

COMMIT;