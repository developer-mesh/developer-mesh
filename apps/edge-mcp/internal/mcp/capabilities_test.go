package mcp

import (
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockToolProvider provides mock tools for testing
type mockToolProvider struct {
	tools []tools.ToolDefinition
}

func (m *mockToolProvider) GetDefinitions() []tools.ToolDefinition {
	return m.tools
}

// createTestRegistry creates a registry with test tools
func createTestRegistry() *tools.Registry {
	registry := tools.NewRegistry()

	// Add agent tools
	agentProvider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{
				Name:        "agent_heartbeat",
				Description: "Send agent heartbeat",
				Category:    "agent",
				Tags:        []string{"write", "monitoring"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"agent_id": map[string]interface{}{"type": "string"},
						"status":   map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"agent_id"},
				},
			},
			{
				Name:        "agent_list",
				Description: "List all agents",
				Category:    "agent",
				Tags:        []string{"read", "batch"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"status": map[string]interface{}{"type": "string"},
						"limit":  map[string]interface{}{"type": "number"},
					},
				},
			},
		},
	}
	registry.Register(agentProvider)

	// Add workflow tools
	workflowProvider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{
				Name:        "workflow_create",
				Description: "Create a new workflow",
				Category:    "workflow",
				Tags:        []string{"write", "async"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"name":        map[string]interface{}{"type": "string"},
						"description": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"name"},
				},
			},
			{
				Name:        "workflow_execute",
				Description: "Execute a workflow",
				Category:    "workflow",
				Tags:        []string{"execute", "async", "idempotent"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"workflow_id": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"workflow_id"},
				},
			},
		},
	}
	registry.Register(workflowProvider)

	// Add GitHub tools (simulating remote tools)
	githubProvider := &mockToolProvider{
		tools: []tools.ToolDefinition{
			{
				Name:        "github_repos_list",
				Description: "List repositories",
				Category:    "repository",
				Tags:        []string{"read", "batch"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"owner": map[string]interface{}{"type": "string"},
					},
				},
			},
			{
				Name:        "github_issues_create",
				Description: "Create an issue",
				Category:    "issues",
				Tags:        []string{"write"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"owner": map[string]interface{}{"type": "string"},
						"repo":  map[string]interface{}{"type": "string"},
						"title": map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"owner", "repo", "title"},
				},
			},
			{
				Name:        "github_repos_delete",
				Description: "Delete a repository",
				Category:    "repository",
				Tags:        []string{"delete", "write"},
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"owner": map[string]interface{}{"type": "string"},
						"repo":  map[string]interface{}{"type": "string"},
					},
					"required": []interface{}{"owner", "repo"},
				},
			},
		},
	}
	registry.Register(githubProvider)

	return registry
}

func TestNewCapabilityManager(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.registry)
	assert.NotNil(t, manager.serviceCapabilities)

	// Should have built-in services initialized
	assert.Contains(t, manager.serviceCapabilities, "agent")
	assert.Contains(t, manager.serviceCapabilities, "workflow")
	assert.Contains(t, manager.serviceCapabilities, "task")
	assert.Contains(t, manager.serviceCapabilities, "context")
}

func TestCapabilityManager_BuiltinServices(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	tests := []struct {
		name        string
		serviceName string
		wantExists  bool
	}{
		{
			name:        "agent service exists",
			serviceName: "agent",
			wantExists:  true,
		},
		{
			name:        "workflow service exists",
			serviceName: "workflow",
			wantExists:  true,
		},
		{
			name:        "task service exists",
			serviceName: "task",
			wantExists:  true,
		},
		{
			name:        "context service exists",
			serviceName: "context",
			wantExists:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, exists := manager.GetServiceCapability(tt.serviceName)
			assert.Equal(t, tt.wantExists, exists)
			if tt.wantExists {
				assert.NotNil(t, svc)
				assert.Equal(t, tt.serviceName, svc.ServiceName)
				assert.Equal(t, "builtin", svc.Provider)
				assert.True(t, svc.AuthRequired)
				assert.Contains(t, svc.AuthTypes, "api_key")
			}
		})
	}
}

func TestCapabilityManager_RefreshFromRegistry(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	manager.RefreshFromRegistry()

	// Should have agent, workflow, and github services
	capabilities := manager.GetAllServiceCapabilities()
	assert.GreaterOrEqual(t, len(capabilities), 3)

	// Check service names
	serviceNames := make(map[string]bool)
	for _, svc := range capabilities {
		serviceNames[svc.ServiceName] = true
	}

	assert.True(t, serviceNames["agent"])
	assert.True(t, serviceNames["workflow"])
	assert.True(t, serviceNames["github"])
}

func TestCapabilityManager_GetServiceCapability(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	tests := []struct {
		name            string
		serviceName     string
		wantExists      bool
		checkOperations bool
		minOperations   int
	}{
		{
			name:            "get agent service",
			serviceName:     "agent",
			wantExists:      true,
			checkOperations: true,
			minOperations:   2,
		},
		{
			name:            "get workflow service",
			serviceName:     "workflow",
			wantExists:      true,
			checkOperations: true,
			minOperations:   2,
		},
		{
			name:            "get github service",
			serviceName:     "github",
			wantExists:      true,
			checkOperations: true,
			minOperations:   3,
		},
		{
			name:        "get non-existent service",
			serviceName: "nonexistent",
			wantExists:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, exists := manager.GetServiceCapability(tt.serviceName)
			assert.Equal(t, tt.wantExists, exists)

			if tt.wantExists {
				assert.NotNil(t, svc)
				assert.Equal(t, tt.serviceName, svc.ServiceName)
				assert.NotEmpty(t, svc.DisplayName)
				assert.NotEmpty(t, svc.Description)

				if tt.checkOperations {
					assert.GreaterOrEqual(t, len(svc.Operations), tt.minOperations)
				}
			}
		})
	}
}

func TestCapabilityManager_GetToolCapability(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	tests := []struct {
		name         string
		toolName     string
		wantExists   bool
		wantCategory string
		wantTags     []string
	}{
		{
			name:         "get agent_heartbeat",
			toolName:     "agent_heartbeat",
			wantExists:   true,
			wantCategory: "agent",
			wantTags:     []string{"write", "monitoring"},
		},
		{
			name:         "get workflow_execute",
			toolName:     "workflow_execute",
			wantExists:   true,
			wantCategory: "workflow",
			wantTags:     []string{"execute", "async", "idempotent"},
		},
		{
			name:         "get github_repos_list",
			toolName:     "github_repos_list",
			wantExists:   true,
			wantCategory: "repository",
			wantTags:     []string{"read", "batch"},
		},
		{
			name:       "get non-existent tool",
			toolName:   "nonexistent_tool",
			wantExists: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, exists := manager.GetToolCapability(tt.toolName)
			assert.Equal(t, tt.wantExists, exists)

			if tt.wantExists {
				assert.NotNil(t, op)
				assert.Equal(t, tt.toolName, op.Name)
				assert.Equal(t, tt.wantCategory, op.Category)
				assert.Equal(t, tt.wantTags, op.Tags)
				assert.NotNil(t, op.InputSchema)
			}
		})
	}
}

func TestCapabilityManager_GetRequiredPermissions(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	tests := []struct {
		name          string
		toolName      string
		wantErr       bool
		checkPerms    bool
		minPerms      int
		wantPermTypes map[string]bool
	}{
		{
			name:       "get permissions for read tool",
			toolName:   "agent_list",
			wantErr:    false,
			checkPerms: true,
			minPerms:   1,
			wantPermTypes: map[string]bool{
				"agent:read": true,
			},
		},
		{
			name:       "get permissions for write tool",
			toolName:   "agent_heartbeat",
			wantErr:    false,
			checkPerms: true,
			minPerms:   1,
			wantPermTypes: map[string]bool{
				"agent:write": true,
			},
		},
		{
			name:       "get permissions for delete tool",
			toolName:   "github_repos_delete",
			wantErr:    false,
			checkPerms: true,
			minPerms:   2, // Should have write and admin
			wantPermTypes: map[string]bool{
				"repository:write": true,
				"repository:admin": true,
			},
		},
		{
			name:     "get permissions for non-existent tool",
			toolName: "nonexistent_tool",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms, err := manager.GetRequiredPermissions(tt.toolName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)

			if tt.checkPerms {
				assert.GreaterOrEqual(t, len(perms), tt.minPerms)

				// Check specific permissions
				for _, perm := range perms {
					if tt.wantPermTypes != nil {
						// At least one permission should match
						found := false
						for permName := range tt.wantPermTypes {
							if perm.Name == permName {
								found = true
								break
							}
						}
						if !found && len(tt.wantPermTypes) > 0 {
							t.Logf("Permission %s not in expected types", perm.Name)
						}
					}
				}
			}
		})
	}
}

func TestCapabilityManager_GetToolFeatures(t *testing.T) {
	registry := createTestRegistry()
	manager := NewCapabilityManager(registry)

	tests := []struct {
		name         string
		toolName     string
		wantErr      bool
		wantFeatures []string
	}{
		{
			name:     "get features for async tool",
			toolName: "workflow_execute",
			wantErr:  false,
			wantFeatures: []string{
				"supports_async",
				"is_idempotent",
				"supports_passthrough_auth",
			},
		},
		{
			name:     "get features for batch tool",
			toolName: "agent_list",
			wantErr:  false,
			wantFeatures: []string{
				"supports_batching",
				"supports_passthrough_auth",
			},
		},
		{
			name:     "get features for non-existent tool",
			toolName: "nonexistent_tool",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features, err := manager.GetToolFeatures(tt.toolName)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, features)

			// Check that expected features are present
			for _, expectedFeature := range tt.wantFeatures {
				assert.Contains(t, features, expectedFeature,
					"Expected feature %s not found in %v", expectedFeature, features)
			}
		})
	}
}

func TestExtractServiceName(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		wantService string
	}{
		{
			name:        "agent tool",
			toolName:    "agent_heartbeat",
			wantService: "agent",
		},
		{
			name:        "workflow tool",
			toolName:    "workflow_execute",
			wantService: "workflow",
		},
		{
			name:        "github tool",
			toolName:    "github_repos_list",
			wantService: "github",
		},
		{
			name:        "harness tool",
			toolName:    "harness_pipelines_get",
			wantService: "harness",
		},
		{
			name:        "task tool",
			toolName:    "task_create",
			wantService: "task",
		},
		{
			name:        "context tool",
			toolName:    "context_update",
			wantService: "context",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := extractServiceName(tt.toolName)
			assert.Equal(t, tt.wantService, service)
		})
	}
}

func TestIsBuiltinService(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantBuiltin bool
	}{
		{
			name:        "agent is builtin",
			serviceName: "agent",
			wantBuiltin: true,
		},
		{
			name:        "workflow is builtin",
			serviceName: "workflow",
			wantBuiltin: true,
		},
		{
			name:        "task is builtin",
			serviceName: "task",
			wantBuiltin: true,
		},
		{
			name:        "context is builtin",
			serviceName: "context",
			wantBuiltin: true,
		},
		{
			name:        "github is not builtin",
			serviceName: "github",
			wantBuiltin: false,
		},
		{
			name:        "harness is not builtin",
			serviceName: "harness",
			wantBuiltin: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isBuiltin := isBuiltinService(tt.serviceName)
			assert.Equal(t, tt.wantBuiltin, isBuiltin)
		})
	}
}

func TestExtractParameters(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name         string
		schema       map[string]interface{}
		wantRequired []string
		wantOptional []string
	}{
		{
			name: "schema with required and optional params",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"required_param": map[string]interface{}{"type": "string"},
					"optional_param": map[string]interface{}{"type": "string"},
				},
				"required": []interface{}{"required_param"},
			},
			wantRequired: []string{"required_param"},
			wantOptional: []string{"optional_param"},
		},
		{
			name: "schema with only required params",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{"type": "string"},
					"param2": map[string]interface{}{"type": "number"},
				},
				"required": []interface{}{"param1", "param2"},
			},
			wantRequired: []string{"param1", "param2"},
			wantOptional: []string{},
		},
		{
			name: "schema with only optional params",
			schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param1": map[string]interface{}{"type": "string"},
					"param2": map[string]interface{}{"type": "number"},
				},
			},
			wantRequired: []string{},
			wantOptional: []string{"param1", "param2"},
		},
		{
			name:         "empty schema",
			schema:       map[string]interface{}{},
			wantRequired: []string{},
			wantOptional: []string{},
		},
		{
			name:         "nil schema",
			schema:       nil,
			wantRequired: []string{},
			wantOptional: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required, optional := manager.extractParameters(tt.schema)

			assert.ElementsMatch(t, tt.wantRequired, required)
			assert.ElementsMatch(t, tt.wantOptional, optional)
		})
	}
}

func TestExtractServiceFeatures(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name         string
		tools        []tools.ToolDefinition
		wantFeatures []string
	}{
		{
			name: "tools with read and write operations",
			tools: []tools.ToolDefinition{
				{
					Name: "tool1",
					Tags: []string{"read"},
				},
				{
					Name: "tool2",
					Tags: []string{"write"},
				},
			},
			wantFeatures: []string{"read_operations", "write_operations"},
		},
		{
			name: "tools with async and batch support",
			tools: []tools.ToolDefinition{
				{
					Name: "tool1",
					Tags: []string{"async", "batch"},
				},
			},
			wantFeatures: []string{"async_operations", "batch_operations"},
		},
		{
			name: "tools with webhook and streaming support",
			tools: []tools.ToolDefinition{
				{
					Name: "tool1",
					Tags: []string{"webhook", "streaming"},
				},
			},
			wantFeatures: []string{"webhook_support", "streaming_support"},
		},
		{
			name: "tools with mixed features",
			tools: []tools.ToolDefinition{
				{
					Name: "tool1",
					Tags: []string{"read", "batch"},
				},
				{
					Name: "tool2",
					Tags: []string{"write", "async"},
				},
				{
					Name: "tool3",
					Tags: []string{"delete"},
				},
			},
			wantFeatures: []string{"read_operations", "write_operations", "delete_operations", "batch_operations", "async_operations"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := manager.extractServiceFeatures(tt.tools)
			assert.ElementsMatch(t, tt.wantFeatures, features)
		})
	}
}

func TestExtractAuthTypes(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name          string
		serviceName   string
		wantAuthTypes []string
	}{
		{
			name:          "github auth types",
			serviceName:   "github",
			wantAuthTypes: []string{"api_key", "oauth", "personal_access_token"},
		},
		{
			name:          "harness auth types",
			serviceName:   "harness",
			wantAuthTypes: []string{"api_key", "bearer_token", "service_account"},
		},
		{
			name:          "unknown service defaults to api_key",
			serviceName:   "unknown",
			wantAuthTypes: []string{"api_key"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			authTypes := manager.extractAuthTypes(tt.serviceName)
			assert.ElementsMatch(t, tt.wantAuthTypes, authTypes)
		})
	}
}

func TestExtractAPIVersion(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name           string
		serviceName    string
		wantAPIVersion string
	}{
		{
			name:           "github api version",
			serviceName:    "github",
			wantAPIVersion: "v3/2022-11-28",
		},
		{
			name:           "harness api version",
			serviceName:    "harness",
			wantAPIVersion: "v1",
		},
		{
			name:           "unknown service defaults to v1",
			serviceName:    "unknown",
			wantAPIVersion: "v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			apiVersion := manager.extractAPIVersion(tt.serviceName)
			assert.Equal(t, tt.wantAPIVersion, apiVersion)
		})
	}
}

func TestFormatDisplayName(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantDisplay string
	}{
		{
			name:        "lowercase name",
			serviceName: "github",
			wantDisplay: "Github",
		},
		{
			name:        "already capitalized",
			serviceName: "Harness",
			wantDisplay: "Harness",
		},
		{
			name:        "empty string",
			serviceName: "",
			wantDisplay: "",
		},
		{
			name:        "single character",
			serviceName: "a",
			wantDisplay: "A",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			display := formatDisplayName(tt.serviceName)
			assert.Equal(t, tt.wantDisplay, display)
		})
	}
}

func TestExtractFeatures(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name         string
		tool         tools.ToolDefinition
		wantFeatures []string
	}{
		{
			name: "async and batch tool",
			tool: tools.ToolDefinition{
				Name: "test_tool",
				Tags: []string{"async", "batch"},
			},
			wantFeatures: []string{"supports_async", "supports_batching", "supports_passthrough_auth"},
		},
		{
			name: "streaming and webhook tool",
			tool: tools.ToolDefinition{
				Name: "test_tool",
				Tags: []string{"streaming", "webhook"},
			},
			wantFeatures: []string{"supports_streaming", "supports_webhooks", "supports_passthrough_auth"},
		},
		{
			name: "idempotent tool",
			tool: tools.ToolDefinition{
				Name: "test_tool",
				Tags: []string{"idempotent"},
			},
			wantFeatures: []string{"is_idempotent", "supports_passthrough_auth"},
		},
		{
			name: "tool with all features",
			tool: tools.ToolDefinition{
				Name: "test_tool",
				Tags: []string{"async", "batch", "streaming", "webhook", "cache", "retry", "idempotent"},
			},
			wantFeatures: []string{
				"supports_async",
				"supports_batching",
				"supports_streaming",
				"supports_webhooks",
				"supports_caching",
				"supports_retry",
				"is_idempotent",
				"supports_passthrough_auth",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			features := manager.extractFeatures(tt.tool)
			assert.ElementsMatch(t, tt.wantFeatures, features)
		})
	}
}

func TestExtractPermissions(t *testing.T) {
	manager := NewCapabilityManager(tools.NewRegistry())

	tests := []struct {
		name      string
		tool      tools.ToolDefinition
		wantCount int
		checkPerm func(*testing.T, []Permission)
	}{
		{
			name: "read permission",
			tool: tools.ToolDefinition{
				Name:     "test_tool",
				Category: "repository",
				Tags:     []string{"read"},
			},
			wantCount: 1,
			checkPerm: func(t *testing.T, perms []Permission) {
				assert.Equal(t, "repository:read", perms[0].Name)
				assert.True(t, perms[0].Required)
			},
		},
		{
			name: "write permission",
			tool: tools.ToolDefinition{
				Name:     "test_tool",
				Category: "issues",
				Tags:     []string{"write"},
			},
			wantCount: 1,
			checkPerm: func(t *testing.T, perms []Permission) {
				assert.Equal(t, "issues:write", perms[0].Name)
				assert.True(t, perms[0].Required)
			},
		},
		{
			name: "delete permission includes write and admin",
			tool: tools.ToolDefinition{
				Name:     "test_tool",
				Category: "repository",
				Tags:     []string{"delete"},
			},
			wantCount: 1,
			checkPerm: func(t *testing.T, perms []Permission) {
				// Should have admin permission for delete
				found := false
				for _, perm := range perms {
					if strings.Contains(perm.Name, "admin") {
						found = true
						break
					}
				}
				assert.True(t, found, "Expected admin permission for delete operation")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := manager.extractPermissions(tt.tool)
			assert.GreaterOrEqual(t, len(perms), tt.wantCount)
			if tt.checkPerm != nil {
				tt.checkPerm(t, perms)
			}
		})
	}
}
