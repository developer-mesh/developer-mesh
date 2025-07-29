package text

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecursiveCharacterSplitter_Chunk(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name         string
		text         string
		config       *RecursiveCharacterSplitterConfig
		minChunks    int
		maxChunks    int
		checkOverlap bool
	}{
		{
			name: "simple paragraph splitting",
			text: `First paragraph here.

Second paragraph here.

Third paragraph here.`,
			config: &RecursiveCharacterSplitterConfig{
				ChunkSize:    30,
				ChunkOverlap: 5,
			},
			minChunks: 3,
			maxChunks: 3,
		},
		{
			name: "single line splitting",
			text: "This is a long sentence that needs to be split. It continues with more content. And even more content here.",
			config: &RecursiveCharacterSplitterConfig{
				ChunkSize:    40,
				ChunkOverlap: 10,
			},
			minChunks:    2,
			maxChunks:    4,
			checkOverlap: true,
		},
		{
			name: "custom separators",
			text: "Part one;Part two;Part three;Part four",
			config: &RecursiveCharacterSplitterConfig{
				Separators:   []string{";", " ", ""},
				ChunkSize:    15,
				ChunkOverlap: 0,
			},
			minChunks: 3,
			maxChunks: 4,
		},
		{
			name: "very long text without good splits",
			text: strings.Repeat("verylongwordwithoutspaces", 10),
			config: &RecursiveCharacterSplitterConfig{
				ChunkSize:    50,
				ChunkOverlap: 10,
			},
			minChunks: 4,
			maxChunks: 6,
		},
		{
			name: "empty text",
			text: "",
			config: &RecursiveCharacterSplitterConfig{
				ChunkSize: 100,
			},
			minChunks: 0,
			maxChunks: 0,
		},
		{
			name: "text with multiple separator types",
			text: `Title: Introduction

This is the first section. It has multiple sentences. Each one is important.

However, this is a new section. It contains different information. The content continues here.

* First bullet point
* Second bullet point
* Third bullet point

In conclusion, this was a good example.`,
			config: &RecursiveCharacterSplitterConfig{
				ChunkSize:     80,
				ChunkOverlap:  20,
				KeepSeparator: true,
			},
			minChunks: 3,
			maxChunks: 6,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter := NewRecursiveCharacterSplitter(tt.config)
			chunks, err := splitter.Chunk(ctx, tt.text, nil)

			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(chunks), tt.minChunks)
			assert.LessOrEqual(t, len(chunks), tt.maxChunks)

			// Verify chunk properties
			for i, chunk := range chunks {
				assert.Equal(t, i, chunk.Index)
				assert.NotEmpty(t, chunk.Content)
				assert.Equal(t, len(chunk.Content), chunk.TokenCount) // Using default length function
				assert.NotNil(t, chunk.Metadata)
				assert.Equal(t, "recursive_character", chunk.Metadata["chunking_method"])
			}

			// Check overlap if requested
			if tt.checkOverlap && tt.config.ChunkOverlap > 0 && len(chunks) > 1 {
				for i := 1; i < len(chunks); i++ {
					// There should be some content overlap
					prevEnd := chunks[i-1].Content
					currStart := chunks[i].Content

					// Find if there's any overlap
					foundOverlap := false
					overlapLen := tt.config.ChunkOverlap
					if overlapLen > len(prevEnd) {
						overlapLen = len(prevEnd)
					}

					for j := 1; j <= overlapLen && j <= len(prevEnd); j++ {
						if strings.HasPrefix(currStart, prevEnd[len(prevEnd)-j:]) {
							foundOverlap = true
							break
						}
					}

					if tt.config.ChunkOverlap > 0 {
						assert.True(t, foundOverlap || len(prevEnd) < tt.config.ChunkOverlap,
							"Expected overlap between chunks %d and %d", i-1, i)
					}
				}
			}
		})
	}
}

func TestRecursiveCharacterSplitter_CustomLengthFunction(t *testing.T) {
	ctx := context.Background()

	// Custom length function that counts words instead of characters
	wordCounter := func(s string) int {
		return len(strings.Fields(s))
	}

	config := &RecursiveCharacterSplitterConfig{
		ChunkSize:      10, // 10 words
		ChunkOverlap:   2,  // 2 words
		LengthFunction: wordCounter,
	}

	text := "This is a test document with multiple words. It should be split based on word count not character count. Each chunk should contain approximately ten words."

	splitter := NewRecursiveCharacterSplitter(config)
	chunks, err := splitter.Chunk(ctx, text, nil)

	require.NoError(t, err)
	assert.Greater(t, len(chunks), 1)

	// Verify chunks are split by word count
	for _, chunk := range chunks {
		wordCount := len(strings.Fields(chunk.Content))
		assert.LessOrEqual(t, wordCount, 12) // Allow some overflow
	}
}

func TestRecursiveCharacterSplitter_KeepSeparator(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name          string
		text          string
		keepSeparator bool
		separator     string
		checkFor      string
	}{
		{
			name:          "keep newlines",
			text:          "Line one\nLine two\nLine three",
			keepSeparator: true,
			separator:     "\n",
			checkFor:      "\n",
		},
		{
			name:          "don't keep newlines",
			text:          "Line one\nLine two\nLine three",
			keepSeparator: false,
			separator:     "\n",
			checkFor:      "\n",
		},
		{
			name:          "keep periods",
			text:          "Sentence one. Sentence two. Sentence three.",
			keepSeparator: true,
			separator:     ". ",
			checkFor:      ".",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &RecursiveCharacterSplitterConfig{
				Separators:    []string{tt.separator},
				ChunkSize:     20,
				ChunkOverlap:  0,
				KeepSeparator: tt.keepSeparator,
			}

			splitter := NewRecursiveCharacterSplitter(config)
			chunks, err := splitter.Chunk(ctx, tt.text, nil)

			require.NoError(t, err)
			assert.Greater(t, len(chunks), 1)

			// Check if separator is kept or removed
			for i, chunk := range chunks {
				if tt.keepSeparator && i < len(chunks)-1 {
					assert.Contains(t, chunk.Content, tt.checkFor)
				} else if !tt.keepSeparator {
					// Separator might still appear in last chunk
					if i < len(chunks)-1 {
						assert.NotContains(t, chunk.Content, tt.checkFor)
					}
				}
			}
		})
	}
}

func TestRecursiveCharacterSplitter_RealDocument(t *testing.T) {
	ctx := context.Background()

	document := `# Project Documentation

## Overview

This project implements a sophisticated text chunking system. The system is designed to handle various types of text content efficiently.

### Features

The main features include:

1. Semantic chunking - intelligently splits text at meaningful boundaries
2. Recursive splitting - uses multiple separators hierarchically  
3. Configurable overlap - maintains context between chunks
4. Custom tokenization - supports different counting methods

### Implementation Details

The implementation uses a recursive approach. First, it attempts to split on major boundaries like paragraphs. If chunks are still too large, it progressively uses smaller separators.

This ensures that the text is split in the most natural way possible while respecting size constraints.

## Usage

To use the chunking system:

1. Create a chunker instance
2. Configure the parameters
3. Call the chunk method

## Conclusion

This system provides flexible and efficient text chunking for various NLP applications.`

	config := &RecursiveCharacterSplitterConfig{
		ChunkSize:     200,
		ChunkOverlap:  40,
		KeepSeparator: true,
	}

	splitter := NewRecursiveCharacterSplitter(config)
	chunks, err := splitter.Chunk(ctx, document, map[string]interface{}{
		"document_type": "markdown",
		"source":        "test",
	})

	require.NoError(t, err)
	assert.Greater(t, len(chunks), 5) // Should create multiple chunks

	// Verify all content is preserved
	var reconstructed strings.Builder
	previousChunk := ""

	for i, chunk := range chunks {
		if i == 0 {
			reconstructed.WriteString(chunk.Content)
			previousChunk = chunk.Content
		} else {
			// Find where the overlap ends
			overlap := 0
			minLen := len(previousChunk)
			if config.ChunkOverlap < minLen {
				minLen = config.ChunkOverlap * 2 // Check a bit more than overlap size
			}

			for j := minLen; j > 0; j-- {
				if len(previousChunk) >= j && strings.HasSuffix(previousChunk, chunk.Content[:min(j, len(chunk.Content))]) {
					overlap = j
					break
				}
			}

			if overlap > 0 && overlap < len(chunk.Content) {
				reconstructed.WriteString(chunk.Content[overlap:])
			} else if overlap == 0 {
				reconstructed.WriteString(chunk.Content)
			}

			previousChunk = chunk.Content
		}
	}

	// Check that important content is preserved
	reconstructedText := reconstructed.String()
	assert.Contains(t, reconstructedText, "Project Documentation")
	assert.Contains(t, reconstructedText, "Features")
	assert.Contains(t, reconstructedText, "Implementation Details")
	assert.Contains(t, reconstructedText, "Conclusion")

	// Verify metadata propagation
	for _, chunk := range chunks {
		assert.Equal(t, "markdown", chunk.Metadata["document_type"])
		assert.Equal(t, "test", chunk.Metadata["source"])
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
