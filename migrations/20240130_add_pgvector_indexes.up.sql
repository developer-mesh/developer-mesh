-- Create GiST index for vector similarity search
CREATE INDEX IF NOT EXISTS idx_cache_metadata_embedding_vector 
ON cache_metadata 
USING ivfflat (embedding vector_cosine_ops)
WITH (lists = 100);

-- Create composite index for tenant-scoped searches
CREATE INDEX IF NOT EXISTS idx_cache_metadata_tenant_embedding 
ON cache_metadata (tenant_id, embedding vector_cosine_ops);

-- Create index for access pattern queries
CREATE INDEX IF NOT EXISTS idx_cache_metadata_tenant_accessed 
ON cache_metadata (tenant_id, last_accessed_at DESC);

-- Create index for hit count queries
CREATE INDEX IF NOT EXISTS idx_cache_metadata_tenant_hits 
ON cache_metadata (tenant_id, hit_count DESC);

-- Create partial index for active entries
CREATE INDEX IF NOT EXISTS idx_cache_metadata_active 
ON cache_metadata (tenant_id, cache_key) 
WHERE is_active = true;

-- Create index for query hash lookups
CREATE INDEX IF NOT EXISTS idx_cache_metadata_query_hash
ON cache_metadata (tenant_id, query_hash);

-- Create index for expiration cleanup
CREATE INDEX IF NOT EXISTS idx_cache_metadata_expires
ON cache_metadata (expires_at)
WHERE is_active = true;

-- Analyze table to update statistics
ANALYZE cache_metadata;