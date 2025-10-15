// Package interfaces defines core interfaces for the RAG loader system
package interfaces

import (
	"context"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// DataSource defines the interface for data sources in the RAG system
type DataSource interface {
	// ID returns a unique identifier for this source
	ID() string

	// Type returns the type of data source (github, web, s3, etc.)
	Type() string

	// Fetch retrieves documents since the given timestamp
	// If since is nil, fetches all available documents
	Fetch(ctx context.Context, since *time.Time) ([]*models.Document, error)

	// GetMetadata returns source-specific metadata
	GetMetadata() map[string]interface{}

	// Validate checks if the source configuration is valid
	Validate() error

	// HealthCheck verifies the source is accessible and working
	HealthCheck(ctx context.Context) error
}

// SourceFactory creates a DataSource from configuration
type SourceFactory func(config map[string]interface{}) (DataSource, error)

// Chunker defines the interface for document chunking strategies
type Chunker interface {
	// Chunk splits a document into chunks based on the strategy
	Chunk(document *models.Document) ([]*models.Chunk, error)

	// GetStrategy returns the name of the chunking strategy
	GetStrategy() string
}

// Processor defines the interface for document processing
type Processor interface {
	// Process applies transformations to a document
	Process(ctx context.Context, document *models.Document) error

	// ExtractMetadata extracts metadata from a document
	ExtractMetadata(document *models.Document) map[string]interface{}
}
