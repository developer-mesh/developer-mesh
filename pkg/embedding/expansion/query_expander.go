package expansion

import (
	"context"
	"fmt"
	"strings"
)

// QueryExpander expands queries for better recall
type QueryExpander interface {
	Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error)
}

// ExpansionOptions configures query expansion
type ExpansionOptions struct {
	MaxExpansions   int
	IncludeOriginal bool
	ExpansionTypes  []ExpansionType
	Language        string
	Domain          string
}

// ExpansionType defines different expansion strategies
type ExpansionType string

const (
	ExpansionTypeSynonym         ExpansionType = "synonym"
	ExpansionTypeHyDE            ExpansionType = "hyde"
	ExpansionTypeDecompose       ExpansionType = "decompose"
	ExpansionTypeBacktranslation ExpansionType = "backtranslation"
)

// ExpandedQuery contains the original and expanded queries
type ExpandedQuery struct {
	Original   string
	Expansions []QueryVariation
}

// QueryVariation represents a single query variation
type QueryVariation struct {
	Text     string
	Type     ExpansionType
	Weight   float32
	Metadata map[string]interface{}
}

// LLMClient interface for language model interactions
type LLMClient interface {
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)
}

// CompletionRequest for LLM calls
type CompletionRequest struct {
	Prompt       string
	MaxTokens    int
	Temperature  float32
	Format       string // "json" or "text"
	SystemPrompt string
}

// CompletionResponse from LLM
type CompletionResponse struct {
	Text   string
	Tokens int
}

// ValidateQuery ensures query is valid for expansion
func ValidateQuery(query string) error {
	if strings.TrimSpace(query) == "" {
		return fmt.Errorf("query cannot be empty")
	}
	if len(query) > 500 {
		return fmt.Errorf("query too long (max 500 characters)")
	}
	return nil
}
