package rerank

import (
	"context"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestIntegration_MultiStageReranking(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	// Create a simple rerank provider
	simpleProvider := providers.NewSimpleRerankProvider("test-reranker-v1")

	// Create cross-encoder reranker
	crossEncoder, err := NewCrossEncoderReranker(
		simpleProvider,
		&CrossEncoderConfig{
			Model:     "test-reranker-v1",
			BatchSize: 5,
		},
		logger,
		metrics,
	)
	require.NoError(t, err)

	// Create MMR reranker with mock embedding service
	mockEmbedding := new(MockEmbeddingService)
	mmrReranker, err := NewMMRReranker(0.7, mockEmbedding, logger)
	require.NoError(t, err)

	// Create multi-stage reranker
	multiStage := NewMultiStageReranker([]RerankStage{
		{
			Reranker: crossEncoder,
			TopK:     10,
			Weight:   0.7,
		},
		{
			Reranker: mmrReranker,
			TopK:     5,
			Weight:   0.3,
		},
	}, logger)

	// Test data
	query := "machine learning algorithms"
	results := []SearchResult{
		{ID: "1", Content: "Introduction to machine learning algorithms", Score: 0.5},
		{ID: "2", Content: "Deep learning neural networks", Score: 0.6},
		{ID: "3", Content: "Machine learning in production", Score: 0.55},
		{ID: "4", Content: "Algorithm design patterns", Score: 0.4},
		{ID: "5", Content: "Statistical learning theory", Score: 0.45},
		{ID: "6", Content: "Natural language processing", Score: 0.42},
		{ID: "7", Content: "Computer vision applications", Score: 0.38},
		{ID: "8", Content: "Reinforcement learning basics", Score: 0.5},
		{ID: "9", Content: "Data preprocessing techniques", Score: 0.35},
		{ID: "10", Content: "Model evaluation metrics", Score: 0.4},
	}

	// Mock embeddings for MMR stage
	embeddings := [][]float32{
		{0.1, 0.9, 0.0, 0.1},    // Doc 1 - ML focused
		{0.15, 0.85, 0.05, 0.1}, // Doc 2 - DL focused (similar to 1)
		{0.12, 0.88, 0.02, 0.1}, // Doc 3 - ML focused (similar to 1)
		{0.7, 0.2, 0.1, 0.0},    // Doc 4 - Algorithms (different)
		{0.3, 0.6, 0.1, 0.0},    // Doc 5 - Statistics (somewhat different)
		{0.5, 0.3, 0.2, 0.0},    // Doc 6 - NLP (different)
		{0.6, 0.2, 0.2, 0.0},    // Doc 7 - CV (different)
		{0.2, 0.7, 0.1, 0.0},    // Doc 8 - RL (ML related)
		{0.8, 0.1, 0.1, 0.0},    // Doc 9 - Data prep (different)
		{0.25, 0.65, 0.1, 0.0},  // Doc 10 - Evaluation (ML related)
	}

	// Mock query embedding
	queryEmbedding := []float32{0.11, 0.89, 0.01, 0.1}
	mockEmbedding.On("GenerateEmbedding", mock.Anything, query, "search_query", "").
		Return(&EmbeddingVector{Vector: queryEmbedding}, nil).Once()

	// Mock document embeddings (only for top results from stage 1)
	for i := 0; i < 10; i++ {
		mockEmbedding.On("GenerateEmbedding", mock.Anything, results[i].Content, "document", "").
			Return(&EmbeddingVector{Vector: embeddings[i]}, nil).Maybe()
	}

	// Run multi-stage reranking
	opts := &RerankOptions{TopK: 5}
	reranked, err := multiStage.Rerank(ctx, query, results, opts)

	assert.NoError(t, err)
	assert.Len(t, reranked, 5)

	// Verify results are reranked
	// Stage 1 (cross-encoder) should boost relevant results
	// Stage 2 (MMR) should add diversity

	// Check that we have a mix of similar and diverse results
	seenTopics := make(map[string]bool)
	for _, r := range reranked {
		t.Logf("Result %s: %s (score: %.3f)", r.ID, r.Content, r.Score)

		// Categorize content
		topic := categorizeContent(r.Content)
		seenTopics[topic] = true
	}

	// Should have at least 3 different topics for diversity
	assert.GreaterOrEqual(t, len(seenTopics), 3, "Results should be diverse")

	// Clean up
	assert.NoError(t, multiStage.Close())
}

func categorizeContent(content string) string {
	switch {
	case contains(content, "machine learning", "statistical learning"):
		return "ml_core"
	case contains(content, "deep learning", "neural"):
		return "deep_learning"
	case contains(content, "algorithm", "design"):
		return "algorithms"
	case contains(content, "reinforcement"):
		return "rl"
	case contains(content, "vision", "nlp", "language"):
		return "applications"
	case contains(content, "data", "preprocessing", "evaluation"):
		return "data_tools"
	default:
		return "other"
	}
}

func contains(text string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(strings.ToLower(text), keyword) {
			return true
		}
	}
	return false
}
