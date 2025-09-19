package api

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/events"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/proxies"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/tools"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/api/websocket"
	"github.com/developer-mesh/developer-mesh/apps/mcp-server/internal/core"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/client/rest"
	"github.com/developer-mesh/developer-mesh/pkg/clients"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	commonLogging "github.com/developer-mesh/developer-mesh/pkg/common/logging"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/protocol/adaptive"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/agent"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	pgservices "github.com/developer-mesh/developer-mesh/pkg/services"
	pkgtools "github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// Global shutdown hooks
var shutdownHooks []func()

// Server represents the API server
type Server struct {
	router            *gin.Engine
	server            *http.Server
	engine            *core.Engine
	config            Config
	logger            observability.Logger
	loggerAdapter     *commonLogging.Logger // For compatibility with old code
	loggerObsAdapter  observability.Logger  // Adapter that wraps commonLogging.Logger as observability.Logger
	db                *sqlx.DB
	metrics           observability.MetricsClient
	metricsAdapter    observability.Client // For compatibility with old code
	cfg               *config.Config
	restClientFactory *rest.Factory // REST API client factory for communication with REST API
	authService       *auth.Service
	authMiddleware    *auth.AuthMiddleware // Enhanced auth with rate limiting, metrics, and audit
	cache             cache.Cache
	restAPIBaseURL    string // REST API base URL from MCPServer config
	// API proxies that delegate to REST API
	vectorAPIProxy  repository.VectorAPIRepository // Proxy for vector operations
	agentAPIProxy   agent.Repository               // Proxy for agent operations
	modelAPIProxy   repository.ModelRepository     // Proxy for model operations
	contextAPIProxy repository.ContextRepository   // Proxy for context operations
	searchAPIProxy  repository.SearchRepository    // Proxy for search operations
	// WebSocket server
	wsServer *websocket.Server
	// REST API client for proxying tool requests
	restAPIClient clients.RESTAPIClient
	// Multi-agent services
	taskService      pgservices.TaskService
	workflowService  pgservices.WorkflowService
	workspaceService pgservices.WorkspaceService
	documentService  pgservices.DocumentService
	conflictService  pgservices.ConflictResolutionService
	// Dynamic tools
	dynamicToolsAPI      *DynamicToolsAPI
	dynamicToolsV2       *DynamicToolsV2Wrapper // New implementation
	healthCheckScheduler *pkgtools.HealthCheckScheduler
	encryptionService    *security.EncryptionService
	// MCP Protocol handler
	mcpProtocolHandler *MCPProtocolHandler
	// Adaptive Protocol Intelligence Layer
	apisIntegration *adaptive.APISIntegration
}

// NewServer creates a new API server
func NewServer(engine *core.Engine, cfg Config, db *sqlx.DB, cacheClient cache.Cache, metrics observability.MetricsClient, config *config.Config) *Server {
	// Create adapter objects for compatibility with existing code
	loggerAdapter := observability.NewLoggerAdapter(observability.DefaultLogger)
	// Create an adapter that wraps commonLogging.Logger as observability.Logger
	loggerObsAdapter := observability.NewCommonLoggerAdapter(loggerAdapter)
	metricsAdapter := observability.NewMetricsAdapter(metrics)
	// Defensive: fail fast if db is nil
	if db == nil {
		panic("[api.NewServer] FATAL: received nil *sqlx.DB. Check database initialization before calling NewServer.")
	}

	router := gin.New()

	// Add middleware
	router.Use(gin.Recovery())
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
	router.Use(TracingMiddleware())      // Add request tracing

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

	// Initialize observability if not already done
	if observability.DefaultLogger == nil {
		observability.DefaultLogger = observability.NewStandardLogger("mcp-server")
	}

	// Initialize auth service with cache
	authConfig := auth.DefaultConfig()
	authConfig.JWTSecret = cfg.Auth.JWTSecret
	authConfig.EnableAPIKeys = true
	authConfig.EnableJWT = true

	// Create auth service with cache
	authService := auth.NewService(authConfig, db, cacheClient, observability.DefaultLogger)

	// Setup enhanced authentication with rate limiting, metrics, and audit logging
	authMiddleware, err := auth.SetupAuthentication(db, cacheClient, observability.DefaultLogger, metrics)
	if err != nil {
		observability.DefaultLogger.Error("Failed to setup enhanced authentication", map[string]interface{}{
			"error": err.Error(),
		})
		// Fall back to basic auth service
		authMiddleware = nil
	}

	// Initialize API keys from configuration
	if cfg.Auth.APIKeys != nil {
		fmt.Printf("API Keys from config: %+v\n", cfg.Auth.APIKeys)

		// Use the new method that handles full configuration including tenant IDs
		if apiKeys, ok := cfg.Auth.APIKeys.(map[string]interface{}); ok {
			authService.InitializeAPIKeysWithConfig(apiKeys)
		} else {
			// Fall back to simple string map if needed
			if apiKeys, ok := cfg.Auth.APIKeys.(map[string]string); ok {
				authService.InitializeDefaultAPIKeys(apiKeys)
			}
		}
	} else {
		fmt.Println("No API keys defined in config")
	}

	// JWT is now handled by the auth service configuration

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

	// Initialize logger
	// We don't initialize the vector database directly anymore
	// Vector operations are now handled by the REST API client
	var restClientFactory *rest.Factory
	var vectorProxy repository.VectorAPIRepository
	var agentProxy agent.Repository
	var modelProxy repository.ModelRepository

	// Use MCPServer config if available, otherwise fall back to default RestAPI config
	restAPIBaseURL := cfg.RestAPI.BaseURL
	restAPIKey := cfg.RestAPI.APIKey
	restAPIEnabled := cfg.RestAPI.Enabled

	if config != nil && config.MCPServer != nil && config.MCPServer.RestAPI.Enabled {
		restAPIBaseURL = config.MCPServer.RestAPI.BaseURL
		restAPIKey = config.MCPServer.RestAPI.APIKey
		restAPIEnabled = config.MCPServer.RestAPI.Enabled
	}

	if restAPIEnabled {
		// Create the REST client factory
		restClientFactory = rest.NewFactory(
			restAPIBaseURL,
			restAPIKey,
			observability.DefaultLogger,
		)

		// Create proxies that implement repository interfaces but delegate to REST API
		vectorProxy = proxies.NewVectorAPIProxy(restClientFactory, observability.DefaultLogger)
		agentProxy = proxies.NewAgentAPIProxy(restClientFactory, observability.DefaultLogger)
		modelProxy = proxies.NewModelAPIProxy(restClientFactory, observability.DefaultLogger)

		observability.DefaultLogger.Info("REST API client initialized", map[string]interface{}{
			"base_url": restAPIBaseURL,
			"timeout":  cfg.RestAPI.Timeout,
		})
	} else {
		observability.DefaultLogger.Warn("REST API client disabled - data operations will not be available", nil)
	}

	// Create the server instance
	s := &Server{
		router: router,
		server: &http.Server{
			Handler:           router,
			ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
		},
		engine:            engine,
		config:            cfg, // Set the API config
		logger:            observability.DefaultLogger,
		loggerAdapter:     loggerAdapter,
		loggerObsAdapter:  loggerObsAdapter,
		db:                db,
		cache:             cacheClient,
		metrics:           metrics,
		metricsAdapter:    metricsAdapter,
		cfg:               config,
		restClientFactory: restClientFactory,
		authService:       authService,
		authMiddleware:    authMiddleware,
		restAPIBaseURL:    restAPIBaseURL, // Store the REST API URL
		// Store the proxies
		vectorAPIProxy: vectorProxy,
		agentAPIProxy:  agentProxy,
		modelAPIProxy:  modelProxy,
	}

	// Initialize WebSocket server first if enabled (needed for MCP handler)
	if cfg.WebSocket.Enabled {
		wsConfig := websocket.Config{
			MaxConnections:  cfg.WebSocket.MaxConnections,
			ReadBufferSize:  cfg.WebSocket.ReadBufferSize,
			WriteBufferSize: cfg.WebSocket.WriteBufferSize,
			PingInterval:    cfg.WebSocket.PingInterval,
			PongTimeout:     cfg.WebSocket.PongTimeout,
			MaxMessageSize:  cfg.WebSocket.MaxMessageSize,
			Security:        cfg.WebSocket.Security,
			RateLimit:       cfg.WebSocket.RateLimit,
		}

		s.wsServer = websocket.NewServer(authService, metrics, observability.DefaultLogger, wsConfig)
		observability.DefaultLogger.Info("WebSocket server initialized", nil)
	}

	// Initialize MCP protocol handler if REST API is enabled
	if restAPIEnabled && restClientFactory != nil {
		restAPIClient := clients.NewRESTAPIClient(clients.RESTClientConfig{
			BaseURL: restAPIBaseURL,
			APIKey:  restAPIKey,
			Logger:  observability.DefaultLogger,
		})

		// Create MCP handler with server reference if WebSocket server exists
		if s.wsServer != nil {
			s.mcpProtocolHandler = NewMCPProtocolHandlerWithServer(restAPIClient, observability.DefaultLogger, s.wsServer)
		} else {
			s.mcpProtocolHandler = NewMCPProtocolHandler(restAPIClient, observability.DefaultLogger)
		}
		observability.DefaultLogger.Info("MCP protocol handler initialized", nil)

		// Set MCP handler on WebSocket server if both are available
		if s.wsServer != nil {
			s.wsServer.SetMCPHandler(s.mcpProtocolHandler)

			// Initialize APIL if enabled
			if os.Getenv("ENABLE_APIL") != "" {
				// Create adapters for APIL integration
				mcpAdapter := NewMCPHandlerAdapter(s.mcpProtocolHandler)
				wsAdapter := NewWebSocketServerAdapter(s.wsServer)

				s.apisIntegration = adaptive.NewAPISIntegration(
					observability.DefaultLogger,
					metrics,
					mcpAdapter,
					wsAdapter,
				)

				if err := s.apisIntegration.Initialize(context.Background()); err != nil {
					observability.DefaultLogger.Error("Failed to initialize APIL", map[string]interface{}{
						"error": err.Error(),
					})
				} else {
					observability.DefaultLogger.Info("APIL initialized successfully", nil)

					// Start migration if configured
					if os.Getenv("APIL_AUTO_MIGRATE") == "true" {
						if err := s.apisIntegration.StartMigration(context.Background()); err != nil {
							observability.DefaultLogger.Error("Failed to start APIL migration", map[string]interface{}{
								"error": err.Error(),
							})
						} else {
							observability.DefaultLogger.Info("APIL migration started", nil)
						}
					}
				}
			}
		}

		// Set dependencies (will be properly implemented in full integration)
		if engine != nil {
			// Use adapter to bridge the interface differences
			contextAdapter := websocket.NewContextManagerAdapter(engine.GetContextManager())
			s.wsServer.SetContextManager(contextAdapter)

			// Initialize and set tool registry
			toolRegistry := tools.NewRegistry(observability.DefaultLogger)
			if err := toolRegistry.RegisterBuiltinTools(); err != nil {
				observability.DefaultLogger.Error("Failed to register builtin tools", map[string]interface{}{
					"error": err.Error(),
				})
			}
			toolRegistryAdapter := NewToolRegistryAdapter(toolRegistry)
			s.wsServer.SetToolRegistry(toolRegistryAdapter)

			// Initialize and set event bus
			eventBus := events.NewBus(observability.DefaultLogger, metrics)
			eventBusAdapter := NewEventBusAdapter(eventBus)
			s.wsServer.SetEventBus(eventBusAdapter)

			observability.DefaultLogger.Info("Tool registry and event bus initialized", map[string]interface{}{
				"tools_count": len(toolRegistry.List()),
			})
		}

		observability.DefaultLogger.Info("WebSocket server initialized", map[string]interface{}{
			"enabled":         true,
			"max_connections": cfg.WebSocket.MaxConnections,
		})
	}

	s.server.Addr = cfg.ListenAddress
	s.server.ReadTimeout = cfg.ReadTimeout
	s.server.WriteTimeout = cfg.WriteTimeout
	s.server.IdleTimeout = cfg.IdleTimeout

	return s
}

// initializeDynamicTools initializes the dynamic tools subsystem
func (s *Server) initializeDynamicTools(ctx context.Context) error {
	// Create encryption service
	masterKey := os.Getenv("ENCRYPTION_MASTER_KEY")
	if masterKey == "" {
		// Generate a random key if not provided, but log a warning
		randomKey, err := security.GenerateSecureToken(32)
		if err != nil {
			s.logger.Error("Failed to generate encryption key", map[string]interface{}{"error": err})
			return fmt.Errorf("encryption key not provided and failed to generate: %w", err)
		}
		masterKey = randomKey
		s.logger.Warn("ENCRYPTION_MASTER_KEY not set - using randomly generated key. This is not suitable for production!", map[string]interface{}{
			"recommendation": "Set ENCRYPTION_MASTER_KEY environment variable with a secure 32+ character key",
		})
	}
	s.encryptionService = security.NewEncryptionService(masterKey)

	// Create health check manager
	// Create cache for health check manager
	healthCache := cache.NewNoOpCache()

	// Create OpenAPI adapter as the handler
	openAPIAdapter := adapters.NewOpenAPIAdapter(s.logger)

	// Create metrics client (using noop for now)
	metricsClient := observability.NewNoOpMetricsClient()

	healthCheckManager := pkgtools.NewHealthCheckManager(healthCache, openAPIAdapter, s.logger, metricsClient)

	// Create health check database implementation
	healthCheckDB := NewHealthCheckDBImpl(s.db.DB, s.encryptionService)

	// Create health check scheduler
	healthCheckInterval := 5 * time.Minute // Default health check interval
	s.healthCheckScheduler = pkgtools.NewHealthCheckScheduler(
		healthCheckManager,
		healthCheckDB,
		s.logger,
		healthCheckInterval,
	)

	// Create dynamic tool service
	dynamicToolService := NewDynamicToolService(s.db.DB, s.logger, s.metrics, s.encryptionService)

	// Create audit logger
	auditLogger := auth.NewAuditLogger(s.logger)

	// Create dynamic tools API
	s.dynamicToolsAPI = NewDynamicToolsAPI(
		dynamicToolService,
		s.logger,
		s.metrics,
		s.encryptionService,
		healthCheckManager,
		auditLogger,
	)

	// Start health check scheduler
	if err := s.healthCheckScheduler.Start(ctx); err != nil {
		return fmt.Errorf("failed to start health check scheduler: %w", err)
	}

	s.logger.Info("Dynamic tools subsystem initialized successfully", map[string]interface{}{
		"health_check_interval": healthCheckInterval.String(),
	})

	return nil
}

// Initialize initializes all components and routes
func (s *Server) Initialize(ctx context.Context) error {
	// Setup HTTP server if not done already
	if s.server == nil {
		s.server = &http.Server{
			Addr:              s.config.ListenAddress,
			Handler:           s.router,
			ReadHeaderTimeout: 5 * time.Second, // Prevent Slowloris attacks
		}
	}

	// Initialize API proxies
	if s.restAPIBaseURL != "" {
		// Context repository initialization
		if s.contextAPIProxy == nil {
			// Create a database-backed context repository
			s.contextAPIProxy = proxies.NewContextRepositoryProxy(s.db, s.logger)
			s.logger.Info("Initialized Context Repository proxy", nil)
		}

		// Search repository initialization
		if s.searchAPIProxy == nil {
			// Create a database-backed search repository (embedding service can be added later)
			s.searchAPIProxy = proxies.NewSearchRepositoryProxy(s.db, nil, s.logger)
			s.logger.Info("Initialized Search Repository proxy", nil)
		}

	}

	// Initialize dynamic tools components
	if err := s.initializeDynamicTools(ctx); err != nil {
		s.logger.Error("Failed to initialize dynamic tools", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't fail server startup, but log the error
	}

	// Initialize new dynamic tools v2 implementation
	if os.Getenv("ENABLE_DYNAMIC_TOOLS_V2") != "" {
		dynamicToolsV2, err := InitializeDynamicToolsV2(ctx, s.db, s.logger)
		if err != nil {
			s.logger.Error("Failed to initialize dynamic tools v2", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			s.dynamicToolsV2 = NewDynamicToolsV2Wrapper(dynamicToolsV2, s.logger)
			s.logger.Info("Dynamic Tools V2 initialized successfully", nil)
		}
	}

	// Initialize routes
	s.setupRoutes()

	return nil
}

// SetRESTClient sets the REST API client for proxying tool requests
func (s *Server) SetRESTClient(client clients.RESTAPIClient) {
	if client != nil {
		s.restAPIClient = client
		s.logger.Info("REST API client configured for tool proxying", nil)

		// Pass the REST client to the WebSocket server if it exists
		if s.wsServer != nil {
			s.wsServer.SetRESTClient(client)
		}
	}
}

// setupRoutes sets up all API routes
func (s *Server) setupRoutes() {
	// MCP Server handles the Model Context Protocol (MCP) for AI agent interactions
	// It does NOT handle webhooks - all webhook traffic should be directed to the REST API service
	// All tools are dynamic and registered through the /api/v1/tools endpoints

	// Setup base routes
	s.router.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "MCP Server is running"})
	})
	s.router.GET("/health", s.healthHandler)

	// Metrics endpoint - public (no authentication required)
	s.router.GET("/metrics", s.metricsHandler)

	// Setup WebSocket endpoint
	s.logger.Info("WebSocket route registration check", map[string]interface{}{
		"enabled":      s.config.WebSocket.Enabled,
		"wsServer_nil": s.wsServer == nil,
	})

	if s.config.WebSocket.Enabled && s.wsServer != nil {
		// Convert gin handler to http.HandlerFunc
		s.router.GET("/ws", func(c *gin.Context) {
			s.wsServer.HandleWebSocket(c.Writer, c.Request)
		})
		s.logger.Info("WebSocket endpoint enabled at /ws", map[string]interface{}{
			"max_connections": s.config.WebSocket.MaxConnections,
		})
	} else {
		s.logger.Warn("WebSocket endpoint NOT enabled", map[string]interface{}{
			"enabled":      s.config.WebSocket.Enabled,
			"wsServer_nil": s.wsServer == nil,
		})
	}

	// Setup API documentation
	// Create API versioned routes
	baseURL := ""
	if s.config.ListenAddress != "" {
		baseURL = fmt.Sprintf("http://%s", s.config.ListenAddress)
	}

	url := ginSwagger.URL(fmt.Sprintf("%s/swagger/doc.json", baseURL))
	s.router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, url))

	// API v1 routes
	v1 := s.router.Group("/api/v1")

	// Use enhanced auth middleware if available, otherwise fall back to basic auth
	if s.authMiddleware != nil {
		// Use enhanced auth with rate limiting, metrics, and audit logging
		v1.Use(s.authMiddleware.GinMiddleware())
		s.logger.Info("Using enhanced authentication with rate limiting and audit logging", nil)
	} else {
		// Use passthrough-enabled auth middleware to support user token forwarding
		v1.Use(s.authService.GinMiddlewareWithPassthrough(auth.TypeAPIKey, auth.TypeJWT))
		s.logger.Info("Using authentication with passthrough token support", nil)
	}

	// Add credential extraction middleware
	v1.Use(auth.CredentialExtractionMiddleware(s.logger))

	// Add a simple v1 API info endpoint
	v1.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"version": "v1",
			"status":  "operational",
			"apis":    []string{"agent", "model", "vector", "embeddings", "mcp"},
		})
	})

	// Register MCP API routes
	if s.engine != nil && s.engine.GetContextManager() != nil {
		mcpAPI := NewMCPAPI(s.engine.GetContextManager())
		mcpAPI.RegisterRoutes(v1)
		s.logger.Info("MCP API routes registered", nil)
	} else {
		s.logger.Warn("MCP API not available - context manager not initialized", nil)
	}

	// Register WebSocket monitoring routes
	if s.config.WebSocket.Enabled && s.wsServer != nil {
		wsMonitoring := websocket.NewMonitoringEndpoints(s.wsServer)
		wsMonitoring.RegisterRoutes(v1)
		s.logger.Info("WebSocket monitoring routes registered", nil)
	}

	// Register APIL monitoring routes
	if s.apisIntegration != nil {
		// APIL status endpoint
		v1.GET("/apil/status", func(c *gin.Context) {
			status := s.apisIntegration.GetMigrationStatus()
			metrics := s.apisIntegration.GetMetrics()
			c.JSON(http.StatusOK, gin.H{
				"status":  status,
				"metrics": metrics,
			})
		})

		// APIL control endpoints
		v1.POST("/apil/routing/:action", func(c *gin.Context) {
			action := c.Param("action")
			switch action {
			case "enable":
				s.apisIntegration.EnableAdaptiveRouting(true)
				c.JSON(http.StatusOK, gin.H{"message": "Adaptive routing enabled"})
			case "disable":
				s.apisIntegration.EnableAdaptiveRouting(false)
				c.JSON(http.StatusOK, gin.H{"message": "Adaptive routing disabled"})
			default:
				c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid action"})
			}
		})

		s.logger.Info("APIL monitoring routes registered", nil)
	}

	// Register Embedding Proxy routes
	if s.restAPIBaseURL != "" {
		embeddingProxy := proxies.NewEmbeddingProxy(s.restAPIBaseURL, s.logger)
		embeddingProxy.RegisterRoutes(v1)
		s.logger.Info("Embedding proxy routes registered", map[string]any{
			"rest_api_url": s.restAPIBaseURL,
		})
	} else {
		s.logger.Warn("Embedding proxy not available - REST API not configured", nil)
	}

	// Register Dynamic Tools API routes
	if s.dynamicToolsAPI != nil {
		s.dynamicToolsAPI.RegisterRoutes(v1)
		s.logger.Info("Dynamic Tools API routes registered", nil)
	} else {
		s.logger.Warn("Dynamic Tools API not available - initialization may have failed", nil)
	}

	// Register Dynamic Tools V2 routes if enabled
	if s.dynamicToolsV2 != nil {
		s.dynamicToolsV2.RegisterRoutes(v1)
		s.logger.Info("Dynamic Tools V2 API routes registered", nil)
	}

	// Log API availability via proxies
	if s.restAPIBaseURL != "" {
		if s.agentAPIProxy != nil {
			s.logger.Info("Agent API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Agent API not available - REST API proxy not configured", nil)
		}

		if s.modelAPIProxy != nil {
			s.logger.Info("Model API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Model API not available - REST API proxy not configured", nil)
		}

		if s.vectorAPIProxy != nil {
			s.logger.Info("Vector API available via REST API proxy", nil)
		} else {
			s.logger.Warn("Vector API not available - REST API proxy not configured", nil)
		}
	} else {
		s.logger.Warn("REST API is disabled - API operations will not be available", nil)
	}
}

// GetListenAddress returns the configured listen address
func (s *Server) GetListenAddress() string {
	return s.server.Addr
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

	// Log that we're shutting down the server
	s.logger.Info("Shutting down MCP server", nil)

	// Stop health check scheduler
	if s.healthCheckScheduler != nil {
		s.logger.Info("Stopping health check scheduler", nil)
		s.healthCheckScheduler.Stop()
	}

	// Close WebSocket server if enabled
	if s.wsServer != nil {
		s.logger.Info("Closing WebSocket connections", nil)
		if err := s.wsServer.Close(); err != nil {
			s.logger.Error("Failed to close WebSocket server", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return s.server.Shutdown(ctx)
}

// healthHandler returns the health status of all components
func (s *Server) healthHandler(c *gin.Context) {
	health := s.engine.Health()

	// Check REST API client health
	if s.restAPIClient != nil {
		metrics := s.restAPIClient.GetMetrics()

		if metrics.Healthy {
			health["rest_api_client"] = "healthy"
		} else {
			health["rest_api_client"] = fmt.Sprintf("unhealthy (circuit_breaker: %s, failed_requests: %d)",
				metrics.CircuitBreakerState, metrics.FailedRequests)
			health["status"] = "degraded"
		}

		// Add detailed metrics if requested
		if c.Query("details") == "true" {
			health["rest_api_metrics"] = fmt.Sprintf(
				"total_requests=%d, successful=%d, failed=%d, cache_hits=%d, cache_misses=%d",
				metrics.TotalRequests, metrics.SuccessfulRequests, metrics.FailedRequests,
				metrics.CacheHits, metrics.CacheMisses,
			)
		}
	} else if s.restClientFactory != nil {
		health["rest_api_client"] = "healthy"
	} else {
		health["rest_api_client"] = "unavailable"
	}

	// Check database tables if detailed health check is requested
	includeDetails := c.Query("details") == "true"
	var tableHealth *DatabaseHealthResult

	if includeDetails && s.db != nil {
		tableChecker := NewTableHealthChecker(s.db)
		ctx := c.Request.Context()

		// Check database connectivity first
		if err := tableChecker.CheckDatabaseConnectivity(ctx); err != nil {
			health["database_tables"] = "unreachable"
		} else {
			// Check required tables
			var err error
			tableHealth, err = tableChecker.CheckRequiredTables(ctx)
			if err != nil {
				health["database_tables"] = fmt.Sprintf("error: %v", err)
			} else if tableHealth.Healthy {
				health["database_tables"] = "healthy"
			} else {
				health["database_tables"] = fmt.Sprintf("unhealthy: %d tables missing", tableHealth.MissingTables)
			}
		}
	}

	// Check if any component is unhealthy
	allHealthy := true
	for _, status := range health {
		// Consider "healthy" or any status starting with "healthy" (like "healthy (mock)") as healthy
		statusStr := fmt.Sprintf("%v", status)
		if statusStr != "healthy" && (len(statusStr) < 7 || statusStr[:7] != "healthy") {
			allHealthy = false
			break
		}
	}

	response := gin.H{
		"status":     "healthy",
		"components": health,
	}

	// Include detailed table health if requested
	if includeDetails && tableHealth != nil {
		response["database_health"] = tableHealth
	}

	if allHealthy {
		c.JSON(http.StatusOK, response)
	} else {
		response["status"] = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, response)
	}
}

// metricsHandler returns metrics for Prometheus (commented out - unused)
// func (s *Server) metricsHandler(c *gin.Context) {
// 	// Implementation depends on metrics client
// 	c.String(http.StatusOK, "# metrics data will be here")
// }

// getBaseURL extracts the base URL from the request for HATEOAS links (commented out - unused)
// func (s *Server) getBaseURL(c *gin.Context) string {
// 	scheme := "http"
// 	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
// 		scheme = "https"
// 	}
//
// 	host := c.Request.Host
// 	if forwardedHost := c.GetHeader("X-Forwarded-Host"); forwardedHost != "" {
// 		host = forwardedHost
// 	}
//
// 	return scheme + "://" + host
// }

// InjectServices injects services into the WebSocket server
func (s *Server) InjectServices(services interface{}) {
	s.logger.Info("InjectServices called", map[string]interface{}{
		"services_type": fmt.Sprintf("%T", services),
		"ws_server_nil": s.wsServer == nil,
	})

	if s.wsServer == nil {
		s.logger.Warn("Cannot inject services: WebSocket server is not initialized", nil)
		return
	}

	// Extract fields from the services struct using reflection
	// This allows us to handle any struct with the required service fields
	servicesValue := reflect.ValueOf(services).Elem()
	servicesType := servicesValue.Type()

	var taskService pgservices.TaskService
	var workflowService pgservices.WorkflowService
	var workspaceService pgservices.WorkspaceService
	var documentService pgservices.DocumentService
	var conflictService pgservices.ConflictResolutionService
	var agentRepo agent.Repository
	var cacheClient cache.Cache

	// Find and extract each service field
	for i := 0; i < servicesType.NumField(); i++ {
		field := servicesType.Field(i)
		fieldValue := servicesValue.Field(i)

		if !fieldValue.IsNil() {
			switch field.Name {
			case "TaskService":
				if ts, ok := fieldValue.Interface().(pgservices.TaskService); ok {
					taskService = ts
				}
			case "WorkflowService":
				if ws, ok := fieldValue.Interface().(pgservices.WorkflowService); ok {
					workflowService = ws
				}
			case "WorkspaceService":
				if ws, ok := fieldValue.Interface().(pgservices.WorkspaceService); ok {
					workspaceService = ws
				}
			case "DocumentService":
				if ds, ok := fieldValue.Interface().(pgservices.DocumentService); ok {
					documentService = ds
				}
			case "ConflictService":
				if cs, ok := fieldValue.Interface().(pgservices.ConflictResolutionService); ok {
					conflictService = cs
				}
			case "AgentRepository":
				if ar, ok := fieldValue.Interface().(agent.Repository); ok {
					agentRepo = ar
				}
			case "Cache":
				if cc, ok := fieldValue.Interface().(cache.Cache); ok {
					cacheClient = cc
				}
			}
		}
	}

	// Inject services into WebSocket server
	s.wsServer.SetServices(taskService, workflowService, workspaceService, documentService, conflictService, agentRepo, cacheClient)

	s.logger.Info("Services successfully injected into WebSocket server", map[string]interface{}{
		"task_service_nil":      taskService == nil,
		"workflow_service_nil":  workflowService == nil,
		"workspace_service_nil": workspaceService == nil,
		"document_service_nil":  documentService == nil,
		"conflict_service_nil":  conflictService == nil,
		"agent_repo_nil":        agentRepo == nil,
		"cache_nil":             cacheClient == nil,
		"has_task_service":      taskService != nil,
		"has_workflow_service":  workflowService != nil,
		"has_workspace_service": workspaceService != nil,
		"has_document_service":  documentService != nil,
		"has_conflict_service":  conflictService != nil,
		"has_agent_repo":        agentRepo != nil,
		"has_cache":             cacheClient != nil,
	})
}

// RegisterShutdownHook registers a function to be called during server shutdown
func RegisterShutdownHook(hook func()) {
	shutdownHooks = append(shutdownHooks, hook)
}

// metricsHandler returns metrics for Prometheus
func (s *Server) metricsHandler(c *gin.Context) {
	// Use the Prometheus handler
	handler := SetupPrometheusHandler()
	handler(c)
}

// SetMultiAgentServices sets the multi-agent collaboration services on the server
func (s *Server) SetMultiAgentServices(
	taskService pgservices.TaskService,
	workflowService pgservices.WorkflowService,
	workspaceService pgservices.WorkspaceService,
	documentService pgservices.DocumentService,
	conflictService pgservices.ConflictResolutionService,
) {
	s.taskService = taskService
	s.workflowService = workflowService
	s.workspaceService = workspaceService
	s.documentService = documentService
	s.conflictService = conflictService

	// Pass services to WebSocket server if it exists
	if s.wsServer != nil {
		// Pass nil for agent repository and cache since they're not available in this method
		// They should be passed through InjectServices instead
		s.wsServer.SetServices(taskService, workflowService, workspaceService, documentService, conflictService, nil, nil)
		s.logger.Info("Multi-agent services set on WebSocket server", map[string]interface{}{
			"taskService_nil":      taskService == nil,
			"workflowService_nil":  workflowService == nil,
			"workspaceService_nil": workspaceService == nil,
			"documentService_nil":  documentService == nil,
			"conflictService_nil":  conflictService == nil,
		})
	} else {
		s.logger.Warn("WebSocket server is nil, cannot set services", nil)
	}
}
