# Reranking Package

This package implements advanced reranking capabilities for search results, part of Phase 2 of the RAG enhancement plan.

## Features

### Cross-Encoder Reranking
- Uses transformer models to re-score search results
- Batch processing for efficiency
- Circuit breaker pattern for resilience
- Retry logic with exponential backoff
- Graceful degradation on failures

### MMR (Maximal Marginal Relevance) Reranking
- Balances relevance and diversity in search results
- Configurable lambda parameter (0-1) to control diversity
- Caches embeddings when available
- Supports custom embedding services

### Multi-Stage Reranking
- Combines multiple rerankers in sequence
- Each stage can have different top-k and weight settings
- Useful for combining relevance and diversity

## Usage

### Basic Cross-Encoder Reranking
```go
provider := providers.NewCohereProvider(apiKey)
config := &CrossEncoderConfig{
    Model:     "rerank-english-v3.0",
    BatchSize: 10,
}
reranker, err := NewCrossEncoderReranker(provider, config, logger, metrics)

results, err := reranker.Rerank(ctx, query, searchResults, &RerankOptions{
    TopK: 20,
})
```

### MMR for Diversity
```go
reranker, err := NewMMRReranker(0.7, embeddingService, logger)

results, err := reranker.Rerank(ctx, query, searchResults, &RerankOptions{
    TopK: 10,
})
```

### Multi-Stage Pipeline
```go
multiStage := NewMultiStageReranker([]RerankStage{
    {
        Reranker: crossEncoder,
        TopK:     50,
        Weight:   0.8,
    },
    {
        Reranker: mmrReranker,
        TopK:     20,
        Weight:   0.2,
    },
}, logger)
```

## Configuration

### Cross-Encoder Config
- `Model`: The reranking model to use
- `BatchSize`: Number of documents per batch (default: 10)
- `MaxConcurrency`: Max concurrent batches (default: 3)
- `TimeoutPerBatch`: Timeout for each batch (default: 5s)

### MMR Config
- `Lambda`: Balance between relevance and diversity (0-1)
  - 0 = Maximum diversity
  - 1 = Maximum relevance
  - 0.5-0.7 = Good balance

## Integration

The reranking package integrates with:
- `UnifiedSearchService` via the `UseReranking` option
- REST API search endpoints
- Multiple reranking providers (Cohere, OpenAI, etc.)

## Testing

```bash
# Run unit tests
go test ./pkg/embedding/rerank -short

# Run integration tests
go test ./pkg/embedding/rerank
```

## Performance Considerations

- Batch processing reduces API calls
- Circuit breakers prevent cascading failures
- Semaphores limit concurrent operations
- Graceful degradation returns original results on failure

## Error Handling

All rerankers implement graceful degradation:
- On provider errors: Return original results
- On timeout: Return partial results
- On circuit breaker open: Skip reranking

## Metrics

The package tracks:
- `rerank.cross_encoder.duration`: Processing time
- `rerank.cross_encoder.batch_failure`: Failed batches
- `rerank.mmr.duration`: MMR processing time
- `rerank.multistage.duration`: Pipeline duration