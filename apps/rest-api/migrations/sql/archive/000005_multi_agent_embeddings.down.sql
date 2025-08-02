BEGIN;

SET search_path TO mcp, public;

-- Drop materialized view and function
DROP MATERIALIZED VIEW IF EXISTS agent_cost_analytics;
DROP FUNCTION IF EXISTS refresh_cost_analytics();

-- Drop tables in reverse order
DROP TABLE IF EXISTS embedding_cache;
DROP TABLE IF EXISTS projection_matrices;
DROP TABLE IF EXISTS embedding_metrics;
DROP TABLE IF EXISTS agent_configs;

-- Remove columns from embeddings table
ALTER TABLE embeddings 
    DROP COLUMN IF EXISTS agent_id,
    DROP COLUMN IF EXISTS task_type,
    DROP COLUMN IF EXISTS normalized_embedding,
    DROP COLUMN IF EXISTS cost_usd,
    DROP COLUMN IF EXISTS generation_time_ms;

-- Drop indexes
DROP INDEX IF EXISTS idx_embeddings_agent_id;
DROP INDEX IF EXISTS idx_embeddings_agent_model;
DROP INDEX IF EXISTS idx_embeddings_task_type;
DROP INDEX IF EXISTS idx_embeddings_normalized_ivfflat;

COMMIT;