package intelligence

import (
	"context"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/google/uuid"
)

// EmbeddingServiceAdapter adapts the embedding.ServiceV2 to our EmbeddingService interface
type EmbeddingServiceAdapter struct {
	service  *embedding.ServiceV2
	tenantID uuid.UUID
	agentID  *uuid.UUID
}

// NewEmbeddingServiceAdapter creates a new adapter
func NewEmbeddingServiceAdapter(service *embedding.ServiceV2, tenantID uuid.UUID, agentID *uuid.UUID) EmbeddingService {
	return &EmbeddingServiceAdapter{
		service:  service,
		tenantID: tenantID,
		agentID:  agentID,
	}
}

// GenerateEmbedding implements the EmbeddingService interface
func (a *EmbeddingServiceAdapter) GenerateEmbedding(ctx context.Context, content string, metadata map[string]interface{}) (*uuid.UUID, error) {
	agentIDStr := ""
	if a.agentID != nil {
		agentIDStr = a.agentID.String()
	}

	req := embedding.GenerateEmbeddingRequest{
		Text:      content,
		TenantID:  a.tenantID,
		AgentID:   agentIDStr,
		Metadata:  metadata,
		RequestID: uuid.New().String(),
	}

	resp, err := a.service.GenerateEmbedding(ctx, req)
	if err != nil {
		return nil, err
	}

	return &resp.EmbeddingID, nil
}
