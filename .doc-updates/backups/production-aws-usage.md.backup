# Production AWS Services Usage Examples

This guide demonstrates how to use Developer Mesh with real AWS services in production.

## Prerequisites

- AWS Account with appropriate permissions
- AWS CLI configured (`aws configure`)
- Environment variables set for AWS credentials
- SSH tunnel to ElastiCache (if using Redis)

## Environment Configuration

### Required Environment Variables

```bash
# AWS Configuration
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=your-access-key
export AWS_SECRET_ACCESS_KEY=your-secret-key

# S3 Configuration
export S3_BUCKET=sean-mcp-dev-contexts

# SQS Configuration
export SQS_QUEUE_URL=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test

# ElastiCache Redis (via SSH tunnel)
export REDIS_ADDR=127.0.0.1:6379
export USE_SSH_TUNNEL_FOR_REDIS=true

# Bedrock Configuration
export BEDROCK_ENABLED=true
export BEDROCK_SESSION_LIMIT=0.10
export GLOBAL_COST_LIMIT=10.0
```

## S3 Context Storage

### Storing Contexts in S3

```go
package main

import (
    "context"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/yourorg/developer-mesh/pkg/storage"
)

func ExampleS3Storage() {
    ctx := context.Background()
    
    // Load AWS configuration
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
    )
    if err != nil {
        panic(err)
    }
    
    // Create S3 client
    s3Client := s3.NewFromConfig(cfg)
    
    // Initialize storage
    store := storage.NewS3Storage(s3Client, "sean-mcp-dev-contexts")
    
    // Store a context
    contextData := &storage.Context{
        ID:          "ctx-123",
        Name:        "Production Deployment",
        Description: "Context for production deployment workflows",
        AgentID:     "agent-prod-1",
        Metadata: map[string]interface{}{
            "environment": "production",
            "version":     "1.0.0",
        },
    }
    
    err = store.SaveContext(ctx, contextData)
    if err != nil {
        panic(err)
    }
}
```

## SQS Task Queue

### Publishing Tasks to SQS

```go
package main

import (
    "context"
    "encoding/json"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
    "github.com/yourorg/developer-mesh/pkg/queue"
)

func ExampleSQSQueue() {
    ctx := context.Background()
    
    // Load AWS configuration
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
    )
    if err != nil {
        panic(err)
    }
    
    // Create SQS client
    sqsClient := sqs.NewFromConfig(cfg)
    
    // Initialize queue
    taskQueue := queue.NewSQSQueue(sqsClient, 
        "https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test")
    
    // Create a task
    task := &queue.Task{
        ID:       "task-456",
        Type:     "embedding-generation",
        Priority: 1,
        Payload: map[string]interface{}{
            "text":  "Generate embeddings for this text",
            "model": "amazon.titan-embed-text-v1",
        },
    }
    
    // Enqueue task
    err = taskQueue.Enqueue(ctx, task)
    if err != nil {
        panic(err)
    }
}
```

## AWS Bedrock Integration

### Generate Embeddings with Bedrock

```go
package main

import (
    "context"
    "encoding/json"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/yourorg/developer-mesh/pkg/embedding"
)

func ExampleBedrockEmbeddings() {
    ctx := context.Background()
    
    // Load AWS configuration
    cfg, err := config.LoadDefaultConfig(ctx,
        config.WithRegion("us-east-1"),
    )
    if err != nil {
        panic(err)
    }
    
    // Create Bedrock Runtime client
    bedrockClient := bedrockruntime.NewFromConfig(cfg)
    
    // Initialize embedding service
    embeddingService := embedding.NewBedrockEmbeddingService(bedrockClient, embedding.Config{
        DefaultModel:  "amazon.titan-embed-text-v1",
        SessionLimit:  0.10,
        GlobalLimit:   10.0,
    })
    
    // Generate embeddings
    text := "Developer Mesh is an AI agent orchestration platform"
    embeddings, err := embeddingService.GenerateEmbedding(ctx, text, "")
    if err != nil {
        panic(err)
    }
    
    fmt.Printf("Generated %d-dimensional embedding\n", len(embeddings))
}
```

## ElastiCache Redis (via SSH Tunnel)

### Setting up SSH Tunnel

```bash
# Start SSH tunnel to ElastiCache
./scripts/aws/connect-elasticache.sh

# This creates a tunnel from localhost:6379 to your ElastiCache instance
# Keep this running in a separate terminal
```

### Using ElastiCache in Code

```go
package main

import (
    "context"
    "github.com/redis/go-redis/v9"
    "github.com/yourorg/developer-mesh/pkg/cache"
)

func ExampleElastiCache() {
    ctx := context.Background()
    
    // Connect to Redis via SSH tunnel
    redisClient := redis.NewClient(&redis.Options{
        Addr:     "127.0.0.1:6379", // IMPORTANT: Use 127.0.0.1, not localhost
        Password: "",               // No password for ElastiCache
        DB:       0,
    })
    
    // Test connection
    _, err := redisClient.Ping(ctx).Result()
    if err != nil {
        panic(err)
    }
    
    // Initialize cache
    cacheService := cache.NewRedisCache(redisClient)
    
    // Cache some data
    err = cacheService.Set(ctx, "agent:status:prod-1", "active", 5*time.Minute)
    if err != nil {
        panic(err)
    }
}
```

## Complete Production Configuration Example

### config.yaml for Production

```yaml
environment: "production"

api:
  listen_address: ":8080"
  auth:
    jwt_secret: "${JWT_SECRET}"  # From environment/secrets
    api_keys:
      admin: "${ADMIN_API_KEY}"
      
database:
  driver: "postgres"
  dsn: "${DATABASE_URL}"  # RDS connection string
  max_open_conns: 50
  max_idle_conns: 10
  
cache:
  type: "redis"
  address: "127.0.0.1:6379"  # Via SSH tunnel
  pool_size: 20
  
aws:
  region: "us-east-1"
  
  elasticache:
    endpoint: "sean-mcp-test-qem3fz.serverless.use1.cache.amazonaws.com:6379"
    
  s3:
    bucket: "sean-mcp-dev-contexts"
    
  sqs:
    queue_url: "https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test"
    
  bedrock:
    enabled: true
    
storage:
  context:
    provider: "s3"
    
worker:
  enabled: true
  queue_type: "sqs"
  concurrency: 20
```

## Testing AWS Connectivity

### Test Script

```bash
#!/bin/bash
# test-aws-services.sh

echo "Testing AWS Services..."

# Test S3
echo "Testing S3..."
aws s3 ls s3://sean-mcp-dev-contexts --region us-east-1

# Test SQS
echo "Testing SQS..."
aws sqs get-queue-attributes \
  --queue-url https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test \
  --attribute-names All \
  --region us-east-1

# Test Bedrock
echo "Testing Bedrock..."
aws bedrock list-foundation-models --region us-east-1

# Test Redis (via tunnel)
echo "Testing Redis..."
redis-cli -h 127.0.0.1 -p 6379 ping
```

## Cost Management

### Monitoring AWS Costs

```go
// Example cost tracking middleware
func CostTrackingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        ctx := r.Context()
        
        // Track Bedrock costs
        ctx = context.WithValue(ctx, "bedrock.session_limit", 0.10)
        ctx = context.WithValue(ctx, "bedrock.global_limit", 10.0)
        
        // Monitor S3 operations
        ctx = context.WithValue(ctx, "s3.request_tracking", true)
        
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Production Best Practices

1. **Use IAM Roles**: Instead of access keys, use IAM roles for EC2/ECS
2. **Enable Encryption**: Use S3 encryption and TLS for all connections
3. **Set Resource Limits**: Configure Bedrock cost limits per session
4. **Monitor Usage**: Track AWS CloudWatch metrics
5. **Use VPC Endpoints**: For private S3/SQS access
6. **Enable Logging**: CloudTrail for audit logging
7. **Regular Backups**: Automated RDS snapshots

## Troubleshooting

### Common Issues

1. **ElastiCache Connection Failed**
   - Ensure SSH tunnel is running
   - Use `127.0.0.1` not `localhost`
   - Check security group allows connection

2. **S3 Access Denied**
   - Verify IAM permissions
   - Check bucket policy
   - Ensure correct region

3. **SQS Message Not Processing**
   - Check queue exists
   - Verify IAM permissions for SQS
   - Monitor dead letter queue

4. **Bedrock Rate Limits**
   - Implement exponential backoff
   - Use different models for load distribution
   - Monitor usage against quotas

---

For more examples, see the [integration guide](../guides/integration-guide.md).