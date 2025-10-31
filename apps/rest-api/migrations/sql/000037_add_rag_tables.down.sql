-- Rollback RAG Loader Database Schema

-- Drop the unified search view
DROP VIEW IF EXISTS rag.unified_search;

-- Remove RAG-specific columns from embeddings table
ALTER TABLE mcp.embeddings
DROP COLUMN IF EXISTS source_type,
DROP COLUMN IF EXISTS importance_score;

-- Drop indexes
DROP INDEX IF EXISTS idx_rag_chunks_content_trgm;
DROP INDEX IF EXISTS idx_rag_chunks_embedding;
DROP INDEX IF EXISTS idx_rag_chunks_document;
DROP INDEX IF EXISTS idx_rag_jobs_status;
DROP INDEX IF EXISTS idx_rag_jobs_tenant;
DROP INDEX IF EXISTS idx_rag_documents_hash;
DROP INDEX IF EXISTS idx_rag_documents_source;
DROP INDEX IF EXISTS idx_rag_documents_tenant;

-- Drop tables in reverse order
DROP TABLE IF EXISTS rag.ingestion_jobs;
DROP TABLE IF EXISTS rag.document_chunks;
DROP TABLE IF EXISTS rag.documents;

-- Drop the rag schema
DROP SCHEMA IF EXISTS rag CASCADE;