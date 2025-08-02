-- Create webhook_configs table for multi-organization webhook support
CREATE TABLE IF NOT EXISTS webhook_configs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    organization_name VARCHAR(255) NOT NULL UNIQUE,
    webhook_secret TEXT NOT NULL, -- Will be encrypted using pgcrypto
    enabled BOOLEAN DEFAULT true,
    allowed_events TEXT[] DEFAULT ARRAY['issues', 'issue_comment', 'pull_request', 'push', 'release'],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

-- Create index for organization lookup
CREATE INDEX idx_webhook_configs_org_name ON webhook_configs(organization_name);
CREATE INDEX idx_webhook_configs_enabled ON webhook_configs(enabled) WHERE enabled = true;

-- Add trigger to update updated_at
CREATE TRIGGER update_webhook_configs_updated_at
    BEFORE UPDATE ON webhook_configs
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Insert the current webhook configuration for developer-mesh org
-- The secret will be updated by the deployment process
INSERT INTO webhook_configs (organization_name, webhook_secret, enabled) 
VALUES ('developer-mesh', 'PLACEHOLDER_WILL_BE_UPDATED_BY_DEPLOYMENT', true)
ON CONFLICT (organization_name) DO NOTHING;

-- Add comment
COMMENT ON TABLE webhook_configs IS 'Stores webhook configurations for multiple GitHub organizations';
COMMENT ON COLUMN webhook_configs.organization_name IS 'GitHub organization name';
COMMENT ON COLUMN webhook_configs.webhook_secret IS 'Encrypted webhook secret for HMAC validation';
COMMENT ON COLUMN webhook_configs.allowed_events IS 'List of GitHub event types this org is allowed to send';
COMMENT ON COLUMN webhook_configs.metadata IS 'Additional configuration data specific to the organization';