// Package repository provides a bridge to the new vector package
package repository

import (
	"context"
	"errors"

	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
	"github.com/jmoiron/sqlx"
)

// This file provides compatibility during the migration from the old repository
// structure to the new package-based structure.

// Note: The interface VectorAPIRepository is now defined in interfaces.go

// NewEmbeddingAdapter creates a new embedding adapter that implements VectorAPIRepository
// This function replaces the previous NewEmbeddingRepository function
func NewEmbeddingAdapter(db any) VectorAPIRepository {
	// Handle different database types
	var sqlxDB *sqlx.DB

	switch typedDB := db.(type) {
	case *sqlx.DB:
		sqlxDB = typedDB
		// Create a new vector repository using the vector package
		vectorRepo := vector.NewRepository(sqlxDB)
		// Create our adapter with proper vector repository
		return &embeddingRepositoryAdapter{db: sqlxDB, vectorRepo: vectorRepo}
	case nil:
		// Create a mock repository when no database is provided
		return newMockEmbeddingAdapter()
	default:
		// For other database types, we can't use them directly with vector.NewRepository
		// so we'll return a mock adapter
		return newMockEmbeddingAdapter()
	}
}

// embeddingRepositoryAdapter provides APIs for the vector database
// when we can't use the new vector package directly
type embeddingRepositoryAdapter struct {
	db         any
	vectorRepo vector.Repository
}

// StoreEmbedding implements VectorAPIRepository.StoreEmbedding
func (r *embeddingRepositoryAdapter) StoreEmbedding(ctx context.Context, embedding *Embedding) error {
	if r.vectorRepo != nil {
		return r.vectorRepo.StoreEmbedding(ctx, embedding)
	}

	// Fallback implementation when vectorRepo is not available
	return errors.New("vector repository not initialized")
}

// SearchEmbeddings implements VectorAPIRepository.SearchEmbeddings
func (r *embeddingRepositoryAdapter) SearchEmbeddings(
	ctx context.Context,
	queryEmbedding []float32,
	contextID string,
	modelID string,
	limit int,
	threshold float64,
) ([]*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.SearchEmbeddings(ctx, queryEmbedding, contextID, modelID, limit, threshold)
	}

	// Fallback implementation when vectorRepo is not available
	return []*Embedding{}, errors.New("vector repository not initialized")
}

// SearchEmbeddings_Legacy implements VectorAPIRepository.SearchEmbeddings_Legacy
func (r *embeddingRepositoryAdapter) SearchEmbeddings_Legacy(
	ctx context.Context,
	queryEmbedding []float32,
	contextID string,
	limit int,
) ([]*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.SearchEmbeddings_Legacy(ctx, queryEmbedding, contextID, limit)
	}

	// Fallback implementation when vectorRepo is not available
	return []*Embedding{}, errors.New("vector repository not initialized")
}

// GetContextEmbeddings implements VectorAPIRepository.GetContextEmbeddings
func (r *embeddingRepositoryAdapter) GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.GetContextEmbeddings(ctx, contextID)
	}

	// Fallback implementation when vectorRepo is not available
	return []*Embedding{}, errors.New("vector repository not initialized")
}

// DeleteContextEmbeddings implements VectorAPIRepository.DeleteContextEmbeddings
func (r *embeddingRepositoryAdapter) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	if r.vectorRepo != nil {
		return r.vectorRepo.DeleteContextEmbeddings(ctx, contextID)
	}

	// Fallback implementation when vectorRepo is not available
	return errors.New("vector repository not initialized")
}

// GetSupportedModels implements VectorAPIRepository.GetSupportedModels
func (r *embeddingRepositoryAdapter) GetSupportedModels(ctx context.Context) ([]string, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.GetSupportedModels(ctx)
	}

	// Fallback implementation when vectorRepo is not available
	return []string{}, errors.New("vector repository not initialized")
}

// GetEmbeddingsByModel implements VectorAPIRepository.GetEmbeddingsByModel
func (r *embeddingRepositoryAdapter) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.GetEmbeddingsByModel(ctx, contextID, modelID)
	}

	// Fallback implementation when vectorRepo is not available
	return []*Embedding{}, errors.New("vector repository not initialized")
}

// DeleteModelEmbeddings implements VectorAPIRepository.DeleteModelEmbeddings
func (r *embeddingRepositoryAdapter) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	if r.vectorRepo != nil {
		return r.vectorRepo.DeleteModelEmbeddings(ctx, contextID, modelID)
	}

	// Fallback implementation when vectorRepo is not available
	return errors.New("vector repository not initialized")
}

// The following methods implement the standard Repository[Embedding] interface

// Create implements Repository[Embedding].Create
func (r *embeddingRepositoryAdapter) Create(ctx context.Context, embedding *Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	return r.StoreEmbedding(ctx, embedding)
}

// Get implements Repository[Embedding].Get
func (r *embeddingRepositoryAdapter) Get(ctx context.Context, id string) (*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.Get(ctx, id)
	}

	// Fallback implementation
	return nil, errors.New("vector repository not initialized")
}

// List implements the vector.Repository.List method
// This uses the vector.Filter and vector.Embedding types directly to match the interface
func (r *embeddingRepositoryAdapter) List(ctx context.Context, filter vector.Filter) ([]*vector.Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.List(ctx, filter)
	}

	// Fallback implementation
	return nil, errors.New("vector repository not initialized")
}

// Update implements Repository[Embedding].Update
func (r *embeddingRepositoryAdapter) Update(ctx context.Context, embedding *Embedding) error {
	// Delegate to StoreEmbedding for backward compatibility
	return r.StoreEmbedding(ctx, embedding)
}

// Delete implements Repository[Embedding].Delete
func (r *embeddingRepositoryAdapter) Delete(ctx context.Context, id string) error {
	if r.vectorRepo != nil {
		return r.vectorRepo.Delete(ctx, id)
	}

	// Fallback implementation
	return errors.New("vector repository not initialized")
}

// Story 2.1: Context-Specific Embedding Methods (Adapter implementation)

// StoreContextEmbedding implements VectorAPIRepository.StoreContextEmbedding
func (r *embeddingRepositoryAdapter) StoreContextEmbedding(
	ctx context.Context,
	contextID string,
	embedding *Embedding,
	sequence int,
	importance float64,
) (string, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.StoreContextEmbedding(ctx, contextID, embedding, sequence, importance)
	}

	// Fallback implementation
	return "", errors.New("vector repository not initialized")
}

// GetContextEmbeddingsBySequence implements VectorAPIRepository.GetContextEmbeddingsBySequence
func (r *embeddingRepositoryAdapter) GetContextEmbeddingsBySequence(
	ctx context.Context,
	contextID string,
	startSeq int,
	endSeq int,
) ([]*Embedding, error) {
	if r.vectorRepo != nil {
		return r.vectorRepo.GetContextEmbeddingsBySequence(ctx, contextID, startSeq, endSeq)
	}

	// Fallback implementation
	return []*Embedding{}, errors.New("vector repository not initialized")
}

// UpdateEmbeddingImportance implements VectorAPIRepository.UpdateEmbeddingImportance
func (r *embeddingRepositoryAdapter) UpdateEmbeddingImportance(
	ctx context.Context,
	embeddingID string,
	importance float64,
) error {
	if r.vectorRepo != nil {
		return r.vectorRepo.UpdateEmbeddingImportance(ctx, embeddingID, importance)
	}

	// Fallback implementation
	return errors.New("vector repository not initialized")
}

// newMockEmbeddingAdapter creates a mock adapter for testing
func newMockEmbeddingAdapter() VectorAPIRepository {
	return &embeddingRepositoryAdapter{
		db:         nil,
		vectorRepo: nil,
	}
}
