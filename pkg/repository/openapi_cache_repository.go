package repository

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

// OpenAPICacheRepository defines the interface for OpenAPI spec caching
type OpenAPICacheRepository interface {
	// Get retrieves a cached OpenAPI spec by URL
	Get(ctx context.Context, url string) (*openapi3.T, error)

	// Set stores an OpenAPI spec with TTL
	Set(ctx context.Context, url string, spec *openapi3.T, ttl time.Duration) error

	// Invalidate removes a cached spec
	Invalidate(ctx context.Context, url string) error

	// GetByHash retrieves by URL and hash for validation
	GetByHash(ctx context.Context, url, hash string) (*openapi3.T, error)
}

// openAPICacheRepository is the SQL implementation
type openAPICacheRepository struct {
	db *sqlx.DB
}

// NewOpenAPICacheRepository creates a new OpenAPI cache repository
func NewOpenAPICacheRepository(db *sqlx.DB) OpenAPICacheRepository {
	return &openAPICacheRepository{db: db}
}

// Get retrieves a cached OpenAPI spec
func (r *openAPICacheRepository) Get(ctx context.Context, url string) (*openapi3.T, error) {
	query := `
		SELECT spec_data 
		FROM openapi_cache 
		WHERE url = $1 AND cache_expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`

	var specData json.RawMessage
	err := r.db.GetContext(ctx, &specData, query, url)
	if err != nil {
		return nil, err
	}

	// Parse the OpenAPI spec
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(specData)
	if err != nil {
		return nil, err
	}

	return spec, nil
}

// Set stores an OpenAPI spec
func (r *openAPICacheRepository) Set(ctx context.Context, url string, spec *openapi3.T, ttl time.Duration) error {
	// Serialize the spec
	specData, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	// Calculate hash
	hasher := sha256.New()
	hasher.Write(specData)
	hash := hex.EncodeToString(hasher.Sum(nil))

	// Extract metadata
	var discoveredActions []string
	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, op := range pathItem.Operations() {
				if op != nil {
					action := method + " " + path
					if op.OperationID != "" {
						action = op.OperationID
					}
					discoveredActions = append(discoveredActions, action)
				}
			}
		}
	}

	query := `
		INSERT INTO openapi_cache (
			url, spec_hash, spec_data, version, title, 
			discovered_actions, cache_expires_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		)
		ON CONFLICT (url, spec_hash) DO UPDATE SET
			cache_expires_at = EXCLUDED.cache_expires_at
	`

	version := ""
	if spec.Info != nil && spec.Info.Version != "" {
		version = spec.Info.Version
	}

	title := ""
	if spec.Info != nil && spec.Info.Title != "" {
		title = spec.Info.Title
	}

	// Use pq.Array to properly handle slice conversion for PostgreSQL
	_, err = r.db.ExecContext(ctx, query,
		url, hash, specData, version, title,
		pq.Array(discoveredActions), time.Now().Add(ttl),
	)

	return err
}

// Invalidate removes a cached spec
func (r *openAPICacheRepository) Invalidate(ctx context.Context, url string) error {
	query := `DELETE FROM openapi_cache WHERE url = $1`
	_, err := r.db.ExecContext(ctx, query, url)
	return err
}

// GetByHash retrieves by URL and hash
func (r *openAPICacheRepository) GetByHash(ctx context.Context, url, hash string) (*openapi3.T, error) {
	query := `
		SELECT spec_data 
		FROM openapi_cache 
		WHERE url = $1 AND spec_hash = $2 AND cache_expires_at > NOW()
		LIMIT 1
	`

	var specData json.RawMessage
	err := r.db.GetContext(ctx, &specData, query, url, hash)
	if err != nil {
		return nil, err
	}

	// Parse the OpenAPI spec
	loader := openapi3.NewLoader()
	spec, err := loader.LoadFromData(specData)
	if err != nil {
		return nil, err
	}

	return spec, nil
}
