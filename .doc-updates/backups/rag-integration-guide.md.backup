# RAG + MCP Integration Guide

> **Purpose**: Guide for understanding embedding capabilities for potential RAG implementation
> **Audience**: Developers interested in building RAG features with MCP
> **Prerequisites**: Basic understanding of RAG, Go programming, AWS services
> **Status**: PARTIAL - MCP has embedding infrastructure but no complete RAG implementation

## Overview

**IMPORTANT**: Developer Mesh provides embedding infrastructure (vector storage, embedding generation) but does NOT include a complete RAG implementation. There is no RAGPipeline, RAGSystem, or any RAG-specific code in the codebase. This guide shows how to use the existing embedding capabilities as a foundation for building your own RAG systems.

### What Actually Exists
- Embedding generation via AWS Bedrock (Titan, Cohere models)
- Vector storage with PostgreSQL + pgvector
- Code chunking service (`pkg/chunking/`)
- Basic similarity search via `SearchEmbeddings` methods
- Embedding pipeline for processing content (`pkg/embedding/pipeline.go`)

### What Doesn't Exist (No Code for These)
- RAGPipeline or any RAG-specific types
- Query augmentation or rewriting
- Context retrieval and ranking algorithms
- Response generation integration
- RAG-specific agents or capabilities

## Quick Start

### 1. Using Existing Embedding Infrastructure

```go
package main

import (
    "context"
    "github.com/developer-mesh/developer-mesh/pkg/embedding"
    "github.com/developer-mesh/developer-mesh/pkg/chunking"
)

func main() {
    // NOTE: There is no pkg/services/embedding_service.go
    // You work directly with embedding providers and pipeline
    
    // Create embedding provider (actual code from pkg/embedding/)
    factory := embedding.NewProviderFactory()
    bedrockProvider := factory.CreateProvider("bedrock", map[string]interface{}{
        "model": "amazon.titan-embed-text-v1",
        "region": "us-east-1",
    })
    
    // Use the actual embedding pipeline
    pipeline, err := embedding.NewEmbeddingPipeline(
        embeddingService, // You need to implement EmbeddingService interface
        storage,          // You need to implement EmbeddingStorage interface
        chunkingService,  // Use actual chunking.ChunkingService
        contentProvider,  // Implement GitHubContentProvider if needed
        config,
    )
    
    // You would need to build ALL RAG functionality yourself
    // MCP only provides the embedding building blocks
}
```

### 2. Environment Setup

```bash
# Required AWS services
export AWS_REGION=us-east-1
export S3_BUCKET=your-rag-bucket
export BEDROCK_ENABLED=true

# ElastiCache for vector caching
export REDIS_ADDR=127.0.0.1:6379
export USE_SSH_TUNNEL_FOR_REDIS=true

# Start services
make dev-native
```

## Core Components Integration

### 1. Using the Existing Embedding Pipeline

```go
// The actual embedding pipeline that exists in MCP
pipelineConfig := &embedding.EmbeddingPipelineConfig{
    Concurrency:     4,      // Default concurrency
    BatchSize:       10,     // Default batch size
    IncludeComments: true,
    EnrichMetadata:  true,
}

// Create embedding pipeline (this exists)
pipeline := embedding.NewEmbeddingPipeline(
    embeddingService,
    storage,
    chunkingService,
    contentProvider,
    pipelineConfig,
)

// Example: Building RAG on top (you need to implement this)
type RAGSystem struct {
    pipeline        *embedding.DefaultEmbeddingPipeline
    embeddingRepo   repository.EmbeddingRepository
    contextBuilder  ContextBuilder  // You implement this
    responseGen     ResponseGenerator // You implement this
}

// This is what you would need to build
func (r *RAGSystem) Query(ctx context.Context, query string) (string, error) {
    // 1. Generate embedding for query
    queryEmbedding, err := r.pipeline.ProcessContent(ctx, query, "query", "user-query")
    
    // 2. Search similar embeddings
    similar, err := r.embeddingRepo.SearchSimilar(ctx, queryEmbedding, 10)
    
    // 3. Build context from retrieved documents
    context := r.contextBuilder.Build(similar)
    
    // 4. Generate response
    return r.responseGen.Generate(query, context)
}
```

### 2. Document Processing with Existing Tools

```go
// Using the actual MCP embedding pipeline
func ProcessDocumentsForRAG(ctx context.Context, pipeline *embedding.DefaultEmbeddingPipeline, docs []Document) error {
    for _, doc := range docs {
        // 1. Use MCP's chunking service
        chunkingService := chunking.NewChunkingService(logger, tracer)
        chunks, err := chunkingService.ChunkCode(ctx, doc.Content, doc.Path)
        if err != nil {
            return fmt.Errorf("chunking failed: %w", err)
        }
        
        // 2. Process each chunk with the embedding pipeline
        for i, chunk := range chunks {
            // The actual pipeline processes content one at a time
            err := pipeline.ProcessContent(
                ctx,
                chunk.Content,
                "code_chunk",
                fmt.Sprintf("%s-chunk-%d", doc.ID, i),
            )
            if err != nil {
                return fmt.Errorf("processing chunk %d failed: %w", i, err)
            }
        }
    }
    
    return nil
}
```

### 3. Query Processing with Existing Infrastructure

```go
// Example: Using MCP's actual vector repository for search
import (
    "github.com/developer-mesh/developer-mesh/pkg/repository/vector"
)

func QueryWithMCPSearch(ctx context.Context, vectorRepo vector.Repository, query string) ([]*vector.Embedding, error) {
    // 1. Generate query embedding (you need to implement this)
    queryVector := generateQueryEmbedding(query) // Your implementation
    
    // 2. Use actual SearchEmbeddings method from vector repository
    // NOTE: The search is simplified - no actual vector similarity!
    results, err := vectorRepo.SearchEmbeddings(
        ctx,
        queryVector,      // Query vector
        "context-id",     // Context ID
        "titan-embed-v1", // Model ID
        10,               // Limit
        0.7,              // Threshold (not actually used in current implementation)
    )
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }
    
    // 3. IMPORTANT: The current SearchEmbeddings doesn't do real vector search!
    // It just returns embeddings ordered by ID, not similarity
    // You need to implement actual vector similarity search
    
    return results, nil
}
```

## Building RAG with MCP Components

### 1. Using MCP's Agent System

```go
// Agents in MCP don't have built-in RAG capabilities
// You would need to implement RAG logic in your agent

// Example: Agent that could be extended for RAG
type RAGCapableAgent struct {
    // Standard MCP agent fields
    conn         *websocket.Conn
    agentID      string
    capabilities []string
    
    // RAG extensions you would add
    embeddingService embedding.EmbeddingService
    searchService    search.Service
    contextBuilder   ContextBuilder // You implement
}

// Register with standard MCP capabilities
func (a *RAGCapableAgent) Register(ctx context.Context) error {
    // Use standard agent registration
    msg := map[string]interface{}{
        "type":         "agent.register",
        "name":         "rag-agent",
        "capabilities": []string{"document_search", "code_analysis"},
    }
    // Send via WebSocket...
    return nil
}
```

### 2. Implementing RAG Logic in Your Agent

```go
// Example: How to handle RAG tasks in your agent
func (a *RAGCapableAgent) HandleTask(ctx context.Context, task *models.Task) error {
    // Check if this is a RAG-related task
    if task.Type != "document_search" {
        return a.handleNormalTask(ctx, task)
    }
    
    // Extract query from task
    query, ok := task.Parameters["query"].(string)
    if !ok {
        return fmt.Errorf("missing query parameter")
    }
    
    // Generate embedding using your provider
    queryVector := a.generateQueryEmbedding(query) // You implement
    
    // Search using vector repository
    results, err := a.vectorRepo.SearchEmbeddings(
        ctx,
        queryVector,
        task.ContextID,
        "model-id",
        10,
        0.7,
    )
    if err != nil {
        return err
    }
    
    // Build context from search results (you implement)
    context := a.contextBuilder.Build(results)
    
    // Generate response (you implement)
    response := a.generateResponse(query, context)
    
    // Send result back via MCP protocol
    return a.sendTaskResult(ctx, task.ID, response)
}
```

## Using MCP's AWS Bedrock Integration

### 1. Available Bedrock Embedding Models

```go
// MCP supports these Bedrock embedding models
// (defined in pkg/embedding/bedrock.go)

// Amazon Titan models
const (
    ModelTitanEmbedTextV1 = "amazon.titan-embed-text-v1"
    ModelTitanEmbedTextV2 = "amazon.titan-embed-text-v2:0"
)

// Cohere models
const (
    ModelCohereEmbedEnglishV3 = "cohere.embed-english-v3"
    ModelCohereEmbedMultilingualV3 = "cohere.embed-multilingual-v3"
)

// Use the existing factory to create providers
factory := embedding.NewProviderFactory()
provider := factory.CreateProvider("bedrock", map[string]interface{}{
    "model": "amazon.titan-embed-text-v1",
    "region": "us-east-1",
})
```

### 2. Cost Tracking with MCP

```go
// MCP tracks embedding costs in the provider implementations
// The costs are defined in pkg/embedding/bedrock.go

// Example: Using Bedrock provider with cost tracking
bedrockProvider := &embedding.BedrockProvider{
    client: bedrockClient,
    model:  "amazon.titan-embed-text-v1",
}

// Generate embedding - provider tracks costs
result, err := bedrockProvider.GenerateEmbedding(ctx, content)
if err != nil {
    return err
}

// Access cost information from the result
if result.Cost > 0 {
    logger.Info("Embedding cost", map[string]interface{}{
        "model": result.Model,
        "cost":  result.Cost,
        "tokens": result.TokenCount,
    })
}

// Cost per model (from bedrock.go):
// Titan v1: $0.0001 per 1000 tokens
// Titan v2: $0.00002 per 1000 tokens  
// Cohere English: $0.0001 per 1000 tokens
// Cohere Multilingual: $0.0001 per 1000 tokens
```

## Using MCP's Vector Storage

### 1. Existing Schema

```sql
-- MCP already has embeddings table (see migrations/000004_embeddings.up.sql)
-- You don't need to create your own - use the existing one:

CREATE TABLE IF NOT EXISTS embeddings (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL,
    session_id UUID,
    content_id VARCHAR(255) NOT NULL,
    content_type VARCHAR(50) NOT NULL,
    content TEXT NOT NULL,
    embedding vector,  -- Dynamic dimension
    model VARCHAR(100) NOT NULL,
    metadata JSONB,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    -- Foreign keys and indexes...
);
```

### 2. Using MCP's Vector Repository

```go
// Use the actual vector repository
// (defined in pkg/repository/vector/repository.go)

import "github.com/developer-mesh/developer-mesh/pkg/repository/vector"

// The actual Repository interface in MCP
type Repository interface {
    // Standard CRUD operations
    Create(ctx context.Context, embedding *Embedding) error
    Get(ctx context.Context, id string) (*Embedding, error)
    List(ctx context.Context, filter Filter) ([]*Embedding, error)
    Update(ctx context.Context, embedding *Embedding) error
    Delete(ctx context.Context, id string) error
    
    // Vector-specific operations
    StoreEmbedding(ctx context.Context, embedding *Embedding) error
    SearchEmbeddings(ctx context.Context, queryVector []float32, contextID string, modelID string, limit int, threshold float64) ([]*Embedding, error)
    SearchEmbeddings_Legacy(ctx context.Context, queryVector []float32, contextID string, limit int) ([]*Embedding, error)
    GetContextEmbeddings(ctx context.Context, contextID string) ([]*Embedding, error)
    DeleteContextEmbeddings(ctx context.Context, contextID string) error
    GetEmbeddingsByModel(ctx context.Context, contextID string, modelID string) ([]*Embedding, error)
    GetSupportedModels(ctx context.Context) ([]string, error)
}

// Example: Using the repository for RAG
func SearchForRAG(ctx context.Context, repo vector.Repository, queryVector []float32) ([]*vector.Embedding, error) {
    // WARNING: Current implementation doesn't do actual vector similarity search!
    // It just returns embeddings ordered by ID
    results, err := repo.SearchEmbeddings(
        ctx,
        queryVector,
        "context-id",
        "model-id",
        10,    // limit
        0.7,   // threshold (not used in current implementation)
    )
    
    if err != nil {
        return nil, fmt.Errorf("search failed: %w", err)
    }
    
    // You need to implement actual similarity calculation
    return results, nil
}
```

## Using MCP's Caching Infrastructure

### 1. Redis Cache for Embeddings

```go
// MCP provides Redis caching via cache package
// Use it for embedding caching in RAG

import "github.com/developer-mesh/developer-mesh/pkg/cache"

// Create cache client
cacheClient := cache.NewRedisCache(redisClient, logger)

// Cache embeddings
key := fmt.Sprintf("embedding:%s", contentHash)
ttl := 24 * time.Hour

// Store embedding
err := cacheClient.Set(ctx, key, embedding, ttl)

// Retrieve embedding
var cachedEmbedding []float32
err := cacheClient.Get(ctx, key, &cachedEmbedding)
```

### 2. MCP's Embedding Pipeline with Caching

```go
// NOTE: There is no services.NewEmbeddingService in MCP
// You need to implement caching yourself using the cache package

// The embedding pipeline doesn't include built-in caching
// You can add caching around your embedding generation:

func CachedEmbeddingGeneration(ctx context.Context, cache cache.Cache, provider embedding.Provider, content string) ([]float32, error) {
    // Create cache key
    key := fmt.Sprintf("embedding:%s", hash(content))
    
    // Check cache
    var cachedEmbedding []float32
    err := cache.Get(ctx, key, &cachedEmbedding)
    if err == nil {
        return cachedEmbedding, nil
    }
    
    // Generate if not cached
    embedding, err := provider.GenerateEmbedding(ctx, content)
    if err != nil {
        return nil, err
    }
    
    // Cache for future use
    cache.Set(ctx, key, embedding.Vector, 24*time.Hour)
    
    return embedding.Vector, nil
}
```

## Integrating with MCP's WebSocket

### 1. Handling RAG Tasks via WebSocket

```go
// Example: Extending your agent to handle RAG tasks
func (a *RAGCapableAgent) handleMessages(ctx context.Context) {
    for {
        var msg map[string]interface{}
        err := wsjson.Read(ctx, a.conn, &msg)
        if err != nil {
            return
        }
        
        msgType, _ := msg["type"].(string)
        switch msgType {
        case "task.execute":
            // Check if this is a RAG task
            if taskType, _ := msg["task_type"].(string); taskType == "rag_query" {
                a.handleRAGTask(ctx, msg)
            }
        }
    }
}

func (a *RAGCapableAgent) handleRAGTask(ctx context.Context, msg map[string]interface{}) {
    taskID := msg["task_id"].(string)
    query := msg["parameters"].(map[string]interface{})["query"].(string)
    
    // Generate embedding using your provider
    queryVector, err := a.embeddingProvider.GenerateEmbedding(ctx, query)
    if err != nil {
        a.sendError(ctx, taskID, err)
        return
    }
    
    // Search using vector repository
    results, err := a.vectorRepo.SearchEmbeddings(ctx, queryVector.Vector, "context-id", "model-id", 10, 0.7)
    if err != nil {
        a.sendError(ctx, taskID, err)
        return
    }
    
    // Build response from results (you implement)
    response := a.processRAGQuery(ctx, query, results)
    
    // Send result via MCP protocol
    result := map[string]interface{}{
        "type":    "task.result",
        "task_id": taskID,
        "result":  response,
    }
    wsjson.Write(ctx, a.conn, result)
}
```

### 2. Using MCP's Session Management

```go
// MCP provides session tracking for agents
// You can extend this for RAG context management

type RAGSession struct {
    sessionID   string
    agentID     string
    queryHistory []string
    context     []string
}

// Store RAG context in MCP's context service
func (a *RAGCapableAgent) storeRAGContext(ctx context.Context, sessionID string, context string) error {
    // Use MCP's context storage
    contextItem := &models.Context{
        AgentID:     a.agentID,
        SessionID:   sessionID,
        Type:        "rag_context",
        Content:     context,
        Metadata: map[string]interface{}{
            "timestamp": time.Now(),
            "source":    "rag_query",
        },
    }
    
    return a.contextService.Create(ctx, contextItem)
}
```

## Performance Optimization with MCP

### 1. Using MCP's Batch Processing

```go
// MCP's embedding pipeline supports concurrent processing
pipelineConfig := &embedding.EmbeddingPipelineConfig{
    Concurrency: 10,  // Process 10 items concurrently
    BatchSize:   50,  // Batch size for processing
}

// Process multiple documents efficiently
func ProcessDocumentsConcurrently(ctx context.Context, pipeline *embedding.DefaultEmbeddingPipeline, docs []Document) error {
    // Use goroutines with MCP's pipeline
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 10) // Limit concurrency
    
    for _, doc := range docs {
        wg.Add(1)
        semaphore <- struct{}{}
        
        go func(d Document) {
            defer wg.Done()
            defer func() { <-semaphore }()
            
            // Process with MCP's pipeline
            err := pipeline.ProcessContent(
                ctx,
                d.Content,
                "document",
                d.ID,
            )
            if err != nil {
                // Handle error
            }
        }(doc)
    }
    
    wg.Wait()
    return nil
}
```

### 2. Leveraging MCP's Caching

```go
// MCP automatically caches embeddings
// You can also precompute and store in the database

func PrecomputeCommonEmbeddings(ctx context.Context, embeddingService embedding.EmbeddingService, commonQueries []string) error {
    for _, query := range commonQueries {
        // Generate and cache embedding
        _, err := embeddingService.GenerateEmbedding(
            ctx,
            query,
            "precomputed_query",
            fmt.Sprintf("precomputed-%s", hash(query)),
        )
        if err != nil {
            return fmt.Errorf("failed to precompute %s: %w", query, err)
        }
    }
    return nil
}
```

## Monitoring with MCP's Observability

### 1. Using MCP's Metrics

```go
// MCP provides built-in metrics for embeddings
// Add your RAG-specific metrics

import "github.com/developer-mesh/developer-mesh/pkg/observability"

func TrackRAGMetrics(metrics observability.MetricsClient) {
    // Use MCP's metrics client
    metrics.RecordHistogram(
        "rag.query.latency",
        time.Since(start).Seconds(),
        map[string]string{
            "query_type": "semantic_search",
            "model":      "titan-embed-text-v1",
        },
    )
    
    metrics.RecordGauge(
        "rag.retrieval.relevance",
        relevanceScore,
        map[string]string{
            "content_type": "code_chunk",
        },
    )
}
```

### 2. Using MCP's Tracing

```go
// MCP includes OpenTelemetry tracing
import "github.com/developer-mesh/developer-mesh/pkg/observability"

func RAGQueryWithTracing(ctx context.Context, tracer observability.TracingClient, query string) error {
    // Start span using MCP's tracer
    ctx, span := tracer.StartSpan(ctx, "rag.query")
    defer span.End()
    
    span.SetAttributes(map[string]interface{}{
        "query.text":   query,
        "query.length": len(query),
    })
    
    // Trace embedding generation
    _, embSpan := tracer.StartSpan(ctx, "rag.embedding.generate")
    // Generate embedding...
    embSpan.End()
    
    // Trace search
    _, searchSpan := tracer.StartSpan(ctx, "rag.vector.search")
    // Perform search...
    searchSpan.End()
    
    return nil
}
```

## Testing RAG with MCP Components

### 1. Integration Tests

```go
// Test with MCP's actual components
func TestRAGWithMCP(t *testing.T) {
    // Set up test database
    db := testutil.SetupTestDB(t)
    
    // Create vector repository
    vectorRepo := vector.NewRepository(db)
    
    // Create embedding provider
    factory := embedding.NewProviderFactory()
    provider := factory.CreateProvider("bedrock", map[string]interface{}{
        "model": "amazon.titan-embed-text-v1",
    })
    
    // Test embedding generation
    t.Run("GenerateEmbedding", func(t *testing.T) {
        result, err := provider.GenerateEmbedding(
            context.Background(),
            "test query",
        )
        
        require.NoError(t, err)
        assert.NotNil(t, result)
        assert.NotEmpty(t, result.Vector)
    })
    
    // Test search (remember: no actual similarity search implemented!)
    t.Run("SearchEmbeddings", func(t *testing.T) {
        results, err := vectorRepo.SearchEmbeddings(
            context.Background(),
            testVector,
            "context-id",
            "model-id",
            5,
            0.5,
        )
        
        require.NoError(t, err)
        // Results are ordered by ID, not similarity!
    })
}
```

### 2. Benchmarking MCP Components

```go
// Benchmark MCP's embedding provider
func BenchmarkMCPEmbedding(b *testing.B) {
    factory := embedding.NewProviderFactory()
    provider := factory.CreateProvider("bedrock", map[string]interface{}{
        "model": "amazon.titan-embed-text-v1",
    })
    
    queries := []string{"test query 1", "test query 2", "test query 3"}
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        query := queries[i%len(queries)]
        _, err := provider.GenerateEmbedding(
            context.Background(),
            query,
        )
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

## Common RAG Patterns with MCP

### 1. Hybrid Search Implementation

```go
// Example: Combining MCP's vector search with text search
func HybridSearchWithMCP(ctx context.Context, embeddingService embedding.EmbeddingService, db *sql.DB, query string) ([]Result, error) {
    // 1. Vector search using MCP
    embedding, _ := embeddingService.GenerateEmbedding(ctx, query, "query", "search")
    vectorResults, _ := searchService.SearchEmbeddings(ctx, embedding.Vector, options)
    
    // 2. Text search using PostgreSQL full-text search
    textResults, _ := performTextSearch(db, query)
    
    // 3. Merge results (you implement)
    return mergeAndRankResults(vectorResults, textResults), nil
}
```

### 2. Building Context from MCP Results

```go
// Build context from MCP search results
func BuildRAGContext(results []*models.EmbeddingVector, maxLength int) string {
    // Sort by similarity (MCP includes this in results)
    var context strings.Builder
    currentLength := 0
    
    for _, result := range results {
        content := result.Content
        if currentLength+len(content) > maxLength {
            // Truncate if needed
            remaining := maxLength - currentLength
            if remaining > 100 { // Only add if meaningful
                context.WriteString(content[:remaining])
            }
            break
        }
        
        context.WriteString(content)
        context.WriteString("\n\n---\n\n")
        currentLength += len(content) + 6
    }
    
    return context.String()
}
```

## Troubleshooting RAG with MCP

### Common Issues

1. **Slow Embedding Generation**
   - Implement your own caching layer (MCP doesn't cache automatically)
   - Increase concurrency in pipeline config
   - Check AWS Bedrock quotas and limits

2. **Poor Retrieval Quality**
   - Current SearchEmbeddings doesn't do actual vector similarity search!
   - You need to implement proper pgvector queries with cosine similarity
   - Use MCP's code chunking service for better chunks
   - Add metadata filters when querying

3. **High AWS Bedrock Costs**
   - MCP tracks costs per embedding in the embedding metadata
   - Use appropriate models (Titan is cheaper than Cohere)
   - Implement Redis caching yourself

4. **WebSocket Agent Issues**
   - Must include `mcp.v1` subprotocol
   - Implement reconnection (see test agent)
   - Handle binary protocol transitions

5. **No Vector Similarity Search**
   - Current `SearchEmbeddings` just returns embeddings by ID order
   - You must implement actual vector similarity using pgvector extensions
   - Example: `ORDER BY embedding <-> $1` for cosine distance

## Next Steps

1. Study MCP's embedding implementation: `/pkg/embedding/`
2. Review search functionality: `/pkg/embedding/search.go`
3. Check test implementations: `/test/e2e/agent/`
4. See [Embedding API Reference](../api-reference/embedding-api-reference.md)

## Resources

### MCP Components for RAG
- Embedding Service: `/pkg/services/embedding_service.go`
- Vector Storage: `/pkg/repository/postgres/embedding_repository.go`
- Search Implementation: `/pkg/embedding/search.go`
- Chunking Service: `/pkg/chunking/`

### External Resources
- [pgvector Documentation](https://github.com/pgvector/pgvector)
- [AWS Bedrock Embeddings](https://docs.aws.amazon.com/bedrock/latest/userguide/embeddings.html)
- [RAG Best Practices](https://www.pinecone.io/learn/retrieval-augmented-generation/)

## Summary

Developer Mesh provides basic building blocks for RAG (embeddings, vector storage) but NOT a complete RAG implementation. Key limitations and clarifications:

**What MCP Actually Provides:**
1. Embedding generation via AWS Bedrock providers
2. PostgreSQL with pgvector extension for storage
3. Basic code chunking service
4. Simple embedding storage/retrieval (no similarity search)

**What You Must Implement Yourself:**
1. Actual vector similarity search (current SearchEmbeddings just returns by ID)
2. Query augmentation and rewriting
3. Context retrieval and ranking algorithms
4. Response generation and LLM integration
5. Caching layer (no automatic caching)
6. Complete RAG pipeline and orchestration

**Critical Notes:**
- There is NO `pkg/services/embedding_service.go` - work directly with providers
- SearchEmbeddings does NOT perform vector similarity search
- No RAGPipeline, RAGSystem, or any RAG-specific types exist
- You must implement pgvector similarity queries yourself
- All RAG logic must be built from scratch using MCP's basic components