package database

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/developer-mesh/developer-mesh/pkg/common"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/jmoiron/sqlx"
)

// VectorConfig contains vector-specific database configuration
type VectorConfig struct {
	Enabled           bool
	ExtensionSchema   string
	IndexType         string
	DistanceMetric    string
	MaxDimensions     int
	DefaultDimensions int
}

// VectorDatabase provides specialized database operations for vector data
type VectorDatabase struct {
	db          *sqlx.DB
	vectorDB    *sqlx.DB
	logger      observability.Logger
	config      *VectorConfig
	initialized bool
	lock        sync.RWMutex
}

// NewVectorDatabase creates a new vector database
func NewVectorDatabase(db *sqlx.DB, cfg any, logger observability.Logger) (*VectorDatabase, error) {
	if logger == nil {
		logger = observability.NewStandardLogger("vector_database")
	}

	// Use the main database connection pool by default
	vectorDB := db

	// Try to extract vector config from the provided config
	var vectorConfig *VectorConfig

	// If the config is already a VectorConfig
	if vConfig, ok := cfg.(*VectorConfig); ok && vConfig != nil {
		vectorConfig = vConfig
	} else {
		// Create a default config if none provided
		vectorConfig = &VectorConfig{
			Enabled:           true,
			DefaultDimensions: 1536,
			DistanceMetric:    "cosine",
			IndexType:         "ivfflat",
			ExtensionSchema:   "public",
			MaxDimensions:     2000,
		}
	}

	return &VectorDatabase{
		db:          db,
		vectorDB:    vectorDB,
		logger:      logger,
		config:      vectorConfig,
		initialized: false,
	}, nil
}

// Initialize ensures the vector database is properly set up
func (vdb *VectorDatabase) Initialize(ctx context.Context) error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()

	if vdb.initialized {
		return nil
	}

	// Check if pgvector extension is installed
	var extExists bool
	err := vdb.vectorDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM pg_extension WHERE extname = 'vector'
		)
	`).Scan(&extExists)

	if err != nil {
		return fmt.Errorf("failed to check if pgvector extension exists: %w", err)
	}

	if !extExists {
		return fmt.Errorf("pgvector extension is not installed")
	}

	// Check if the embeddings table exists
	var tableExists bool
	err = vdb.vectorDB.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1 
			FROM information_schema.tables 
			WHERE table_schema = 'mcp' AND table_name = 'embeddings'
		)
	`).Scan(&tableExists)

	if err != nil {
		return fmt.Errorf("failed to check if embeddings table exists: %w", err)
	}

	if !tableExists {
		// The embeddings table should be created by migrations/init.sql, not by application code
		return fmt.Errorf("embeddings table does not exist in mcp schema; please run database migrations")
	}

	vdb.initialized = true
	vdb.logger.Info("Vector database initialized successfully", nil)

	return nil
}

// Transaction runs a vector database operation in a transaction
func (vdb *VectorDatabase) Transaction(ctx context.Context, fn func(tx *sqlx.Tx) error) error {
	// Use the vector-specific connection pool
	tx, err := vdb.vectorDB.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Execute the operation
	if err := fn(tx); err != nil {
		// Roll back on error
		if rbErr := tx.Rollback(); rbErr != nil {
			vdb.logger.Error("Failed to roll back transaction", map[string]any{
				"error": rbErr.Error(),
			})
		}
		return err
	}

	// Commit the transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Close closes the vector database connection pool
func (vdb *VectorDatabase) Close() error {
	vdb.lock.Lock()
	defer vdb.lock.Unlock()

	// Only close the dedicated pool if it's different from the main pool
	if vdb.vectorDB != vdb.db {
		return vdb.vectorDB.Close()
	}

	return nil
}

// GetVectorSearchConfig returns the current vector search configuration
func (vdb *VectorDatabase) GetVectorSearchConfig() *VectorConfig {
	vdb.lock.RLock()
	defer vdb.lock.RUnlock()

	return vdb.config
}

// CheckVectorDimensions returns the available vector dimensions in the database
func (vdb *VectorDatabase) CheckVectorDimensions(ctx context.Context) ([]int, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return nil, err
	}

	// Query distinct dimensions
	rows, err := vdb.vectorDB.QueryContext(ctx, `
		SELECT DISTINCT vector_dimensions
		FROM mcp.embeddings
		ORDER BY vector_dimensions
	`)

	if err != nil {
		return nil, fmt.Errorf("failed to query vector dimensions: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			log.Printf("Failed to close rows: %v", err)
		}
	}()

	// Process results
	var dimensions []int

	for rows.Next() {
		var dim int
		if err := rows.Scan(&dim); err != nil {
			return nil, fmt.Errorf("failed to scan dimension: %w", err)
		}
		dimensions = append(dimensions, dim)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over dimensions: %w", err)
	}

	return dimensions, nil
}

// NormalizeVector applies normalization to a vector based on the chosen similarity metric
func NormalizeVector(vector []float32, method string) ([]float32, error) {
	if len(vector) == 0 {
		return vector, nil
	}

	switch method {
	case "cosine":
		// Cosine similarity requires L2 normalization
		return common.NormalizeVectorL2(vector), nil
	case "dot":
		// Dot product doesn't require normalization
		return vector, nil
	case "euclidean":
		// Euclidean distance doesn't require normalization
		return vector, nil
	default:
		return nil, fmt.Errorf("unsupported normalization method: %s", method)
	}
}

// CreateVector creates a new vector from float32 array
func (vdb *VectorDatabase) CreateVector(ctx context.Context, vector []float32) (string, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return "", err
	}

	// Convert []float32 to []float64 as the database driver doesn't handle float32 arrays directly
	floatArray := make([]float64, len(vector))
	for i, v := range vector {
		floatArray[i] = float64(v)
	}

	// Convert to JSON array format that pgvector expects: [a,b,c]
	// pgvector can parse JSON array format directly
	pgArray := fmt.Sprintf("[%s]", formatFloatArray(floatArray))

	// Convert vector to pgvector format
	var vectorStr string
	err := vdb.vectorDB.QueryRowContext(ctx, `
		SELECT $1::vector::text
	`, pgArray).Scan(&vectorStr)

	if err != nil {
		return "", fmt.Errorf("failed to create vector: %w", err)
	}

	return vectorStr, nil
}

// formatFloatArray formats a float slice as a comma-separated string
func formatFloatArray(arr []float64) string {
	if len(arr) == 0 {
		return ""
	}

	result := fmt.Sprintf("%f", arr[0])
	for i := 1; i < len(arr); i++ {
		result += fmt.Sprintf(",%f", arr[i])
	}

	return result
}

// CalculateSimilarity calculates the similarity between two vectors
func (vdb *VectorDatabase) CalculateSimilarity(ctx context.Context, vector1, vector2 []float32, method string) (float64, error) {
	if err := vdb.Initialize(ctx); err != nil {
		return 0, err
	}

	// Convert []float32 to []float64 as the database driver doesn't handle float32 arrays directly
	floatArray1 := make([]float64, len(vector1))
	for i, v := range vector1 {
		floatArray1[i] = float64(v)
	}

	floatArray2 := make([]float64, len(vector2))
	for i, v := range vector2 {
		floatArray2[i] = float64(v)
	}

	// Convert to JSON array format that pgvector expects: [a,b,c]
	pgArray1 := fmt.Sprintf("[%s]", formatFloatArray(floatArray1))
	pgArray2 := fmt.Sprintf("[%s]", formatFloatArray(floatArray2))

	// Convert vectors to pgvector format
	var v1Str, v2Str string
	err := vdb.vectorDB.QueryRowContext(ctx, `
		SELECT $1::vector::text
	`, pgArray1).Scan(&v1Str)

	if err != nil {
		return 0, fmt.Errorf("failed to create vector1: %w", err)
	}

	err = vdb.vectorDB.QueryRowContext(ctx, `
		SELECT $1::vector::text
	`, pgArray2).Scan(&v2Str)

	if err != nil {
		return 0, fmt.Errorf("failed to create vector2: %w", err)
	}

	// Calculate similarity based on method
	var similarity float64

	switch method {
	case "cosine":
		// 1 - cosine distance = cosine similarity
		err = vdb.vectorDB.QueryRowContext(ctx, `
			SELECT 1 - ($1::vector <=> $2::vector)
		`, v1Str, v2Str).Scan(&similarity)
	case "dot":
		// Dot product
		err = vdb.vectorDB.QueryRowContext(ctx, `
			SELECT $1::vector <#> $2::vector
		`, v1Str, v2Str).Scan(&similarity)
	case "euclidean":
		// Negative euclidean distance (higher is more similar)
		err = vdb.vectorDB.QueryRowContext(ctx, `
			SELECT -($1::vector <-> $2::vector)
		`, v1Str, v2Str).Scan(&similarity)
	default:
		return 0, fmt.Errorf("unsupported similarity method: %s", method)
	}

	if err != nil {
		return 0, fmt.Errorf("failed to calculate similarity: %w", err)
	}

	return similarity, nil
}
