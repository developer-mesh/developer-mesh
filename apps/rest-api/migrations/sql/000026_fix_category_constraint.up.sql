-- ==============================================================================
-- Migration: 000030_fix_category_constraint
-- Description: Fix category constraint to include more categories and normalize case
-- Author: DevMesh Team
-- Date: 2025-08-29
-- ==============================================================================

BEGIN;

-- Drop the existing constraint
ALTER TABLE mcp.tool_templates 
DROP CONSTRAINT IF EXISTS chk_category;

-- Add the updated constraint with more categories
ALTER TABLE mcp.tool_templates 
ADD CONSTRAINT chk_category CHECK (category IS NULL OR category IN (
    'version_control', 'issue_tracking', 'ci_cd', 'monitoring',
    'collaboration', 'cloud', 'security', 'documentation', 
    'artifact_management', 'repository_management', 'projects', 'custom'
));

COMMIT;