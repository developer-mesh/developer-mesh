package text

import (
	"context"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
)

// RecursiveCharacterSplitter implements recursive splitting with multiple separators
type RecursiveCharacterSplitter struct {
	separators     []string
	chunkSize      int
	chunkOverlap   int
	lengthFunction func(string) int
	keepSeparator  bool
}

// RecursiveCharacterSplitterConfig configures the recursive splitter
type RecursiveCharacterSplitterConfig struct {
	Separators     []string
	ChunkSize      int
	ChunkOverlap   int
	LengthFunction func(string) int
	KeepSeparator  bool
}

// DefaultSeparators returns the default separators for splitting
func DefaultSeparators() []string {
	return []string{
		"\n\n\n", // Triple newline (major sections)
		"\n\n",   // Double newline (paragraphs)
		"\n",     // Single newline
		". ",     // Sentence ending
		"! ",     // Exclamation
		"? ",     // Question
		"; ",     // Semicolon
		": ",     // Colon
		", ",     // Comma
		" ",      // Space
		"",       // Character-level fallback
	}
}

// NewRecursiveCharacterSplitter creates a new recursive character splitter
func NewRecursiveCharacterSplitter(config *RecursiveCharacterSplitterConfig) *RecursiveCharacterSplitter {
	if config == nil {
		config = &RecursiveCharacterSplitterConfig{}
	}

	if len(config.Separators) == 0 {
		config.Separators = DefaultSeparators()
	}

	if config.ChunkSize <= 0 {
		config.ChunkSize = 1000
	}

	if config.ChunkOverlap < 0 {
		config.ChunkOverlap = 200
	}

	if config.LengthFunction == nil {
		config.LengthFunction = func(s string) int { return len(s) }
	}

	return &RecursiveCharacterSplitter{
		separators:     config.Separators,
		chunkSize:      config.ChunkSize,
		chunkOverlap:   config.ChunkOverlap,
		lengthFunction: config.LengthFunction,
		keepSeparator:  config.KeepSeparator,
	}
}

// Chunk splits text into chunks using recursive character splitting
func (r *RecursiveCharacterSplitter) Chunk(ctx context.Context, text string, metadata map[string]interface{}) ([]*chunking.TextChunk, error) {
	if text == "" {
		return []*chunking.TextChunk{}, nil
	}

	// Split the text recursively
	splits := r.splitText(text, r.separators)

	// Merge splits into chunks of appropriate size
	chunks := r.mergeSplits(splits, metadata)

	return chunks, nil
}

// GetConfig returns the chunker configuration
func (r *RecursiveCharacterSplitter) GetConfig() interface{} {
	return RecursiveCharacterSplitterConfig{
		Separators:     r.separators,
		ChunkSize:      r.chunkSize,
		ChunkOverlap:   r.chunkOverlap,
		LengthFunction: r.lengthFunction,
		KeepSeparator:  r.keepSeparator,
	}
}

// splitText recursively splits text using separators
func (r *RecursiveCharacterSplitter) splitText(text string, separators []string) []string {
	finalChunks := []string{}

	// Find the separator that works for this text
	separator := ""
	newSeparators := []string{}

	for i, sep := range separators {
		if sep == "" || strings.Contains(text, sep) {
			separator = sep
			newSeparators = separators[i+1:]
			break
		}
	}

	// Split by the separator
	var splits []string
	if separator == "" {
		// Character-level split as last resort
		splits = r.splitByCharacters(text)
	} else {
		splits = r.splitBySeparator(text, separator)
	}

	// Process each split
	for _, split := range splits {
		if split == "" {
			continue
		}

		splitLen := r.lengthFunction(split)

		if splitLen < r.chunkSize {
			// Small enough, add it
			finalChunks = append(finalChunks, split)
		} else if splitLen > r.chunkSize && len(newSeparators) > 0 {
			// Too big and we have more separators, recurse
			otherSplits := r.splitText(split, newSeparators)
			finalChunks = append(finalChunks, otherSplits...)
		} else {
			// Too big but no more separators, force split
			forcedSplits := r.forceSplit(split)
			finalChunks = append(finalChunks, forcedSplits...)
		}
	}

	return finalChunks
}

// splitBySeparator splits text by a separator
func (r *RecursiveCharacterSplitter) splitBySeparator(text string, separator string) []string {
	if separator == "" {
		return []string{text}
	}

	splits := strings.Split(text, separator)

	if !r.keepSeparator {
		return splits
	}

	// Add separator back to splits (except the last one)
	result := make([]string, 0, len(splits))
	for i, split := range splits {
		if i < len(splits)-1 {
			result = append(result, split+separator)
		} else if split != "" {
			result = append(result, split)
		}
	}

	return result
}

// splitByCharacters splits text into individual characters
func (r *RecursiveCharacterSplitter) splitByCharacters(text string) []string {
	// Split into chunks of chunkSize characters
	var chunks []string
	runes := []rune(text)

	for i := 0; i < len(runes); i += r.chunkSize {
		end := i + r.chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}

	return chunks
}

// forceSplit forces a split when text is too long
func (r *RecursiveCharacterSplitter) forceSplit(text string) []string {
	var chunks []string

	for r.lengthFunction(text) > r.chunkSize {
		// Find a good split point
		splitPoint := r.findSplitPoint(text)
		chunks = append(chunks, text[:splitPoint])
		text = text[splitPoint:]
	}

	if text != "" {
		chunks = append(chunks, text)
	}

	return chunks
}

// findSplitPoint finds a good point to split text
func (r *RecursiveCharacterSplitter) findSplitPoint(text string) int {
	if r.lengthFunction(text) <= r.chunkSize {
		return len(text)
	}

	// Try to split at a natural boundary
	targetLen := r.chunkSize

	// Look for space near target length
	for i := targetLen; i > targetLen/2; i-- {
		if i < len(text) && text[i] == ' ' {
			return i + 1
		}
	}

	// No good split point found, split at chunk size
	return targetLen
}

// mergeSplits merges splits into chunks with overlap
func (r *RecursiveCharacterSplitter) mergeSplits(splits []string, metadata map[string]interface{}) []*chunking.TextChunk {
	if len(splits) == 0 {
		return []*chunking.TextChunk{}
	}

	var chunks []*chunking.TextChunk
	currentDocs := []string{}
	currentLen := 0
	startChar := 0
	currentChar := 0

	for _, split := range splits {
		splitLen := r.lengthFunction(split)

		// Check if adding this split would exceed chunk size
		if currentLen > 0 && currentLen+splitLen > r.chunkSize {
			// Create chunk from current docs
			chunk := r.createChunk(currentDocs, len(chunks), metadata, startChar, currentChar)
			chunks = append(chunks, chunk)

			// Handle overlap
			currentDocs = r.getOverlapDocs(currentDocs)
			currentLen = 0
			for _, doc := range currentDocs {
				currentLen += r.lengthFunction(doc)
			}
			startChar = currentChar - currentLen
		}

		currentDocs = append(currentDocs, split)
		currentLen += splitLen
		currentChar += splitLen
	}

	// Add final chunk
	if len(currentDocs) > 0 {
		chunk := r.createChunk(currentDocs, len(chunks), metadata, startChar, currentChar)
		chunks = append(chunks, chunk)
	}

	return chunks
}

// getOverlapDocs returns documents for overlap
func (r *RecursiveCharacterSplitter) getOverlapDocs(docs []string) []string {
	if r.chunkOverlap == 0 || len(docs) == 0 {
		return []string{}
	}

	overlapDocs := []string{}
	overlapLen := 0

	// Add docs from the end until we reach overlap size
	for i := len(docs) - 1; i >= 0 && overlapLen < r.chunkOverlap; i-- {
		doc := docs[i]
		docLen := r.lengthFunction(doc)

		if overlapLen+docLen <= r.chunkOverlap {
			overlapDocs = append([]string{doc}, overlapDocs...)
			overlapLen += docLen
		} else {
			// Partial doc to reach exact overlap
			remainingLen := r.chunkOverlap - overlapLen
			if remainingLen > 0 && remainingLen < docLen {
				partialDoc := doc[len(doc)-remainingLen:]
				overlapDocs = append([]string{partialDoc}, overlapDocs...)
			}
			break
		}
	}

	return overlapDocs
}

// createChunk creates a chunk from documents
func (r *RecursiveCharacterSplitter) createChunk(docs []string, index int, metadata map[string]interface{}, startChar, endChar int) *chunking.TextChunk {
	content := strings.Join(docs, "")

	chunk := &chunking.TextChunk{
		Content:    content,
		Index:      index,
		TokenCount: r.lengthFunction(content),
		StartChar:  startChar,
		EndChar:    endChar,
		Metadata:   copyMetadata(metadata),
	}

	if chunk.Metadata == nil {
		chunk.Metadata = make(map[string]interface{})
	}

	chunk.Metadata["chunking_method"] = "recursive_character"
	chunk.Metadata["chunk_index"] = index
	chunk.Metadata["config"] = map[string]interface{}{
		"chunk_size":    r.chunkSize,
		"chunk_overlap": r.chunkOverlap,
	}

	return chunk
}
