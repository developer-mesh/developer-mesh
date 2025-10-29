package services

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"gopkg.in/yaml.v3"
)

// ToolFilterConfig represents the tool filtering configuration
type ToolFilterConfig struct {
	Harness HarnessFilterConfig  `yaml:"harness"`
	GitHub  ProviderFilterConfig `yaml:"github"`
}

// HarnessFilterConfig represents Harness-specific filtering rules
type HarnessFilterConfig struct {
	ExcludedPatterns   []string            `yaml:"excluded_patterns"`
	WorkflowOperations []string            `yaml:"workflow_operations"`
	KeptTools          map[string][]string `yaml:"kept_tools"`
}

// ProviderFilterConfig represents generic provider filtering rules
type ProviderFilterConfig struct {
	ExcludedPatterns []string `yaml:"excluded_patterns"`
}

// ToolFilterService handles tool filtering logic
type ToolFilterService struct {
	config *ToolFilterConfig
	logger observability.Logger
}

// NewToolFilterService creates a new tool filter service
func NewToolFilterService(configPath string, logger observability.Logger) (*ToolFilterService, error) {
	// If no path provided, use default
	if configPath == "" {
		configPath = "configs/tool-filters.yaml"
	}

	// Read configuration file
	data, err := os.ReadFile(configPath)
	if err != nil {
		// If config file doesn't exist, return service with empty config (no filtering)
		logger.Warn("Tool filter config not found, tool filtering disabled", map[string]interface{}{
			"config_path": configPath,
			"error":       err.Error(),
		})
		return &ToolFilterService{
			config: &ToolFilterConfig{},
			logger: logger,
		}, nil
	}

	// Parse YAML configuration
	var config ToolFilterConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		logger.Error("Failed to parse tool filter config", map[string]interface{}{
			"config_path": configPath,
			"error":       err.Error(),
		})
		return nil, err
	}

	logger.Info("Loaded tool filter configuration", map[string]interface{}{
		"config_path":        configPath,
		"harness_patterns":   len(config.Harness.ExcludedPatterns),
		"harness_exceptions": len(config.Harness.WorkflowOperations),
		"github_patterns":    len(config.GitHub.ExcludedPatterns),
	})

	return &ToolFilterService{
		config: &config,
		logger: logger,
	}, nil
}

// ShouldIncludeTool determines if a tool should be included based on filtering rules
func (s *ToolFilterService) ShouldIncludeTool(toolName string) bool {
	// Determine provider from tool name
	provider := s.getProviderFromToolName(toolName)

	switch provider {
	case "harness":
		return s.shouldIncludeHarnessTool(toolName)
	case "github":
		return s.shouldIncludeGitHubTool(toolName)
	default:
		// Unknown provider - include by default
		return true
	}
}

// shouldIncludeHarnessTool checks if a Harness tool should be included
func (s *ToolFilterService) shouldIncludeHarnessTool(toolName string) bool {
	// First, check if this is a workflow operation (exception)
	// These are ALWAYS included even if they match excluded patterns
	for _, exception := range s.config.Harness.WorkflowOperations {
		if matchesPattern(toolName, exception) {
			s.logger.Debug("Harness tool included (workflow operation)", map[string]interface{}{
				"tool_name": toolName,
				"pattern":   exception,
			})
			return true
		}
	}

	// Check if tool matches any excluded pattern
	for _, pattern := range s.config.Harness.ExcludedPatterns {
		if matchesPattern(toolName, pattern) {
			s.logger.Debug("Harness tool excluded", map[string]interface{}{
				"tool_name": toolName,
				"pattern":   pattern,
			})
			return false
		}
	}

	// Tool doesn't match any exclusion pattern - include it
	return true
}

// shouldIncludeGitHubTool checks if a GitHub tool should be included
func (s *ToolFilterService) shouldIncludeGitHubTool(toolName string) bool {
	// Check if tool matches any excluded pattern
	for _, pattern := range s.config.GitHub.ExcludedPatterns {
		if matchesPattern(toolName, pattern) {
			s.logger.Debug("GitHub tool excluded", map[string]interface{}{
				"tool_name": toolName,
				"pattern":   pattern,
			})
			return false
		}
	}

	// Tool doesn't match any exclusion pattern - include it
	return true
}

// getProviderFromToolName extracts the provider name from a tool name
func (s *ToolFilterService) getProviderFromToolName(toolName string) string {
	// Tool names follow pattern: mcp__devmesh__<provider>_<operation>
	// or: <provider>_<operation>

	// Remove mcp__devmesh__ prefix if present
	cleanName := strings.TrimPrefix(toolName, "mcp__devmesh__")

	// Extract provider (everything before first underscore after provider name)
	parts := strings.SplitN(cleanName, "_", 2)
	if len(parts) > 0 {
		return parts[0]
	}

	return "unknown"
}

// matchesPattern checks if a tool name matches a pattern
// Supports wildcards: * matches any characters
func matchesPattern(toolName, pattern string) bool {
	// Handle simple exact match
	if pattern == toolName {
		return true
	}

	// Handle wildcard patterns
	if strings.Contains(pattern, "*") {
		// Convert pattern to regex-like matching
		// Split pattern by * and check if all parts exist in order
		parts := strings.Split(pattern, "*")

		// Start matching from the beginning
		currentPos := 0
		for i, part := range parts {
			if part == "" {
				continue
			}

			// For first part, must match at start
			if i == 0 && !strings.HasPrefix(toolName[currentPos:], part) {
				return false
			}

			// For last part, must match at end
			if i == len(parts)-1 && !strings.HasSuffix(toolName, part) {
				return false
			}

			// For middle parts, find next occurrence
			idx := strings.Index(toolName[currentPos:], part)
			if idx == -1 {
				return false
			}
			currentPos += idx + len(part)
		}

		return true
	}

	// No match
	return false
}

// FilterTools applies filtering rules to a list of tool names
func (s *ToolFilterService) FilterTools(tools []string) []string {
	if s.config == nil {
		return tools // No filtering if config not loaded
	}

	filtered := make([]string, 0, len(tools))
	excluded := 0

	for _, tool := range tools {
		if s.ShouldIncludeTool(tool) {
			filtered = append(filtered, tool)
		} else {
			excluded++
		}
	}

	s.logger.Info("Applied tool filtering", map[string]interface{}{
		"total_tools":    len(tools),
		"filtered_tools": len(filtered),
		"excluded_tools": excluded,
		"reduction_pct":  float64(excluded) / float64(len(tools)) * 100,
	})

	return filtered
}

// GetFilterStats returns statistics about configured filters
func (s *ToolFilterService) GetFilterStats() map[string]interface{} {
	if s.config == nil {
		return map[string]interface{}{
			"enabled": false,
		}
	}

	return map[string]interface{}{
		"enabled": true,
		"harness": map[string]interface{}{
			"excluded_patterns":   len(s.config.Harness.ExcludedPatterns),
			"workflow_exceptions": len(s.config.Harness.WorkflowOperations),
		},
		"github": map[string]interface{}{
			"excluded_patterns": len(s.config.GitHub.ExcludedPatterns),
		},
	}
}

// GetConfigPath returns the path to the tool-filters.yaml file
// Searches in multiple locations: current dir, project root, configs dir
func GetToolFilterConfigPath() string {
	candidates := []string{
		"configs/tool-filters.yaml",
		"../../configs/tool-filters.yaml",
		"../../../configs/tool-filters.yaml",
		filepath.Join(os.Getenv("CONFIG_DIR"), "tool-filters.yaml"),
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Return default path even if it doesn't exist
	// Service will handle missing file gracefully
	return "configs/tool-filters.yaml"
}
