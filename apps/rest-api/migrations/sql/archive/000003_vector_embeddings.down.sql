BEGIN;

SET search_path TO mcp, public;

DROP TABLE IF EXISTS embedding_searches CASCADE;
DROP TABLE IF EXISTS embeddings CASCADE;
DROP TABLE IF EXISTS embedding_models CASCADE;
DROP EXTENSION IF EXISTS vector;

COMMIT;