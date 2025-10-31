-- Migration: Add package releases tracking schema
-- This migration adds tables for tracking internal package releases and their metadata
-- to enable AI assistants to have comprehensive knowledge of internal packages

-- Package releases table - stores metadata about package versions
CREATE TABLE IF NOT EXISTS mcp.package_releases (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    repository_name TEXT NOT NULL,
    package_name TEXT NOT NULL,
    version TEXT NOT NULL,
    version_major INTEGER,
    version_minor INTEGER,
    version_patch INTEGER,
    prerelease TEXT,
    is_breaking_change BOOLEAN DEFAULT FALSE,
    release_notes TEXT,
    changelog TEXT,
    published_at TIMESTAMP WITH TIME ZONE NOT NULL,
    author_login TEXT,
    github_release_id BIGINT,
    artifactory_path TEXT,
    package_type TEXT, -- npm, maven, python, go, docker, generic
    description TEXT,
    license TEXT,
    homepage TEXT,
    documentation_url TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(tenant_id, repository_name, version)
);

-- Package assets/artifacts - stores information about release assets
CREATE TABLE IF NOT EXISTS mcp.package_assets (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES mcp.package_releases(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    content_type TEXT,
    size_bytes BIGINT,
    download_url TEXT,
    artifactory_url TEXT,
    sha256_checksum TEXT,
    sha1_checksum TEXT,
    md5_checksum TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- API/Interface changes tracking - captures API evolution
CREATE TABLE IF NOT EXISTS mcp.package_api_changes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES mcp.package_releases(id) ON DELETE CASCADE,
    change_type TEXT NOT NULL CHECK (change_type IN ('added', 'modified', 'deprecated', 'removed')),
    api_signature TEXT NOT NULL,
    description TEXT,
    breaking BOOLEAN DEFAULT FALSE,
    migration_guide TEXT,
    file_path TEXT,
    line_number INTEGER,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Package dependencies - tracks dependency information
CREATE TABLE IF NOT EXISTS mcp.package_dependencies (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    release_id UUID NOT NULL REFERENCES mcp.package_releases(id) ON DELETE CASCADE,
    dependency_name TEXT NOT NULL,
    version_constraint TEXT,
    dependency_type TEXT CHECK (dependency_type IN ('runtime', 'dev', 'peer', 'optional', 'build')),
    repository_url TEXT,
    resolved_version TEXT,
    metadata JSONB DEFAULT '{}'::jsonb,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX IF NOT EXISTS idx_package_releases_tenant ON mcp.package_releases(tenant_id);
CREATE INDEX IF NOT EXISTS idx_package_releases_name_version ON mcp.package_releases(package_name, version);
CREATE INDEX IF NOT EXISTS idx_package_releases_published_at ON mcp.package_releases(published_at DESC);
CREATE INDEX IF NOT EXISTS idx_package_releases_repo ON mcp.package_releases(repository_name);
CREATE INDEX IF NOT EXISTS idx_package_releases_type ON mcp.package_releases(package_type);
CREATE INDEX IF NOT EXISTS idx_package_releases_breaking ON mcp.package_releases(is_breaking_change) WHERE is_breaking_change = TRUE;

CREATE INDEX IF NOT EXISTS idx_package_assets_release ON mcp.package_assets(release_id);
CREATE INDEX IF NOT EXISTS idx_package_api_changes_release ON mcp.package_api_changes(release_id);
CREATE INDEX IF NOT EXISTS idx_package_api_changes_type ON mcp.package_api_changes(change_type);
CREATE INDEX IF NOT EXISTS idx_package_api_changes_breaking ON mcp.package_api_changes(breaking) WHERE breaking = TRUE;
CREATE INDEX IF NOT EXISTS idx_package_dependencies_release ON mcp.package_dependencies(release_id);
CREATE INDEX IF NOT EXISTS idx_package_dependencies_name ON mcp.package_dependencies(dependency_name);

-- Metadata indexes for JSONB queries
CREATE INDEX IF NOT EXISTS idx_package_releases_metadata ON mcp.package_releases USING gin(metadata);
CREATE INDEX IF NOT EXISTS idx_package_assets_metadata ON mcp.package_assets USING gin(metadata);
CREATE INDEX IF NOT EXISTS idx_package_api_changes_metadata ON mcp.package_api_changes USING gin(metadata);
CREATE INDEX IF NOT EXISTS idx_package_dependencies_metadata ON mcp.package_dependencies USING gin(metadata);

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION update_package_releases_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER package_releases_updated_at
    BEFORE UPDATE ON mcp.package_releases
    FOR EACH ROW
    EXECUTE FUNCTION update_package_releases_updated_at();

-- Add comments for documentation
COMMENT ON TABLE mcp.package_releases IS 'Tracks internal package releases and their metadata for AI assistant context';
COMMENT ON TABLE mcp.package_assets IS 'Stores release artifacts and their download information';
COMMENT ON TABLE mcp.package_api_changes IS 'Captures API/interface changes between package versions';
COMMENT ON TABLE mcp.package_dependencies IS 'Tracks package dependencies and their versions';

COMMENT ON COLUMN mcp.package_releases.version_major IS 'Semantic version major component';
COMMENT ON COLUMN mcp.package_releases.version_minor IS 'Semantic version minor component';
COMMENT ON COLUMN mcp.package_releases.version_patch IS 'Semantic version patch component';
COMMENT ON COLUMN mcp.package_releases.prerelease IS 'Prerelease identifier (alpha, beta, rc, etc.)';
COMMENT ON COLUMN mcp.package_releases.is_breaking_change IS 'Whether this release contains breaking changes';
COMMENT ON COLUMN mcp.package_releases.github_release_id IS 'GitHub release ID for cross-reference';
COMMENT ON COLUMN mcp.package_releases.artifactory_path IS 'Path to package in Artifactory';
