package tokenizer

import (
	"strings"
	"unicode"
)

// Tokenizer interface for counting and splitting tokens
type Tokenizer interface {
	// CountTokens returns the number of tokens in the text
	CountTokens(text string) int
	// Tokenize splits text into tokens
	Tokenize(text string) []string
	// GetTokenLimit returns the maximum token limit for this tokenizer
	GetTokenLimit() int
}

// SimpleTokenizer provides a basic word-based tokenization
// This is a simplified implementation - in production, use tiktoken or similar
type SimpleTokenizer struct {
	tokenLimit int
}

// NewSimpleTokenizer creates a new simple tokenizer
func NewSimpleTokenizer(tokenLimit int) *SimpleTokenizer {
	if tokenLimit <= 0 {
		tokenLimit = 8192 // Default token limit
	}
	return &SimpleTokenizer{
		tokenLimit: tokenLimit,
	}
}

// CountTokens estimates token count based on words and punctuation
func (t *SimpleTokenizer) CountTokens(text string) int {
	if text == "" {
		return 0
	}

	// Simple heuristic: count words and punctuation
	// This approximates GPT tokenization reasonably well for English
	tokens := 0
	inWord := false

	for _, r := range text {
		if unicode.IsSpace(r) {
			if inWord {
				tokens++
				inWord = false
			}
		} else if unicode.IsPunct(r) {
			tokens++
			inWord = false
		} else {
			inWord = true
		}
	}

	if inWord {
		tokens++
	}

	// Adjust for common subword tokens (rough approximation)
	// On average, every 0.75 words is approximately 1 token
	wordCount := len(strings.Fields(text))
	estimatedTokens := int(float64(wordCount) * 1.3)

	// Return the higher estimate
	if estimatedTokens > tokens {
		return estimatedTokens
	}
	return tokens
}

// Tokenize splits text into tokens
func (t *SimpleTokenizer) Tokenize(text string) []string {
	if text == "" {
		return []string{}
	}

	var tokens []string
	var currentToken strings.Builder

	for _, r := range text {
		if unicode.IsSpace(r) {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			// Include significant whitespace as tokens
			if r == '\n' {
				tokens = append(tokens, "\n")
			}
		} else if unicode.IsPunct(r) {
			if currentToken.Len() > 0 {
				tokens = append(tokens, currentToken.String())
				currentToken.Reset()
			}
			tokens = append(tokens, string(r))
		} else {
			currentToken.WriteRune(r)
		}
	}

	if currentToken.Len() > 0 {
		tokens = append(tokens, currentToken.String())
	}

	return tokens
}

// GetTokenLimit returns the maximum token limit
func (t *SimpleTokenizer) GetTokenLimit() int {
	return t.tokenLimit
}

// TikTokenTokenizer would be a more accurate implementation using tiktoken library
// This is a placeholder for when we integrate with the actual tiktoken library
type TikTokenTokenizer struct {
	model      string
	tokenLimit int
}

// NewTikTokenTokenizer creates a new tiktoken-based tokenizer
func NewTikTokenTokenizer(model string) *TikTokenTokenizer {
	// Token limits for different models
	limits := map[string]int{
		"gpt-4":                  8192,
		"gpt-4-32k":              32768,
		"gpt-3.5-turbo":          4096,
		"gpt-3.5-turbo-16k":      16384,
		"text-embedding-3-small": 8191,
		"text-embedding-3-large": 8191,
	}

	limit, ok := limits[model]
	if !ok {
		limit = 8192 // default
	}

	return &TikTokenTokenizer{
		model:      model,
		tokenLimit: limit,
	}
}

// CountTokens counts tokens using tiktoken encoding
func (t *TikTokenTokenizer) CountTokens(text string) int {
	// TODO: Integrate with actual tiktoken library
	// For now, use the simple approximation
	return NewSimpleTokenizer(t.tokenLimit).CountTokens(text)
}

// Tokenize splits text into tokens using tiktoken
func (t *TikTokenTokenizer) Tokenize(text string) []string {
	// TODO: Integrate with actual tiktoken library
	// For now, use the simple tokenization
	return NewSimpleTokenizer(t.tokenLimit).Tokenize(text)
}

// GetTokenLimit returns the token limit for the model
func (t *TikTokenTokenizer) GetTokenLimit() int {
	return t.tokenLimit
}
