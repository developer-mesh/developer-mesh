package model_catalog

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/mock"
)

// MockModelCatalogRepository is a mock implementation of ModelCatalogRepository
type MockModelCatalogRepository struct {
	mock.Mock
}

// GetByID mocks the GetByID method
func (m *MockModelCatalogRepository) GetByID(ctx context.Context, id uuid.UUID) (*EmbeddingModel, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingModel), args.Error(1)
}

// GetByModelID mocks the GetByModelID method
func (m *MockModelCatalogRepository) GetByModelID(ctx context.Context, modelID string) (*EmbeddingModel, error) {
	args := m.Called(ctx, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingModel), args.Error(1)
}

// ListAvailable mocks the ListAvailable method
func (m *MockModelCatalogRepository) ListAvailable(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingModel), args.Error(1)
}

// ListAll mocks the ListAll method
func (m *MockModelCatalogRepository) ListAll(ctx context.Context, filter *ModelFilter) ([]*EmbeddingModel, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*EmbeddingModel), args.Error(1)
}

// Create mocks the Create method
func (m *MockModelCatalogRepository) Create(ctx context.Context, model *EmbeddingModel) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

// Update mocks the Update method
func (m *MockModelCatalogRepository) Update(ctx context.Context, model *EmbeddingModel) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}

// Delete mocks the Delete method
func (m *MockModelCatalogRepository) Delete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// SetAvailability mocks the SetAvailability method
func (m *MockModelCatalogRepository) SetAvailability(ctx context.Context, id uuid.UUID, isAvailable bool) error {
	args := m.Called(ctx, id, isAvailable)
	return args.Error(0)
}

// MarkDeprecated mocks the MarkDeprecated method
func (m *MockModelCatalogRepository) MarkDeprecated(ctx context.Context, id uuid.UUID, deprecationDate *time.Time) error {
	args := m.Called(ctx, id, deprecationDate)
	return args.Error(0)
}

// GetProviders mocks the GetProviders method
func (m *MockModelCatalogRepository) GetProviders(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

// BulkUpsert mocks the BulkUpsert method
func (m *MockModelCatalogRepository) BulkUpsert(ctx context.Context, models []*EmbeddingModel) error {
	args := m.Called(ctx, models)
	return args.Error(0)
}
