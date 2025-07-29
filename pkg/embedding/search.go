package embedding

import (
	"context"
	"time"

	"github.com/google/uuid"
)

// SearchFilter defines a filter for metadata fields
type SearchFilter struct {
	// Field is the metadata field to filter on
	Field string `json:"field"`
	// Value is the value to match
	Value interface{} `json:"value"`
	// Operator is the comparison operator (eq, ne, gt, lt, gte, lte, in, contains)
	Operator string `json:"operator"`
}

// SearchSort defines a sort order for results
type SearchSort struct {
	// Field is the field to sort on (can be "similarity" or any metadata field)
	Field string `json:"field"`
	// Direction is the sort direction ("asc" or "desc")
	Direction string `json:"direction"`
}

// SearchOptions contains options for search queries
type SearchOptions struct {
	// ContentTypes filters results to specific content types
	ContentTypes []string `json:"content_types,omitempty"`
	// Filters are metadata filters to apply to the search
	Filters []SearchFilter `json:"filters,omitempty"`
	// Sorts defines the sort order for results
	Sorts []SearchSort `json:"sorts,omitempty"`
	// Limit is the maximum number of results to return
	Limit int `json:"limit"`
	// Offset is the number of results to skip (for pagination)
	Offset int `json:"offset"`
	// MinSimilarity is the minimum similarity score required (0.0 to 1.0)
	MinSimilarity float32 `json:"min_similarity"`
	// WeightFactors defines how to weight different scoring factors
	WeightFactors map[string]float32 `json:"weight_factors,omitempty"`
	// UseReranking enables reranking of results
	UseReranking bool `json:"use_reranking,omitempty"`
	// RerankModel specifies which reranking model to use
	RerankModel string `json:"rerank_model,omitempty"`
	// RerankQuery allows overriding the query used for reranking
	RerankQuery string `json:"rerank_query,omitempty"`
	// UseQueryExpansion enables query expansion
	UseQueryExpansion bool `json:"use_query_expansion,omitempty"`
	// QueryExpansionTypes specifies which expansion types to use
	QueryExpansionTypes []string `json:"query_expansion_types,omitempty"`
	// MaxExpansions limits the number of query expansions
	MaxExpansions int `json:"max_expansions,omitempty"`
}

// SearchResult represents a single search result
type SearchResult struct {
	// Content is the embedding that matched
	Content *EmbeddingVector `json:"content"`
	// Score is the calculated relevance score (0.0 to 1.0)
	Score float32 `json:"score"`
	// Matches contains information about why this result matched
	Matches map[string]interface{} `json:"matches,omitempty"`
}

// SearchResults represents a collection of search results
type SearchResults struct {
	// Results is the list of search results
	Results []*SearchResult `json:"results"`
	// Total is the total number of results found (for pagination)
	Total int `json:"total"`
	// HasMore indicates if there are more results available
	HasMore bool `json:"has_more"`
}

// SearchService defines the interface for vector search operations
type SearchService interface {
	// Search performs a vector search with the given text
	Search(ctx context.Context, text string, options *SearchOptions) (*SearchResults, error)

	// SearchByVector performs a vector search with a pre-computed vector
	SearchByVector(ctx context.Context, vector []float32, options *SearchOptions) (*SearchResults, error)

	// SearchByContentID performs a "more like this" search based on an existing content ID
	SearchByContentID(ctx context.Context, contentID string, options *SearchOptions) (*SearchResults, error)
}

// CrossModelSearchRequest defines parameters for cross-model search
type CrossModelSearchRequest struct {
	// Query is the search query text
	Query string `json:"query"`
	// QueryEmbedding is the pre-computed query embedding (optional)
	QueryEmbedding []float32 `json:"query_embedding,omitempty"`
	// SearchModel is the model to use for generating query embeddings
	SearchModel string `json:"search_model"`
	// IncludeModels limits results to specific models (empty means all)
	IncludeModels []string `json:"include_models,omitempty"`
	// ExcludeModels excludes results from specific models
	ExcludeModels []string `json:"exclude_models,omitempty"`
	// TenantID is the tenant to search within
	TenantID uuid.UUID `json:"tenant_id"`
	// ContextID optionally limits search to a specific context
	ContextID *uuid.UUID `json:"context_id,omitempty"`
	// Limit is the maximum number of results to return
	Limit int `json:"limit"`
	// MinSimilarity is the minimum similarity threshold
	MinSimilarity float32 `json:"min_similarity"`
	// MetadataFilter is a JSONB filter for metadata
	MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
	// TaskType optionally specifies the type of task for scoring
	TaskType string `json:"task_type,omitempty"`
	// Options for additional search parameters
	Options *SearchOptions `json:"options,omitempty"`
}

// CrossModelSearchResult represents a result from cross-model search
type CrossModelSearchResult struct {
	// ID is the embedding ID
	ID uuid.UUID `json:"id"`
	// ContextID is the context this embedding belongs to
	ContextID *uuid.UUID `json:"context_id,omitempty"`
	// Content is the text content
	Content string `json:"content"`
	// OriginalModel is the model that created this embedding
	OriginalModel string `json:"original_model"`
	// OriginalDimension is the original embedding dimension
	OriginalDimension int `json:"original_dimension"`
	// Similarity is the normalized similarity score
	Similarity float32 `json:"similarity"`
	// RawSimilarity is the raw similarity score before normalization
	RawSimilarity float32 `json:"raw_similarity"`
	// AgentID is the agent that created this content
	AgentID string `json:"agent_id,omitempty"`
	// Metadata contains additional information
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// CreatedAt is when the embedding was created
	CreatedAt time.Time `json:"created_at"`
	// ModelQualityScore is the quality score for this model
	ModelQualityScore float32 `json:"model_quality_score"`
	// FinalScore is the final weighted score
	FinalScore float32 `json:"final_score"`
}

// HybridSearchRequest defines parameters for hybrid search
type HybridSearchRequest struct {
	// Query is the main search query for semantic search
	Query string `json:"query"`
	// Keywords are additional keywords for keyword-based search
	Keywords []string `json:"keywords,omitempty"`
	// HybridWeight determines the balance between semantic and keyword results (0.0 to 1.0)
	HybridWeight float32 `json:"hybrid_weight"`
	// TenantID is the tenant to search within
	TenantID uuid.UUID `json:"tenant_id"`
	// ModelName is the embedding model to use
	ModelName string `json:"model_name"`
	// Limit is the maximum number of results
	Limit int `json:"limit"`
	// MinSimilarity is the minimum similarity threshold
	MinSimilarity float32 `json:"min_similarity"`
	// MetadataFilter is a JSONB filter for metadata
	MetadataFilter map[string]interface{} `json:"metadata_filter,omitempty"`
	// Options for additional search parameters
	Options *SearchOptions `json:"options,omitempty"`
	// QueryEmbedding allows pre-computed embedding to be passed
	QueryEmbedding []float32 `json:"query_embedding,omitempty"`
}

// HybridSearchResult represents a result from hybrid search
type HybridSearchResult struct {
	// Embed the cross-model search result
	CrossModelSearchResult
	// Result is the combined search result
	Result *SearchResult `json:"result"`
	// SemanticScore is the semantic similarity score
	SemanticScore float32 `json:"semantic_score"`
	// KeywordScore is the keyword relevance score
	KeywordScore float32 `json:"keyword_score"`
	// HybridScore is the combined score
	HybridScore float32 `json:"hybrid_score"`
}

// AdvancedSearchService extends SearchService with cross-model and hybrid search capabilities
type AdvancedSearchService interface {
	SearchService

	// CrossModelSearch performs search across embeddings from different models
	CrossModelSearch(ctx context.Context, req CrossModelSearchRequest) ([]CrossModelSearchResult, error)

	// HybridSearch performs hybrid search combining semantic and keyword search
	HybridSearch(ctx context.Context, req HybridSearchRequest) ([]HybridSearchResult, error)
}
