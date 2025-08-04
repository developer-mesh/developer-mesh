-- ==============================================================================
-- Rollback Migration: 000002_dynamic_tools
-- Description: Remove dynamic tools discovery and execution system
-- Author: DBA Team
-- Date: 2025-08-04
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- DROP FUNCTIONS (must be done before dropping tables they reference)
-- ==============================================================================

-- Drop functions with CASCADE to handle dependent triggers
DROP FUNCTION IF EXISTS mcp.execute_tool_action(UUID, VARCHAR(255), JSONB, UUID) CASCADE;
DROP FUNCTION IF EXISTS mcp.get_tool_by_name(UUID, VARCHAR(255)) CASCADE;
DROP FUNCTION IF EXISTS mcp.update_tool_health_status() CASCADE;
DROP FUNCTION IF EXISTS mcp.update_discovery_pattern_stats() CASCADE;

-- ==============================================================================
-- DROP POLICIES (must be done before dropping tables)
-- ==============================================================================

-- Drop RLS policies
DROP POLICY IF EXISTS tenant_isolation_tool_configurations ON mcp.tool_configurations;
DROP POLICY IF EXISTS tenant_isolation_tool_executions ON mcp.tool_executions;
DROP POLICY IF EXISTS tenant_isolation_tool_discovery_sessions ON mcp.tool_discovery_sessions;
DROP POLICY IF EXISTS tenant_isolation_tool_auth_configs ON mcp.tool_auth_configs;
DROP POLICY IF EXISTS tenant_isolation_tool_health_checks ON mcp.tool_health_checks;
DROP POLICY IF EXISTS public_read_discovery_patterns ON mcp.tool_discovery_patterns;
DROP POLICY IF EXISTS tenant_write_discovery_patterns ON mcp.tool_discovery_patterns;

-- ==============================================================================
-- DROP TRIGGERS
-- ==============================================================================

DROP TRIGGER IF EXISTS update_tool_configurations_updated_at ON mcp.tool_configurations;
DROP TRIGGER IF EXISTS update_tool_auth_configs_updated_at ON mcp.tool_auth_configs;
DROP TRIGGER IF EXISTS update_tool_health_after_check ON mcp.tool_health_checks;
DROP TRIGGER IF EXISTS update_pattern_stats_after_discovery ON mcp.tool_discovery_sessions;

-- ==============================================================================
-- DROP INDEXES
-- ==============================================================================

-- Tool configuration indexes
DROP INDEX IF EXISTS mcp.idx_tool_configurations_tenant_active;
DROP INDEX IF EXISTS mcp.idx_tool_configurations_type;
DROP INDEX IF EXISTS mcp.idx_tool_configurations_health;
DROP INDEX IF EXISTS mcp.idx_tool_configurations_endpoints;
DROP INDEX IF EXISTS mcp.idx_tool_configurations_tags;

-- Discovery indexes
DROP INDEX IF EXISTS mcp.idx_tool_discovery_sessions_tool;
DROP INDEX IF EXISTS mcp.idx_tool_discovery_sessions_status;
DROP INDEX IF EXISTS mcp.idx_tool_discovery_patterns_domain;
DROP INDEX IF EXISTS mcp.idx_tool_discovery_patterns_confidence;

-- Execution indexes
DROP INDEX IF EXISTS mcp.idx_tool_executions_tool;
DROP INDEX IF EXISTS mcp.idx_tool_executions_tenant_time;
DROP INDEX IF EXISTS mcp.idx_tool_executions_status;
DROP INDEX IF EXISTS mcp.idx_tool_executions_correlation;

-- Auth and health indexes
DROP INDEX IF EXISTS mcp.idx_tool_auth_configs_tool;
DROP INDEX IF EXISTS mcp.idx_tool_health_checks_tool_time;

-- ==============================================================================
-- DROP TABLES (in reverse dependency order)
-- ==============================================================================

-- Drop dependent tables first
DROP TABLE IF EXISTS mcp.tool_health_checks;
DROP TABLE IF EXISTS mcp.tool_auth_configs;
DROP TABLE IF EXISTS mcp.tool_executions;
DROP TABLE IF EXISTS mcp.tool_discovery_patterns;
DROP TABLE IF EXISTS mcp.tool_discovery_sessions;

-- Drop main table last
DROP TABLE IF EXISTS mcp.tool_configurations;

-- ==============================================================================
-- VERIFY CLEANUP
-- ==============================================================================

-- Ensure no orphaned objects remain
DO $$
DECLARE
    v_count INTEGER;
BEGIN
    -- Check for any remaining tool-related tables
    SELECT COUNT(*) INTO v_count
    FROM information_schema.tables
    WHERE table_schema = 'mcp'
    AND table_name LIKE 'tool_%';
    
    IF v_count > 0 THEN
        RAISE WARNING 'Found % tool-related tables still present after rollback', v_count;
    END IF;
    
    -- Check for any remaining tool-related indexes
    SELECT COUNT(*) INTO v_count
    FROM pg_indexes
    WHERE schemaname = 'mcp'
    AND indexname LIKE '%tool_%';
    
    IF v_count > 0 THEN
        RAISE WARNING 'Found % tool-related indexes still present after rollback', v_count;
    END IF;
END;
$$;

COMMIT;