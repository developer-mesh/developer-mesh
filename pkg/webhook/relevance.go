package webhook

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RelevanceScorer defines the interface for relevance scoring
type RelevanceScorer interface {
	ScoreRelevance(ctx context.Context, query string, context *ContextData) (float64, error)
	ScoreBatch(ctx context.Context, query string, contexts []*ContextData) ([]float64, error)
	ComputeFeatures(ctx context.Context, context *ContextData) (*ContextFeatures, error)
}

// RelevanceConfig contains configuration for relevance scoring
type RelevanceConfig struct {
	// Scoring weights
	TextSimilarityWeight  float64
	RecencyWeight         float64
	AccessFrequencyWeight float64
	ImportanceWeight      float64
	SemanticWeight        float64

	// Decay parameters
	RecencyDecayHalfLife   time.Duration
	FrequencyDecayHalfLife time.Duration

	// Semantic scoring
	UseEmbeddings      bool
	EmbeddingThreshold float64

	// Feature extraction
	MaxKeywords      int
	MinKeywordLength int
}

// DefaultRelevanceConfig returns default configuration
func DefaultRelevanceConfig() *RelevanceConfig {
	return &RelevanceConfig{
		TextSimilarityWeight:   0.3,
		RecencyWeight:          0.2,
		AccessFrequencyWeight:  0.2,
		ImportanceWeight:       0.2,
		SemanticWeight:         0.1,
		RecencyDecayHalfLife:   24 * time.Hour,
		FrequencyDecayHalfLife: 7 * 24 * time.Hour,
		UseEmbeddings:          true,
		EmbeddingThreshold:     0.7,
		MaxKeywords:            20,
		MinKeywordLength:       3,
	}
}

// RelevanceService provides context relevance scoring
type RelevanceService struct {
	config     *RelevanceConfig
	embedding  *EmbeddingService
	summarizer *SummarizationService
	logger     observability.Logger

	// Feature cache
	featureCache sync.Map // map[contextID]*ContextFeatures

	// Metrics
	metrics RelevanceMetrics
}

// RelevanceMetrics tracks relevance scoring statistics
type RelevanceMetrics struct {
	mu                 sync.RWMutex
	TotalScored        int64
	AverageScoringTime time.Duration
	CacheHits          int64
	CacheMisses        int64
	HighRelevanceCount int64 // Score > 0.8
}

// ContextFeatures represents extracted features for relevance scoring
type ContextFeatures struct {
	ContextID    string
	Keywords     []string
	Entities     []Entity
	Topics       []string
	Embedding    []float32
	TextLength   int
	LastAccessed time.Time
	AccessCount  int
	Importance   float64
	ExtractedAt  time.Time
}

// Entity represents a named entity
type Entity struct {
	Name  string `json:"name"`
	Type  string `json:"type"` // "person", "organization", "location", etc.
	Count int    `json:"count"`
}

// NewRelevanceService creates a new relevance scoring service
func NewRelevanceService(
	config *RelevanceConfig,
	embedding *EmbeddingService,
	summarizer *SummarizationService,
	logger observability.Logger,
) *RelevanceService {
	if config == nil {
		config = DefaultRelevanceConfig()
	}

	return &RelevanceService{
		config:     config,
		embedding:  embedding,
		summarizer: summarizer,
		logger:     logger,
	}
}

// ScoreRelevance scores the relevance of a context to a query
func (s *RelevanceService) ScoreRelevance(ctx context.Context, query string, contextData *ContextData) (float64, error) {
	start := time.Now()
	defer func() {
		s.updateMetrics(time.Since(start))
	}()

	// Extract or retrieve features
	features, err := s.getOrComputeFeatures(ctx, contextData)
	if err != nil {
		return 0, fmt.Errorf("failed to compute features: %w", err)
	}

	// Compute individual scores
	scores := make(map[string]float64)

	// Text similarity score
	scores["text"] = s.computeTextSimilarity(query, contextData, features)

	// Recency score
	scores["recency"] = s.computeRecencyScore(features.LastAccessed)

	// Access frequency score
	scores["frequency"] = s.computeFrequencyScore(features.AccessCount)

	// Importance score
	scores["importance"] = features.Importance

	// Semantic similarity score (if embeddings are available)
	if s.config.UseEmbeddings && s.embedding != nil {
		semanticScore, err := s.computeSemanticSimilarity(ctx, query, features)
		if err == nil {
			scores["semantic"] = semanticScore
		}
	}

	// Weighted combination
	finalScore := s.combineScores(scores)

	// Track high relevance contexts
	if finalScore > 0.8 {
		s.metrics.mu.Lock()
		s.metrics.HighRelevanceCount++
		s.metrics.mu.Unlock()
	}

	return finalScore, nil
}

// ScoreBatch scores relevance for multiple contexts
func (s *RelevanceService) ScoreBatch(ctx context.Context, query string, contexts []*ContextData) ([]float64, error) {
	scores := make([]float64, len(contexts))

	// Generate query embedding once if using semantic scoring
	if s.config.UseEmbeddings && s.embedding != nil {
		_, err := s.embedding.GenerateEmbedding(ctx, query)
		if err != nil {
			s.logger.Warn("Failed to generate query embedding", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	// Score each context
	for i, context := range contexts {
		score, err := s.ScoreRelevance(ctx, query, context)
		if err != nil {
			s.logger.Warn("Failed to score context", map[string]interface{}{
				"context_id": context.Metadata.ID,
				"error":      err.Error(),
			})
			scores[i] = 0
		} else {
			scores[i] = score
		}
	}

	return scores, nil
}

// RankContexts ranks contexts by relevance to a query
func (s *RelevanceService) RankContexts(ctx context.Context, query string, contexts []*ContextData, topK int) ([]*RankedContext, error) {
	if len(contexts) == 0 {
		return []*RankedContext{}, nil
	}

	// Score all contexts
	scores, err := s.ScoreBatch(ctx, query, contexts)
	if err != nil {
		return nil, err
	}

	// Create ranked contexts
	ranked := make([]*RankedContext, len(contexts))
	for i, context := range contexts {
		ranked[i] = &RankedContext{
			Context:        context,
			RelevanceScore: scores[i],
			Rank:           0, // Will be set after sorting
		}
	}

	// Sort by relevance score (descending)
	sort.Slice(ranked, func(i, j int) bool {
		return ranked[i].RelevanceScore > ranked[j].RelevanceScore
	})

	// Set ranks
	for i, rc := range ranked {
		rc.Rank = i + 1
	}

	// Return top K
	if topK > 0 && topK < len(ranked) {
		ranked = ranked[:topK]
	}

	return ranked, nil
}

// ComputeFeatures extracts features from a context
func (s *RelevanceService) ComputeFeatures(ctx context.Context, contextData *ContextData) (*ContextFeatures, error) {
	features := &ContextFeatures{
		ContextID:    contextData.Metadata.ID,
		LastAccessed: contextData.Metadata.LastAccessed,
		AccessCount:  contextData.Metadata.AccessCount,
		Importance:   contextData.Metadata.Importance,
		ExtractedAt:  time.Now(),
	}

	// Extract text for analysis
	text := s.extractTextForAnalysis(contextData)
	features.TextLength = len(text)

	// Extract keywords
	features.Keywords = s.extractKeywords(text)

	// Extract entities
	features.Entities = s.extractEntities(text)

	// Extract topics
	features.Topics = s.extractTopics(text, features.Keywords)

	// Generate embedding if enabled
	if s.config.UseEmbeddings && s.embedding != nil {
		embedding, err := s.embedding.GenerateContextEmbedding(ctx, contextData)
		if err == nil {
			features.Embedding = embedding.Embedding
		}
	}

	return features, nil
}

// getOrComputeFeatures retrieves features from cache or computes them
func (s *RelevanceService) getOrComputeFeatures(ctx context.Context, contextData *ContextData) (*ContextFeatures, error) {
	// Check cache
	if cached, ok := s.featureCache.Load(contextData.Metadata.ID); ok {
		features := cached.(*ContextFeatures)
		// Check if features are still fresh (within 1 hour)
		if time.Since(features.ExtractedAt) < time.Hour {
			s.metrics.mu.Lock()
			s.metrics.CacheHits++
			s.metrics.mu.Unlock()
			return features, nil
		}
	}

	s.metrics.mu.Lock()
	s.metrics.CacheMisses++
	s.metrics.mu.Unlock()

	// Compute features
	features, err := s.ComputeFeatures(ctx, contextData)
	if err != nil {
		return nil, err
	}

	// Cache features
	s.featureCache.Store(contextData.Metadata.ID, features)

	return features, nil
}

// computeTextSimilarity computes text-based similarity
func (s *RelevanceService) computeTextSimilarity(query string, context *ContextData, features *ContextFeatures) float64 {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	score := float64(0)

	// Check keyword matches
	keywordMatches := 0
	for _, keyword := range features.Keywords {
		keywordLower := strings.ToLower(keyword)
		for _, queryWord := range queryWords {
			if strings.Contains(keywordLower, queryWord) || strings.Contains(queryWord, keywordLower) {
				keywordMatches++
				break
			}
		}
	}

	if len(features.Keywords) > 0 {
		score += float64(keywordMatches) / float64(len(features.Keywords))
	}

	// Check entity matches
	entityMatches := 0
	for _, entity := range features.Entities {
		entityLower := strings.ToLower(entity.Name)
		if strings.Contains(queryLower, entityLower) || strings.Contains(entityLower, queryLower) {
			entityMatches++
		}
	}

	if len(features.Entities) > 0 {
		score += float64(entityMatches) / float64(len(features.Entities))
	}

	// Normalize score
	return math.Min(score/2, 1.0)
}

// computeRecencyScore computes score based on recency
func (s *RelevanceService) computeRecencyScore(lastAccessed time.Time) float64 {
	age := time.Since(lastAccessed)
	halfLife := s.config.RecencyDecayHalfLife

	// Exponential decay
	return math.Exp(-0.693 * age.Hours() / halfLife.Hours())
}

// computeFrequencyScore computes score based on access frequency
func (s *RelevanceService) computeFrequencyScore(accessCount int) float64 {
	if accessCount == 0 {
		return 0
	}

	// Logarithmic scaling with saturation
	return math.Min(math.Log10(float64(accessCount+1))/2, 1.0)
}

// computeSemanticSimilarity computes semantic similarity using embeddings
func (s *RelevanceService) computeSemanticSimilarity(ctx context.Context, query string, features *ContextFeatures) (float64, error) {
	if features.Embedding == nil {
		return 0, nil
	}

	// Generate query embedding
	queryEmbedding, err := s.embedding.GenerateEmbedding(ctx, query)
	if err != nil {
		return 0, err
	}

	// Compute cosine similarity
	similarity, err := VectorSimilarity(queryEmbedding, features.Embedding)
	if err != nil {
		return 0, err
	}

	// Apply threshold
	if similarity < float32(s.config.EmbeddingThreshold) {
		similarity = similarity * 0.5 // Reduce score for below-threshold similarities
	}

	return float64(similarity), nil
}

// combineScores combines individual scores with weights
func (s *RelevanceService) combineScores(scores map[string]float64) float64 {
	weightedSum := float64(0)
	totalWeight := float64(0)

	weights := map[string]float64{
		"text":       s.config.TextSimilarityWeight,
		"recency":    s.config.RecencyWeight,
		"frequency":  s.config.AccessFrequencyWeight,
		"importance": s.config.ImportanceWeight,
		"semantic":   s.config.SemanticWeight,
	}

	for scoreType, score := range scores {
		if weight, ok := weights[scoreType]; ok {
			weightedSum += score * weight
			totalWeight += weight
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// extractTextForAnalysis extracts analyzable text from context
func (s *RelevanceService) extractTextForAnalysis(contextData *ContextData) string {
	var parts []string

	// Summary is most important
	if contextData.Summary != "" {
		parts = append(parts, contextData.Summary)
	}

	// Extract text from data fields
	for key, value := range contextData.Data {
		if str, ok := value.(string); ok && str != "" {
			parts = append(parts, fmt.Sprintf("%s: %s", key, str))
		}
	}

	// Include tags
	if len(contextData.Metadata.Tags) > 0 {
		parts = append(parts, strings.Join(contextData.Metadata.Tags, " "))
	}

	return strings.Join(parts, " ")
}

// extractKeywords extracts keywords from text
func (s *RelevanceService) extractKeywords(text string) []string {
	// Simple keyword extraction (production would use TF-IDF or TextRank)
	words := strings.Fields(strings.ToLower(text))
	wordFreq := make(map[string]int)

	// Count word frequencies
	for _, word := range words {
		// Clean word
		word = strings.Trim(word, ".,!?;:\"'()")

		// Filter by length
		if len(word) >= s.config.MinKeywordLength {
			wordFreq[word]++
		}
	}

	// Sort by frequency
	type wordCount struct {
		word  string
		count int
	}

	counts := make([]wordCount, 0, len(wordFreq))
	for word, count := range wordFreq {
		counts = append(counts, wordCount{word, count})
	}

	sort.Slice(counts, func(i, j int) bool {
		return counts[i].count > counts[j].count
	})

	// Extract top keywords
	keywords := make([]string, 0, s.config.MaxKeywords)
	for i := 0; i < len(counts) && i < s.config.MaxKeywords; i++ {
		keywords = append(keywords, counts[i].word)
	}

	return keywords
}

// extractEntities extracts named entities from text
func (s *RelevanceService) extractEntities(text string) []Entity {
	// Simple entity extraction based on capitalization patterns
	// Production would use NER models

	entities := make(map[string]*Entity)
	words := strings.Fields(text)

	for i, word := range words {
		// Check for capitalized words (potential entities)
		if len(word) > 2 && unicode.IsUpper(rune(word[0])) {
			// Look for multi-word entities
			entity := word
			entityType := s.classifyEntity(word)

			// Check next words
			for j := i + 1; j < len(words) && j < i+3; j++ {
				if unicode.IsUpper(rune(words[j][0])) {
					entity += " " + words[j]
				} else {
					break
				}
			}

			// Add or update entity
			if existing, ok := entities[entity]; ok {
				existing.Count++
			} else {
				entities[entity] = &Entity{
					Name:  entity,
					Type:  entityType,
					Count: 1,
				}
			}
		}
	}

	// Convert to slice
	result := make([]Entity, 0, len(entities))
	for _, entity := range entities {
		result = append(result, *entity)
	}

	// Sort by count
	sort.Slice(result, func(i, j int) bool {
		return result[i].Count > result[j].Count
	})

	return result
}

// classifyEntity attempts to classify an entity type
func (s *RelevanceService) classifyEntity(text string) string {
	lower := strings.ToLower(text)

	// Simple heuristics (production would use NER)
	if strings.Contains(lower, "inc") || strings.Contains(lower, "corp") || strings.Contains(lower, "ltd") {
		return "organization"
	}

	// Common location suffixes
	locationSuffixes := []string{"city", "town", "country", "state", "province"}
	for _, suffix := range locationSuffixes {
		if strings.HasSuffix(lower, suffix) {
			return "location"
		}
	}

	// Default to generic entity
	return "entity"
}

// extractTopics extracts topics from text and keywords
func (s *RelevanceService) extractTopics(text string, keywords []string) []string {
	// Simple topic extraction based on keyword clustering
	// Production would use LDA or similar topic modeling

	topics := make(map[string]bool)

	// Technology topics
	techKeywords := []string{"api", "database", "server", "code", "deploy", "bug", "feature", "test"}
	for _, kw := range keywords {
		for _, tech := range techKeywords {
			if strings.Contains(kw, tech) {
				topics["technology"] = true
				break
			}
		}
	}

	// Business topics
	businessKeywords := []string{"customer", "revenue", "sales", "market", "product", "user"}
	for _, kw := range keywords {
		for _, biz := range businessKeywords {
			if strings.Contains(kw, biz) {
				topics["business"] = true
				break
			}
		}
	}

	// Convert to slice
	result := make([]string, 0, len(topics))
	for topic := range topics {
		result = append(result, topic)
	}

	return result
}

// updateMetrics updates relevance scoring metrics
func (s *RelevanceService) updateMetrics(duration time.Duration) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.TotalScored++

	// Update average scoring time
	if s.metrics.AverageScoringTime == 0 {
		s.metrics.AverageScoringTime = duration
	} else {
		s.metrics.AverageScoringTime = (s.metrics.AverageScoringTime + duration) / 2
	}
}

// GetMetrics returns relevance service metrics
func (s *RelevanceService) GetMetrics() map[string]interface{} {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	cacheHitRate := float64(0)
	if total := s.metrics.CacheHits + s.metrics.CacheMisses; total > 0 {
		cacheHitRate = float64(s.metrics.CacheHits) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_scored":         s.metrics.TotalScored,
		"average_scoring_time": s.metrics.AverageScoringTime,
		"cache_hits":           s.metrics.CacheHits,
		"cache_misses":         s.metrics.CacheMisses,
		"cache_hit_rate":       cacheHitRate,
		"high_relevance_count": s.metrics.HighRelevanceCount,
	}
}

// RankedContext represents a context with its relevance ranking
type RankedContext struct {
	Context        *ContextData     `json:"context"`
	RelevanceScore float64          `json:"relevance_score"`
	Rank           int              `json:"rank"`
	Features       *ContextFeatures `json:"features,omitempty"`
}

// RelevanceExplanation explains why a context was deemed relevant
type RelevanceExplanation struct {
	ContextID       string             `json:"context_id"`
	TotalScore      float64            `json:"total_score"`
	ScoreBreakdown  map[string]float64 `json:"score_breakdown"`
	MatchedKeywords []string           `json:"matched_keywords"`
	MatchedEntities []string           `json:"matched_entities"`
	Explanation     string             `json:"explanation"`
}
