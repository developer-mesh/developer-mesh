-- Rollback Edge MCP Session Management System
BEGIN;

-- Drop views and functions first
DROP VIEW IF EXISTS mcp.session_metrics;
DROP FUNCTION IF EXISTS mcp.cleanup_expired_sessions();

-- Drop tables (cascade will handle foreign keys)
DROP TABLE IF EXISTS mcp.session_tool_executions CASCADE;
DROP TABLE IF EXISTS mcp.edge_mcp_sessions CASCADE;

COMMIT;