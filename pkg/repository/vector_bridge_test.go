package repository

import (
	"context"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
	"github.com/jmoiron/sqlx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// mockVectorRepository is a mock implementation of vector.Repository
type mockVectorRepository struct {
	mock.Mock
}

// Implement all methods from vector.Repository interface
func (m *mockVectorRepository) StoreEmbedding(ctx context.Context, embedding *vector.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *mockVectorRepository) SearchEmbeddings(ctx context.Context, queryEmbedding []float32, contextID string, modelID string, limit int, threshold float64) ([]*vector.Embedding, error) {
	args := m.Called(ctx, queryEmbedding, contextID, modelID, limit, threshold)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) SearchEmbeddings_Legacy(ctx context.Context, queryEmbedding []float32, contextID string, limit int) ([]*vector.Embedding, error) {
	args := m.Called(ctx, queryEmbedding, contextID, limit)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*vector.Embedding, error) {
	args := m.Called(ctx, contextID)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *mockVectorRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	return args.Get(0).([]string), args.Error(1)
}

func (m *mockVectorRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*vector.Embedding, error) {
	args := m.Called(ctx, contextID, modelID)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	args := m.Called(ctx, contextID, modelID)
	return args.Error(0)
}

func (m *mockVectorRepository) Create(ctx context.Context, embedding *vector.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *mockVectorRepository) Get(ctx context.Context, id string) (*vector.Embedding, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) List(ctx context.Context, filter vector.Filter) ([]*vector.Embedding, error) {
	args := m.Called(ctx, filter)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) Update(ctx context.Context, embedding *vector.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *mockVectorRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Story 2.1: Context-Specific Embedding Methods (Test mock implementation)

func (m *mockVectorRepository) StoreContextEmbedding(
	ctx context.Context,
	contextID string,
	embedding *vector.Embedding,
	sequence int,
	importance float64,
) (string, error) {
	args := m.Called(ctx, contextID, embedding, sequence, importance)
	return args.String(0), args.Error(1)
}

func (m *mockVectorRepository) GetContextEmbeddingsBySequence(
	ctx context.Context,
	contextID string,
	startSeq int,
	endSeq int,
) ([]*vector.Embedding, error) {
	args := m.Called(ctx, contextID, startSeq, endSeq)
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *mockVectorRepository) UpdateEmbeddingImportance(
	ctx context.Context,
	embeddingID string,
	importance float64,
) error {
	args := m.Called(ctx, embeddingID, importance)
	return args.Error(0)
}

// TestNewEmbeddingRepository tests the NewEmbeddingRepository function
func TestNewEmbeddingRepository(t *testing.T) {
	t.Run("with sqlx.DB", func(t *testing.T) {
		db := &sqlx.DB{}
		repo := NewEmbeddingRepository(db)
		assert.NotNil(t, repo, "Repository should not be nil")
		// Verify the repository is not nil (interface check is implicit in return type)
	})

	t.Run("with nil", func(t *testing.T) {
		repo := NewEmbeddingRepository(nil)
		assert.NotNil(t, repo, "Repository should not be nil")
		// Verify the repository is not nil (interface check is implicit in return type)
	})

	// We don't need to test with unsupported DB types since NewEmbeddingRepository expects *sqlx.DB
}

// TestEmbeddingRepositoryAdapter tests the embeddingRepositoryAdapter methods
func TestEmbeddingRepositoryAdapter(t *testing.T) {
	ctx := context.Background()
	mockRepo := new(mockVectorRepository)

	// Test embedding
	embedding := &Embedding{
		ID:        "test-id",
		ContextID: "test-context",
		ModelID:   "test-model",
		Text:      "test content",
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	// Setup adapter with mock vector repository
	adapter := &embeddingRepositoryAdapter{
		db:         nil,
		vectorRepo: mockRepo,
	}

	// Test StoreEmbedding with successful repository
	t.Run("StoreEmbedding with repository", func(t *testing.T) {
		mockRepo.On("StoreEmbedding", ctx, embedding).Return(nil).Once()
		err := adapter.StoreEmbedding(ctx, embedding)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	// Test SearchEmbeddings with successful repository
	t.Run("SearchEmbeddings with repository", func(t *testing.T) {
		queryEmbedding := []float32{0.1, 0.2, 0.3}
		contextID := "test-context"
		modelID := "test-model"
		limit := 10
		threshold := 0.8
		expectedResults := []*Embedding{embedding}

		mockRepo.On("SearchEmbeddings", ctx, queryEmbedding, contextID, modelID, limit, threshold).
			Return(expectedResults, nil).Once()

		results, err := adapter.SearchEmbeddings(ctx, queryEmbedding, contextID, modelID, limit, threshold)
		assert.NoError(t, err)
		assert.Equal(t, expectedResults, results)
		mockRepo.AssertExpectations(t)
	})

	// Test delegation pattern between API methods and Repository methods
	t.Run("Create delegates to StoreEmbedding", func(t *testing.T) {
		mockRepo.On("StoreEmbedding", ctx, embedding).Return(nil).Once()
		err := adapter.Create(ctx, embedding)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Update delegates to StoreEmbedding", func(t *testing.T) {
		mockRepo.On("StoreEmbedding", ctx, embedding).Return(nil).Once()
		err := adapter.Update(ctx, embedding)
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Get calls repository Get", func(t *testing.T) {
		mockRepo.On("Get", ctx, "test-id").Return(embedding, nil).Once()
		result, err := adapter.Get(ctx, "test-id")
		assert.NoError(t, err)
		assert.Equal(t, embedding, result)
		mockRepo.AssertExpectations(t)
	})

	t.Run("Delete calls repository Delete", func(t *testing.T) {
		mockRepo.On("Delete", ctx, "test-id").Return(nil).Once()
		err := adapter.Delete(ctx, "test-id")
		assert.NoError(t, err)
		mockRepo.AssertExpectations(t)
	})

	// Test List with filter
	t.Run("List with filter", func(t *testing.T) {
		filter := vector.Filter{"model_id": "test-model"}
		expectedResults := []*vector.Embedding{embedding}

		mockRepo.On("List", ctx, filter).Return(expectedResults, nil).Once()

		results, err := adapter.List(ctx, filter)
		assert.NoError(t, err)
		assert.Equal(t, expectedResults, results)
		mockRepo.AssertExpectations(t)
	})
}

// TestEmbeddingRepositoryAdapter_Fallbacks tests the fallback implementations when vectorRepo is nil
func TestEmbeddingRepositoryAdapter_Fallbacks(t *testing.T) {
	ctx := context.Background()
	adapter := &embeddingRepositoryAdapter{
		db:         nil,
		vectorRepo: nil,
	}

	// Test embedding
	embedding := &Embedding{
		ID:        "test-id",
		ContextID: "test-context",
		ModelID:   "test-model",
		Text:      "test content",
		Embedding: []float32{0.1, 0.2, 0.3},
	}

	t.Run("StoreEmbedding with nil repository", func(t *testing.T) {
		err := adapter.StoreEmbedding(ctx, embedding)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
	})

	t.Run("SearchEmbeddings with nil repository", func(t *testing.T) {
		queryEmbedding := []float32{0.1, 0.2, 0.3}
		results, err := adapter.SearchEmbeddings(ctx, queryEmbedding, "test-context", "test-model", 10, 0.8)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
		assert.Empty(t, results)
	})

	t.Run("Create with nil repository", func(t *testing.T) {
		err := adapter.Create(ctx, embedding)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
	})

	t.Run("Get with nil repository", func(t *testing.T) {
		result, err := adapter.Get(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
		assert.Nil(t, result)
	})

	t.Run("List with nil repository", func(t *testing.T) {
		filter := vector.Filter{"model_id": "test-model"}
		results, err := adapter.List(ctx, filter)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
		assert.Nil(t, results)
	})

	t.Run("Update with nil repository", func(t *testing.T) {
		err := adapter.Update(ctx, embedding)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
	})

	t.Run("Delete with nil repository", func(t *testing.T) {
		err := adapter.Delete(ctx, "test-id")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
	})

	// Test other API-specific methods
	t.Run("GetContextEmbeddings with nil repository", func(t *testing.T) {
		results, err := adapter.GetContextEmbeddings(ctx, "test-context")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "vector repository not initialized")
		assert.Empty(t, results)
	})
}
