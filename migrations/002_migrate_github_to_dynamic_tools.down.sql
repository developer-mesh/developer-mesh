-- Rollback Migration: Migrate GitHub Configurations to Dynamic Tools
-- Version: 002

BEGIN;

-- Remove migrated GitHub tools
DELETE FROM tool_configurations
WHERE created_by = 'migration-002';

-- Drop the GitHub tools view
DROP VIEW IF EXISTS github_tools;

-- Drop indexes
DROP INDEX IF EXISTS idx_tool_config_type;
DROP INDEX IF EXISTS idx_tool_config_base_url;

-- Remove migration audit entries
DELETE FROM migration_audit
WHERE migration_version = '002_migrate_github';

-- Drop the encryption function
DROP FUNCTION IF EXISTS encrypt_credential(UUID, TEXT);

-- Log rollback
DO $$
BEGIN
    RAISE NOTICE 'Rollback completed: Removed migrated GitHub tools';
END $$;

COMMIT;