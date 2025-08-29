-- Seed Harness tool template for organizations to use
-- This creates a platform-wide template that organizations can instantiate
-- Supports all Harness modules: CI, CD, FF, CCM, STO

-- Insert Harness provider template
INSERT INTO mcp.tool_templates (
    id,
    provider_name,
    provider_version,
    display_name,
    description,
    icon_url,
    category,
    operation_mappings,
    operation_groups,
    default_config,
    required_credentials,
    ai_definitions,
    documentation_url,
    api_documentation_url,
    tags,
    is_active,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'harness',
    '1.0.0',
    'Harness Platform',
    'Harness Platform integration for CI/CD, Feature Flags, Cloud Cost Management, and Security Testing',
    'https://www.harness.io/favicon.ico',
    'ci_cd',
    jsonb_build_object(
        -- CI Operations
        'ci/pipelines/list', jsonb_build_object('method', 'GET', 'path', '/ci/pipelines'),
        'ci/pipelines/get', jsonb_build_object('method', 'GET', 'path', '/ci/pipelines/{id}'),
        'ci/pipelines/create', jsonb_build_object('method', 'POST', 'path', '/ci/pipelines'),
        'ci/pipelines/execute', jsonb_build_object('method', 'POST', 'path', '/ci/pipelines/{id}/execute'),
        'ci/builds/list', jsonb_build_object('method', 'GET', 'path', '/ci/builds'),
        'ci/builds/get', jsonb_build_object('method', 'GET', 'path', '/ci/builds/{id}'),
        
        -- CD Operations
        'cd/services/list', jsonb_build_object('method', 'GET', 'path', '/cd/services'),
        'cd/services/get', jsonb_build_object('method', 'GET', 'path', '/cd/services/{id}'),
        'cd/services/create', jsonb_build_object('method', 'POST', 'path', '/cd/services'),
        'cd/environments/list', jsonb_build_object('method', 'GET', 'path', '/cd/environments'),
        'cd/environments/get', jsonb_build_object('method', 'GET', 'path', '/cd/environments/{id}'),
        'cd/deployments/create', jsonb_build_object('method', 'POST', 'path', '/cd/deployments'),
        
        -- Feature Flags Operations
        'ff/flags/list', jsonb_build_object('method', 'GET', 'path', '/cf/flags'),
        'ff/flags/get', jsonb_build_object('method', 'GET', 'path', '/cf/flags/{identifier}'),
        'ff/flags/create', jsonb_build_object('method', 'POST', 'path', '/cf/flags'),
        'ff/flags/toggle', jsonb_build_object('method', 'PATCH', 'path', '/cf/flags/{identifier}/toggle'),
        
        -- CCM Operations
        'ccm/perspectives/list', jsonb_build_object('method', 'GET', 'path', '/ccm/perspectives'),
        'ccm/budgets/list', jsonb_build_object('method', 'GET', 'path', '/ccm/budgets'),
        
        -- STO Operations
        'sto/scans/list', jsonb_build_object('method', 'GET', 'path', '/sto/scans'),
        'sto/scans/create', jsonb_build_object('method', 'POST', 'path', '/sto/scans'),
        
        -- Platform Operations
        'platform/secrets/list', jsonb_build_object('method', 'GET', 'path', '/v1/secrets'),
        'platform/connectors/list', jsonb_build_object('method', 'GET', 'path', '/v1/connectors')
    ),
    jsonb_build_array(
        jsonb_build_object('name', 'CI', 'operations', jsonb_build_array('ci/pipelines/list', 'ci/pipelines/get', 'ci/pipelines/create', 'ci/pipelines/execute', 'ci/builds/list', 'ci/builds/get')),
        jsonb_build_object('name', 'CD', 'operations', jsonb_build_array('cd/services/list', 'cd/services/get', 'cd/services/create', 'cd/environments/list', 'cd/environments/get', 'cd/deployments/create')),
        jsonb_build_object('name', 'Feature Flags', 'operations', jsonb_build_array('ff/flags/list', 'ff/flags/get', 'ff/flags/create', 'ff/flags/toggle')),
        jsonb_build_object('name', 'Cloud Cost', 'operations', jsonb_build_array('ccm/perspectives/list', 'ccm/budgets/list')),
        jsonb_build_object('name', 'Security', 'operations', jsonb_build_array('sto/scans/list', 'sto/scans/create')),
        jsonb_build_object('name', 'Platform', 'operations', jsonb_build_array('platform/secrets/list', 'platform/connectors/list'))
    ),
    jsonb_build_object(
        'base_url', 'https://app.harness.io',
        'auth_type', 'api_key',
        'headers', jsonb_build_object(
            'Content-Type', 'application/json',
            'Accept', 'application/json'
        ),
        'rate_limits', jsonb_build_object(
            'requests_per_minute', 100,
            'requests_per_hour', 1000
        ),
        'modules', jsonb_build_array('ci', 'cd', 'ff', 'ccm', 'sto'),
        'api_version', '1'
    ),
    ARRAY['api_key', 'account_id'],
    jsonb_build_object(
        'tool_definitions', jsonb_build_array(
            -- CI Module Tools
            jsonb_build_object(
                'name', 'harness_ci_pipelines',
                'description', 'Manage CI/CD pipelines in Harness',
                'category', 'ci',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'orgIdentifier', 'Organization identifier',
                    'projectIdentifier', 'Project identifier',
                    'pipelineIdentifier', 'Pipeline identifier',
                    'action', 'Operation: list, get, create, execute, status'
                )
            ),
            -- CD Module Tools
            jsonb_build_object(
                'name', 'harness_cd_services',
                'description', 'Manage services for deployment',
                'category', 'cd',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'serviceId', 'Service identifier',
                    'action', 'Operation: list, get, create, update'
                )
            ),
            -- Feature Flags Module Tools
            jsonb_build_object(
                'name', 'harness_ff_flags',
                'description', 'Manage feature flags',
                'category', 'ff',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'identifier', 'Flag identifier',
                    'action', 'Operation: list, get, create, update, toggle'
                )
            ),
            -- CCM Module Tools
            jsonb_build_object(
                'name', 'harness_ccm_perspectives',
                'description', 'Manage cost perspectives and views',
                'category', 'ccm',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'perspectiveId', 'Perspective identifier',
                    'action', 'Operation: list, get, create'
                )
            ),
            -- STO Module Tools
            jsonb_build_object(
                'name', 'harness_sto_scans',
                'description', 'Manage security scans and tests',
                'category', 'sto',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'scanId', 'Scan identifier',
                    'action', 'Operation: list, get, create, status'
                )
            ),
            -- Platform Tools
            jsonb_build_object(
                'name', 'harness_platform_secrets',
                'description', 'Manage platform secrets',
                'category', 'platform',
                'parameters', jsonb_build_object(
                    'accountIdentifier', 'Harness account ID',
                    'secretIdentifier', 'Secret identifier',
                    'action', 'Operation: list, create, update'
                )
            )
        ),
        'semantic_tags', jsonb_build_array(
            'harness', 'ci', 'cd', 'continuous-integration', 'continuous-delivery',
            'pipelines', 'deployments', 'feature-flags', 'cost-management',
            'security-testing', 'devops', 'automation', 'orchestration'
        )
    ),
    'https://developer.harness.io/docs',
    'https://apidocs.harness.io',
    ARRAY['harness', 'ci-cd', 'devops', 'pipelines', 'feature-flags', 'cost-management', 'security'],
    true,
    NOW(),
    NOW()
) ON CONFLICT (provider_name, provider_version) DO UPDATE SET
    operation_mappings = EXCLUDED.operation_mappings,
    operation_groups = EXCLUDED.operation_groups,
    default_config = EXCLUDED.default_config,
    ai_definitions = EXCLUDED.ai_definitions,
    updated_at = NOW(),
    is_active = true;

-- Output the template ID for reference
SELECT id, provider_name, display_name 
FROM mcp.tool_templates 
WHERE provider_name = 'harness';