package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/embedding"
	"github.com/google/uuid"
)

// SearchHandler manages vector search API endpoints
type SearchHandler struct {
	searchService embedding.SearchService
}

// NewSearchHandler creates a new search handler
func NewSearchHandler(searchService embedding.SearchService) *SearchHandler {
	return &SearchHandler{
		searchService: searchService,
	}
}

// SearchRequest represents a vector search request
type SearchRequest struct {
	// Query text to search for
	Query string `json:"query"`
	// ContentTypes to filter by
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters to apply to metadata
	Filters []embedding.SearchFilter `json:"filters,omitempty"`
	// Sorting criteria
	Sorts []embedding.SearchSort `json:"sorts,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit,omitempty"`
	// Number of results to skip (for pagination)
	Offset int `json:"offset,omitempty"`
	// Minimum similarity threshold (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity,omitempty"`
	// Weight factors for scoring
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
	// UseReranking enables reranking of results
	UseReranking bool `json:"use_reranking,omitempty"`
	// RerankModel specifies which reranking model to use
	RerankModel string `json:"rerank_model,omitempty"`
	// UseQueryExpansion enables query expansion
	UseQueryExpansion bool `json:"use_query_expansion,omitempty"`
	// QueryExpansionTypes specifies which expansion types to use
	QueryExpansionTypes []string `json:"query_expansion_types,omitempty"`
	// MaxExpansions limits the number of query expansions
	MaxExpansions int `json:"max_expansions,omitempty"`
}

// SearchByVectorRequest represents a vector search request with a pre-computed vector
type SearchByVectorRequest struct {
	// Pre-computed vector to search with
	Vector []float32 `json:"vector"`
	// ContentTypes to filter by
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters to apply to metadata
	Filters []embedding.SearchFilter `json:"filters,omitempty"`
	// Sorting criteria
	Sorts []embedding.SearchSort `json:"sorts,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit,omitempty"`
	// Number of results to skip (for pagination)
	Offset int `json:"offset,omitempty"`
	// Minimum similarity threshold (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity,omitempty"`
	// Weight factors for scoring
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
	// UseReranking enables reranking of results
	UseReranking bool `json:"use_reranking,omitempty"`
	// RerankModel specifies which reranking model to use
	RerankModel string `json:"rerank_model,omitempty"`
	// RerankQuery allows overriding the query used for reranking (for vector search)
	RerankQuery string `json:"rerank_query,omitempty"`
}

// SearchResponse represents the API response for search endpoints
type SearchResponse struct {
	// Results is the list of search results
	Results []*embedding.SearchResult `json:"results"`
	// Total is the total number of results found (for pagination)
	Total int `json:"total"`
	// HasMore indicates if there are more results available
	HasMore bool `json:"has_more"`
	// Query information for debugging and auditing
	Query struct {
		// Text or ContentID that was searched for
		Input string `json:"input,omitempty"`
		// Options that were used for the search
		Options *embedding.SearchOptions `json:"options,omitempty"`
	} `json:"query"`
}

// RegisterRoutes registers the search endpoints with the provided router
func (h *SearchHandler) RegisterRoutes(router *http.ServeMux) {
	router.HandleFunc("/api/v1/search", h.HandleSearch)
	router.HandleFunc("/api/v1/search/vector", h.HandleSearchByVector)
	router.HandleFunc("/api/v1/search/similar", h.HandleSearchSimilar)
	router.HandleFunc("/api/v1/search/hybrid", h.HandleHybridSearch)
}

// HandleSearch handles text-based vector search requests
func (h *SearchHandler) HandleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var searchReq SearchRequest

	// Handle both GET and POST requests
	if r.Method == http.MethodPost {
		// Parse JSON request body
		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&searchReq); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}
	} else {
		// Parse query parameters
		q := r.URL.Query()

		searchReq.Query = q.Get("query")
		if types := q.Get("content_types"); types != "" {
			searchReq.ContentTypes = strings.Split(types, ",")
		}

		if limit := q.Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				searchReq.Limit = l
			}
		}

		if offset := q.Get("offset"); offset != "" {
			if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
				searchReq.Offset = o
			}
		}

		if minSim := q.Get("min_similarity"); minSim != "" {
			if ms, err := strconv.ParseFloat(minSim, 32); err == nil {
				searchReq.MinSimilarity = float32(ms)
			}
		}

		// Note: Complex parameters like filters and weight factors
		// are not supported in GET requests for simplicity
	}

	// Validate the request
	if searchReq.Query == "" {
		http.Error(w, "Query parameter is required", http.StatusBadRequest)
		return
	}

	// Prepare search options
	options := &embedding.SearchOptions{
		ContentTypes:        searchReq.ContentTypes,
		Filters:             searchReq.Filters,
		Sorts:               searchReq.Sorts,
		Limit:               searchReq.Limit,
		Offset:              searchReq.Offset,
		MinSimilarity:       searchReq.MinSimilarity,
		WeightFactors:       searchReq.WeightFactors,
		UseReranking:        searchReq.UseReranking,
		RerankModel:         searchReq.RerankModel,
		UseQueryExpansion:   searchReq.UseQueryExpansion,
		QueryExpansionTypes: searchReq.QueryExpansionTypes,
		MaxExpansions:       searchReq.MaxExpansions,
	}

	// Perform the search
	results, err := h.searchService.Search(r.Context(), searchReq.Query, options)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search error: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = searchReq.Query
	response.Query.Options = options

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleSearchByVector handles vector-based search requests
func (h *SearchHandler) HandleSearchByVector(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse JSON request body
	var searchReq SearchByVectorRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&searchReq); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate the request
	if len(searchReq.Vector) == 0 {
		http.Error(w, "Vector parameter is required", http.StatusBadRequest)
		return
	}

	// Prepare search options
	options := &embedding.SearchOptions{
		ContentTypes:  searchReq.ContentTypes,
		Filters:       searchReq.Filters,
		Sorts:         searchReq.Sorts,
		Limit:         searchReq.Limit,
		Offset:        searchReq.Offset,
		MinSimilarity: searchReq.MinSimilarity,
		WeightFactors: searchReq.WeightFactors,
		UseReranking:  searchReq.UseReranking,
		RerankModel:   searchReq.RerankModel,
		RerankQuery:   searchReq.RerankQuery, // For vector search, we need the query text for reranking
	}

	// Perform the search
	results, err := h.searchService.SearchByVector(r.Context(), searchReq.Vector, options)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search error: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = fmt.Sprintf("vector[%d]", len(searchReq.Vector))
	response.Query.Options = options

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// HandleSearchSimilar godoc
// @Summary Find similar content
// @Description Find content similar to a given content ID
// @Tags search
// @Accept json
// @Produce json
// @Param content_id query string true "Content ID to find similar items for"
// @Param limit query integer false "Maximum number of results"
// @Param offset query integer false "Pagination offset"
// @Param min_similarity query number false "Minimum similarity threshold (0.0-1.0)"
// @Success 200 {object} SearchResponse "Similar content results"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /search/similar [get]
func (h *SearchHandler) HandleSearchSimilar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var contentID string
	var options embedding.SearchOptions

	// Handle both GET and POST requests
	if r.Method == http.MethodPost {
		// Parse JSON request body
		var requestBody struct {
			ContentID string                   `json:"content_id"`
			Options   *embedding.SearchOptions `json:"options,omitempty"`
		}

		decoder := json.NewDecoder(r.Body)
		if err := decoder.Decode(&requestBody); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		contentID = requestBody.ContentID
		if requestBody.Options != nil {
			options = *requestBody.Options
		}
	} else {
		// Parse query parameters
		q := r.URL.Query()

		contentID = q.Get("content_id")

		if types := q.Get("content_types"); types != "" {
			options.ContentTypes = strings.Split(types, ",")
		}

		if limit := q.Get("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				options.Limit = l
			}
		}

		if offset := q.Get("offset"); offset != "" {
			if o, err := strconv.Atoi(offset); err == nil && o >= 0 {
				options.Offset = o
			}
		}

		if minSim := q.Get("min_similarity"); minSim != "" {
			if ms, err := strconv.ParseFloat(minSim, 32); err == nil {
				options.MinSimilarity = float32(ms)
			}
		}
	}

	// Validate the request
	if contentID == "" {
		http.Error(w, "content_id parameter is required", http.StatusBadRequest)
		return
	}

	// Perform the search
	results, err := h.searchService.SearchByContentID(r.Context(), contentID, &options)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search error: %v", err), http.StatusInternalServerError)
		return
	}

	// Prepare the response
	response := SearchResponse{
		Results: results.Results,
		Total:   results.Total,
		HasMore: results.HasMore,
	}
	response.Query.Input = contentID
	response.Query.Options = &options

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// HybridSearchRequest represents a hybrid search request
type HybridSearchRequest struct {
	// Query text for semantic search
	Query string `json:"query"`
	// Keywords for keyword-based search
	Keywords []string `json:"keywords,omitempty"`
	// Balance between semantic and keyword results (0.0 to 1.0)
	HybridWeight float32 `json:"hybrid_weight,omitempty"`
	// ContentTypes to filter by
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters to apply to metadata
	Filters []embedding.SearchFilter `json:"filters,omitempty"`
	// Maximum number of results to return
	Limit int `json:"limit,omitempty"`
	// Number of results to skip (for pagination)
	Offset int `json:"offset,omitempty"`
	// Minimum similarity threshold (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity,omitempty"`
}

// HandleHybridSearch godoc
// @Summary Perform hybrid search
// @Description Perform hybrid search combining vector and keyword search
// @Tags search
// @Accept json
// @Produce json
// @Param request body HybridSearchRequest true "Hybrid search request"
// @Success 200 {object} SearchResponse "Search results"
// @Failure 400 {object} map[string]interface{} "Invalid request"
// @Failure 401 {object} map[string]interface{} "Unauthorized"
// @Failure 500 {object} map[string]interface{} "Internal server error"
// @Security ApiKeyAuth
// @Security BearerAuth
// @Router /search/hybrid [post]
func (h *SearchHandler) HandleHybridSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse request body
	var req HybridSearchRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate request
	if req.Query == "" && len(req.Keywords) == 0 {
		http.Error(w, "Either query or keywords must be provided", http.StatusBadRequest)
		return
	}

	// Set defaults
	if req.HybridWeight == 0 {
		req.HybridWeight = 0.7 // Default to 70% semantic, 30% keyword
	}
	if req.Limit == 0 {
		req.Limit = 10
	}
	if req.MinSimilarity == 0 {
		req.MinSimilarity = 0.5
	}

	// Build HybridSearchRequest for the service
	hybridReq := embedding.HybridSearchRequest{
		Query:         req.Query,
		Keywords:      req.Keywords,
		HybridWeight:  req.HybridWeight,
		TenantID:      getTenantIDFromContext(r.Context()),
		Limit:         req.Limit,
		MinSimilarity: req.MinSimilarity,
	}

	// Perform hybrid search
	searchService, ok := h.searchService.(*embedding.UnifiedSearchService)
	if !ok {
		http.Error(w, "Hybrid search not supported", http.StatusNotImplemented)
		return
	}

	results, err := searchService.HybridSearch(r.Context(), hybridReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("Search error: %v", err), http.StatusInternalServerError)
		return
	}

	// Convert results to SearchResponse format
	searchResults := make([]*embedding.SearchResult, len(results))
	for i, result := range results {
		searchResults[i] = &embedding.SearchResult{
			Content: &embedding.EmbeddingVector{
				ContentID:   result.Content,
				ContentType: "text",
				Metadata:    result.Metadata,
			},
			Score: result.HybridScore,
			Matches: map[string]interface{}{
				"vector_score":  result.SemanticScore,
				"keyword_score": result.KeywordScore,
				"hybrid_score":  result.HybridScore,
			},
		}
	}

	// Prepare the response
	response := SearchResponse{
		Results: searchResults,
		Total:   len(searchResults),
		HasMore: false, // Could be enhanced with proper pagination
	}
	response.Query.Input = req.Query
	response.Query.Options = &embedding.SearchOptions{
		ContentTypes:  req.ContentTypes,
		Filters:       req.Filters,
		Limit:         req.Limit,
		Offset:        req.Offset,
		MinSimilarity: req.MinSimilarity,
	}

	// Send the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// getTenantIDFromContext extracts tenant ID from request context
func getTenantIDFromContext(ctx context.Context) uuid.UUID {
	// This should be implemented based on your auth middleware
	// For now, returning a placeholder
	return uuid.New()
}
