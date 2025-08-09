package embedding

import (
	"context"

	"github.com/google/uuid"
)

// ModelSelector defines the interface for selecting embedding models
type ModelSelector interface {
	// SelectModel selects the best available model for a request
	SelectModel(ctx context.Context, req ModelSelectionRequest) (*ModelSelectionResult, error)

	// TrackUsage records embedding usage asynchronously
	TrackUsage(ctx context.Context, tenantID, modelID uuid.UUID, agentID *uuid.UUID, tokens int, latencyMs int, taskType *string)
}

// ModelSelectionRequest represents a request for model selection
type ModelSelectionRequest struct {
	TenantID       uuid.UUID  `json:"tenant_id"`
	AgentID        *uuid.UUID `json:"agent_id,omitempty"`
	TaskType       *string    `json:"task_type,omitempty"`
	RequestedModel *string    `json:"requested_model,omitempty"`
	TokenEstimate  int        `json:"token_estimate,omitempty"`
}

// ModelSelectionResult represents the selected model and its details
type ModelSelectionResult struct {
	ModelID              uuid.UUID              `json:"model_id"`
	ModelIdentifier      string                 `json:"model_identifier"`
	Provider             string                 `json:"provider"`
	Dimensions           int                    `json:"dimensions"`
	CostPerMillionTokens float64                `json:"cost_per_million_tokens"`
	EstimatedCost        float64                `json:"estimated_cost,omitempty"`
	IsWithinQuota        bool                   `json:"is_within_quota"`
	QuotaRemaining       *int64                 `json:"quota_remaining,omitempty"`
	Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

// DefaultModelSelector provides a default implementation that uses hardcoded values
type DefaultModelSelector struct{}

// NewDefaultModelSelector creates a new default model selector
func NewDefaultModelSelector() ModelSelector {
	return &DefaultModelSelector{}
}

// SelectModel returns default model selection
func (d *DefaultModelSelector) SelectModel(ctx context.Context, req ModelSelectionRequest) (*ModelSelectionResult, error) {
	// Default to Titan v2 embedding model
	return &ModelSelectionResult{
		ModelID:              uuid.New(), // Generate a dummy ID
		ModelIdentifier:      "amazon.titan-embed-text-v2:0",
		Provider:             "bedrock",
		Dimensions:           1024,
		CostPerMillionTokens: 0.02,
		IsWithinQuota:        true,
		Metadata: map[string]interface{}{
			"is_default": true,
		},
	}, nil
}

// TrackUsage is a no-op for the default selector
func (d *DefaultModelSelector) TrackUsage(ctx context.Context, tenantID, modelID uuid.UUID, agentID *uuid.UUID, tokens int, latencyMs int, taskType *string) {
	// No-op for default implementation
}
