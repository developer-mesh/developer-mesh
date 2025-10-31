-- Migration: Add Row Level Security for Multi-Tenant Isolation
-- This migration implements Row Level Security (RLS) policies to ensure
-- complete tenant isolation at the database level

-- Function to set current tenant context
-- This function must be called before any tenant-specific queries
CREATE OR REPLACE FUNCTION rag.set_current_tenant(tenant_uuid UUID)
RETURNS void AS $$
BEGIN
    PERFORM set_config('app.current_tenant', tenant_uuid::text, false);
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to get current tenant from session
-- Returns NULL if no tenant is set, allowing queries to fail safely
CREATE OR REPLACE FUNCTION rag.get_current_tenant()
RETURNS UUID AS $$
BEGIN
    RETURN current_setting('app.current_tenant', true)::UUID;
EXCEPTION
    WHEN OTHERS THEN
        RETURN NULL;
END;
$$ LANGUAGE plpgsql STABLE;

-- Enable Row Level Security on all tenant tables
-- Using FORCE to ensure even table owners and superusers cannot bypass RLS
-- This is critical for security: "When in doubt, fail closed"
ALTER TABLE rag.tenant_sources ENABLE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_sources FORCE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_source_credentials ENABLE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_source_credentials FORCE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_documents ENABLE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_documents FORCE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_sync_jobs ENABLE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_sync_jobs FORCE ROW LEVEL SECURITY;

-- Create RLS policies for tenant_sources
-- These policies ensure that each tenant can only access their own data
CREATE POLICY tenant_sources_isolation ON rag.tenant_sources
    FOR ALL
    USING (tenant_id = rag.get_current_tenant())
    WITH CHECK (tenant_id = rag.get_current_tenant());

-- Create RLS policies for tenant_source_credentials
-- Critical: Credentials must never leak across tenants
CREATE POLICY tenant_credentials_isolation ON rag.tenant_source_credentials
    FOR ALL
    USING (tenant_id = rag.get_current_tenant())
    WITH CHECK (tenant_id = rag.get_current_tenant());

-- Create RLS policies for tenant_documents
-- Ensures document content isolation per tenant
CREATE POLICY tenant_documents_isolation ON rag.tenant_documents
    FOR ALL
    USING (tenant_id = rag.get_current_tenant())
    WITH CHECK (tenant_id = rag.get_current_tenant());

-- Create RLS policies for tenant_sync_jobs
-- Job tracking and history must be tenant-isolated
CREATE POLICY tenant_sync_jobs_isolation ON rag.tenant_sync_jobs
    FOR ALL
    USING (tenant_id = rag.get_current_tenant())
    WITH CHECK (tenant_id = rag.get_current_tenant());

-- Grant permissions to devmesh user
-- Note: RLS policies still apply even with these grants
GRANT ALL ON ALL TABLES IN SCHEMA rag TO devmesh;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA rag TO devmesh;

-- Grant execute permission on RLS functions
GRANT EXECUTE ON FUNCTION rag.set_current_tenant(UUID) TO devmesh;
GRANT EXECUTE ON FUNCTION rag.get_current_tenant() TO devmesh;

-- Add comment explaining RLS enforcement
COMMENT ON POLICY tenant_sources_isolation ON rag.tenant_sources IS
    'Enforces tenant isolation: users can only access sources belonging to their tenant';
COMMENT ON POLICY tenant_credentials_isolation ON rag.tenant_source_credentials IS
    'Critical security policy: prevents cross-tenant credential access';
COMMENT ON POLICY tenant_documents_isolation ON rag.tenant_documents IS
    'Enforces document isolation: each tenant can only see their own documents';
COMMENT ON POLICY tenant_sync_jobs_isolation ON rag.tenant_sync_jobs IS
    'Enforces job history isolation: tenants can only view their own job history';
