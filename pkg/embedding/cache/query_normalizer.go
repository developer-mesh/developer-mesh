package cache

import (
	"regexp"
	"strings"
	"unicode"
)

// QueryNormalizer preprocesses queries for consistent caching
type QueryNormalizer interface {
	Normalize(query string) string
}

// DefaultQueryNormalizer implements standard query normalization
type DefaultQueryNormalizer struct {
	// Regular expressions for normalization
	whitespaceRegex  *regexp.Regexp
	punctuationRegex *regexp.Regexp
	stopWords        map[string]bool
	enableStopWords  bool
	preserveNumbers  bool
}

// NewQueryNormalizer creates a new query normalizer with default settings
func NewQueryNormalizer() QueryNormalizer {
	return &DefaultQueryNormalizer{
		whitespaceRegex:  regexp.MustCompile(`\s+`),
		punctuationRegex: regexp.MustCompile(`[^\w\s-]`),
		enableStopWords:  true,
		preserveNumbers:  true,
		stopWords:        getDefaultStopWords(),
	}
}

// NewQueryNormalizerWithOptions creates a normalizer with custom options
func NewQueryNormalizerWithOptions(enableStopWords, preserveNumbers bool) QueryNormalizer {
	return &DefaultQueryNormalizer{
		whitespaceRegex:  regexp.MustCompile(`\s+`),
		punctuationRegex: regexp.MustCompile(`[^\w\s-]`),
		enableStopWords:  enableStopWords,
		preserveNumbers:  preserveNumbers,
		stopWords:        getDefaultStopWords(),
	}
}

// Normalize processes a query for consistent caching
func (n *DefaultQueryNormalizer) Normalize(query string) string {
	if query == "" {
		return ""
	}

	// Convert to lowercase
	normalized := strings.ToLower(query)

	// Remove extra whitespace
	normalized = strings.TrimSpace(normalized)
	normalized = n.whitespaceRegex.ReplaceAllString(normalized, " ")

	// Remove punctuation except hyphens (for hyphenated words)
	normalized = n.punctuationRegex.ReplaceAllString(normalized, " ")

	// Split into words
	words := strings.Fields(normalized)

	// Filter and process words
	filteredWords := make([]string, 0, len(words))
	for _, word := range words {
		// Skip empty words
		if word == "" {
			continue
		}

		// Skip stop words if enabled
		if n.enableStopWords && n.isStopWord(word) {
			continue
		}

		// Handle numbers
		if !n.preserveNumbers && isNumber(word) {
			continue
		}

		// Skip very short words (except numbers if preserved)
		if len(word) < 2 && (!n.preserveNumbers || !isNumber(word)) {
			continue
		}

		filteredWords = append(filteredWords, word)
	}

	// Sort words for consistent ordering (improves cache hit rate)
	// We'll keep the original order to preserve semantic meaning
	// but deduplicate consecutive duplicates
	dedupedWords := deduplicateConsecutive(filteredWords)

	return strings.Join(dedupedWords, " ")
}

// isStopWord checks if a word is a stop word
func (n *DefaultQueryNormalizer) isStopWord(word string) bool {
	return n.stopWords[word]
}

// isNumber checks if a string is numeric
func isNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) && r != '.' && r != ',' {
			return false
		}
	}
	return true
}

// deduplicateConsecutive removes consecutive duplicate words
func deduplicateConsecutive(words []string) []string {
	if len(words) <= 1 {
		return words
	}

	result := make([]string, 0, len(words))
	result = append(result, words[0])

	for i := 1; i < len(words); i++ {
		if words[i] != words[i-1] {
			result = append(result, words[i])
		}
	}

	return result
}

// getDefaultStopWords returns common English stop words
func getDefaultStopWords() map[string]bool {
	stopWords := map[string]bool{
		// Articles
		"a": true, "an": true, "the": true,
		// Pronouns
		"i": true, "me": true, "my": true, "myself": true, "we": true, "our": true,
		"ours": true, "ourselves": true, "you": true, "your": true, "yours": true,
		"yourself": true, "yourselves": true, "he": true, "him": true, "his": true,
		"himself": true, "she": true, "her": true, "hers": true, "herself": true,
		"it": true, "its": true, "itself": true, "they": true, "them": true,
		"their": true, "theirs": true, "themselves": true,
		// Common verbs
		"is": true, "am": true, "are": true, "was": true, "were": true, "be": true,
		"been": true, "being": true, "have": true, "has": true, "had": true,
		"having": true, "do": true, "does": true, "did": true, "doing": true,
		"will": true, "would": true, "should": true, "could": true, "ought": true,
		"might": true, "must": true, "can": true, "may": true,
		// Prepositions
		"at": true, "by": true, "for": true, "with": true, "about": true,
		"against": true, "between": true, "into": true, "through": true,
		"during": true, "before": true, "after": true, "above": true, "below": true,
		"to": true, "from": true, "up": true, "down": true, "in": true, "out": true,
		"on": true, "off": true, "over": true, "under": true,
		// Conjunctions
		"and": true, "but": true, "or": true, "nor": true, "if": true, "then": true,
		"else": true, "when": true, "where": true, "how": true, "why": true,
		"what": true, "which": true, "who": true, "whom": true, "this": true,
		"that": true, "these": true, "those": true,
		// Other common words
		"all": true, "each": true, "few": true, "more": true, "most": true,
		"other": true, "some": true, "such": true, "only": true, "own": true,
		"same": true, "so": true, "than": true, "too": true, "very": true,
		"just": true, "now": true, "here": true, "there": true,
	}
	return stopWords
}

// AdvancedQueryNormalizer provides more sophisticated normalization
type AdvancedQueryNormalizer struct {
	*DefaultQueryNormalizer
	synonymMap     map[string]string
	stemmer        func(string) string
	enableStemming bool
	enableSynonyms bool
}

// NewAdvancedQueryNormalizer creates an advanced normalizer
func NewAdvancedQueryNormalizer() *AdvancedQueryNormalizer {
	return &AdvancedQueryNormalizer{
		DefaultQueryNormalizer: &DefaultQueryNormalizer{
			whitespaceRegex:  regexp.MustCompile(`\s+`),
			punctuationRegex: regexp.MustCompile(`[^\w\s-]`),
			enableStopWords:  true,
			preserveNumbers:  true,
			stopWords:        getDefaultStopWords(),
		},
		synonymMap:     getDefaultSynonyms(),
		enableStemming: false, // Disabled by default as it requires external library
		enableSynonyms: true,
	}
}

// Normalize applies advanced normalization
func (n *AdvancedQueryNormalizer) Normalize(query string) string {
	// First apply basic normalization
	normalized := n.DefaultQueryNormalizer.Normalize(query)

	if normalized == "" {
		return ""
	}

	words := strings.Fields(normalized)
	processedWords := make([]string, 0, len(words))

	for _, word := range words {
		// Apply synonyms if enabled
		if n.enableSynonyms {
			if synonym, exists := n.synonymMap[word]; exists {
				word = synonym
			}
		}

		// Apply stemming if enabled and stemmer is available
		if n.enableStemming && n.stemmer != nil {
			word = n.stemmer(word)
		}

		processedWords = append(processedWords, word)
	}

	return strings.Join(processedWords, " ")
}

// getDefaultSynonyms returns common synonyms for normalization
func getDefaultSynonyms() map[string]string {
	return map[string]string{
		// Common tech synonyms
		"javascript": "js",
		"python":     "py",
		"golang":     "go",
		"ruby":       "rb",
		"csharp":     "c#",
		"cpp":        "c++",

		// Common abbreviations
		"application":    "app",
		"database":       "db",
		"configuration":  "config",
		"authentication": "auth",
		"authorization":  "authz",
		"kubernetes":     "k8s",
		"development":    "dev",
		"production":     "prod",
		"environment":    "env",

		// Plural to singular (basic cases)
		"databases":    "database",
		"applications": "app", // Map to abbreviation directly
		"services":     "service",
		"containers":   "container",
		"documents":    "document",
		"queries":      "query",
		"results":      "result",
	}
}
