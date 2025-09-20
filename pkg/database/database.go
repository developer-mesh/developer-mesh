package database

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/common/aws"
	"github.com/developer-mesh/developer-mesh/pkg/database/migration"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/jmoiron/sqlx"

	// Import PostgreSQL driver
	_ "github.com/lib/pq"
)

// Common errors
var (
	ErrMissingAWSRegion      = errors.New("AWS region is required when using IAM authentication")
	ErrMissingRDSHost        = errors.New("RDS host is required when using AWS RDS")
	ErrInvalidDatabaseConfig = errors.New("invalid database configuration: missing required fields")
	ErrNotFound              = errors.New("record not found")
	ErrDuplicateKey          = errors.New("duplicate key violation")
)

// Config is defined in config.go

// sanitizeDSN removes sensitive information from a DSN for safe logging
func sanitizeDSN(dsn string) string {
	// Handle PostgreSQL DSN format
	if strings.Contains(dsn, "password=") {
		parts := strings.Split(dsn, " ")
		var sanitized []string
		for _, part := range parts {
			if strings.HasPrefix(part, "password=") {
				sanitized = append(sanitized, "password=***")
			} else {
				sanitized = append(sanitized, part)
			}
		}
		return strings.Join(sanitized, " ")
	}
	// Handle URL format DSN
	if strings.Contains(dsn, "@") {
		// Find the @ symbol and mask everything between :// and @
		if idx := strings.Index(dsn, "://"); idx != -1 {
			if atIdx := strings.Index(dsn[idx:], "@"); atIdx != -1 {
				prefix := dsn[:idx+3]     // Keep protocol://
				suffix := dsn[idx+atIdx:] // Keep @host...
				return prefix + "***:***" + suffix
			}
		}
	}
	return dsn
}

// Database represents the database access layer
type Database struct {
	db         *sqlx.DB
	config     Config
	statements map[string]*sqlx.Stmt
	rdsClient  *aws.ExtendedRDSClient
}

// NewDatabase creates a new database connection
func NewDatabase(ctx context.Context, cfg Config) (*Database, error) {
	var dsn string
	var err error
	var rdsClient *aws.ExtendedRDSClient

	// If we're using AWS RDS with IAM authentication
	if cfg.UseAWS && cfg.UseIAM && cfg.RDSHost != "" {
		// Create AWS RDS configuration
		awsRDSConfig := aws.RDSConnectionConfig{
			Host:              cfg.RDSHost,
			Port:              cfg.RDSPort,
			Database:          cfg.RDSDatabase,
			Username:          cfg.RDSUsername,
			Password:          cfg.Password,
			UseIAMAuth:        cfg.UseIAM,
			TokenExpiration:   cfg.RDSTokenExpiration,
			MaxOpenConns:      cfg.MaxOpenConns,
			MaxIdleConns:      cfg.MaxIdleConns,
			ConnMaxLifetime:   cfg.ConnMaxLifetime,
			EnablePooling:     cfg.RDSEnablePooling,
			MinPoolSize:       cfg.RDSMinPoolSize,
			MaxPoolSize:       cfg.RDSMaxPoolSize,
			ConnectionTimeout: cfg.RDSConnectionTimeout,
			Auth: aws.AuthConfig{
				Region:     cfg.AWSRegion,
				AssumeRole: cfg.AWSRoleARN,
			},
		}

		// Initialize the RDS client
		rdsClient, err = aws.NewExtendedRDSClient(ctx, awsRDSConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize RDS client: %w", err)
		}

		// Get DSN with IAM authentication
		dsn, err = rdsClient.BuildPostgresConnectionString(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to build PostgreSQL connection string with IAM auth: %w", err)
		}
	} else if cfg.DSN != "" {
		// Use provided DSN (fallback method)
		log.Println("Warning: Using explicit DSN for database connection instead of IAM authentication")
		dsn = cfg.DSN

		// Ensure search_path is included in DSN if not already present
		if !strings.Contains(dsn, "search_path") && cfg.SearchPath == "" {
			if strings.Contains(dsn, "?") {
				dsn = dsn + "&search_path=mcp,public"
			} else {
				dsn = dsn + "?search_path=mcp,public"
			}
		}
		log.Printf("Using DSN: %s", sanitizeDSN(dsn))
	} else {
		// Build DSN from individual components (least recommended option)
		log.Println("Warning: Building database connection string from components instead of using IAM authentication")

		sslMode := cfg.SSLMode
		if sslMode == "" {
			sslMode = "disable"
		}

		// Check that password is not empty when not using IAM auth
		if !cfg.UseIAM && cfg.Password == "" {
			return nil, fmt.Errorf("password is required when not using IAM authentication")
		}

		dsn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s search_path=mcp,public",
			cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.Database, sslMode)
	}

	// Connect to the database
	db, err := sqlx.ConnectContext(ctx, cfg.Driver, dsn)
	if err != nil {
		return nil, err
	}

	// Configure connection pool
	db.SetMaxOpenConns(cfg.MaxOpenConns)
	db.SetMaxIdleConns(cfg.MaxIdleConns)
	db.SetConnMaxLifetime(cfg.ConnMaxLifetime)

	// Create database instance
	database := &Database{
		db:         db,
		config:     cfg,
		statements: make(map[string]*sqlx.Stmt),
		rdsClient:  rdsClient,
	}

	// Verify search_path is correctly set
	var searchPath string
	if err := db.QueryRowContext(ctx, "SHOW search_path").Scan(&searchPath); err == nil {
		log.Printf("Database search_path: %s", searchPath)
		// If search_path doesn't include mcp, warn about it
		if !strings.Contains(searchPath, "mcp") {
			log.Printf("WARNING: search_path does not include 'mcp' schema: %s", searchPath)
			log.Printf("This may cause table lookup failures. Ensure DSN includes search_path parameter.")
		}
	}

	// Skip preparing statements here - they'll be prepared after migrations run
	// The tables may not exist yet during initial setup

	// Run migrations if enabled
	if cfg.AutoMigrate {
		log.Println("Running automatic database migrations...")
		migrationOpts := migration.DefaultOptions()
		migrationOpts.Path = cfg.MigrationsPath
		migrationOpts.FailOnError = cfg.FailOnMigrationError

		if err := migration.AutoMigrate(ctx, db, cfg.Driver, migrationOpts); err != nil {
			if migrationOpts.FailOnError {
				if closeErr := db.Close(); closeErr != nil {
					log.Printf("Failed to close database after migration error: %v", closeErr)
				}
				return nil, fmt.Errorf("database migration failed: %w", err)
			}
			log.Printf("Warning: Database migration had errors but continuing: %v", err)
		} else {
			log.Println("Database migrations completed successfully")
		}
	}

	// Initialize database tables
	if err := database.InitializeTables(ctx); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			log.Printf("Failed to close database after initialization error: %v", closeErr)
		}
		return nil, fmt.Errorf("failed to initialize database tables: %w", err)
	}

	// Prepare statements after migrations have run
	// Use exponential backoff in case tables are still being created
	if err := database.prepareStatementsWithRetry(ctx); err != nil {
		log.Printf("Warning: Failed to prepare statements after retries: %v", err)
		// Don't fail initialization - statements can be prepared on demand
	}

	return database, nil
}

// prepareStatementsWithRetry prepares statements with exponential backoff
func (d *Database) prepareStatementsWithRetry(ctx context.Context) error {
	maxRetries := 5
	baseDelay := 100 * time.Millisecond

	for i := 0; i < maxRetries; i++ {
		err := d.prepareStatements(ctx)
		if err == nil {
			return nil // Success!
		}

		// Check if error is due to missing tables
		if strings.Contains(err.Error(), "does not exist") {
			if i < maxRetries-1 {
				delay := baseDelay * (1 << uint(i)) // Exponential backoff
				if delay > 2*time.Second {
					delay = 2 * time.Second
				}
				log.Printf("Failed to prepare statements (attempt %d/%d), retrying in %v: %v",
					i+1, maxRetries, delay, err)

				select {
				case <-time.After(delay):
					continue
				case <-ctx.Done():
					return ctx.Err()
				}
			}
		} else {
			// Non-retryable error
			return err
		}
	}

	return fmt.Errorf("failed to prepare statements after %d attempts", maxRetries)
}

// prepareStatements prepares common SQL statements for reuse
func (d *Database) prepareStatements(ctx context.Context) error {
	// Prepare common SQL statements for better performance
	// This is a placeholder implementation - add actual statements as needed
	queries := map[string]string{
		"get_event":                 "SELECT * FROM mcp.events WHERE id = $1",
		"insert_event":              "INSERT INTO mcp.events (tenant_id, aggregate_id, aggregate_type, event_type, event_data, source, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id",
		"get_context":               "SELECT * FROM mcp.contexts WHERE id = $1",
		"insert_context":            "INSERT INTO mcp.contexts (id, tenant_id, agent_id, model_id, type, status, metadata, token_count, max_tokens, created_at, updated_at, expires_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)",
		"update_context":            "UPDATE mcp.contexts SET agent_id = $1, model_id = $2, type = $3, status = $4, metadata = $5, token_count = $6, max_tokens = $7, updated_at = $8, expires_at = $9 WHERE id = $10",
		"delete_context":            "DELETE FROM mcp.contexts WHERE id = $1",
		"list_contexts":             "SELECT * FROM mcp.contexts WHERE agent_id = $1 ORDER BY updated_at DESC",
		"get_context_items":         "SELECT * FROM mcp.context_items WHERE context_id = $1 ORDER BY sequence_number",
		"insert_context_item":       "INSERT INTO mcp.context_items (id, context_id, type, role, content, token_count, sequence_number, metadata, created_at) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		"check_context_item_exists": "SELECT EXISTS(SELECT 1 FROM mcp.context_items WHERE id = $1)",
		"get_integration":           "SELECT * FROM mcp.integrations WHERE id = $1",
	}

	for name, query := range queries {
		stmt, err := d.db.PreparexContext(ctx, query)
		if err != nil {
			return err
		}
		d.statements[name] = stmt
	}

	return nil
}

// Repository interface for data access
type Repository interface {
	FindByID(ctx context.Context, id string) (any, error)
	FindAll(ctx context.Context, options models.QueryOptions) (any, error)
	Create(ctx context.Context, entity any) error
	Update(ctx context.Context, entity any) error
	Delete(ctx context.Context, id string) error
}

// Transaction executes a function within a database transaction
func (d *Database) Transaction(ctx context.Context, fn func(*sqlx.Tx) error) error {
	// Defensive: panic early if the database connection is nil
	if d == nil || d.db == nil {
		panic("[database.Transaction] FATAL: Database or underlying *sqlx.DB is nil. Check initialization and connection setup.")
	}

	tx, err := d.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback() // Ignore error in panic recovery
			panic(p)          // Re-throw panic after rollback
		}
	}()

	if err := fn(tx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			log.Printf("Failed to rollback transaction: %v (original error: %v)", rbErr, err)
		}
		return err
	}

	return tx.Commit()
}

// RefreshConnection refreshes the database connection (especially for IAM auth)
func (d *Database) RefreshConnection(ctx context.Context) error {
	// If we're using AWS RDS with IAM authentication, we need to refresh the token
	if d.config.UseAWS && d.config.UseIAM && d.rdsClient != nil {
		// Get fresh DSN with a new IAM authentication token
		dsn, err := d.rdsClient.BuildPostgresConnectionString(ctx)
		if err != nil {
			return fmt.Errorf("failed to build PostgreSQL connection string: %w", err)
		}

		// Close existing connection
		if err := d.Close(); err != nil {
			return fmt.Errorf("failed to close existing database connection: %w", err)
		}

		// Create new connection
		db, err := sqlx.ConnectContext(ctx, d.config.Driver, dsn)
		if err != nil {
			return fmt.Errorf("failed to create new database connection: %w", err)
		}

		// Configure connection pool
		db.SetMaxOpenConns(d.config.MaxOpenConns)
		db.SetMaxIdleConns(d.config.MaxIdleConns)
		db.SetConnMaxLifetime(d.config.ConnMaxLifetime)

		// Update database instance
		d.db = db

		// Prepare statements
		if err := d.prepareStatements(ctx); err != nil {
			if closeErr := db.Close(); closeErr != nil {
				// Log close error but return original error
				log.Printf("Failed to close database after prepare error: %v", closeErr)
			}
			return fmt.Errorf("failed to prepare statements: %w", err)
		}
	}

	return nil
}

// Close closes the database connection
func (d *Database) Close() error {
	// Close all prepared statements
	for _, stmt := range d.statements {
		_ = stmt.Close() // Ignore close errors in cleanup
	}

	// Clear statements map
	d.statements = make(map[string]*sqlx.Stmt)

	// Close database connection
	return d.db.Close()
}

// Ping checks if the database connection is alive
func (d *Database) Ping() error {
	return d.db.Ping()
}

// DB returns the underlying sqlx.DB instance (compatible with the field name used in the API)
func (d *Database) DB() *sqlx.DB {
	return d.db
}

// GetDB returns the underlying sqlx.DB instance
func (d *Database) GetDB() *sqlx.DB {
	return d.db
}

// NewDatabaseWithConnection creates a new Database instance with an existing connection
func NewDatabaseWithConnection(db *sqlx.DB) *Database {
	return &Database{
		db:         db,
		statements: make(map[string]*sqlx.Stmt),
	}
}
