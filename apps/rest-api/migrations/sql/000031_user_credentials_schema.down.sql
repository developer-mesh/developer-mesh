-- Rollback: User Credentials Storage
-- Description: Drop user credentials tables and related objects

-- Drop RLS policies
DROP POLICY IF EXISTS user_credentials_tenant_isolation ON mcp.user_credentials;

-- Drop trigger and function
DROP TRIGGER IF EXISTS user_credentials_updated_at ON mcp.user_credentials;
DROP FUNCTION IF EXISTS mcp.update_user_credentials_updated_at();

-- Drop audit table first (has FK to user_credentials)
DROP TABLE IF EXISTS mcp.user_credentials_audit;

-- Drop main table
DROP TABLE IF EXISTS mcp.user_credentials;
