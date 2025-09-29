-- ==============================================================================
-- Migration: 000032_add_github_categories (rollback)
-- Description: Revert to original category constraint
-- Author: DevMesh Team
-- Date: 2025-09-29
-- ==============================================================================

BEGIN;

-- Drop the updated constraint
ALTER TABLE mcp.tool_templates
DROP CONSTRAINT IF EXISTS chk_category;

-- Restore the original constraint
ALTER TABLE mcp.tool_templates
ADD CONSTRAINT chk_category CHECK (category IS NULL OR category IN (
    'version_control', 'issue_tracking', 'ci_cd', 'monitoring',
    'collaboration', 'cloud', 'security', 'documentation',
    'artifact_management', 'repository_management', 'projects', 'custom'
));

COMMIT;