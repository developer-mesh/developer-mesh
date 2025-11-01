-- Migration: Create Multi-Tenant RAG Tables
-- This migration creates the multi-tenant structure for RAG loader
-- with proper tenant isolation and security

-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS vector;

-- Ensure rag schema exists
CREATE SCHEMA IF NOT EXISTS rag;

-- Grant usage to app user (if devmesh role exists)
-- In test/CI environments, the role may be 'test' instead of 'devmesh'
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'devmesh') THEN
        GRANT USAGE ON SCHEMA rag TO devmesh;
        GRANT CREATE ON SCHEMA rag TO devmesh;
    END IF;
END $$;

-- Tenant sources configuration
-- This table stores source configurations per tenant
CREATE TABLE IF NOT EXISTS rag.tenant_sources (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL CHECK (
        source_type IN ('github_org', 'github_repo', 'gitlab', 'confluence', 'slack', 's3', 'jira', 'notion')
    ),
    enabled BOOLEAN DEFAULT true,
    schedule VARCHAR(100), -- Cron expression
    config JSONB NOT NULL DEFAULT '{}',
    last_sync_at TIMESTAMP,
    next_sync_at TIMESTAMP,
    sync_status VARCHAR(50) DEFAULT 'pending' CHECK (
        sync_status IN ('pending', 'running', 'completed', 'failed', 'cancelled')
    ),
    sync_error_count INTEGER DEFAULT 0,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    UNIQUE(tenant_id, source_id)
);

-- Encrypted credentials storage
-- Stores encrypted credentials per tenant/source with proper key management
CREATE TABLE IF NOT EXISTS rag.tenant_source_credentials (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    credential_type VARCHAR(50) NOT NULL CHECK (
        credential_type IN ('token', 'api_key', 'oauth', 'basic_auth', 'aws_role')
    ),
    encrypted_value TEXT NOT NULL,
    kms_key_id VARCHAR(255), -- For AWS KMS
    expires_at TIMESTAMP,
    last_rotated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id, source_id)
        REFERENCES rag.tenant_sources(tenant_id, source_id) ON DELETE CASCADE,
    UNIQUE (tenant_id, source_id, credential_type)
);

-- Document storage
-- Enhanced document table with tenant isolation and embeddings support
CREATE TABLE IF NOT EXISTS rag.tenant_documents (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    document_id VARCHAR(500) NOT NULL,
    parent_id VARCHAR(500),
    document_type VARCHAR(50) DEFAULT 'file',
    title TEXT,
    content TEXT NOT NULL,
    content_hash VARCHAR(64),
    metadata JSONB DEFAULT '{}',
    chunk_index INTEGER DEFAULT 0,
    chunk_total INTEGER DEFAULT 1,
    token_count INTEGER,
    language VARCHAR(50),
    embedding vector(1536),
    importance_score FLOAT DEFAULT 0.5,
    indexed_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id, source_id)
        REFERENCES rag.tenant_sources(tenant_id, source_id) ON DELETE CASCADE,
    UNIQUE(tenant_id, source_id, document_id, chunk_index)
);

-- Sync job tracking
-- Comprehensive job tracking with detailed metrics
CREATE TABLE IF NOT EXISTS rag.tenant_sync_jobs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    job_type VARCHAR(50) NOT NULL DEFAULT 'scheduled' CHECK (
        job_type IN ('scheduled', 'manual', 'initial', 'incremental', 'full')
    ),
    status VARCHAR(50) NOT NULL DEFAULT 'pending' CHECK (
        status IN ('pending', 'queued', 'running', 'completed', 'failed', 'cancelled')
    ),
    priority INTEGER DEFAULT 5,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    documents_processed INTEGER DEFAULT 0,
    documents_added INTEGER DEFAULT 0,
    documents_updated INTEGER DEFAULT 0,
    documents_deleted INTEGER DEFAULT 0,
    chunks_created INTEGER DEFAULT 0,
    errors_count INTEGER DEFAULT 0,
    error_message TEXT,
    error_details JSONB,
    duration_ms INTEGER,
    memory_used_mb INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (tenant_id, source_id)
        REFERENCES rag.tenant_sources(tenant_id, source_id) ON DELETE CASCADE
);

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_tenant_sources_tenant ON rag.tenant_sources(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_sources_enabled ON rag.tenant_sources(enabled) WHERE enabled = true;
CREATE INDEX IF NOT EXISTS idx_tenant_sources_schedule ON rag.tenant_sources(next_sync_at) WHERE enabled = true;

CREATE INDEX IF NOT EXISTS idx_tenant_credentials_tenant ON rag.tenant_source_credentials(tenant_id, source_id);
CREATE INDEX IF NOT EXISTS idx_tenant_credentials_expiry ON rag.tenant_source_credentials(expires_at) WHERE expires_at IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_tenant_documents_tenant ON rag.tenant_documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_documents_source ON rag.tenant_documents(tenant_id, source_id);
CREATE INDEX IF NOT EXISTS idx_tenant_documents_type ON rag.tenant_documents(document_type);
CREATE INDEX IF NOT EXISTS idx_tenant_documents_content_gin ON rag.tenant_documents USING GIN(to_tsvector('english', content));
CREATE INDEX IF NOT EXISTS idx_tenant_documents_metadata ON rag.tenant_documents USING GIN(metadata);

CREATE INDEX IF NOT EXISTS idx_tenant_sync_jobs_tenant ON rag.tenant_sync_jobs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_tenant_sync_jobs_status ON rag.tenant_sync_jobs(status, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_tenant_sync_jobs_source ON rag.tenant_sync_jobs(tenant_id, source_id, created_at DESC);

-- Create update trigger for updated_at
CREATE OR REPLACE FUNCTION rag.update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_tenant_sources_updated_at
    BEFORE UPDATE ON rag.tenant_sources
    FOR EACH ROW EXECUTE FUNCTION rag.update_updated_at_column();

CREATE TRIGGER update_tenant_documents_updated_at
    BEFORE UPDATE ON rag.tenant_documents
    FOR EACH ROW EXECUTE FUNCTION rag.update_updated_at_column();

CREATE TRIGGER update_tenant_source_credentials_updated_at
    BEFORE UPDATE ON rag.tenant_source_credentials
    FOR EACH ROW EXECUTE FUNCTION rag.update_updated_at_column();

-- Grant permissions (if devmesh role exists)
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'devmesh') THEN
        GRANT ALL ON ALL TABLES IN SCHEMA rag TO devmesh;
        GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA rag TO devmesh;
    END IF;
END $$;
