-- Drop resource-related tables and functions
DROP TRIGGER IF EXISTS trigger_update_resource_updated_at ON mcp.resources;
DROP FUNCTION IF EXISTS mcp.update_resource_updated_at();
DROP TABLE IF EXISTS mcp.resource_subscriptions;
DROP TABLE IF EXISTS mcp.resources;