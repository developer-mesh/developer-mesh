package webhook

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// SummarizationProvider defines the interface for AI summarization
type SummarizationProvider interface {
	Summarize(ctx context.Context, text string, options SummarizationOptions) (string, error)
	SummarizeConversation(ctx context.Context, messages []Message, options SummarizationOptions) (string, error)
	ExtractKeyPoints(ctx context.Context, text string) ([]string, error)
	GetModelName() string
}

// SummarizationOptions contains options for summarization
type SummarizationOptions struct {
	MaxLength        int      // Maximum summary length in tokens
	MinLength        int      // Minimum summary length in tokens
	Style            string   // "bullets", "paragraph", "technical"
	PreserveEntities bool     // Preserve named entities
	Keywords         []string // Important keywords to preserve
	Language         string   // Output language
}

// Message type is imported from types.go

// SummarizationConfig contains configuration for summarization service
type SummarizationConfig struct {
	Provider       string                 // "openai", "anthropic", "local"
	Model          string                 // Model name
	MaxInputLength int                    // Maximum input length
	DefaultOptions SummarizationOptions   // Default options
	ProviderConfig map[string]interface{} // Provider-specific config
	CacheDuration  time.Duration          // Cache duration for summaries
}

// DefaultSummarizationConfig returns default configuration
func DefaultSummarizationConfig() *SummarizationConfig {
	return &SummarizationConfig{
		Provider:       "local",
		Model:          "extractive",
		MaxInputLength: 4096,
		DefaultOptions: SummarizationOptions{
			MaxLength:        150,
			MinLength:        50,
			Style:            "paragraph",
			PreserveEntities: true,
			Language:         "en",
		},
		CacheDuration: 24 * time.Hour,
	}
}

// SummarizationService provides AI-powered summarization
type SummarizationService struct {
	config   *SummarizationConfig
	provider SummarizationProvider
	cache    SummarizationCache
	logger   observability.Logger

	// Metrics
	metrics SummarizationMetrics
}

// SummarizationMetrics tracks summarization statistics
type SummarizationMetrics struct {
	mu                   sync.RWMutex
	TotalSummarized      int64
	TotalTokensProcessed int64
	TotalCacheHits       int64
	TotalCacheMisses     int64
	AverageSummaryTime   time.Duration
	TotalErrors          int64
}

// SummarizationCache defines the interface for summary cache
type SummarizationCache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, summary string, ttl time.Duration) error
}

// NewSummarizationService creates a new summarization service
func NewSummarizationService(config *SummarizationConfig, cache SummarizationCache, logger observability.Logger) (*SummarizationService, error) {
	if config == nil {
		config = DefaultSummarizationConfig()
	}

	// Create provider based on configuration
	provider, err := createSummarizationProvider(config, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create summarization provider: %w", err)
	}

	return &SummarizationService{
		config:   config,
		provider: provider,
		cache:    cache,
		logger:   logger,
	}, nil
}

// SummarizeContext summarizes webhook context data
func (s *SummarizationService) SummarizeContext(ctx context.Context, contextData *ContextData) (*ContextSummary, error) {
	start := time.Now()

	// Extract text from context
	text := s.extractTextForSummarization(contextData)

	// Check if text is too short to summarize
	if len(text) < 100 {
		return &ContextSummary{
			ContextID: contextData.Metadata.ID,
			Summary:   text,
			Generated: false,
		}, nil
	}

	// Generate cache key
	cacheKey := s.generateCacheKey(text, s.config.DefaultOptions)

	// Check cache
	if s.cache != nil {
		if cached, err := s.cache.Get(ctx, cacheKey); err == nil && cached != "" {
			s.updateCacheMetrics(true)
			return &ContextSummary{
				ContextID:   contextData.Metadata.ID,
				Summary:     cached,
				Generated:   true,
				FromCache:   true,
				GeneratedAt: time.Now(),
				Model:       s.provider.GetModelName(),
			}, nil
		}
	}

	s.updateCacheMetrics(false)

	// Truncate if needed
	if len(text) > s.config.MaxInputLength {
		text = s.truncateIntelligently(text, s.config.MaxInputLength)
	}

	// Generate summary
	summary, err := s.provider.Summarize(ctx, text, s.config.DefaultOptions)
	if err != nil {
		s.metrics.mu.Lock()
		s.metrics.TotalErrors++
		s.metrics.mu.Unlock()
		return nil, fmt.Errorf("failed to generate summary: %w", err)
	}

	// Extract key points
	keyPoints, _ := s.provider.ExtractKeyPoints(ctx, text)

	// Update metrics
	s.updateMetrics(time.Since(start), len(text))

	// Cache the result
	if s.cache != nil {
		if err := s.cache.Set(ctx, cacheKey, summary, s.config.CacheDuration); err != nil {
			s.logger.Warn("Failed to cache summary", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	return &ContextSummary{
		ContextID:    contextData.Metadata.ID,
		Summary:      summary,
		KeyPoints:    keyPoints,
		Generated:    true,
		GeneratedAt:  time.Now(),
		Model:        s.provider.GetModelName(),
		InputLength:  len(text),
		OutputLength: len(summary),
	}, nil
}

// SummarizeEventStream summarizes a stream of webhook events
func (s *SummarizationService) SummarizeEventStream(ctx context.Context, events []*WebhookEvent) (*StreamSummary, error) {
	if len(events) == 0 {
		return &StreamSummary{}, nil
	}

	// Group events by type
	eventGroups := s.groupEventsByType(events)

	// Generate summaries for each group
	groupSummaries := make(map[string]string)
	for eventType, groupEvents := range eventGroups {
		text := s.extractEventGroupText(groupEvents)
		summary, err := s.provider.Summarize(ctx, text, s.config.DefaultOptions)
		if err != nil {
			s.logger.Warn("Failed to summarize event group", map[string]interface{}{
				"event_type": eventType,
				"error":      err.Error(),
			})
			continue
		}
		groupSummaries[eventType] = summary
	}

	// Generate overall summary
	overallText := s.combineGroupSummaries(groupSummaries)
	overallSummary, _ := s.provider.Summarize(ctx, overallText, SummarizationOptions{
		MaxLength: 300,
		MinLength: 100,
		Style:     "paragraph",
	})

	return &StreamSummary{
		TotalEvents:    len(events),
		TimeRange:      s.calculateTimeRange(events),
		EventTypes:     s.extractEventTypes(events),
		GroupSummaries: groupSummaries,
		OverallSummary: overallSummary,
		GeneratedAt:    time.Now(),
	}, nil
}

// extractTextForSummarization extracts text from context for summarization
func (s *SummarizationService) extractTextForSummarization(contextData *ContextData) string {
	var parts []string

	// Priority 1: Existing summary
	if contextData.Summary != "" {
		parts = append(parts, contextData.Summary)
	}

	// Priority 2: Important fields from data
	importantFields := []string{"description", "title", "message", "content", "body"}
	for _, field := range importantFields {
		if value, exists := contextData.Data[field]; exists {
			if str, ok := value.(string); ok && str != "" {
				parts = append(parts, str)
			}
		}
	}

	// Priority 3: Other string fields
	for key, value := range contextData.Data {
		if str, ok := value.(string); ok && str != "" {
			// Skip if already included
			isImportant := false
			for _, field := range importantFields {
				if key == field {
					isImportant = true
					break
				}
			}
			if !isImportant {
				parts = append(parts, fmt.Sprintf("%s: %s", key, str))
			}
		}
	}

	// Add metadata context
	parts = append(parts, fmt.Sprintf("Source: %s event from %s",
		contextData.Metadata.SourceType,
		contextData.Metadata.SourceID))

	return strings.Join(parts, "\n\n")
}

// truncateIntelligently truncates text while preserving sentence boundaries
func (s *SummarizationService) truncateIntelligently(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// Find sentence boundary near the limit
	truncated := text[:maxLength]
	lastPeriod := strings.LastIndex(truncated, ". ")
	lastNewline := strings.LastIndex(truncated, "\n")

	cutPoint := maxLength
	if lastPeriod > maxLength*3/4 {
		cutPoint = lastPeriod + 1
	} else if lastNewline > maxLength*3/4 {
		cutPoint = lastNewline
	}

	return text[:cutPoint] + "..."
}

// generateCacheKey generates a cache key for summarization
func (s *SummarizationService) generateCacheKey(text string, options SummarizationOptions) string {
	optionsJSON, _ := json.Marshal(options)
	combined := text + string(optionsJSON)
	return fmt.Sprintf("summary:%s:%s", s.provider.GetModelName(), generateChecksum(combined))
}

// updateMetrics updates summarization metrics
func (s *SummarizationService) updateMetrics(duration time.Duration, tokensProcessed int) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	s.metrics.TotalSummarized++
	s.metrics.TotalTokensProcessed += int64(tokensProcessed)

	// Update average time
	if s.metrics.AverageSummaryTime == 0 {
		s.metrics.AverageSummaryTime = duration
	} else {
		s.metrics.AverageSummaryTime = (s.metrics.AverageSummaryTime + duration) / 2
	}
}

// updateCacheMetrics updates cache hit/miss metrics
func (s *SummarizationService) updateCacheMetrics(hit bool) {
	s.metrics.mu.Lock()
	defer s.metrics.mu.Unlock()

	if hit {
		s.metrics.TotalCacheHits++
	} else {
		s.metrics.TotalCacheMisses++
	}
}

// groupEventsByType groups events by their type
func (s *SummarizationService) groupEventsByType(events []*WebhookEvent) map[string][]*WebhookEvent {
	groups := make(map[string][]*WebhookEvent)
	for _, event := range events {
		groups[event.EventType] = append(groups[event.EventType], event)
	}
	return groups
}

// extractEventGroupText extracts text from a group of events
func (s *SummarizationService) extractEventGroupText(events []*WebhookEvent) string {
	var parts []string
	for _, event := range events {
		// Extract meaningful text from event payload
		if description, ok := event.Payload["description"].(string); ok {
			parts = append(parts, description)
		} else if message, ok := event.Payload["message"].(string); ok {
			parts = append(parts, message)
		}
	}
	return strings.Join(parts, "\n")
}

// combineGroupSummaries combines group summaries into overall text
func (s *SummarizationService) combineGroupSummaries(summaries map[string]string) string {
	var parts []string
	for eventType, summary := range summaries {
		parts = append(parts, fmt.Sprintf("%s events: %s", eventType, summary))
	}
	return strings.Join(parts, "\n\n")
}

// calculateTimeRange calculates the time range of events
func (s *SummarizationService) calculateTimeRange(events []*WebhookEvent) TimeRange {
	if len(events) == 0 {
		return TimeRange{}
	}

	start := events[0].Timestamp
	end := events[0].Timestamp

	for _, event := range events[1:] {
		eventTime := event.Timestamp
		if eventTime.Before(start) {
			start = eventTime
		}
		if eventTime.After(end) {
			end = eventTime
		}
	}

	return TimeRange{
		Start:    start,
		End:      end,
		Duration: end.Sub(start),
	}
}

// extractEventTypes extracts unique event types
func (s *SummarizationService) extractEventTypes(events []*WebhookEvent) []string {
	typeMap := make(map[string]bool)
	for _, event := range events {
		typeMap[event.EventType] = true
	}

	types := make([]string, 0, len(typeMap))
	for eventType := range typeMap {
		types = append(types, eventType)
	}
	return types
}

// GetMetrics returns summarization service metrics
func (s *SummarizationService) GetMetrics() map[string]interface{} {
	s.metrics.mu.RLock()
	defer s.metrics.mu.RUnlock()

	cacheHitRate := float64(0)
	if total := s.metrics.TotalCacheHits + s.metrics.TotalCacheMisses; total > 0 {
		cacheHitRate = float64(s.metrics.TotalCacheHits) / float64(total) * 100
	}

	return map[string]interface{}{
		"total_summarized":       s.metrics.TotalSummarized,
		"total_tokens_processed": s.metrics.TotalTokensProcessed,
		"total_cache_hits":       s.metrics.TotalCacheHits,
		"total_cache_misses":     s.metrics.TotalCacheMisses,
		"cache_hit_rate":         cacheHitRate,
		"average_summary_time":   s.metrics.AverageSummaryTime,
		"total_errors":           s.metrics.TotalErrors,
		"provider":               s.config.Provider,
		"model":                  s.config.Model,
	}
}

// ContextSummary represents a summarized context
type ContextSummary struct {
	ContextID    string    `json:"context_id"`
	Summary      string    `json:"summary"`
	KeyPoints    []string  `json:"key_points,omitempty"`
	Generated    bool      `json:"generated"`
	FromCache    bool      `json:"from_cache"`
	GeneratedAt  time.Time `json:"generated_at"`
	Model        string    `json:"model"`
	InputLength  int       `json:"input_length"`
	OutputLength int       `json:"output_length"`
}

// StreamSummary represents a summary of an event stream
type StreamSummary struct {
	TotalEvents    int               `json:"total_events"`
	TimeRange      TimeRange         `json:"time_range"`
	EventTypes     []string          `json:"event_types"`
	GroupSummaries map[string]string `json:"group_summaries"`
	OverallSummary string            `json:"overall_summary"`
	GeneratedAt    time.Time         `json:"generated_at"`
}

// TimeRange represents a time range
type TimeRange struct {
	Start    time.Time     `json:"start"`
	End      time.Time     `json:"end"`
	Duration time.Duration `json:"duration"`
}

// createSummarizationProvider creates the appropriate summarization provider
func createSummarizationProvider(config *SummarizationConfig, logger observability.Logger) (SummarizationProvider, error) {
	switch config.Provider {
	case "local":
		return NewLocalSummarizationProvider(config, logger), nil
	case "openai":
		return NewOpenAISummarizationProvider(config, logger)
	case "anthropic":
		return NewAnthropicSummarizationProvider(config, logger)
	default:
		return nil, fmt.Errorf("unsupported summarization provider: %s", config.Provider)
	}
}

// LocalSummarizationProvider provides extractive summarization
type LocalSummarizationProvider struct {
	config *SummarizationConfig
	logger observability.Logger
}

// NewLocalSummarizationProvider creates a new local provider
func NewLocalSummarizationProvider(config *SummarizationConfig, logger observability.Logger) *LocalSummarizationProvider {
	return &LocalSummarizationProvider{
		config: config,
		logger: logger,
	}
}

// Summarize performs extractive summarization
func (p *LocalSummarizationProvider) Summarize(ctx context.Context, text string, options SummarizationOptions) (string, error) {
	// Simple extractive summarization
	sentences := p.splitIntoSentences(text)
	if len(sentences) == 0 {
		return "", nil
	}

	// Score sentences based on word frequency
	scores := p.scoreSentences(sentences)

	// Select top sentences
	numSentences := 3
	if options.MaxLength > 200 {
		numSentences = 5
	}

	topSentences := p.selectTopSentences(sentences, scores, numSentences)

	// Format based on style
	if options.Style == "bullets" {
		var bullets []string
		for _, sent := range topSentences {
			bullets = append(bullets, "• "+sent)
		}
		return strings.Join(bullets, "\n"), nil
	}

	return strings.Join(topSentences, " "), nil
}

// SummarizeConversation summarizes a conversation
func (p *LocalSummarizationProvider) SummarizeConversation(ctx context.Context, messages []Message, options SummarizationOptions) (string, error) {
	// Extract key messages
	var keyMessages []string
	for _, msg := range messages {
		if msg.Role == "user" || (msg.Role == "assistant" && len(msg.Content) > 50) {
			keyMessages = append(keyMessages, fmt.Sprintf("%s: %s", msg.Role, msg.Content))
		}
	}

	text := strings.Join(keyMessages, "\n")
	return p.Summarize(ctx, text, options)
}

// ExtractKeyPoints extracts key points from text
func (p *LocalSummarizationProvider) ExtractKeyPoints(ctx context.Context, text string) ([]string, error) {
	sentences := p.splitIntoSentences(text)
	scores := p.scoreSentences(sentences)

	// Select top 3-5 sentences as key points
	keyPoints := p.selectTopSentences(sentences, scores, 3)

	// Clean up key points
	for i, point := range keyPoints {
		keyPoints[i] = strings.TrimSpace(point)
	}

	return keyPoints, nil
}

// GetModelName returns the model name
func (p *LocalSummarizationProvider) GetModelName() string {
	return "extractive-local"
}

// splitIntoSentences splits text into sentences
func (p *LocalSummarizationProvider) splitIntoSentences(text string) []string {
	// Simple sentence splitting (production would use NLP library)
	text = strings.ReplaceAll(text, "\n", " ")
	sentences := strings.Split(text, ". ")

	var result []string
	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if len(sent) > 20 { // Filter short sentences
			if !strings.HasSuffix(sent, ".") {
				sent += "."
			}
			result = append(result, sent)
		}
	}

	return result
}

// scoreSentences scores sentences based on word frequency
func (p *LocalSummarizationProvider) scoreSentences(sentences []string) []float64 {
	// Calculate word frequencies
	wordFreq := make(map[string]int)
	for _, sent := range sentences {
		words := strings.Fields(strings.ToLower(sent))
		for _, word := range words {
			wordFreq[word]++
		}
	}

	// Score sentences
	scores := make([]float64, len(sentences))
	for i, sent := range sentences {
		words := strings.Fields(strings.ToLower(sent))
		score := 0.0
		for _, word := range words {
			score += float64(wordFreq[word])
		}
		scores[i] = score / float64(len(words)) // Normalize by length
	}

	return scores
}

// selectTopSentences selects top scoring sentences
func (p *LocalSummarizationProvider) selectTopSentences(sentences []string, scores []float64, n int) []string {
	if n > len(sentences) {
		n = len(sentences)
	}

	// Create indices for sorting
	indices := make([]int, len(sentences))
	for i := range indices {
		indices[i] = i
	}

	// Sort by score
	for i := 0; i < len(indices)-1; i++ {
		for j := i + 1; j < len(indices); j++ {
			if scores[indices[j]] > scores[indices[i]] {
				indices[i], indices[j] = indices[j], indices[i]
			}
		}
	}

	// Select top n sentences in original order
	selected := make([]bool, len(sentences))
	for i := 0; i < n; i++ {
		selected[indices[i]] = true
	}

	var result []string
	for i, sent := range sentences {
		if selected[i] {
			result = append(result, sent)
		}
	}

	return result
}

// OpenAISummarizationProvider uses OpenAI for summarization
type OpenAISummarizationProvider struct {
	config *SummarizationConfig
	apiKey string
	logger observability.Logger
}

// NewOpenAISummarizationProvider creates a new OpenAI provider
func NewOpenAISummarizationProvider(config *SummarizationConfig, logger observability.Logger) (*OpenAISummarizationProvider, error) {
	apiKey, ok := config.ProviderConfig["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("api_key not configured for OpenAI provider")
	}

	return &OpenAISummarizationProvider{
		config: config,
		apiKey: apiKey,
		logger: logger,
	}, nil
}

// Summarize generates a summary using OpenAI
func (p *OpenAISummarizationProvider) Summarize(ctx context.Context, text string, options SummarizationOptions) (string, error) {
	// This would make an API call to OpenAI
	// For now, return a mock summary
	return fmt.Sprintf("Summary of %d characters of text in %s style.", len(text), options.Style), nil
}

// SummarizeConversation summarizes a conversation
func (p *OpenAISummarizationProvider) SummarizeConversation(ctx context.Context, messages []Message, options SummarizationOptions) (string, error) {
	return fmt.Sprintf("Summary of conversation with %d messages.", len(messages)), nil
}

// ExtractKeyPoints extracts key points
func (p *OpenAISummarizationProvider) ExtractKeyPoints(ctx context.Context, text string) ([]string, error) {
	return []string{
		"Key point 1 from the text",
		"Key point 2 from the text",
		"Key point 3 from the text",
	}, nil
}

// GetModelName returns the model name
func (p *OpenAISummarizationProvider) GetModelName() string {
	return p.config.Model
}

// AnthropicSummarizationProvider uses Anthropic for summarization
type AnthropicSummarizationProvider struct {
	config *SummarizationConfig
	apiKey string
	logger observability.Logger
}

// NewAnthropicSummarizationProvider creates a new Anthropic provider
func NewAnthropicSummarizationProvider(config *SummarizationConfig, logger observability.Logger) (*AnthropicSummarizationProvider, error) {
	apiKey, ok := config.ProviderConfig["api_key"].(string)
	if !ok || apiKey == "" {
		return nil, fmt.Errorf("api_key not configured for Anthropic provider")
	}

	return &AnthropicSummarizationProvider{
		config: config,
		apiKey: apiKey,
		logger: logger,
	}, nil
}

// Summarize generates a summary using Anthropic
func (p *AnthropicSummarizationProvider) Summarize(ctx context.Context, text string, options SummarizationOptions) (string, error) {
	// This would make an API call to Anthropic
	// For now, return a mock summary
	return fmt.Sprintf("Anthropic summary of %d characters in %s style.", len(text), options.Style), nil
}

// SummarizeConversation summarizes a conversation
func (p *AnthropicSummarizationProvider) SummarizeConversation(ctx context.Context, messages []Message, options SummarizationOptions) (string, error) {
	return fmt.Sprintf("Anthropic conversation summary with %d messages.", len(messages)), nil
}

// ExtractKeyPoints extracts key points
func (p *AnthropicSummarizationProvider) ExtractKeyPoints(ctx context.Context, text string) ([]string, error) {
	return []string{
		"Anthropic key point 1",
		"Anthropic key point 2",
		"Anthropic key point 3",
	}, nil
}

// GetModelName returns the model name
func (p *AnthropicSummarizationProvider) GetModelName() string {
	return p.config.Model
}
