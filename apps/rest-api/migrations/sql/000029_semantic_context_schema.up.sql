-- Story 1.1: Context-Embedding Link Tables
-- Migration: 000033_semantic_context_schema
-- Created: 2025-10-11
-- Purpose: Enable semantic context management with embedding relationships

-- Link table between contexts and embeddings
CREATE TABLE IF NOT EXISTS mcp.context_embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES mcp.contexts(id) ON DELETE CASCADE,
    embedding_id UUID NOT NULL REFERENCES mcp.embeddings(id) ON DELETE CASCADE,
    chunk_sequence INT NOT NULL,
    importance_score FLOAT DEFAULT 0.5 CHECK (importance_score >= 0 AND importance_score <= 1),
    is_summary BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(context_id, chunk_sequence)
);

-- Indexes for performance
CREATE INDEX idx_context_embeddings_context ON mcp.context_embeddings(context_id);
CREATE INDEX idx_context_embeddings_embedding ON mcp.context_embeddings(embedding_id);
CREATE INDEX idx_context_embeddings_importance ON mcp.context_embeddings(context_id, importance_score DESC);
CREATE INDEX idx_context_embeddings_created ON mcp.context_embeddings(created_at);
CREATE INDEX idx_context_embeddings_sequence ON mcp.context_embeddings(context_id, chunk_sequence);

-- Audit log table for compliance
CREATE TABLE IF NOT EXISTS mcp.context_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    context_id UUID NOT NULL REFERENCES mcp.contexts(id) ON DELETE CASCADE,
    operation VARCHAR(50) NOT NULL CHECK (operation IN ('create', 'read', 'update', 'delete', 'compact', 'semantic_retrieval')),
    user_id VARCHAR(255),
    tenant_id UUID,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMP DEFAULT NOW()
);

-- Indexes for audit log
CREATE INDEX idx_context_audit_context ON mcp.context_audit_log(context_id);
CREATE INDEX idx_context_audit_created ON mcp.context_audit_log(created_at DESC);
CREATE INDEX idx_context_audit_tenant ON mcp.context_audit_log(tenant_id);
CREATE INDEX idx_context_audit_operation ON mcp.context_audit_log(operation, created_at DESC);

-- Add trigger for updated_at on context_embeddings
CREATE TRIGGER update_context_embeddings_updated_at
    BEFORE UPDATE ON mcp.context_embeddings
    FOR EACH ROW
    EXECUTE FUNCTION update_updated_at_column();

-- Add compaction metadata columns to contexts table
ALTER TABLE mcp.contexts
    ADD COLUMN IF NOT EXISTS compaction_strategy VARCHAR(50),
    ADD COLUMN IF NOT EXISTS last_compacted_at TIMESTAMP,
    ADD COLUMN IF NOT EXISTS compaction_count INT DEFAULT 0;

-- Index for contexts needing compaction
CREATE INDEX idx_contexts_compaction ON mcp.contexts(last_compacted_at)
    WHERE status = 'active' AND compression_enabled = true;

-- Comments for documentation
COMMENT ON TABLE mcp.context_embeddings IS 'Links context items to their semantic embeddings with importance scoring';
COMMENT ON TABLE mcp.context_audit_log IS 'Audit trail for all context operations for compliance and debugging';
COMMENT ON COLUMN mcp.context_embeddings.importance_score IS 'Importance score (0-1) for relevance ranking';
COMMENT ON COLUMN mcp.context_embeddings.is_summary IS 'True if this embedding represents a summary of multiple items';
COMMENT ON COLUMN mcp.contexts.compaction_strategy IS 'Last compaction strategy used: summarize, prune, semantic, sliding, tool_clear';
COMMENT ON COLUMN mcp.contexts.last_compacted_at IS 'Timestamp of last compaction operation';
