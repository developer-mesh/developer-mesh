package embedding_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// MockProvider implements the providers.Provider interface for testing
type MockProvider struct {
	mock.Mock
}

func (m *MockProvider) Name() string {
	args := m.Called()
	return args.String(0)
}

func (m *MockProvider) GenerateEmbedding(ctx context.Context, req providers.GenerateEmbeddingRequest) (*providers.EmbeddingResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providers.EmbeddingResponse), args.Error(1)
}

func (m *MockProvider) BatchGenerateEmbeddings(ctx context.Context, req providers.BatchGenerateEmbeddingRequest) (*providers.BatchEmbeddingResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*providers.BatchEmbeddingResponse), args.Error(1)
}

func (m *MockProvider) GetSupportedModels() []providers.ModelInfo {
	args := m.Called()
	return args.Get(0).([]providers.ModelInfo)
}

func (m *MockProvider) GetModel(modelName string) (providers.ModelInfo, error) {
	args := m.Called(modelName)
	return args.Get(0).(providers.ModelInfo), args.Error(1)
}

func (m *MockProvider) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockProvider) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockLogger implements observability.Logger for testing
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

// Test NewContextEmbeddingClient
func TestNewContextEmbeddingClient(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	assert.NotNil(client)
}

// Test RegisterProvider
func TestContextEmbeddingClient_RegisterProvider(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	mockProvider := new(MockProvider)
	mockProvider.On("Name").Return("test-provider")

	client.RegisterProvider("test-provider", mockProvider)

	// Verify by checking GetProviderInfo
	mockProvider.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "test-model", Dimensions: 1536},
	})

	info := client.GetProviderInfo()
	assert.Contains(info, "test-provider")
	assert.Len(info["test-provider"], 1)
}

// Test SelectModel with code content
func TestContextEmbeddingClient_SelectModel_CodeContent(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	// Register provider with code model
	mockProvider := new(MockProvider)
	mockProvider.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "voyage-code-3", Dimensions: 1024},
	})
	mockProvider.On("GetModel", "voyage-code-3").Return(
		providers.ModelInfo{Name: "voyage-code-3", Dimensions: 1024},
		nil,
	)

	client.RegisterProvider("voyage", mockProvider)

	// Test with code content
	codeContent := "```python\nprint('hello')\n```"
	model := client.SelectModel(codeContent)

	assert.Equal("voyage-code-3", model)
}

// Test SelectModel with regular text
func TestContextEmbeddingClient_SelectModel_RegularText(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	// Register provider with text model
	mockProvider := new(MockProvider)
	mockProvider.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "text-embedding-3-small", Dimensions: 1536},
	})
	mockProvider.On("GetModel", "text-embedding-3-small").Return(
		providers.ModelInfo{Name: "text-embedding-3-small", Dimensions: 1536},
		nil,
	)

	client.RegisterProvider("openai", mockProvider)

	// Test with regular text
	model := client.SelectModel("This is regular text content")

	assert.Equal("text-embedding-3-small", model)
}

// Test SelectModel with no providers
func TestContextEmbeddingClient_SelectModel_NoProviders(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	model := client.SelectModel("any content")

	assert.Empty(model)
}

// Test EmbedContent success
func TestContextEmbeddingClient_EmbedContent_Success(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	// Register provider
	mockProvider := new(MockProvider)
	mockProvider.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "text-embedding-3-small", Dimensions: 1536},
	}).Maybe()
	mockProvider.On("GetModel", "text-embedding-3-small").Return(
		providers.ModelInfo{Name: "text-embedding-3-small", Dimensions: 1536},
		nil,
	).Maybe()

	expectedEmbedding := make([]float32, 1536)
	for i := range expectedEmbedding {
		expectedEmbedding[i] = 0.1
	}

	mockProvider.On("GenerateEmbedding", mock.Anything, mock.MatchedBy(func(req providers.GenerateEmbeddingRequest) bool {
		return req.Text == "test content" && req.Model == "text-embedding-3-small"
	})).Return(&providers.EmbeddingResponse{
		Embedding:  expectedEmbedding,
		Model:      "text-embedding-3-small",
		Dimensions: 1536,
		TokensUsed: 10,
		ProviderInfo: providers.ProviderMetadata{
			Provider:  "openai",
			LatencyMs: 100,
		},
	}, nil)

	client.RegisterProvider("openai", mockProvider)

	// Test embedding generation
	ctx := context.Background()
	embedding, model, err := client.EmbedContent(ctx, "test content", "")

	assert.NoError(err)
	assert.Equal("text-embedding-3-small", model)
	assert.Len(embedding, 1536)
	assert.Equal(float32(0.1), embedding[0])

	mockProvider.AssertExpectations(t)
}

// Test EmbedContent with model override
func TestContextEmbeddingClient_EmbedContent_ModelOverride(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	// Register provider
	mockProvider := new(MockProvider)
	mockProvider.On("GetModel", "voyage-3").Return(
		providers.ModelInfo{Name: "voyage-3", Dimensions: 1024},
		nil,
	)

	expectedEmbedding := make([]float32, 1024)
	mockProvider.On("GenerateEmbedding", mock.Anything, mock.MatchedBy(func(req providers.GenerateEmbeddingRequest) bool {
		return req.Model == "voyage-3"
	})).Return(&providers.EmbeddingResponse{
		Embedding:  expectedEmbedding,
		Model:      "voyage-3",
		Dimensions: 1024,
		TokensUsed: 5,
		ProviderInfo: providers.ProviderMetadata{
			Provider:  "voyage",
			LatencyMs: 80,
		},
	}, nil)

	client.RegisterProvider("voyage", mockProvider)

	// Test with model override
	ctx := context.Background()
	embedding, model, err := client.EmbedContent(ctx, "test content", "voyage-3")

	assert.NoError(err)
	assert.Equal("voyage-3", model)
	assert.Len(embedding, 1024)

	mockProvider.AssertExpectations(t)
}

// Test EmbedContent with no model available
func TestContextEmbeddingClient_EmbedContent_NoModelAvailable(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	ctx := context.Background()
	_, _, err := client.EmbedContent(ctx, "test content", "")

	assert.Error(err)
	assert.Contains(err.Error(), "no embedding model available")
}

// Test BatchEmbedContent success
func TestContextEmbeddingClient_BatchEmbedContent_Success(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Debug", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	// Register provider
	mockProvider := new(MockProvider)
	mockProvider.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "text-embedding-3-small", Dimensions: 1536},
	}).Maybe()
	mockProvider.On("GetModel", "text-embedding-3-small").Return(
		providers.ModelInfo{Name: "text-embedding-3-small", Dimensions: 1536},
		nil,
	).Maybe()

	expectedEmbeddings := [][]float32{
		make([]float32, 1536),
		make([]float32, 1536),
		make([]float32, 1536),
	}

	mockProvider.On("BatchGenerateEmbeddings", mock.Anything, mock.MatchedBy(func(req providers.BatchGenerateEmbeddingRequest) bool {
		return len(req.Texts) == 3
	})).Return(&providers.BatchEmbeddingResponse{
		Embeddings:  expectedEmbeddings,
		Model:       "text-embedding-3-small",
		Dimensions:  1536,
		TotalTokens: 30,
		ProviderInfo: providers.ProviderMetadata{
			Provider:  "openai",
			LatencyMs: 200,
		},
	}, nil)

	client.RegisterProvider("openai", mockProvider)

	// Test batch embedding generation
	ctx := context.Background()
	contents := []string{"content 1", "content 2", "content 3"}
	embeddings, model, err := client.BatchEmbedContent(ctx, contents, "")

	assert.NoError(err)
	assert.Equal("text-embedding-3-small", model)
	assert.Len(embeddings, 3)
	assert.Len(embeddings[0], 1536)

	mockProvider.AssertExpectations(t)
}

// Test BatchEmbedContent with empty contents
func TestContextEmbeddingClient_BatchEmbedContent_EmptyContents(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	ctx := context.Background()
	_, _, err := client.BatchEmbedContent(ctx, []string{}, "")

	assert.Error(err)
	assert.Contains(err.Error(), "no content provided")
}

// Test ChunkContent with small content
func TestContextEmbeddingClient_ChunkContent_SmallContent(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	chunks := client.ChunkContent("small text", 100)

	assert.Len(chunks, 1)
	assert.Equal("small text", chunks[0])
}

// Test ChunkContent with large content
func TestContextEmbeddingClient_ChunkContent_LargeContent(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	// Create long text
	longText := strings.Repeat("word ", 500)
	chunks := client.ChunkContent(longText, 100)

	assert.Greater(len(chunks), 1)

	// Verify no chunk exceeds max size
	for _, chunk := range chunks {
		assert.LessOrEqual(len(chunk), 100)
	}
}

// Test ChunkContent with default chunk size
func TestContextEmbeddingClient_ChunkContent_DefaultSize(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	// Create text larger than default (1000)
	longText := strings.Repeat("word ", 500) // ~2500 chars
	chunks := client.ChunkContent(longText, 0)

	assert.Greater(len(chunks), 1)

	// Verify chunks use default size
	for _, chunk := range chunks {
		assert.LessOrEqual(len(chunk), 1000)
	}
}

// Test GetProviderInfo
func TestContextEmbeddingClient_GetProviderInfo(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	// Register multiple providers
	provider1 := new(MockProvider)
	provider1.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "model-1", Dimensions: 1536},
	})

	provider2 := new(MockProvider)
	provider2.On("GetSupportedModels").Return([]providers.ModelInfo{
		{Name: "model-2", Dimensions: 1024},
		{Name: "model-3", Dimensions: 768},
	})

	client.RegisterProvider("provider-1", provider1)
	client.RegisterProvider("provider-2", provider2)

	info := client.GetProviderInfo()

	assert.Len(info, 2)
	assert.Len(info["provider-1"], 1)
	assert.Len(info["provider-2"], 2)
	assert.Equal("model-1", info["provider-1"][0].Name)
	assert.Equal("model-2", info["provider-2"][0].Name)
}

// Test HealthCheck all providers healthy
func TestContextEmbeddingClient_HealthCheck_AllHealthy(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	client := embedding.NewContextEmbeddingClient(logger)

	provider1 := new(MockProvider)
	provider1.On("HealthCheck", mock.Anything).Return(nil)

	provider2 := new(MockProvider)
	provider2.On("HealthCheck", mock.Anything).Return(nil)

	client.RegisterProvider("provider-1", provider1)
	client.RegisterProvider("provider-2", provider2)

	ctx := context.Background()
	results := client.HealthCheck(ctx)

	assert.Empty(results)
	provider1.AssertExpectations(t)
	provider2.AssertExpectations(t)
}

// Test HealthCheck with unhealthy provider
func TestContextEmbeddingClient_HealthCheck_UnhealthyProvider(t *testing.T) {
	assert := assert.New(t)

	logger := new(MockLogger)
	logger.On("Warn", mock.Anything, mock.Anything).Maybe()
	client := embedding.NewContextEmbeddingClient(logger)

	provider1 := new(MockProvider)
	provider1.On("HealthCheck", mock.Anything).Return(nil)

	provider2 := new(MockProvider)
	provider2.On("HealthCheck", mock.Anything).Return(errors.New("provider unhealthy"))

	client.RegisterProvider("provider-1", provider1)
	client.RegisterProvider("provider-2", provider2)

	ctx := context.Background()
	results := client.HealthCheck(ctx)

	assert.Len(results, 1)
	assert.Contains(results, "provider-2")
	assert.Error(results["provider-2"])

	provider1.AssertExpectations(t)
	provider2.AssertExpectations(t)
}
