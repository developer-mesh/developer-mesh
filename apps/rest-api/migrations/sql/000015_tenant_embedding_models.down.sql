-- =====================================================================
-- Rollback Multi-Tenant Embedding Model Management System
-- =====================================================================
-- This migration safely removes all multi-tenant embedding model 
-- management structures in reverse order of dependencies

-- Drop views first
DROP VIEW IF EXISTS mcp.v_tenant_available_models CASCADE;

-- Drop policies
DROP POLICY IF EXISTS tenant_models_isolation ON mcp.tenant_embedding_models;
DROP POLICY IF EXISTS agent_preferences_isolation ON mcp.agent_embedding_preferences;
DROP POLICY IF EXISTS usage_tracking_isolation ON mcp.embedding_usage_tracking;

-- Disable RLS
ALTER TABLE IF EXISTS mcp.tenant_embedding_models DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.agent_embedding_preferences DISABLE ROW LEVEL SECURITY;
ALTER TABLE IF EXISTS mcp.embedding_usage_tracking DISABLE ROW LEVEL SECURITY;

-- Drop triggers
DROP TRIGGER IF EXISTS update_catalog_timestamp ON mcp.embedding_model_catalog;
DROP TRIGGER IF EXISTS update_tenant_models_timestamp ON mcp.tenant_embedding_models;

-- Drop functions
DROP FUNCTION IF EXISTS mcp.update_embedding_model_timestamp() CASCADE;
DROP FUNCTION IF EXISTS mcp.track_embedding_usage(UUID, UUID, UUID, INTEGER, INTEGER, INTEGER, VARCHAR) CASCADE;
DROP FUNCTION IF EXISTS mcp.get_embedding_model_for_request(UUID, UUID, VARCHAR, VARCHAR) CASCADE;

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_tenant_models_tenant;
DROP INDEX IF EXISTS mcp.idx_tenant_models_enabled;
DROP INDEX IF EXISTS mcp.idx_agent_preferences_agent;
DROP INDEX IF EXISTS mcp.idx_usage_tracking_tenant_date;
DROP INDEX IF EXISTS mcp.idx_usage_tracking_agent;
DROP INDEX IF EXISTS mcp.idx_catalog_available;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS mcp.model_discovery_registry CASCADE;
DROP TABLE IF EXISTS mcp.embedding_usage_tracking CASCADE;
DROP TABLE IF EXISTS mcp.agent_embedding_preferences CASCADE;
DROP TABLE IF EXISTS mcp.tenant_embedding_models CASCADE;
DROP TABLE IF EXISTS mcp.embedding_model_catalog CASCADE;

-- Note: This migration preserves existing embeddings data in mcp.embeddings table
-- Only the multi-tenant management layer is removed