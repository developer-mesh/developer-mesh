package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/mcptools"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"
)

// ToolsetService handles toolset generation and management
type ToolsetService struct {
	dynamicToolRepo pkgrepository.DynamicToolRepository
	logger          observability.Logger
}

// NewToolsetService creates a new toolset service
func NewToolsetService(
	dynamicToolRepo pkgrepository.DynamicToolRepository,
	logger observability.Logger,
) *ToolsetService {
	return &ToolsetService{
		dynamicToolRepo: dynamicToolRepo,
		logger:          logger,
	}
}

// ToolsetInfo represents a generated toolset with its tools
type ToolsetInfo struct {
	Toolset      mcptools.Toolset
	Tools        []*models.DynamicTool
	ActionCounts map[string]int
}

// AutoGenerateToolsets analyzes dynamic tools and groups them into toolsets
func (s *ToolsetService) AutoGenerateToolsets(ctx context.Context, tenantID string) ([]ToolsetInfo, error) {
	// Get all active dynamic tools for tenant
	// Pass empty status to get all tools, we'll filter active ones below
	tools, err := s.dynamicToolRepo.List(ctx, tenantID, "")
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	// Filter to only active tools
	activeTools := make([]*models.DynamicTool, 0)
	for _, tool := range tools {
		if tool.Status == "active" {
			activeTools = append(activeTools, tool)
		}
	}

	s.logger.Info("Auto-generating toolsets", map[string]interface{}{
		"tenant_id":    tenantID,
		"total_tools":  len(tools),
		"active_tools": len(activeTools),
	})

	// Group tools into toolsets
	toolsets := s.groupToolsIntoToolsets(activeTools)

	s.logger.Info("Generated toolsets", map[string]interface{}{
		"tenant_id":      tenantID,
		"toolset_count":  len(toolsets),
		"tools_grouped":  len(activeTools),
	})

	return toolsets, nil
}

// groupToolsIntoToolsets creates logical groupings from tool names
func (s *ToolsetService) groupToolsIntoToolsets(tools []*models.DynamicTool) []ToolsetInfo {
	// Pattern: mcp__devmesh__{provider}_{action}_{resource}
	// Examples:
	//   mcp__devmesh__github_get_repository -> provider: github, resource: repository
	//   mcp__devmesh__github_list_issues -> provider: github, resource: issues
	//   mcp__devmesh__harness_pipelines_execute -> provider: harness, resource: pipelines

	toolsetMap := make(map[string]*ToolsetInfo)

	for _, tool := range tools {
		provider, resource, action := s.parseToolName(tool.ToolName)

		if provider == "" {
			s.logger.Warn("Could not parse tool name", map[string]interface{}{
				"tool_name": tool.ToolName,
			})
			continue
		}

		// Create toolset key: provider_resource
		var toolsetKey string
		if resource != "" {
			toolsetKey = fmt.Sprintf("%s_%s", provider, resource)
		} else {
			// Fallback to provider-only grouping
			toolsetKey = provider
		}

		// Get or create toolset
		if _, exists := toolsetMap[toolsetKey]; !exists {
			toolsetMap[toolsetKey] = &ToolsetInfo{
				Toolset: mcptools.Toolset{
					Name:        toolsetKey,
					DisplayName: s.generateDisplayName(provider, resource),
					Description: s.generateDescription(provider, resource),
					Category:    s.inferCategory(resource),
					Provider:    provider,
					Version:     "1.0.0",
					Tags:        []string{provider},
					Enabled:     false,
				},
				Tools:        make([]*models.DynamicTool, 0),
				ActionCounts: make(map[string]int),
			}
		}

		// Add tool to toolset
		toolsetMap[toolsetKey].Tools = append(toolsetMap[toolsetKey].Tools, tool)

		// Track action
		if action != "" {
			toolsetMap[toolsetKey].ActionCounts[action]++

			// Add action to tags if not already present
			actionTag := fmt.Sprintf("action:%s", action)
			if !contains(toolsetMap[toolsetKey].Toolset.Tags, actionTag) {
				toolsetMap[toolsetKey].Toolset.Tags = append(toolsetMap[toolsetKey].Toolset.Tags, actionTag)
			}
		}
	}

	// Convert map to slice and finalize toolsets
	result := make([]ToolsetInfo, 0, len(toolsetMap))
	for _, info := range toolsetMap {
		// Set tools list (just the tool names/IDs)
		info.Toolset.Tools = make([]string, len(info.Tools))
		for i, tool := range info.Tools {
			info.Toolset.Tools[i] = tool.ToolName
		}

		result = append(result, *info)
	}

	return result
}

// parseToolName extracts provider, resource, and action from tool name
// Pattern: mcp__devmesh__{provider}_{action}_{resource}
func (s *ToolsetService) parseToolName(toolName string) (provider, resource, action string) {
	// Remove mcp__devmesh__ prefix
	if !strings.HasPrefix(toolName, "mcp__devmesh__") {
		return "", "", ""
	}

	remainder := strings.TrimPrefix(toolName, "mcp__devmesh__")

	// Split by underscore
	parts := strings.Split(remainder, "_")
	if len(parts) < 2 {
		return "", "", ""
	}

	// First part is provider
	provider = parts[0]

	// Common action verbs
	actions := map[string]bool{
		"get": true, "list": true, "create": true, "update": true, "delete": true,
		"add": true, "remove": true, "enable": true, "disable": true,
		"execute": true, "run": true, "cancel": true, "rerun": true,
		"merge": true, "fork": true, "search": true, "lock": true, "unlock": true,
	}

	// Check if second part is an action
	if len(parts) >= 2 && actions[parts[1]] {
		action = parts[1]
		// Resource is everything after action
		if len(parts) > 2 {
			resource = s.normalizeResource(strings.Join(parts[2:], "_"))
		}
	} else {
		// No clear action, try to infer resource from remaining parts
		resource = s.normalizeResource(strings.Join(parts[1:], "_"))
	}

	return provider, resource, action
}

// normalizeResource converts resource names to standard forms
func (s *ToolsetService) normalizeResource(raw string) string {
	// Normalize common plurals and variations
	normalizations := map[string]string{
		"repositories": "repos",
		"repository":   "repos",
		"repos":        "repos",
		"issues":       "issues",
		"issue":        "issues",
		"pulls":        "pulls",
		"pull_requests": "pulls",
		"pull_request":  "pulls",
		"workflows":    "workflows",
		"workflow":     "workflows",
		"pipelines":    "pipelines",
		"pipeline":     "pipelines",
		"branches":     "branches",
		"branch":       "branches",
		"releases":     "releases",
		"release":      "releases",
		"tags":         "tags",
		"tag":          "tags",
		"commits":      "commits",
		"commit":       "commits",
		"files":        "code",
		"file":         "code",
		"contents":     "code",
		"content":      "code",
		"secrets":      "security",
		"secret":       "security",
		"vulnerabilities": "security",
		"alerts":       "security",
		"dependabot":   "security",
	}

	if normalized, exists := normalizations[raw]; exists {
		return normalized
	}

	return raw
}

// generateDisplayName creates a user-friendly display name
func (s *ToolsetService) generateDisplayName(provider, resource string) string {
	providerNames := map[string]string{
		"github":  "GitHub",
		"harness": "Harness",
		"gitlab":  "GitLab",
		"jira":    "Jira",
	}

	resourceNames := map[string]string{
		"repos":     "Repositories",
		"issues":    "Issues",
		"pulls":     "Pull Requests",
		"workflows": "Workflows",
		"pipelines": "Pipelines",
		"branches":  "Branches",
		"releases":  "Releases",
		"code":      "Code",
		"security":  "Security",
		"tags":      "Tags",
		"commits":   "Commits",
	}

	providerDisplay := providerNames[provider]
	if providerDisplay == "" {
		providerDisplay = strings.Title(provider)
	}

	if resource == "" {
		return providerDisplay
	}

	resourceDisplay := resourceNames[resource]
	if resourceDisplay == "" {
		resourceDisplay = strings.Title(resource)
	}

	return fmt.Sprintf("%s %s", providerDisplay, resourceDisplay)
}

// generateDescription creates a description for the toolset
func (s *ToolsetService) generateDescription(provider, resource string) string {
	if resource == "" {
		return fmt.Sprintf("Tools for %s integration", provider)
	}

	descriptions := map[string]string{
		"repos":     "Repository management operations",
		"issues":    "Issue tracking and management",
		"pulls":     "Pull request workflows",
		"workflows": "CI/CD workflow automation",
		"pipelines": "Pipeline execution and management",
		"branches":  "Branch management operations",
		"releases":  "Release management",
		"code":      "Code and file operations",
		"security":  "Security scanning and alerts",
		"tags":      "Tag management",
		"commits":   "Commit operations",
	}

	if desc, exists := descriptions[resource]; exists {
		return desc
	}

	return fmt.Sprintf("%s operations for %s", strings.Title(resource), provider)
}

// inferCategory maps resources to categories
func (s *ToolsetService) inferCategory(resource string) string {
	categoryMap := map[string]string{
		"repos":     "repository",
		"issues":    "issues",
		"pulls":     "pull_requests",
		"workflows": "ci_cd",
		"pipelines": "ci_cd",
		"branches":  "repository",
		"releases":  "repository",
		"code":      "repository",
		"security":  "security",
		"tags":      "repository",
		"commits":   "repository",
	}

	if category, exists := categoryMap[resource]; exists {
		return category
	}

	return "general"
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
