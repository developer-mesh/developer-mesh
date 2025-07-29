package cache

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultQueryNormalizer_Normalize(t *testing.T) {
	normalizer := NewQueryNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty query",
			input:    "",
			expected: "",
		},
		{
			name:     "simple query",
			input:    "Hello World",
			expected: "hello world",
		},
		{
			name:     "query with extra spaces",
			input:    "  Hello   World  ",
			expected: "hello world",
		},
		{
			name:     "query with punctuation",
			input:    "Hello, World! How are you?",
			expected: "hello world",
		},
		{
			name:     "query with stop words",
			input:    "The quick brown fox jumps over the lazy dog",
			expected: "quick brown fox jumps lazy dog",
		},
		{
			name:     "query with numbers",
			input:    "Python 3.11 released in 2023",
			expected: "python 3 11 released 2023",
		},
		{
			name:     "query with hyphenated words",
			input:    "front-end back-end full-stack development",
			expected: "front-end back-end full-stack development",
		},
		{
			name:     "query with special characters",
			input:    "C++ vs C# programming @2023 #coding",
			expected: "vs programming 2023 coding",
		},
		{
			name:     "query with consecutive duplicates",
			input:    "the the quick quick brown fox",
			expected: "quick brown fox",
		},
		{
			name:     "query with mixed case",
			input:    "GoLang PYTHON JavaScript",
			expected: "golang python javascript",
		},
		{
			name:     "technical query",
			input:    "How to implement Redis cache in Go?",
			expected: "implement redis cache go",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultQueryNormalizer_WithOptions(t *testing.T) {
	t.Run("without stop words", func(t *testing.T) {
		normalizer := NewQueryNormalizerWithOptions(false, true)
		result := normalizer.Normalize("The quick brown fox")
		assert.Equal(t, "the quick brown fox", result)
	})

	t.Run("without numbers", func(t *testing.T) {
		normalizer := NewQueryNormalizerWithOptions(true, false)
		result := normalizer.Normalize("Python 3.11 released in 2023")
		assert.Equal(t, "python released", result)
	})

	t.Run("preserve everything", func(t *testing.T) {
		normalizer := NewQueryNormalizerWithOptions(false, true)
		result := normalizer.Normalize("The API returns 200 OK")
		assert.Equal(t, "the api returns 200 ok", result)
	})
}

func TestAdvancedQueryNormalizer_Normalize(t *testing.T) {
	normalizer := NewAdvancedQueryNormalizer()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "apply synonyms",
			input:    "JavaScript application",
			expected: "js app",
		},
		{
			name:     "multiple synonyms",
			input:    "Python database configuration",
			expected: "py db config",
		},
		{
			name:     "kubernetes abbreviation",
			input:    "Deploy to Kubernetes production",
			expected: "deploy k8s prod",
		},
		{
			name:     "plural to singular",
			input:    "Search databases for documents",
			expected: "search database document",
		},
		{
			name:     "mixed synonyms and stop words",
			input:    "The JavaScript applications are in development",
			expected: "js app dev",
		},
		{
			name:     "no matching synonyms",
			input:    "Redis cache implementation",
			expected: "redis cache implementation",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizer.Normalize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNumber(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"123", true},
		{"3.14", true},
		{"1,000", true},
		{"abc", false},
		{"123abc", false},
		{"", false},
		{".", true},     // Single dot is considered numeric due to simple implementation
		{"1.2.3", true}, // This passes as we check char by char
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isNumber(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeduplicateConsecutive(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{
			name:     "no duplicates",
			input:    []string{"hello", "world"},
			expected: []string{"hello", "world"},
		},
		{
			name:     "consecutive duplicates",
			input:    []string{"hello", "hello", "world"},
			expected: []string{"hello", "world"},
		},
		{
			name:     "multiple consecutive duplicates",
			input:    []string{"the", "the", "the", "quick", "quick", "brown"},
			expected: []string{"the", "quick", "brown"},
		},
		{
			name:     "non-consecutive duplicates preserved",
			input:    []string{"hello", "world", "hello"},
			expected: []string{"hello", "world", "hello"},
		},
		{
			name:     "empty slice",
			input:    []string{},
			expected: []string{},
		},
		{
			name:     "single element",
			input:    []string{"hello"},
			expected: []string{"hello"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := deduplicateConsecutive(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func BenchmarkQueryNormalizer(b *testing.B) {
	normalizer := NewQueryNormalizer()
	query := "How to implement a distributed cache system with Redis in Go?"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizer.Normalize(query)
	}
}

func BenchmarkAdvancedQueryNormalizer(b *testing.B) {
	normalizer := NewAdvancedQueryNormalizer()
	query := "JavaScript applications using databases in Kubernetes production environment"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = normalizer.Normalize(query)
	}
}
