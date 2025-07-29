package rerank

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewCrossEncoderReranker(t *testing.T) {
	mockProvider := new(providers.MockRerankProvider)
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	tests := []struct {
		name     string
		provider providers.RerankProvider
		config   *CrossEncoderConfig
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "valid config",
			provider: mockProvider,
			config: &CrossEncoderConfig{
				Model:     "test-model",
				BatchSize: 5,
			},
			wantErr: false,
		},
		{
			name:     "nil provider",
			provider: nil,
			config:   &CrossEncoderConfig{Model: "test"},
			wantErr:  true,
			errMsg:   "provider is required",
		},
		{
			name:     "nil config",
			provider: mockProvider,
			config:   nil,
			wantErr:  true,
			errMsg:   "config is required",
		},
		{
			name:     "default values",
			provider: mockProvider,
			config:   &CrossEncoderConfig{Model: "test"},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reranker, err := NewCrossEncoderReranker(tt.provider, tt.config, logger, metrics)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
				assert.Nil(t, reranker)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, reranker)

				// Check defaults were set
				if tt.config.BatchSize == 0 {
					assert.Equal(t, 10, reranker.config.BatchSize)
				}
				if tt.config.MaxConcurrency == 0 {
					assert.Equal(t, 3, reranker.config.MaxConcurrency)
				}
			}
		})
	}
}

func TestCrossEncoderReranker_Rerank(t *testing.T) {
	ctx := context.Background()

	t.Run("successful reranking", func(t *testing.T) {
		mockProvider := new(providers.MockRerankProvider)
		logger := observability.NewLogger("test")
		metrics := observability.NewMetricsClient()

		config := &CrossEncoderConfig{
			Model:           "test-model",
			BatchSize:       2,
			MaxConcurrency:  1,
			TimeoutPerBatch: 1 * time.Second,
		}

		reranker, err := NewCrossEncoderReranker(mockProvider, config, logger, metrics)
		require.NoError(t, err)

		// Test data
		results := []SearchResult{
			{ID: "1", Content: "First document", Score: 0.5},
			{ID: "2", Content: "Second document", Score: 0.6},
			{ID: "3", Content: "Third document", Score: 0.4},
		}

		// Mock provider responses for batches
		// Batch 1: docs 0,1
		mockProvider.On("Rerank", mock.Anything, providers.RerankRequest{
			Query:     "test query",
			Documents: []string{"First document", "Second document"},
			Model:     "test-model",
		}).Return(&providers.RerankResponse{
			Results: []providers.RerankResult{
				{Index: 0, Score: 0.9, Document: "First document"},
				{Index: 1, Score: 0.8, Document: "Second document"},
			},
			Model: "test-model",
		}, nil).Once()

		// Batch 2: doc 2
		mockProvider.On("Rerank", mock.Anything, providers.RerankRequest{
			Query:     "test query",
			Documents: []string{"Third document"},
			Model:     "test-model",
		}).Return(&providers.RerankResponse{
			Results: []providers.RerankResult{
				{Index: 0, Score: 0.95, Document: "Third document"},
			},
			Model: "test-model",
		}, nil).Once()

		opts := &RerankOptions{TopK: 2}
		reranked, err := reranker.Rerank(ctx, "test query", results, opts)

		assert.NoError(t, err)
		assert.Len(t, reranked, 2) // TopK = 2

		// Check ordering (highest score first)
		assert.Equal(t, "3", reranked[0].ID)
		assert.Equal(t, float32(0.95), reranked[0].Score)
		assert.Equal(t, "1", reranked[1].ID)
		assert.Equal(t, float32(0.9), reranked[1].Score)

		// Check metadata
		assert.Equal(t, "test-model", reranked[0].Metadata["rerank_model"])
		assert.True(t, reranked[0].Metadata["reranked"].(bool))
		assert.Equal(t, float32(0.4), reranked[0].Metadata["original_score"])

		mockProvider.AssertExpectations(t)
	})

	t.Run("empty results", func(t *testing.T) {
		mockProvider := new(providers.MockRerankProvider)
		reranker, err := NewCrossEncoderReranker(mockProvider, &CrossEncoderConfig{Model: "test"}, nil, nil)
		require.NoError(t, err)

		reranked, err := reranker.Rerank(ctx, "query", []SearchResult{}, nil)
		assert.NoError(t, err)
		assert.Empty(t, reranked)
	})

	t.Run("batch failure with graceful degradation", func(t *testing.T) {
		mockProvider := new(providers.MockRerankProvider)
		logger := observability.NewLogger("test")
		metrics := observability.NewMetricsClient()

		config := &CrossEncoderConfig{
			Model:     "test-model",
			BatchSize: 2,
		}

		reranker, err := NewCrossEncoderReranker(mockProvider, config, logger, metrics)
		require.NoError(t, err)

		results := []SearchResult{
			{ID: "1", Content: "First", Score: 0.5},
			{ID: "2", Content: "Second", Score: 0.6},
		}

		// Mock provider to fail
		mockProvider.On("Rerank", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("API error")).Times(4) // Fail all retries

		// Should return original results on failure
		reranked, err := reranker.Rerank(ctx, "test", results, nil)
		assert.NoError(t, err) // No error due to graceful degradation
		assert.Len(t, reranked, 2)

		// Check that original scores are preserved
		assert.Equal(t, float32(0.6), reranked[0].Score)
		assert.Equal(t, "2", reranked[0].ID)
	})
}

func TestCrossEncoderReranker_createBatches(t *testing.T) {
	reranker := &CrossEncoderReranker{}

	results := []SearchResult{
		{ID: "1"}, {ID: "2"}, {ID: "3"}, {ID: "4"}, {ID: "5"},
	}

	tests := []struct {
		name      string
		results   []SearchResult
		batchSize int
		expected  int // expected number of batches
	}{
		{
			name:      "even split",
			results:   results[:4],
			batchSize: 2,
			expected:  2,
		},
		{
			name:      "uneven split",
			results:   results,
			batchSize: 2,
			expected:  3,
		},
		{
			name:      "single batch",
			results:   results,
			batchSize: 10,
			expected:  1,
		},
		{
			name:      "batch size 1",
			results:   results[:3],
			batchSize: 1,
			expected:  3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			batches := reranker.createBatches(tt.results, tt.batchSize)
			assert.Len(t, batches, tt.expected)

			// Verify all results are included
			totalResults := 0
			for _, batch := range batches {
				totalResults += len(batch)
			}
			assert.Equal(t, len(tt.results), totalResults)
		})
	}
}
