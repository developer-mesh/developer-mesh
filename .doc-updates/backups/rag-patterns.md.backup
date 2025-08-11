# RAG Implementation Patterns

> **Purpose**: Common patterns and best practices for RAG implementations with MCP
> **Audience**: Developers and architects implementing RAG systems
> **Scope**: Design patterns, implementation strategies, optimization techniques

## Overview

This guide documents proven patterns for implementing Retrieval-Augmented Generation (RAG) systems using the Developer Mesh platform. These patterns address common challenges and leverage MCP's multi-agent orchestration capabilities.

## Core RAG Patterns

### 1. Basic Query-Response Pattern

The fundamental RAG pattern for simple question-answering.

```go
// Pattern: Simple RAG Pipeline
type SimpleRAG struct {
    embedder    embedding.Service
    vectorStore vector.Store
    generator   llm.Service
}

func (r *SimpleRAG) Query(ctx context.Context, question string) (string, error) {
    // 1. Embed query
    queryVec := r.embedder.Embed(ctx, question)
    
    // 2. Retrieve context
    docs := r.vectorStore.Search(ctx, queryVec, 5)
    
    // 3. Generate response
    prompt := r.buildPrompt(question, docs)
    return r.generator.Generate(ctx, prompt)
}
```

**When to use:**
- Simple Q&A systems
- Documentation search
- Basic chatbots

**Limitations:**
- Single embedding model
- No query understanding
- Limited context optimization

### 2. Hybrid Search Pattern

Combines vector similarity with keyword matching for better retrieval.

```go
// Pattern: Hybrid Search RAG
type HybridSearchRAG struct {
    vectorSearch  vector.Store
    keywordSearch search.Engine
    reranker      ranking.Service
}

func (h *HybridSearchRAG) Search(ctx context.Context, query string) ([]Document, error) {
    // Parallel search
    var wg sync.WaitGroup
    vectorResults := make(chan []Document)
    keywordResults := make(chan []Document)
    
    // Vector search
    wg.Add(1)
    go func() {
        defer wg.Done()
        embedding := h.embedder.Embed(ctx, query)
        results := h.vectorSearch.Search(ctx, embedding, 20)
        vectorResults <- results
    }()
    
    // Keyword search
    wg.Add(1)
    go func() {
        defer wg.Done()
        results := h.keywordSearch.Search(ctx, query, 20)
        keywordResults <- results
    }()
    
    wg.Wait()
    
    // Merge and rerank
    merged := h.mergeResults(<-vectorResults, <-keywordResults)
    return h.reranker.Rerank(ctx, query, merged, 10)
}
```

**Benefits:**
- Better recall for specific terms
- Handles acronyms and technical terms
- More robust retrieval

### 3. Multi-Stage Retrieval Pattern

Progressive refinement for complex queries.

```go
// Pattern: Multi-Stage Retrieval
type MultiStageRAG struct {
    stages []RetrievalStage
}

type RetrievalStage struct {
    Name      string
    Retriever Retriever
    Filter    FilterFunc
    TopK      int
}

func (m *MultiStageRAG) Retrieve(ctx context.Context, query string) ([]Document, error) {
    var documents []Document
    
    for _, stage := range m.stages {
        // Apply stage-specific retrieval
        stageResults := stage.Retriever.Retrieve(ctx, query, stage.TopK)
        
        // Apply filters
        if stage.Filter != nil {
            stageResults = stage.Filter(stageResults, query)
        }
        
        // Accumulate results
        documents = append(documents, stageResults...)
    }
    
    // Deduplicate and rank
    return m.deduplicateAndRank(documents)
}

// Example configuration
rag := &MultiStageRAG{
    stages: []RetrievalStage{
        {
            Name:      "broad-context",
            Retriever: semanticRetriever,
            TopK:      50,
            Filter:    recencyFilter,
        },
        {
            Name:      "precise-match",
            Retriever: bm25Retriever,
            TopK:      20,
            Filter:    relevanceFilter,
        },
        {
            Name:      "expert-knowledge",
            Retriever: graphRetriever,
            TopK:      10,
            Filter:    authorityFilter,
        },
    },
}
```

### 4. Query Decomposition Pattern

Break complex queries into sub-queries for better results.

```go
// Pattern: Query Decomposition
type QueryDecomposer struct {
    analyzer  QueryAnalyzer
    planner   QueryPlanner
    executor  QueryExecutor
}

func (q *QueryDecomposer) Process(ctx context.Context, complexQuery string) (*Response, error) {
    // 1. Analyze query complexity
    analysis := q.analyzer.Analyze(complexQuery)
    
    if analysis.Complexity == "simple" {
        return q.executor.ExecuteSimple(ctx, complexQuery)
    }
    
    // 2. Decompose into sub-queries
    subQueries := q.planner.Decompose(complexQuery, analysis)
    
    // 3. Execute sub-queries in parallel
    results := make(chan SubQueryResult, len(subQueries))
    for _, subQuery := range subQueries {
        go func(sq SubQuery) {
            result := q.executor.Execute(ctx, sq)
            results <- result
        }(subQuery)
    }
    
    // 4. Aggregate results
    var subResults []SubQueryResult
    for i := 0; i < len(subQueries); i++ {
        subResults = append(subResults, <-results)
    }
    
    // 5. Synthesize final response
    return q.synthesize(complexQuery, subResults)
}

// Example decomposition
// Query: "Compare the performance of our API endpoints over the last month and identify bottlenecks"
// Decomposed:
// 1. "What are our API endpoints?"
// 2. "What are the performance metrics for each endpoint in the last month?"
// 3. "What constitutes a bottleneck in API performance?"
// 4. "Which endpoints show signs of bottlenecks?"
```

### 5. Conversational RAG Pattern

Maintains context across multiple interactions.

```go
// Pattern: Conversational RAG with Memory
type ConversationalRAG struct {
    memory      ConversationMemory
    rag         RAGPipeline
    summarizer  Summarizer
}

type ConversationMemory struct {
    shortTerm []Message      // Recent messages
    longTerm  []Summary      // Summarized conversations
    context   CurrentContext // Active context
}

func (c *ConversationalRAG) Chat(ctx context.Context, message string) (*Response, error) {
    // 1. Update conversation context
    c.memory.AddMessage(UserMessage{Content: message})
    
    // 2. Build enhanced query with context
    enhancedQuery := c.buildContextualQuery(message, c.memory)
    
    // 3. Retrieve with conversation awareness
    retrievalContext := c.rag.RetrieveWithContext(ctx, enhancedQuery, c.memory.context)
    
    // 4. Generate response
    response := c.rag.Generate(ctx, message, retrievalContext, c.memory.GetRecentHistory())
    
    // 5. Update memory
    c.memory.AddMessage(AssistantMessage{Content: response})
    
    // 6. Periodically summarize
    if c.memory.ShouldSummarize() {
        summary := c.summarizer.Summarize(c.memory.shortTerm)
        c.memory.AddSummary(summary)
    }
    
    return response, nil
}
```

### 6. Multi-Modal RAG Pattern

Handles text, code, images, and other data types.

```go
// Pattern: Multi-Modal RAG
type MultiModalRAG struct {
    textProcessor  TextProcessor
    codeProcessor  CodeProcessor
    imageProcessor ImageProcessor
    audioProcessor AudioProcessor
    coordinator    ModalityCoordinator
}

func (m *MultiModalRAG) Process(ctx context.Context, input MultiModalInput) (*Response, error) {
    // 1. Identify modalities
    modalities := m.identifyModalities(input)
    
    // 2. Process each modality in parallel
    results := make(map[string]ModalityResult)
    var wg sync.WaitGroup
    
    for _, modality := range modalities {
        wg.Add(1)
        go func(mod Modality) {
            defer wg.Done()
            
            switch mod.Type {
            case "text":
                results["text"] = m.textProcessor.Process(ctx, mod.Data)
            case "code":
                results["code"] = m.codeProcessor.Process(ctx, mod.Data)
            case "image":
                results["image"] = m.imageProcessor.Process(ctx, mod.Data)
            case "audio":
                results["audio"] = m.audioProcessor.Process(ctx, mod.Data)
            }
        }(modality)
    }
    
    wg.Wait()
    
    // 3. Coordinate results
    return m.coordinator.Synthesize(ctx, results)
}
```

### 7. Adaptive RAG Pattern

Adjusts retrieval strategy based on query characteristics.

```go
// Pattern: Adaptive RAG
type AdaptiveRAG struct {
    classifier   QueryClassifier
    strategies   map[string]RAGStrategy
    metrics      PerformanceMetrics
}

type RAGStrategy interface {
    Execute(ctx context.Context, query string) (*Response, error)
}

func (a *AdaptiveRAG) Query(ctx context.Context, query string) (*Response, error) {
    // 1. Classify query
    queryType := a.classifier.Classify(query)
    
    // 2. Select strategy
    strategy, exists := a.strategies[queryType]
    if !exists {
        strategy = a.strategies["default"]
    }
    
    // 3. Execute with monitoring
    start := time.Now()
    response, err := strategy.Execute(ctx, query)
    
    // 4. Track performance
    a.metrics.Record(queryType, time.Since(start), err == nil)
    
    // 5. Adapt strategies based on performance
    if a.metrics.ShouldAdapt(queryType) {
        a.adaptStrategy(queryType)
    }
    
    return response, err
}

// Strategy examples
strategies := map[string]RAGStrategy{
    "factual":    &DenseRetrievalStrategy{topK: 5},
    "analytical": &ChainOfThoughtStrategy{steps: 3},
    "creative":   &DiverseRetrievalStrategy{temperature: 0.8},
    "technical":  &HybridSearchStrategy{codeWeight: 0.7},
}
```

## Advanced Patterns

### 8. Agent-Specialized RAG Pattern

Different agents handle different aspects of RAG.

```go
// Pattern: Agent-Specialized RAG
type AgentRAG struct {
    router      AgentRouter
    agents      map[string]SpecializedAgent
    coordinator ResultCoordinator
}

type SpecializedAgent interface {
    Capabilities() []string
    Process(ctx context.Context, task Task) (*Result, error)
}

// Specialized agents
type CodeAnalysisAgent struct {
    codeEmbedder embedding.CodeEmbedder
    astAnalyzer  ast.Analyzer
}

type DocumentationAgent struct {
    docEmbedder embedding.DocEmbedder
    linker      reference.Linker
}

type MetricsAgent struct {
    timeSeriesDB tsdb.Client
    analyzer     metrics.Analyzer
}

func (a *AgentRAG) Process(ctx context.Context, query Query) (*Response, error) {
    // 1. Route to specialized agents
    tasks := a.router.CreateTasks(query)
    
    // 2. Execute in parallel
    results := make(chan AgentResult, len(tasks))
    for agentID, task := range tasks {
        agent := a.agents[agentID]
        go func(a SpecializedAgent, t Task) {
            result, _ := a.Process(ctx, t)
            results <- AgentResult{AgentID: agentID, Result: result}
        }(agent, task)
    }
    
    // 3. Coordinate results
    var agentResults []AgentResult
    for i := 0; i < len(tasks); i++ {
        agentResults = append(agentResults, <-results)
    }
    
    return a.coordinator.Synthesize(query, agentResults)
}
```

### 9. Federated RAG Pattern

Queries multiple independent RAG systems.

```go
// Pattern: Federated RAG
type FederatedRAG struct {
    endpoints []RAGEndpoint
    aggregator ResultAggregator
    cache     DistributedCache
}

type RAGEndpoint struct {
    Name         string
    URL          string
    Specialization string
    Weight       float64
}

func (f *FederatedRAG) Query(ctx context.Context, query string) (*Response, error) {
    // 1. Check distributed cache
    if cached := f.cache.Get(ctx, query); cached != nil {
        return cached.(*Response), nil
    }
    
    // 2. Query all endpoints in parallel
    results := make(chan EndpointResult, len(f.endpoints))
    
    for _, endpoint := range f.endpoints {
        go func(ep RAGEndpoint) {
            result := f.queryEndpoint(ctx, ep, query)
            results <- EndpointResult{
                Endpoint: ep,
                Result:   result,
            }
        }(endpoint)
    }
    
    // 3. Collect and aggregate
    var endpointResults []EndpointResult
    for i := 0; i < len(f.endpoints); i++ {
        endpointResults = append(endpointResults, <-results)
    }
    
    // 4. Weighted aggregation
    response := f.aggregator.Aggregate(endpointResults)
    
    // 5. Cache result
    f.cache.Set(ctx, query, response, 1*time.Hour)
    
    return response, nil
}
```

### 10. Self-Improving RAG Pattern

Learns from user feedback to improve retrieval.

```go
// Pattern: Self-Improving RAG
type SelfImprovingRAG struct {
    rag          RAGPipeline
    feedback     FeedbackCollector
    reranker     LearnedReranker
    finetuner    ModelFinetuner
}

func (s *SelfImprovingRAG) QueryWithFeedback(ctx context.Context, query string) (*Response, error) {
    // 1. Standard RAG query
    candidates := s.rag.Retrieve(ctx, query, 20)
    
    // 2. Apply learned reranking
    reranked := s.reranker.Rerank(ctx, query, candidates)
    
    // 3. Generate response
    response := s.rag.Generate(ctx, query, reranked[:5])
    
    // 4. Collect implicit feedback
    s.feedback.StartTracking(query, response)
    
    return response, nil
}

func (s *SelfImprovingRAG) ProcessFeedback(ctx context.Context, feedback UserFeedback) error {
    // 1. Record feedback
    s.feedback.Record(feedback)
    
    // 2. Update reranker model
    if s.feedback.HasSufficientData() {
        trainingData := s.feedback.PrepareTrainingData()
        s.reranker.Update(trainingData)
    }
    
    // 3. Fine-tune embeddings if needed
    if s.feedback.SuggestsEmbeddingIssues() {
        s.finetuner.ScheduleFineTuning()
    }
    
    return nil
}
```

## Implementation Best Practices

### 1. Chunking Strategies

```go
// Smart chunking based on content type
type SmartChunker struct {
    codeChunker chunking.CodeChunker
    textChunker chunking.TextChunker
    listChunker chunking.ListChunker
}

func (s *SmartChunker) Chunk(content Content) []Chunk {
    switch content.Type {
    case "code":
        // Respect function boundaries
        return s.codeChunker.ChunkByFunction(content)
    case "markdown":
        // Respect heading hierarchy
        return s.textChunker.ChunkBySection(content)
    case "logs":
        // Chunk by time windows
        return s.listChunker.ChunkByTimestamp(content)
    default:
        // Fallback to sliding window
        return s.textChunker.SlidingWindow(content, 1000, 200)
    }
}
```

### 2. Context Window Optimization

```go
// Optimize context for LLM token limits
type ContextOptimizer struct {
    tokenizer    Tokenizer
    maxTokens    int
    prioritizer  ContentPrioritizer
}

func (c *ContextOptimizer) Optimize(query string, documents []Document) string {
    // 1. Calculate token budget
    queryTokens := c.tokenizer.Count(query)
    contextBudget := c.maxTokens - queryTokens - 500 // Reserve for response
    
    // 2. Prioritize documents
    prioritized := c.prioritizer.Prioritize(query, documents)
    
    // 3. Build context within budget
    var context strings.Builder
    currentTokens := 0
    
    for _, doc := range prioritized {
        docTokens := c.tokenizer.Count(doc.Content)
        if currentTokens+docTokens > contextBudget {
            // Truncate if necessary
            remainingBudget := contextBudget - currentTokens
            truncated := c.truncateToTokens(doc.Content, remainingBudget)
            context.WriteString(truncated)
            break
        }
        context.WriteString(doc.Content)
        context.WriteString("\n\n")
        currentTokens += docTokens
    }
    
    return context.String()
}
```

### 3. Query Enhancement

```go
// Enhance queries for better retrieval
type QueryEnhancer struct {
    synonyms     SynonymService
    spellcheck   SpellChecker
    expander     QueryExpander
}

func (q *QueryEnhancer) Enhance(query string) EnhancedQuery {
    // 1. Spell correction
    corrected := q.spellcheck.Correct(query)
    
    // 2. Synonym expansion
    synonymQueries := q.synonyms.Expand(corrected)
    
    // 3. Query expansion with related terms
    expanded := q.expander.Expand(corrected)
    
    return EnhancedQuery{
        Original:  query,
        Corrected: corrected,
        Synonyms:  synonymQueries,
        Expanded:  expanded,
        Weights: map[string]float64{
            "original":  1.0,
            "corrected": 0.9,
            "synonyms":  0.7,
            "expanded":  0.5,
        },
    }
}
```

### 4. Result Caching

```go
// Intelligent caching for RAG
type RAGCache struct {
    semantic    SemanticCache
    exact       ExactMatchCache
    temporal    TemporalCache
}

func (r *RAGCache) Get(ctx context.Context, query string) (*CachedResult, bool) {
    // 1. Check exact match
    if result := r.exact.Get(query); result != nil {
        return result, true
    }
    
    // 2. Check semantic similarity
    embedding := r.embed(query)
    if similar := r.semantic.FindSimilar(embedding, 0.95); similar != nil {
        return similar, true
    }
    
    // 3. Check temporal relevance
    if temporal := r.temporal.GetRecent(query, 5*time.Minute); temporal != nil {
        return temporal, true
    }
    
    return nil, false
}

func (r *RAGCache) Set(ctx context.Context, query string, result *Response) {
    // Cache at multiple levels
    r.exact.Set(query, result, 1*time.Hour)
    r.semantic.Set(r.embed(query), result, 30*time.Minute)
    r.temporal.Set(query, result)
}
```

## Performance Patterns

### 1. Streaming RAG

```go
// Stream responses as they're generated
type StreamingRAG struct {
    retriever Retriever
    generator StreamingLLM
}

func (s *StreamingRAG) StreamResponse(ctx context.Context, query string, output chan<- string) error {
    // 1. Retrieve context (non-blocking)
    contextChan := make(chan Document, 10)
    go s.retriever.StreamRetrieve(ctx, query, contextChan)
    
    // 2. Build prompt progressively
    var promptBuilder strings.Builder
    promptBuilder.WriteString(fmt.Sprintf("Query: %s\n\nContext:\n", query))
    
    // 3. Stream generation as context arrives
    for doc := range contextChan {
        promptBuilder.WriteString(doc.Summary())
        promptBuilder.WriteString("\n")
        
        // Start generating when we have enough context
        if promptBuilder.Len() > 1000 {
            prompt := promptBuilder.String()
            return s.generator.StreamGenerate(ctx, prompt, output)
        }
    }
    
    // Generate with all available context
    return s.generator.StreamGenerate(ctx, promptBuilder.String(), output)
}
```

### 2. Batch RAG Processing

```go
// Process multiple queries efficiently
type BatchRAG struct {
    pipeline     RAGPipeline
    batchSize    int
    parallelism  int
}

func (b *BatchRAG) ProcessBatch(ctx context.Context, queries []string) ([]*Response, error) {
    // 1. Group similar queries
    groups := b.groupSimilarQueries(queries)
    
    // 2. Process groups in parallel
    sem := make(chan struct{}, b.parallelism)
    results := make([]*Response, len(queries))
    var wg sync.WaitGroup
    
    for _, group := range groups {
        wg.Add(1)
        go func(g QueryGroup) {
            defer wg.Done()
            sem <- struct{}{}
            defer func() { <-sem }()
            
            // Share context retrieval for similar queries
            sharedContext := b.pipeline.RetrieveShared(ctx, g.Queries)
            
            // Generate responses
            for i, query := range g.Queries {
                response := b.pipeline.GenerateWithContext(ctx, query, sharedContext)
                results[g.Indices[i]] = response
            }
        }(group)
    }
    
    wg.Wait()
    return results, nil
}
```

## Monitoring Patterns

### 1. Quality Metrics

```go
// Monitor RAG quality
type RAGQualityMonitor struct {
    metrics map[string]QualityMetric
}

type QualityMetric struct {
    RetrievalPrecision   float64
    RetrievalRecall      float64
    ResponseRelevance    float64
    ResponseCoherence    float64
    ResponseCompleteness float64
    LatencyP50          time.Duration
    LatencyP99          time.Duration
}

func (m *RAGQualityMonitor) Track(ctx context.Context, query string, response *Response) {
    // Track retrieval quality
    m.trackRetrievalQuality(query, response.RetrievedDocs)
    
    // Track response quality
    m.trackResponseQuality(query, response.Content)
    
    // Track performance
    m.trackPerformance(response.Latency)
    
    // Alert on quality degradation
    if m.detectQualityDegradation() {
        m.alert("RAG quality degradation detected")
    }
}
```

### 2. Cost Tracking

```go
// Track and optimize costs
type RAGCostTracker struct {
    embedding CostModel
    llm       CostModel
    storage   CostModel
}

func (c *RAGCostTracker) EstimateQueryCost(query string, strategy RAGStrategy) float64 {
    cost := 0.0
    
    // Embedding cost
    cost += c.embedding.Calculate(len(query))
    
    // Retrieval cost (storage reads)
    cost += c.storage.Calculate(strategy.RetrievalCount)
    
    // Generation cost
    cost += c.llm.Calculate(strategy.ExpectedTokens)
    
    return cost
}

func (c *RAGCostTracker) OptimizeStrategy(budget float64) RAGStrategy {
    // Find optimal strategy within budget
    strategies := []RAGStrategy{
        {Name: "minimal", RetrievalCount: 3, Model: "small"},
        {Name: "balanced", RetrievalCount: 5, Model: "medium"},
        {Name: "comprehensive", RetrievalCount: 10, Model: "large"},
    }
    
    for i := len(strategies) - 1; i >= 0; i-- {
        if c.EstimateQueryCost("average", strategies[i]) <= budget {
            return strategies[i]
        }
    }
    
    return strategies[0] // Fallback to minimal
}
```

## Anti-Patterns to Avoid

### 1. Over-Retrieval
```go
// ❌ Bad: Retrieving too many documents
docs := vectorStore.Search(query, 100) // Too many!

// ✅ Good: Retrieve intelligently
docs := vectorStore.SearchWithFilters(query, 10, filters)
```

### 2. Context Stuffing
```go
// ❌ Bad: Including all retrieved content
context := strings.Join(allDocs, "\n")

// ✅ Good: Selective context building
context := optimizer.SelectRelevant(query, docs, maxTokens)
```

### 3. Single-Model Dependency
```go
// ❌ Bad: Using one embedding model for everything
embedding := model.Embed(anyContent)

// ✅ Good: Content-aware model selection
embedding := selector.SelectModel(content.Type).Embed(content)
```

## Conclusion

These RAG patterns provide a foundation for building sophisticated retrieval-augmented generation systems with MCP. Choose patterns based on your specific requirements:

- **Simple RAG**: Quick prototypes and basic Q&A
- **Hybrid Search**: Better retrieval accuracy
- **Multi-Agent**: Complex queries requiring specialization
- **Adaptive**: Dynamic optimization based on performance
- **Self-Improving**: Systems that learn from usage

Remember to monitor quality, optimize costs, and continuously improve based on user feedback.

## Next Steps

1. Implement patterns incrementally
2. Measure performance and quality
3. Optimize based on real usage
4. Share learnings with the community

## Resources

- [MCP RAG Architecture](./rag-mcp-architecture.md)
- [RAG Integration Guide](./rag-integration-guide.md)
- [AI Agent Orchestration](./ai-agent-orchestration.md)
- [Performance Tuning Guide](./performance-tuning-guide.md)