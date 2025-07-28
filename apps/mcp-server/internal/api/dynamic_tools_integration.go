package api

import (
	"context"
	"os"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/handlers"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/services"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
)

// InitializeDynamicToolsV2 initializes the new dynamic tools implementation
func InitializeDynamicToolsV2(
	ctx context.Context,
	db *sqlx.DB,
	logger observability.Logger,
) (*handlers.DynamicToolAPI, error) {
	// Get encryption key
	masterKey := os.Getenv("ENCRYPTION_MASTER_KEY")
	if masterKey == "" {
		masterKey = os.Getenv("ENCRYPTION_KEY")
	}
	if masterKey == "" {
		// For development only - in production this should fail
		logger.Warn("No encryption key provided, using temporary key for development", nil)
		masterKey = "temporary-development-key-change-in-production"
	}

	// Create credential manager
	credentialManager, err := services.NewCredentialManager(masterKey)
	if err != nil {
		return nil, err
	}

	// Create health checker
	healthChecker := services.NewHealthChecker(logger)
	healthChecker.StartPeriodicCleanup(ctx)

	// Create tool service
	toolService := services.NewToolService(db, credentialManager, logger)

	// Create tool registry
	toolRegistry := services.NewToolRegistry(toolService, healthChecker, logger)

	// Create retry handler
	retryHandler := services.NewRetryHandler(logger)

	// Create discovery service
	discoveryService := services.NewDiscoveryService(db, toolRegistry, logger)

	// Create execution service
	executionService := services.NewExecutionService(db, toolRegistry, retryHandler, logger)

	// Create API handler
	api := handlers.NewDynamicToolAPI(
		toolRegistry,
		discoveryService,
		executionService,
		logger,
	)

	return api, nil
}

// DynamicToolsV2Wrapper wraps the new implementation to work with existing infrastructure
type DynamicToolsV2Wrapper struct {
	api    *handlers.DynamicToolAPI
	logger observability.Logger
}

// NewDynamicToolsV2Wrapper creates a wrapper for the new dynamic tools API
func NewDynamicToolsV2Wrapper(api *handlers.DynamicToolAPI, logger observability.Logger) *DynamicToolsV2Wrapper {
	return &DynamicToolsV2Wrapper{
		api:    api,
		logger: logger,
	}
}

// RegisterRoutes registers the dynamic tools v2 routes
func (w *DynamicToolsV2Wrapper) RegisterRoutes(router *gin.RouterGroup) {
	// Register under a versioned path for migration
	v2 := router.Group("/tools/v2")
	w.api.RegisterRoutes(v2)

	// Also register under the main path if ENABLE_DYNAMIC_TOOLS_V2 is set
	if os.Getenv("ENABLE_DYNAMIC_TOOLS_V2") == "true" {
		w.api.RegisterRoutes(router)
		w.logger.Info("Dynamic Tools V2 enabled as primary implementation", nil)
	}
}

// MigrateFromV1 helps with migration from the old implementation
func (w *DynamicToolsV2Wrapper) MigrateFromV1(ctx context.Context) error {
	// This would contain migration logic if needed
	// For now, it's a placeholder
	w.logger.Info("Migration from V1 to V2 dynamic tools", map[string]interface{}{
		"status": "ready",
	})
	return nil
}
