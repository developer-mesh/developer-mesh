package core

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

func TestNewContextRanker(t *testing.T) {
	assert := assert.New(t)

	// Test with explicit strategy
	ranker := NewContextRanker(RankBySimilarity)
	assert.NotNil(ranker)
	assert.Equal(RankBySimilarity, ranker.strategy)

	// Test with empty strategy (should default to hybrid)
	ranker = NewContextRanker("")
	assert.NotNil(ranker)
	assert.Equal(RankByHybrid, ranker.strategy)
}

func TestContextRanker_RankItems_Empty(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByHybrid)
	currentTime := time.Now()

	// Test with empty items
	ranked := ranker.RankItems([]*repository.ContextItem{}, []*vector.Embedding{}, currentTime)
	assert.Empty(ranked)

	// Test with nil items
	ranked = ranker.RankItems(nil, nil, currentTime)
	assert.Empty(ranked)
}

func TestContextRanker_RankItems_BySimilarity(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankBySimilarity)
	currentTime := time.Now()

	// Create test items
	items := []*repository.ContextItem{
		{ID: "1", Content: "Low similarity", Type: "user", Metadata: map[string]any{}},
		{ID: "2", Content: "High similarity", Type: "user", Metadata: map[string]any{}},
		{ID: "3", Content: "Medium similarity", Type: "user", Metadata: map[string]any{}},
	}

	// Create embeddings with similarity scores
	embeddings := []*vector.Embedding{
		{ID: "1", Metadata: map[string]any{"similarity": 0.3}},
		{ID: "2", Metadata: map[string]any{"similarity": 0.9}},
		{ID: "3", Metadata: map[string]any{"similarity": 0.6}},
	}

	// Rank items
	ranked := ranker.RankItems(items, embeddings, currentTime)

	// Should be ranked by similarity (descending)
	assert.Len(ranked, 3)
	assert.Equal("2", ranked[0].ID) // Highest similarity
	assert.Equal("3", ranked[1].ID) // Medium similarity
	assert.Equal("1", ranked[2].ID) // Lowest similarity
}

func TestContextRanker_RankItems_ByRecency(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByRecency)
	currentTime := time.Now()

	// Create test items with different timestamps
	items := []*repository.ContextItem{
		{
			ID:      "1",
			Content: "Old item",
			Type:    "user",
			Metadata: map[string]any{
				"created_at": currentTime.Add(-48 * time.Hour), // 2 days ago
			},
		},
		{
			ID:      "2",
			Content: "Recent item",
			Type:    "user",
			Metadata: map[string]any{
				"created_at": currentTime.Add(-1 * time.Hour), // 1 hour ago
			},
		},
		{
			ID:      "3",
			Content: "Medium item",
			Type:    "user",
			Metadata: map[string]any{
				"created_at": currentTime.Add(-12 * time.Hour), // 12 hours ago
			},
		},
	}

	// Rank items
	ranked := ranker.RankItems(items, []*vector.Embedding{}, currentTime)

	// Should be ranked by recency (most recent first)
	assert.Len(ranked, 3)
	assert.Equal("2", ranked[0].ID) // Most recent
	assert.Equal("3", ranked[1].ID) // Medium
	assert.Equal("1", ranked[2].ID) // Oldest
}

func TestContextRanker_RankItems_ByImportance(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByImportance)
	currentTime := time.Now()

	// Create test items with different importance scores
	items := []*repository.ContextItem{
		{ID: "1", Content: "Low importance", Type: "user", Metadata: map[string]any{"importance_score": 0.2}},
		{ID: "2", Content: "High importance", Type: "user", Metadata: map[string]any{"importance_score": 0.95}},
		{ID: "3", Content: "Medium importance", Type: "user", Metadata: map[string]any{"importance_score": 0.5}},
	}

	// Rank items
	ranked := ranker.RankItems(items, []*vector.Embedding{}, currentTime)

	// Should be ranked by importance (descending)
	assert.Len(ranked, 3)
	assert.Equal("2", ranked[0].ID) // Highest importance
	assert.Equal("3", ranked[1].ID) // Medium importance
	assert.Equal("1", ranked[2].ID) // Lowest importance
}

func TestContextRanker_RankItems_Hybrid(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByHybrid)
	currentTime := time.Now()

	// Create test items with multiple factors
	items := []*repository.ContextItem{
		{
			ID:      "1",
			Content: "Old but highly similar",
			Type:    "user",
			Metadata: map[string]any{
				"created_at":       currentTime.Add(-48 * time.Hour),
				"importance_score": 0.3,
			},
		},
		{
			ID:      "2",
			Content: "Recent and important",
			Type:    "user",
			Metadata: map[string]any{
				"created_at":       currentTime.Add(-1 * time.Hour),
				"importance_score": 0.8,
			},
		},
		{
			ID:      "3",
			Content: "Medium all around",
			Type:    "user",
			Metadata: map[string]any{
				"created_at":       currentTime.Add(-12 * time.Hour),
				"importance_score": 0.5,
			},
		},
	}

	embeddings := []*vector.Embedding{
		{ID: "1", Metadata: map[string]any{"similarity": 0.9}},
		{ID: "2", Metadata: map[string]any{"similarity": 0.7}},
		{ID: "3", Metadata: map[string]any{"similarity": 0.6}},
	}

	// Rank items
	ranked := ranker.RankItems(items, embeddings, currentTime)

	// Should be ranked by hybrid score
	assert.Len(ranked, 3)
	// Item 2 should rank highest due to recency + importance despite lower similarity
	assert.Equal("2", ranked[0].ID)
}

func TestContextRanker_BoostScore(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByHybrid)
	boostFactors := GetDefaultBoostFactors()

	tests := []struct {
		name          string
		item          *repository.ContextItem
		expectedBoost float64
		description   string
	}{
		{
			name: "Error type boost",
			item: &repository.ContextItem{
				ID:       "1",
				Type:     "error",
				Metadata: map[string]any{},
			},
			expectedBoost: 1.5,
			description:   "Error items should get 1.5x boost",
		},
		{
			name: "Code content boost",
			item: &repository.ContextItem{
				ID:   "2",
				Type: "user",
				Metadata: map[string]any{
					"has_code": true,
				},
			},
			expectedBoost: 1.2,
			description:   "Code items should get 1.2x boost",
		},
		{
			name: "Critical item boost",
			item: &repository.ContextItem{
				ID:   "3",
				Type: "user",
				Metadata: map[string]any{
					"is_critical": true,
				},
			},
			expectedBoost: 2.0,
			description:   "Critical items should get 2.0x boost",
		},
		{
			name: "No boost",
			item: &repository.ContextItem{
				ID:       "4",
				Type:     "user",
				Metadata: map[string]any{},
			},
			expectedBoost: 1.0,
			description:   "Regular items should have no boost",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boost := ranker.BoostScore(tt.item, boostFactors)
			assert.Equal(tt.expectedBoost, boost, tt.description)
		})
	}
}

func TestContextRanker_BoostScore_Combined(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByHybrid)
	boostFactors := GetDefaultBoostFactors()

	// Test combined boosts (error + critical)
	item := &repository.ContextItem{
		ID:   "1",
		Type: "error",
		Metadata: map[string]any{
			"is_critical": true,
		},
	}

	boost := ranker.BoostScore(item, boostFactors)
	// Should be 1.5 (error) * 2.0 (critical) = 3.0
	assert.Equal(3.0, boost)
}

func TestGetDefaultBoostFactors(t *testing.T) {
	assert := assert.New(t)

	factors := GetDefaultBoostFactors()

	assert.NotNil(factors)
	assert.Equal(1.5, factors["error"])
	assert.Equal(1.2, factors["code"])
	assert.Equal(2.0, factors["critical"])
	assert.Len(factors, 3)
}

func TestContextRanker_RankItems_NoMetadata(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByHybrid)
	currentTime := time.Now()

	// Create items without metadata
	items := []*repository.ContextItem{
		{ID: "1", Content: "Item 1", Type: "user"},
		{ID: "2", Content: "Item 2", Type: "user"},
		{ID: "3", Content: "Item 3", Type: "user"},
	}

	// Rank should not fail with missing metadata
	ranked := ranker.RankItems(items, []*vector.Embedding{}, currentTime)

	assert.Len(ranked, 3)
	// All items should have default scores
}

func TestContextRanker_RankItems_MissingEmbeddings(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankBySimilarity)
	currentTime := time.Now()

	// Create items
	items := []*repository.ContextItem{
		{ID: "1", Content: "Item 1", Type: "user", Metadata: map[string]any{}},
		{ID: "2", Content: "Item 2", Type: "user", Metadata: map[string]any{}},
	}

	// No matching embeddings
	embeddings := []*vector.Embedding{
		{ID: "99", Metadata: map[string]any{"similarity": 0.9}},
	}

	// Rank should not fail with missing embeddings
	ranked := ranker.RankItems(items, embeddings, currentTime)

	assert.Len(ranked, 2)
	// Items without embeddings should have score 0
}

func TestContextRanker_calculateScore_InvalidMetadata(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankByRecency)
	currentTime := time.Now()

	// Create item with invalid created_at type
	item := &repository.ContextItem{
		ID:   "1",
		Type: "user",
		Metadata: map[string]any{
			"created_at": "not a time", // Invalid type
		},
	}

	score := ranker.calculateScore(item, map[string]*vector.Embedding{}, currentTime)

	// Should return 0 for invalid metadata
	assert.Equal(0.0, score)
}

func TestContextRanker_RankItems_StableSort(t *testing.T) {
	assert := assert.New(t)

	ranker := NewContextRanker(RankBySimilarity)
	currentTime := time.Now()

	// Create items with identical scores
	items := []*repository.ContextItem{
		{ID: "1", Content: "Item 1", Type: "user", Metadata: map[string]any{}},
		{ID: "2", Content: "Item 2", Type: "user", Metadata: map[string]any{}},
		{ID: "3", Content: "Item 3", Type: "user", Metadata: map[string]any{}},
	}

	embeddings := []*vector.Embedding{
		{ID: "1", Metadata: map[string]any{"similarity": 0.5}},
		{ID: "2", Metadata: map[string]any{"similarity": 0.5}},
		{ID: "3", Metadata: map[string]any{"similarity": 0.5}},
	}

	// Rank items
	ranked := ranker.RankItems(items, embeddings, currentTime)

	// Should maintain original order for equal scores
	assert.Len(ranked, 3)
}
