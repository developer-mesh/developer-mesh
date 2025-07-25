-- Migration: Create Dynamic Tools Schema
-- Version: 20250125_01
-- Description: Creates tables for dynamic tool management system

-- Create migration metadata table if not exists
CREATE TABLE IF NOT EXISTS migration_metadata (
    id SERIAL PRIMARY KEY,
    version VARCHAR(50) UNIQUE NOT NULL,
    description TEXT,
    applied_at TIMESTAMP DEFAULT NOW(),
    rollback_at TIMESTAMP,
    status VARCHAR(20) DEFAULT 'applied'
);

-- Insert migration record
INSERT INTO migration_metadata (version, description) 
VALUES ('20250125_01', 'Create dynamic tools schema');

-- 1. Create tool_configurations table
CREATE TABLE tool_configurations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    tool_name VARCHAR(100) NOT NULL,
    display_name VARCHAR(200),
    config JSONB NOT NULL DEFAULT '{}',
    credentials_encrypted BYTEA,
    auth_type VARCHAR(20) NOT NULL DEFAULT 'token' CHECK (auth_type IN (
        'token', 'basic', 'oauth2', 'api_key', 'custom'
    )),
    retry_policy JSONB DEFAULT '{
        "initial_delay": "1s",
        "max_delay": "30s",
        "jitter": 0.1,
        "max_attempts": 3,
        "retryable_errors": ["timeout", "rate_limit", "temporary_failure"],
        "retry_on_timeout": true,
        "retry_on_rate_limit": true
    }',
    status VARCHAR(20) DEFAULT 'active' CHECK (status IN (
        'active', 'inactive', 'pending', 'error'
    )),
    health_status VARCHAR(20) DEFAULT 'unknown' CHECK (health_status IN (
        'healthy', 'unhealthy', 'degraded', 'unknown'
    )),
    last_health_check TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    created_by VARCHAR(100),
    UNIQUE(tenant_id, tool_name)
);

-- Create indexes for tool_configurations
CREATE INDEX idx_tool_config_tenant_status ON tool_configurations(tenant_id, status);
CREATE INDEX idx_tool_config_tenant_name ON tool_configurations(tenant_id, tool_name);
CREATE INDEX idx_tool_config_health_check ON tool_configurations(last_health_check, health_status);

-- 2. Create tool_discovery_sessions table
CREATE TABLE tool_discovery_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    session_id VARCHAR(100) UNIQUE NOT NULL,
    base_url VARCHAR(500) NOT NULL,
    status VARCHAR(50) DEFAULT 'pending' CHECK (status IN (
        'pending', 'discovering', 'discovered', 'confirmed', 'failed', 'expired'
    )),
    discovered_urls JSONB DEFAULT '[]',
    selected_url VARCHAR(500),
    discovery_metadata JSONB DEFAULT '{}',
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    expires_at TIMESTAMP DEFAULT NOW() + INTERVAL '1 hour'
);

-- Create indexes for tool_discovery_sessions
CREATE INDEX idx_discovery_tenant_session ON tool_discovery_sessions(tenant_id, session_id);
CREATE INDEX idx_discovery_expires ON tool_discovery_sessions(expires_at) WHERE status IN ('pending', 'discovering');

-- 3. Create tool_executions table
CREATE TABLE tool_executions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_config_id UUID REFERENCES tool_configurations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    action VARCHAR(100) NOT NULL,
    parameters JSONB DEFAULT '{}',
    status VARCHAR(20) DEFAULT 'pending' CHECK (status IN (
        'pending', 'running', 'success', 'failed', 'timeout', 'retrying'
    )),
    result JSONB,
    retry_count INT DEFAULT 0,
    error TEXT,
    error_type VARCHAR(50),
    response_time_ms INT,
    executed_at TIMESTAMP DEFAULT NOW(),
    completed_at TIMESTAMP,
    executed_by VARCHAR(100),
    correlation_id VARCHAR(100)
);

-- Create indexes for tool_executions
CREATE INDEX idx_exec_tenant_tool_time ON tool_executions(tenant_id, tool_config_id, executed_at DESC);
CREATE INDEX idx_exec_status_time ON tool_executions(status, executed_at) WHERE status IN ('pending', 'running', 'retrying');
CREATE INDEX idx_exec_correlation ON tool_executions(correlation_id) WHERE correlation_id IS NOT NULL;

-- 4. Create tool_execution_retries table
CREATE TABLE tool_execution_retries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    execution_id UUID REFERENCES tool_executions(id) ON DELETE CASCADE,
    attempt_number INT NOT NULL,
    error_type VARCHAR(50),
    error_message TEXT,
    backoff_ms INT,
    attempted_at TIMESTAMP DEFAULT NOW(),
    response_time_ms INT
);

-- Create indexes for tool_execution_retries
CREATE INDEX idx_retry_execution ON tool_execution_retries(execution_id, attempt_number);

-- 5. Create tool_health_checks table for detailed health history
CREATE TABLE tool_health_checks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_config_id UUID REFERENCES tool_configurations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    is_healthy BOOLEAN NOT NULL,
    response_time_ms INT,
    status_code INT,
    error_message TEXT,
    version_info VARCHAR(100),
    capabilities JSONB,
    checked_at TIMESTAMP DEFAULT NOW()
);

-- Create indexes for tool_health_checks
CREATE INDEX idx_health_tool_time ON tool_health_checks(tool_config_id, checked_at DESC);
CREATE INDEX idx_health_tenant_unhealthy ON tool_health_checks(tenant_id, checked_at) WHERE is_healthy = false;

-- 6. Create tool_credentials table for secure credential storage
CREATE TABLE tool_credentials (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tool_config_id UUID REFERENCES tool_configurations(id) ON DELETE CASCADE,
    tenant_id UUID NOT NULL,
    credential_type VARCHAR(50) NOT NULL,
    encrypted_value BYTEA NOT NULL,
    encryption_key_version INT DEFAULT 1,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(tool_config_id, credential_type)
);

-- Create indexes for tool_credentials
CREATE INDEX idx_cred_tool ON tool_credentials(tool_config_id);
CREATE INDEX idx_cred_expires ON tool_credentials(expires_at) WHERE expires_at IS NOT NULL;

-- 7. Create openapi_cache table for caching discovered OpenAPI specs
CREATE TABLE openapi_cache (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    url VARCHAR(500) NOT NULL,
    spec_hash VARCHAR(64) NOT NULL,
    spec_data JSONB NOT NULL,
    version VARCHAR(20),
    title VARCHAR(200),
    discovered_actions TEXT[],
    cache_expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(url, spec_hash)
);

-- Create indexes for openapi_cache
CREATE INDEX idx_openapi_url ON openapi_cache(url);
CREATE INDEX idx_openapi_expires ON openapi_cache(cache_expires_at);

-- Create updated_at triggers
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_tool_configurations_updated_at BEFORE UPDATE
    ON tool_configurations FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_tool_credentials_updated_at BEFORE UPDATE
    ON tool_credentials FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Create cleanup function for expired sessions
CREATE OR REPLACE FUNCTION cleanup_expired_discovery_sessions()
RETURNS void AS $$
BEGIN
    DELETE FROM tool_discovery_sessions 
    WHERE expires_at < NOW() AND status IN ('pending', 'discovering');
END;
$$ LANGUAGE plpgsql;

-- Create cleanup function for old executions
CREATE OR REPLACE FUNCTION cleanup_old_executions(days_to_keep INT DEFAULT 30)
RETURNS void AS $$
BEGIN
    DELETE FROM tool_executions 
    WHERE executed_at < NOW() - INTERVAL '1 day' * days_to_keep;
END;
$$ LANGUAGE plpgsql;

-- Grant permissions (adjust as needed for your user/role structure)
-- GRANT ALL ON ALL TABLES IN SCHEMA public TO mcp_app_user;
-- GRANT USAGE ON ALL SEQUENCES IN SCHEMA public TO mcp_app_user;