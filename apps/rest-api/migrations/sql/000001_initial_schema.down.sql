-- Rollback initial schema
-- This will remove all tables and schema created in the up migration

-- Disable Row Level Security
ALTER TABLE IF EXISTS mcp.models DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.agents DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.contexts DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.users DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.api_keys DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.embeddings DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.tasks DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.workflows DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.workspaces DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.integrations DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.webhook_configs DISABLE ROW LEVEL SECURITY;

-- Drop all policies
DROP POLICY IF EXISTS tenant_isolation_models ON mcp.models;
DROP POLICY IF EXISTS tenant_isolation_agents ON mcp.agents;
DROP POLICY IF EXISTS tenant_isolation_contexts ON mcp.contexts;
DROP POLICY IF EXISTS tenant_isolation_users ON mcp.users;
DROP POLICY IF EXISTS tenant_isolation_api_keys ON mcp.api_keys;
DROP POLICY IF EXISTS tenant_isolation_embeddings ON mcp.embeddings;
DROP POLICY IF EXISTS tenant_isolation_tasks ON mcp.tasks;
DROP POLICY IF EXISTS tenant_isolation_workflows ON mcp.workflows;
DROP POLICY IF EXISTS tenant_isolation_workspaces ON mcp.workspaces;
DROP POLICY IF EXISTS tenant_isolation_integrations ON mcp.integrations;
DROP POLICY IF EXISTS tenant_isolation_webhook_configs ON mcp.webhook_configs;

-- Drop functions
DROP FUNCTION IF EXISTS mcp.update_updated_at_column() CASCADE;
DROP FUNCTION IF EXISTS mcp.current_tenant_id() CASCADE;

-- Drop all tables in reverse dependency order
DROP TABLE IF EXISTS mcp.document_operations CASCADE;
DROP TABLE IF EXISTS mcp.shared_documents CASCADE;
DROP TABLE IF EXISTS mcp.workspace_members CASCADE;
DROP TABLE IF EXISTS mcp.workspaces CASCADE;
DROP TABLE IF EXISTS mcp.workflow_executions CASCADE;
DROP TABLE IF EXISTS mcp.workflows CASCADE;
DROP TABLE IF EXISTS mcp.task_delegations CASCADE;
DROP TABLE IF EXISTS mcp.tasks CASCADE;
DROP TABLE IF EXISTS mcp.embedding_metrics CASCADE;
DROP TABLE IF EXISTS mcp.agent_configs CASCADE;
DROP TABLE IF EXISTS mcp.embedding_cache CASCADE;
DROP TABLE IF EXISTS mcp.embeddings CASCADE;
DROP TABLE IF EXISTS mcp.embedding_models CASCADE;
DROP TABLE IF EXISTS mcp.webhook_dlq CASCADE;
DROP TABLE IF EXISTS mcp.webhook_event_logs CASCADE;
DROP TABLE IF EXISTS mcp.webhook_events CASCADE;
DROP TABLE IF EXISTS mcp.tool_discovery_patterns CASCADE;
DROP TABLE IF EXISTS mcp.openapi_cache CASCADE;
DROP TABLE IF EXISTS mcp.tool_credentials CASCADE;
DROP TABLE IF EXISTS mcp.tool_health_checks CASCADE;
DROP TABLE IF EXISTS mcp.tool_execution_retries CASCADE;
DROP TABLE IF EXISTS mcp.tool_executions CASCADE;
DROP TABLE IF EXISTS mcp.tool_discovery_sessions CASCADE;
DROP TABLE IF EXISTS mcp.tool_configurations CASCADE;
DROP TABLE IF EXISTS mcp.webhook_configs CASCADE;
DROP TABLE IF EXISTS mcp.integrations CASCADE;
DROP TABLE IF EXISTS mcp.events CASCADE;
DROP TABLE IF EXISTS mcp.audit_log CASCADE;
DROP TABLE IF EXISTS mcp.context_items CASCADE;
DROP TABLE IF EXISTS mcp.contexts CASCADE;
DROP TABLE IF EXISTS mcp.api_key_usage CASCADE;
DROP TABLE IF EXISTS mcp.api_keys CASCADE;
DROP TABLE IF EXISTS mcp.users CASCADE;
DROP TABLE IF EXISTS mcp.tenant_config CASCADE;
DROP TABLE IF EXISTS mcp.agents CASCADE;
DROP TABLE IF EXISTS mcp.models CASCADE;

-- Drop the schema
DROP SCHEMA IF EXISTS mcp CASCADE;