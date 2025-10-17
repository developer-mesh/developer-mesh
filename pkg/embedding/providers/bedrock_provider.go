package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

// BedrockProvider implements Provider interface for AWS Bedrock
type BedrockProvider struct {
	config ProviderConfig
	client *bedrockruntime.Client
	models map[string]ModelInfo
}

// BedrockRequest/Response types for different models

// Titan embedding request
type titanEmbeddingRequest struct {
	InputText string `json:"inputText"`
}

type titanEmbeddingResponse struct {
	Embedding           []float32 `json:"embedding"`
	InputTextTokenCount int       `json:"inputTextTokenCount"`
}

// Cohere embedding request
type cohereEmbeddingRequest struct {
	Texts     []string `json:"texts"`
	InputType string   `json:"input_type"`
}

type cohereEmbeddingResponse struct {
	Embeddings   [][]float32 `json:"embeddings"`
	ID           string      `json:"id"`
	ResponseType string      `json:"response_type"`
	Meta         struct {
		BilledUnits struct {
			InputTokens int `json:"input_tokens"`
		} `json:"billed_units"`
	} `json:"meta"`
}

// NewBedrockProvider creates a new AWS Bedrock provider
func NewBedrockProvider(providerConfig ProviderConfig) (*BedrockProvider, error) {
	// Load AWS configuration with HTTP client timeout
	cfg, err := config.LoadDefaultConfig(context.Background(),
		config.WithRegion(providerConfig.Region),
		config.WithHTTPClient(&http.Client{
			Timeout: 30 * time.Second, // Overall request timeout
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
				TLSHandshakeTimeout: 10 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second, // Connection timeout
					KeepAlive: 30 * time.Second,
				}).DialContext,
			},
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	// Create Bedrock Runtime client
	client := bedrockruntime.NewFromConfig(cfg)

	p := &BedrockProvider{
		config: providerConfig,
		client: client,
	}

	// Initialize supported models
	p.models = map[string]ModelInfo{
		"amazon.titan-embed-text-v1": {
			Name:               "amazon.titan-embed-text-v1",
			DisplayName:        "Amazon Titan Text Embeddings v1",
			Dimensions:         1536,
			MaxTokens:          8192,
			CostPer1MTokens:    0.02,
			SupportedTaskTypes: []string{"default", "general_qa"},
			IsActive:           true,
		},
		"amazon.titan-embed-text-v2:0": {
			Name:                       "amazon.titan-embed-text-v2:0",
			DisplayName:                "Amazon Titan Text Embeddings v2",
			Dimensions:                 1024,
			MaxTokens:                  8192,
			CostPer1MTokens:            0.02,
			SupportedTaskTypes:         []string{"default", "general_qa", "search"},
			SupportsDimensionReduction: true,
			MinDimensions:              256,
			IsActive:                   true,
		},
		"cohere.embed-english-v3": {
			Name:               "cohere.embed-english-v3",
			DisplayName:        "Cohere Embed English v3",
			Dimensions:         1024,
			MaxTokens:          0, // Cohere doesn't specify max tokens
			CostPer1MTokens:    0.10,
			SupportedTaskTypes: []string{"search_document", "search_query", "classification", "clustering"},
			IsActive:           true,
		},
		"cohere.embed-multilingual-v3": {
			Name:               "cohere.embed-multilingual-v3",
			DisplayName:        "Cohere Embed Multilingual v3",
			Dimensions:         1024,
			MaxTokens:          0,
			CostPer1MTokens:    0.10,
			SupportedTaskTypes: []string{"search_document", "search_query", "classification", "clustering", "multilingual"},
			IsActive:           true,
		},
	}

	// Anthropic via Bedrock support (Claude 3 models)
	// Note: Claude models don't natively support embeddings, but can be used with specific prompting
	p.models["anthropic.claude-3-sonnet-20240229-v1:0"] = ModelInfo{
		Name:               "anthropic.claude-3-sonnet-20240229-v1:0",
		DisplayName:        "Claude 3 Sonnet (via synthetic embeddings)",
		Dimensions:         1536, // Synthetic dimension
		MaxTokens:          200000,
		CostPer1MTokens:    3.00, // Input pricing
		SupportedTaskTypes: []string{"synthetic", "research"},
		IsActive:           false, // Disabled by default due to cost
	}

	return p, nil
}

// Name returns the provider name
func (p *BedrockProvider) Name() string {
	return "bedrock"
}

// GenerateEmbedding generates an embedding for the given text
func (p *BedrockProvider) GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingResponse, error) {
	model, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	// Route to appropriate handler based on model
	var resp *EmbeddingResponse
	var lastErr error

	switch {
	case contains(req.Model, "titan"):
		resp, lastErr = p.generateTitanEmbedding(ctx, req, model)
	case contains(req.Model, "cohere"):
		resp, lastErr = p.generateCohereEmbedding(ctx, req, model)
	case contains(req.Model, "anthropic"):
		resp, lastErr = p.generateClaudeEmbedding(ctx, req, model)
	default:
		return nil, fmt.Errorf("unsupported model: %s", req.Model)
	}

	if lastErr != nil {
		return nil, lastErr
	}

	resp.ProviderInfo.LatencyMs = time.Since(start).Milliseconds()
	return resp, nil
}

// BatchGenerateEmbeddings generates embeddings for multiple texts
func (p *BedrockProvider) BatchGenerateEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest) (*BatchEmbeddingResponse, error) {
	model, err := p.GetModel(req.Model)
	if err != nil {
		return nil, err
	}

	// Cohere supports batch natively
	if contains(req.Model, "cohere") {
		return p.batchGenerateCohereEmbeddings(ctx, req, model)
	}

	// For other models, process sequentially
	embeddings := make([][]float32, len(req.Texts))
	totalTokens := 0

	for i, text := range req.Texts {
		resp, err := p.GenerateEmbedding(ctx, GenerateEmbeddingRequest{
			Text:      text,
			Model:     req.Model,
			Metadata:  req.Metadata,
			RequestID: req.RequestID,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to generate embedding %d: %w", i, err)
		}
		embeddings[i] = resp.Embedding
		totalTokens += resp.TokensUsed
	}

	return &BatchEmbeddingResponse{
		Embeddings:  embeddings,
		Model:       req.Model,
		Dimensions:  model.Dimensions,
		TotalTokens: totalTokens,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider: "bedrock",
		},
	}, nil
}

// GetSupportedModels returns the list of supported models
func (p *BedrockProvider) GetSupportedModels() []ModelInfo {
	models := make([]ModelInfo, 0, len(p.models))
	for _, model := range p.models {
		if model.IsActive {
			models = append(models, model)
		}
	}
	return models
}

// GetModel returns information about a specific model
func (p *BedrockProvider) GetModel(modelName string) (ModelInfo, error) {
	model, exists := p.models[modelName]
	if !exists {
		return ModelInfo{}, &ProviderError{
			Provider:   "bedrock",
			Code:       "MODEL_NOT_FOUND",
			Message:    fmt.Sprintf("model %s not found", modelName),
			StatusCode: 404,
		}
	}
	return model, nil
}

// HealthCheck verifies the provider is accessible
func (p *BedrockProvider) HealthCheck(ctx context.Context) error {
	// Perform a lightweight health check by attempting to invoke with minimal payload
	// Use Titan V2 model which is our primary model
	titanReq := titanEmbeddingRequest{
		InputText: "health",
	}

	requestBody, err := json.Marshal(titanReq)
	if err != nil {
		return fmt.Errorf("failed to marshal health check request: %w", err)
	}

	_, err = p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String("amazon.titan-embed-text-v2:0"),
		ContentType: aws.String("application/json"),
		Body:        requestBody,
	})

	if err != nil {
		errStr := err.Error()

		// Only fail for actual connectivity/authentication issues
		// Success scenarios that shouldn't fail health check:
		// - Model invocation succeeded (no error)
		// - Model-specific errors (model not found, validation errors, etc.)

		// Fail scenarios (real health issues):
		if contains(errStr, "AccessDeniedException") ||
			contains(errStr, "UnauthorizedClient") ||
			contains(errStr, "ExpiredToken") ||
			contains(errStr, "InvalidSignature") ||
			contains(errStr, "no valid credentials") {
			return fmt.Errorf("bedrock authentication failed: %s", errStr)
		}

		// Network/connectivity issues
		if contains(errStr, "connection") ||
			contains(errStr, "timeout") ||
			contains(errStr, "network") {
			return fmt.Errorf("bedrock connectivity issue: %s", errStr)
		}

		// For other errors (like model-specific issues), log but consider healthy
		// This prevents false negatives where credentials are valid but request format has issues
		// The actual embedding generation will surface real problems
	}

	return nil
}

// Close cleans up resources
func (p *BedrockProvider) Close() error {
	// AWS SDK doesn't require explicit cleanup
	return nil
}

// Private methods

func (p *BedrockProvider) generateTitanEmbedding(ctx context.Context, req GenerateEmbeddingRequest, model ModelInfo) (*EmbeddingResponse, error) {
	// Prepare request
	titanReq := titanEmbeddingRequest{
		InputText: req.Text,
	}

	requestBody, err := json.Marshal(titanReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Invoke model
	resp, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(req.Model),
		ContentType: aws.String("application/json"),
		Body:        requestBody,
	})
	if err != nil {
		return nil, &ProviderError{
			Provider:    "bedrock",
			Code:        "INVOCATION_ERROR",
			Message:     err.Error(),
			IsRetryable: isRetryableBedrockError(err),
		}
	}

	// Parse response
	var titanResp titanEmbeddingResponse
	if err := json.Unmarshal(resp.Body, &titanResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &EmbeddingResponse{
		Embedding:  titanResp.Embedding,
		Model:      req.Model,
		Dimensions: len(titanResp.Embedding),
		TokensUsed: titanResp.InputTextTokenCount,
		Metadata:   req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider: "bedrock",
		},
	}, nil
}

func (p *BedrockProvider) generateCohereEmbedding(ctx context.Context, req GenerateEmbeddingRequest, model ModelInfo) (*EmbeddingResponse, error) {
	// Determine input type from metadata or default
	inputType := "search_document"
	if it, ok := req.Metadata["input_type"].(string); ok {
		inputType = it
	}

	// Prepare request
	cohereReq := cohereEmbeddingRequest{
		Texts:     []string{req.Text},
		InputType: inputType,
	}

	requestBody, err := json.Marshal(cohereReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Invoke model
	resp, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(req.Model),
		ContentType: aws.String("application/json"),
		Body:        requestBody,
	})
	if err != nil {
		return nil, &ProviderError{
			Provider:    "bedrock",
			Code:        "INVOCATION_ERROR",
			Message:     err.Error(),
			IsRetryable: isRetryableBedrockError(err),
		}
	}

	// Parse response
	var cohereResp cohereEmbeddingResponse
	if err := json.Unmarshal(resp.Body, &cohereResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(cohereResp.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings in response")
	}

	return &EmbeddingResponse{
		Embedding:  cohereResp.Embeddings[0],
		Model:      req.Model,
		Dimensions: len(cohereResp.Embeddings[0]),
		TokensUsed: cohereResp.Meta.BilledUnits.InputTokens,
		Metadata:   req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider: "bedrock",
		},
	}, nil
}

func (p *BedrockProvider) generateClaudeEmbedding(ctx context.Context, req GenerateEmbeddingRequest, model ModelInfo) (*EmbeddingResponse, error) {
	// Claude doesn't support embeddings natively
	// This would implement a synthetic embedding approach using Claude's understanding
	return nil, fmt.Errorf("claude embedding generation not yet implemented")
}

func (p *BedrockProvider) batchGenerateCohereEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest, model ModelInfo) (*BatchEmbeddingResponse, error) {
	// Determine input type
	inputType := "search_document"
	if it, ok := req.Metadata["input_type"].(string); ok {
		inputType = it
	}

	// Prepare request
	cohereReq := cohereEmbeddingRequest{
		Texts:     req.Texts,
		InputType: inputType,
	}

	requestBody, err := json.Marshal(cohereReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Invoke model
	resp, err := p.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(req.Model),
		ContentType: aws.String("application/json"),
		Body:        requestBody,
	})
	if err != nil {
		return nil, &ProviderError{
			Provider:    "bedrock",
			Code:        "INVOCATION_ERROR",
			Message:     err.Error(),
			IsRetryable: isRetryableBedrockError(err),
		}
	}

	// Parse response
	var cohereResp cohereEmbeddingResponse
	if err := json.Unmarshal(resp.Body, &cohereResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &BatchEmbeddingResponse{
		Embeddings:  cohereResp.Embeddings,
		Model:       req.Model,
		Dimensions:  model.Dimensions,
		TotalTokens: cohereResp.Meta.BilledUnits.InputTokens,
		Metadata:    req.Metadata,
		ProviderInfo: ProviderMetadata{
			Provider: "bedrock",
		},
	}, nil
}

func isRetryableBedrockError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	retryableErrors := []string{
		"ThrottlingException",
		"ServiceUnavailable",
		"TooManyRequests",
		"RequestTimeout",
		"ModelStreamErrorException",
		"ModelTimeoutException",
	}

	for _, retryable := range retryableErrors {
		if bytes.Contains([]byte(errStr), []byte(retryable)) {
			return true
		}
	}

	return false
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && bytes.Contains([]byte(s), []byte(substr))
}
