package providers

import (
	"context"
	"fmt"
	"time"
)

// Provider represents an embedding provider (OpenAI, Bedrock, etc.)
type Provider interface {
	// Name returns the provider name (e.g., "openai", "bedrock")
	Name() string

	// GenerateEmbedding generates an embedding for the given text using the specified model
	GenerateEmbedding(ctx context.Context, req GenerateEmbeddingRequest) (*EmbeddingResponse, error)

	// BatchGenerateEmbeddings generates embeddings for multiple texts
	BatchGenerateEmbeddings(ctx context.Context, req BatchGenerateEmbeddingRequest) (*BatchEmbeddingResponse, error)

	// GetSupportedModels returns the list of models supported by this provider
	GetSupportedModels() []ModelInfo

	// GetModel returns information about a specific model
	GetModel(modelName string) (ModelInfo, error)

	// HealthCheck verifies the provider is accessible and functioning
	HealthCheck(ctx context.Context) error

	// Close cleans up any resources (connections, clients, etc.)
	Close() error
}

// GenerateEmbeddingRequest represents a request to generate an embedding
type GenerateEmbeddingRequest struct {
	Text      string                 `json:"text"`
	Model     string                 `json:"model"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RequestID string                 `json:"request_id,omitempty"` // For idempotency
}

// BatchGenerateEmbeddingRequest represents a batch embedding request
type BatchGenerateEmbeddingRequest struct {
	Texts     []string               `json:"texts"`
	Model     string                 `json:"model"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	RequestID string                 `json:"request_id,omitempty"`
}

// EmbeddingResponse represents the response from generating an embedding
type EmbeddingResponse struct {
	Embedding    []float32              `json:"embedding"`
	Model        string                 `json:"model"`
	Dimensions   int                    `json:"dimensions"`
	TokensUsed   int                    `json:"tokens_used"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ProviderInfo ProviderMetadata       `json:"provider_info"`
}

// BatchEmbeddingResponse represents the response from batch embedding generation
type BatchEmbeddingResponse struct {
	Embeddings   [][]float32            `json:"embeddings"`
	Model        string                 `json:"model"`
	Dimensions   int                    `json:"dimensions"`
	TotalTokens  int                    `json:"total_tokens"`
	Metadata     map[string]interface{} `json:"metadata,omitempty"`
	ProviderInfo ProviderMetadata       `json:"provider_info"`
}

// ProviderMetadata contains provider-specific metadata
type ProviderMetadata struct {
	Provider      string        `json:"provider"`
	Region        string        `json:"region,omitempty"`
	LatencyMs     int64         `json:"latency_ms"`
	RateLimitInfo RateLimitInfo `json:"rate_limit_info,omitempty"`
}

// RateLimitInfo contains rate limit information from the provider
type RateLimitInfo struct {
	RequestsRemaining int       `json:"requests_remaining,omitempty"`
	TokensRemaining   int       `json:"tokens_remaining,omitempty"`
	ResetsAt          time.Time `json:"resets_at,omitempty"`
}

// ModelInfo contains information about an embedding model
type ModelInfo struct {
	Name                       string     `json:"name"`
	DisplayName                string     `json:"display_name"`
	Dimensions                 int        `json:"dimensions"`
	MaxTokens                  int        `json:"max_tokens"`
	CostPer1MTokens            float64    `json:"cost_per_1m_tokens"`
	SupportedTaskTypes         []string   `json:"supported_task_types"`
	SupportsDimensionReduction bool       `json:"supports_dimension_reduction"`
	MinDimensions              int        `json:"min_dimensions,omitempty"`
	IsActive                   bool       `json:"is_active"`
	DeprecatedAt               *time.Time `json:"deprecated_at,omitempty"`
}

// ProviderConfig contains common configuration for providers
type ProviderConfig struct {
	// API credentials
	APIKey          string `json:"api_key,omitempty"`
	AccessKeyID     string `json:"access_key_id,omitempty"`
	SecretAccessKey string `json:"secret_access_key,omitempty"`

	// Endpoints and regions
	Endpoint string `json:"endpoint,omitempty"`
	Region   string `json:"region,omitempty"`

	// AWS IAM Role ARN for AssumeRole authentication (for Bedrock and other AWS services)
	RoleARN string `json:"role_arn,omitempty"`

	// Rate limiting
	MaxRequestsPerMinute int `json:"max_requests_per_minute,omitempty"`
	MaxTokensPerMinute   int `json:"max_tokens_per_minute,omitempty"`

	// Timeouts
	RequestTimeout time.Duration `json:"request_timeout,omitempty"`

	// Retry configuration
	MaxRetries     int           `json:"max_retries,omitempty"`
	RetryDelayBase time.Duration `json:"retry_delay_base,omitempty"`
	RetryDelayMax  time.Duration `json:"retry_delay_max,omitempty"`

	// Custom headers or parameters
	CustomHeaders map[string]string      `json:"custom_headers,omitempty"`
	ExtraParams   map[string]interface{} `json:"extra_params,omitempty"`
}

// ProviderError represents an error from a provider
type ProviderError struct {
	Provider    string         `json:"provider"`
	Code        string         `json:"code"`
	Message     string         `json:"message"`
	StatusCode  int            `json:"status_code,omitempty"`
	RetryAfter  *time.Duration `json:"retry_after,omitempty"`
	IsRetryable bool           `json:"is_retryable"`
}

func (e *ProviderError) Error() string {
	return fmt.Sprintf("%s provider error [%s]: %s", e.Provider, e.Code, e.Message)
}

// TaskType represents the type of task for embedding optimization
type TaskType string

const (
	TaskTypeCodeAnalysis TaskType = "code_analysis"
	TaskTypeGeneralQA    TaskType = "general_qa"
	TaskTypeMultilingual TaskType = "multilingual"
	TaskTypeResearch     TaskType = "research"
	TaskTypeDefault      TaskType = "default"
)
