// Package retrieval provides MMR (Maximal Marginal Relevance) for diversity
package retrieval

import (
	"fmt"
	"math"
)

// MMR implements Maximal Marginal Relevance for result diversity
type MMR struct {
	Lambda float64 // Balance between relevance (1.0) and diversity (0.0)
}

// NewMMR creates a new MMR instance
func NewMMR(lambda float64) *MMR {
	if lambda < 0 || lambda > 1 {
		lambda = 0.7 // Default: 70% relevance, 30% diversity
	}
	return &MMR{
		Lambda: lambda,
	}
}

// Rerank reorders search results to maximize diversity while maintaining relevance
// queryEmbedding is the vector representation of the search query
// candidates are the initial search results to rerank
func (m *MMR) Rerank(candidates []SearchResult, queryEmbedding []float32) []SearchResult {
	if len(candidates) == 0 {
		return candidates
	}

	if len(candidates) == 1 {
		return candidates
	}

	// Selected results (maintains order)
	selected := []SearchResult{candidates[0]}
	remaining := make([]SearchResult, len(candidates)-1)
	copy(remaining, candidates[1:])

	// Iteratively select most diverse results
	for len(remaining) > 0 {
		bestScore := -math.MaxFloat64
		bestIdx := -1

		for i, candidate := range remaining {
			// Skip if embedding is not available
			if len(candidate.Embedding) == 0 {
				continue
			}

			// Calculate relevance to query
			relevance := m.cosineSimilarity(candidate.Embedding, queryEmbedding)

			// Calculate maximum similarity to already selected documents
			maxSim := 0.0
			for _, sel := range selected {
				if len(sel.Embedding) == 0 {
					continue
				}
				sim := m.cosineSimilarity(candidate.Embedding, sel.Embedding)
				if sim > maxSim {
					maxSim = sim
				}
			}

			// MMR score: balance relevance and diversity
			mmrScore := m.Lambda*relevance - (1-m.Lambda)*maxSim

			if mmrScore > bestScore {
				bestScore = mmrScore
				bestIdx = i
			}
		}

		// No valid candidate found (all missing embeddings)
		if bestIdx < 0 {
			break
		}

		// Add best candidate to selected
		selected = append(selected, remaining[bestIdx])

		// Remove from remaining
		remaining = append(remaining[:bestIdx], remaining[bestIdx+1:]...)
	}

	return selected
}

// RerankWithScores reranks results and updates their scores based on MMR
func (m *MMR) RerankWithScores(candidates []SearchResult, queryEmbedding []float32) ([]SearchResult, error) {
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("query embedding cannot be empty")
	}

	reranked := m.Rerank(candidates, queryEmbedding)

	// Update scores based on final ranking
	for i := range reranked {
		// Decay score based on position (linear decay)
		positionWeight := 1.0 - (float64(i) / float64(len(reranked)))
		reranked[i].Score = reranked[i].Score * (0.5 + 0.5*positionWeight)
	}

	return reranked, nil
}

// cosineSimilarity calculates cosine similarity between two vectors
func (m *MMR) cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) {
		return 0
	}

	if len(a) == 0 {
		return 0
	}

	var dotProduct, normA, normB float64
	for i := range a {
		dotProduct += float64(a[i] * b[i])
		normA += float64(a[i] * a[i])
		normB += float64(b[i] * b[i])
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// SetLambda updates the lambda parameter
func (m *MMR) SetLambda(lambda float64) {
	if lambda >= 0 && lambda <= 1 {
		m.Lambda = lambda
	}
}

// GetDiversityScore calculates how diverse a set of results is
func (m *MMR) GetDiversityScore(results []SearchResult) float64 {
	if len(results) < 2 {
		return 1.0 // Perfectly diverse (trivially)
	}

	totalSimilarity := 0.0
	comparisons := 0

	for i := 0; i < len(results)-1; i++ {
		if len(results[i].Embedding) == 0 {
			continue
		}

		for j := i + 1; j < len(results); j++ {
			if len(results[j].Embedding) == 0 {
				continue
			}

			sim := m.cosineSimilarity(results[i].Embedding, results[j].Embedding)
			totalSimilarity += sim
			comparisons++
		}
	}

	if comparisons == 0 {
		return 1.0
	}

	avgSimilarity := totalSimilarity / float64(comparisons)

	// Diversity is inverse of average similarity
	return 1.0 - avgSimilarity
}
