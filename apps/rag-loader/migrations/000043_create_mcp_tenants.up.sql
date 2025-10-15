-- Migration: Create MCP Tenants Table
-- This migration creates the mcp.tenants table for multi-tenant support

-- Ensure mcp schema exists
CREATE SCHEMA IF NOT EXISTS mcp;

-- Grant usage to app user
GRANT USAGE ON SCHEMA mcp TO devmesh;
GRANT CREATE ON SCHEMA mcp TO devmesh;

-- Create tenants table
-- Note: This is separate from organizations to allow multiple tenants per organization
CREATE TABLE IF NOT EXISTS mcp.tenants (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    organization_id UUID, -- Optional: link to organization
    is_active BOOLEAN DEFAULT true NOT NULL,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_by UUID,
    updated_by UUID
);

-- Create indexes
CREATE INDEX IF NOT EXISTS idx_tenants_organization ON mcp.tenants(organization_id) WHERE organization_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_tenants_active ON mcp.tenants(is_active) WHERE is_active = true;
CREATE INDEX IF NOT EXISTS idx_tenants_name ON mcp.tenants(name);

-- Create update trigger for updated_at
CREATE OR REPLACE FUNCTION mcp.update_tenants_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_tenants_updated_at
    BEFORE UPDATE ON mcp.tenants
    FOR EACH ROW EXECUTE FUNCTION mcp.update_tenants_updated_at();

-- Grant permissions
GRANT ALL ON ALL TABLES IN SCHEMA mcp TO devmesh;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA mcp TO devmesh;

-- Add comment
COMMENT ON TABLE mcp.tenants IS 'Multi-tenant isolation table for RAG loader and other services';
