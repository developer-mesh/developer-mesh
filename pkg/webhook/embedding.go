package webhook

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// EmbeddingProvider defines the interface for embedding generation
type EmbeddingProvider interface {
	GenerateEmbedding(ctx context.Context, text string) ([]float32, error)
	GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error)
	GetDimensions() int
	GetModelName() string
}

// EmbeddingConfig contains configuration for the embedding service
type EmbeddingConfig struct {
	Provider       string                 // "openai", "sentence-transformers", "custom"
	Model          string                 // Model name
	Dimensions     int                    // Embedding dimensions
	MaxTextLength  int                    // Maximum text length
	BatchSize      int                    // Batch processing size
	CacheDuration  time.Duration          // Cache duration for embeddings
	ProviderConfig map[string]interface{} // Provider-specific config
}

// DefaultEmbeddingConfig returns default embedding configuration
func DefaultEmbeddingConfig() *EmbeddingConfig {
	return &EmbeddingConfig{
		Provider:      "sentence-transformers",
		Model:         "all-MiniLM-L6-v2",
		Dimensions:    384,
		MaxTextLength: 512,
		BatchSize:     32,
		CacheDuration: 24 * time.Hour,
		ProviderConfig: map[string]interface{}{
			"endpoint": "http://localhost:8001/embed", // Local embedding service
		},
	}
}

// EmbeddingService manages embeddings for webhook contexts
type EmbeddingService struct {
	config   *EmbeddingConfig
	provider EmbeddingProvider
	cache    EmbeddingCache
	logger   observability.Logger

	// Metrics
	metrics EmbeddingMetrics
}

// EmbeddingMetrics tracks embedding generation statistics
type EmbeddingMetrics struct {
	mu                    sync.RWMutex
	TotalGenerated        int64
	TotalCacheHits        int64
	TotalCacheMisses      int64
	AverageGenerationTime time.Duration
	TotalErrors           int64
}

// EmbeddingCache defines the interface for embedding cache
type EmbeddingCache interface {
	Get(ctx context.Context, key string) ([]float32, error)
	Set(ctx context.Context, key string, embedding []float32, ttl time.Duration) error
	GetBatch(ctx context.Context, keys []string) (map[string][]float32, error)
	SetBatch(ctx context.Context, embeddings map[string][]float32, ttl time.Duration) error
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(config *EmbeddingConfig, cache EmbeddingCache, logger observability.Logger) (*EmbeddingService, error) {
	if config == nil {
		config = DefaultEmbeddingConfig()
	}

	// Create provider based on configuration
	provider, err := createEmbeddingProvider(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create embedding provider: %w", err)
	}

	return &EmbeddingService{
		config:   config,
		provider: provider,
		cache:    cache,
		logger:   logger,
	}, nil
}

// GenerateEmbedding generates an embedding for a single text
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	start := time.Now()

	// Truncate text if needed
	if len(text) > s.config.MaxTextLength {
		text = text[:s.config.MaxTextLength]
	}

	// Generate cache key
	cacheKey := s.generateCacheKey(text)

	// Check cache first
	if s.cache != nil {
		if embedding, err := s.cache.Get(ctx, cacheKey); err == nil && embedding != nil {
			s.metrics.mu.Lock()
			s.metrics.TotalCacheHits++
			s.metrics.mu.Unlock()
			return embedding, nil
		}
	}

	s.metrics.mu.Lock()
	s.metrics.TotalCacheMisses++
	s.metrics.mu.Unlock()

	// Generate embedding
	embedding, err := s.provider.GenerateEmbedding(ctx, text)
	if err != nil {
		s.metrics.mu.Lock()
		s.metrics.TotalErrors++
		s.metrics.mu.Unlock()
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Update metrics
	duration := time.Since(start)
	s.updateMetrics(duration)

	// Cache the result
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, embedding, s.config.CacheDuration); err != nil {
			s.logger.Warn("Failed to cache embedding", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return embedding, nil
}

// GenerateContextEmbedding generates an embedding for webhook context
func (s *EmbeddingService) GenerateContextEmbedding(ctx context.Context, contextData *ContextData) (*ContextEmbedding, error) {
	// Extract text representation from context
	text := s.extractTextFromContext(contextData)

	// Generate embedding
	embedding, err := s.GenerateEmbedding(ctx, text)
	if err != nil {
		return nil, err
	}

	// Create context embedding
	contextEmbedding := &ContextEmbedding{
		ContextID:    contextData.Metadata.ID,
		Embedding:    embedding,
		GeneratedAt:  time.Now(),
		Model:        s.provider.GetModelName(),
		Dimensions:   len(embedding),
		SourceText:   text,
		TextChecksum: generateChecksum(text),
	}

	return contextEmbedding, nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (s *EmbeddingService) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return [][]float32{}, nil
	}

	// Process in batches
	results := make([][]float32, len(texts))
	for i := 0; i < len(texts); i += s.config.BatchSize {
		end := i + s.config.BatchSize
		if end > len(texts) {
			end = len(texts)
		}

		batch := texts[i:end]
		batchEmbeddings, err := s.provider.GenerateBatchEmbeddings(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("failed to generate batch embeddings: %w", err)
		}

		// Copy results
		for j, embedding := range batchEmbeddings {
			results[i+j] = embedding
		}
	}

	return results, nil
}

// SearchSimilar finds similar contexts using vector similarity
func (s *EmbeddingService) SearchSimilar(ctx context.Context, queryEmbedding []float32, topK int) ([]*SimilarityResult, error) {
	// This would integrate with a vector database like Pinecone, Weaviate, or pgvector
	// For now, return placeholder
	return []*SimilarityResult{}, nil
}

// extractTextFromContext extracts searchable text from context data
func (s *EmbeddingService) extractTextFromContext(contextData *ContextData) string {
	var parts []string

	// Add summary if available
	if contextData.Summary != "" {
		parts = append(parts, contextData.Summary)
	}

	// Extract key-value pairs from data
	for key, value := range contextData.Data {
		// Convert value to string representation
		valueStr := fmt.Sprintf("%v", value)
		parts = append(parts, fmt.Sprintf("%s: %s", key, valueStr))
	}

	// Add tags
	if len(contextData.Metadata.Tags) > 0 {
		parts = append(parts, "Tags: "+strings.Join(contextData.Metadata.Tags, ", "))
	}

	// Add source information
	parts = append(parts, fmt.Sprintf("Source: %s %s", contextData.Metadata.SourceType, contextData.Metadata.SourceID))

	return strings.Join(parts, " ")
}

// generateCacheKey generates a cache key for text
func (s *EmbeddingService) generateCacheKey(text string) string {
	return fmt.Sprintf("embedding:%s:%s", s.provider.GetModelName(), generateChecksum(text))
}

// updateMetrics updates embedding generation metrics
func (s *EmbeddingService) updateMetrics(duration time.Duration) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.TotalGenerated++

	// Update average generation time
	if s.metrics.AverageGenerationTime == 0 {
		s.metrics.AverageGenerationTime = duration
	} else {
		// Running average
		s.metrics.AverageGenerationTime = (s.metrics.AverageGenerationTime + duration) / 2
	}
}

// GetMetrics returns embedding service metrics
func (s *EmbeddingService) GetMetrics() map[string]interface{} {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	cacheHitRate := float64(0)
	if total := s.metrics.TotalCacheHits + s.metrics.TotalCacheMisses; total > 0 {
		cacheHitRate = float64(s.metrics.TotalCacheHits) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_generated":         s.metrics.TotalGenerated,
		"total_cache_hits":        s.metrics.TotalCacheHits,
		"total_cache_misses":      s.metrics.TotalCacheMisses,
		"cache_hit_rate":          cacheHitRate,
		"average_generation_time": s.metrics.AverageGenerationTime,
		"total_errors":            s.metrics.TotalErrors,
		"provider":                s.config.Provider,
		"model":                   s.config.Model,
		"dimensions":              s.config.Dimensions,
	}
}

// ContextEmbedding represents an embedding for a context
type ContextEmbedding struct {
	ContextID    string    `json:"context_id"`
	Embedding    []float32 `json:"embedding"`
	GeneratedAt  time.Time `json:"generated_at"`
	Model        string    `json:"model"`
	Dimensions   int       `json:"dimensions"`
	SourceText   string    `json:"source_text,omitempty"`
	TextChecksum string    `json:"text_checksum"`
}

// SimilarityResult represents a similarity search result
type SimilarityResult struct {
	ContextID string       `json:"context_id"`
	Score     float32      `json:"score"`
	Context   *ContextData `json:"context,omitempty"`
}

// createEmbeddingProvider creates the appropriate embedding provider
func createEmbeddingProvider(config *EmbeddingConfig, logger observability.Logger) (EmbeddingProvider, error) {
	switch config.Provider {
	case "sentence-transformers":
		return NewSentenceTransformersProvider(config, logger)
	case "openai":
		return NewOpenAIProvider(config, logger)
	case "mock":
		return NewMockEmbeddingProvider(config.Dimensions), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", config.Provider)
	}
}

// SentenceTransformersProvider implements embeddings using sentence-transformers
type SentenceTransformersProvider struct {
	config   *EmbeddingConfig
	endpoint string
	logger   observability.Logger
	client   *http.Client
}

// NewSentenceTransformersProvider creates a new sentence transformers provider
func NewSentenceTransformersProvider(config *EmbeddingConfig, logger observability.Logger) (*SentenceTransformersProvider, error) {
	endpoint, ok := config.ProviderConfig["endpoint"].(string)
	if !ok || endpoint == "" {
		return nil, fmt.Errorf("endpoint not configured for sentence-transformers provider")
	}

	return &SentenceTransformersProvider{
		config:   config,
		endpoint: endpoint,
		logger:   logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// GenerateEmbedding generates an embedding for text
func (p *SentenceTransformersProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	embeddings, err := p.GenerateBatchEmbeddings(ctx, []string{text})
	if err != nil {
		return nil, err
	}
	if len(embeddings) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return embeddings[0], nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (p *SentenceTransformersProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	// This would make an HTTP request to the sentence-transformers service
	// For now, return mock embeddings
	embeddings := make([][]float32, len(texts))
	for i := range texts {
		embeddings[i] = generateMockEmbedding(p.config.Dimensions, texts[i])
	}
	return embeddings, nil
}

// GetDimensions returns the embedding dimensions
func (p *SentenceTransformersProvider) GetDimensions() int {
	return p.config.Dimensions
}

// GetModelName returns the model name
func (p *SentenceTransformersProvider) GetModelName() string {
	return p.config.Model
}

// OpenAIProvider implements embeddings using OpenAI API
type OpenAIProvider struct {
	config *EmbeddingConfig
	apiKey string
	logger observability.Logger
}

// NewOpenAIProvider creates a new OpenAI provider
func NewOpenAIProvider(config *EmbeddingConfig, logger observability.Logger) (*OpenAIProvider, error) {
	apiKey, ok := config.ProviderConfig["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("api_key not configured for OpenAI provider")
	}

	return &OpenAIProvider{
		config: config,
		apiKey: apiKey,
		logger: logger,
	}, nil
}

// GenerateEmbedding generates an embedding for text
func (p *OpenAIProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	// This would make an API call to OpenAI
	// For now, return mock embedding
	return generateMockEmbedding(p.config.Dimensions, text), nil
}

// GenerateBatchEmbeddings generates embeddings for multiple texts
func (p *OpenAIProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = generateMockEmbedding(p.config.Dimensions, text)
	}
	return embeddings, nil
}

// GetDimensions returns the embedding dimensions
func (p *OpenAIProvider) GetDimensions() int {
	return p.config.Dimensions
}

// GetModelName returns the model name
func (p *OpenAIProvider) GetModelName() string {
	return p.config.Model
}

// MockEmbeddingProvider provides mock embeddings for testing
type MockEmbeddingProvider struct {
	dimensions int
}

// NewMockEmbeddingProvider creates a new mock provider
func NewMockEmbeddingProvider(dimensions int) *MockEmbeddingProvider {
	return &MockEmbeddingProvider{dimensions: dimensions}
}

// GenerateEmbedding generates a mock embedding
func (p *MockEmbeddingProvider) GenerateEmbedding(ctx context.Context, text string) ([]float32, error) {
	return generateMockEmbedding(p.dimensions, text), nil
}

// GenerateBatchEmbeddings generates mock embeddings for multiple texts
func (p *MockEmbeddingProvider) GenerateBatchEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))
	for i, text := range texts {
		embeddings[i] = generateMockEmbedding(p.dimensions, text)
	}
	return embeddings, nil
}

// GetDimensions returns the embedding dimensions
func (p *MockEmbeddingProvider) GetDimensions() int {
	return p.dimensions
}

// GetModelName returns the model name
func (p *MockEmbeddingProvider) GetModelName() string {
	return "mock"
}

// generateMockEmbedding generates a deterministic mock embedding based on text
func generateMockEmbedding(dimensions int, text string) []float32 {
	embedding := make([]float32, dimensions)

	// Use text hash to generate deterministic values
	hash := generateChecksum(text)
	seed := int64(0)
	for _, b := range []byte(hash[:8]) {
		seed = seed*256 + int64(b)
	}

	// Generate normalized random values
	for i := 0; i < dimensions; i++ {
		// Simple linear congruential generator
		seed = (seed*1103515245 + 12345) & 0x7fffffff
		value := float32(seed) / float32(0x7fffffff)
		embedding[i] = (value - 0.5) * 2 // Normalize to [-1, 1]
	}

	// Normalize the vector
	norm := float32(0)
	for _, v := range embedding {
		norm += v * v
	}
	norm = float32(math.Sqrt(float64(norm)))

	if norm > 0 {
		for i := range embedding {
			embedding[i] /= norm
		}
	}

	return embedding
}

// VectorSimilarity calculates cosine similarity between two embeddings
func VectorSimilarity(a, b []float32) (float32, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("embedding dimensions mismatch: %d vs %d", len(a), len(b))
	}

	var dotProduct, normA, normB float32
	for i := range a {
		dotProduct += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}

	if normA == 0 || normB == 0 {
		return 0, nil
	}

	return dotProduct / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB)))), nil
}

// generateChecksum generates a checksum for text
func generateChecksum(text string) string {
	h := sha256.New()
	h.Write([]byte(text))
	return hex.EncodeToString(h.Sum(nil))
}
