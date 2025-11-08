package builtin

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSearchToolProvider(t *testing.T) {
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())

	provider := NewSearchToolProvider(registry, manager)

	assert.NotNil(t, provider)
	assert.NotNil(t, provider.registry)
	assert.NotNil(t, provider.toolsetManager)
}

func TestSearchToolProvider_GetDefinitions(t *testing.T) {
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())
	provider := NewSearchToolProvider(registry, manager)

	definitions := provider.GetDefinitions()

	// Should have 3 tools: search_tools, list_toolsets, get_toolset
	assert.Equal(t, 3, len(definitions))

	toolNames := make([]string, len(definitions))
	for i, def := range definitions {
		toolNames[i] = def.Name
	}

	assert.Contains(t, toolNames, "devmesh_search_tools")
	assert.Contains(t, toolNames, "devmesh_list_toolsets")
	assert.Contains(t, toolNames, "devmesh_get_toolset")

	// Verify search_tools has correct schema
	var searchTool tools.ToolDefinition
	for _, def := range definitions {
		if def.Name == "devmesh_search_tools" {
			searchTool = def
			break
		}
	}

	assert.NotNil(t, searchTool.InputSchema)
	schema := searchTool.InputSchema
	props := schema["properties"].(map[string]interface{})
	assert.Contains(t, props, "keyword")
	assert.Contains(t, props, "category")
	assert.Contains(t, props, "tags")
	assert.Contains(t, props, "detail_level")
	assert.Contains(t, props, "max_results")
	assert.Contains(t, props, "fuzzy_match")
}

func TestSearchToolProvider_HandleSearchTools(t *testing.T) {
	// Setup
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())

	// Register some test tools
	registry.Register(&mockToolProvider{
		tools: []tools.ToolDefinition{
			{
				Name:        "github_repos",
				Description: "Manage GitHub repositories",
				Category:    string(tools.CategoryRepository),
				Tags:        []string{"github", "read", "write"},
				Handler:     func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
			},
			{
				Name:        "github_issues",
				Description: "Manage GitHub issues",
				Category:    string(tools.CategoryIssues),
				Tags:        []string{"github", "read", "write"},
				Handler:     func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
			},
		},
	})

	provider := NewSearchToolProvider(registry, manager)

	tests := []struct {
		name       string
		input      map[string]interface{}
		wantCount  int
		wantErr    bool
		validateFn func(t *testing.T, result interface{})
	}{
		{
			name: "search by keyword",
			input: map[string]interface{}{
				"keyword": "github",
			},
			wantCount: 2,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 2, resultMap["count"])
				results := resultMap["results"].([]map[string]interface{})
				assert.Equal(t, 2, len(results))
			},
		},
		{
			name: "search by category",
			input: map[string]interface{}{
				"category": string(tools.CategoryRepository),
			},
			wantCount: 1,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 1, resultMap["count"])
				results := resultMap["results"].([]map[string]interface{})
				assert.Equal(t, "github_repos", results[0]["name"])
			},
		},
		{
			name: "search by tags",
			input: map[string]interface{}{
				"tags": []string{"read"},
			},
			wantCount: 2,
			wantErr:   false,
		},
		{
			name: "detail level name",
			input: map[string]interface{}{
				"keyword":      "github",
				"detail_level": "name",
			},
			wantCount: 2,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				results := resultMap["results"].([]map[string]interface{})
				// Name level should only have name and relevance_score
				firstResult := results[0]
				assert.Contains(t, firstResult, "name")
				assert.Contains(t, firstResult, "relevance_score")
				// Should not have description at name level
				assert.NotContains(t, firstResult, "description")
			},
		},
		{
			name: "detail level description",
			input: map[string]interface{}{
				"keyword":      "github",
				"detail_level": "description",
			},
			wantCount: 2,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				results := resultMap["results"].([]map[string]interface{})
				firstResult := results[0]
				assert.Contains(t, firstResult, "name")
				assert.Contains(t, firstResult, "description")
				assert.Contains(t, firstResult, "category")
				assert.Contains(t, firstResult, "tags")
			},
		},
		{
			name: "detail level full",
			input: map[string]interface{}{
				"keyword":      "github",
				"detail_level": "full",
			},
			wantCount: 2,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				results := resultMap["results"].([]map[string]interface{})
				firstResult := results[0]
				assert.Contains(t, firstResult, "name")
				assert.Contains(t, firstResult, "description")
				assert.Contains(t, firstResult, "category")
				assert.Contains(t, firstResult, "tags")
				assert.Contains(t, firstResult, "input_schema")
			},
		},
		{
			name: "max results limit",
			input: map[string]interface{}{
				"keyword":     "github",
				"max_results": 1,
			},
			wantCount: 1,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 1, resultMap["count"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Marshal input to JSON
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			// Call handler
			result, err := provider.handleSearchTools(context.Background(), inputJSON)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestSearchToolProvider_HandleListToolsets(t *testing.T) {
	// Setup
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())

	// Register test toolsets
	testToolsets := []mcptools.Toolset{
		{
			Name:        "github_repos",
			DisplayName: "GitHub Repositories",
			Description: "Repository management",
			Category:    string(tools.CategoryRepository),
			Provider:    "github",
			Enabled:     false,
			Tools:       []string{"repos_get", "repos_list"},
			ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
				return []mcptools.ToolDefinition{}, nil
			},
		},
		{
			Name:        "github_issues",
			DisplayName: "GitHub Issues",
			Description: "Issue management",
			Category:    string(tools.CategoryIssues),
			Provider:    "github",
			Enabled:     true,
			Tools:       []string{"issues_get", "issues_list"},
			ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
				return []mcptools.ToolDefinition{}, nil
			},
		},
		{
			Name:        "harness_pipelines",
			DisplayName: "Harness Pipelines",
			Description: "Pipeline management",
			Category:    string(tools.CategoryCICD),
			Provider:    "harness",
			Enabled:     false,
			Tools:       []string{"pipelines_execute"},
			ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
				return []mcptools.ToolDefinition{}, nil
			},
		},
	}

	for _, ts := range testToolsets {
		require.NoError(t, manager.RegisterToolset(ts))
	}

	provider := NewSearchToolProvider(registry, manager)

	tests := []struct {
		name       string
		input      map[string]interface{}
		wantCount  int
		wantErr    bool
		validateFn func(t *testing.T, result interface{})
	}{
		{
			name:      "list all toolsets",
			input:     map[string]interface{}{},
			wantCount: 3,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 3, resultMap["count"])
				toolsets := resultMap["toolsets"].([]map[string]interface{})
				assert.Equal(t, 3, len(toolsets))
			},
		},
		{
			name: "filter by provider",
			input: map[string]interface{}{
				"provider": "github",
			},
			wantCount: 2,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 2, resultMap["count"])
				toolsets := resultMap["toolsets"].([]map[string]interface{})
				for _, ts := range toolsets {
					assert.Equal(t, "github", ts["provider"])
				}
			},
		},
		{
			name: "enabled only",
			input: map[string]interface{}{
				"enabled_only": true,
			},
			wantCount: 1,
			wantErr:   false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, 1, resultMap["count"])
				toolsets := resultMap["toolsets"].([]map[string]interface{})
				assert.Equal(t, "github_issues", toolsets[0]["name"])
				assert.Equal(t, true, toolsets[0]["enabled"])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			result, err := provider.handleListToolsets(context.Background(), inputJSON)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

func TestSearchToolProvider_HandleGetToolset(t *testing.T) {
	// Setup
	registry := tools.NewRegistry()
	manager := tools.NewToolsetManager(registry, observability.NewNoopLogger())

	// Register test toolset
	testToolset := mcptools.Toolset{
		Name:        "github_repos",
		DisplayName: "GitHub Repositories",
		Description: "Repository management",
		Category:    string(tools.CategoryRepository),
		Provider:    "github",
		Version:     "1.0.0",
		Tags:        []string{"github", "repository"},
		Tools:       []string{"repos_get", "repos_list"},
		ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
			tools := []mcptools.ToolDefinition{
				{
					Name:        "repos_get",
					Description: "Get repository details",
					Category:    string(tools.CategoryRepository),
					Handler:     func(ctx context.Context, args json.RawMessage) (interface{}, error) { return nil, nil },
				},
			}

			// Add schema only for full detail
			if detailLevel == mcptools.DetailLevelFull {
				tools[0].InputSchema = map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"owner": map[string]interface{}{"type": "string"},
						"repo":  map[string]interface{}{"type": "string"},
					},
				}
			}

			return tools, nil
		},
	}

	require.NoError(t, manager.RegisterToolset(testToolset))

	provider := NewSearchToolProvider(registry, manager)

	tests := []struct {
		name        string
		input       map[string]interface{}
		wantErr     bool
		errContains string
		validateFn  func(t *testing.T, result interface{})
	}{
		{
			name: "get toolset with full details",
			input: map[string]interface{}{
				"name":         "github_repos",
				"detail_level": "full",
			},
			wantErr: false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				assert.Equal(t, "github_repos", resultMap["name"])
				assert.Equal(t, "GitHub Repositories", resultMap["display_name"])
				assert.Equal(t, "Repository management", resultMap["description"])
				assert.Equal(t, "github", resultMap["provider"])
				assert.Equal(t, "1.0.0", resultMap["version"])

				toolsArray := resultMap["tools"].([]map[string]interface{})
				assert.Equal(t, 1, len(toolsArray))
				assert.Contains(t, toolsArray[0], "input_schema")
			},
		},
		{
			name: "get toolset with description level",
			input: map[string]interface{}{
				"name":         "github_repos",
				"detail_level": "description",
			},
			wantErr: false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				toolsArray := resultMap["tools"].([]map[string]interface{})
				assert.Equal(t, 1, len(toolsArray))
				assert.Contains(t, toolsArray[0], "name")
				assert.Contains(t, toolsArray[0], "description")
				// Should not have full schema at description level
				assert.NotContains(t, toolsArray[0], "input_schema")
			},
		},
		{
			name: "get toolset with name level",
			input: map[string]interface{}{
				"name":         "github_repos",
				"detail_level": "name",
			},
			wantErr: false,
			validateFn: func(t *testing.T, result interface{}) {
				resultMap := result.(map[string]interface{})
				toolsArray := resultMap["tools"].([]map[string]interface{})
				assert.Equal(t, 1, len(toolsArray))
				assert.Contains(t, toolsArray[0], "name")
				assert.NotContains(t, toolsArray[0], "description")
			},
		},
		{
			name: "missing name parameter",
			input: map[string]interface{}{
				"detail_level": "full",
			},
			wantErr:     true,
			errContains: "name parameter is required",
		},
		{
			name: "non-existent toolset",
			input: map[string]interface{}{
				"name": "nonexistent",
			},
			wantErr:     true,
			errContains: "toolset not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputJSON, err := json.Marshal(tt.input)
			require.NoError(t, err)

			result, err := provider.handleGetToolset(context.Background(), inputJSON)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotNil(t, result)

			if tt.validateFn != nil {
				tt.validateFn(t, result)
			}
		})
	}
}

// Mock tool provider for testing
type mockToolProvider struct {
	tools []tools.ToolDefinition
}

func (m *mockToolProvider) GetDefinitions() []tools.ToolDefinition {
	return m.tools
}
