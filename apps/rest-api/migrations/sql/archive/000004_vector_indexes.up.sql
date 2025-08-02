BEGIN;

SET search_path TO mcp, public;

-- Create indexes for vector search
-- Note: IVFFlat indexes can't handle >2000 dimensions, so we use btree indexes
-- for partial filtering and rely on vector operators for similarity search

-- Create btree indexes for filtering by model and dimensions
-- These help narrow down the search space before vector operations

-- Combined index for model filtering
CREATE INDEX IF NOT EXISTS idx_embeddings_model_dims ON embeddings(model_name, model_dimensions);
CREATE INDEX IF NOT EXISTS idx_embeddings_provider_dims ON embeddings(model_provider, model_dimensions);

-- Tenant and context filtering
CREATE INDEX IF NOT EXISTS idx_embeddings_tenant_model ON embeddings(tenant_id, model_name);
CREATE INDEX IF NOT EXISTS idx_embeddings_context_model ON embeddings(context_id, model_name) WHERE context_id IS NOT NULL;

-- Note: IVFFlat indexes cannot be created on vector(4096) columns due to 2000 dimension limit
-- For production, consider creating separate tables or columns for each dimension size
-- or using HNSW indexes when pgvector is upgraded to support them

-- Function for similarity search with proper dimension handling
CREATE OR REPLACE FUNCTION search_embeddings(
    p_query_embedding vector,
    p_model_name TEXT,
    p_tenant_id UUID,
    p_context_id UUID DEFAULT NULL,
    p_limit INTEGER DEFAULT 10,
    p_threshold FLOAT DEFAULT 0.0,
    p_metadata_filter JSONB DEFAULT NULL
) RETURNS TABLE (
    id UUID,
    context_id UUID,
    content TEXT,
    similarity FLOAT,
    metadata JSONB,
    model_provider VARCHAR(50)
) AS $$
DECLARE
    v_dimensions INTEGER;
    v_provider VARCHAR(50);
BEGIN
    -- Get dimensions and provider for the model
    SELECT dimensions, provider 
    INTO v_dimensions, v_provider
    FROM embedding_models
    WHERE model_name = p_model_name
    AND is_active = true
    LIMIT 1;
    
    IF v_dimensions IS NULL THEN
        RAISE EXCEPTION 'Model % not found or inactive', p_model_name;
    END IF;
    
    -- Dynamic query with proper casting
    RETURN QUERY EXECUTE format(
        'SELECT 
            e.id,
            e.context_id,
            e.content,
            1 - (e.embedding::vector(%1$s) <=> $1::vector(%1$s)) AS similarity,
            e.metadata,
            e.model_provider
        FROM embeddings e
        WHERE e.tenant_id = $2
            AND e.model_name = $3
            AND e.model_dimensions = %1$s
            AND ($4::UUID IS NULL OR e.context_id = $4)
            AND ($7::JSONB IS NULL OR e.metadata @> $7)
            AND 1 - (e.embedding::vector(%1$s) <=> $1::vector(%1$s)) >= $5
        ORDER BY e.embedding::vector(%1$s) <=> $1::vector(%1$s)
        LIMIT $6',
        v_dimensions
    ) USING p_query_embedding, p_tenant_id, p_model_name, p_context_id, p_threshold, p_limit, p_metadata_filter;
END;
$$ LANGUAGE plpgsql STABLE PARALLEL SAFE;

-- Function to insert embeddings with automatic padding
CREATE OR REPLACE FUNCTION insert_embedding(
    p_context_id UUID,
    p_content TEXT,
    p_embedding FLOAT[],
    p_model_name TEXT,
    p_tenant_id UUID,
    p_metadata JSONB DEFAULT '{}',
    p_content_index INTEGER DEFAULT 0,
    p_chunk_index INTEGER DEFAULT 0,
    p_configured_dimensions INTEGER DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_id UUID;
    v_model_id UUID;
    v_model_provider VARCHAR(50);
    v_dimensions INTEGER;
    v_supports_reduction BOOLEAN;
    v_min_dimensions INTEGER;
    v_padded_embedding vector(4096);
    v_actual_dimensions INTEGER;
BEGIN
    -- Get model info
    SELECT id, provider, dimensions, supports_dimensionality_reduction, min_dimensions
    INTO v_model_id, v_model_provider, v_dimensions, v_supports_reduction, v_min_dimensions
    FROM embedding_models
    WHERE model_name = p_model_name
    AND is_active = true
    LIMIT 1;
    
    IF v_model_id IS NULL THEN
        RAISE EXCEPTION 'Model % not found or inactive', p_model_name;
    END IF;
    
    -- Determine actual dimensions
    v_actual_dimensions := COALESCE(p_configured_dimensions, v_dimensions);
    
    -- Validate configured dimensions if provided
    IF p_configured_dimensions IS NOT NULL THEN
        IF NOT v_supports_reduction THEN
            RAISE EXCEPTION 'Model % does not support dimension reduction', p_model_name;
        END IF;
        IF p_configured_dimensions < v_min_dimensions OR p_configured_dimensions > v_dimensions THEN
            RAISE EXCEPTION 'Configured dimensions % outside valid range [%, %] for model %', 
                p_configured_dimensions, v_min_dimensions, v_dimensions, p_model_name;
        END IF;
    END IF;
    
    -- Validate embedding dimensions
    IF array_length(p_embedding, 1) != v_actual_dimensions THEN
        RAISE EXCEPTION 'Embedding dimensions % do not match expected dimensions %', 
            array_length(p_embedding, 1), v_actual_dimensions;
    END IF;
    
    -- Pad embedding to 4096 dimensions
    v_padded_embedding := array_cat(
        p_embedding, 
        array_fill(0::float, ARRAY[4096 - v_actual_dimensions])
    )::vector(4096);
    
    -- Insert the embedding
    INSERT INTO embeddings (
        context_id, content, embedding,
        model_id, model_provider, model_name, model_dimensions,
        configured_dimensions, tenant_id, metadata, 
        content_index, chunk_index, magnitude
    ) VALUES (
        p_context_id, p_content, v_padded_embedding,
        v_model_id, v_model_provider, p_model_name, v_dimensions,
        p_configured_dimensions, p_tenant_id, p_metadata, 
        p_content_index, p_chunk_index, NULL
    ) RETURNING id INTO v_id;
    
    RETURN v_id;
END;
$$ LANGUAGE plpgsql;

-- Helper function to get available models by provider
CREATE OR REPLACE FUNCTION get_available_models(
    p_provider VARCHAR(50) DEFAULT NULL,
    p_model_type VARCHAR(50) DEFAULT NULL
) RETURNS TABLE (
    provider VARCHAR(50),
    model_name VARCHAR(100),
    model_version VARCHAR(20),
    dimensions INTEGER,
    max_tokens INTEGER,
    model_type VARCHAR(50),
    supports_dimensionality_reduction BOOLEAN,
    min_dimensions INTEGER,
    is_active BOOLEAN
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        em.provider,
        em.model_name,
        em.model_version,
        em.dimensions,
        em.max_tokens,
        em.model_type,
        em.supports_dimensionality_reduction,
        em.min_dimensions,
        em.is_active
    FROM embedding_models em
    WHERE (p_provider IS NULL OR em.provider = p_provider)
        AND (p_model_type IS NULL OR em.model_type = p_model_type)
        AND em.is_active = true
    ORDER BY em.provider, em.model_name;
END;
$$ LANGUAGE plpgsql STABLE;

-- Create statistics for query planning
ANALYZE embeddings;

COMMIT;