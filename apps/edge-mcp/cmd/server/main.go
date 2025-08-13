package main

import (
    "context"
    "flag"
    "fmt"
    "log"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/auth"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/config"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/core"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/mcp"
    "github.com/developer-mesh/developer-mesh/apps/edge-mcp/internal/tools"
    "github.com/developer-mesh/developer-mesh/pkg/common/cache"
    "github.com/gin-gonic/gin"
    "github.com/coder/websocket"
)

var (
    version = "1.0.0"
    commit  = "unknown"
)

func main() {
    var (
        configFile = flag.String("config", "configs/config.yaml", "Path to configuration file")
        port       = flag.Int("port", 8082, "Port to listen on")
        apiKey     = flag.String("api-key", "", "API key for authentication")
        coreURL    = flag.String("core-url", "", "Core Platform URL for advanced features")
        showVersion = flag.Bool("version", false, "Show version information")
    )
    flag.Parse()

    if *showVersion {
        fmt.Printf("Edge MCP v%s (commit: %s)\n", version, commit)
        os.Exit(0)
    }

    // Load configuration
    cfg, err := config.Load(*configFile)
    if err != nil {
        log.Printf("Warning: Could not load config file: %v. Using defaults.", err)
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

    // Initialize components using existing pkg/common/cache
    memCache := cache.NewMemoryCache(1000, 5*time.Minute)
    
    // Initialize Core Platform client (optional)
    var coreClient *core.Client
    if cfg.Core.URL != "" {
        coreClient = core.NewClient(
            cfg.Core.URL,
            cfg.Core.APIKey,
            cfg.Core.TenantID,
            cfg.Core.EdgeMCPID,
        )
        
        // Authenticate with Core Platform
        if err := coreClient.AuthenticateWithCore(context.Background()); err != nil {
            log.Printf("Warning: Could not authenticate with Core Platform: %v. Running in standalone mode.", err)
            coreClient = nil
        }
    }

    // Initialize authentication
    authenticator := auth.NewEdgeAuthenticator(cfg.Auth.APIKey)

    // Initialize tool registry
    toolRegistry := tools.NewRegistry()
    
    // Register local tools
    toolRegistry.Register(tools.NewFileSystemTool())
    toolRegistry.Register(tools.NewGitTool())
    toolRegistry.Register(tools.NewDockerTool())
    toolRegistry.Register(tools.NewShellTool())
    
    // Fetch and register remote tools from Core Platform
    if coreClient != nil {
        remoteTools, err := coreClient.FetchRemoteTools(context.Background())
        if err != nil {
            log.Printf("Warning: Could not fetch remote tools: %v", err)
        } else {
            for _, tool := range remoteTools {
                toolRegistry.RegisterRemote(tool)
            }
        }
    }

    // Initialize MCP handler
    mcpHandler := mcp.NewHandler(
        toolRegistry,
        memCache,
        coreClient,
        authenticator,
    )

    // Setup HTTP server with Gin
    gin.SetMode(gin.ReleaseMode)
    router := gin.New()
    router.Use(gin.Recovery())

    // Health check endpoint
    router.GET("/health", func(c *gin.Context) {
        c.JSON(http.StatusOK, gin.H{
            "status": "healthy",
            "version": version,
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
            log.Printf("WebSocket upgrade failed: %v", err)
            return
        }
        defer conn.Close(websocket.StatusNormalClosure, "")

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

        log.Println("Shutting down Edge MCP...")
        
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        
        if err := srv.Shutdown(ctx); err != nil {
            log.Printf("Server shutdown error: %v", err)
        }
    }()

    log.Printf("Edge MCP v%s starting on port %d", version, cfg.Server.Port)
    if coreClient != nil {
        log.Printf("Connected to Core Platform at %s", cfg.Core.URL)
    } else {
        log.Println("Running in standalone mode (no Core Platform connection)")
    }

    if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("Server failed to start: %v", err)
    }
}