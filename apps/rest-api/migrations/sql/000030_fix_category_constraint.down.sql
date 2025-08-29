-- ==============================================================================
-- Migration: 000030_fix_category_constraint (DOWN)
-- Description: Revert category constraint to original state
-- Author: DevMesh Team
-- Date: 2025-08-29
-- ==============================================================================

BEGIN;

-- Drop the updated constraint
ALTER TABLE mcp.tool_templates 
DROP CONSTRAINT IF EXISTS chk_category;

-- Restore the original constraint
ALTER TABLE mcp.tool_templates 
ADD CONSTRAINT chk_category CHECK (category IS NULL OR category IN (
    'version_control', 'issue_tracking', 'ci_cd', 'monitoring',
    'collaboration', 'cloud', 'security', 'documentation', 'custom'
));

COMMIT;