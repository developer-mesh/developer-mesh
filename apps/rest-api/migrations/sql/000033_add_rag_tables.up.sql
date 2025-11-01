-- Phase 1: RAG Loader Database Schema
-- Create RAG schema for document ingestion and processing

CREATE SCHEMA IF NOT EXISTS rag;

-- Documents table - stores document metadata
CREATE TABLE IF NOT EXISTS rag.documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    source_type VARCHAR(50) NOT NULL,
    url TEXT,
    title TEXT,
    content_hash VARCHAR(64) UNIQUE,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rag_documents_tenant ON rag.documents(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rag_documents_source ON rag.documents(source_id, source_type);
CREATE INDEX IF NOT EXISTS idx_rag_documents_hash ON rag.documents(content_hash);

-- Document chunks table - stores chunked content from documents
CREATE TABLE IF NOT EXISTS rag.document_chunks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    document_id UUID REFERENCES rag.documents(id) ON DELETE CASCADE,
    chunk_index INT NOT NULL,
    content TEXT NOT NULL,
    start_char INT,
    end_char INT,
    embedding_id UUID REFERENCES mcp.embeddings(id),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(document_id, chunk_index)
);

CREATE INDEX IF NOT EXISTS idx_rag_chunks_document ON rag.document_chunks(document_id);
CREATE INDEX IF NOT EXISTS idx_rag_chunks_embedding ON rag.document_chunks(embedding_id);

-- Ingestion jobs table - tracks RAG ingestion jobs
CREATE TABLE IF NOT EXISTS rag.ingestion_jobs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    source_id VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL,
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    documents_processed INT DEFAULT 0,
    chunks_created INT DEFAULT 0,
    embeddings_created INT DEFAULT 0,
    error_message TEXT,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_rag_jobs_tenant ON rag.ingestion_jobs(tenant_id);
CREATE INDEX IF NOT EXISTS idx_rag_jobs_status ON rag.ingestion_jobs(status);

-- Add RAG-specific columns to existing embeddings table
ALTER TABLE mcp.embeddings
ADD COLUMN IF NOT EXISTS source_type VARCHAR(50) DEFAULT 'context',
ADD COLUMN IF NOT EXISTS importance_score FLOAT DEFAULT 0.5;

-- For BM25 search (keyword search), install pg_trgm extension
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Create text search index for keyword search
CREATE INDEX IF NOT EXISTS idx_rag_chunks_content_trgm
ON rag.document_chunks USING gin (content gin_trgm_ops);

-- Create a unified search view (optional, for easier querying)
CREATE OR REPLACE VIEW rag.unified_search AS
SELECT
    e.id as embedding_id,
    e.content,
    e.embedding,
    e.model_name,
    e.tenant_id,
    COALESCE(d.source_type, 'context') as content_source,
    COALESCE(d.source_id, e.context_id::text) as source_identifier,
    COALESCE(dc.metadata, e.metadata) as metadata,
    e.importance_score
FROM mcp.embeddings e
LEFT JOIN rag.document_chunks dc ON dc.embedding_id = e.id
LEFT JOIN rag.documents d ON d.id = dc.document_id;

-- Grant appropriate permissions (if devmesh role exists)
-- In test/CI environments, the role may be 'test' instead of 'devmesh'
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'devmesh') THEN
        GRANT USAGE ON SCHEMA rag TO devmesh;
        GRANT ALL ON ALL TABLES IN SCHEMA rag TO devmesh;
        GRANT ALL ON ALL SEQUENCES IN SCHEMA rag TO devmesh;
    END IF;
END $$;