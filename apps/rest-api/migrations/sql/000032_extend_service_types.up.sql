-- Migration: Extend Service Types
-- Description: Add support for SonarQube, Artifactory, Jenkins, GitLab, Bitbucket, and Confluence
-- Author: DevMesh Team
-- Date: 2025-10-21

BEGIN;

-- Drop existing constraint
ALTER TABLE mcp.user_credentials
DROP CONSTRAINT IF EXISTS user_credentials_service_type_check;

-- Add new constraint with extended service types
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
    'sonarqube',
    'artifactory',
    'jenkins',
    'gitlab',
    'bitbucket',
    'confluence',
    'generic'
));

COMMIT;
