-- Claude Code: Organization hierarchy tables for agent registration system
BEGIN;

-- Organizations table
CREATE TABLE IF NOT EXISTS mcp.organizations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    slug VARCHAR(255) UNIQUE NOT NULL,
    isolation_mode VARCHAR(50) DEFAULT 'strict',
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Organization-tenant mapping
CREATE TABLE IF NOT EXISTS mcp.organization_tenants (
    organization_id UUID REFERENCES mcp.organizations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    tenant_name VARCHAR(255),
    tenant_type VARCHAR(50) DEFAULT 'standard',
    isolation_level VARCHAR(50) DEFAULT 'normal',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (organization_id, tenant_id)
);

-- Tenant access matrix
CREATE TABLE IF NOT EXISTS mcp.tenant_access_matrix (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    source_tenant_id UUID NOT NULL,
    target_tenant_id UUID NOT NULL,
    organization_id UUID REFERENCES mcp.organizations(id) ON DELETE CASCADE,
    access_type VARCHAR(50) NOT NULL,
    permissions JSONB DEFAULT '[]',
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(source_tenant_id, target_tenant_id, access_type)
);

-- Indexes
CREATE INDEX idx_org_tenants_org ON mcp.organization_tenants(organization_id);
CREATE INDEX idx_org_tenants_tenant ON mcp.organization_tenants(tenant_id);
CREATE INDEX idx_access_matrix_source ON mcp.tenant_access_matrix(source_tenant_id);
CREATE INDEX idx_access_matrix_target ON mcp.tenant_access_matrix(target_tenant_id);

COMMIT;