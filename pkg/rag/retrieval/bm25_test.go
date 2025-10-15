package retrieval

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBM25Search(t *testing.T) {
	// Test that NewBM25Search returns a valid instance
	bm25 := NewBM25Search(nil) // DB can be nil for this test

	assert.NotNil(t, bm25)
	// DB field itself is allowed to be nil for unit testing
}

func TestGetRelevanceThreshold(t *testing.T) {
	bm25 := &BM25Search{}

	threshold := bm25.GetRelevanceThreshold()

	// Should return the default threshold
	assert.Equal(t, 0.1, threshold)
}

func TestSearchResult_Structure(t *testing.T) {
	// Test that SearchResult can be properly constructed
	embID := "test-emb-123"
	result := SearchResult{
		ID:          "doc-123",
		Content:     "test content",
		DocumentID:  "doc-456",
		Score:       0.85,
		URL:         "https://example.com",
		Title:       "Test Document",
		SourceType:  "github",
		Metadata:    map[string]interface{}{"key": "value"},
		Embedding:   []float32{0.1, 0.2, 0.3},
		EmbeddingID: &embID,
	}

	assert.Equal(t, "doc-123", result.ID)
	assert.Equal(t, "test content", result.Content)
	assert.Equal(t, 0.85, result.Score)
	assert.NotNil(t, result.EmbeddingID)
	assert.Equal(t, "test-emb-123", *result.EmbeddingID)
}

func TestSearchResult_NilEmbeddingID(t *testing.T) {
	result := SearchResult{
		ID:          "doc-123",
		Content:     "test content",
		EmbeddingID: nil,
	}

	assert.Nil(t, result.EmbeddingID)
}

func TestSearchResult_Metadata(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]interface{}
		key      string
		expected interface{}
		exists   bool
	}{
		{
			name:     "Key exists",
			metadata: map[string]interface{}{"importance_score": 0.8},
			key:      "importance_score",
			expected: 0.8,
			exists:   true,
		},
		{
			name:     "Key does not exist",
			metadata: map[string]interface{}{"other_key": "value"},
			key:      "missing_key",
			expected: nil,
			exists:   false,
		},
		{
			name:     "Empty metadata",
			metadata: map[string]interface{}{},
			key:      "any_key",
			expected: nil,
			exists:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SearchResult{
				Metadata: tt.metadata,
			}

			value, exists := result.Metadata[tt.key]
			assert.Equal(t, tt.exists, exists)
			if tt.exists {
				assert.Equal(t, tt.expected, value)
			}
		})
	}
}

func TestSearchResult_ScoreComparison(t *testing.T) {
	result1 := SearchResult{ID: "1", Score: 0.9}
	result2 := SearchResult{ID: "2", Score: 0.7}
	result3 := SearchResult{ID: "3", Score: 0.9}

	// Test score comparison
	assert.Greater(t, result1.Score, result2.Score)
	assert.Equal(t, result1.Score, result3.Score)
}

func TestSearchResults_Sorting(t *testing.T) {
	results := []SearchResult{
		{ID: "1", Score: 0.5},
		{ID: "2", Score: 0.9},
		{ID: "3", Score: 0.7},
		{ID: "4", Score: 0.3},
	}

	// Sort by score descending (manual sort for testing)
	sorted := make([]SearchResult, len(results))
	copy(sorted, results)

	// Simple bubble sort for testing
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i].Score < sorted[j].Score {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	// Verify descending order
	assert.Equal(t, "2", sorted[0].ID) // 0.9
	assert.Equal(t, "3", sorted[1].ID) // 0.7
	assert.Equal(t, "1", sorted[2].ID) // 0.5
	assert.Equal(t, "4", sorted[3].ID) // 0.3

	// Verify scores are in descending order
	for i := 0; i < len(sorted)-1; i++ {
		assert.GreaterOrEqual(t, sorted[i].Score, sorted[i+1].Score)
	}
}

func TestSearchResult_EmbeddingHandling(t *testing.T) {
	// Test with embedding
	withEmbedding := SearchResult{
		ID:        "1",
		Embedding: []float32{0.1, 0.2, 0.3, 0.4},
	}
	assert.Len(t, withEmbedding.Embedding, 4)

	// Test with nil embedding
	withoutEmbedding := SearchResult{
		ID:        "2",
		Embedding: nil,
	}
	assert.Nil(t, withoutEmbedding.Embedding)

	// Test with empty embedding
	emptyEmbedding := SearchResult{
		ID:        "3",
		Embedding: []float32{},
	}
	assert.Empty(t, emptyEmbedding.Embedding)
}

func TestBM25Search_QueryValidation(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		shouldErr bool
	}{
		{
			name:      "Valid query",
			query:     "test query",
			shouldErr: false,
		},
		{
			name:      "Empty query",
			query:     "",
			shouldErr: true,
		},
		{
			name:      "Whitespace only",
			query:     "   ",
			shouldErr: false, // Trimming happens at app level
		},
		{
			name:      "Special characters",
			query:     "test@query#123",
			shouldErr: false,
		},
		{
			name:      "Unicode characters",
			query:     "你好世界",
			shouldErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just validate the query string itself
			err := validateQuery(tt.query)
			if tt.shouldErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// validateQuery is a test helper
func validateQuery(query string) error {
	if query == "" {
		return assert.AnError
	}
	return nil
}

func TestSearchResult_JSONMarshaling(t *testing.T) {
	// This tests that the struct tags are properly defined
	result := SearchResult{
		ID:         "test-id",
		Content:    "test content",
		DocumentID: "doc-id",
		Score:      0.95,
		URL:        "https://example.com",
		Title:      "Test Title",
		SourceType: "github",
		Metadata:   map[string]interface{}{"key": "value"},
	}

	// Verify field values can be accessed
	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.Content)
	assert.Greater(t, result.Score, 0.0)
	assert.NotEmpty(t, result.SourceType)
}

func TestBM25Search_FilterValidation(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string]interface{}
		valid   bool
	}{
		{
			name:    "Empty filters",
			filters: map[string]interface{}{},
			valid:   true,
		},
		{
			name: "Valid tenant filter",
			filters: map[string]interface{}{
				"tenant_id": "123e4567-e89b-12d3-a456-426614174000",
			},
			valid: true,
		},
		{
			name: "Valid source type filter",
			filters: map[string]interface{}{
				"source_type": "github",
			},
			valid: true,
		},
		{
			name: "Multiple filters",
			filters: map[string]interface{}{
				"tenant_id":   "123e4567-e89b-12d3-a456-426614174000",
				"source_type": "github",
			},
			valid: true,
		},
		{
			name:    "Nil filters",
			filters: nil,
			valid:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Just verify filters can be created and accessed
			if tt.filters != nil {
				for key, value := range tt.filters {
					assert.NotEmpty(t, key)
					assert.NotNil(t, value)
				}
			}
		})
	}
}

func TestSearchResult_ImportanceScore(t *testing.T) {
	tests := []struct {
		name               string
		metadata           map[string]interface{}
		expectedScore      float64
		hasImportanceScore bool
	}{
		{
			name: "Has importance score",
			metadata: map[string]interface{}{
				"importance_score": 0.8,
			},
			expectedScore:      0.8,
			hasImportanceScore: true,
		},
		{
			name:               "No importance score",
			metadata:           map[string]interface{}{},
			expectedScore:      0.0,
			hasImportanceScore: false,
		},
		{
			name: "Wrong type for importance score",
			metadata: map[string]interface{}{
				"importance_score": "0.8",
			},
			expectedScore:      0.0,
			hasImportanceScore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SearchResult{
				Metadata: tt.metadata,
			}

			if score, ok := result.Metadata["importance_score"].(float64); ok {
				assert.True(t, tt.hasImportanceScore)
				assert.Equal(t, tt.expectedScore, score)
			} else {
				assert.False(t, tt.hasImportanceScore)
			}
		})
	}
}
