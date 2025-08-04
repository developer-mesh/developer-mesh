-- Rollback Initial Schema for Developer Mesh
-- This will drop all objects created in 000001_initial_schema.up.sql
-- Must be executed in reverse dependency order

-- Set search path
-- SET search_path TO mcp, public; -- Removed to avoid confusion

-- ==============================================================================
-- DROP POLICIES
-- ==============================================================================

DROP POLICY IF EXISTS tenant_isolation_events ON mcp.events;
DROP POLICY IF EXISTS tenant_isolation_integrations ON mcp.integrations;
DROP POLICY IF EXISTS tenant_isolation_workspaces ON mcp.workspaces;
DROP POLICY IF EXISTS tenant_isolation_workflows ON mcp.workflows;
DROP POLICY IF EXISTS tenant_isolation_tasks ON mcp.tasks;
DROP POLICY IF EXISTS tenant_isolation_embeddings ON mcp.embeddings;
DROP POLICY IF EXISTS tenant_isolation_api_keys ON mcp.api_keys;
DROP POLICY IF EXISTS tenant_isolation_users ON mcp.users;
DROP POLICY IF EXISTS tenant_isolation_contexts ON mcp.contexts;
DROP POLICY IF EXISTS tenant_isolation_agents ON mcp.agents;
DROP POLICY IF EXISTS tenant_isolation_models ON mcp.models;

-- ==============================================================================
-- DISABLE ROW LEVEL SECURITY
-- ==============================================================================

ALTER TABLE IF EXISTS mcp.webhook_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.integrations DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.workspaces DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.workflows DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.tasks DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.embeddings DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.api_keys DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.users DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.contexts DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.agents DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.models DISABLE ROW LEVEL SECURITY;

-- ==============================================================================
-- DROP TRIGGERS
-- ==============================================================================

DROP TRIGGER IF EXISTS update_embeddings_tsvector ON mcp.embeddings;
DROP TRIGGER IF EXISTS update_agent_configs_updated_at ON mcp.agent_configs;
DROP TRIGGER IF EXISTS update_tenant_config_updated_at ON mcp.tenant_config;
DROP TRIGGER IF EXISTS update_embedding_models_updated_at ON mcp.embedding_models;
DROP TRIGGER IF EXISTS update_webhook_configs_updated_at ON mcp.webhook_configs;
DROP TRIGGER IF EXISTS update_integrations_updated_at ON mcp.integrations;
DROP TRIGGER IF EXISTS update_shared_documents_updated_at ON mcp.shared_documents;
DROP TRIGGER IF EXISTS update_workspaces_updated_at ON mcp.workspaces;
DROP TRIGGER IF EXISTS update_workflows_updated_at ON mcp.workflows;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON mcp.api_keys;
DROP TRIGGER IF EXISTS update_users_updated_at ON mcp.users;
DROP TRIGGER IF EXISTS update_contexts_updated_at ON mcp.contexts;
DROP TRIGGER IF EXISTS update_agents_updated_at ON mcp.agents;
DROP TRIGGER IF EXISTS update_models_updated_at ON mcp.models;
DROP TRIGGER IF EXISTS update_tasks_updated_at ON mcp.tasks;

-- ==============================================================================
-- DROP INDEXES
-- ==============================================================================

-- Event indexes
DROP INDEX IF EXISTS idx_events_created_at;
DROP INDEX IF EXISTS idx_events_aggregate;
DROP INDEX IF EXISTS idx_events_tenant_id;

-- Integration indexes
DROP INDEX IF EXISTS idx_webhook_configs_active;
DROP INDEX IF EXISTS idx_webhook_configs_integration;
DROP INDEX IF EXISTS idx_integrations_tenant_id;

-- Workspace indexes
DROP INDEX IF EXISTS idx_workspace_activities_actor;
DROP INDEX IF EXISTS idx_workspace_activities_workspace;
DROP INDEX IF EXISTS idx_workspace_members_user;
DROP INDEX IF EXISTS idx_workspaces_tenant_id;

-- Workflow indexes
DROP INDEX IF EXISTS idx_workflow_executions_status;
DROP INDEX IF EXISTS idx_workflow_executions_workflow;
DROP INDEX IF EXISTS idx_workflows_status;
DROP INDEX IF EXISTS idx_workflows_tenant_id;

-- Task delegation indexes
DROP INDEX IF EXISTS idx_task_idempotency_expires;
DROP INDEX IF EXISTS idx_task_state_transitions_task;
DROP INDEX IF EXISTS idx_task_delegation_history_task;
DROP INDEX IF EXISTS idx_task_delegations_to;
DROP INDEX IF EXISTS idx_task_delegations_from;
DROP INDEX IF EXISTS idx_task_delegations_task;

-- Task indexes
DROP INDEX IF EXISTS idx_tasks_parent;
DROP INDEX IF EXISTS idx_tasks_priority;
DROP INDEX IF EXISTS idx_tasks_status;
DROP INDEX IF EXISTS idx_tasks_agent_id;
DROP INDEX IF EXISTS idx_tasks_tenant_id;

-- Embedding indexes
DROP INDEX IF EXISTS idx_embeddings_task_type;
DROP INDEX IF EXISTS idx_embeddings_agent_id;
DROP INDEX IF EXISTS idx_embeddings_fts;
DROP INDEX IF EXISTS idx_embeddings_normalized_ivfflat;
DROP INDEX IF EXISTS idx_embeddings_vector;
DROP INDEX IF EXISTS idx_embeddings_content_hash;
DROP INDEX IF EXISTS idx_embeddings_model_id;
DROP INDEX IF EXISTS idx_embeddings_context_id;
DROP INDEX IF EXISTS idx_embeddings_tenant_id;

-- API Key indexes
DROP INDEX IF EXISTS idx_api_keys_parent;
DROP INDEX IF EXISTS idx_api_keys_key_type;
DROP INDEX IF EXISTS idx_api_keys_active;
DROP INDEX IF EXISTS idx_api_keys_key_prefix;
DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_api_keys_tenant_id;

-- User indexes
DROP INDEX IF EXISTS idx_users_email;
DROP INDEX IF EXISTS idx_users_tenant_id;

-- Context indexes
DROP INDEX IF EXISTS idx_contexts_status;
DROP INDEX IF EXISTS idx_contexts_agent_id;
DROP INDEX IF EXISTS idx_contexts_tenant_id;

-- Agent indexes
DROP INDEX IF EXISTS idx_agents_workload;
DROP INDEX IF EXISTS idx_agents_status;
DROP INDEX IF EXISTS idx_agents_tenant_id;

-- Agent config indexes
DROP INDEX IF EXISTS idx_agent_configs_active;

-- Model indexes
DROP INDEX IF EXISTS idx_models_provider;
DROP INDEX IF EXISTS idx_models_tenant_id;

-- ==============================================================================
-- DROP TABLES (in reverse dependency order)
-- ==============================================================================

-- Drop partitioned tables first (children before parents)
DROP TABLE IF EXISTS mcp.embedding_metrics_2025_03;
DROP TABLE IF EXISTS mcp.embedding_metrics_2025_02;
DROP TABLE IF EXISTS mcp.embedding_metrics_2025_01;
DROP TABLE IF EXISTS mcp.audit_log_2025_03;
DROP TABLE IF EXISTS mcp.audit_log_2025_02;
DROP TABLE IF EXISTS mcp.audit_log_2025_01;
DROP TABLE IF EXISTS mcp.tasks_2025_03;
DROP TABLE IF EXISTS mcp.tasks_2025_02;
DROP TABLE IF EXISTS mcp.tasks_2025_01;
DROP TABLE IF EXISTS mcp.api_key_usage_2025_03;
DROP TABLE IF EXISTS mcp.api_key_usage_2025_02;
DROP TABLE IF EXISTS mcp.api_key_usage_2025_01;

-- Drop monitoring tables
DROP TABLE IF EXISTS mcp.audit_log CASCADE;
DROP TABLE IF EXISTS mcp.events CASCADE;

-- Drop integration tables
DROP TABLE IF EXISTS mcp.webhook_configs CASCADE;
DROP TABLE IF EXISTS mcp.integrations CASCADE;

-- Drop collaboration tables
DROP TABLE IF EXISTS mcp.shared_documents CASCADE;
DROP TABLE IF EXISTS mcp.workspace_activities CASCADE;
DROP TABLE IF EXISTS mcp.workspace_members CASCADE;
DROP TABLE IF EXISTS mcp.workspaces CASCADE;

-- Drop workflow tables
DROP TABLE IF EXISTS mcp.workflow_executions CASCADE;
DROP TABLE IF EXISTS mcp.workflows CASCADE;

-- Drop task management tables
DROP TABLE IF EXISTS mcp.task_idempotency_keys CASCADE;
DROP TABLE IF EXISTS mcp.task_state_transitions CASCADE;
DROP TABLE IF EXISTS mcp.task_delegation_history CASCADE;
DROP TABLE IF EXISTS mcp.task_delegations CASCADE;
DROP TABLE IF EXISTS mcp.tasks CASCADE;

-- Drop embedding system tables
DROP TABLE IF EXISTS mcp.embedding_metrics CASCADE;
DROP TABLE IF EXISTS mcp.agent_configs CASCADE;
DROP TABLE IF EXISTS mcp.projection_matrices CASCADE;
DROP TABLE IF EXISTS mcp.embedding_statistics CASCADE;
DROP TABLE IF EXISTS mcp.embedding_cache CASCADE;
DROP TABLE IF EXISTS mcp.embeddings CASCADE;
DROP TABLE IF EXISTS mcp.embedding_models CASCADE;

-- Drop auth tables
DROP TABLE IF EXISTS mcp.tenant_config CASCADE;
DROP TABLE IF EXISTS mcp.api_key_usage CASCADE;
DROP TABLE IF EXISTS mcp.api_keys CASCADE;
DROP TABLE IF EXISTS mcp.users CASCADE;

-- Drop foundation tables
DROP TABLE IF EXISTS mcp.context_items CASCADE;
DROP TABLE IF EXISTS mcp.contexts CASCADE;
DROP TABLE IF EXISTS mcp.agents CASCADE;
DROP TABLE IF EXISTS mcp.models CASCADE;

-- ==============================================================================
-- DROP FUNCTIONS
-- ==============================================================================

DROP FUNCTION IF EXISTS jsonb_merge_recursive(JSONB, JSONB);
DROP FUNCTION IF EXISTS update_content_tsvector();
DROP FUNCTION IF EXISTS bm25_score(FLOAT, INTEGER, INTEGER, INTEGER, FLOAT, FLOAT, FLOAT);
DROP FUNCTION IF EXISTS current_tenant_id();
DROP FUNCTION IF EXISTS update_updated_at_column();

-- ==============================================================================
-- DROP TYPES
-- ==============================================================================

DROP TYPE IF EXISTS mcp.member_role CASCADE;
DROP TYPE IF EXISTS mcp.workspace_visibility CASCADE;
DROP TYPE IF EXISTS mcp.delegation_type CASCADE;
DROP TYPE IF EXISTS mcp.workflow_status CASCADE;
DROP TYPE IF EXISTS mcp.workflow_type CASCADE;
DROP TYPE IF EXISTS mcp.task_priority CASCADE;
DROP TYPE IF EXISTS mcp.task_status CASCADE;

-- ==============================================================================
-- DROP SCHEMA (optional)
-- ==============================================================================

-- Don't drop the schema if other objects might exist
-- DROP SCHEMA IF EXISTS mcp CASCADE;

-- ==============================================================================
-- DROP EXTENSIONS (optional - usually keep these)
-- ==============================================================================

-- Uncomment if you want to remove extensions too
-- DROP EXTENSION IF EXISTS "vector";
-- DROP EXTENSION IF EXISTS "pgcrypto";
-- DROP EXTENSION IF EXISTS "uuid-ossp";

-- End of rollback