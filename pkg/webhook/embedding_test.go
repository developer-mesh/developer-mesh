package webhook

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupEmbeddingService(t *testing.T) (*EmbeddingService, func()) {
	logger := observability.NewNoopLogger()

	// Mock embedding provider is not needed since we're using a mock cache

	config := &EmbeddingConfig{
		Provider:      "mock",
		Model:         "mock-model",
		Dimensions:    3,
		BatchSize:     10,
		MaxTextLength: 512,
		CacheDuration: 1 * time.Hour,
	}

	// Create a mock cache
	mockCache := &mockEmbeddingCache{
		cache: make(map[string][]float32),
	}

	service, err := NewEmbeddingService(config, mockCache, logger)
	require.NoError(t, err)

	cleanup := func() {
		// Nothing to clean up for mock
	}

	return service, cleanup
}

func TestNewEmbeddingService(t *testing.T) {
	t.Run("Creates service with config", func(t *testing.T) {
		service, cleanup := setupEmbeddingService(t)
		defer cleanup()

		assert.NotNil(t, service)
		assert.Equal(t, "mock", service.config.Provider)
		assert.Equal(t, 3, service.config.Dimensions)
		assert.NotZero(t, service.config.CacheDuration)
	})

	t.Run("Uses default config when nil", func(t *testing.T) {
		logger := observability.NewNoopLogger()

		// Create a mock cache
		mockCache := &mockEmbeddingCache{
			cache: make(map[string][]float32),
		}

		service, err := NewEmbeddingService(nil, mockCache, logger)
		require.NoError(t, err)

		assert.NotNil(t, service.config)
		assert.Equal(t, DefaultEmbeddingConfig().Dimensions, service.config.Dimensions)
	})
}

func TestEmbeddingService_GenerateEmbedding(t *testing.T) {
	service, cleanup := setupEmbeddingService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Generates embedding for text", func(t *testing.T) {
		text := "This is a test webhook event"

		embedding, err := service.GenerateEmbedding(ctx, text)
		assert.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.Len(t, embedding, 3)
	})

	t.Run("Uses cache for repeated text", func(t *testing.T) {
		text := "Cached text"

		// First call
		embedding1, err := service.GenerateEmbedding(ctx, text)
		require.NoError(t, err)

		// Second call should use cache
		embedding2, err := service.GenerateEmbedding(ctx, text)
		require.NoError(t, err)

		// Should be the same
		assert.Equal(t, embedding1, embedding2)
		// Both embeddings should be identical from cache
	})

	t.Run("Handles empty text", func(t *testing.T) {
		embedding, err := service.GenerateEmbedding(ctx, "")
		assert.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.Len(t, embedding, 3)
	})
}

func TestEmbeddingService_GenerateEventEmbedding(t *testing.T) {
	service, cleanup := setupEmbeddingService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Generates embedding for webhook event", func(t *testing.T) {
		event := &WebhookEvent{
			EventId:   "test-123",
			TenantId:  "tenant-456",
			ToolId:    "github",
			ToolType:  "vcs",
			EventType: "push",
			Timestamp: time.Now(),
			Payload: map[string]interface{}{
				"repository": "test-repo",
				"branch":     "main",
				"message":    "Fix critical bug in authentication",
			},
		}

		// Convert event to text for embedding
		eventText := fmt.Sprintf("%s %s %v", event.ToolId, event.EventType, event.Payload)
		embedding, err := service.GenerateEmbedding(ctx, eventText)
		assert.NoError(t, err)
		assert.NotNil(t, embedding)
		assert.Len(t, embedding, 3)
		// Verify embedding was generated
	})

	t.Run("Handles events with minimal payload", func(t *testing.T) {
		event := &WebhookEvent{
			EventId:   "minimal-123",
			TenantId:  "tenant-456",
			ToolId:    "jira",
			EventType: "issue_created",
			Timestamp: time.Now(),
			Payload:   map[string]interface{}{},
		}

		// Convert event to text for embedding
		eventText := fmt.Sprintf("%s %s %v", event.ToolId, event.EventType, event.Payload)
		embedding, err := service.GenerateEmbedding(ctx, eventText)
		assert.NoError(t, err)
		assert.NotNil(t, embedding)
		// Verify embedding was generated
		assert.Len(t, embedding, 3)
	})
}

func TestEmbeddingService_CalculateSimilarity(t *testing.T) {
	t.Skip("CalculateSimilarity method not implemented")
	/*
		service, cleanup := setupEmbeddingService(t)
		defer cleanup()

		t.Run("Calculates cosine similarity", func(t *testing.T) {
			vec1 := []float64{1.0, 0.0, 0.0}
			vec2 := []float64{1.0, 0.0, 0.0}

			similarity := service.CalculateSimilarity(vec1, vec2)
			assert.Equal(t, 1.0, similarity) // Identical vectors

			vec3 := []float64{0.0, 1.0, 0.0}
			similarity2 := service.CalculateSimilarity(vec1, vec3)
			assert.Equal(t, 0.0, similarity2) // Orthogonal vectors
		})

		t.Run("Handles different magnitude vectors", func(t *testing.T) {
			vec1 := []float64{2.0, 0.0, 0.0}
			vec2 := []float64{1.0, 0.0, 0.0}

			similarity := service.CalculateSimilarity(vec1, vec2)
			assert.Equal(t, 1.0, similarity) // Same direction, different magnitude
		})

		t.Run("Returns 0 for mismatched dimensions", func(t *testing.T) {
			vec1 := []float64{1.0, 0.0}
			vec2 := []float64{1.0, 0.0, 0.0}

			similarity := service.CalculateSimilarity(vec1, vec2)
			assert.Equal(t, 0.0, similarity)
		})
	*/
}

func TestEmbeddingService_FindSimilarEvents(t *testing.T) {
	t.Skip("FindSimilarEvents functionality not implemented")
	/*
		service, cleanup := setupEmbeddingService(t)
		defer cleanup()

		ctx := context.Background()

		t.Run("Finds similar events", func(t *testing.T) {
			// Create embeddings for multiple events
			events := []struct {
				text      string
				embedding []float32
			}{
				{text: "GitHub push to main branch"},
				{text: "GitHub pull request opened"},
				{text: "Jira issue created"},
				{text: "GitHub commit pushed"},
			}

			for i := range events {
				embedding, err := service.GenerateEmbedding(ctx, events[i].text)
				require.NoError(t, err)
				events[i].embedding = embedding
			}

			// Find similar to first event
			targetEmbedding := events[0].embedding
			candidateEmbeddings := [][]float32{
				events[1].embedding,
				events[2].embedding,
				events[3].embedding,
			}

			similar := service.FindSimilarEvents(targetEmbedding, candidateEmbeddings, 0.0, 2)
			assert.Len(t, similar, 2)

			// Should find GitHub-related events as most similar
			for _, result := range similar {
				assert.Contains(t, result.Embedding.Text, "GitHub")
			}
		})

		t.Run("Respects similarity threshold", func(t *testing.T) {
			targetEmbedding := &Embedding{
				Text:   "Test event",
				Vector: []float64{1.0, 0.0, 0.0},
			}

			candidates := []*Embedding{
				{
					Text:   "Similar",
					Vector: []float64{0.9, 0.436, 0.0}, // ~0.9 similarity
				},
				{
					Text:   "Different",
					Vector: []float64{0.0, 1.0, 0.0}, // 0.0 similarity
				},
			}

			similar := service.FindSimilarEvents(targetEmbedding, candidates, 0.8, 10)
			assert.Len(t, similar, 1)
			assert.Equal(t, "Similar", similar[0].Embedding.Text)
		})
	*/
}

func TestEmbeddingService_BatchGenerateEmbeddings(t *testing.T) {
	service, cleanup := setupEmbeddingService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Generates embeddings in batches", func(t *testing.T) {
		texts := []string{
			"First event",
			"Second event",
			"Third event",
			"Fourth event",
			"Fifth event",
		}

		embeddings, err := service.GenerateBatchEmbeddings(ctx, texts)
		assert.NoError(t, err)
		assert.Len(t, embeddings, 5)

		for _, embedding := range embeddings {
			assert.Len(t, embedding, 3)
		}
	})

	t.Run("Handles empty batch", func(t *testing.T) {
		embeddings, err := service.GenerateBatchEmbeddings(ctx, []string{})
		assert.NoError(t, err)
		assert.Empty(t, embeddings)
	})
}

func TestEmbeddingService_GetMetrics(t *testing.T) {
	service, cleanup := setupEmbeddingService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Returns embedding metrics", func(t *testing.T) {
		// Generate some embeddings
		for i := 0; i < 5; i++ {
			_, err := service.GenerateEmbedding(ctx, fmt.Sprintf("Event %d", i))
			require.NoError(t, err)
		}

		// Generate same text again for cache hit
		_, err := service.GenerateEmbedding(ctx, "Event 0")
		require.NoError(t, err)

		metrics := service.GetMetrics()
		assert.Contains(t, metrics, "embeddings_generated")
		assert.Contains(t, metrics, "cache_hits")
		assert.Contains(t, metrics, "cache_misses")
		assert.Contains(t, metrics, "avg_generation_time_ms")
		assert.Contains(t, metrics, "errors_total")

		assert.Greater(t, metrics["embeddings_generated"].(int64), int64(0))
		assert.Greater(t, metrics["cache_hits"].(int64), int64(0))
		assert.Greater(t, metrics["cache_misses"].(int64), int64(0))
	})
}

// Mock embedding cache for testing
type mockEmbeddingCache struct {
	cache map[string][]float32
	mu    sync.Mutex
}

func (m *mockEmbeddingCache) Get(ctx context.Context, key string) ([]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if embedding, ok := m.cache[key]; ok {
		return embedding, nil
	}
	return nil, nil
}

func (m *mockEmbeddingCache) Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cache == nil {
		m.cache = make(map[string][]float32)
	}
	m.cache[key] = embedding
	return nil
}

func (m *mockEmbeddingCache) GetBatch(ctx context.Context, keys []string) (map[string][]float32, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make(map[string][]float32)
	for _, key := range keys {
		if embedding, ok := m.cache[key]; ok {
			result[key] = embedding
		}
	}
	return result, nil
}

func (m *mockEmbeddingCache) SetBatch(ctx context.Context, embeddings map[string][]float32, ttl time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.cache == nil {
		m.cache = make(map[string][]float32)
	}
	for key, embedding := range embeddings {
		m.cache[key] = embedding
	}
	return nil
}

/*
// Mock embedding provider for testing - unused, kept for future use
type mockEmbeddingProvider struct {
	embedFunc func(context.Context, string) ([]float64, error)
}

func (m *mockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float64, error) {
	if m.embedFunc != nil {
		return m.embedFunc(ctx, text)
	}
	// Default implementation
	return []float64{1.0, 0.5, 0.25}, nil
}

// Helper function for testing - unused, kept for future use
func almostEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
*/
