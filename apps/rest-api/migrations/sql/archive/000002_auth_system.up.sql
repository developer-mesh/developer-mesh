BEGIN;

SET search_path TO mcp, public;

-- Ensure the trigger function exists (in case it's not in the search path)
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

-- Users table (if needed)
CREATE TABLE IF NOT EXISTS users (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    email VARCHAR(255) UNIQUE NOT NULL,
    tenant_id UUID NOT NULL,
    
    -- User metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_login_at TIMESTAMP WITH TIME ZONE,
    deleted_at TIMESTAMP WITH TIME ZONE,
    
    CONSTRAINT users_email_valid CHECK (email ~* '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}$')
);

-- API Keys table with production features
CREATE TABLE IF NOT EXISTS api_keys (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    key_hash VARCHAR(64) UNIQUE NOT NULL, -- SHA-256 hash of the key
    key_prefix VARCHAR(8) NOT NULL,       -- First 8 chars for identification
    
    -- Ownership
    user_id UUID REFERENCES users(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    
    -- Key metadata
    name VARCHAR(255) NOT NULL,
    description TEXT,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    
    -- Expiration and rotation
    expires_at TIMESTAMP WITH TIME ZONE,
    last_used_at TIMESTAMP WITH TIME ZONE,
    last_rotated_at TIMESTAMP WITH TIME ZONE,
    rotation_version INTEGER NOT NULL DEFAULT 1,
    
    -- Status tracking
    is_active BOOLEAN NOT NULL DEFAULT true,
    revoked_at TIMESTAMP WITH TIME ZONE,
    revoked_reason TEXT,
    
    -- Rate limiting
    rate_limit_requests INTEGER DEFAULT 1000,
    rate_limit_window_seconds INTEGER DEFAULT 3600,
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    ip_whitelist INET[] DEFAULT '{}',
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    CONSTRAINT api_keys_name_not_empty CHECK (length(trim(name)) > 0),
    CONSTRAINT api_keys_valid_expiry CHECK (expires_at IS NULL OR expires_at > created_at)
);

-- Indexes for API keys
CREATE INDEX IF NOT EXISTS idx_api_keys_key_prefix ON api_keys(key_prefix) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_tenant_id ON api_keys(tenant_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_user_id ON api_keys(user_id) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_api_keys_expires_at ON api_keys(expires_at) WHERE is_active = true AND expires_at IS NOT NULL;

-- API key usage tracking for analytics
CREATE TABLE IF NOT EXISTS api_key_usage (
    api_key_id UUID NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    used_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    ip_address INET,
    user_agent TEXT,
    endpoint VARCHAR(255),
    status_code INTEGER,
    response_time_ms INTEGER,
    
    -- Partitioned by month for performance
    PRIMARY KEY (api_key_id, used_at)
) PARTITION BY RANGE (used_at);

-- Create partitions for the next 12 months
DO $$
DECLARE
    start_date date;
    end_date date;
    partition_name text;
BEGIN
    FOR i IN 0..11 LOOP
        start_date := date_trunc('month', CURRENT_DATE + (i || ' months')::interval);
        end_date := start_date + interval '1 month';
        partition_name := 'api_key_usage_' || to_char(start_date, 'YYYY_MM');
        
        EXECUTE format(
            'CREATE TABLE IF NOT EXISTS mcp.%I PARTITION OF mcp.api_key_usage
            FOR VALUES FROM (%L) TO (%L)',
            partition_name, start_date, end_date
        );
    END LOOP;
END $$;

-- Triggers
DROP TRIGGER IF EXISTS update_users_updated_at ON users;
CREATE TRIGGER update_users_updated_at BEFORE UPDATE
ON users FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

DROP TRIGGER IF EXISTS update_api_keys_updated_at ON api_keys;
CREATE TRIGGER update_api_keys_updated_at BEFORE UPDATE
ON api_keys FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

COMMIT;