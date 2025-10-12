package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/coder/websocket"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/config"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/mcp"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/metrics"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/platform"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools/builtin"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tracing"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	version = "1.0.0"
	commit  = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/config.yaml", "Path to configuration file")
		port        = flag.Int("port", 0, "Port to listen on (0 for stdio mode)")
		apiKey      = flag.String("api-key", "", "API key for authentication")
		coreURL     = flag.String("core-url", "", "Core Platform URL for advanced features")
		showVersion = flag.Bool("version", false, "Show version information")
		logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		stdioMode   = flag.Bool("stdio", false, "Run in stdio mode for Claude Code")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Edge MCP v%s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Check if we're in stdio mode early to suppress logs
	isStdioMode := *stdioMode || *port == 0

	// Initialize logger with appropriate level for mode
	logger := observability.NewStandardLogger("edge-mcp")

	// Set log level based on flag or mode
	levelMap := map[string]observability.LogLevel{
		"debug": observability.LogLevelDebug,
		"info":  observability.LogLevelInfo,
		"warn":  observability.LogLevelWarn,
		"error": observability.LogLevelError,
	}

	// In stdio mode, only log errors by default unless explicitly set
	if isStdioMode && *logLevel == "info" {
		// Suppress most logs in stdio mode
		if stdLogger, ok := logger.(*observability.StandardLogger); ok {
			logger = stdLogger.WithLevel(observability.LogLevelError)
		}
	} else if level, ok := levelMap[*logLevel]; ok {
		if stdLogger, ok := logger.(*observability.StandardLogger); ok {
			logger = stdLogger.WithLevel(level)
		}
	}

	// Only log platform info if not in stdio mode or if debug level
	if !isStdioMode || *logLevel == "debug" {
		platformInfo := platform.GetInfo()
		logger.Info("Edge MCP starting", map[string]interface{}{
			"version":      version,
			"platform":     platformInfo.OS,
			"architecture": platformInfo.Architecture,
			"go_version":   platformInfo.Version,
			"hostname":     platformInfo.Hostname,
			"capabilities": platformInfo.Capabilities,
		})
	}

	// Load configuration
	cfg, err := config.Load(*configFile)
	if err != nil {
		logger.Warn("Could not load config file, using defaults", map[string]interface{}{
			"error": err.Error(),
		})
		cfg = config.Default()
	}

	// Override with command line flags
	if *apiKey != "" {
		cfg.Auth.APIKey = *apiKey
	}
	if *coreURL != "" {
		cfg.Core.URL = *coreURL
	}
	// Set port from flag or use default for WebSocket mode
	if *port != 0 {
		cfg.Server.Port = *port
	} else if !*stdioMode {
		// If port is 0 and stdio mode not explicitly set, use default port for backward compatibility
		cfg.Server.Port = 8082
	}

	// Initialize distributed tracing (optional)
	// Tracing is disabled by default - enable via environment variables
	tracingConfig := &tracing.Config{
		Enabled:        os.Getenv("TRACING_ENABLED") == "true",
		ServiceName:    "edge-mcp",
		ServiceVersion: version,
		Environment:    getEnv("ENVIRONMENT", "development"),
		OTLPEndpoint:   getEnv("OTLP_ENDPOINT", "localhost:4317"),
		OTLPInsecure:   getEnv("OTLP_INSECURE", "true") == "true",
		ZipkinEndpoint: os.Getenv("ZIPKIN_ENDPOINT"),
		SamplingRate:   getSamplingRate(),
	}
	tracerProvider, err := tracing.NewTracerProvider(tracingConfig)
	if err != nil {
		logger.Warn("Could not initialize tracing", map[string]interface{}{
			"error": err.Error(),
		})
		tracerProvider = nil
	} else if tracingConfig.Enabled {
		logger.Info("Initialized distributed tracing", map[string]interface{}{
			"service":       tracingConfig.ServiceName,
			"version":       tracingConfig.ServiceVersion,
			"sampling_rate": tracingConfig.SamplingRate,
		})
	}

	// Initialize in-memory cache (no Redis/DB dependencies)
	memCache := cache.NewMemoryCache(1000, 5*time.Minute)

	// Wrap cache with tracing if tracer is available
	if tracerProvider != nil && tracingConfig.Enabled {
		spanHelper := tracing.NewSpanHelper(tracerProvider)
		memCache = cache.NewTracedCache(memCache, spanHelper)
	}

	// Initialize Core Platform client (optional)
	var coreClient *core.Client
	if cfg.Core.URL != "" {
		coreClient = core.NewClient(
			cfg.Core.URL,
			cfg.Core.APIKey,
			cfg.Core.EdgeMCPID,
			logger,
			tracerProvider,
		)

		// Authenticate with Core Platform
		if err := coreClient.AuthenticateWithCore(context.Background()); err != nil {
			logger.Warn("Could not authenticate with Core Platform, running in standalone mode", map[string]interface{}{
				"error": err.Error(),
			})
			coreClient = nil
		}
	}

	// Initialize authentication
	authenticator := auth.NewEdgeAuthenticator(cfg.Auth.APIKey)

	// Initialize Prometheus metrics
	metricsCollector := metrics.New()
	logger.Info("Initialized Prometheus metrics", nil)

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()

	// Register built-in MCP tools for agent orchestration
	// These tools provide core functionality for DevMesh operations
	// Context provider gets Core Platform client for delegation (if available)
	var contextProvider interface{ GetDefinitions() []tools.ToolDefinition }
	if coreClient != nil {
		contextProvider = builtin.NewContextProviderWithClient(coreClient)
	} else {
		contextProvider = builtin.NewContextProvider()
	}

	builtinProviders := []interface{ GetDefinitions() []tools.ToolDefinition }{
		builtin.NewAgentProvider(),
		builtin.NewWorkflowProvider(),
		builtin.NewTaskProvider(),
		contextProvider,               // Context provider with optional Core Platform delegation
		builtin.NewTemplateProvider(), // Workflow templates for common patterns
	}

	for _, provider := range builtinProviders {
		toolRegistry.Register(provider)
	}
	logger.Info("Registered built-in tools", map[string]interface{}{
		"count":        toolRegistry.Count(),
		"context_mode": map[bool]string{true: "core_platform", false: "standalone"}[coreClient != nil],
	})

	// Command executor no longer needed since we're not using local tools
	// All execution happens via remote tools through the Core Platform

	// No longer registering local tools - only use dynamic tools from Core Platform
	// This ensures all tools come from the REST API for consistency
	// Removed: FileSystemTool, GitTool, DockerTool, ShellTool

	// Fetch and register remote tools from Core Platform
	if coreClient != nil {
		remoteTools, err := coreClient.FetchRemoteTools(context.Background())
		if err != nil {
			logger.Warn("Could not fetch remote tools", map[string]interface{}{
				"error": err.Error(),
			})
		} else {
			for _, tool := range remoteTools {
				toolRegistry.RegisterRemote(tool)
			}
			logger.Info("Registered remote tools", map[string]interface{}{
				"count": len(remoteTools),
			})
		}
	}

	// Initialize MCP handler
	// Story 6.3: Edge-MCP delegates semantic context operations to Core Platform (REST API)
	// which has the full semantic context manager with database and embedding services.
	// This keeps Edge-MCP lightweight without database dependencies.
	mcpHandler := mcp.NewHandler(
		toolRegistry,
		memCache,
		coreClient,
		authenticator,
		logger,
		metricsCollector,
		tracerProvider,
		nil, // semanticContextMgr - Edge-MCP delegates to Core Platform instead
	)

	// Check if we should run in stdio mode
	if isStdioMode {
		// Run in stdio mode for Claude Code integration
		// Only log if debug mode
		if *logLevel == "debug" {
			logger.Info("Edge MCP starting in stdio mode", map[string]interface{}{
				"version": version,
			})
			if coreClient != nil {
				logger.Info("Connected to Core Platform", map[string]interface{}{
					"url": cfg.Core.URL,
				})
			} else {
				logger.Info("Running in standalone mode (no Core Platform connection)", nil)
			}
		}

		// Handle stdio communication
		mcpHandler.HandleStdio()
		return
	}

	// Setup HTTP server with Gin for WebSocket mode
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Initialize health checker
	healthChecker := api.NewHealthChecker(
		toolRegistry,
		memCache,
		coreClient,
		mcpHandler,
		logger,
		version,
	)

	// Set config and authenticator for startup validation
	healthChecker.SetConfig(cfg)
	healthChecker.SetAuthenticator(authenticator)

	// Register health check endpoints
	healthChecker.RegisterRoutes(router)

	// Legacy health check endpoint for backward compatibility
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        version,
			"core_connected": coreClient != nil,
		})
	})

	// Prometheus metrics endpoint
	router.GET("/metrics", gin.WrapH(promhttp.Handler()))

	// MCP WebSocket endpoint
	router.GET("/ws", func(c *gin.Context) {
		// Authenticate request
		if !authenticator.AuthenticateRequest(c.Request) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			return
		}

		// Accept WebSocket connection using coder/websocket
		conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{
			OriginPatterns: []string{"*"}, // Allow all origins for local development
			// Enable compression to handle large tool lists
			// This compresses messages >128 bytes, which will reduce our 143KB tools list to ~30KB
			CompressionMode: websocket.CompressionContextTakeover,
		})
		if err != nil {
			logger.Error("WebSocket upgrade failed", map[string]interface{}{
				"error": err.Error(),
			})
			return
		}
		defer func() {
			_ = conn.Close(websocket.StatusNormalClosure, "")
		}()

		// Handle MCP connection
		// Note: Compression is enabled above, so large tool lists will be compressed
		// from ~143KB to ~30KB, fitting within default WebSocket limits
		mcpHandler.HandleConnection(conn, c.Request)
	})

	// Start server
	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.Server.Port),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second, // Prevent Slowloris attacks
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown
	shutdownChan := make(chan struct{})
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigChan

		logger.Info("Received shutdown signal", map[string]interface{}{
			"signal": sig.String(),
		})

		// Total shutdown timeout: 30 seconds
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer shutdownCancel()

		// Step 1: Drain active MCP connections and complete in-flight requests (15s timeout)
		logger.Info("Draining active connections", nil)
		handlerCtx, handlerCancel := context.WithTimeout(shutdownCtx, 15*time.Second)
		if err := mcpHandler.Shutdown(handlerCtx); err != nil {
			logger.Error("Handler shutdown error", map[string]interface{}{
				"error": err.Error(),
			})
		}
		handlerCancel()

		// Step 2: Stop accepting new HTTP connections and wait for active requests (10s timeout)
		logger.Info("Shutting down HTTP server", nil)
		serverCtx, serverCancel := context.WithTimeout(shutdownCtx, 10*time.Second)
		if err := srv.Shutdown(serverCtx); err != nil {
			logger.Error("Server shutdown error", map[string]interface{}{
				"error": err.Error(),
			})
		}
		serverCancel()

		// Step 3: Close cache if it implements io.Closer
		logger.Info("Closing cache", nil)
		if closer, ok := memCache.(interface{ Close() error }); ok {
			if err := closer.Close(); err != nil {
				logger.Warn("Cache close error", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

		// Step 4: Flush metrics and shutdown tracing (5s timeout)
		if tracerProvider != nil && tracingConfig.Enabled {
			logger.Info("Flushing traces", nil)
			tracerCtx, tracerCancel := context.WithTimeout(shutdownCtx, 5*time.Second)
			if err := tracerProvider.Shutdown(tracerCtx); err != nil {
				logger.Warn("Tracer shutdown error", map[string]interface{}{
					"error": err.Error(),
				})
			}
			tracerCancel()
		}

		logger.Info("Shutdown complete", nil)
		close(shutdownChan)
	}()

	// Mark startup as complete with metrics
	builtinToolCount := toolRegistry.Count()
	remoteToolCount := 0
	if coreClient != nil {
		// Remote tools were already registered, get the count
		remoteTools, _ := coreClient.FetchRemoteTools(context.Background())
		remoteToolCount = len(remoteTools)
	}

	healthChecker.MarkStartupComplete(map[string]interface{}{
		"builtin_tools":   builtinToolCount - remoteToolCount,
		"remote_tools":    remoteToolCount,
		"total_tools":     builtinToolCount,
		"core_connected":  coreClient != nil,
		"cache_ready":     true,
		"auth_configured": true,
		"version":         version,
	})

	logger.Info("Edge MCP starting", map[string]interface{}{
		"version": version,
		"port":    cfg.Server.Port,
	})
	if coreClient != nil {
		logger.Info("Connected to Core Platform", map[string]interface{}{
			"url": cfg.Core.URL,
		})
	} else {
		logger.Info("Running in standalone mode (no Core Platform connection)", nil)
	}

	// Start server in goroutine so we can wait for shutdown
	errChan := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	// Wait for either server error or shutdown completion
	select {
	case err := <-errChan:
		logger.Fatal("Server failed to start", map[string]interface{}{
			"error": err.Error(),
		})
	case <-shutdownChan:
		// Graceful shutdown completed
	}
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getSamplingRate returns sampling rate from environment or default
func getSamplingRate() float64 {
	if rate := os.Getenv("TRACING_SAMPLING_RATE"); rate != "" {
		if parsed, err := parseFloat(rate); err == nil {
			return parsed
		}
	}
	return 1.0 // Default to sampling all traces
}

// parseFloat parses a float64 from string
func parseFloat(s string) (float64, error) {
	var f float64
	_, err := fmt.Sscanf(s, "%f", &f)
	return f, err
}
