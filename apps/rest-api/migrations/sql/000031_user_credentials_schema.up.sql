-- Migration: User Credentials Storage
-- Description: Stores encrypted user credentials for third-party service integrations
-- Author: DevMesh Team
-- Date: 2025-10-20

-- Create user_credentials table for storing encrypted service credentials
CREATE TABLE IF NOT EXISTS mcp.user_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL, -- Multi-tenant isolation (no FK, follows users.tenant_id pattern)
    user_id UUID NOT NULL REFERENCES mcp.users(id) ON DELETE CASCADE,
    service_type VARCHAR(50) NOT NULL CHECK (service_type IN (
        'github',
        'harness',
        'aws',
        'azure',
        'gcp',
        'snyk',
        'jira',
        'slack',
        'generic'
    )),
    encrypted_credentials BYTEA NOT NULL, -- Encrypted JSON containing service-specific credentials
    encryption_key_version INT NOT NULL DEFAULT 1, -- Track encryption key version for rotation
    is_active BOOLEAN DEFAULT true, -- Allow disabling without deleting
    metadata JSONB DEFAULT '{}', -- Store service-specific metadata (regions, scopes, etc.)
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    last_used_at TIMESTAMP, -- Track when credentials were last used
    expires_at TIMESTAMP, -- Optional expiration for credentials
    UNIQUE(tenant_id, user_id, service_type) -- One set of credentials per service per user
);

-- Create indexes for efficient lookups
CREATE INDEX idx_user_credentials_lookup ON mcp.user_credentials(tenant_id, user_id, service_type) WHERE is_active = true;
CREATE INDEX idx_user_credentials_user ON mcp.user_credentials(user_id) WHERE is_active = true;
CREATE INDEX idx_user_credentials_last_used ON mcp.user_credentials(last_used_at) WHERE last_used_at IS NOT NULL;

-- Create audit table for credential operations
CREATE TABLE IF NOT EXISTS mcp.user_credentials_audit (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    credential_id UUID NOT NULL REFERENCES mcp.user_credentials(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    user_id UUID NOT NULL,
    service_type VARCHAR(50) NOT NULL,
    operation VARCHAR(20) NOT NULL CHECK (operation IN ('created', 'updated', 'deleted', 'used', 'validated')),
    success BOOLEAN NOT NULL DEFAULT true,
    error_message TEXT, -- Store error if operation failed
    ip_address INET, -- Track where request came from
    user_agent TEXT, -- Track client information
    metadata JSONB DEFAULT '{}', -- Additional context
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

-- Index for audit queries
CREATE INDEX idx_user_credentials_audit_lookup ON mcp.user_credentials_audit(tenant_id, user_id, created_at DESC);
CREATE INDEX idx_user_credentials_audit_credential ON mcp.user_credentials_audit(credential_id, created_at DESC);

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION mcp.update_user_credentials_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER user_credentials_updated_at
    BEFORE UPDATE ON mcp.user_credentials
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_user_credentials_updated_at();

-- Add RLS policies for multi-tenant isolation
ALTER TABLE mcp.user_credentials ENABLE ROW LEVEL SECURITY;

-- Policy: Users can only access their own credentials
CREATE POLICY user_credentials_tenant_isolation ON mcp.user_credentials
    FOR ALL
    USING (tenant_id = current_setting('app.tenant_id', true)::UUID);

-- Grant permissions (if devmesh role exists)
-- In test/CI environments, the role may be 'test' instead of 'devmesh'
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'devmesh') THEN
        GRANT SELECT, INSERT, UPDATE, DELETE ON mcp.user_credentials TO devmesh;
        GRANT SELECT, INSERT ON mcp.user_credentials_audit TO devmesh;
    END IF;
END $$;

-- Add comments for documentation
COMMENT ON TABLE mcp.user_credentials IS 'Stores encrypted user credentials for third-party service integrations with per-tenant encryption';
COMMENT ON COLUMN mcp.user_credentials.encrypted_credentials IS 'AES-256-GCM encrypted JSON blob containing service-specific credentials (tokens, keys, etc.)';
COMMENT ON COLUMN mcp.user_credentials.encryption_key_version IS 'Version of encryption key used, allows for key rotation without re-encrypting all data';
COMMENT ON COLUMN mcp.user_credentials.metadata IS 'Service-specific metadata like AWS region, Harness modules, etc.';
COMMENT ON TABLE mcp.user_credentials_audit IS 'Audit trail for all credential operations for security and compliance';
