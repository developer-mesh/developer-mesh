-- Rollback Initial Schema for Developer Mesh
-- This will drop all objects created in 000001_initial_schema.up.sql
-- Must be executed in reverse dependency order

-- Set search path
SET search_path TO mcp, public;

-- ==============================================================================
-- DROP POLICIES
-- ==============================================================================

DROP POLICY IF EXISTS tenant_isolation_events ON events;
DROP POLICY IF EXISTS tenant_isolation_integrations ON integrations;
DROP POLICY IF EXISTS tenant_isolation_workspaces ON workspaces;
DROP POLICY IF EXISTS tenant_isolation_workflows ON workflows;
DROP POLICY IF EXISTS tenant_isolation_tasks ON tasks;
DROP POLICY IF EXISTS tenant_isolation_embeddings ON embeddings;
DROP POLICY IF EXISTS tenant_isolation_api_keys ON api_keys;
DROP POLICY IF EXISTS tenant_isolation_users ON users;
DROP POLICY IF EXISTS tenant_isolation_contexts ON contexts;
DROP POLICY IF EXISTS tenant_isolation_agents ON agents;
DROP POLICY IF EXISTS tenant_isolation_models ON models;

-- ==============================================================================
-- DISABLE ROW LEVEL SECURITY
-- ==============================================================================

ALTER TABLE IF EXISTS webhook_configs DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS integrations DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS workspaces DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS workflows DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS tasks DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS embeddings DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS api_keys DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS users DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS contexts DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS agents DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS models DISABLE ROW LEVEL SECURITY;

-- ==============================================================================
-- DROP TRIGGERS
-- ==============================================================================

DROP TRIGGER IF EXISTS update_embeddings_tsvector ON embeddings;
DROP TRIGGER IF EXISTS update_agent_configs_updated_at ON agent_configs;
DROP TRIGGER IF EXISTS update_tenant_config_updated_at ON tenant_config;
DROP TRIGGER IF EXISTS update_embedding_models_updated_at ON embedding_models;
DROP TRIGGER IF EXISTS update_webhook_configs_updated_at ON webhook_configs;
DROP TRIGGER IF EXISTS update_integrations_updated_at ON integrations;
DROP TRIGGER IF EXISTS update_shared_documents_updated_at ON shared_documents;
DROP TRIGGER IF EXISTS update_workspaces_updated_at ON workspaces;
DROP TRIGGER IF EXISTS update_workflows_updated_at ON workflows;
DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
DROP TRIGGER IF EXISTS update_contexts_updated_at ON contexts;
DROP TRIGGER IF EXISTS update_agents_updated_at ON agents;
DROP TRIGGER IF EXISTS update_models_updated_at ON models;
DROP TRIGGER IF EXISTS update_tasks_updated_at ON tasks;

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
DROP TABLE IF EXISTS embedding_metrics_2025_03;
DROP TABLE IF EXISTS embedding_metrics_2025_02;
DROP TABLE IF EXISTS embedding_metrics_2025_01;
DROP TABLE IF EXISTS audit_log_2025_03;
DROP TABLE IF EXISTS audit_log_2025_02;
DROP TABLE IF EXISTS audit_log_2025_01;
DROP TABLE IF EXISTS tasks_2025_03;
DROP TABLE IF EXISTS tasks_2025_02;
DROP TABLE IF EXISTS tasks_2025_01;
DROP TABLE IF EXISTS api_key_usage_2025_03;
DROP TABLE IF EXISTS api_key_usage_2025_02;
DROP TABLE IF EXISTS api_key_usage_2025_01;

-- Drop monitoring tables
DROP TABLE IF EXISTS audit_log CASCADE;
DROP TABLE IF EXISTS events CASCADE;

-- Drop integration tables
DROP TABLE IF EXISTS webhook_configs CASCADE;
DROP TABLE IF EXISTS integrations CASCADE;

-- Drop collaboration tables
DROP TABLE IF EXISTS shared_documents CASCADE;
DROP TABLE IF EXISTS workspace_activities CASCADE;
DROP TABLE IF EXISTS workspace_members CASCADE;
DROP TABLE IF EXISTS workspaces CASCADE;

-- Drop workflow tables
DROP TABLE IF EXISTS workflow_executions CASCADE;
DROP TABLE IF EXISTS workflows CASCADE;

-- Drop task management tables
DROP TABLE IF EXISTS task_idempotency_keys CASCADE;
DROP TABLE IF EXISTS task_state_transitions CASCADE;
DROP TABLE IF EXISTS task_delegation_history CASCADE;
DROP TABLE IF EXISTS task_delegations CASCADE;
DROP TABLE IF EXISTS tasks CASCADE;

-- Drop embedding system tables
DROP TABLE IF EXISTS embedding_metrics CASCADE;
DROP TABLE IF EXISTS agent_configs CASCADE;
DROP TABLE IF EXISTS projection_matrices CASCADE;
DROP TABLE IF EXISTS embedding_statistics CASCADE;
DROP TABLE IF EXISTS embedding_cache CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS embedding_models CASCADE;

-- Drop auth tables
DROP TABLE IF EXISTS tenant_config CASCADE;
DROP TABLE IF EXISTS api_key_usage CASCADE;
DROP TABLE IF EXISTS api_keys CASCADE;
DROP TABLE IF EXISTS users CASCADE;

-- Drop foundation tables
DROP TABLE IF EXISTS context_items CASCADE;
DROP TABLE IF EXISTS contexts CASCADE;
DROP TABLE IF EXISTS agents CASCADE;
DROP TABLE IF EXISTS models CASCADE;

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

DROP TYPE IF EXISTS member_role CASCADE;
DROP TYPE IF EXISTS workspace_visibility CASCADE;
DROP TYPE IF EXISTS delegation_type CASCADE;
DROP TYPE IF EXISTS workflow_status CASCADE;
DROP TYPE IF EXISTS workflow_type CASCADE;
DROP TYPE IF EXISTS task_priority CASCADE;
DROP TYPE IF EXISTS task_status CASCADE;

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