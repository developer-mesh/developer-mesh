#!/bin/bash
# Create seed data SQL script

cat > scripts/db/seed-test-data.sql << 'EOF'
-- Test Data Seeding Script for Developer Mesh
-- This script is idempotent - safe to run multiple times

-- Enable UUID extension if not already enabled
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- Insert test tenants
INSERT INTO tenants (id, name, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000001', 'Test Tenant 1', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000002', 'Test Tenant 2', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET updated_at = NOW();

-- Insert test agents for tenant 1
INSERT INTO agents (id, tenant_id, name, type, capabilities, status, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000101', '00000000-0000-0000-0000-000000000001', 'test-code-agent', 'code_analysis', '["code_analysis", "code_review"]', 'active', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000102', '00000000-0000-0000-0000-000000000001', 'test-security-agent', 'security', '["security_scanning", "vulnerability_detection"]', 'active', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000103', '00000000-0000-0000-0000-000000000001', 'test-devops-agent', 'devops', '["deployment", "monitoring"]', 'active', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET updated_at = NOW();

-- Insert test models
INSERT INTO models (id, tenant_id, name, provider, type, is_active, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000201', '00000000-0000-0000-0000-000000000001', 'claude-3-opus', 'anthropic', 'llm', true, NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000202', '00000000-0000-0000-0000-000000000001', 'gpt-4', 'openai', 'llm', true, NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000203', '00000000-0000-0000-0000-000000000001', 'text-embedding-3-small', 'openai', 'embedding', true, NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET updated_at = NOW();

-- Insert test tool configurations
INSERT INTO tool_configurations (id, tenant_id, name, type, base_url, auth_config, health_check_url, created_at, updated_at) VALUES
    ('00000000-0000-0000-0000-000000000301', '00000000-0000-0000-0000-000000000001', 'GitHub API', 'github', 'https://api.github.com', '{"type": "bearer", "token": "test-token"}', 'https://api.github.com/rate_limit', NOW(), NOW()),
    ('00000000-0000-0000-0000-000000000302', '00000000-0000-0000-0000-000000000001', 'Test Tool', 'custom', 'http://localhost:8082', '{"type": "api_key", "key": "test-key"}', 'http://localhost:8082/health', NOW(), NOW())
ON CONFLICT (id) DO UPDATE SET updated_at = NOW();

-- Insert test API keys (using the development key)
-- Note: In production, these would be properly hashed
INSERT INTO api_keys (id, tenant_id, key_hash, name, role, scopes, is_active, created_at, expires_at) VALUES
    ('00000000-0000-0000-0000-000000000401', '00000000-0000-0000-0000-000000000001', 
     crypt('dev-admin-key-1234567890', gen_salt('bf')), 
     'Development Admin Key', 'admin', '["read", "write", "admin"]', true, NOW(), NOW() + INTERVAL '1 year')
ON CONFLICT (id) DO UPDATE SET expires_at = NOW() + INTERVAL '1 year';

-- Grant necessary permissions
GRANT ALL PRIVILEGES ON ALL TABLES IN SCHEMA public TO ${DATABASE_USER:-dev};
GRANT ALL PRIVILEGES ON ALL SEQUENCES IN SCHEMA public TO ${DATABASE_USER:-dev};

-- Output summary
DO \$\$
BEGIN
    RAISE NOTICE 'Test data seeding completed:';
    RAISE NOTICE '  - 2 test tenants';
    RAISE NOTICE '  - 3 test agents';
    RAISE NOTICE '  - 3 test models';
    RAISE NOTICE '  - 2 test tool configurations';
    RAISE NOTICE '  - 1 test API key';
END\$\$;
EOF