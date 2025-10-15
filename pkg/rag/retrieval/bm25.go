// Package retrieval provides document retrieval and ranking functionality
package retrieval

import (
	"context"
	"fmt"

	"github.com/jmoiron/sqlx"
)

// BM25Search implements BM25 keyword search using PostgreSQL
type BM25Search struct {
	db *sqlx.DB
}

// NewBM25Search creates a new BM25 search instance
func NewBM25Search(db *sqlx.DB) *BM25Search {
	return &BM25Search{
		db: db,
	}
}

// SearchResult represents a single search result
type SearchResult struct {
	ID          string                 `json:"id" db:"id"`
	Content     string                 `json:"content" db:"content"`
	DocumentID  string                 `json:"document_id" db:"document_id"`
	Score       float64                `json:"score" db:"score"`
	URL         string                 `json:"url" db:"url"`
	Title       string                 `json:"title" db:"title"`
	SourceType  string                 `json:"source_type" db:"source_type"`
	Metadata    map[string]interface{} `json:"metadata" db:"metadata"`
	Embedding   []float32              `json:"-" db:"-"` // For MMR processing
	EmbeddingID *string                `json:"embedding_id" db:"embedding_id"`
}

// Search performs BM25 keyword search using PostgreSQL trigram similarity
func (b *BM25Search) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	sql := `
		SELECT
			dc.id,
			dc.content,
			dc.document_id,
			similarity(dc.content, $1) as score,
			d.url,
			d.title,
			d.source_type,
			d.metadata,
			dc.embedding_id
		FROM rag.document_chunks dc
		JOIN rag.documents d ON d.id = dc.document_id
		WHERE dc.content % $1  -- Trigram similarity operator
		ORDER BY similarity(dc.content, $1) DESC
		LIMIT $2
	`

	var results []SearchResult
	err := b.db.SelectContext(ctx, &results, sql, query, limit)
	if err != nil {
		return nil, fmt.Errorf("BM25 search failed: %w", err)
	}

	return results, nil
}

// FullTextSearch performs PostgreSQL full-text search as an alternative to trigram
func (b *BM25Search) FullTextSearch(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	sql := `
		SELECT
			dc.id,
			dc.content,
			dc.document_id,
			ts_rank(to_tsvector('english', dc.content),
			       plainto_tsquery('english', $1)) as score,
			d.url,
			d.title,
			d.source_type,
			d.metadata,
			dc.embedding_id
		FROM rag.document_chunks dc
		JOIN rag.documents d ON d.id = dc.document_id
		WHERE to_tsvector('english', dc.content) @@ plainto_tsquery('english', $1)
		ORDER BY score DESC
		LIMIT $2
	`

	var results []SearchResult
	err := b.db.SelectContext(ctx, &results, sql, query, limit)
	if err != nil {
		return nil, fmt.Errorf("full-text search failed: %w", err)
	}

	return results, nil
}

// SearchWithFilters performs BM25 search with additional filters
func (b *BM25Search) SearchWithFilters(ctx context.Context, query string, filters map[string]interface{}, limit int) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Build base query
	sql := `
		SELECT
			dc.id,
			dc.content,
			dc.document_id,
			similarity(dc.content, $1) as score,
			d.url,
			d.title,
			d.source_type,
			d.metadata,
			dc.embedding_id
		FROM rag.document_chunks dc
		JOIN rag.documents d ON d.id = dc.document_id
		WHERE dc.content % $1
	`

	args := []interface{}{query}
	argIndex := 2

	// Add source type filter
	if sourceType, ok := filters["source_type"].(string); ok && sourceType != "" {
		sql += fmt.Sprintf(" AND d.source_type = $%d", argIndex)
		args = append(args, sourceType)
		argIndex++
	}

	// Add tenant filter
	if tenantID, ok := filters["tenant_id"].(string); ok && tenantID != "" {
		sql += fmt.Sprintf(" AND d.tenant_id = $%d", argIndex)
		args = append(args, tenantID)
		argIndex++
	}

	sql += " ORDER BY similarity(dc.content, $1) DESC"
	sql += fmt.Sprintf(" LIMIT $%d", argIndex)
	args = append(args, limit)

	var results []SearchResult
	err := b.db.SelectContext(ctx, &results, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("filtered BM25 search failed: %w", err)
	}

	return results, nil
}

// GetRelevanceThreshold returns the minimum relevance threshold for BM25 results
func (b *BM25Search) GetRelevanceThreshold() float64 {
	return 0.1 // Minimum trigram similarity score
}
