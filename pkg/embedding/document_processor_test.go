package embedding

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingService for testing
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text string, contentType string, contentID string) (*EmbeddingVector, error) {
	args := m.Called(ctx, text, contentType, contentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingVector), args.Error(1)
}

func (m *MockEmbeddingService) BatchGenerateEmbeddings(ctx context.Context, texts []string, contentType string, contentIDs []string) ([]*EmbeddingVector, error) {
	args := m.Called(ctx, texts, contentType, contentIDs)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingVector), args.Error(1)
}

func (m *MockEmbeddingService) GetModelConfig() ModelConfig {
	args := m.Called()
	return args.Get(0).(ModelConfig)
}

func (m *MockEmbeddingService) GetModelDimensions() int {
	args := m.Called()
	return args.Int(0)
}

func TestDocumentProcessor_ProcessDocument(t *testing.T) {
	ctx := context.Background()

	t.Run("successful processing", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		processor := NewDocumentProcessor(mockEmbedding, nil)

		doc := &Document{
			ID:          "test-doc-1",
			Content:     "This is a test document. It contains multiple sentences. The content should be chunked appropriately.",
			ContentType: "text/plain",
			Metadata: map[string]interface{}{
				"source": "test",
			},
		}

		// Use custom config with smaller chunk sizes for this short test text
		config := &ChunkingConfig{
			MinChunkSize:    10, // Very small for testing
			MaxChunkSize:    50,
			TargetChunkSize: 30,
			OverlapSize:     5,
		}

		// Mock embedding creation - use mock.Anything for context due to tracing wrapper
		mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, "text/plain", mock.MatchedBy(func(contentID string) bool {
			return strings.HasPrefix(contentID, "test-doc-1")
		})).Return(&EmbeddingVector{
			ContentID:   "embedding-1",
			ContentType: "text/plain",
			Vector:      []float32{0.1, 0.2, 0.3},
			Metadata:    map[string]interface{}{},
		}, nil)

		embeddings, err := processor.ProcessDocumentWithConfig(ctx, doc, config)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(embeddings), 1) // Should create at least 1 embedding
		assert.Equal(t, "embedding-1", embeddings[0].ContentID)

		mockEmbedding.AssertExpectations(t)
	})

	t.Run("long document chunking", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		processor := NewDocumentProcessor(mockEmbedding, nil)

		// Create a longer document that will be chunked
		longContent := `# Introduction

This is a comprehensive document about software architecture. It covers various aspects of building scalable systems.

## Microservices Architecture

Microservices architecture is an approach to developing applications as a suite of small services. Each service runs in its own process and communicates through well-defined interfaces.

### Benefits of Microservices

1. Independent deployment - Services can be deployed independently
2. Technology diversity - Different services can use different technologies
3. Fault isolation - Failure in one service doesn't affect others
4. Scalability - Services can be scaled independently

### Challenges

However, microservices also come with challenges:
- Increased complexity in service coordination
- Network latency between services
- Data consistency across services
- Operational overhead

## Best Practices

When implementing microservices, consider these best practices:

1. Design for failure - Assume services will fail and plan accordingly
2. Implement proper monitoring and logging
3. Use circuit breakers for resilience
4. Maintain clear service boundaries

## Conclusion

Microservices architecture can provide significant benefits when implemented correctly. However, it's important to understand the trade-offs and ensure your team is prepared for the operational complexity.`

		doc := &Document{
			ID:          "test-doc-2",
			Content:     longContent,
			ContentType: "text/markdown",
			Metadata: map[string]interface{}{
				"source": "architecture-guide",
			},
		}

		// Mock multiple embedding creations (one for each chunk) - use mock.Anything for context
		mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, "text/markdown", mock.MatchedBy(func(contentID string) bool {
			return strings.HasPrefix(contentID, "test-doc-2")
		})).Return(&EmbeddingVector{
			ContentID:   "embedding-chunk",
			ContentType: "text/markdown",
			Vector:      []float32{0.1, 0.2, 0.3},
			Metadata:    map[string]interface{}{},
		}, nil)

		// Use custom config to ensure multiple chunks for this test
		config := &ChunkingConfig{
			MinChunkSize:    50,
			MaxChunkSize:    200, // Force smaller chunks
			TargetChunkSize: 100,
			OverlapSize:     20,
		}

		embeddings, err := processor.ProcessDocumentWithConfig(ctx, doc, config)

		require.NoError(t, err)
		assert.Greater(t, len(embeddings), 1) // Should create multiple embeddings

		// Verify chunk metadata
		for _, embedding := range embeddings {
			// Each embedding should have chunk metadata
			assert.NotNil(t, embedding)
			// Note: metadata is set in the request, not the response
			// In a real implementation, the embedding service would preserve metadata
		}

		mockEmbedding.AssertExpectations(t)
	})

	t.Run("empty document", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		processor := NewDocumentProcessor(mockEmbedding, nil)

		doc := &Document{
			ID:          "test-doc-3",
			Content:     "",
			ContentType: "text/plain",
		}

		_, err := processor.ProcessDocument(ctx, doc)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "content is empty")

		mockEmbedding.AssertNotCalled(t, "GenerateEmbedding")
	})

	t.Run("custom chunking config", func(t *testing.T) {
		mockEmbedding := new(MockEmbeddingService)
		processor := NewDocumentProcessor(mockEmbedding, nil)

		doc := &Document{
			ID:          "test-doc-4",
			Content:     "This is the first part of the document with important information. Then we continue with more details about the topic. Finally we conclude with a summary of all the key points discussed.",
			ContentType: "text/plain",
		}

		config := &ChunkingConfig{
			MinChunkSize:    10,
			MaxChunkSize:    30,
			TargetChunkSize: 20,
			OverlapSize:     5,
		}

		// Mock multiple embedding creations due to small chunk size - use mock.Anything for context
		mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, "text/plain", mock.Anything).Return(&EmbeddingVector{
			ContentID:   "embedding-small-chunk",
			ContentType: "text/plain",
			Vector:      []float32{0.1, 0.2, 0.3},
			Metadata:    map[string]interface{}{},
		}, nil)

		embeddings, err := processor.ProcessDocumentWithConfig(ctx, doc, config)

		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(embeddings), 2) // Should create multiple small chunks

		mockEmbedding.AssertExpectations(t)
	})
}

func TestDocumentProcessor_BatchProcessDocuments(t *testing.T) {
	ctx := context.Background()
	mockEmbedding := new(MockEmbeddingService)
	processor := NewDocumentProcessor(mockEmbedding, nil)

	docs := []*Document{
		{
			ID:          "batch-1",
			Content:     "First document content with enough text to create at least one chunk even with small sizes.",
			ContentType: "text/plain",
		},
		{
			ID:          "batch-2",
			Content:     "Second document content also with sufficient text to ensure proper chunking happens.",
			ContentType: "text/plain",
		},
		{
			ID:          "batch-3",
			Content:     "", // This will fail
			ContentType: "text/plain",
		},
	}

	// Mock successful embeddings - use mock.Anything for context
	mockEmbedding.On("GenerateEmbedding", mock.Anything, mock.Anything, "text/plain", mock.MatchedBy(func(contentID string) bool {
		return strings.HasPrefix(contentID, "batch-1") || strings.HasPrefix(contentID, "batch-2")
	})).Return(&EmbeddingVector{
		ContentID:   "batch-embedding",
		ContentType: "text/plain",
		Vector:      []float32{0.1, 0.2, 0.3},
		Metadata:    map[string]interface{}{},
	}, nil)

	// Use custom config with smaller chunk sizes for short test texts
	config := &ChunkingConfig{
		MinChunkSize:    5, // Very small for testing
		MaxChunkSize:    30,
		TargetChunkSize: 15,
		OverlapSize:     2,
	}

	results, err := processor.BatchProcessDocumentsWithConfig(ctx, docs, config)

	require.NoError(t, err)
	assert.Len(t, results, 2) // Only 2 successful
	assert.NotNil(t, results["batch-1"])
	assert.NotNil(t, results["batch-2"])
	assert.Nil(t, results["batch-3"]) // Failed document not in results

	mockEmbedding.AssertExpectations(t)
}
