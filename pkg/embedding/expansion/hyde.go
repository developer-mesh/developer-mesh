package expansion

import (
	"context"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// HyDEExpander implements Hypothetical Document Embeddings
type HyDEExpander struct {
	llmClient LLMClient
	templates map[string]string
	logger    observability.Logger
}

// NewHyDEExpander creates a new HyDE expander
func NewHyDEExpander(llmClient LLMClient, logger observability.Logger) *HyDEExpander {
	if logger == nil {
		logger = observability.NewLogger("expansion.hyde")
	}

	return &HyDEExpander{
		llmClient: llmClient,
		logger:    logger,
		templates: map[string]string{
			"default": `Generate a detailed, technical answer to this question: "%s"

Include specific examples, code snippets if relevant, and technical details.
The answer should be comprehensive and directly address the query.`,

			"code": `Write a complete code example that answers this programming question: "%s"

Include:
- Complete, runnable code
- Comments explaining key parts
- Any necessary imports or setup
- Example usage`,

			"documentation": `Write detailed technical documentation that answers: "%s"

Include:
- Overview and context
- Step-by-step explanations
- Best practices
- Common pitfalls
- Examples`,

			"troubleshooting": `Provide a detailed troubleshooting guide for: "%s"

Include:
- Common causes
- Diagnostic steps
- Solutions
- Prevention tips
- Related issues`,
		},
	}
}

// Expand generates hypothetical documents for the query
func (h *HyDEExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "expansion.hyde")
	defer span.End()

	span.SetAttribute("query", query)

	if err := ValidateQuery(query); err != nil {
		return nil, err
	}

	// Detect query type
	queryType := h.detectQueryType(query)
	template, exists := h.templates[queryType]
	if !exists {
		template = h.templates["default"]
	}

	prompt := fmt.Sprintf(template, query)

	h.logger.Debug("Generating HyDE expansion", map[string]interface{}{
		"query_type":   queryType,
		"query_length": len(query),
	})

	// Generate hypothetical document
	response, err := h.llmClient.Complete(ctx, CompletionRequest{
		Prompt:       prompt,
		MaxTokens:    500,
		Temperature:  0.7,
		SystemPrompt: "You are a technical expert helping to generate relevant documents for search queries. Be specific and detailed.",
	})
	if err != nil {
		h.logger.Error("Failed to generate HyDE", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		return nil, fmt.Errorf("failed to generate HyDE: %w", err)
	}

	// Clean up the response
	expandedText := strings.TrimSpace(response.Text)
	if expandedText == "" {
		return nil, fmt.Errorf("empty HyDE response")
	}

	span.SetAttribute("expansion_length", len(expandedText))
	span.SetAttribute("query_type", queryType)

	return &ExpandedQuery{
		Original: query,
		Expansions: []QueryVariation{
			{
				Text:   expandedText,
				Type:   ExpansionTypeHyDE,
				Weight: 0.3, // Lower weight for hypothetical documents
				Metadata: map[string]interface{}{
					"query_type":     queryType,
					"template_used":  queryType,
					"original_query": query,
					"token_count":    response.Tokens,
				},
			},
		},
	}, nil
}

// detectQueryType analyzes the query to determine its type
func (h *HyDEExpander) detectQueryType(query string) string {
	lowerQuery := strings.ToLower(query)

	// Code-related keywords
	codeKeywords := []string{
		"code", "function", "implement", "example", "snippet",
		"how to write", "syntax", "algorithm", "method", "class",
		"debug", "error", "bug", "compile",
	}
	codeScore := 0
	for _, keyword := range codeKeywords {
		if strings.Contains(lowerQuery, keyword) {
			codeScore++
		}
	}

	// Documentation keywords
	docKeywords := []string{
		"documentation", "explain", "what is", "describe", "guide",
		"tutorial", "how does", "understand", "concept", "theory",
	}
	docScore := 0
	for _, keyword := range docKeywords {
		if strings.Contains(lowerQuery, keyword) {
			docScore++
		}
	}

	// Troubleshooting keywords
	troubleKeywords := []string{
		"error", "issue", "problem", "fix", "solve", "troubleshoot",
		"not working", "failed", "broken", "debug", "why",
	}
	troubleScore := 0
	for _, keyword := range troubleKeywords {
		if strings.Contains(lowerQuery, keyword) {
			troubleScore++
		}
	}

	// Determine type based on scores
	maxScore := codeScore
	queryType := "default"

	if docScore > maxScore {
		maxScore = docScore
		queryType = "documentation"
	}
	if troubleScore > maxScore {
		queryType = "troubleshooting"
	} else if codeScore > 0 && codeScore == maxScore {
		queryType = "code"
	}

	h.logger.Debug("Query type detection", map[string]interface{}{
		"query":         query,
		"type":          queryType,
		"code_score":    codeScore,
		"doc_score":     docScore,
		"trouble_score": troubleScore,
	})

	return queryType
}
