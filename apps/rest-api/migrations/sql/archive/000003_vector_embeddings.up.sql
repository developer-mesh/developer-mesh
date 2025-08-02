BEGIN;

SET search_path TO mcp, public;

-- Enable pgvector extension
CREATE EXTENSION IF NOT EXISTS vector;

-- Model registry table (for tracking available models)
CREATE TABLE IF NOT EXISTS embedding_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,     -- 'openai', 'voyage', 'amazon', 'google', 'cohere'
    model_name VARCHAR(100) NOT NULL,  -- 'text-embedding-3-large'
    model_version VARCHAR(20),         -- '3.0.0', 'v2', etc.
    dimensions INTEGER NOT NULL,       -- 768, 1024, 1536, 3072, etc.
    
    -- Model characteristics
    max_tokens INTEGER,
    supports_binary BOOLEAN DEFAULT false,
    supports_dimensionality_reduction BOOLEAN DEFAULT false,
    min_dimensions INTEGER,            -- For models that support dimension reduction
    cost_per_million_tokens DECIMAL(10, 4),
    
    -- Provider-specific fields
    model_id VARCHAR(255),             -- For Bedrock models like 'amazon.titan-embed-text-v2:0'
    model_type VARCHAR(50),            -- 'text', 'multimodal', 'code'
    
    -- Status
    is_active BOOLEAN DEFAULT true,
    deprecated_at TIMESTAMP WITH TIME ZONE,
    sunset_at TIMESTAMP WITH TIME ZONE,
    
    -- Metadata
    capabilities JSONB DEFAULT '{}',
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    CONSTRAINT unique_model_provider UNIQUE(provider, model_name, model_version),
    CONSTRAINT valid_dimensions CHECK (dimensions > 0 AND dimensions <= 4096)
);

-- Insert all currently available models
INSERT INTO embedding_models (provider, model_name, model_version, dimensions, max_tokens, cost_per_million_tokens, model_type, supports_dimensionality_reduction, min_dimensions) VALUES
-- OpenAI models
('openai', 'text-embedding-3-small', 'v3', 1536, 8191, 0.02, 'text', true, 512),
('openai', 'text-embedding-3-large', 'v3', 3072, 8191, 0.13, 'text', true, 256),
('openai', 'text-embedding-ada-002', 'v2', 1536, 8191, 0.10, 'text', false, NULL),

-- Anthropic/Voyage AI models
('voyage', 'voyage-large-2', 'v2', 1024, NULL, 0.12, 'text', true, 256),
('voyage', 'voyage-code-3', 'v3', 1024, NULL, 0.12, 'code', true, 256),
('voyage', 'voyage-2', 'v2', 1024, NULL, 0.10, 'text', true, 256),
('voyage', 'voyage-code-2', 'v2', 1024, NULL, 0.10, 'code', true, 256),

-- Amazon Bedrock models
('amazon', 'titan-embed-text-v2', 'v2', 1024, 8192, 0.02, 'text', true, 256),
('cohere', 'embed-english-v3', 'v3', 1024, NULL, 0.10, 'text', false, NULL),
('cohere', 'embed-multilingual-v3', 'v3', 1024, NULL, 0.10, 'text', false, NULL),

-- Google Vertex AI models
('google', 'gemini-embedding-001', 'v1', 3072, 2048, 0.025, 'text', false, NULL),
('google', 'text-embedding-004', 'v4', 768, 2048, 0.025, 'text', true, NULL),
('google', 'text-multilingual-embedding-002', 'v2', 768, 2048, 0.025, 'text', true, NULL),
('google', 'multimodal-embedding', 'v1', 1408, NULL, 0.025, 'multimodal', false, NULL)
ON CONFLICT (provider, model_name, model_version) DO NOTHING;

-- Update model_id for Bedrock models
UPDATE embedding_models SET model_id = 'amazon.titan-embed-text-v2:0' WHERE provider = 'amazon' AND model_name = 'titan-embed-text-v2';
UPDATE embedding_models SET model_id = 'cohere.embed-english-v3' WHERE provider = 'cohere' AND model_name = 'embed-english-v3';
UPDATE embedding_models SET model_id = 'cohere.embed-multilingual-v3' WHERE provider = 'cohere' AND model_name = 'embed-multilingual-v3';

-- Handle embeddings table - either alter existing or create new
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'mcp' AND table_name = 'embeddings') THEN
        -- Table exists in mcp schema (created by vector.go), alter it to add missing columns
        RAISE NOTICE 'Table mcp.embeddings already exists, adding missing columns';
        
        -- Add missing columns to match migration schema
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS chunk_index INTEGER NOT NULL DEFAULT 0;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS content_hash VARCHAR(64);
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS content_tokens INTEGER;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS model_provider VARCHAR(50);
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS model_name VARCHAR(100);
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS model_dimensions INTEGER;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS configured_dimensions INTEGER;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS processing_time_ms INTEGER;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS embedding_created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS magnitude FLOAT;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS tenant_id UUID;
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS metadata JSONB NOT NULL DEFAULT '{}';
        ALTER TABLE mcp.embeddings ADD COLUMN IF NOT EXISTS updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP;
        
        -- Rename 'text' column to 'content' if it exists
        IF EXISTS (SELECT 1 FROM information_schema.columns 
                  WHERE table_schema = 'mcp' AND table_name = 'embeddings' AND column_name = 'text') THEN
            ALTER TABLE mcp.embeddings RENAME COLUMN text TO content;
        END IF;
        
        -- Update model_name from model_id if needed
        UPDATE mcp.embeddings SET model_name = model_id WHERE model_name IS NULL;
        
        -- Set search path to work with mcp schema
        SET search_path TO mcp, public;
    ELSE
        -- Create new table in public schema
        CREATE TABLE embeddings (
            id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
            
            -- Content relationship
            context_id UUID NOT NULL REFERENCES contexts(id) ON DELETE CASCADE,
            content_index INTEGER NOT NULL DEFAULT 0,
            chunk_index INTEGER NOT NULL DEFAULT 0,
    
    -- Content
    content TEXT NOT NULL,
    content_hash VARCHAR(64) GENERATED ALWAYS AS (encode(sha256(content::bytea), 'hex')) STORED,
    content_tokens INTEGER,
    
    -- Model information (denormalized for performance)
    model_id UUID NOT NULL REFERENCES embedding_models(id),
    model_provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_dimensions INTEGER NOT NULL,
    configured_dimensions INTEGER,     -- Actual dimensions if reduced from default
    
    -- The embedding vector (max size 4096 for future models)
    embedding vector(4096) NOT NULL,
    
    -- Processing metadata
    processing_time_ms INTEGER,
    embedding_created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Quality metrics (calculated at insert time)
    magnitude FLOAT,
    
    -- Tenant isolation
    tenant_id UUID NOT NULL,
    
    -- Flexible metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT valid_dimensions CHECK (model_dimensions > 0 AND model_dimensions <= 4096),
    CONSTRAINT valid_content CHECK (length(content) > 0),
            CONSTRAINT valid_indices CHECK (content_index >= 0 AND chunk_index >= 0)
        );
    END IF;
END $$;

-- Create compound indexes for common access patterns (only if we created the table)
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'mcp' AND table_name = 'embeddings') THEN
        -- Only create indexes if we created the table in public schema
        CREATE INDEX IF NOT EXISTS idx_embeddings_context_content ON embeddings(context_id, content_index, chunk_index);
        CREATE INDEX IF NOT EXISTS idx_embeddings_tenant_model ON embeddings(tenant_id, model_name);
        CREATE INDEX IF NOT EXISTS idx_embeddings_content_hash ON embeddings(content_hash);
        CREATE INDEX IF NOT EXISTS idx_embeddings_created_at ON embeddings(embedding_created_at DESC);
        CREATE INDEX IF NOT EXISTS idx_embeddings_provider ON embeddings(model_provider, tenant_id);
        
        -- Trigger for public.embeddings
        DROP TRIGGER IF EXISTS update_embeddings_updated_at ON embeddings;
        CREATE TRIGGER update_embeddings_updated_at BEFORE UPDATE
        ON embeddings FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
    END IF;
END $$;

-- Embedding search history for analytics
CREATE TABLE IF NOT EXISTS embedding_searches (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    model_provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    
    -- Search metadata
    search_type VARCHAR(50) NOT NULL, -- 'similarity', 'hybrid', 'filtered'
    result_count INTEGER NOT NULL,
    threshold FLOAT,
    filters JSONB,
    
    -- Performance metrics
    search_time_ms INTEGER NOT NULL,
    total_candidates INTEGER,
    
    -- Timestamp
    searched_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Comments for documentation
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.tables WHERE table_schema = 'mcp' AND table_name = 'embeddings') THEN
        -- Comment on existing mcp.embeddings table (different schema from vector.go)
        COMMENT ON TABLE mcp.embeddings IS 'Stores vector embeddings (legacy schema from vector.go)';
    ELSE
        -- Comment on newly created public.embeddings table
        COMMENT ON TABLE embeddings IS 'Stores vector embeddings with full model metadata for multi-provider support';
        COMMENT ON COLUMN embeddings.content_hash IS 'SHA-256 hash for deduplication';
        COMMENT ON COLUMN embeddings.magnitude IS 'L2 norm of the embedding vector for quality checks';
        COMMENT ON COLUMN embeddings.metadata IS 'Flexible storage for model-specific parameters, chunking info, provider-specific data';
        COMMENT ON COLUMN embeddings.configured_dimensions IS 'Actual embedding dimensions if reduced from model default';
    END IF;
END $$;

COMMIT;