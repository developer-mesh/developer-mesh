package tools

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewToolsetManager(t *testing.T) {
	registry := NewRegistry()
	manager := NewToolsetManager(registry)

	assert.NotNil(t, manager)
	assert.NotNil(t, manager.toolsets)
	assert.NotNil(t, manager.registry)
	assert.Equal(t, 3, len(manager.defaultToolsets), "should have 3 default toolsets")
}

func TestToolsetManager_RegisterToolset(t *testing.T) {
	tests := []struct {
		name        string
		toolset     mcptools.Toolset
		wantErr     bool
		errContains string
	}{
		{
			name: "valid toolset",
			toolset: mcptools.Toolset{
				Name:          "test_toolset",
				DisplayName:   "Test Toolset",
				Description:   "A test toolset",
				Category:      string(CategoryGeneral),
				Enabled:       false,
				Tools:         []string{"tool1", "tool2"},
				ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
					return []mcptools.ToolDefinition{}, nil
				},
			},
			wantErr: false,
		},
		{
			name: "empty name",
			toolset: mcptools.Toolset{
				DisplayName: "Test",
			},
			wantErr:     true,
			errContains: "name cannot be empty",
		},
		{
			name: "nil generator",
			toolset: mcptools.Toolset{
				Name:        "test",
				DisplayName: "Test",
			},
			wantErr:     true,
			errContains: "must have a tool generator",
		},
		{
			name: "default toolset auto-enabled",
			toolset: mcptools.Toolset{
				Name:          "devmesh_context",
				DisplayName:   "Context Management",
				Description:   "Context operations",
				Category:      string(CategoryGeneral),
				Enabled:       false, // Will be auto-enabled
				ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
					return []mcptools.ToolDefinition{}, nil
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			registry := NewRegistry()
			manager := NewToolsetManager(registry)

			err := manager.RegisterToolset(tt.toolset)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				assert.NoError(t, err)

				// Verify toolset was registered
				toolset, exists := manager.GetToolset(tt.toolset.Name)
				assert.True(t, exists)
				assert.Equal(t, tt.toolset.Name, toolset.Name)

				// Check if default toolsets are auto-enabled
				if tt.toolset.Name == "devmesh_context" || tt.toolset.Name == "devmesh_workflow" || tt.toolset.Name == "devmesh_task" {
					assert.True(t, toolset.Enabled, "default toolset should be auto-enabled")
				}
			}
		})
	}
}

func TestToolsetManager_EnableToolset(t *testing.T) {
	ctx := context.Background()

	// Create a test toolset with generator
	testToolset := mcptools.Toolset{
		Name:        "test_toolset",
		DisplayName: "Test Toolset",
		Description: "A test toolset",
		Category:    string(CategoryGeneral),
		Enabled:     false,
		Tools:       []string{"test_tool"},
		ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
			return []mcptools.ToolDefinition{
				{
					Name:        "test_tool",
					Description: "A test tool",
					InputSchema: map[string]interface{}{
						"type": "object",
					},
					Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
						return "success", nil
					},
					Category: string(CategoryGeneral),
				},
			}, nil
		},
	}

	t.Run("enable non-existent toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		err := manager.EnableToolset(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset not found")
	})

	t.Run("enable toolset successfully", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		// Register toolset
		err := manager.RegisterToolset(testToolset)
		require.NoError(t, err)

		// Enable it
		err = manager.EnableToolset(ctx, "test_toolset")
		assert.NoError(t, err)

		// Verify it's enabled
		toolset, exists := manager.GetToolset("test_toolset")
		assert.True(t, exists)
		assert.True(t, toolset.Enabled)

		// Verify tools were registered in registry
		tool, exists := registry.Get("test_tool")
		assert.True(t, exists)
		assert.Equal(t, "test_tool", tool.Name)
	})

	t.Run("enable already enabled toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		// Register and enable toolset
		err := manager.RegisterToolset(testToolset)
		require.NoError(t, err)
		err = manager.EnableToolset(ctx, "test_toolset")
		require.NoError(t, err)

		// Enable again - should be no-op
		err = manager.EnableToolset(ctx, "test_toolset")
		assert.NoError(t, err)
	})
}

func TestToolsetManager_DisableToolset(t *testing.T) {
	ctx := context.Background()

	testToolset := mcptools.Toolset{
		Name:        "test_toolset",
		DisplayName: "Test Toolset",
		Description: "A test toolset",
		Category:    string(CategoryGeneral),
		Enabled:     false,
		ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
			return []mcptools.ToolDefinition{}, nil
		},
	}

	t.Run("disable non-existent toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		err := manager.DisableToolset(ctx, "nonexistent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset not found")
	})

	t.Run("disable enabled toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		// Register and enable toolset
		err := manager.RegisterToolset(testToolset)
		require.NoError(t, err)
		err = manager.EnableToolset(ctx, "test_toolset")
		require.NoError(t, err)

		// Disable it
		err = manager.DisableToolset(ctx, "test_toolset")
		assert.NoError(t, err)

		// Verify it's disabled
		toolset, exists := manager.GetToolset("test_toolset")
		assert.True(t, exists)
		assert.False(t, toolset.Enabled)
	})

	t.Run("disable already disabled toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		// Register toolset (disabled by default)
		err := manager.RegisterToolset(testToolset)
		require.NoError(t, err)

		// Disable - should be no-op
		err = manager.DisableToolset(ctx, "test_toolset")
		assert.NoError(t, err)
	})
}

func TestToolsetManager_EnableToolsets(t *testing.T) {
	ctx := context.Background()

	createTestToolset := func(name string) mcptools.Toolset {
		return mcptools.Toolset{
			Name:        name,
			DisplayName: name,
			Description: "Test toolset",
			Category:    string(CategoryGeneral),
			ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
				return []mcptools.ToolDefinition{}, nil
			},
		}
	}

	t.Run("enable multiple toolsets", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		// Register multiple toolsets
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset1")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset2")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset3")))

		// Enable them all
		err := manager.EnableToolsets(ctx, []string{"toolset1", "toolset2", "toolset3"})
		assert.NoError(t, err)

		// Verify all are enabled
		enabled := manager.ListEnabledToolsets()
		assert.Equal(t, 3, len(enabled))
	})

	t.Run("fail on non-existent toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset1")))

		err := manager.EnableToolsets(ctx, []string{"toolset1", "nonexistent"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset not found")
	})
}

func TestToolsetManager_List_Operations(t *testing.T) {
	ctx := context.Background()

	createTestToolset := func(name, provider string) mcptools.Toolset {
		return mcptools.Toolset{
			Name:        name,
			DisplayName: name,
			Description: "Test toolset",
			Category:    string(CategoryGeneral),
			Provider:    provider,
			ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
				return []mcptools.ToolDefinition{}, nil
			},
		}
	}

	t.Run("list all toolsets", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset1", "github")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset2", "harness")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset3", "github")))

		all := manager.ListToolsets()
		assert.Equal(t, 3, len(all))
	})

	t.Run("list enabled toolsets only", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset1", "github")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset2", "harness")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset3", "github")))

		// Enable only one
		require.NoError(t, manager.EnableToolset(ctx, "toolset1"))

		enabled := manager.ListEnabledToolsets()
		assert.Equal(t, 1, len(enabled))
		assert.Equal(t, "toolset1", enabled[0].Name)
	})

	t.Run("list by provider", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset1", "github")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset2", "harness")))
		require.NoError(t, manager.RegisterToolset(createTestToolset("toolset3", "github")))

		githubToolsets := manager.ListToolsetsByProvider("github")
		assert.Equal(t, 2, len(githubToolsets))

		harnessToolsets := manager.ListToolsetsByProvider("harness")
		assert.Equal(t, 1, len(harnessToolsets))
	})
}

func TestToolsetManager_GenerateToolDefinitions(t *testing.T) {
	toolGenerator := func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
		tool := mcptools.ToolDefinition{
			Name:        "test_tool",
			Description: "A test tool",
			Category:    string(CategoryGeneral),
			Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return "success", nil
			},
		}

		// Return different schemas based on detail level
		switch detailLevel {
		case mcptools.DetailLevelName:
			// Minimal info
		case mcptools.DetailLevelDescription:
			// Add description
		case mcptools.DetailLevelFull:
			tool.InputSchema = map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"param": map[string]interface{}{
						"type": "string",
					},
				},
			}
		}

		return []mcptools.ToolDefinition{tool}, nil
	}

	testToolset := mcptools.Toolset{
		Name:          "test_toolset",
		DisplayName:   "Test Toolset",
		Description:   "A test toolset",
		Category:      string(CategoryGeneral),
		ToolGenerator: toolGenerator,
	}

	t.Run("generate with different detail levels", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		require.NoError(t, manager.RegisterToolset(testToolset))

		// Test each detail level
		detailLevels := []mcptools.DetailLevel{
			mcptools.DetailLevelName,
			mcptools.DetailLevelDescription,
			mcptools.DetailLevelFull,
		}

		for _, level := range detailLevels {
			tools, err := manager.GenerateToolDefinitions("test_toolset", level, nil)
			assert.NoError(t, err)
			assert.Equal(t, 1, len(tools))
			assert.Equal(t, "test_tool", tools[0].Name)

			// Full level should have input schema
			if level == mcptools.DetailLevelFull {
				assert.NotNil(t, tools[0].InputSchema)
			}
		}
	})

	t.Run("generate for non-existent toolset", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		_, err := manager.GenerateToolDefinitions("nonexistent", mcptools.DetailLevelFull, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "toolset not found")
	})
}

func TestToolsetManager_RegisterProvider(t *testing.T) {
	mockProvider := &mockToolsetProvider{
		toolsets: []mcptools.Toolset{
			{
				Name:        "provider_toolset1",
				DisplayName: "Provider Toolset 1",
				Description: "First toolset from provider",
				Category:    string(CategoryGeneral),
				ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
					return []mcptools.ToolDefinition{}, nil
				},
			},
			{
				Name:        "provider_toolset2",
				DisplayName: "Provider Toolset 2",
				Description: "Second toolset from provider",
				Category:    string(CategoryGeneral),
				ToolGenerator: func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
					return []mcptools.ToolDefinition{}, nil
				},
			},
		},
	}

	t.Run("register provider successfully", func(t *testing.T) {
		registry := NewRegistry()
		manager := NewToolsetManager(registry)

		err := manager.RegisterProvider(mockProvider)
		assert.NoError(t, err)

		// Verify both toolsets were registered
		all := manager.ListToolsets()
		assert.GreaterOrEqual(t, len(all), 2)

		_, exists1 := manager.GetToolset("provider_toolset1")
		_, exists2 := manager.GetToolset("provider_toolset2")
		assert.True(t, exists1)
		assert.True(t, exists2)
	})
}

// Mock provider for testing
type mockToolsetProvider struct {
	toolsets []mcptools.Toolset
}

func (m *mockToolsetProvider) GetToolsets() []mcptools.Toolset {
	return m.toolsets
}

func (m *mockToolsetProvider) GetToolsetByName(name string) (mcptools.Toolset, bool) {
	for _, ts := range m.toolsets {
		if ts.Name == name {
			return ts, true
		}
	}
	return mcptools.Toolset{}, false
}
