package expansion

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewDecompositionExpander(t *testing.T) {
	mockLLM := new(MockLLMClient)
	logger := observability.NewLogger("test")

	expander := NewDecompositionExpander(mockLLM, logger)
	assert.NotNil(t, expander)
	assert.NotNil(t, expander.llmClient)
	assert.NotNil(t, expander.logger)
}

func TestDecompositionExpander_Expand(t *testing.T) {
	ctx := context.Background()

	t.Run("successful decomposition", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewDecompositionExpander(mockLLM, nil)

		query := "how to implement authentication and authorization in microservices"

		subQueries := []SubQuery{
			{Query: "microservices authentication", Focus: "authentication aspect"},
			{Query: "microservices authorization", Focus: "authorization aspect"},
			{Query: "security patterns for microservices", Focus: "overall security patterns"},
		}

		jsonResponse, _ := json.Marshal(subQueries)

		mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(req CompletionRequest) bool {
			return req.Format == "json" && req.Temperature == 0.3
		})).Return(&CompletionResponse{
			Text:   string(jsonResponse),
			Tokens: 50,
		}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, query, result.Original)
		assert.Len(t, result.Expansions, 3)

		// Check first expansion
		exp := result.Expansions[0]
		assert.Equal(t, "microservices authentication", exp.Text)
		assert.Equal(t, ExpansionTypeDecompose, exp.Type)
		assert.Equal(t, float32(0.5), exp.Weight) // 1/(0+2)
		assert.Equal(t, "authentication aspect", exp.Metadata["focus"])

		// Check weights are decreasing
		assert.Greater(t, result.Expansions[0].Weight, result.Expansions[1].Weight)
		assert.Greater(t, result.Expansions[1].Weight, result.Expansions[2].Weight)

		mockLLM.AssertExpectations(t)
	})

	t.Run("simple query skipped", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewDecompositionExpander(mockLLM, nil)

		query := "Python list"

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, query, result.Original)
		assert.Empty(t, result.Expansions)

		// LLM should not be called for simple queries
		mockLLM.AssertNotCalled(t, "Complete")
	})

	t.Run("invalid JSON fallback", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		logger := observability.NewLogger("test")
		expander := NewDecompositionExpander(mockLLM, logger)

		query := "complex query with multiple topics and subtopics"

		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(&CompletionResponse{
				Text:   "invalid json response",
				Tokens: 10,
			}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		// Should fall back to simple decomposition
		assert.Greater(t, len(result.Expansions), 0)

		mockLLM.AssertExpectations(t)
	})

	t.Run("LLM error fallback", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		logger := observability.NewLogger("test")
		expander := NewDecompositionExpander(mockLLM, logger)

		query := "error handling and logging in distributed systems"

		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("LLM service error")).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err) // Should not propagate error
		assert.NotNil(t, result)
		// Should use simple decomposition
		assert.Len(t, result.Expansions, 2) // Split on "and"

		mockLLM.AssertExpectations(t)
	})

	t.Run("filters duplicate queries", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewDecompositionExpander(mockLLM, nil)

		query := "test query with multiple concepts and ideas"

		subQueries := []SubQuery{
			{Query: "test concepts", Focus: "main topic"},
			{Query: "test query with multiple concepts and ideas", Focus: "duplicate"}, // Same as original
			{Query: "query examples", Focus: "examples"},
		}

		jsonResponse, _ := json.Marshal(subQueries)

		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(&CompletionResponse{
				Text:   string(jsonResponse),
				Tokens: 50,
			}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.Len(t, result.Expansions, 2) // Duplicate filtered out

		mockLLM.AssertExpectations(t)
	})
}

func TestDecompositionExpander_isSimpleQuery(t *testing.T) {
	expander := &DecompositionExpander{}

	tests := []struct {
		name     string
		query    string
		expected bool
	}{
		{
			name:     "very short query",
			query:    "Python",
			expected: true,
		},
		{
			name:     "three word query",
			query:    "Python list methods",
			expected: true,
		},
		{
			name:     "query with and",
			query:    "authentication and authorization",
			expected: true, // This is only 3 words, so it's simple
		},
		{
			name:     "query with comma",
			query:    "Python, Java, Go comparison",
			expected: false,
		},
		{
			name:     "question mark",
			query:    "What is dependency injection?",
			expected: false,
		},
		{
			name:     "long simple query",
			query:    "implement binary search tree",
			expected: true,
		},
		{
			name:     "complex with multiple indicators",
			query:    "compare REST vs GraphQL with examples",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expander.isSimpleQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDecompositionExpander_simpleDecompose(t *testing.T) {
	// simpleDecompose is a private method, test it through the public API
	mockLLM := new(MockLLMClient)
	expander := NewDecompositionExpander(mockLLM, nil)

	t.Run("split on and via fallback", func(t *testing.T) {
		query := "authentication and authorization and logging"

		// Mock LLM to return error, forcing fallback to simple decompose
		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("LLM error")).Once()

		result, err := expander.Expand(context.Background(), query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, query, result.Original)
		assert.Len(t, result.Expansions, 3)
		assert.Equal(t, "authentication", result.Expansions[0].Text)
		assert.Equal(t, "authorization", result.Expansions[1].Text)
		assert.Equal(t, "logging", result.Expansions[2].Text)
	})

	t.Run("split long query via fallback", func(t *testing.T) {
		query := "implement secure REST API with authentication"

		// Mock LLM to return error, forcing fallback to simple decompose
		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("LLM error")).Once()

		result, err := expander.Expand(context.Background(), query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Len(t, result.Expansions, 2)
		assert.Contains(t, result.Expansions[0].Text, "implement")
		assert.Contains(t, result.Expansions[1].Text, "authentication")
	})
}

func TestDecompositionExpander_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	llmClient := NewSimpleLLMClient()
	expander := NewDecompositionExpander(llmClient, nil)

	query := "implement authentication and authorization in microservices"
	result, err := expander.Expand(ctx, query, nil)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Greater(t, len(result.Expansions), 0)

	// Check that expansions are different from original
	for _, exp := range result.Expansions {
		assert.NotEqual(t, query, exp.Text)
		assert.Equal(t, ExpansionTypeDecompose, exp.Type)
	}
}
