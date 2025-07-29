package text

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultSentenceSplitter_Split(t *testing.T) {
	splitter := NewSentenceSplitter()

	tests := []struct {
		name     string
		text     string
		expected []string
	}{
		{
			name: "simple sentences",
			text: "This is the first sentence. This is the second sentence. This is the third.",
			expected: []string{
				"This is the first sentence.",
				"This is the second sentence.",
				"This is the third.",
			},
		},
		{
			name: "mixed punctuation",
			text: "Is this a question? Yes! And this is a statement.",
			expected: []string{
				"Is this a question?",
				"Yes!",
				"And this is a statement.",
			},
		},
		{
			name: "abbreviations",
			text: "Dr. Smith works at Google Inc. He lives in the U.S.A. with his family.",
			expected: []string{
				"Dr. Smith works at Google Inc.",
				"He lives in the U.S.A. with his family.",
			},
		},
		{
			name: "decimal numbers",
			text: "The value of pi is 3.14159. The temperature is 98.6 degrees.",
			expected: []string{
				"The value of pi is 3.14159.",
				"The temperature is 98.6 degrees.",
			},
		},
		{
			name: "ellipsis",
			text: "I was thinking... Maybe we should go. But then again...",
			expected: []string{
				"I was thinking... Maybe we should go.",
				"But then again...",
			},
		},
		{
			name: "quotes and parentheses",
			text: `She said, "Hello there." He replied, "Hi!" (This was unexpected.)`,
			expected: []string{
				`She said, "Hello there."`,
				`He replied, "Hi!"`,
				`(This was unexpected.)`,
			},
		},
		{
			name: "newlines and paragraphs",
			text: "First paragraph sentence one. Sentence two.\n\nSecond paragraph here. With another sentence.",
			expected: []string{
				"First paragraph sentence one.",
				"Sentence two.",
				"Second paragraph here.",
				"With another sentence.",
			},
		},
		{
			name: "technical abbreviations",
			text: "The API uses HTTP. The SDK supports OAuth. Download the .zip file.",
			expected: []string{
				"The API uses HTTP.",
				"The SDK supports OAuth.",
				"Download the .zip file.",
			},
		},
		{
			name: "no sentence endings",
			text: "This text has no sentence endings",
			expected: []string{
				"This text has no sentence endings",
			},
		},
		{
			name:     "empty text",
			text:     "",
			expected: []string{},
		},
		{
			name:     "only whitespace",
			text:     "   \n\t  ",
			expected: []string{},
		},
		{
			name: "single word sentences",
			text: "Yes. No. Maybe.",
			expected: []string{
				"Yes.",
				"No.",
				"Maybe.",
			},
		},
		{
			name: "complex technical text",
			text: "The function calc() returns 3.14. See Fig. 2 for details. The U.S. patent no. 123 was filed.",
			expected: []string{
				"The function calc() returns 3.14.",
				"See Fig. 2 for details.",
				"The U.S. patent no. 123 was filed.",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitter.Split(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDefaultSentenceSplitter_EdgeCases(t *testing.T) {
	splitter := NewSentenceSplitter()

	tests := []struct {
		name     string
		text     string
		minSents int
		maxSents int
	}{
		{
			name:     "multiple spaces",
			text:     "First sentence.     Second sentence.    Third one.",
			minSents: 3,
			maxSents: 3,
		},
		{
			name:     "mixed line endings",
			text:     "First.\nSecond.\r\nThird.",
			minSents: 3,
			maxSents: 3,
		},
		{
			name:     "urls and emails",
			text:     "Visit https://example.com. Email us at info@example.com.",
			minSents: 2,
			maxSents: 2,
		},
		{
			name:     "code snippets",
			text:     "The code is: if (x > 0) { return true; }. It works well.",
			minSents: 2,
			maxSents: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitter.Split(tt.text)
			assert.GreaterOrEqual(t, len(result), tt.minSents)
			assert.LessOrEqual(t, len(result), tt.maxSents)

			// Verify no empty sentences
			for _, sent := range result {
				assert.NotEmpty(t, strings.TrimSpace(sent))
			}
		})
	}
}

func TestDefaultSentenceSplitter_RealWorldText(t *testing.T) {
	splitter := NewSentenceSplitter()

	// Test with a realistic paragraph
	text := `The quick brown fox jumps over the lazy dog. This pangram sentence contains every letter of the English alphabet at least once. Did you know that? It's commonly used for testing fonts and keyboards! In fact, many typographers use it regularly. However, there are other pangrams too... "Pack my box with five dozen liquor jugs" is another example.`

	sentences := splitter.Split(text)

	// Should split into reasonable sentences
	assert.Greater(t, len(sentences), 4)
	assert.Less(t, len(sentences), 10)

	// Check that each sentence is properly formed
	for _, sent := range sentences {
		assert.NotEmpty(t, sent)
		assert.True(t, strings.HasSuffix(sent, ".") ||
			strings.HasSuffix(sent, "!") ||
			strings.HasSuffix(sent, "?") ||
			strings.HasSuffix(sent, `"`))
	}

	// Specific checks
	assert.Contains(t, sentences[0], "quick brown fox")
	assert.Contains(t, sentences[1], "pangram")
}

func BenchmarkSentenceSplitter(b *testing.B) {
	splitter := NewSentenceSplitter()

	// Create a large text
	var sb strings.Builder
	for i := 0; i < 100; i++ {
		sb.WriteString("This is sentence number ")
		sb.WriteString(string(rune('0' + i%10)))
		sb.WriteString(". It contains some text. ")
		if i%5 == 0 {
			sb.WriteString("Is this a question? Yes! ")
		}
		if i%10 == 0 {
			sb.WriteString("\n\n")
		}
	}
	text := sb.String()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = splitter.Split(text)
	}
}
