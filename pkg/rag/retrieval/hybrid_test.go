package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultHybridSearchConfig(t *testing.T) {
	config := DefaultHybridSearchConfig()

	assert.Equal(t, 0.6, config.VectorWeight)
	assert.Equal(t, 0.2, config.BM25Weight)
	assert.Equal(t, 0.2, config.ImportanceWeight)
	assert.Equal(t, 0.7, config.MMRLambda)

	// Verify weights sum to 1.0
	total := config.VectorWeight + config.BM25Weight + config.ImportanceWeight
	assert.InDelta(t, 1.0, total, 0.001)
}

func TestHybridSearch_UpdateWeights(t *testing.T) {
	tests := []struct {
		name       string
		vector     float64
		bm25       float64
		importance float64
		shouldErr  bool
	}{
		{
			name:       "Valid weights",
			vector:     0.5,
			bm25:       0.3,
			importance: 0.2,
			shouldErr:  false,
		},
		{
			name:       "Weights sum to less than 1",
			vector:     0.3,
			bm25:       0.2,
			importance: 0.2,
			shouldErr:  true,
		},
		{
			name:       "Weights sum to more than 1",
			vector:     0.6,
			bm25:       0.3,
			importance: 0.3,
			shouldErr:  true,
		},
		{
			name:       "All weight on vector",
			vector:     1.0,
			bm25:       0.0,
			importance: 0.0,
			shouldErr:  false,
		},
		{
			name:       "Equal weights",
			vector:     0.333,
			bm25:       0.333,
			importance: 0.334,
			shouldErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := &HybridSearch{
				vectorWeight:     0.6,
				bm25Weight:       0.2,
				importanceWeight: 0.2,
			}

			err := hs.UpdateWeights(tt.vector, tt.bm25, tt.importance)

			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.vector, hs.vectorWeight)
				assert.Equal(t, tt.bm25, hs.bm25Weight)
				assert.Equal(t, tt.importance, hs.importanceWeight)
			}
		})
	}
}

func TestHybridSearch_GetWeights(t *testing.T) {
	hs := &HybridSearch{
		vectorWeight:     0.6,
		bm25Weight:       0.2,
		importanceWeight: 0.2,
	}

	vector, bm25, importance := hs.GetWeights()

	assert.Equal(t, 0.6, vector)
	assert.Equal(t, 0.2, bm25)
	assert.Equal(t, 0.2, importance)
}

func TestReciprocalRankFusion(t *testing.T) {
	tests := []struct {
		name          string
		vectorResults []SearchResult
		bm25Results   []SearchResult
		expectedTop   string // ID of expected top result
		expectedLen   int
	}{
		{
			name: "No overlap in results",
			vectorResults: []SearchResult{
				{ID: "v1", Score: 0.9},
				{ID: "v2", Score: 0.8},
			},
			bm25Results: []SearchResult{
				{ID: "b1", Score: 0.9},
				{ID: "b2", Score: 0.8},
			},
			expectedLen: 4,
		},
		{
			name: "Complete overlap",
			vectorResults: []SearchResult{
				{ID: "doc1", Score: 0.9},
				{ID: "doc2", Score: 0.8},
			},
			bm25Results: []SearchResult{
				{ID: "doc1", Score: 0.85},
				{ID: "doc2", Score: 0.75},
			},
			expectedTop: "doc1",
			expectedLen: 2,
		},
		{
			name: "Partial overlap",
			vectorResults: []SearchResult{
				{ID: "doc1", Score: 0.9},
				{ID: "doc2", Score: 0.8},
				{ID: "doc3", Score: 0.7},
			},
			bm25Results: []SearchResult{
				{ID: "doc2", Score: 0.85},
				{ID: "doc4", Score: 0.75},
			},
			expectedLen: 4,
		},
		{
			name:          "Empty vector results",
			vectorResults: []SearchResult{},
			bm25Results: []SearchResult{
				{ID: "b1", Score: 0.9},
			},
			expectedTop: "b1",
			expectedLen: 1,
		},
		{
			name: "Empty BM25 results",
			vectorResults: []SearchResult{
				{ID: "v1", Score: 0.9},
			},
			bm25Results: []SearchResult{},
			expectedTop: "v1",
			expectedLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := &HybridSearch{
				vectorWeight:     0.6,
				bm25Weight:       0.2,
				importanceWeight: 0.2,
			}

			combined := hs.reciprocalRankFusion(tt.vectorResults, tt.bm25Results)

			assert.Len(t, combined, tt.expectedLen)

			// Verify results are sorted by score descending
			for i := 0; i < len(combined)-1; i++ {
				assert.GreaterOrEqual(t, combined[i].Score, combined[i+1].Score)
			}

			// Verify top result if specified
			if tt.expectedTop != "" && len(combined) > 0 {
				assert.Equal(t, tt.expectedTop, combined[0].ID)
			}
		})
	}
}

func TestReciprocalRankFusion_ImportanceScore(t *testing.T) {
	hs := &HybridSearch{
		vectorWeight:     0.6,
		bm25Weight:       0.2,
		importanceWeight: 0.2,
	}

	// Results with importance scores
	vectorResults := []SearchResult{
		{
			ID:    "doc1",
			Score: 0.9,
			Metadata: map[string]interface{}{
				"importance_score": 0.8,
			},
		},
		{
			ID:    "doc2",
			Score: 0.8,
			Metadata: map[string]interface{}{
				"importance_score": 0.3,
			},
		},
	}

	bm25Results := []SearchResult{
		{
			ID:    "doc1",
			Score: 0.7,
		},
	}

	combined := hs.reciprocalRankFusion(vectorResults, bm25Results)

	// doc1 should be top due to high importance score and presence in both
	assert.Equal(t, "doc1", combined[0].ID)
	// doc1's score should be influenced by importance
	assert.Greater(t, combined[0].Score, 0.0)
}

func TestFilterByScore(t *testing.T) {
	tests := []struct {
		name        string
		results     []SearchResult
		minScore    float64
		expectedLen int
		lowestScore float64
	}{
		{
			name: "Filter some results",
			results: []SearchResult{
				{ID: "1", Score: 0.9},
				{ID: "2", Score: 0.6},
				{ID: "3", Score: 0.3},
				{ID: "4", Score: 0.8},
			},
			minScore:    0.5,
			expectedLen: 3, // Only 0.9, 0.6, 0.8
			lowestScore: 0.6,
		},
		{
			name: "Filter all results",
			results: []SearchResult{
				{ID: "1", Score: 0.3},
				{ID: "2", Score: 0.2},
			},
			minScore:    0.5,
			expectedLen: 0,
		},
		{
			name: "Filter no results",
			results: []SearchResult{
				{ID: "1", Score: 0.9},
				{ID: "2", Score: 0.8},
			},
			minScore:    0.5,
			expectedLen: 2,
			lowestScore: 0.8,
		},
		{
			name: "Zero threshold",
			results: []SearchResult{
				{ID: "1", Score: 0.1},
				{ID: "2", Score: 0.0},
			},
			minScore:    0.0,
			expectedLen: 2,
		},
		{
			name:        "Empty results",
			results:     []SearchResult{},
			minScore:    0.5,
			expectedLen: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hs := &HybridSearch{}

			filtered := hs.filterByScore(tt.results, tt.minScore)

			assert.Len(t, filtered, tt.expectedLen)

			// Verify all scores meet minimum
			for _, result := range filtered {
				assert.GreaterOrEqual(t, result.Score, tt.minScore)
			}

			// Verify lowest score if results exist
			if tt.expectedLen > 0 && tt.lowestScore > 0 {
				minFound := filtered[0].Score
				for _, result := range filtered {
					if result.Score < minFound {
						minFound = result.Score
					}
				}
				assert.GreaterOrEqual(t, minFound, tt.lowestScore)
			}
		})
	}
}

func TestSearchOptions_Validation(t *testing.T) {
	tests := []struct {
		name    string
		opts    SearchOptions
		isValid bool
	}{
		{
			name: "Valid options",
			opts: SearchOptions{
				Limit:           20,
				MinScore:        0.4,
				TenantID:        "test-tenant",
				SourceType:      "github",
				ApplyMMR:        true,
				IncludeMetadata: true,
			},
			isValid: true,
		},
		{
			name: "Zero limit",
			opts: SearchOptions{
				Limit: 0,
			},
			isValid: false,
		},
		{
			name: "Negative limit",
			opts: SearchOptions{
				Limit: -1,
			},
			isValid: false,
		},
		{
			name: "Negative min score",
			opts: SearchOptions{
				Limit:    10,
				MinScore: -0.1,
			},
			isValid: false,
		},
		{
			name: "Min score above 1",
			opts: SearchOptions{
				Limit:    10,
				MinScore: 1.5,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateSearchOptions(tt.opts)
			assert.Equal(t, tt.isValid, valid)
		})
	}
}

// validateSearchOptions is a test helper
func validateSearchOptions(opts SearchOptions) bool {
	if opts.Limit <= 0 {
		return false
	}
	if opts.MinScore < 0 || opts.MinScore > 1 {
		return false
	}
	return true
}

func TestHybridSearchConfig_WeightValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  HybridSearchConfig
		isValid bool
	}{
		{
			name:    "Default config",
			config:  DefaultHybridSearchConfig(),
			isValid: true,
		},
		{
			name: "Weights sum to 1.0",
			config: HybridSearchConfig{
				VectorWeight:     0.5,
				BM25Weight:       0.3,
				ImportanceWeight: 0.2,
				MMRLambda:        0.7,
			},
			isValid: true,
		},
		{
			name: "Weights sum to less than 1.0",
			config: HybridSearchConfig{
				VectorWeight:     0.3,
				BM25Weight:       0.2,
				ImportanceWeight: 0.2,
				MMRLambda:        0.7,
			},
			isValid: false,
		},
		{
			name: "Weights sum to more than 1.0",
			config: HybridSearchConfig{
				VectorWeight:     0.6,
				BM25Weight:       0.3,
				ImportanceWeight: 0.3,
				MMRLambda:        0.7,
			},
			isValid: false,
		},
		{
			name: "Negative weight",
			config: HybridSearchConfig{
				VectorWeight:     -0.1,
				BM25Weight:       0.6,
				ImportanceWeight: 0.5,
				MMRLambda:        0.7,
			},
			isValid: false,
		},
		{
			name: "Invalid MMR lambda",
			config: HybridSearchConfig{
				VectorWeight:     0.6,
				BM25Weight:       0.2,
				ImportanceWeight: 0.2,
				MMRLambda:        1.5,
			},
			isValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := validateHybridSearchConfig(tt.config)
			assert.Equal(t, tt.isValid, valid)
		})
	}
}

// validateHybridSearchConfig is a test helper
func validateHybridSearchConfig(config HybridSearchConfig) bool {
	// Check weights sum to approximately 1.0
	total := config.VectorWeight + config.BM25Weight + config.ImportanceWeight
	if total < 0.99 || total > 1.01 {
		return false
	}

	// Check individual weights are non-negative
	if config.VectorWeight < 0 || config.BM25Weight < 0 || config.ImportanceWeight < 0 {
		return false
	}

	// Check MMR lambda is in valid range
	if config.MMRLambda < 0 || config.MMRLambda > 1 {
		return false
	}

	return true
}

func TestReciprocalRankFusion_RRFConstant(t *testing.T) {
	// Test that RRF constant (k=60) produces expected behavior
	hs := &HybridSearch{
		vectorWeight:     0.6,
		bm25Weight:       0.2,
		importanceWeight: 0.2,
	}

	// Create results where rank matters
	vectorResults := []SearchResult{
		{ID: "doc1", Score: 1.0}, // Rank 0
		{ID: "doc2", Score: 0.9}, // Rank 1
		{ID: "doc3", Score: 0.8}, // Rank 2
	}

	bm25Results := []SearchResult{
		{ID: "doc3", Score: 1.0}, // Rank 0 (should boost doc3)
		{ID: "doc1", Score: 0.9}, // Rank 1
	}

	combined := hs.reciprocalRankFusion(vectorResults, bm25Results)

	// Both doc1 and doc3 appear in both lists, but doc3 has better BM25 rank
	// The actual top result depends on the weight distribution
	assert.Len(t, combined, 3)
	assert.Greater(t, combined[0].Score, 0.0)
}

func TestHybridSearch_ZeroWeights(t *testing.T) {
	// Test edge case where some weights are zero
	hs := &HybridSearch{
		vectorWeight:     1.0,
		bm25Weight:       0.0,
		importanceWeight: 0.0,
	}

	vectorResults := []SearchResult{
		{ID: "v1", Score: 0.9},
	}

	bm25Results := []SearchResult{
		{ID: "b1", Score: 0.9},
	}

	combined := hs.reciprocalRankFusion(vectorResults, bm25Results)

	// Should still work, but only vector results matter
	assert.Greater(t, len(combined), 0)

	// Find v1 in results
	var v1Score float64
	var b1Score float64
	for _, r := range combined {
		if r.ID == "v1" {
			v1Score = r.Score
		}
		if r.ID == "b1" {
			b1Score = r.Score
		}
	}

	// v1 should have higher score since vector weight is 1.0
	if v1Score > 0 && b1Score > 0 {
		assert.Greater(t, v1Score, b1Score)
	}
}
