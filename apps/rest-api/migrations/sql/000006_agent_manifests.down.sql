-- Drop triggers
DROP TRIGGER IF EXISTS update_agent_manifests_timestamp ON mcp.agent_manifests;
DROP TRIGGER IF EXISTS update_agent_registrations_timestamp ON mcp.agent_registrations;

-- Drop function
DROP FUNCTION IF EXISTS mcp.update_agent_manifest_timestamp();

-- Drop indexes
DROP INDEX IF EXISTS mcp.idx_agent_channels_active;
DROP INDEX IF EXISTS mcp.idx_agent_channels_type;
DROP INDEX IF EXISTS mcp.idx_agent_channels_registration_id;

DROP INDEX IF EXISTS mcp.idx_agent_capabilities_type;
DROP INDEX IF EXISTS mcp.idx_agent_capabilities_manifest_id;

DROP INDEX IF EXISTS mcp.idx_agent_registrations_health;
DROP INDEX IF EXISTS mcp.idx_agent_registrations_status;
DROP INDEX IF EXISTS mcp.idx_agent_registrations_tenant_id;
DROP INDEX IF EXISTS mcp.idx_agent_registrations_manifest_id;

DROP INDEX IF EXISTS mcp.idx_agent_manifests_capabilities;
DROP INDEX IF EXISTS mcp.idx_agent_manifests_status;
DROP INDEX IF EXISTS mcp.idx_agent_manifests_agent_type;
DROP INDEX IF EXISTS mcp.idx_agent_manifests_org_id;

-- Drop tables in reverse order of dependencies
DROP TABLE IF EXISTS mcp.agent_channels;
DROP TABLE IF EXISTS mcp.agent_capabilities;
DROP TABLE IF EXISTS mcp.agent_registrations;
DROP TABLE IF EXISTS mcp.agent_manifests;