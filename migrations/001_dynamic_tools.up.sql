-- Migration: Dynamic Tools Feature
-- Version: 001
-- Date: 2025-01-27

-- Tool configurations per tenant
CREATE TABLE IF NOT EXISTS tool_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    tool_type VARCHAR(50) NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    display_name VARCHAR(200),
    config JSONB NOT NULL,
    credentials_encrypted BYTEA,
    auth_type VARCHAR(20) NOT NULL DEFAULT 'token',
    retry_policy JSONB,
    status VARCHAR(20) DEFAULT 'active',
    health_status VARCHAR(20) DEFAULT 'unknown',
    last_health_check TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100),
    UNIQUE(tenant_id, tool_name)
);

-- Indexes for performance
CREATE INDEX idx_tool_tenant_status ON tool_configurations(tenant_id, status);
CREATE INDEX idx_tool_tenant_type ON tool_configurations(tenant_id, tool_type);
CREATE INDEX idx_tool_health_check ON tool_configurations(last_health_check) WHERE health_status = 'unknown';

-- Tool discovery sessions
CREATE TABLE IF NOT EXISTS tool_discovery_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    session_id VARCHAR(100) UNIQUE,
    tool_type VARCHAR(50),
    base_url VARCHAR(500),
    status VARCHAR(50),
    discovered_urls JSONB,
    selected_url VARCHAR(500),
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '1 hour'
);

-- Index for session cleanup
CREATE INDEX idx_discovery_expires ON tool_discovery_sessions(expires_at);

-- Tool execution audit log
CREATE TABLE IF NOT EXISTS tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_config_id UUID REFERENCES tool_configurations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    action VARCHAR(100),
    parameters JSONB,
    status VARCHAR(20),
    retry_count INT DEFAULT 0,
    error TEXT,
    response_time_ms INT,
    executed_at TIMESTAMP DEFAULT NOW(),
    executed_by VARCHAR(100)
);

-- Index for audit queries
CREATE INDEX idx_execution_tenant_tool_time ON tool_executions(tenant_id, tool_config_id, executed_at DESC);
CREATE INDEX idx_execution_status ON tool_executions(status, executed_at DESC);

-- Retry attempts tracking
CREATE TABLE IF NOT EXISTS tool_execution_retries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID REFERENCES tool_executions(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    error_type VARCHAR(50),
    error_message TEXT,
    backoff_ms INT,
    attempted_at TIMESTAMP DEFAULT NOW()
);

-- Index for retry analysis
CREATE INDEX idx_retry_execution ON tool_execution_retries(execution_id, attempt_number);

-- Update trigger for updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tool_configurations_updated_at BEFORE UPDATE
    ON tool_configurations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();