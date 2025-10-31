-- User Registration and Authentication System
BEGIN;

-- Add password and registration fields to users table
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS password_hash VARCHAR(255);
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS organization_id UUID REFERENCES mcp.organizations(id) ON DELETE CASCADE;
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS role VARCHAR(50) DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'readonly'));
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS last_login_at TIMESTAMP;
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS password_changed_at TIMESTAMP;
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS failed_login_attempts INTEGER DEFAULT 0;
ALTER TABLE mcp.users ADD COLUMN IF NOT EXISTS locked_until TIMESTAMP;

-- User invitations table
CREATE TABLE IF NOT EXISTS mcp.user_invitations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL REFERENCES mcp.organizations(id) ON DELETE CASCADE,
    email VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'member' CHECK (role IN ('admin', 'member', 'readonly')),
    invitation_token VARCHAR(255) UNIQUE NOT NULL,
    invited_by UUID REFERENCES mcp.users(id),
    expires_at TIMESTAMP NOT NULL,
    accepted_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(organization_id, email)
);

-- Password reset tokens
CREATE TABLE IF NOT EXISTS mcp.password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES mcp.users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    used_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Email verification tokens
CREATE TABLE IF NOT EXISTS mcp.email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES mcp.users(id) ON DELETE CASCADE,
    token_hash VARCHAR(255) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    verified_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Organization registration requests (for approval workflow if needed)
CREATE TABLE IF NOT EXISTS mcp.organization_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_name VARCHAR(255) NOT NULL,
    organization_slug VARCHAR(255) UNIQUE NOT NULL,
    admin_email VARCHAR(255) NOT NULL,
    admin_name VARCHAR(255) NOT NULL,
    company_size VARCHAR(50),
    industry VARCHAR(100),
    use_case TEXT,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected', 'active')),
    approval_token VARCHAR(255) UNIQUE,
    approved_by UUID REFERENCES mcp.users(id),
    approved_at TIMESTAMP,
    rejection_reason TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Session management for JWT refresh tokens
CREATE TABLE IF NOT EXISTS mcp.user_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES mcp.users(id) ON DELETE CASCADE,
    refresh_token_hash VARCHAR(255) UNIQUE NOT NULL,
    device_info JSONB,
    ip_address INET,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_activity TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Audit log for security events
CREATE TABLE IF NOT EXISTS mcp.auth_audit_log (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id UUID REFERENCES mcp.users(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES mcp.organizations(id) ON DELETE SET NULL,
    event_type VARCHAR(50) NOT NULL,
    event_details JSONB,
    ip_address INET,
    user_agent TEXT,
    success BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for performance
CREATE INDEX idx_users_organization ON mcp.users(organization_id);
CREATE INDEX idx_users_email_org ON mcp.users(email, organization_id);
CREATE INDEX idx_invitations_token ON mcp.user_invitations(invitation_token);
CREATE INDEX idx_invitations_org ON mcp.user_invitations(organization_id);
CREATE INDEX idx_password_reset_token ON mcp.password_reset_tokens(token_hash);
CREATE INDEX idx_email_verification_token ON mcp.email_verification_tokens(token_hash);
CREATE INDEX idx_sessions_user ON mcp.user_sessions(user_id);
CREATE INDEX idx_sessions_token ON mcp.user_sessions(refresh_token_hash);
CREATE INDEX idx_audit_user ON mcp.auth_audit_log(user_id);
CREATE INDEX idx_audit_org ON mcp.auth_audit_log(organization_id);
CREATE INDEX idx_audit_created ON mcp.auth_audit_log(created_at);

-- Add organization owner tracking
ALTER TABLE mcp.organizations ADD COLUMN IF NOT EXISTS owner_user_id UUID REFERENCES mcp.users(id);
ALTER TABLE mcp.organizations ADD COLUMN IF NOT EXISTS subscription_tier VARCHAR(50) DEFAULT 'free' CHECK (subscription_tier IN ('free', 'starter', 'pro', 'enterprise'));
ALTER TABLE mcp.organizations ADD COLUMN IF NOT EXISTS max_users INTEGER DEFAULT 5;
ALTER TABLE mcp.organizations ADD COLUMN IF NOT EXISTS max_agents INTEGER DEFAULT 10;
ALTER TABLE mcp.organizations ADD COLUMN IF NOT EXISTS billing_email VARCHAR(255);

COMMIT;