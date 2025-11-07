package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
)

// SearchToolProvider provides search and discovery tools
type SearchToolProvider struct {
	registry       *tools.Registry
	toolsetManager *tools.ToolsetManager
}

// NewSearchToolProvider creates a new search tool provider
func NewSearchToolProvider(registry *tools.Registry, toolsetManager *tools.ToolsetManager) *SearchToolProvider {
	return &SearchToolProvider{
		registry:       registry,
		toolsetManager: toolsetManager,
	}
}

// GetDefinitions returns the search tool definitions
func (p *SearchToolProvider) GetDefinitions() []tools.ToolDefinition {
	return []tools.ToolDefinition{
		{
			Name:        "devmesh_search_tools",
			Description: "Search for relevant tools by keyword, category, or capability. Use this to discover tools before using them, reducing context consumption by loading only what you need.",
			Category:    string(tools.CategoryGeneral),
			Tags:        []string{"search", "discovery", "read"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"keyword": map[string]interface{}{
						"type":        "string",
						"description": "Keyword to search for in tool names and descriptions (e.g., 'github', 'workflow', 'issue')",
					},
					"category": map[string]interface{}{
						"type":        "string",
						"description": "Filter by category (repository, issues, pull_requests, ci_cd, workflow, task_management, etc.)",
					},
					"tags": map[string]interface{}{
						"type":        "array",
						"description": "Filter by capability tags (read, write, update, delete, list, search, etc.)",
						"items": map[string]interface{}{
							"type": "string",
						},
					},
					"detail_level": map[string]interface{}{
						"type":        "string",
						"description": "Level of detail to return: 'name' (just names), 'description' (name + description), 'full' (complete schema)",
						"enum":        []string{"name", "description", "full"},
						"default":     "description",
					},
					"max_results": map[string]interface{}{
						"type":        "integer",
						"description": "Maximum number of results to return (default: 20)",
						"default":     20,
						"minimum":     1,
						"maximum":     100,
					},
					"fuzzy_match": map[string]interface{}{
						"type":        "boolean",
						"description": "Enable fuzzy matching for keywords (default: true)",
						"default":     true,
					},
				},
			},
			Handler: p.handleSearchTools,
		},
		{
			Name:        "devmesh_list_toolsets",
			Description: "List available toolsets (functional groups of tools). Use this to discover what toolsets are available before enabling them.",
			Category:    string(tools.CategoryGeneral),
			Tags:        []string{"list", "discovery", "read"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"provider": map[string]interface{}{
						"type":        "string",
						"description": "Filter by provider (e.g., 'github', 'harness', 'devmesh')",
					},
					"enabled_only": map[string]interface{}{
						"type":        "boolean",
						"description": "Only return enabled toolsets (default: false)",
						"default":     false,
					},
				},
			},
			Handler: p.handleListToolsets,
		},
		{
			Name:        "devmesh_get_toolset",
			Description: "Get detailed information about a specific toolset, including all its tools. Use this after discovering a toolset with devmesh_list_toolsets.",
			Category:    string(tools.CategoryGeneral),
			Tags:        []string{"read", "discovery"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"required": []string{"name"},
				"properties": map[string]interface{}{
					"name": map[string]interface{}{
						"type":        "string",
						"description": "Toolset name (e.g., 'github_repos', 'github_issues')",
					},
					"detail_level": map[string]interface{}{
						"type":        "string",
						"description": "Level of detail for tools: 'name', 'description', or 'full'",
						"enum":        []string{"name", "description", "full"},
						"default":     "description",
					},
				},
			},
			Handler: p.handleGetToolset,
		},
	}
}

// handleSearchTools searches for tools based on criteria
func (p *SearchToolProvider) handleSearchTools(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Keyword     string   `json:"keyword"`
		Category    string   `json:"category"`
		Tags        []string `json:"tags"`
		DetailLevel string   `json:"detail_level"`
		MaxResults  int      `json:"max_results"`
		FuzzyMatch  bool     `json:"fuzzy_match"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Set defaults
	if params.DetailLevel == "" {
		params.DetailLevel = "description"
	}
	if params.MaxResults == 0 {
		params.MaxResults = 20
	}

	// Convert detail level
	detailLevel := mcptools.DetailLevel(params.DetailLevel)

	// Build search options
	searchOpts := tools.SearchOptions{
		Keyword:          params.Keyword,
		Category:         params.Category,
		Tags:             params.Tags,
		MaxResults:       params.MaxResults,
		EnableFuzzyMatch: params.FuzzyMatch,
	}

	// Perform search
	results := p.registry.Search(searchOpts)

	// Format results based on detail level
	return p.formatSearchResults(results, detailLevel), nil
}

// handleListToolsets lists available toolsets
func (p *SearchToolProvider) handleListToolsets(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Provider    string `json:"provider"`
		EnabledOnly bool   `json:"enabled_only"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	var toolsets []mcptools.Toolset
	if params.EnabledOnly {
		toolsets = p.toolsetManager.ListEnabledToolsets()
	} else if params.Provider != "" {
		toolsets = p.toolsetManager.ListToolsetsByProvider(params.Provider)
	} else {
		toolsets = p.toolsetManager.ListToolsets()
	}

	// Format for display
	result := make([]map[string]interface{}, len(toolsets))
	for i, ts := range toolsets {
		result[i] = map[string]interface{}{
			"name":         ts.Name,
			"display_name": ts.DisplayName,
			"description":  ts.Description,
			"category":     ts.Category,
			"icon":         ts.Icon,
			"enabled":      ts.Enabled,
			"provider":     ts.Provider,
			"tool_count":   len(ts.Tools),
			"tags":         ts.Tags,
		}
	}

	return map[string]interface{}{
		"toolsets": result,
		"count":    len(result),
	}, nil
}

// handleGetToolset gets detailed information about a specific toolset
func (p *SearchToolProvider) handleGetToolset(ctx context.Context, args json.RawMessage) (interface{}, error) {
	var params struct {
		Name        string `json:"name"`
		DetailLevel string `json:"detail_level"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Name == "" {
		return nil, fmt.Errorf("name parameter is required")
	}

	// Set default detail level
	if params.DetailLevel == "" {
		params.DetailLevel = "description"
	}
	detailLevel := mcptools.DetailLevel(params.DetailLevel)

	// Get toolset
	toolset, exists := p.toolsetManager.GetToolset(params.Name)
	if !exists {
		return nil, fmt.Errorf("toolset not found: %s", params.Name)
	}

	// Generate tool definitions with requested detail level
	toolDefs, err := p.toolsetManager.GenerateToolDefinitions(params.Name, detailLevel, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to generate tool definitions: %w", err)
	}

	// Format tools based on detail level
	formattedTools := p.formatMCPToolDefinitions(toolDefs, detailLevel)

	return map[string]interface{}{
		"name":         toolset.Name,
		"display_name": toolset.DisplayName,
		"description":  toolset.Description,
		"category":     toolset.Category,
		"icon":         toolset.Icon,
		"enabled":      toolset.Enabled,
		"provider":     toolset.Provider,
		"version":      toolset.Version,
		"tags":         toolset.Tags,
		"tools":        formattedTools,
		"tool_count":   len(formattedTools),
	}, nil
}

// formatSearchResults formats search results based on detail level
func (p *SearchToolProvider) formatSearchResults(results []tools.SearchResult, detailLevel mcptools.DetailLevel) interface{} {
	formatted := make([]map[string]interface{}, len(results))

	for i, result := range results {
		tool := result.Tool
		item := map[string]interface{}{
			"relevance_score": result.RelevanceScore,
			"match_details":   result.MatchDetails,
		}

		switch detailLevel {
		case mcptools.DetailLevelName:
			item["name"] = tool.Name

		case mcptools.DetailLevelDescription:
			item["name"] = tool.Name
			item["description"] = tool.Description
			item["category"] = tool.Category
			item["tags"] = tool.Tags

		case mcptools.DetailLevelFull:
			item["name"] = tool.Name
			item["description"] = tool.Description
			item["category"] = tool.Category
			item["tags"] = tool.Tags
			item["input_schema"] = tool.InputSchema
			item["prerequisites"] = tool.Prerequisites
			item["commonly_used_with"] = tool.CommonlyUsedWith
			item["next_steps"] = tool.NextSteps
		}

		formatted[i] = item
	}

	return map[string]interface{}{
		"results": formatted,
		"count":   len(formatted),
		"detail_level": string(detailLevel),
	}
}

// formatMCPToolDefinitions formats MCP tool definitions based on detail level
func (p *SearchToolProvider) formatMCPToolDefinitions(toolDefs []mcptools.ToolDefinition, detailLevel mcptools.DetailLevel) []map[string]interface{} {
	formatted := make([]map[string]interface{}, len(toolDefs))

	for i, tool := range toolDefs {
		switch detailLevel {
		case mcptools.DetailLevelName:
			formatted[i] = map[string]interface{}{
				"name": tool.Name,
			}

		case mcptools.DetailLevelDescription:
			formatted[i] = map[string]interface{}{
				"name":        tool.Name,
				"description": tool.Description,
				"category":    tool.Category,
				"tags":        tool.Tags,
			}

		case mcptools.DetailLevelFull:
			formatted[i] = map[string]interface{}{
				"name":               tool.Name,
				"description":        tool.Description,
				"category":           tool.Category,
				"tags":               tool.Tags,
				"input_schema":       tool.InputSchema,
				"prerequisites":      tool.Prerequisites,
				"commonly_used_with": tool.CommonlyUsedWith,
				"next_steps":         tool.NextSteps,
				"alternatives":       tool.Alternatives,
				"io_compatibility":   tool.IOCompatibility,
			}
		}
	}

	return formatted
}

// Utility function to build a hierarchical tool tree for filesystem-like navigation
func (p *SearchToolProvider) BuildToolTree() map[string]interface{} {
	toolsets := p.toolsetManager.ListToolsets()

	tree := make(map[string]interface{})

	for _, ts := range toolsets {
		provider := ts.Provider
		if provider == "" {
			provider = "devmesh"
		}

		if tree[provider] == nil {
			tree[provider] = make(map[string]interface{})
		}

		providerMap := tree[provider].(map[string]interface{})
		providerMap[ts.Name] = map[string]interface{}{
			"display_name": ts.DisplayName,
			"description":  ts.Description,
			"enabled":      ts.Enabled,
			"tool_count":   len(ts.Tools),
		}
	}

	return tree
}

// GetToolSuggestions provides intelligent tool suggestions
func (p *SearchToolProvider) GetToolSuggestions(partial string) []string {
	results := p.registry.GetToolSuggestions(partial, 10)

	suggestions := make([]string, len(results))
	for i, result := range results {
		suggestions[i] = result.Tool.Name
	}

	return suggestions
}

// ExplainTool provides a detailed explanation of what a tool does and how to use it
func (p *SearchToolProvider) ExplainTool(toolName string) (map[string]interface{}, error) {
	tool, exists := p.registry.Get(toolName)
	if !exists {
		return nil, fmt.Errorf("tool not found: %s", toolName)
	}

	// Build comprehensive explanation
	explanation := map[string]interface{}{
		"name":        tool.Name,
		"description": tool.Description,
		"category":    tool.Category,
		"tags":        tool.Tags,
		"schema":      tool.InputSchema,
	}

	// Add usage examples based on category
	examples := p.buildUsageExamples(tool)
	if len(examples) > 0 {
		explanation["examples"] = examples
	}

	// Add workflow suggestions
	if len(tool.CommonlyUsedWith) > 0 {
		explanation["commonly_used_with"] = tool.CommonlyUsedWith
	}

	if len(tool.NextSteps) > 0 {
		explanation["suggested_next_steps"] = tool.NextSteps
	}

	return explanation, nil
}

// buildUsageExamples builds usage examples for a tool
func (p *SearchToolProvider) buildUsageExamples(tool tools.ToolDefinition) []string {
	examples := make([]string, 0)

	// Generate examples based on tool name patterns
	name := tool.Name

	if strings.Contains(name, "list") {
		examples = append(examples, "Use this to get an overview of available items")
	}

	if strings.Contains(name, "create") {
		examples = append(examples, "Use this to add new items to the system")
	}

	if strings.Contains(name, "update") {
		examples = append(examples, "Use this to modify existing items")
	}

	if strings.Contains(name, "get") {
		examples = append(examples, "Use this to retrieve detailed information about a specific item")
	}

	if strings.Contains(name, "search") {
		examples = append(examples, "Use this to find items matching specific criteria")
	}

	return examples
}
