// Story 2.2: Context-Aware Embedding Client
// LOCATION: pkg/embedding/context_embedding_client.go

package embedding

import (
	"context"
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ContextEmbeddingClient wraps embedding providers for context-specific operations
type ContextEmbeddingClient struct {
	providers map[string]providers.Provider
	logger    observability.Logger
}

// NewContextEmbeddingClient creates a new context embedding client
func NewContextEmbeddingClient(logger observability.Logger) *ContextEmbeddingClient {
	return &ContextEmbeddingClient{
		providers: make(map[string]providers.Provider),
		logger:    logger,
	}
}

// RegisterProvider adds an embedding provider
func (c *ContextEmbeddingClient) RegisterProvider(name string, provider providers.Provider) {
	c.providers[name] = provider
}

// SelectModel chooses appropriate embedding model based on content
func (c *ContextEmbeddingClient) SelectModel(content string) string {
	// Check if content contains code blocks
	if strings.Contains(content, "```") || strings.Contains(content, "func ") || strings.Contains(content, "class ") {
		// Prefer code-specific models
		codeModels := []string{
			"voyage-code-3",
			"voyage-code-2",
		}
		for _, model := range codeModels {
			if provider := c.findProviderForModel(model); provider != nil {
				return model
			}
		}
	}

	// Default models in priority order (2025 standards)
	preferredModels := []string{
		"text-embedding-3-small",       // OpenAI (1536 dimensions, $0.02/1M tokens)
		"voyage-3",                     // Voyage AI (1024 dimensions, Anthropic partner)
		"amazon.titan-embed-text-v2:0", // AWS Bedrock (1024 dimensions)
		"cohere.embed-english-v3",      // Cohere on Bedrock (1024 dimensions)
	}

	for _, model := range preferredModels {
		if provider := c.findProviderForModel(model); provider != nil {
			return model
		}
	}

	// Return first available model from any provider
	for name, provider := range c.providers {
		models := provider.GetSupportedModels()
		if len(models) > 0 {
			c.logger.Debug("Using fallback model", map[string]interface{}{
				"provider": name,
				"model":    models[0].Name,
			})
			return models[0].Name
		}
	}

	return ""
}

// findProviderForModel finds a provider that supports the given model
func (c *ContextEmbeddingClient) findProviderForModel(modelName string) providers.Provider {
	for _, provider := range c.providers {
		if _, err := provider.GetModel(modelName); err == nil {
			return provider
		}
	}
	return nil
}

// EmbedContent generates embedding for content with appropriate model
func (c *ContextEmbeddingClient) EmbedContent(
	ctx context.Context,
	content string,
	modelOverride string,
) ([]float32, string, error) {
	// Select model
	model := modelOverride
	if model == "" {
		model = c.SelectModel(content)
	}

	if model == "" {
		return nil, "", fmt.Errorf("no embedding model available")
	}

	// Find provider for this model
	provider := c.findProviderForModel(model)
	if provider == nil {
		return nil, "", fmt.Errorf("no provider found for model: %s", model)
	}

	// Create embedding request
	req := providers.GenerateEmbeddingRequest{
		Text:  content,
		Model: model,
		Metadata: map[string]interface{}{
			"source": "context_embedding_client",
		},
	}

	// Generate embedding using provider interface
	response, err := provider.GenerateEmbedding(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate embedding with model %s: %w", model, err)
	}

	// Log embedding generation
	if c.logger != nil {
		c.logger.Debug("Embedding generated", map[string]interface{}{
			"model":      response.Model,
			"dimensions": response.Dimensions,
			"tokens":     response.TokensUsed,
			"provider":   response.ProviderInfo.Provider,
			"latency_ms": response.ProviderInfo.LatencyMs,
		})
	}

	return response.Embedding, response.Model, nil
}

// BatchEmbedContent generates embeddings for multiple content items
func (c *ContextEmbeddingClient) BatchEmbedContent(
	ctx context.Context,
	contents []string,
	modelOverride string,
) ([][]float32, string, error) {
	if len(contents) == 0 {
		return nil, "", fmt.Errorf("no content provided for batch embedding")
	}

	// Select model based on first content item
	model := modelOverride
	if model == "" {
		model = c.SelectModel(contents[0])
	}

	if model == "" {
		return nil, "", fmt.Errorf("no embedding model available")
	}

	// Find provider for this model
	provider := c.findProviderForModel(model)
	if provider == nil {
		return nil, "", fmt.Errorf("no provider found for model: %s", model)
	}

	// Create batch embedding request
	req := providers.BatchGenerateEmbeddingRequest{
		Texts: contents,
		Model: model,
		Metadata: map[string]interface{}{
			"source":     "context_embedding_client",
			"batch_size": len(contents),
		},
	}

	// Generate embeddings using provider interface
	response, err := provider.BatchGenerateEmbeddings(ctx, req)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate batch embeddings: %w", err)
	}

	// Log batch embedding generation
	if c.logger != nil {
		c.logger.Debug("Batch embeddings generated", map[string]interface{}{
			"model":        response.Model,
			"dimensions":   response.Dimensions,
			"total_tokens": response.TotalTokens,
			"batch_size":   len(contents),
			"provider":     response.ProviderInfo.Provider,
			"latency_ms":   response.ProviderInfo.LatencyMs,
		})
	}

	return response.Embeddings, response.Model, nil
}

// ChunkContent splits content into chunks for embedding
func (c *ContextEmbeddingClient) ChunkContent(content string, maxChunkSize int) []string {
	if maxChunkSize <= 0 {
		maxChunkSize = 1000 // Default chunk size
	}

	if len(content) <= maxChunkSize {
		return []string{content}
	}

	var chunks []string
	words := strings.Fields(content)
	currentChunk := ""

	for _, word := range words {
		// Check if adding this word would exceed the limit
		if len(currentChunk)+len(word)+1 > maxChunkSize {
			if currentChunk != "" {
				chunks = append(chunks, strings.TrimSpace(currentChunk))
				currentChunk = word
			} else {
				// Word itself is longer than maxChunkSize, split it
				chunks = append(chunks, word[:maxChunkSize])
				currentChunk = word[maxChunkSize:]
			}
		} else {
			if currentChunk != "" {
				currentChunk += " "
			}
			currentChunk += word
		}
	}

	if currentChunk != "" {
		chunks = append(chunks, strings.TrimSpace(currentChunk))
	}

	return chunks
}

// GetProviderInfo returns information about registered providers
func (c *ContextEmbeddingClient) GetProviderInfo() map[string][]providers.ModelInfo {
	info := make(map[string][]providers.ModelInfo)
	for name, provider := range c.providers {
		info[name] = provider.GetSupportedModels()
	}
	return info
}

// HealthCheck verifies all registered providers are accessible
func (c *ContextEmbeddingClient) HealthCheck(ctx context.Context) map[string]error {
	results := make(map[string]error)
	for name, provider := range c.providers {
		if err := provider.HealthCheck(ctx); err != nil {
			results[name] = err
			if c.logger != nil {
				c.logger.Warn("Provider health check failed", map[string]interface{}{
					"provider": name,
					"error":    err.Error(),
				})
			}
		}
	}
	return results
}
