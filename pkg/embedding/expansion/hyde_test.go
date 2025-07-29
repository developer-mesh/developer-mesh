package expansion

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestNewHyDEExpander(t *testing.T) {
	mockLLM := new(MockLLMClient)
	logger := observability.NewLogger("test")

	expander := NewHyDEExpander(mockLLM, logger)
	assert.NotNil(t, expander)
	assert.NotNil(t, expander.llmClient)
	assert.NotNil(t, expander.templates)
	assert.Len(t, expander.templates, 4) // default, code, documentation, troubleshooting
}

func TestHyDEExpander_Expand(t *testing.T) {
	ctx := context.Background()

	t.Run("successful expansion", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewHyDEExpander(mockLLM, nil)

		query := "how to implement error handling in Go"
		hydeResponse := `Error handling in Go is implemented using the built-in error type and explicit error checking.

Key concepts:
1. Functions return error as the last return value
2. Check errors immediately after function calls
3. Use custom error types for specific error cases

Example:
` + "```go" + `
func doSomething() error {
    result, err := someOperation()
    if err != nil {
        return fmt.Errorf("operation failed: %w", err)
    }
    return nil
}
` + "```" + `

Best practices include wrapping errors with context and handling errors at appropriate levels.`

		mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(req CompletionRequest) bool {
			return req.MaxTokens == 500 && req.Temperature == 0.7
		})).Return(&CompletionResponse{
			Text:   hydeResponse,
			Tokens: 150,
		}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, query, result.Original)
		assert.Len(t, result.Expansions, 1)

		expansion := result.Expansions[0]
		assert.Equal(t, ExpansionTypeHyDE, expansion.Type)
		assert.Equal(t, hydeResponse, expansion.Text)
		assert.Equal(t, float32(0.3), expansion.Weight)
		assert.Equal(t, "code", expansion.Metadata["query_type"])

		mockLLM.AssertExpectations(t)
	})

	t.Run("documentation query type", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewHyDEExpander(mockLLM, nil)

		query := "explain what is dependency injection"

		mockLLM.On("Complete", mock.Anything, mock.MatchedBy(func(req CompletionRequest) bool {
			return strings.Contains(req.Prompt, "technical documentation")
		})).Return(&CompletionResponse{
			Text:   "Dependency injection is a design pattern...",
			Tokens: 100,
		}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.NoError(t, err)
		assert.Equal(t, "documentation", result.Expansions[0].Metadata["query_type"])

		mockLLM.AssertExpectations(t)
	})

	t.Run("empty query validation", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewHyDEExpander(mockLLM, nil)

		result, err := expander.Expand(ctx, "", nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "query cannot be empty")
		assert.Nil(t, result)
	})

	t.Run("LLM error handling", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		logger := observability.NewLogger("test")
		expander := NewHyDEExpander(mockLLM, logger)

		query := "test query"

		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(nil, fmt.Errorf("LLM service unavailable")).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate HyDE")
		assert.Nil(t, result)

		mockLLM.AssertExpectations(t)
	})

	t.Run("empty response handling", func(t *testing.T) {
		mockLLM := new(MockLLMClient)
		expander := NewHyDEExpander(mockLLM, nil)

		query := "test query"

		mockLLM.On("Complete", mock.Anything, mock.Anything).
			Return(&CompletionResponse{
				Text:   "   \n\t  ",
				Tokens: 0,
			}, nil).Once()

		result, err := expander.Expand(ctx, query, nil)

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty HyDE response")
		assert.Nil(t, result)

		mockLLM.AssertExpectations(t)
	})
}

func TestHyDEExpander_detectQueryType(t *testing.T) {
	expander := &HyDEExpander{
		logger: observability.NewLogger("test"),
	}

	tests := []struct {
		name     string
		query    string
		expected string
	}{
		{
			name:     "code query with function",
			query:    "how to implement a function in Python",
			expected: "code",
		},
		{
			name:     "code query with debug",
			query:    "debug memory leak in Java application",
			expected: "code", // "debug" alone isn't enough to classify as troubleshooting
		},
		{
			name:     "documentation query",
			query:    "explain the concept of microservices",
			expected: "documentation",
		},
		{
			name:     "troubleshooting query",
			query:    "fix connection timeout error",
			expected: "troubleshooting",
		},
		{
			name:     "default query",
			query:    "best practices for software development",
			expected: "default",
		},
		{
			name:     "mixed signals - code wins",
			query:    "explain how to write functions",
			expected: "code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expander.detectQueryType(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHyDEExpander_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	ctx := context.Background()
	llmClient := NewSimpleLLMClient()
	expander := NewHyDEExpander(llmClient, nil)

	query := "implement error handling"
	result, err := expander.Expand(ctx, query, nil)

	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Expansions, 1)
	assert.Contains(t, result.Expansions[0].Text, "implement error handling")
	assert.Equal(t, ExpansionTypeHyDE, result.Expansions[0].Type)
}
