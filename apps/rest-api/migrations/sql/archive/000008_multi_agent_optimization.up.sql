-- Migration: Multi-Agent Performance Optimization and Test Data

BEGIN;

-- Set search path to mcp schema
SET search_path TO mcp, public;

-- Ensure current month partitions exist (June 2025)
CREATE TABLE IF NOT EXISTS mcp.tasks_2025_06 PARTITION OF mcp.tasks FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');
CREATE TABLE IF NOT EXISTS mcp.audit_log_2025_06 PARTITION OF mcp.audit_log FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');

-- Configure autovacuum for partition tables (applied to partitions, not parents)
-- For tasks partitions
ALTER TABLE mcp.tasks_2025_01 SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

ALTER TABLE mcp.tasks_2025_02 SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

-- Configure June partition
ALTER TABLE mcp.tasks_2025_06 SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02,
    autovacuum_vacuum_cost_delay = 10,
    autovacuum_vacuum_cost_limit = 1000
);

-- For non-partitioned tables
ALTER TABLE mcp.workflow_executions SET (
    autovacuum_vacuum_scale_factor = 0.05,
    autovacuum_analyze_scale_factor = 0.02
);
ALTER TABLE mcp.document_operations SET (
    autovacuum_vacuum_scale_factor = 0.1,
    autovacuum_analyze_scale_factor = 0.05
);

-- Create statistics for better query planning
CREATE STATISTICS IF NOT EXISTS tasks_status_priority ON status, priority FROM mcp.tasks;
CREATE STATISTICS IF NOT EXISTS workflows_tenant_type ON tenant_id, type FROM mcp.workflows;

-- Add table comments for documentation
COMMENT ON TABLE mcp.tasks IS 'Core task management table supporting delegation and distributed execution';
COMMENT ON TABLE mcp.workflows IS 'Workflow definitions for multi-agent orchestration';
COMMENT ON TABLE mcp.workspaces IS 'Shared collaboration spaces for agent coordination';
COMMENT ON TABLE mcp.shared_documents IS 'Collaborative documents with CRDT support';

-- Insert test data only in development/test environments
DO $$
DECLARE
    test_tenant_id UUID := '00000000-0000-0000-0000-000000000001'::uuid;
    test_workspace_id UUID;
BEGIN
    IF current_database() LIKE '%_dev' OR current_database() LIKE '%_test' THEN
        -- Set tenant context
        PERFORM set_config('app.current_tenant', test_tenant_id::text, true);
        
        -- Insert test workflow
        INSERT INTO mcp.workflows (tenant_id, name, type, created_by, agents, steps)
        VALUES (
            test_tenant_id,
            'Test Code Review Workflow',
            'collaborative',
            'test-coordinator',
            '{"analyzer": "code-analyzer-agent", "reviewer": "code-reviewer-agent"}'::jsonb,
            '[
                {"id": "analyze", "agent": "analyzer", "action": "analyze_code"},
                {"id": "review", "agent": "reviewer", "action": "review_findings", "depends_on": ["analyze"]}
            ]'::jsonb
        );
        
        -- Insert test workspace
        INSERT INTO mcp.workspaces (id, tenant_id, name, type, owner_id)
        VALUES (
            uuid_generate_v4(),
            test_tenant_id,
            'Test Collaboration Space',
            'general',
            'test-agent-1'
        ) RETURNING id INTO test_workspace_id;
        
        -- Add workspace members
        INSERT INTO mcp.workspace_members (workspace_id, agent_id, tenant_id, role)
        VALUES 
            (test_workspace_id, 'test-agent-1', test_tenant_id, 'owner'),
            (test_workspace_id, 'test-agent-2', test_tenant_id, 'member');
            
        -- Insert sample tasks with created_at for partitioning
        INSERT INTO mcp.tasks (tenant_id, type, title, created_by, assigned_to, priority, status, created_at)
        VALUES 
            (test_tenant_id, 'code_review', 'Review authentication module', 'test-agent-1', 'test-agent-2', 'high', 'assigned', CURRENT_TIMESTAMP),
            (test_tenant_id, 'testing', 'Write unit tests for API', 'test-agent-1', NULL, 'normal', 'pending', CURRENT_TIMESTAMP);
    END IF;
END $$;

COMMIT;