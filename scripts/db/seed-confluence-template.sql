-- Seed Confluence tool template for organizations to use
-- This creates a platform-wide template that organizations can instantiate

-- Insert Confluence provider template
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
    'confluence',
    'Confluence Cloud',
    'Atlassian Confluence Cloud integration for documentation, knowledge management, and collaboration',
    'documentation',
    ARRAY[
        -- Content Operations
        'content/list', 'content/get', 'content/create', 'content/update', 'content/delete',
        'content/search', 'content/children', 'content/descendants', 'content/versions', 'content/restore',
        
        -- Space Operations
        'space/list', 'space/get', 'space/create', 'space/update', 'space/delete',
        'space/content', 'space/permissions',
        
        -- Attachment Operations
        'attachment/list', 'attachment/get', 'attachment/create', 'attachment/update', 
        'attachment/delete', 'attachment/download',
        
        -- Comment Operations
        'comment/list', 'comment/get', 'comment/create', 'comment/update', 'comment/delete',
        
        -- Label Operations
        'label/list', 'label/add', 'label/remove', 'label/search',
        
        -- User and Group Operations
        'user/list', 'user/get', 'user/current', 'user/groups', 'user/watch', 'user/unwatch',
        'group/list', 'group/get', 'group/members',
        
        -- Permission Operations
        'permission/check', 'permission/list', 'permission/add', 'permission/remove',
        
        -- Template Operations
        'template/list', 'template/get', 'template/create', 'template/update', 'template/delete',
        
        -- Settings and Audit Operations
        'settings/theme', 'settings/update-theme', 'settings/lookandfeel',
        'audit/list', 'audit/create', 'audit/retention', 'audit/set-retention'
    ],
    jsonb_build_object(
        'base_url', 'https://{domain}.atlassian.net/wiki/rest/api',
        'auth_type', 'basic',
        'headers', jsonb_build_object(
            'Accept', 'application/json',
            'Content-Type', 'application/json'
        ),
        'rate_limits', jsonb_build_object(
            'requests_per_minute', 100,
            'requests_per_hour', 5000
        ),
        'api_version', 'v2',
        'required_scopes', jsonb_build_array(
            'read:confluence-content.all',
            'write:confluence-content.all'
        )
    ),
    ARRAY['email', 'api_token'],
    jsonb_build_object(
        'tool_definitions', jsonb_build_array(
            jsonb_build_object(
                'name', 'confluence_content',
                'description', 'Manage Confluence pages and blog posts',
                'parameters', jsonb_build_object(
                    'spaceKey', 'Space key',
                    'contentId', 'Content ID',
                    'title', 'Page/blog title',
                    'type', 'Content type (page, blogpost)',
                    'body', 'Content body in storage format',
                    'action', 'Operation: list, get, create, update, delete, search'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_space',
                'description', 'Manage Confluence spaces',
                'parameters', jsonb_build_object(
                    'spaceKey', 'Unique space key',
                    'name', 'Space name',
                    'description', 'Space description',
                    'action', 'Operation: list, get, create, update, delete'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_search',
                'description', 'Search Confluence using CQL',
                'parameters', jsonb_build_object(
                    'cql', 'Confluence Query Language string',
                    'limit', 'Maximum results',
                    'expand', 'Fields to expand'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_attachment',
                'description', 'Manage file attachments',
                'parameters', jsonb_build_object(
                    'contentId', 'Parent content ID',
                    'attachmentId', 'Attachment ID',
                    'file', 'File to upload',
                    'comment', 'Version comment',
                    'action', 'Operation: list, create, update, delete, download'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_comment',
                'description', 'Manage page comments',
                'parameters', jsonb_build_object(
                    'contentId', 'Parent content ID',
                    'commentId', 'Comment ID',
                    'body', 'Comment body',
                    'inline', 'Inline comment properties',
                    'action', 'Operation: list, get, create, update, delete'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_label',
                'description', 'Manage content labels',
                'parameters', jsonb_build_object(
                    'contentId', 'Content ID',
                    'labels', 'Array of label names',
                    'labelName', 'Single label name',
                    'action', 'Operation: list, add, remove, search'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_user',
                'description', 'Manage users and permissions',
                'parameters', jsonb_build_object(
                    'accountId', 'User account ID',
                    'username', 'Username (deprecated)',
                    'contentId', 'Content to watch/unwatch',
                    'action', 'Operation: list, get, current, watch, unwatch'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_permission',
                'description', 'Manage content restrictions',
                'parameters', jsonb_build_object(
                    'contentId', 'Content ID',
                    'subject', 'User or group',
                    'operation', 'Permission operation',
                    'restrictions', 'Restriction details',
                    'action', 'Operation: check, list, add, remove'
                )
            ),
            jsonb_build_object(
                'name', 'confluence_template',
                'description', 'Manage page templates',
                'parameters', jsonb_build_object(
                    'templateId', 'Template ID',
                    'name', 'Template name',
                    'templateType', 'Type of template',
                    'body', 'Template body',
                    'action', 'Operation: list, get, create, update, delete'
                )
            )
        ),
        'semantic_tags', jsonb_build_array(
            'confluence', 'wiki', 'documentation', 'knowledge-base',
            'collaboration', 'atlassian', 'content-management',
            'pages', 'blogs', 'spaces', 'cql'
        ),
        'capabilities', jsonb_build_object(
            'pagination', true,
            'cql_search', true,
            'versioning', true,
            'attachments', true,
            'comments', true,
            'labels', true,
            'permissions', true,
            'templates', true,
            'rate_limited', true
        ),
        'cql_examples', jsonb_build_array(
            'type = page AND space = DEV',
            'text ~ "deployment" AND lastmodified > now("-7d")',
            'label = "important" AND type = page',
            'creator = currentUser() AND created > now("-30d")',
            'parent = 123456',
            'ancestor = 789012 AND type = page'
        )
    ),
    '1.0.0',
    true,
    NOW(),
    NOW()
) ON CONFLICT (provider_name) DO UPDATE SET
    supported_operations = EXCLUDED.supported_operations,
    default_config = EXCLUDED.default_config,
    ai_optimized_schema = EXCLUDED.ai_optimized_schema,
    updated_at = NOW(),
    is_active = true;

-- Output the template ID for reference
SELECT id, provider_name, display_name 
FROM mcp.tool_templates 
WHERE provider_name = 'confluence';