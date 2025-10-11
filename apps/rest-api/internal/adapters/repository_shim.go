package adapters

import (
	"context"
	"fmt"

	internalRepo "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"

	pkgRepo "github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// This file contains bridge code to make our adapters package compatible with
// code that expects imports from the pkg/repository package.

// Convert internal embedding to pkg repository embedding
func convertToPkgEmbedding(internal *internalRepo.Embedding) *pkgRepo.Embedding {
	if internal == nil {
		return nil
	}
	return &pkgRepo.Embedding{
		ID:           internal.ID,
		ContextID:    internal.ContextID,
		ContentIndex: internal.ContentIndex,
		Text:         internal.Text,
		Embedding:    internal.Embedding,
		ModelID:      internal.ModelID,
		// Metadata field not used in pkg.Embedding
	}
}

// Convert pkg repository embedding to internal one
func convertToInternalEmbedding(pkgEmb *pkgRepo.Embedding) *internalRepo.Embedding {
	if pkgEmb == nil {
		return nil
	}
	return &internalRepo.Embedding{
		ID:           pkgEmb.ID,
		ContextID:    pkgEmb.ContextID,
		ContentIndex: pkgEmb.ContentIndex,
		Text:         pkgEmb.Text,
		Embedding:    pkgEmb.Embedding,
		ModelID:      pkgEmb.ModelID,
		// No Metadata mapping
	}
}

// Convert slice of internal embeddings to pkg repository embeddings
func convertToPkgEmbeddings(internalEmbs []*internalRepo.Embedding) []*pkgRepo.Embedding {
	if internalEmbs == nil {
		return nil
	}
	result := make([]*pkgRepo.Embedding, len(internalEmbs))
	for i, emb := range internalEmbs {
		result[i] = convertToPkgEmbedding(emb)
	}
	return result
}

// PkgVectorAPIAdapter implements the pkg repository VectorAPIRepository interface
// by delegating to our internal repository
type PkgVectorAPIAdapter struct {
	internal internalRepo.VectorAPIRepository
}

// NewPkgVectorAPIAdapter creates a new adapter for the pkg VectorAPIRepository interface
func NewPkgVectorAPIAdapter(repo internalRepo.VectorAPIRepository) pkgRepo.VectorAPIRepository {
	return &PkgVectorAPIAdapter{internal: repo}
}

// StoreEmbedding implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) StoreEmbedding(ctx context.Context, embedding *pkgRepo.Embedding) error {
	return a.internal.StoreEmbedding(ctx, convertToInternalEmbedding(embedding))
}

// SearchEmbeddings implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, similarityThreshold float64) ([]*pkgRepo.Embedding, error) {
	internalResults, err := a.internal.SearchEmbeddings(ctx, queryVector, contextID, modelID, limit, similarityThreshold)
	if err != nil {
		return nil, err
	}
	return convertToPkgEmbeddings(internalResults), nil
}

// SearchEmbeddings_Legacy implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*pkgRepo.Embedding, error) {
	internalResults, err := a.internal.SearchEmbeddings_Legacy(ctx, queryVector, contextID, limit)
	if err != nil {
		return nil, err
	}
	return convertToPkgEmbeddings(internalResults), nil
}

// GetContextEmbeddings implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*pkgRepo.Embedding, error) {
	internalResults, err := a.internal.GetContextEmbeddings(ctx, contextID)
	if err != nil {
		return nil, err
	}
	return convertToPkgEmbeddings(internalResults), nil
}

// DeleteContextEmbeddings implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	return a.internal.DeleteContextEmbeddings(ctx, contextID)
}

// GetEmbeddingsByModel implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*pkgRepo.Embedding, error) {
	internalResults, err := a.internal.GetEmbeddingsByModel(ctx, contextID, modelID)
	if err != nil {
		return nil, err
	}
	return convertToPkgEmbeddings(internalResults), nil
}

// GetSupportedModels implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	return a.internal.GetSupportedModels(ctx)
}

// DeleteModelEmbeddings implements the pkg repository interface by delegating to internal implementation
func (a *PkgVectorAPIAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	return a.internal.DeleteModelEmbeddings(ctx, contextID, modelID)
}

// Create stores a new embedding (required by vector.Repository interface)
func (a *PkgVectorAPIAdapter) Create(ctx context.Context, embedding *pkgRepo.Embedding) error {
	return a.StoreEmbedding(ctx, embedding)
}

// Get retrieves an embedding by its ID (required by vector.Repository interface)
func (a *PkgVectorAPIAdapter) Get(ctx context.Context, id string) (*pkgRepo.Embedding, error) {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return nil, fmt.Errorf("Get operation not supported")
}

// List retrieves embeddings matching the provided filter (required by vector.Repository interface)
func (a *PkgVectorAPIAdapter) List(ctx context.Context, filter vector.Filter) ([]*pkgRepo.Embedding, error) {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return nil, fmt.Errorf("List operation not supported")
}

// Update modifies an existing embedding (required by vector.Repository interface)
func (a *PkgVectorAPIAdapter) Update(ctx context.Context, embedding *pkgRepo.Embedding) error {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return fmt.Errorf("Update operation not supported")
}

// Delete removes an embedding by its ID (required by vector.Repository interface)
func (a *PkgVectorAPIAdapter) Delete(ctx context.Context, id string) error {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return fmt.Errorf("Delete operation not supported")
}

// Story 2.1: Context-Specific Embedding Methods
// These methods are not implemented in the internal interface yet

// StoreContextEmbedding stores an embedding and links it to a context with metadata
func (a *PkgVectorAPIAdapter) StoreContextEmbedding(ctx context.Context, contextID string, embedding *pkgRepo.Embedding, sequence int, importance float64) (string, error) {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return "", fmt.Errorf("StoreContextEmbedding operation not supported")
}

// GetContextEmbeddingsBySequence retrieves embeddings for a context within a sequence range
func (a *PkgVectorAPIAdapter) GetContextEmbeddingsBySequence(ctx context.Context, contextID string, startSeq int, endSeq int) ([]*pkgRepo.Embedding, error) {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return nil, fmt.Errorf("GetContextEmbeddingsBySequence operation not supported")
}

// UpdateEmbeddingImportance updates the importance score for an embedding
func (a *PkgVectorAPIAdapter) UpdateEmbeddingImportance(ctx context.Context, embeddingID string, importance float64) error {
	// This method is not implemented in the internal interface
	// Return an error indicating this operation is not supported
	return fmt.Errorf("UpdateEmbeddingImportance operation not supported")
}

// Make sure the type compatibility is verified at compile time
func init() {
	// Verify that the adapters implement the pkg repository interfaces
	var _ pkgRepo.VectorAPIRepository = (*PkgVectorAPIAdapter)(nil)
}
