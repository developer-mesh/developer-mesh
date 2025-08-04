-- ==============================================================================
-- Migration: 000002_dynamic_tools
-- Description: Add dynamic tools discovery and execution system
-- Author: DBA Team
-- Date: 2025-08-04
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- VERIFY DEPENDENCIES
-- ==============================================================================

-- Check if required functions exist from previous migrations
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'update_updated_at_column') THEN
        RAISE EXCEPTION 'Required function update_updated_at_column not found. Please ensure migration 000001 completed successfully.';
    END IF;
    
    IF NOT EXISTS (SELECT 1 FROM pg_proc WHERE proname = 'current_tenant_id' AND pronamespace = 'mcp'::regnamespace) THEN
        RAISE EXCEPTION 'Required function current_tenant_id not found. Please ensure migration 000001 completed successfully.';
    END IF;
END;
$$;

-- ==============================================================================
-- DYNAMIC TOOLS CONFIGURATION
-- ==============================================================================

-- Tool configurations table
-- Stores API tool definitions with authentication and discovery settings
CREATE TABLE IF NOT EXISTS mcp.tool_configurations (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tenant_id UUID NOT NULL,
    name VARCHAR(255) NOT NULL,
    type VARCHAR(50) NOT NULL,
    base_url TEXT NOT NULL,
    
    -- Configuration fields
    auth_config JSONB NOT NULL DEFAULT '{}',
    api_spec JSONB,
    discovered_endpoints JSONB NOT NULL DEFAULT '[]',
    health_check_config JSONB NOT NULL DEFAULT '{}',
    
    -- Status fields
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_health_check TIMESTAMP WITH TIME ZONE,
    health_status VARCHAR(20),
    health_message TEXT,
    
    -- Metadata
    description TEXT,
    tags TEXT[],
    metadata JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Constraints
    CONSTRAINT uk_tool_configurations_tenant_name UNIQUE(tenant_id, name),
    CONSTRAINT chk_tool_type CHECK (type IN ('rest', 'graphql', 'grpc', 'webhook', 'custom')),
    CONSTRAINT chk_health_status CHECK (health_status IS NULL OR health_status IN ('healthy', 'degraded', 'unhealthy', 'unknown')),
    CONSTRAINT chk_base_url_format CHECK (base_url ~ '^https?://.*'),
    CONSTRAINT chk_name_format CHECK (name ~ '^[a-zA-Z0-9][a-zA-Z0-9-_]*$')
);

-- Tool discovery sessions table
-- Tracks API discovery operations and their results
CREATE TABLE IF NOT EXISTS mcp.tool_discovery_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID NOT NULL REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    
    -- Session details
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    discovered_endpoints INTEGER NOT NULL DEFAULT 0,
    total_endpoints INTEGER,
    
    -- Discovery configuration
    discovery_config JSONB DEFAULT '{}',
    discovery_method VARCHAR(50),
    
    -- Results and errors
    discovery_results JSONB,
    error_message TEXT,
    error_details JSONB,
    
    -- Timing
    started_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER GENERATED ALWAYS AS (
        CASE 
            WHEN completed_at IS NOT NULL 
            THEN EXTRACT(EPOCH FROM (completed_at - started_at)) * 1000
            ELSE NULL
        END::INTEGER
    ) STORED,
    
    -- Metadata
    initiated_by UUID,
    metadata JSONB DEFAULT '{}',
    
    -- Constraints
    CONSTRAINT chk_discovery_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'cancelled')),
    CONSTRAINT chk_discovery_method CHECK (discovery_method IS NULL OR discovery_method IN ('openapi', 'swagger', 'graphql', 'manual', 'auto')),
    CONSTRAINT chk_endpoints_count CHECK (discovered_endpoints >= 0),
    CONSTRAINT chk_completion_time CHECK (completed_at IS NULL OR completed_at >= started_at)
);

-- Tool discovery patterns table
-- Stores learned patterns for API discovery optimization
CREATE TABLE IF NOT EXISTS mcp.tool_discovery_patterns (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    domain VARCHAR(255) NOT NULL,
    
    -- Pattern details
    successful_paths JSONB NOT NULL DEFAULT '[]',
    failed_paths JSONB DEFAULT '[]',
    auth_method VARCHAR(50),
    auth_location VARCHAR(50),
    api_format VARCHAR(50),
    
    -- Statistics
    success_count INTEGER NOT NULL DEFAULT 0,
    failure_count INTEGER NOT NULL DEFAULT 0,
    avg_discovery_time_ms INTEGER,
    
    -- Learned configurations
    common_headers JSONB DEFAULT '{}',
    rate_limits JSONB DEFAULT '{}',
    
    -- Metadata
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_successful_discovery TIMESTAMP WITH TIME ZONE,
    confidence_score DECIMAL(3,2) DEFAULT 0.00 CHECK (confidence_score >= 0 AND confidence_score <= 1),
    
    -- Constraints
    CONSTRAINT uk_discovery_patterns_domain UNIQUE(domain),
    CONSTRAINT chk_domain_format CHECK (domain ~ '^[a-zA-Z0-9][a-zA-Z0-9.-]*\.[a-zA-Z]{2,}$'),
    CONSTRAINT chk_auth_method CHECK (auth_method IS NULL OR auth_method IN ('none', 'api_key', 'bearer', 'basic', 'oauth2', 'custom')),
    CONSTRAINT chk_auth_location CHECK (auth_location IS NULL OR auth_location IN ('header', 'query', 'body', 'cookie')),
    CONSTRAINT chk_api_format CHECK (api_format IS NULL OR api_format IN ('openapi', 'swagger', 'graphql', 'rest', 'grpc'))
);

-- Tool executions table
-- Logs all tool execution attempts with results
CREATE TABLE IF NOT EXISTS mcp.tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID NOT NULL REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    
    -- Execution details
    action VARCHAR(255) NOT NULL,
    method VARCHAR(10),
    endpoint TEXT,
    
    -- Request/Response data
    input_data JSONB NOT NULL DEFAULT '{}',
    output_data JSONB,
    headers_sent JSONB DEFAULT '{}',
    headers_received JSONB DEFAULT '{}',
    
    -- Status and error handling
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    status_code INTEGER,
    error_message TEXT,
    error_details JSONB,
    retry_count INTEGER DEFAULT 0,
    
    -- Performance metrics
    executed_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    completed_at TIMESTAMP WITH TIME ZONE,
    duration_ms INTEGER,
    request_size_bytes INTEGER,
    response_size_bytes INTEGER,
    
    -- Context
    initiated_by UUID,
    correlation_id UUID,
    parent_execution_id UUID REFERENCES mcp.tool_executions(id),
    
    -- Metadata
    metadata JSONB DEFAULT '{}',
    
    -- Constraints
    CONSTRAINT chk_execution_status CHECK (status IN ('pending', 'running', 'completed', 'failed', 'timeout', 'cancelled')),
    CONSTRAINT chk_http_method CHECK (method IS NULL OR method IN ('GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'HEAD', 'OPTIONS')),
    CONSTRAINT chk_status_code CHECK (status_code IS NULL OR (status_code >= 100 AND status_code < 600)),
    CONSTRAINT chk_duration CHECK (duration_ms IS NULL OR duration_ms >= 0),
    CONSTRAINT chk_retry_count CHECK (retry_count >= 0)
);

-- Tool authentication configurations
-- Separate table for enhanced security and credential rotation
CREATE TABLE IF NOT EXISTS mcp.tool_auth_configs (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID NOT NULL REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    
    -- Authentication details
    auth_type VARCHAR(50) NOT NULL,
    credentials_encrypted BYTEA,
    encryption_key_id UUID,
    
    -- Token management
    access_token_encrypted BYTEA,
    refresh_token_encrypted BYTEA,
    token_expires_at TIMESTAMP WITH TIME ZONE,
    
    -- OAuth specific
    oauth_client_id VARCHAR(255),
    oauth_scope TEXT,
    oauth_redirect_uri TEXT,
    
    -- Rotation and validity
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_rotated_at TIMESTAMP WITH TIME ZONE,
    expires_at TIMESTAMP WITH TIME ZONE,
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    -- Constraints
    CONSTRAINT chk_auth_type CHECK (auth_type IN ('none', 'api_key', 'bearer', 'basic', 'oauth2', 'aws_signature', 'custom'))
);

-- Tool health checks table
-- Tracks health check history for monitoring
CREATE TABLE IF NOT EXISTS mcp.tool_health_checks (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    tool_id UUID NOT NULL REFERENCES mcp.tool_configurations(id) ON DELETE CASCADE,
    
    -- Check details
    check_type VARCHAR(50) NOT NULL DEFAULT 'ping',
    endpoint_tested TEXT,
    
    -- Results
    status VARCHAR(20) NOT NULL,
    response_time_ms INTEGER,
    status_code INTEGER,
    
    -- Diagnostics
    error_message TEXT,
    diagnostics JSONB DEFAULT '{}',
    
    -- Timing
    checked_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    
    -- Constraints
    CONSTRAINT chk_health_check_type CHECK (check_type IN ('ping', 'auth', 'full', 'custom')),
    CONSTRAINT chk_health_check_status CHECK (status IN ('healthy', 'degraded', 'unhealthy', 'timeout'))
);

-- ==============================================================================
-- INDEXES
-- ==============================================================================

-- Primary access patterns
CREATE INDEX idx_tool_configurations_tenant_active 
    ON mcp.tool_configurations(tenant_id, is_active) 
    WHERE is_active = true;

CREATE INDEX idx_tool_configurations_type 
    ON mcp.tool_configurations(type, tenant_id) 
    WHERE is_active = true;

CREATE INDEX idx_tool_configurations_health 
    ON mcp.tool_configurations(health_status, last_health_check) 
    WHERE is_active = true;

-- Discovery optimization
CREATE INDEX idx_tool_discovery_sessions_tool 
    ON mcp.tool_discovery_sessions(tool_id);

CREATE INDEX idx_tool_discovery_sessions_status 
    ON mcp.tool_discovery_sessions(status, started_at DESC) 
    WHERE status IN ('pending', 'running');

CREATE INDEX idx_tool_discovery_patterns_domain 
    ON mcp.tool_discovery_patterns(domain);

CREATE INDEX idx_tool_discovery_patterns_confidence 
    ON mcp.tool_discovery_patterns(confidence_score DESC) 
    WHERE confidence_score > 0.5;

-- Execution performance
CREATE INDEX idx_tool_executions_tool 
    ON mcp.tool_executions(tool_id);

CREATE INDEX idx_tool_executions_tenant_time 
    ON mcp.tool_executions(tenant_id, executed_at DESC);

CREATE INDEX idx_tool_executions_status 
    ON mcp.tool_executions(status, executed_at DESC) 
    WHERE status IN ('pending', 'running');

CREATE INDEX idx_tool_executions_correlation 
    ON mcp.tool_executions(correlation_id) 
    WHERE correlation_id IS NOT NULL;

-- JSONB indexes for common queries
CREATE INDEX idx_tool_configurations_endpoints 
    ON mcp.tool_configurations USING gin(discovered_endpoints jsonb_path_ops);

CREATE INDEX idx_tool_configurations_tags 
    ON mcp.tool_configurations USING gin(tags);

-- Auth and health
CREATE INDEX idx_tool_auth_configs_tool 
    ON mcp.tool_auth_configs(tool_id) 
    WHERE is_active = true;

-- Partial unique index to ensure only one active auth config per tool
CREATE UNIQUE INDEX uk_tool_auth_active 
    ON mcp.tool_auth_configs(tool_id) 
    WHERE is_active = true;

CREATE INDEX idx_tool_health_checks_tool_time 
    ON mcp.tool_health_checks(tool_id, checked_at DESC);

-- ==============================================================================
-- TRIGGERS
-- ==============================================================================

-- Update timestamp triggers
CREATE TRIGGER update_tool_configurations_updated_at 
    BEFORE UPDATE ON mcp.tool_configurations
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_auth_configs_updated_at 
    BEFORE UPDATE ON mcp.tool_auth_configs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Update tool health status after health check
CREATE OR REPLACE FUNCTION mcp.update_tool_health_status()
RETURNS TRIGGER AS $$
BEGIN
    UPDATE mcp.tool_configurations
    SET 
        health_status = NEW.status,
        last_health_check = NEW.checked_at,
        health_message = NEW.error_message
    WHERE id = NEW.tool_id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_tool_health_after_check
    AFTER INSERT ON mcp.tool_health_checks
    FOR EACH ROW EXECUTE FUNCTION mcp.update_tool_health_status();

-- Update discovery pattern statistics
CREATE OR REPLACE FUNCTION mcp.update_discovery_pattern_stats()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.status = 'completed' THEN
        UPDATE mcp.tool_discovery_patterns
        SET 
            success_count = success_count + 1,
            last_successful_discovery = NEW.completed_at,
            confidence_score = LEAST(1.0, confidence_score + 0.1)
        WHERE domain = (
            SELECT REGEXP_REPLACE(base_url, '^https?://([^/]+).*', '\1')
            FROM mcp.tool_configurations
            WHERE id = NEW.tool_id
        );
    ELSIF NEW.status = 'failed' THEN
        UPDATE mcp.tool_discovery_patterns
        SET 
            failure_count = failure_count + 1,
            confidence_score = GREATEST(0.0, confidence_score - 0.05)
        WHERE domain = (
            SELECT REGEXP_REPLACE(base_url, '^https?://([^/]+).*', '\1')
            FROM mcp.tool_configurations
            WHERE id = NEW.tool_id
        );
    END IF;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_pattern_stats_after_discovery
    AFTER UPDATE OF status ON mcp.tool_discovery_sessions
    FOR EACH ROW 
    WHEN (OLD.status != NEW.status AND NEW.status IN ('completed', 'failed'))
    EXECUTE FUNCTION mcp.update_discovery_pattern_stats();

-- ==============================================================================
-- ROW LEVEL SECURITY
-- ==============================================================================

-- Enable RLS on all tables
ALTER TABLE mcp.tool_configurations ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_discovery_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_discovery_patterns ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_executions ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_auth_configs ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_health_checks ENABLE ROW LEVEL SECURITY;

-- Create RLS policies
CREATE POLICY tenant_isolation_tool_configurations ON mcp.tool_configurations
    USING (tenant_id = mcp.current_tenant_id());

CREATE POLICY tenant_isolation_tool_executions ON mcp.tool_executions
    USING (tenant_id = mcp.current_tenant_id());

-- Discovery sessions inherit tenant from tool
CREATE POLICY tenant_isolation_tool_discovery_sessions ON mcp.tool_discovery_sessions
    USING (tool_id IN (
        SELECT id FROM mcp.tool_configurations 
        WHERE tenant_id = mcp.current_tenant_id()
    ));

-- Auth configs inherit tenant from tool
CREATE POLICY tenant_isolation_tool_auth_configs ON mcp.tool_auth_configs
    USING (tool_id IN (
        SELECT id FROM mcp.tool_configurations 
        WHERE tenant_id = mcp.current_tenant_id()
    ));

-- Health checks inherit tenant from tool
CREATE POLICY tenant_isolation_tool_health_checks ON mcp.tool_health_checks
    USING (tool_id IN (
        SELECT id FROM mcp.tool_configurations 
        WHERE tenant_id = mcp.current_tenant_id()
    ));

-- Discovery patterns are shared across tenants (learning system)
CREATE POLICY public_read_discovery_patterns ON mcp.tool_discovery_patterns
    FOR SELECT USING (true);

CREATE POLICY tenant_write_discovery_patterns ON mcp.tool_discovery_patterns
    FOR ALL USING (mcp.current_tenant_id() IS NOT NULL);

-- ==============================================================================
-- FUNCTIONS
-- ==============================================================================

-- Function to safely execute tool action
CREATE OR REPLACE FUNCTION mcp.execute_tool_action(
    p_tool_id UUID,
    p_action VARCHAR(255),
    p_input_data JSONB,
    p_initiated_by UUID DEFAULT NULL
) RETURNS UUID AS $$
DECLARE
    v_execution_id UUID;
    v_tenant_id UUID;
    v_is_active BOOLEAN;
BEGIN
    -- Verify tool exists and is active
    SELECT tenant_id, is_active 
    INTO v_tenant_id, v_is_active
    FROM mcp.tool_configurations
    WHERE id = p_tool_id;
    
    IF NOT FOUND THEN
        RAISE EXCEPTION 'Tool % not found', p_tool_id;
    END IF;
    
    IF NOT v_is_active THEN
        RAISE EXCEPTION 'Tool % is not active', p_tool_id;
    END IF;
    
    -- Create execution record
    INSERT INTO mcp.tool_executions (
        tool_id, tenant_id, action, input_data, initiated_by, status
    ) VALUES (
        p_tool_id, v_tenant_id, p_action, p_input_data, p_initiated_by, 'pending'
    ) RETURNING id INTO v_execution_id;
    
    RETURN v_execution_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to get tool by name within tenant
CREATE OR REPLACE FUNCTION mcp.get_tool_by_name(
    p_tenant_id UUID,
    p_name VARCHAR(255)
) RETURNS TABLE (
    id UUID,
    name VARCHAR(255),
    type VARCHAR(50),
    base_url TEXT,
    is_active BOOLEAN,
    health_status VARCHAR(20)
) AS $$
BEGIN
    RETURN QUERY
    SELECT 
        t.id,
        t.name,
        t.type,
        t.base_url,
        t.is_active,
        t.health_status
    FROM mcp.tool_configurations t
    WHERE t.tenant_id = p_tenant_id
        AND t.name = p_name;
END;
$$ LANGUAGE plpgsql STABLE;

-- ==============================================================================
-- INITIAL DATA
-- ==============================================================================

-- Insert common discovery patterns for well-known APIs
INSERT INTO mcp.tool_discovery_patterns (domain, successful_paths, api_format, auth_method, confidence_score) VALUES
    ('github.com', '["/.well-known/openapi.json", "/api/v3/openapi.json"]'::jsonb, 'openapi', 'bearer', 0.9),
    ('api.github.com', '["/openapi.json", "/swagger.json"]'::jsonb, 'openapi', 'bearer', 0.9),
    ('slack.com', '["/api/openapi.json", "/api/specs/openapi.json"]'::jsonb, 'openapi', 'bearer', 0.8),
    ('api.stripe.com', '["/v1/openapi_spec.json"]'::jsonb, 'openapi', 'bearer', 0.9)
ON CONFLICT (domain) DO NOTHING;

COMMIT;