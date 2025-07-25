-- Rollback Migration: Create Dynamic Tools Schema
-- Version: 20250125_01

-- Drop triggers
DROP TRIGGER IF EXISTS update_tool_configurations_updated_at ON tool_configurations;
DROP TRIGGER IF EXISTS update_tool_credentials_updated_at ON tool_credentials;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP FUNCTION IF EXISTS cleanup_expired_discovery_sessions();
DROP FUNCTION IF EXISTS cleanup_old_executions(INT);

-- Drop tables in reverse order of creation (due to foreign key constraints)
DROP TABLE IF EXISTS openapi_cache CASCADE;
DROP TABLE IF EXISTS tool_credentials CASCADE;
DROP TABLE IF EXISTS tool_health_checks CASCADE;
DROP TABLE IF EXISTS tool_execution_retries CASCADE;
DROP TABLE IF EXISTS tool_executions CASCADE;
DROP TABLE IF EXISTS tool_discovery_sessions CASCADE;
DROP TABLE IF EXISTS tool_configurations CASCADE;

-- Update migration metadata
UPDATE migration_metadata 
SET status = 'rolled_back', 
    rollback_at = NOW() 
WHERE version = '20250125_01';