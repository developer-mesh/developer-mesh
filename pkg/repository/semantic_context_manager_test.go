package repository_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/repository"
)

func TestSemanticContextManager_CompactionStrategies(t *testing.T) {
	assert := assert.New(t)

	// Verify all strategies are defined
	strategies := []repository.CompactionStrategy{
		repository.CompactionSummarize,
		repository.CompactionPrune,
		repository.CompactionSemantic,
		repository.CompactionSliding,
		repository.CompactionToolClear,
	}

	for _, strategy := range strategies {
		assert.NotEmpty(string(strategy), "Strategy should not be empty")
	}

	// Verify unique values
	uniqueStrategies := make(map[string]bool)
	for _, strategy := range strategies {
		strategyStr := string(strategy)
		assert.False(uniqueStrategies[strategyStr], "Strategy %s should be unique", strategyStr)
		uniqueStrategies[strategyStr] = true
	}

	assert.Equal(5, len(uniqueStrategies), "Should have exactly 5 strategies")
}

func TestRetrievalOptions_Structure(t *testing.T) {
	assert := assert.New(t)

	// Test default initialization
	opts := &repository.RetrievalOptions{}
	assert.NotNil(opts)
	assert.False(opts.IncludeEmbeddings)
	assert.Equal(0, opts.MaxTokens)
	assert.Empty(opts.RelevanceQuery)
	assert.Nil(opts.TimeRange)
	assert.Equal(float64(0), opts.MinSimilarity)

	// Test with values
	opts = &repository.RetrievalOptions{
		IncludeEmbeddings: true,
		MaxTokens:         4000,
		RelevanceQuery:    "test query",
		MinSimilarity:     0.8,
	}
	assert.True(opts.IncludeEmbeddings)
	assert.Equal(4000, opts.MaxTokens)
	assert.Equal("test query", opts.RelevanceQuery)
	assert.Equal(0.8, opts.MinSimilarity)
}

func TestContextUpdate_Structure(t *testing.T) {
	assert := assert.New(t)

	update := &repository.ContextUpdate{
		Role:    "user",
		Content: "test content",
		Metadata: map[string]interface{}{
			"source": "test",
		},
	}

	assert.Equal("user", update.Role)
	assert.Equal("test content", update.Content)
	assert.NotNil(update.Metadata)
	assert.Equal("test", update.Metadata["source"])
}

func TestCreateContextRequest_Structure(t *testing.T) {
	assert := assert.New(t)

	req := &repository.CreateContextRequest{
		Name:      "test-context",
		AgentID:   "agent-123",
		SessionID: "session-456",
		Properties: map[string]interface{}{
			"type": "semantic",
		},
		MaxTokens: 8000,
	}

	assert.Equal("test-context", req.Name)
	assert.Equal("agent-123", req.AgentID)
	assert.Equal("session-456", req.SessionID)
	assert.NotNil(req.Properties)
	assert.Equal("semantic", req.Properties["type"])
	assert.Equal(8000, req.MaxTokens)
}

func TestCompactionStrategy_StringValues(t *testing.T) {
	assert := assert.New(t)

	// Verify string values match expected format
	assert.Equal("summarize", string(repository.CompactionSummarize))
	assert.Equal("prune", string(repository.CompactionPrune))
	assert.Equal("semantic", string(repository.CompactionSemantic))
	assert.Equal("sliding", string(repository.CompactionSliding))
	assert.Equal("tool_clear", string(repository.CompactionToolClear))
}
