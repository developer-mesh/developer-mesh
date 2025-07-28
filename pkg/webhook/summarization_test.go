package webhook

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupSummarizationService(t *testing.T) (*SummarizationService, func()) {
	logger := observability.NewNoopLogger()

	// Mock summarization cache
	mockCache := &mockSummarizationCache{
		cache: make(map[string]string),
	}

	config := &SummarizationConfig{
		Provider:       "mock",
		Model:          "mock-model",
		MaxInputLength: 512,
		DefaultOptions: SummarizationOptions{
			MaxLength: 100,
			MinLength: 10,
			Style:     "paragraph",
		},
		CacheDuration: 1 * time.Hour,
	}

	service, err := NewSummarizationService(config, mockCache, logger)
	require.NoError(t, err)

	cleanup := func() {
		// Nothing to clean up for mock
	}

	return service, cleanup
}

func TestNewSummarizationService(t *testing.T) {
	t.Run("Creates service with config", func(t *testing.T) {
		service, cleanup := setupSummarizationService(t)
		defer cleanup()

		assert.NotNil(t, service)
		assert.Equal(t, "mock", service.config.Provider)
		assert.Equal(t, 100, service.config.DefaultOptions.MaxLength)
		assert.Equal(t, "mock", service.config.Provider)
	})

	t.Run("Uses default config when nil", func(t *testing.T) {
		logger := observability.NewNoopLogger()
		mockCache := &mockSummarizationCache{
			cache: make(map[string]string),
		}

		service, err := NewSummarizationService(nil, mockCache, logger)
		require.NoError(t, err)

		assert.NotNil(t, service.config)
		assert.Equal(t, DefaultSummarizationConfig().DefaultOptions.MaxLength, service.config.DefaultOptions.MaxLength)
	})
}

func TestSummarizationService_SummarizeContext(t *testing.T) {
	service, cleanup := setupSummarizationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Summarizes context with short text", func(t *testing.T) {
		contextData := &ContextData{
			Data: map[string]interface{}{
				"message": "This is a short webhook event description.",
				"type":    "test",
			},
			Metadata: &ContextMetadata{
				ID:        "test-123",
				TenantID:  "tenant-456",
				CreatedAt: time.Now(),
			},
		}

		summary, err := service.SummarizeContext(ctx, contextData)
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.NotEmpty(t, summary.Summary)
	})

	t.Run("Summarizes context with long text", func(t *testing.T) {
		longText := strings.Repeat("This is a very long text that needs summarization. ", 20)
		contextData := &ContextData{
			Data: map[string]interface{}{
				"message": longText,
				"type":    "test",
			},
			Metadata: &ContextMetadata{
				ID:        "test-long-123",
				TenantID:  "tenant-456",
				CreatedAt: time.Now(),
			},
		}

		summary, err := service.SummarizeContext(ctx, contextData)
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.NotEmpty(t, summary.Summary)
	})

	t.Run("Uses cache for repeated context", func(t *testing.T) {
		contextData := &ContextData{
			Data: map[string]interface{}{
				"message": "Cached text for summarization",
				"type":    "test",
			},
			Metadata: &ContextMetadata{
				ID:        "test-cache-123",
				TenantID:  "tenant-456",
				CreatedAt: time.Now(),
			},
		}

		// First call
		summary1, err := service.SummarizeContext(ctx, contextData)
		require.NoError(t, err)

		// Second call should use cache
		summary2, err := service.SummarizeContext(ctx, contextData)
		require.NoError(t, err)

		assert.Equal(t, summary1.Summary, summary2.Summary)
	})
}

func TestSummarizationService_SummarizeEventStream(t *testing.T) {
	service, cleanup := setupSummarizationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Summarizes event stream", func(t *testing.T) {
		events := []*WebhookEvent{
			{
				EventId:   "test-123",
				TenantId:  "tenant-456",
				ToolId:    "github",
				ToolType:  "vcs",
				EventType: "pull_request",
				Timestamp: time.Now(),
				Payload: map[string]interface{}{
					"action":      "opened",
					"repository":  "test-repo",
					"title":       "Add new feature for user authentication",
					"description": "This PR implements OAuth2 authentication with support for multiple providers",
					"author":      "john-doe",
				},
			},
			{
				EventId:   "test-124",
				TenantId:  "tenant-456",
				ToolId:    "github",
				ToolType:  "vcs",
				EventType: "push",
				Timestamp: time.Now().Add(5 * time.Minute),
				Payload: map[string]interface{}{
					"branch": "main",
					"commit": "abc123",
				},
			},
		}

		summary, err := service.SummarizeEventStream(ctx, events)
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.NotEmpty(t, summary.OverallSummary)
		assert.Len(t, summary.EventTypes, 2)
		assert.Equal(t, 2, summary.TotalEvents)
	})

	t.Run("Handles empty event stream", func(t *testing.T) {
		summary, err := service.SummarizeEventStream(ctx, []*WebhookEvent{})
		assert.NoError(t, err)
		assert.NotNil(t, summary)
		assert.Equal(t, 0, summary.TotalEvents)
	})
}

func TestSummarizationService_GetMetrics(t *testing.T) {
	service, cleanup := setupSummarizationService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("Returns summarization metrics", func(t *testing.T) {
		// Generate some summaries
		for i := 0; i < 5; i++ {
			contextData := &ContextData{
				Data: map[string]interface{}{
					"message": fmt.Sprintf("Event %d description with some details", i),
					"type":    "test",
				},
				Metadata: &ContextMetadata{
					ID:        fmt.Sprintf("metric-%d", i),
					TenantID:  "tenant-456",
					CreatedAt: time.Now(),
				},
			}
			_, err := service.SummarizeContext(ctx, contextData)
			require.NoError(t, err)
		}

		// Generate same context again for cache hit
		contextData := &ContextData{
			Data: map[string]interface{}{
				"message": "Event 0 description with some details",
				"type":    "test",
			},
			Metadata: &ContextMetadata{
				ID:        "metric-0",
				TenantID:  "tenant-456",
				CreatedAt: time.Now(),
			},
		}
		_, err := service.SummarizeContext(ctx, contextData)
		require.NoError(t, err)

		metrics := service.GetMetrics()
		// Just check that metrics exist
		assert.NotEmpty(t, metrics)
	})
}
