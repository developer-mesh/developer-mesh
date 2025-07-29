package rerank

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingService is a mock implementation of EmbeddingService
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text, contentType, model string) (*EmbeddingVector, error) {
	args := m.Called(ctx, text, contentType, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingVector), args.Error(1)
}

func TestNewMMRReranker(t *testing.T) {
	mockEmbedding := new(MockEmbeddingService)
	logger := observability.NewLogger("test")

	tests := []struct {
		name       string
		lambda     float64
		embedding  EmbeddingService
		wantErr    bool
		wantLambda float64
	}{
		{
			name:       "valid config",
			lambda:     0.7,
			embedding:  mockEmbedding,
			wantErr:    false,
			wantLambda: 0.7,
		},
		{
			name:      "nil embedding service",
			lambda:    0.5,
			embedding: nil,
			wantErr:   true,
		},
		{
			name:       "lambda too low",
			lambda:     -0.1,
			embedding:  mockEmbedding,
			wantErr:    false,
			wantLambda: 0.5, // Should default to 0.5
		},
		{
			name:       "lambda too high",
			lambda:     1.5,
			embedding:  mockEmbedding,
			wantErr:    false,
			wantLambda: 0.5, // Should default to 0.5
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reranker, err := NewMMRReranker(tt.lambda, tt.embedding, logger)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, reranker)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reranker)
				assert.Equal(t, tt.wantLambda, reranker.lambda)
			}
		})
	}
}

func TestMMRReranker_Rerank(t *testing.T) {
	ctx := context.Background()

	t.Run("successful MMR reranking", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		logger := observability.NewLogger("test")

		reranker, err := NewMMRReranker(0.7, mockEmbedding, logger)
		require.NoError(t, err)

		// Test data - 3 documents with different similarities
		results := []SearchResult{
			{ID: "1", Content: "Machine learning algorithms", Score: 0.9},
			{ID: "2", Content: "Deep learning neural networks", Score: 0.85},
			{ID: "3", Content: "Database systems design", Score: 0.8}, // Different topic
		}

		// Mock embeddings for documents
		embeddings := [][]float32{
			{0.1, 0.9, 0.0},    // Doc 1
			{0.15, 0.85, 0.05}, // Doc 2 (similar to Doc 1)
			{0.8, 0.1, 0.2},    // Doc 3 (different from Doc 1 & 2)
		}

		// Mock query embedding
		queryEmbedding := []float32{0.12, 0.88, 0.02}
		mockEmbedding.On("GenerateEmbedding", mock.Anything, "test query", "search_query", "").
			Return(&EmbeddingVector{Vector: queryEmbedding}, nil).Once()

		// Mock document embeddings
		for i, result := range results {
			mockEmbedding.On("GenerateEmbedding", mock.Anything, result.Content, "document", "").
				Return(&EmbeddingVector{Vector: embeddings[i]}, nil).Once()
		}

		opts := &RerankOptions{TopK: 2}
		reranked, err := reranker.Rerank(ctx, "test query", results, opts)

		assert.NoError(t, err)
		assert.Len(t, reranked, 2)

		// With MMR, Doc 1 should be selected first (highest relevance)
		// The second document should be selected for diversity
		assert.Equal(t, "1", reranked[0].ID)
		// Either Doc 2 or Doc 3 could be selected second (both are different enough)
		assert.Contains(t, []string{"2", "3"}, reranked[1].ID)

		// Check metadata
		assert.Contains(t, reranked[0].Metadata, "mmr_score")
		assert.Equal(t, 0.7, reranked[0].Metadata["mmr_lambda"])

		mockEmbedding.AssertExpectations(t)
	})

	t.Run("single result", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		reranker, err := NewMMRReranker(0.5, mockEmbedding, nil)
		require.NoError(t, err)

		results := []SearchResult{{ID: "1", Content: "Test", Score: 0.5}}
		reranked, err := reranker.Rerank(ctx, "query", results, nil)

		assert.NoError(t, err)
		assert.Equal(t, results, reranked)
		mockEmbedding.AssertNotCalled(t, "GenerateEmbedding")
	})

	t.Run("cached embeddings", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		reranker, err := NewMMRReranker(0.5, mockEmbedding, nil)
		require.NoError(t, err)

		// Results with cached embeddings in metadata
		results := []SearchResult{
			{
				ID:      "1",
				Content: "Test 1",
				Score:   0.5,
				Metadata: map[string]interface{}{
					"embedding": []float32{0.1, 0.2, 0.3},
				},
			},
			{
				ID:      "2",
				Content: "Test 2",
				Score:   0.6,
				Metadata: map[string]interface{}{
					"embedding": []float32{0.4, 0.5, 0.6},
				},
			},
		}

		// Only query embedding should be generated
		mockEmbedding.On("GenerateEmbedding", mock.Anything, "query", "search_query", "").
			Return(&EmbeddingVector{Vector: []float32{0.2, 0.3, 0.4}}, nil).Once()

		reranked, err := reranker.Rerank(ctx, "query", results, nil)
		assert.NoError(t, err)
		assert.Len(t, reranked, 2)

		mockEmbedding.AssertExpectations(t)
		// Should not call GenerateEmbedding for documents (using cached)
		mockEmbedding.AssertNumberOfCalls(t, "GenerateEmbedding", 1)
	})
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name     string
		a        []float32
		b        []float32
		expected float32
	}{
		{
			name:     "identical vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{1, 0, 0},
			expected: 1.0,
		},
		{
			name:     "orthogonal vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{0, 1, 0},
			expected: 0.0,
		},
		{
			name:     "opposite vectors",
			a:        []float32{1, 0, 0},
			b:        []float32{-1, 0, 0},
			expected: -1.0,
		},
		{
			name:     "different lengths",
			a:        []float32{1, 0},
			b:        []float32{1, 0, 0},
			expected: 0.0,
		},
		{
			name:     "zero vectors",
			a:        []float32{0, 0, 0},
			b:        []float32{0, 0, 0},
			expected: 0.0,
		},
		{
			name:     "normalized similar vectors",
			a:        []float32{0.6, 0.8, 0},
			b:        []float32{0.8, 0.6, 0},
			expected: 0.96, // 0.6*0.8 + 0.8*0.6 = 0.48 + 0.48 = 0.96
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cosineSimilarity(tt.a, tt.b)
			assert.InDelta(t, tt.expected, result, 0.0001)
		})
	}
}
