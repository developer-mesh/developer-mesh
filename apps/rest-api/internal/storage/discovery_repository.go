package storage

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/tools/adapters"
	"github.com/google/uuid"
)

// DiscoveryPatternRepository handles persistence of discovery patterns
type DiscoveryPatternRepository struct {
	db *sql.DB
}

// NewDiscoveryPatternRepository creates a new repository
func NewDiscoveryPatternRepository(db *sql.DB) *DiscoveryPatternRepository {
	return &DiscoveryPatternRepository{db: db}
}

// SavePattern saves or updates a discovery pattern
func (r *DiscoveryPatternRepository) SavePattern(pattern *adapters.DiscoveryPattern) error {
	query := `
		INSERT INTO tool_discovery_patterns (
			id, domain, successful_paths, auth_method, 
			api_format, last_updated, success_count
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (domain) DO UPDATE SET
			successful_paths = EXCLUDED.successful_paths,
			auth_method = EXCLUDED.auth_method,
			api_format = EXCLUDED.api_format,
			last_updated = EXCLUDED.last_updated,
			success_count = EXCLUDED.success_count
	`

	pathsJSON, err := json.Marshal(pattern.SuccessfulPaths)
	if err != nil {
		return fmt.Errorf("failed to marshal paths: %w", err)
	}

	_, err = r.db.Exec(
		query,
		uuid.New().String(),
		pattern.Domain,
		pathsJSON,
		pattern.AuthMethod,
		pattern.APIFormat,
		pattern.LastUpdated,
		pattern.SuccessCount,
	)

	return err
}

// LoadPatterns loads all discovery patterns
func (r *DiscoveryPatternRepository) LoadPatterns() (map[string]*adapters.DiscoveryPattern, error) {
	query := `
		SELECT domain, successful_paths, auth_method, 
		       api_format, last_updated, success_count
		FROM tool_discovery_patterns
	`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query patterns: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			// Log error but don't fail the operation
			_ = err
		}
	}()

	patterns := make(map[string]*adapters.DiscoveryPattern)
	for rows.Next() {
		var pattern adapters.DiscoveryPattern
		var pathsJSON []byte

		err := rows.Scan(
			&pattern.Domain,
			&pathsJSON,
			&pattern.AuthMethod,
			&pattern.APIFormat,
			&pattern.LastUpdated,
			&pattern.SuccessCount,
		)
		if err != nil {
			continue
		}

		if err := json.Unmarshal(pathsJSON, &pattern.SuccessfulPaths); err != nil {
			continue
		}

		patterns[pattern.Domain] = &pattern
	}

	return patterns, nil
}

// GetPatternByDomain gets a pattern by domain
func (r *DiscoveryPatternRepository) GetPatternByDomain(domain string) (*adapters.DiscoveryPattern, error) {
	query := `
		SELECT successful_paths, auth_method, api_format, 
		       last_updated, success_count
		FROM tool_discovery_patterns
		WHERE domain = $1
	`

	var pattern adapters.DiscoveryPattern
	var pathsJSON []byte

	err := r.db.QueryRow(query, domain).Scan(
		&pathsJSON,
		&pattern.AuthMethod,
		&pattern.APIFormat,
		&pattern.LastUpdated,
		&pattern.SuccessCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("pattern not found for domain: %s", domain)
		}
		return nil, err
	}

	pattern.Domain = domain
	if err := json.Unmarshal(pathsJSON, &pattern.SuccessfulPaths); err != nil {
		return nil, err
	}

	return &pattern, nil
}

// DiscoveryHintRepository handles persistence of discovery hints
type DiscoveryHintRepository struct {
	db *sql.DB
}

// NewDiscoveryHintRepository creates a new hint repository
func NewDiscoveryHintRepository(db *sql.DB) *DiscoveryHintRepository {
	return &DiscoveryHintRepository{db: db}
}

// SaveHints saves discovery hints for a tool
func (r *DiscoveryHintRepository) SaveHints(ctx context.Context, toolID string, hints *adapters.DiscoveryHints) error {
	hintsJSON, err := json.Marshal(hints)
	if err != nil {
		return fmt.Errorf("failed to marshal hints: %w", err)
	}

	query := `
		UPDATE tool_configurations 
		SET config = jsonb_set(config, '{discovery_hints}', $1::jsonb)
		WHERE id = $2
	`

	_, err = r.db.ExecContext(ctx, query, hintsJSON, toolID)
	return err
}

// GetHints retrieves discovery hints for a tool
func (r *DiscoveryHintRepository) GetHints(ctx context.Context, toolID string) (*adapters.DiscoveryHints, error) {
	query := `
		SELECT config->'discovery_hints' 
		FROM tool_configurations 
		WHERE id = $1
	`

	var hintsJSON []byte
	err := r.db.QueryRowContext(ctx, query, toolID).Scan(&hintsJSON)
	if err != nil {
		return nil, err
	}

	var hints adapters.DiscoveryHints
	if err := json.Unmarshal(hintsJSON, &hints); err != nil {
		return nil, err
	}

	return &hints, nil
}
