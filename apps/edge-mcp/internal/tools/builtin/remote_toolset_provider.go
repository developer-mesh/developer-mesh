package builtin

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// CoreClient interface defines methods needed from Core Platform client
type CoreClient interface {
	FetchRemoteToolsets(ctx context.Context) ([]core.ToolsetInfo, error)
	FetchRemoteTools(ctx context.Context) ([]tools.ToolDefinition, error)
}

// RemoteToolsetProvider fetches and registers toolsets from Core Platform
type RemoteToolsetProvider struct {
	coreClient     CoreClient
	toolsetManager *tools.ToolsetManager
	logger         observability.Logger
}

// NewRemoteToolsetProvider creates a new remote toolset provider
func NewRemoteToolsetProvider(
	coreClient CoreClient,
	toolsetManager *tools.ToolsetManager,
	logger observability.Logger,
) *RemoteToolsetProvider {
	return &RemoteToolsetProvider{
		coreClient:     coreClient,
		toolsetManager: toolsetManager,
		logger:         logger,
	}
}

// RegisterRemoteToolsets fetches toolsets from Core Platform and registers them
func (p *RemoteToolsetProvider) RegisterRemoteToolsets(ctx context.Context) error {
	// Fetch toolsets from Core Platform
	toolsetInfos, err := p.coreClient.FetchRemoteToolsets(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch remote toolsets: %w", err)
	}

	p.logger.Info("Fetched remote toolsets from Core Platform", map[string]interface{}{
		"count": len(toolsetInfos),
	})

	// Register each toolset
	for _, info := range toolsetInfos {
		toolset := mcptools.Toolset{
			Name:        info.Name,
			DisplayName: info.DisplayName,
			Description: info.Description,
			Category:    info.Category,
			Provider:    info.Provider,
			Tools:       info.Tools,
			Tags:        info.Tags,
			Enabled:     false, // Start disabled, user can enable via devmesh_enable_toolsets
			Version:     "1.0.0",
		}

		// Create a tool generator for this toolset
		// The generator will fetch full tool details from Core Platform when needed
		toolset.ToolGenerator = p.createToolGenerator(info.Name)

		// Register the toolset
		if err := p.toolsetManager.RegisterToolset(toolset); err != nil {
			p.logger.Warn("Failed to register remote toolset", map[string]interface{}{
				"toolset": info.Name,
				"error":   err.Error(),
			})
			continue
		}

		p.logger.Debug("Registered remote toolset", map[string]interface{}{
			"name":       info.Name,
			"tool_count": info.ToolCount,
			"provider":   info.Provider,
		})
	}

	p.logger.Info("Completed remote toolset registration", map[string]interface{}{
		"registered": len(toolsetInfos),
	})

	return nil
}

// createToolGenerator creates a generator function for a specific toolset
// This generator will be called when tools from this toolset need to be loaded
func (p *RemoteToolsetProvider) createToolGenerator(toolsetName string) mcptools.ToolsetGenerator {
	return func(detailLevel mcptools.DetailLevel, actions []string) ([]mcptools.ToolDefinition, error) {
		// Fetch full tool details for this toolset from Core Platform
		// For now, we'll use the existing FetchRemoteTools which returns all tools
		// In a future optimization, we could create an endpoint to fetch tools for a specific toolset
		ctx := context.Background()
		allTools, err := p.coreClient.FetchRemoteTools(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to fetch tools for toolset %s: %w", toolsetName, err)
		}

		// Filter tools that belong to this toolset
		// Tool names follow pattern: {provider}_{resource}_{action}
		// Toolset names follow pattern: {provider}_{resource}
		var toolsetTools []mcptools.ToolDefinition
		for _, tool := range allTools {
			// Check if tool belongs to this toolset by name prefix
			// e.g., toolset "github_repos" matches tools "github_repos_list", "github_repos_get", etc.
			if matchesToolset(tool.Name, toolsetName) {
				// Create a wrapper handler to convert between tool handler types
				handler := wrapToolHandler(tool.Handler)

				// Convert tools.ToolDefinition to mcptools.ToolDefinition
				mcpTool := mcptools.ToolDefinition{
					Name:             tool.Name,
					Description:      tool.Description,
					InputSchema:      tool.InputSchema,
					Handler:          handler,
					Category:         tool.Category,
					Tags:             tool.Tags,
					Prerequisites:    tool.Prerequisites,
					CommonlyUsedWith: tool.CommonlyUsedWith,
					NextSteps:        tool.NextSteps,
					Alternatives:     tool.Alternatives,
					ConflictsWith:    tool.ConflictsWith,
				}

				// Apply detail level filtering
				// For "name" level, only include name
				if detailLevel == mcptools.DetailLevelName {
					mcpTool.Description = ""
					mcpTool.InputSchema = nil
				} else if detailLevel == mcptools.DetailLevelDescription {
					// Include name and description, but not full schema
					mcpTool.InputSchema = nil
				}
				// For "full" level, include everything (no filtering needed)

				// Filter by actions if specified
				if len(actions) > 0 {
					// Extract action from tool name
					// e.g., "github_repos_list" -> action is "list"
					toolAction := extractAction(tool.Name, toolsetName)
					if !contains(actions, toolAction) {
						continue
					}
				}

				toolsetTools = append(toolsetTools, mcpTool)
			}
		}

		p.logger.Debug("Generated tools for remote toolset", map[string]interface{}{
			"toolset":      toolsetName,
			"tool_count":   len(toolsetTools),
			"detail_level": string(detailLevel),
		})

		return toolsetTools, nil
	}
}

// matchesToolset checks if a tool name belongs to a given toolset
// Toolset: github_repos, Tool: github_repos_list -> match
// Toolset: github_repos, Tool: github_issues_list -> no match
func matchesToolset(toolName, toolsetName string) bool {
	// Simple prefix match for now
	// Could be enhanced with more sophisticated matching logic
	return len(toolName) > len(toolsetName) &&
		toolName[:len(toolsetName)] == toolsetName &&
		(toolName[len(toolsetName)] == '_' || toolName[len(toolsetName)] == '-')
}

// extractAction extracts the action part from a tool name
// e.g., "github_repos_list" with toolset "github_repos" -> "list"
func extractAction(toolName, toolsetName string) string {
	if !matchesToolset(toolName, toolsetName) {
		return ""
	}
	// Skip the toolset name and the separator
	remainder := toolName[len(toolsetName)+1:]
	return remainder
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
}

// wrapToolHandler wraps a tools.ToolHandler to mcptools.ToolHandler
// Both have the same signature but are different types
func wrapToolHandler(handler tools.ToolHandler) mcptools.ToolHandler {
	return func(ctx context.Context, args json.RawMessage) (interface{}, error) {
		return handler(ctx, args)
	}
}
