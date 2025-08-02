BEGIN;

SET search_path TO mcp, public;

DROP FUNCTION IF EXISTS get_available_models CASCADE;
DROP FUNCTION IF EXISTS search_embeddings CASCADE;
DROP FUNCTION IF EXISTS insert_embedding CASCADE;

DROP INDEX IF EXISTS idx_embeddings_model_dims;
DROP INDEX IF EXISTS idx_embeddings_provider_dims;
DROP INDEX IF EXISTS idx_embeddings_tenant_model;
DROP INDEX IF EXISTS idx_embeddings_context_model;

COMMIT;