-- Seed GitHub tool template for organizations to use
-- This creates a platform-wide template that organizations can instantiate

-- Insert GitHub provider template
INSERT INTO mcp.tool_templates (
    id,
    provider_name,
    display_name,
    description,
    category,
    supported_operations,
    default_config,
    required_credentials,
    ai_optimized_schema,
    version,
    is_active,
    created_at,
    updated_at
) VALUES (
    gen_random_uuid(),
    'github',
    'GitHub',
    'GitHub integration for repository management, issues, pull requests, and CI/CD',
    'version_control',
    ARRAY[
        'repos/list', 'repos/get', 'repos/create', 'repos/update', 'repos/delete',
        'issues/list', 'issues/get', 'issues/create', 'issues/update', 'issues/close',
        'pulls/list', 'pulls/get', 'pulls/create', 'pulls/merge',
        'actions/list-workflows', 'actions/trigger-workflow'
    ],
    jsonb_build_object(
        'base_url', 'https://api.github.com',
        'auth_type', 'bearer',
        'headers', jsonb_build_object(
            'Accept', 'application/vnd.github.v3+json',
            'X-GitHub-Api-Version', '2022-11-28'
        ),
        'rate_limits', jsonb_build_object(
            'requests_per_minute', 60
        )
    ),
    ARRAY['token', 'personal_access_token'],
    jsonb_build_object(
        'tool_definitions', jsonb_build_array(
            jsonb_build_object(
                'name', 'github_repos',
                'description', 'Manage GitHub repositories',
                'parameters', jsonb_build_object(
                    'owner', 'Repository owner or organization',
                    'repo', 'Repository name',
                    'action', 'Operation: list, get, create, update, delete'
                )
            ),
            jsonb_build_object(
                'name', 'github_issues',
                'description', 'Manage GitHub issues',
                'parameters', jsonb_build_object(
                    'owner', 'Repository owner',
                    'repo', 'Repository name',
                    'issue_number', 'Issue number',
                    'action', 'Operation: list, get, create, update, close'
                )
            ),
            jsonb_build_object(
                'name', 'github_pulls',
                'description', 'Manage pull requests',
                'parameters', jsonb_build_object(
                    'owner', 'Repository owner',
                    'repo', 'Repository name',
                    'pull_number', 'PR number',
                    'action', 'Operation: list, get, create, merge'
                )
            )
        )
    ),
    '1.0.0',
    true,
    NOW(),
    NOW()
) ON CONFLICT (provider_name) DO UPDATE SET
    updated_at = NOW(),
    is_active = true;

-- Output the template ID for reference
SELECT id, provider_name, display_name 
FROM mcp.tool_templates 
WHERE provider_name = 'github';