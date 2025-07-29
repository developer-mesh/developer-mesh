package embedding

import (
	"context"
	"fmt"

	"github.com/developer-mesh/developer-mesh/pkg/chunking"
	"github.com/developer-mesh/developer-mesh/pkg/chunking/text"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tokenizer"
)

// DocumentProcessor handles document chunking and embedding
type DocumentProcessor struct {
	embeddingService EmbeddingService
	textChunker      *text.SemanticChunker
	tokenizer        tokenizer.Tokenizer
	logger           observability.Logger
}

// NewDocumentProcessor creates a new document processor
func NewDocumentProcessor(embeddingService EmbeddingService, logger observability.Logger) *DocumentProcessor {
	if logger == nil {
		logger = observability.NewLogger("embedding.document_processor")
	}

	// Create tokenizer
	tok := tokenizer.NewSimpleTokenizer(8192)

	// Create semantic chunker with default config
	chunkerConfig := &text.Config{
		MinChunkSize:    200,
		MaxChunkSize:    1000,
		TargetChunkSize: 500,
		OverlapSize:     50,
	}

	textChunker := text.NewSemanticChunker(tok, chunkerConfig)

	return &DocumentProcessor{
		embeddingService: embeddingService,
		textChunker:      textChunker,
		tokenizer:        tok,
		logger:           logger,
	}
}

// ProcessDocument chunks and embeds a document
func (p *DocumentProcessor) ProcessDocument(ctx context.Context, doc *Document) ([]*EmbeddingVector, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "document_processor.process")
	defer span.End()

	span.SetAttribute("document_id", doc.ID)
	span.SetAttribute("content_type", doc.ContentType)

	// Chunk the document
	chunks, err := p.chunkDocument(ctx, doc)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk document: %w", err)
	}

	p.logger.Info("Document chunked", map[string]interface{}{
		"document_id": doc.ID,
		"chunk_count": len(chunks),
	})

	// Create embeddings for each chunk
	embeddings := make([]*EmbeddingVector, 0, len(chunks))

	for i, chunk := range chunks {
		// Create metadata for the chunk
		metadata := map[string]interface{}{
			"document_id":  doc.ID,
			"chunk_index":  i,
			"chunk_count":  len(chunks),
			"content_type": doc.ContentType,
			"token_count":  chunk.TokenCount,
			"start_char":   chunk.StartChar,
			"end_char":     chunk.EndChar,
		}

		// Merge with document metadata
		for k, v := range doc.Metadata {
			if _, exists := metadata[k]; !exists {
				metadata[k] = v
			}
		}

		// Merge with chunk metadata
		for k, v := range chunk.Metadata {
			metadata[k] = v
		}

		// Generate embedding
		embedding, err := p.embeddingService.GenerateEmbedding(ctx, chunk.Content, doc.ContentType, doc.ID+fmt.Sprintf("_chunk_%d", i))
		if err != nil {
			p.logger.Error("Failed to create embedding for chunk", map[string]interface{}{
				"error":       err.Error(),
				"document_id": doc.ID,
				"chunk_index": i,
			})
			continue
		}

		// Add metadata to the embedding
		if embedding.Metadata == nil {
			embedding.Metadata = make(map[string]interface{})
		}
		for k, v := range metadata {
			embedding.Metadata[k] = v
		}

		embeddings = append(embeddings, embedding)
	}

	if len(embeddings) == 0 {
		return nil, fmt.Errorf("failed to create any embeddings for document %s", doc.ID)
	}

	p.logger.Info("Document processed", map[string]interface{}{
		"document_id":        doc.ID,
		"chunks_created":     len(chunks),
		"embeddings_created": len(embeddings),
	})

	return embeddings, nil
}

// chunkDocument chunks a document based on its content type
func (p *DocumentProcessor) chunkDocument(ctx context.Context, doc *Document) ([]*chunking.TextChunk, error) {
	// For now, we only support text chunking
	// In the future, we can add support for code chunking, PDF chunking, etc.

	if doc.Content == "" {
		return nil, fmt.Errorf("document content is empty")
	}

	// Use semantic chunker for all text content
	chunks, err := p.textChunker.Chunk(ctx, doc.Content, doc.Metadata)
	if err != nil {
		return nil, fmt.Errorf("failed to chunk text: %w", err)
	}

	return chunks, nil
}

// Document represents a document to be processed
type Document struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType string                 `json:"content_type"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// ChunkingConfig allows customization of chunking parameters
type ChunkingConfig struct {
	MinChunkSize    int `json:"min_chunk_size"`
	MaxChunkSize    int `json:"max_chunk_size"`
	TargetChunkSize int `json:"target_chunk_size"`
	OverlapSize     int `json:"overlap_size"`
}

// ProcessDocumentWithConfig processes a document with custom chunking config
func (p *DocumentProcessor) ProcessDocumentWithConfig(ctx context.Context, doc *Document, config *ChunkingConfig) ([]*EmbeddingVector, error) {
	// Create a new chunker with custom config
	chunkerConfig := &text.Config{
		MinChunkSize:    config.MinChunkSize,
		MaxChunkSize:    config.MaxChunkSize,
		TargetChunkSize: config.TargetChunkSize,
		OverlapSize:     config.OverlapSize,
	}

	customChunker := text.NewSemanticChunker(p.tokenizer, chunkerConfig)

	// Temporarily replace the chunker
	originalChunker := p.textChunker
	p.textChunker = customChunker
	defer func() {
		p.textChunker = originalChunker
	}()

	return p.ProcessDocument(ctx, doc)
}

// BatchProcessDocuments processes multiple documents
func (p *DocumentProcessor) BatchProcessDocuments(ctx context.Context, docs []*Document) (map[string][]*EmbeddingVector, error) {
	results := make(map[string][]*EmbeddingVector)

	for _, doc := range docs {
		embeddings, err := p.ProcessDocument(ctx, doc)
		if err != nil {
			p.logger.Error("Failed to process document", map[string]interface{}{
				"error":       err.Error(),
				"document_id": doc.ID,
			})
			continue
		}
		results[doc.ID] = embeddings
	}

	return results, nil
}

// BatchProcessDocumentsWithConfig processes multiple documents with custom config
func (p *DocumentProcessor) BatchProcessDocumentsWithConfig(ctx context.Context, docs []*Document, config *ChunkingConfig) (map[string][]*EmbeddingVector, error) {
	results := make(map[string][]*EmbeddingVector)

	for _, doc := range docs {
		embeddings, err := p.ProcessDocumentWithConfig(ctx, doc, config)
		if err != nil {
			p.logger.Error("Failed to process document", map[string]interface{}{
				"error":       err.Error(),
				"document_id": doc.ID,
			})
			continue
		}
		results[doc.ID] = embeddings
	}

	return results, nil
}
