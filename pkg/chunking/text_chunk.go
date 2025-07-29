package chunking

// TextChunk represents a chunk of text with metadata
// This is distinct from CodeChunk which is for code parsing
type TextChunk struct {
	// The actual text content of the chunk
	Content string `json:"content"`

	// Index of this chunk in the sequence
	Index int `json:"index"`

	// Number of tokens in this chunk
	TokenCount int `json:"token_count"`

	// Character positions in the original document
	StartChar int `json:"start_char,omitempty"`
	EndChar   int `json:"end_char,omitempty"`

	// Additional metadata about the chunk
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TextChunker interface for text chunking strategies
type TextChunker interface {
	// Chunk splits text into chunks based on the implementation strategy
	Chunk(text string, metadata map[string]interface{}) ([]*TextChunk, error)

	// GetConfig returns the chunker configuration
	GetConfig() interface{}
}
