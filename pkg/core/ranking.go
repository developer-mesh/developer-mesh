// Story 3.1: Relevance Ranking Algorithm
package core

import (
	"sort"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// RankingStrategy defines how to rank context items
type RankingStrategy string

const (
	RankBySimilarity RankingStrategy = "similarity"
	RankByRecency    RankingStrategy = "recency"
	RankByImportance RankingStrategy = "importance"
	RankByHybrid     RankingStrategy = "hybrid"
)

// ContextRanker ranks context items by relevance
type ContextRanker struct {
	strategy RankingStrategy
}

// NewContextRanker creates a new context ranker
func NewContextRanker(strategy RankingStrategy) *ContextRanker {
	if strategy == "" {
		strategy = RankByHybrid
	}
	return &ContextRanker{strategy: strategy}
}

// RankItems ranks context items based on embeddings and metadata
func (r *ContextRanker) RankItems(
	items []*repository.ContextItem,
	embeddings []*vector.Embedding,
	currentTime time.Time,
) []*repository.ContextItem {
	// Create map for quick embedding lookup
	embeddingMap := make(map[string]*vector.Embedding)
	for _, emb := range embeddings {
		embeddingMap[emb.ID] = emb
	}

	// Calculate scores for each item
	type scoredItem struct {
		item  *repository.ContextItem
		score float64
	}

	scoredItems := make([]scoredItem, 0, len(items))

	for _, item := range items {
		score := r.calculateScore(item, embeddingMap, currentTime)
		scoredItems = append(scoredItems, scoredItem{
			item:  item,
			score: score,
		})
	}

	// Sort by score (descending)
	sort.Slice(scoredItems, func(i, j int) bool {
		return scoredItems[i].score > scoredItems[j].score
	})

	// Extract sorted items
	ranked := make([]*repository.ContextItem, len(scoredItems))
	for i, scored := range scoredItems {
		ranked[i] = scored.item
	}

	return ranked
}

// calculateScore computes relevance score for an item
func (r *ContextRanker) calculateScore(
	item *repository.ContextItem,
	embeddingMap map[string]*vector.Embedding,
	currentTime time.Time,
) float64 {
	var score float64

	switch r.strategy {
	case RankBySimilarity:
		// Pure similarity score from embedding
		if emb, exists := embeddingMap[item.ID]; exists {
			if similarity, ok := emb.Metadata["similarity"].(float64); ok {
				score = similarity
			}
		}

	case RankByRecency:
		// Time decay factor (exponential decay over 24 hours)
		if createdAt, ok := item.Metadata["created_at"].(time.Time); ok {
			hoursSince := currentTime.Sub(createdAt).Hours()
			score = 1.0 / (1.0 + hoursSince/24.0)
		}

	case RankByImportance:
		// Use importance score from metadata
		if importance, ok := item.Metadata["importance_score"].(float64); ok {
			score = importance
		}

	case RankByHybrid:
		// Combine multiple factors
		var similarityScore, recencyScore, importanceScore float64

		// Similarity component (weight: 0.5)
		if emb, exists := embeddingMap[item.ID]; exists {
			if similarity, ok := emb.Metadata["similarity"].(float64); ok {
				similarityScore = similarity * 0.5
			}
		}

		// Recency component (weight: 0.3)
		if createdAt, ok := item.Metadata["created_at"].(time.Time); ok {
			hoursSince := currentTime.Sub(createdAt).Hours()
			recencyScore = (1.0 / (1.0 + hoursSince/24.0)) * 0.3
		}

		// Importance component (weight: 0.2)
		if importance, ok := item.Metadata["importance_score"].(float64); ok {
			importanceScore = importance * 0.2
		} else {
			importanceScore = 0.5 * 0.2 // Default importance
		}

		score = similarityScore + recencyScore + importanceScore
	}

	return score
}

// BoostScore applies additional boosting factors
func (r *ContextRanker) BoostScore(item *repository.ContextItem, boostFactors map[string]float64) float64 {
	baseScore := 1.0

	// Check for error messages (boost by 1.5x)
	if item.Type == "error" {
		if boost, ok := boostFactors["error"]; ok {
			baseScore *= boost
		}
	}

	// Check for code content (boost by 1.2x)
	if _, hasCode := item.Metadata["has_code"].(bool); hasCode {
		if boost, ok := boostFactors["code"]; ok {
			baseScore *= boost
		}
	}

	// Check for user-marked critical (boost by 2x)
	if critical, ok := item.Metadata["is_critical"].(bool); ok && critical {
		if boost, ok := boostFactors["critical"]; ok {
			baseScore *= boost
		}
	}

	return baseScore
}

// GetDefaultBoostFactors returns default boost factors
func GetDefaultBoostFactors() map[string]float64 {
	return map[string]float64{
		"error":    1.5,
		"code":     1.2,
		"critical": 2.0,
	}
}
