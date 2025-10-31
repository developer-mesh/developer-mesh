-- Rollback User Registration and Authentication System
BEGIN;

-- Drop tables
DROP TABLE IF EXISTS mcp.auth_audit_log;
DROP TABLE IF EXISTS mcp.user_sessions;
DROP TABLE IF EXISTS mcp.organization_registrations;
DROP TABLE IF EXISTS mcp.email_verification_tokens;
DROP TABLE IF EXISTS mcp.password_reset_tokens;
DROP TABLE IF EXISTS mcp.user_invitations;

-- Remove columns from organizations
ALTER TABLE mcp.organizations DROP COLUMN IF EXISTS owner_user_id;
ALTER TABLE mcp.organizations DROP COLUMN IF EXISTS subscription_tier;
ALTER TABLE mcp.organizations DROP COLUMN IF EXISTS max_users;
ALTER TABLE mcp.organizations DROP COLUMN IF EXISTS max_agents;
ALTER TABLE mcp.organizations DROP COLUMN IF EXISTS billing_email;

-- Remove columns from users
ALTER TABLE mcp.users DROP COLUMN IF EXISTS password_hash;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS organization_id;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS role;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS last_login_at;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS password_changed_at;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS failed_login_attempts;
ALTER TABLE mcp.users DROP COLUMN IF EXISTS locked_until;

COMMIT;