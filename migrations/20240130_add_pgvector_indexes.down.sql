-- Drop pgvector indexes
DROP INDEX IF EXISTS idx_cache_metadata_embedding_vector;
DROP INDEX IF EXISTS idx_cache_metadata_tenant_embedding;
DROP INDEX IF EXISTS idx_cache_metadata_tenant_accessed;
DROP INDEX IF EXISTS idx_cache_metadata_tenant_hits;
DROP INDEX IF EXISTS idx_cache_metadata_active;
DROP INDEX IF EXISTS idx_cache_metadata_query_hash;
DROP INDEX IF EXISTS idx_cache_metadata_expires;