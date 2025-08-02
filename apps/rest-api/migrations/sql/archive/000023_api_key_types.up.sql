BEGIN;

-- Add key type and gateway features to api_keys
ALTER TABLE mcp.api_keys 
ADD COLUMN IF NOT EXISTS key_type VARCHAR(50) NOT NULL DEFAULT 'user',
ADD COLUMN IF NOT EXISTS parent_key_id UUID REFERENCES mcp.api_keys(id),
ADD COLUMN IF NOT EXISTS allowed_services TEXT[] DEFAULT '{}';

-- Create tenant configuration table
CREATE TABLE IF NOT EXISTS mcp.tenant_config (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID UNIQUE NOT NULL,
    
    -- Rate limit overrides
    rate_limit_config JSONB NOT NULL DEFAULT '{}',
    
    -- Service tokens (encrypted)
    service_tokens JSONB DEFAULT '{}', -- {"github": "encrypted_token", ...}
    
    -- Allowed origins for CORS
    allowed_origins TEXT[] DEFAULT '{}',
    
    -- Feature flags
    features JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Add indexes
CREATE INDEX idx_api_keys_type ON mcp.api_keys(key_type, tenant_id) WHERE is_active = true;
CREATE INDEX idx_api_keys_parent ON mcp.api_keys(parent_key_id) WHERE parent_key_id IS NOT NULL;

-- Update existing keys to have type 'user'
UPDATE mcp.api_keys SET key_type = 'user' WHERE key_type IS NULL;

-- Add trigger for tenant_config updated_at
DROP TRIGGER IF EXISTS update_tenant_config_updated_at ON mcp.tenant_config;
CREATE TRIGGER update_tenant_config_updated_at BEFORE UPDATE
ON mcp.tenant_config FOR EACH ROW EXECUTE FUNCTION mcp.update_updated_at_column();

COMMIT;