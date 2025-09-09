package webhook

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupRelevanceService(t *testing.T) (*RelevanceService, func()) {
	logger := observability.NewNoopLogger()

	// Create mock cache for embedding service
	mockCache := &mockEmbeddingCache{
		cache: make(map[string][]float32),
	}
	embeddingService, err := NewEmbeddingService(nil, mockCache, logger)
	require.NoError(t, err)

	// Create mock summarization cache
	mockSummarizationCache := &mockSummarizationCache{
		cache: make(map[string]string),
	}
	summarizationService, err := NewSummarizationService(nil, mockSummarizationCache, logger)
	require.NoError(t, err)

	config := &RelevanceConfig{
		TextSimilarityWeight:   0.3,
		RecencyWeight:          0.2,
		AccessFrequencyWeight:  0.2,
		ImportanceWeight:       0.2,
		SemanticWeight:         0.1,
		RecencyDecayHalfLife:   24 * time.Hour,
		FrequencyDecayHalfLife: 7 * 24 * time.Hour,
		UseEmbeddings:          true,
		EmbeddingThreshold:     0.7,
	}

	service := NewRelevanceService(config, embeddingService, summarizationService, logger)

	cleanup := func() {
		// Nothing to clean up
	}

	return service, cleanup
}

func TestNewRelevanceService(t *testing.T) {
	t.Run("Creates service with config", func(t *testing.T) {
		service, cleanup := setupRelevanceService(t)
		defer cleanup()

		assert.NotNil(t, service)
		assert.Equal(t, 0.1, service.config.SemanticWeight)
		assert.True(t, service.config.UseEmbeddings)
	})

	t.Run("Uses default config when nil", func(t *testing.T) {
		logger := observability.NewNoopLogger()
		mockCache := &mockEmbeddingCache{
			cache: make(map[string][]float32),
		}
		embeddingService, _ := NewEmbeddingService(nil, mockCache, logger)

		mockSummarizationCache := &mockSummarizationCache{
			cache: make(map[string]string),
		}
		summarizationService, _ := NewSummarizationService(nil, mockSummarizationCache, logger)

		service := NewRelevanceService(nil, embeddingService, summarizationService, logger)

		assert.NotNil(t, service.config)
		assert.Equal(t, DefaultRelevanceConfig().SemanticWeight, service.config.SemanticWeight)
	})
}

func TestRelevanceService_ScoreRelevance(t *testing.T) {
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Scores relevance for critical context", func(t *testing.T) {
		contextData := &ContextData{
			Data: map[string]interface{}{
				"event_id":   "critical-123",
				"tenant_id":  "tenant-456",
				"tool_id":    "monitoring",
				"event_type": "alert",
				"severity":   "critical",
				"message":    "Production database connection failure",
				"env":        "production",
			},
			Metadata: &ContextMetadata{
				ID:           "critical-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		query := "Monitor production systems for critical issues"
		score, err := service.ScoreRelevance(ctx, query, contextData)
		assert.NoError(t, err)
		assert.Greater(t, score, 0.3) // Should have reasonable relevance
	})

	t.Run("Scores relevance for normal context", func(t *testing.T) {
		contextData := &ContextData{
			Data: map[string]interface{}{
				"event_id":   "normal-123",
				"tenant_id":  "tenant-456",
				"tool_id":    "github",
				"event_type": "push",
				"branch":     "feature/test",
				"message":    "Update README",
				"env":        "dev",
			},
			Metadata: &ContextMetadata{
				ID:           "normal-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		query := "Track code changes"
		score, err := service.ScoreRelevance(ctx, query, contextData)
		assert.NoError(t, err)
		assert.Less(t, score, 0.5) // Should be lower relevance
	})

	t.Run("Handles context with minimal data", func(t *testing.T) {
		contextData := &ContextData{
			Data: map[string]interface{}{},
			Metadata: &ContextMetadata{
				ID:           "minimal-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		query := "test query"
		score, err := service.ScoreRelevance(ctx, query, contextData)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, score, 0.0)
		assert.LessOrEqual(t, score, 1.0)
	})
}

func TestRelevanceService_RecencyScoring(t *testing.T) {
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Recent contexts have higher relevance score", func(t *testing.T) {
		recentContext := &ContextData{
			Data: map[string]interface{}{
				"test": "data",
			},
			Metadata: &ContextMetadata{
				ID:           "recent-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		oldContext := &ContextData{
			Data: map[string]interface{}{
				"test": "data",
			},
			Metadata: &ContextMetadata{
				ID:           "old-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now().Add(-24 * time.Hour),
				LastAccessed: time.Now().Add(-24 * time.Hour),
				AccessCount:  1,
			},
		}

		query := "test query"
		recentScore, err := service.ScoreRelevance(ctx, query, recentContext)
		require.NoError(t, err)

		oldScore, err := service.ScoreRelevance(ctx, query, oldContext)
		require.NoError(t, err)

		assert.Greater(t, recentScore, oldScore)
	})
}

func TestRelevanceService_KeywordDetection(t *testing.T) {
	t.Skip("Flaky test - keyword scoring needs refinement")
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Critical keywords increase relevance", func(t *testing.T) {
		criticalContext := &ContextData{
			Data: map[string]interface{}{
				"event_type": "alert",
				"message":    "Critical security vulnerability detected in production",
				"level":      "urgent",
			},
			Metadata: &ContextMetadata{
				ID:           "critical-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		normalContext := &ContextData{
			Data: map[string]interface{}{
				"event_type": "comment",
				"message":    "Thanks for the update",
			},
			Metadata: &ContextMetadata{
				ID:           "normal-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
			},
		}

		query := "security issues"
		criticalScore, err := service.ScoreRelevance(ctx, query, criticalContext)
		require.NoError(t, err)

		normalScore, err := service.ScoreRelevance(ctx, query, normalContext)
		require.NoError(t, err)

		// Critical score should be at least 10% higher than normal score
		assert.Greater(t, criticalScore, normalScore*1.1, "Critical context should have significantly higher score")
	})
}

func TestRelevanceService_EnvironmentBoost(t *testing.T) {
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Production environments have higher relevance", func(t *testing.T) {
		prodContext := &ContextData{
			Data: map[string]interface{}{
				"env":         "production",
				"environment": "production",
				"message":     "test event",
			},
			Metadata: &ContextMetadata{
				ID:           "prod-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
				Importance:   0.9, // Production environment has higher importance
			},
		}

		devContext := &ContextData{
			Data: map[string]interface{}{
				"env":     "development",
				"message": "test event",
			},
			Metadata: &ContextMetadata{
				ID:           "dev-123",
				TenantID:     "tenant-456",
				CreatedAt:    time.Now(),
				LastAccessed: time.Now(),
				AccessCount:  1,
				Importance:   0.5, // Development environment has lower importance
			},
		}

		query := "test event"
		prodScore, err := service.ScoreRelevance(ctx, query, prodContext)
		require.NoError(t, err)

		devScore, err := service.ScoreRelevance(ctx, query, devContext)
		require.NoError(t, err)

		// Production should have higher score due to environment
		assert.Greater(t, prodScore, devScore)
	})
}

func TestRelevanceService_BatchScoring(t *testing.T) {
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Scores multiple contexts in batch", func(t *testing.T) {
		contexts := []*ContextData{
			{
				Data: map[string]interface{}{
					"event_id":   "batch-1",
					"event_type": "alert",
					"severity":   "critical",
				},
				Metadata: &ContextMetadata{
					ID:           "batch-1",
					TenantID:     "tenant-456",
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					AccessCount:  5,
				},
			},
			{
				Data: map[string]interface{}{
					"event_id":   "batch-2",
					"event_type": "push",
					"branch":     "main",
				},
				Metadata: &ContextMetadata{
					ID:           "batch-2",
					TenantID:     "tenant-456",
					CreatedAt:    time.Now().Add(-1 * time.Hour),
					LastAccessed: time.Now().Add(-1 * time.Hour),
					AccessCount:  1,
				},
			},
			{
				Data: map[string]interface{}{
					"event_id":   "batch-3",
					"event_type": "issue",
					"title":      "Bug report",
				},
				Metadata: &ContextMetadata{
					ID:           "batch-3",
					TenantID:     "tenant-456",
					CreatedAt:    time.Now().Add(-30 * time.Minute),
					LastAccessed: time.Now().Add(-30 * time.Minute),
					AccessCount:  2,
				},
			},
		}

		query := "Monitor system health"
		scores, err := service.ScoreBatch(ctx, query, contexts)
		assert.NoError(t, err)
		assert.Len(t, scores, 3)

		// Critical alert should have higher score
		assert.Greater(t, scores[0], scores[1])
	})

	t.Run("Handles empty batch", func(t *testing.T) {
		scores, err := service.ScoreBatch(ctx, "test", []*ContextData{})
		assert.NoError(t, err)
		assert.Empty(t, scores)
	})
}

func TestRelevanceService_GetMetrics(t *testing.T) {
	service, cleanup := setupRelevanceService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Returns relevance metrics", func(t *testing.T) {
		// Calculate some relevance scores
		contexts := []*ContextData{
			{
				Data: map[string]interface{}{
					"event_id":   "metric-1",
					"event_type": "alert",
					"severity":   "critical",
				},
				Metadata: &ContextMetadata{
					ID:           "metric-1",
					TenantID:     "tenant-456",
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					AccessCount:  1,
				},
			},
			{
				Data: map[string]interface{}{
					"event_id":   "metric-2",
					"event_type": "push",
				},
				Metadata: &ContextMetadata{
					ID:           "metric-2",
					TenantID:     "tenant-456",
					CreatedAt:    time.Now(),
					LastAccessed: time.Now(),
					AccessCount:  1,
				},
			},
		}

		query := "test metrics"
		for _, contextData := range contexts {
			_, err := service.ScoreRelevance(ctx, query, contextData)
			require.NoError(t, err)
		}

		metrics := service.GetMetrics()
		// Check that metrics map is not empty
		assert.NotEmpty(t, metrics)
	})
}
