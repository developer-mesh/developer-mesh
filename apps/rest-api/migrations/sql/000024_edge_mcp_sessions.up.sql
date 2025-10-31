-- Edge MCP Session Management System
-- Critical infrastructure for tracking edge-mcp client sessions
BEGIN;

-- Create edge MCP sessions table
CREATE TABLE IF NOT EXISTS mcp.edge_mcp_sessions (
    -- Primary identification
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id VARCHAR(255) UNIQUE NOT NULL,
    
    -- Association
    tenant_id UUID NOT NULL,
    user_id UUID REFERENCES mcp.users(id) ON DELETE SET NULL,
    edge_mcp_id VARCHAR(255) NOT NULL,
    
    -- Client information
    client_name VARCHAR(255),
    client_type VARCHAR(50) CHECK (client_type IN ('claude-code', 'ide', 'agent', 'cli')),
    client_version VARCHAR(50),
    
    -- Session state
    status VARCHAR(50) DEFAULT 'active' CHECK (status IN ('active', 'idle', 'expired', 'terminated')),
    initialized BOOLEAN DEFAULT false,
    core_session_id VARCHAR(255), -- Link to MCP WebSocket session if applicable
    
    -- Passthrough auth (encrypted)
    passthrough_auth_encrypted TEXT,
    
    -- Metadata
    connection_metadata JSONB,
    context_id UUID REFERENCES mcp.contexts(id) ON DELETE SET NULL,
    
    -- Activity tracking
    last_activity_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    tool_execution_count INTEGER DEFAULT 0,
    total_tokens_used INTEGER DEFAULT 0,
    
    -- Lifecycle
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    expires_at TIMESTAMP WITH TIME ZONE,
    terminated_at TIMESTAMP WITH TIME ZONE,
    termination_reason TEXT
);

-- Create indexes for performance
CREATE INDEX idx_edge_sessions_tenant ON mcp.edge_mcp_sessions(tenant_id);
CREATE INDEX idx_edge_sessions_user ON mcp.edge_mcp_sessions(user_id) WHERE user_id IS NOT NULL;
CREATE INDEX idx_edge_sessions_edge_mcp ON mcp.edge_mcp_sessions(edge_mcp_id);
CREATE INDEX idx_edge_sessions_status ON mcp.edge_mcp_sessions(status);
CREATE INDEX idx_edge_sessions_active ON mcp.edge_mcp_sessions(status, expires_at) WHERE status = 'active';
CREATE INDEX idx_edge_sessions_activity ON mcp.edge_mcp_sessions(last_activity_at);
CREATE INDEX idx_edge_sessions_core ON mcp.edge_mcp_sessions(core_session_id) WHERE core_session_id IS NOT NULL;

-- Tool execution audit trail
CREATE TABLE IF NOT EXISTS mcp.session_tool_executions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    session_id UUID NOT NULL REFERENCES mcp.edge_mcp_sessions(id) ON DELETE CASCADE,
    tool_name VARCHAR(255) NOT NULL,
    tool_id UUID REFERENCES mcp.tool_configurations(id) ON DELETE SET NULL,
    
    -- Execution details
    arguments JSONB,
    result JSONB,
    error TEXT,
    
    -- Performance metrics
    duration_ms INTEGER,
    tokens_used INTEGER,
    
    -- Timestamps
    executed_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for tool executions
CREATE INDEX idx_tool_exec_session ON mcp.session_tool_executions(session_id);
CREATE INDEX idx_tool_exec_tool ON mcp.session_tool_executions(tool_id) WHERE tool_id IS NOT NULL;
CREATE INDEX idx_tool_exec_time ON mcp.session_tool_executions(executed_at);
CREATE INDEX idx_tool_exec_name ON mcp.session_tool_executions(tool_name);

-- Add session metrics view for analytics
CREATE OR REPLACE VIEW mcp.session_metrics AS
SELECT 
    tenant_id,
    COUNT(*) FILTER (WHERE status = 'active') as active_sessions,
    COUNT(*) as total_sessions,
    SUM(tool_execution_count) as total_tool_executions,
    SUM(total_tokens_used) as total_tokens_used,
    AVG(EXTRACT(EPOCH FROM (COALESCE(terminated_at, CURRENT_TIMESTAMP) - created_at))/60) as avg_session_duration_minutes
FROM mcp.edge_mcp_sessions
GROUP BY tenant_id;

-- Function to cleanup expired sessions
CREATE OR REPLACE FUNCTION mcp.cleanup_expired_sessions()
RETURNS INTEGER AS $$
DECLARE
    updated_count INTEGER;
BEGIN
    UPDATE mcp.edge_mcp_sessions
    SET status = 'expired',
        terminated_at = CURRENT_TIMESTAMP,
        termination_reason = 'Session expired'
    WHERE status = 'active' 
    AND expires_at < CURRENT_TIMESTAMP;
    
    GET DIAGNOSTICS updated_count = ROW_COUNT;
    RETURN updated_count;
END;
$$ LANGUAGE plpgsql;

-- Add comment documentation
COMMENT ON TABLE mcp.edge_mcp_sessions IS 'Tracks edge-mcp client sessions for authentication, auditing, and metrics';
COMMENT ON TABLE mcp.session_tool_executions IS 'Audit trail of all tool executions within edge-mcp sessions';
COMMENT ON VIEW mcp.session_metrics IS 'Aggregated metrics for session analytics per tenant';
COMMENT ON FUNCTION mcp.cleanup_expired_sessions() IS 'Marks expired sessions as expired, returns count of updated sessions';

COMMIT;