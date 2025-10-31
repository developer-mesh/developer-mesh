-- Rollback: Drop Multi-Tenant RAG Tables
-- This migration removes all tenant-specific tables and structures

-- Drop triggers
DROP TRIGGER IF EXISTS update_tenant_source_credentials_updated_at ON rag.tenant_source_credentials;
DROP TRIGGER IF EXISTS update_tenant_documents_updated_at ON rag.tenant_documents;
DROP TRIGGER IF EXISTS update_tenant_sources_updated_at ON rag.tenant_sources;

-- Drop trigger function
DROP FUNCTION IF EXISTS rag.update_updated_at_column();

-- Drop indexes
DROP INDEX IF EXISTS idx_tenant_sync_jobs_source;
DROP INDEX IF EXISTS idx_tenant_sync_jobs_status;
DROP INDEX IF EXISTS idx_tenant_sync_jobs_tenant;

DROP INDEX IF EXISTS idx_tenant_documents_metadata;
DROP INDEX IF EXISTS idx_tenant_documents_content_gin;
DROP INDEX IF EXISTS idx_tenant_documents_type;
DROP INDEX IF EXISTS idx_tenant_documents_source;
DROP INDEX IF EXISTS idx_tenant_documents_tenant;

DROP INDEX IF EXISTS idx_tenant_credentials_expiry;
DROP INDEX IF EXISTS idx_tenant_credentials_tenant;

DROP INDEX IF EXISTS idx_tenant_sources_schedule;
DROP INDEX IF EXISTS idx_tenant_sources_enabled;
DROP INDEX IF EXISTS idx_tenant_sources_tenant;

-- Drop tables in reverse dependency order
DROP TABLE IF EXISTS rag.tenant_sync_jobs;
DROP TABLE IF EXISTS rag.tenant_documents;
DROP TABLE IF EXISTS rag.tenant_source_credentials;
DROP TABLE IF EXISTS rag.tenant_sources;

-- Note: We don't drop the rag schema itself as it may contain other tables
-- Note: We don't drop extensions as they may be used by other schemas
