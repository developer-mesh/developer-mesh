package core_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/core"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockLogger is a mock implementation of Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Info(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Fatal(msg string, fields map[string]interface{}) {
	m.Called(msg, fields)
}

func (m *MockLogger) Debugf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Infof(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Warnf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Errorf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) Fatalf(format string, args ...interface{}) {
	m.Called(format, args)
}

func (m *MockLogger) WithPrefix(prefix string) observability.Logger {
	args := m.Called(prefix)
	return args.Get(0).(observability.Logger)
}

func (m *MockLogger) With(fields map[string]interface{}) observability.Logger {
	args := m.Called(fields)
	return args.Get(0).(observability.Logger)
}

// TestCompactionExecutor_ToolClear tests the tool clear compaction strategy
func TestCompactionExecutor_ToolClear(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Create test items
	items := []*repository.ContextItem{
		{
			ID:        "item-1",
			ContextID: "ctx-123",
			Type:      "user",
			Content:   "User message",
		},
		{
			ID:        "item-2",
			ContextID: "ctx-123",
			Type:      "tool_result",
			Content:   "Large tool result that should be cleared",
			Metadata: map[string]interface{}{
				"tool_name": "github_get_repo",
			},
		},
	}

	// Add 10 more items to make tool result old
	for i := 3; i < 15; i++ {
		items = append(items, &repository.ContextItem{
			ID:        fmt.Sprintf("item-%d", i),
			ContextID: "ctx-123",
			Type:      "user",
			Content:   fmt.Sprintf("Message %d", i),
		})
	}

	// Setup expectations
	mockRepo.On("GetContextItems", mock.Anything, "ctx-123").Return(items, nil)
	mockRepo.On("UpdateContextItem", mock.Anything, mock.MatchedBy(func(item *repository.ContextItem) bool {
		return item.ID == "item-2" && item.Metadata["compacted"] == true
	})).Return(nil)
	mockRepo.On("UpdateCompactionMetadata", mock.Anything, "ctx-123", "tool_clear", mock.Anything).Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute compaction
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionToolClear)

	// Assertions
	assert.NoError(err)
	mockRepo.AssertExpectations(t)
	mockLogger.AssertCalled(t, "Info", "Tool clear compaction completed", mock.Anything)
}

// TestCompactionExecutor_Prune tests the prune compaction strategy
func TestCompactionExecutor_Prune(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Create test items with low importance
	items := []*repository.ContextItem{
		{
			ID:        "item-1",
			ContextID: "ctx-123",
			Type:      "user",
			Content:   "Low importance message",
			Metadata: map[string]interface{}{
				"embedding_id": "emb-1",
			},
		},
		{
			ID:        "item-2",
			ContextID: "ctx-123",
			Type:      "error",
			Content:   "Important error",
			Metadata: map[string]interface{}{
				"embedding_id": "emb-2",
			},
		},
	}

	// Create embedding links with importance scores
	links := []repository.ContextEmbeddingLink{
		{
			EmbeddingID:     "emb-1",
			ImportanceScore: 0.2, // Below threshold
		},
		{
			EmbeddingID:     "emb-2",
			ImportanceScore: 0.8, // High importance
		},
	}

	// Setup expectations
	mockRepo.On("GetContextItems", mock.Anything, "ctx-123").Return(items, nil)
	mockRepo.On("GetContextEmbeddingLinks", mock.Anything, "ctx-123").Return(links, nil)
	mockRepo.On("Delete", mock.Anything, "item-1").Return(nil)
	mockRepo.On("UpdateCompactionMetadata", mock.Anything, "ctx-123", "prune", mock.Anything).Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute compaction
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionPrune)

	// Assertions
	assert.NoError(err)
	mockRepo.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Delete", mock.Anything, "item-2") // Error should not be deleted
}

// TestCompactionExecutor_Sliding tests the sliding window compaction strategy
func TestCompactionExecutor_Sliding(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Create 30 items (should compact first 10, keep last 20)
	items := make([]*repository.ContextItem, 30)
	for i := 0; i < 30; i++ {
		items[i] = &repository.ContextItem{
			ID:        fmt.Sprintf("item-%d", i),
			ContextID: "ctx-123",
			Type:      "user",
			Content:   fmt.Sprintf("Message with content that is longer than 100 characters to test truncation behavior %d", i),
		}
	}

	// Setup expectations
	mockRepo.On("GetContextItems", mock.Anything, "ctx-123").Return(items, nil)

	// Expect updates for first 10 items only
	for i := 0; i < 10; i++ {
		mockRepo.On("UpdateContextItem", mock.Anything, mock.MatchedBy(func(item *repository.ContextItem) bool {
			return item.ID == fmt.Sprintf("item-%d", i) &&
				item.Metadata["compacted"] == true &&
				item.Metadata["compaction_strategy"] == "sliding"
		})).Return(nil)
	}

	mockRepo.On("UpdateCompactionMetadata", mock.Anything, "ctx-123", "sliding", mock.Anything).Return(nil)
	mockLogger.On("Info", mock.Anything, mock.Anything).Return()

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute compaction
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionSliding)

	// Assertions
	assert.NoError(err)
	mockRepo.AssertExpectations(t)
}

// TestCompactionExecutor_Summarize tests the summarize placeholder
func TestCompactionExecutor_Summarize(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Setup expectations
	mockRepo.On("UpdateCompactionMetadata", mock.Anything, "ctx-123", "summarize", mock.Anything).Return(nil)
	mockLogger.On("Info", "Summarize compaction placeholder", mock.Anything).Return()

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute compaction
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionSummarize)

	// Assertions
	assert.NoError(err)
	mockRepo.AssertExpectations(t)
	mockLogger.AssertCalled(t, "Info", "Summarize compaction placeholder", mock.Anything)
}

// TestCompactionExecutor_InvalidStrategy tests error handling for invalid strategy
func TestCompactionExecutor_InvalidStrategy(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute with invalid strategy
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionStrategy("invalid"))

	// Assertions
	assert.Error(err)
	assert.Contains(err.Error(), "unknown compaction strategy")
}

// TestCompactionExecutor_SlidingNoCompactionNeeded tests sliding when no compaction is needed
func TestCompactionExecutor_SlidingNoCompactionNeeded(t *testing.T) {
	assert := assert.New(t)

	// Setup mocks
	mockRepo := new(MockContextRepository)
	mockEmbedding := new(MockVectorAPIRepository)
	mockLogger := new(MockLogger)

	// Create only 15 items (less than window size of 20)
	items := make([]*repository.ContextItem, 15)
	for i := 0; i < 15; i++ {
		items[i] = &repository.ContextItem{
			ID:        fmt.Sprintf("item-%d", i),
			ContextID: "ctx-123",
			Type:      "user",
			Content:   fmt.Sprintf("Message %d", i),
		}
	}

	// Setup expectations
	mockRepo.On("GetContextItems", mock.Anything, "ctx-123").Return(items, nil)

	// Create executor
	executor := core.NewCompactionExecutor(mockRepo, mockEmbedding, mockLogger)

	// Execute compaction
	err := executor.ExecuteCompaction(context.Background(), "ctx-123", repository.CompactionSliding)

	// Assertions
	assert.NoError(err)
	mockRepo.AssertNotCalled(t, "UpdateContextItem", mock.Anything, mock.Anything)
}
