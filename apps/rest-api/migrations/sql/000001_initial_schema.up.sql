-- Initial Schema for Developer Mesh
-- Consolidated from 26 migrations into a single clean schema
-- Created: 2025-08-02

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Create MCP schema for core platform
CREATE SCHEMA IF NOT EXISTS mcp;

-- Set search path
SET search_path TO mcp, public;

-- ==============================================================================
-- FOUNDATION TABLES
-- ==============================================================================

-- Models table (AI model registry)
CREATE TABLE IF NOT EXISTS models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    provider VARCHAR(100) NOT NULL,
    type VARCHAR(50) NOT NULL CHECK (type IN ('llm', 'embedding', 'vision', 'audio')),
    capabilities TEXT[],
    is_active BOOLEAN DEFAULT true,
    configuration JSONB DEFAULT '{}',
    version VARCHAR(50),
    base_url TEXT,
    api_key_encrypted TEXT,
    description TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name, provider)
);

-- Agents table
CREATE TABLE IF NOT EXISTS agents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    model_id VARCHAR(255), -- Changed from UUID to support external model IDs
    capabilities TEXT[],
    status VARCHAR(50) DEFAULT 'available',
    configuration JSONB DEFAULT '{}',
    system_prompt TEXT,
    temperature FLOAT DEFAULT 0.7 CHECK (temperature >= 0 AND temperature <= 2),
    max_tokens INTEGER DEFAULT 4096,
    current_workload INTEGER DEFAULT 0,
    max_workload INTEGER DEFAULT 10,
    last_task_assigned_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- Contexts table
CREATE TABLE IF NOT EXISTS contexts (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    agent_id UUID REFERENCES agents(id) ON DELETE CASCADE,
    model_id UUID REFERENCES models(id),
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    token_count INTEGER DEFAULT 0,
    max_tokens INTEGER DEFAULT 100000,
    compression_enabled BOOLEAN DEFAULT true,
    compression_ratio FLOAT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP
);

-- Context items table
CREATE TABLE IF NOT EXISTS context_items (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
    type VARCHAR(50) NOT NULL,
    role VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    metadata JSONB DEFAULT '{}',
    token_count INTEGER,
    sequence_number INTEGER NOT NULL,
    is_compressed BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(context_id, sequence_number)
);

-- ==============================================================================
-- AUTHENTICATION & AUTHORIZATION
-- ==============================================================================

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(255),
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'suspended')),
    email_verified BOOLEAN DEFAULT false,
    email_verified_at TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- API Keys table
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    key_prefix VARCHAR(10) NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) DEFAULT 'standard' CHECK (type IN ('standard', 'service', 'personal', 'temporary')),
    role VARCHAR(50) DEFAULT 'user' CHECK (role IN ('admin', 'user', 'readonly', 'service')),
    scopes TEXT[],
    rate_limit INTEGER DEFAULT 1000,
    rate_window VARCHAR(10) DEFAULT '1h',
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    usage_count BIGINT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    rotated_from UUID REFERENCES api_keys(id),
    rotated_at TIMESTAMP,
    CONSTRAINT check_expiry CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- API Key Usage tracking (partitioned by month)
CREATE TABLE IF NOT EXISTS api_key_usage (
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    endpoint VARCHAR(255) NOT NULL,
    method VARCHAR(10) NOT NULL,
    status_code INTEGER,
    response_time_ms INTEGER,
    tokens_used INTEGER,
    cost_usd DECIMAL(10, 6),
    metadata JSONB DEFAULT '{}'
) PARTITION BY RANGE (timestamp);

-- Tenant configuration
CREATE TABLE IF NOT EXISTS tenant_config (
    tenant_id UUID PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    settings JSONB DEFAULT '{}',
    features JSONB DEFAULT '{}',
    limits JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- VECTOR EMBEDDINGS SYSTEM
-- ==============================================================================

-- Embedding models registry
CREATE TABLE IF NOT EXISTS embedding_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    dimensions INTEGER NOT NULL,
    max_tokens INTEGER,
    cost_per_token DECIMAL(12, 8),
    is_active BOOLEAN DEFAULT true,
    capabilities JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, model_name)
);

-- Embeddings table with vector support
CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    context_id UUID REFERENCES contexts(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    content_hash VARCHAR(64) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(50) DEFAULT 'text',
    vector vector(1536), -- Standard dimension size for text-embedding-3-small
    metadata JSONB DEFAULT '{}',
    token_count INTEGER,
    processing_time_ms INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    UNIQUE(tenant_id, content_hash, model_id)
);

-- Embedding cache for performance
CREATE TABLE IF NOT EXISTS embedding_cache (
    content_hash VARCHAR(64) NOT NULL,
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    vector vector(1536),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP + INTERVAL '30 days',
    hit_count INTEGER DEFAULT 0,
    last_hit_at TIMESTAMP,
    PRIMARY KEY (content_hash, model_id)
);

-- Agent-specific embedding configurations
CREATE TABLE IF NOT EXISTS agent_configs (
    agent_id UUID PRIMARY KEY REFERENCES agents(id) ON DELETE CASCADE,
    embedding_model_id UUID REFERENCES embedding_models(id),
    chunk_size INTEGER DEFAULT 512,
    chunk_overlap INTEGER DEFAULT 50,
    embedding_batch_size INTEGER DEFAULT 100,
    cache_ttl_hours INTEGER DEFAULT 24,
    preprocessing_config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Embedding metrics (partitioned by month)
CREATE TABLE IF NOT EXISTS embedding_metrics (
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tenant_id UUID NOT NULL,
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    tokens_processed BIGINT NOT NULL,
    vectors_created INTEGER NOT NULL,
    processing_time_ms BIGINT NOT NULL,
    estimated_cost_usd DECIMAL(12, 6),
    cache_hits INTEGER DEFAULT 0,
    cache_misses INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}'
) PARTITION BY RANGE (timestamp);

-- ==============================================================================
-- TASK MANAGEMENT & WORKFLOWS
-- ==============================================================================

-- Tasks table (partitioned by created_at)
CREATE TABLE IF NOT EXISTS tasks (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    type VARCHAR(100) NOT NULL,
    title VARCHAR(500) NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'pending',
    priority VARCHAR(20) DEFAULT 'normal',
    parent_task_id UUID,
    context_id UUID REFERENCES contexts(id),
    assigned_to UUID REFERENCES agents(id),
    created_by UUID REFERENCES agents(id),
    assigned_at TIMESTAMP,
    accepted_at TIMESTAMP,
    delegated_from VARCHAR(255),
    delegation_count INTEGER DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    tags TEXT[],
    due_date TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at),
    FOREIGN KEY (parent_task_id, created_at) REFERENCES tasks(id, created_at)
) PARTITION BY RANGE (created_at);

-- Task delegations tracking
CREATE TABLE IF NOT EXISTS task_delegations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL,
    from_agent_id UUID NOT NULL REFERENCES agents(id),
    to_agent_id UUID NOT NULL REFERENCES agents(id),
    reason TEXT,
    delegation_type VARCHAR(50),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(100) NOT NULL,
    steps JSONB NOT NULL DEFAULT '[]',
    agents UUID[] DEFAULT '{}',
    configuration JSONB DEFAULT '{}',
    coordination_mode VARCHAR(50) DEFAULT 'centralized',
    decision_strategy VARCHAR(50) DEFAULT 'majority',
    is_active BOOLEAN DEFAULT true,
    version INTEGER DEFAULT 1,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name, version)
);

-- Workflow executions
CREATE TABLE IF NOT EXISTS workflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    status VARCHAR(50) DEFAULT 'running',
    current_step INTEGER DEFAULT 0,
    context JSONB DEFAULT '{}',
    result JSONB,
    error TEXT,
    retry_count INTEGER DEFAULT 0,
    parent_execution_id UUID,
    error_details JSONB,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP
);

-- ==============================================================================
-- COLLABORATION SYSTEM
-- ==============================================================================

-- Workspaces for collaboration
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL,
    status VARCHAR(50) DEFAULT 'active',
    configuration JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_by UUID REFERENCES agents(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- Workspace members with roles
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    role VARCHAR(50) NOT NULL DEFAULT 'member',
    permissions JSONB DEFAULT '{}',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_active_at TIMESTAMP,
    PRIMARY KEY (workspace_id, agent_id)
);

-- Shared documents with CRDT support
CREATE TABLE IF NOT EXISTS shared_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    content JSONB NOT NULL DEFAULT '{}',
    version INTEGER DEFAULT 1,
    created_by UUID REFERENCES agents(id),
    locked_by UUID REFERENCES agents(id),
    locked_at TIMESTAMP,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(workspace_id, name)
);

-- Document operations for CRDT
CREATE TABLE IF NOT EXISTS document_operations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    document_id UUID NOT NULL REFERENCES shared_documents(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(id),
    operation_type VARCHAR(50) NOT NULL,
    operation_data JSONB NOT NULL,
    vector_clock JSONB NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- DYNAMIC TOOLS SYSTEM
-- ==============================================================================

-- Tool configurations
CREATE TABLE IF NOT EXISTS tool_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    tool_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    config JSONB NOT NULL,
    credentials_encrypted BYTEA,
    auth_type VARCHAR(50),
    retry_policy JSONB DEFAULT '{"max_attempts": 3, "backoff_ms": 1000}',
    status VARCHAR(50) DEFAULT 'active',
    health_status VARCHAR(50) DEFAULT 'unknown',
    last_health_check TIMESTAMP,
    last_used_at TIMESTAMP,
    base_url TEXT,
    webhook_config JSONB DEFAULT '{}',
    provider VARCHAR(100),
    passthrough_config JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    version INTEGER DEFAULT 1,
    UNIQUE(tenant_id, tool_name)
);

-- Tool discovery sessions
CREATE TABLE IF NOT EXISTS tool_discovery_sessions (
    session_id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    base_url TEXT NOT NULL,
    status VARCHAR(50) DEFAULT 'discovering',
    discovered_urls JSONB DEFAULT '[]',
    selected_url TEXT,
    discovery_metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP + INTERVAL '1 hour'
);

-- Tool executions audit
CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    tool_config_id UUID NOT NULL REFERENCES tool_configurations(id) ON DELETE CASCADE,
    agent_id UUID REFERENCES agents(id),
    action VARCHAR(255) NOT NULL,
    parameters JSONB,
    status VARCHAR(50) NOT NULL,
    result JSONB,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    response_time_ms INTEGER,
    executed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    correlation_id UUID
);

-- Tool execution retries
CREATE TABLE IF NOT EXISTS tool_execution_retries (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    execution_id UUID NOT NULL REFERENCES tool_executions(id) ON DELETE CASCADE,
    attempt_number INTEGER NOT NULL,
    error_type VARCHAR(100),
    error_message TEXT,
    backoff_ms INTEGER,
    attempted_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tool health checks
CREATE TABLE IF NOT EXISTS tool_health_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_config_id UUID NOT NULL REFERENCES tool_configurations(id) ON DELETE CASCADE,
    is_healthy BOOLEAN NOT NULL,
    response_time_ms INTEGER,
    status_code INTEGER,
    error_message TEXT,
    capabilities JSONB,
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Tool credentials
CREATE TABLE IF NOT EXISTS tool_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_config_id UUID NOT NULL REFERENCES tool_configurations(id) ON DELETE CASCADE,
    credential_type VARCHAR(50) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    encryption_key_version INTEGER DEFAULT 1,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tool_config_id, credential_type)
);

-- OpenAPI cache
CREATE TABLE IF NOT EXISTS openapi_cache (
    url TEXT PRIMARY KEY,
    spec_hash VARCHAR(64) NOT NULL,
    spec_data JSONB NOT NULL,
    version VARCHAR(50),
    title VARCHAR(255),
    description TEXT,
    discovered_actions TEXT[],
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_accessed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    cache_expires_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP + INTERVAL '7 days',
    access_count INTEGER DEFAULT 1
);

-- Tool discovery patterns (learning system)
CREATE TABLE IF NOT EXISTS tool_discovery_patterns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    domain VARCHAR(255) NOT NULL,
    successful_paths JSONB DEFAULT '[]',
    failed_paths JSONB DEFAULT '[]',
    auth_method VARCHAR(50),
    api_format VARCHAR(50),
    success_count INTEGER DEFAULT 0,
    failure_count INTEGER DEFAULT 0,
    last_success_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, domain)
);

-- Webhook events
CREATE TABLE IF NOT EXISTS webhook_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    tool_id UUID REFERENCES tool_configurations(id) ON DELETE SET NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSONB NOT NULL,
    headers JSONB DEFAULT '{}',
    source_ip INET,
    status VARCHAR(50) DEFAULT 'pending',
    processed_at TIMESTAMP,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Webhook event logs
CREATE TABLE IF NOT EXISTS webhook_event_logs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID NOT NULL REFERENCES webhook_events(id) ON DELETE CASCADE,
    action VARCHAR(100) NOT NULL,
    status VARCHAR(50) NOT NULL,
    message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Webhook dead letter queue
CREATE TABLE IF NOT EXISTS webhook_dlq (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    event_id UUID,
    event_type VARCHAR(100),
    payload JSONB NOT NULL,
    error_message TEXT,
    retry_count INTEGER DEFAULT 0,
    max_retries INTEGER DEFAULT 5,
    next_retry_at TIMESTAMP,
    status VARCHAR(50) DEFAULT 'pending',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- INTEGRATIONS & WEBHOOKS
-- ==============================================================================

-- External integrations
CREATE TABLE IF NOT EXISTS integrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    configuration JSONB NOT NULL DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'active',
    last_sync_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- Webhook configurations
CREATE TABLE IF NOT EXISTS webhook_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    organization_id UUID,
    name VARCHAR(255) NOT NULL,
    url TEXT NOT NULL,
    events TEXT[] NOT NULL,
    headers JSONB DEFAULT '{}',
    secret_encrypted TEXT,
    retry_config JSONB DEFAULT '{"max_attempts": 3, "backoff_seconds": [1, 5, 30]}',
    is_active BOOLEAN DEFAULT true,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- ==============================================================================
-- AUDIT & MONITORING
-- ==============================================================================

-- Events table for event sourcing
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_data JSONB NOT NULL,
    event_metadata JSONB DEFAULT '{}',
    source VARCHAR(100),
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP
);

-- Audit log (partitioned by created_at)
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    actor_id UUID NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    changes JSONB,
    metadata JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- ==============================================================================
-- PARTITIONS
-- ==============================================================================

-- Create initial partitions for 2025
CREATE TABLE IF NOT EXISTS api_key_usage_2025_01 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS api_key_usage_2025_02 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE IF NOT EXISTS api_key_usage_2025_03 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

CREATE TABLE IF NOT EXISTS embedding_metrics_2025_01 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS embedding_metrics_2025_02 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE IF NOT EXISTS embedding_metrics_2025_03 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

CREATE TABLE IF NOT EXISTS tasks_2025_01 PARTITION OF tasks
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS tasks_2025_02 PARTITION OF tasks
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE IF NOT EXISTS tasks_2025_03 PARTITION OF tasks
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_01 PARTITION OF audit_log
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_02 PARTITION OF audit_log
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE IF NOT EXISTS audit_log_2025_03 PARTITION OF audit_log
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- ==============================================================================
-- INDEXES
-- ==============================================================================

-- Model indexes
CREATE INDEX idx_models_tenant_id ON models(tenant_id);
CREATE INDEX idx_models_provider ON models(provider);
CREATE INDEX idx_models_type ON models(type);
CREATE INDEX idx_models_is_active ON models(is_active) WHERE is_active = true;

-- Agent indexes
CREATE INDEX idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_capabilities ON agents USING GIN(capabilities);
CREATE INDEX idx_agents_tenant_status_workload ON agents(tenant_id, status, current_workload);
CREATE INDEX idx_agents_available ON agents(status, current_workload) 
    WHERE status = 'available' AND current_workload < max_workload;

-- Context indexes
CREATE INDEX idx_contexts_tenant_id ON contexts(tenant_id);
CREATE INDEX idx_contexts_agent_id ON contexts(agent_id);
CREATE INDEX idx_contexts_status ON contexts(status);
CREATE INDEX idx_context_items_context_id ON context_items(context_id);

-- User and API Key indexes
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_active ON api_keys(is_active) WHERE is_active = true;

-- Embedding indexes
CREATE INDEX idx_embeddings_tenant_id ON embeddings(tenant_id);
CREATE INDEX idx_embeddings_context_id ON embeddings(context_id);
CREATE INDEX idx_embeddings_model_id ON embeddings(model_id);
CREATE INDEX idx_embeddings_content_hash ON embeddings(content_hash);
CREATE INDEX idx_embeddings_vector ON embeddings USING ivfflat (vector vector_cosine_ops);

-- Task indexes
CREATE INDEX idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_assigned_to ON tasks(assigned_to);
CREATE INDEX idx_tasks_created_by ON tasks(created_by);
CREATE INDEX idx_tasks_priority_status ON tasks(priority, status);
CREATE INDEX idx_tasks_tenant_status ON tasks(tenant_id, status);
CREATE INDEX idx_tasks_tenant_created_by_status ON tasks(tenant_id, created_by, status);
CREATE INDEX idx_tasks_pending ON tasks(status) WHERE status = 'pending';
CREATE INDEX idx_tasks_in_progress ON tasks(status, assigned_to) WHERE status = 'in_progress';

-- Workflow indexes
CREATE INDEX idx_workflows_tenant_id ON workflows(tenant_id);
CREATE INDEX idx_workflows_type ON workflows(type);
CREATE INDEX idx_workflows_is_active ON workflows(is_active) WHERE is_active = true;
CREATE INDEX idx_workflow_executions_workflow_id ON workflow_executions(workflow_id);
CREATE INDEX idx_workflow_executions_status ON workflow_executions(workflow_id, status);

-- Workspace indexes
CREATE INDEX idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX idx_workspaces_type ON workspaces(type);
CREATE INDEX idx_workspace_members_agent_id ON workspace_members(agent_id);
CREATE INDEX idx_shared_documents_workspace_id ON shared_documents(workspace_id);
CREATE INDEX idx_shared_documents_created_by ON shared_documents(created_by);

-- Integration indexes
CREATE INDEX idx_integrations_tenant_id ON integrations(tenant_id);
CREATE INDEX idx_integrations_type ON integrations(type);
CREATE INDEX idx_webhook_configs_tenant_id ON webhook_configs(tenant_id);
CREATE INDEX idx_webhook_configs_organization_id ON webhook_configs(organization_id);

-- Dynamic tools indexes
CREATE INDEX idx_tool_configurations_tenant_id ON tool_configurations(tenant_id);
CREATE INDEX idx_tool_configurations_status ON tool_configurations(status);
CREATE INDEX idx_tool_configurations_health_status ON tool_configurations(health_status);
CREATE INDEX idx_tool_discovery_sessions_tenant_id ON tool_discovery_sessions(tenant_id);
CREATE INDEX idx_tool_discovery_sessions_status ON tool_discovery_sessions(status);
CREATE INDEX idx_tool_discovery_sessions_expires_at ON tool_discovery_sessions(expires_at);
CREATE INDEX idx_tool_executions_tenant_id ON tool_executions(tenant_id);
CREATE INDEX idx_tool_executions_tool_config_id ON tool_executions(tool_config_id);
CREATE INDEX idx_tool_executions_status ON tool_executions(status);
CREATE INDEX idx_tool_executions_executed_at ON tool_executions(executed_at DESC);
CREATE INDEX idx_tool_health_checks_tool_config_id ON tool_health_checks(tool_config_id);
CREATE INDEX idx_tool_health_checks_checked_at ON tool_health_checks(checked_at DESC);
CREATE INDEX idx_openapi_cache_cache_expires_at ON openapi_cache(cache_expires_at);
CREATE INDEX idx_tool_discovery_patterns_tenant_domain ON tool_discovery_patterns(tenant_id, domain);
CREATE INDEX idx_webhook_events_tenant_id ON webhook_events(tenant_id);
CREATE INDEX idx_webhook_events_tool_id ON webhook_events(tool_id);
CREATE INDEX idx_webhook_events_status ON webhook_events(status);
CREATE INDEX idx_webhook_events_created_at ON webhook_events(created_at DESC);
CREATE INDEX idx_webhook_dlq_status ON webhook_dlq(status);
CREATE INDEX idx_webhook_dlq_next_retry_at ON webhook_dlq(next_retry_at);

-- Event indexes
CREATE INDEX idx_events_tenant_id ON events(tenant_id);
CREATE INDEX idx_events_aggregate ON events(aggregate_id, aggregate_type);
CREATE INDEX idx_events_created_at ON events(created_at DESC);

-- ==============================================================================
-- FUNCTIONS & TRIGGERS
-- ==============================================================================

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Apply update timestamp triggers
CREATE TRIGGER update_models_updated_at BEFORE UPDATE ON models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agents_updated_at BEFORE UPDATE ON agents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_contexts_updated_at BEFORE UPDATE ON contexts
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_users_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE ON api_keys
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workflows_updated_at BEFORE UPDATE ON workflows
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_workspaces_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_shared_documents_updated_at BEFORE UPDATE ON shared_documents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_configurations_updated_at BEFORE UPDATE ON tool_configurations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_discovery_sessions_updated_at BEFORE UPDATE ON tool_discovery_sessions
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_credentials_updated_at BEFORE UPDATE ON tool_credentials
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_discovery_patterns_updated_at BEFORE UPDATE ON tool_discovery_patterns
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_webhook_dlq_updated_at BEFORE UPDATE ON webhook_dlq
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Tenant isolation function for RLS
CREATE OR REPLACE FUNCTION current_tenant_id()
RETURNS UUID AS $$
BEGIN
    RETURN current_setting('app.tenant_id', true)::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- ==============================================================================
-- ROW LEVEL SECURITY
-- ==============================================================================

-- Enable RLS on tenant-isolated tables
ALTER TABLE models ENABLE ROW LEVEL SECURITY;
ALTER TABLE agents ENABLE ROW LEVEL SECURITY;
ALTER TABLE contexts ENABLE ROW LEVEL SECURITY;
ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE api_keys ENABLE ROW LEVEL SECURITY;
ALTER TABLE embeddings ENABLE ROW LEVEL SECURITY;
ALTER TABLE tasks ENABLE ROW LEVEL SECURITY;
ALTER TABLE workflows ENABLE ROW LEVEL SECURITY;
ALTER TABLE workspaces ENABLE ROW LEVEL SECURITY;
ALTER TABLE integrations ENABLE ROW LEVEL SECURITY;
ALTER TABLE webhook_configs ENABLE ROW LEVEL SECURITY;

-- Create RLS policies
CREATE POLICY tenant_isolation_models ON models
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_agents ON agents
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_contexts ON contexts
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_users ON users
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_api_keys ON api_keys
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_embeddings ON embeddings
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_tasks ON tasks
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_workflows ON workflows
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_workspaces ON workspaces
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_integrations ON integrations
    USING (tenant_id = current_tenant_id());

CREATE POLICY tenant_isolation_webhook_configs ON webhook_configs
    USING (tenant_id = current_tenant_id());

-- ==============================================================================
-- SEED EMBEDDING MODELS
-- ==============================================================================

INSERT INTO embedding_models (provider, model_name, dimensions, max_tokens, cost_per_token) VALUES
    ('openai', 'text-embedding-3-small', 1536, 8191, 0.00002),
    ('openai', 'text-embedding-3-large', 3072, 8191, 0.00013),
    ('openai', 'text-embedding-ada-002', 1536, 8191, 0.00010),
    ('voyage', 'voyage-2', 1024, 4000, 0.00010),
    ('voyage', 'voyage-large-2', 1536, 16000, 0.00012),
    ('amazon', 'amazon.titan-embed-text-v1', 1536, 8000, 0.00010),
    ('amazon', 'amazon.titan-embed-text-v2', 1024, 8000, 0.00002),
    ('google', 'textembedding-gecko', 768, 3072, 0.00005),
    ('google', 'textembedding-gecko-multilingual', 768, 3072, 0.00005)
ON CONFLICT (provider, model_name) DO NOTHING;

-- ==============================================================================
-- PERMISSIONS
-- ==============================================================================

-- Grant schema usage
GRANT USAGE ON SCHEMA mcp TO public;

-- Grant table permissions
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO public;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO public;

-- Grant sequence permissions
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA mcp TO public;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO public;