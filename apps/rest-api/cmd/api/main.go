package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/core"

	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/common/cache"
	"github.com/developer-mesh/developer-mesh/pkg/common/config"
	"github.com/developer-mesh/developer-mesh/pkg/database"
	"github.com/developer-mesh/developer-mesh/pkg/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/metrics"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/services"

	// Import PostgreSQL driver
	_ "github.com/lib/pq"
)

// Command-line flags
var (
	skipMigration = flag.Bool("skip-migration", false, "Skip database migration on startup")
	migrateOnly   = flag.Bool("migrate", false, "Run database migrations and exit")
	healthCheck   = flag.Bool("health-check", false, "Run health check and exit")
)

func main() {
	flag.Parse()

	// Initialize secure random seed
	initSecureRandom()

	// Setup context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle health check flag
	if *healthCheck {
		// Perform actual health check by calling the health endpoint
		// Use the PORT env variable or default to 8080 (internal container port)
		port := os.Getenv("PORT")
		if port == "" {
			port = os.Getenv("API_PORT")
			if port == "" {
				port = "8080"
			}
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://localhost:%s/health", port))
		if err != nil {
			log.Printf("Health check failed: %v", err)
			os.Exit(1)
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			log.Printf("Health check failed with status: %d", resp.StatusCode)
			os.Exit(1)
		}
		os.Exit(0)
	}

	var err error

	// Initialize configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate critical configuration
	if err := validateConfiguration(cfg); err != nil {
		log.Fatalf("Invalid configuration: %v", err)
	}

	// Initialize logging
	logger := observability.NewLogger("server")

	// Initialize connection helper
	connHelper := api.NewConnectionHelper(logger)

	// Wait for dependencies if in container environment
	if os.Getenv("ENVIRONMENT") == "docker" {
		deps := []string{"database", "redis"}
		if err := connHelper.WaitForDependencies(ctx, deps); err != nil {
			logger.Warn("Failed to wait for dependencies", map[string]any{
				"error": err.Error(),
			})
		}
	}

	// Initialize metrics
	metricsClient := metrics.NewClient(cfg.Metrics)
	defer func() {
		if err := metricsClient.Close(); err != nil {
			logger.Warn("Failed to close metrics client", map[string]any{"error": err})
		}
	}()

	// Check if IRSA is enabled (IAM Roles for Service Accounts)
	if aws.IsIRSAEnabled() {
		logger.Info("IRSA (IAM Roles for Service Accounts) is enabled for AWS services", map[string]any{
			"aws_role_arn":                os.Getenv("AWS_ROLE_ARN"),
			"aws_web_identity_token_file": os.Getenv("AWS_WEB_IDENTITY_TOKEN_FILE"),
		})
	} else {
		logger.Info("IRSA not detected, will use standard AWS credential provider chain if IAM auth is enabled", nil)
	}

	// Prepare database config
	var db *database.Database
	dbConfig := database.Config{
		Driver:          cfg.Database.Driver,
		DSN:             cfg.Database.DSN,
		Host:            cfg.Database.Host,
		Port:            cfg.Database.Port,
		Database:        cfg.Database.Database,
		Username:        cfg.Database.Username,
		Password:        cfg.Database.Password,
		SSLMode:         cfg.Database.SSLMode,
		SearchPath:      cfg.Database.SearchPath,
		MaxOpenConns:    cfg.Database.MaxOpenConns,
		MaxIdleConns:    cfg.Database.MaxIdleConns,
		ConnMaxLifetime: cfg.Database.ConnMaxLifetime,
		// Migration settings
		AutoMigrate:          !*skipMigration && os.Getenv("SKIP_MIGRATIONS") != "true",
		MigrationsPath:       getMigrationDir(),
		FailOnMigrationError: os.Getenv("MIGRATIONS_FAIL_FAST") == "true",
	}

	// Check if we should use IAM authentication
	if cfg.Database.UseIAMAuth && aws.IsIRSAEnabled() {
		logger.Info("Using IAM authentication for RDS", nil)
		dbConfig.UseAWS = true
		dbConfig.UseIAM = true
		dbConfig.AWSRegion = cfg.Database.AuthConfig.Region
		dbConfig.RDSHost = cfg.Database.Host
		dbConfig.RDSPort = cfg.Database.Port
		dbConfig.RDSDatabase = cfg.Database.Database
		dbConfig.RDSUsername = cfg.Database.Username
		dbConfig.RDSTokenExpiration = cfg.Database.TokenExpiration
	}

	// Track migration status globally for health checks
	if dbConfig.AutoMigrate {
		api.GlobalMigrationStatus.SetInProgress()
		logger.Info("Starting database migrations", nil)
	}

	// Initialize database with retry logic
	db, err = connHelper.ConnectToDatabase(ctx, dbConfig)
	if err != nil {
		// Mark migrations as failed if they were expected to run
		if dbConfig.AutoMigrate {
			api.GlobalMigrationStatus.SetFailed(err)
		}
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database connection: %v", err)
		}
	}()

	// Mark migrations as completed if they ran successfully
	if dbConfig.AutoMigrate {
		api.GlobalMigrationStatus.SetCompleted("latest")
		logger.Info("Database migrations completed successfully", nil)
	} else {
		// If we skipped migrations, mark as ready anyway
		api.GlobalMigrationStatus.SetCompleted("skipped")
	}

	// Prepare cache config with AWS integration if needed
	var cacheClient cache.Cache

	// Convert cache configuration similar to MCP server
	var cacheConfig cache.RedisConfig

	// Check if we should use AWS ElastiCache
	if cfg.AWS.ElastiCache.UseIAMAuth && aws.IsIRSAEnabled() {
		logger.Info("Using IAM authentication for ElastiCache", nil)
		cacheConfig = cache.RedisConfig{
			Type:              "redis_cluster",
			UseAWS:            true,
			ClusterMode:       cfg.AWS.ElastiCache.ClusterMode,
			ElastiCacheConfig: &cfg.AWS.ElastiCache,
			MaxRetries:        cfg.AWS.ElastiCache.MaxRetries,
			DialTimeout:       cfg.AWS.ElastiCache.DialTimeout,
			ReadTimeout:       cfg.AWS.ElastiCache.ReadTimeout,
			WriteTimeout:      cfg.AWS.ElastiCache.WriteTimeout,
			PoolSize:          cfg.AWS.ElastiCache.PoolSize,
			MinIdleConns:      cfg.AWS.ElastiCache.MinIdleConnections,
			PoolTimeout:       cfg.AWS.ElastiCache.PoolTimeout,
		}
	} else {
		// Use standard Redis configuration from cfg.Cache
		// Note: cfg.Cache is already a cache.RedisConfig, so we use it directly
		cacheConfig = cfg.Cache

		// The TLS config should already be properly set in cfg.Cache
		// Just log for debugging
		if cacheConfig.TLS != nil && cacheConfig.TLS.Enabled {
			logger.Info("Cache TLS is enabled", map[string]any{
				"address":              cacheConfig.Address,
				"insecure_skip_verify": cacheConfig.TLS.InsecureSkipVerify,
			})
		}
	}

	// Initialize cache with retry logic and graceful degradation
	cacheClient, err = connHelper.ConnectToCache(ctx, cacheConfig)
	if err != nil {
		logger.Warn("Cache initialization failed, running without cache", map[string]any{
			"error": err.Error(),
		})
		// Create a no-op cache for graceful degradation
		cacheClient = cache.NewNoOpCache()
	}
	if cacheClient != nil {
		defer func() {
			if err := cacheClient.Close(); err != nil {
				logger.Warn("Failed to close cache client", map[string]any{"error": err})
			}
		}()
	}

	// Initialize core engine with logger
	engine := core.NewEngine(logger)
	defer func() {
		if err := engine.Shutdown(ctx); err != nil {
			logger.Error("Failed to shutdown engine", map[string]any{
				"error": err.Error(),
			})
		}
	}()

	// Migrations are now handled by the database package during initialization
	// The AutoMigrate flag in dbConfig controls whether migrations run

	// Initialize webhook configurations from environment
	if db != nil {
		webhookRepo := repository.NewWebhookConfigRepository(db.GetDB())
		webhookInitializer := services.NewWebhookInitializer(webhookRepo, logger)

		// Initialize webhook configurations from environment variables
		if err := webhookInitializer.InitializeFromEnvironment(ctx); err != nil {
			logger.Error("Failed to initialize webhook configurations", map[string]any{
				"error": err.Error(),
			})
			// Don't fail startup, but log the error
		}
	}

	// If migrate-only flag was set, exit after migrations
	if *migrateOnly {
		logger.Info("Migrations completed, exiting (--migrate flag)", nil)
		os.Exit(0)
	}

	// Initialize context manager based on environment configuration
	useMock := os.Getenv("USE_MOCK_CONTEXT_MANAGER")
	if strings.ToLower(useMock) == "true" {
		logger.Info("Using mock context manager as specified by environment", nil)
		engine.SetContextManager(core.NewMockContextManager())
	}
	// Note: Production context manager will be initialized in server.Initialize()
	// with the properly configured queue client

	// Convert configuration to api.Config
	apiConfig := api.Config{
		ListenAddress: cfg.API.ListenAddress,
		ReadTimeout:   30 * time.Second, // Default value
		WriteTimeout:  30 * time.Second, // Default value
		IdleTimeout:   90 * time.Second, // Default value
		EnableCORS:    cfg.API.EnableCORS,
		EnableSwagger: cfg.API.EnableSwagger,
		TLSCertFile:   cfg.API.TLSCertFile,
		TLSKeyFile:    cfg.API.TLSKeyFile,
		Auth: api.AuthConfig{
			JWTSecret: getStringFromConfig(cfg.API.Auth, "jwt_secret"),
			APIKeys:   cfg.API.Auth["api_keys"],
		},
		RateLimit: parseRateLimitConfig(cfg.API.RateLimit),
		// Use webhook configuration from config file
		Webhook: interfaces.WebhookConfig{
			EnabledField:             true, // Enable webhooks
			GitHubEndpointField:      "/api/webhooks/github",
			GitHubSecretField:        os.Getenv("GITHUB_WEBHOOK_SECRET"), // Get from env
			GitHubIPValidationField:  getBoolFromEnv("MCP_GITHUB_IP_VALIDATION", true),
			GitHubAllowedEventsField: parseAllowedEvents(os.Getenv("MCP_GITHUB_ALLOWED_EVENTS")),
		},
		// Default values for other fields
		Versioning: api.VersioningConfig{
			Enabled:           true,
			DefaultVersion:    "1.0",
			SupportedVersions: []string{"1.0"},
		},
		Performance: api.DefaultConfig().Performance,
	}

	// Create a MetricsClient instance
	obsMetricsClient := observability.NewMetricsClient()

	// Initialize API server
	// Pass both config and cache client
	server := api.NewServer(engine, apiConfig, db.GetDB(), obsMetricsClient, cfg, cacheClient)

	// Initialize server components
	if err := server.Initialize(ctx); err != nil {
		log.Fatalf("Failed to initialize server components: %v", err)
	}

	// Determine the correct port based on environment
	port := cfg.GetListenPort()
	logger.Info("Server configuration", map[string]any{
		"port":      port,
		"env":       cfg.Environment,
		"vector_db": cfg.Database.Vector.Enabled,
	})

	// Start server in a goroutine
	go func() {
		logger.Info("Starting server", map[string]any{
			"address": cfg.API.ListenAddress,
		})

		// If we're in production and TLS is configured, use HTTPS
		if cfg.IsProduction() && cfg.API.TLSCertFile != "" && cfg.API.TLSKeyFile != "" {
			logger.Info("Starting server with TLS (HTTPS)", nil)
			if err := server.StartTLS(cfg.API.TLSCertFile, cfg.API.TLSKeyFile); err != nil {
				log.Fatalf("Failed to start server with TLS: %v", err)
			}
		} else {
			// Otherwise use HTTP
			if err := server.Start(); err != nil {
				log.Fatalf("Failed to start server: %v", err)
			}
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan
	logger.Info("Received shutdown signal", nil)

	// Graceful shutdown
	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 30*time.Second)
	defer shutdownCancel()

	// Shutdown API server first
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("API server shutdown error", map[string]any{
			"error": err.Error(),
		})
	}

	logger.Info("Server stopped gracefully", nil)
}

// initSecureRandom is kept for backward compatibility but is no longer needed
// as of Go 1.20, the global random number generator is automatically seeded
func initSecureRandom() {
	// Go 1.20+ automatically seeds the global random number generator
	// No manual seeding is required
	log.Println("Random number generator is automatically seeded (Go 1.20+)")
}

// getBoolFromEnv gets a boolean value from environment variable with a default
func getBoolFromEnv(key string, defaultValue bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultValue
	}
	return strings.ToLower(val) == "true"
}

// parseAllowedEvents parses comma-separated allowed events
func parseAllowedEvents(events string) []string {
	if events == "" {
		// Default allowed events
		return []string{"issues", "issue_comment", "pull_request", "push", "release"}
	}
	// Split by comma and trim spaces
	eventList := strings.Split(events, ",")
	for i := range eventList {
		eventList[i] = strings.TrimSpace(eventList[i])
	}
	return eventList
}

// validateConfiguration validates critical configuration settings
func validateConfiguration(cfg *config.Config) error {
	// Check database configuration
	if cfg.Database.DSN == "" && (cfg.Database.Host == "" || cfg.Database.Port == 0 || cfg.Database.Database == "") {
		// If we're using IAM authentication from Database config, we don't need DSN or database credentials
		if !cfg.Database.UseIAMAuth {
			return fmt.Errorf("invalid database configuration: DSN or host/port/database must be provided")
		}
	}

	// Basic API validation
	if cfg.API.ListenAddress == "" {
		return fmt.Errorf("invalid API configuration: listen address must be provided")
	}

	return nil
}

// getStringFromConfig safely extracts a string value from a configuration map
func getStringFromConfig(m map[string]any, key string) string {
	if m == nil {
		return ""
	}
	if val, ok := m[key]; ok {
		if str, ok := val.(string); ok {
			return str
		}
	}
	return ""
}

// parseRateLimitConfig parses rate limit configuration from either a map or integer
func parseRateLimitConfig(input any) api.RateLimitConfig {
	config := api.RateLimitConfig{
		Enabled:     false,
		Limit:       100,
		Period:      time.Minute,
		BurstFactor: 3,
	}

	if input == nil {
		return config
	}

	// Handle backward compatibility - rate_limit as integer
	switch v := input.(type) {
	case int:
		config.Enabled = true
		config.Limit = v
		return config
	case float64: // JSON numbers are parsed as float64
		config.Enabled = true
		config.Limit = int(v)
		return config
	case map[string]any:
		// Handle structured rate_limit configuration
		if enabled, ok := v["enabled"].(bool); ok {
			config.Enabled = enabled
		}
		if limit, ok := v["limit"].(int); ok {
			config.Limit = limit
		} else if limit, ok := v["limit"].(float64); ok {
			config.Limit = int(limit)
		}
		if period, ok := v["period"].(string); ok {
			if d, err := time.ParseDuration(period); err == nil {
				config.Period = d
			}
		}
		if burstFactor, ok := v["burst_factor"].(int); ok {
			config.BurstFactor = burstFactor
		} else if burstFactor, ok := v["burst_factor"].(float64); ok {
			config.BurstFactor = int(burstFactor)
		}
		return config
	default:
		// Log warning and return default config
		fmt.Printf("Warning: unexpected type for rate_limit configuration: %T\n", input)
		return config
	}
}

// getMigrationDir returns the path to the migrations directory
func getMigrationDir() string {
	// Check environment variable first
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		return envPath
	}

	// Check multiple possible locations
	possiblePaths := []string{
		"/app/migrations/sql",                // Production Docker path
		"migrations/sql",                     // Local path relative to binary
		"apps/rest-api/migrations/sql",       // From project root
		"../../apps/rest-api/migrations/sql", // Relative to cmd/api
		filepath.Join(os.Getenv("PROJECT_ROOT"), "apps/rest-api/migrations/sql"),
	}

	// Find the first existing directory
	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// Default to the most likely production path
	return "/app/migrations/sql"
}
