-- =====================================================================
-- Agent Lifecycle Management System - Rollback Migration
-- Version: 1.0.0
-- Description: Safely rollback agent lifecycle changes
-- =====================================================================

-- Drop views
DROP VIEW IF EXISTS mcp.v_agent_status;

-- Drop policies
DROP POLICY IF EXISTS tenant_isolation_agent_events ON mcp.agent_events;
DROP POLICY IF EXISTS tenant_isolation_agent_sessions ON mcp.agent_sessions;
DROP POLICY IF EXISTS tenant_isolation_agent_registry ON mcp.agent_registry;
DROP POLICY IF EXISTS tenant_isolation_agent_health ON mcp.agent_health_metrics;

-- Drop triggers
DROP TRIGGER IF EXISTS validate_agent_state_change ON mcp.agents;
DROP TRIGGER IF EXISTS create_agent_config_on_insert ON mcp.agents;

-- Drop functions
DROP FUNCTION IF EXISTS mcp.validate_agent_state_transition();
DROP FUNCTION IF EXISTS mcp.create_default_agent_config();
DROP FUNCTION IF EXISTS mcp.update_agent_health(UUID, VARCHAR, VARCHAR, NUMERIC, VARCHAR, JSONB);

-- Drop tables (in reverse order of dependencies)
DROP TABLE IF EXISTS mcp.agent_config_templates;
DROP TABLE IF EXISTS mcp.agent_registry;
DROP TABLE IF EXISTS mcp.agent_state_transitions;
DROP TABLE IF EXISTS mcp.agent_sessions;
DROP TABLE IF EXISTS mcp.agent_health_metrics;
DROP TABLE IF EXISTS mcp.agent_capability_templates;
DROP TABLE IF EXISTS mcp.agent_events;

-- Remove columns from agents table
ALTER TABLE mcp.agents 
DROP COLUMN IF EXISTS state,
DROP COLUMN IF EXISTS state_reason,
DROP COLUMN IF EXISTS state_changed_at,
DROP COLUMN IF EXISTS state_changed_by,
DROP COLUMN IF EXISTS health_status,
DROP COLUMN IF EXISTS health_checked_at,
DROP COLUMN IF EXISTS metadata,
DROP COLUMN IF EXISTS version,
DROP COLUMN IF EXISTS activation_count,
DROP COLUMN IF EXISTS last_error,
DROP COLUMN IF EXISTS last_error_at,
DROP COLUMN IF EXISTS retry_count;

-- Remove columns from agent_configs
ALTER TABLE mcp.agent_configs
DROP COLUMN IF EXISTS agent_type,
DROP COLUMN IF EXISTS priority,
DROP COLUMN IF EXISTS retry_policy,
DROP COLUMN IF EXISTS timeout_ms,
DROP COLUMN IF EXISTS circuit_breaker,
DROP COLUMN IF EXISTS capabilities_config,
DROP COLUMN IF EXISTS scheduling_preferences;

-- Remove columns from agent_capabilities
ALTER TABLE mcp.agent_capabilities
DROP COLUMN IF EXISTS capability_type,
DROP COLUMN IF EXISTS priority,
DROP COLUMN IF EXISTS dependencies,
DROP COLUMN IF EXISTS health_endpoint,
DROP COLUMN IF EXISTS last_health_check,
DROP COLUMN IF EXISTS health_status,
DROP COLUMN IF EXISTS performance_metrics,
DROP COLUMN IF EXISTS validation_errors;

-- Drop type
DROP TYPE IF EXISTS mcp.agent_state CASCADE;

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_agents_state;
DROP INDEX IF EXISTS mcp.idx_agents_state_tenant;
DROP INDEX IF EXISTS mcp.idx_agents_health_checked;
DROP INDEX IF EXISTS mcp.idx_agent_configs_agent_active;
DROP INDEX IF EXISTS mcp.idx_agents_tenant_type_state;
DROP INDEX IF EXISTS mcp.idx_agent_events_agent_type_time;
DROP INDEX IF EXISTS mcp.idx_agent_configs_agent_version;