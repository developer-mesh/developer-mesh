-- Migration rollback: Extend Service Types
-- Description: Revert to original service types
-- Author: DevMesh Team
-- Date: 2025-10-21

BEGIN;

-- Drop extended constraint
ALTER TABLE mcp.user_credentials
DROP CONSTRAINT IF EXISTS user_credentials_service_type_check;

-- Restore original constraint
ALTER TABLE mcp.user_credentials
ADD CONSTRAINT user_credentials_service_type_check
CHECK (service_type IN (
    'github',
    'harness',
    'aws',
    'azure',
    'gcp',
    'snyk',
    'jira',
    'slack',
    'generic'
));

COMMIT;
