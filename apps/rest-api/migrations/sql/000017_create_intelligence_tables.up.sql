-- Create schema for intelligence processing system
-- This implements the auto-embedding pipeline for tool executions

-- Execution tracking table
CREATE TABLE IF NOT EXISTS mcp.execution_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL UNIQUE,
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    tool_id UUID NOT NULL,
    
    -- Execution details
    action VARCHAR(100) NOT NULL,
    request_data JSONB NOT NULL,
    response_data JSONB,
    execution_mode VARCHAR(20) NOT NULL DEFAULT 'sync', -- sync, async, hybrid
    status VARCHAR(50) NOT NULL DEFAULT 'pending', -- pending, processing, completed, failed
    
    -- Intelligence results
    content_type VARCHAR(50),
    intelligence_metadata JSONB,
    context_id UUID,
    embedding_id UUID,
    
    -- Performance metrics
    execution_time_ms INTEGER,
    embedding_time_ms INTEGER,
    total_tokens INTEGER,
    total_cost_usd DECIMAL(10, 6),
    
    -- Error tracking
    error_message TEXT,
    error_code VARCHAR(50),
    retry_count INTEGER DEFAULT 0,
    
    -- Timestamps
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    started_at TIMESTAMP WITH TIME ZONE,
    completed_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for execution_history
CREATE INDEX idx_execution_history_tenant_id ON mcp.execution_history(tenant_id);
CREATE INDEX idx_execution_history_agent_id ON mcp.execution_history(agent_id);
CREATE INDEX idx_execution_history_tool_id ON mcp.execution_history(tool_id);
CREATE INDEX idx_execution_history_status ON mcp.execution_history(status);
CREATE INDEX idx_execution_history_created_at ON mcp.execution_history(created_at DESC);
CREATE INDEX idx_execution_history_execution_id ON mcp.execution_history(execution_id);

-- Semantic relationships table
CREATE TABLE IF NOT EXISTS mcp.semantic_relationships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    source_context_id UUID NOT NULL,
    target_context_id UUID NOT NULL,
    relationship_type VARCHAR(50) NOT NULL, -- similar, references, extends, contradicts, etc.
    confidence_score FLOAT NOT NULL CHECK (confidence_score >= 0 AND confidence_score <= 1),
    metadata JSONB,
    tenant_id UUID NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure unique relationships
    UNIQUE(source_context_id, target_context_id, relationship_type)
);

-- Create indexes for semantic_relationships
CREATE INDEX idx_semantic_relationships_source ON mcp.semantic_relationships(source_context_id);
CREATE INDEX idx_semantic_relationships_target ON mcp.semantic_relationships(target_context_id);
CREATE INDEX idx_semantic_relationships_type ON mcp.semantic_relationships(relationship_type);
CREATE INDEX idx_semantic_relationships_tenant ON mcp.semantic_relationships(tenant_id);

-- Content analysis cache
CREATE TABLE IF NOT EXISTS mcp.content_analysis_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    content_hash VARCHAR(64) NOT NULL,
    content_type VARCHAR(50) NOT NULL,
    
    -- Analysis results
    entities JSONB,              -- Extracted entities
    topics JSONB,                -- Identified topics
    sentiment JSONB,             -- Sentiment analysis
    keywords JSONB,              -- Key terms
    summary TEXT,                -- Generated summary
    language VARCHAR(10),        -- Detected language
    
    -- Classification
    data_classification VARCHAR(20), -- public, internal, confidential, restricted
    pii_detected BOOLEAN DEFAULT FALSE,
    pii_types JSONB,            -- Types of PII found
    
    -- Metadata
    analysis_version VARCHAR(20),
    model_used VARCHAR(100),
    processing_time_ms INTEGER,
    tenant_id UUID NOT NULL,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Ensure unique content per tenant
    UNIQUE(content_hash, tenant_id)
);

-- Create indexes for content_analysis_cache
CREATE INDEX idx_content_analysis_hash ON mcp.content_analysis_cache(content_hash);
CREATE INDEX idx_content_analysis_tenant ON mcp.content_analysis_cache(tenant_id);
CREATE INDEX idx_content_analysis_expires ON mcp.content_analysis_cache(expires_at);

-- Execution checkpoints for recovery
CREATE TABLE IF NOT EXISTS mcp.execution_checkpoints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID NOT NULL,
    stage VARCHAR(50) NOT NULL,
    status VARCHAR(20) NOT NULL, -- started, completed, failed
    
    -- Stage data
    input_data JSONB,
    output_data JSONB,
    error_data JSONB,
    
    -- Timing
    started_at TIMESTAMP WITH TIME ZONE NOT NULL,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    
    -- Compensation
    compensation_required BOOLEAN DEFAULT FALSE,
    compensation_executed BOOLEAN DEFAULT FALSE,
    compensation_data JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for execution_checkpoints
CREATE INDEX idx_execution_checkpoints_execution ON mcp.execution_checkpoints(execution_id);
CREATE INDEX idx_execution_checkpoints_stage ON mcp.execution_checkpoints(stage);
CREATE INDEX idx_execution_checkpoints_status ON mcp.execution_checkpoints(status);

-- Cost tracking per tenant
CREATE TABLE IF NOT EXISTS mcp.intelligence_cost_tracking (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    date DATE NOT NULL,
    
    -- Cost breakdown
    tool_execution_cost DECIMAL(10, 6) DEFAULT 0,
    embedding_cost DECIMAL(10, 6) DEFAULT 0,
    analysis_cost DECIMAL(10, 6) DEFAULT 0,
    storage_cost DECIMAL(10, 6) DEFAULT 0,
    total_cost DECIMAL(10, 6) DEFAULT 0,
    
    -- Usage metrics
    execution_count INTEGER DEFAULT 0,
    embedding_count INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    storage_mb DECIMAL(10, 2) DEFAULT 0,
    
    -- Budget tracking
    daily_budget DECIMAL(10, 2),
    budget_remaining DECIMAL(10, 2),
    budget_exceeded BOOLEAN DEFAULT FALSE,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    
    -- Ensure one record per tenant per day
    UNIQUE(tenant_id, date)
);

-- Create indexes for intelligence_cost_tracking
CREATE INDEX idx_cost_tracking_tenant_date ON mcp.intelligence_cost_tracking(tenant_id, date DESC);
CREATE INDEX idx_cost_tracking_budget_exceeded ON mcp.intelligence_cost_tracking(budget_exceeded);

-- Performance metrics table
CREATE TABLE IF NOT EXISTS mcp.intelligence_metrics (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    metric_type VARCHAR(50) NOT NULL, -- latency, throughput, error_rate, etc.
    metric_name VARCHAR(100) NOT NULL,
    
    -- Metric values
    value FLOAT NOT NULL,
    unit VARCHAR(20),
    
    -- Context
    tenant_id UUID,
    agent_id UUID,
    tool_id UUID,
    execution_id UUID,
    
    -- Labels for filtering
    labels JSONB,
    
    -- Time window
    window_start TIMESTAMP WITH TIME ZONE NOT NULL,
    window_end TIMESTAMP WITH TIME ZONE NOT NULL,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for intelligence_metrics
CREATE INDEX idx_intelligence_metrics_type ON mcp.intelligence_metrics(metric_type);
CREATE INDEX idx_intelligence_metrics_name ON mcp.intelligence_metrics(metric_name);
CREATE INDEX idx_intelligence_metrics_window ON mcp.intelligence_metrics(window_start, window_end);
CREATE INDEX idx_intelligence_metrics_tenant ON mcp.intelligence_metrics(tenant_id);

-- Security audit log
CREATE TABLE IF NOT EXISTS mcp.security_audit_log (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    
    -- Security context
    tenant_id UUID NOT NULL,
    agent_id UUID,
    user_id UUID,
    
    -- Classification results
    data_classification VARCHAR(20),
    pii_detected BOOLEAN DEFAULT FALSE,
    secrets_detected BOOLEAN DEFAULT FALSE,
    
    -- Action taken
    action VARCHAR(50), -- allowed, blocked, redacted, encrypted
    block_reason TEXT,
    
    -- Details
    details JSONB,
    
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for security_audit_log
CREATE INDEX idx_security_audit_event ON mcp.security_audit_log(event_id);
CREATE INDEX idx_security_audit_type ON mcp.security_audit_log(event_type);
CREATE INDEX idx_security_audit_tenant ON mcp.security_audit_log(tenant_id);
CREATE INDEX idx_security_audit_created ON mcp.security_audit_log(created_at DESC);

-- Create function to auto-generate embeddings on tool execution
CREATE OR REPLACE FUNCTION mcp.auto_generate_embedding()
RETURNS TRIGGER AS $$
DECLARE
    v_should_embed BOOLEAN;
    v_content TEXT;
BEGIN
    -- Check if execution completed successfully
    IF NEW.status = 'completed' AND NEW.response_data IS NOT NULL THEN
        -- Determine if content should be embedded
        v_should_embed := COALESCE(
            (NEW.intelligence_metadata->>'should_embed')::BOOLEAN,
            TRUE
        );
        
        IF v_should_embed THEN
            -- Extract content from response
            v_content := COALESCE(
                NEW.response_data->>'content',
                NEW.response_data->>'text',
                NEW.response_data->>'result',
                NEW.response_data::TEXT
            );
            
            -- Queue embedding generation (would typically send to worker)
            INSERT INTO mcp.embedding_queue (
                execution_id,
                content,
                tenant_id,
                agent_id,
                metadata,
                priority
            ) VALUES (
                NEW.execution_id,
                v_content,
                NEW.tenant_id,
                NEW.agent_id,
                jsonb_build_object(
                    'tool_id', NEW.tool_id,
                    'action', NEW.action,
                    'content_type', NEW.content_type
                ),
                CASE 
                    WHEN NEW.content_type IN ('code', 'documentation', 'api_response') THEN 1
                    ELSE 2
                END
            ) ON CONFLICT (execution_id) DO NOTHING;
        END IF;
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for auto-embedding
CREATE TRIGGER trigger_auto_generate_embedding
    AFTER INSERT OR UPDATE OF status ON mcp.execution_history
    FOR EACH ROW
    EXECUTE FUNCTION mcp.auto_generate_embedding();

-- Create embedding queue table for async processing
CREATE TABLE IF NOT EXISTS mcp.embedding_queue (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID UNIQUE,
    content TEXT NOT NULL,
    tenant_id UUID NOT NULL,
    agent_id UUID NOT NULL,
    metadata JSONB,
    priority INTEGER DEFAULT 2, -- 1 = high, 2 = normal, 3 = low
    status VARCHAR(20) DEFAULT 'pending', -- pending, processing, completed, failed
    retry_count INTEGER DEFAULT 0,
    error_message TEXT,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    processed_at TIMESTAMP WITH TIME ZONE
);

-- Create indexes for embedding_queue
CREATE INDEX idx_embedding_queue_status ON mcp.embedding_queue(status);
CREATE INDEX idx_embedding_queue_priority ON mcp.embedding_queue(priority, created_at);
CREATE INDEX idx_embedding_queue_tenant ON mcp.embedding_queue(tenant_id);

-- Create function to clean up old data
CREATE OR REPLACE FUNCTION mcp.cleanup_old_intelligence_data()
RETURNS void AS $$
BEGIN
    -- Delete old execution history (keep 90 days)
    DELETE FROM mcp.execution_history 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '90 days';
    
    -- Delete expired content analysis cache
    DELETE FROM mcp.content_analysis_cache 
    WHERE expires_at IS NOT NULL AND expires_at < CURRENT_TIMESTAMP;
    
    -- Delete old checkpoints (keep 30 days)
    DELETE FROM mcp.execution_checkpoints 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '30 days';
    
    -- Delete old metrics (keep 30 days)
    DELETE FROM mcp.intelligence_metrics 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '30 days';
    
    -- Delete old audit logs (keep 1 year)
    DELETE FROM mcp.security_audit_log 
    WHERE created_at < CURRENT_TIMESTAMP - INTERVAL '1 year';
    
    -- Delete processed embedding queue items (keep 7 days)
    DELETE FROM mcp.embedding_queue 
    WHERE status IN ('completed', 'failed') 
    AND created_at < CURRENT_TIMESTAMP - INTERVAL '7 days';
END;
$$ LANGUAGE plpgsql;

-- Create indexes for common queries
CREATE INDEX IF NOT EXISTS idx_execution_history_recent 
    ON mcp.execution_history(tenant_id, created_at DESC) 
    WHERE status = 'completed';

CREATE INDEX IF NOT EXISTS idx_semantic_relationships_graph 
    ON mcp.semantic_relationships(source_context_id, confidence_score DESC);

CREATE INDEX IF NOT EXISTS idx_content_analysis_recent 
    ON mcp.content_analysis_cache(tenant_id, created_at DESC);

-- Grant permissions
GRANT ALL ON ALL TABLES IN SCHEMA mcp TO devmesh;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA mcp TO devmesh;