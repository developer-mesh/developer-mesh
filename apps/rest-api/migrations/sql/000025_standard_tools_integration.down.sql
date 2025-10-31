-- ==============================================================================
-- Migration: 000029_standard_tools_integration (DOWN)
-- Description: Remove standard tool templates and organization-specific instances
-- ==============================================================================

BEGIN;

-- Drop views
DROP VIEW IF EXISTS mcp.all_available_tools;

-- Drop functions
DROP FUNCTION IF EXISTS mcp.track_tool_usage(UUID, VARCHAR, BOOLEAN, INTEGER);
DROP FUNCTION IF EXISTS mcp.create_organization_tool(UUID, UUID, VARCHAR, VARCHAR, JSONB, JSONB);

-- Drop triggers
DROP TRIGGER IF EXISTS update_organization_tools_updated_at ON mcp.organization_tools;
DROP TRIGGER IF EXISTS update_tool_templates_updated_at ON mcp.tool_templates;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS mcp.organization_tool_usage;
DROP TABLE IF EXISTS mcp.tool_template_versions;
DROP TABLE IF EXISTS mcp.organization_tools;
DROP TABLE IF EXISTS mcp.tool_templates;

COMMIT;