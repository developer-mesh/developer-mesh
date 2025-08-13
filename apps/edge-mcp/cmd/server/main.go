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
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/cache"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/config"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/executor"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/mcp"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/platform"
	"github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/gin-gonic/gin"
)

var (
	version = "1.0.0"
	commit  = "unknown"
)

func main() {
	var (
		configFile  = flag.String("config", "configs/config.yaml", "Path to configuration file")
		port        = flag.Int("port", 8082, "Port to listen on")
		apiKey      = flag.String("api-key", "", "API key for authentication")
		coreURL     = flag.String("core-url", "", "Core Platform URL for advanced features")
		showVersion = flag.Bool("version", false, "Show version information")
		logLevel    = flag.String("log-level", "info", "Log level (debug, info, warn, error)")
		workDir     = flag.String("work-dir", "", "Working directory for command execution")
	)
	flag.Parse()

	if *showVersion {
		fmt.Printf("Edge MCP v%s (commit: %s)\n", version, commit)
		os.Exit(0)
	}

	// Initialize logger
	logger := observability.NewStandardLogger("edge-mcp")

	// Log platform information at startup
	platformInfo := platform.GetInfo()
	logger.Info("Edge MCP starting", map[string]interface{}{
		"version":      version,
		"platform":     platformInfo.OS,
		"architecture": platformInfo.Architecture,
		"go_version":   platformInfo.Version,
		"hostname":     platformInfo.Hostname,
		"capabilities": platformInfo.Capabilities,
	})

	// Set log level based on flag
	levelMap := map[string]observability.LogLevel{
		"debug": observability.LogLevelDebug,
		"info":  observability.LogLevelInfo,
		"warn":  observability.LogLevelWarn,
		"error": observability.LogLevelError,
	}
	if level, ok := levelMap[*logLevel]; ok {
		if stdLogger, ok := logger.(*observability.StandardLogger); ok {
			logger = stdLogger.WithLevel(level)
		}
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
	if *port != 0 {
		cfg.Server.Port = *port
	}

	// Initialize in-memory cache (no Redis/DB dependencies)
	memCache := cache.NewMemoryCache(1000, 5*time.Minute)

	// Initialize Core Platform client (optional)
	var coreClient *core.Client
	if cfg.Core.URL != "" {
		coreClient = core.NewClient(
			cfg.Core.URL,
			cfg.Core.APIKey,
			cfg.Core.TenantID,
			cfg.Core.EdgeMCPID,
			logger,
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

	// Initialize tool registry
	toolRegistry := tools.NewRegistry()

	// Create command executor for tools
	if *workDir == "" {
		*workDir, _ = os.Getwd()
	}
	cmdExecutor := executor.NewCommandExecutor(logger, *workDir, 30*time.Second)

	// Register local tools with proper initialization
	toolRegistry.Register(tools.NewFileSystemTool(*workDir, logger))
	toolRegistry.Register(tools.NewGitTool(cmdExecutor, logger))
	toolRegistry.Register(tools.NewDockerTool(cmdExecutor, logger))
	toolRegistry.Register(tools.NewShellTool(cmdExecutor, logger))

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
	mcpHandler := mcp.NewHandler(
		toolRegistry,
		memCache,
		coreClient,
		authenticator,
		logger,
	)

	// Setup HTTP server with Gin
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(gin.Recovery())

	// Health check endpoint
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":         "healthy",
			"version":        version,
			"core_connected": coreClient != nil,
		})
	})

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
		mcpHandler.HandleConnection(conn, c.Request)
	})

	// Start server
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Server.Port),
		Handler: router,
	}

	// Graceful shutdown
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan

		logger.Info("Shutting down Edge MCP", nil)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := srv.Shutdown(ctx); err != nil {
			logger.Error("Server shutdown error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

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

	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Fatal("Server failed to start", map[string]interface{}{
			"error": err.Error(),
		})
	}
}
