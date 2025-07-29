package embedding

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/expansion"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/hybrid"
	"github.com/developer-mesh/developer-mesh/pkg/embedding/rerank"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	repositorySearch "github.com/developer-mesh/developer-mesh/pkg/repository/search"
	"github.com/google/uuid"
	"github.com/lib/pq"
)

// UnifiedSearchService implements the SearchService interface with advanced features
type UnifiedSearchService struct {
	db               *sql.DB
	repository       *Repository
	searchRepository repositorySearch.Repository
	embeddingService EmbeddingService
	dimensionAdapter *DimensionAdapter
	hybridSearch     *hybrid.HybridSearchService
	reranker         rerank.Reranker
	queryExpander    expansion.QueryExpander
	logger           observability.Logger
	metrics          observability.MetricsClient
}

// UnifiedSearchConfig contains configuration for the unified search service
type UnifiedSearchConfig struct {
	DB               *sql.DB
	Repository       *Repository
	SearchRepository repositorySearch.Repository
	EmbeddingService EmbeddingService
	DimensionAdapter *DimensionAdapter
	HybridSearch     *hybrid.HybridSearchService
	Reranker         rerank.Reranker
	QueryExpander    expansion.QueryExpander
	Logger           observability.Logger
	Metrics          observability.MetricsClient
}

// NewUnifiedSearchService creates a new unified search service
func NewUnifiedSearchService(config *UnifiedSearchConfig) (*UnifiedSearchService, error) {
	if config.DB == nil {
		return nil, errors.New("database connection is required")
	}
	if config.EmbeddingService == nil {
		return nil, errors.New("embedding service is required")
	}
	if config.SearchRepository == nil {
		return nil, errors.New("search repository is required")
	}

	// Create logger and metrics if not provided
	if config.Logger == nil {
		config.Logger = observability.NewLogger("embedding.search.unified")
	}
	if config.Metrics == nil {
		config.Metrics = observability.NewMetricsClient()
	}

	// Create hybrid search service if not provided
	if config.HybridSearch == nil && config.DB != nil && config.EmbeddingService != nil {
		// Create an adapter to convert between embedding types
		adapter := &embeddingServiceAdapter{service: config.EmbeddingService}

		hybridConfig := &hybrid.Config{
			DB:               config.DB,
			EmbeddingService: adapter,
			Logger:           config.Logger,
			Metrics:          config.Metrics,
		}
		var err error
		config.HybridSearch, err = hybrid.NewHybridSearchService(hybridConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create hybrid search service: %w", err)
		}
	}

	return &UnifiedSearchService{
		db:               config.DB,
		repository:       config.Repository,
		searchRepository: config.SearchRepository,
		embeddingService: config.EmbeddingService,
		dimensionAdapter: config.DimensionAdapter,
		hybridSearch:     config.HybridSearch,
		reranker:         config.Reranker,
		queryExpander:    config.QueryExpander,
		logger:           config.Logger,
		metrics:          config.Metrics,
	}, nil
}

// Search performs a vector search with the given text
func (s *UnifiedSearchService) Search(ctx context.Context, text string, options *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.text")
	defer span.End()

	span.SetAttribute("operation", "search_by_text")
	span.SetAttribute("query_length", len(text))

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	s.logger.Info("Performing text search", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
		"query_length":   len(text),
		"has_options":    options != nil,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"method": "text",
			"tenant": tenantID.String(),
		}
		s.metrics.RecordHistogram("search.unified.duration", duration.Seconds(), labels)
		s.metrics.IncrementCounter("search.unified.total", 1.0)
	}()

	if text == "" {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		err := errors.New("search text cannot be empty")
		span.RecordError(err)
		span.SetStatus(400, "Invalid input")
		return nil, err
	}

	// Apply query expansion if configured
	queries := []string{text}
	if s.queryExpander != nil && options != nil && options.UseQueryExpansion {
		expandedQueries, err := s.expandQuery(ctx, text, options)
		if err != nil {
			// Log error but continue with original query
			s.logger.Warn("Query expansion failed", map[string]interface{}{
				"error": err.Error(),
				"query": text,
			})
		} else if len(expandedQueries) > 0 {
			queries = expandedQueries
		}
	}

	// Perform searches with all queries
	if len(queries) > 1 {
		return s.multiQuerySearch(ctx, queries, options)
	}

	// Generate embedding for the search text
	s.logger.Debug("Generating embedding for search text", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
	})

	embedding, err := s.embeddingService.GenerateEmbedding(ctx, text, "search_query", "")
	if err != nil {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		s.logger.Error("Failed to generate embedding", map[string]interface{}{
			"error":          err.Error(),
			"tenant_id":      tenantID.String(),
			"correlation_id": correlationID,
		})
		span.RecordError(err)
		span.SetStatus(500, "Embedding generation failed")
		return nil, fmt.Errorf("failed to generate embedding: %w", err)
	}

	// Search with the generated vector
	results, err := s.SearchByVector(ctx, embedding.Vector, options)
	if err != nil {
		return nil, err
	}

	// Apply reranking if configured
	if s.reranker != nil && options != nil && options.UseReranking {
		return s.applyReranking(ctx, text, results, options)
	}

	return results, nil
}

// SearchByVector performs a vector search with a pre-computed vector
func (s *UnifiedSearchService) SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.vector")
	defer span.End()

	span.SetAttribute("operation", "search_by_vector")
	span.SetAttribute("vector_dimensions", len(vector))

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	s.logger.Info("Performing vector search", map[string]interface{}{
		"tenant_id":         tenantID.String(),
		"correlation_id":    correlationID,
		"vector_dimensions": len(vector),
		"has_options":       options != nil,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"method": "vector",
			"tenant": tenantID.String(),
		}
		s.metrics.RecordHistogram("search.unified.duration", duration.Seconds(), labels)
	}()

	if len(vector) == 0 {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		err := errors.New("search vector cannot be empty")
		span.RecordError(err)
		span.SetStatus(400, "Invalid input")
		return nil, err
	}

	// Convert SearchOptions to repository SearchOptions
	repoOptions := s.convertToRepoOptions(options)

	// Use the search repository for vector search
	resultsPtr, err := s.searchRepository.SearchByVector(ctx, vector, repoOptions)
	if err != nil {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		s.logger.Error("Vector search failed", map[string]interface{}{
			"error":          err.Error(),
			"tenant_id":      tenantID.String(),
			"correlation_id": correlationID,
		})
		span.RecordError(err)
		span.SetStatus(500, "Search failed")
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	// Convert repository results to SearchResults
	var results []repositorySearch.SearchResult
	if resultsPtr != nil && resultsPtr.Results != nil {
		// Convert pointer slice to value slice
		for _, r := range resultsPtr.Results {
			if r != nil {
				results = append(results, *r)
			}
		}
	}
	searchResults := s.convertToSearchResults(results)

	s.logger.Debug("Vector search completed", map[string]interface{}{
		"result_count":   len(searchResults.Results),
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
	})

	// Apply reranking if configured for vector search
	if s.reranker != nil && options != nil && options.UseReranking && options.RerankQuery != "" {
		return s.applyReranking(ctx, options.RerankQuery, searchResults, options)
	}

	return searchResults, nil
}

// SearchByContentID performs a "more like this" search based on an existing content ID
func (s *UnifiedSearchService) SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.content_id")
	defer span.End()

	span.SetAttribute("operation", "search_by_content_id")
	span.SetAttribute("content_id", contentID)

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	s.logger.Info("Performing similar content search", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
		"content_id":     contentID,
		"has_options":    options != nil,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"method": "content_id",
			"tenant": tenantID.String(),
		}
		s.metrics.RecordHistogram("search.unified.duration", duration.Seconds(), labels)
	}()

	if contentID == "" {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		err := errors.New("content ID cannot be empty")
		span.RecordError(err)
		span.SetStatus(400, "Invalid input")
		return nil, err
	}

	// Convert SearchOptions to repository SearchOptions
	repoOptions := s.convertToRepoOptions(options)

	// Use the search repository for content-based search
	resultsPtr, err := s.searchRepository.SearchByContentID(ctx, contentID, repoOptions)
	if err != nil {
		s.metrics.IncrementCounter("search.unified.error", 1.0)
		s.logger.Error("Content search failed", map[string]interface{}{
			"error":          err.Error(),
			"tenant_id":      tenantID.String(),
			"correlation_id": correlationID,
			"content_id":     contentID,
		})
		span.RecordError(err)
		span.SetStatus(500, "Search failed")
		return nil, fmt.Errorf("content search failed: %w", err)
	}

	// Convert repository results to SearchResults
	var results []repositorySearch.SearchResult
	if resultsPtr != nil && resultsPtr.Results != nil {
		// Convert pointer slice to value slice
		for _, r := range resultsPtr.Results {
			if r != nil {
				results = append(results, *r)
			}
		}
	}
	searchResults := s.convertToSearchResults(results)

	s.logger.Debug("Content search completed", map[string]interface{}{
		"result_count":   len(searchResults.Results),
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
		"content_id":     contentID,
	})

	return searchResults, nil
}

// CrossModelSearch performs search across embeddings from different models
func (s *UnifiedSearchService) CrossModelSearch(ctx context.Context, req CrossModelSearchRequest) ([]CrossModelSearchResult, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.cross_model")
	defer span.End()

	span.SetAttribute("operation", "cross_model_search")
	span.SetAttribute("search_model", req.SearchModel)
	span.SetAttribute("limit", req.Limit)

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	s.logger.Info("Performing cross-model search", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
		"search_model":   req.SearchModel,
		"include_models": req.IncludeModels,
		"exclude_models": req.ExcludeModels,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"method": "cross_model",
			"tenant": req.TenantID.String(),
			"model":  req.SearchModel,
		}
		s.metrics.RecordHistogram("search.unified.cross_model.duration", duration.Seconds(), labels)
		s.metrics.IncrementCounter("search.unified.cross_model.total", 1.0)
	}()

	// Validate request
	if err := s.validateCrossModelRequest(&req); err != nil {
		s.metrics.IncrementCounter("search.unified.cross_model.error", 1.0)
		span.RecordError(err)
		span.SetStatus(400, "Invalid request")
		return nil, err
	}

	// Generate embedding if needed
	if len(req.QueryEmbedding) == 0 && req.Query != "" {
		embedding, err := s.embeddingService.GenerateEmbedding(ctx, req.Query, "search_query", req.SearchModel)
		if err != nil {
			s.metrics.IncrementCounter("search.unified.cross_model.error", 1.0)
			span.RecordError(err)
			return nil, fmt.Errorf("failed to generate embedding: %w", err)
		}
		req.QueryEmbedding = embedding.Vector
	}

	// Determine target dimension
	targetDimension := StandardDimension
	if req.SearchModel != "" {
		if model, err := s.repository.GetModelByName(ctx, req.SearchModel); err == nil {
			targetDimension = model.Dimensions
		}
	}

	// Build and execute query
	query, args := s.buildCrossModelQuery(req, targetDimension)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		s.metrics.IncrementCounter("search.unified.cross_model.error", 1.0)
		span.RecordError(err)
		return nil, fmt.Errorf("failed to execute cross-model search: %w", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	// Process results
	results, err := s.processCrossModelResults(rows, req, targetDimension)
	if err != nil {
		s.metrics.IncrementCounter("search.unified.cross_model.error", 1.0)
		span.RecordError(err)
		return nil, err
	}

	s.logger.Debug("Cross-model search completed", map[string]interface{}{
		"result_count":   len(results),
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
	})

	return results, nil
}

// HybridSearch performs hybrid search combining semantic and keyword search
func (s *UnifiedSearchService) HybridSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.hybrid")
	defer span.End()

	span.SetAttribute("operation", "hybrid_search")
	span.SetAttribute("hybrid_weight", req.HybridWeight)

	// Extract context for logging
	tenantID := auth.GetTenantID(ctx)
	correlationID := observability.GetCorrelationID(ctx)

	s.logger.Info("Performing hybrid search", map[string]interface{}{
		"tenant_id":      tenantID.String(),
		"correlation_id": correlationID,
		"query":          req.Query,
		"keywords":       req.Keywords,
		"hybrid_weight":  req.HybridWeight,
	})

	// Track metrics
	start := time.Now()
	defer func() {
		duration := time.Since(start)
		labels := map[string]string{
			"method": "hybrid",
			"tenant": req.TenantID.String(),
		}
		s.metrics.RecordHistogram("search.unified.hybrid.duration", duration.Seconds(), labels)
		s.metrics.IncrementCounter("search.unified.hybrid.total", 1.0)
	}()

	// Perform semantic search
	semanticResults, err := s.semanticSearch(ctx, req)
	if err != nil {
		s.metrics.IncrementCounter("search.unified.hybrid.error", 1.0)
		span.RecordError(err)
		return nil, fmt.Errorf("semantic search failed: %w", err)
	}

	// Perform keyword search if keywords provided
	var keywordResults []HybridSearchResult
	if len(req.Keywords) > 0 {
		keywordResults, err = s.keywordSearch(ctx, req)
		if err != nil {
			s.metrics.IncrementCounter("search.unified.hybrid.error", 1.0)
			span.RecordError(err)
			return nil, fmt.Errorf("keyword search failed: %w", err)
		}
	}

	// Merge and rank results
	merged := s.mergeHybridResults(semanticResults, keywordResults, float64(req.HybridWeight))

	// Apply limit
	if req.Limit > 0 && len(merged) > req.Limit {
		merged = merged[:req.Limit]
	}

	s.logger.Debug("Hybrid search completed", map[string]interface{}{
		"result_count":     len(merged),
		"semantic_results": len(semanticResults),
		"keyword_results":  len(keywordResults),
		"tenant_id":        tenantID.String(),
		"correlation_id":   correlationID,
	})

	return merged, nil
}

// Helper methods

func (s *UnifiedSearchService) convertToRepoOptions(options *SearchOptions) *repositorySearch.SearchOptions {
	if options == nil {
		return &repositorySearch.SearchOptions{
			Limit:         10,
			MinSimilarity: 0.7,
		}
	}

	// Map content types to metadata filters
	metadataFilters := make(map[string]interface{})
	if len(options.ContentTypes) > 0 {
		metadataFilters["content_types"] = options.ContentTypes
	}

	// Add existing metadata filters
	for _, filter := range options.Filters {
		if strings.HasPrefix(filter.Field, "metadata.") {
			field := strings.TrimPrefix(filter.Field, "metadata.")
			metadataFilters[field] = filter.Value
		}
	}

	// Determine ranking algorithm from weight factors
	rankingAlgorithm := "cosine"
	// WeightFactors is map[string]float32, so we can't store algorithm there
	// Default to cosine for now

	return &repositorySearch.SearchOptions{
		Limit:               options.Limit,
		Offset:              options.Offset,
		MinSimilarity:       options.MinSimilarity,
		SimilarityThreshold: options.MinSimilarity,
		MetadataFilters:     metadataFilters,
		RankingAlgorithm:    rankingAlgorithm,
		MaxResults:          options.Limit,
	}
}

func (s *UnifiedSearchService) convertToSearchResults(results []repositorySearch.SearchResult) *SearchResults {
	searchResults := &SearchResults{
		Results: make([]*SearchResult, len(results)),
		Total:   len(results),
		HasMore: false, // This would need pagination info from repository
	}

	for i, result := range results {
		// Create EmbeddingVector from repository result
		embedding := &EmbeddingVector{
			ContentID:   result.ID,
			ContentType: result.Type,
			Metadata:    make(map[string]interface{}),
		}

		// Copy metadata - it's already a map
		if result.Metadata != nil {
			for k, v := range result.Metadata {
				embedding.Metadata[k] = v
			}
		}

		// Extract similarity from result
		similarity := float32(0.0)
		if result.Score > 0 {
			similarity = result.Score
		}

		// Add similarity to metadata
		embedding.Metadata["similarity"] = similarity

		searchResults.Results[i] = &SearchResult{
			Content: embedding,
			Score:   similarity,
			Matches: map[string]interface{}{
				"similarity": similarity,
			},
		}
	}

	return searchResults
}

func (s *UnifiedSearchService) validateCrossModelRequest(req *CrossModelSearchRequest) error {
	if len(req.Query) == 0 && len(req.QueryEmbedding) == 0 {
		return fmt.Errorf("either query or query_embedding must be provided")
	}

	if req.Limit <= 0 {
		req.Limit = 10
	} else if req.Limit > 100 {
		req.Limit = 100
	}

	if req.MinSimilarity <= 0 {
		req.MinSimilarity = 0.7
	}

	return nil
}

func (s *UnifiedSearchService) buildCrossModelQuery(req CrossModelSearchRequest, targetDimension int) (string, []interface{}) {
	query := `
		WITH normalized_embeddings AS (
			SELECT 
				e.id,
				e.context_id,
				e.content,
				e.model_name as original_model,
				e.model_dimensions as original_dimension,
				e.embedding,
				e.metadata,
				e.created_at,
				COALESCE(e.metadata->>'agent_id', '') as agent_id,
				-- Calculate similarity based on normalized dimensions
				CASE 
					WHEN e.model_dimensions = $1 THEN
						1 - (e.embedding <=> $2::vector)
					ELSE
						-- Apply dimension normalization penalty
						(1 - (e.embedding <=> $2::vector)) * 
						(1 - ABS(e.model_dimensions - $1)::float / GREATEST(e.model_dimensions, $1)::float * 0.1)
				END as similarity
			FROM mcp.embeddings e
			WHERE e.tenant_id = $3
	`

	args := []interface{}{targetDimension, pq.Array(req.QueryEmbedding), req.TenantID}
	argCount := 3

	// Add filters
	if req.ContextID != nil {
		argCount++
		query += fmt.Sprintf(" AND e.context_id = $%d", argCount)
		args = append(args, *req.ContextID)
	}

	if len(req.IncludeModels) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.model_name = ANY($%d)", argCount)
		args = append(args, pq.Array(req.IncludeModels))
	}

	if len(req.ExcludeModels) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.model_name != ALL($%d)", argCount)
		args = append(args, pq.Array(req.ExcludeModels))
	}

	if len(req.MetadataFilter) > 0 {
		argCount++
		query += fmt.Sprintf(" AND e.metadata @> $%d", argCount)
		metadataJSON, _ := json.Marshal(req.MetadataFilter)
		args = append(args, metadataJSON)
	}

	// Close CTE and select results
	query += fmt.Sprintf(`
		)
		SELECT 
			id,
			context_id,
			content,
			original_model,
			original_dimension,
			embedding,
			similarity,
			agent_id,
			metadata,
			created_at
		FROM normalized_embeddings
		WHERE similarity >= $%d
		ORDER BY similarity DESC
		LIMIT $%d
	`, argCount+1, argCount+2)

	args = append(args, req.MinSimilarity, req.Limit)

	return query, args
}

func (s *UnifiedSearchService) processCrossModelResults(rows *sql.Rows, req CrossModelSearchRequest, targetDimension int) ([]CrossModelSearchResult, error) {
	var results []CrossModelSearchResult

	for rows.Next() {
		var result CrossModelSearchResult
		var metadataJSON []byte
		var embedding pq.Float32Array

		err := rows.Scan(
			&result.ID,
			&result.ContextID,
			&result.Content,
			&result.OriginalModel,
			&result.OriginalDimension,
			&embedding,
			&result.RawSimilarity,
			&result.AgentID,
			&metadataJSON,
			&result.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan result: %w", err)
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &result.Metadata); err != nil {
				result.Metadata = make(map[string]interface{})
			}
		}

		// Calculate normalized score
		result.Similarity = float32(s.normalizeScore(
			float64(result.RawSimilarity),
			result.OriginalModel,
			req.SearchModel,
			result.OriginalDimension,
			targetDimension,
		))

		// Get model quality score
		result.ModelQualityScore = float32(s.getModelQualityScore(result.OriginalModel))

		// Calculate final score
		result.FinalScore = float32(s.calculateFinalScore(
			float64(result.Similarity),
			float64(result.ModelQualityScore),
			req.TaskType,
		))

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating results: %w", err)
	}

	// Sort by final score
	sort.Slice(results, func(i, j int) bool {
		return results[i].FinalScore > results[j].FinalScore
	})

	return results, nil
}

func (s *UnifiedSearchService) semanticSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Generate embedding if needed
	var queryEmbedding []float32
	if len(req.QueryEmbedding) > 0 {
		queryEmbedding = req.QueryEmbedding
	} else if req.Query != "" {
		embedding, err := s.embeddingService.GenerateEmbedding(ctx, req.Query, "search_query", "")
		if err != nil {
			return nil, err
		}
		queryEmbedding = embedding.Vector
	}

	// Convert to cross-model search request
	crossReq := CrossModelSearchRequest{
		Query:          req.Query,
		QueryEmbedding: queryEmbedding,
		TenantID:       req.TenantID,
		Limit:          req.Limit * 2, // Get more for merging
		MetadataFilter: req.MetadataFilter,
		MinSimilarity:  0.5, // Lower threshold for hybrid
	}

	results, err := s.CrossModelSearch(ctx, crossReq)
	if err != nil {
		return nil, err
	}

	// Convert to hybrid results
	hybridResults := make([]HybridSearchResult, len(results))
	for i, r := range results {
		hybridResults[i] = HybridSearchResult{
			CrossModelSearchResult: r,
			SemanticScore:          r.FinalScore,
		}
	}

	return hybridResults, nil
}

func (s *UnifiedSearchService) keywordSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error) {
	// Build query string from keywords
	queryStr := s.buildTsQuery(req.Keywords)

	query := `
		SELECT 
			e.id,
			e.context_id,
			e.content,
			e.model_name,
			e.model_dimensions,
			e.metadata,
			e.created_at,
			COALESCE(e.metadata->>'agent_id', '') as agent_id,
			ts_rank_cd(to_tsvector('english', e.content), query) as rank
		FROM mcp.embeddings e,
			to_tsquery('english', $1) query
		WHERE e.tenant_id = $2
			AND to_tsvector('english', e.content) @@ query
		ORDER BY rank DESC
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, query, queryStr, req.TenantID, req.Limit*2)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = rows.Close()
	}()

	var results []HybridSearchResult
	for rows.Next() {
		var r HybridSearchResult
		var metadataJSON []byte
		var rank float64

		err := rows.Scan(
			&r.ID,
			&r.ContextID,
			&r.Content,
			&r.OriginalModel,
			&r.OriginalDimension,
			&metadataJSON,
			&r.CreatedAt,
			&r.AgentID,
			&rank,
		)
		if err != nil {
			return nil, err
		}

		// Parse metadata
		if len(metadataJSON) > 0 {
			if err := json.Unmarshal(metadataJSON, &r.Metadata); err != nil {
				r.Metadata = make(map[string]interface{})
			}
		}

		// Normalize keyword score to 0-1 range
		r.KeywordScore = float32(math.Min(1.0, rank/4.0))
		results = append(results, r)
	}

	return results, nil
}

func (s *UnifiedSearchService) mergeHybridResults(semantic, keyword []HybridSearchResult, weight float64) []HybridSearchResult {
	// Create map for deduplication
	resultMap := make(map[uuid.UUID]*HybridSearchResult)

	// Add semantic results
	for i := range semantic {
		r := semantic[i]
		r.HybridScore = float32(weight) * r.SemanticScore
		resultMap[r.ID] = &r
	}

	// Merge keyword results
	for i := range keyword {
		k := keyword[i]
		if existing, ok := resultMap[k.ID]; ok {
			// Combine scores
			existing.KeywordScore = k.KeywordScore
			existing.HybridScore = float32(weight)*existing.SemanticScore + float32(1-weight)*k.KeywordScore
		} else {
			// Add new result
			k.HybridScore = float32(1-weight) * k.KeywordScore
			resultMap[k.ID] = &k
		}
	}

	// Convert to slice
	results := make([]HybridSearchResult, 0, len(resultMap))
	for _, r := range resultMap {
		results = append(results, *r)
	}

	// Sort by hybrid score
	sort.Slice(results, func(i, j int) bool {
		return results[i].HybridScore > results[j].HybridScore
	})

	return results
}

func (s *UnifiedSearchService) normalizeScore(rawScore float64, sourceModel, targetModel string, sourceDim, targetDim int) float64 {
	// Base normalization
	normalized := rawScore

	// Apply dimension difference penalty
	if sourceDim != targetDim {
		dimRatio := float64(min(sourceDim, targetDim)) / float64(max(sourceDim, targetDim))
		normalized *= (0.9 + 0.1*dimRatio) // 10% max penalty for dimension mismatch
	}

	// Apply model-specific calibration
	modelCalibration := s.getModelCalibration(sourceModel, targetModel)
	normalized *= modelCalibration

	return math.Min(1.0, math.Max(0.0, normalized))
}

func (s *UnifiedSearchService) getModelQualityScore(model string) float64 {
	// Model quality scores based on empirical performance
	qualityScores := map[string]float64{
		"text-embedding-3-large":       0.95,
		"text-embedding-3-small":       0.90,
		"text-embedding-ada-002":       0.85,
		"voyage-large-2":               0.93,
		"voyage-2":                     0.88,
		"voyage-code-2":                0.92,
		"amazon.titan-embed-text-v2:0": 0.87,
		"cohere.embed-english-v3":      0.89,
		"cohere.embed-multilingual-v3": 0.91,
	}

	if score, ok := qualityScores[model]; ok {
		return score
	}
	return 0.80 // Default score for unknown models
}

func getModelFamily(model string) string {
	if strings.Contains(model, "text-embedding-ada") || strings.Contains(model, "text-embedding-3") {
		return "openai"
	}
	if strings.Contains(model, "voyage") {
		return "voyage"
	}
	if strings.Contains(model, "amazon.titan") || strings.Contains(model, "cohere") {
		return "bedrock"
	}
	if strings.Contains(model, "embed-") {
		return "cohere"
	}
	return "unknown"
}

func (s *UnifiedSearchService) getModelCalibration(sourceModel, targetModel string) float64 {
	if sourceModel == targetModel {
		return 1.0
	}

	// Simple heuristic based on model families
	sourceFamily := getModelFamily(sourceModel)
	targetFamily := getModelFamily(targetModel)

	if sourceFamily == targetFamily {
		return 0.95 // Same family, minor adjustment
	}

	// Cross-family calibration
	calibrationMap := map[string]map[string]float64{
		"openai": {
			"voyage":  0.92,
			"bedrock": 0.90,
			"cohere":  0.88,
		},
		"voyage": {
			"openai":  0.93,
			"bedrock": 0.91,
			"cohere":  0.89,
		},
		"bedrock": {
			"openai": 0.91,
			"voyage": 0.90,
			"cohere": 0.92,
		},
	}

	if cal, ok := calibrationMap[sourceFamily][targetFamily]; ok {
		return cal
	}

	return 0.85 // Default cross-family calibration
}

func (s *UnifiedSearchService) calculateFinalScore(similarity, quality float64, taskType string) float64 {
	// Task-specific weighting
	var simWeight, qualWeight float64

	switch taskType {
	case "research":
		simWeight = 0.6
		qualWeight = 0.4
	case "code_analysis":
		simWeight = 0.7
		qualWeight = 0.3
	case "multilingual":
		simWeight = 0.65
		qualWeight = 0.35
	default:
		simWeight = 0.8
		qualWeight = 0.2
	}

	return simWeight*similarity + qualWeight*quality
}

func (s *UnifiedSearchService) buildTsQuery(keywords []string) string {
	if len(keywords) == 0 {
		return ""
	}

	// Join keywords with AND operator
	query := ""
	for i, kw := range keywords {
		if i > 0 {
			query += " & "
		}
		query += kw
	}
	return query
}

// applyReranking applies reranking to search results
func (s *UnifiedSearchService) applyReranking(ctx context.Context, query string, results *SearchResults, options *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.rerank")
	defer span.End()

	span.SetAttribute("query", query)
	span.SetAttribute("input_count", len(results.Results))

	// Convert SearchResults to rerank.SearchResult
	rerankInput := make([]rerank.SearchResult, 0, len(results.Results))
	for _, result := range results.Results {
		if result.Content == nil {
			continue
		}

		// Get content text from metadata or use content ID
		contentText := ""
		if content, ok := result.Content.Metadata["content"].(string); ok {
			contentText = content
		} else if text, ok := result.Content.Metadata["text"].(string); ok {
			contentText = text
		} else {
			// Fallback to content ID
			contentText = result.Content.ContentID
		}

		rerankInput = append(rerankInput, rerank.SearchResult{
			ID:       result.Content.ContentID,
			Content:  contentText,
			Score:    result.Score,
			Metadata: result.Content.Metadata,
		})
	}

	// Configure reranking options
	rerankOpts := &rerank.RerankOptions{
		TopK: options.Limit,
	}

	// Perform reranking
	reranked, err := s.reranker.Rerank(ctx, query, rerankInput, rerankOpts)
	if err != nil {
		s.logger.Error("Reranking failed", map[string]interface{}{
			"error": err.Error(),
			"query": query,
		})
		span.RecordError(err)
		// Return original results on error
		return results, nil
	}

	// Convert back to SearchResults
	rerankedResults := &SearchResults{
		Results: make([]*SearchResult, len(reranked)),
		Total:   len(reranked),
		HasMore: false,
	}

	for i, r := range reranked {
		// Find original result to preserve all metadata
		var originalResult *SearchResult
		for _, orig := range results.Results {
			if orig.Content != nil && orig.Content.ContentID == r.ID {
				originalResult = orig
				break
			}
		}

		if originalResult != nil {
			// Update score and metadata
			originalResult.Score = r.Score
			if r.Metadata != nil {
				for k, v := range r.Metadata {
					originalResult.Content.Metadata[k] = v
				}
			}
			rerankedResults.Results[i] = originalResult
		} else {
			// Create new result if original not found
			rerankedResults.Results[i] = &SearchResult{
				Content: &EmbeddingVector{
					ContentID: r.ID,
					Metadata:  r.Metadata,
				},
				Score: r.Score,
				Matches: map[string]interface{}{
					"reranked": true,
				},
			}
		}
	}

	span.SetAttribute("output_count", len(rerankedResults.Results))
	s.logger.Debug("Reranking completed", map[string]interface{}{
		"input_count":  len(results.Results),
		"output_count": len(rerankedResults.Results),
	})

	return rerankedResults, nil
}

// expandQuery expands the query using configured strategies
func (s *UnifiedSearchService) expandQuery(ctx context.Context, query string, options *SearchOptions) ([]string, error) {
	// Convert expansion types
	expansionTypes := make([]expansion.ExpansionType, 0, len(options.QueryExpansionTypes))
	for _, t := range options.QueryExpansionTypes {
		expansionTypes = append(expansionTypes, expansion.ExpansionType(t))
	}

	// Set default expansion types if none specified
	if len(expansionTypes) == 0 {
		expansionTypes = []expansion.ExpansionType{
			expansion.ExpansionTypeSynonym,
			expansion.ExpansionTypeDecompose,
		}
	}

	expansionOpts := &expansion.ExpansionOptions{
		MaxExpansions:   options.MaxExpansions,
		IncludeOriginal: true,
		ExpansionTypes:  expansionTypes,
	}

	expanded, err := s.queryExpander.Expand(ctx, query, expansionOpts)
	if err != nil {
		return nil, err
	}

	// Extract text from expansions
	queries := make([]string, 0, len(expanded.Expansions))
	for _, exp := range expanded.Expansions {
		queries = append(queries, exp.Text)
	}

	return queries, nil
}

// multiQuerySearch performs search with multiple queries and merges results
func (s *UnifiedSearchService) multiQuerySearch(ctx context.Context, queries []string, options *SearchOptions) (*SearchResults, error) {
	// Start span for tracing
	ctx, span := observability.StartSpan(ctx, "unified.search.multi_query")
	defer span.End()

	span.SetAttribute("query_count", len(queries))

	// Perform searches in parallel
	type searchResult struct {
		results *SearchResults
		err     error
		query   string
		weight  float32
	}

	resultChan := make(chan searchResult, len(queries))

	for i, query := range queries {
		weight := float32(1.0)
		if i > 0 {
			// Lower weight for expanded queries
			weight = 1.0 / float32(i+1)
		}

		go func(q string, w float32) {
			// Clone options to avoid race conditions
			queryOpts := *options
			// Disable expansion for individual queries
			queryOpts.UseQueryExpansion = false

			results, err := s.Search(ctx, q, &queryOpts)
			resultChan <- searchResult{
				results: results,
				err:     err,
				query:   q,
				weight:  w,
			}
		}(query, weight)
	}

	// Collect results
	allResults := make([]*SearchResult, 0)
	resultMap := make(map[string]*SearchResult)
	var firstError error

	for i := 0; i < len(queries); i++ {
		result := <-resultChan
		if result.err != nil {
			if firstError == nil {
				firstError = result.err
			}
			s.logger.Warn("Query failed in multi-query search", map[string]interface{}{
				"query": result.query,
				"error": result.err.Error(),
			})
			continue
		}

		// Merge results with weighting
		for _, r := range result.results.Results {
			if r.Content == nil {
				continue
			}

			key := r.Content.ContentID
			if existing, exists := resultMap[key]; exists {
				// Combine scores
				existing.Score = existing.Score + (r.Score * result.weight)
			} else {
				// Apply weight to score
				r.Score *= result.weight
				resultMap[key] = r
			}
		}
	}

	// If all queries failed, return error
	if len(resultMap) == 0 && firstError != nil {
		return nil, firstError
	}

	// Convert map to slice
	for _, r := range resultMap {
		allResults = append(allResults, r)
	}

	// Sort by score
	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Score > allResults[j].Score
	})

	// Apply limit
	if options.Limit > 0 && len(allResults) > options.Limit {
		allResults = allResults[:options.Limit]
	}

	return &SearchResults{
		Results: allResults,
		Total:   len(allResults),
		HasMore: false,
	}, nil
}

// embeddingServiceAdapter adapts EmbeddingService to hybrid.EmbeddingService
type embeddingServiceAdapter struct {
	service EmbeddingService
}

func (a *embeddingServiceAdapter) GenerateEmbedding(ctx context.Context, text, contentType, model string) (*hybrid.EmbeddingVector, error) {
	embedding, err := a.service.GenerateEmbedding(ctx, text, contentType, model)
	if err != nil {
		return nil, err
	}

	// Convert to hybrid.EmbeddingVector
	return &hybrid.EmbeddingVector{
		ContentID:   embedding.ContentID,
		ContentType: embedding.ContentType,
		Vector:      embedding.Vector,
		Metadata:    embedding.Metadata,
		ModelName:   embedding.ModelID, // Use ModelID as ModelName
		CreatedAt:   time.Now(),        // Set current time
	}, nil
}
