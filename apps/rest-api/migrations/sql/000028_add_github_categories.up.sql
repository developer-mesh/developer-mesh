-- ==============================================================================
-- Migration: 000032_add_github_categories
-- Description: Add GitHub-specific categories to support toolset organization
-- Author: DevMesh Team
-- Date: 2025-09-29
-- ==============================================================================

BEGIN;

-- Drop the existing constraint
ALTER TABLE mcp.tool_templates
DROP CONSTRAINT IF EXISTS chk_category;

-- Add the updated constraint with GitHub-specific categories
ALTER TABLE mcp.tool_templates
ADD CONSTRAINT chk_category CHECK (category IS NULL OR category IN (
    -- Original categories
    'version_control', 'issue_tracking', 'ci_cd', 'monitoring',
    'collaboration', 'cloud', 'security', 'documentation',
    'artifact_management', 'repository_management', 'projects', 'custom',

    -- GitHub-specific categories (from toolsets)
    'repos', 'issues', 'pull_requests', 'actions', 'context',
    'git', 'organizations', 'graphql', 'discussions',

    -- Additional provider-specific categories that may be needed
    'search', 'webhooks', 'releases', 'deployments'
));

COMMIT;