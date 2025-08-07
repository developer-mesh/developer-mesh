-- Agent Manifest System for Universal Agent Registration
-- Supports IDEs, Slack, monitoring tools, and any other agent type

-- Agent manifests table
CREATE TABLE IF NOT EXISTS mcp.agent_manifests (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID REFERENCES mcp.organizations(id) ON DELETE CASCADE,
    agent_id VARCHAR(255) UNIQUE NOT NULL,
    agent_type VARCHAR(100) NOT NULL, -- ide, slack, monitoring, custom, etc.
    name VARCHAR(255) NOT NULL,
    version VARCHAR(50) NOT NULL,
    description TEXT,
    
    -- Capabilities (what the agent can do)
    capabilities JSONB DEFAULT '[]',
    
    -- Requirements (what the agent needs)
    requirements JSONB DEFAULT '{}',
    
    -- Connection configuration
    connection_config JSONB DEFAULT '{}',
    
    -- Authentication configuration
    auth_config JSONB DEFAULT '{}',
    
    -- Metadata and status
    metadata JSONB DEFAULT '{}',
    status VARCHAR(50) DEFAULT 'inactive',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    
    -- Audit fields
    created_by UUID,
    updated_by UUID,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Agent registrations (instances of agents)
CREATE TABLE IF NOT EXISTS mcp.agent_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    manifest_id UUID REFERENCES mcp.agent_manifests(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    instance_id VARCHAR(255) NOT NULL,
    
    -- Registration details
    registration_token TEXT,
    registration_status VARCHAR(50) DEFAULT 'pending',
    activation_date TIMESTAMP WITH TIME ZONE,
    expiration_date TIMESTAMP WITH TIME ZONE,
    
    -- Runtime configuration
    runtime_config JSONB DEFAULT '{}',
    
    -- Connection details
    connection_details JSONB DEFAULT '{}',
    
    -- Health and metrics
    health_status VARCHAR(50) DEFAULT 'unknown',
    health_check_url TEXT,
    last_health_check TIMESTAMP WITH TIME ZONE,
    metrics JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(tenant_id, instance_id)
);

-- Agent capabilities table (normalized for querying)
CREATE TABLE IF NOT EXISTS mcp.agent_capabilities (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    manifest_id UUID REFERENCES mcp.agent_manifests(id) ON DELETE CASCADE,
    capability_type VARCHAR(100) NOT NULL,
    capability_name VARCHAR(255) NOT NULL,
    capability_config JSONB DEFAULT '{}',
    required BOOLEAN DEFAULT false,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(manifest_id, capability_type, capability_name)
);

-- Agent communication channels
CREATE TABLE IF NOT EXISTS mcp.agent_channels (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    registration_id UUID REFERENCES mcp.agent_registrations(id) ON DELETE CASCADE,
    channel_type VARCHAR(50) NOT NULL, -- websocket, http, grpc, redis, etc.
    channel_config JSONB NOT NULL,
    priority INTEGER DEFAULT 0,
    active BOOLEAN DEFAULT true,
    last_message_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(registration_id, channel_type)
);

-- Create indexes for performance
CREATE INDEX idx_agent_manifests_org_id ON mcp.agent_manifests(organization_id);
CREATE INDEX idx_agent_manifests_agent_type ON mcp.agent_manifests(agent_type);
CREATE INDEX idx_agent_manifests_status ON mcp.agent_manifests(status);
CREATE INDEX idx_agent_manifests_capabilities ON mcp.agent_manifests USING gin(capabilities);

CREATE INDEX idx_agent_registrations_manifest_id ON mcp.agent_registrations(manifest_id);
CREATE INDEX idx_agent_registrations_tenant_id ON mcp.agent_registrations(tenant_id);
CREATE INDEX idx_agent_registrations_status ON mcp.agent_registrations(registration_status);
CREATE INDEX idx_agent_registrations_health ON mcp.agent_registrations(health_status);

CREATE INDEX idx_agent_capabilities_manifest_id ON mcp.agent_capabilities(manifest_id);
CREATE INDEX idx_agent_capabilities_type ON mcp.agent_capabilities(capability_type);

CREATE INDEX idx_agent_channels_registration_id ON mcp.agent_channels(registration_id);
CREATE INDEX idx_agent_channels_type ON mcp.agent_channels(channel_type);
CREATE INDEX idx_agent_channels_active ON mcp.agent_channels(active);

-- Add trigger to update updated_at timestamp
CREATE OR REPLACE FUNCTION mcp.update_agent_manifest_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_agent_manifests_timestamp
    BEFORE UPDATE ON mcp.agent_manifests
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_agent_manifest_timestamp();

CREATE TRIGGER update_agent_registrations_timestamp
    BEFORE UPDATE ON mcp.agent_registrations
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_agent_manifest_timestamp();