-- Migration: Remove hardcoded tool configurations
-- Purpose: Clean up legacy tool-specific tables and configurations

-- Remove any hardcoded tool references from existing tables
-- Note: We're not dropping the tool_configurations table as it's used for dynamic tools

-- Remove any tool-specific columns if they exist
-- (These may not exist, but we check just in case)
DO $$ 
BEGIN
    -- Check if any columns reference specific tools
    IF EXISTS (
        SELECT 1 FROM information_schema.columns 
        WHERE table_name = 'tool_configurations' 
        AND column_name = 'tool_type'
    ) THEN
        -- First, update any existing data to remove tool_type dependencies
        UPDATE tool_configurations 
        SET config = jsonb_set(
            COALESCE(config, '{}'::jsonb),
            '{legacy_tool_type}',
            to_jsonb(tool_type)
        )
        WHERE tool_type IS NOT NULL;
        
        -- Then drop the column
        ALTER TABLE tool_configurations DROP COLUMN tool_type;
    END IF;
END $$;

-- Remove any legacy tool-specific tables if they exist
DROP TABLE IF EXISTS github_configurations CASCADE;
DROP TABLE IF EXISTS harness_configurations CASCADE;
DROP TABLE IF EXISTS sonarqube_configurations CASCADE;
DROP TABLE IF EXISTS artifactory_configurations CASCADE;
DROP TABLE IF EXISTS xray_configurations CASCADE;
DROP TABLE IF EXISTS jira_configurations CASCADE;
DROP TABLE IF EXISTS jenkins_configurations CASCADE;

-- Remove any legacy views that might reference hardcoded tools
DROP VIEW IF EXISTS v_github_tools CASCADE;
DROP VIEW IF EXISTS v_harness_tools CASCADE;
DROP VIEW IF EXISTS v_sonarqube_tools CASCADE;

-- Remove any legacy functions that might be tool-specific
DROP FUNCTION IF EXISTS get_github_token(UUID) CASCADE;
DROP FUNCTION IF EXISTS get_harness_config(UUID) CASCADE;
DROP FUNCTION IF EXISTS validate_tool_type() CASCADE;

-- Remove any legacy indexes that might be tool-specific
DROP INDEX IF EXISTS idx_tool_configurations_tool_type;

-- Add migration record
INSERT INTO schema_migrations (version, description, applied_at)
VALUES ('20250126_01', 'Remove hardcoded tool configurations', NOW())
ON CONFLICT (version) DO NOTHING;