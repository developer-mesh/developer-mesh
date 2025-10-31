-- Rollback: Remove Row Level Security Policies
-- This migration removes all RLS policies and functions
-- WARNING: Removing RLS will allow cross-tenant access - use with caution

-- Revoke execute permissions on RLS functions
REVOKE EXECUTE ON FUNCTION rag.set_current_tenant(UUID) FROM devmesh;
REVOKE EXECUTE ON FUNCTION rag.get_current_tenant() FROM devmesh;

-- Drop RLS policies
DROP POLICY IF EXISTS tenant_sync_jobs_isolation ON rag.tenant_sync_jobs;
DROP POLICY IF EXISTS tenant_documents_isolation ON rag.tenant_documents;
DROP POLICY IF EXISTS tenant_credentials_isolation ON rag.tenant_source_credentials;
DROP POLICY IF EXISTS tenant_sources_isolation ON rag.tenant_sources;

-- Disable Row Level Security on all tables
-- First remove FORCE, then disable RLS
ALTER TABLE rag.tenant_sync_jobs NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_sync_jobs DISABLE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_documents NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_documents DISABLE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_source_credentials NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_source_credentials DISABLE ROW LEVEL SECURITY;

ALTER TABLE rag.tenant_sources NO FORCE ROW LEVEL SECURITY;
ALTER TABLE rag.tenant_sources DISABLE ROW LEVEL SECURITY;

-- Drop RLS functions
DROP FUNCTION IF EXISTS rag.get_current_tenant();
DROP FUNCTION IF EXISTS rag.set_current_tenant(UUID);
