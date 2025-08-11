-- =====================================================
-- Cache System for Tool Execution Results
-- =====================================================
-- Provides high-performance caching for tool executions
-- with support for exact and semantic similarity matching

-- Create unified cache entries table
CREATE TABLE IF NOT EXISTS mcp.cache_entries (
    key_hash VARCHAR(64) PRIMARY KEY,
    tenant_id UUID NOT NULL,
    tool_id VARCHAR(255) NOT NULL,
    action VARCHAR(255) NOT NULL,
    parameters_hash VARCHAR(64) NOT NULL,
    response_data JSONB NOT NULL,
    embedding vector(1536),
    metadata JSONB DEFAULT '{}',
    ttl_seconds INTEGER DEFAULT 3600,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    last_accessed_at TIMESTAMPTZ DEFAULT NOW(),
    hit_count INTEGER DEFAULT 0,
    expires_at TIMESTAMPTZ
);

-- Update expires_at based on created_at and ttl_seconds
CREATE OR REPLACE FUNCTION mcp.update_expires_at() RETURNS TRIGGER AS $$
BEGIN
    NEW.expires_at = NEW.created_at + (NEW.ttl_seconds || ' seconds')::INTERVAL;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_cache_expires_at
    BEFORE INSERT OR UPDATE ON mcp.cache_entries
    FOR EACH ROW
    EXECUTE FUNCTION mcp.update_expires_at();

-- Performance indexes for fast lookups
CREATE INDEX idx_cache_tenant_lookup ON mcp.cache_entries(tenant_id, key_hash);
CREATE INDEX idx_cache_expiry ON mcp.cache_entries(expires_at);
CREATE INDEX idx_cache_frequently_used ON mcp.cache_entries(tenant_id, hit_count DESC, last_accessed_at DESC);
CREATE INDEX idx_cache_tool_action ON mcp.cache_entries(tenant_id, tool_id, action);

-- Semantic search index for similarity matching
CREATE INDEX idx_cache_embedding_search ON mcp.cache_entries 
    USING ivfflat (embedding vector_cosine_ops) 
    WHERE embedding IS NOT NULL;

-- Cache statistics for monitoring and optimization
CREATE TABLE IF NOT EXISTS mcp.cache_statistics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    date DATE NOT NULL DEFAULT CURRENT_DATE,
    total_requests INTEGER DEFAULT 0,
    cache_hits INTEGER DEFAULT 0,
    cache_misses INTEGER DEFAULT 0,
    avg_response_time_ms NUMERIC(10,2),
    p95_response_time_ms NUMERIC(10,2),
    p99_response_time_ms NUMERIC(10,2),
    bytes_saved BIGINT DEFAULT 0,
    api_calls_saved INTEGER DEFAULT 0,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    UNIQUE(tenant_id, date)
);

CREATE INDEX idx_cache_stats_lookup ON mcp.cache_statistics(tenant_id, date DESC);

-- Function to get or create cache entry (atomic operation)
CREATE OR REPLACE FUNCTION mcp.get_or_create_cache_entry(
    p_key_hash VARCHAR(64),
    p_tenant_id UUID,
    p_tool_id VARCHAR(255),
    p_action VARCHAR(255),
    p_parameters_hash VARCHAR(64),
    p_response_data JSONB,
    p_embedding vector(1536),
    p_metadata JSONB,
    p_ttl_seconds INTEGER
) RETURNS TABLE(
    response_data JSONB,
    from_cache BOOLEAN,
    hit_count INTEGER,
    created_at TIMESTAMPTZ
) AS $$
DECLARE
    v_existing_entry RECORD;
BEGIN
    -- Try to get existing entry and increment hit count atomically
    UPDATE mcp.cache_entries 
    SET 
        hit_count = hit_count + 1,
        last_accessed_at = NOW()
    WHERE 
        key_hash = p_key_hash 
        AND tenant_id = p_tenant_id
        AND expires_at > NOW()
    RETURNING 
        cache_entries.response_data,
        cache_entries.hit_count,
        cache_entries.created_at
    INTO v_existing_entry;
    
    IF FOUND THEN
        -- Cache hit
        RETURN QUERY SELECT 
            v_existing_entry.response_data,
            true AS from_cache,
            v_existing_entry.hit_count,
            v_existing_entry.created_at;
    ELSE
        -- Cache miss - insert new entry
        INSERT INTO mcp.cache_entries (
            key_hash,
            tenant_id,
            tool_id,
            action,
            parameters_hash,
            response_data,
            embedding,
            metadata,
            ttl_seconds
        ) VALUES (
            p_key_hash,
            p_tenant_id,
            p_tool_id,
            p_action,
            p_parameters_hash,
            p_response_data,
            p_embedding,
            p_metadata,
            COALESCE(p_ttl_seconds, 3600)
        )
        ON CONFLICT (key_hash) DO NOTHING
        RETURNING 
            cache_entries.response_data,
            false AS from_cache,
            cache_entries.hit_count,
            cache_entries.created_at
        INTO v_existing_entry;
        
        IF FOUND THEN
            -- Successfully inserted
            RETURN QUERY SELECT 
                v_existing_entry.response_data,
                false AS from_cache,
                v_existing_entry.hit_count,
                v_existing_entry.created_at;
        ELSE
            -- Another process inserted it, get the existing one
            SELECT 
                ce.response_data,
                ce.hit_count,
                ce.created_at
            INTO v_existing_entry
            FROM mcp.cache_entries ce
            WHERE ce.key_hash = p_key_hash AND ce.tenant_id = p_tenant_id;
            
            RETURN QUERY SELECT 
                v_existing_entry.response_data,
                true AS from_cache,
                v_existing_entry.hit_count,
                v_existing_entry.created_at;
        END IF;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to update cache statistics
CREATE OR REPLACE FUNCTION mcp.update_cache_stats(
    p_tenant_id UUID,
    p_is_hit BOOLEAN,
    p_response_time_ms NUMERIC,
    p_bytes_saved BIGINT DEFAULT 0,
    p_api_calls_saved INTEGER DEFAULT 0
) RETURNS VOID AS $$
BEGIN
    INSERT INTO mcp.cache_statistics (
        tenant_id, 
        date,
        total_requests,
        cache_hits,
        cache_misses,
        avg_response_time_ms,
        bytes_saved,
        api_calls_saved,
        updated_at
    ) VALUES (
        p_tenant_id,
        CURRENT_DATE,
        1,
        CASE WHEN p_is_hit THEN 1 ELSE 0 END,
        CASE WHEN p_is_hit THEN 0 ELSE 1 END,
        p_response_time_ms,
        CASE WHEN p_is_hit THEN p_bytes_saved ELSE 0 END,
        CASE WHEN p_is_hit THEN p_api_calls_saved ELSE 0 END,
        NOW()
    )
    ON CONFLICT (tenant_id, date) DO UPDATE SET
        total_requests = cache_statistics.total_requests + 1,
        cache_hits = cache_statistics.cache_hits + CASE WHEN p_is_hit THEN 1 ELSE 0 END,
        cache_misses = cache_statistics.cache_misses + CASE WHEN p_is_hit THEN 0 ELSE 1 END,
        avg_response_time_ms = (
            (cache_statistics.avg_response_time_ms * cache_statistics.total_requests + p_response_time_ms) / 
            (cache_statistics.total_requests + 1)
        ),
        bytes_saved = cache_statistics.bytes_saved + CASE WHEN p_is_hit THEN p_bytes_saved ELSE 0 END,
        api_calls_saved = cache_statistics.api_calls_saved + CASE WHEN p_is_hit THEN p_api_calls_saved ELSE 0 END,
        updated_at = NOW();
END;
$$ LANGUAGE plpgsql;

-- Function to find similar cached entries using embeddings
CREATE OR REPLACE FUNCTION mcp.find_similar_cache_entries(
    p_tenant_id UUID,
    p_embedding vector(1536),
    p_similarity_threshold FLOAT DEFAULT 0.85,
    p_limit INTEGER DEFAULT 5
) RETURNS TABLE(
    key_hash VARCHAR(64),
    tool_id VARCHAR(255),
    action VARCHAR(255),
    response_data JSONB,
    similarity FLOAT,
    hit_count INTEGER
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        ce.key_hash,
        ce.tool_id,
        ce.action,
        ce.response_data,
        1 - (ce.embedding <=> p_embedding) AS similarity,
        ce.hit_count
    FROM mcp.cache_entries ce
    WHERE 
        ce.tenant_id = p_tenant_id
        AND ce.embedding IS NOT NULL
        AND ce.expires_at > NOW()
        AND 1 - (ce.embedding <=> p_embedding) > p_similarity_threshold
    ORDER BY ce.embedding <=> p_embedding
    LIMIT p_limit;
END;
$$ LANGUAGE plpgsql;

-- Cleanup function for expired entries
CREATE OR REPLACE FUNCTION mcp.cleanup_expired_cache() RETURNS INTEGER AS $$
DECLARE
    v_deleted_count INTEGER;
BEGIN
    DELETE FROM mcp.cache_entries 
    WHERE expires_at < NOW()
    RETURNING 1 INTO v_deleted_count;
    
    RETURN COALESCE(v_deleted_count, 0);
END;
$$ LANGUAGE plpgsql;

-- Create a scheduled job to cleanup expired cache (requires pg_cron or external scheduler)
-- This is a placeholder - actual scheduling depends on your setup
COMMENT ON FUNCTION mcp.cleanup_expired_cache() IS 
'Run this function periodically (e.g., every hour) to clean up expired cache entries. 
Schedule with pg_cron: SELECT cron.schedule(''cleanup-cache'', ''0 * * * *'', ''SELECT mcp.cleanup_expired_cache()'');';

-- Grant permissions (using devmesh user which exists)
GRANT ALL ON mcp.cache_entries TO devmesh;
GRANT ALL ON mcp.cache_statistics TO devmesh;
GRANT EXECUTE ON FUNCTION mcp.get_or_create_cache_entry TO devmesh;
GRANT EXECUTE ON FUNCTION mcp.update_cache_stats TO devmesh;
GRANT EXECUTE ON FUNCTION mcp.find_similar_cache_entries TO devmesh;
GRANT EXECUTE ON FUNCTION mcp.cleanup_expired_cache TO devmesh;

-- Add cache hit rate to cache statistics view
CREATE OR REPLACE VIEW mcp.cache_performance AS
SELECT 
    tenant_id,
    date,
    total_requests,
    cache_hits,
    cache_misses,
    CASE 
        WHEN total_requests > 0 
        THEN ROUND((cache_hits::NUMERIC / total_requests) * 100, 2)
        ELSE 0 
    END AS cache_hit_rate,
    avg_response_time_ms,
    p95_response_time_ms,
    p99_response_time_ms,
    bytes_saved,
    api_calls_saved
FROM mcp.cache_statistics
ORDER BY date DESC;

GRANT SELECT ON mcp.cache_performance TO devmesh;