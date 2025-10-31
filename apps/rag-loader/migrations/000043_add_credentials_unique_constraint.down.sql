-- Rollback: Remove unique constraint from tenant_source_credentials

ALTER TABLE rag.tenant_source_credentials
DROP CONSTRAINT IF EXISTS tenant_source_credentials_tenant_source_type_key;
