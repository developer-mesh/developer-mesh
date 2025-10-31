-- =====================================================================
-- Multi-Tenant Embedding Model Management System
-- =====================================================================
-- This migration creates a comprehensive system for managing embedding
-- models in a multi-tenant environment with per-tenant configuration,
-- cost tracking, and usage quotas.

-- 1. Global Model Catalog (Platform Level)
-- =====================================================================
-- This table contains ALL available models in the platform
CREATE TABLE IF NOT EXISTS mcp.embedding_model_catalog (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    
    -- Model Identification
    provider VARCHAR(50) NOT NULL,
    model_name VARCHAR(100) NOT NULL,
    model_id VARCHAR(255) UNIQUE NOT NULL, -- e.g., "amazon.titan-embed-text-v2:0"
    model_version VARCHAR(50),
    
    -- Technical Specifications
    dimensions INTEGER NOT NULL,
    max_tokens INTEGER,
    supports_binary BOOLEAN DEFAULT false,
    supports_dimensionality_reduction BOOLEAN DEFAULT false,
    min_dimensions INTEGER,
    model_type VARCHAR(50) DEFAULT 'text', -- text, multimodal, code
    
    -- Cost Information (Platform Defaults)
    cost_per_million_tokens DECIMAL(10, 4),
    cost_per_million_chars DECIMAL(10, 4),
    
    -- Platform Configuration
    is_available BOOLEAN DEFAULT true,  -- Platform can disable models
    is_deprecated BOOLEAN DEFAULT false,
    deprecation_date TIMESTAMP WITH TIME ZONE,
    minimum_tier VARCHAR(50), -- 'free', 'starter', 'pro', 'enterprise'
    
    -- Provider Requirements
    requires_api_key BOOLEAN DEFAULT true,
    provider_config JSONB DEFAULT '{}', -- Provider-specific config
    
    -- Metadata
    capabilities JSONB DEFAULT '{}',
    performance_metrics JSONB DEFAULT '{}', -- latency, throughput stats
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(provider, model_name)
);

-- 2. Tenant Model Configurations (Tenant Level)
-- =====================================================================
-- Controls which models each tenant can access and their preferences
CREATE TABLE IF NOT EXISTS mcp.tenant_embedding_models (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,  -- References tenant_id from organization_tenants
    model_id UUID NOT NULL REFERENCES mcp.embedding_model_catalog(id),
    
    -- Tenant-Specific Configuration
    is_enabled BOOLEAN DEFAULT true,
    is_default BOOLEAN DEFAULT false, -- Default model for this tenant
    
    -- Cost Overrides (for custom pricing agreements)
    custom_cost_per_million_tokens DECIMAL(10, 4),
    custom_cost_per_million_chars DECIMAL(10, 4),
    
    -- Usage Limits
    monthly_token_limit BIGINT,
    daily_token_limit BIGINT,
    monthly_request_limit INTEGER,
    
    -- Priority and Routing
    priority INTEGER DEFAULT 100, -- Higher = preferred
    fallback_model_id UUID REFERENCES mcp.embedding_model_catalog(id),
    
    -- Agent-Level Preferences
    agent_preferences JSONB DEFAULT '{}', -- {"agent_type": "ide", "models": [...]}
    
    -- Metadata
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    UNIQUE(tenant_id, model_id)
);

-- 3. Agent Model Preferences (Agent Level)
-- =====================================================================
-- Fine-grained control per agent within a tenant
CREATE TABLE IF NOT EXISTS mcp.agent_embedding_preferences (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,  -- References tenant_id from organization_tenants
    agent_id UUID NOT NULL REFERENCES mcp.agents(id) ON DELETE CASCADE,
    
    -- Model Selection Strategy
    selection_strategy VARCHAR(50) DEFAULT 'tenant_default', -- tenant_default, fixed, cost_optimized, performance_optimized
    
    -- Preferred Models (in order)
    primary_model_id UUID REFERENCES mcp.embedding_model_catalog(id),
    secondary_model_id UUID REFERENCES mcp.embedding_model_catalog(id),
    tertiary_model_id UUID REFERENCES mcp.embedding_model_catalog(id),
    
    -- Task-Specific Models
    task_models JSONB DEFAULT '{}', -- {"code_analysis": "model_id", "documentation": "model_id"}
    
    -- Cost Constraints
    max_cost_per_request DECIMAL(10, 6),
    monthly_budget DECIMAL(10, 2),
    
    -- Performance Requirements
    max_latency_ms INTEGER,
    required_dimensions INTEGER,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    UNIQUE(tenant_id, agent_id)
);

-- 4. Model Usage Tracking (For Billing & Analytics)
-- =====================================================================
CREATE TABLE IF NOT EXISTS mcp.embedding_usage_tracking (
    id UUID DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,  -- References tenant_id from organization_tenants
    agent_id UUID REFERENCES mcp.agents(id),
    model_id UUID NOT NULL REFERENCES mcp.embedding_model_catalog(id),
    
    -- Usage Metrics
    tokens_used INTEGER NOT NULL,
    characters_processed INTEGER,
    embeddings_generated INTEGER DEFAULT 1,
    
    -- Cost Tracking
    actual_cost DECIMAL(10, 6),
    billed_cost DECIMAL(10, 6), -- May differ due to agreements
    
    -- Performance Metrics
    latency_ms INTEGER,
    provider_latency_ms INTEGER,
    
    -- Context
    request_id UUID,
    task_type VARCHAR(50),
    
    -- Timestamp
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    PRIMARY KEY (id, created_at)
) PARTITION BY RANGE (created_at);

-- Create monthly partitions for usage tracking
CREATE TABLE mcp.embedding_usage_tracking_2025_01 PARTITION OF mcp.embedding_usage_tracking
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');
    
CREATE TABLE mcp.embedding_usage_tracking_2025_02 PARTITION OF mcp.embedding_usage_tracking
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

-- 5. Model Discovery Registry (For Auto-Discovery)
-- =====================================================================
CREATE TABLE IF NOT EXISTS mcp.model_discovery_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider VARCHAR(50) NOT NULL,
    discovery_endpoint VARCHAR(500),
    discovery_type VARCHAR(50), -- 'api', 'static', 'dynamic'
    
    -- Authentication
    auth_type VARCHAR(50), -- 'api_key', 'oauth', 'iam'
    auth_config JSONB DEFAULT '{}', -- Encrypted credentials
    
    -- Discovery Configuration
    check_interval_hours INTEGER DEFAULT 24,
    last_checked_at TIMESTAMP WITH TIME ZONE,
    last_successful_check TIMESTAMP WITH TIME ZONE,
    
    -- Auto-Registration Rules
    auto_register BOOLEAN DEFAULT false,
    registration_rules JSONB DEFAULT '{}', -- {"min_dimensions": 512, "max_cost": 0.5}
    
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- =====================================================================
-- FUNCTIONS
-- =====================================================================

-- Function to get the best model for a tenant/agent
CREATE OR REPLACE FUNCTION mcp.get_embedding_model_for_request(
    p_tenant_id UUID,
    p_agent_id UUID DEFAULT NULL,
    p_task_type VARCHAR DEFAULT NULL,
    p_requested_model VARCHAR DEFAULT NULL
) RETURNS TABLE (
    model_id UUID,
    model_identifier VARCHAR,
    provider VARCHAR,
    dimensions INTEGER,
    cost_per_million_tokens DECIMAL
) AS $$
BEGIN
    -- If specific model requested and tenant has access, use it
    IF p_requested_model IS NOT NULL THEN
        RETURN QUERY
        SELECT 
            c.id,
            c.model_id,
            c.provider,
            c.dimensions,
            COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
        FROM mcp.embedding_model_catalog c
        JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id
        WHERE tm.tenant_id = p_tenant_id 
            AND tm.is_enabled = true
            AND c.model_id = p_requested_model
            AND c.is_available = true
        LIMIT 1;
        
        IF FOUND THEN RETURN; END IF;
    END IF;
    
    -- Check agent preferences
    IF p_agent_id IS NOT NULL THEN
        -- Check task-specific model
        IF p_task_type IS NOT NULL THEN
            RETURN QUERY
            SELECT 
                c.id,
                c.model_id,
                c.provider,
                c.dimensions,
                COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
            FROM mcp.agent_embedding_preferences aep
            JOIN mcp.embedding_model_catalog c ON c.id::text = (aep.task_models->>(p_task_type))::text
            JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id AND tm.tenant_id = p_tenant_id
            WHERE aep.agent_id = p_agent_id
                AND aep.tenant_id = p_tenant_id
                AND tm.is_enabled = true
                AND c.is_available = true
            LIMIT 1;
            
            IF FOUND THEN RETURN; END IF;
        END IF;
        
        -- Check agent's primary model
        RETURN QUERY
        SELECT 
            c.id,
            c.model_id,
            c.provider,
            c.dimensions,
            COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
        FROM mcp.agent_embedding_preferences aep
        JOIN mcp.embedding_model_catalog c ON c.id = aep.primary_model_id
        JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id AND tm.tenant_id = p_tenant_id
        WHERE aep.agent_id = p_agent_id
            AND aep.tenant_id = p_tenant_id
            AND tm.is_enabled = true
            AND c.is_available = true
        LIMIT 1;
        
        IF FOUND THEN RETURN; END IF;
    END IF;
    
    -- Fall back to tenant's default model
    RETURN QUERY
    SELECT 
        c.id,
        c.model_id,
        c.provider,
        c.dimensions,
        COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
    FROM mcp.embedding_model_catalog c
    JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id
    WHERE tm.tenant_id = p_tenant_id
        AND tm.is_enabled = true
        AND tm.is_default = true
        AND c.is_available = true
    ORDER BY tm.priority DESC
    LIMIT 1;
    
    -- Final fallback: any enabled model for tenant
    IF NOT FOUND THEN
        RETURN QUERY
        SELECT 
            c.id,
            c.model_id,
            c.provider,
            c.dimensions,
            COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
        FROM mcp.embedding_model_catalog c
        JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id
        WHERE tm.tenant_id = p_tenant_id
            AND tm.is_enabled = true
            AND c.is_available = true
        ORDER BY tm.priority DESC, c.cost_per_million_tokens ASC
        LIMIT 1;
    END IF;
END;
$$ LANGUAGE plpgsql;

-- Function to track usage
CREATE OR REPLACE FUNCTION mcp.track_embedding_usage(
    p_tenant_id UUID,
    p_agent_id UUID,
    p_model_id UUID,
    p_tokens INTEGER,
    p_characters INTEGER,
    p_latency_ms INTEGER,
    p_task_type VARCHAR DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_usage_id UUID;
    v_cost DECIMAL(10, 6);
    v_model_cost DECIMAL(10, 4);
BEGIN
    -- Get the cost for this model/tenant
    SELECT COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens)
    INTO v_model_cost
    FROM mcp.embedding_model_catalog c
    LEFT JOIN mcp.tenant_embedding_models tm ON tm.model_id = c.id AND tm.tenant_id = p_tenant_id
    WHERE c.id = p_model_id;
    
    -- Calculate actual cost
    v_cost := (p_tokens::DECIMAL / 1000000) * v_model_cost;
    
    -- Insert usage record
    INSERT INTO mcp.embedding_usage_tracking (
        tenant_id, agent_id, model_id, tokens_used, 
        characters_processed, actual_cost, billed_cost,
        latency_ms, task_type
    ) VALUES (
        p_tenant_id, p_agent_id, p_model_id, p_tokens,
        p_characters, v_cost, v_cost,
        p_latency_ms, p_task_type
    ) RETURNING id INTO v_usage_id;
    
    RETURN v_usage_id;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- TRIGGERS
-- =====================================================================

-- Auto-update updated_at timestamps
CREATE OR REPLACE FUNCTION mcp.update_embedding_model_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_catalog_timestamp
    BEFORE UPDATE ON mcp.embedding_model_catalog
    FOR EACH ROW EXECUTE FUNCTION mcp.update_embedding_model_timestamp();

CREATE TRIGGER update_tenant_models_timestamp
    BEFORE UPDATE ON mcp.tenant_embedding_models
    FOR EACH ROW EXECUTE FUNCTION mcp.update_embedding_model_timestamp();

-- =====================================================================
-- INDEXES
-- =====================================================================

CREATE INDEX idx_tenant_models_tenant ON mcp.tenant_embedding_models(tenant_id);
CREATE INDEX idx_tenant_models_enabled ON mcp.tenant_embedding_models(tenant_id, is_enabled);
CREATE INDEX idx_agent_preferences_agent ON mcp.agent_embedding_preferences(agent_id);
CREATE INDEX idx_usage_tracking_tenant_date ON mcp.embedding_usage_tracking(tenant_id, created_at DESC);
CREATE INDEX idx_usage_tracking_agent ON mcp.embedding_usage_tracking(agent_id, created_at DESC);
CREATE INDEX idx_catalog_available ON mcp.embedding_model_catalog(is_available, is_deprecated);

-- =====================================================================
-- ROW LEVEL SECURITY
-- =====================================================================

ALTER TABLE mcp.tenant_embedding_models ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.agent_embedding_preferences ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.embedding_usage_tracking ENABLE ROW LEVEL SECURITY;

-- Tenants can only see their own configurations
-- Note: current_tenant_id() would need to be implemented based on your auth system
-- For now, we'll create placeholder policies that can be updated
CREATE POLICY tenant_models_isolation ON mcp.tenant_embedding_models
    FOR ALL USING (true);  -- TODO: Update with proper tenant isolation

CREATE POLICY agent_preferences_isolation ON mcp.agent_embedding_preferences
    FOR ALL USING (true);  -- TODO: Update with proper tenant isolation

CREATE POLICY usage_tracking_isolation ON mcp.embedding_usage_tracking
    FOR ALL USING (true);  -- TODO: Update with proper tenant isolation

-- =====================================================================
-- INITIAL DATA - Common Embedding Models
-- =====================================================================

INSERT INTO mcp.embedding_model_catalog (
    provider, model_name, model_id, dimensions, max_tokens, 
    cost_per_million_tokens, model_type, is_available
) VALUES 
    -- AWS Bedrock
    ('bedrock', 'Titan Embed Text v1', 'amazon.titan-embed-text-v1', 1536, 8192, 0.10, 'text', true),
    ('bedrock', 'Titan Embed Text v2', 'amazon.titan-embed-text-v2:0', 1024, 8192, 0.02, 'text', true),
    ('bedrock', 'Titan Embed Multi-Modal', 'amazon.titan-embed-image-v1', 1024, 128, 0.08, 'multimodal', true),
    
    -- OpenAI
    ('openai', 'Text Embedding 3 Small', 'text-embedding-3-small', 1536, 8191, 0.02, 'text', true),
    ('openai', 'Text Embedding 3 Large', 'text-embedding-3-large', 3072, 8191, 0.13, 'text', true),
    ('openai', 'Ada v2', 'text-embedding-ada-002', 1536, 8191, 0.10, 'text', true),
    
    -- Google Vertex AI
    ('google', 'Gecko 003', 'textembedding-gecko@003', 768, 3072, 0.025, 'text', true),
    ('google', 'Gecko Multilingual', 'textembedding-gecko-multilingual@001', 768, 3072, 0.025, 'text', true),
    
    -- Anthropic (via Bedrock)
    ('bedrock', 'Claude 3 Embeddings', 'anthropic.claude-3-embeddings', 1024, 8192, 0.08, 'text', false),
    
    -- Cohere
    ('cohere', 'Embed v3 English', 'embed-english-v3.0', 1024, 512, 0.10, 'text', true),
    ('cohere', 'Embed v3 Multilingual', 'embed-multilingual-v3.0', 1024, 512, 0.10, 'text', true)
ON CONFLICT (provider, model_name) DO UPDATE SET
    dimensions = EXCLUDED.dimensions,
    cost_per_million_tokens = EXCLUDED.cost_per_million_tokens,
    is_available = EXCLUDED.is_available;

-- =====================================================================
-- HELPER VIEW - Current Tenant Models
-- =====================================================================

CREATE OR REPLACE VIEW mcp.v_tenant_available_models AS
SELECT 
    tm.tenant_id,
    c.id as model_id,
    c.provider,
    c.model_name,
    c.model_id as model_identifier,
    c.dimensions,
    COALESCE(tm.custom_cost_per_million_tokens, c.cost_per_million_tokens) as cost_per_million_tokens,
    tm.is_enabled,
    tm.is_default,
    tm.priority,
    tm.monthly_token_limit,
    tm.daily_token_limit
FROM mcp.tenant_embedding_models tm
JOIN mcp.embedding_model_catalog c ON c.id = tm.model_id
WHERE c.is_available = true AND c.is_deprecated = false;

-- Grant permissions (commented out - role may not exist)
-- GRANT SELECT ON mcp.v_tenant_available_models TO authenticated;