package expansion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// DecompositionExpander breaks complex queries into sub-queries
type DecompositionExpander struct {
	llmClient LLMClient
	logger    observability.Logger
}

// NewDecompositionExpander creates a new decomposition expander
func NewDecompositionExpander(llmClient LLMClient, logger observability.Logger) *DecompositionExpander {
	if logger == nil {
		logger = observability.NewLogger("expansion.decomposition")
	}
	return &DecompositionExpander{
		llmClient: llmClient,
		logger:    logger,
	}
}

// SubQuery represents a decomposed sub-query
type SubQuery struct {
	Query string `json:"query"`
	Focus string `json:"focus"`
}

// Expand decomposes the query into simpler sub-queries
func (d *DecompositionExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "expansion.decomposition")
	defer span.End()

	span.SetAttribute("query", query)

	if err := ValidateQuery(query); err != nil {
		return nil, err
	}

	// Check if query is already simple
	if d.isSimpleQuery(query) {
		d.logger.Debug("Query is already simple, skipping decomposition", map[string]interface{}{
			"query": query,
		})
		return &ExpandedQuery{
			Original:   query,
			Expansions: []QueryVariation{},
		}, nil
	}

	prompt := fmt.Sprintf(`Decompose this search query into simpler sub-queries: "%s"

Rules:
1. Each sub-query should capture a specific aspect of the original query
2. Sub-queries should be self-contained and searchable
3. Avoid redundancy between sub-queries
4. Maximum 4 sub-queries
5. Focus on breaking down complex concepts, multiple topics, or compound questions

Return as JSON array of objects with 'query' and 'focus' fields.
Example: [{"query": "Python error handling", "focus": "language and topic"}, {"query": "try except blocks", "focus": "specific construct"}]`, query)

	response, err := d.llmClient.Complete(ctx, CompletionRequest{
		Prompt:       prompt,
		MaxTokens:    300,
		Temperature:  0.3,
		Format:       "json",
		SystemPrompt: "You are an expert at breaking down complex search queries into simpler, more targeted sub-queries.",
	})
	if err != nil {
		d.logger.Error("Failed to decompose query", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		// Fallback to simple decomposition
		return d.simpleDecompose(query), nil
	}

	// Parse JSON response
	var decomposed []SubQuery
	if err := json.Unmarshal([]byte(response.Text), &decomposed); err != nil {
		d.logger.Warn("Failed to parse decomposition response, using fallback", map[string]interface{}{
			"error":    err.Error(),
			"response": response.Text,
		})
		// Fallback to simple decomposition
		return d.simpleDecompose(query), nil
	}

	// Validate and filter decomposed queries
	expansions := make([]QueryVariation, 0, len(decomposed))
	for i, subQuery := range decomposed {
		// Skip if sub-query is too similar to original
		if strings.TrimSpace(subQuery.Query) == "" ||
			strings.EqualFold(subQuery.Query, query) {
			continue
		}

		// Calculate weight based on position (earlier = higher weight)
		weight := 1.0 / float32(i+2)

		expansions = append(expansions, QueryVariation{
			Text:   subQuery.Query,
			Type:   ExpansionTypeDecompose,
			Weight: weight,
			Metadata: map[string]interface{}{
				"focus":          subQuery.Focus,
				"original_query": query,
				"position":       i + 1,
			},
		})
	}

	span.SetAttribute("sub_queries_count", len(expansions))

	d.logger.Info("Query decomposed", map[string]interface{}{
		"original_query": query,
		"sub_queries":    len(expansions),
	})

	return &ExpandedQuery{
		Original:   query,
		Expansions: expansions,
	}, nil
}

// isSimpleQuery checks if a query is already simple enough
func (d *DecompositionExpander) isSimpleQuery(query string) bool {
	words := strings.Fields(query)

	// Very short queries are already simple
	if len(words) <= 3 {
		return true
	}

	lowerQuery := strings.ToLower(query)

	// Check for complex indicators
	complexIndicators := []string{
		" or ", " with ", " without ", " but ",
		" vs ", " versus ", " compared to ", " along with ",
		"?", ",", ";", "(",
	}

	// Special case: "and" only counts as complex if query is longer than 3 words
	if len(words) > 3 && strings.Contains(lowerQuery, " and ") {
		complexIndicators = append(complexIndicators, " and ")
	}

	hasComplexity := false
	for _, indicator := range complexIndicators {
		if strings.Contains(lowerQuery, indicator) {
			hasComplexity = true
			break
		}
	}

	return !hasComplexity && len(words) <= 5
}

// simpleDecompose provides a heuristic-based decomposition fallback
func (d *DecompositionExpander) simpleDecompose(query string) *ExpandedQuery {
	words := strings.Fields(query)
	expansions := []QueryVariation{}

	// If query has "and" or "or", split on those
	if strings.Contains(strings.ToLower(query), " and ") {
		parts := strings.Split(query, " and ")
		for i, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				expansions = append(expansions, QueryVariation{
					Text:   trimmed,
					Type:   ExpansionTypeDecompose,
					Weight: 1.0 / float32(i+2),
					Metadata: map[string]interface{}{
						"focus":          "split_on_and",
						"original_query": query,
					},
				})
			}
		}
	}

	// Extract noun phrases (simple heuristic)
	if len(expansions) == 0 && len(words) > 4 {
		// Take first half and second half as separate queries
		mid := len(words) / 2
		firstHalf := strings.Join(words[:mid], " ")
		secondHalf := strings.Join(words[mid:], " ")

		expansions = append(expansions,
			QueryVariation{
				Text:   firstHalf,
				Type:   ExpansionTypeDecompose,
				Weight: 0.5,
				Metadata: map[string]interface{}{
					"focus":          "first_half",
					"original_query": query,
				},
			},
			QueryVariation{
				Text:   secondHalf,
				Type:   ExpansionTypeDecompose,
				Weight: 0.3,
				Metadata: map[string]interface{}{
					"focus":          "second_half",
					"original_query": query,
				},
			},
		)
	}

	return &ExpandedQuery{
		Original:   query,
		Expansions: expansions,
	}
}
