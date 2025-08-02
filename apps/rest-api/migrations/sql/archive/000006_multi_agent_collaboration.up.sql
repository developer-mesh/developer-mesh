-- Migration: Multi-Agent Collaboration Schema
-- Phase 1: Database Schema and Migrations

BEGIN;

-- Set search path for this transaction
SET search_path TO mcp, public;

-- Enable required extensions (if not already enabled)
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Create custom types
DO $$ BEGIN
    CREATE TYPE task_status AS ENUM (
        'pending', 'assigned', 'accepted', 'rejected', 
        'in_progress', 'completed', 'failed', 'cancelled', 'timeout'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE task_priority AS ENUM ('low', 'normal', 'high', 'critical');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'collaborative');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE workflow_status AS ENUM (
        'pending', 'running', 'paused', 'completed', 
        'failed', 'cancelled', 'timeout'
    );
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE delegation_type AS ENUM ('manual', 'automatic', 'failover', 'load_balance');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'public');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

DO $$ BEGIN
    CREATE TYPE member_role AS ENUM ('owner', 'admin', 'member', 'viewer');
EXCEPTION
    WHEN duplicate_object THEN null;
END $$;

-- Helper function for updated_at triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Function to get current tenant (for RLS)
CREATE OR REPLACE FUNCTION current_tenant_id() 
RETURNS UUID AS $$
BEGIN
    RETURN NULLIF(current_setting('app.current_tenant', true), '')::UUID;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to validate JSONB structure
CREATE OR REPLACE FUNCTION validate_jsonb_keys(
    data JSONB,
    required_keys TEXT[]
) RETURNS BOOLEAN AS $$
BEGIN
    RETURN data ?& required_keys;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Function to merge JSONB objects recursively
CREATE OR REPLACE FUNCTION jsonb_merge_recursive(
    target JSONB,
    source JSONB
) RETURNS JSONB AS $$
BEGIN
    RETURN CASE 
        WHEN jsonb_typeof(target) = 'object' AND jsonb_typeof(source) = 'object' 
        THEN (
            SELECT jsonb_object_agg(
                COALESCE(t.key, s.key),
                CASE
                    WHEN t.value IS NULL THEN s.value
                    WHEN s.value IS NULL THEN t.value
                    WHEN jsonb_typeof(t.value) = 'object' AND jsonb_typeof(s.value) = 'object'
                    THEN jsonb_merge_recursive(t.value, s.value)
                    ELSE s.value
                END
            )
            FROM jsonb_each(target) t
            FULL OUTER JOIN jsonb_each(source) s ON t.key = s.key
        )
        ELSE source
    END;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Create tasks table with partitioning
CREATE TABLE IF NOT EXISTS tasks (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    status task_status NOT NULL DEFAULT 'pending',
    priority task_priority DEFAULT 'normal',
    
    -- Agent relationships
    created_by VARCHAR(255) NOT NULL,
    assigned_to VARCHAR(255),
    
    -- Task hierarchy
    parent_task_id UUID,
    
    -- Task data
    title VARCHAR(500) NOT NULL CONSTRAINT title_not_empty CHECK (length(trim(title)) > 0),
    description TEXT,
    parameters JSONB DEFAULT '{}' NOT NULL,
    result JSONB,
    error TEXT,
    
    -- Execution control
    max_retries INTEGER DEFAULT 3 CONSTRAINT valid_max_retries CHECK (max_retries >= 0 AND max_retries <= 10),
    retry_count INTEGER DEFAULT 0,
    timeout_seconds INTEGER DEFAULT 3600 CONSTRAINT valid_timeout CHECK (timeout_seconds > 0),
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    assigned_at TIMESTAMP WITH TIME ZONE,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Primary key includes partition column
    PRIMARY KEY (id, created_at),
    
    -- Constraints
    CONSTRAINT retry_count_valid CHECK (retry_count >= 0 AND retry_count <= max_retries),
    CONSTRAINT parameters_is_object CHECK (jsonb_typeof(parameters) = 'object'),
    CONSTRAINT result_is_object CHECK (result IS NULL OR jsonb_typeof(result) = 'object')
) PARTITION BY RANGE (created_at);

-- Create initial partitions
CREATE TABLE IF NOT EXISTS tasks_2025_01 PARTITION OF tasks FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE IF NOT EXISTS tasks_2025_02 PARTITION OF tasks FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
-- Add partition for current month (June 2025)
CREATE TABLE IF NOT EXISTS tasks_2025_06 PARTITION OF tasks FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');

-- Performance indexes for tasks
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assigned_to ON tasks(assigned_to) WHERE assigned_to IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_status ON tasks(status) WHERE status NOT IN ('completed', 'failed', 'cancelled') AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_tenant_status_priority ON tasks(tenant_id, status, priority) 
    WHERE status IN ('pending', 'assigned') AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_parent_id ON tasks(parent_task_id) WHERE parent_task_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tasks_created_at ON tasks(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tasks_list_covering ON tasks(tenant_id, assigned_to, status) 
    INCLUDE (title, priority, created_at)
    WHERE deleted_at IS NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;
CREATE TRIGGER update_tasks_updated_at
    BEFORE UPDATE ON tasks
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tasks_tenant_isolation ON tasks;
CREATE POLICY tasks_tenant_isolation ON tasks
    FOR ALL
    USING (tenant_id = current_tenant_id());

DROP POLICY IF EXISTS tasks_soft_delete ON tasks;
CREATE POLICY tasks_soft_delete ON tasks
    FOR SELECT
    USING (deleted_at IS NULL);

-- Performance settings (applied to partitions, not parent)
-- Note: These settings will be applied to partitions in the optimization migration

-- Task delegations table
CREATE TABLE IF NOT EXISTS task_delegations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL,
    task_created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    from_agent_id VARCHAR(255) NOT NULL,
    to_agent_id VARCHAR(255) NOT NULL,
    reason TEXT,
    delegation_type delegation_type DEFAULT 'manual',
    metadata JSONB DEFAULT '{}' NOT NULL,
    delegated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT metadata_is_object CHECK (jsonb_typeof(metadata) = 'object'),
    FOREIGN KEY (task_id, task_created_at) REFERENCES tasks(id, created_at) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_delegations_task_id ON task_delegations(task_id);
CREATE INDEX IF NOT EXISTS idx_delegations_from_agent ON task_delegations(from_agent_id);
CREATE INDEX IF NOT EXISTS idx_delegations_to_agent ON task_delegations(to_agent_id);
CREATE INDEX IF NOT EXISTS idx_delegations_delegated_at ON task_delegations(delegated_at DESC);

-- Workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL CONSTRAINT name_not_empty CHECK (length(trim(name)) > 0),
    type workflow_type NOT NULL DEFAULT 'sequential',
    version INTEGER DEFAULT 1 CONSTRAINT version_positive CHECK (version > 0),
    
    -- Workflow definition
    created_by VARCHAR(255) NOT NULL,
    agents JSONB NOT NULL DEFAULT '{}' CONSTRAINT valid_agents_jsonb CHECK (jsonb_typeof(agents) = 'object'),
    steps JSONB NOT NULL DEFAULT '[]' CONSTRAINT valid_steps_jsonb CHECK (jsonb_typeof(steps) = 'array'),
    config JSONB DEFAULT '{}' NOT NULL CONSTRAINT valid_config_jsonb CHECK (jsonb_typeof(config) = 'object'),
    
    -- Metadata
    description TEXT,
    tags TEXT[] DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_workflows_tenant_id ON workflows(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workflows_name ON workflows(name) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflows_type ON workflows(type);
CREATE INDEX IF NOT EXISTS idx_workflows_tenant_active_type ON workflows(tenant_id, is_active, type) 
    WHERE is_active = true AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workflows_tags ON workflows USING gin(tags);

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_workflows_updated_at ON workflows;
CREATE TRIGGER update_workflows_updated_at
    BEFORE UPDATE ON workflows
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS workflows_tenant_isolation ON workflows;
CREATE POLICY workflows_tenant_isolation ON workflows
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Workflow executions table
CREATE TABLE IF NOT EXISTS workflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    tenant_id UUID NOT NULL, -- Denormalized for performance
    
    -- Execution state
    status workflow_status NOT NULL DEFAULT 'pending',
    current_step_index INTEGER DEFAULT 0,
    current_step_id VARCHAR(255),
    
    -- Execution data
    triggered_by VARCHAR(255) NOT NULL,
    input JSONB DEFAULT '{}' NOT NULL,
    context JSONB DEFAULT '{}' NOT NULL,
    step_results JSONB DEFAULT '{}' NOT NULL,
    output JSONB,
    error TEXT,
    
    -- Timing
    started_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    
    -- Execution metadata
    execution_metadata JSONB DEFAULT '{}' NOT NULL,
    
    CONSTRAINT valid_input CHECK (jsonb_typeof(input) = 'object'),
    CONSTRAINT valid_context CHECK (jsonb_typeof(context) = 'object'),
    CONSTRAINT valid_step_results CHECK (jsonb_typeof(step_results) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX IF NOT EXISTS idx_executions_tenant_id ON workflow_executions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_executions_status ON workflow_executions(status) 
    WHERE status NOT IN ('completed', 'failed', 'cancelled');
CREATE INDEX IF NOT EXISTS idx_executions_triggered_by ON workflow_executions(triggered_by);
CREATE INDEX IF NOT EXISTS idx_executions_started_at ON workflow_executions(started_at DESC);

-- Row Level Security
ALTER TABLE workflow_executions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS executions_tenant_isolation ON workflow_executions;
CREATE POLICY executions_tenant_isolation ON workflow_executions
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Performance settings applied to partition in optimization migration

-- Workspaces table
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL CONSTRAINT workspace_name_not_empty CHECK (length(trim(name)) > 0),
    type VARCHAR(50) DEFAULT 'general',
    
    -- Ownership and access
    owner_id VARCHAR(255) NOT NULL,
    visibility workspace_visibility DEFAULT 'private',
    
    -- Workspace state with optimistic locking
    state JSONB DEFAULT '{}' NOT NULL,
    state_version INTEGER DEFAULT 1,
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Configuration
    config JSONB DEFAULT '{}' NOT NULL,
    max_members INTEGER DEFAULT 100 CONSTRAINT valid_max_members CHECK (max_members > 0 AND max_members <= 1000),
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT valid_state CHECK (jsonb_typeof(state) = 'object'),
    CONSTRAINT valid_config CHECK (jsonb_typeof(config) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_owner_id ON workspaces(owner_id);
CREATE INDEX IF NOT EXISTS idx_workspaces_type ON workspaces(type) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_workspaces_last_activity ON workspaces(last_activity_at DESC) WHERE deleted_at IS NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_workspaces_updated_at ON workspaces;
CREATE TRIGGER update_workspaces_updated_at
    BEFORE UPDATE ON workspaces
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS workspaces_tenant_isolation ON workspaces;
CREATE POLICY workspaces_tenant_isolation ON workspaces
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Workspace members table
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL,
    tenant_id UUID NOT NULL, -- Denormalized for RLS
    
    -- Membership details
    role member_role DEFAULT 'member',
    permissions JSONB DEFAULT '["read", "write"]' NOT NULL,
    
    -- Activity tracking
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    last_seen_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Member-specific state
    member_state JSONB DEFAULT '{}' NOT NULL,
    
    PRIMARY KEY (workspace_id, agent_id),
    
    CONSTRAINT valid_permissions CHECK (jsonb_typeof(permissions) = 'array'),
    CONSTRAINT valid_member_state CHECK (jsonb_typeof(member_state) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_workspace_members_agent_id ON workspace_members(agent_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_tenant_id ON workspace_members(tenant_id);
CREATE INDEX IF NOT EXISTS idx_workspace_members_role ON workspace_members(role);
CREATE INDEX IF NOT EXISTS idx_workspace_members_last_seen ON workspace_members(last_seen_at DESC);

-- Row Level Security
ALTER TABLE workspace_members ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS members_tenant_isolation ON workspace_members;
CREATE POLICY members_tenant_isolation ON workspace_members
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Shared documents table
CREATE TABLE IF NOT EXISTS shared_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    
    -- Document metadata
    title VARCHAR(255) NOT NULL CONSTRAINT doc_title_not_empty CHECK (length(trim(title)) > 0),
    content TEXT,
    content_type VARCHAR(50) DEFAULT 'text' CHECK (content_type IN ('text', 'markdown', 'json', 'yaml', 'code')),
    language VARCHAR(50), -- For code documents
    
    -- Version control
    version INTEGER DEFAULT 1 CONSTRAINT doc_version_positive CHECK (version > 0),
    is_locked BOOLEAN DEFAULT false,
    locked_by VARCHAR(255),
    locked_at TIMESTAMP WITH TIME ZONE,
    lock_expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Tracking
    created_by VARCHAR(255) NOT NULL,
    updated_by VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    metadata JSONB DEFAULT '{}' NOT NULL,
    
    CONSTRAINT valid_metadata CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT lock_consistency CHECK (
        (is_locked = false AND locked_by IS NULL AND locked_at IS NULL AND lock_expires_at IS NULL) OR
        (is_locked = true AND locked_by IS NOT NULL AND locked_at IS NOT NULL AND lock_expires_at IS NOT NULL)
    )
);

CREATE INDEX IF NOT EXISTS idx_documents_workspace_id ON shared_documents(workspace_id) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_documents_tenant_id ON shared_documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_documents_created_by ON shared_documents(created_by);
CREATE INDEX IF NOT EXISTS idx_documents_updated_at ON shared_documents(updated_at DESC) WHERE deleted_at IS NULL;

-- Trigger for updated_at
DROP TRIGGER IF EXISTS update_documents_updated_at ON shared_documents;
CREATE TRIGGER update_documents_updated_at
    BEFORE UPDATE ON shared_documents
    FOR EACH ROW
    EXECUTE PROCEDURE update_updated_at_column();

-- Row Level Security
ALTER TABLE shared_documents ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS documents_tenant_isolation ON shared_documents;
CREATE POLICY documents_tenant_isolation ON shared_documents
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Document operations table
-- Sequence for operation ordering
CREATE SEQUENCE IF NOT EXISTS document_operation_seq;

CREATE TABLE IF NOT EXISTS document_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES shared_documents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL, -- Denormalized for RLS
    
    -- Operation details
    agent_id VARCHAR(255) NOT NULL,
    operation_type VARCHAR(50) NOT NULL CHECK (operation_type IN ('insert', 'delete', 'update', 'move', 'format')),
    operation_data JSONB NOT NULL,
    
    -- CRDT metadata
    vector_clock JSONB NOT NULL,
    sequence_number BIGINT NOT NULL DEFAULT nextval('document_operation_seq'),
    
    -- Timing
    timestamp TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Conflict resolution
    parent_operation_id UUID REFERENCES document_operations(id),
    is_applied BOOLEAN DEFAULT false,
    
    CONSTRAINT valid_operation_data CHECK (jsonb_typeof(operation_data) = 'object'),
    CONSTRAINT valid_vector_clock CHECK (jsonb_typeof(vector_clock) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_operations_document_id ON document_operations(document_id);
CREATE INDEX IF NOT EXISTS idx_operations_tenant_id ON document_operations(tenant_id);
CREATE INDEX IF NOT EXISTS idx_operations_timestamp ON document_operations(document_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_operations_sequence ON document_operations(document_id, sequence_number);
CREATE INDEX IF NOT EXISTS idx_operations_not_applied ON document_operations(document_id) WHERE is_applied = false;
CREATE INDEX IF NOT EXISTS idx_operations_parent_id ON document_operations(parent_operation_id) 
    WHERE parent_operation_id IS NOT NULL;

-- Row Level Security
ALTER TABLE document_operations ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS operations_tenant_isolation ON document_operations;
CREATE POLICY operations_tenant_isolation ON document_operations
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Audit log table with partitioning
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    table_name VARCHAR(50) NOT NULL,
    record_id UUID NOT NULL,
    action VARCHAR(20) NOT NULL CHECK (action IN ('INSERT', 'UPDATE', 'DELETE', 'SELECT')),
    old_data JSONB,
    new_data JSONB,
    changed_fields TEXT[],
    changed_by VARCHAR(255) NOT NULL,
    ip_address INET,
    user_agent TEXT,
    changed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, changed_at)
) PARTITION BY RANGE (changed_at);

-- Create initial partitions
CREATE TABLE IF NOT EXISTS audit_log_2025_01 PARTITION OF audit_log FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE IF NOT EXISTS audit_log_2025_02 PARTITION OF audit_log FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
-- Add partition for current month (June 2025)
CREATE TABLE IF NOT EXISTS audit_log_2025_06 PARTITION OF audit_log FOR VALUES FROM ('2025-06-01') TO ('2025-07-01');

CREATE INDEX IF NOT EXISTS idx_audit_log_tenant_table ON audit_log(tenant_id, table_name);
CREATE INDEX IF NOT EXISTS idx_audit_log_record ON audit_log(record_id);
CREATE INDEX IF NOT EXISTS idx_audit_log_changed_at ON audit_log(changed_at DESC);
CREATE INDEX IF NOT EXISTS idx_audit_log_changed_by ON audit_log(changed_by);

-- Row Level Security
ALTER TABLE audit_log ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS audit_tenant_isolation ON audit_log;
CREATE POLICY audit_tenant_isolation ON audit_log
    FOR ALL
    USING (tenant_id = current_tenant_id());

-- Conflict resolutions table
CREATE TABLE IF NOT EXISTS conflict_resolutions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id UUID NOT NULL,
    conflict_type VARCHAR(50) NOT NULL,
    description TEXT,
    resolution_strategy VARCHAR(50),
    details JSONB DEFAULT '{}' NOT NULL,
    resolved_by VARCHAR(255),
    resolved_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT valid_details CHECK (jsonb_typeof(details) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_conflicts_tenant_id ON conflict_resolutions(tenant_id);
CREATE INDEX IF NOT EXISTS idx_conflicts_resource ON conflict_resolutions(resource_type, resource_id);
CREATE INDEX IF NOT EXISTS idx_conflicts_created_at ON conflict_resolutions(created_at DESC);

-- Row Level Security
ALTER TABLE conflict_resolutions ENABLE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS conflicts_tenant_isolation ON conflict_resolutions;
CREATE POLICY conflicts_tenant_isolation ON conflict_resolutions
    FOR ALL
    USING (tenant_id = current_tenant_id());

COMMIT;