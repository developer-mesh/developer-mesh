package expansion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/stretchr/testify/mock"
)

// MockLLMClient is a mock implementation of LLMClient for testing
type MockLLMClient struct {
	mock.Mock
}

// Complete mocks the LLM completion call
func (m *MockLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*CompletionResponse), args.Error(1)
}

// SimpleLLMClient provides a simple implementation for testing
type SimpleLLMClient struct {
	responseMap map[string]string
}

// NewSimpleLLMClient creates a simple LLM client for testing
func NewSimpleLLMClient() *SimpleLLMClient {
	return &SimpleLLMClient{
		responseMap: initializeResponseMap(),
	}
}

// Complete provides simple responses based on prompt patterns
func (s *SimpleLLMClient) Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error) {
	// Check for HyDE requests
	if strings.Contains(req.Prompt, "Generate a detailed, technical answer") ||
		strings.Contains(req.Prompt, "Write a complete code example") {
		query := extractQueryFromPrompt(req.Prompt)
		// Generate a focused hypothetical document that directly answers the query
		response := fmt.Sprintf(`To implement %s, follow these best practices:

1. Use structured logging with correlation IDs to trace requests across services
2. Implement circuit breakers for fault tolerance between service calls
3. Centralize error handling with middleware that captures and formats errors consistently
4. Use distributed tracing tools like OpenTelemetry for observability
5. Implement retry logic with exponential backoff for transient failures

Example implementation using Go:
`+"```go"+`
func errorMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        defer func() {
            if err := recover(); err != nil {
                log.WithField("error", err).Error("Panic recovered")
                http.Error(w, "Internal Server Error", 500)
            }
        }()
        next.ServeHTTP(w, r)
    })
}
`+"```", query)
		return &CompletionResponse{
			Text:   response,
			Tokens: 150,
		}, nil
	}

	// Check for decomposition requests
	if strings.Contains(req.Prompt, "Decompose this search query") {
		query := extractQueryFromPrompt(req.Prompt)
		subQueries := decomposeQuery(query)

		jsonBytes, _ := json.Marshal(subQueries)
		return &CompletionResponse{
			Text:   string(jsonBytes),
			Tokens: 50,
		}, nil
	}

	// Check for synonym requests
	if strings.Contains(req.Prompt, "Generate synonyms") {
		query := extractQueryFromPrompt(req.Prompt)
		synonyms := generateSynonyms(query)

		jsonBytes, _ := json.Marshal(synonyms)
		return &CompletionResponse{
			Text:   string(jsonBytes),
			Tokens: 50,
		}, nil
	}

	// Default response
	return &CompletionResponse{
		Text:   "Default response for: " + req.Prompt,
		Tokens: 10,
	}, nil
}

// Helper functions

func extractQueryFromPrompt(prompt string) string {
	// Extract query between quotes
	start := strings.Index(prompt, `"`)
	if start == -1 {
		return "unknown query"
	}
	end := strings.Index(prompt[start+1:], `"`)
	if end == -1 {
		return "unknown query"
	}
	return prompt[start+1 : start+1+end]
}

func decomposeQuery(query string) []SubQuery {
	words := strings.Fields(query)
	var subQueries []SubQuery

	if len(words) > 3 {
		// Split into smaller queries
		mid := len(words) / 2
		subQueries = append(subQueries,
			SubQuery{
				Query: strings.Join(words[:mid], " "),
				Focus: "first part",
			},
			SubQuery{
				Query: strings.Join(words[mid:], " "),
				Focus: "second part",
			},
		)
	}

	// Add a more specific query
	if len(words) > 0 {
		subQueries = append(subQueries, SubQuery{
			Query: words[len(words)-1] + " examples",
			Focus: "specific examples",
		})
	}

	return subQueries
}

func generateSynonyms(query string) []SynonymResult {
	// Simple synonym generation
	var results []SynonymResult

	// Common programming synonyms
	if strings.Contains(strings.ToLower(query), "function") {
		results = append(results, SynonymResult{
			Term:    "method implementation",
			Context: "alternative term for function",
		})
	}

	if strings.Contains(strings.ToLower(query), "error") {
		results = append(results, SynonymResult{
			Term:    "exception handling",
			Context: "related to error management",
		})
	}

	// General alternatives
	results = append(results,
		SynonymResult{
			Term:    query + " tutorial",
			Context: "educational variant",
		},
		SynonymResult{
			Term:    "how to " + query,
			Context: "question format",
		},
	)

	return results
}

func initializeResponseMap() map[string]string {
	return map[string]string{
		"default": "This is a default response",
	}
}
