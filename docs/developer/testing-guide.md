<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:42:27
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Testing Guide

> **Purpose**: Comprehensive testing strategies and patterns for the Developer Mesh platform
> **Audience**: Developers writing and maintaining tests
> **Scope**: Unit, integration, E2E tests with real AWS services

## Overview

This guide covers testing strategies, tools, and best practices for Developer Mesh with emphasis on testing against real AWS services (S3, Bedrock, ElastiCache) and Redis Streams. Note that many advanced testing tools mentioned (Cypress, Playwright, Gatling, Pact) are not currently implemented but shown as examples of potential testing approaches.

## Testing Pyramid

```
         /\
        /  \    E2E Tests (5%)
       /    \   - User journeys
      /      \  - Critical paths
     /________\ 
    /          \ Integration Tests (20%)
   /            \ - API contracts
  /              \ - Database interactions
 /________________\ Unit Tests (75%)
                    - Business logic
                    - Pure functions
```

## Testing Philosophy

### Core Principles

1. **Test with Real Services**: Use actual AWS services, not mocks, for integration tests
2. **Test Coverage**: Maintain >85% coverage across all packages
3. **Fast Feedback**: Unit tests should run in <30 seconds
4. **Deterministic**: Tests must be repeatable and reliable
5. **Isolated**: Tests should not depend on each other

### Test Categories

| Type | Purpose | AWS Services | Runtime | Coverage |
|------|---------|--------------|---------|----------|
| Unit | Business logic | Mocked | <30s | 85%+ |
| Integration | Service boundaries | Real AWS | <5min | Key flows |
| E2E | User journeys | Real AWS | <15min | Critical paths |
| Load | Performance | Real AWS | Variable | Benchmarks |

## Unit Testing

### Go Unit Tests

```go
// pkg/adapters/github/adapter_test.go
package github_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

func TestGitHubAdapter_CreateIssue(t *testing.T) {
    // Arrange
    mockClient := new(MockGitHubClient)
    adapter := &GitHubAdapter{client: mockClient}
    
    expectedIssue := &Issue{
        ID:     123,
        Number: 42,
        Title:  "Test Issue",
    }
    
    mockClient.On("CreateIssue", mock.Anything, "owner/repo", mock.Anything).
        Return(expectedIssue, nil)
    
    // Act
    issue, err := adapter.CreateIssue(context.Background(), "owner/repo", "Test Issue", "Test body")
    
    // Assert
    assert.NoError(t, err)
    assert.Equal(t, expectedIssue, issue)
    mockClient.AssertExpectations(t)
}

// Table-driven tests
func TestValidateWebhookSignature(t *testing.T) {
    tests := []struct {
        name      string
        payload   string
        signature string
        secret    string
        want      bool
    }{
        {
            name:      "valid signature",
            payload:   `{"action":"opened"}`,
            signature: "sha256=valid_signature_here",
            secret:    "webhook_secret",
            want:      true,
        },
        {
            name:      "invalid signature",
            payload:   `{"action":"opened"}`,
            signature: "sha256=invalid",
            secret:    "webhook_secret",
            want:      false,
        },
        {
            name:      "empty signature",
            payload:   `{"action":"opened"}`,
            signature: "",
            secret:    "webhook_secret",
            want:      false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            got := ValidateWebhookSignature([]byte(tt.payload), tt.signature, tt.secret)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Test Coverage Requirements

```bash
# Run tests with coverage (must be >85%)
make test-coverage

# Generate HTML coverage report
make test-coverage-html

# View coverage in browser
open coverage.html

# Check specific package coverage
go test -cover ./pkg/services/...

# Fail if coverage below threshold
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//g' | awk '{if ($1 < 85) exit 1}'
```

### Mocking Best Practices

```go
// Use interfaces for mockability
type GitHubClient interface {
    CreateIssue(ctx context.Context, repo string, issue *IssueRequest) (*Issue, error)
    GetIssue(ctx context.Context, repo string, number int) (*Issue, error)
}

// Generate mocks
//go:generate mockery --name=GitHubClient --output=mocks --filename=github_client.go

// Use testify/mock for assertions
func TestWithMock(t *testing.T) {
    mockClient := new(mocks.GitHubClient)
    
    // Set expectations
    mockClient.On("GetIssue", mock.Anything, "owner/repo", 123).
        Return(&Issue{ID: 123}, nil).
        Once() // Ensure called exactly once
    
    // Your test code here
    
    // Verify all expectations met
    mockClient.AssertExpectations(t)
}
```

## Integration Testing with AWS

### AWS Service Integration Tests

```go
// tests/integration/aws_integration_test.go
// +build integration

package integration_test

import (
    "context"
    "testing"
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/s3"
    "github.com/aws/aws-sdk-go-v2/service/sqs"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
    "github.com/stretchr/testify/suite"
)

type AWSIntegrationTestSuite struct {
    suite.Suite
    ctx      context.Context
    s3Client *s3.Client
    sqsClient *sqs.Client
    bedrockClient *bedrockruntime.Client
    testBucket string
    testQueue  string
}

func (s *AWSIntegrationTestSuite) SetupSuite() {
    // Skip if not in CI or AWS credentials not available
    if os.Getenv("RUN_AWS_INTEGRATION_TESTS") != "true" {
        s.T().Skip("AWS integration tests disabled")
    }
    
    // Note: Some tests may use LocalStack if configured
    // Check AWS_ENDPOINT_URL environment variable
    
    s.ctx = context.Background()
    
    // Load AWS config
    cfg, err := config.LoadDefaultConfig(s.ctx,
        config.WithRegion("us-east-1"),
    )
    s.Require().NoError(err)
    
    // Initialize clients
    s.s3Client = s3.NewFromConfig(cfg)
    s.sqsClient = sqs.NewFromConfig(cfg)
    s.bedrockClient = bedrockruntime.NewFromConfig(cfg)
    
    // Use test resources
    s.testBucket = os.Getenv("TEST_S3_BUCKET") // sean-mcp-dev-contexts
    s.testQueue = os.Getenv("TEST_SQS_QUEUE")   // https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
}

func (s *AWSIntegrationTestSuite) TestS3Operations() {
    key := fmt.Sprintf("test/integration-%d.json", time.Now().Unix())
    content := []byte(`{"test": "data"}`)
    
    // Upload to S3
    _, err := s.s3Client.PutObject(s.ctx, &s3.PutObjectInput{
        Bucket: aws.String(s.testBucket),
        Key:    aws.String(key),
        Body:   bytes.NewReader(content),
    })
    s.Require().NoError(err)
    
    // Download from S3
    result, err := s.s3Client.GetObject(s.ctx, &s3.GetObjectInput{
        Bucket: aws.String(s.testBucket),
        Key:    aws.String(key),
    })
    s.Require().NoError(err)
    defer result.Body.Close()
    
    downloaded, err := io.ReadAll(result.Body)
    s.Require().NoError(err)
    s.Equal(content, downloaded)
    
    // Cleanup
    _, err = s.s3Client.DeleteObject(s.ctx, &s3.DeleteObjectInput{
        Bucket: aws.String(s.testBucket),
        Key:    aws.String(key),
    })
    s.Require().NoError(err)
}

func (s *AWSIntegrationTestSuite) TestSQSMessageFlow() {
    messageBody := fmt.Sprintf(`{"taskId": "test-%d", "type": "integration-test"}`, time.Now().Unix())
    
    // Send message
    sendResult, err := s.sqsClient.SendMessage(s.ctx, &sqs.SendMessageInput{
        QueueUrl:    aws.String(s.testQueue),
        MessageBody: aws.String(messageBody),
        MessageAttributes: map[string]types.MessageAttributeValue{
            "TraceID": {
                DataType:    aws.String("String"),
                StringValue: aws.String("test-trace-123"),
            },
        },
    })
    s.Require().NoError(err)
    s.NotEmpty(sendResult.MessageId)
    
    // Receive message
    receiveResult, err := s.sqsClient.ReceiveMessage(s.ctx, &sqs.ReceiveMessageInput{
        QueueUrl:            aws.String(s.testQueue),
        MaxNumberOfMessages: 1,
        WaitTimeSeconds:     5,
        MessageAttributeNames: []string{"All"},
    })
    s.Require().NoError(err)
    s.Require().Len(receiveResult.Messages, 1)
    
    msg := receiveResult.Messages[0]
    s.Equal(messageBody, *msg.Body)
    s.Equal("test-trace-123", *msg.MessageAttributes["TraceID"].StringValue)
    
    // Delete message
    _, err = s.sqsClient.DeleteMessage(s.ctx, &sqs.DeleteMessageInput{
        QueueUrl:      aws.String(s.testQueue),
        ReceiptHandle: msg.ReceiptHandle,
    })
    s.Require().NoError(err)
}

func (s *AWSIntegrationTestSuite) TestBedrockEmbedding() {
    // Test with Titan embedding model
    input := map[string]interface{}{
        "inputText": "This is a test embedding for integration testing",
    }
    
    body, err := json.Marshal(input)
    s.Require().NoError(err)
    
    // Invoke model
    result, err := s.bedrockClient.InvokeModel(s.ctx, &bedrockruntime.InvokeModelInput{
        ModelId:     aws.String("amazon.titan-embed-text-v1"),
        ContentType: aws.String("application/json"),
        Body:        body,
    })
    
    // Check for rate limiting
    if err != nil && strings.Contains(err.Error(), "ThrottlingException") {
        s.T().Skip("Bedrock rate limited, skipping test")
    }
    s.Require().NoError(err)
    
    // Parse response
    var response map[string]interface{}
    err = json.Unmarshal(result.Body, &response)
    s.Require().NoError(err)
    
    embedding, ok := response["embedding"].([]interface{})
    s.Require().True(ok)
    s.Require().Equal(1536, len(embedding)) // Titan v1 dimension
}

### Database Integration Tests

```go
// tests/integration/repository_test.go
// +build integration

package integration_test

import (
    "context"
    "testing"
    "github.com/stretchr/testify/suite"
)

type RepositoryTestSuite struct {
    suite.Suite
    db   *sql.DB
    repo repository.Repository
}

func (s *RepositoryTestSuite) SetupSuite() {
    // Setup test database
    dsn := os.Getenv("TEST_DATABASE_URL")
    if dsn == "" {
        // Use local PostgreSQL for testing
        dsn = "postgres://postgres:postgres@localhost:5432 (PostgreSQL)/devops_mcp_test?sslmode=disable"
    }
    
    db, err := sql.Open("postgres", dsn)
    s.Require().NoError(err)
    
    // Run migrations
    migrator, err := migrate.New(
        "file://migrations",
        dsn,
    )
    s.Require().NoError(err)
    
    err = migrator.Up()
    if err != nil && err != migrate.ErrNoChange {
        s.Require().NoError(err)
    }
    
    s.db = db
    s.repo = repository.New(db)
}

func (s *RepositoryTestSuite) TearDownSuite() {
    s.db.Close()
}

func (s *RepositoryTestSuite) SetupTest() {
    // Clean database before each test
    _, err := s.db.Exec("TRUNCATE contexts, users CASCADE")
    s.Require().NoError(err)
}

func (s *RepositoryTestSuite) TestCreateContext() {
    // Test context creation with database
    ctx := context.Background()
    
    context := &models.Context{
        Name:     "Test Context",
        Content:  "Test content",
        TenantID: "test-tenant",
    }
    
    created, err := s.repo.CreateContext(ctx, context)
    s.Require().NoError(err)
    s.NotEmpty(created.ID)
    s.Equal(context.Name, created.Name)
    
    // Verify in database
    var count int
    err = s.db.QueryRow("SELECT COUNT(*) FROM contexts WHERE id = $1", created.ID).Scan(&count)
    s.Require().NoError(err)
    s.Equal(1, count)
}

func TestRepositoryTestSuite(t *testing.T) {
    suite.Run(t, new(RepositoryTestSuite))
}
```

### API Integration Tests with AWS

```go
// tests/integration/api_aws_test.go
// +build integration

func TestAPIWithAWSIntegration(t *testing.T) {
    // Ensure AWS services are available
    if os.Getenv("RUN_AWS_INTEGRATION_TESTS") != "true" {
        t.Skip("AWS integration tests disabled")
    }
    
    // Start ElastiCache tunnel
    require.Equal(t, "127.0.0.1:6379 (Redis)", os.Getenv("REDIS_ADDR"), "ElastiCache tunnel must be running")
    
    // Configure with real AWS services
    testConfig := &config.Config{
        DatabaseURL: os.Getenv("DATABASE_URL"),
        RedisAddr:   "127.0.0.1:6379 (Redis)", // Via SSH tunnel
        S3Bucket:    "sean-mcp-dev-contexts",
        SQSQueueURL: "https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test",
        BedrockEnabled: true,
    }
    
    // Start test server
    app := api.NewServer(testConfig)
    server := httptest.NewServer(app.Handler())
    defer server.Close()
    
    client := &http.Client{Timeout: 30 * time.Second}
    
    t.Run("Full context lifecycle with embeddings", func(t *testing.T) {
        // Create context with content for embedding
        payload := `{
            "name":"AWS Integration Test",
            "content":"This is a comprehensive test of the Developer Mesh platform including vector embeddings and search",
            "metadata": {"test": true}
        }`
        
        resp, err := client.Post(
            server.URL+"/api/v1/contexts",
            "application/json",
            strings.NewReader(payload),
        )
        require.NoError(t, err)
        defer resp.Body.Close()
        
        assert.Equal(t, http.StatusCreated, resp.StatusCode)
        
        var created models.Context
        err = json.NewDecoder(resp.Body).Decode(&created)
        require.NoError(t, err)
        
        // Wait for async embedding generation
        time.Sleep(5 * time.Second)
        
        // Search using vector similarity
        searchPayload := `{"query":"platform vector search","limit":10}`
        resp, err = client.Post(
            server.URL+"/api/v1/contexts/search",
            "application/json",
            strings.NewReader(searchPayload),
        )
        require.NoError(t, err)
        defer resp.Body.Close()
        
        var results []models.Context
        err = json.NewDecoder(resp.Body).Decode(&results)
        require.NoError(t, err)
        
        // Should find our context
        found := false
        for _, ctx := range results {
            if ctx.ID == created.ID {
                found = true
                break
            }
        }
        assert.True(t, found, "Created context should be found in search results")
        
        // Cleanup
        req, _ := http.NewRequest("DELETE", server.URL+"/api/v1/contexts/"+created.ID, nil)
        resp, err = client.Do(req)
        require.NoError(t, err)
        assert.Equal(t, http.StatusNoContent, resp.StatusCode)
    })
    
    t.Run("WebSocket agent registration", func(t *testing.T) { <!-- Source: pkg/models/websocket/binary.go -->
        wsURL := strings.Replace(server.URL, "http", "ws", 1) + "/ws"
        
        // Connect WebSocket <!-- Source: pkg/models/websocket/binary.go -->
        headers := http.Header{
            "X-Agent-ID": []string{"test-agent-" + uuid.New().String()},
        }
        
        conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers) <!-- Source: pkg/models/websocket/binary.go -->
        require.NoError(t, err)
        defer conn.Close()
        
        // Register agent
        registration := map[string]interface{}{
            "type": "agent.register",
            "payload": map[string]interface{}{
                "capabilities": []string{"code_analysis", "embedding"},
                "model": "test-model",
                "maxConcurrent": 5,
            },
        }
        
        err = conn.WriteJSON(registration)
        require.NoError(t, err)
        
        // Read acknowledgment
        var response map[string]interface{}
        err = conn.ReadJSON(&response)
        require.NoError(t, err)
        assert.Equal(t, "agent.registered", response["type"])
    })
}
```

## E2E Testing

### Frontend Testing (Not Currently Implemented)

**Note**: Developer Mesh is currently a backend-only platform with no frontend. The examples below show potential testing approaches if a frontend were added.

#### Cypress Example (Theoretical)

```javascript
// cypress/integration/user_journey.spec.js
// THEORETICAL: No frontend exists in current implementation
describe('User Journey: Create and Search Context', () => {
  before(() => {
    cy.task('db:seed'); // Seed test data
  });

  it('should create a new context and search for it', () => {
    // Login
    cy.visit('/login');
    cy.get('[data-cy=email]').type('test@example.com');
    cy.get('[data-cy=password]').type('password123');
    cy.get('[data-cy=submit]').click();
    
    // Create context
    cy.get('[data-cy=create-context]').click();
    cy.get('[data-cy=context-name]').type('E2E Test Context');
    cy.get('[data-cy=context-content]').type('This is an E2E test');
    cy.get('[data-cy=save-context]').click();
    
    // Verify creation
    cy.get('[data-cy=success-message]').should('contain', 'Context created');
    
    // Search for context
    cy.get('[data-cy=search-input]').type('E2E Test');
    cy.get('[data-cy=search-button]').click();
    
    // Verify search results
    cy.get('[data-cy=search-results]').should('contain', 'E2E Test Context');
  });
});
```

#### Playwright Example (Theoretical)

```typescript
// tests/e2e/api.spec.ts
// THEORETICAL: While API testing is possible, Playwright is not currently used
import { test, expect } from '@playwright/test';

test.describe('API E2E Tests', () => {
  let apiContext;
  let authToken;

  test.beforeAll(async ({ playwright }) => {
    apiContext = await playwright.request.newContext({
      baseURL: process.env.API_URL || 'http://localhost:8080 (MCP Server)',
    });
    
    // Authenticate
    const loginResponse = await apiContext.post('/api/v1/auth/login', {
      data: {
        email: 'test@example.com',
        password: 'password123'
      }
    });
    
    const { token } = await loginResponse.json();
    authToken = token;
  });

  test('Full context lifecycle', async () => {
    // Create context
    const createResponse = await apiContext.post('/api/v1/contexts', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        name: 'Playwright Test',
        content: 'E2E test content'
      }
    });
    
    expect(createResponse.ok()).toBeTruthy();
    const context = await createResponse.json();
    
    // Update context
    const updateResponse = await apiContext.put(`/api/v1/contexts/${context.id}`, {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        content: 'Updated content'
      }
    });
    
    expect(updateResponse.ok()).toBeTruthy();
    
    // Search for context
    const searchResponse = await apiContext.post('/api/v1/contexts/search', {
      headers: {
        'Authorization': `Bearer ${authToken}`
      },
      data: {
        query: 'Playwright'
      }
    });
    
    const results = await searchResponse.json();
    expect(results).toHaveLength(1);
    expect(results[0].id).toBe(context.id);
    
    // Delete context
    const deleteResponse = await apiContext.delete(`/api/v1/contexts/${context.id}`, {
      headers: {
        'Authorization': `Bearer ${authToken}`
      }
    });
    
    expect(deleteResponse.ok()).toBeTruthy();
  });
});
```

### Redis/ElastiCache Integration Tests

```go
// tests/integration/redis_test.go
// +build integration

func TestRedisIntegration(t *testing.T) {
    // For local development: SSH tunnel to ElastiCache
    // For CI: May use local Redis container
    redisAddr := os.Getenv("REDIS_ADDR")
    if redisAddr == "" {
        redisAddr = "localhost:6379 (Redis)" // Default for CI
    }
    
    // Connect to Redis via tunnel
    rdb := redis.NewClient(&redis.Options{
        Addr:     redisAddr,
        Password: "", // ElastiCache auth token if configured
        DB:       0,
    })
    defer rdb.Close()
    
    ctx := context.Background()
    
    // Test connection
    pong, err := rdb.Ping(ctx).Result()
    require.NoError(t, err)
    assert.Equal(t, "PONG", pong)
    
    // Test cache operations
    key := fmt.Sprintf("test:integration:%d", time.Now().Unix())
    value := map[string]interface{}{
        "test": true,
        "timestamp": time.Now().Unix(),
    }
    
    // Set with expiration
    data, err := json.Marshal(value)
    require.NoError(t, err)
    
    err = rdb.Set(ctx, key, data, 5*time.Minute).Err()
    require.NoError(t, err)
    
    // Get value
    result, err := rdb.Get(ctx, key).Bytes()
    require.NoError(t, err)
    assert.Equal(t, data, result)
    
    // Test distributed locking
    lockKey := "test:lock:" + uuid.New().String()
    lock := redislock.New(rdb)
    
    // Obtain lock
    locker, err := lock.Obtain(ctx, lockKey, 10*time.Second, nil)
    require.NoError(t, err)
    defer locker.Release(ctx)
    
    // Verify lock is held
    _, err = lock.Obtain(ctx, lockKey, 1*time.Second, nil)
    assert.Error(t, err) // Should fail as lock is held
    
    // Cleanup
    rdb.Del(ctx, key)
}
```

## Load Testing

### K6 Load Tests with AWS

```javascript
// tests/load/spike-test.js
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate } from 'k6/metrics';

const errorRate = new Rate('errors');

export let options = {
  stages: [
    { duration: '2m', target: 100 }, // Ramp up
    { duration: '5m', target: 100 }, // Stay at 100
    { duration: '2m', target: 1000 }, // Spike to 1000
    { duration: '5m', target: 1000 }, // Stay at 1000
    { duration: '2m', target: 100 }, // Scale down
    { duration: '5m', target: 0 }, // Ramp down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500', 'p(99)<1000'],
    errors: ['rate<0.1'], // Error rate under 10%
  },
};

const API_BASE = __ENV.API_URL || 'http://localhost:8080 (MCP Server)';
const API_KEY = __ENV.API_KEY;

// Cost tracking for load tests
let totalCost = 0;
const EMBEDDING_COST_PER_1K_TOKENS = 0.0001; // Titan pricing

export default function () {
  // Create context with embedding
  const content = 'Load test context content for embedding generation and vector search validation';
  let createRes = http.post(
    `${API_BASE}/api/v1/contexts`,
    JSON.stringify({
      name: `Load Test ${__VU}-${__ITER}`,
      content: content,
      metadata: {
        loadTest: true,
        virtualUser: __VU,
        iteration: __ITER
      }
    }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': API_KEY,
      },
    }
  );
  
  check(createRes, {
    'create status is 201': (r) => r.status === 201,
    'create time < 500ms': (r) => r.timings.duration < 500,
    'has context ID': (r) => JSON.parse(r.body).id !== undefined,
  }) || errorRate.add(1);
  
  // Track embedding cost
  if (createRes.status === 201) {
    const tokens = Math.ceil(content.length / 4); // Rough token estimate
    totalCost += (tokens / 1000) * EMBEDDING_COST_PER_1K_TOKENS;
  }
  
  sleep(1);
  
  // Search contexts
  let searchRes = http.post(
    `${API_BASE}/api/v1/contexts/search`,
    JSON.stringify({ query: 'Load Test' }),
    {
      headers: {
        'Content-Type': 'application/json',
        'X-API-Key': API_KEY,
      },
    }
  );
  
  check(searchRes, {
    'search status is 200': (r) => r.status === 200,
    'search time < 1000ms': (r) => r.timings.duration < 1000,
  }) || errorRate.add(1);
  
  sleep(Math.random() * 3);
}
```

### Alternative Load Testing Tools (Not Implemented)

#### Gatling Example (Theoretical)

```scala
// tests/load/BasicSimulation.scala
// THEORETICAL: Gatling is not currently used in the project
import io.gatling.core.Predef._
import io.gatling.http.Predef._
import scala.concurrent.duration._

class BasicSimulation extends Simulation {
  val httpProtocol = http
    .baseUrl("http://localhost:8080 (MCP Server)")
    .acceptHeader("application/json")
    .header("X-API-Key", System.getenv("API_KEY"))

  val createContext = exec(http("Create Context")
    .post("/api/v1/contexts")
    .body(StringBody("""{"name":"Gatling Test","content":"Load test"}"""))
    .check(status.is(201))
    .check(jsonPath("$.id").saveAs("contextId")))

  val getContext = exec(http("Get Context")
    .get("/api/v1/contexts/${contextId}")
    .check(status.is(200)))

  val userJourney = scenario("User Journey")
    .exec(createContext)
    .pause(1)
    .exec(getContext)

  setUp(
    userJourney.inject(
      rampUsersPerSec(1).to(100).during(5.minutes),
      constantUsersPerSec(100).during(10.minutes),
      rampUsersPerSec(100).to(0).during(5.minutes)
    )
  ).protocols(httpProtocol)
   .assertions(
     global.responseTime.max.lt(2000),
     global.successfulRequests.percent.gt(95)
   )
}
```

## Chaos Engineering (Not Implemented)

**Note**: Chaos engineering tools are not currently implemented in Developer Mesh, which runs on a single EC2 instance with Docker Compose.

### Litmus Chaos Example (Theoretical)

```yaml
# chaos/network-delay.yaml
# THEORETICAL: Kubernetes-based chaos testing not applicable to current Docker Compose deployment
apiVersion: litmuschaos.io/v1alpha1
kind: ChaosEngine
metadata:
  name: network-chaos
spec:
  appinfo:
    appns: mcp-prod
    applabel: app=mcp-server
  chaosServiceAccount: litmus-admin
  experiments:
    - name: pod-network-latency
      spec:
        components:
          env:
            - name: NETWORK_INTERFACE
              value: eth0
            - name: NETWORK_LATENCY
              value: "2000" # 2 second delay
            - name: TOTAL_CHAOS_DURATION
              value: "600" # 10 minutes
            - name: PODS_AFFECTED_PERC
              value: "50" # Affect 50% of pods
```

### Chaos Toolkit Example (Theoretical)

```json
// THEORETICAL: Not implemented in current single-instance deployment
{
  "title": "Verify system handles database failure",
  "description": "Kill database and ensure graceful degradation",
  "steady-state-hypothesis": {
    "title": "System is healthy",
    "probes": [
      {
        "type": "probe",
        "name": "api-health-check",
        "provider": {
          "type": "http",
          "url": "http://localhost:8080 (MCP Server)/health",
          "timeout": 5
        }
      }
    ]
  },
  "method": [
    {
      "type": "action",
      "name": "kill-database",
      "provider": {
        "type": "process",
        "path": "kubectl",
        "arguments": ["delete", "pod", "-l", "app=postgres", "-n", "mcp-prod"]
      }
    }
  ],
  "rollbacks": [
    {
      "type": "action",
      "name": "restart-database",
      "provider": {
        "type": "process",
        "path": "kubectl",
        "arguments": ["rollout", "restart", "deployment/postgres", "-n", "mcp-prod"]
      }
    }
  ]
}
```

## Contract Testing (Not Implemented)

**Note**: Contract testing with Pact is not currently implemented but could be valuable for API versioning.

### Pact Consumer Example (Theoretical)

```javascript
// tests/contract/consumer.pact.test.js
const { Pact } = require('@pact-foundation/pact');
const { getContext } = require('../src/api-client');

describe('API Client Pact Tests', () => {
  const provider = new Pact({
    consumer: 'Frontend',
    provider: 'MCP-API',
    port: 8989,
  });

  beforeAll(() => provider.setup());
  afterAll(() => provider.finalize());
  afterEach(() => provider.verify());

  describe('get context', () => {
    test('should return a context', async () => {
      const expectedContext = {
        id: '123',
        name: 'Test Context',
        content: 'Test content',
      };

      await provider.addInteraction({
        state: 'a context with ID 123 exists',
        uponReceiving: 'a request to get context',
        withRequest: {
          method: 'GET',
          path: '/api/v1/contexts/123',
          headers: {
            'Authorization': 'Bearer token',
          },
        },
        willRespondWith: {
          status: 200,
          headers: {
            'Content-Type': 'application/json',
          },
          body: expectedContext,
        },
      });

      const context = await getContext('123', 'token');
      expect(context).toEqual(expectedContext);
    });
  });
});
```

### Pact Provider Verification

```go
// tests/contract/provider_test.go
func TestPactProvider(t *testing.T) {
    pact := &dsl.Pact{
        Provider: "MCP-API",
        LogLevel: "INFO",
    }

    // Start test server
    server := httptest.NewServer(api.NewServer(testConfig).Handler())
    defer server.Close()

    _, err := pact.VerifyProvider(t, types.VerifyRequest{
        ProviderBaseURL:    server.URL,
        PactURLs:           []string{"./pacts/frontend-mcp-api.json"},
        StateHandlers: types.StateHandlers{
            "a context with ID 123 exists": func() error {
                return seedContext("123", "Test Context", "Test content")
            },
        },
    })

    assert.NoError(t, err)
}
```

## Test Data Management

### Test Fixtures

```go
// tests/fixtures/contexts.go
package fixtures

import (
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

func ValidContext() *models.Context {
    return &models.Context{
        Name:     "Test Context",
        Content:  "Test content",
        TenantID: "test-tenant",
        Metadata: map[string]interface{}{
            "source": "test",
        },
    }
}

func ContextWithLongContent() *models.Context {
    ctx := ValidContext()
    ctx.Content = strings.Repeat("a", 10000)
    return ctx
}
```

### Test Database Seeding

```go
// tests/helpers/seed.go
func SeedTestData(db *sql.DB) error {
    // Use transactions for isolation
    tx, err := db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // Seed users
    _, err = tx.Exec(`
        INSERT INTO users (id, email, name, tenant_id)
        VALUES 
            ('user-1', 'test1@example.com', 'Test User 1', 'tenant-1'),
            ('user-2', 'test2@example.com', 'Test User 2', 'tenant-2')
    `)
    if err != nil {
        return err
    }

    // Seed contexts
    _, err = tx.Exec(`
        INSERT INTO contexts (id, name, content, tenant_id, created_by)
        VALUES 
            ('ctx-1', 'Test Context 1', 'Content 1', 'tenant-1', 'user-1'),
            ('ctx-2', 'Test Context 2', 'Content 2', 'tenant-1', 'user-1')
    `)
    if err != nil {
        return err
    }

    return tx.Commit()
}
```

## Test Environment Setup

### AWS Test Resources

```bash
# Setup test environment
export AWS_REGION=us-east-1
export TEST_S3_BUCKET=sean-mcp-dev-contexts
export TEST_SQS_QUEUE=https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
export BEDROCK_ENABLED=true
export BEDROCK_SESSION_LIMIT=0.10  # $0.10 per test session

# Start ElastiCache tunnel (REQUIRED for local development)
./scripts/aws/connect-elasticache.sh

# Verify AWS connectivity
./scripts/aws/test-aws-services.sh

# Run integration tests
export RUN_AWS_INTEGRATION_TESTS=true
make test-integration

# Note: Integration tests may use LocalStack for some AWS services
# Check docker-compose.local.yml for LocalStack configuration
```

### Local Test Database

```bash
# Start PostgreSQL with pgvector
docker run -d \
  --name postgres-test \
  -p 5432:5432 (PostgreSQL) \
  -e POSTGRES_USER=postgres \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=devops_mcp_test \
  ankane/pgvector:v0.5.1

# Run migrations
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432 (PostgreSQL)/devops_mcp_test?sslmode=disable" up
```

## CI/CD Test Pipeline

### GitHub Actions with AWS

```yaml
# .github/workflows/test.yml
name: Test Suite

on:
  pull_request:
  push:
    branches: [main]

env:
  GO_VERSION: '1.24.4'  # Current version used in CI
  AWS_REGION: us-east-1
  TEST_S3_BUCKET: sean-mcp-dev-contexts
  TEST_SQS_QUEUE: https://sqs.us-east-1.amazonaws.com/594992249511/sean-mcp-test
  BEDROCK_SESSION_LIMIT: 0.10

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Run unit tests
        run: |
          make test
          # Ensure coverage is above 85%
          go tool cover -func=coverage.out | grep total | awk '{print $3}' | sed 's/%//g' | awk '{if ($1 < 85) exit 1}'
      
      - name: Upload coverage
        uses: codecov/codecov-action@v3
        with:
          file: ./coverage.out
          fail_ci_if_error: true

  integration-tests:
    runs-on: ubuntu-latest
    services:
      postgres:
        image: ankane/pgvector:v0.5.1
        env:
          POSTGRES_USER: postgres
          POSTGRES_PASSWORD: test
          POSTGRES_DB: devops_mcp_test
        options: >-
          --health-cmd pg_isready
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 5432:5432 (PostgreSQL)
      
      redis:
        image: redis:7-alpine
        options: >-
          --health-cmd "redis-cli ping"
          --health-interval 10s
          --health-timeout 5s
          --health-retries 5
        ports:
          - 6379:6379 (Redis)
    
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: ${{ env.GO_VERSION }}
      
      - name: Configure AWS credentials
        uses: aws-actions/configure-aws-credentials@v2
        with:
          aws-access-key-id: ${{ secrets.AWS_ACCESS_KEY_ID }}
          aws-secret-access-key: ${{ secrets.AWS_SECRET_ACCESS_KEY }}
          aws-region: ${{ env.AWS_REGION }}
      
      - name: Install migrate
        run: |
          curl -L https://github.com/golang-migrate/migrate/releases/download/v4.16.2/migrate.linux-amd64.tar.gz | tar xvz
          sudo mv migrate /usr/local/bin/
      
      - name: Run migrations
        run: |
          migrate -path migrations -database "postgresql://postgres:test@localhost:5432 (PostgreSQL)/devops_mcp_test?sslmode=disable" up
      
      - name: Run integration tests
        env:
          RUN_AWS_INTEGRATION_TESTS: true
          TEST_DATABASE_URL: postgres://postgres:test@localhost:5432 (PostgreSQL)/devops_mcp_test?sslmode=disable
          REDIS_ADDR: localhost:6379 (Redis)
        run: |
          # Test AWS connectivity first
          ./scripts/aws/test-aws-services.sh || echo "AWS tests will be limited"
          
          # Run integration tests
          make test-integration

  e2e-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Start services
        run: docker-compose up -d
      
      - name: Wait for services
        run: |
          timeout 300 bash -c 'until curl -f http://localhost:8080 (MCP Server)/health; do sleep 5; done'
      
      - name: Run E2E tests
        run: |
          npm install
          npm run test:e2e
      
      - name: Upload artifacts
        if: failure()
        uses: actions/upload-artifact@v3
        with:
          name: e2e-artifacts
          path: |
            cypress/screenshots
            cypress/videos

  load-tests:
    runs-on: ubuntu-latest
    if: github.event_name == 'push' && github.ref == 'refs/heads/main'
    steps:
      - uses: actions/checkout@v3
      
      - name: Run load tests
        run: |
          docker run --rm \
            -v $PWD:/scripts \
            loadimpact/k6 run /scripts/tests/load/spike-test.js
```

## Testing AWS Services

### Bedrock Testing Strategies

```go
// pkg/embedding/bedrock_test.go
func TestBedrockEmbedding(t *testing.T) {
    // Use test mode for unit tests
    if os.Getenv("BEDROCK_TEST_MODE") == "mock" {
        // Return consistent mock embeddings
        mockProvider := &MockEmbeddingProvider{
            EmbeddingFunc: func(ctx context.Context, text string) ([]float32, error) {
                // Return deterministic embedding based on text hash
                hash := sha256.Sum256([]byte(text))
                embedding := make([]float32, 1536)
                for i := 0; i < len(embedding); i++ {
                    embedding[i] = float32(hash[i%32]) / 255.0
                }
                return embedding, nil
            },
        }
        // Test with mock
    }
    
    // For integration tests, use real Bedrock with limits
    if os.Getenv("RUN_AWS_INTEGRATION_TESTS") == "true" {
        provider := bedrock.NewProvider(bedrock.Config{
            Model: "amazon.titan-embed-text-v1",
            SessionLimit: 0.01, // $0.01 limit for tests
        })
        
        ctx := context.Background()
        embedding, err := provider.GenerateEmbedding(ctx, "Test text")
        
        // Handle rate limiting gracefully
        if err != nil && bedrock.IsRateLimitError(err) {
            t.Skip("Bedrock rate limited")
        }
        
        require.NoError(t, err)
        assert.Len(t, embedding, 1536)
    }
}
```

### SQS Testing Patterns

```go
// Test with real SQS queue
func TestSQSWorker(t *testing.T) {
    if os.Getenv("TEST_SQS_QUEUE") == "" {
        t.Skip("SQS queue not configured")
    }
    
    // Use test queue with proper cleanup
    // Note: May use LocalStack SQS in local development
    queueURL := os.Getenv("TEST_SQS_QUEUE")
    if os.Getenv("AWS_ENDPOINT_URL") != "" {
        // Using LocalStack or other local AWS mock
        queueURL = strings.Replace(queueURL, "amazonaws.com", "localhost:4566", 1)
    }
    
    worker := NewWorker(WorkerConfig{
        QueueURL: queueURL,
        VisibilityTimeout: 30,
        MaxMessages: 1,
    })
    
    // Send test message
    testMsg := &TaskMessage{
        ID:   "test-" + uuid.New().String(),
        Type: "test",
        Payload: json.RawMessage(`{"test": true}`),
    }
    
    err := worker.SendMessage(context.Background(), testMsg)
    require.NoError(t, err)
    
    // Process with timeout
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    processed := false
    worker.ProcessMessages(ctx, func(msg *TaskMessage) error {
        if msg.ID == testMsg.ID {
            processed = true
            return nil
        }
        return fmt.Errorf("unexpected message")
    })
    
    assert.True(t, processed, "Test message should be processed")
}
```

## Test Best Practices

### 1. Test Naming
```go
// Good: Descriptive test names following pattern
func TestCreateContext_WithValidData_ReturnsCreatedContext(t *testing.T) {}
func TestCreateContext_WithMissingName_ReturnsValidationError(t *testing.T) {}
func TestBedrockProvider_GenerateEmbedding_HandlesRateLimit(t *testing.T) {}

// Bad: Unclear names
func TestCreate(t *testing.T) {}
func TestError(t *testing.T) {}
```

### 2. Test Isolation
```go
// Each test should be independent
func TestExample(t *testing.T) {
    // Setup
    db := setupTestDB(t)
    defer cleanupDB(db)
    
    // Test logic
    
    // No dependency on other tests
}
```

### 3. Use Test Helpers
```go
func assertContextEqual(t *testing.T, expected, actual *Context) {
    t.Helper() // Mark as test helper
    
    assert.Equal(t, expected.Name, actual.Name)
    assert.Equal(t, expected.Content, actual.Content)
    assert.Equal(t, expected.TenantID, actual.TenantID)
}
```

### 4. Parallel Testing
```go
func TestParallel(t *testing.T) {
    t.Parallel() // Run tests in parallel
    
    // Test must be thread-safe
}
```

### 5. AWS Resource Cleanup
```go
func TestWithAWSCleanup(t *testing.T) {
    // Track resources for cleanup
    var s3Keys []string
    var sqsMessages []string
    
    t.Cleanup(func() {
        // Clean S3 objects
        for _, key := range s3Keys {
            s3Client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
                Bucket: aws.String(testBucket),
                Key:    aws.String(key),
            })
        }
        
        // Purge test queue messages
        if len(sqsMessages) > 0 {
            sqsClient.PurgeQueue(context.Background(), &sqs.PurgeQueueInput{
                QueueUrl: aws.String(testQueue),
            })
        }
    })
    
    // Test logic that creates resources
    key := uploadTestFile()
    s3Keys = append(s3Keys, key)
}
```

### 6. Cost-Aware Testing
```go
func TestWithCostTracking(t *testing.T) {
    startCost := getCurrentAWSCost()
    
    t.Cleanup(func() {
        endCost := getCurrentAWSCost()
        testCost := endCost - startCost
        
        // Log test cost
        t.Logf("Test cost: $%.4f", testCost)
        
        // Fail if test exceeds budget
        if testCost > 0.10 {
            t.Errorf("Test exceeded cost budget: $%.4f > $0.10", testCost)
        }
    })
    
    // Run test
}
```

## Testing Checklist

### Pre-Test Setup
- [ ] AWS credentials configured
- [ ] ElastiCache SSH tunnel running
- [ ] Test database with pgvector installed
- [ ] Environment variables set
- [ ] Cost limits configured

### Test Coverage
- [ ] Unit tests cover >85% of business logic
- [ ] Integration tests verify AWS service interactions
- [ ] Database tests include pgvector operations
- [ ] WebSocket tests cover binary protocol <!-- Source: pkg/models/websocket/binary.go -->
- [ ] E2E tests validate critical paths
- [ ] Load tests respect cost limits

### AWS Service Testing
- [ ] S3 operations tested with IP-restricted bucket
- [ ] SQS message flow tested with real queue
- [ ] Bedrock embeddings tested with rate limit handling
- [ ] ElastiCache operations tested via SSH tunnel
- [ ] Cost tracking implemented for all AWS operations

### CI/CD Pipeline
- [ ] All tests run automatically on PR
- [ ] Coverage reports generated and tracked
- [ ] Integration tests use real AWS services
- [ ] Test costs monitored and limited
- [ ] Flaky tests identified and fixed
- [ ] Test artifacts preserved for debugging

### Best Practices
- [ ] Tests are deterministic and repeatable
- [ ] AWS resources cleaned up after tests
- [ ] Sensitive data not logged in tests
- [ ] Rate limits handled gracefully
- [ ] Parallel tests don't conflict
- [ ] Test documentation current

## Quick Commands

```bash
# Run all tests with coverage
make test-coverage

# Run only unit tests (fast)
go test ./pkg/... -short

# Run integration tests with AWS
RUN_AWS_INTEGRATION_TESTS=true make test-integration

# Run specific package tests
go test -v ./pkg/embedding/... -run TestBedrock

# Run tests with race detection
go test -race ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Run benchmarks
go test -bench=. -benchmem ./pkg/...
```

Last Updated: 2024-01-10
