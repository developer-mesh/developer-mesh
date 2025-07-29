package expansion

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SynonymExpander generates synonym-based query expansions
type SynonymExpander struct {
	llmClient      LLMClient
	logger         observability.Logger
	domainSynonyms map[string][]string
}

// NewSynonymExpander creates a new synonym expander
func NewSynonymExpander(llmClient LLMClient, logger observability.Logger) *SynonymExpander {
	if logger == nil {
		logger = observability.NewLogger("expansion.synonym")
	}

	return &SynonymExpander{
		llmClient:      llmClient,
		logger:         logger,
		domainSynonyms: initializeDomainSynonyms(),
	}
}

// SynonymResult represents synonyms for a term
type SynonymResult struct {
	Term     string   `json:"term"`
	Synonyms []string `json:"synonyms"`
	Context  string   `json:"context"`
}

// Expand generates synonym-based expansions
func (s *SynonymExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "expansion.synonym")
	defer span.End()

	span.SetAttribute("query", query)

	if err := ValidateQuery(query); err != nil {
		return nil, err
	}

	// First try domain-specific synonyms
	domainExpansions := s.expandWithDomainSynonyms(query, opts)

	// Then use LLM for contextual synonyms
	llmExpansions, err := s.expandWithLLM(ctx, query, opts)
	if err != nil {
		s.logger.Warn("LLM synonym expansion failed, using only domain synonyms", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		// Continue with just domain expansions
	}

	// Combine and deduplicate expansions
	allExpansions := append(domainExpansions, llmExpansions...)
	deduped := s.deduplicateExpansions(allExpansions)

	span.SetAttribute("total_expansions", len(deduped))

	return &ExpandedQuery{
		Original:   query,
		Expansions: deduped,
	}, nil
}

// expandWithDomainSynonyms uses predefined domain synonyms
func (s *SynonymExpander) expandWithDomainSynonyms(query string, opts *ExpansionOptions) []QueryVariation {
	var expansions []QueryVariation
	lowerQuery := strings.ToLower(query)
	words := strings.Fields(lowerQuery)

	for _, word := range words {
		if synonyms, exists := s.domainSynonyms[word]; exists {
			for _, synonym := range synonyms {
				// Replace word with synonym in query
				expanded := strings.ReplaceAll(query, word, synonym)
				if expanded != query {
					expansions = append(expansions, QueryVariation{
						Text:   expanded,
						Type:   ExpansionTypeSynonym,
						Weight: 0.7,
						Metadata: map[string]interface{}{
							"source":        "domain",
							"original_term": word,
							"synonym":       synonym,
						},
					})
				}
			}
		}
	}

	return expansions
}

// expandWithLLM uses the language model for contextual synonyms
func (s *SynonymExpander) expandWithLLM(ctx context.Context, query string, opts *ExpansionOptions) ([]QueryVariation, error) {
	domain := "general"
	if opts != nil && opts.Domain != "" {
		domain = opts.Domain
	}

	prompt := fmt.Sprintf(`Generate synonyms and related terms for this search query: "%s"

Context: %s domain
Rules:
1. Provide synonyms that maintain the same search intent
2. Include related technical terms if applicable
3. Consider common abbreviations and full forms
4. Maximum 5 variations
5. Each variation should be a complete, searchable query

Return as JSON array of objects with fields:
- "term": the synonym or variation
- "context": brief explanation of why this synonym is relevant

Example output:
[
  {"term": "machine learning models", "context": "alternative phrasing"},
  {"term": "ML algorithms", "context": "common abbreviation"}
]`, query, domain)

	response, err := s.llmClient.Complete(ctx, CompletionRequest{
		Prompt:       prompt,
		MaxTokens:    300,
		Temperature:  0.5,
		Format:       "json",
		SystemPrompt: "You are an expert at generating synonyms and related terms for search queries, maintaining search intent while expanding coverage.",
	})
	if err != nil {
		return nil, fmt.Errorf("LLM synonym generation failed: %w", err)
	}

	// Parse JSON response
	var synonymResults []SynonymResult
	if err := json.Unmarshal([]byte(response.Text), &synonymResults); err != nil {
		s.logger.Warn("Failed to parse synonym response", map[string]interface{}{
			"error":    err.Error(),
			"response": response.Text,
		})
		return nil, fmt.Errorf("failed to parse synonym response: %w", err)
	}

	// Convert to query variations
	var expansions []QueryVariation
	for i, result := range synonymResults {
		if strings.TrimSpace(result.Term) == "" ||
			strings.EqualFold(result.Term, query) {
			continue
		}

		weight := 0.8 - (float32(i) * 0.1) // Decreasing weights
		if weight < 0.3 {
			weight = 0.3
		}

		expansions = append(expansions, QueryVariation{
			Text:   result.Term,
			Type:   ExpansionTypeSynonym,
			Weight: weight,
			Metadata: map[string]interface{}{
				"source":         "llm",
				"context":        result.Context,
				"original_query": query,
			},
		})
	}

	return expansions, nil
}

// deduplicateExpansions removes duplicate expansions
func (s *SynonymExpander) deduplicateExpansions(expansions []QueryVariation) []QueryVariation {
	seen := make(map[string]bool)
	var deduped []QueryVariation

	for _, exp := range expansions {
		key := strings.ToLower(strings.TrimSpace(exp.Text))
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, exp)
		}
	}

	return deduped
}

// initializeDomainSynonyms creates common domain-specific synonyms
func initializeDomainSynonyms() map[string][]string {
	return map[string][]string{
		// Programming terms
		"function":  {"method", "func", "procedure"},
		"variable":  {"var", "parameter", "argument"},
		"error":     {"exception", "bug", "issue"},
		"debug":     {"troubleshoot", "diagnose", "fix"},
		"implement": {"create", "build", "develop"},
		"algorithm": {"algo", "procedure", "method"},
		"database":  {"db", "data store", "repository"},
		"api":       {"interface", "endpoint", "service"},

		// DevOps terms
		"deploy":     {"release", "rollout", "ship"},
		"container":  {"docker", "pod"},
		"kubernetes": {"k8s", "k8"},
		"cicd":       {"ci/cd", "continuous integration", "continuous deployment"},
		"pipeline":   {"workflow", "job"},
		"monitor":    {"observe", "track", "watch"},

		// General tech
		"search": {"find", "query", "lookup"},
		"create": {"make", "generate", "produce"},
		"delete": {"remove", "destroy", "drop"},
		"update": {"modify", "change", "edit"},
		"list":   {"show", "display", "get"},
		"config": {"configuration", "settings", "preferences"},

		// AI/ML terms
		"ml":        {"machine learning"},
		"ai":        {"artificial intelligence"},
		"llm":       {"large language model", "language model"},
		"embedding": {"vector", "representation"},
		"model":     {"algorithm", "network"},
	}
}
