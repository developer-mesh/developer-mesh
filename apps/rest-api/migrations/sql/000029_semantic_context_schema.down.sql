-- Rollback for Story 1.1: Semantic Context Schema
-- Migration: 000033_semantic_context_schema (DOWN)

-- Drop indexes first
DROP INDEX IF EXISTS mcp.idx_contexts_compaction;
DROP INDEX IF EXISTS mcp.idx_context_audit_operation;
DROP INDEX IF EXISTS mcp.idx_context_audit_tenant;
DROP INDEX IF EXISTS mcp.idx_context_audit_created;
DROP INDEX IF EXISTS mcp.idx_context_audit_context;
DROP INDEX IF EXISTS mcp.idx_context_embeddings_sequence;
DROP INDEX IF EXISTS mcp.idx_context_embeddings_created;
DROP INDEX IF EXISTS mcp.idx_context_embeddings_importance;
DROP INDEX IF EXISTS mcp.idx_context_embeddings_embedding;
DROP INDEX IF EXISTS mcp.idx_context_embeddings_context;

-- Drop trigger
DROP TRIGGER IF EXISTS update_context_embeddings_updated_at ON mcp.context_embeddings;

-- Remove added columns from contexts
ALTER TABLE mcp.contexts
    DROP COLUMN IF EXISTS compaction_strategy,
    DROP COLUMN IF EXISTS last_compacted_at,
    DROP COLUMN IF EXISTS compaction_count;

-- Drop tables (order matters due to foreign keys)
DROP TABLE IF EXISTS mcp.context_audit_log;
DROP TABLE IF EXISTS mcp.context_embeddings;
