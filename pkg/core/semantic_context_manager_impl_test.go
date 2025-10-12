// Story 2.3: Unit Tests for Semantic Context Manager
// LOCATION: pkg/core/semantic_context_manager_impl_test.go

package core_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/developer-mesh/developer-mesh/pkg/core"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

// MockContextRepository implements repository.ContextRepository for testing
type MockContextRepository struct {
	mock.Mock
}

func (m *MockContextRepository) Create(ctx context.Context, contextObj *repository.Context) error {
	args := m.Called(ctx, contextObj)
	return args.Error(0)
}

func (m *MockContextRepository) Get(ctx context.Context, id string) (*repository.Context, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Context), args.Error(1)
}

func (m *MockContextRepository) Update(ctx context.Context, contextObj *repository.Context) error {
	args := m.Called(ctx, contextObj)
	return args.Error(0)
}

func (m *MockContextRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContextRepository) List(ctx context.Context, filter map[string]any) ([]*repository.Context, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Context), args.Error(1)
}

func (m *MockContextRepository) Search(ctx context.Context, contextID, query string) ([]repository.ContextItem, error) {
	args := m.Called(ctx, contextID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.ContextItem), args.Error(1)
}

func (m *MockContextRepository) Summarize(ctx context.Context, contextID string) (string, error) {
	args := m.Called(ctx, contextID)
	return args.String(0), args.Error(1)
}

func (m *MockContextRepository) AddContextItem(ctx context.Context, contextID string, item *repository.ContextItem) error {
	args := m.Called(ctx, contextID, item)
	return args.Error(0)
}

func (m *MockContextRepository) GetContextItems(ctx context.Context, contextID string) ([]*repository.ContextItem, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.ContextItem), args.Error(1)
}

func (m *MockContextRepository) UpdateContextItem(ctx context.Context, item *repository.ContextItem) error {
	args := m.Called(ctx, item)
	return args.Error(0)
}

func (m *MockContextRepository) UpdateCompactionMetadata(ctx context.Context, contextID string, strategy string, lastCompactedAt time.Time) error {
	args := m.Called(ctx, contextID, strategy, lastCompactedAt)
	return args.Error(0)
}

func (m *MockContextRepository) GetContextsNeedingCompaction(ctx context.Context, threshold int) ([]*repository.Context, error) {
	args := m.Called(ctx, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Context), args.Error(1)
}

func (m *MockContextRepository) LinkEmbeddingToContext(ctx context.Context, contextID string, embeddingID string, sequence int, importance float64) error {
	args := m.Called(ctx, contextID, embeddingID, sequence, importance)
	return args.Error(0)
}

func (m *MockContextRepository) GetContextEmbeddingLinks(ctx context.Context, contextID string) ([]repository.ContextEmbeddingLink, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]repository.ContextEmbeddingLink), args.Error(1)
}

// MockVectorAPIRepository implements repository.VectorAPIRepository for testing
type MockVectorAPIRepository struct {
	mock.Mock
}

func (m *MockVectorAPIRepository) StoreEmbedding(ctx context.Context, embedding *repository.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, threshold float64) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, modelID, limit, threshold)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*repository.Embedding, error) {
	args := m.Called(ctx, queryVector, contextID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) GetContextEmbeddings(ctx context.Context, contextID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) DeleteContextEmbeddings(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID, modelID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) GetSupportedModels(ctx context.Context) ([]string, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockVectorAPIRepository) DeleteModelEmbeddings(ctx context.Context, contextID string, modelID string) error {
	args := m.Called(ctx, contextID, modelID)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) GetEmbeddingByID(ctx context.Context, id string) (*repository.Embedding, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) DeleteEmbedding(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) BatchDeleteEmbeddings(ctx context.Context, ids []string) error {
	args := m.Called(ctx, ids)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) StoreContextEmbedding(ctx context.Context, contextID string, embedding *repository.Embedding, sequence int, importance float64) (string, error) {
	args := m.Called(ctx, contextID, embedding, sequence, importance)
	return args.String(0), args.Error(1)
}

func (m *MockVectorAPIRepository) GetContextEmbeddingsBySequence(ctx context.Context, contextID string, startSeq int, endSeq int) ([]*repository.Embedding, error) {
	args := m.Called(ctx, contextID, startSeq, endSeq)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) UpdateEmbeddingImportance(ctx context.Context, embeddingID string, importance float64) error {
	args := m.Called(ctx, embeddingID, importance)
	return args.Error(0)
}

// Implement base Repository[Embedding] interface methods
func (m *MockVectorAPIRepository) Create(ctx context.Context, embedding *repository.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) Get(ctx context.Context, id string) (*repository.Embedding, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) List(ctx context.Context, filter vector.Filter) ([]*vector.Embedding, error) {
	args := m.Called(ctx, filter)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*vector.Embedding), args.Error(1)
}

func (m *MockVectorAPIRepository) Update(ctx context.Context, embedding *repository.Embedding) error {
	args := m.Called(ctx, embedding)
	return args.Error(0)
}

func (m *MockVectorAPIRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Tests

func TestSemanticContextManager_CreateContext(t *testing.T) {
	assert := assert.New(t)

	mockRepo := new(MockContextRepository)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(nil)

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo not needed for create
		nil, // embedding client not needed for create
		nil, // queue client
		nil, // lifecycle manager not needed for create
		nil, // logger
		nil, // encryption
	)

	req := &repository.CreateContextRequest{
		Name:      "test-context",
		AgentID:   "agent-123",
		SessionID: "session-456",
		Properties: map[string]interface{}{
			"test": "value",
		},
	}

	contextData, err := manager.CreateContext(context.Background(), req)

	assert.NoError(err)
	assert.NotNil(contextData)
	assert.Equal("test-context", contextData.Name)
	assert.Equal("agent-123", contextData.AgentID)
	assert.Equal("session-456", contextData.SessionID)
	assert.Equal("active", contextData.Status)
	assert.NotEmpty(contextData.ID)
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_CreateContext_RepositoryError(t *testing.T) {
	assert := assert.New(t)

	mockRepo := new(MockContextRepository)
	mockRepo.On("Create", mock.Anything, mock.Anything).Return(errors.New("database error"))

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	req := &repository.CreateContextRequest{
		Name:      "test-context",
		AgentID:   "agent-123",
		SessionID: "session-456",
	}

	contextData, err := manager.CreateContext(context.Background(), req)

	assert.Error(err)
	assert.Nil(contextData)
	assert.Contains(err.Error(), "failed to create context")
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_GetContext_Standard(t *testing.T) {
	assert := assert.New(t)

	expectedContext := &repository.Context{
		ID:        "ctx-123",
		Name:      "test-context",
		AgentID:   "agent-123",
		SessionID: "session-456",
		Status:    "active",
	}

	mockRepo := new(MockContextRepository)
	mockRepo.On("Get", mock.Anything, "ctx-123").Return(expectedContext, nil)

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	// Get without semantic options
	contextData, err := manager.GetContext(context.Background(), "ctx-123", nil)

	assert.NoError(err)
	assert.NotNil(contextData)
	assert.Equal("ctx-123", contextData.ID)
	assert.Equal("test-context", contextData.Name)
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_UpdateContext(t *testing.T) {
	assert := assert.New(t)

	mockContextRepo := new(MockContextRepository)
	mockContextRepo.On("AddContextItem", mock.Anything, "ctx-123", mock.Anything).Return(nil)
	mockContextRepo.On("GetContextItems", mock.Anything, "ctx-123").Return([]*repository.ContextItem{}, nil)

	manager := core.NewSemanticContextManager(
		mockContextRepo,
		nil, // embedding repo not needed if embedding fails
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	update := &repository.ContextUpdate{
		Role:    "user",
		Content: "test content",
		Metadata: map[string]interface{}{
			"importance_score": 0.8,
		},
	}

	err := manager.UpdateContext(context.Background(), "ctx-123", update)

	assert.NoError(err)
	mockContextRepo.AssertExpectations(t)
}

func TestSemanticContextManager_DeleteContext(t *testing.T) {
	assert := assert.New(t)

	mockContextRepo := new(MockContextRepository)
	mockContextRepo.On("Delete", mock.Anything, "ctx-123").Return(nil)

	mockEmbeddingRepo := new(MockVectorAPIRepository)
	mockEmbeddingRepo.On("DeleteContextEmbeddings", mock.Anything, "ctx-123").Return(nil)

	manager := core.NewSemanticContextManager(
		mockContextRepo,
		mockEmbeddingRepo,
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	err := manager.DeleteContext(context.Background(), "ctx-123")

	assert.NoError(err)
	mockContextRepo.AssertExpectations(t)
	mockEmbeddingRepo.AssertExpectations(t)
}

func TestSemanticContextManager_SearchContext(t *testing.T) {
	assert := assert.New(t)

	expectedItems := []repository.ContextItem{
		{
			ID:        "item-1",
			ContextID: "ctx-123",
			Content:   "test content 1",
			Type:      "user",
		},
		{
			ID:        "item-2",
			ContextID: "ctx-123",
			Content:   "test content 2",
			Type:      "assistant",
		},
	}

	mockRepo := new(MockContextRepository)
	mockRepo.On("Search", mock.Anything, "ctx-123", "test query").Return(expectedItems, nil)

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	items, err := manager.SearchContext(context.Background(), "test query", "ctx-123", 10)

	assert.NoError(err)
	assert.Len(items, 2)
	assert.Equal("item-1", items[0].ID)
	assert.Equal("item-2", items[1].ID)
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_CompactContext(t *testing.T) {
	assert := assert.New(t)

	mockRepo := new(MockContextRepository)
	mockRepo.On("UpdateCompactionMetadata", mock.Anything, "ctx-123", "summarize", mock.Anything).Return(nil)

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	err := manager.CompactContext(context.Background(), "ctx-123", repository.CompactionSummarize)

	assert.NoError(err)
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_ValidateContextIntegrity(t *testing.T) {
	assert := assert.New(t)

	existingContext := &repository.Context{
		ID:     "ctx-123",
		Name:   "test-context",
		Status: "active",
	}

	mockRepo := new(MockContextRepository)
	mockRepo.On("Get", mock.Anything, "ctx-123").Return(existingContext, nil)

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	err := manager.ValidateContextIntegrity(context.Background(), "ctx-123")

	assert.NoError(err)
	mockRepo.AssertExpectations(t)
}

func TestSemanticContextManager_ValidateContextIntegrity_NotFound(t *testing.T) {
	assert := assert.New(t)

	mockRepo := new(MockContextRepository)
	mockRepo.On("Get", mock.Anything, "ctx-999").Return(nil, errors.New("context not found"))

	manager := core.NewSemanticContextManager(
		mockRepo,
		nil, // embedding repo
		nil, // embedding client
		nil, // queue client
		nil, // lifecycle manager
		nil, // logger
		nil, // encryption
	)

	err := manager.ValidateContextIntegrity(context.Background(), "ctx-999")

	assert.Error(err)
	assert.Contains(err.Error(), "context integrity check failed")
	mockRepo.AssertExpectations(t)
}
