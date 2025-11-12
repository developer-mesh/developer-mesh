//go:build exclude_storage_tests

package providers

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockS3Client is a mock implementation of the S3 client
type MockS3Client struct {
	mock.Mock
}

func (m *MockS3Client) GetBucketName() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockS3Client) UploadFile(ctx context.Context, key string, data []byte, contentType string) error {
	args := m.Called(ctx, key, data, contentType)
	return args.Error(0)
}

func (m *MockS3Client) DownloadFile(ctx context.Context, key string) ([]byte, error) {
	args := m.Called(ctx, key)
	return args.Get(0).([]byte), args.Error(1)
}

func (m *MockS3Client) DeleteFile(ctx context.Context, key string) error {
	args := m.Called(ctx, key)
	return args.Error(0)
}

func (m *MockS3Client) ListFiles(ctx context.Context, prefix string) ([]string, error) {
	args := m.Called(ctx, prefix)
	return args.Get(0).([]string), args.Error(1)
}

func TestNewS3ContextStorage(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	storage := NewS3ContextStorage(mockClient, "test-prefix")

	assert.NotNil(t, storage)
	assert.Equal(t, "test-bucket", storage.bucketName)
	assert.Equal(t, "test-prefix", storage.prefix)

	mockClient.AssertExpectations(t)
}

func TestStoreContext(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	ctx := context.Background()
	contextData := &models.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []models.ContextItem{
			{
				Role:    "system",
				Content: "Test content",
				Tokens:  10,
			},
		},
		CurrentTokens: 10,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Test successful upload
	jsonData, _ := json.Marshal(contextData)
	mockClient.On("UploadFile", ctx, "test-prefix/test-id.json", mock.AnythingOfType("[]uint8"), "application/json").Return(nil).Once()

	storage := NewS3ContextStorage(mockClient, "test-prefix")
	err := storage.StoreContext(ctx, contextData)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test upload failure
	mockClient.On("UploadFile", ctx, "test-prefix/test-id.json", mock.AnythingOfType("[]uint8"), "application/json").Return(errors.New("upload error")).Once()

	err = storage.StoreContext(ctx, contextData)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to upload context to S3")
	mockClient.AssertExpectations(t)
}

func TestGetContext(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	ctx := context.Background()
	contextData := &models.Context{
		ID:      "test-id",
		AgentID: "test-agent",
		ModelID: "test-model",
		Content: []models.ContextItem{
			{
				Role:    "system",
				Content: "Test content",
				Tokens:  10,
			},
		},
		CurrentTokens: 10,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	// Test successful download
	jsonData, _ := json.Marshal(contextData)
	mockClient.On("DownloadFile", ctx, "test-prefix/test-id.json").Return(jsonData, nil).Once()

	storage := NewS3ContextStorage(mockClient, "test-prefix")
	result, err := storage.GetContext(ctx, "test-id")

	assert.NoError(t, err)
	assert.Equal(t, contextData.ID, result.ID)
	assert.Equal(t, contextData.AgentID, result.AgentID)
	assert.Equal(t, contextData.ModelID, result.ModelID)
	mockClient.AssertExpectations(t)

	// Test download failure
	mockClient.On("DownloadFile", ctx, "test-prefix/test-id.json").Return([]byte{}, errors.New("download error")).Once()

	result, err = storage.GetContext(ctx, "test-id")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to download context from S3")
	mockClient.AssertExpectations(t)

	// Test unmarshal failure
	mockClient.On("DownloadFile", ctx, "test-prefix/test-id.json").Return([]byte("invalid json"), nil).Once()

	result, err = storage.GetContext(ctx, "test-id")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to deserialize context data")
	mockClient.AssertExpectations(t)
}

func TestDeleteContext(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	ctx := context.Background()

	// Test successful delete
	mockClient.On("DeleteFile", ctx, "test-prefix/test-id.json").Return(nil).Once()

	storage := NewS3ContextStorage(mockClient, "test-prefix")
	err := storage.DeleteContext(ctx, "test-id")

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)

	// Test delete failure
	mockClient.On("DeleteFile", ctx, "test-prefix/test-id.json").Return(errors.New("delete error")).Once()

	err = storage.DeleteContext(ctx, "test-id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to delete context from S3")
	mockClient.AssertExpectations(t)
}

func TestExtractContextID(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	storage := NewS3ContextStorage(mockClient, "test-prefix")

	// Test valid cases
	assert.Equal(t, "context1", storage.extractContextID("test-prefix/context1.json"))
	assert.Equal(t, "nested/context2", storage.extractContextID("test-prefix/nested/context2.json"))

	// Test edge cases
	assert.Equal(t, "", storage.extractContextID("different-prefix/context.json")) // Different prefix
	assert.Equal(t, "", storage.extractContextID("test-prefix"))                   // No trailing slash
	assert.Equal(t, "", storage.extractContextID("test-prefix/"))                  // Empty filename
	assert.Equal(t, "", storage.extractContextID("test-prefix/file.txt"))          // Not a .json file
	assert.Equal(t, "", storage.extractContextID(""))                              // Empty string

	mockClient.AssertExpectations(t)
}

func TestListContexts(t *testing.T) {
	mockClient := new(MockS3Client)
	mockClient.On("GetBucketName").Return("test-bucket")

	ctx := context.Background()

	// Test listing with no agent ID
	mockClient.On("ListFiles", ctx, "test-prefix").Return([]string{
		"test-prefix/context1.json",
		"test-prefix/context2.json",
	}, nil).Once()

	// Mock getting context1
	contextData1 := &mcp.Context{
		ID:      "context1",
		AgentID: "agent1",
		ModelID: "model1",
	}
	jsonData1, _ := json.Marshal(contextData1)
	mockClient.On("DownloadFile", ctx, "test-prefix/context1.json").Return(jsonData1, nil).Once()

	// Mock getting context2
	contextData2 := &mcp.Context{
		ID:      "context2",
		AgentID: "agent2",
		ModelID: "model2",
	}
	jsonData2, _ := json.Marshal(contextData2)
	mockClient.On("DownloadFile", ctx, "test-prefix/context2.json").Return(jsonData2, nil).Once()

	storage := NewS3ContextStorage(mockClient, "test-prefix")
	results, err := storage.ListContexts(ctx, "", "")

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	assert.Equal(t, "context1", results[0].ID)
	assert.Equal(t, "context2", results[1].ID)
	mockClient.AssertExpectations(t)

	// Test listing with agent ID
	mockClient.On("ListFiles", ctx, "test-prefix/agent/agent1").Return([]string{
		"test-prefix/agent/agent1/context3.json",
	}, nil).Once()

	// Mock getting context3
	contextData3 := &models.Context{
		ID:        "context3",
		AgentID:   "agent1",
		ModelID:   "model1",
		SessionID: "session1",
	}
	jsonData3, _ := json.Marshal(contextData3)
	mockClient.On("DownloadFile", ctx, "test-prefix/agent/agent1/context3.json").Return(jsonData3, nil).Once()

	results, err = storage.ListContexts(ctx, "agent1", "")

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "context3", results[0].ID)
	mockClient.AssertExpectations(t)

	// Test when list fails
	mockClient.On("ListFiles", ctx, "test-prefix").Return([]string{}, errors.New("list error")).Once()

	results, err = storage.ListContexts(ctx, "", "")
	assert.Error(t, err)
	assert.Nil(t, results)
	assert.Contains(t, err.Error(), "failed to list contexts from S3")
	mockClient.AssertExpectations(t)

	// Test session ID filtering
	mockClient.On("ListFiles", ctx, "test-prefix").Return([]string{
		"test-prefix/context4.json",
		"test-prefix/context5.json",
	}, nil).Once()

	// Mock contexts with different session IDs
	contextData4 := &models.Context{
		ID:        "context4",
		AgentID:   "agent1",
		ModelID:   "model1",
		SessionID: "session1",
	}
	jsonData4, _ := json.Marshal(contextData4)
	mockClient.On("DownloadFile", ctx, "test-prefix/context4.json").Return(jsonData4, nil).Once()

	contextData5 := &models.Context{
		ID:        "context5",
		AgentID:   "agent1",
		ModelID:   "model1",
		SessionID: "session2",
	}
	jsonData5, _ := json.Marshal(contextData5)
	mockClient.On("DownloadFile", ctx, "test-prefix/context5.json").Return(jsonData5, nil).Once()

	results, err = storage.ListContexts(ctx, "", "session1")

	assert.NoError(t, err)
	assert.Len(t, results, 1)
	assert.Equal(t, "context4", results[0].ID)
	mockClient.AssertExpectations(t)
}
