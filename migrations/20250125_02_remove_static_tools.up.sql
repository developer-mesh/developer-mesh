-- Migration: Remove Static Tool Implementations
-- Version: 20250125_02
-- Description: Removes any static tool implementations to prepare for dynamic tools

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250125_02', 'Remove static tool implementations');

-- Drop any static tool-specific tables or views
-- Note: This is a placeholder - adjust based on actual static implementation

-- Example: Drop static GitHub tables if they exist
DROP TABLE IF EXISTS github_configurations CASCADE;
DROP TABLE IF EXISTS github_repositories CASCADE;
DROP TABLE IF EXISTS github_webhooks CASCADE;

-- Example: Drop static GitLab tables if they exist
DROP TABLE IF EXISTS gitlab_configurations CASCADE;
DROP TABLE IF EXISTS gitlab_projects CASCADE;

-- Example: Drop static SonarQube tables if they exist
DROP TABLE IF EXISTS sonarqube_configurations CASCADE;
DROP TABLE IF EXISTS sonarqube_projects CASCADE;

-- Example: Drop static JFrog tables if they exist
DROP TABLE IF EXISTS jfrog_configurations CASCADE;
DROP TABLE IF EXISTS jfrog_repositories CASCADE;

-- Drop any tool-specific views
DROP VIEW IF EXISTS github_tools CASCADE;
DROP VIEW IF EXISTS gitlab_tools CASCADE;
DROP VIEW IF EXISTS sonarqube_tools CASCADE;
DROP VIEW IF EXISTS jfrog_tools CASCADE;

-- Drop any tool-specific functions
DROP FUNCTION IF EXISTS get_github_config(UUID) CASCADE;
DROP FUNCTION IF EXISTS get_gitlab_config(UUID) CASCADE;
DROP FUNCTION IF EXISTS get_sonarqube_config(UUID) CASCADE;
DROP FUNCTION IF EXISTS get_jfrog_config(UUID) CASCADE;

-- Log cleanup actions
DO $$
BEGIN
    RAISE NOTICE 'Removed static tool implementations to prepare for dynamic tools';
END $$;

-- Create generic tool view for all dynamic tools
CREATE OR REPLACE VIEW dynamic_tools AS
SELECT 
    id,
    tenant_id,
    tool_name,
    display_name,
    config,
    status,
    health_status,
    last_health_check,
    created_at,
    updated_at
FROM tool_configurations;

-- Add comment for documentation
COMMENT ON VIEW dynamic_tools IS 'View of all dynamically configured tools';