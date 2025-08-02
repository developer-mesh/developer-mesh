BEGIN;

SET search_path TO mcp, public;

-- Add multi-agent support to existing embeddings table
ALTER TABLE embeddings 
    ADD COLUMN IF NOT EXISTS agent_id VARCHAR(255),
    ADD COLUMN IF NOT EXISTS task_type VARCHAR(50),
    ADD COLUMN IF NOT EXISTS normalized_embedding vector(1536),  -- Standard dimension for cross-model search
    ADD COLUMN IF NOT EXISTS cost_usd DECIMAL(10, 6),
    ADD COLUMN IF NOT EXISTS generation_time_ms INTEGER;

-- Create indexes for multi-agent search
CREATE INDEX IF NOT EXISTS idx_embeddings_agent_id ON embeddings(agent_id);
CREATE INDEX IF NOT EXISTS idx_embeddings_agent_model ON embeddings(agent_id, model_provider, model_name);
CREATE INDEX IF NOT EXISTS idx_embeddings_task_type ON embeddings(task_type);

-- Create index for normalized embeddings (using IVFFlat for scale)
CREATE INDEX IF NOT EXISTS idx_embeddings_normalized_ivfflat 
    ON embeddings USING ivfflat (normalized_embedding vector_cosine_ops) 
    WITH (lists = 100)
    WHERE normalized_embedding IS NOT NULL;

-- Agent configuration table with versioning
CREATE TABLE IF NOT EXISTS agent_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id VARCHAR(255) NOT NULL,
    version INTEGER NOT NULL DEFAULT 1,
    
    -- Configuration
    embedding_strategy VARCHAR(50) NOT NULL CHECK (embedding_strategy IN ('balanced', 'quality', 'speed', 'cost')),
    model_preferences JSONB NOT NULL DEFAULT '[]',
    constraints JSONB NOT NULL DEFAULT '{}',
    fallback_behavior JSONB NOT NULL DEFAULT '{}',
    
    -- Metadata
    metadata JSONB NOT NULL DEFAULT '{}',
    is_active BOOLEAN DEFAULT true,
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by VARCHAR(255),
    
    -- Constraints
    CONSTRAINT unique_agent_version UNIQUE(agent_id, version),
    CONSTRAINT valid_model_preferences CHECK (jsonb_typeof(model_preferences) = 'array'),
    CONSTRAINT valid_constraints CHECK (jsonb_typeof(constraints) = 'object'),
    CONSTRAINT valid_fallback CHECK (jsonb_typeof(fallback_behavior) = 'object')
);

-- Index for getting latest active config
CREATE INDEX idx_agent_configs_active ON agent_configs(agent_id, version DESC) 
    WHERE is_active = true;

-- Real-time metrics table for cost tracking and performance monitoring
CREATE TABLE IF NOT EXISTS embedding_metrics (
    id UUID DEFAULT uuid_generate_v4(),
    agent_id VARCHAR(255) NOT NULL,
    
    -- Model information
    model_provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_dimensions INTEGER NOT NULL,
    
    -- Request data
    request_id UUID,
    token_count INTEGER NOT NULL,
    
    -- Performance metrics
    total_latency_ms INTEGER NOT NULL,
    provider_latency_ms INTEGER,
    normalization_latency_ms INTEGER,
    
    -- Cost tracking
    cost_usd DECIMAL(10, 6) NOT NULL,
    
    -- Status and error tracking
    status VARCHAR(20) NOT NULL CHECK (status IN ('success', 'failure', 'timeout', 'fallback')),
    error_message TEXT,
    
    -- Circuit breaker data
    retry_count INTEGER DEFAULT 0,
    final_provider VARCHAR(50), -- If different from requested due to failover
    
    -- Tenant isolation
    tenant_id UUID NOT NULL,
    
    -- Timestamp
    timestamp TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Composite primary key including partition column
    PRIMARY KEY (id, timestamp)
) PARTITION BY RANGE (timestamp);

-- Create monthly partitions for metrics (example for first few months of 2025)
CREATE TABLE embedding_metrics_2025_01 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
CREATE TABLE embedding_metrics_2025_02 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');
CREATE TABLE embedding_metrics_2025_03 PARTITION OF embedding_metrics
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Indexes for metrics queries
CREATE INDEX idx_embedding_metrics_agent_timestamp ON embedding_metrics(agent_id, timestamp DESC);
CREATE INDEX idx_embedding_metrics_provider ON embedding_metrics(model_provider, timestamp DESC);
CREATE INDEX idx_embedding_metrics_status ON embedding_metrics(status, timestamp DESC);

-- Projection matrices for dimension adaptation
CREATE TABLE IF NOT EXISTS projection_matrices (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    from_dimensions INTEGER NOT NULL,
    to_dimensions INTEGER NOT NULL,
    from_model VARCHAR(100),
    to_model VARCHAR(100),
    
    -- Matrix stored as 2D array in JSONB
    matrix JSONB NOT NULL,
    
    -- Quality metrics
    training_samples INTEGER,
    accuracy_score FLOAT,
    validation_loss FLOAT,
    
    -- Metadata
    training_method VARCHAR(50), -- 'pca', 'linear', 'neural'
    training_params JSONB DEFAULT '{}',
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT unique_projection UNIQUE(from_dimensions, to_dimensions, from_model, to_model),
    CONSTRAINT valid_dimensions_proj CHECK (from_dimensions > 0 AND to_dimensions > 0),
    CONSTRAINT valid_matrix CHECK (jsonb_typeof(matrix) = 'array'),
    CONSTRAINT valid_accuracy CHECK (accuracy_score >= 0 AND accuracy_score <= 1)
);

-- Index for fast projection lookup
CREATE INDEX idx_projection_lookup ON projection_matrices(from_dimensions, to_dimensions);

-- Materialized view for cost analytics
CREATE MATERIALIZED VIEW IF NOT EXISTS agent_cost_analytics AS
SELECT 
    agent_id,
    DATE_TRUNC('day', timestamp) as day,
    model_provider,
    model_name,
    COUNT(*) as request_count,
    SUM(token_count) as total_tokens,
    SUM(cost_usd) as total_cost,
    AVG(total_latency_ms) as avg_latency,
    SUM(CASE WHEN status = 'failure' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as error_rate,
    SUM(CASE WHEN status = 'fallback' THEN 1 ELSE 0 END)::FLOAT / COUNT(*) as fallback_rate
FROM embedding_metrics
WHERE timestamp >= CURRENT_DATE - INTERVAL '30 days'
GROUP BY agent_id, day, model_provider, model_name;

-- Index for materialized view
CREATE UNIQUE INDEX idx_agent_cost_analytics_unique 
    ON agent_cost_analytics(agent_id, day, model_provider, model_name);

-- Function to refresh cost analytics
CREATE OR REPLACE FUNCTION refresh_cost_analytics()
RETURNS void AS $$
BEGIN
    REFRESH MATERIALIZED VIEW CONCURRENTLY agent_cost_analytics;
END;
$$ LANGUAGE plpgsql;

-- Cache table for frequently accessed embeddings
CREATE TABLE IF NOT EXISTS embedding_cache (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    cache_key VARCHAR(255) NOT NULL UNIQUE,
    embedding_id UUID REFERENCES embeddings(id) ON DELETE CASCADE,
    
    -- Cache metadata
    hit_count INTEGER DEFAULT 0,
    last_accessed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Performance
    avg_retrieval_time_ms INTEGER,
    
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Index for cache cleanup
CREATE INDEX idx_embedding_cache_expires ON embedding_cache(expires_at) 
    WHERE expires_at IS NOT NULL;

-- Update trigger for agent_configs
CREATE TRIGGER update_agent_configs_updated_at BEFORE UPDATE
    ON agent_configs FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Comments for documentation
COMMENT ON TABLE agent_configs IS 'Stores agent-specific embedding configuration with versioning';
COMMENT ON TABLE embedding_metrics IS 'Real-time metrics for cost tracking and performance monitoring';
COMMENT ON TABLE projection_matrices IS 'Learned transformations for cross-dimensional embedding compatibility';
COMMENT ON TABLE embedding_cache IS 'Cache layer for frequently accessed embeddings';
COMMENT ON COLUMN embeddings.agent_id IS 'Identifier of the AI agent that created this embedding';
COMMENT ON COLUMN embeddings.normalized_embedding IS 'Standardized 1536-dimensional embedding for cross-model search';
COMMENT ON COLUMN embeddings.task_type IS 'Type of task: code_analysis, general_qa, multilingual, research';

COMMIT;