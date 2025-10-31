-- Create resources table for MCP protocol support
CREATE TABLE IF NOT EXISTS mcp.resources (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    uri TEXT NOT NULL,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    mime_type VARCHAR(100) NOT NULL,
    content TEXT,
    metadata JSONB DEFAULT '{}',
    tags TEXT[] DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique URI per tenant
    CONSTRAINT unique_tenant_resource_uri UNIQUE(tenant_id, uri)
);

-- Create indexes for efficient querying
CREATE INDEX idx_resources_tenant_id ON mcp.resources(tenant_id);
CREATE INDEX idx_resources_uri ON mcp.resources(uri);
CREATE INDEX idx_resources_mime_type ON mcp.resources(mime_type);
CREATE INDEX idx_resources_tags ON mcp.resources USING GIN(tags);
CREATE INDEX idx_resources_metadata ON mcp.resources USING GIN(metadata);
CREATE INDEX idx_resources_created_at ON mcp.resources(created_at DESC);

-- Create resource subscriptions table
CREATE TABLE IF NOT EXISTS mcp.resource_subscriptions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    resource_id UUID NOT NULL REFERENCES mcp.resources(id) ON DELETE CASCADE,
    agent_id UUID NOT NULL,
    events TEXT[] NOT NULL DEFAULT '{"created", "updated", "deleted"}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique subscription per agent per resource
    CONSTRAINT unique_agent_resource_subscription UNIQUE(agent_id, resource_id)
);

-- Create indexes for subscriptions
CREATE INDEX idx_resource_subscriptions_tenant_id ON mcp.resource_subscriptions(tenant_id);
CREATE INDEX idx_resource_subscriptions_resource_id ON mcp.resource_subscriptions(resource_id);
CREATE INDEX idx_resource_subscriptions_agent_id ON mcp.resource_subscriptions(agent_id);

-- Create trigger to update the updated_at timestamp
CREATE OR REPLACE FUNCTION mcp.update_resource_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER trigger_update_resource_updated_at
    BEFORE UPDATE ON mcp.resources
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_resource_updated_at();

-- Add comments for documentation
COMMENT ON TABLE mcp.resources IS 'Stores resources accessible via MCP protocol';
COMMENT ON TABLE mcp.resource_subscriptions IS 'Tracks agent subscriptions to resource changes';
COMMENT ON COLUMN mcp.resources.uri IS 'Unique resource identifier (URI format)';
COMMENT ON COLUMN mcp.resources.mime_type IS 'MIME type of the resource content';
COMMENT ON COLUMN mcp.resources.metadata IS 'Additional metadata as JSON';
COMMENT ON COLUMN mcp.resource_subscriptions.events IS 'List of events to subscribe to (created, updated, deleted)';