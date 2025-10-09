package mcp

import (
	"encoding/json"
	"net/http"
	"os"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractPassthroughAuth_FromHeaders(t *testing.T) {
	handler := &Handler{
		logger: observability.NewNoopLogger(),
	}

	req, _ := http.NewRequest("GET", "/test", nil)
	req.Header.Set("X-GitHub-Token", "ghp_testtoken123")
	req.Header.Set("X-Service-Slack-Token", "xoxb-slack-token")
	req.Header.Set("X-AWS-Access-Key", "AKIAIOSFODNN7EXAMPLE")
	req.Header.Set("X-AWS-Secret-Key", "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY")
	req.Header.Set("X-AWS-Region", "us-west-2")

	bundle := handler.extractPassthroughAuth(req)

	// Verify GitHub token
	require.NotNil(t, bundle)
	assert.Contains(t, bundle.Credentials, "github")
	assert.Equal(t, "bearer", bundle.Credentials["github"].Type)
	assert.Equal(t, "ghp_testtoken123", bundle.Credentials["github"].Token)

	// Verify Slack token
	assert.Contains(t, bundle.Credentials, "slack")
	assert.Equal(t, "xoxb-slack-token", bundle.Credentials["slack"].Token)

	// Verify AWS credentials
	assert.Contains(t, bundle.Credentials, "aws")
	assert.Equal(t, "aws_signature", bundle.Credentials["aws"].Type)
	assert.Equal(t, "AKIAIOSFODNN7EXAMPLE", bundle.Credentials["aws"].Properties["access_key"])
	assert.Equal(t, "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY", bundle.Credentials["aws"].Properties["secret_key"])
	assert.Equal(t, "us-west-2", bundle.Credentials["aws"].Properties["region"])
}

func TestExtractPassthroughAuthFromEnv(t *testing.T) {
	handler := &Handler{
		logger: observability.NewNoopLogger(),
	}

	// Set environment variables
	oldGitHub := os.Getenv("GITHUB_TOKEN")
	oldHarness := os.Getenv("HARNESS_TOKEN")
	defer func() {
		_ = os.Setenv("GITHUB_TOKEN", oldGitHub)
		_ = os.Setenv("HARNESS_TOKEN", oldHarness)
	}()

	_ = os.Setenv("GITHUB_TOKEN", "ghp_env_token")
	_ = os.Setenv("HARNESS_TOKEN", "pat.harness.token")

	bundle := handler.extractPassthroughAuthFromEnv()

	// Verify tokens from environment
	require.NotNil(t, bundle)
	assert.Contains(t, bundle.Credentials, "github")
	assert.Equal(t, "ghp_env_token", bundle.Credentials["github"].Token)

	assert.Contains(t, bundle.Credentials, "harness")
	assert.Equal(t, "pat.harness.token", bundle.Credentials["harness"].Token)
}

func TestFilterToolsByPermissions_NoPermissions(t *testing.T) {
	handler := &Handler{
		logger: observability.NewNoopLogger(),
	}

	// Create sample tools
	allTools := []tools.ToolDefinition{
		{Name: "github_list_repos", Description: "List GitHub repositories"},
		{Name: "harness_pipelines_list", Description: "List Harness pipelines"},
		{Name: "generic_tool", Description: "Generic tool"},
	}

	// Session without permissions
	session := &Session{
		ID:              "test-session",
		PassthroughAuth: nil,
	}

	// Should return all tools when no permissions
	filtered := handler.filterToolsByPermissions(allTools, session)
	assert.Len(t, filtered, 3, "Should return all tools when no permissions")
}

func TestFilterToolsByPermissions_WithHarnessPermissions(t *testing.T) {
	handler := &Handler{
		logger: observability.NewNoopLogger(),
	}

	// Create sample tools
	allTools := []tools.ToolDefinition{
		{Name: "github_list_repos", Description: "List GitHub repositories"},
		{Name: "harness_pipelines_list", Description: "List Harness pipelines"},
		{Name: "harness_executions_get", Description: "Get Harness executions"},
		{Name: "harness_featureflags_list", Description: "List feature flags"},
		{Name: "generic_tool", Description: "Generic tool"},
	}

	// Create Harness permissions (only CI module enabled)
	permissions := &harness.HarnessPermissions{
		EnabledModules: map[string]bool{
			"ci": true,
			"cf": false, // Feature flags disabled
		},
	}

	permsJSON, _ := json.Marshal(permissions)

	// Session with Harness permissions
	session := &Session{
		ID: "test-session",
		PassthroughAuth: &models.PassthroughAuthBundle{
			Credentials: map[string]*models.PassthroughCredential{
				"harness": {
					Type:  "api_key",
					Token: "test-token",
					Properties: map[string]string{
						"permissions": string(permsJSON),
					},
				},
			},
		},
	}

	// Filter tools
	filtered := handler.filterToolsByPermissions(allTools, session)

	// Verify filtering
	toolNames := make(map[string]bool)
	for _, tool := range filtered {
		toolNames[tool.Name] = true
	}

	assert.True(t, toolNames["github_list_repos"], "GitHub tools should pass")
	assert.True(t, toolNames["harness_pipelines_list"], "Pipelines (CI module) should pass")
	assert.True(t, toolNames["harness_executions_get"], "Executions (CI module) should pass")
	assert.False(t, toolNames["harness_featureflags_list"], "Feature flags should be filtered out")
	assert.True(t, toolNames["generic_tool"], "Generic tools should pass")
}
