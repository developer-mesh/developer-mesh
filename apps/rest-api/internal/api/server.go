package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/adapters"
	contextAPI "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/api/context"
	webhooksAPI "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/api/webhooks"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/services"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/storage"

	pkgrepository "github.com/developer-mesh/developer-mesh/pkg/repository"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Helper function to extract string from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// Helper function to get last N characters of a string
func lastN(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// Global shutdown hooks
var shutdownHooks []func()

// Server represents the API server
type Server struct {
	router         *gin.Engine
	server         *http.Server
	engine         *core.Engine
	config         Config
	logger         observability.Logger
	db             *sqlx.DB
	metrics        observability.MetricsClient
	cfg            *config.Config
	authMiddleware *auth.AuthMiddleware // Enhanced auth with rate limiting, metrics, and audit
	healthChecker  *HealthChecker
	cache          cache.Cache
	webhookRepo    pkgrepository.WebhookConfigRepository
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config, db *sqlx.DB, metrics observability.MetricsClient, config *config.Config, cacheClient cache.Cache) *Server {
	// Defensive: fail fast if db is nil
	if db == nil {
		panic("[api.NewServer] FATAL: received nil *sqlx.DB. Check database initialization before calling NewServer.")
	}

	// Initialize logger first
	logger := observability.NewLogger("api-server")

	router := gin.New()

	// Add middleware
	// Use custom recovery middleware for better error handling
	router.Use(CustomRecoveryMiddleware(logger))
	router.Use(RequestLogger())

	// Apply performance optimizations based on configuration
	if cfg.Performance.EnableCompression {
		router.Use(CompressionMiddleware()) // Add response compression
	}

	if cfg.Performance.EnableETagCaching {
		router.Use(CachingMiddleware()) // Add HTTP caching
	}

	router.Use(MetricsMiddleware())
	router.Use(ErrorHandlerMiddleware()) // Add centralized error handling
	// router.Use(TracingMiddleware())      // Add request tracing - TODO: Fix OpenTelemetry dependency

	// Apply API versioning
	router.Use(VersioningMiddleware(cfg.Versioning))

	if cfg.RateLimit.Enabled {
		limiterConfig := NewRateLimiterConfigFromConfig(cfg.RateLimit)
		router.Use(RateLimiter(limiterConfig))
	}

	// Enable CORS if configured
	if cfg.EnableCORS {
		corsConfig := CORSConfig{
			AllowedOrigins: []string{"*"}, // Default to allow all origins in development
		}
		router.Use(CORSMiddleware(corsConfig))
	}

	// Setup authentication configuration
	authConfig := &auth.AuthSystemConfig{
		Service: &auth.ServiceConfig{
			JWTSecret:         cfg.Auth.JWTSecret,
			JWTExpiration:     24 * time.Hour,
			APIKeyHeader:      "X-API-Key",
			EnableAPIKeys:     true,
			EnableJWT:         true,
			CacheEnabled:      false, // Disable cache for now
			MaxFailedAttempts: 5,
			LockoutDuration:   15 * time.Minute,
		},
		RateLimiter: auth.DefaultRateLimiterConfig(),
		APIKeys:     make(map[string]auth.APIKeySettings),
	}

	// Parse API keys from configuration
	if apiKeysRaw, ok := cfg.Auth.APIKeys.(map[string]interface{}); ok {
		if staticKeys, ok := apiKeysRaw["static_keys"].(map[string]interface{}); ok {
			for key, settings := range staticKeys {
				if settingsMap, ok := settings.(map[string]interface{}); ok {
					apiKeySettings := auth.APIKeySettings{
						Role:     getStringFromMap(settingsMap, "role"),
						TenantID: getStringFromMap(settingsMap, "tenant_id"),
					}

					// Parse scopes
					if scopesRaw, ok := settingsMap["scopes"].([]interface{}); ok {
						scopes := make([]string, 0, len(scopesRaw))
						for _, s := range scopesRaw {
							if scope, ok := s.(string); ok {
								scopes = append(scopes, scope)
							}
						}
						apiKeySettings.Scopes = scopes
					}

					authConfig.APIKeys[key] = apiKeySettings

					// Debug logging
					logger.Info("API Key from config", map[string]interface{}{
						"key_suffix": lastN(key, 8),
						"role":       apiKeySettings.Role,
						"tenant_id":  apiKeySettings.TenantID,
						"scopes":     apiKeySettings.Scopes,
					})
				}
			}
		}
	}

	// Set JWT secret environment variable if provided
	if cfg.Auth.JWTSecret != "" {
		if err := os.Setenv("JWT_SECRET", cfg.Auth.JWTSecret); err != nil {
			logger.Warn("Failed to set JWT_SECRET environment variable", map[string]interface{}{"error": err})
		}
	}

	// Use the cache client passed from main.go
	cacheImpl := cacheClient
	if cacheImpl == nil {
		logger.Warn("No cache client provided, using no-op cache", map[string]interface{}{})
		cacheImpl = cache.NewNoOpCache()
	}

	// Use the enhanced setup that gives us control over configuration
	authMiddleware, err := auth.SetupAuthenticationWithConfig(authConfig, db, cacheImpl, logger, metrics)
	if err != nil {
		logger.Error("Failed to setup enhanced authentication", map[string]interface{}{
			"error": err.Error(),
		})
		// Still panic as the function signature doesn't support error return
		panic("Failed to setup authentication: " + err.Error())
	}

	logger.Info("Enhanced authentication initialized", map[string]interface{}{
		"environment":    os.Getenv("ENVIRONMENT"),
		"api_key_source": os.Getenv("API_KEY_SOURCE"),
		"cache_enabled":  cacheImpl != nil,
	})

	// Configure HTTP client transport for external service calls
	httpTransport := &http.Transport{
		MaxIdleConns:          cfg.Performance.HTTPMaxIdleConns,
		MaxConnsPerHost:       cfg.Performance.HTTPMaxConnsPerHost,
		IdleConnTimeout:       cfg.Performance.HTTPIdleConnTimeout,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableCompression:    false,
		ForceAttemptHTTP2:     true,
	}

	// Create custom HTTP client with the optimized transport
	httpClient := &http.Client{
		Transport: httpTransport,
		Timeout:   60 * time.Second,
	}

	// Use the custom HTTP client for external service calls
	http.DefaultClient = httpClient

	// Initialize health checker
	healthChecker := NewHealthChecker(db)

	// Initialize webhook repository
	webhookRepo := pkgrepository.NewWebhookConfigRepository(db)

	server := &Server{
		router:         router,
		engine:         engine,
		config:         cfg,
		logger:         logger,
		db:             db,
		metrics:        metrics,
		cfg:            config,
		authMiddleware: authMiddleware,
		healthChecker:  healthChecker,
		cache:          cacheImpl,
		webhookRepo:    webhookRepo,
		server: &http.Server{
			Addr:         cfg.ListenAddress,
			Handler:      router,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			IdleTimeout:  cfg.IdleTimeout,
		},
	}

	return server
}

// Initialize initializes all components and routes
func (s *Server) Initialize(ctx context.Context) error {

	// Ensure we have a valid context manager
	if s.engine != nil {
		// Always create a context manager as follows:
		// 1. First check if one is already set
		// 2. If not, check the environment to determine if we should use a mock
		// 3. Create and set either a real or mock context manager
		// 4. Verify that it was correctly set before proceeding

		// Get current context manager (if any)
		ctxManager := s.engine.GetContextManager()

		// Set a new context manager if none exists
		if ctxManager == nil {
			// Check environment variable to determine whether to use mock or real
			useMock := os.Getenv("USE_MOCK_CONTEXT_MANAGER")

			s.logger.Info("Context manager not found, initializing new one", map[string]any{
				"use_mock": useMock,
			})

			if strings.ToLower(useMock) == "true" {
				// Create mock context manager for development/testing
				s.logger.Info("Using mock context manager as specified by environment", nil)
				ctxManager = core.NewMockContextManager()
			} else {
				// Use our production-ready context manager implementation
				s.logger.Info("Initializing production-ready context manager", nil)

				// Pass existing components to the context manager
				s.logger.Info("Creating production context manager", nil)

				// Create the production context manager with available components
				// We're using an updated version of NewContextManager that accepts *sqlx.DB directly
				ctxManager = core.NewContextManager(s.db, s.logger, s.metrics)
				s.logger.Info("Production context manager initialized", nil)
			}

			// Set the context manager on the engine
			s.engine.SetContextManager(ctxManager)

			// Log the change
			s.logger.Info("Context manager set on engine", nil)
		} else {
			s.logger.Info("Using existing context manager", nil)
		}

		// Explicitly verify that a context manager is set before continuing
		if verifyCtx := s.engine.GetContextManager(); verifyCtx == nil {
			s.logger.Error("Context manager initialization failed - still nil after setting", nil)
			return fmt.Errorf("failed to initialize context manager, engine reports nil after setting")
		} else {
			s.logger.Info("Context manager initialization confirmed successful", nil)
		}
	} else {
		s.logger.Error("Engine is nil, cannot initialize context manager", nil)
		return fmt.Errorf("engine is nil, cannot initialize context manager")
	}

	// Initialize routes
	s.setupRoutes(ctx)

	// Mark server as ready
	s.healthChecker.SetReady(true)
	s.logger.Info("Server initialization complete and ready to serve requests", nil)

	return nil
}

// setupRoutes initializes all API routes
func (s *Server) setupRoutes(ctx context.Context) {
	// Public endpoints
	// Health check endpoints
	s.router.GET("/health", s.healthChecker.HealthHandler)
	s.router.GET("/healthz", s.healthChecker.LivenessHandler) // Kubernetes liveness probe
	s.router.GET("/readyz", s.healthChecker.ReadinessHandler) // Kubernetes readiness probe

	// Swagger API documentation
	if s.config.EnableSwagger {
		s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	}

	// Metrics endpoint - public (no authentication required)
	s.router.GET("/metrics", s.metricsHandler)

	// API v1 routes - require authentication
	v1 := s.router.Group("/api/v1")

	// Always use enhanced auth middleware - it includes all authentication features
	v1.Use(s.authMiddleware.GinMiddleware())
	s.logger.Info("Using enhanced authentication with rate limiting and audit logging", nil)

	// Add tenant context extraction middleware AFTER authentication
	v1.Use(ExtractTenantContext())

	// Root endpoint to provide API entry points (HATEOAS)
	v1.GET("/", func(c *gin.Context) {
		// Check for authentication result set by AuthMiddleware
		user, exists := c.Get("user")
		if !exists || user == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "authentication required"})
			return
		}

		baseURL := s.getBaseURL(c)
		c.JSON(http.StatusOK, gin.H{
			"api_version": "1.0",
			"description": "MCP Server API for DevOps tool integration following Model Context Protocol",
			"links": map[string]string{
				"contexts":      baseURL + "/api/v1/contexts",
				"embeddings":    baseURL + "/api/embeddings",
				"health":        baseURL + "/health",
				"documentation": baseURL + "/swagger/index.html",
			},
		})
	})

	// Webhook routes are now handled by the dynamic webhook handler
	// GitHub-specific webhook handling has been removed in favor of the dynamic handler

	// Dynamic Tools API - Enhanced discovery and management
	patternRepo := storage.NewDiscoveryPatternRepository(s.db.DB)
	// Create encryption service with a secure key from environment or config
	encryptionKey := os.Getenv("DEVMESH_ENCRYPTION_KEY")
	if encryptionKey == "" {
		// Generate a random key if not provided, but log a warning
		randomKey, err := security.GenerateSecureToken(32)
		if err != nil {
			s.logger.Error("Failed to generate encryption key", map[string]interface{}{
				"error": err.Error(),
			})
			s.logger.Fatal("encryption key not provided and failed to generate", map[string]interface{}{
				"error": err.Error(),
			})
			panic("Failed to generate encryption key")
		}
		encryptionKey = randomKey
		s.logger.Warn("DEVMESH_ENCRYPTION_KEY not set - using randomly generated key. This is not suitable for production!", map[string]interface{}{
			"recommendation": "Set DEVMESH_ENCRYPTION_KEY environment variable with a secure 32+ character key",
		})
	}
	encryptionService := security.NewEncryptionService(encryptionKey)
	dynamicToolsService := services.NewDynamicToolsService(
		s.db,
		s.logger,
		s.metrics,
		encryptionService,
		patternRepo,
	)

	// Dynamic webhook routes for dynamic tools
	// Create webhook event repository
	webhookEventRepo := pkgrepository.NewWebhookEventRepository(s.db)

	// Create dynamic webhook handler
	dynamicWebhookHandler := webhooksAPI.NewDynamicWebhookHandler(
		dynamicToolsService.GetDynamicToolRepository(),
		webhookEventRepo,
		s.logger,
	)

	// Register dynamic webhook routes - no authentication for webhooks
	webhooks := s.router.Group("/api/webhooks")
	// Add middleware to inject request time
	webhooks.Use(func(c *gin.Context) {
		c.Set("request_time", time.Now())
		c.Next()
	})
	webhooks.POST("/tools/:toolId", dynamicWebhookHandler.HandleDynamicWebhook)
	dynamicToolsAPI := NewDynamicToolsAPI(
		dynamicToolsService,
		s.logger,
		s.metrics,
		auth.NewAuditLogger(s.logger),
	)
	dynamicToolsAPI.RegisterRoutes(v1)

	// Agent and Model APIs - create repositories first as they're needed by context API
	agentRepo := repository.NewAgentRepository(s.db.DB)
	agentAPI := NewAgentAPI(agentRepo)
	agentAPI.RegisterRoutes(v1)
	modelRepo := repository.NewModelRepository(s.db.DB)

	// Context API - register the context endpoints
	ctxAPI := contextAPI.NewAPI(
		s.engine.GetContextManager(),
		s.logger,
		s.metrics,
		s.db,
		modelRepo,
	)
	ctxAPI.RegisterRoutes(v1)
	modelAPI := NewModelAPI(modelRepo)
	modelAPI.RegisterRoutes(v1)

	// Embedding API v2 - Multi-agent embedding system
	// Initialize the embedding service with all configured providers
	embeddingService, embeddingErr := adapters.CreateEmbeddingService(s.cfg, *database.NewDatabaseWithConnection(s.db), s.cache)
	if embeddingErr != nil {
		s.logger.Error("Failed to create embedding service", map[string]any{
			"error": embeddingErr.Error(),
		})
		// Use mock or partial service if initialization fails
		s.logger.Warn("Embedding service initialization failed, some features may be limited", nil)
	} else {
		// Create agent repository and service using the PostgreSQL implementation
		agentPostgresRepo := agents.NewPostgresRepository(s.db, "mcp")
		agentService := agents.NewService(agentPostgresRepo)

		// Create and register embedding API
		embeddingAPI := NewEmbeddingAPI(embeddingService, agentService, s.logger)
		embeddingAPI.RegisterRoutes(v1)

		s.logger.Info("Embedding API v2 initialized successfully", nil)
	}
}

// Start starts the API server without TLS
func (s *Server) Start() error {
	// Start without TLS
	return s.server.ListenAndServe()
}

// StartTLS starts the API server with TLS
func (s *Server) StartTLS(certFile, keyFile string) error {
	// If specific files are provided, use those
	if certFile != "" && keyFile != "" {
		return s.server.ListenAndServeTLS(certFile, keyFile)
	}

	// Otherwise use the ones from config
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		return s.server.ListenAndServeTLS(s.config.TLSCertFile, s.config.TLSKeyFile)
	}

	// If no TLS files are available, return an error
	return nil
}

// Shutdown gracefully shuts down the API server
func (s *Server) Shutdown(ctx context.Context) error {
	// Execute all registered shutdown hooks
	for _, hook := range shutdownHooks {
		hook()
	}

	return s.server.Shutdown(ctx)
}

// healthHandler function removed - using health.HandleHealthCheck instead

// metricsHandler returns metrics for Prometheus
func (s *Server) metricsHandler(c *gin.Context) {
	// Use the Prometheus handler
	handler := SetupPrometheusHandler()
	handler(c)
}

// getBaseURL extracts the base URL from the request for HATEOAS links
func (s *Server) getBaseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}

	host := c.Request.Host
	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
		host = forwardedHost
	}

	return scheme + "://" + host
}

// RegisterShutdownHook registers a function to be called during server shutdown
func RegisterShutdownHook(hook func()) {
	shutdownHooks = append(shutdownHooks, hook)
}
