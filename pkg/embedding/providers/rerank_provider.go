package providers

import (
	"context"
)

// RerankProvider interface for providers that support reranking (e.g., Cohere)
type RerankProvider interface {
	// Rerank reorders documents based on relevance to a query
	Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error)

	// GetRerankModels returns available reranking models
	GetRerankModels() []string

	// SupportsReranking indicates if this provider supports reranking
	SupportsReranking() bool
}

// RerankRequest represents a request to rerank documents
type RerankRequest struct {
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	Model     string   `json:"model"`
	TopK      int      `json:"top_k,omitempty"`
}

// RerankResponse represents the response from a rerank request
type RerankResponse struct {
	Results []RerankResult `json:"results"`
	Model   string         `json:"model"`
}

// RerankResult represents a single reranked document
type RerankResult struct {
	Index    int     `json:"index"`
	Score    float64 `json:"score"`
	Document string  `json:"document"`
}
