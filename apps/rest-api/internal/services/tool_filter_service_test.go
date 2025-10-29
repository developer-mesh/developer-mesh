package services

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatchesPattern(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		pattern  string
		expected bool
	}{
		// Exact matches
		{
			name:     "exact match",
			toolName: "harness_users_list",
			pattern:  "harness_users_list",
			expected: true,
		},
		// Wildcard at end
		{
			name:     "wildcard suffix match",
			toolName: "harness_users_list",
			pattern:  "harness_users_*",
			expected: true,
		},
		{
			name:     "wildcard suffix no match",
			toolName: "harness_pipelines_list",
			pattern:  "harness_users_*",
			expected: false,
		},
		// Wildcard at start
		{
			name:     "wildcard prefix match",
			toolName: "harness_users_create",
			pattern:  "*_create",
			expected: true,
		},
		{
			name:     "wildcard prefix no match",
			toolName: "harness_users_list",
			pattern:  "*_create",
			expected: false,
		},
		// Multiple wildcards
		{
			name:     "multiple wildcards match",
			toolName: "harness_ccm_perspectives_list",
			pattern:  "harness_ccm_*",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := matchesPattern(tt.toolName, tt.pattern)
			assert.Equal(t, tt.expected, result, "Pattern matching failed for %s vs %s", tt.toolName, tt.pattern)
		})
	}
}

func TestGetProviderFromToolName(t *testing.T) {
	logger := observability.NewStandardLogger("test")
	service := &ToolFilterService{logger: logger}

	tests := []struct {
		name     string
		toolName string
		expected string
	}{
		{
			name:     "harness tool with prefix",
			toolName: "mcp__devmesh__harness_pipelines_execute",
			expected: "harness",
		},
		{
			name:     "harness tool without prefix",
			toolName: "harness_pipelines_execute",
			expected: "harness",
		},
		{
			name:     "github tool",
			toolName: "github_list_repositories",
			expected: "github",
		},
		{
			name:     "unknown provider",
			toolName: "unknown_tool",
			expected: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.getProviderFromToolName(tt.toolName)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestShouldIncludeHarnessTool(t *testing.T) {
	// Create temporary config for testing
	configContent := `
harness:
  excluded_patterns:
    - "harness_users_*"
    - "harness_ccm_*"
    - "*_create"
    - "*_update"
    - "*_delete"
  workflow_operations:
    - "harness_pipelines_execute"
    - "harness_pullrequests_merge"
    - "harness_approvals_approve"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tool-filters.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := observability.NewStandardLogger("test")
	service, err := NewToolFilterService(configPath, logger)
	require.NoError(t, err)
	require.NotNil(t, service)

	tests := []struct {
		name     string
		toolName string
		expected bool
		reason   string
	}{
		// Workflow operations (exceptions - always included)
		{
			name:     "workflow operation included",
			toolName: "harness_pipelines_execute",
			expected: true,
			reason:   "workflow operation exception",
		},
		{
			name:     "pullrequest merge included",
			toolName: "harness_pullrequests_merge",
			expected: true,
			reason:   "workflow operation exception",
		},
		// Excluded patterns
		{
			name:     "user management excluded",
			toolName: "harness_users_list",
			expected: false,
			reason:   "matches harness_users_* pattern",
		},
		{
			name:     "ccm tool excluded",
			toolName: "harness_ccm_perspectives_list",
			expected: false,
			reason:   "matches harness_ccm_* pattern",
		},
		{
			name:     "create operation excluded",
			toolName: "harness_services_create",
			expected: false,
			reason:   "matches *_create pattern",
		},
		// Included tools (don't match any exclusion)
		{
			name:     "pipelines list included",
			toolName: "harness_pipelines_list",
			expected: true,
			reason:   "doesn't match any exclusion pattern",
		},
		{
			name:     "executions status included",
			toolName: "harness_executions_status",
			expected: true,
			reason:   "doesn't match any exclusion pattern",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.ShouldIncludeTool(tt.toolName)
			assert.Equal(t, tt.expected, result, "Tool %s should be %v (%s)", tt.toolName, tt.expected, tt.reason)
		})
	}
}

func TestFilterTools(t *testing.T) {
	// Create temporary config
	configContent := `
harness:
  excluded_patterns:
    - "harness_users_*"
    - "harness_ccm_*"
    - "*_create"
  workflow_operations:
    - "harness_pipelines_execute"
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tool-filters.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := observability.NewStandardLogger("test")
	service, err := NewToolFilterService(configPath, logger)
	require.NoError(t, err)

	tools := []string{
		"harness_pipelines_execute",     // Included (workflow exception)
		"harness_pipelines_list",        // Included
		"harness_users_list",            // Excluded (users_*)
		"harness_ccm_perspectives_list", // Excluded (ccm_*)
		"harness_services_create",       // Excluded (*_create)
		"harness_services_list",         // Included
		"github_list_repositories",      // Included (different provider)
	}

	filtered := service.FilterTools(tools)

	expected := []string{
		"harness_pipelines_execute",
		"harness_pipelines_list",
		"harness_services_list",
		"github_list_repositories",
	}

	assert.Equal(t, len(expected), len(filtered), "Filtered tool count mismatch")
	for _, tool := range expected {
		assert.Contains(t, filtered, tool, "Expected tool %s not in filtered list", tool)
	}

	// Verify excluded tools are not present
	assert.NotContains(t, filtered, "harness_users_list")
	assert.NotContains(t, filtered, "harness_ccm_perspectives_list")
	assert.NotContains(t, filtered, "harness_services_create")
}

func TestFilterToolsWithNoConfig(t *testing.T) {
	logger := observability.NewStandardLogger("test")

	// Create service with non-existent config path
	service, err := NewToolFilterService("/nonexistent/path.yaml", logger)
	require.NoError(t, err) // Should not error, just disable filtering
	require.NotNil(t, service)

	tools := []string{
		"harness_users_list",
		"harness_pipelines_list",
		"github_list_repositories",
	}

	// With no config, all tools should pass through
	filtered := service.FilterTools(tools)
	assert.Equal(t, len(tools), len(filtered), "All tools should be included when no config is loaded")
}

func TestGetFilterStats(t *testing.T) {
	configContent := `
harness:
  excluded_patterns:
    - "harness_users_*"
    - "harness_ccm_*"
  workflow_operations:
    - "harness_pipelines_execute"
    - "harness_pullrequests_merge"
github:
  excluded_patterns: []
`

	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tool-filters.yaml")
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	logger := observability.NewStandardLogger("test")
	service, err := NewToolFilterService(configPath, logger)
	require.NoError(t, err)

	stats := service.GetFilterStats()
	assert.True(t, stats["enabled"].(bool))

	harnessStats := stats["harness"].(map[string]interface{})
	assert.Equal(t, 2, harnessStats["excluded_patterns"])
	assert.Equal(t, 2, harnessStats["workflow_exceptions"])
}
