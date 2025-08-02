BEGIN;

-- Drop statistics table
DROP TABLE IF EXISTS embedding_statistics;

-- Drop functions
DROP FUNCTION IF EXISTS update_embedding_statistics(UUID);
DROP FUNCTION IF EXISTS reciprocal_rank_fusion(float[], float[], uuid[], uuid[], integer);
DROP FUNCTION IF EXISTS bm25_score(text[], tsvector, integer, float, integer, float, float);

-- Drop trigger
DROP TRIGGER IF EXISTS embeddings_tsvector_update ON embeddings;

-- Drop trigger function
DROP FUNCTION IF EXISTS update_content_tsvector();

-- Drop indexes
DROP INDEX IF EXISTS idx_embeddings_trigram;
DROP INDEX IF EXISTS idx_embeddings_fts;

-- Remove columns
ALTER TABLE embeddings 
    DROP COLUMN IF EXISTS content_tsvector,
    DROP COLUMN IF EXISTS term_frequencies,
    DROP COLUMN IF EXISTS document_length,
    DROP COLUMN IF EXISTS idf_scores;

COMMIT;