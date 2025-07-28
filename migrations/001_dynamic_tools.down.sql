-- Rollback Migration: Dynamic Tools Feature
-- Version: 001

-- Drop triggers first
DROP TRIGGER IF EXISTS update_tool_configurations_updated_at ON tool_configurations;

-- Drop function
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS tool_execution_retries;
DROP TABLE IF EXISTS tool_executions;
DROP TABLE IF EXISTS tool_discovery_sessions;
DROP TABLE IF EXISTS tool_configurations;