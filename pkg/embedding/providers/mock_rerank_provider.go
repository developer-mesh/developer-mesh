package providers

import (
	"context"
	"fmt"
	"math"
	"strings"

	"github.com/stretchr/testify/mock"
)

// MockRerankProvider is a mock implementation of RerankProvider for testing
type MockRerankProvider struct {
	mock.Mock
}

// Rerank mock implementation
func (m *MockRerankProvider) Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*RerankResponse), args.Error(1)
}

// GetRerankModels mock implementation
func (m *MockRerankProvider) GetRerankModels() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

// SupportsReranking mock implementation
func (m *MockRerankProvider) SupportsReranking() bool {
	args := m.Called()
	return args.Bool(0)
}

// SimpleRerankProvider is a simple implementation for testing
type SimpleRerankProvider struct {
	model string
}

// NewSimpleRerankProvider creates a simple rerank provider for testing
func NewSimpleRerankProvider(model string) *SimpleRerankProvider {
	if model == "" {
		model = "simple-reranker-v1"
	}
	return &SimpleRerankProvider{model: model}
}

// Rerank implements a simple reranking based on keyword matching
func (s *SimpleRerankProvider) Rerank(ctx context.Context, req RerankRequest) (*RerankResponse, error) {
	if req.Query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if len(req.Documents) == 0 {
		return nil, fmt.Errorf("documents cannot be empty")
	}

	// Simple scoring based on keyword overlap
	queryWords := strings.Fields(strings.ToLower(req.Query))
	results := make([]RerankResult, len(req.Documents))

	for i, doc := range req.Documents {
		docWords := strings.Fields(strings.ToLower(doc))
		score := s.calculateScore(queryWords, docWords)
		results[i] = RerankResult{
			Index:    i,
			Score:    score,
			Document: doc,
		}
	}

	return &RerankResponse{
		Results: results,
		Model:   s.model,
	}, nil
}

// calculateScore calculates a simple score based on keyword overlap
func (s *SimpleRerankProvider) calculateScore(queryWords, docWords []string) float64 {
	if len(queryWords) == 0 || len(docWords) == 0 {
		return 0
	}

	// Create word frequency map for document
	docFreq := make(map[string]int)
	for _, word := range docWords {
		docFreq[word]++
	}

	// Calculate overlap
	matches := 0
	for _, queryWord := range queryWords {
		if count, exists := docFreq[queryWord]; exists {
			matches += count
		}
	}

	// Normalize by query length and doc length
	score := float64(matches) / math.Sqrt(float64(len(queryWords)*len(docWords)))

	// Ensure score is between 0 and 1
	return math.Min(1.0, score)
}

// GetRerankModels returns available models
func (s *SimpleRerankProvider) GetRerankModels() []string {
	return []string{s.model}
}

// SupportsReranking returns true
func (s *SimpleRerankProvider) SupportsReranking() bool {
	return true
}
