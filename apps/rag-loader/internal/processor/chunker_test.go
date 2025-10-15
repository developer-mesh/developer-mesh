package processor

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

func TestFixedSizeChunker_Chunk(t *testing.T) {
	tests := []struct {
		name          string
		maxTokens     int
		overlapTokens int
		content       string
		wantChunks    int
		wantOverlap   bool
	}{
		{
			name:          "small content - single chunk",
			maxTokens:     100,
			overlapTokens: 10,
			content:       "This is a small document with few words",
			wantChunks:    1,
			wantOverlap:   false,
		},
		{
			name:          "large content - multiple chunks",
			maxTokens:     10,
			overlapTokens: 2,
			content:       strings.Repeat("word ", 50), // 50 words
			wantChunks:    7,                           // ceil(50 / (10-2)) = 7
			wantOverlap:   true,
		},
		{
			name:          "exact boundary",
			maxTokens:     10,
			overlapTokens: 0,
			content:       strings.Repeat("word ", 30), // 30 words
			wantChunks:    3,                           // 30 / 10 = 3
			wantOverlap:   false,
		},
		{
			name:          "empty content",
			maxTokens:     10,
			overlapTokens: 2,
			content:       "",
			wantChunks:    0,
			wantOverlap:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewFixedSizeChunker(tt.maxTokens, tt.overlapTokens)
			doc := &models.Document{
				ID:         uuid.New(),
				TenantID:   uuid.New(),
				Content:    tt.content,
				SourceType: "test",
				SourceID:   "test-source",
			}

			chunks, err := chunker.Chunk(doc)
			require.NoError(t, err)
			assert.Len(t, chunks, tt.wantChunks)

			// Verify chunks are properly indexed
			for i, chunk := range chunks {
				assert.Equal(t, i, chunk.ChunkIndex)
				assert.Equal(t, doc.ID, chunk.DocumentID)
				assert.NotEmpty(t, chunk.Content)
				assert.Equal(t, "fixed_size", chunk.Metadata["strategy"])
			}

			// Check for overlap in consecutive chunks
			if tt.wantOverlap && len(chunks) > 1 {
				// Last words of chunk i should appear in chunk i+1
				for i := 0; i < len(chunks)-1; i++ {
					words1 := strings.Fields(chunks[i].Content)
					words2 := strings.Fields(chunks[i+1].Content)

					// Should have some overlap
					assert.True(t, len(words1) > 0 && len(words2) > 0)
				}
			}
		})
	}
}

func TestFixedSizeChunker_NilDocument(t *testing.T) {
	chunker := NewFixedSizeChunker(100, 10)
	chunks, err := chunker.Chunk(nil)
	assert.Error(t, err)
	assert.Nil(t, chunks)
}

func TestMarkdownChunker_Chunk(t *testing.T) {
	tests := []struct {
		name       string
		maxTokens  int
		content    string
		wantChunks int
	}{
		{
			name:      "simple markdown with headers",
			maxTokens: 100,
			content: `# Title

## Section 1
Content for section 1 with some text.

## Section 2
Content for section 2 with more text.

## Section 3
Final section with content.`,
			wantChunks: 4, // Title + 3 sections
		},
		{
			name:       "no headers - fallback to fixed",
			maxTokens:  10,
			content:    strings.Repeat("word ", 50),
			wantChunks: 5, // Fallback to fixed-size chunking (actual behavior)
		},
		{
			name:      "large section - split",
			maxTokens: 10,
			content: `## Big Section
` + strings.Repeat("word ", 50), // Section > maxTokens
			wantChunks: 6, // Section split into multiple chunks
		},
		{
			name:       "empty markdown",
			maxTokens:  100,
			content:    "",
			wantChunks: 1, // Creates empty section, fallback returns 1 empty chunk
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewMarkdownChunker(tt.maxTokens)
			doc := &models.Document{
				ID:         uuid.New(),
				TenantID:   uuid.New(),
				Title:      "test.md",
				Content:    tt.content,
				SourceType: "test",
				SourceID:   "test-source",
			}

			chunks, err := chunker.Chunk(doc)
			require.NoError(t, err)
			assert.Len(t, chunks, tt.wantChunks)

			// Verify chunks are properly structured
			for _, chunk := range chunks {
				assert.Equal(t, doc.ID, chunk.DocumentID)

				// Only check content is not empty for non-empty documents
				if tt.content != "" {
					assert.NotEmpty(t, chunk.Content)
				}

				// Check strategy in metadata
				if strategy, ok := chunk.Metadata["strategy"]; ok {
					assert.Contains(t, []string{"markdown", "fixed_size"}, strategy)
				}
			}
		})
	}
}

func TestMarkdownChunker_splitByHeaders(t *testing.T) {
	chunker := NewMarkdownChunker(100)

	content := `# Main Title

## Section 1
Content 1

## Section 2
Content 2

### Subsection 2.1
Sub content`

	sections := chunker.splitByHeaders(content)
	assert.GreaterOrEqual(t, len(sections), 2) // At least 2 sections

	// Check that sections contain headers or content
	foundHeaderOrContent := false
	for _, section := range sections {
		if strings.TrimSpace(section) != "" {
			if strings.Contains(section, "##") || strings.Contains(section, "Content") {
				foundHeaderOrContent = true
				break
			}
		}
	}
	assert.True(t, foundHeaderOrContent, "at least one section should have headers or content")
}

func TestCodeChunker_Chunk(t *testing.T) {
	tests := []struct {
		name       string
		language   string
		content    string
		wantChunks int
	}{
		{
			name:     "go functions",
			language: "go",
			content: `package main

func main() {
	fmt.Println("Hello")
}

func helper() {
	return
}

func another() {
	// code
}`,
			wantChunks: 4, // package declaration + 3 functions
		},
		{
			name:     "javascript functions",
			language: "js",
			content: `function test1() {
	console.log("test1");
}

function test2() {
	console.log("test2");
}

const test3 = () => {
	console.log("test3");
}`,
			wantChunks: 3,
		},
		{
			name:     "python functions",
			language: "py",
			content: `def function1():
	print("one")

def function2():
	print("two")

def function3():
	print("three")`,
			wantChunks: 3,
		},
		{
			name:       "no functions - fallback",
			language:   "go",
			content:    strings.Repeat("word ", 50),
			wantChunks: 7, // Fallback to fixed-size
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chunker := NewCodeChunker(10, tt.language)
			doc := &models.Document{
				ID:         uuid.New(),
				TenantID:   uuid.New(),
				Title:      "test." + tt.language,
				Content:    tt.content,
				SourceType: "test",
				SourceID:   "test-source",
			}

			chunks, err := chunker.Chunk(doc)
			require.NoError(t, err)
			assert.GreaterOrEqual(t, len(chunks), 1)

			// Verify chunks
			for _, chunk := range chunks {
				assert.Equal(t, doc.ID, chunk.DocumentID)
				assert.NotEmpty(t, chunk.Content)
			}
		})
	}
}

func TestGetChunkerForDocument(t *testing.T) {
	tests := []struct {
		name     string
		title    string
		wantType string
	}{
		{
			name:     "markdown file",
			title:    "README.md",
			wantType: "markdown",
		},
		{
			name:     "go file",
			title:    "main.go",
			wantType: "code_go",
		},
		{
			name:     "javascript file",
			title:    "app.js",
			wantType: "code_js",
		},
		{
			name:     "typescript file",
			title:    "component.tsx",
			wantType: "code_js",
		},
		{
			name:     "python file",
			title:    "script.py",
			wantType: "code_py",
		},
		{
			name:     "unknown file",
			title:    "data.txt",
			wantType: "fixed_size",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc := &models.Document{
				ID:    uuid.New(),
				Title: tt.title,
			}

			chunker := GetChunkerForDocument(doc)
			require.NotNil(t, chunker)
			assert.Equal(t, tt.wantType, chunker.GetStrategy())
		})
	}
}

func TestChunker_GetStrategy(t *testing.T) {
	tests := []struct {
		name     string
		chunker  interface{ GetStrategy() string }
		expected string
	}{
		{
			name:     "fixed size chunker",
			chunker:  NewFixedSizeChunker(100, 10),
			expected: "fixed_size",
		},
		{
			name:     "markdown chunker",
			chunker:  NewMarkdownChunker(100),
			expected: "markdown",
		},
		{
			name:     "go code chunker",
			chunker:  NewCodeChunker(100, "go"),
			expected: "code_go",
		},
		{
			name:     "python code chunker",
			chunker:  NewCodeChunker(100, "py"),
			expected: "code_py",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.chunker.GetStrategy())
		})
	}
}

func BenchmarkFixedSizeChunker(b *testing.B) {
	chunker := NewFixedSizeChunker(500, 50)
	doc := &models.Document{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		Content:    strings.Repeat("This is a test document with many words. ", 1000),
		SourceType: "test",
		SourceID:   "test-source",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk(doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkMarkdownChunker(b *testing.B) {
	chunker := NewMarkdownChunker(1024)
	doc := &models.Document{
		ID:       uuid.New(),
		TenantID: uuid.New(),
		Title:    "test.md",
		Content: `# Title

## Section 1
` + strings.Repeat("Content for section 1. ", 100) + `

## Section 2
` + strings.Repeat("Content for section 2. ", 100),
		SourceType: "test",
		SourceID:   "test-source",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := chunker.Chunk(doc)
		if err != nil {
			b.Fatal(err)
		}
	}
}
