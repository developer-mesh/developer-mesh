-- ==============================================================================
-- Migration: 000029_standard_tools_integration
-- Description: Add support for standard tool templates and organization-specific instances
-- Author: DevMesh Team
-- Date: 2025-08-22
-- ==============================================================================

BEGIN;

-- ==============================================================================
-- TOOL TEMPLATES TABLE
-- ==============================================================================

-- Tool templates table - Pre-defined tool configurations for standard providers
CREATE TABLE IF NOT EXISTS mcp.tool_templates (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    provider_name VARCHAR(100) NOT NULL,
    provider_version VARCHAR(50) NOT NULL,
    display_name VARCHAR(255) NOT NULL,
    description TEXT,
    icon_url TEXT,
    category VARCHAR(100),
    
    -- Pre-defined configurations
    default_config JSONB NOT NULL DEFAULT '{}',
    operation_groups JSONB NOT NULL DEFAULT '[]',
    operation_mappings JSONB NOT NULL DEFAULT '{}',
    ai_definitions JSONB,
    
    -- Customization rules
    customization_schema JSONB,
    required_credentials TEXT[],
    optional_credentials TEXT[],
    optional_features JSONB,
    
    -- Feature flags
    features JSONB DEFAULT '{
        "supportsWebhooks": false,
        "supportsPagination": true,
        "supportsRateLimit": true,
        "supportsBatchOps": false,
        "supportsAsync": false,
        "supportsSearch": true,
        "supportsFiltering": true,
        "supportsSorting": true
    }',
    
    -- Metadata
    tags TEXT[],
    documentation_url TEXT,
    api_documentation_url TEXT,
    example_configurations JSONB,
    
    -- Visibility
    is_public BOOLEAN DEFAULT true,
    is_active BOOLEAN DEFAULT true,
    is_deprecated BOOLEAN DEFAULT false,
    deprecated_at TIMESTAMP WITH TIME ZONE,
    deprecated_message TEXT,
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Constraints
    CONSTRAINT uk_provider_version UNIQUE(provider_name, provider_version),
    CONSTRAINT chk_provider_name CHECK (provider_name ~ '^[a-z][a-z0-9_-]*$'),
    CONSTRAINT chk_category CHECK (category IS NULL OR category IN (
        'version_control', 'issue_tracking', 'ci_cd', 'monitoring',
        'collaboration', 'cloud', 'security', 'documentation', 
        'artifact_management', 'repository_management', 'projects', 'custom'
    ))
);

-- ==============================================================================
-- ORGANIZATION TOOLS TABLE
-- ==============================================================================

-- Organization-specific tool instances based on templates
CREATE TABLE IF NOT EXISTS mcp.organization_tools (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_id UUID NOT NULL,
    tenant_id UUID NOT NULL,
    template_id UUID REFERENCES mcp.tool_templates(id) ON DELETE RESTRICT,
    
    -- Instance configuration
    instance_name VARCHAR(255) NOT NULL,
    display_name VARCHAR(255),
    description TEXT,
    
    -- Configuration and credentials
    instance_config JSONB NOT NULL DEFAULT '{}',
    credentials_encrypted BYTEA,
    encryption_key_id UUID,
    
    -- Customizations
    custom_mappings JSONB,
    enabled_features JSONB,
    disabled_operations TEXT[],
    rate_limit_overrides JSONB,
    custom_headers JSONB,
    
    -- Health and status
    status VARCHAR(50) NOT NULL DEFAULT 'active',
    is_active BOOLEAN NOT NULL DEFAULT true,
    last_health_check TIMESTAMP WITH TIME ZONE,
    health_status JSONB,
    health_message TEXT,
    
    -- Usage tracking
    last_used_at TIMESTAMP WITH TIME ZONE,
    usage_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    
    -- Metadata
    tags TEXT[],
    metadata JSONB DEFAULT '{}',
    
    -- Audit fields
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    updated_by UUID,
    
    -- Constraints
    CONSTRAINT uk_org_tool_name UNIQUE(organization_id, instance_name),
    CONSTRAINT chk_status CHECK (status IN ('active', 'inactive', 'suspended', 'error', 'provisioning')),
    CONSTRAINT chk_instance_name CHECK (instance_name ~ '^[a-zA-Z0-9][a-zA-Z0-9_-]*$')
);

-- ==============================================================================
-- TOOL TEMPLATE VERSIONS TABLE
-- ==============================================================================

-- Track version history of tool templates
CREATE TABLE IF NOT EXISTS mcp.tool_template_versions (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    template_id UUID NOT NULL REFERENCES mcp.tool_templates(id) ON DELETE CASCADE,
    version_number VARCHAR(50) NOT NULL,
    
    -- Snapshot of template at this version
    template_snapshot JSONB NOT NULL,
    
    -- Version metadata
    change_summary TEXT,
    breaking_changes BOOLEAN DEFAULT false,
    migration_guide TEXT,
    
    -- Audit
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    created_by UUID,
    
    -- Constraints
    CONSTRAINT uk_template_version UNIQUE(template_id, version_number)
);

-- ==============================================================================
-- ORGANIZATION TOOL USAGE TABLE
-- ==============================================================================

-- Track usage of organization tools for analytics and optimization
CREATE TABLE IF NOT EXISTS mcp.organization_tool_usage (
    id UUID PRIMARY KEY DEFAULT uuid_generate_v4(),
    organization_tool_id UUID NOT NULL REFERENCES mcp.organization_tools(id) ON DELETE CASCADE,
    
    -- Usage details
    operation_name VARCHAR(255) NOT NULL,
    execution_count INTEGER NOT NULL DEFAULT 1,
    success_count INTEGER NOT NULL DEFAULT 0,
    error_count INTEGER NOT NULL DEFAULT 0,
    
    -- Performance metrics
    avg_response_time_ms INTEGER,
    min_response_time_ms INTEGER,
    max_response_time_ms INTEGER,
    
    -- Time window
    period_start TIMESTAMP WITH TIME ZONE NOT NULL,
    period_end TIMESTAMP WITH TIME ZONE NOT NULL,
    
    -- Metadata
    error_types JSONB,
    
    -- Constraints
    CONSTRAINT uk_tool_usage_period UNIQUE(organization_tool_id, operation_name, period_start)
);

-- ==============================================================================
-- COMBINED VIEW FOR MCP ACCESS
-- ==============================================================================

-- View that combines dynamic and templated tools for unified MCP access
CREATE OR REPLACE VIEW mcp.all_available_tools AS
-- Dynamic tools (existing)
SELECT 
    tc.id,
    tc.tool_name,
    tc.display_name,
    'dynamic' as source_type,
    tc.tenant_id,
    tc.tenant_id as organization_id,
    tc.base_url,
    tc.config,
    tc.status,
    tc.is_active,
    tc.health_status,
    tc.last_health_check,
    tc.created_at,
    tc.updated_at
FROM mcp.tool_configurations tc
WHERE tc.status = 'active' AND tc.is_active = true

UNION ALL

-- Template-based organization tools
SELECT 
    ot.id,
    ot.instance_name as tool_name,
    COALESCE(ot.display_name, tt.display_name) as display_name,
    'template' as source_type,
    ot.tenant_id,
    ot.organization_id,
    tt.default_config->>'baseUrl' as base_url,
    COALESCE(ot.instance_config, tt.default_config) as config,
    ot.status,
    ot.is_active,
    ot.health_status,
    ot.last_health_check,
    ot.created_at,
    ot.updated_at
FROM mcp.organization_tools ot
JOIN mcp.tool_templates tt ON ot.template_id = tt.id
WHERE ot.status = 'active' AND ot.is_active = true AND tt.is_active = true;

-- ==============================================================================
-- INDEXES
-- ==============================================================================

-- Tool templates indexes
CREATE INDEX idx_tool_templates_provider ON mcp.tool_templates(provider_name);
CREATE INDEX idx_tool_templates_category ON mcp.tool_templates(category) WHERE is_active = true;
CREATE INDEX idx_tool_templates_public ON mcp.tool_templates(is_public) WHERE is_active = true;
CREATE INDEX idx_tool_templates_tags ON mcp.tool_templates USING gin(tags);

-- Organization tools indexes
CREATE INDEX idx_organization_tools_org ON mcp.organization_tools(organization_id);
CREATE INDEX idx_organization_tools_tenant ON mcp.organization_tools(tenant_id);
CREATE INDEX idx_organization_tools_template ON mcp.organization_tools(template_id);
CREATE INDEX idx_organization_tools_status ON mcp.organization_tools(status) WHERE is_active = true;
CREATE INDEX idx_organization_tools_health ON mcp.organization_tools(last_health_check) WHERE is_active = true;

-- Template versions indexes
CREATE INDEX idx_tool_template_versions_template ON mcp.tool_template_versions(template_id);
CREATE INDEX idx_tool_template_versions_created ON mcp.tool_template_versions(created_at DESC);

-- Usage tracking indexes
CREATE INDEX idx_organization_tool_usage_tool ON mcp.organization_tool_usage(organization_tool_id);
CREATE INDEX idx_organization_tool_usage_period ON mcp.organization_tool_usage(period_start DESC);
CREATE INDEX idx_organization_tool_usage_operation ON mcp.organization_tool_usage(operation_name);

-- ==============================================================================
-- TRIGGERS
-- ==============================================================================

-- Update timestamp triggers
CREATE TRIGGER update_tool_templates_updated_at 
    BEFORE UPDATE ON mcp.tool_templates
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_organization_tools_updated_at 
    BEFORE UPDATE ON mcp.organization_tools
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- ==============================================================================
-- FUNCTIONS
-- ==============================================================================

-- Function to create an organization tool from a template
CREATE OR REPLACE FUNCTION mcp.create_organization_tool(
    p_organization_id UUID,
    p_tenant_id UUID,
    p_template_name VARCHAR(100),
    p_instance_name VARCHAR(255),
    p_credentials JSONB,
    p_config JSONB DEFAULT '{}'
) RETURNS UUID AS $$
DECLARE
    v_template_id UUID;
    v_tool_id UUID;
BEGIN
    -- Find the template
    SELECT id INTO v_template_id
    FROM mcp.tool_templates
    WHERE provider_name = p_template_name
        AND is_active = true
        AND is_deprecated = false
    ORDER BY provider_version DESC
    LIMIT 1;
    
    IF v_template_id IS NULL THEN
        RAISE EXCEPTION 'Template % not found or not active', p_template_name;
    END IF;
    
    -- Create the organization tool
    INSERT INTO mcp.organization_tools (
        organization_id,
        tenant_id,
        template_id,
        instance_name,
        instance_config,
        status
    ) VALUES (
        p_organization_id,
        p_tenant_id,
        v_template_id,
        p_instance_name,
        p_config,
        'provisioning'
    ) RETURNING id INTO v_tool_id;
    
    -- Note: Credential encryption should be handled by the application layer
    -- before calling this function
    
    RETURN v_tool_id;
END;
$$ LANGUAGE plpgsql SECURITY DEFINER;

-- Function to track tool usage
CREATE OR REPLACE FUNCTION mcp.track_tool_usage(
    p_tool_id UUID,
    p_operation VARCHAR(255),
    p_success BOOLEAN,
    p_response_time_ms INTEGER
) RETURNS VOID AS $$
BEGIN
    -- Update organization_tools usage counters
    UPDATE mcp.organization_tools
    SET 
        usage_count = usage_count + 1,
        error_count = CASE WHEN NOT p_success THEN error_count + 1 ELSE error_count END,
        last_used_at = CURRENT_TIMESTAMP
    WHERE id = p_tool_id;
    
    -- Insert or update usage tracking
    INSERT INTO mcp.organization_tool_usage (
        organization_tool_id,
        operation_name,
        execution_count,
        success_count,
        error_count,
        avg_response_time_ms,
        min_response_time_ms,
        max_response_time_ms,
        period_start,
        period_end
    ) VALUES (
        p_tool_id,
        p_operation,
        1,
        CASE WHEN p_success THEN 1 ELSE 0 END,
        CASE WHEN NOT p_success THEN 1 ELSE 0 END,
        p_response_time_ms,
        p_response_time_ms,
        p_response_time_ms,
        date_trunc('hour', CURRENT_TIMESTAMP),
        date_trunc('hour', CURRENT_TIMESTAMP) + INTERVAL '1 hour'
    )
    ON CONFLICT (organization_tool_id, operation_name, period_start)
    DO UPDATE SET
        execution_count = mcp.organization_tool_usage.execution_count + 1,
        success_count = mcp.organization_tool_usage.success_count + CASE WHEN p_success THEN 1 ELSE 0 END,
        error_count = mcp.organization_tool_usage.error_count + CASE WHEN NOT p_success THEN 1 ELSE 0 END,
        avg_response_time_ms = (
            (mcp.organization_tool_usage.avg_response_time_ms * mcp.organization_tool_usage.execution_count + p_response_time_ms) /
            (mcp.organization_tool_usage.execution_count + 1)
        ),
        min_response_time_ms = LEAST(mcp.organization_tool_usage.min_response_time_ms, p_response_time_ms),
        max_response_time_ms = GREATEST(mcp.organization_tool_usage.max_response_time_ms, p_response_time_ms);
END;
$$ LANGUAGE plpgsql;

-- ==============================================================================
-- ROW LEVEL SECURITY
-- ==============================================================================

-- Enable RLS on new tables
ALTER TABLE mcp.tool_templates ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.organization_tools ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.tool_template_versions ENABLE ROW LEVEL SECURITY;
ALTER TABLE mcp.organization_tool_usage ENABLE ROW LEVEL SECURITY;

-- Tool templates are public read, admin write
CREATE POLICY public_read_tool_templates ON mcp.tool_templates
    FOR SELECT USING (is_public = true);

CREATE POLICY admin_all_tool_templates ON mcp.tool_templates
    FOR ALL USING (mcp.current_tenant_id() IS NOT NULL); -- TODO: Add admin check

-- Organization tools are tenant-isolated
CREATE POLICY tenant_isolation_organization_tools ON mcp.organization_tools
    USING (tenant_id = mcp.current_tenant_id());

-- Template versions follow template permissions
CREATE POLICY template_versions_access ON mcp.tool_template_versions
    FOR SELECT USING (
        template_id IN (
            SELECT id FROM mcp.tool_templates WHERE is_public = true
        )
    );

-- Usage tracking follows organization tool permissions
CREATE POLICY usage_tracking_access ON mcp.organization_tool_usage
    FOR ALL USING (
        organization_tool_id IN (
            SELECT id FROM mcp.organization_tools 
            WHERE tenant_id = mcp.current_tenant_id()
        )
    );

-- ==============================================================================
-- INITIAL DATA - Standard Tool Templates
-- ==============================================================================

-- Insert GitHub template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    tags,
    documentation_url
) VALUES (
    'github',
    '1.0.0',
    'GitHub',
    'GitHub integration for repositories, issues, pull requests, and more',
    'version_control',
    '{"baseUrl": "https://api.github.com", "authType": "bearer", "rateLimits": {"requestsPerHour": 5000}}'::jsonb,
    '[
        {"name": "repositories", "displayName": "Repositories", "operations": ["list", "get", "create", "update", "delete"]},
        {"name": "issues", "displayName": "Issues", "operations": ["list", "get", "create", "update", "close"]},
        {"name": "pull_requests", "displayName": "Pull Requests", "operations": ["list", "get", "create", "update", "merge"]}
    ]'::jsonb,
    ARRAY['token'],
    ARRAY['git', 'vcs', 'collaboration', 'code'],
    'https://docs.github.com/rest'
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert GitLab template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    tags,
    documentation_url
) VALUES (
    'gitlab',
    '1.0.0',
    'GitLab',
    'GitLab integration for projects, issues, merge requests, and CI/CD',
    'version_control',
    '{"baseUrl": "https://gitlab.com/api/v4", "authType": "bearer", "rateLimits": {"requestsPerMinute": 600}}'::jsonb,
    '[
        {"name": "projects", "displayName": "Projects", "operations": ["list", "get", "create", "update", "delete"]},
        {"name": "issues", "displayName": "Issues", "operations": ["list", "get", "create", "update", "close"]},
        {"name": "merge_requests", "displayName": "Merge Requests", "operations": ["list", "get", "create", "update", "merge"]}
    ]'::jsonb,
    ARRAY['token'],
    ARRAY['git', 'vcs', 'ci-cd', 'devops'],
    'https://docs.gitlab.com/ee/api/'
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert Jira template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    tags,
    documentation_url
) VALUES (
    'jira',
    '1.0.0',
    'Jira',
    'Atlassian Jira integration for issue tracking and project management',
    'issue_tracking',
    '{"authType": "basic", "rateLimits": {"requestsPerMinute": 100}}'::jsonb,
    '[
        {"name": "issues", "displayName": "Issues", "operations": ["list", "get", "create", "update", "transition"]},
        {"name": "projects", "displayName": "Projects", "operations": ["list", "get"]},
        {"name": "boards", "displayName": "Boards", "operations": ["list", "get"]}
    ]'::jsonb,
    ARRAY['email', 'api_token'],
    ARRAY['agile', 'scrum', 'kanban', 'project-management'],
    'https://developer.atlassian.com/cloud/jira/platform/rest/v3/'
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert Harness template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    optional_credentials,
    tags,
    documentation_url,
    features
) VALUES (
    'harness',
    '1.0.0',
    'Harness Platform',
    'Harness Platform integration for CI/CD, Feature Flags, Cloud Cost Management, and Security Testing',
    'ci_cd',
    '{
        "baseUrl": "https://app.harness.io",
        "authType": "api_key",
        "rateLimits": {"requestsPerMinute": 100, "requestsPerHour": 1000},
        "modules": ["ci", "cd", "ff", "ccm", "sto"]
    }'::jsonb,
    '[
        {"name": "ci", "displayName": "Continuous Integration", "operations": ["pipelines", "builds", "execute", "logs"]},
        {"name": "cd", "displayName": "Continuous Delivery", "operations": ["services", "environments", "deployments", "rollback"]},
        {"name": "ff", "displayName": "Feature Flags", "operations": ["flags", "targets", "segments", "toggle"]},
        {"name": "ccm", "displayName": "Cloud Cost Management", "operations": ["perspectives", "budgets", "anomalies", "recommendations"]},
        {"name": "sto", "displayName": "Security Testing", "operations": ["scans", "targets", "baselines", "exemptions"]},
        {"name": "platform", "displayName": "Platform", "operations": ["projects", "organizations", "secrets", "connectors", "delegates"]}
    ]'::jsonb,
    ARRAY['api_key', 'account_id'],
    ARRAY['org_identifier', 'project_identifier'],
    ARRAY['harness', 'ci-cd', 'devops', 'pipelines', 'feature-flags', 'cost-management', 'security'],
    'https://developer.harness.io/docs',
    '{
        "supportsWebhooks": true,
        "supportsPagination": true,
        "supportsRateLimit": true,
        "supportsBatchOps": true,
        "supportsAsync": true,
        "supportsSearch": true,
        "supportsFiltering": true,
        "supportsSorting": true,
        "supportsGraphQL": true
    }'::jsonb
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert Confluence template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    tags,
    documentation_url,
    features
) VALUES (
    'confluence',
    '1.0.0',
    'Confluence Cloud',
    'Atlassian Confluence Cloud integration for documentation, knowledge management, and collaboration',
    'documentation',
    '{
        "authType": "basic",
        "rateLimits": {"requestsPerMinute": 100, "requestsPerHour": 5000},
        "apiVersion": "v2",
        "requiredScopes": ["read:confluence-content.all", "write:confluence-content.all"]
    }'::jsonb,
    '[
        {"name": "content", "displayName": "Content Management", "operations": ["list", "get", "create", "update", "delete", "search", "versions"]},
        {"name": "space", "displayName": "Spaces", "operations": ["list", "get", "create", "update", "delete", "permissions"]},
        {"name": "attachment", "displayName": "Attachments", "operations": ["list", "get", "create", "update", "delete", "download"]},
        {"name": "comment", "displayName": "Comments", "operations": ["list", "get", "create", "update", "delete"]},
        {"name": "label", "displayName": "Labels", "operations": ["list", "add", "remove", "search"]},
        {"name": "permission", "displayName": "Permissions", "operations": ["check", "list", "add", "remove"]},
        {"name": "template", "displayName": "Templates", "operations": ["list", "get", "create", "update", "delete"]}
    ]'::jsonb,
    ARRAY['email', 'api_token'],
    ARRAY['confluence', 'wiki', 'documentation', 'knowledge-base', 'collaboration', 'atlassian'],
    'https://developer.atlassian.com/cloud/confluence/rest/v2/',
    '{
        "supportsWebhooks": true,
        "supportsPagination": true,
        "supportsRateLimit": true,
        "supportsBatchOps": false,
        "supportsAsync": false,
        "supportsSearch": true,
        "supportsFiltering": true,
        "supportsSorting": true,
        "supportsCQL": true,
        "supportsVersioning": true
    }'::jsonb
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert Artifactory template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    optional_credentials,
    tags,
    documentation_url,
    features
) VALUES (
    'artifactory',
    '1.0.0',
    'JFrog Artifactory',
    'JFrog Artifactory integration for artifact management, build promotion, and dependency management',
    'artifact_management',
    '{
        "baseUrl": "https://artifactory.example.com/artifactory",
        "authType": "api_key",
        "rateLimits": {"requestsPerMinute": 1000, "requestsPerHour": 10000},
        "apiVersion": "7"
    }'::jsonb,
    '[
        {"name": "repositories", "displayName": "Repositories", "operations": ["list", "get", "create", "update", "delete", "replicate"]},
        {"name": "artifacts", "displayName": "Artifacts", "operations": ["search", "download", "upload", "delete", "properties", "copy", "move"]},
        {"name": "builds", "displayName": "Builds", "operations": ["list", "get", "create", "promote", "distribute", "delete"]},
        {"name": "packages", "displayName": "Packages", "operations": ["search", "get", "version", "dependencies", "scan"]},
        {"name": "security", "displayName": "Security", "operations": ["xray_scan", "vulnerabilities", "licenses", "policies"]},
        {"name": "replication", "displayName": "Replication", "operations": ["configure", "status", "execute"]},
        {"name": "permissions", "displayName": "Permissions", "operations": ["list", "get", "create", "update", "delete"]}
    ]'::jsonb,
    ARRAY['api_key'],
    ARRAY['username', 'password', 'token'],
    ARRAY['artifactory', 'jfrog', 'artifacts', 'packages', 'docker', 'maven', 'npm', 'nuget', 'pypi', 'helm'],
    'https://www.jfrog.com/confluence/display/JFROG/Artifactory+REST+API',
    '{
        "supportsWebhooks": true,
        "supportsPagination": true,
        "supportsRateLimit": true,
        "supportsBatchOps": true,
        "supportsAsync": true,
        "supportsSearch": true,
        "supportsFiltering": true,
        "supportsSorting": true,
        "supportsAQL": true,
        "supportsXray": true
    }'::jsonb
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

-- Insert Nexus template
INSERT INTO mcp.tool_templates (
    provider_name,
    provider_version,
    display_name,
    description,
    category,
    default_config,
    operation_groups,
    required_credentials,
    optional_credentials,
    tags,
    documentation_url,
    features
) VALUES (
    'nexus',
    '1.0.0',
    'Sonatype Nexus',
    'Sonatype Nexus Repository Manager integration for artifact storage, component management, and security scanning',
    'artifact_management',
    '{
        "baseUrl": "https://nexus.example.com",
        "authType": "api_key",
        "rateLimits": {"requestsPerMinute": 500, "requestsPerHour": 5000},
        "apiVersion": "3"
    }'::jsonb,
    '[
        {"name": "repositories", "displayName": "Repositories", "operations": ["list", "get", "create", "update", "delete", "browse"]},
        {"name": "components", "displayName": "Components", "operations": ["search", "get", "upload", "delete", "list"]},
        {"name": "assets", "displayName": "Assets", "operations": ["list", "get", "download", "upload", "delete"]},
        {"name": "formats", "displayName": "Format Specific", "operations": ["maven", "npm", "docker", "nuget", "pypi", "raw", "rubygems", "helm", "apt", "yum"]},
        {"name": "search", "displayName": "Search", "operations": ["components", "assets", "advanced"]},
        {"name": "security", "displayName": "Security", "operations": ["users", "roles", "privileges", "realms", "ldap", "certificates"]},
        {"name": "blobstores", "displayName": "Blob Stores", "operations": ["list", "get", "create", "update", "delete", "quota"]},
        {"name": "cleanup", "displayName": "Cleanup Policies", "operations": ["list", "get", "create", "update", "delete", "preview"]},
        {"name": "tasks", "displayName": "Tasks", "operations": ["list", "get", "create", "run", "stop", "delete"]},
        {"name": "lifecycle", "displayName": "Lifecycle", "operations": ["status", "health", "metrics", "support"]}
    ]'::jsonb,
    ARRAY['api_key'],
    ARRAY['username', 'password'],
    ARRAY['nexus', 'sonatype', 'artifacts', 'maven', 'npm', 'docker', 'nuget', 'pypi', 'repository-manager'],
    'https://help.sonatype.com/repomanager3/rest-and-integration-api',
    '{
        "supportsWebhooks": true,
        "supportsPagination": true,
        "supportsRateLimit": true,
        "supportsBatchOps": true,
        "supportsAsync": true,
        "supportsSearch": true,
        "supportsFiltering": true,
        "supportsSorting": true,
        "supportsCleanup": true,
        "supportsProxying": true
    }'::jsonb
) ON CONFLICT (provider_name, provider_version) DO NOTHING;

COMMIT;