package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
)

// ToolsetManagementProvider provides tools for managing toolsets
type ToolsetManagementProvider struct {
	toolsetManager *tools.ToolsetManager
}

// NewToolsetManagementProvider creates a new toolset management provider
func NewToolsetManagementProvider(toolsetManager *tools.ToolsetManager) *ToolsetManagementProvider {
	return &ToolsetManagementProvider{
		toolsetManager: toolsetManager,
	}
}

// GetDefinitions returns the toolset management tool definitions
func (p *ToolsetManagementProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "devmesh_enable_toolsets",
			Description: "Enable one or more toolsets to make their tools available. Use 'default' to enable default toolsets, or 'all' to enable everything.",
			Category:    string(tools.CategoryConfiguration),
			Tags:        []string{"write", "configuration"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"required": []string{"toolsets"},
				"properties": map[string]interface{}{
					"toolsets": map[string]interface{}{
						"type":        "array",
						"description": "List of toolset names to enable (e.g., ['github_repos', 'github_issues']) or special values: 'default', 'all'",
						"items": map[string]interface{}{
							"type": "string",
						},
						"minItems": 1,
					},
				},
			},
			Handler: p.handleEnableToolsets,
		},
		{
			Name:        "devmesh_disable_toolsets",
			Description: "Disable one or more toolsets to remove their tools from availability. This helps reduce context size when you don't need certain functionality.",
			Category:    string(tools.CategoryConfiguration),
			Tags:        []string{"write", "configuration"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"required": []string{"toolsets"},
				"properties": map[string]interface{}{
					"toolsets": map[string]interface{}{
						"type":        "array",
						"description": "List of toolset names to disable",
						"items": map[string]interface{}{
							"type": "string",
						},
						"minItems": 1,
					},
				},
			},
			Handler: p.handleDisableToolsets,
		},
		{
			Name:        "devmesh_get_enabled_toolsets",
			Description: "Get a list of currently enabled toolsets. Use this to see what tools are currently available.",
			Category:    string(tools.CategoryConfiguration),
			Tags:        []string{"read", "configuration"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"include_tools": map[string]interface{}{
						"type":        "boolean",
						"description": "Include tool names for each toolset (default: false)",
						"default":     false,
					},
				},
			},
			Handler: p.handleGetEnabledToolsets,
		},
	}
}

// handleEnableToolsets enables one or more toolsets
func (p *ToolsetManagementProvider) handleEnableToolsets(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Toolsets []string `json:"toolsets"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if len(params.Toolsets) == 0 {
		return nil, fmt.Errorf("toolsets parameter is required and must not be empty")
	}

	// Handle special values
	toolsetsToEnable := p.resolveToolsetNames(params.Toolsets)

	// Track results
	enabled := make([]string, 0)
	failed := make(map[string]string)

	// Enable each toolset
	for _, name := range toolsetsToEnable {
		if err := p.toolsetManager.EnableToolset(ctx, name); err != nil {
			failed[name] = err.Error()
		} else {
			enabled = append(enabled, name)
		}
	}

	result := map[string]interface{}{
		"enabled": enabled,
		"count":   len(enabled),
	}

	if len(failed) > 0 {
		result["failed"] = failed
	}

	return result, nil
}

// handleDisableToolsets disables one or more toolsets
func (p *ToolsetManagementProvider) handleDisableToolsets(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Toolsets []string `json:"toolsets"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if len(params.Toolsets) == 0 {
		return nil, fmt.Errorf("toolsets parameter is required and must not be empty")
	}

	// Track results
	disabled := make([]string, 0)
	failed := make(map[string]string)

	// Disable each toolset
	for _, name := range params.Toolsets {
		if err := p.toolsetManager.DisableToolset(ctx, name); err != nil {
			failed[name] = err.Error()
		} else {
			disabled = append(disabled, name)
		}
	}

	result := map[string]interface{}{
		"disabled": disabled,
		"count":    len(disabled),
	}

	if len(failed) > 0 {
		result["failed"] = failed
	}

	return result, nil
}

// handleGetEnabledToolsets returns currently enabled toolsets
func (p *ToolsetManagementProvider) handleGetEnabledToolsets(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		IncludeTools bool `json:"include_tools"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	toolsets := p.toolsetManager.ListEnabledToolsets()

	// Format for display
	result := make([]map[string]interface{}, len(toolsets))
	for i, ts := range toolsets {
		item := map[string]interface{}{
			"name":         ts.Name,
			"display_name": ts.DisplayName,
			"description":  ts.Description,
			"provider":     ts.Provider,
			"category":     ts.Category,
			"tool_count":   len(ts.Tools),
		}

		if params.IncludeTools {
			item["tools"] = ts.Tools
		}

		result[i] = item
	}

	return map[string]interface{}{
		"toolsets": result,
		"count":    len(result),
	}, nil
}

// resolveToolsetNames resolves special toolset names like "default" and "all"
func (p *ToolsetManagementProvider) resolveToolsetNames(names []string) []string {
	resolved := make([]string, 0)

	for _, name := range names {
		switch name {
		case "all":
			// Add all toolsets
			allToolsets := p.toolsetManager.ListToolsets()
			for _, ts := range allToolsets {
				resolved = append(resolved, ts.Name)
			}

		case "default":
			// Add default toolsets
			resolved = append(resolved,
				"devmesh_context",
				"devmesh_workflow",
				"devmesh_task",
				"github_repos",
				"github_issues",
				"github_pulls",
			)

		default:
			// Regular toolset name
			resolved = append(resolved, name)
		}
	}

	return resolved
}
