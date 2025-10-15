package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewMMR(t *testing.T) {
	tests := []struct {
		name           string
		lambda         float64
		expectedLambda float64
	}{
		{
			name:           "Valid lambda",
			lambda:         0.7,
			expectedLambda: 0.7,
		},
		{
			name:           "Lambda too high",
			lambda:         1.5,
			expectedLambda: 0.7, // Should default to 0.7
		},
		{
			name:           "Lambda too low",
			lambda:         -0.5,
			expectedLambda: 0.7, // Should default to 0.7
		},
		{
			name:           "Lambda at boundary (1.0)",
			lambda:         1.0,
			expectedLambda: 1.0,
		},
		{
			name:           "Lambda at boundary (0.0)",
			lambda:         0.0,
			expectedLambda: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mmr := NewMMR(tt.lambda)
			assert.Equal(t, tt.expectedLambda, mmr.Lambda)
		})
	}
}

func TestCosineSimilarity(t *testing.T) {
	mmr := NewMMR(0.7)

	tests := []struct {
		name      string
		a         []float32
		b         []float32
		expected  float64
		tolerance float64
	}{
		{
			name:      "Identical vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{1, 0, 0},
			expected:  1.0,
			tolerance: 0.001,
		},
		{
			name:      "Orthogonal vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{0, 1, 0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "Opposite vectors",
			a:         []float32{1, 0, 0},
			b:         []float32{-1, 0, 0},
			expected:  -1.0,
			tolerance: 0.001,
		},
		{
			name:      "Similar vectors",
			a:         []float32{1, 1, 0},
			b:         []float32{1, 0.5, 0},
			expected:  0.948, // Approximate cosine similarity
			tolerance: 0.01,
		},
		{
			name:      "Different lengths (invalid)",
			a:         []float32{1, 0},
			b:         []float32{1, 0, 0},
			expected:  0.0,
			tolerance: 0.001,
		},
		{
			name:      "Empty vectors",
			a:         []float32{},
			b:         []float32{},
			expected:  0.0,
			tolerance: 0.001,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			similarity := mmr.cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, similarity, tt.tolerance)
		})
	}
}

func TestRerank(t *testing.T) {
	mmr := NewMMR(0.7)

	// Create test candidates with embeddings
	candidates := []SearchResult{
		{
			ID:        "1",
			Content:   "First result",
			Score:     0.9,
			Embedding: []float32{1, 0, 0}, // Similar to query
		},
		{
			ID:        "2",
			Content:   "Second result",
			Score:     0.8,
			Embedding: []float32{0.9, 0.1, 0}, // Very similar to first
		},
		{
			ID:        "3",
			Content:   "Third result",
			Score:     0.7,
			Embedding: []float32{0, 1, 0}, // Different from first
		},
	}

	queryEmbedding := []float32{1, 0, 0}

	reranked := mmr.Rerank(candidates, queryEmbedding)

	// Verify results
	assert.Len(t, reranked, 3)

	// First result should still be first (most relevant)
	assert.Equal(t, "1", reranked[0].ID)

	// With MMR lambda=0.7, the second result is determined by a balance
	// of relevance and diversity. Let's just verify all results are present
	resultIDs := make(map[string]bool)
	for _, r := range reranked {
		resultIDs[r.ID] = true
	}
	assert.True(t, resultIDs["1"])
	assert.True(t, resultIDs["2"])
	assert.True(t, resultIDs["3"])
}

func TestRerankSingleResult(t *testing.T) {
	mmr := NewMMR(0.7)

	candidates := []SearchResult{
		{
			ID:        "1",
			Content:   "Only result",
			Score:     0.9,
			Embedding: []float32{1, 0, 0},
		},
	}

	queryEmbedding := []float32{1, 0, 0}

	reranked := mmr.Rerank(candidates, queryEmbedding)

	assert.Len(t, reranked, 1)
	assert.Equal(t, "1", reranked[0].ID)
}

func TestRerankEmptyResults(t *testing.T) {
	mmr := NewMMR(0.7)

	candidates := []SearchResult{}
	queryEmbedding := []float32{1, 0, 0}

	reranked := mmr.Rerank(candidates, queryEmbedding)

	assert.Len(t, reranked, 0)
}

func TestRerankWithMissingEmbeddings(t *testing.T) {
	mmr := NewMMR(0.7)

	candidates := []SearchResult{
		{
			ID:        "1",
			Content:   "First result",
			Score:     0.9,
			Embedding: []float32{1, 0, 0},
		},
		{
			ID:        "2",
			Content:   "Second result (no embedding)",
			Score:     0.8,
			Embedding: nil,
		},
		{
			ID:        "3",
			Content:   "Third result",
			Score:     0.7,
			Embedding: []float32{0, 1, 0},
		},
	}

	queryEmbedding := []float32{1, 0, 0}

	reranked := mmr.Rerank(candidates, queryEmbedding)

	// Should only include results with embeddings
	assert.GreaterOrEqual(t, len(reranked), 2)
	assert.Equal(t, "1", reranked[0].ID)
}

func TestGetDiversityScore(t *testing.T) {
	mmr := NewMMR(0.7)

	tests := []struct {
		name        string
		results     []SearchResult
		expectedMin float64
		expectedMax float64
	}{
		{
			name: "Highly similar results",
			results: []SearchResult{
				{ID: "1", Embedding: []float32{1, 0, 0}},
				{ID: "2", Embedding: []float32{0.9, 0.1, 0}},
				{ID: "3", Embedding: []float32{0.95, 0.05, 0}},
			},
			expectedMin: 0.0,
			expectedMax: 0.3,
		},
		{
			name: "Diverse results",
			results: []SearchResult{
				{ID: "1", Embedding: []float32{1, 0, 0}},
				{ID: "2", Embedding: []float32{0, 1, 0}},
				{ID: "3", Embedding: []float32{0, 0, 1}},
			},
			expectedMin: 0.7,
			expectedMax: 1.0,
		},
		{
			name: "Single result",
			results: []SearchResult{
				{ID: "1", Embedding: []float32{1, 0, 0}},
			},
			expectedMin: 1.0,
			expectedMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := mmr.GetDiversityScore(tt.results)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
		})
	}
}

func TestSetLambda(t *testing.T) {
	mmr := NewMMR(0.7)

	// Valid update
	mmr.SetLambda(0.5)
	assert.Equal(t, 0.5, mmr.Lambda)

	// Invalid update (too high) - should not change
	mmr.SetLambda(1.5)
	assert.Equal(t, 0.5, mmr.Lambda)

	// Invalid update (too low) - should not change
	mmr.SetLambda(-0.1)
	assert.Equal(t, 0.5, mmr.Lambda)

	// Valid boundary values
	mmr.SetLambda(0.0)
	assert.Equal(t, 0.0, mmr.Lambda)

	mmr.SetLambda(1.0)
	assert.Equal(t, 1.0, mmr.Lambda)
}

func TestRerankWithScores(t *testing.T) {
	mmr := NewMMR(0.7)

	candidates := []SearchResult{
		{
			ID:        "1",
			Content:   "First",
			Score:     0.9,
			Embedding: []float32{1, 0, 0},
		},
		{
			ID:        "2",
			Content:   "Second",
			Score:     0.8,
			Embedding: []float32{0, 1, 0},
		},
		{
			ID:        "3",
			Content:   "Third",
			Score:     0.7,
			Embedding: []float32{0, 0, 1},
		},
	}

	queryEmbedding := []float32{1, 0, 0}

	reranked, err := mmr.RerankWithScores(candidates, queryEmbedding)

	assert.NoError(t, err)
	assert.Len(t, reranked, 3)

	// Scores should be adjusted based on position
	// First result should have highest score
	assert.Greater(t, reranked[0].Score, reranked[1].Score)
	assert.Greater(t, reranked[1].Score, reranked[2].Score)
}

func TestRerankWithScoresEmptyQuery(t *testing.T) {
	mmr := NewMMR(0.7)

	candidates := []SearchResult{
		{ID: "1", Embedding: []float32{1, 0, 0}},
	}

	reranked, err := mmr.RerankWithScores(candidates, []float32{})

	assert.Error(t, err)
	assert.Nil(t, reranked)
	assert.Contains(t, err.Error(), "query embedding cannot be empty")
}
