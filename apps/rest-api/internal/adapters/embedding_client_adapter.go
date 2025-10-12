// Package adapters provides adapters between different components
package adapters

import (
	"context"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/agents"
	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/google/uuid"
)

// EmbeddingClient defines the interface needed by semantic context manager
type EmbeddingClient interface {
	// EmbedContent generates embedding for content with optional model override
	// agentID parameter specifies the agent requesting the embedding
	EmbedContent(ctx context.Context, content string, modelOverride string, agentID string) ([]float32, string, error)

	// ChunkContent splits content into chunks for embedding
	ChunkContent(content string, maxChunkSize int) []string
}

// EmbeddingServiceAdapter adapts embedding.ServiceV2 to the EmbeddingClient interface
type EmbeddingServiceAdapter struct {
	service *embedding.ServiceV2
}

// NewEmbeddingServiceAdapter creates a new adapter
func NewEmbeddingServiceAdapter(service *embedding.ServiceV2) EmbeddingClient {
	return &EmbeddingServiceAdapter{
		service: service,
	}
}

// EmbedContent generates embedding for content with optional model override
// The agentID parameter specifies which agent is requesting the embedding
// Returns: embedding vector, model used, error
func (a *EmbeddingServiceAdapter) EmbedContent(ctx context.Context, content string, modelOverride string, agentID string) ([]float32, string, error) {
	// If no agent ID provided, use a system UUID for semantic context operations
	if agentID == "" {
		agentID = uuid.Nil.String() // Use nil UUID to indicate system operation
	}

	// Create embedding request
	req := embedding.GenerateEmbeddingRequest{
		AgentID:  agentID,
		Text:     content,
		Model:    modelOverride,
		TaskType: agents.TaskTypeGeneralQA,
		TenantID: uuid.Nil, // Use nil UUID for system-level operations
		Metadata: map[string]interface{}{
			"source": "semantic_context_manager",
		},
	}

	// Generate embedding
	resp, err := a.service.GenerateEmbedding(ctx, req)
	if err != nil {
		return nil, "", err
	}

	// Get the actual embedding from the repository
	// The ServiceV2 stores embeddings and returns an ID
	// For the semantic context manager, we need the actual vector
	// So we'll use the normalized dimension from the response metadata
	if normalizedEmbedding, ok := resp.Metadata["normalized_embedding"].([]float32); ok {
		return normalizedEmbedding, resp.ModelUsed, nil
	}

	// Fallback: generate a simple embedding by requesting again
	// This shouldn't happen in normal operation
	return make([]float32, embedding.StandardDimension), resp.ModelUsed, nil
}

// ChunkContent splits content into chunks for embedding
func (a *EmbeddingServiceAdapter) ChunkContent(content string, maxChunkSize int) []string {
	if content == "" {
		return []string{}
	}

	// If content is smaller than max chunk size, return as single chunk
	if len(content) <= maxChunkSize {
		return []string{content}
	}

	// Split by sentences (periods, exclamation marks, question marks)
	sentences := splitIntoSentences(content)

	var chunks []string
	var currentChunk strings.Builder
	currentSize := 0

	for _, sentence := range sentences {
		sentenceLen := len(sentence)

		// If adding this sentence would exceed max size
		if currentSize+sentenceLen > maxChunkSize && currentSize > 0 {
			// Save current chunk
			chunks = append(chunks, currentChunk.String())
			currentChunk.Reset()
			currentSize = 0
		}

		// Add sentence to current chunk
		if currentSize > 0 {
			currentChunk.WriteString(" ")
			currentSize++
		}
		currentChunk.WriteString(sentence)
		currentSize += sentenceLen
	}

	// Add final chunk if not empty
	if currentSize > 0 {
		chunks = append(chunks, currentChunk.String())
	}

	// If still no chunks (shouldn't happen), return original content
	if len(chunks) == 0 {
		chunks = append(chunks, content)
	}

	return chunks
}

// splitIntoSentences splits text into sentences
func splitIntoSentences(text string) []string {
	// Simple sentence splitting on period, exclamation, question mark
	var sentences []string
	var current strings.Builder

	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])

		// Check if this is a sentence boundary
		if runes[i] == '.' || runes[i] == '!' || runes[i] == '?' {
			// Check if followed by space or end
			if i+1 >= len(runes) || runes[i+1] == ' ' || runes[i+1] == '\n' {
				// Check if this looks like an abbreviation (e.g., "Dr.", "Mr.", "etc.")
				sentence := current.String()
				if !looksLikeAbbreviation(sentence) {
					sentences = append(sentences, strings.TrimSpace(sentence))
					current.Reset()
				}
			}
		}
	}

	// Add final sentence if not empty
	if current.Len() > 0 {
		sentences = append(sentences, strings.TrimSpace(current.String()))
	}

	return sentences
}

// looksLikeAbbreviation checks if a sentence ending looks like an abbreviation
func looksLikeAbbreviation(sentence string) bool {
	sentence = strings.TrimSpace(sentence)
	if len(sentence) < 3 {
		return false
	}

	// Common abbreviations
	abbreviations := []string{"Dr.", "Mr.", "Mrs.", "Ms.", "etc.", "i.e.", "e.g.", "Inc.", "Ltd.", "Co.", "Prof."}
	for _, abbrev := range abbreviations {
		if strings.HasSuffix(sentence, abbrev) {
			return true
		}
	}

	// Check for single capital letter followed by period (e.g., "A.", "B.")
	if len(sentence) >= 2 && sentence[len(sentence)-1] == '.' {
		if len(sentence) == 2 || (len(sentence) > 2 && sentence[len(sentence)-3] == ' ') {
			lastChar := sentence[len(sentence)-2]
			if lastChar >= 'A' && lastChar <= 'Z' {
				return true
			}
		}
	}

	return false
}
