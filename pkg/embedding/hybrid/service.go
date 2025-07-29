package hybrid

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/resilience"
	"github.com/developer-mesh/developer-mesh/pkg/retry"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"golang.org/x/sync/semaphore"
)

// EmbeddingService interface to avoid import cycle
type EmbeddingService interface {
	GenerateEmbedding(ctx context.Context, text, contentType, model string) (*EmbeddingVector, error)
}

// EmbeddingVector represents an embedding vector
type EmbeddingVector struct {
	ContentID   string                 `json:"content_id"`
	ContentType string                 `json:"content_type"`
	Vector      []float32              `json:"vector"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
	ModelName   string                 `json:"model_name"`
	CreatedAt   time.Time              `json:"created_at"`
}

// HybridSearchService provides hybrid search capabilities combining vector and keyword search
type HybridSearchService struct {
	db               *sql.DB
	embeddingService EmbeddingService
	logger           observability.Logger
	metrics          observability.MetricsClient
	semaphore        *semaphore.Weighted
	circuitBreaker   *resilience.CircuitBreaker
	retryPolicy      retry.Policy
}

// Config contains configuration for the hybrid search service
type Config struct {
	DB                 *sql.DB
	EmbeddingService   EmbeddingService
	Logger             observability.Logger
	Metrics            observability.MetricsClient
	MaxConcurrency     int64
	CircuitBreakerName string
}

// SearchOptions contains options for hybrid search
type SearchOptions struct {
	Query             string                 // Search query text
	Limit             int                    // Maximum results to return
	Offset            int                    // Pagination offset
	VectorWeight      float32                // Weight for vector search (0-1)
	KeywordWeight     float32                // Weight for keyword search (0-1)
	MinSimilarity     float32                // Minimum similarity threshold
	Filters           map[string]interface{} // Metadata filters
	UseReranking      bool                   // Whether to use reranking
	IncludeContent    bool                   // Include full content in results
	HighlightKeywords bool                   // Highlight matched keywords
	ExpandQuery       bool                   // Use query expansion
	SearchModels      []string               // Specific models to search
	ExcludeModels     []string               // Models to exclude
	AgentID           *uuid.UUID             // Filter by agent ID
	ContentTypes      []string               // Filter by content types
	DateFrom          *time.Time             // Filter by date range
	DateTo            *time.Time             // Filter by date range
	FusionK           int                    // K parameter for RRF (default 60)
}

// SearchResult represents a single search result
type SearchResult struct {
	ID              uuid.UUID              `json:"id"`
	ContentID       string                 `json:"content_id"`
	Content         string                 `json:"content"`
	ContentType     string                 `json:"content_type"`
	VectorScore     float32                `json:"vector_score"`
	KeywordScore    float32                `json:"keyword_score"`
	HybridScore     float32                `json:"hybrid_score"`
	HighlightedText string                 `json:"highlighted_text,omitempty"`
	Metadata        map[string]interface{} `json:"metadata"`
	ModelName       string                 `json:"model_name"`
	CreatedAt       time.Time              `json:"created_at"`
}

// SearchResults contains the search results and metadata
type SearchResults struct {
	Results       []*SearchResult `json:"results"`
	TotalResults  int             `json:"total_results"`
	SearchTime    float64         `json:"search_time_ms"`
	VectorHits    int             `json:"vector_hits"`
	KeywordHits   int             `json:"keyword_hits"`
	Query         string          `json:"query"`
	ExpandedTerms []string        `json:"expanded_terms,omitempty"`
}

// NewHybridSearchService creates a new hybrid search service
func NewHybridSearchService(config *Config) (*HybridSearchService, error) {
	if config.DB == nil {
		return nil, fmt.Errorf("database connection is required")
	}
	if config.EmbeddingService == nil {
		return nil, fmt.Errorf("embedding service is required")
	}

	// Set defaults
	if config.Logger == nil {
		config.Logger = observability.NewLogger("embedding.hybrid")
	}
	if config.Metrics == nil {
		config.Metrics = observability.NewMetricsClient()
	}
	if config.MaxConcurrency <= 0 {
		config.MaxConcurrency = 10
	}
	if config.CircuitBreakerName == "" {
		config.CircuitBreakerName = "hybrid_search"
	}

	// Create circuit breaker
	cb := resilience.NewCircuitBreaker(config.CircuitBreakerName, resilience.CircuitBreakerConfig{
		FailureThreshold:    5,
		FailureRatio:        0.5,
		ResetTimeout:        60 * time.Second,
		SuccessThreshold:    2,
		MaxRequestsHalfOpen: 10,
		TimeoutThreshold:    30 * time.Second,
	}, config.Logger, config.Metrics)

	// Create retry policy
	retryPolicy := retry.NewExponentialBackoff(retry.Config{
		InitialInterval: 100 * time.Millisecond,
		MaxInterval:     5 * time.Second,
		MaxElapsedTime:  30 * time.Second,
		Multiplier:      2.0,
	})

	return &HybridSearchService{
		db:               config.DB,
		embeddingService: config.EmbeddingService,
		logger:           config.Logger,
		metrics:          config.Metrics,
		semaphore:        semaphore.NewWeighted(config.MaxConcurrency),
		circuitBreaker:   cb,
		retryPolicy:      retryPolicy,
	}, nil
}

// Search performs a hybrid search combining vector and keyword search
func (h *HybridSearchService) Search(ctx context.Context, query string, opts *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "hybrid.search")
	defer span.End()

	// Input validation
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	// Extract tenant ID from context
	tenantID := auth.GetTenantID(ctx)
	if tenantID == uuid.Nil {
		return nil, fmt.Errorf("tenant ID not found in context")
	}

	// Set default options
	if opts == nil {
		opts = &SearchOptions{
			Limit:         10,
			VectorWeight:  0.7,
			KeywordWeight: 0.3,
			MinSimilarity: 0.5,
			FusionK:       60,
		}
	}

	// Validate weights
	if opts.VectorWeight+opts.KeywordWeight != 1.0 {
		// Normalize weights
		total := opts.VectorWeight + opts.KeywordWeight
		if total > 0 {
			opts.VectorWeight = opts.VectorWeight / total
			opts.KeywordWeight = opts.KeywordWeight / total
		} else {
			opts.VectorWeight = 0.7
			opts.KeywordWeight = 0.3
		}
	}

	// Log search request
	h.logger.Info("Performing hybrid search", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"query":          query,
		"limit":          opts.Limit,
		"vector_weight":  opts.VectorWeight,
		"keyword_weight": opts.KeywordWeight,
	})

	// Track metrics
	startTime := time.Now()
	defer func() {
		duration := time.Since(startTime)
		h.metrics.RecordHistogram("hybrid_search.duration", duration.Seconds(), map[string]string{
			"tenant": tenantID.String(),
		})
	}()

	// Acquire semaphore
	if err := h.semaphore.Acquire(ctx, 1); err != nil {
		return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
	}
	defer h.semaphore.Release(1)

	// Perform search with circuit breaker
	result, err := h.circuitBreaker.Execute(ctx, func() (interface{}, error) {
		return h.doSearch(ctx, tenantID, query, opts)
	})

	if err != nil {
		h.metrics.IncrementCounter("hybrid_search.error", 1.0)
		return nil, fmt.Errorf("hybrid search failed: %w", err)
	}

	results, ok := result.(*SearchResults)
	if !ok {
		return nil, fmt.Errorf("unexpected result type from circuit breaker")
	}

	h.metrics.IncrementCounter("hybrid_search.success", 1.0)
	return results, nil
}

// doSearch performs the actual hybrid search
func (h *HybridSearchService) doSearch(ctx context.Context, tenantID uuid.UUID, query string, opts *SearchOptions) (*SearchResults, error) {
	startTime := time.Now()

	// Update statistics for tenant
	if err := h.updateStatistics(ctx, tenantID); err != nil {
		h.logger.Warn("Failed to update statistics", map[string]interface{}{
			"error":     err.Error(),
			"tenant_id": tenantID.String(),
		})
	}

	// Perform vector and keyword searches in parallel
	type vectorResult struct {
		results []*SearchResult
		err     error
	}
	type keywordResult struct {
		results []*SearchResult
		err     error
	}

	vectorChan := make(chan vectorResult, 1)
	keywordChan := make(chan keywordResult, 1)

	// Vector search
	go func() {
		results, err := h.vectorSearch(ctx, tenantID, query, opts)
		vectorChan <- vectorResult{results: results, err: err}
	}()

	// Keyword search
	go func() {
		results, err := h.keywordSearch(ctx, tenantID, query, opts)
		keywordChan <- keywordResult{results: results, err: err}
	}()

	// Wait for results
	vResult := <-vectorChan
	kResult := <-keywordChan

	if vResult.err != nil && kResult.err != nil {
		return nil, fmt.Errorf("both searches failed: vector=%w, keyword=%w", vResult.err, kResult.err)
	}

	// Handle partial failures gracefully
	var vectorResults, keywordResults []*SearchResult
	if vResult.err == nil {
		vectorResults = vResult.results
	} else {
		h.logger.Warn("Vector search failed, continuing with keyword only", map[string]interface{}{
			"error": vResult.err.Error(),
		})
	}

	if kResult.err == nil {
		keywordResults = kResult.results
	} else {
		h.logger.Warn("Keyword search failed, continuing with vector only", map[string]interface{}{
			"error": kResult.err.Error(),
		})
	}

	// Fuse results using RRF
	fusedResults := h.fuseResults(vectorResults, keywordResults, opts)

	// Apply limit
	if len(fusedResults) > opts.Limit {
		fusedResults = fusedResults[:opts.Limit]
	}

	searchTime := time.Since(startTime).Milliseconds()

	return &SearchResults{
		Results:      fusedResults,
		TotalResults: len(fusedResults),
		SearchTime:   float64(searchTime),
		VectorHits:   len(vectorResults),
		KeywordHits:  len(keywordResults),
		Query:        query,
	}, nil
}

// vectorSearch performs vector similarity search
func (h *HybridSearchService) vectorSearch(ctx context.Context, tenantID uuid.UUID, query string, opts *SearchOptions) ([]*SearchResult, error) {
	// Generate embedding with retry
	var embedding *EmbeddingVector
	err := h.retryPolicy.Execute(ctx, func(ctx context.Context) error {
		var err error
		embedding, err = h.embeddingService.GenerateEmbedding(ctx, query, "search_query", "")
		return err
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Build query
	query = `
		SELECT 
			e.id,
			e.content_id,
			e.content,
			e.content_type,
			e.metadata,
			e.model_name,
			e.created_at,
			1 - (e.embedding <=> $1::vector) as similarity
		FROM embeddings e
		WHERE e.tenant_id = $2
			AND 1 - (e.embedding <=> $1::vector) >= $3
	`

	args := []interface{}{
		pq.Array(embedding.Vector),
		tenantID,
		opts.MinSimilarity,
	}
	argCount := 3

	// Add filters
	query += h.buildFilters(opts, &args, &argCount)

	// Add ordering and limit
	query += fmt.Sprintf(" ORDER BY similarity DESC LIMIT %d OFFSET %d", opts.Limit*2, opts.Offset)

	// Execute query
	rows, err := h.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search query failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			h.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []*SearchResult
	for rows.Next() {
		var r SearchResult
		var metadataJSON []byte

		err := rows.Scan(
			&r.ID,
			&r.ContentID,
			&r.Content,
			&r.ContentType,
			&metadataJSON,
			&r.ModelName,
			&r.CreatedAt,
			&r.VectorScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			r.Metadata = make(map[string]interface{})
			// Ignore unmarshal errors for resilience
			_ = json.Unmarshal(metadataJSON, &r.Metadata)
		}

		results = append(results, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

// keywordSearch performs BM25 keyword search
func (h *HybridSearchService) keywordSearch(ctx context.Context, tenantID uuid.UUID, query string, opts *SearchOptions) ([]*SearchResult, error) {
	// Get statistics
	var totalDocs int
	var avgDocLength float64
	err := h.db.QueryRowContext(ctx, `
		SELECT total_documents, avg_document_length 
		FROM embedding_statistics 
		WHERE tenant_id = $1
	`, tenantID).Scan(&totalDocs, &avgDocLength)
	if err != nil {
		// Use defaults if statistics not available
		totalDocs = 1000
		avgDocLength = 100
	}

	// Tokenize query
	queryTerms := h.tokenizeQuery(query)

	// Build query
	sqlQuery := `
		SELECT 
			e.id,
			e.content_id,
			e.content,
			e.content_type,
			e.metadata,
			e.model_name,
			e.created_at,
			bm25_score($1::text[], e.content_tsvector, e.document_length, $2, $3) as score
		FROM embeddings e
		WHERE e.tenant_id = $4
			AND e.content_tsvector @@ to_tsquery('english', $5)
	`

	args := []interface{}{
		pq.Array(queryTerms),
		avgDocLength,
		totalDocs,
		tenantID,
		strings.Join(queryTerms, " & "),
	}
	argCount := 5

	// Add filters
	sqlQuery += h.buildFilters(opts, &args, &argCount)

	// Add ordering and limit
	sqlQuery += fmt.Sprintf(" ORDER BY score DESC LIMIT %d OFFSET %d", opts.Limit*2, opts.Offset)

	// Execute query
	rows, err := h.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("keyword search query failed: %w", err)
	}
	defer func() {
		if err := rows.Close(); err != nil {
			h.logger.Warn("Failed to close rows", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var results []*SearchResult
	for rows.Next() {
		var r SearchResult
		var metadataJSON []byte

		err := rows.Scan(
			&r.ID,
			&r.ContentID,
			&r.Content,
			&r.ContentType,
			&metadataJSON,
			&r.ModelName,
			&r.CreatedAt,
			&r.KeywordScore,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			r.Metadata = make(map[string]interface{})
			_ = json.Unmarshal(metadataJSON, &r.Metadata)
		}

		// Highlight keywords if requested
		if opts.HighlightKeywords {
			r.HighlightedText = h.highlightKeywords(r.Content, queryTerms)
		}

		results = append(results, &r)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	return results, nil
}

// fuseResults combines vector and keyword results using Reciprocal Rank Fusion
func (h *HybridSearchService) fuseResults(vectorResults, keywordResults []*SearchResult, opts *SearchOptions) []*SearchResult {
	// Create map for deduplication and score combination
	resultMap := make(map[uuid.UUID]*SearchResult)

	// Process vector results
	for i, result := range vectorResults {
		r := *result // Copy
		r.HybridScore = opts.VectorWeight * r.VectorScore / float32(opts.FusionK+i+1)
		resultMap[r.ID] = &r
	}

	// Process keyword results
	for i, result := range keywordResults {
		if existing, ok := resultMap[result.ID]; ok {
			// Combine scores
			existing.KeywordScore = result.KeywordScore
			existing.HybridScore += opts.KeywordWeight * result.KeywordScore / float32(opts.FusionK+i+1)
			if result.HighlightedText != "" {
				existing.HighlightedText = result.HighlightedText
			}
		} else {
			// New result from keyword search
			r := *result // Copy
			r.HybridScore = opts.KeywordWeight * r.KeywordScore / float32(opts.FusionK+i+1)
			resultMap[r.ID] = &r
		}
	}

	// Convert to slice and sort by hybrid score
	results := make([]*SearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, r)
	}

	// Sort by hybrid score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})

	return results
}

// buildFilters builds SQL filter clauses
func (h *HybridSearchService) buildFilters(opts *SearchOptions, args *[]interface{}, argCount *int) string {
	var filters []string

	// Agent ID filter
	if opts.AgentID != nil {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.metadata->>'agent_id' = $%d", *argCount))
		*args = append(*args, opts.AgentID.String())
	}

	// Content type filter
	if len(opts.ContentTypes) > 0 {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.content_type = ANY($%d)", *argCount))
		*args = append(*args, pq.Array(opts.ContentTypes))
	}

	// Model filters
	if len(opts.SearchModels) > 0 {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.model_name = ANY($%d)", *argCount))
		*args = append(*args, pq.Array(opts.SearchModels))
	}

	if len(opts.ExcludeModels) > 0 {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.model_name != ALL($%d)", *argCount))
		*args = append(*args, pq.Array(opts.ExcludeModels))
	}

	// Date filters
	if opts.DateFrom != nil {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.created_at >= $%d", *argCount))
		*args = append(*args, *opts.DateFrom)
	}

	if opts.DateTo != nil {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.created_at <= $%d", *argCount))
		*args = append(*args, *opts.DateTo)
	}

	// Metadata filters
	for key, value := range opts.Filters {
		*argCount++
		filters = append(filters, fmt.Sprintf("e.metadata->>%s = $%d", pq.QuoteLiteral(key), *argCount))
		*args = append(*args, fmt.Sprintf("%v", value))
	}

	if len(filters) > 0 {
		return " AND " + strings.Join(filters, " AND ")
	}
	return ""
}

// tokenizeQuery tokenizes the search query
func (h *HybridSearchService) tokenizeQuery(query string) []string {
	// Simple tokenization - in production, use NLP library
	query = strings.ToLower(query)
	words := strings.Fields(query)

	// Remove common stop words
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "and": true, "or": true,
		"but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "from": true,
		"is": true, "was": true, "are": true, "were": true,
	}

	var tokens []string
	for _, word := range words {
		word = strings.Trim(word, ".,!?;:\"'")
		if len(word) > 2 && !stopWords[word] {
			tokens = append(tokens, word)
		}
	}

	return tokens
}

// highlightKeywords highlights keywords in text
func (h *HybridSearchService) highlightKeywords(content string, keywords []string) string {
	highlighted := content
	for _, keyword := range keywords {
		// Case-insensitive replacement
		re := regexp.MustCompile("(?i)\\b" + regexp.QuoteMeta(keyword) + "\\b")
		highlighted = re.ReplaceAllString(highlighted, "<mark>$0</mark>")
	}
	return highlighted
}

// updateStatistics updates embedding statistics for a tenant
func (h *HybridSearchService) updateStatistics(ctx context.Context, tenantID uuid.UUID) error {
	_, err := h.db.ExecContext(ctx, "SELECT update_embedding_statistics($1)", tenantID)
	return err
}
