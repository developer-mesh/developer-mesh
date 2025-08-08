-- =====================================================================
-- Agent Lifecycle Management System - Production Grade Migration
-- Version: 1.0.0
-- Author: Principal Engineer
-- Description: Comprehensive agent lifecycle management with state machine,
--              event sourcing, health monitoring, and capability discovery
-- =====================================================================

-- =====================================================================
-- PART 1: Core Agent Lifecycle Tables
-- =====================================================================

-- Agent states enum (if not exists)
DO $$ 
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'agent_state') THEN
        CREATE TYPE mcp.agent_state AS ENUM (
            'pending',       -- Initial registration, awaiting configuration
            'configuring',   -- Configuration in progress
            'validating',    -- Validating capabilities and connections
            'ready',         -- Ready for activation
            'active',        -- Fully operational
            'degraded',      -- Partial failure, some capabilities unavailable
            'suspended',     -- Temporarily disabled by admin
            'terminating',   -- Cleanup in progress
            'terminated'     -- Fully removed (terminal state)
        );
    END IF;
END$$;

-- Add state column to agents table if not exists
ALTER TABLE mcp.agents 
ADD COLUMN IF NOT EXISTS state mcp.agent_state DEFAULT 'pending'::mcp.agent_state,
ADD COLUMN IF NOT EXISTS state_reason TEXT,
ADD COLUMN IF NOT EXISTS state_changed_at TIMESTAMPTZ DEFAULT NOW(),
ADD COLUMN IF NOT EXISTS state_changed_by UUID,
ADD COLUMN IF NOT EXISTS health_status JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS health_checked_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS metadata JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS version INTEGER DEFAULT 1,
ADD COLUMN IF NOT EXISTS activation_count INTEGER DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_error TEXT,
ADD COLUMN IF NOT EXISTS last_error_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS retry_count INTEGER DEFAULT 0;

-- Add indexes for state queries
CREATE INDEX IF NOT EXISTS idx_agents_state ON mcp.agents(state);
CREATE INDEX IF NOT EXISTS idx_agents_state_tenant ON mcp.agents(tenant_id, state);
CREATE INDEX IF NOT EXISTS idx_agents_health_checked ON mcp.agents(health_checked_at) 
    WHERE state IN ('active', 'degraded');

-- =====================================================================
-- PART 2: Agent Event Sourcing
-- =====================================================================

-- Agent events table for audit trail and event sourcing
CREATE TABLE IF NOT EXISTS mcp.agent_events (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES mcp.agents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    event_version VARCHAR(10) DEFAULT '1.0.0',
    from_state mcp.agent_state,
    to_state mcp.agent_state,
    payload JSONB DEFAULT '{}'::jsonb,
    error_message TEXT,
    error_code VARCHAR(50),
    initiated_by UUID,
    correlation_id UUID,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- Create indexes for agent_events
CREATE INDEX idx_agent_events_agent_id ON mcp.agent_events(agent_id);
CREATE INDEX idx_agent_events_tenant_id ON mcp.agent_events(tenant_id);
CREATE INDEX idx_agent_events_type ON mcp.agent_events(event_type);
CREATE INDEX idx_agent_events_created_at ON mcp.agent_events(created_at DESC);
CREATE INDEX idx_agent_events_correlation ON mcp.agent_events(correlation_id);

-- Create partitioned table for high-volume events (monthly partitions)
-- This is for production scalability
DO $$ 
BEGIN
    -- Only create partitioning if table doesn't exist or isn't partitioned
    IF NOT EXISTS (
        SELECT 1 FROM pg_class c 
        JOIN pg_namespace n ON n.oid = c.relnamespace 
        WHERE n.nspname = 'mcp' 
        AND c.relname = 'agent_events' 
        AND c.relkind = 'p'
    ) THEN
        -- For now, we'll use the regular table
        -- Partitioning can be added later without data loss
        NULL;
    END IF;
END$$;

-- =====================================================================
-- PART 3: Agent Capabilities Discovery
-- =====================================================================

-- Enhance agent_capabilities table if it exists
ALTER TABLE mcp.agent_capabilities 
ADD COLUMN IF NOT EXISTS capability_type VARCHAR(50) DEFAULT 'standard',
ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 100,
ADD COLUMN IF NOT EXISTS dependencies JSONB DEFAULT '[]'::jsonb,
ADD COLUMN IF NOT EXISTS health_endpoint VARCHAR(500),
ADD COLUMN IF NOT EXISTS last_health_check TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS health_status VARCHAR(50) DEFAULT 'unknown',
ADD COLUMN IF NOT EXISTS performance_metrics JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS validated_at TIMESTAMPTZ,
ADD COLUMN IF NOT EXISTS validation_errors JSONB;

-- Add capability discovery patterns
CREATE TABLE IF NOT EXISTS mcp.agent_capability_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_type VARCHAR(100) NOT NULL,
    capability_name VARCHAR(100) NOT NULL,
    capability_version VARCHAR(20) DEFAULT '1.0.0',
    required BOOLEAN DEFAULT false,
    default_config JSONB DEFAULT '{}'::jsonb,
    validation_schema JSONB,
    dependencies JSONB DEFAULT '[]'::jsonb,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(agent_type, capability_name, capability_version)
);

-- =====================================================================
-- PART 4: Agent Health Monitoring
-- =====================================================================

-- Agent health metrics table (partitioned for performance)
CREATE TABLE IF NOT EXISTS mcp.agent_health_metrics (
    agent_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    metric_type VARCHAR(50) NOT NULL,
    metric_name VARCHAR(100) NOT NULL,
    value NUMERIC,
    unit VARCHAR(20),
    tags JSONB DEFAULT '{}'::jsonb,
    metadata JSONB DEFAULT '{}'::jsonb,
    recorded_at TIMESTAMPTZ DEFAULT NOW(),
    
    PRIMARY KEY (agent_id, metric_type, metric_name, recorded_at)
) PARTITION BY RANGE (recorded_at);

-- Create initial partitions (3 months)
CREATE TABLE IF NOT EXISTS mcp.agent_health_metrics_2025_01 
    PARTITION OF mcp.agent_health_metrics 
    FOR VALUES FROM ('2025-01-01') TO ('2025-02-01');

CREATE TABLE IF NOT EXISTS mcp.agent_health_metrics_2025_02 
    PARTITION OF mcp.agent_health_metrics 
    FOR VALUES FROM ('2025-02-01') TO ('2025-03-01');

CREATE TABLE IF NOT EXISTS mcp.agent_health_metrics_2025_03 
    PARTITION OF mcp.agent_health_metrics 
    FOR VALUES FROM ('2025-03-01') TO ('2025-04-01');

-- Indexes for health metrics
CREATE INDEX IF NOT EXISTS idx_health_metrics_agent_time 
    ON mcp.agent_health_metrics (agent_id, recorded_at DESC);
CREATE INDEX IF NOT EXISTS idx_health_metrics_type 
    ON mcp.agent_health_metrics (metric_type, recorded_at DESC);

-- =====================================================================
-- PART 5: Agent Configuration Enhancements
-- =====================================================================

-- Ensure agent_configs has all necessary fields
ALTER TABLE mcp.agent_configs
ADD COLUMN IF NOT EXISTS agent_type VARCHAR(100),
ADD COLUMN IF NOT EXISTS priority INTEGER DEFAULT 100,
ADD COLUMN IF NOT EXISTS retry_policy JSONB DEFAULT '{"max_retries": 3, "backoff_ms": 1000}'::jsonb,
ADD COLUMN IF NOT EXISTS timeout_ms INTEGER DEFAULT 30000,
ADD COLUMN IF NOT EXISTS circuit_breaker JSONB DEFAULT '{"enabled": true, "threshold": 5, "timeout_ms": 60000}'::jsonb,
ADD COLUMN IF NOT EXISTS capabilities_config JSONB DEFAULT '{}'::jsonb,
ADD COLUMN IF NOT EXISTS scheduling_preferences JSONB DEFAULT '{}'::jsonb;

-- Add index for active configs per agent
CREATE INDEX IF NOT EXISTS idx_agent_configs_agent_active 
    ON mcp.agent_configs(agent_id, is_active, version DESC);

-- =====================================================================
-- PART 6: Agent Session Management
-- =====================================================================

CREATE TABLE IF NOT EXISTS mcp.agent_sessions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES mcp.agents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    session_token VARCHAR(500) UNIQUE NOT NULL,
    connection_type VARCHAR(50) NOT NULL, -- 'websocket', 'rest', 'grpc'
    connection_metadata JSONB DEFAULT '{}'::jsonb,
    ip_address INET,
    user_agent TEXT,
    started_at TIMESTAMPTZ DEFAULT NOW(),
    last_activity_at TIMESTAMPTZ DEFAULT NOW(),
    ended_at TIMESTAMPTZ,
    end_reason VARCHAR(100),
    metrics JSONB DEFAULT '{}'::jsonb
);

-- Create indexes for agent_sessions
CREATE INDEX idx_agent_sessions_agent ON mcp.agent_sessions(agent_id);
CREATE INDEX idx_agent_sessions_token ON mcp.agent_sessions(session_token);
CREATE INDEX idx_agent_sessions_active ON mcp.agent_sessions(agent_id, ended_at) WHERE ended_at IS NULL;

-- =====================================================================
-- PART 7: Agent State Machine Validation
-- =====================================================================

-- Valid state transitions table
CREATE TABLE IF NOT EXISTS mcp.agent_state_transitions (
    from_state mcp.agent_state NOT NULL,
    to_state mcp.agent_state NOT NULL,
    requires_validation BOOLEAN DEFAULT false,
    validation_function VARCHAR(255),
    description TEXT,
    
    PRIMARY KEY (from_state, to_state)
);

-- Insert valid state transitions
INSERT INTO mcp.agent_state_transitions (from_state, to_state, requires_validation, description) VALUES
    ('pending', 'configuring', false, 'Start configuration process'),
    ('pending', 'terminating', false, 'Abort registration'),
    ('configuring', 'validating', true, 'Configuration complete, start validation'),
    ('configuring', 'pending', false, 'Configuration failed, retry'),
    ('configuring', 'terminating', false, 'Abort during configuration'),
    ('validating', 'ready', true, 'Validation successful'),
    ('validating', 'configuring', false, 'Validation failed, reconfigure'),
    ('validating', 'terminating', false, 'Critical validation failure'),
    ('ready', 'active', true, 'Activate agent'),
    ('ready', 'validating', false, 'Re-validate before activation'),
    ('ready', 'terminating', false, 'Cancel activation'),
    ('active', 'degraded', false, 'Partial failure detected'),
    ('active', 'suspended', false, 'Admin suspension'),
    ('active', 'terminating', false, 'Deactivate agent'),
    ('degraded', 'active', true, 'Recovery successful'),
    ('degraded', 'suspended', false, 'Suspend degraded agent'),
    ('degraded', 'terminating', false, 'Terminate failing agent'),
    ('suspended', 'active', true, 'Resume from suspension'),
    ('suspended', 'terminating', false, 'Terminate suspended agent'),
    ('terminating', 'terminated', false, 'Cleanup complete')
ON CONFLICT (from_state, to_state) DO NOTHING;

-- =====================================================================
-- PART 8: Agent Registry and Discovery
-- =====================================================================

CREATE TABLE IF NOT EXISTS mcp.agent_registry (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    agent_id UUID NOT NULL REFERENCES mcp.agents(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    service_name VARCHAR(255) NOT NULL,
    service_version VARCHAR(50),
    endpoint_url VARCHAR(500),
    protocol VARCHAR(50), -- 'rest', 'grpc', 'websocket'
    authentication_type VARCHAR(50),
    authentication_config JSONB DEFAULT '{}'::jsonb,
    health_check_url VARCHAR(500),
    discovery_metadata JSONB DEFAULT '{}'::jsonb,
    is_public BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    registered_at TIMESTAMPTZ DEFAULT NOW(),
    last_seen_at TIMESTAMPTZ DEFAULT NOW(),
    
    UNIQUE(agent_id, service_name)
);

-- Create indexes for agent_registry
CREATE INDEX idx_agent_registry_tenant ON mcp.agent_registry(tenant_id);
CREATE INDEX idx_agent_registry_service ON mcp.agent_registry(service_name);
CREATE INDEX idx_agent_registry_active ON mcp.agent_registry(is_active, last_seen_at DESC);

-- =====================================================================
-- PART 9: Agent Configuration Templates
-- =====================================================================

CREATE TABLE IF NOT EXISTS mcp.agent_config_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    name VARCHAR(255) NOT NULL,
    agent_type VARCHAR(100) NOT NULL,
    description TEXT,
    template_config JSONB NOT NULL,
    embedding_strategy VARCHAR(50) DEFAULT 'balanced',
    model_preferences JSONB DEFAULT '[]'::jsonb,
    capabilities TEXT[] DEFAULT '{}',
    constraints JSONB DEFAULT '{}'::jsonb,
    is_default BOOLEAN DEFAULT false,
    is_active BOOLEAN DEFAULT true,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    created_by UUID,
    
    UNIQUE(name, agent_type)
);

-- Create indexes for agent_config_templates
CREATE INDEX idx_config_templates_type ON mcp.agent_config_templates(agent_type);
CREATE INDEX idx_config_templates_default ON mcp.agent_config_templates(agent_type, is_default) WHERE is_default = true;

-- Insert default templates for each agent type
INSERT INTO mcp.agent_config_templates (name, agent_type, description, template_config, embedding_strategy, model_preferences, capabilities) VALUES
(
    'IDE Agent Default',
    'ide',
    'Default configuration for IDE agents with code capabilities',
    '{"auto_complete": true, "semantic_search": true, "code_generation": true}'::jsonb,
    'balanced',
    '[{"provider": "bedrock", "model": "amazon.titan-embed-text-v1", "priority": 1}, {"provider": "openai", "model": "text-embedding-3-small", "priority": 2}]'::jsonb,
    ARRAY['code_generation', 'code_review', 'debugging', 'github_integration', 'embeddings']
),
(
    'Slack Agent Default',
    'slack',
    'Default configuration for Slack integration agents',
    '{"response_time_ms": 3000, "batch_messages": true}'::jsonb,
    'speed',
    '[{"provider": "openai", "model": "text-embedding-3-small", "priority": 1}]'::jsonb,
    ARRAY['chat', 'notifications', 'embeddings', 'search']
),
(
    'CI/CD Agent Default',
    'cicd',
    'Default configuration for CI/CD pipeline agents',
    '{"parallel_jobs": 5, "timeout_minutes": 30}'::jsonb,
    'speed',
    '[{"provider": "bedrock", "model": "amazon.titan-embed-text-v1", "priority": 1}]'::jsonb,
    ARRAY['build', 'test', 'deploy', 'monitoring']
),
(
    'Monitoring Agent Default',
    'monitoring',
    'Default configuration for system monitoring agents',
    '{"poll_interval_seconds": 60, "alert_threshold": 0.8}'::jsonb,
    'cost',
    '[{"provider": "openai", "model": "text-embedding-3-small", "priority": 1}]'::jsonb,
    ARRAY['metrics', 'alerts', 'logging', 'tracing']
)
ON CONFLICT (name, agent_type) DO NOTHING;

-- =====================================================================
-- PART 10: Functions and Triggers
-- =====================================================================

-- Function to validate state transitions
CREATE OR REPLACE FUNCTION mcp.validate_agent_state_transition()
RETURNS TRIGGER AS $$
BEGIN
    -- Check if state is actually changing
    IF OLD.state = NEW.state THEN
        RETURN NEW;
    END IF;
    
    -- Check if transition is valid
    IF NOT EXISTS (
        SELECT 1 FROM mcp.agent_state_transitions 
        WHERE from_state = OLD.state AND to_state = NEW.state
    ) THEN
        RAISE EXCEPTION 'Invalid state transition from % to %', OLD.state, NEW.state;
    END IF;
    
    -- Update state change metadata
    NEW.state_changed_at = NOW();
    NEW.version = OLD.version + 1;
    
    -- Record event
    INSERT INTO mcp.agent_events (
        agent_id, tenant_id, event_type, from_state, to_state, 
        initiated_by, payload
    ) VALUES (
        NEW.id, NEW.tenant_id, 'state_transition', OLD.state, NEW.state,
        NEW.state_changed_by, 
        jsonb_build_object(
            'reason', NEW.state_reason,
            'old_version', OLD.version,
            'new_version', NEW.version
        )
    );
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for state validation
DROP TRIGGER IF EXISTS validate_agent_state_change ON mcp.agents;
CREATE TRIGGER validate_agent_state_change
    BEFORE UPDATE OF state ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.validate_agent_state_transition();

-- Function to automatically create default config when agent is created
CREATE OR REPLACE FUNCTION mcp.create_default_agent_config()
RETURNS TRIGGER AS $$
DECLARE
    template_record RECORD;
    config_id UUID;
BEGIN
    -- Skip if agent already has a config
    IF EXISTS (SELECT 1 FROM mcp.agent_configs WHERE agent_id = NEW.id) THEN
        RETURN NEW;
    END IF;
    
    -- Find default template for this agent type
    SELECT * INTO template_record
    FROM mcp.agent_config_templates
    WHERE agent_type = NEW.type 
    AND is_default = true 
    AND is_active = true
    LIMIT 1;
    
    -- If no template found, create minimal config
    IF template_record IS NULL THEN
        INSERT INTO mcp.agent_configs (
            agent_id, agent_type, embedding_strategy, 
            model_preferences, constraints, metadata
        ) VALUES (
            NEW.id, NEW.type, 'balanced',
            '[{"provider": "bedrock", "model": "amazon.titan-embed-text-v1", "priority": 1}]'::jsonb,
            '{}'::jsonb,
            jsonb_build_object('auto_created', true, 'created_at', NOW())
        ) RETURNING id INTO config_id;
    ELSE
        -- Create config from template
        INSERT INTO mcp.agent_configs (
            agent_id, agent_type, embedding_strategy,
            model_preferences, constraints, capabilities_config,
            metadata
        ) VALUES (
            NEW.id, NEW.type, template_record.embedding_strategy,
            template_record.model_preferences,
            template_record.template_config,
            template_record.template_config,
            jsonb_build_object(
                'auto_created', true, 
                'template_id', template_record.id,
                'template_name', template_record.name,
                'created_at', NOW()
            )
        ) RETURNING id INTO config_id;
    END IF;
    
    -- Record event
    INSERT INTO mcp.agent_events (
        agent_id, tenant_id, event_type, payload
    ) VALUES (
        NEW.id, NEW.tenant_id, 'config_created',
        jsonb_build_object('config_id', config_id, 'auto_created', true)
    );
    
    -- Automatically transition to configuring state
    UPDATE mcp.agents 
    SET state = 'configuring'::mcp.agent_state,
        state_reason = 'Auto-configuration initiated'
    WHERE id = NEW.id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger for auto config creation
DROP TRIGGER IF EXISTS create_agent_config_on_insert ON mcp.agents;
CREATE TRIGGER create_agent_config_on_insert
    AFTER INSERT ON mcp.agents
    FOR EACH ROW
    EXECUTE FUNCTION mcp.create_default_agent_config();

-- Function to update agent health metrics
CREATE OR REPLACE FUNCTION mcp.update_agent_health(
    p_agent_id UUID,
    p_metric_type VARCHAR,
    p_metric_name VARCHAR,
    p_value NUMERIC,
    p_unit VARCHAR DEFAULT NULL,
    p_tags JSONB DEFAULT '{}'::jsonb
) RETURNS VOID AS $$
DECLARE
    v_tenant_id UUID;
BEGIN
    -- Get tenant_id from agent
    SELECT tenant_id INTO v_tenant_id FROM mcp.agents WHERE id = p_agent_id;
    
    -- Insert health metric
    INSERT INTO mcp.agent_health_metrics (
        agent_id, tenant_id, metric_type, metric_name, 
        value, unit, tags, recorded_at
    ) VALUES (
        p_agent_id, v_tenant_id, p_metric_type, p_metric_name,
        p_value, p_unit, p_tags, NOW()
    );
    
    -- Update agent's last health check time
    UPDATE mcp.agents 
    SET health_checked_at = NOW()
    WHERE id = p_agent_id;
END;
$$ LANGUAGE plpgsql;

-- =====================================================================
-- PART 11: Indexes for Performance
-- =====================================================================

-- Composite indexes for common queries
CREATE INDEX IF NOT EXISTS idx_agents_tenant_type_state 
    ON mcp.agents(tenant_id, type, state);

CREATE INDEX IF NOT EXISTS idx_agent_events_agent_type_time 
    ON mcp.agent_events(agent_id, event_type, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_agent_configs_agent_version 
    ON mcp.agent_configs(agent_id, version DESC);

-- =====================================================================
-- PART 12: Row Level Security
-- =====================================================================

-- Enable RLS on new tables
ALTER TABLE mcp.agent_events ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.agent_sessions ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.agent_registry ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.agent_health_metrics ENABLE ROW LEVEL SECURITY;

-- Create RLS policies for tenant isolation
CREATE POLICY tenant_isolation_agent_events ON mcp.agent_events
    FOR ALL USING (tenant_id = mcp.current_tenant_id());

CREATE POLICY tenant_isolation_agent_sessions ON mcp.agent_sessions
    FOR ALL USING (tenant_id = mcp.current_tenant_id());

CREATE POLICY tenant_isolation_agent_registry ON mcp.agent_registry
    FOR ALL USING (tenant_id = mcp.current_tenant_id());

CREATE POLICY tenant_isolation_agent_health ON mcp.agent_health_metrics
    FOR ALL USING (tenant_id = mcp.current_tenant_id());

-- =====================================================================
-- PART 13: Monitoring Views
-- =====================================================================

-- View for agent status overview
CREATE OR REPLACE VIEW mcp.v_agent_status AS
SELECT 
    a.id,
    a.tenant_id,
    a.name,
    a.type,
    a.state,
    a.state_reason,
    a.state_changed_at,
    a.health_checked_at,
    a.version,
    a.activation_count,
    ac.embedding_strategy,
    ac.model_preferences,
    COUNT(DISTINCT s.id) FILTER (WHERE s.ended_at IS NULL) as active_sessions,
    COUNT(DISTINCT e.id) FILTER (WHERE e.created_at > NOW() - INTERVAL '1 hour') as recent_events
FROM mcp.agents a
LEFT JOIN mcp.agent_configs ac ON a.id = ac.agent_id AND ac.is_active = true
LEFT JOIN mcp.agent_sessions s ON a.id = s.agent_id
LEFT JOIN mcp.agent_events e ON a.id = e.agent_id
GROUP BY a.id, a.tenant_id, a.name, a.type, a.state, a.state_reason, 
         a.state_changed_at, a.health_checked_at, a.version, a.activation_count,
         ac.embedding_strategy, ac.model_preferences;

-- Grant permissions
GRANT SELECT ON mcp.v_agent_status TO CURRENT_USER;

-- =====================================================================
-- MIGRATION COMPLETE
-- =====================================================================