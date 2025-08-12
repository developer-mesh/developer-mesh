package proxies

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository/search"
	"github.com/jmoiron/sqlx"
)

// SearchRepositoryProxy provides a database-backed implementation of search.Repository
type SearchRepositoryProxy struct {
	db               *sqlx.DB
	embeddingService EmbeddingService
	logger           observability.Logger
}

// EmbeddingService interface for generating embeddings
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text, model string) ([]float32, error)
}

// NewSearchRepositoryProxy creates a new search repository proxy
func NewSearchRepositoryProxy(db *sqlx.DB, embeddingService EmbeddingService, logger observability.Logger) search.Repository {
	if logger == nil {
		logger = observability.NewLogger("search-repository-proxy")
	}

	return &SearchRepositoryProxy{
		db:               db,
		embeddingService: embeddingService,
		logger:           logger,
	}
}

// Create stores a new search result
func (s *SearchRepositoryProxy) Create(ctx context.Context, result *search.SearchResult) error {
	// Generate embedding if embedding service is available
	var embedding []float32
	if s.embeddingService != nil && result.Content != "" {
		var err error
		embedding, err = s.embeddingService.GenerateEmbedding(ctx, result.Content, "amazon.titan-embed-text-v1")
		if err != nil {
			s.logger.Warn("Failed to generate embedding", map[string]interface{}{
				"error": err.Error(),
				"id":    result.ID,
			})
			// Continue without embedding
		}
	}

	metadataJSON, err := json.Marshal(result.Metadata)
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	query := `
		INSERT INTO mcp.search_documents (
			id, content, type, content_hash, metadata, embedding
		) VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (id) DO UPDATE SET
			content = EXCLUDED.content,
			type = EXCLUDED.type,
			content_hash = EXCLUDED.content_hash,
			metadata = EXCLUDED.metadata,
			embedding = EXCLUDED.embedding
	`

	_, err = s.db.ExecContext(ctx, query,
		result.ID,
		result.Content,
		result.Type,
		result.ContentHash,
		metadataJSON,
		embedding,
	)

	if err != nil {
		return fmt.Errorf("failed to create search result: %w", err)
	}

	return nil
}

// Get retrieves a search result by its ID
func (s *SearchRepositoryProxy) Get(ctx context.Context, id string) (*search.SearchResult, error) {
	query := `
		SELECT id, content, type, content_hash, metadata
		FROM mcp.search_documents
		WHERE id = $1
	`

	var result search.SearchResult
	var metadataJSON []byte

	err := s.db.QueryRowxContext(ctx, query, id).Scan(
		&result.ID,
		&result.Content,
		&result.Type,
		&result.ContentHash,
		&metadataJSON,
	)

	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("search result not found: %s", id)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get search result: %w", err)
	}

	if len(metadataJSON) > 0 {
		if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
			result.Metadata = make(map[string]interface{})
		}
	} else {
		result.Metadata = make(map[string]interface{})
	}

	return &result, nil
}

// List retrieves search results matching the provided filter
func (s *SearchRepositoryProxy) List(ctx context.Context, filter search.Filter) ([]*search.SearchResult, error) {
	query := `
		SELECT id, content, type, content_hash, metadata
		FROM mcp.search_documents
		WHERE 1=1
	`

	var args []interface{}
	argCount := 0

	// Add filters dynamically
	if contentType, ok := filter["type"].(string); ok && contentType != "" {
		argCount++
		query += fmt.Sprintf(" AND type = $%d", argCount)
		args = append(args, contentType)
	}

	if contentHash, ok := filter["content_hash"].(string); ok && contentHash != "" {
		argCount++
		query += fmt.Sprintf(" AND content_hash = $%d", argCount)
		args = append(args, contentHash)
	}

	query += " ORDER BY id DESC LIMIT 100"

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list search results: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []*search.SearchResult
	for rows.Next() {
		var result search.SearchResult
		var metadataJSON []byte

		err := rows.Scan(
			&result.ID,
			&result.Content,
			&result.Type,
			&result.ContentHash,
			&metadataJSON,
		)

		if err != nil {
			s.logger.Warn("Failed to scan search result", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		results = append(results, &result)
	}

	return results, nil
}

// Update modifies an existing search result
func (s *SearchRepositoryProxy) Update(ctx context.Context, result *search.SearchResult) error {
	// Same as Create with ON CONFLICT DO UPDATE
	return s.Create(ctx, result)
}

// Delete removes a search result by its ID
func (s *SearchRepositoryProxy) Delete(ctx context.Context, id string) error {
	query := `DELETE FROM mcp.search_documents WHERE id = $1`

	result, err := s.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete search result: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("search result not found: %s", id)
	}

	return nil
}

// SearchByText performs a vector search using text
func (s *SearchRepositoryProxy) SearchByText(ctx context.Context, query string, options *search.SearchOptions) (*search.SearchResults, error) {
	if options == nil {
		options = &search.SearchOptions{
			Limit:         20,
			MinSimilarity: 0.7,
		}
	}

	// If we have embedding service, convert text to vector
	if s.embeddingService != nil {
		queryEmbedding, err := s.embeddingService.GenerateEmbedding(ctx, query, "amazon.titan-embed-text-v1")
		if err != nil {
			s.logger.Warn("Failed to generate query embedding, falling back to text search", map[string]interface{}{
				"error": err.Error(),
			})
			return s.performTextSearch(ctx, query, options)
		}
		return s.SearchByVector(ctx, queryEmbedding, options)
	}

	// Fallback to text search
	return s.performTextSearch(ctx, query, options)
}

// performTextSearch performs a text-based search using PostgreSQL full-text search
func (s *SearchRepositoryProxy) performTextSearch(ctx context.Context, query string, options *search.SearchOptions) (*search.SearchResults, error) {
	searchQuery := `
		SELECT id, content, type, content_hash, metadata,
		       ts_rank(to_tsvector('english', content), plainto_tsquery('english', $1)) as score
		FROM mcp.search_documents
		WHERE to_tsvector('english', content) @@ plainto_tsquery('english', $1)
	`

	var args []interface{}
	args = append(args, query)
	argCount := 1

	// Add content type filters
	if len(options.ContentTypes) > 0 {
		argCount++
		searchQuery += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, options.ContentTypes)
	}

	// Order by relevance
	searchQuery += " ORDER BY score DESC"

	// Add limit
	if options.Limit > 0 {
		searchQuery += fmt.Sprintf(" LIMIT %d", options.Limit)
	} else {
		searchQuery += " LIMIT 20"
	}

	// Add offset
	if options.Offset > 0 {
		searchQuery += fmt.Sprintf(" OFFSET %d", options.Offset)
	}

	rows, err := s.db.QueryxContext(ctx, searchQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute text search: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []*search.SearchResult
	for rows.Next() {
		var result search.SearchResult
		var metadataJSON []byte
		var score float32

		err := rows.Scan(
			&result.ID,
			&result.Content,
			&result.Type,
			&result.ContentHash,
			&metadataJSON,
			&score,
		)

		if err != nil {
			s.logger.Warn("Failed to scan search result", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		result.Score = score
		results = append(results, &result)
	}

	return &search.SearchResults{
		Results: results,
		Total:   len(results),
		HasMore: len(results) == options.Limit,
	}, nil
}

// SearchByVector performs a vector search using a pre-computed vector
func (s *SearchRepositoryProxy) SearchByVector(ctx context.Context, vector []float32, options *search.SearchOptions) (*search.SearchResults, error) {
	if options == nil {
		options = &search.SearchOptions{
			Limit:         20,
			MinSimilarity: 0.7,
		}
	}

	// Perform vector similarity search using pgvector
	query := `
		SELECT id, content, type, content_hash, metadata,
		       1 - (embedding <=> $1::vector) as similarity
		FROM mcp.search_documents
		WHERE embedding IS NOT NULL
		  AND 1 - (embedding <=> $1::vector) > $2
	`

	var args []interface{}
	args = append(args, vector, options.MinSimilarity)
	argCount := 2

	// Add content type filters
	if len(options.ContentTypes) > 0 {
		argCount++
		query += fmt.Sprintf(" AND type = ANY($%d)", argCount)
		args = append(args, options.ContentTypes)
	}

	// Add metadata filters
	for key, value := range options.MetadataFilters {
		argCount++
		query += fmt.Sprintf(" AND metadata->>'%s' = $%d", key, argCount)
		args = append(args, fmt.Sprintf("%v", value))
	}

	// Order by similarity
	query += " ORDER BY similarity DESC"

	// Add limit
	if options.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", options.Limit)
	} else {
		query += " LIMIT 20"
	}

	// Add offset
	if options.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", options.Offset)
	}

	rows, err := s.db.QueryxContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute vector search: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []*search.SearchResult
	for rows.Next() {
		var result search.SearchResult
		var metadataJSON []byte
		var similarity float32

		err := rows.Scan(
			&result.ID,
			&result.Content,
			&result.Type,
			&result.ContentHash,
			&metadataJSON,
			&similarity,
		)

		if err != nil {
			s.logger.Warn("Failed to scan vector search result", map[string]interface{}{
				"error": err.Error(),
			})
			continue
		}

		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		} else {
			result.Metadata = make(map[string]interface{})
		}

		result.Score = similarity
		results = append(results, &result)
	}

	return &search.SearchResults{
		Results: results,
		Total:   len(results),
		HasMore: len(results) == options.Limit,
	}, nil
}

// SearchByContentID performs a "more like this" search
func (s *SearchRepositoryProxy) SearchByContentID(ctx context.Context, contentID string, options *search.SearchOptions) (*search.SearchResults, error) {
	// First, get the content's embedding
	query := `SELECT embedding FROM mcp.search_documents WHERE id = $1 AND embedding IS NOT NULL`

	var embedding []float32
	err := s.db.GetContext(ctx, &embedding, query, contentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("content not found or has no embedding: %s", contentID)
		}
		return nil, fmt.Errorf("failed to get content embedding: %w", err)
	}

	// Use the embedding to search for similar content
	return s.SearchByVector(ctx, embedding, options)
}

// GetSupportedModels returns a list of models with embeddings
func (s *SearchRepositoryProxy) GetSupportedModels(ctx context.Context) ([]string, error) {
	// For now, return the default model
	// In production, this would query available models
	return []string{"amazon.titan-embed-text-v1"}, nil
}

// GetSearchStats retrieves statistics about the search index
func (s *SearchRepositoryProxy) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Get total document count
	var totalDocs int
	totalQuery := `SELECT COUNT(*) FROM mcp.search_documents`
	if err := s.db.GetContext(ctx, &totalDocs, totalQuery); err != nil {
		return nil, fmt.Errorf("failed to get total documents: %w", err)
	}
	stats["total_documents"] = totalDocs

	// Get documents with embeddings
	var docsWithEmbeddings int
	embeddingQuery := `SELECT COUNT(*) FROM mcp.search_documents WHERE embedding IS NOT NULL`
	if err := s.db.GetContext(ctx, &docsWithEmbeddings, embeddingQuery); err != nil {
		return nil, fmt.Errorf("failed to get documents with embeddings: %w", err)
	}
	stats["documents_with_embeddings"] = docsWithEmbeddings

	// Get document count by type
	typeQuery := `
		SELECT type, COUNT(*) as count
		FROM mcp.search_documents
		GROUP BY type
	`

	rows, err := s.db.QueryxContext(ctx, typeQuery)
	if err != nil {
		return nil, fmt.Errorf("failed to get type stats: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			s.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	typeCounts := make(map[string]int)
	for rows.Next() {
		var contentType string
		var count int
		if err := rows.Scan(&contentType, &count); err != nil {
			continue
		}
		typeCounts[contentType] = count
	}
	stats["documents_by_type"] = typeCounts

	return stats, nil
}
