package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/queue"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/google/uuid"
)

// ContextEmbeddingProcessor processes context items and generates embeddings
type ContextEmbeddingProcessor struct {
	embeddingService *embedding.ServiceV2
	contextRepo      repository.ContextRepository
	logger           observability.Logger
	metrics          observability.MetricsClient
}

// NewContextEmbeddingProcessor creates a new context embedding processor
func NewContextEmbeddingProcessor(
	embeddingService *embedding.ServiceV2,
	contextRepo repository.ContextRepository,
	logger observability.Logger,
	metrics observability.MetricsClient,
) *ContextEmbeddingProcessor {
	return &ContextEmbeddingProcessor{
		embeddingService: embeddingService,
		contextRepo:      contextRepo,
		logger:           logger,
		metrics:          metrics,
	}
}

// ProcessEvent processes a context embedding event
func (p *ContextEmbeddingProcessor) ProcessEvent(ctx context.Context, event queue.Event) error {
	start := time.Now()

	// Parse event payload
	var payload struct {
		ContextID string               `json:"context_id"`
		TenantID  string               `json:"tenant_id"`
		AgentID   string               `json:"agent_id"`
		Items     []models.ContextItem `json:"items"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to parse context event payload: %w", err)
	}

	p.logger.Info("Processing context embeddings", map[string]interface{}{
		"context_id": payload.ContextID,
		"item_count": len(payload.Items),
	})

	// Parse tenant ID
	tenantID, err := uuid.Parse(payload.TenantID)
	if err != nil {
		return fmt.Errorf("invalid tenant ID: %w", err)
	}

	// Parse context ID
	contextUUID, err := uuid.Parse(payload.ContextID)
	if err != nil {
		return fmt.Errorf("invalid context ID: %w", err)
	}

	// Process each item
	successCount := 0
	for _, item := range payload.Items {
		// Only generate embeddings for user and assistant messages
		if item.Role != "user" && item.Role != "assistant" {
			continue
		}

		// Skip empty content
		if item.Content == "" {
			continue
		}

		// Generate embedding
		req := embedding.GenerateEmbeddingRequest{
			AgentID:   payload.AgentID,
			Text:      item.Content,
			TenantID:  tenantID,
			ContextID: &contextUUID,
			Metadata: map[string]interface{}{
				"item_id":   item.ID,
				"role":      item.Role,
				"timestamp": item.Timestamp,
			},
		}

		resp, err := p.embeddingService.GenerateEmbedding(ctx, req)
		if err != nil {
			p.logger.Error("Failed to generate embedding", map[string]interface{}{
				"error":      err.Error(),
				"context_id": payload.ContextID,
				"item_id":    item.ID,
			})
			// Continue processing other items
			continue
		}

		// Link embedding to context (using the fixed table name)
		err = p.contextRepo.LinkEmbeddingToContext(
			ctx,
			payload.ContextID,
			resp.EmbeddingID.String(),
			successCount, // sequence number
			1.0,          // importance score (could be calculated based on content)
		)
		if err != nil {
			p.logger.Error("Failed to link embedding to context", map[string]interface{}{
				"error":        err.Error(),
				"context_id":   payload.ContextID,
				"embedding_id": resp.EmbeddingID,
			})
			continue
		}

		successCount++

		p.logger.Debug("Generated embedding for context item", map[string]interface{}{
			"context_id":   payload.ContextID,
			"item_id":      item.ID,
			"embedding_id": resp.EmbeddingID,
			"model":        resp.ModelUsed,
			"tokens":       resp.TokensUsed,
		})
	}

	// Record metrics
	if p.metrics != nil {
		p.metrics.IncrementCounterWithLabels("context_embeddings_generated_total", float64(successCount), map[string]string{
			"tenant_id": payload.TenantID,
		})
		p.metrics.RecordHistogram("context_embedding_generation_duration_seconds", time.Since(start).Seconds(), map[string]string{
			"tenant_id": payload.TenantID,
		})
	}

	p.logger.Info("Completed context embedding generation", map[string]interface{}{
		"context_id":      payload.ContextID,
		"items_processed": successCount,
		"duration_ms":     time.Since(start).Milliseconds(),
	})

	return nil
}

// ValidateEvent validates a context embedding event
func (p *ContextEmbeddingProcessor) ValidateEvent(event queue.Event) error {
	// Parse event payload to validate structure
	var payload struct {
		ContextID string               `json:"context_id"`
		TenantID  string               `json:"tenant_id"`
		AgentID   string               `json:"agent_id"`
		Items     []models.ContextItem `json:"items"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("invalid event payload: %w", err)
	}

	if payload.ContextID == "" {
		return fmt.Errorf("missing context_id in event payload")
	}

	if payload.TenantID == "" {
		return fmt.Errorf("missing tenant_id in event payload")
	}

	if payload.AgentID == "" {
		return fmt.Errorf("missing agent_id in event payload")
	}

	return nil
}

// GetProcessingMode returns the processing mode for context embedding events
func (p *ContextEmbeddingProcessor) GetProcessingMode() string {
	return "async"
}
