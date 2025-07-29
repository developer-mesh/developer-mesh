package tokenizer

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimpleTokenizer_CountTokens(t *testing.T) {
	tokenizer := NewSimpleTokenizer(8192)

	tests := []struct {
		name     string
		text     string
		minCount int
		maxCount int
	}{
		{
			name:     "empty text",
			text:     "",
			minCount: 0,
			maxCount: 0,
		},
		{
			name:     "single word",
			text:     "Hello",
			minCount: 1,
			maxCount: 1,
		},
		{
			name:     "simple sentence",
			text:     "This is a simple sentence.",
			minCount: 6,
			maxCount: 8,
		},
		{
			name:     "text with punctuation",
			text:     "Hello, world! How are you?",
			minCount: 6,
			maxCount: 10,
		},
		{
			name:     "text with numbers",
			text:     "The price is $19.99 (plus tax).",
			minCount: 8,
			maxCount: 12,
		},
		{
			name:     "code snippet",
			text:     "func main() { fmt.Println(\"Hello\") }",
			minCount: 9,
			maxCount: 15,
		},
		{
			name:     "text with newlines",
			text:     "First line\nSecond line\n\nThird line",
			minCount: 6,
			maxCount: 10,
		},
		{
			name:     "unicode text",
			text:     "Hello 世界! 🌍",
			minCount: 3,
			maxCount: 5,
		},
		{
			name:     "long text",
			text:     strings.Repeat("This is a test. ", 100),
			minCount: 400,
			maxCount: 600,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			count := tokenizer.CountTokens(tt.text)
			assert.GreaterOrEqual(t, count, tt.minCount,
				"Token count %d should be >= %d for text: %s", count, tt.minCount, tt.text)
			assert.LessOrEqual(t, count, tt.maxCount,
				"Token count %d should be <= %d for text: %s", count, tt.maxCount, tt.text)
		})
	}
}

func TestSimpleTokenizer_Tokenize(t *testing.T) {
	tokenizer := NewSimpleTokenizer(8192)

	tests := []struct {
		name           string
		text           string
		expectedTokens []string
		checkExact     bool
	}{
		{
			name:           "simple words",
			text:           "Hello world",
			expectedTokens: []string{"Hello", "world"},
			checkExact:     true,
		},
		{
			name:           "with punctuation",
			text:           "Hello, world!",
			expectedTokens: []string{"Hello", ",", "world", "!"},
			checkExact:     true,
		},
		{
			name:           "sentence with period",
			text:           "This is a test.",
			expectedTokens: []string{"This", "is", "a", "test", "."},
			checkExact:     true,
		},
		{
			name:           "with newlines",
			text:           "First\nSecond",
			expectedTokens: []string{"First", "\n", "Second"},
			checkExact:     true,
		},
		{
			name:           "empty string",
			text:           "",
			expectedTokens: []string{},
			checkExact:     true,
		},
		{
			name:           "only punctuation",
			text:           "...",
			expectedTokens: []string{".", ".", "."},
			checkExact:     true,
		},
		{
			name:           "mixed content",
			text:           "Test 123, works!",
			expectedTokens: []string{"Test", "123", ",", "works", "!"},
			checkExact:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenizer.Tokenize(tt.text)

			if tt.checkExact {
				assert.Equal(t, tt.expectedTokens, tokens)
			} else {
				assert.Equal(t, len(tt.expectedTokens), len(tokens))
				for i, expected := range tt.expectedTokens {
					assert.Equal(t, expected, tokens[i])
				}
			}
		})
	}
}

func TestSimpleTokenizer_GetTokenLimit(t *testing.T) {
	tests := []struct {
		name     string
		limit    int
		expected int
	}{
		{
			name:     "default limit",
			limit:    0,
			expected: 8192,
		},
		{
			name:     "custom limit",
			limit:    4096,
			expected: 4096,
		},
		{
			name:     "negative limit uses default",
			limit:    -1,
			expected: 8192,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokenizer := NewSimpleTokenizer(tt.limit)
			assert.Equal(t, tt.expected, tokenizer.GetTokenLimit())
		})
	}
}

func TestTikTokenTokenizer_Models(t *testing.T) {
	tests := []struct {
		model         string
		expectedLimit int
	}{
		{"gpt-4", 8192},
		{"gpt-4-32k", 32768},
		{"gpt-3.5-turbo", 4096},
		{"gpt-3.5-turbo-16k", 16384},
		{"text-embedding-3-small", 8191},
		{"text-embedding-3-large", 8191},
		{"unknown-model", 8192}, // default
	}

	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			tokenizer := NewTikTokenTokenizer(tt.model)
			assert.Equal(t, tt.expectedLimit, tokenizer.GetTokenLimit())
			assert.Equal(t, tt.model, tokenizer.model)
		})
	}
}

func TestTikTokenTokenizer_Compatibility(t *testing.T) {
	// Test that TikToken tokenizer works similarly to SimpleTokenizer
	// until we integrate the actual tiktoken library

	simple := NewSimpleTokenizer(8192)
	tiktoken := NewTikTokenTokenizer("gpt-4")

	texts := []string{
		"Simple test",
		"This is a longer sentence with punctuation!",
		"Multiple\nlines\nhere",
		"Code example: func main() { }",
	}

	for _, text := range texts {
		simpleCount := simple.CountTokens(text)
		tiktokenCount := tiktoken.CountTokens(text)

		// They should give the same results since TikToken
		// currently uses SimpleTokenizer internally
		assert.Equal(t, simpleCount, tiktokenCount,
			"Token counts should match for: %s", text)

		simpleTokens := simple.Tokenize(text)
		tiktokenTokens := tiktoken.Tokenize(text)

		assert.Equal(t, simpleTokens, tiktokenTokens,
			"Tokens should match for: %s", text)
	}
}

func BenchmarkSimpleTokenizer_CountTokens(b *testing.B) {
	tokenizer := NewSimpleTokenizer(8192)
	text := strings.Repeat("This is a benchmark test. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenizer.CountTokens(text)
	}
}

func BenchmarkSimpleTokenizer_Tokenize(b *testing.B) {
	tokenizer := NewSimpleTokenizer(8192)
	text := strings.Repeat("This is a benchmark test. ", 100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = tokenizer.Tokenize(text)
	}
}
