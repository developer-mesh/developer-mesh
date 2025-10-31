-- Fix ambiguous column reference in cache function
DROP FUNCTION IF EXISTS mcp.get_or_create_cache_entry(VARCHAR, UUID, VARCHAR, VARCHAR, VARCHAR, JSONB, vector(1536), JSONB, INTEGER);

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
    UPDATE mcp.cache_entries ce
    SET 
        hit_count = ce.hit_count + 1,
        last_accessed_at = NOW()
    WHERE 
        ce.key_hash = p_key_hash 
        AND ce.tenant_id = p_tenant_id
        AND ce.expires_at > NOW()
    RETURNING 
        ce.response_data,
        ce.hit_count,
        ce.created_at
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
            mcp.cache_entries.response_data,
            false AS from_cache,
            mcp.cache_entries.hit_count,
            mcp.cache_entries.created_at
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

GRANT EXECUTE ON FUNCTION mcp.get_or_create_cache_entry TO devmesh;