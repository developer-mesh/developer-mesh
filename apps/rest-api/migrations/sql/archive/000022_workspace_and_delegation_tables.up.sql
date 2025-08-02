-- Create workspace members table for role-based membership
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL CHECK (role IN ('owner', 'admin', 'member', 'viewer')),
    permissions JSONB DEFAULT '{}',
    joined_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    joined_by VARCHAR(255) NOT NULL,
    last_active TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    metadata JSONB DEFAULT '{}',
    PRIMARY KEY (workspace_id, agent_id)
);

-- Create indexes for workspace members
CREATE INDEX idx_workspace_members_agent ON workspace_members(agent_id);
CREATE INDEX idx_workspace_members_role ON workspace_members(role);
CREATE INDEX idx_workspace_members_last_active ON workspace_members(last_active);

-- Create workspace activities table for audit trail
CREATE TABLE IF NOT EXISTS workspace_activities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) NOT NULL,
    activity_type VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50),
    resource_id VARCHAR(255),
    action VARCHAR(50) NOT NULL,
    metadata JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for workspace activities
CREATE INDEX idx_workspace_activities_workspace ON workspace_activities(workspace_id);
CREATE INDEX idx_workspace_activities_agent ON workspace_activities(agent_id);
CREATE INDEX idx_workspace_activities_type ON workspace_activities(activity_type);
CREATE INDEX idx_workspace_activities_created ON workspace_activities(created_at DESC);
CREATE INDEX idx_workspace_activities_resource ON workspace_activities(resource_type, resource_id);

-- Create task delegation history table
CREATE TABLE IF NOT EXISTS task_delegation_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_agent_id VARCHAR(255) NOT NULL,
    to_agent_id VARCHAR(255) NOT NULL,
    delegation_type VARCHAR(50) NOT NULL CHECK (delegation_type IN ('assign', 'transfer', 'escalate', 'delegate')),
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    delegated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    accepted_at TIMESTAMP WITH TIME ZONE,
    rejected_at TIMESTAMP WITH TIME ZONE,
    rejection_reason TEXT
);

-- Create indexes for task delegation history
CREATE INDEX idx_task_delegation_task ON task_delegation_history(task_id);
CREATE INDEX idx_task_delegation_from_agent ON task_delegation_history(from_agent_id);
CREATE INDEX idx_task_delegation_to_agent ON task_delegation_history(to_agent_id);
CREATE INDEX idx_task_delegation_type ON task_delegation_history(delegation_type);
CREATE INDEX idx_task_delegation_time ON task_delegation_history(delegated_at DESC);

-- Create task state transitions table for state machine tracking
CREATE TABLE IF NOT EXISTS task_state_transitions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    from_status VARCHAR(50),
    to_status VARCHAR(50) NOT NULL,
    agent_id VARCHAR(255) NOT NULL,
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    transitioned_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create indexes for task state transitions
CREATE INDEX idx_task_transitions_task ON task_state_transitions(task_id);
CREATE INDEX idx_task_transitions_agent ON task_state_transitions(agent_id);
CREATE INDEX idx_task_transitions_time ON task_state_transitions(transitioned_at DESC);
CREATE INDEX idx_task_transitions_status ON task_state_transitions(from_status, to_status);

-- Create idempotency keys table for task operations
CREATE TABLE IF NOT EXISTS task_idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    operation VARCHAR(50) NOT NULL,
    request_hash VARCHAR(64),
    response JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    expires_at TIMESTAMP WITH TIME ZONE DEFAULT NOW() + INTERVAL '24 hours'
);

-- Create index for cleanup
CREATE INDEX idx_task_idempotency_expires ON task_idempotency_keys(expires_at);

-- Add workspace quota tracking columns
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS max_members INTEGER DEFAULT 100;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS max_storage_bytes BIGINT DEFAULT 10737418240; -- 10GB default
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS current_storage_bytes BIGINT DEFAULT 0;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS max_documents INTEGER DEFAULT 1000;
ALTER TABLE workspaces ADD COLUMN IF NOT EXISTS current_documents INTEGER DEFAULT 0;

-- Add task delegation tracking columns
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS delegation_count INTEGER DEFAULT 0;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS max_delegations INTEGER DEFAULT 5;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS auto_escalate BOOLEAN DEFAULT FALSE;
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS escalation_timeout INTERVAL;