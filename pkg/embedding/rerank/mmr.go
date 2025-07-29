package rerank

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MMRReranker implements Maximal Marginal Relevance for diversity
type MMRReranker struct {
	lambda           float64 // Balance between relevance and diversity (0-1)
	embeddingService EmbeddingService
	logger           observability.Logger
	metrics          observability.MetricsClient
}

// EmbeddingService interface for generating embeddings
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text, contentType, model string) (*EmbeddingVector, error)
}

// EmbeddingVector represents an embedding vector
type EmbeddingVector struct {
	Vector []float32
}

// NewMMRReranker creates a new MMR reranker
func NewMMRReranker(lambda float64, embeddingService EmbeddingService, logger observability.Logger) (*MMRReranker, error) {
	if embeddingService == nil {
		return nil, fmt.Errorf("embedding service is required")
	}

	// Validate lambda is between 0 and 1
	if lambda < 0 || lambda > 1 {
		lambda = 0.5 // Default to balanced
	}

	if logger == nil {
		logger = observability.NewLogger("rerank.mmr")
	}

	return &MMRReranker{
		lambda:           lambda,
		embeddingService: embeddingService,
		logger:           logger,
		metrics:          observability.NewMetricsClient(),
	}, nil
}

// Rerank reorders results using Maximal Marginal Relevance
func (m *MMRReranker) Rerank(ctx context.Context, query string, results []SearchResult, opts *RerankOptions) ([]SearchResult, error) {
	if len(results) <= 1 {
		return results, nil
	}

	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "rerank.mmr")
	defer span.End()

	span.SetAttribute("lambda", m.lambda)
	span.SetAttribute("input_count", len(results))

	start := time.Now()
	defer func() {
		m.metrics.RecordHistogram("rerank.mmr.duration", time.Since(start).Seconds(), nil)
	}()

	m.logger.Info("Starting MMR reranking", map[string]interface{}{
		"query":       query,
		"num_results": len(results),
		"lambda":      m.lambda,
	})

	// Get embeddings for all results
	embeddings, err := m.getEmbeddings(ctx, results)
	if err != nil {
		m.logger.Error("Failed to get embeddings", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get embeddings: %w", err)
	}

	// Get query embedding
	queryEmbedding, err := m.embeddingService.GenerateEmbedding(ctx, query, "search_query", "")
	if err != nil {
		m.logger.Error("Failed to get query embedding", map[string]interface{}{
			"error": err.Error(),
		})
		return nil, fmt.Errorf("failed to get query embedding: %w", err)
	}

	// MMR algorithm
	selected := make([]SearchResult, 0, len(results))
	selectedIndices := make(map[int]bool)

	// Determine how many to select
	selectCount := len(results)
	if opts != nil && opts.TopK > 0 && opts.TopK < len(results) {
		selectCount = opts.TopK
	}

	// Select results iteratively
	for len(selected) < selectCount {
		bestScore := -math.MaxFloat64
		bestIdx := -1

		for i := range results {
			if selectedIndices[i] {
				continue
			}

			// Calculate relevance (similarity to query)
			relevance := float64(cosineSimilarity(embeddings[i], queryEmbedding.Vector))

			// Calculate diversity (min similarity to selected items)
			diversity := 1.0
			if len(selected) > 0 {
				for j := range results {
					if selectedIndices[j] {
						sim := float64(cosineSimilarity(embeddings[i], embeddings[j]))
						diversity = math.Min(diversity, 1.0-sim)
					}
				}
			}

			// MMR score = λ * relevance + (1-λ) * diversity
			score := m.lambda*relevance + (1-m.lambda)*diversity

			m.logger.Debug("MMR calculation", map[string]interface{}{
				"index":     i,
				"relevance": relevance,
				"diversity": diversity,
				"score":     score,
			})

			if score > bestScore {
				bestScore = score
				bestIdx = i
			}
		}

		if bestIdx >= 0 {
			selectedResult := results[bestIdx]
			// Update score to reflect MMR score
			selectedResult.Score = float32(bestScore)
			if selectedResult.Metadata == nil {
				selectedResult.Metadata = make(map[string]interface{})
			}
			selectedResult.Metadata["mmr_score"] = bestScore
			selectedResult.Metadata["mmr_lambda"] = m.lambda

			selected = append(selected, selectedResult)
			selectedIndices[bestIdx] = true
		} else {
			break
		}
	}

	span.SetAttribute("output_count", len(selected))
	m.metrics.IncrementCounter("rerank.mmr.success", 1.0)

	return selected, nil
}

// GetName returns the name of the reranker
func (m *MMRReranker) GetName() string {
	return fmt.Sprintf("mmr_lambda%.2f", m.lambda)
}

// Close cleans up resources
func (m *MMRReranker) Close() error {
	return nil
}

// getEmbeddings retrieves embeddings for all results
func (m *MMRReranker) getEmbeddings(ctx context.Context, results []SearchResult) ([][]float32, error) {
	embeddings := make([][]float32, len(results))

	// Check if embeddings are already cached in metadata
	allCached := true
	for i, result := range results {
		if result.Metadata != nil {
			if embeddingRaw, ok := result.Metadata["embedding"]; ok {
				if embedding, ok := embeddingRaw.([]float32); ok {
					embeddings[i] = embedding
					continue
				}
			}
		}
		allCached = false
		break
	}

	if allCached {
		m.logger.Debug("Using cached embeddings", map[string]interface{}{})
		return embeddings, nil
	}

	// Generate embeddings for results without cached embeddings
	m.logger.Debug("Generating embeddings for results", map[string]interface{}{})
	for i, result := range results {
		if embeddings[i] != nil {
			continue // Already have cached embedding
		}

		embedding, err := m.embeddingService.GenerateEmbedding(ctx, result.Content, "document", "")
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding for result %d: %w", i, err)
		}
		embeddings[i] = embedding.Vector
	}

	return embeddings, nil
}

// cosineSimilarity calculates the cosine similarity between two vectors
func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}
