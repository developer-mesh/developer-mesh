// Package main is the entry point for the RAG loader service
package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/auth"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/config"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/middleware"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/repository"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/scheduler"
	"github.com/developer-mesh/developer-mesh/apps/rag-loader/internal/service"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	embeddingProviders "github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/rag/security"
	repoVector "github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

var (
	// Version information (set via ldflags during build)
	version   = "dev"
	buildTime = "unknown"
	gitCommit = "unknown"
)

func main() {
	// Parse command-line flags
	var (
		showVersion = flag.Bool("version", false, "Show version information")
		configPath  = flag.String("config", "", "Path to configuration file")
	)
	flag.Parse()

	// Show version if requested
	if *showVersion {
		fmt.Printf("RAG Loader\nVersion: %s\nBuild Time: %s\nGit Commit: %s\n",
			version, buildTime, gitCommit)
		os.Exit(0)
	}

	// Initialize logger
	logger := observability.NewStandardLogger("rag-loader")
	logger.Info("Starting RAG Loader", map[string]interface{}{
		"version":    version,
		"build_time": buildTime,
		"git_commit": gitCommit,
	})

	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Override config path if provided
	if *configPath != "" {
		logger.Info("Using custom config path", map[string]interface{}{
			"path": *configPath,
		})
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Connect to database
	db, err := connectDatabase(ctx, cfg.Database, logger)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("Failed to close database connection", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	// Create embedding client
	embeddingClient := embedding.NewContextEmbeddingClient(logger)

	// Register Bedrock provider for embeddings
	bedrockRegion := os.Getenv("AWS_REGION")
	if bedrockRegion == "" {
		bedrockRegion = os.Getenv("MCP_EMBEDDING_PROVIDERS_BEDROCK_REGION")
	}
	if bedrockRegion == "" {
		bedrockRegion = "us-east-1" // Default region
	}

	bedrockEnabled := os.Getenv("MCP_EMBEDDING_PROVIDERS_BEDROCK_ENABLED")
	if bedrockEnabled == "" || bedrockEnabled == "true" {
		bedrockCfg := embeddingProviders.ProviderConfig{
			Region:         bedrockRegion,
			MaxRetries:     3,
			RetryDelayBase: 100 * time.Millisecond,
		}

		bedrockProvider, err := embeddingProviders.NewBedrockProvider(bedrockCfg)
		if err != nil {
			log.Fatalf("Failed to create Bedrock provider: %v", err)
		}

		embeddingClient.RegisterProvider("bedrock", bedrockProvider)
		logger.Info("Bedrock provider registered", map[string]interface{}{
			"region": bedrockRegion,
		})
	}

	// Create vector repository
	vectorRepo := repoVector.NewRepository(db)

	// Create the loader service
	loaderService := service.NewLoaderService(cfg, db, embeddingClient, vectorRepo, logger)

	// Start the service in a goroutine
	serviceDone := make(chan error, 1)
	go func() {
		serviceDone <- loaderService.Start(ctx)
	}()

	// Get RAG master key for credential encryption (base64 encoded, 32 bytes)
	ragMasterKeyB64 := os.Getenv("RAG_MASTER_KEY")
	if ragMasterKeyB64 == "" {
		logger.Warn("RAG_MASTER_KEY not set, using default (insecure)", nil)
		ragMasterKeyB64 = "K5UjoD45dEV/PehMDwar9ORfItM39KtUg5dT+HymK2A="
	}

	// Decode base64 master key
	ragMasterKey, err := base64.StdEncoding.DecodeString(ragMasterKeyB64)
	if err != nil {
		log.Fatalf("Failed to decode RAG_MASTER_KEY: %v", err)
	}
	if len(ragMasterKey) != 32 {
		log.Fatalf("RAG_MASTER_KEY must be 32 bytes (got %d bytes)", len(ragMasterKey))
	}

	credManager := security.NewCredentialManager(db, ragMasterKey)

	// Create and start job processor
	jobProcessorConfig := scheduler.JobProcessorConfig{
		PollInterval:  30 * time.Second,
		MaxConcurrent: cfg.Scheduler.MaxConcurrentJobs,
		BatchSize:     cfg.Processing.Embedding.BatchSize,
		RetryAttempts: 3,
		RetryDelay:    time.Second,
	}
	jobProcessor := scheduler.NewJobProcessor(
		db,
		credManager,
		embeddingClient,
		vectorRepo,
		logger,
		jobProcessorConfig,
	)

	// Start job processor in background
	go func() {
		if err := jobProcessor.Start(); err != nil {
			logger.Error("Job processor error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	logger.Info("Job processor started", map[string]interface{}{
		"poll_interval":  jobProcessorConfig.PollInterval.String(),
		"max_concurrent": jobProcessorConfig.MaxConcurrent,
	})

	// Start API server if enabled
	var httpServer *http.Server
	if cfg.Scheduler.EnableAPI {
		httpServer = startAPIServer(cfg, db, logger)
	}

	// Start health check endpoint
	healthServer := startHealthServer(cfg, loaderService, logger)

	// Wait for shutdown signal
	select {
	case sig := <-sigChan:
		logger.Info("Received shutdown signal", map[string]interface{}{
			"signal": sig.String(),
		})
	case err := <-serviceDone:
		if err != nil {
			logger.Error("Service error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Graceful shutdown
	logger.Info("Starting graceful shutdown", nil)

	// Create shutdown context with timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), cfg.Service.ShutdownTimeout)
	defer shutdownCancel()

	// Stop the job processor
	jobProcessor.Stop()
	logger.Info("Job processor stopped", nil)

	// Stop the loader service
	if err := loaderService.Stop(); err != nil {
		logger.Error("Failed to stop loader service", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Shutdown HTTP servers
	if httpServer != nil {
		if err := httpServer.Shutdown(shutdownCtx); err != nil {
			logger.Error("Failed to shutdown API server", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("Failed to shutdown health server", map[string]interface{}{
			"error": err.Error(),
		})
	}

	// Cancel main context
	cancel()

	logger.Info("Shutdown complete", nil)
}

// connectDatabase establishes a database connection with retry logic
func connectDatabase(ctx context.Context, cfg config.DatabaseConfig, logger observability.Logger) (*sqlx.DB, error) {
	// Configure database with retry logic
	dbConfig := database.Config{
		Driver:     "postgres",
		Host:       cfg.Host,
		Port:       cfg.Port,
		Database:   cfg.Database,
		Username:   cfg.Username,
		Password:   cfg.Password,
		SSLMode:    cfg.SSLMode,
		SearchPath: cfg.SearchPath,
	}

	maxRetries := 10
	baseDelay := 1 * time.Second

	logger.Info("Connecting to database", map[string]interface{}{
		"host":     cfg.Host,
		"database": cfg.Database,
	})

	for i := 0; i < maxRetries; i++ {
		db, err := database.NewDatabase(ctx, dbConfig)
		if err == nil {
			// Test connection
			if pingErr := db.Ping(); pingErr == nil {
				logger.Info("Database connection established", nil)

				// Set connection pool settings
				sqlDB := db.GetDB()
				sqlDB.SetMaxOpenConns(cfg.MaxConns)
				sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)

				return sqlDB, nil
			} else {
				_ = db.Close()
				err = fmt.Errorf("failed to ping database: %w", pingErr)
			}
		}

		if i < maxRetries-1 {
			delay := baseDelay * (1 << uint(i))
			if delay > 30*time.Second {
				delay = 30 * time.Second
			}

			logger.Warn("Database connection failed, retrying...", map[string]interface{}{
				"attempt":      i + 1,
				"max_attempts": maxRetries,
				"delay":        delay.String(),
				"error":        err.Error(),
			})

			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
	}

	return nil, fmt.Errorf("failed to connect to database after %d attempts", maxRetries)
}

// startAPIServer starts the HTTP API server with multi-tenant support
func startAPIServer(cfg *config.Config, db *sqlx.DB, logger observability.Logger) *http.Server {
	// Set Gin mode based on log level
	if cfg.Service.LogLevel == "debug" {
		gin.SetMode(gin.DebugMode)
	} else {
		gin.SetMode(gin.ReleaseMode)
	}

	// Create Gin router
	router := gin.New()
	router.Use(gin.Recovery())

	// Create dependencies
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "docker-jwt-secret-change-in-production"
	}
	jwtValidator := auth.NewJWTValidator([]byte(jwtSecret), "devmesh")

	// Get RAG master key for credential encryption (base64 encoded, 32 bytes)
	ragMasterKeyB64 := os.Getenv("RAG_MASTER_KEY")
	if ragMasterKeyB64 == "" {
		logger.Warn("RAG_MASTER_KEY not set, using default (insecure)", nil)
		ragMasterKeyB64 = "K5UjoD45dEV/PehMDwar9ORfItM39KtUg5dT+HymK2A="
	}

	// Decode base64 master key
	ragMasterKey, err := base64.StdEncoding.DecodeString(ragMasterKeyB64)
	if err != nil {
		log.Fatalf("Failed to decode RAG_MASTER_KEY: %v", err)
	}
	if len(ragMasterKey) != 32 {
		log.Fatalf("RAG_MASTER_KEY must be 32 bytes (got %d bytes)", len(ragMasterKey))
	}

	credManager := security.NewCredentialManager(db, ragMasterKey)

	sourceRepo := repository.NewSourceRepository(db)

	// Create middleware
	tenantMw := middleware.NewTenantMiddleware(db, jwtValidator)

	// Create handlers
	sourceHandler := api.NewSourceHandler(sourceRepo, credManager)

	// Register routes
	v1 := router.Group("/api/v1/rag")
	{
		// Protected routes (require authentication)
		protected := v1.Group("")
		protected.Use(tenantMw.ExtractTenant())
		{
			// Source management
			protected.POST("/sources", sourceHandler.CreateSource)
			protected.GET("/sources", sourceHandler.ListSources)
			protected.GET("/sources/:id", sourceHandler.GetSource)
			protected.PUT("/sources/:id", sourceHandler.UpdateSource)
			protected.DELETE("/sources/:id", sourceHandler.DeleteSource)

			// Sync management
			protected.POST("/sources/:id/sync", sourceHandler.TriggerSync)
			protected.GET("/sources/:id/jobs", sourceHandler.GetSyncJobs)
		}
	}

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Service.Port),
		Handler: router,
	}

	go func() {
		logger.Info("Starting API server", map[string]interface{}{
			"port": cfg.Service.Port,
		})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("API server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return server
}

// startHealthServer starts the health check and metrics endpoint
func startHealthServer(cfg *config.Config, svc *service.LoaderService, logger observability.Logger) *http.Server {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := svc.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "unhealthy: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "healthy")
	})

	// Ready endpoint
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		// Check if service is ready to accept requests
		if err := svc.Health(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = fmt.Fprintf(w, "not ready: %v", err)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprint(w, "ready")
	})

	// Metrics endpoint (Prometheus)
	mux.Handle("/metrics", promhttp.Handler())

	server := &http.Server{
		Addr:    fmt.Sprintf(":%d", cfg.Service.MetricsPort),
		Handler: mux,
	}

	go func() {
		logger.Info("Starting health and metrics server", map[string]interface{}{
			"port": cfg.Service.MetricsPort,
		})

		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("Health server error", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	return server
}
