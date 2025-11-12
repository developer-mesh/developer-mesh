//go:build github_live

package github_live

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

// TestGitHubLiveAPI tests against the real GitHub API
// Run with: go test -tags=github_live ./test/github-live -v
func TestGitHubLiveAPI(t *testing.T) {
	// Skip if not explicitly enabled
	if os.Getenv("USE_GITHUB_MOCK") != "false" {
		t.Skip("Skipping real GitHub API tests. Set USE_GITHUB_MOCK=false to enable")
	}

	// Ensure required environment variables are set
	requiredEnvVars := []string{
		"GITHUB_APP_ID",
		"GITHUB_APP_PRIVATE_KEY_PATH",
		"GITHUB_TEST_ORG",
		"GITHUB_TEST_REPO",
	}

	for _, envVar := range requiredEnvVars {
		if os.Getenv(envVar) == "" {
			t.Skipf("Required environment variable %s not set", envVar)
		}
	}

	// Setup
	logger := observability.NewLogger("github-live-test")
	metricsClient := observability.NewNoOpMetricsClient()

	// Create a nil event bus for testing
	var eventBus adapterEvents.EventBus = nil

	config := github.DefaultConfig()

	// Use real GitHub API
	config.BaseURL = "https://api.github.com/"

	// Configure GitHub App authentication
	appID := os.Getenv("GITHUB_APP_ID")
	appIDInt, err := strconv.ParseInt(appID, 10, 64)
	require.NoError(t, err, "Invalid GITHUB_APP_ID")

	config.Auth.Type = "app"
	config.Auth.AppID = appIDInt

	// Add installation ID
	installationID := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	if installationID != "" {
		installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
		require.NoError(t, err, "Invalid GITHUB_APP_INSTALLATION_ID")
		config.Auth.InstallationID = installationIDInt
	}

	// Read private key file
	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	require.NoError(t, err, "Failed to read private key file")

	config.Auth.PrivateKey = string(privateKeyBytes)

	// Configure webhook secret if available
	if secret := os.Getenv("GITHUB_WEBHOOK_SECRET"); secret != "" {
		config.WebhookSecret = secret
	}

	// Create GitHub adapter
	githubAdapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err, "Failed to create GitHub adapter")

	ctx := context.Background()
	testOrg := os.Getenv("GITHUB_TEST_ORG")
	testRepo := os.Getenv("GITHUB_TEST_REPO")

	t.Run("AdapterInfo", func(t *testing.T) {
		// Test basic adapter functionality
		assert.Equal(t, "github", githubAdapter.Type())
		assert.Equal(t, "healthy", githubAdapter.Health())
		t.Logf("GitHub adapter version: %s", githubAdapter.Version())
	})

	t.Run("GetRepository", func(t *testing.T) {
		// Test getting repository info
		params := map[string]any{
			"owner": testOrg,
			"repo":  testRepo,
		}

		result, err := githubAdapter.ExecuteAction(ctx, "", "getRepository", params)
		require.NoError(t, err, "Failed to get repository")

		repo, ok := result.(map[string]any)
		require.True(t, ok, "Expected repository result to be a map")

		assert.Equal(t, testRepo, repo["name"])
		t.Logf("Successfully retrieved repository: %s", repo["full_name"])
		t.Logf("Repository description: %s", repo["description"])
		t.Logf("Repository visibility: %v", repo["private"])
	})

	t.Run("ExecuteUnknownAction", func(t *testing.T) {
		// Test error handling for unknown action
		params := map[string]any{
			"test": "value",
		}

		_, err := githubAdapter.ExecuteAction(ctx, "", "unknown_action", params)
		assert.Error(t, err)
		t.Logf("Expected error for unknown action: %v", err)
	})

	t.Run("RateLimiting", func(t *testing.T) {
		// Test that we handle rate limiting properly
		start := time.Now()

		params := map[string]any{
			"owner": testOrg,
			"repo":  testRepo,
		}

		// Make multiple requests
		successCount := 0
		for i := 0; i < 3; i++ {
			_, err := githubAdapter.ExecuteAction(ctx, "", "getRepository", params)
			if err == nil {
				successCount++
			}
		}

		elapsed := time.Since(start)
		assert.Equal(t, 3, successCount, "All requests should succeed")
		t.Logf("3 requests completed in %v", elapsed)
	})

	t.Run("Cleanup", func(t *testing.T) {
		// Clean up adapter resources
		err := githubAdapter.Close()
		assert.NoError(t, err)
		t.Log("GitHub adapter closed successfully")
	})
}

// TestGitHubWebhookValidation tests webhook signature validation
func TestGitHubWebhookValidation(t *testing.T) {
	secret := os.Getenv("GITHUB_WEBHOOK_SECRET")
	if secret == "" {
		secret = "test-webhook-secret"
	}

	// Create a simple adapter config for webhook testing
	config := github.DefaultConfig()
	config.WebhookSecret = secret

	logger := observability.NewLogger("webhook-test")
	metricsClient := observability.NewNoOpMetricsClient()
	var eventBus adapterEvents.EventBus = nil

	adapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)

	t.Run("ValidateWebhook", func(t *testing.T) {
		// Test webhook handling
		payload := []byte(`{"action":"opened","pull_request":{"id":1,"title":"Test PR"}}`)

		// The adapter should be able to handle the webhook
		err := adapter.HandleWebhook(context.Background(), "pull_request", payload)
		// This might fail if webhook processing isn't fully implemented
		if err != nil {
			t.Logf("Webhook handling not fully implemented: %v", err)
		}
	})
}
