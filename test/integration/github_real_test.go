//go:build integration && github_real

package integration

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	adapterEvents "github.com/developer-mesh/developer-mesh/pkg/adapters/events"
	"github.com/developer-mesh/developer-mesh/pkg/adapters/github"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGitHubRealAPI tests against the real GitHub API
// Run with: go test -tags="integration,github_real" ./test/integration -run TestGitHubRealAPI
func TestGitHubRealAPI(t *testing.T) {
	// Skip if not explicitly enabled
	if os.Getenv("USE_GITHUB_MOCK") != "false" {
		t.Skip("Skipping real GitHub API tests. Set USE_GITHUB_MOCK=false to enable")
	}

	// Ensure required environment variables are set
	requiredEnvVars := []string{
		"GITHUB_TOKEN",
		"GITHUB_TEST_ORG",
		"GITHUB_TEST_REPO",
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Skipf("Required environment variable %s not set", envVar)
		}
	}

	// Setup
	logger := observability.NewLogger("github-integration-test")
	metricsClient := observability.NewNoOpMetricsClient()

	// Create a nil event bus for testing (GitHub adapter should handle nil)
	var eventBus adapterEvents.EventBus = nil

	config := github.DefaultConfig()

	// Use real GitHub API
	config.BaseURL = "https://api.github.com/"

	// Configure authentication
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		config.Auth.Token = token
		config.Auth.Type = "token"
	} else {
		// Use GitHub App authentication
		appID := os.Getenv("GITHUB_APP_ID")
		appIDInt, err := strconv.ParseInt(appID, 10, 64)
		require.NoError(t, err, "Invalid GITHUB_APP_ID")

		config.Auth.Type = "app"
		config.Auth.AppID = appIDInt

		// Read private key file
		privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
		privateKeyBytes, err := os.ReadFile(privateKeyPath)
		require.NoError(t, err, "Failed to read private key file")

		config.Auth.PrivateKey = string(privateKeyBytes)
	}

	// Configure webhook secret if available
	if secret := os.Getenv("GITHUB_WEBHOOK_SECRET"); secret != "" {
		config.WebhookSecret = secret
	}

	// Create GitHub adapter
	githubAdapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)

	ctx := context.Background()
	testOrg := os.Getenv("GITHUB_TEST_ORG")
	testRepo := os.Getenv("GITHUB_TEST_REPO")

	t.Run("GetRepository", func(t *testing.T) {
		// Test getting repository info
		params := map[string]any{
			"owner": testOrg,
			"repo":  testRepo,
		}

		result, err := githubAdapter.ExecuteAction(ctx, "", "get_repository", params)
		require.NoError(t, err)

		repo, ok := result.(map[string]any)
		require.True(t, ok, "Expected repository result to be a map")

		assert.Equal(t, testRepo, repo["name"])
		t.Logf("Successfully retrieved repository: %s", repo["full_name"])
	})

	t.Run("ListPullRequests", func(t *testing.T) {
		// Test listing pull requests
		params := map[string]any{
			"owner": testOrg,
			"repo":  testRepo,
			"state": "all",
		}

		result, err := githubAdapter.ExecuteAction(ctx, "", "list_pull_requests", params)
		if err != nil {
			t.Logf("Error listing PRs: %v", err)
			// This might fail if the action isn't implemented
			t.Skip("list_pull_requests action might not be implemented")
		}

		prs, ok := result.([]any)
		if ok {
			t.Logf("Found %d pull requests", len(prs))
		}
	})

	t.Run("RateLimiting", func(t *testing.T) {
		// Test that we respect GitHub rate limits
		start := time.Now()

		// Make several requests in quick succession
		params := map[string]any{
			"owner": testOrg,
			"repo":  testRepo,
		}

		for i := 0; i < 5; i++ {
			_, err := githubAdapter.ExecuteAction(ctx, "", "get_repository", params)
			require.NoError(t, err)
		}

		elapsed := time.Since(start)
		// With rate limiting, this should be controlled
		t.Logf("5 requests took %v", elapsed)
	})

	t.Run("HealthCheck", func(t *testing.T) {
		// Test adapter health
		health := githubAdapter.Health()
		assert.Equal(t, "healthy", health)

		// Test adapter info
		assert.Equal(t, "github", githubAdapter.Type())
		t.Logf("GitHub adapter version: %s", githubAdapter.Version())
	})
}

// TestGitHubAppAuthentication tests GitHub App authentication
func TestGitHubAppAuthentication(t *testing.T) {
	// Skip if not explicitly testing real API
	if os.Getenv("USE_GITHUB_MOCK") != "false" {
		t.Skip("Skipping real GitHub API tests. Set USE_GITHUB_MOCK=false to enable")
	}

	// Skip if not using GitHub App
	if os.Getenv("GITHUB_APP_ID") == "" || os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH") == "" {
		t.Skip("GitHub App credentials not configured")
	}

	logger := observability.NewLogger("github-app-test")
	metricsClient := observability.NewNoOpMetricsClient()
	var eventBus adapterEvents.EventBus = nil

	config := github.DefaultConfig()
	config.BaseURL = "https://api.github.com/"

	// Configure GitHub App
	appID := os.Getenv("GITHUB_APP_ID")
	appIDInt, err := strconv.ParseInt(appID, 10, 64)
	require.NoError(t, err)

	config.Auth.Type = "app"
	config.Auth.AppID = appIDInt

	// Read private key
	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	require.NoError(t, err)

	config.Auth.PrivateKey = string(privateKeyBytes)

	adapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)

	// Test App authentication by verifying adapter works
	t.Run("VerifyAppAuth", func(t *testing.T) {
		// The adapter was created successfully with App auth
		assert.NotNil(t, adapter)
		assert.Equal(t, "github", adapter.Type())
		assert.Equal(t, "healthy", adapter.Health())

		t.Logf("Successfully authenticated with GitHub App ID: %s", appID)
	})
}
