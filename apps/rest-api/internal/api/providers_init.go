package api

import (
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
)

// InitializeStandardProviders registers all standard tool providers with the enhanced registry
func InitializeStandardProviders(registry *pkgservices.EnhancedToolRegistry, logger observability.Logger) error {
	logger.Info("Initializing standard tool providers", nil)

	providersCount := 0

	// Register GitHub provider
	githubProvider := github.NewGitHubProvider(logger)
	if err := registry.RegisterProvider(githubProvider); err != nil {
		logger.Error("Failed to register GitHub provider", map[string]interface{}{
			"error": err.Error(),
		})
		return err
	}
	logger.Info("Registered GitHub provider", map[string]interface{}{
		"provider": "github",
		"tools":    len(githubProvider.GetToolDefinitions()),
	})
	providersCount++

	// Register Harness provider
	harnessProvider := harness.NewHarnessProvider(logger)
	if err := registry.RegisterProvider(harnessProvider); err != nil {
		logger.Error("Failed to register Harness provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Harness provider", map[string]interface{}{
			"provider":        "harness",
			"tools":           len(harnessProvider.GetToolDefinitions()),
			"enabled_modules": harnessProvider.GetEnabledModules(),
		})
		providersCount++
	}

	// TODO: Register additional providers
	// - GitLab provider
	// - Jira provider
	// - Confluence provider
	// - Azure DevOps provider
	// - CircleCI provider
	// - Jenkins provider

	logger.Info("Standard tool providers initialized", map[string]interface{}{
		"count": providersCount,
	})

	return nil
}
