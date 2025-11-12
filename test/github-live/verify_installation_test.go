//go:build github_live

package github_live

import (
	"context"
	"os"
	"strconv"
	"testing"

	adapterEvents "github.com/developer-mesh/developer-mesh/pkg/adapters/events"
	"github.com/developer-mesh/developer-mesh/pkg/adapters/github"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/require"
)

// TestVerifyInstallation checks the GitHub App installation and lists accessible repositories
func TestVerifyInstallation(t *testing.T) {
	// Load environment
	appID := os.Getenv("GITHUB_APP_ID")
	if appID == "" {
		t.Skip("GITHUB_APP_ID not set")
	}

	logger := observability.NewLogger("verify-installation")
	metricsClient := observability.NewNoOpMetricsClient()
	var eventBus adapterEvents.EventBus = nil

	config := github.DefaultConfig()
	config.BaseURL = "https://api.github.com/"

	// Configure GitHub App
	appIDInt, err := strconv.ParseInt(appID, 10, 64)
	require.NoError(t, err)

	config.Auth.Type = "app"
	config.Auth.AppID = appIDInt

	// Add installation ID
	installationID := os.Getenv("GITHUB_APP_INSTALLATION_ID")
	if installationID != "" {
		installationIDInt, err := strconv.ParseInt(installationID, 10, 64)
		require.NoError(t, err)
		config.Auth.InstallationID = installationIDInt
	}

	// Read private key
	privateKeyPath := os.Getenv("GITHUB_APP_PRIVATE_KEY_PATH")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	require.NoError(t, err)

	config.Auth.PrivateKey = string(privateKeyBytes)

	// Create adapter
	adapter, err := github.New(config, logger, metricsClient, eventBus)
	require.NoError(t, err)
	defer adapter.Close()

	ctx := context.Background()

	t.Run("FindAccessibleRepos", func(t *testing.T) {
		t.Logf("Checking repositories accessible by installation %s", installationID)

		// Try multiple possible locations
		testCases := []struct {
			owner string
			repo  string
		}{
			{"developer-mesh", "developer-mesh"},
			{"developer-mesh", "developer-mesh-test"},
			{"developer-mesh-TEST", "developer-mesh-test"},
		}

		foundRepo := false
		for _, tc := range testCases {
			params := map[string]any{
				"owner": tc.owner,
				"repo":  tc.repo,
			}

			t.Logf("Trying %s/%s...", tc.owner, tc.repo)
			result, err := adapter.ExecuteAction(ctx, "", "getRepository", params)
			if err != nil {
				t.Logf("  ❌ Error: %v", err)
			} else {
				repo := result.(map[string]any)
				t.Logf("  ✓ Successfully accessed repository: %s", repo["full_name"])
				t.Logf("    Description: %s", repo["description"])
				t.Logf("    Private: %v", repo["private"])
				t.Log("\n  Your GitHub App installation is working correctly!")
				t.Log("  Update your .env.test with:")
				t.Logf("  GITHUB_TEST_ORG=%s", tc.owner)
				t.Logf("  GITHUB_TEST_REPO=%s", tc.repo)
				foundRepo = true
				break
			}
		}

		if !foundRepo {
			t.Log("\nNo repositories were accessible with this installation.")
			t.Log("This means:")
			t.Logf("1. Your GitHub App is installed on installation ID %s", installationID)
			t.Log("2. But it doesn't have access to the test repository")
			t.Log("\nYou need to either:")
			t.Log("- Install the GitHub App on the developer-mesh-TEST organization")
			t.Log("- Create a test repository under your personal account developer-mesh")
			t.Log("- Or grant the existing installation access to the test repo")
		}
	})
}
