<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:34:47
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Multi-Agent Embedding System Examples

This document provides practical examples of using the Multi-Agent Embedding System in various scenarios.

## Table of Contents

1. [Basic Usage Examples](#basic-usage-examples)
2. [Agent Configuration Examples](#agent-configuration-examples)
3. [Language-Specific Examples](#language-specific-examples)
4. [Advanced Scenarios](#advanced-scenarios)
5. [Integration Examples](#integration-examples)

## Basic Usage Examples

### Simple Embedding Generation

```bash
# Generate a single embedding
curl -X POST http://localhost:8081/api/v1/embeddings \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '{
    "agent_id": "general-agent",
    "text": "The quick brown fox jumps over the lazy dog",
    "task_type": "general_qa"
  }'
```

### Batch Embedding Generation

```bash
# Process multiple texts at once
curl -X POST http://localhost:8081/api/v1/embeddings/batch \
  -H "X-API-Key: your-api-key" \
  -H "Content-Type: application/json" \
  -d '[
    {
      "agent_id": "document-processor",
      "text": "Introduction to machine learning",
      "task_type": "search_document",
      "metadata": {"chapter": 1, "section": "intro"}
    },
    {
      "agent_id": "document-processor",
      "text": "Neural networks are computational models",
      "task_type": "search_document",
      "metadata": {"chapter": 2, "section": "neural-nets"}
    }
  ]'
```

## Agent Configuration Examples

### Document Processing Agent

Optimized for processing large documents with high quality:

```json
{
  "agent_id": "document-processor",
  "embedding_strategy": "quality",
  "model_preferences": {
    "search_document": {
      "primary_models": ["text-embedding-3-large"],
      "fallback_models": ["text-embedding-3-small"]
    },
    "search_query": {
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 8000,
    "max_cost_per_day": 100.0,
    "preferred_dimensions": 3072,
    "allow_dimension_reduction": true
  },
  "fallback_behavior": {
    "enabled": true,
    "max_retries": 5,
    "retry_delay": "2s",
    "exponential_backoff": true,
    "use_cache_on_failure": true
  },
  "metadata": {
    "team": "content",
    "purpose": "knowledge-base"
  }
}
```

### Real-Time Chat Agent

Optimized for low latency in conversational applications:

```json
{
  "agent_id": "chat-agent",
  "embedding_strategy": "speed",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["text-embedding-3-small"],
      "fallback_models": ["text-embedding-ada-002"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 500,
    "max_cost_per_day": 50.0,
    "preferred_dimensions": 1536,
    "allow_dimension_reduction": true,
    "max_latency_ms": 200
  },
  "metadata": {
    "team": "chatbot",
    "sla": "200ms"
  }
}
```

### Multilingual Content Agent

Configured for handling multiple languages:

```json
{
  "agent_id": "multilingual-agent",
  "embedding_strategy": "quality",
  "model_preferences": {
    "general_qa": {
      "primary_models": ["cohere.embed-multilingual-v3"],
      "fallback_models": ["text-embedding-3-large"]
    }
  },
  "constraints": {
    "max_tokens_per_request": 512,
    "max_cost_per_day": 75.0,
    "preferred_dimensions": 1024,
    "allow_dimension_reduction": true
  },
  "metadata": {
    "supported_languages": ["en", "es", "fr", "de", "ja", "zh"],
    "team": "international"
  }
}
```

## Language-Specific Examples

### Python

```python
import requests
import json

class EmbeddingClient:
    def __init__(self, base_url, api_key):
        self.base_url = base_url
        self.headers = {
            "X-API-Key": api_key,
            "Content-Type": "application/json"
        }
    
    def generate_embedding(self, agent_id, text, task_type="general_qa", metadata=None):
        """Generate a single embedding"""
        payload = {
            "agent_id": agent_id,
            "text": text,
            "task_type": task_type
        }
        if metadata:
            payload["metadata"] = metadata
        
        response = requests.post(
            f"{self.base_url}/embeddings",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()
    
    def batch_generate_embeddings(self, requests_list):
        """Generate multiple embeddings in one request"""
        response = requests.post(
            f"{self.base_url}/embeddings/batch",
            headers=self.headers,
            json=requests_list
        )
        response.raise_for_status()
        return response.json()
    
    def search_embeddings(self, agent_id, query, limit=10, min_similarity=0.7):
        """Search for similar embeddings"""
        payload = {
            "agent_id": agent_id,
            "query": query,
            "limit": limit,
            "min_similarity": min_similarity
        }
        
        response = requests.post(
            f"{self.base_url}/embeddings/search",
            headers=self.headers,
            json=payload
        )
        response.raise_for_status()
        return response.json()

# Usage example
client = EmbeddingClient(
    base_url="http://localhost:8081/api/v1",
    api_key="your-api-key"
)

# Single embedding
result = client.generate_embedding(
    agent_id="python-agent",
    text="Python is a high-level programming language",
    metadata={"source": "documentation"}
)
print(f"Embedding ID: {result['embedding_id']}")
print(f"Model used: {result['model_used']}")
print(f"Cost: ${result['cost_usd']:.6f}")

# Batch embeddings
documents = [
    {
        "agent_id": "python-agent",
        "text": "Functions are first-class objects in Python",
        "metadata": {"doc_id": "func-001"}
    },
    {
        "agent_id": "python-agent",
        "text": "Python supports multiple programming paradigms",
        "metadata": {"doc_id": "paradigm-001"}
    }
]

batch_result = client.batch_generate_embeddings(documents)
print(f"Generated {batch_result['count']} embeddings")
```

### JavaScript/TypeScript

```typescript
interface EmbeddingRequest {
  agent_id: string;
  text: string;
  task_type?: string;
  metadata?: Record<string, any>;
}

interface EmbeddingResponse {
  embedding_id: string;
  request_id: string;
  model_used: string;
  provider: string;
  dimensions: number;
  cost_usd: number;
  tokens_used: number;
  generation_time_ms: number;
  cached: boolean;
  metadata?: Record<string, any>;
}

class EmbeddingClient {
  private baseUrl: string;
  private headers: HeadersInit;

  constructor(baseUrl: string, apiKey: string) {
    this.baseUrl = baseUrl;
    this.headers = {
      'X-API-Key': apiKey,
      'Content-Type': 'application/json'
    };
  }

  async generateEmbedding(request: EmbeddingRequest): Promise<EmbeddingResponse> {
    const response = await fetch(`${this.baseUrl}/embeddings`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(request)
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return await response.json();
  }

  async batchGenerateEmbeddings(requests: EmbeddingRequest[]): Promise<{
    embeddings: EmbeddingResponse[];
    count: number;
  }> {
    const response = await fetch(`${this.baseUrl}/embeddings/batch`, {
      method: 'POST',
      headers: this.headers,
      body: JSON.stringify(requests)
    });

    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return await response.json();
  }
}

// Usage
const client = new EmbeddingClient(
  'http://localhost:8081/api/v1',
  'your-api-key'
);

// Generate single embedding
async function generateExample() {
  try {
    const result = await client.generateEmbedding({
      agent_id: 'js-agent',
      text: 'JavaScript is a dynamic programming language',
      task_type: 'general_qa',
      metadata: { source: 'tutorial' }
    });
    
    console.log(`Embedding ID: ${result.embedding_id}`);
    console.log(`Cost: $${result.cost_usd.toFixed(6)}`);
    console.log(`Generation time: ${result.generation_time_ms}ms`);
  } catch (error) {
    console.error('Error generating embedding:', error);
  }
}

// Batch processing with error handling
async function batchExample() {
  const documents = [
    {
      agent_id: 'js-agent',
      text: 'React is a JavaScript library for building UIs',
      metadata: { framework: 'react' }
    },
    {
      agent_id: 'js-agent',
      text: 'Vue.js is a progressive JavaScript framework',
      metadata: { framework: 'vue' }
    }
  ];

  try {
    const result = await client.batchGenerateEmbeddings(documents);
    console.log(`Generated ${result.count} embeddings`);
    
    result.embeddings.forEach((embedding, index) => {
      console.log(`[${index}] Model: ${embedding.model_used}, Cost: $${embedding.cost_usd}`);
    });
  } catch (error) {
    console.error('Error in batch processing:', error);
  }
}
```

### Go

```go
package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
)

type EmbeddingRequest struct {
    AgentID  string                 `json:"agent_id"`
    Text     string                 `json:"text"`
    TaskType string                 `json:"task_type,omitempty"`
    Metadata map[string]interface{} `json:"metadata,omitempty"`
}

type EmbeddingResponse struct {
    EmbeddingID          string                 `json:"embedding_id"`
    RequestID            string                 `json:"request_id"`
    ModelUsed            string                 `json:"model_used"`
    Provider             string                 `json:"provider"`
    Dimensions           int                    `json:"dimensions"`
    NormalizedDimensions int                    `json:"normalized_dimensions"`
    CostUSD              float64                `json:"cost_usd"`
    TokensUsed           int                    `json:"tokens_used"`
    GenerationTimeMs     int64                  `json:"generation_time_ms"`
    Cached               bool                   `json:"cached"`
    Metadata             map[string]interface{} `json:"metadata,omitempty"`
}

type EmbeddingClient struct {
    BaseURL    string
    APIKey     string
    HTTPClient *http.Client
}

func NewEmbeddingClient(baseURL, apiKey string) *EmbeddingClient {
    return &EmbeddingClient{
        BaseURL: baseURL,
        APIKey:  apiKey,
        HTTPClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

func (c *EmbeddingClient) GenerateEmbedding(req EmbeddingRequest) (*EmbeddingResponse, error) {
    jsonData, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    httpReq, err := http.NewRequest("POST", c.BaseURL+"/embeddings", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    httpReq.Header.Set("X-API-Key", c.APIKey)
    httpReq.Header.Set("Content-Type", "application/json")

    resp, err := c.HTTPClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
    }

    var embResp EmbeddingResponse
    if err := json.NewDecoder(resp.Body).Decode(&embResp); err != nil {
        return nil, fmt.Errorf("failed to decode response: %w", err)
    }

    return &embResp, nil
}

func main() {
    client := NewEmbeddingClient("http://localhost:8081/api/v1", "your-api-key")

    // Generate embedding
    resp, err := client.GenerateEmbedding(EmbeddingRequest{
        AgentID:  "go-agent",
        Text:     "Go is a statically typed, compiled programming language",
        TaskType: "general_qa",
        Metadata: map[string]interface{}{
            "language": "go",
            "version":  "1.21",
        },
    })

    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }

    fmt.Printf("Embedding ID: %s\n", resp.EmbeddingID)
    fmt.Printf("Model: %s (%s)\n", resp.ModelUsed, resp.Provider)
    fmt.Printf("Cost: $%.6f\n", resp.CostUSD)
    fmt.Printf("Generation time: %dms\n", resp.GenerationTimeMs)
}
```

## Advanced Scenarios

### Document Chunking and Processing

```python
import tiktoken
from typing import List, Dict

class DocumentProcessor:
    def __init__(self, embedding_client, agent_id, max_tokens=2000):
        self.client = embedding_client
        self.agent_id = agent_id
        self.max_tokens = max_tokens
        self.encoder = tiktoken.encoding_for_model("text-embedding-3-small")
    
    def chunk_text(self, text: str, overlap: int = 200) -> List[str]:
        """Split text into chunks with token limit and overlap"""
        tokens = self.encoder.encode(text)
        chunks = []
        
        for i in range(0, len(tokens), self.max_tokens - overlap):
            chunk_tokens = tokens[i:i + self.max_tokens]
            chunk_text = self.encoder.decode(chunk_tokens)
            chunks.append(chunk_text)
        
        return chunks
    
    def process_document(self, document_id: str, text: str) -> List[Dict]:
        """Process a document by chunking and generating embeddings"""
        chunks = self.chunk_text(text)
        
        # Prepare batch requests
        requests = []
        for i, chunk in enumerate(chunks):
            requests.append({
                "agent_id": self.agent_id,
                "text": chunk,
                "task_type": "search_document",
                "metadata": {
                    "document_id": document_id,
                    "chunk_index": i,
                    "total_chunks": len(chunks)
                }
            })
        
        # Process in batches of 50
        all_results = []
        for i in range(0, len(requests), 50):
            batch = requests[i:i+50]
            result = self.client.batch_generate_embeddings(batch)
            all_results.extend(result['embeddings'])
        
        return all_results

# Usage
processor = DocumentProcessor(client, "document-processor")

# Process a large document
with open("large_document.txt", "r") as f:
    content = f.read()

embeddings = processor.process_document("doc-001", content)
print(f"Generated {len(embeddings)} embeddings for document")
```

### Semantic Search Implementation

```python
import numpy as np
from typing import List, Tuple

class SemanticSearch:
    def __init__(self, embedding_client):
        self.client = embedding_client
        self.embeddings_cache = {}
    
    def store_embeddings(self, embeddings: List[Dict]):
        """Store embeddings in cache (in production, use vector DB)"""
        for emb in embeddings:
            self.embeddings_cache[emb['embedding_id']] = {
                'vector': emb.get('vector'),  # Would be returned by API
                'metadata': emb.get('metadata', {})
            }
    
    def search(self, agent_id: str, query: str, top_k: int = 10) -> List[Tuple[str, float]]:
        """Search for similar documents"""
        # Generate query embedding
        query_result = self.client.generate_embedding(
            agent_id=agent_id,
            text=query,
            task_type="search_query"
        )
        
        # In production, this would call the search API
        # For demo, we'll show the pattern
        search_result = self.client.search_embeddings(
            agent_id=agent_id,
            query=query,
            limit=top_k,
            min_similarity=0.7
        )
        
        return search_result

# Example usage for RAG (Retrieval Augmented Generation)
class RAGSystem:
    def __init__(self, embedding_client, llm_client):
        self.embedding_client = embedding_client
        self.llm_client = llm_client
        self.search = SemanticSearch(embedding_client)
    
    def answer_question(self, question: str, agent_id: str) -> str:
        # Search for relevant context
        results = self.search.search(agent_id, question, top_k=5)
        
        # Build context from search results
        context = "\n\n".join([r['content'] for r in results['results']])
        
        # Generate answer using LLM with context
        prompt = f"""Based on the following context, answer the question.
        
Context:
{context}

Question: {question}

Answer:"""
        
        # Call your LLM here
        answer = self.llm_client.generate(prompt)
        
        return answer
```

### Cost Monitoring and Optimization

```python
from datetime import datetime, timedelta
from collections import defaultdict

class CostMonitor:
    def __init__(self, embedding_client):
        self.client = embedding_client
        self.cost_data = defaultdict(lambda: defaultdict(float))
    
    def track_embedding_cost(self, result: Dict):
        """Track costs by agent and model"""
        date = datetime.now().date()
        agent_id = result.get('agent_id', 'unknown')
        model = result.get('model_used', 'unknown')
        cost = result.get('cost_usd', 0)
        
        self.cost_data[date][f"{agent_id}:{model}"] += cost
    
    def get_daily_report(self, date=None) -> Dict:
        """Generate daily cost report"""
        if date is None:
            date = datetime.now().date()
        
        report = {
            'date': str(date),
            'total_cost': 0,
            'by_agent': defaultdict(float),
            'by_model': defaultdict(float),
            'details': []
        }
        
        for key, cost in self.cost_data[date].items():
            agent_id, model = key.split(':')
            report['total_cost'] += cost
            report['by_agent'][agent_id] += cost
            report['by_model'][model] += cost
            report['details'].append({
                'agent_id': agent_id,
                'model': model,
                'cost': cost
            })
        
        return dict(report)
    
    def optimize_agent_config(self, agent_id: str, target_cost: float) -> Dict:
        """Suggest optimized configuration based on usage"""
        # Analyze historical usage
        total_cost = sum(
            cost for key, cost in self.cost_data.items()
            if key.startswith(f"{agent_id}:")
        )
        
        # Suggest optimizations
        suggestions = []
        
        if total_cost > target_cost * 1.2:
            suggestions.append({
                'action': 'switch_model',
                'from': 'text-embedding-3-large',
                'to': 'text-embedding-3-small',
                'estimated_savings': '60%'
            })
        
        return {
            'current_cost': total_cost,
            'target_cost': target_cost,
            'suggestions': suggestions
        }
```

## Integration Examples

### Integration with LangChain

```python
from langchain.embeddings.base import Embeddings
from typing import List

class DevOpsMCPEmbeddings(Embeddings):
    """Custom LangChain embeddings using Developer Mesh"""
    
    def __init__(self, base_url: str, api_key: str, agent_id: str):
        self.client = EmbeddingClient(base_url, api_key)
        self.agent_id = agent_id
    
    def embed_documents(self, texts: List[str]) -> List[List[float]]:
        """Embed search docs"""
        requests = [
            {
                "agent_id": self.agent_id,
                "text": text,
                "task_type": "search_document"
            }
            for text in texts
        ]
        
        result = self.client.batch_generate_embeddings(requests)
        
        # Extract vectors (would need to fetch from DB or cache)
        # This is a simplified example
        embeddings = []
        for emb in result['embeddings']:
            # In real implementation, fetch actual vector
            embeddings.append([0.0] * emb['dimensions'])
        
        return embeddings
    
    def embed_query(self, text: str) -> List[float]:
        """Embed query text"""
        result = self.client.generate_embedding(
            agent_id=self.agent_id,
            text=text,
            task_type="search_query"
        )
        
        # In real implementation, fetch actual vector
        return [0.0] * result['dimensions']

# Usage with LangChain
embeddings = DevOpsMCPEmbeddings(
    base_url="http://localhost:8081/api/v1",
    api_key="your-api-key",
    agent_id="langchain-agent"
)

# Use with vector store
from langchain.vectorstores import FAISS

vector_store = FAISS.from_texts(
    texts=["Document 1", "Document 2"],
    embedding=embeddings
)
```

### Integration with Vector Databases

```python
import psycopg2
from pgvector.psycopg2 import register_vector

class PgVectorIntegration:
    def __init__(self, db_config, embedding_client):
        self.conn = psycopg2.connect(**db_config)
        register_vector(self.conn)
        self.client = embedding_client
    
    def store_embedding(self, text: str, agent_id: str, metadata: Dict):
        """Generate and store embedding in PostgreSQL"""
        # Generate embedding
        result = self.client.generate_embedding(
            agent_id=agent_id,
            text=text,
            metadata=metadata
        )
        
        # Store in database
        with self.conn.cursor() as cur:
            cur.execute("""
                INSERT INTO embeddings (
                    id, content, embedding, metadata, 
                    model_name, agent_id, created_at
                ) VALUES (%s, %s, %s, %s, %s, %s, NOW())
            """, (
                result['embedding_id'],
                text,
                result['embedding_id'],  # Placeholder - real vector would be fetched
                json.dumps(metadata),
                result['model_used'],
                agent_id
            ))
        
        self.conn.commit()
        return result['embedding_id']
    
    def similarity_search(self, query: str, agent_id: str, limit: int = 10):
        """Perform similarity search"""
        # Generate query embedding
        query_result = self.client.generate_embedding(
            agent_id=agent_id,
            text=query,
            task_type="search_query"
        )
        
        # Search in database
        with self.conn.cursor() as cur:
            cur.execute("""
                SELECT id, content, metadata,
                       1 - (embedding <-> %s) as similarity
                FROM embeddings
                WHERE agent_id = %s
                ORDER BY embedding <-> %s
                LIMIT %s
            """, (
                query_result['embedding_id'],  # Placeholder
                agent_id,
                query_result['embedding_id'],  # Placeholder
                limit
            ))
            
            results = []
            for row in cur.fetchall():
                results.append({
                    'id': row[0],
                    'content': row[1],
                    'metadata': row[2],
                    'similarity': row[3]
                })
            
            return results
```

## Error Handling and Retry Logic

```python
import time
from typing import Optional
import backoff

class ResilientEmbeddingClient(EmbeddingClient):
    """Enhanced client with retry logic and error handling"""
    
    @backoff.on_exception(
        backoff.expo,
        requests.exceptions.RequestException,
        max_tries=3,
        max_time=30
    )
    def generate_embedding_with_retry(self, **kwargs):
        """Generate embedding with automatic retry"""
        try:
            return self.generate_embedding(**kwargs)
        except requests.exceptions.HTTPError as e:
            if e.response.status_code == 429:  # Rate limit
                retry_after = int(e.response.headers.get('Retry-After', 60))
                time.sleep(retry_after)
                return self.generate_embedding(**kwargs)
            elif e.response.status_code == 503:  # Service unavailable
                # Try fallback agent if configured
                if kwargs.get('fallback_agent_id'):
                    kwargs['agent_id'] = kwargs['fallback_agent_id']
                    return self.generate_embedding(**kwargs)
            raise
    
    def generate_with_fallback(
        self, 
        primary_agent_id: str, 
        fallback_agent_id: str,
        text: str,
        **kwargs
    ) -> Optional[Dict]:
        """Try primary agent, fall back to secondary if needed"""
        try:
            return self.generate_embedding(
                agent_id=primary_agent_id,
                text=text,
                **kwargs
            )
        except Exception as e:
            print(f"Primary agent failed: {e}, trying fallback")
            try:
                return self.generate_embedding(
                    agent_id=fallback_agent_id,
                    text=text,
                    **kwargs
                )
            except Exception as e2:
                print(f"Fallback also failed: {e2}")
                return None
```

## Performance Testing

```python
import asyncio
import aiohttp
import time
from concurrent.futures import ThreadPoolExecutor

class PerformanceTester:
    def __init__(self, base_url: str, api_key: str):
        self.base_url = base_url
        self.api_key = api_key
    
    async def async_generate_embedding(self, session, request):
        """Async embedding generation"""
        headers = {
            "X-API-Key": self.api_key,
            "Content-Type": "application/json"
        }
        
        async with session.post(
            f"{self.base_url}/embeddings",
            headers=headers,
            json=request
        ) as response:
            return await response.json()
    
    async def load_test(self, num_requests: int, agent_id: str):
        """Run load test with concurrent requests"""
        async with aiohttp.ClientSession() as session:
            tasks = []
            
            for i in range(num_requests):
                request = {
                    "agent_id": agent_id,
                    "text": f"Test document number {i}",
                    "metadata": {"test_id": i}
                }
                task = self.async_generate_embedding(session, request)
                tasks.append(task)
            
            start_time = time.time()
            results = await asyncio.gather(*tasks)
            end_time = time.time()
            
            # Calculate metrics
            total_time = end_time - start_time
            avg_time = total_time / num_requests
            requests_per_second = num_requests / total_time
            
            return {
                "total_requests": num_requests,
                "total_time": total_time,
                "average_time": avg_time,
                "requests_per_second": requests_per_second,
                "results": results
            }

# Run performance test
async def main():
    tester = PerformanceTester(
        "http://localhost:8081/api/v1",
        "your-api-key"
    )
    
    results = await tester.load_test(100, "perf-test-agent")
    print(f"Processed {results['total_requests']} requests")
    print(f"Average time: {results['average_time']:.3f} seconds")
    print(f"Throughput: {results['requests_per_second']:.1f} req/s")

# asyncio.run(main())
