-- Initial Schema for Developer Mesh
-- Consolidated from 26 migrations into a single clean schema
-- Created: 2025-08-02
-- Updated: 2025-08-02 - Added missing types, tables, columns, and functions

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Create MCP schema for core platform
CREATE SCHEMA IF NOT EXISTS mcp;

-- Set search path
SET search_path TO mcp, public;

-- ==============================================================================
-- CUSTOM TYPES
-- ==============================================================================

-- Task management types
CREATE TYPE task_status AS ENUM ('pending', 'assigned', 'in_progress', 'completed', 'failed', 'cancelled', 'delegated');
CREATE TYPE task_priority AS ENUM ('low', 'medium', 'high', 'critical');

-- Workflow types
CREATE TYPE workflow_type AS ENUM ('sequential', 'parallel', 'conditional', 'loop', 'map_reduce', 'scatter_gather');
CREATE TYPE workflow_status AS ENUM ('draft', 'active', 'paused', 'completed', 'failed', 'archived');

-- Delegation types
CREATE TYPE delegation_type AS ENUM ('handoff', 'collaboration', 'supervision', 'consultation');

-- Workspace types
CREATE TYPE workspace_visibility AS ENUM ('private', 'team', 'organization', 'public');
CREATE TYPE member_role AS ENUM ('viewer', 'contributor', 'moderator', 'admin', 'owner');

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
    last_seen_at TIMESTAMP WITH TIME ZONE, -- Added from gap analysis
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

-- API Keys table (FIXED: type -> key_type)
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    key_prefix VARCHAR(10) NOT NULL,
    name VARCHAR(255) NOT NULL,
    key_type VARCHAR(50) DEFAULT 'user' CHECK (key_type IN ('user', 'admin', 'agent', 'service', 'gateway')),
    role VARCHAR(50) DEFAULT 'user' CHECK (role IN ('admin', 'user', 'readonly', 'service')),
    scopes TEXT[],
    rate_limit INTEGER DEFAULT 1000,
    rate_window VARCHAR(10) DEFAULT '1h',
    is_active BOOLEAN DEFAULT true,
    last_used_at TIMESTAMP,
    usage_count BIGINT DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    parent_key_id UUID REFERENCES api_keys(id), -- Added from gap analysis
    allowed_services TEXT[] DEFAULT '{}', -- Added from gap analysis
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

-- Tenant configuration (UPDATED from gap analysis)
CREATE TABLE IF NOT EXISTS tenant_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID UNIQUE NOT NULL,
    name VARCHAR(255) NOT NULL,
    settings JSONB DEFAULT '{}',
    features JSONB DEFAULT '{}',
    limits JSONB DEFAULT '{}',
    rate_limit_config JSONB NOT NULL DEFAULT '{}', -- Added from gap analysis
    service_tokens JSONB DEFAULT '{}', -- Added from gap analysis
    allowed_origins TEXT[] DEFAULT '{}', -- Added from gap analysis
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
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

-- Embeddings table with vector support (UPDATED with missing columns)
CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    context_id UUID REFERENCES contexts(id) ON DELETE CASCADE,
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    content_hash VARCHAR(64) NOT NULL,
    content TEXT NOT NULL,
    content_type VARCHAR(50) DEFAULT 'text',
    vector vector(1536), -- Standard dimension size for text-embedding-3-small
    normalized_embedding vector(1536), -- Added from gap analysis
    metadata JSONB DEFAULT '{}',
    token_count INTEGER,
    processing_time_ms INTEGER,
    agent_id VARCHAR(255), -- Added from gap analysis
    task_type VARCHAR(50), -- Added from gap analysis
    cost_usd DECIMAL(10, 6), -- Added from gap analysis
    generation_time_ms INTEGER, -- Added from gap analysis
    content_tsvector tsvector, -- Added from gap analysis
    term_frequencies jsonb, -- Added from gap analysis
    document_length integer, -- Added from gap analysis
    idf_scores jsonb, -- Added from gap analysis
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    UNIQUE(tenant_id, content_hash, model_id)
);

-- Embedding cache for performance (Already exists, keeping as is)
CREATE TABLE IF NOT EXISTS embedding_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    content_hash VARCHAR(64) NOT NULL,
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    embedding vector(4096) NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP,
    access_count INTEGER DEFAULT 0,
    last_accessed TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(content_hash, model_id)
);

-- Embedding statistics for hybrid search (Added from gap analysis)
CREATE TABLE IF NOT EXISTS embedding_statistics (
    collection_id VARCHAR(255) PRIMARY KEY,
    total_documents INTEGER NOT NULL DEFAULT 0,
    avg_document_length FLOAT NOT NULL DEFAULT 0.0,
    term_document_counts JSONB NOT NULL DEFAULT '{}',
    last_updated TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Projection matrices for cross-model compatibility (Added from gap analysis)
CREATE TABLE IF NOT EXISTS projection_matrices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_model_id UUID NOT NULL REFERENCES models(id),
    target_model_id UUID NOT NULL REFERENCES models(id),
    source_dimension INTEGER NOT NULL,
    target_dimension INTEGER NOT NULL,
    matrix_data BYTEA NOT NULL,
    training_loss FLOAT,
    validation_loss FLOAT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_model_id, target_model_id)
);

-- Agent configs for embeddings with versioning (Updated to match Go code expectations)
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES agents(id),
    version INTEGER NOT NULL DEFAULT 1,
    
    -- Configuration
    embedding_strategy VARCHAR(50) NOT NULL DEFAULT 'balanced' CHECK (embedding_strategy IN ('balanced', 'quality', 'speed', 'cost')),
    model_preferences JSONB NOT NULL DEFAULT '[]',
    constraints JSONB NOT NULL DEFAULT '{}',
    fallback_behavior JSONB NOT NULL DEFAULT '{}',
    
    -- Legacy columns for compatibility
    embedding_model_id UUID REFERENCES models(id),
    embedding_config JSONB DEFAULT '{}',
    cost_limit_usd DECIMAL(10, 2),
    rate_limit_per_minute INTEGER,
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    
    -- Audit
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    -- Constraints
    CONSTRAINT unique_agent_version UNIQUE(agent_id, version),
    CONSTRAINT valid_model_preferences CHECK (jsonb_typeof(model_preferences) = 'array'),
    CONSTRAINT valid_constraints CHECK (jsonb_typeof(constraints) = 'object'),
    CONSTRAINT valid_fallback CHECK (jsonb_typeof(fallback_behavior) = 'object')
);

-- Embedding metrics for monitoring (Added from gap analysis)
CREATE TABLE IF NOT EXISTS embedding_metrics (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    model_id UUID NOT NULL REFERENCES models(id),
    timestamp TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    tokens_used INTEGER NOT NULL,
    cost_usd DECIMAL(10, 6) NOT NULL,
    latency_ms INTEGER NOT NULL,
    batch_size INTEGER DEFAULT 1,
    success BOOLEAN DEFAULT true,
    error_message TEXT,
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- ==============================================================================
-- TASK MANAGEMENT SYSTEM
-- ==============================================================================

-- Tasks table (partitioned by created_at) - UPDATED with missing columns
CREATE TABLE IF NOT EXISTS tasks (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    parent_task_id UUID,
    agent_id UUID REFERENCES agents(id),
    status task_status DEFAULT 'pending',
    priority task_priority DEFAULT 'medium',
    title VARCHAR(255) NOT NULL,
    description TEXT,
    input_data JSONB DEFAULT '{}',
    output_data JSONB DEFAULT '{}',
    error_message TEXT,
    max_retries INTEGER DEFAULT 3,
    retry_count INTEGER DEFAULT 0,
    assigned_at TIMESTAMP,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    deadline TIMESTAMP,
    auto_escalate BOOLEAN DEFAULT FALSE, -- Added from gap analysis
    escalation_timeout INTERVAL, -- Added from gap analysis
    max_delegations INTEGER DEFAULT 3, -- Added from gap analysis
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Task delegations
CREATE TABLE IF NOT EXISTS task_delegations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_id UUID NOT NULL REFERENCES agents(id),
    delegation_type delegation_type,
    status VARCHAR(50) DEFAULT 'pending',
    reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    accepted_at TIMESTAMP,
    completed_at TIMESTAMP
);

-- Task delegation history (Added from gap analysis)
CREATE TABLE IF NOT EXISTS task_delegation_history (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL,
    from_agent_id UUID REFERENCES agents(id),
    to_agent_id UUID NOT NULL REFERENCES agents(id),
    delegation_type delegation_type,
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Task state transitions (Added from gap analysis)
CREATE TABLE IF NOT EXISTS task_state_transitions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    task_id UUID NOT NULL,
    from_status task_status,
    to_status task_status NOT NULL,
    transitioned_by UUID REFERENCES agents(id),
    reason TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Task idempotency keys (Added from gap analysis)
CREATE TABLE IF NOT EXISTS task_idempotency_keys (
    idempotency_key VARCHAR(255) PRIMARY KEY,
    task_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL
);

-- ==============================================================================
-- WORKFLOW SYSTEM
-- ==============================================================================

-- Workflows table
CREATE TABLE IF NOT EXISTS workflows (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    type workflow_type NOT NULL,
    status workflow_status DEFAULT 'draft',
    definition JSONB NOT NULL,
    configuration JSONB DEFAULT '{}',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- Workflow executions
CREATE TABLE IF NOT EXISTS workflow_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workflow_id UUID NOT NULL REFERENCES workflows(id),
    status VARCHAR(50) DEFAULT 'running',
    input_data JSONB DEFAULT '{}',
    output_data JSONB DEFAULT '{}',
    error_message TEXT,
    started_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP,
    metadata JSONB DEFAULT '{}'
);

-- ==============================================================================
-- COLLABORATION SYSTEM
-- ==============================================================================

-- Workspaces table - UPDATED with missing columns
CREATE TABLE IF NOT EXISTS workspaces (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    visibility workspace_visibility DEFAULT 'private',
    configuration JSONB DEFAULT '{}',
    max_members INTEGER DEFAULT 50, -- Added from gap analysis
    max_storage_bytes BIGINT DEFAULT 1073741824, -- Added from gap analysis (1GB)
    current_storage_bytes BIGINT DEFAULT 0, -- Added from gap analysis
    max_documents INTEGER DEFAULT 1000, -- Added from gap analysis
    current_documents INTEGER DEFAULT 0, -- Added from gap analysis
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    archived_at TIMESTAMP,
    UNIQUE(tenant_id, name)
);

-- Workspace members
CREATE TABLE IF NOT EXISTS workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role member_role DEFAULT 'viewer',
    permissions JSONB DEFAULT '{}',
    joined_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    invited_by UUID REFERENCES users(id),
    PRIMARY KEY (workspace_id, user_id)
);

-- Workspace activities (Added from gap analysis)
CREATE TABLE IF NOT EXISTS workspace_activities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    actor_id UUID NOT NULL REFERENCES agents(id),
    action VARCHAR(100) NOT NULL,
    target_type VARCHAR(50),
    target_id UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Shared documents
CREATE TABLE IF NOT EXISTS shared_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    content TEXT,
    content_type VARCHAR(50) DEFAULT 'markdown',
    version INTEGER DEFAULT 1,
    vector_clock JSONB DEFAULT '{}',
    created_by UUID REFERENCES users(id),
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- INTEGRATION SYSTEM
-- ==============================================================================

-- External integrations
CREATE TABLE IF NOT EXISTS integrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(100) NOT NULL,
    configuration JSONB NOT NULL DEFAULT '{}',
    credentials_encrypted TEXT,
    status VARCHAR(50) DEFAULT 'active',
    last_sync_at TIMESTAMP,
    sync_frequency_minutes INTEGER,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, name, type)
);

-- Webhook configurations
CREATE TABLE IF NOT EXISTS webhook_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    integration_id UUID REFERENCES integrations(id) ON DELETE CASCADE,
    url TEXT NOT NULL,
    secret_encrypted TEXT,
    events TEXT[] NOT NULL,
    is_active BOOLEAN DEFAULT true,
    retry_config JSONB DEFAULT '{"max_retries": 3, "retry_delay_ms": 1000}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- ==============================================================================
-- MONITORING & ANALYTICS
-- ==============================================================================

-- Events table for event sourcing
CREATE TABLE IF NOT EXISTS events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    aggregate_id UUID NOT NULL,
    aggregate_type VARCHAR(100) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version INTEGER DEFAULT 1,
    event_data JSONB NOT NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID
);

-- Audit log (partitioned by created_at)
CREATE TABLE IF NOT EXISTS audit_log (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    entity_type VARCHAR(50) NOT NULL,
    entity_id UUID NOT NULL,
    action VARCHAR(50) NOT NULL,
    actor_type VARCHAR(50) NOT NULL,
    actor_id UUID NOT NULL,
    changes JSONB,
    metadata JSONB DEFAULT '{}',
    ip_address INET,
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- ==============================================================================
-- UTILITY FUNCTIONS
-- ==============================================================================

-- Update timestamp trigger function
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Current tenant ID function for RLS (Added from gap analysis)
CREATE OR REPLACE FUNCTION current_tenant_id() RETURNS UUID AS $$
BEGIN
    RETURN current_setting('app.tenant_id', true)::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql;

-- BM25 scoring function (Added from gap analysis)
CREATE OR REPLACE FUNCTION bm25_score(
    term_freq FLOAT,
    doc_freq INTEGER,
    total_docs INTEGER,
    doc_length INTEGER,
    avg_doc_length FLOAT,
    k1 FLOAT DEFAULT 1.2,
    b FLOAT DEFAULT 0.75
) RETURNS FLOAT AS $$
BEGIN
    IF doc_freq = 0 OR total_docs = 0 OR avg_doc_length = 0 THEN
        RETURN 0;
    END IF;
    
    RETURN ((term_freq * (k1 + 1)) / 
            (term_freq + k1 * (1 - b + b * (doc_length / avg_doc_length)))) *
           ln((total_docs - doc_freq + 0.5) / (doc_freq + 0.5));
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- Update content TSVector function (Added from gap analysis)
CREATE OR REPLACE FUNCTION update_content_tsvector() RETURNS trigger AS $$
BEGIN
    NEW.content_tsvector := to_tsvector('english', COALESCE(NEW.content, ''));
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- JSONB merge recursive function (Added from gap analysis)
CREATE OR REPLACE FUNCTION jsonb_merge_recursive(target JSONB, source JSONB) 
RETURNS JSONB AS $$
BEGIN
    IF jsonb_typeof(target) = 'object' AND jsonb_typeof(source) = 'object' THEN
        RETURN (
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
        );
    ELSE
        RETURN source;
    END IF;
END;
$$ LANGUAGE plpgsql IMMUTABLE;

-- ==============================================================================
-- PARTITIONS
-- ==============================================================================

-- Create initial partitions for api_key_usage
CREATE TABLE api_key_usage_2025_01 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE api_key_usage_2025_02 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE api_key_usage_2025_03 PARTITION OF api_key_usage
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Create initial partitions for tasks
CREATE TABLE tasks_2025_01 PARTITION OF tasks
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE tasks_2025_02 PARTITION OF tasks
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE tasks_2025_03 PARTITION OF tasks
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Create initial partitions for audit_log
CREATE TABLE audit_log_2025_01 PARTITION OF audit_log
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE audit_log_2025_02 PARTITION OF audit_log
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE audit_log_2025_03 PARTITION OF audit_log
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Create initial partitions for embedding_metrics
CREATE TABLE embedding_metrics_2025_01 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE embedding_metrics_2025_02 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE embedding_metrics_2025_03 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- ==============================================================================
-- INDEXES
-- ==============================================================================

-- Model indexes
CREATE INDEX idx_models_tenant_id ON models(tenant_id);
CREATE INDEX idx_models_provider ON models(provider) WHERE is_active = true;

-- Agent indexes
CREATE INDEX idx_agents_tenant_id ON agents(tenant_id);
CREATE INDEX idx_agents_status ON agents(status);
CREATE INDEX idx_agents_workload ON agents(current_workload) WHERE status = 'available';

-- Agent config indexes
CREATE INDEX idx_agent_configs_active ON agent_configs(agent_id, version DESC) WHERE is_active = true;

-- Context indexes
CREATE INDEX idx_contexts_tenant_id ON contexts(tenant_id);
CREATE INDEX idx_contexts_agent_id ON contexts(agent_id);
CREATE INDEX idx_contexts_status ON contexts(status);

-- User indexes
CREATE INDEX idx_users_tenant_id ON users(tenant_id);
CREATE INDEX idx_users_email ON users(email);

-- API Key indexes
CREATE INDEX idx_api_keys_tenant_id ON api_keys(tenant_id);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_prefix ON api_keys(key_prefix);
CREATE INDEX idx_api_keys_active ON api_keys(is_active) WHERE is_active = true;
CREATE INDEX idx_api_keys_key_type ON api_keys(key_type, tenant_id) WHERE is_active = true; -- Added from gap analysis
CREATE INDEX idx_api_keys_parent ON api_keys(parent_key_id) WHERE parent_key_id IS NOT NULL; -- Added from gap analysis

-- Embedding indexes
CREATE INDEX idx_embeddings_tenant_id ON embeddings(tenant_id);
CREATE INDEX idx_embeddings_context_id ON embeddings(context_id);
CREATE INDEX idx_embeddings_model_id ON embeddings(model_id);
CREATE INDEX idx_embeddings_content_hash ON embeddings(content_hash);
CREATE INDEX idx_embeddings_vector ON embeddings USING ivfflat (vector vector_cosine_ops);
CREATE INDEX idx_embeddings_normalized_ivfflat ON embeddings USING ivfflat (normalized_embedding vector_cosine_ops); -- Added
CREATE INDEX idx_embeddings_fts ON embeddings USING gin(content_tsvector); -- Added
CREATE INDEX idx_embeddings_agent_id ON embeddings(agent_id); -- Added
CREATE INDEX idx_embeddings_task_type ON embeddings(task_type); -- Added

-- Task indexes
CREATE INDEX idx_tasks_tenant_id ON tasks(tenant_id);
CREATE INDEX idx_tasks_agent_id ON tasks(agent_id);
CREATE INDEX idx_tasks_status ON tasks(status);
CREATE INDEX idx_tasks_priority ON tasks(priority) WHERE status IN ('pending', 'assigned');
CREATE INDEX idx_tasks_parent ON tasks(parent_task_id) WHERE parent_task_id IS NOT NULL;

-- Task delegation indexes
CREATE INDEX idx_task_delegations_task ON task_delegations(task_id);
CREATE INDEX idx_task_delegations_from ON task_delegations(from_agent_id);
CREATE INDEX idx_task_delegations_to ON task_delegations(to_agent_id);
CREATE INDEX idx_task_delegation_history_task ON task_delegation_history(task_id); -- Added
CREATE INDEX idx_task_state_transitions_task ON task_state_transitions(task_id); -- Added
CREATE INDEX idx_task_idempotency_expires ON task_idempotency_keys(expires_at); -- Added

-- Workflow indexes
CREATE INDEX idx_workflows_tenant_id ON workflows(tenant_id);
CREATE INDEX idx_workflows_status ON workflows(status);
CREATE INDEX idx_workflow_executions_workflow ON workflow_executions(workflow_id);
CREATE INDEX idx_workflow_executions_status ON workflow_executions(status);

-- Workspace indexes
CREATE INDEX idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX idx_workspace_members_user ON workspace_members(user_id);
CREATE INDEX idx_workspace_activities_workspace ON workspace_activities(workspace_id); -- Added
CREATE INDEX idx_workspace_activities_actor ON workspace_activities(actor_id); -- Added

-- Integration indexes
CREATE INDEX idx_integrations_tenant_id ON integrations(tenant_id);
CREATE INDEX idx_webhook_configs_integration ON webhook_configs(integration_id);
CREATE INDEX idx_webhook_configs_active ON webhook_configs(is_active) WHERE is_active = true;

-- Event indexes
CREATE INDEX idx_events_tenant_id ON events(tenant_id);
CREATE INDEX idx_events_aggregate ON events(aggregate_id, aggregate_type);
CREATE INDEX idx_events_created_at ON events(created_at DESC);

-- ==============================================================================
-- TRIGGERS
-- ==============================================================================

-- Update timestamp triggers
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

CREATE TRIGGER update_integrations_updated_at BEFORE UPDATE ON integrations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_webhook_configs_updated_at BEFORE UPDATE ON webhook_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_embedding_models_updated_at BEFORE UPDATE ON embedding_models
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tenant_config_updated_at BEFORE UPDATE ON tenant_config
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_agent_configs_updated_at BEFORE UPDATE ON agent_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Content TSVector update trigger (Added from gap analysis)
CREATE TRIGGER update_embeddings_tsvector 
    BEFORE INSERT OR UPDATE OF content ON embeddings
    FOR EACH ROW 
    EXECUTE FUNCTION update_content_tsvector();

-- ==============================================================================
-- ROW LEVEL SECURITY (RLS)
-- ==============================================================================

-- Enable RLS on tenant-scoped tables
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

CREATE POLICY tenant_isolation_events ON events
    USING (tenant_id = current_tenant_id());

-- ==============================================================================
-- INITIAL DATA
-- ==============================================================================

-- Insert default embedding models
INSERT INTO embedding_models (provider, model_name, dimensions, max_tokens, cost_per_token) VALUES
    ('openai', 'text-embedding-3-small', 1536, 8192, 0.00002),
    ('openai', 'text-embedding-3-large', 3072, 8192, 0.00013),
    ('amazon', 'amazon.titan-embed-text-v1', 1536, 8192, 0.00001),
    ('google', 'text-embedding-004', 768, 2048, 0.00001)
ON CONFLICT (provider, model_name) DO NOTHING;

-- ==============================================================================
-- GRANTS (adjust based on your user requirements)
-- ==============================================================================

-- Grant usage on schema
GRANT USAGE ON SCHEMA mcp TO PUBLIC;

-- Grant appropriate permissions on tables (customize as needed)
-- Example: GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA mcp TO your_app_user;

-- End of schema creation