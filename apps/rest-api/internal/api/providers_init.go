package api

import (
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	pkgservices "github.com/developer-mesh/developer-mesh/pkg/services"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/artifactory"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/confluence"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/github"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/gitlab"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/harness"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/jira"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/nexus"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/xray"
)

// InitializeStandardProviders registers all standard tool providers with the enhanced registry
func InitializeStandardProviders(registry *pkgservices.EnhancedToolRegistry, logger observability.Logger) error {
	logger.Info("Initializing standard tool providers", nil)

	providersCount := 0

	// Create common dependencies for providers
	encryptionSvc := security.NewEncryptionService("devmesh-encryption-key-32bytes!!")

	// Circuit breaker configuration
	cbConfig := resilience.CircuitBreakerConfig{
		FailureThreshold:    5,
		FailureRatio:        0.6,
		ResetTimeout:        30 * time.Second,
		SuccessThreshold:    2,
		TimeoutThreshold:    5 * time.Second,
		MaxRequestsHalfOpen: 5,
		MinimumRequestCount: 10,
	}
	circuitBreaker := resilience.NewCircuitBreaker("github", cbConfig, logger, nil)

	// Rate limiter configuration
	rlConfig := resilience.RateLimiterConfig{
		Limit:       100,
		Period:      time.Minute,
		BurstFactor: 3,
	}
	rateLimiter := resilience.NewRateLimiter("github", rlConfig)

	// Retry policy configuration
	retryPolicy := &resilience.RetryPolicy{
		MaxAttempts:  3,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     5 * time.Second,
		Multiplier:   2.0,
		Jitter:       0.1,
	}

	// Register GitHub provider
	githubProvider := github.NewGitHubProvider(logger, encryptionSvc, circuitBreaker, rateLimiter, retryPolicy)
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

	// Register Artifactory provider
	artifactoryProvider := artifactory.NewArtifactoryProvider(logger)
	if err := registry.RegisterProvider(artifactoryProvider); err != nil {
		logger.Error("Failed to register Artifactory provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Artifactory provider", map[string]interface{}{
			"provider":   "artifactory",
			"tools":      len(artifactoryProvider.GetToolDefinitions()),
			"operations": len(artifactoryProvider.GetOperationMappings()),
		})
		providersCount++
	}

	// Register Xray provider (JFrog Xray security scanning)
	xrayProvider := xray.NewXrayProvider(logger)
	if err := registry.RegisterProvider(xrayProvider); err != nil {
		logger.Error("Failed to register Xray provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Xray provider", map[string]interface{}{
			"provider":   "xray",
			"tools":      len(xrayProvider.GetToolDefinitions()),
			"operations": len(xrayProvider.GetOperationMappings()),
		})
		providersCount++
	}

	// Register GitLab provider
	gitlabProvider := gitlab.NewGitLabProvider(logger)
	if err := registry.RegisterProvider(gitlabProvider); err != nil {
		logger.Error("Failed to register GitLab provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered GitLab provider", map[string]interface{}{
			"provider":        "gitlab",
			"tools":           len(gitlabProvider.GetToolDefinitions()),
			"enabled_modules": gitlabProvider.GetEnabledModules(),
		})
		providersCount++
	}

	// Register Nexus provider
	nexusProvider := nexus.NewNexusProvider(logger)
	if err := registry.RegisterProvider(nexusProvider); err != nil {
		logger.Error("Failed to register Nexus provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Nexus provider", map[string]interface{}{
			"provider":        "nexus",
			"tools":           len(nexusProvider.GetToolDefinitions()),
			"enabled_modules": nexusProvider.GetEnabledModules(),
			"operations":      len(nexusProvider.GetOperationMappings()),
		})
		providersCount++
	}

	// Register Jira provider
	// Default domain can be overridden via configuration
	jiraProvider := jira.NewJiraProvider(logger, "devmesh")
	if err := registry.RegisterProvider(jiraProvider); err != nil {
		logger.Error("Failed to register Jira provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Jira provider", map[string]interface{}{
			"provider":   "jira",
			"tools":      len(jiraProvider.GetToolDefinitions()),
			"operations": len(jiraProvider.GetOperationMappings()),
		})
		providersCount++
	}

	// Register Confluence provider
	// Default domain can be overridden via configuration
	confluenceProvider := confluence.NewConfluenceProvider(logger, "devmesh")
	if err := registry.RegisterProvider(confluenceProvider); err != nil {
		logger.Error("Failed to register Confluence provider", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail initialization if one provider fails
		// return err
	} else {
		logger.Info("Registered Confluence provider", map[string]interface{}{
			"provider":        "confluence",
			"tools":           len(confluenceProvider.GetToolDefinitions()),
			"enabled_modules": confluenceProvider.GetEnabledModules(),
			"operations":      len(confluenceProvider.GetOperationMappings()),
		})
		providersCount++
	}

	// TODO: Register additional providers
	// - Azure DevOps provider
	// - CircleCI provider
	// - Jenkins provider

	logger.Info("Standard tool providers initialized", map[string]interface{}{
		"count": providersCount,
	})

	return nil
}
