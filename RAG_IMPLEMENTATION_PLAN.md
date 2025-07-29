# RAG Enhancement Implementation Plan

## Overview
This document provides a detailed implementation plan for enhancing the RAG (Retrieval-Augmented Generation) capabilities of Developer Mesh. The plan addresses five critical gaps identified in the current implementation.

## Current State Assessment
- ✅ **Implemented**: Multi-provider embeddings, pgvector storage, code chunking, agent-based routing
- ❌ **Missing**: Hybrid search, reranking, query expansion, advanced text chunking, semantic caching

## Design Principles
1. **Reuse existing packages**: Leverage existing observability, auth, database, and security packages
2. **Security first**: Input validation, parameterized queries, tenant isolation
3. **Observable**: Comprehensive metrics, tracing, and logging using existing infrastructure
4. **Resilient**: Circuit breakers, retries, timeouts, graceful degradation
5. **Testable**: Interfaces for mocking, comprehensive test coverage
6. **Performance**: Concurrent operations, caching, connection pooling

## Package Dependencies and Integration

### Existing Packages to Enhance
- `pkg/embedding`: Add hybrid search, reranking, and expansion capabilities
- `pkg/chunking`: Extend with semantic text chunking strategies
- `pkg/database`: Use existing connection pooling and error handling
- `pkg/observability`: Leverage existing logging, metrics, and tracing
- `pkg/auth`: Use for tenant isolation and security
- `pkg/redis`: Extend for semantic caching
- `pkg/circuitbreaker`: Use for external API calls (reranking)
- `pkg/retry`: Apply to all external service calls

### New Sub-packages
- `pkg/embedding/hybrid`: Hybrid search implementation
- `pkg/embedding/rerank`: Reranking strategies
- `pkg/embedding/expansion`: Query expansion
- `pkg/embedding/cache`: Semantic caching
- `pkg/chunking/text`: Advanced text chunking

## Implementation Phases

### Phase 1: Hybrid Search (Weeks 1-3)

#### 1.1 Database Schema Changes
```sql
-- File: apps/rest-api/migrations/sql/000006_hybrid_search.up.sql

BEGIN;

-- Add full-text search columns
ALTER TABLE embeddings 
    ADD COLUMN IF NOT EXISTS content_tsvector tsvector,
    ADD COLUMN IF NOT EXISTS term_frequencies jsonb,
    ADD COLUMN IF NOT EXISTS document_length integer,
    ADD COLUMN IF NOT EXISTS idf_scores jsonb;

-- Create GIN indexes for full-text search
CREATE INDEX IF NOT EXISTS idx_embeddings_fts ON embeddings USING gin(content_tsvector);
CREATE INDEX IF NOT EXISTS idx_embeddings_trigram ON embeddings USING gin(content gin_trgm_ops);

-- Create function to update tsvector
CREATE OR REPLACE FUNCTION update_content_tsvector() RETURNS trigger AS $$
BEGIN
    NEW.content_tsvector := to_tsvector('english', NEW.content);
    NEW.document_length := array_length(string_to_array(NEW.content, ' '), 1);
    RETURN NEW;
END
$$ LANGUAGE plpgsql;

-- Create trigger
CREATE TRIGGER embeddings_tsvector_update BEFORE INSERT OR UPDATE ON embeddings
    FOR EACH ROW EXECUTE FUNCTION update_content_tsvector();

-- Create BM25 scoring function
CREATE OR REPLACE FUNCTION bm25_score(
    query_terms text[],
    doc_tsvector tsvector,
    doc_length integer,
    avg_doc_length float,
    total_docs integer,
    k1 float DEFAULT 1.2,
    b float DEFAULT 0.75
) RETURNS float AS $$
DECLARE
    score float := 0;
    term text;
    tf integer;
    df integer;
    idf float;
BEGIN
    FOREACH term IN ARRAY query_terms
    LOOP
        -- Get term frequency
        tf := ts_rank_cd(doc_tsvector, plainto_tsquery(term))::integer;
        
        -- Get document frequency (simplified - in production, use cached values)
        SELECT COUNT(*) INTO df FROM embeddings WHERE content_tsvector @@ plainto_tsquery(term);
        
        -- Calculate IDF
        idf := ln((total_docs - df + 0.5) / (df + 0.5));
        
        -- Calculate BM25 component
        score := score + (idf * tf * (k1 + 1)) / (tf + k1 * (1 - b + b * (doc_length / avg_doc_length)));
    END LOOP;
    
    RETURN score;
END
$$ LANGUAGE plpgsql;

COMMIT;
```

#### 1.2 Core Implementation Files

##### File: `pkg/embedding/hybrid/search.go`
```go
package hybrid

import (
    "context"
    "database/sql"
    "fmt"
    "sort"
    "strings"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/lib/pq"
    "github.com/google/uuid"
)

// HybridSearchService combines vector and keyword search
type HybridSearchService struct {
    db               *sql.DB
    vectorSearch     embedding.SearchService
    fusionAlgorithm  FusionAlgorithm
    logger           observability.Logger
    metrics          observability.MetricsClient
    config           *Config
}

// Config for hybrid search
type Config struct {
    VectorWeight    float64 // Weight for vector search (0-1)
    KeywordWeight   float64 // Weight for keyword search (0-1)
    RRFConstant     float64 // Reciprocal Rank Fusion constant (default: 60)
    MinScore        float64 // Minimum score threshold
    EnableBM25      bool    // Use BM25 instead of TF-IDF
}

// SearchOptions extends base search options
type SearchOptions struct {
    *embedding.SearchOptions
    UseHybrid       bool
    KeywordBoost    float64
    PhraseSearch    bool
}

// NewHybridSearchService creates a new hybrid search service
func NewHybridSearchService(
    db *sql.DB,
    vectorSearch embedding.SearchService,
    logger observability.Logger,
    config *Config,
) (*HybridSearchService, error) {
    if db == nil {
        return nil, fmt.Errorf("database connection is required")
    }
    if vectorSearch == nil {
        return nil, fmt.Errorf("vector search service is required")
    }
    
    // Set defaults
    if config.RRFConstant == 0 {
        config.RRFConstant = 60
    }
    if config.VectorWeight == 0 && config.KeywordWeight == 0 {
        config.VectorWeight = 0.7
        config.KeywordWeight = 0.3
    }
    
    // Normalize weights
    totalWeight := config.VectorWeight + config.KeywordWeight
    config.VectorWeight = config.VectorWeight / totalWeight
    config.KeywordWeight = config.KeywordWeight / totalWeight
    
    return &HybridSearchService{
        db:               db,
        vectorSearch:     vectorSearch,
        fusionAlgorithm:  NewReciprocalRankFusion(config.RRFConstant),
        logger:           logger,
        metrics:          observability.NewMetricsClient(),
        config:           config,
    }, nil
}

// Search performs hybrid search with proper error handling and observability
func (h *HybridSearchService) Search(ctx context.Context, query string, opts *SearchOptions) (*embedding.SearchResults, error) {
    // Start span for tracing
    ctx, span := observability.StartSpan(ctx, "hybrid.search")
    defer span.End()
    
    span.SetAttribute("query", query)
    span.SetAttribute("use_hybrid", opts.UseHybrid)
    
    // Input validation
    if strings.TrimSpace(query) == "" {
        return nil, fmt.Errorf("query cannot be empty")
    }
    
    // Extract tenant ID from context
    tenantID := auth.GetTenantID(ctx)
    if tenantID == uuid.Nil {
        return nil, fmt.Errorf("tenant ID not found in context")
    }
    
    // Track metrics
    start := time.Now()
    defer func() {
        h.metrics.RecordDuration("search.hybrid.duration", time.Since(start), 
            map[string]string{"tenant": tenantID.String()})
    }()
    
    // Parallel execution with timeout
    searchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    
    type searchResult struct {
        results []embedding.SearchResult
        err     error
        source  string
    }
    
    vectorCh := make(chan searchResult, 1)
    keywordCh := make(chan searchResult, 1)
    
    // Vector search
    go func() {
        defer func() {
            if r := recover(); r != nil {
                vectorCh <- searchResult{err: fmt.Errorf("vector search panic: %v", r), source: "vector"}
            }
        }()
        
        results, err := h.vectorSearch.Search(searchCtx, query, opts.SearchOptions)
        if err != nil {
            vectorCh <- searchResult{err: err, source: "vector"}
            return
        }
        vectorCh <- searchResult{results: results.Results, source: "vector"}
    }()
    
    // Keyword search
    go func() {
        defer func() {
            if r := recover(); r != nil {
                keywordCh <- searchResult{err: fmt.Errorf("keyword search panic: %v", r), source: "keyword"}
            }
        }()
        
        results, err := h.keywordSearch(searchCtx, query, opts, tenantID)
        keywordCh <- searchResult{results: results, err: err, source: "keyword"}
    }()
    
    // Collect results with timeout handling
    var vectorRes, keywordRes searchResult
    for i := 0; i < 2; i++ {
        select {
        case res := <-vectorCh:
            vectorRes = res
        case res := <-keywordCh:
            keywordRes = res
        case <-searchCtx.Done():
            return nil, fmt.Errorf("search timeout exceeded")
        }
    }
    
    // Handle errors with graceful degradation
    if vectorRes.err != nil && keywordRes.err != nil {
        return nil, fmt.Errorf("both searches failed: vector=%w, keyword=%v", vectorRes.err, keywordRes.err)
    }
    
    if vectorRes.err != nil {
        h.logger.Warn("Vector search failed, using keyword results only", 
            map[string]interface{}{
                "error": vectorRes.err.Error(),
                "query": query,
            })
        h.metrics.Increment("search.hybrid.vector_failure", map[string]string{"tenant": tenantID.String()})
        return &embedding.SearchResults{Results: keywordRes.results}, nil
    }
    
    if keywordRes.err != nil {
        h.logger.Warn("Keyword search failed, using vector results only", 
            map[string]interface{}{
                "error": keywordRes.err.Error(),
                "query": query,
            })
        h.metrics.Increment("search.hybrid.keyword_failure", map[string]string{"tenant": tenantID.String()})
        return &embedding.SearchResults{Results: vectorRes.results}, nil
    }
    
    // Fuse results
    fusedResults := h.fusionAlgorithm.Fuse(
        vectorRes.results,
        keywordRes.results,
        h.config.VectorWeight,
        h.config.KeywordWeight,
    )
    
    h.metrics.RecordValue("search.hybrid.results_count", float64(len(fusedResults)),
        map[string]string{"tenant": tenantID.String()})
    
    return &embedding.SearchResults{
        Results: fusedResults,
        Total:   len(fusedResults),
        HasMore: false,
    }, nil
}

// keywordSearch performs BM25-based keyword search with security measures
func (h *HybridSearchService) keywordSearch(ctx context.Context, query string, opts *SearchOptions, tenantID uuid.UUID) ([]embedding.SearchResult, error) {
    // Start span for tracing
    ctx, span := observability.StartSpan(ctx, "hybrid.keyword_search")
    defer span.End()
    
    // Prepare and sanitize query
    queryTerms := h.preprocessQuery(query)
    if len(queryTerms) == 0 {
        return []embedding.SearchResult{}, nil
    }
    
    // Validate query terms to prevent injection
    for _, term := range queryTerms {
        if err := h.validateSearchTerm(term); err != nil {
            return nil, fmt.Errorf("invalid search term: %w", err)
        }
    }
    
    // Build parameterized SQL query - using PostgreSQL parameterized queries for security
    sqlQuery := `
        WITH doc_stats AS (
            SELECT 
                AVG(document_length)::float as avg_length,
                COUNT(*)::integer as total_docs
            FROM embeddings
            WHERE tenant_id = $1
                AND deleted_at IS NULL
        ),
        scored_docs AS (
            SELECT 
                e.id,
                e.content,
                e.metadata,
                e.content_hash,
                e.created_at,
                bm25_score(
                    $2::text[],
                    e.content_tsvector,
                    e.document_length,
                    ds.avg_length,
                    ds.total_docs,
                    $3::float,  -- k1 parameter
                    $4::float   -- b parameter
                ) as score
            FROM embeddings e, doc_stats ds
            WHERE e.tenant_id = $1
                AND e.content_tsvector @@ plainto_tsquery('english', $5)
                AND e.deleted_at IS NULL
        )
        SELECT id, content, metadata, content_hash, created_at, score
        FROM scored_docs
        WHERE score > $6
        ORDER BY score DESC
        LIMIT $7
        OFFSET $8
    `
    
    // Set BM25 parameters
    k1 := float64(1.2)
    b := float64(0.75)
    if h.config.EnableBM25 {
        // Could be configurable
        k1 = 1.5
        b = 0.8
    }
    
    // Execute query with proper error handling
    rows, err := h.db.QueryContext(ctx, sqlQuery, 
        tenantID,                           // $1
        pq.Array(queryTerms),               // $2 - using pq.Array for proper array handling
        k1,                                 // $3
        b,                                  // $4
        strings.Join(queryTerms, " "),      // $5
        h.config.MinScore,                  // $6
        opts.Limit,                         // $7
        opts.Offset,                        // $8
    )
    if err != nil {
        span.RecordError(err)
        return nil, fmt.Errorf("keyword search query failed: %w", err)
    }
    defer rows.Close()
    
    results := []embedding.SearchResult{}
    for rows.Next() {
        var result embedding.SearchResult
        var contentHash sql.NullString
        var createdAt time.Time
        
        err := rows.Scan(
            &result.ID, 
            &result.Content, 
            &result.Metadata, 
            &contentHash,
            &createdAt,
            &result.Score,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to scan result: %w", err)
        }
        
        // Add additional metadata
        if result.Metadata == nil {
            result.Metadata = make(map[string]interface{})
        }
        result.Metadata["search_type"] = "keyword"
        result.Metadata["created_at"] = createdAt
        if contentHash.Valid {
            result.Metadata["content_hash"] = contentHash.String
        }
        
        results = append(results, result)
    }
    
    if err := rows.Err(); err != nil {
        return nil, fmt.Errorf("error iterating results: %w", err)
    }
    
    span.SetAttribute("results_count", len(results))
    return results, nil
}

// preprocessQuery tokenizes and normalizes the search query
func (h *HybridSearchService) preprocessQuery(query string) []string {
    // Convert to lowercase
    query = strings.ToLower(query)
    
    // Remove special characters but keep alphanumeric and spaces
    query = strings.Map(func(r rune) rune {
        if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == ' ' || r == '-' || r == '_' {
            return r
        }
        return ' '
    }, query)
    
    // Split into terms
    terms := strings.Fields(query)
    
    // Remove duplicates and empty terms
    seen := make(map[string]bool)
    unique := []string{}
    for _, term := range terms {
        term = strings.TrimSpace(term)
        if term != "" && !seen[term] && len(term) > 1 { // Skip single character terms
            seen[term] = true
            unique = append(unique, term)
        }
    }
    
    return unique
}

// validateSearchTerm ensures search terms are safe
func (h *HybridSearchService) validateSearchTerm(term string) error {
    // Maximum term length
    if len(term) > 100 {
        return fmt.Errorf("search term too long: %d characters", len(term))
    }
    
    // Check for SQL injection patterns
    dangerousPatterns := []string{
        "--", "/*", "*/", "xp_", "sp_", "';", "\"",
    }
    
    lowerTerm := strings.ToLower(term)
    for _, pattern := range dangerousPatterns {
        if strings.Contains(lowerTerm, pattern) {
            return fmt.Errorf("potentially dangerous pattern detected: %s", pattern)
        }
    }
    
    return nil
}
```

##### File: `pkg/embedding/hybrid/fusion.go`
```go
package hybrid

import (
    "sort"
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
)

// FusionAlgorithm combines vector and keyword search results
type FusionAlgorithm interface {
    Fuse(vectorResults, keywordResults []embedding.SearchResult, vectorWeight, keywordWeight float64) []embedding.SearchResult
}

// ReciprocalRankFusion implements RRF algorithm
type ReciprocalRankFusion struct {
    k float64 // Constant (typically 60)
}

func NewReciprocalRankFusion(k float64) *ReciprocalRankFusion {
    if k <= 0 {
        k = 60
    }
    return &ReciprocalRankFusion{k: k}
}

func (r *ReciprocalRankFusion) Fuse(vectorResults, keywordResults []embedding.SearchResult, vectorWeight, keywordWeight float64) []embedding.SearchResult {
    scores := make(map[string]*fusionScore)
    
    // Score vector results
    for rank, result := range vectorResults {
        if _, exists := scores[result.ID]; !exists {
            scores[result.ID] = &fusionScore{result: result}
        }
        scores[result.ID].vectorRRF = vectorWeight / (r.k + float64(rank+1))
        scores[result.ID].vectorScore = result.Score
    }
    
    // Score keyword results
    for rank, result := range keywordResults {
        if _, exists := scores[result.ID]; !exists {
            scores[result.ID] = &fusionScore{result: result}
        }
        scores[result.ID].keywordRRF = keywordWeight / (r.k + float64(rank+1))
        scores[result.ID].keywordScore = result.Score
    }
    
    // Calculate final scores
    fusedResults := make([]embedding.SearchResult, 0, len(scores))
    for _, fs := range scores {
        fs.result.Score = float32(fs.vectorRRF + fs.keywordRRF)
        // Store component scores in metadata
        if fs.result.Metadata == nil {
            fs.result.Metadata = make(map[string]interface{})
        }
        fs.result.Metadata["vector_score"] = fs.vectorScore
        fs.result.Metadata["keyword_score"] = fs.keywordScore
        fusedResults = append(fusedResults, fs.result)
    }
    
    // Sort by fused score
    sort.Slice(fusedResults, func(i, j int) bool {
        return fusedResults[i].Score > fusedResults[j].Score
    })
    
    return fusedResults
}

type fusionScore struct {
    result       embedding.SearchResult
    vectorRRF    float64
    keywordRRF   float64
    vectorScore  float32
    keywordScore float32
}

// LinearCombination implements weighted linear combination
type LinearCombination struct{}

func (l *LinearCombination) Fuse(vectorResults, keywordResults []embedding.SearchResult, vectorWeight, keywordWeight float64) []embedding.SearchResult {
    scores := make(map[string]*embedding.SearchResult)
    
    // Normalize weights
    totalWeight := vectorWeight + keywordWeight
    vectorWeight = vectorWeight / totalWeight
    keywordWeight = keywordWeight / totalWeight
    
    // Process vector results
    for _, result := range vectorResults {
        result.Score = float32(float64(result.Score) * vectorWeight)
        scores[result.ID] = &result
    }
    
    // Process keyword results
    for _, result := range keywordResults {
        if existing, exists := scores[result.ID]; exists {
            existing.Score += float32(float64(result.Score) * keywordWeight)
        } else {
            result.Score = float32(float64(result.Score) * keywordWeight)
            scores[result.ID] = &result
        }
    }
    
    // Convert to slice and sort
    results := make([]embedding.SearchResult, 0, len(scores))
    for _, result := range scores {
        results = append(results, *result)
    }
    
    sort.Slice(results, func(i, j int) bool {
        return results[i].Score > results[j].Score
    })
    
    return results
}
```

### Phase 2: Reranking (Weeks 4-5)

#### 2.1 Reranker Interface and Implementations

##### File: `pkg/embedding/rerank/reranker.go`
```go
package rerank

import (
    "context"
    "fmt"
    "sync"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/circuitbreaker"
    "github.com/developer-mesh/developer-mesh/pkg/retry"
    "golang.org/x/sync/semaphore"
)

// Reranker re-scores and reorders search results
type Reranker interface {
    Rerank(ctx context.Context, query string, results []embedding.SearchResult, opts *RerankOptions) ([]embedding.SearchResult, error)
    GetName() string
    Close() error
}

// RerankOptions configures reranking behavior
type RerankOptions struct {
    TopK            int
    Model           string
    IncludeScores   bool
    DiversityFactor float64 // For MMR
    MaxConcurrency  int
}

// MultiStageReranker applies multiple rerankers in sequence
type MultiStageReranker struct {
    stages  []RerankStage
    logger  observability.Logger
    metrics observability.MetricsClient
}

type RerankStage struct {
    Reranker Reranker
    TopK     int // How many to pass to next stage
    Weight   float64
}

func NewMultiStageReranker(stages []RerankStage, logger observability.Logger) *MultiStageReranker {
    return &MultiStageReranker{
        stages:  stages,
        logger:  logger,
        metrics: observability.NewMetricsClient(),
    }
}

func (m *MultiStageReranker) Rerank(ctx context.Context, query string, results []embedding.SearchResult, opts *RerankOptions) ([]embedding.SearchResult, error) {
    start := time.Now()
    defer func() {
        m.metrics.RecordDuration("rerank.multistage.duration", time.Since(start), nil)
    }()
    
    currentResults := results
    
    for i, stage := range m.stages {
        stageStart := time.Now()
        
        // Apply reranker
        reranked, err := stage.Reranker.Rerank(ctx, query, currentResults, &RerankOptions{
            TopK:  stage.TopK,
            Model: opts.Model,
        })
        if err != nil {
            m.logger.Error("Rerank stage failed", map[string]interface{}{
                "stage": i,
                "reranker": stage.Reranker.GetName(),
                "error": err.Error(),
            })
            return nil, fmt.Errorf("rerank stage %d failed: %w", i, err)
        }
        
        // Track metrics
        m.metrics.RecordDuration(
            fmt.Sprintf("rerank.stage.%s.duration", stage.Reranker.GetName()),
            time.Since(stageStart),
            map[string]string{"stage": fmt.Sprintf("%d", i)},
        )
        
        currentResults = reranked
    }
    
    return currentResults, nil
}

func (m *MultiStageReranker) GetName() string {
    return "multistage"
}
```

##### File: `pkg/embedding/rerank/cross_encoder.go`
```go
package rerank

import (
    "context"
    "fmt"
    "sort"
    "sync"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/providers"
    "github.com/developer-mesh/developer-mesh/pkg/circuitbreaker"
    "github.com/developer-mesh/developer-mesh/pkg/retry"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "golang.org/x/sync/semaphore"
)

// CrossEncoderReranker uses a cross-encoder model for reranking with resilience patterns
type CrossEncoderReranker struct {
    provider    providers.RerankProvider
    config      *CrossEncoderConfig
    breaker     *circuitbreaker.CircuitBreaker
    retryPolicy retry.Policy
    semaphore   *semaphore.Weighted
    logger      observability.Logger
    metrics     observability.MetricsClient
    mu          sync.RWMutex
}

type CrossEncoderConfig struct {
    Model               string
    BatchSize           int
    MaxConcurrency      int
    TimeoutPerBatch     time.Duration
    CircuitBreakerConfig *circuitbreaker.Config
}

func NewCrossEncoderReranker(
    provider providers.RerankProvider, 
    config *CrossEncoderConfig,
    logger observability.Logger,
) (*CrossEncoderReranker, error) {
    if provider == nil {
        return nil, fmt.Errorf("provider is required")
    }
    
    // Set defaults
    if config.BatchSize <= 0 {
        config.BatchSize = 10
    }
    if config.MaxConcurrency <= 0 {
        config.MaxConcurrency = 3
    }
    if config.TimeoutPerBatch == 0 {
        config.TimeoutPerBatch = 5 * time.Second
    }
    
    // Create circuit breaker with defaults if not provided
    breakerConfig := config.CircuitBreakerConfig
    if breakerConfig == nil {
        breakerConfig = &circuitbreaker.Config{
            RequestThreshold:   10,
            ErrorThreshold:     0.5,
            Timeout:            10 * time.Second,
            ResetTimeout:       30 * time.Second,
        }
    }
    
    breaker := circuitbreaker.New(
        fmt.Sprintf("reranker_%s", config.Model),
        breakerConfig,
    )
    
    // Create retry policy
    retryPolicy := retry.NewExponentialBackoff(
        retry.WithMaxRetries(3),
        retry.WithInitialDelay(100*time.Millisecond),
        retry.WithMaxDelay(2*time.Second),
        retry.WithMultiplier(2.0),
    )
    
    return &CrossEncoderReranker{
        provider:    provider,
        config:      config,
        breaker:     breaker,
        retryPolicy: retryPolicy,
        semaphore:   semaphore.NewWeighted(int64(config.MaxConcurrency)),
        logger:      logger,
        metrics:     observability.NewMetricsClient(),
    }, nil
}

func (c *CrossEncoderReranker) Rerank(ctx context.Context, query string, results []embedding.SearchResult, opts *RerankOptions) ([]embedding.SearchResult, error) {
    if len(results) == 0 {
        return results, nil
    }
    
    // Start span for tracing
    ctx, span := observability.StartSpan(ctx, "rerank.cross_encoder")
    defer span.End()
    
    span.SetAttribute("model", c.config.Model)
    span.SetAttribute("input_count", len(results))
    span.SetAttribute("batch_size", c.config.BatchSize)
    
    start := time.Now()
    defer func() {
        c.metrics.RecordDuration("rerank.cross_encoder.duration", time.Since(start),
            map[string]string{"model": c.config.Model})
    }()
    
    // Process in batches to avoid overwhelming the API
    batches := c.createBatches(results, c.config.BatchSize)
    allScores := make([]float64, 0, len(results))
    
    for batchIdx, batch := range batches {
        // Acquire semaphore to limit concurrency
        if err := c.semaphore.Acquire(ctx, 1); err != nil {
            return nil, fmt.Errorf("failed to acquire semaphore: %w", err)
        }
        
        batchScores, err := c.processBatchWithRetry(ctx, query, batch, batchIdx)
        c.semaphore.Release(1)
        
        if err != nil {
            // Log error but continue with other batches (graceful degradation)
            c.logger.Error("Batch reranking failed", map[string]interface{}{
                "batch": batchIdx,
                "error": err.Error(),
            })
            
            // Use original scores for failed batch
            for _, result := range batch {
                allScores = append(allScores, float64(result.Score))
            }
        } else {
            allScores = append(allScores, batchScores...)
        }
    }
    
    // Apply new scores
    rerankedResults := c.applyScores(results, allScores)
    
    // Sort by new scores
    sort.Slice(rerankedResults, func(i, j int) bool {
        return rerankedResults[i].Score > rerankedResults[j].Score
    })
    
    // Return top K if specified
    if opts.TopK > 0 && opts.TopK < len(rerankedResults) {
        rerankedResults = rerankedResults[:opts.TopK]
    }
    
    span.SetAttribute("output_count", len(rerankedResults))
    return rerankedResults, nil
}

func (c *CrossEncoderReranker) processBatchWithRetry(ctx context.Context, query string, batch []embedding.SearchResult, batchIdx int) ([]float64, error) {
    var scores []float64
    var lastErr error
    
    // Use retry policy with circuit breaker
    err := c.retryPolicy.Execute(ctx, func(ctx context.Context) error {
        // Check circuit breaker
        return c.breaker.Execute(ctx, func() error {
            // Create timeout context for this batch
            batchCtx, cancel := context.WithTimeout(ctx, c.config.TimeoutPerBatch)
            defer cancel()
            
            // Prepare documents
            documents := make([]string, len(batch))
            for i, result := range batch {
                documents[i] = result.Content
            }
            
            // Call provider
            resp, err := c.provider.Rerank(batchCtx, providers.RerankRequest{
                Query:     query,
                Documents: documents,
                Model:     c.config.Model,
            })
            if err != nil {
                lastErr = err
                return err
            }
            
            scores = resp.Scores
            return nil
        })
    })
    
    if err != nil {
        c.metrics.Increment("rerank.cross_encoder.batch_failure", 
            map[string]string{
                "model": c.config.Model,
                "batch": fmt.Sprintf("%d", batchIdx),
            })
        return nil, fmt.Errorf("batch reranking failed after retries: %w", lastErr)
    }
    
    return scores, nil
}

func (c *CrossEncoderReranker) createBatches(results []embedding.SearchResult, batchSize int) [][]embedding.SearchResult {
    var batches [][]embedding.SearchResult
    
    for i := 0; i < len(results); i += batchSize {
        end := i + batchSize
        if end > len(results) {
            end = len(results)
        }
        batches = append(batches, results[i:end])
    }
    
    return batches
}

func (c *CrossEncoderReranker) applyScores(results []embedding.SearchResult, scores []float64) []embedding.SearchResult {
    rerankedResults := make([]embedding.SearchResult, len(results))
    
    for i, result := range results {
        rerankedResults[i] = result
        
        if i < len(scores) {
            rerankedResults[i].Score = float32(scores[i])
            
            if rerankedResults[i].Metadata == nil {
                rerankedResults[i].Metadata = make(map[string]interface{})
            }
            rerankedResults[i].Metadata["original_score"] = result.Score
            rerankedResults[i].Metadata["rerank_model"] = c.config.Model
            rerankedResults[i].Metadata["reranked"] = true
        }
    }
    
    return rerankedResults
}

func (c *CrossEncoderReranker) GetName() string {
    return fmt.Sprintf("cross_encoder_%s", c.config.Model)
}

func (c *CrossEncoderReranker) Close() error {
    // Clean up any resources
    return c.breaker.Close()
}
```

##### File: `pkg/embedding/rerank/mmr.go`
```go
package rerank

import (
    "context"
    "math"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
)

// MMRReranker implements Maximal Marginal Relevance for diversity
type MMRReranker struct {
    lambda           float64 // Balance between relevance and diversity (0-1)
    embeddingService embedding.EmbeddingService
}

func NewMMRReranker(lambda float64, embeddingService embedding.EmbeddingService) *MMRReranker {
    if lambda < 0 || lambda > 1 {
        lambda = 0.5
    }
    return &MMRReranker{
        lambda:           lambda,
        embeddingService: embeddingService,
    }
}

func (m *MMRReranker) Rerank(ctx context.Context, query string, results []embedding.SearchResult, opts *RerankOptions) ([]embedding.SearchResult, error) {
    if len(results) <= 1 {
        return results, nil
    }
    
    // Get embeddings for all results
    embeddings, err := m.getEmbeddings(ctx, results)
    if err != nil {
        return nil, err
    }
    
    // Get query embedding
    queryEmbedding, err := m.embeddingService.GenerateEmbedding(ctx, embedding.GenerateEmbeddingRequest{
        Text: query,
    })
    if err != nil {
        return nil, err
    }
    
    selected := make([]embedding.SearchResult, 0, len(results))
    selectedIndices := make(map[int]bool)
    
    // Select results iteratively
    for len(selected) < len(results) && (opts.TopK == 0 || len(selected) < opts.TopK) {
        bestScore := -math.MaxFloat64
        bestIdx := -1
        
        for i, result := range results {
            if selectedIndices[i] {
                continue
            }
            
            // Calculate relevance (similarity to query)
            relevance := cosineSimilarity(embeddings[i], queryEmbedding.Embedding)
            
            // Calculate diversity (min similarity to selected items)
            diversity := 1.0
            for j := range selected {
                if selectedIndices[j] {
                    sim := cosineSimilarity(embeddings[i], embeddings[j])
                    diversity = math.Min(diversity, 1.0-sim)
                }
            }
            
            // MMR score
            score := m.lambda*relevance + (1-m.lambda)*diversity
            
            if score > bestScore {
                bestScore = score
                bestIdx = i
            }
        }
        
        if bestIdx >= 0 {
            selected = append(selected, results[bestIdx])
            selectedIndices[bestIdx] = true
        } else {
            break
        }
    }
    
    return selected, nil
}

func (m *MMRReranker) GetName() string {
    return "mmr"
}

func (m *MMRReranker) getEmbeddings(ctx context.Context, results []embedding.SearchResult) ([][]float32, error) {
    // In production, these might be cached or stored with results
    embeddings := make([][]float32, len(results))
    
    for i, result := range results {
        resp, err := m.embeddingService.GenerateEmbedding(ctx, embedding.GenerateEmbeddingRequest{
            Text: result.Content,
        })
        if err != nil {
            return nil, err
        }
        embeddings[i] = resp.Embedding
    }
    
    return embeddings, nil
}

func cosineSimilarity(a, b []float32) float64 {
    if len(a) != len(b) {
        return 0
    }
    
    var dotProduct, normA, normB float64
    for i := range a {
        dotProduct += float64(a[i] * b[i])
        normA += float64(a[i] * a[i])
        normB += float64(b[i] * b[i])
    }
    
    if normA == 0 || normB == 0 {
        return 0
    }
    
    return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}
```

### Phase 3: Query Expansion (Weeks 6-7)

##### File: `pkg/embedding/expansion/query_expander.go`
```go
package expansion

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/llm"
)

// QueryExpander expands queries for better recall
type QueryExpander interface {
    Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error)
}

// ExpansionOptions configures query expansion
type ExpansionOptions struct {
    MaxExpansions   int
    IncludeOriginal bool
    ExpansionTypes  []ExpansionType
    Language        string
    Domain          string
}

type ExpansionType string

const (
    ExpansionTypeSynonym     ExpansionType = "synonym"
    ExpansionTypeHyDE        ExpansionType = "hyde"
    ExpansionTypeDecompose   ExpansionType = "decompose"
    ExpansionTypeBacktranslation ExpansionType = "backtranslation"
)

// ExpandedQuery contains the original and expanded queries
type ExpandedQuery struct {
    Original   string
    Expansions []QueryVariation
}

type QueryVariation struct {
    Text      string
    Type      ExpansionType
    Weight    float32
    Metadata  map[string]interface{}
}

// MultiStrategyExpander combines multiple expansion strategies
type MultiStrategyExpander struct {
    strategies map[ExpansionType]QueryExpander
    llmClient  llm.Client
    config     *Config
}

type Config struct {
    DefaultMaxExpansions int
    EnabledStrategies    []ExpansionType
}

func NewMultiStrategyExpander(llmClient llm.Client, config *Config) *MultiStrategyExpander {
    expander := &MultiStrategyExpander{
        strategies: make(map[ExpansionType]QueryExpander),
        llmClient:  llmClient,
        config:     config,
    }
    
    // Initialize strategies
    expander.strategies[ExpansionTypeSynonym] = NewSynonymExpander(llmClient)
    expander.strategies[ExpansionTypeHyDE] = NewHyDEExpander(llmClient)
    expander.strategies[ExpansionTypeDecompose] = NewDecompositionExpander(llmClient)
    
    return expander
}

func (m *MultiStrategyExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
    if opts == nil {
        opts = &ExpansionOptions{
            MaxExpansions:   m.config.DefaultMaxExpansions,
            IncludeOriginal: true,
            ExpansionTypes:  m.config.EnabledStrategies,
        }
    }
    
    expanded := &ExpandedQuery{
        Original:   query,
        Expansions: []QueryVariation{},
    }
    
    if opts.IncludeOriginal {
        expanded.Expansions = append(expanded.Expansions, QueryVariation{
            Text:   query,
            Type:   "original",
            Weight: 1.0,
        })
    }
    
    // Apply each requested strategy
    for _, strategyType := range opts.ExpansionTypes {
        if strategy, ok := m.strategies[strategyType]; ok {
            strategyExpanded, err := strategy.Expand(ctx, query, opts)
            if err != nil {
                // Log error but continue with other strategies
                continue
            }
            
            expanded.Expansions = append(expanded.Expansions, strategyExpanded.Expansions...)
        }
    }
    
    // Limit total expansions
    if opts.MaxExpansions > 0 && len(expanded.Expansions) > opts.MaxExpansions {
        expanded.Expansions = expanded.Expansions[:opts.MaxExpansions]
    }
    
    return expanded, nil
}
```

##### File: `pkg/embedding/expansion/hyde.go`
```go
package expansion

import (
    "context"
    "fmt"
    "strings"
    
    "github.com/developer-mesh/developer-mesh/pkg/llm"
)

// HyDEExpander implements Hypothetical Document Embeddings
type HyDEExpander struct {
    llmClient llm.Client
    templates map[string]string
}

func NewHyDEExpander(llmClient llm.Client) *HyDEExpander {
    return &HyDEExpander{
        llmClient: llmClient,
        templates: map[string]string{
            "default": `Generate a detailed, technical answer to this question: "%s"
Include specific examples, code snippets if relevant, and technical details.
The answer should be comprehensive and directly address the query.`,
            
            "code": `Write a complete code example that answers this programming question: "%s"
Include:
- Complete, runnable code
- Comments explaining key parts
- Any necessary imports or setup
- Example usage`,
            
            "documentation": `Write detailed technical documentation that answers: "%s"
Include:
- Overview and context
- Step-by-step explanations
- Best practices
- Common pitfalls
- Examples`,
        },
    }
}

func (h *HyDEExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
    // Detect query type
    queryType := h.detectQueryType(query)
    template := h.templates[queryType]
    
    prompt := fmt.Sprintf(template, query)
    
    // Generate hypothetical document
    response, err := h.llmClient.Complete(ctx, llm.CompletionRequest{
        Prompt:      prompt,
        MaxTokens:   500,
        Temperature: 0.7,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to generate HyDE: %w", err)
    }
    
    return &ExpandedQuery{
        Original: query,
        Expansions: []QueryVariation{
            {
                Text:   response.Text,
                Type:   ExpansionTypeHyDE,
                Weight: 0.3, // Lower weight for hypothetical documents
                Metadata: map[string]interface{}{
                    "query_type": queryType,
                    "template":   template,
                },
            },
        },
    }, nil
}

func (h *HyDEExpander) detectQueryType(query string) string {
    lowerQuery := strings.ToLower(query)
    
    codeKeywords := []string{"code", "function", "implement", "example", "snippet", "how to write"}
    for _, keyword := range codeKeywords {
        if strings.Contains(lowerQuery, keyword) {
            return "code"
        }
    }
    
    docKeywords := []string{"documentation", "explain", "what is", "describe", "guide"}
    for _, keyword := range docKeywords {
        if strings.Contains(lowerQuery, keyword) {
            return "documentation"
        }
    }
    
    return "default"
}
```

##### File: `pkg/embedding/expansion/decomposition.go`
```go
package expansion

import (
    "context"
    "encoding/json"
    "fmt"
    "strings"
    
    "github.com/developer-mesh/developer-mesh/pkg/llm"
)

// DecompositionExpander breaks complex queries into sub-queries
type DecompositionExpander struct {
    llmClient llm.Client
}

func NewDecompositionExpander(llmClient llm.Client) *DecompositionExpander {
    return &DecompositionExpander{llmClient: llmClient}
}

func (d *DecompositionExpander) Expand(ctx context.Context, query string, opts *ExpansionOptions) (*ExpandedQuery, error) {
    prompt := fmt.Sprintf(`Decompose this search query into simpler sub-queries: "%s"

Rules:
1. Each sub-query should capture a specific aspect of the original query
2. Sub-queries should be self-contained and searchable
3. Avoid redundancy between sub-queries
4. Maximum 4 sub-queries

Return as JSON array of objects with 'query' and 'focus' fields.
Example: [{"query": "Python error handling", "focus": "language and topic"}, {"query": "try except blocks", "focus": "specific construct"}]`, query)
    
    response, err := d.llmClient.Complete(ctx, llm.CompletionRequest{
        Prompt:      prompt,
        MaxTokens:   300,
        Temperature: 0.3,
        Format:      "json",
    })
    if err != nil {
        return nil, fmt.Errorf("failed to decompose query: %w", err)
    }
    
    // Parse JSON response
    var decomposed []struct {
        Query string `json:"query"`
        Focus string `json:"focus"`
    }
    
    if err := json.Unmarshal([]byte(response.Text), &decomposed); err != nil {
        // Fallback to simple decomposition
        return d.simpleDecompose(query), nil
    }
    
    expansions := make([]QueryVariation, 0, len(decomposed))
    for i, subQuery := range decomposed {
        weight := 1.0 / float32(i+2) // Decreasing weights
        expansions = append(expansions, QueryVariation{
            Text:   subQuery.Query,
            Type:   ExpansionTypeDecompose,
            Weight: weight,
            Metadata: map[string]interface{}{
                "focus":         subQuery.Focus,
                "original_query": query,
            },
        })
    }
    
    return &ExpandedQuery{
        Original:   query,
        Expansions: expansions,
    }, nil
}

func (d *DecompositionExpander) simpleDecompose(query string) *ExpandedQuery {
    // Simple heuristic-based decomposition
    words := strings.Fields(query)
    expansions := []QueryVariation{}
    
    // Look for conjunctions and split
    for i, word := range words {
        if word == "and" || word == "with" || word == "for" {
            if i > 0 && i < len(words)-1 {
                part1 := strings.Join(words[:i], " ")
                part2 := strings.Join(words[i+1:], " ")
                
                expansions = append(expansions, 
                    QueryVariation{Text: part1, Type: ExpansionTypeDecompose, Weight: 0.5},
                    QueryVariation{Text: part2, Type: ExpansionTypeDecompose, Weight: 0.5},
                )
            }
        }
    }
    
    return &ExpandedQuery{
        Original:   query,
        Expansions: expansions,
    }
}
```

### Phase 4: Advanced Text Chunking (Week 8)

##### File: `pkg/chunking/text/semantic_chunker.go`
```go
package text

import (
    "context"
    "strings"
    
    "github.com/developer-mesh/developer-mesh/pkg/chunking"
    "github.com/developer-mesh/developer-mesh/pkg/tokenizer"
)

// SemanticChunker implements semantic-aware text chunking
type SemanticChunker struct {
    tokenizer        tokenizer.Tokenizer
    sentenceSplitter SentenceSplitter
    config           *Config
}

type Config struct {
    MinChunkSize     int     // Minimum tokens per chunk
    MaxChunkSize     int     // Maximum tokens per chunk
    TargetChunkSize  int     // Target size for chunks
    OverlapSize      int     // Token overlap between chunks
    SimilarityThreshold float32 // For semantic boundaries
}

func NewSemanticChunker(tokenizer tokenizer.Tokenizer, config *Config) *SemanticChunker {
    if config.TargetChunkSize == 0 {
        config.TargetChunkSize = 512
    }
    if config.MinChunkSize == 0 {
        config.MinChunkSize = 100
    }
    if config.MaxChunkSize == 0 {
        config.MaxChunkSize = 1024
    }
    
    return &SemanticChunker{
        tokenizer:        tokenizer,
        sentenceSplitter: NewSentenceSplitter(),
        config:           config,
    }
}

func (s *SemanticChunker) Chunk(ctx context.Context, text string, metadata map[string]interface{}) ([]*chunking.TextChunk, error) {
    // Split into sentences
    sentences := s.sentenceSplitter.Split(text)
    if len(sentences) == 0 {
        return []*chunking.TextChunk{}, nil
    }
    
    chunks := []*chunking.TextChunk{}
    currentChunk := &chunking.TextChunk{
        Metadata: metadata,
    }
    currentTokens := 0
    
    for i, sentence := range sentences {
        sentenceTokens := s.tokenizer.CountTokens(sentence)
        
        // Check if adding sentence exceeds max size
        if currentTokens > 0 && currentTokens+sentenceTokens > s.config.MaxChunkSize {
            // Finalize current chunk
            chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))
            
            // Start new chunk with overlap
            overlapText := s.getOverlapText(currentChunk.Content, s.config.OverlapSize)
            currentChunk = &chunking.TextChunk{
                Content:  overlapText,
                Metadata: metadata,
            }
            currentTokens = s.tokenizer.CountTokens(overlapText)
        }
        
        // Add sentence to current chunk
        if currentChunk.Content != "" {
            currentChunk.Content += " "
        }
        currentChunk.Content += sentence
        currentTokens += sentenceTokens
        
        // Check if we should create a chunk at semantic boundary
        if currentTokens >= s.config.TargetChunkSize {
            if s.isSemanticBoundary(sentences, i) {
                chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))
                
                // Start new chunk
                currentChunk = &chunking.TextChunk{
                    Metadata: metadata,
                }
                currentTokens = 0
            }
        }
    }
    
    // Add final chunk if it meets minimum size
    if currentTokens >= s.config.MinChunkSize {
        chunks = append(chunks, s.finalizeChunk(currentChunk, len(chunks)))
    } else if len(chunks) > 0 {
        // Merge with previous chunk if too small
        chunks[len(chunks)-1].Content += " " + currentChunk.Content
    }
    
    return chunks, nil
}

func (s *SemanticChunker) isSemanticBoundary(sentences []string, index int) bool {
    if index >= len(sentences)-1 {
        return true
    }
    
    currentSentence := sentences[index]
    nextSentence := sentences[index+1]
    
    // Check for paragraph boundaries
    if strings.HasSuffix(currentSentence, "\n\n") {
        return true
    }
    
    // Check for section headers (simple heuristic)
    if len(nextSentence) < 100 && !strings.HasSuffix(nextSentence, ".") {
        return true
    }
    
    // Check for topic shift indicators
    topicShiftIndicators := []string{
        "however", "furthermore", "additionally", "in conclusion",
        "on the other hand", "in summary", "next", "finally",
    }
    
    lowerNext := strings.ToLower(nextSentence)
    for _, indicator := range topicShiftIndicators {
        if strings.HasPrefix(lowerNext, indicator) {
            return true
        }
    }
    
    return false
}

func (s *SemanticChunker) getOverlapText(content string, overlapTokens int) string {
    if overlapTokens <= 0 {
        return ""
    }
    
    sentences := s.sentenceSplitter.Split(content)
    overlapContent := ""
    tokenCount := 0
    
    // Add sentences from the end until we reach overlap size
    for i := len(sentences) - 1; i >= 0 && tokenCount < overlapTokens; i-- {
        sentence := sentences[i]
        sentTokens := s.tokenizer.CountTokens(sentence)
        
        if tokenCount+sentTokens <= overlapTokens*1.2 { // Allow 20% overflow
            overlapContent = sentence + " " + overlapContent
            tokenCount += sentTokens
        } else {
            break
        }
    }
    
    return strings.TrimSpace(overlapContent)
}

func (s *SemanticChunker) finalizeChunk(chunk *chunking.TextChunk, index int) *chunking.TextChunk {
    chunk.Index = index
    chunk.TokenCount = s.tokenizer.CountTokens(chunk.Content)
    
    if chunk.Metadata == nil {
        chunk.Metadata = make(map[string]interface{})
    }
    chunk.Metadata["chunking_method"] = "semantic"
    chunk.Metadata["chunk_index"] = index
    
    return chunk
}
```

##### File: `pkg/chunking/text/recursive_splitter.go`
```go
package text

import (
    "context"
    "strings"
    
    "github.com/developer-mesh/developer-mesh/pkg/chunking"
)

// RecursiveCharacterSplitter implements recursive splitting with multiple separators
type RecursiveCharacterSplitter struct {
    separators      []string
    chunkSize       int
    chunkOverlap    int
    lengthFunction  func(string) int
}

func NewRecursiveCharacterSplitter(chunkSize, chunkOverlap int) *RecursiveCharacterSplitter {
    return &RecursiveCharacterSplitter{
        separators: []string{
            "\n\n\n",  // Triple newline (major sections)
            "\n\n",    // Double newline (paragraphs)
            "\n",      // Single newline
            ". ",      // Sentence end
            "! ",      // Exclamation
            "? ",      // Question
            "; ",      // Semicolon
            ", ",      // Comma
            " ",       // Space
            "",        // Character level
        },
        chunkSize:    chunkSize,
        chunkOverlap: chunkOverlap,
        lengthFunction: func(s string) int {
            return len(s) // Can be replaced with token counter
        },
    }
}

func (r *RecursiveCharacterSplitter) SplitText(text string) []string {
    return r.splitTextRecursive(text, r.separators)
}

func (r *RecursiveCharacterSplitter) splitTextRecursive(text string, separators []string) []string {
    finalChunks := []string{}
    
    // Find the separator to use
    separator := ""
    for _, sep := range separators {
        if sep == "" || strings.Contains(text, sep) {
            separator = sep
            break
        }
    }
    
    // Split the text
    var splits []string
    if separator == "" {
        // Character level split
        splits = r.splitByCharacters(text)
    } else {
        splits = strings.Split(text, separator)
        
        // Add separator back to chunks (except last)
        for i := 0; i < len(splits)-1; i++ {
            splits[i] += separator
        }
    }
    
    // Merge splits into chunks
    currentChunk := ""
    for _, split := range splits {
        splitLen := r.lengthFunction(split)
        
        if splitLen > r.chunkSize {
            // Split is too large, need to split it further
            if currentChunk != "" {
                finalChunks = append(finalChunks, currentChunk)
                currentChunk = ""
            }
            
            // Recursively split the large chunk
            if len(separators) > 1 {
                subChunks := r.splitTextRecursive(split, separators[1:])
                finalChunks = append(finalChunks, subChunks...)
            } else {
                // Force split at chunk size
                forceSplit := r.forceSplit(split)
                finalChunks = append(finalChunks, forceSplit...)
            }
        } else if r.lengthFunction(currentChunk)+splitLen > r.chunkSize {
            // Adding this split would exceed chunk size
            finalChunks = append(finalChunks, currentChunk)
            currentChunk = split
        } else {
            // Add to current chunk
            currentChunk += split
        }
    }
    
    // Add final chunk
    if currentChunk != "" {
        finalChunks = append(finalChunks, currentChunk)
    }
    
    // Apply overlap
    if r.chunkOverlap > 0 {
        finalChunks = r.applyOverlap(finalChunks)
    }
    
    return finalChunks
}

func (r *RecursiveCharacterSplitter) forceSplit(text string) []string {
    chunks := []string{}
    
    for i := 0; i < len(text); i += r.chunkSize {
        end := i + r.chunkSize
        if end > len(text) {
            end = len(text)
        }
        chunks = append(chunks, text[i:end])
    }
    
    return chunks
}

func (r *RecursiveCharacterSplitter) applyOverlap(chunks []string) []string {
    if len(chunks) <= 1 || r.chunkOverlap == 0 {
        return chunks
    }
    
    overlapped := make([]string, len(chunks))
    
    for i := 0; i < len(chunks); i++ {
        start := chunks[i]
        
        // Add overlap from previous chunk
        if i > 0 {
            prevChunk := chunks[i-1]
            overlapStart := len(prevChunk) - r.chunkOverlap
            if overlapStart < 0 {
                overlapStart = 0
            }
            start = prevChunk[overlapStart:] + start
        }
        
        overlapped[i] = start
    }
    
    return overlapped
}

func (r *RecursiveCharacterSplitter) splitByCharacters(text string) []string {
    if len(text) <= r.chunkSize {
        return []string{text}
    }
    
    chunks := []string{}
    for i := 0; i < len(text); i += r.chunkSize - r.chunkOverlap {
        end := i + r.chunkSize
        if end > len(text) {
            end = len(text)
        }
        chunks = append(chunks, text[i:end])
        
        if end >= len(text) {
            break
        }
    }
    
    return chunks
}
```

### Phase 5: Semantic Caching (Week 9)

##### File: `pkg/embedding/cache/semantic_cache.go`
```go
package cache

import (
    "context"
    "encoding/json"
    "fmt"
    "sync"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/go-redis/redis/v8"
)

// SemanticCache implements similarity-based caching for embeddings
type SemanticCache struct {
    redis            *redis.Client
    embeddingService embedding.EmbeddingService
    vectorDB         embedding.VectorDatabase
    config           *Config
    normalizer       QueryNormalizer
    mu               sync.RWMutex
}

type Config struct {
    SimilarityThreshold float32       // Minimum similarity for cache hit
    TTL                 time.Duration // Cache entry TTL
    MaxCandidates       int           // Max candidates to check
    Prefix              string        // Redis key prefix
    WarmupQueries       []string      // Queries to pre-warm
}

// CacheEntry represents a cached query result
type CacheEntry struct {
    Query           string                    `json:"query"`
    NormalizedQuery string                    `json:"normalized_query"`
    Embedding       []float32                 `json:"embedding"`
    Results         []embedding.SearchResult  `json:"results"`
    Metadata        map[string]interface{}    `json:"metadata"`
    CachedAt        time.Time                 `json:"cached_at"`
    HitCount        int                       `json:"hit_count"`
    LastAccessedAt  time.Time                 `json:"last_accessed_at"`
}

func NewSemanticCache(
    redis *redis.Client,
    embeddingService embedding.EmbeddingService,
    vectorDB embedding.VectorDatabase,
    config *Config,
) *SemanticCache {
    if config.SimilarityThreshold == 0 {
        config.SimilarityThreshold = 0.95
    }
    if config.TTL == 0 {
        config.TTL = 24 * time.Hour
    }
    if config.MaxCandidates == 0 {
        config.MaxCandidates = 10
    }
    
    return &SemanticCache{
        redis:            redis,
        embeddingService: embeddingService,
        vectorDB:         vectorDB,
        config:           config,
        normalizer:       NewQueryNormalizer(),
    }
}

// Get retrieves cached results for a query
func (c *SemanticCache) Get(ctx context.Context, query string) (*CacheEntry, error) {
    // Normalize query
    normalized := c.normalizer.Normalize(query)
    
    // Try exact match first
    entry, err := c.getExactMatch(ctx, normalized)
    if err == nil && entry != nil {
        c.updateAccessStats(ctx, entry)
        return entry, nil
    }
    
    // Generate embedding for semantic search
    embedding, err := c.embeddingService.GenerateEmbedding(ctx, embedding.GenerateEmbeddingRequest{
        Text: query,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to generate embedding: %w", err)
    }
    
    // Search for similar cached queries
    candidates, err := c.searchSimilarQueries(ctx, embedding.Embedding, c.config.MaxCandidates)
    if err != nil {
        return nil, fmt.Errorf("failed to search similar queries: %w", err)
    }
    
    // Find best match above threshold
    for _, candidate := range candidates {
        if candidate.Similarity >= c.config.SimilarityThreshold {
            entry, err := c.getCacheEntry(ctx, candidate.CacheKey)
            if err == nil && entry != nil {
                c.updateAccessStats(ctx, entry)
                return entry, nil
            }
        }
    }
    
    return nil, nil // Cache miss
}

// Set stores query results in cache
func (c *SemanticCache) Set(ctx context.Context, query string, results []embedding.SearchResult) error {
    normalized := c.normalizer.Normalize(query)
    
    // Generate embedding
    embedding, err := c.embeddingService.GenerateEmbedding(ctx, embedding.GenerateEmbeddingRequest{
        Text: query,
    })
    if err != nil {
        return fmt.Errorf("failed to generate embedding: %w", err)
    }
    
    entry := &CacheEntry{
        Query:           query,
        NormalizedQuery: normalized,
        Embedding:       embedding.Embedding,
        Results:         results,
        CachedAt:        time.Now(),
        HitCount:        0,
        LastAccessedAt:  time.Now(),
        Metadata: map[string]interface{}{
            "embedding_model": embedding.Model,
            "result_count":    len(results),
        },
    }
    
    // Store in Redis
    key := c.getCacheKey(normalized)
    data, err := json.Marshal(entry)
    if err != nil {
        return err
    }
    
    err = c.redis.Set(ctx, key, data, c.config.TTL).Err()
    if err != nil {
        return err
    }
    
    // Store embedding in vector DB for similarity search
    err = c.storeCacheEmbedding(ctx, normalized, embedding.Embedding, key)
    if err != nil {
        // Log error but don't fail - exact match will still work
        return nil
    }
    
    return nil
}

// Warm pre-loads cache with common queries
func (c *SemanticCache) Warm(ctx context.Context, queries []string) error {
    var wg sync.WaitGroup
    errors := make(chan error, len(queries))
    
    // Limit concurrency
    sem := make(chan struct{}, 10)
    
    for _, query := range queries {
        wg.Add(1)
        go func(q string) {
            defer wg.Done()
            
            sem <- struct{}{}
            defer func() { <-sem }()
            
            // Check if already cached
            if entry, _ := c.Get(ctx, q); entry != nil {
                return
            }
            
            // Execute search
            results, err := c.executeSearch(ctx, q)
            if err != nil {
                errors <- fmt.Errorf("failed to warm query %s: %w", q, err)
                return
            }
            
            // Cache results
            if err := c.Set(ctx, q, results); err != nil {
                errors <- fmt.Errorf("failed to cache query %s: %w", q, err)
            }
        }(query)
    }
    
    wg.Wait()
    close(errors)
    
    // Collect errors
    var errs []error
    for err := range errors {
        errs = append(errs, err)
    }
    
    if len(errs) > 0 {
        return fmt.Errorf("cache warming completed with %d errors", len(errs))
    }
    
    return nil
}

// Stats returns cache statistics
func (c *SemanticCache) Stats(ctx context.Context) (*CacheStats, error) {
    pattern := fmt.Sprintf("%s:*", c.config.Prefix)
    keys, err := c.redis.Keys(ctx, pattern).Result()
    if err != nil {
        return nil, err
    }
    
    stats := &CacheStats{
        TotalEntries: len(keys),
        Timestamp:    time.Now(),
    }
    
    // Analyze cache entries
    for _, key := range keys {
        data, err := c.redis.Get(ctx, key).Bytes()
        if err != nil {
            continue
        }
        
        var entry CacheEntry
        if err := json.Unmarshal(data, &entry); err != nil {
            continue
        }
        
        stats.TotalHits += entry.HitCount
        age := time.Since(entry.CachedAt)
        if age > stats.OldestEntry {
            stats.OldestEntry = age
        }
        
        stats.TotalResults += len(entry.Results)
    }
    
    if stats.TotalEntries > 0 {
        stats.AverageHitsPerEntry = float64(stats.TotalHits) / float64(stats.TotalEntries)
        stats.AverageResultsPerEntry = float64(stats.TotalResults) / float64(stats.TotalEntries)
    }
    
    return stats, nil
}

type CacheStats struct {
    TotalEntries          int
    TotalHits             int
    AverageHitsPerEntry   float64
    AverageResultsPerEntry float64
    OldestEntry           time.Duration
    Timestamp             time.Time
}

// Helper methods

func (c *SemanticCache) getCacheKey(normalized string) string {
    return fmt.Sprintf("%s:%s", c.config.Prefix, normalized)
}

func (c *SemanticCache) getExactMatch(ctx context.Context, normalized string) (*CacheEntry, error) {
    key := c.getCacheKey(normalized)
    data, err := c.redis.Get(ctx, key).Bytes()
    if err != nil {
        return nil, err
    }
    
    var entry CacheEntry
    if err := json.Unmarshal(data, &entry); err != nil {
        return nil, err
    }
    
    return &entry, nil
}

func (c *SemanticCache) searchSimilarQueries(ctx context.Context, embedding []float32, limit int) ([]SimilarQuery, error) {
    // This would use the vector DB to find similar cached queries
    // Implementation depends on the specific vector DB being used
    return []SimilarQuery{}, nil
}

type SimilarQuery struct {
    CacheKey   string
    Similarity float32
}

func (c *SemanticCache) updateAccessStats(ctx context.Context, entry *CacheEntry) {
    entry.HitCount++
    entry.LastAccessedAt = time.Now()
    
    // Update in Redis
    key := c.getCacheKey(entry.NormalizedQuery)
    data, _ := json.Marshal(entry)
    c.redis.Set(ctx, key, data, c.config.TTL)
}

func (c *SemanticCache) executeSearch(ctx context.Context, query string) ([]embedding.SearchResult, error) {
    // This would execute the actual search
    // Placeholder for the actual implementation
    return []embedding.SearchResult{}, nil
}

func (c *SemanticCache) storeCacheEmbedding(ctx context.Context, query string, embedding []float32, cacheKey string) error {
    // Store in vector DB for similarity search
    // Implementation depends on the specific vector DB
    return nil
}
```

## Testing Strategy

### Unit Tests with Mocking

#### File: `pkg/embedding/hybrid/search_test.go`
```go
package hybrid

import (
    "context"
    "database/sql"
    "errors"
    "testing"
    "time"
    
    "github.com/DATA-DOG/go-sqlmock"
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    "github.com/stretchr/testify/require"
)

// MockVectorSearch implements embedding.SearchService for testing
type MockVectorSearch struct {
    mock.Mock
}

func (m *MockVectorSearch) Search(ctx context.Context, query string, opts *embedding.SearchOptions) (*embedding.SearchResults, error) {
    args := m.Called(ctx, query, opts)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*embedding.SearchResults), args.Error(1)
}

func TestHybridSearchService_Search(t *testing.T) {
    tests := []struct {
        name          string
        query         string
        setupMocks    func(*MockVectorSearch, sqlmock.Sqlmock)
        setupContext  func() context.Context
        expectedError bool
        errorContains string
        validate      func(*testing.T, *embedding.SearchResults)
    }{
        {
            name:  "successful hybrid search",
            query: "test query",
            setupMocks: func(mv *MockVectorSearch, sqlMock sqlmock.Sqlmock) {
                // Mock vector search
                mv.On("Search", mock.Anything, "test query", mock.Anything).
                    Return(&embedding.SearchResults{
                        Results: []embedding.SearchResult{
                            {ID: "1", Content: "vector result 1", Score: 0.9},
                            {ID: "2", Content: "vector result 2", Score: 0.8},
                        },
                    }, nil)
                
                // Mock keyword search SQL
                rows := sqlmock.NewRows([]string{"id", "content", "metadata", "content_hash", "created_at", "score"}).
                    AddRow("3", "keyword result 1", nil, "hash1", time.Now(), 0.85).
                    AddRow("4", "keyword result 2", nil, "hash2", time.Now(), 0.75)
                
                sqlMock.ExpectQuery("WITH doc_stats AS").
                    WithArgs(
                        mock.Anything, // tenant_id
                        mock.Anything, // query terms array
                        mock.Anything, // k1
                        mock.Anything, // b
                        mock.Anything, // query string
                        mock.Anything, // min score
                        mock.Anything, // limit
                        mock.Anything, // offset
                    ).
                    WillReturnRows(rows)
            },
            setupContext: func() context.Context {
                ctx := context.Background()
                ctx = auth.WithTenantID(ctx, uuid.New())
                return ctx
            },
            expectedError: false,
            validate: func(t *testing.T, results *embedding.SearchResults) {
                assert.Len(t, results.Results, 4)
                // Check that results are properly fused and sorted
                assert.Greater(t, results.Results[0].Score, results.Results[3].Score)
            },
        },
        {
            name:  "vector search fails, fallback to keyword",
            query: "test query",
            setupMocks: func(mv *MockVectorSearch, sqlMock sqlmock.Sqlmock) {
                // Mock vector search failure
                mv.On("Search", mock.Anything, "test query", mock.Anything).
                    Return(nil, errors.New("vector search error"))
                
                // Mock successful keyword search
                rows := sqlmock.NewRows([]string{"id", "content", "metadata", "content_hash", "created_at", "score"}).
                    AddRow("1", "keyword result", nil, "hash1", time.Now(), 0.9)
                
                sqlMock.ExpectQuery("WITH doc_stats AS").
                    WillReturnRows(rows)
            },
            setupContext: func() context.Context {
                ctx := context.Background()
                ctx = auth.WithTenantID(ctx, uuid.New())
                return ctx
            },
            expectedError: false,
            validate: func(t *testing.T, results *embedding.SearchResults) {
                assert.Len(t, results.Results, 1)
                assert.Equal(t, "keyword", results.Results[0].Metadata["search_type"])
            },
        },
        {
            name:  "missing tenant ID",
            query: "test query",
            setupMocks: func(mv *MockVectorSearch, sqlMock sqlmock.Sqlmock) {
                // No mocks needed - should fail before calling
            },
            setupContext: func() context.Context {
                return context.Background() // No tenant ID
            },
            expectedError: true,
            errorContains: "tenant ID not found",
        },
        {
            name:  "empty query",
            query: "   ",
            setupMocks: func(mv *MockVectorSearch, sqlMock sqlmock.Sqlmock) {
                // No mocks needed - should fail validation
            },
            setupContext: func() context.Context {
                ctx := context.Background()
                ctx = auth.WithTenantID(ctx, uuid.New())
                return ctx
            },
            expectedError: true,
            errorContains: "query cannot be empty",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks
            db, sqlMock, err := sqlmock.New()
            require.NoError(t, err)
            defer db.Close()
            
            mockVectorSearch := new(MockVectorSearch)
            
            if tt.setupMocks != nil {
                tt.setupMocks(mockVectorSearch, sqlMock)
            }
            
            // Create service
            service, err := NewHybridSearchService(
                db,
                mockVectorSearch,
                observability.NewLogger("test"),
                &Config{
                    VectorWeight:  0.7,
                    KeywordWeight: 0.3,
                    RRFConstant:   60,
                    MinScore:      0.1,
                },
            )
            require.NoError(t, err)
            
            // Execute search
            ctx := tt.setupContext()
            results, err := service.Search(ctx, tt.query, &SearchOptions{
                SearchOptions: &embedding.SearchOptions{
                    Limit: 10,
                },
                UseHybrid: true,
            })
            
            // Validate
            if tt.expectedError {
                assert.Error(t, err)
                if tt.errorContains != "" {
                    assert.Contains(t, err.Error(), tt.errorContains)
                }
            } else {
                assert.NoError(t, err)
                if tt.validate != nil {
                    tt.validate(t, results)
                }
            }
            
            // Verify all expectations met
            assert.NoError(t, sqlMock.ExpectationsWereMet())
            mockVectorSearch.AssertExpectations(t)
        })
    }
}

func TestHybridSearchService_validateSearchTerm(t *testing.T) {
    service := &HybridSearchService{}
    
    tests := []struct {
        term          string
        expectedError bool
        errorContains string
    }{
        {"normal", false, ""},
        {"test-term_123", false, ""},
        {strings.Repeat("a", 101), true, "too long"},
        {"--comment", true, "dangerous pattern"},
        {"'; DROP TABLE", true, "dangerous pattern"},
        {"/*comment*/", true, "dangerous pattern"},
    }
    
    for _, tt := range tests {
        t.Run(tt.term, func(t *testing.T) {
            err := service.validateSearchTerm(tt.term)
            if tt.expectedError {
                assert.Error(t, err)
                assert.Contains(t, err.Error(), tt.errorContains)
            } else {
                assert.NoError(t, err)
            }
        })
    }
}

func BenchmarkHybridSearch(b *testing.B) {
    // Setup
    db, sqlMock, _ := sqlmock.New()
    defer db.Close()
    
    mockVectorSearch := new(MockVectorSearch)
    mockVectorSearch.On("Search", mock.Anything, mock.Anything, mock.Anything).
        Return(&embedding.SearchResults{
            Results: generateMockResults(100),
        }, nil)
    
    // Mock SQL for all iterations
    for i := 0; i < b.N; i++ {
        rows := sqlmock.NewRows([]string{"id", "content", "metadata", "content_hash", "created_at", "score"})
        for j := 0; j < 100; j++ {
            rows.AddRow(fmt.Sprintf("k%d", j), "content", nil, "hash", time.Now(), float64(j)/100)
        }
        sqlMock.ExpectQuery("WITH doc_stats AS").WillReturnRows(rows)
    }
    
    service, _ := NewHybridSearchService(db, mockVectorSearch, observability.NewLogger("bench"), &Config{
        VectorWeight:  0.7,
        KeywordWeight: 0.3,
    })
    
    ctx := auth.WithTenantID(context.Background(), uuid.New())
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := service.Search(ctx, "benchmark query", &SearchOptions{
            SearchOptions: &embedding.SearchOptions{Limit: 50},
        })
        if err != nil {
            b.Fatal(err)
        }
    }
}

func generateMockResults(count int) []embedding.SearchResult {
    results := make([]embedding.SearchResult, count)
    for i := 0; i < count; i++ {
        results[i] = embedding.SearchResult{
            ID:      fmt.Sprintf("v%d", i),
            Content: fmt.Sprintf("vector result %d", i),
            Score:   float32(count-i) / float32(count),
        }
    }
    return results
}
```

### Integration Tests
```go
// test/integration/rag_enhancement_test.go
package integration

import (
    "context"
    "strings"
    "testing"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/auth"
    "github.com/developer-mesh/developer-mesh/pkg/database"
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/hybrid"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
    "github.com/google/uuid"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestHybridSearchIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup test database
    db, cleanup := setupTestDatabase(t)
    defer cleanup()
    
    // Setup services
    vectorSearch := setupVectorSearch(t, db)
    hybridSearch, err := hybrid.NewHybridSearchService(
        db.DB,
        vectorSearch,
        observability.NewLogger("test"),
        &hybrid.Config{
            VectorWeight:  0.7,
            KeywordWeight: 0.3,
            EnableBM25:    true,
        },
    )
    require.NoError(t, err)
    
    // Create test data
    tenantID := uuid.New()
    ctx := auth.WithTenantID(context.Background(), tenantID)
    
    testData := []struct {
        content string
        tags    []string
    }{
        {"AWS-123: Error in lambda function execution", []string{"aws", "error", "lambda"}},
        {"AWS error codes reference documentation", []string{"aws", "documentation"}},
        {"Python retry logic with exponential backoff implementation", []string{"python", "retry", "code"}},
        {"Implementing exponential backoff in distributed systems", []string{"distributed", "retry"}},
    }
    
    // Insert test data
    for _, data := range testData {
        err := insertTestEmbedding(ctx, db, tenantID, data.content, data.tags)
        require.NoError(t, err)
    }
    
    // Test cases
    testQueries := []struct {
        query          string
        expectedInTop3 []string
        description    string
    }{
        {
            query:          "AWS-123 error",
            expectedInTop3: []string{"AWS-123", "AWS error codes"},
            description:    "Should find exact ID match with hybrid search",
        },
        {
            query:          "implement retry logic Python",
            expectedInTop3: []string{"Python retry", "exponential backoff"},
            description:    "Should find relevant code examples",
        },
        {
            query:          "lambda function issues",
            expectedInTop3: []string{"AWS-123", "lambda"},
            description:    "Should find semantically related content",
        },
    }
    
    for _, tc := range testQueries {
        t.Run(tc.description, func(t *testing.T) {
            // Execute hybrid search
            results, err := hybridSearch.Search(ctx, tc.query, &hybrid.SearchOptions{
                SearchOptions: &embedding.SearchOptions{
                    Limit: 10,
                },
                UseHybrid: true,
            })
            require.NoError(t, err)
            require.NotEmpty(t, results.Results)
            
            // Get top 3 results
            topResults := results.Results
            if len(topResults) > 3 {
                topResults = topResults[:3]
            }
            
            // Verify expected content appears in top results
            foundCount := 0
            for _, expected := range tc.expectedInTop3 {
                for _, result := range topResults {
                    if strings.Contains(strings.ToLower(result.Content), strings.ToLower(expected)) {
                        foundCount++
                        t.Logf("Found '%s' with score %.3f", expected, result.Score)
                        break
                    }
                }
            }
            
            assert.GreaterOrEqual(t, foundCount, len(tc.expectedInTop3)/2, 
                "Should find at least half of expected results in top 3")
        })
    }
}
```

### Performance Benchmarks
```go
// test/benchmark/rag_benchmark_test.go
func BenchmarkHybridSearch(b *testing.B) {
    queries := loadTestQueries()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        query := queries[i%len(queries)]
        _, err := hybridSearch.Search(context.Background(), query, defaultOptions)
        if err != nil {
            b.Fatal(err)
        }
    }
}

func BenchmarkSemanticCache(b *testing.B) {
    cache := setupSemanticCache()
    queries := loadTestQueries()
    
    // Warm cache
    cache.Warm(context.Background(), queries[:100])
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        query := queries[i%len(queries)]
        _, err := cache.Get(context.Background(), query)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Configuration Updates

### Update `configs/config.base.yaml`:
```yaml
rag:
  hybrid_search:
    enabled: true
    vector_weight: 0.7
    keyword_weight: 0.3
    rrf_constant: 60
    min_score: 0.3
    
  reranking:
    enabled: true
    providers:
      - name: cohere
        model: rerank-english-v3.0
        batch_size: 10
      - name: mmr
        lambda: 0.7
        
  query_expansion:
    enabled: true
    strategies:
      - hyde
      - decompose
      - synonym
    max_expansions: 5
    
  text_chunking:
    strategy: semantic
    target_size: 512
    min_size: 100
    max_size: 1024
    overlap: 50
    
  semantic_cache:
    enabled: true
    similarity_threshold: 0.95
    ttl: 24h
    max_candidates: 10
```

## Security Best Practices

### Input Validation
1. **Query Sanitization**: All user queries sanitized before processing
2. **SQL Injection Prevention**: Parameterized queries throughout
3. **Term Length Limits**: Maximum 100 characters per search term
4. **Request Rate Limiting**: Per-tenant rate limits enforced
5. **Tenant Isolation**: Strict tenant-based data separation

### Authentication & Authorization
```go
// Middleware for all RAG endpoints
func RequireRAGAccess() gin.HandlerFunc {
    return func(c *gin.Context) {
        tenantID := auth.GetTenantID(c.Request.Context())
        if tenantID == uuid.Nil {
            c.JSON(401, gin.H{"error": "unauthorized"})
            c.Abort()
            return
        }
        
        // Check feature flag
        if !features.IsEnabled(c.Request.Context(), "rag_enhancements", tenantID) {
            c.JSON(403, gin.H{"error": "feature not enabled"})
            c.Abort()
            return
        }
        
        c.Next()
    }
}
```

### Error Handling Patterns
```go
// Consistent error handling across all components
type RAGError struct {
    Code    string
    Message string
    Details map[string]interface{}
    Cause   error
}

func (e *RAGError) Error() string {
    return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *RAGError) Unwrap() error {
    return e.Cause
}

// Error codes
const (
    ErrCodeInvalidQuery     = "INVALID_QUERY"
    ErrCodeSearchTimeout    = "SEARCH_TIMEOUT"
    ErrCodeProviderFailure  = "PROVIDER_FAILURE"
    ErrCodeRateLimited      = "RATE_LIMITED"
    ErrCodeTenantNotFound   = "TENANT_NOT_FOUND"
)
```

## Deployment Considerations

### Database Migrations
```bash
# Run migrations in order
migrate -path apps/rest-api/migrations/sql -database $DATABASE_URL up

# Backfill existing data
psql $DATABASE_URL << EOF
-- Update existing embeddings with tsvector
UPDATE embeddings 
SET content_tsvector = to_tsvector('english', content),
    document_length = array_length(string_to_array(content, ' '), 1)
WHERE content_tsvector IS NULL;

-- Analyze tables for query planner
ANALYZE embeddings;
EOF
```

### Performance Tuning
1. **pgvector Index Optimization**:
   ```sql
   -- Adjust IVFFlat parameters based on data size
   -- For < 1M vectors
   CREATE INDEX idx_embeddings_vector_ivfflat ON embeddings 
   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 100);
   
   -- For > 1M vectors
   CREATE INDEX idx_embeddings_vector_ivfflat ON embeddings 
   USING ivfflat (embedding vector_cosine_ops) WITH (lists = 1000);
   ```

2. **Redis Configuration**:
   ```conf
   # redis.conf
   maxmemory 2gb
   maxmemory-policy allkeys-lru
   
   # Stream settings
   stream-node-max-bytes 4096
   stream-node-max-entries 100
   ```

3. **Connection Pooling**:
   ```yaml
   database:
     max_open_conns: 100
     max_idle_conns: 25
     conn_max_lifetime: 5m
   
   redis:
     pool_size: 50
     min_idle_conns: 10
   ```

### Monitoring & Alerting
```yaml
# Prometheus alerts
groups:
  - name: rag_alerts
    rules:
      - alert: HybridSearchHighLatency
        expr: histogram_quantile(0.95, search_hybrid_duration_seconds) > 1
        for: 5m
        annotations:
          summary: "Hybrid search P95 latency > 1s"
          
      - alert: RerankingFailureRate
        expr: rate(rerank_cross_encoder_batch_failure_total[5m]) > 0.1
        for: 5m
        annotations:
          summary: "Reranking failure rate > 10%"
          
      - alert: SemanticCacheLowHitRate
        expr: rate(cache_semantic_hits_total[1h]) / rate(cache_semantic_requests_total[1h]) < 0.2
        for: 1h
        annotations:
          summary: "Semantic cache hit rate < 20%"
```

## Success Metrics

### Search Quality
- **MRR (Mean Reciprocal Rank)**: Target 20% improvement
- **Precision@5**: Target 15% improvement
- **Zero-result rate**: Target 50% reduction

### Performance
- **P95 latency**: < 200ms for hybrid search
- **Cache hit rate**: > 30% for semantic cache
- **Reranking overhead**: < 50ms

### Cost
- **Embedding API calls**: 40% reduction via caching
- **Vector DB queries**: 30% reduction via caching

## Rollout Plan

### Week 1-3: Hybrid Search
- Deploy database changes
- Enable for 10% of traffic
- Monitor performance and quality

### Week 4-5: Reranking
- Start with Cohere reranker
- A/B test reranked vs non-reranked results
- Gradually increase coverage

### Week 6-7: Query Expansion
- Enable HyDE for technical queries
- Monitor impact on recall

### Week 8: Text Chunking
- Migrate existing content
- Compare retrieval quality

### Week 9: Semantic Caching
- Pre-warm with popular queries
- Monitor cache effectiveness

## API Integration

### REST API Endpoints
```go
// File: apps/rest-api/internal/api/rag_api.go
package api

import (
    "github.com/gin-gonic/gin"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/hybrid"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/rerank"
    "github.com/developer-mesh/developer-mesh/pkg/embedding/expansion"
)

type RAGAPI struct {
    hybridSearch  *hybrid.HybridSearchService
    reranker      rerank.Reranker
    queryExpander expansion.QueryExpander
    logger        observability.Logger
}

func (api *RAGAPI) RegisterRoutes(router *gin.RouterGroup) {
    rag := router.Group("/rag")
    rag.Use(RequireRAGAccess())
    {
        // Hybrid search
        rag.POST("/search", api.hybridSearch)
        rag.POST("/search/explain", api.explainSearch)
        
        // Reranking
        rag.POST("/rerank", api.rerank)
        
        // Query expansion
        rag.POST("/expand", api.expandQuery)
        
        // Configuration
        rag.GET("/config", api.getConfig)
        rag.PUT("/config", api.updateConfig)
    }
}

// hybridSearch performs RAG-enhanced search
// @Summary Perform hybrid vector and keyword search
// @Tags rag
// @Accept json
// @Produce json
// @Param request body HybridSearchRequest true "Search request"
// @Success 200 {object} HybridSearchResponse
// @Security ApiKeyAuth
// @Router /api/v1/rag/search [post]
func (api *RAGAPI) hybridSearch(c *gin.Context) {
    var req HybridSearchRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(400, ErrorResponse{
            Code:    ErrBadRequest,
            Message: "Invalid request: " + err.Error(),
        })
        return
    }
    
    ctx := c.Request.Context()
    
    // Step 1: Query expansion (if enabled)
    expandedQueries := []string{req.Query}
    if req.EnableExpansion {
        expanded, err := api.queryExpander.Expand(ctx, req.Query, &expansion.ExpansionOptions{
            MaxExpansions: 3,
            ExpansionTypes: []expansion.ExpansionType{
                expansion.ExpansionTypeSynonym,
                expansion.ExpansionTypeDecompose,
            },
        })
        if err != nil {
            api.logger.Warn("Query expansion failed", map[string]interface{}{
                "error": err.Error(),
                "query": req.Query,
            })
        } else {
            for _, exp := range expanded.Expansions {
                expandedQueries = append(expandedQueries, exp.Text)
            }
        }
    }
    
    // Step 2: Hybrid search
    allResults := []embedding.SearchResult{}
    for _, query := range expandedQueries {
        results, err := api.hybridSearch.Search(ctx, query, &hybrid.SearchOptions{
            SearchOptions: &embedding.SearchOptions{
                Limit:          req.Limit * 2, // Get more for reranking
                TenantID:       req.TenantID,
                MetadataFilter: req.Filters,
            },
            UseHybrid:    true,
            KeywordBoost: req.KeywordBoost,
        })
        if err != nil {
            c.JSON(500, ErrorResponse{
                Code:    ErrInternalServer,
                Message: "Search failed: " + err.Error(),
            })
            return
        }
        allResults = append(allResults, results.Results...)
    }
    
    // Step 3: Deduplication
    dedupedResults := deduplicateResults(allResults)
    
    // Step 4: Reranking (if enabled)
    if req.EnableReranking && api.reranker != nil {
        reranked, err := api.reranker.Rerank(ctx, req.Query, dedupedResults, &rerank.RerankOptions{
            TopK:  req.Limit,
            Model: req.RerankModel,
        })
        if err != nil {
            api.logger.Warn("Reranking failed", map[string]interface{}{
                "error": err.Error(),
            })
            // Continue with original results
        } else {
            dedupedResults = reranked
        }
    }
    
    // Step 5: Final limit
    if len(dedupedResults) > req.Limit {
        dedupedResults = dedupedResults[:req.Limit]
    }
    
    c.JSON(200, HybridSearchResponse{
        Results:         dedupedResults,
        Total:           len(dedupedResults),
        ExpandedQueries: expandedQueries,
        Metadata: map[string]interface{}{
            "hybrid_enabled":    true,
            "expansion_enabled": req.EnableExpansion,
            "reranking_enabled": req.EnableReranking,
        },
    })
}
```

## Risk Mitigation

1. **Feature Flags**: Each enhancement behind a flag
   ```go
   if features.IsEnabled(ctx, "rag_hybrid_search", tenantID) {
       // Use hybrid search
   } else {
       // Fall back to vector-only search
   }
   ```

2. **Gradual Rollout**: Start with small traffic percentage
   ```yaml
   features:
     rag_hybrid_search:
       rollout_percentage: 10
       enabled_tenants:
         - "early_adopter_tenant_id"
   ```

3. **Fallback Mechanisms**: Automatic fallback to simpler approaches
   - If hybrid search fails → vector-only search
   - If reranking fails → original order
   - If expansion fails → original query

4. **Performance Monitoring**: Real-time alerting on latency spikes
   - P95 latency alerts
   - Error rate monitoring
   - Resource usage tracking

5. **Quality Monitoring**: Track search result quality metrics
   - Click-through rates
   - Result relevance scores
   - User feedback integration

## Implementation Checklist

- [ ] Phase 1: Hybrid Search
  - [ ] Database migrations
  - [ ] BM25 implementation
  - [ ] Fusion algorithms
  - [ ] Unit tests
  - [ ] Integration tests
  - [ ] Performance benchmarks

- [ ] Phase 2: Reranking
  - [ ] Provider integrations
  - [ ] Circuit breaker setup
  - [ ] Batch processing
  - [ ] MMR implementation
  - [ ] Testing suite

- [ ] Phase 3: Query Expansion
  - [ ] LLM integration
  - [ ] Expansion strategies
  - [ ] Caching layer
  - [ ] Performance optimization

- [ ] Phase 4: Text Chunking
  - [ ] Semantic splitter
  - [ ] Overlap strategies
  - [ ] Document awareness
  - [ ] Migration scripts

- [ ] Phase 5: Semantic Caching
  - [ ] Similarity matching
  - [ ] Cache warming
  - [ ] TTL management
  - [ ] Monitoring setup

This implementation plan provides all the technical details needed for Opus 4 to implement these RAG enhancements systematically and safely, following Developer Mesh's established patterns and best practices.