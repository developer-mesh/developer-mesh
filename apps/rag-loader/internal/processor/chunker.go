// Package processor provides document processing capabilities including chunking
package processor

import (
	"fmt"
	"strings"

	"github.com/google/uuid"

	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// FixedSizeChunker implements fixed-size chunking with overlap
type FixedSizeChunker struct {
	MaxTokens     int // Maximum tokens per chunk (approx 4 chars per token)
	OverlapTokens int // Number of tokens to overlap between chunks
}

// NewFixedSizeChunker creates a new fixed-size chunker
func NewFixedSizeChunker(maxTokens, overlapTokens int) *FixedSizeChunker {
	return &FixedSizeChunker{
		MaxTokens:     maxTokens,
		OverlapTokens: overlapTokens,
	}
}

// Chunk splits a document into fixed-size chunks with overlap
func (f *FixedSizeChunker) Chunk(document *models.Document) ([]*models.Chunk, error) {
	if document == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	var chunks []*models.Chunk

	// Split content into words
	words := strings.Fields(document.Content)
	if len(words) == 0 {
		return chunks, nil
	}

	chunkIndex := 0
	startChar := 0

	for i := 0; i < len(words); {
		// Calculate chunk size
		endIdx := i + f.MaxTokens
		if endIdx > len(words) {
			endIdx = len(words)
		}

		// Extract chunk words
		chunkWords := words[i:endIdx]
		chunkContent := strings.Join(chunkWords, " ")

		// Calculate character positions (approximate)
		endChar := startChar + len(chunkContent)

		chunk := &models.Chunk{
			ID:          uuid.New(),
			DocumentID:  document.ID,
			ChunkIndex:  chunkIndex,
			Content:     chunkContent,
			StartChar:   startChar,
			EndChar:     endChar,
			EmbeddingID: nil,
			Metadata: map[string]interface{}{
				"strategy":    "fixed_size",
				"max_tokens":  f.MaxTokens,
				"overlap":     f.OverlapTokens,
				"word_count":  len(chunkWords),
				"source_type": document.SourceType,
				"source_id":   document.SourceID,
			},
		}

		chunks = append(chunks, chunk)

		// Move forward with overlap
		step := f.MaxTokens - f.OverlapTokens
		if step <= 0 {
			step = 1 // Ensure progress
		}
		i += step
		chunkIndex++
		startChar = endChar + 1 // Account for space
	}

	return chunks, nil
}

// GetStrategy returns the name of the chunking strategy
func (f *FixedSizeChunker) GetStrategy() string {
	return "fixed_size"
}

// MarkdownChunker implements markdown-aware chunking that respects headers
type MarkdownChunker struct {
	MaxTokens int // Maximum tokens per chunk
}

// NewMarkdownChunker creates a new markdown-aware chunker
func NewMarkdownChunker(maxTokens int) *MarkdownChunker {
	return &MarkdownChunker{
		MaxTokens: maxTokens,
	}
}

// Chunk splits a markdown document by headers and size
func (m *MarkdownChunker) Chunk(document *models.Document) ([]*models.Chunk, error) {
	if document == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	var chunks []*models.Chunk

	// Split by headers (##)
	sections := m.splitByHeaders(document.Content)
	if len(sections) == 0 {
		// Fallback to fixed-size chunking if no headers found
		fallback := NewFixedSizeChunker(m.MaxTokens, 50)
		return fallback.Chunk(document)
	}

	chunkIndex := 0
	startChar := 0

	for _, section := range sections {
		// Count approximate tokens (words)
		words := strings.Fields(section)
		wordCount := len(words)

		if wordCount > m.MaxTokens {
			// Split large sections
			subChunks := m.splitLargeSection(section, m.MaxTokens)
			for _, subChunk := range subChunks {
				endChar := startChar + len(subChunk)

				chunk := &models.Chunk{
					ID:          uuid.New(),
					DocumentID:  document.ID,
					ChunkIndex:  chunkIndex,
					Content:     subChunk,
					StartChar:   startChar,
					EndChar:     endChar,
					EmbeddingID: nil,
					Metadata: map[string]interface{}{
						"strategy":      "markdown",
						"max_tokens":    m.MaxTokens,
						"word_count":    len(strings.Fields(subChunk)),
						"is_subsection": true,
						"source_type":   document.SourceType,
						"source_id":     document.SourceID,
					},
				}

				chunks = append(chunks, chunk)
				chunkIndex++
				startChar = endChar + 1
			}
		} else {
			// Keep section as a single chunk
			endChar := startChar + len(section)

			chunk := &models.Chunk{
				ID:          uuid.New(),
				DocumentID:  document.ID,
				ChunkIndex:  chunkIndex,
				Content:     section,
				StartChar:   startChar,
				EndChar:     endChar,
				EmbeddingID: nil,
				Metadata: map[string]interface{}{
					"strategy":      "markdown",
					"max_tokens":    m.MaxTokens,
					"word_count":    wordCount,
					"is_subsection": false,
					"source_type":   document.SourceType,
					"source_id":     document.SourceID,
				},
			}

			chunks = append(chunks, chunk)
			chunkIndex++
			startChar = endChar + 1
		}
	}

	return chunks, nil
}

// splitByHeaders splits markdown content by ## headers
func (m *MarkdownChunker) splitByHeaders(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		// Check if line is a header (## or ###)
		if strings.HasPrefix(strings.TrimSpace(line), "##") {
			// Save current section if not empty
			if len(current) > 0 {
				sections = append(sections, strings.Join(current, "\n"))
			}
			// Start new section with the header
			current = []string{line}
		} else {
			current = append(current, line)
		}
	}

	// Add final section
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

// splitLargeSection splits a large section into smaller chunks
func (m *MarkdownChunker) splitLargeSection(section string, maxTokens int) []string {
	words := strings.Fields(section)
	var chunks []string

	for i := 0; i < len(words); i += maxTokens {
		end := i + maxTokens
		if end > len(words) {
			end = len(words)
		}
		chunks = append(chunks, strings.Join(words[i:end], " "))
	}

	return chunks
}

// GetStrategy returns the name of the chunking strategy
func (m *MarkdownChunker) GetStrategy() string {
	return "markdown"
}

// CodeChunker implements code-aware chunking that respects function/class boundaries
type CodeChunker struct {
	MaxTokens int
	Language  string // go, js, py, etc.
}

// NewCodeChunker creates a new code-aware chunker
func NewCodeChunker(maxTokens int, language string) *CodeChunker {
	return &CodeChunker{
		MaxTokens: maxTokens,
		Language:  language,
	}
}

// Chunk splits code by function/class boundaries
func (c *CodeChunker) Chunk(document *models.Document) ([]*models.Chunk, error) {
	if document == nil {
		return nil, fmt.Errorf("document cannot be nil")
	}

	var chunks []*models.Chunk

	// For now, split by function definitions (Go example)
	// This is a simplified implementation - a full parser would be better
	sections := c.splitByFunctions(document.Content)
	if len(sections) == 0 {
		// Fallback to fixed-size chunking
		fallback := NewFixedSizeChunker(c.MaxTokens, 50)
		return fallback.Chunk(document)
	}

	chunkIndex := 0
	startChar := 0

	for _, section := range sections {
		endChar := startChar + len(section)

		chunk := &models.Chunk{
			ID:          uuid.New(),
			DocumentID:  document.ID,
			ChunkIndex:  chunkIndex,
			Content:     section,
			StartChar:   startChar,
			EndChar:     endChar,
			EmbeddingID: nil,
			Metadata: map[string]interface{}{
				"strategy":    "code",
				"language":    c.Language,
				"max_tokens":  c.MaxTokens,
				"word_count":  len(strings.Fields(section)),
				"source_type": document.SourceType,
				"source_id":   document.SourceID,
			},
		}

		chunks = append(chunks, chunk)
		chunkIndex++
		startChar = endChar + 1
	}

	return chunks, nil
}

// splitByFunctions splits code by function definitions (simplified)
func (c *CodeChunker) splitByFunctions(content string) []string {
	lines := strings.Split(content, "\n")
	var sections []string
	var current []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Detect function start (language-specific)
		isFuncStart := false
		switch c.Language {
		case "go":
			isFuncStart = strings.HasPrefix(trimmed, "func ")
		case "js", "ts":
			isFuncStart = strings.HasPrefix(trimmed, "function ") ||
				strings.Contains(trimmed, " => ")
		case "py":
			isFuncStart = strings.HasPrefix(trimmed, "def ")
		}

		if isFuncStart && len(current) > 0 {
			// Save current section
			sections = append(sections, strings.Join(current, "\n"))
			current = []string{line}
		} else {
			current = append(current, line)
		}
	}

	// Add final section
	if len(current) > 0 {
		sections = append(sections, strings.Join(current, "\n"))
	}

	return sections
}

// GetStrategy returns the name of the chunking strategy
func (c *CodeChunker) GetStrategy() string {
	return fmt.Sprintf("code_%s", c.Language)
}

// Ensure implementations satisfy interfaces.Chunker
var _ interfaces.Chunker = (*FixedSizeChunker)(nil)
var _ interfaces.Chunker = (*MarkdownChunker)(nil)
var _ interfaces.Chunker = (*CodeChunker)(nil)

// GetChunkerForDocument returns an appropriate chunker based on document characteristics
func GetChunkerForDocument(doc *models.Document) interfaces.Chunker {
	title := strings.ToLower(doc.Title)

	// Markdown files
	if strings.HasSuffix(title, ".md") {
		return NewMarkdownChunker(1024)
	}

	// Go code files
	if strings.HasSuffix(title, ".go") {
		return NewCodeChunker(800, "go")
	}

	// JavaScript/TypeScript files
	if strings.HasSuffix(title, ".js") || strings.HasSuffix(title, ".ts") ||
		strings.HasSuffix(title, ".jsx") || strings.HasSuffix(title, ".tsx") {
		return NewCodeChunker(800, "js")
	}

	// Python files
	if strings.HasSuffix(title, ".py") {
		return NewCodeChunker(800, "py")
	}

	// Default: fixed-size chunking
	return NewFixedSizeChunker(512, 50)
}
