// Package retrieval provides hybrid search combining multiple retrieval methods
package retrieval

import (
	"context"
	"fmt"
	"sort"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

// EmbeddingClient is an interface for embedding generation
type EmbeddingClient interface {
	EmbedContent(ctx context.Context, content string, modelOverride string) ([]float32, string, error)
}

// HybridSearch combines vector search, BM25, and importance scoring
type HybridSearch struct {
	vectorRepo      repository.VectorAPIRepository
	bm25            *BM25Search
	mmr             *MMR
	embeddingClient EmbeddingClient

	// Weights for combining different search methods
	vectorWeight     float64 // Weight for dense vector search
	bm25Weight       float64 // Weight for BM25 keyword search
	importanceWeight float64 // Weight for importance score
}

// HybridSearchConfig configures the hybrid search
type HybridSearchConfig struct {
	VectorWeight     float64 // Default: 0.6
	BM25Weight       float64 // Default: 0.2
	ImportanceWeight float64 // Default: 0.2
	MMRLambda        float64 // Default: 0.7
}

// DefaultHybridSearchConfig returns default configuration
func DefaultHybridSearchConfig() HybridSearchConfig {
	return HybridSearchConfig{
		VectorWeight:     0.6,
		BM25Weight:       0.2,
		ImportanceWeight: 0.2,
		MMRLambda:        0.7,
	}
}

// NewHybridSearch creates a new hybrid search instance
func NewHybridSearch(
	vectorRepo repository.VectorAPIRepository,
	bm25 *BM25Search,
	embeddingClient EmbeddingClient,
	config HybridSearchConfig,
) *HybridSearch {
	return &HybridSearch{
		vectorRepo:       vectorRepo,
		bm25:             bm25,
		mmr:              NewMMR(config.MMRLambda),
		embeddingClient:  embeddingClient,
		vectorWeight:     config.VectorWeight,
		bm25Weight:       config.BM25Weight,
		importanceWeight: config.ImportanceWeight,
	}
}

// Search performs hybrid search combining vector and keyword search
func (h *HybridSearch) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	return h.SearchWithOptions(ctx, query, SearchOptions{
		Limit:           limit,
		MinScore:        0.4,
		ApplyMMR:        true,
		IncludeMetadata: true,
	})
}

// SearchOptions configures search behavior
type SearchOptions struct {
	Limit           int
	MinScore        float64
	TenantID        string
	SourceType      string
	ApplyMMR        bool
	IncludeMetadata bool
}

// SearchWithOptions performs hybrid search with custom options
func (h *HybridSearch) SearchWithOptions(ctx context.Context, query string, opts SearchOptions) ([]SearchResult, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Generate query embedding
	queryEmbedding, modelName, err := h.embeddingClient.EmbedContent(
		ctx,
		query,
		"", // Use default model
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate query embedding: %w", err)
	}

	// Retrieve more candidates for re-ranking
	candidateLimit := opts.Limit * 3

	// Perform vector search
	vectorResults, err := h.vectorSearch(ctx, queryEmbedding, modelName, candidateLimit)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Perform BM25 search
	bm25Results, err := h.bm25Search(ctx, query, candidateLimit, opts)
	if err != nil {
		return nil, fmt.Errorf("BM25 search failed: %w", err)
	}

	// Combine results using Reciprocal Rank Fusion
	combined := h.reciprocalRankFusion(vectorResults, bm25Results)

	// Filter by minimum score
	filtered := h.filterByScore(combined, opts.MinScore)

	// Apply MMR for diversity if requested
	if opts.ApplyMMR && len(filtered) > opts.Limit {
		filtered = h.mmr.Rerank(filtered, queryEmbedding)
	}

	// Limit results
	if len(filtered) > opts.Limit {
		filtered = filtered[:opts.Limit]
	}

	return filtered, nil
}

// vectorSearch performs dense vector similarity search
func (h *HybridSearch) vectorSearch(ctx context.Context, queryEmbedding []float32, modelName string, limit int) ([]SearchResult, error) {
	// Search using existing vector repository
	embeddings, err := h.vectorRepo.SearchEmbeddings(
		ctx,
		queryEmbedding,
		"", // Search across all contexts
		modelName,
		limit,
		0.0, // No threshold filtering here
	)
	if err != nil {
		return nil, fmt.Errorf("vector repository search failed: %w", err)
	}

	// Convert to SearchResult format
	results := make([]SearchResult, 0, len(embeddings))
	for _, emb := range embeddings {
		// Extract document metadata from embedding
		docID, _ := emb.Metadata["document_id"].(string)
		sourceType, _ := emb.Metadata["source_type"].(string)

		result := SearchResult{
			ID:          emb.ID,
			Content:     emb.Text, // Use Text field (JSON) from Embedding struct
			DocumentID:  docID,
			Score:       1.0, // Will be calculated in fusion
			SourceType:  sourceType,
			Metadata:    emb.Metadata,
			Embedding:   emb.Embedding,
			EmbeddingID: &emb.ID,
		}

		results = append(results, result)
	}

	return results, nil
}

// bm25Search performs keyword-based search
func (h *HybridSearch) bm25Search(ctx context.Context, query string, limit int, opts SearchOptions) ([]SearchResult, error) {
	filters := make(map[string]interface{})

	if opts.TenantID != "" {
		filters["tenant_id"] = opts.TenantID
	}

	if opts.SourceType != "" {
		filters["source_type"] = opts.SourceType
	}

	return h.bm25.SearchWithFilters(ctx, query, filters, limit)
}

// reciprocalRankFusion combines results from multiple search methods using RRF
func (h *HybridSearch) reciprocalRankFusion(vectorResults, bm25Results []SearchResult) []SearchResult {
	const k = 60 // RRF constant

	// Map to track combined scores
	scoreMap := make(map[string]*SearchResult)

	// Process vector results
	for rank, result := range vectorResults {
		id := result.ID
		if _, exists := scoreMap[id]; !exists {
			scoreMap[id] = &result
			scoreMap[id].Score = 0
		}
		scoreMap[id].Score += h.vectorWeight / float64(rank+k)
	}

	// Process BM25 results
	for rank, result := range bm25Results {
		id := result.ID
		if _, exists := scoreMap[id]; !exists {
			scoreMap[id] = &result
			scoreMap[id].Score = 0
		}
		scoreMap[id].Score += h.bm25Weight / float64(rank+k)
	}

	// Add importance score if available
	for id, result := range scoreMap {
		if importance, ok := result.Metadata["importance_score"].(float64); ok {
			scoreMap[id].Score += h.importanceWeight * importance
		}
	}

	// Convert map to slice
	combined := make([]SearchResult, 0, len(scoreMap))
	for _, result := range scoreMap {
		combined = append(combined, *result)
	}

	// Sort by score descending
	sort.Slice(combined, func(i, j int) bool {
		return combined[i].Score > combined[j].Score
	})

	return combined
}

// filterByScore removes results below the minimum score threshold
func (h *HybridSearch) filterByScore(results []SearchResult, minScore float64) []SearchResult {
	if minScore <= 0 {
		return results
	}

	filtered := make([]SearchResult, 0, len(results))
	for _, result := range results {
		if result.Score >= minScore {
			filtered = append(filtered, result)
		}
	}

	return filtered
}

// UpdateWeights updates the search method weights
func (h *HybridSearch) UpdateWeights(vector, bm25, importance float64) error {
	total := vector + bm25 + importance
	if total > 1.01 || total < 0.99 {
		return fmt.Errorf("weights must sum to 1.0, got %.2f", total)
	}

	h.vectorWeight = vector
	h.bm25Weight = bm25
	h.importanceWeight = importance

	return nil
}

// GetWeights returns current search method weights
func (h *HybridSearch) GetWeights() (vector, bm25, importance float64) {
	return h.vectorWeight, h.bm25Weight, h.importanceWeight
}
