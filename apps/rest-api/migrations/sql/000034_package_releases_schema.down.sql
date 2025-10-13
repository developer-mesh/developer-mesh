-- Rollback migration: Remove package releases tracking schema

-- Drop trigger and function
DROP TRIGGER IF EXISTS package_releases_updated_at ON mcp.package_releases;
DROP FUNCTION IF EXISTS update_package_releases_updated_at();

-- Drop indexes
DROP INDEX IF EXISTS idx_package_dependencies_metadata;
DROP INDEX IF EXISTS idx_package_api_changes_metadata;
DROP INDEX IF EXISTS idx_package_assets_metadata;
DROP INDEX IF EXISTS idx_package_releases_metadata;

DROP INDEX IF EXISTS idx_package_dependencies_name;
DROP INDEX IF EXISTS idx_package_dependencies_release;
DROP INDEX IF EXISTS idx_package_api_changes_breaking;
DROP INDEX IF EXISTS idx_package_api_changes_type;
DROP INDEX IF EXISTS idx_package_api_changes_release;
DROP INDEX IF EXISTS idx_package_assets_release;

DROP INDEX IF EXISTS idx_package_releases_breaking;
DROP INDEX IF EXISTS idx_package_releases_type;
DROP INDEX IF EXISTS idx_package_releases_repo;
DROP INDEX IF EXISTS idx_package_releases_published_at;
DROP INDEX IF EXISTS idx_package_releases_name_version;
DROP INDEX IF EXISTS idx_package_releases_tenant;

-- Drop tables (in reverse dependency order)
DROP TABLE IF EXISTS mcp.package_dependencies;
DROP TABLE IF EXISTS mcp.package_api_changes;
DROP TABLE IF EXISTS mcp.package_assets;
DROP TABLE IF EXISTS mcp.package_releases;
