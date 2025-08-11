# Complete AI Agent Integration Guide

> **Purpose**: Step-by-step guide for integrating AI agents with the Developer Mesh platform
> **Audience**: Developers implementing AI agent systems from start to finish
> **Scope**: Full integration lifecycle, from setup to production deployment

## Table of Contents

1. [Prerequisites](#prerequisites)
2. [Environment Setup](#environment-setup)
3. [Agent Development](#agent-development)
4. [Integration Steps](#integration-steps)
5. [Testing & Validation](#testing--validation)
6. [Deployment](#deployment)
7. [Monitoring & Maintenance](#monitoring--maintenance)

## Prerequisites

### Required Knowledge
- Go programming (intermediate level)
- WebSocket protocols
- AI/ML concepts (basic understanding)
- AWS services (S3, SQS, Bedrock)
- Docker and containerization

### Required Tools
```bash
# Install required tools
brew install go@1.23
brew install docker
brew install awscli
brew install jq

# Verify installations
go version  # Should be 1.23+
docker --version
aws --version
```

### AWS Setup
```bash
# Configure AWS credentials
aws configure

# Verify access to required services
aws s3 ls s3://sean-mcp-dev-contexts
aws sqs list-queues
aws bedrock list-foundation-models --region us-east-1
```

## Environment Setup

### 1. Clone and Setup Repository

```bash
# Clone the repository
git clone https://github.com/developer-mesh/developer-mesh.git
cd developer-mesh

# Install dependencies
make deps

# Setup environment
cp .env.example .env
# Edit .env with your configuration

# Start ElastiCache tunnel (keep running)
./scripts/aws/connect-elasticache.sh

# Verify AWS connectivity
./scripts/aws/test-aws-services.sh
```

### 2. Local Development Environment

```bash
# Build all services
make build

# Run tests to verify setup
make test

# Start local development
make dev-native
```

### 3. Database Setup

```sql
-- Run migrations
make migrate-up

-- Verify pgvector extension
SELECT * FROM pg_extension WHERE extname = 'vector';

-- Check agent tables
SELECT table_name FROM information_schema.tables 
WHERE table_schema = 'public' 
AND table_name LIKE 'agent%';
```

## Agent Development

### Step 1: Define Agent Structure

```go
// agent.go
package myagent

import (
    "github.com/developer-mesh/developer-mesh/pkg/agents"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

// MyCustomAgent represents your AI agent
type MyCustomAgent struct {
    ID           string
    Name         string
    Type         agents.AgentType
    Capabilities []agents.Capability
    Config       *agents.AgentConfig
    client       *AgentClient
}

// NewMyCustomAgent creates a new instance
func NewMyCustomAgent(name string) *MyCustomAgent {
    return &MyCustomAgent{
        ID:   generateAgentID(name),
        Name: name,
        Type: agents.AgentTypeCustom,
        Capabilities: defineCapabilities(),
        Config: createDefaultConfig(),
    }
}

// Define agent capabilities
func defineCapabilities() []agents.Capability {
    return []agents.Capability{
        {
            Name:       "code_analysis",
            Confidence: 0.92,
            TaskTypes:  []agents.TaskType{
                agents.TaskTypeCodeReview,
                agents.TaskTypeBugDetection,
            },
            Languages: []string{"go", "python", "javascript"},
            Specialties: []string{
                "performance",
                "security",
                "best-practices",
            },
        },
        {
            Name:       "documentation",
            Confidence: 0.88,
            TaskTypes:  []agents.TaskType{
                agents.TaskTypeDocGeneration,
            },
            Specialties: []string{
                "api-docs",
                "readme",
                "architecture",
            },
        },
    }
}
```

### Step 2: Implement Agent Configuration

```go
// config.go
package myagent

import (
    "github.com/developer-mesh/developer-mesh/pkg/agents"
)

func createDefaultConfig() *agents.AgentConfig {
    return &agents.AgentConfig{
        AgentID:           generateAgentID("custom"),
        Version:           1,
        EmbeddingStrategy: agents.StrategyBalanced,
        ModelPreferences: []agents.ModelPreference{
            {
                TaskType:       agents.TaskTypeCodeAnalysis,
                PrimaryModels:  []string{"gpt-4", "claude-2"},
                FallbackModels: []string{"gpt-3.5-turbo"},
                Weight:         0.8,
            },
            {
                TaskType:       agents.TaskTypeGeneralQA,
                PrimaryModels:  []string{"claude-2"},
                FallbackModels: []string{"gpt-3.5-turbo"},
                Weight:         0.2,
            },
        },
        Constraints: agents.AgentConstraints{
            MaxCostPerMonthUSD: 100.0,
            MaxLatencyP99Ms:    5000,
            MinAvailabilitySLA: 0.99,
            RateLimits: agents.RateLimitConfig{
                RequestsPerMinute:  60,
                TokensPerHour:      100000,
                ConcurrentRequests: 10,
            },
            QualityThresholds: agents.QualityConfig{
                MinCosineSimilarity:   0.8,
                MinEmbeddingMagnitude: 0.5,
                AcceptableErrorRate:   0.01,
            },
        },
        FallbackBehavior: agents.FallbackConfig{
            MaxRetries:      3,
            InitialDelayMs:  1000,
            MaxDelayMs:      10000,
            ExponentialBase: 2.0,
            QueueOnFailure:  true,
            CircuitBreaker: agents.CircuitConfig{
                Enabled:          true,
                FailureThreshold: 5,
                SuccessThreshold: 2,
                TimeoutSeconds:   30,
            },
        },
        IsActive:  true,
        CreatedBy: "system",
    }
}
```

### Step 3: Implement WebSocket Client

```go
// client.go
package myagent

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "time"
    
    "github.com/gorilla/websocket"
)

type AgentClient struct {
    conn      *websocket.Conn
    agent     *MyCustomAgent
    msgChan   chan Message
    errorChan chan error
}

// Connect establishes WebSocket connection
func (a *MyCustomAgent) Connect(serverURL, apiKey string) error {
    headers := http.Header{
        "Authorization": []string{fmt.Sprintf("Bearer %s", apiKey)},
        "X-Agent-ID":    []string{a.ID},
        "X-Agent-Type":  []string{string(a.Type)},
    }
    
    conn, _, err := websocket.DefaultDialer.Dial(serverURL, headers)
    if err != nil {
        return fmt.Errorf("websocket connection failed: %w", err)
    }
    
    a.client = &AgentClient{
        conn:      conn,
        agent:     a,
        msgChan:   make(chan Message, 100),
        errorChan: make(chan error, 10),
    }
    
    // Start message handlers
    go a.client.readPump()
    go a.client.writePump()
    
    // Register agent
    return a.register()
}

// Register agent with MCP server
func (a *MyCustomAgent) register() error {
    msg := RegistrationMessage{
        Type:         "agent.register",
        AgentID:      a.ID,
        AgentName:    a.Name,
        AgentType:    a.Type,
        Capabilities: a.Capabilities,
        Config:       a.Config,
        Timestamp:    time.Now(),
    }
    
    return a.client.sendMessage(msg)
}

// Message handling
func (c *AgentClient) readPump() {
    defer close(c.msgChan)
    
    for {
        var msg Message
        err := c.conn.ReadJSON(&msg)
        if err != nil {
            c.errorChan <- fmt.Errorf("read error: %w", err)
            return
        }
        
        c.msgChan <- msg
    }
}

func (c *AgentClient) writePump() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            // Send heartbeat
            c.sendHeartbeat()
        }
    }
}
```

### Step 4: Implement Task Processing

```go
// task_processor.go
package myagent

import (
    "context"
    "fmt"
    "time"
)

// ProcessTask handles incoming tasks
func (a *MyCustomAgent) ProcessTask(ctx context.Context, task Task) (*TaskResult, error) {
    // Start processing span
    ctx, span := tracer.Start(ctx, "agent.process_task",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    // Validate task
    if err := a.validateTask(task); err != nil {
        return nil, fmt.Errorf("task validation failed: %w", err)
    }
    
    // Route to appropriate handler
    var result *TaskResult
    var err error
    
    switch task.Type {
    case "code_review":
        result, err = a.processCodeReview(ctx, task)
    case "documentation":
        result, err = a.processDocumentation(ctx, task)
    case "analysis":
        result, err = a.processAnalysis(ctx, task)
    default:
        return nil, fmt.Errorf("unsupported task type: %s", task.Type)
    }
    
    if err != nil {
        span.RecordError(err)
        return nil, err
    }
    
    // Update metrics
    a.updateMetrics(task, result)
    
    return result, nil
}

// Code review implementation
func (a *MyCustomAgent) processCodeReview(ctx context.Context, task Task) (*TaskResult, error) {
    codeReview := task.Payload.(CodeReviewRequest)
    
    // Phase 1: Static analysis
    staticResults := a.performStaticAnalysis(codeReview.Code)
    
    // Phase 2: AI-powered review
    aiReview, err := a.performAIReview(ctx, codeReview)
    if err != nil {
        return nil, err
    }
    
    // Phase 3: Generate recommendations
    recommendations := a.generateRecommendations(staticResults, aiReview)
    
    return &TaskResult{
        TaskID:    task.ID,
        AgentID:   a.ID,
        Status:    "completed",
        Result:    recommendations,
        Metadata: map[string]interface{}{
            "static_issues": len(staticResults.Issues),
            "ai_suggestions": len(aiReview.Suggestions),
            "confidence": aiReview.Confidence,
        },
        CompletedAt: time.Now(),
    }, nil
}
```

### Step 5: Implement AI Model Integration

```go
// model_integration.go
package myagent

import (
    "context"
    "fmt"
    
    "github.com/developer-mesh/developer-mesh/pkg/bedrock"
)

type ModelIntegration struct {
    bedrock  *bedrock.Client
    modelID  string
    strategy ModelStrategy
}

// AI-powered review using Bedrock
func (a *MyCustomAgent) performAIReview(ctx context.Context, review CodeReviewRequest) (*AIReview, error) {
    // Select model based on task
    model := a.selectModel(review)
    
    // Build prompt
    prompt := a.buildReviewPrompt(review)
    
    // Call AI model
    response, err := a.callModel(ctx, model, prompt)
    if err != nil {
        return nil, fmt.Errorf("model call failed: %w", err)
    }
    
    // Parse response
    return a.parseAIResponse(response)
}

func (a *MyCustomAgent) buildReviewPrompt(review CodeReviewRequest) string {
    return fmt.Sprintf(`
You are an expert code reviewer. Analyze the following code and provide:
1. Security vulnerabilities
2. Performance issues
3. Best practice violations
4. Improvement suggestions

Language: %s
Code:
%s

Provide structured JSON response with findings.
`, review.Language, review.Code)
}

func (a *MyCustomAgent) callModel(ctx context.Context, modelID string, prompt string) (string, error) {
    request := &bedrock.InvokeModelInput{
        ModelId: modelID,
        Body: map[string]interface{}{
            "prompt": prompt,
            "max_tokens": 2000,
            "temperature": 0.3,
        },
    }
    
    response, err := a.client.bedrock.InvokeModel(ctx, request)
    if err != nil {
        return "", err
    }
    
    return response.Body, nil
}
```

## Integration Steps

### Step 1: Agent Registration

```go
// main.go - Agent entry point
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    
    "github.com/developer-mesh/developer-mesh/myagent"
)

func main() {
    // Create agent
    agent := myagent.NewMyCustomAgent("code-review-specialist")
    
    // Connect to MCP server
    serverURL := os.Getenv("MCP_SERVER_URL")
    apiKey := os.Getenv("MCP_API_KEY")
    
    if err := agent.Connect(serverURL, apiKey); err != nil {
        log.Fatalf("Failed to connect: %v", err)
    }
    
    log.Printf("Agent %s connected successfully", agent.ID)
    
    // Start task processing
    ctx, cancel := context.WithCancel(context.Background())
    go agent.StartTaskProcessing(ctx)
    
    // Wait for shutdown signal
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan
    
    log.Println("Shutting down agent...")
    cancel()
    agent.Disconnect()
}
```

### Step 2: Task Processing Loop

```go
// task_loop.go
func (a *MyCustomAgent) StartTaskProcessing(ctx context.Context) {
    for {
        select {
        case msg := <-a.client.msgChan:
            go a.handleMessage(ctx, msg)
            
        case err := <-a.client.errorChan:
            log.Printf("Error: %v", err)
            a.handleError(err)
            
        case <-ctx.Done():
            log.Println("Task processing stopped")
            return
        }
    }
}

func (a *MyCustomAgent) handleMessage(ctx context.Context, msg Message) {
    switch msg.Type {
    case "task.assigned":
        task := msg.Payload.(Task)
        a.processTaskAsync(ctx, task)
        
    case "config.update":
        config := msg.Payload.(AgentConfig)
        a.updateConfiguration(config)
        
    case "health.check":
        a.sendHealthStatus()
        
    case "capability.query":
        a.reportCapabilities()
    }
}
```

### Step 3: State Management

```go
// state.go
type AgentState struct {
    mu              sync.RWMutex
    status          AgentStatus
    activeTasks     map[string]*Task
    completedTasks  int64
    failedTasks     int64
    metrics         *AgentMetrics
}

func (s *AgentState) UpdateTaskStatus(taskID string, status TaskStatus) {
    s.mu.Lock()
    defer s.mu.Unlock()
    
    if task, exists := s.activeTasks[taskID]; exists {
        task.Status = status
        
        if status == TaskStatusCompleted {
            s.completedTasks++
            delete(s.activeTasks, taskID)
        } else if status == TaskStatusFailed {
            s.failedTasks++
            delete(s.activeTasks, taskID)
        }
    }
    
    // Update metrics
    s.metrics.Update(status)
}
```

### Step 4: Error Handling

```go
// error_handling.go
func (a *MyCustomAgent) handleError(err error) {
    // Classify error
    errorType := classifyError(err)
    
    switch errorType {
    case ErrorTypeConnection:
        a.handleConnectionError(err)
        
    case ErrorTypeAuthentication:
        a.handleAuthError(err)
        
    case ErrorTypeTaskProcessing:
        a.handleTaskError(err)
        
    case ErrorTypeRateLimit:
        a.handleRateLimitError(err)
    }
    
    // Report to monitoring
    a.reportError(err, errorType)
}

func (a *MyCustomAgent) handleConnectionError(err error) {
    log.Printf("Connection error: %v", err)
    
    // Implement exponential backoff
    backoff := time.Second
    maxBackoff := time.Minute * 5
    
    for {
        time.Sleep(backoff)
        
        if err := a.reconnect(); err == nil {
            log.Println("Reconnected successfully")
            return
        }
        
        backoff *= 2
        if backoff > maxBackoff {
            backoff = maxBackoff
        }
    }
}
```

## Testing & Validation

### Step 1: Unit Tests

```go
// agent_test.go
package myagent

import (
    "context"
    "testing"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAgentCreation(t *testing.T) {
    agent := NewMyCustomAgent("test-agent")
    
    assert.NotEmpty(t, agent.ID)
    assert.Equal(t, "test-agent", agent.Name)
    assert.NotNil(t, agent.Config)
    assert.Len(t, agent.Capabilities, 2)
}

func TestTaskProcessing(t *testing.T) {
    agent := NewMyCustomAgent("test-agent")
    
    task := Task{
        ID:   "test-task-1",
        Type: "code_review",
        Payload: CodeReviewRequest{
            Language: "go",
            Code:     "func main() { fmt.Println(\"Hello\") }",
        },
    }
    
    result, err := agent.ProcessTask(context.Background(), task)
    require.NoError(t, err)
    assert.Equal(t, "completed", result.Status)
    assert.NotNil(t, result.Result)
}
```

### Step 2: Integration Tests

```go
// integration_test.go
func TestAgentIntegration(t *testing.T) {
    // Start test MCP server
    server := startTestServer(t)
    defer server.Close()
    
    // Create and connect agent
    agent := NewMyCustomAgent("integration-test")
    err := agent.Connect(server.URL, "test-api-key")
    require.NoError(t, err)
    defer agent.Disconnect()
    
    // Test registration
    assert.Eventually(t, func() bool {
        return server.IsAgentRegistered(agent.ID)
    }, 5*time.Second, 100*time.Millisecond)
    
    // Test task processing
    task := createTestTask()
    server.AssignTask(agent.ID, task)
    
    assert.Eventually(t, func() bool {
        return server.IsTaskCompleted(task.ID)
    }, 10*time.Second, 100*time.Millisecond)
}
```

### Step 3: Load Testing

```go
// load_test.go
func TestAgentLoadHandling(t *testing.T) {
    agent := NewMyCustomAgent("load-test")
    
    // Simulate concurrent tasks
    numTasks := 100
    var wg sync.WaitGroup
    errors := make(chan error, numTasks)
    
    for i := 0; i < numTasks; i++ {
        wg.Add(1)
        go func(taskNum int) {
            defer wg.Done()
            
            task := generateTestTask(taskNum)
            _, err := agent.ProcessTask(context.Background(), task)
            if err != nil {
                errors <- err
            }
        }(i)
    }
    
    wg.Wait()
    close(errors)
    
    // Check for errors
    var errorCount int
    for err := range errors {
        t.Logf("Task error: %v", err)
        errorCount++
    }
    
    assert.Less(t, errorCount, 5, "Too many errors under load")
}
```

## Deployment

### Step 1: Containerization

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent ./cmd/agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/agent .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./agent"]
```

### Step 2: Kubernetes Deployment

```yaml
# k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-agent
  namespace: mcp
spec:
  replicas: 3
  selector:
    matchLabels:
      app: ai-agent
  template:
    metadata:
      labels:
        app: ai-agent
    spec:
      containers:
      - name: agent
        image: your-registry/ai-agent:latest
        env:
        - name: MCP_SERVER_URL
          value: "wss://mcp-server.mcp.svc.cluster.local:8080/ws"
        - name: MCP_API_KEY
          valueFrom:
            secretKeyRef:
              name: mcp-secrets
              key: api-key
        - name: AWS_REGION
          value: "us-east-1"
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "500m"
        livenessProbe:
          httpGet:
            path: /health
            port: 8080
          initialDelaySeconds: 30
          periodSeconds: 10
        readinessProbe:
          httpGet:
            path: /ready
            port: 8080
          initialDelaySeconds: 5
          periodSeconds: 5
```

### Step 3: CI/CD Pipeline

```yaml
# .github/workflows/agent-deploy.yaml
name: Deploy AI Agent

on:
  push:
    branches: [main]
    paths:
      - 'myagent/**'
      - 'cmd/agent/**'

jobs:
  test:
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Set up Go
      uses: actions/setup-go@v4
      with:
        go-version: '1.23'
    
    - name: Test
      run: |
        go test -v ./myagent/...
        go test -race -coverprofile=coverage.txt ./myagent/...
    
    - name: Upload coverage
      uses: codecov/codecov-action@v3

  build:
    needs: test
    runs-on: ubuntu-latest
    steps:
    - uses: actions/checkout@v3
    
    - name: Build and push Docker image
      env:
        DOCKER_REGISTRY: ${{ secrets.DOCKER_REGISTRY }}
      run: |
        docker build -t $DOCKER_REGISTRY/ai-agent:$GITHUB_SHA .
        docker push $DOCKER_REGISTRY/ai-agent:$GITHUB_SHA
    
    - name: Update Kubernetes deployment
      env:
        KUBE_CONFIG: ${{ secrets.KUBE_CONFIG }}
      run: |
        echo "$KUBE_CONFIG" | base64 -d > kubeconfig
        export KUBECONFIG=kubeconfig
        kubectl set image deployment/ai-agent agent=$DOCKER_REGISTRY/ai-agent:$GITHUB_SHA -n mcp
```

## Monitoring & Maintenance

### Step 1: Metrics Collection

```go
// metrics.go
var (
    tasksProcessed = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "agent_tasks_processed_total",
            Help: "Total number of tasks processed",
        },
        []string{"agent_id", "task_type", "status"},
    )
    
    taskDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name: "agent_task_duration_seconds",
            Help: "Task processing duration",
        },
        []string{"agent_id", "task_type"},
    )
    
    agentHealth = prometheus.NewGaugeVec(
        prometheus.GaugeOpts{
            Name: "agent_health_status",
            Help: "Agent health status (1=healthy, 0=unhealthy)",
        },
        []string{"agent_id"},
    )
)

func (a *MyCustomAgent) updateMetrics(task Task, result *TaskResult) {
    tasksProcessed.WithLabelValues(a.ID, task.Type, result.Status).Inc()
    taskDuration.WithLabelValues(a.ID, task.Type).Observe(result.Duration.Seconds())
}
```

### Step 2: Health Monitoring

```go
// health.go
func (a *MyCustomAgent) StartHealthMonitoring(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            health := a.checkHealth()
            a.reportHealth(health)
            
        case <-ctx.Done():
            return
        }
    }
}

func (a *MyCustomAgent) checkHealth() HealthStatus {
    status := HealthStatus{
        AgentID:   a.ID,
        Timestamp: time.Now(),
        Healthy:   true,
    }
    
    // Check connection
    if !a.client.IsConnected() {
        status.Healthy = false
        status.Issues = append(status.Issues, "WebSocket disconnected")
    }
    
    // Check task queue
    queueSize := len(a.state.activeTasks)
    if queueSize > MaxQueueSize {
        status.Healthy = false
        status.Issues = append(status.Issues, "Task queue overloaded")
    }
    
    // Check error rate
    errorRate := a.calculateErrorRate()
    if errorRate > MaxErrorRate {
        status.Healthy = false
        status.Issues = append(status.Issues, "High error rate")
    }
    
    // Update metric
    healthValue := 1.0
    if !status.Healthy {
        healthValue = 0.0
    }
    agentHealth.WithLabelValues(a.ID).Set(healthValue)
    
    return status
}
```

### Step 3: Logging

```go
// logging.go
func setupLogging() *zap.Logger {
    config := zap.NewProductionConfig()
    config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    
    logger, _ := config.Build(
        zap.AddCaller(),
        zap.AddStacktrace(zap.ErrorLevel),
    )
    
    return logger
}

func (a *MyCustomAgent) logTaskCompletion(task Task, result *TaskResult) {
    a.logger.Info("Task completed",
        zap.String("task_id", task.ID),
        zap.String("task_type", task.Type),
        zap.String("status", result.Status),
        zap.Duration("duration", result.Duration),
        zap.Any("metadata", result.Metadata),
    )
}
```

## Best Practices

### 1. Error Recovery
- Implement circuit breakers for external calls
- Use exponential backoff for retries
- Log all errors with context
- Graceful degradation on failures

### 2. Performance
- Use connection pooling
- Implement caching where appropriate
- Batch similar operations
- Monitor resource usage

### 3. Security
- Rotate API keys regularly
- Validate all inputs
- Use TLS for all communications
- Implement rate limiting

### 4. Maintenance
- Regular health checks
- Automated testing
- Performance profiling
- Documentation updates

## Troubleshooting

### Common Issues

1. **Connection Failures**
   ```bash
   # Check network connectivity
   curl -v wss://mcp-server:8080/ws
   
   # Verify API key
   echo $MCP_API_KEY | base64 -d
   ```

2. **Task Processing Errors**
   ```go
   // Enable debug logging
   logger.SetLevel(zap.DebugLevel)
   
   // Add detailed error context
   return fmt.Errorf("task processing failed: %w", err)
   ```

3. **Performance Issues**
   ```bash
   # Profile CPU usage
   go tool pprof http://localhost:6060/debug/pprof/profile
   
   # Check memory usage
   go tool pprof http://localhost:6060/debug/pprof/heap
   ```

## Next Steps

1. Review [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details
2. Explore [Agent Integration Examples](./agent-integration-examples.md) for more patterns
3. See [Agent SDK Guide](./agent-sdk-guide.md) for SDK usage
4. Check [Agent Integration Troubleshooting](./agent-integration-troubleshooting.md) for issues

## Resources

- [MCP Documentation](https://docs.mcp.dev)
- [Go WebSocket Programming](https://github.com/gorilla/websocket)
- [AWS Bedrock SDK](https://docs.aws.amazon.com/bedrock/latest/userguide/api-reference.html)
- [Kubernetes Deployment Guide](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)