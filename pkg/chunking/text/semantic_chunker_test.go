package text

import (
	"context"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/tokenizer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSemanticChunker_Chunk(t *testing.T) {
	ctx := context.Background()
	tok := tokenizer.NewSimpleTokenizer(8192)

	tests := []struct {
		name           string
		text           string
		config         *Config
		expectedChunks int
		checkContent   bool
	}{
		{
			name: "simple text with paragraphs",
			text: `This is the first paragraph. It contains multiple sentences. Each sentence adds to the content.

This is the second paragraph. It also has multiple sentences. The chunker should recognize the paragraph boundary.

This is the third paragraph. It continues the document. The semantic boundaries should be respected.`,
			config: &Config{
				MinChunkSize:    20,
				MaxChunkSize:    100,
				TargetChunkSize: 50,
				OverlapSize:     10,
			},
			expectedChunks: 3,
			checkContent:   true,
		},
		{
			name: "text with topic shifts",
			text: `Introduction to the topic. This establishes the context.

However, there is another perspective to consider. This shifts the discussion.

Furthermore, additional points need to be made. This adds new information.

In conclusion, we can summarize the main points. This wraps up the discussion.`,
			config: &Config{
				MinChunkSize:    20,
				MaxChunkSize:    100,
				TargetChunkSize: 50,
				OverlapSize:     5,
			},
			expectedChunks: 4,
		},
		{
			name: "text with numbered list",
			text: `Here are the main points to consider:

1. First point with detailed explanation.
2. Second point with more information.
3. Third point concluding the list.

After the list, we continue with regular text.`,
			config: &Config{
				MinChunkSize:    15,
				MaxChunkSize:    80,
				TargetChunkSize: 40,
				OverlapSize:     0,
			},
			expectedChunks: 2,
		},
		{
			name: "short text below minimum",
			text: "This is too short.",
			config: &Config{
				MinChunkSize:    50,
				MaxChunkSize:    100,
				TargetChunkSize: 75,
			},
			expectedChunks: 0,
		},
		{
			name: "text requiring forced split",
			text: strings.Repeat("This is a very long sentence that goes on and on without any good breaking points. ", 20),
			config: &Config{
				MinChunkSize:    50,
				MaxChunkSize:    200,
				TargetChunkSize: 100,
				OverlapSize:     20,
			},
			expectedChunks: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewSemanticChunker(tok, tt.config)
			chunks, err := chunker.Chunk(ctx, tt.text, nil)

			require.NoError(t, err)
			assert.Len(t, chunks, tt.expectedChunks)

			if tt.expectedChunks > 0 {
				// Verify chunk properties
				for i, chunk := range chunks {
					assert.Equal(t, i, chunk.Index)
					assert.NotEmpty(t, chunk.Content)
					assert.Greater(t, chunk.TokenCount, 0)
					assert.NotNil(t, chunk.Metadata)
					assert.Equal(t, "semantic", chunk.Metadata["chunking_method"])

					// Check chunk size constraints
					if i < len(chunks)-1 { // Not the last chunk
						assert.GreaterOrEqual(t, chunk.TokenCount, tt.config.MinChunkSize)
					}
					assert.LessOrEqual(t, chunk.TokenCount, tt.config.MaxChunkSize)
				}

				// Check overlap
				if tt.config.OverlapSize > 0 && len(chunks) > 1 {
					for i := 1; i < len(chunks); i++ {
						prevEnd := chunks[i-1].Content[len(chunks[i-1].Content)-20:]
						currStart := chunks[i].Content[:20]
						// There should be some overlap visible
						if tt.checkContent {
							t.Logf("Chunk %d end: ...%s", i-1, prevEnd)
							t.Logf("Chunk %d start: %s...", i, currStart)
						}
					}
				}
			}
		})
	}
}

func TestSemanticChunker_isSemanticBoundary(t *testing.T) {
	chunker := &SemanticChunker{
		sentenceSplitter: NewSentenceSplitter(),
	}

	tests := []struct {
		name      string
		sentences []string
		index     int
		expected  bool
	}{
		{
			name: "paragraph boundary",
			sentences: []string{
				"First sentence.\n\n",
				"Second sentence.",
			},
			index:    0,
			expected: true,
		},
		{
			name: "topic shift with however",
			sentences: []string{
				"First point.",
				"However, there is another view.",
			},
			index:    0,
			expected: true,
		},
		{
			name: "numbered list start",
			sentences: []string{
				"Introduction.",
				"1. First item",
			},
			index:    0,
			expected: true,
		},
		{
			name: "no boundary",
			sentences: []string{
				"First sentence.",
				"Second sentence continues the thought.",
			},
			index:    0,
			expected: false,
		},
		{
			name: "last sentence",
			sentences: []string{
				"First.",
				"Last.",
			},
			index:    1,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := chunker.isSemanticBoundary(tt.sentences, tt.index)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSemanticChunker_getOverlapText(t *testing.T) {
	tok := tokenizer.NewSimpleTokenizer(8192)
	chunker := &SemanticChunker{
		tokenizer:        tok,
		sentenceSplitter: NewSentenceSplitter(),
	}

	tests := []struct {
		name          string
		content       string
		overlapTokens int
		expectedWords int // Approximate
	}{
		{
			name:          "simple overlap",
			content:       "First sentence. Second sentence. Third sentence. Fourth sentence.",
			overlapTokens: 10,
			expectedWords: 2, // Should get roughly "Fourth sentence."
		},
		{
			name:          "no overlap requested",
			content:       "Some content here.",
			overlapTokens: 0,
			expectedWords: 0,
		},
		{
			name:          "overlap larger than content",
			content:       "Short text.",
			overlapTokens: 100,
			expectedWords: 2, // Should get all content
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			overlap := chunker.getOverlapText(tt.content, tt.overlapTokens)

			if tt.expectedWords == 0 {
				assert.Empty(t, overlap)
			} else {
				assert.NotEmpty(t, overlap)
				words := strings.Fields(overlap)
				assert.GreaterOrEqual(t, len(words), tt.expectedWords-1)
				assert.LessOrEqual(t, len(words), tt.expectedWords+3)
			}
		})
	}
}

func TestSemanticChunker_Integration(t *testing.T) {
	ctx := context.Background()
	tok := tokenizer.NewSimpleTokenizer(8192)

	// Test with a realistic document
	document := `# Introduction to Machine Learning

Machine learning is a subset of artificial intelligence that focuses on the development of algorithms and statistical models. These systems can learn from and make predictions or decisions based on data. The field has grown tremendously in recent years.

## Types of Machine Learning

There are three main types of machine learning:

1. Supervised Learning: The algorithm learns from labeled training data.
2. Unsupervised Learning: The algorithm finds patterns in unlabeled data.
3. Reinforcement Learning: The algorithm learns through interaction with an environment.

### Supervised Learning Details

Supervised learning is the most common type of machine learning. In this approach, the algorithm is trained on a labeled dataset. Each training example consists of an input and the desired output.

However, supervised learning has its limitations. It requires large amounts of labeled data, which can be expensive and time-consuming to obtain. Additionally, the model's performance is limited by the quality of the training data.

## Deep Learning

Deep learning is a subset of machine learning based on artificial neural networks. These networks are inspired by the structure and function of the human brain. Deep learning has revolutionized many fields including computer vision and natural language processing.

In conclusion, machine learning continues to evolve and impact various industries. Understanding its fundamentals is crucial for anyone working in technology today.`

	config := &Config{
		MinChunkSize:    100,
		MaxChunkSize:    500,
		TargetChunkSize: 300,
		OverlapSize:     50,
	}

	chunker := NewSemanticChunker(tok, config)
	metadata := map[string]interface{}{
		"source": "test_document",
		"type":   "markdown",
	}

	chunks, err := chunker.Chunk(ctx, document, metadata)
	require.NoError(t, err)

	// Should create multiple chunks
	assert.Greater(t, len(chunks), 3)

	// Verify all content is captured
	var reconstructed strings.Builder
	for i, chunk := range chunks {
		if i == 0 {
			reconstructed.WriteString(chunk.Content)
		} else {
			// Account for overlap by finding where new content starts
			overlap := findOverlap(chunks[i-1].Content, chunk.Content)
			if overlap > 0 {
				reconstructed.WriteString(chunk.Content[overlap:])
			} else {
				reconstructed.WriteString(" ")
				reconstructed.WriteString(chunk.Content)
			}
		}
	}

	// The reconstructed text should contain all the important content
	// (might have minor differences in spacing)
	assert.Contains(t, reconstructed.String(), "Introduction to Machine Learning")
	assert.Contains(t, reconstructed.String(), "Types of Machine Learning")
	assert.Contains(t, reconstructed.String(), "Deep Learning")
	assert.Contains(t, reconstructed.String(), "In conclusion")

	// Check metadata propagation
	for _, chunk := range chunks {
		assert.Equal(t, "test_document", chunk.Metadata["source"])
		assert.Equal(t, "markdown", chunk.Metadata["type"])
	}
}

// findOverlap finds the overlap between the end of s1 and start of s2
func findOverlap(s1, s2 string) int {
	maxOverlap := len(s1)
	if len(s2) < maxOverlap {
		maxOverlap = len(s2)
	}

	for overlap := maxOverlap; overlap > 0; overlap-- {
		if strings.HasSuffix(s1, s2[:overlap]) {
			return overlap
		}
	}

	return 0
}
