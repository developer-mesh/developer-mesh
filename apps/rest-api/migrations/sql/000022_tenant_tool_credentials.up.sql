-- Migration: 000026_tenant_tool_credentials.up.sql
-- Purpose: Enable tenant-specific tool credentials for Edge MCPs
-- Since this is greenfield, we go directly to the desired state

BEGIN;

-- Tenant tool credentials table
CREATE TABLE IF NOT EXISTS mcp.tenant_tool_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    tool_id UUID REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    
    -- Credential details
    credential_name VARCHAR(255) NOT NULL,
    credential_type VARCHAR(50) NOT NULL,
    encrypted_value TEXT NOT NULL,
    
    -- OAuth specific fields (optional)
    oauth_client_id VARCHAR(255),
    oauth_client_secret_encrypted TEXT,
    oauth_refresh_token_encrypted TEXT,
    oauth_token_expiry TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    description TEXT,
    tags TEXT[],
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_used_at TIMESTAMP WITH TIME ZONE,
    
    -- Edge MCP associations
    edge_mcp_id VARCHAR(255),
    allowed_edge_mcps TEXT[],
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Constraints
    CONSTRAINT uk_tenant_tool_credential UNIQUE(tenant_id, tool_id, credential_name),
    CONSTRAINT chk_credential_type CHECK (credential_type IN ('api_key', 'oauth2', 'basic', 'custom'))
);

-- Edge MCP registrations table
CREATE TABLE IF NOT EXISTS mcp.edge_mcp_registrations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    edge_mcp_id VARCHAR(255) NOT NULL,
    
    -- Registration details
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    host_machine VARCHAR(255),
    
    -- Authentication
    api_key_hash VARCHAR(255) NOT NULL,
    
    -- Configuration
    allowed_tools TEXT[],
    max_connections INTEGER DEFAULT 10,
    
    -- Status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    last_heartbeat TIMESTAMP WITH TIME ZONE,
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT uk_edge_mcp_registration UNIQUE(tenant_id, edge_mcp_id),
    CONSTRAINT chk_edge_status CHECK (status IN ('pending', 'active', 'suspended', 'deactivated'))
);

-- Indexes for performance
CREATE INDEX idx_tenant_tool_credentials_lookup 
    ON mcp.tenant_tool_credentials(tenant_id, tool_id, is_active);

CREATE INDEX idx_edge_mcp_active 
    ON mcp.edge_mcp_registrations(tenant_id, status) 
    WHERE status = 'active';

COMMIT;
