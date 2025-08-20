# Embeddings Documentation

> Vector embeddings and semantic search capabilities for Developer Mesh

## 📚 Documentation Structure

### Getting Started
- **[Quick Start](./quickstart/quickstart.md)** - Get started with embeddings

### Core Documentation
- **[Architecture](./architecture.md)** - Multi-agent embedding architecture
- **[Diagrams](./diagrams.md)** - Embedding model sequence diagrams

### Guides
- **[Operations Guide](./guides/operations.md)** - Operating embedding services

### Examples
- **[Working Examples](./examples/working.md)** - Working embedding examples

### Reference
- **[API Reference](./reference/api.md)** - Embedding API documentation
- **[Models (Not Implemented)](./reference/models-not-implemented.md)** - Future model API

### Troubleshooting
- **[Model Troubleshooting](./troubleshooting/models.md)** - Common model issues

## 🚀 Quick Start

### 1. Understanding Embeddings
Embeddings convert text into high-dimensional vectors that capture semantic meaning:
- Similar texts have similar vectors
- Enables semantic search and similarity matching
- Powers AI agent memory and context

### 2. Basic Usage
```bash
# Generate embedding for text
curl -X POST http://localhost:8081/api/v1/embeddings \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "text": "Your text to embed",
    "model": "text-embedding-ada-002"
  }'
```

### 3. Semantic Search
```bash
# Search for similar content
curl -X POST http://localhost:8081/api/v1/embeddings/search \
  -H "Authorization: Bearer $API_KEY" \
  -d '{
    "query": "search query",
    "limit": 10
  }'
```

## 🎯 Key Features

### Multi-Model Support
- **OpenAI**: text-embedding-ada-002, text-embedding-3-small
- **AWS Bedrock**: Amazon Titan, Cohere models
- **Custom Models**: Bring your own embedding models

### Multi-Tenant Architecture
- Per-tenant model selection
- Isolated vector spaces
- Quota management
- Usage tracking

### Vector Storage
- **PostgreSQL pgvector**: Production-ready vector database
- **Indexing**: HNSW and IVFFlat indexes
- **Similarity Metrics**: Cosine, L2, Inner Product

## 🏗️ Architecture

### Components
```
Text Input → Embedding Service → Vector Storage → Search Service
                ↓                      ↓
          Model Selection         pgvector DB
```

### Processing Pipeline
1. **Text Preprocessing**: Cleaning and normalization
2. **Chunking**: Split long texts into chunks
3. **Embedding Generation**: Convert to vectors
4. **Storage**: Save in vector database
5. **Indexing**: Build search indexes

## 📊 Use Cases

### Semantic Search
- Find similar documents
- Question answering
- Content recommendation

### AI Agent Memory
- Long-term memory storage
- Context retrieval
- Experience replay

### Code Understanding
- Similar code detection
- API documentation search
- Code review assistance

## 🔧 Configuration

### Model Configuration
```yaml
embeddings:
  default_model: text-embedding-ada-002
  chunk_size: 1000
  overlap: 200
  max_tokens: 8191
```

### Database Configuration
```yaml
pgvector:
  dimensions: 1536
  index_type: hnsw
  metric: cosine
```

## 📈 Performance

### Optimization Strategies
- Batch processing for multiple texts
- Caching frequently used embeddings
- Asynchronous processing
- Index optimization

### Monitoring Metrics
- Embedding generation latency
- Search query performance
- Model usage and costs
- Storage utilization

## 💰 Cost Management

### Token Usage
- Track tokens per request
- Monitor monthly usage
- Set tenant quotas
- Cost allocation

### Model Selection
- Choose appropriate model for use case
- Balance accuracy vs. cost
- Use caching to reduce API calls

## 🔗 Related Documentation

- [AI Agents](../agents/) - Agent integration with embeddings
- [API Reference](../api/) - Complete API documentation
- [Dynamic Tools](../dynamic-tools/) - Tool integration

## 🆘 Getting Help

- Check [Troubleshooting Guide](./troubleshooting/models.md)
- Review [Working Examples](./examples/working.md)
- See [API Reference](./reference/api.md)

---

*Last updated: August 2025*