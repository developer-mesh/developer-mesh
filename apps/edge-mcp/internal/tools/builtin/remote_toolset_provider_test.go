package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockCoreClient is a mock implementation of Core Client
type MockCoreClient struct {
	toolsets []core.ToolsetInfo
	tools    []tools.ToolDefinition
	err      error
}

func (m *MockCoreClient) FetchRemoteToolsets(ctx context.Context) ([]core.ToolsetInfo, error) {
	return m.toolsets, m.err
}

func (m *MockCoreClient) FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error) {
	return m.tools, m.err
}

func TestNewRemoteToolsetProvider(t *testing.T) {
	mockClient := &MockCoreClient{}
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
	logger := observability.NewStandardLogger("test")

	provider := NewRemoteToolsetProvider(mockClient, manager, logger)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.coreClient)
	assert.NotNil(t, provider.toolsetManager)
	assert.NotNil(t, provider.logger)
}

func TestRegisterRemoteToolsets(t *testing.T) {
	tests := []struct {
		name             string
		toolsets         []core.ToolsetInfo
		expectedToolsets int
		expectError      bool
	}{
		{
			name: "successful registration of multiple toolsets",
			toolsets: []core.ToolsetInfo{
				{
					Name:        "github_repos",
					DisplayName: "GitHub Repositories",
					Description: "Manage GitHub repositories",
					Category:    "repository",
					Provider:    "github",
					ToolCount:   5,
					Tools:       []string{"github_repos_list", "github_repos_get", "github_repos_create"},
					Tags:        []string{"github", "repos"},
					Enabled:     true,
				},
				{
					Name:        "github_issues",
					DisplayName: "GitHub Issues",
					Description: "Manage GitHub issues",
					Category:    "issues",
					Provider:    "github",
					ToolCount:   4,
					Tools:       []string{"github_issues_list", "github_issues_get"},
					Tags:        []string{"github", "issues"},
					Enabled:     true,
				},
			},
			expectedToolsets: 2,
			expectError:      false,
		},
		{
			name:             "empty toolset list",
			toolsets:         []core.ToolsetInfo{},
			expectedToolsets: 0,
			expectError:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := &MockCoreClient{
				toolsets: tt.toolsets,
			}
			registry := tools.NewRegistry()
			manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
			logger := observability.NewStandardLogger("test")

			provider := NewRemoteToolsetProvider(mockClient, manager, logger)

			err := provider.RegisterRemoteToolsets(context.Background())

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)

				// Verify toolsets were registered
				allToolsets := manager.ListToolsets()
				remoteToolsets := 0
				for _, ts := range allToolsets {
					// Count non-default toolsets (default ones are devmesh_context, devmesh_workflow, devmesh_task)
					if ts.Provider != "devmesh" {
						remoteToolsets++
					}
				}
				assert.Equal(t, tt.expectedToolsets, remoteToolsets)
			}
		})
	}
}

func TestRegisterRemoteToolsets_Error(t *testing.T) {
	mockClient := &MockCoreClient{
		err: assert.AnError,
	}
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
	logger := observability.NewStandardLogger("test")

	provider := NewRemoteToolsetProvider(mockClient, manager, logger)

	err := provider.RegisterRemoteToolsets(context.Background())

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to fetch remote toolsets")
}

func TestMatchesToolset(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		toolsetName string
		expected    bool
	}{
		{
			name:        "exact prefix match with underscore",
			toolName:    "github_repos_list",
			toolsetName: "github_repos",
			expected:    true,
		},
		{
			name:        "exact prefix match with hyphen",
			toolName:    "github-repos-list",
			toolsetName: "github-repos",
			expected:    true,
		},
		{
			name:        "no match - different resource",
			toolName:    "github_issues_list",
			toolsetName: "github_repos",
			expected:    false,
		},
		{
			name:        "no match - tool name too short",
			toolName:    "github_repos",
			toolsetName: "github_repos",
			expected:    false,
		},
		{
			name:        "no match - tool name shorter than toolset",
			toolName:    "github",
			toolsetName: "github_repos",
			expected:    false,
		},
		{
			name:        "no separator after toolset name",
			toolName:    "github_reposfoo",
			toolsetName: "github_repos",
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesToolset(tt.toolName, tt.toolsetName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestExtractAction(t *testing.T) {
	tests := []struct {
		name        string
		toolName    string
		toolsetName string
		expected    string
	}{
		{
			name:        "extract list action",
			toolName:    "github_repos_list",
			toolsetName: "github_repos",
			expected:    "list",
		},
		{
			name:        "extract get action",
			toolName:    "github_repos_get",
			toolsetName: "github_repos",
			expected:    "get",
		},
		{
			name:        "extract compound action",
			toolName:    "github_repos_create_from_template",
			toolsetName: "github_repos",
			expected:    "create_from_template",
		},
		{
			name:        "no match returns empty",
			toolName:    "github_issues_list",
			toolsetName: "github_repos",
			expected:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractAction(tt.toolName, tt.toolsetName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		slice    []string
		value    string
		expected bool
	}{
		{
			name:     "value exists",
			slice:    []string{"foo", "bar", "baz"},
			value:    "bar",
			expected: true,
		},
		{
			name:     "value does not exist",
			slice:    []string{"foo", "bar", "baz"},
			value:    "qux",
			expected: false,
		},
		{
			name:     "empty slice",
			slice:    []string{},
			value:    "foo",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.slice, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToolGenerator(t *testing.T) {
	// Setup
	mockTools := []tools.ToolDefinition{
		{
			Name:        "github_repos_list",
			Description: "List repositories",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{"type": "string"},
				},
			},
			Handler:  func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
			Category: "repository",
			Tags:     []string{"read", "list"},
		},
		{
			Name:        "github_repos_get",
			Description: "Get a repository",
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{"type": "string"},
					"repo":  map[string]interface{}{"type": "string"},
				},
			},
			Handler:  func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
			Category: "repository",
			Tags:     []string{"read", "get"},
		},
		{
			Name:        "github_issues_list",
			Description: "List issues",
			InputSchema: map[string]interface{}{},
			Handler:     func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
			Category:    "issues",
			Tags:        []string{"read", "list"},
		},
	}

	mockClient := &MockCoreClient{
		tools: mockTools,
	}
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
	logger := observability.NewStandardLogger("test")

	provider := NewRemoteToolsetProvider(mockClient, manager, logger)

	// Create a generator for github_repos toolset
	generator := provider.createToolGenerator("github_repos")

	tests := []struct {
		name              string
		detailLevel       mcptools.DetailLevel
		actions           []string
		expectedToolCount int
		validateFn        func(t *testing.T, tools []mcptools.ToolDefinition)
	}{
		{
			name:              "full detail level - all tools",
			detailLevel:       mcptools.DetailLevelFull,
			actions:           nil,
			expectedToolCount: 2, // Only github_repos_* tools
			validateFn: func(t *testing.T, tools []mcptools.ToolDefinition) {
				// Should have full schemas
				for _, tool := range tools {
					assert.NotEmpty(t, tool.Name)
					assert.NotEmpty(t, tool.Description)
					assert.NotNil(t, tool.InputSchema)
				}
			},
		},
		{
			name:              "description detail level",
			detailLevel:       mcptools.DetailLevelDescription,
			actions:           nil,
			expectedToolCount: 2,
			validateFn: func(t *testing.T, tools []mcptools.ToolDefinition) {
				// Should have name and description, but no schema
				for _, tool := range tools {
					assert.NotEmpty(t, tool.Name)
					assert.NotEmpty(t, tool.Description)
					assert.Nil(t, tool.InputSchema)
				}
			},
		},
		{
			name:              "name detail level",
			detailLevel:       mcptools.DetailLevelName,
			actions:           nil,
			expectedToolCount: 2,
			validateFn: func(t *testing.T, tools []mcptools.ToolDefinition) {
				// Should have only name
				for _, tool := range tools {
					assert.NotEmpty(t, tool.Name)
					assert.Empty(t, tool.Description)
					assert.Nil(t, tool.InputSchema)
				}
			},
		},
		{
			name:              "filter by action - list only",
			detailLevel:       mcptools.DetailLevelFull,
			actions:           []string{"list"},
			expectedToolCount: 1,
			validateFn: func(t *testing.T, tools []mcptools.ToolDefinition) {
				require.Equal(t, 1, len(tools))
				assert.Equal(t, "github_repos_list", tools[0].Name)
			},
		},
		{
			name:              "filter by action - get only",
			detailLevel:       mcptools.DetailLevelFull,
			actions:           []string{"get"},
			expectedToolCount: 1,
			validateFn: func(t *testing.T, tools []mcptools.ToolDefinition) {
				require.Equal(t, 1, len(tools))
				assert.Equal(t, "github_repos_get", tools[0].Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tools, err := generator(tt.detailLevel, tt.actions)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedToolCount, len(tools))

			if tt.validateFn != nil {
				tt.validateFn(t, tools)
			}
		})
	}
}

func TestToolGenerator_Error(t *testing.T) {
	mockClient := &MockCoreClient{
		err: assert.AnError,
	}
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
	logger := observability.NewStandardLogger("test")

	provider := NewRemoteToolsetProvider(mockClient, manager, logger)
	generator := provider.createToolGenerator("github_repos")

	tools, err := generator(mcptools.DetailLevelFull, nil)

	assert.Error(t, err)
	assert.Nil(t, tools)
	assert.Contains(t, err.Error(), "failed to fetch tools for toolset")
}

func TestWrapToolHandler(t *testing.T) {
	// Create a test handler
	testHandler := func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		var params map[string]interface{}
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, err
		}
		return map[string]interface{}{
			"result": "success",
			"params": params,
		}, nil
	}

	// Wrap it
	wrappedHandler := wrapToolHandler(testHandler)

	// Test the wrapped handler
	testArgs := map[string]interface{}{
		"foo": "bar",
		"baz": 123,
	}
	argsJSON, err := json.Marshal(testArgs)
	require.NoError(t, err)

	result, err := wrappedHandler(context.Background(), argsJSON)

	assert.NoError(t, err)
	assert.NotNil(t, result)

	resultMap := result.(map[string]interface{})
	assert.Equal(t, "success", resultMap["result"])
	assert.NotNil(t, resultMap["params"])
}
