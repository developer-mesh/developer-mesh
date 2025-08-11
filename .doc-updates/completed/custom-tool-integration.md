<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:33:14
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Custom Tool Integration Guide

This guide demonstrates how to extend Developer Mesh with custom tool integrations using our production-ready adapter architecture.

## Overview

Developer Mesh's extensible architecture enables seamless integration of any DevOps tool through:

- **🔌 Adapter Pattern**: Clean separation between tool logic and API layer
- **🏗️ Go Workspace Structure**: Shared packages across multiple services
- **🔄 Event-Driven Processing**: Asynchronous operations via event bus
- **💪 Production Features**: Retries, circuit breakers, observability
- **🧪 Comprehensive Testing**: Unit, integration, and E2E test support

## Architecture Components

```mermaid
graph TB
    A[REST API] --> B[Adapter Layer]
    B --> C[Tool Client]
    C --> D[External Tool API]
    B --> E[Event Bus]
    E --> F[Worker Service]
    B --> G[Context Store]
    
    style A fill:#0366d6,color:#fff
    style B fill:#28a745,color:#fff
    style D fill:#6f42c1,color:#fff
```

When implementing a new tool, you'll create:

1. **Tool Adapter**: Implements the adapter interface
2. **API Client**: Handles external API communication
3. **Event Handlers**: Process tool-specific events
4. **MCP Tools**: Expose operations to AI agents
5. **Webhook Handlers**: React to tool webhooks

## Tool-Specific Authentication

### Credential Passthrough for Tools
```go
// Example: Tool with credential passthrough
type SecureToolExecutor struct {
    authManager *auth.Manager
}

func (s *SecureToolExecutor) ExecuteTool(ctx context.Context, toolName string, params map[string]interface{}) (interface{}, error) {
    // Extract user credentials from context
    creds, ok := auth.CredentialsFromContext(ctx)
    if !ok {
        return nil, errors.New("no credentials in context")
    }
    
    // Get tool-specific credentials
    toolCreds, err := s.authManager.GetToolCredentials(ctx, creds.TenantID, toolName)
    if err != nil {
        return nil, fmt.Errorf("failed to get tool credentials: %w", err)
    }
    
    // Execute tool with credentials
    switch toolName {
    case "github":
        return executeGitHubTool(params, toolCreds.Token)
    case "aws":
        return executeAWSTool(params, toolCreds.AccessKey, toolCreds.SecretKey)
    default:
        return nil, fmt.Errorf("unknown tool: %s", toolName)
    }
}

// Store tool credentials securely
curl -X POST http://localhost:8081/api/v1/auth/tools/credentials \
  -H "Authorization: Bearer $JWT_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "tool_name": "github",
    "credentials": {
      "token": "ghp_...",
      "type": "personal_access_token"
    },
    "tenant_id": "my-tenant"
  }'
```

## Implementation Guide

### 1. Define the Adapter Interface

Create your adapter interface following the established pattern:

```go
// pkg/adapters/interfaces/ci.go
package interfaces

import (
    "context"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

// CIAdapter defines the interface for CI/CD tool integrations
type CIAdapter interface {
    Adapter // Embed base adapter interface
    
    // Build operations
    TriggerBuild(ctx context.Context, req *models.BuildRequest) (*models.Build, error)
    GetBuild(ctx context.Context, buildID string) (*models.Build, error)
    ListBuilds(ctx context.Context, projectID string, opts *models.ListOptions) ([]*models.Build, error)
    CancelBuild(ctx context.Context, buildID string) error
    
    // Pipeline operations
    GetPipeline(ctx context.Context, pipelineID string) (*models.Pipeline, error)
    TriggerPipeline(ctx context.Context, req *models.PipelineRequest) (*models.PipelineRun, error)
    
    // Artifact operations
    GetArtifacts(ctx context.Context, buildID string) ([]*models.Artifact, error)
    DownloadArtifact(ctx context.Context, artifactID string) ([]byte, error)
}

// Models
type Build struct {
    ID          string                 `json:"id"`
    ProjectID   string                 `json:"project_id"`
    PipelineID  string                 `json:"pipeline_id"`
    Branch      string                 `json:"branch"`
    Commit      string                 `json:"commit"`
    Status      BuildStatus           `json:"status"`
    StartedAt   time.Time             `json:"started_at"`
    FinishedAt  *time.Time            `json:"finished_at,omitempty"`
    Duration    *time.Duration        `json:"duration,omitempty"`
    URL         string                `json:"url"`
    Logs        string                `json:"logs,omitempty"`
    Metadata    map[string]interface{} `json:"metadata"`
}

type BuildStatus string

const (
    BuildStatusQueued    BuildStatus = "queued"
    BuildStatusRunning   BuildStatus = "running"
    BuildStatusSucceeded BuildStatus = "succeeded"
    BuildStatusFailed    BuildStatus = "failed"
    BuildStatusCancelled BuildStatus = "cancelled"
)
```

### 2. Implement the Adapter

Create your tool adapter with production features:

```go
// pkg/adapters/gitlab/adapter.go
package gitlab

import (
    "context"
    "fmt"
    "time"
    
    "github.com/developer-mesh/developer-mesh/pkg/adapters/interfaces"
    "github.com/developer-mesh/developer-mesh/pkg/adapters/resilience"
    "github.com/developer-mesh/developer-mesh/pkg/models"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Config holds GitLab adapter configuration
type Config struct {
    URL      string `yaml:"url" validate:"required,url"`
    Username string `yaml:"username" validate:"required"`
    Token    string `yaml:"token" validate:"required"`
    
    // Resilience settings
    Timeout         time.Duration `yaml:"timeout" default:"30s"`
    MaxRetries      int          `yaml:"max_retries" default:"3"`
    CircuitBreaker  resilience.CircuitBreakerConfig `yaml:"circuit_breaker"`
    RateLimit       resilience.RateLimitConfig      `yaml:"rate_limit"`
}

// GitLabAdapter implements the CIAdapter interface
type GitLabAdapter struct {
    client  *gitlabClient
    config  *Config
    logger  observability.Logger
    metrics observability.Metrics
    
    // Resilience components
    circuitBreaker *resilience.CircuitBreaker
    rateLimiter    *resilience.RateLimiter
    retrier        *resilience.Retrier
}

// NewGitLabAdapter creates a new GitLab adapter
func NewGitLabAdapter(config *Config, logger observability.Logger, metrics observability.Metrics) (*GitLabAdapter, error) {
    // Validate configuration
    if err := config.Validate(); err != nil {
        return nil, fmt.Errorf("invalid config: %w", err)
    }
    
    // Create HTTP client with observability
    httpClient := &http.Client{
        Timeout: config.Timeout,
        Transport: observability.NewHTTPTransport(
            http.DefaultTransport,
            logger,
            metrics,
        ),
    }
    
    // Initialize GitLab client
    client := newGitLabClient(config.URL, config.Username, config.Token, httpClient)
    
    // Setup resilience components
    cb := resilience.NewCircuitBreaker(config.CircuitBreaker)
    rl := resilience.NewRateLimiter(config.RateLimit)
    rt := resilience.NewRetrier(resilience.RetrierConfig{
        MaxAttempts: config.MaxRetries,
        InitialDelay: 1 * time.Second,
        MaxDelay: 30 * time.Second,
        Multiplier: 2.0,
    })
    
    return &GitLabAdapter{
        client:         client,
        config:         config,
        logger:         logger,
        metrics:        metrics,
        circuitBreaker: cb,
        rateLimiter:    rl,
        retrier:        rt,
    }, nil
}

// TriggerBuild implements CIAdapter.TriggerBuild with resilience
func (a *GitLabAdapter) TriggerBuild(ctx context.Context, req *models.BuildRequest) (*models.Build, error) {
    // Add tracing
    ctx, span := a.startSpan(ctx, "TriggerBuild")
    defer span.End()
    
    // Check rate limit
    if err := a.rateLimiter.Wait(ctx); err != nil {
        return nil, fmt.Errorf("rate limit exceeded: %w", err)
    }
    
    // Execute with circuit breaker and retry
    var build *models.Build
    err := a.circuitBreaker.Execute(func() error {
        return a.retrier.Do(ctx, func() error {
            // Call GitLab API
            gitlabJob, err := a.client.TriggerJob(ctx, req.ProjectID, map[string]string{
                "BRANCH": req.Branch,
                "COMMIT": req.Commit,
            })
            if err != nil {
                return err
            }
            
            // Convert to standard model
            build = a.gitlabJobToBuild(gitlabJob)
            return nil
        })
    })
    
    if err != nil {
        a.recordError("trigger_build_failed", err)
        return nil, err
    }
    
    // Store in context for future reference
    if err := a.storeContext(ctx, "build", build); err != nil {
        a.logger.Warn("failed to store build context", "error", err)
    }
    
    // Emit event
    a.emitEvent("build.triggered", build)
    
    // Record metrics
    a.metrics.Counter("gitlab_builds_triggered", 1, map[string]string{
        "project": req.ProjectID,
        "branch": req.Branch,
    })
    
    return build, nil
}

// Helper methods
func (a *GitLabAdapter) gitlabJobToBuild(job *gitlabJob) *models.Build {
    return &models.Build{
        ID:         fmt.Sprintf("%s-%d", job.Name, job.Number),
        ProjectID:  job.Name,
        Branch:     job.Parameters["BRANCH"],
        Commit:     job.Parameters["COMMIT"],
        Status:     a.mapBuildStatus(job.Result),
        StartedAt:  time.Unix(job.Timestamp/1000, 0),
        URL:        job.URL,
        Metadata: map[string]interface{}{
            "gitlab_number": job.Number,
            "node": job.BuiltOn,
        },
    }
}
```

### 3. Create the API Client

Implement a robust API client with proper error handling:

```go
// pkg/adapters/gitlab/client.go
package gitlab

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "net/url"
    "time"
)

type gitlabClient struct {
    baseURL    string
    username   string
    token      string
    httpClient *http.Client
}

type gitlabJob struct {
    Name       string            `json:"name"`
    Number     int64             `json:"number"`
    Result     string            `json:"result"`
    Timestamp  int64             `json:"timestamp"`
    Duration   int64             `json:"duration"`
    URL        string            `json:"url"`
    BuiltOn    string            `json:"builtOn"`
    Parameters map[string]string `json:"parameters"`
}

func newGitLabClient(baseURL, username, token string, httpClient *http.Client) *gitlabClient {
    return &gitlabClient{
        baseURL:    baseURL,
        username:   username,
        token:      token,
        httpClient: httpClient,
    }
}

func (c *gitlabClient) TriggerJob(ctx context.Context, jobName string, params map[string]string) (*gitlabJob, error) {
    // Build URL with parameters
    u, err := url.Parse(fmt.Sprintf("%s/job/%s/buildWithParameters", c.baseURL, jobName))
    if err != nil {
        return nil, fmt.Errorf("invalid URL: %w", err)
    }
    
    // Add parameters
    q := u.Query()
    for k, v := range params {
        q.Set(k, v)
    }
    u.RawQuery = q.Encode()
    
    // Create request
    req, err := http.NewRequestWithContext(ctx, "POST", u.String(), nil)
    if err != nil {
        return nil, fmt.Errorf("create request: %w", err)
    }
    
    // Add authentication
    req.SetBasicAuth(c.username, c.token)
    req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    
    // Execute request
    resp, err := c.httpClient.Do(req)
    if err != nil {
        return nil, fmt.Errorf("execute request: %w", err)
    }
    defer resp.Body.Close()
    
    // Check response
    if resp.StatusCode != http.StatusCreated {
        return nil, c.parseError(resp)
    }
    
    // Get queue location from header
    location := resp.Header.Get("Location")
    if location == "" {
        return nil, fmt.Errorf("no queue location in response")
    }
    
    // Poll for build to start
    return c.waitForBuildStart(ctx, location)
}

func (c *gitlabClient) waitForBuildStart(ctx context.Context, queueURL string) (*gitlabJob, error) {
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    timeout := time.After(5 * time.Minute)
    
    for {
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        case <-timeout:
            return nil, fmt.Errorf("timeout waiting for build to start")
        case <-ticker.C:
            // Check queue item
            req, err := http.NewRequestWithContext(ctx, "GET", queueURL+"/api/json", nil)
            if err != nil {
                return nil, err
            }
            req.SetBasicAuth(c.username, c.token)
            
            resp, err := c.httpClient.Do(req)
            if err != nil {
                return nil, err
            }
            defer resp.Body.Close()
            
            var queueItem struct {
                Executable *struct {
                    URL string `json:"url"`
                } `json:"executable"`
            }
            
            if err := json.NewDecoder(resp.Body).Decode(&queueItem); err != nil {
                return nil, err
            }
            
            // If build has started, get details
            if queueItem.Executable != nil {
                return c.getBuild(ctx, queueItem.Executable.URL)
            }
        }
    }
}

func (c *gitlabClient) parseError(resp *http.Response) error {
    var errResp struct {
        Message string `json:"message"`
        Error   string `json:"error"`
    }
    
    if err := json.NewDecoder(resp.Body).Decode(&errResp); err != nil {
        return fmt.Errorf("status %d: failed to parse error", resp.StatusCode)
    }
    
    if errResp.Message != "" {
        return fmt.Errorf("status %d: %s", resp.StatusCode, errResp.Message)
    }
    
    return fmt.Errorf("status %d: %s", resp.StatusCode, errResp.Error)
}
```

### 4. Register the Adapter

Register your adapter with the factory pattern:

```go
// pkg/adapters/factory.go
package adapters

import (
    "fmt"
    
    "github.com/developer-mesh/developer-mesh/pkg/adapters/github"
    "github.com/developer-mesh/developer-mesh/pkg/adapters/gitlab"
    "github.com/developer-mesh/developer-mesh/pkg/adapters/interfaces"
    "github.com/developer-mesh/developer-mesh/pkg/config"
    "github.com/developer-mesh/developer-mesh/pkg/observability"
)

// Factory creates adapters based on configuration
type Factory struct {
    config  *config.Config
    logger  observability.Logger
    metrics observability.Metrics
}

// CreateCIAdapter creates a CI adapter based on provider
func (f *Factory) CreateCIAdapter(provider string) (interfaces.CIAdapter, error) {
    switch provider {
    case "gitlab":
        cfg := &gitlab.Config{}
        if err := f.config.UnmarshalKey(fmt.Sprintf("adapters.ci.%s", provider), cfg); err != nil {
            return nil, fmt.Errorf("load gitlab config: %w", err)
        }
        return gitlab.NewGitLabAdapter(cfg, f.logger, f.metrics)
        
    case "github":
        cfg := &github.Config{}
        if err := f.config.UnmarshalKey(fmt.Sprintf("adapters.ci.%s", provider), cfg); err != nil {
            return nil, fmt.Errorf("load github config: %w", err)
        }
        return github.NewGitHubAdapter(cfg, f.logger, f.metrics)
        
    case "circleci":
        cfg := &circleci.Config{}
        if err := f.config.UnmarshalKey(fmt.Sprintf("adapters.ci.%s", provider), cfg); err != nil {
            return nil, fmt.Errorf("load circleci config: %w", err)
        }
        return circleci.NewCircleCIAdapter(cfg, f.logger, f.metrics)
        
    default:
        return nil, fmt.Errorf("unknown CI provider: %s", provider)
    }
}

// RegisterAdapter allows external packages to register adapters
func (f *Factory) RegisterAdapter(provider string, constructor AdapterConstructor) {
    f.constructors[provider] = constructor
}
```

### 5. Create MCP Tool Definitions

Expose your tool to AI agents via MCP:

```go
// apps/mcp-server/internal/api/tools/gitlab/tools.go
package gitlab

import (
    "context"
    "encoding/json"
    
    "github.com/developer-mesh/developer-mesh/pkg/adapters/interfaces"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

// ToolProvider implements MCP tool provider for GitLab
type ToolProvider struct {
    adapter interfaces.CIAdapter
}

// GetTools returns available GitLab tools
func (p *ToolProvider) GetTools() []models.Tool {
    return []models.Tool{
        {
            Name:        "gitlab.trigger_build",
            Description: "Trigger a GitLab build for a project",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "project": {"type": "string", "description": "GitLab job name"},
                    "branch": {"type": "string", "description": "Git branch to build"},
                    "commit": {"type": "string", "description": "Git commit SHA"},
                    "parameters": {
                        "type": "object",
                        "description": "Additional build parameters"
                    }
                },
                "required": ["project", "branch"]
            }`),
        },
        {
            Name:        "gitlab.get_build_status",
            Description: "Get the status of a GitLab build",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "build_id": {"type": "string", "description": "Build identifier"}
                },
                "required": ["build_id"]
            }`),
        },
        {
            Name:        "gitlab.get_build_logs",
            Description: "Retrieve build logs from GitLab",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "build_id": {"type": "string", "description": "Build identifier"},
                    "lines": {"type": "integer", "description": "Number of log lines to retrieve", "default": 100}
                },
                "required": ["build_id"]
            }`),
        },
        {
            Name:        "gitlab.list_failed_builds",
            Description: "List recent failed builds for investigation",
            InputSchema: json.RawMessage(`{
                "type": "object",
                "properties": {
                    "project": {"type": "string", "description": "GitLab job name"},
                    "limit": {"type": "integer", "description": "Maximum number of builds", "default": 10}
                },
                "required": ["project"]
            }`),
        },
    }
}

// ExecuteTool executes a GitLab tool
func (p *ToolProvider) ExecuteTool(ctx context.Context, tool string, input json.RawMessage) (interface{}, error) {
    switch tool {
    case "gitlab.trigger_build":
        var params struct {
            Project    string                 `json:"project"`
            Branch     string                 `json:"branch"`
            Commit     string                 `json:"commit"`
            Parameters map[string]interface{} `json:"parameters"`
        }
        if err := json.Unmarshal(input, &params); err != nil {
            return nil, err
        }
        
        build, err := p.adapter.TriggerBuild(ctx, &models.BuildRequest{
            ProjectID: params.Project,
            Branch:    params.Branch,
            Commit:    params.Commit,
            Metadata:  params.Parameters,
        })
        if err != nil {
            return nil, err
        }
        
        return map[string]interface{}{
            "build_id": build.ID,
            "status":   build.Status,
            "url":      build.URL,
            "message":  fmt.Sprintf("Build %s triggered successfully", build.ID),
        }, nil
        
    case "gitlab.get_build_status":
        var params struct {
            BuildID string `json:"build_id"`
        }
        if err := json.Unmarshal(input, &params); err != nil {
            return nil, err
        }
        
        build, err := p.adapter.GetBuild(ctx, params.BuildID)
        if err != nil {
            return nil, err
        }
        
        return map[string]interface{}{
            "build_id":    build.ID,
            "status":      build.Status,
            "started_at":  build.StartedAt,
            "finished_at": build.FinishedAt,
            "duration":    build.Duration,
            "url":         build.URL,
        }, nil
        
    default:
        return nil, fmt.Errorf("unknown tool: %s", tool)
    }
}
```

### 6. Add Event Handlers

Process tool events asynchronously:

```go
// apps/worker/internal/handlers/gitlab_handler.go
package handlers

import (
    "context"
    "encoding/json"
    
    "github.com/developer-mesh/developer-mesh/pkg/events"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

// GitLabEventHandler processes GitLab events
type GitLabEventHandler struct {
    adapter     interfaces.CIAdapter
    contextMgr  interfaces.ContextManager
    vectorStore interfaces.VectorStore
}

// HandleEvent processes GitLab events
func (h *GitLabEventHandler) HandleEvent(ctx context.Context, event *events.Event) error {
    switch event.Type {
    case "gitlab.build.completed":
        return h.handleBuildCompleted(ctx, event)
        
    case "gitlab.build.failed":
        return h.handleBuildFailed(ctx, event)
        
    case "gitlab.pipeline.finished":
        return h.handlePipelineFinished(ctx, event)
        
    default:
        return nil // Ignore unknown events
    }
}

func (h *GitLabEventHandler) handleBuildCompleted(ctx context.Context, event *events.Event) error {
    var build models.Build
    if err := json.Unmarshal(event.Payload, &build); err != nil {
        return fmt.Errorf("unmarshal build: %w", err)
    }
    
    // Store build result in context
    contextData := map[string]interface{}{
        "type":       "gitlab_build",
        "build_id":   build.ID,
        "project":    build.ProjectID,
        "status":     build.Status,
        "duration":   build.Duration,
        "timestamp":  build.FinishedAt,
    }
    
    if err := h.contextMgr.StoreContext(ctx, fmt.Sprintf("build-%s", build.ID), contextData); err != nil {
        return fmt.Errorf("store context: %w", err)
    }
    
    // Index build logs for search
    if build.Logs != "" {
        embedding := map[string]interface{}{
            "text":       build.Logs,
            "type":       "build_log",
            "build_id":   build.ID,
            "project":    build.ProjectID,
            "status":     build.Status,
        }
        
        if err := h.vectorStore.CreateEmbedding(ctx, embedding); err != nil {
            return fmt.Errorf("create embedding: %w", err)
        }
    }
    
    // Trigger notifications if needed
    if build.Status == models.BuildStatusFailed {
        h.notifyBuildFailure(ctx, &build)
    }
    
    return nil
}

func (h *GitLabEventHandler) handleBuildFailed(ctx context.Context, event *events.Event) error {
    var build models.Build
    if err := json.Unmarshal(event.Payload, &build); err != nil {
        return err
    }
    
    // Analyze failure patterns
    analysis, err := h.analyzeFailure(ctx, &build)
    if err != nil {
        return fmt.Errorf("analyze failure: %w", err)
    }
    
    // Store analysis for AI agents
    if err := h.contextMgr.StoreContext(ctx, fmt.Sprintf("failure-analysis-%s", build.ID), analysis); err != nil {
        return err
    }
    
    // Check if auto-retry is enabled
    if analysis["retriable"] == true && analysis["confidence"].(float64) > 0.8 {
        return h.triggerRetry(ctx, &build)
    }
    
    return nil
}
```

### 7. Configure and Deploy

Add configuration for your tool:

```yaml
# configs/config.yaml
adapters:
  ci:
    provider: gitlab  # or gitlab, circleci, etc.
    gitlab:
      url: ${GITLAB_URL}
      username: ${GITLAB_USERNAME}
      token: ${GITLAB_TOKEN}
      
      # Resilience settings
      timeout: 30s
      max_retries: 3
      
      circuit_breaker:
        max_failures: 5
        timeout: 60s
        
      rate_limit:
        requests_per_second: 10
        burst: 20
        
      # Webhook configuration
      webhook:
        secret: ${JENKINS_WEBHOOK_SECRET}
        allowed_events:
          - build.started
          - build.completed
          - build.failed
          - pipeline.finished

# Event processing
events:
  handlers:
    gitlab:
      enabled: true
      workers: 5
      retry_policy:
        max_attempts: 3
        backoff: exponential
```

### 8. Docker Deployment

```dockerfile
# Dockerfile.gitlab-adapter
FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -o gitlab-adapter ./cmd/adapters/gitlab

FROM alpine:latest
RUN apk --no-cache add ca-certificates

COPY --from=builder /app/gitlab-adapter /gitlab-adapter
COPY configs/config.yaml /config.yaml

EXPOSE 8080
CMD ["/gitlab-adapter", "--config", "/config.yaml"]
```

```yaml
# docker-compose.yml
services:
  gitlab-adapter:
    build:
      context: .
      dockerfile: Dockerfile.gitlab-adapter
    environment:
      GITLAB_URL: ${GITLAB_URL}
      GITLAB_USERNAME: ${GITLAB_USERNAME}
      GITLAB_TOKEN: ${GITLAB_TOKEN}
      REDIS_URL: redis://redis:6379
      DATABASE_URL: postgres://user:pass@postgres:5432/mcp
    depends_on:
      - redis
      - postgres
    ports:
      - "8083:8080"
```

## Usage Examples

### Python Client

```python
from devops_mcp import MCPClient
import asyncio

class GitLabIntegration:
    def __init__(self, mcp_client: MCPClient):
        self.mcp = mcp_client
    
    async def trigger_and_monitor_build(self, project: str, branch: str):
        """Trigger a build and monitor its progress"""
        
        # Trigger build via MCP tool
        result = await self.mcp.execute_tool("gitlab.trigger_build", {
            "project": project,
            "branch": branch,
            "parameters": {
                "CLEAN_BUILD": "true",
                "RUN_TESTS": "true"
            }
        })
        
        build_id = result["build_id"]
        print(f"Build triggered: {build_id}")
        print(f"URL: {result['url']}")
        
        # Monitor build status
        while True:
            status = await self.mcp.execute_tool("gitlab.get_build_status", {
                "build_id": build_id
            })
            
            print(f"Status: {status['status']}")
            
            if status["status"] in ["succeeded", "failed", "cancelled"]:
                break
            
            await asyncio.sleep(30)
        
        # Get logs if failed
        if status["status"] == "failed":
            logs = await self.mcp.execute_tool("gitlab.get_build_logs", {
                "build_id": build_id,
                "lines": 500
            })
            
            print("\nBuild failed. Last 500 lines of logs:")
            print(logs["content"])
            
            # Search for similar failures
            similar = await self.mcp.search_contexts({
                "query": f"gitlab build failed {logs['error_pattern']}",
                "type": "gitlab_build",
                "limit": 5
            })
            
            if similar:
                print("\nSimilar failures found:")
                for s in similar:
                    print(f"- {s['build_id']}: {s['resolution']}")
        
        return status

# Usage
async def main():
    mcp = MCPClient(base_url="http://localhost:8080/api/v1")
    gitlab = GitLabIntegration(mcp)
    
    # Trigger build for feature branch
    result = await gitlab.trigger_and_monitor_build(
        project="my-app",
        branch="feature/new-feature"
    )
    
    print(f"\nBuild completed with status: {result['status']}")
    print(f"Duration: {result['duration']}")

asyncio.run(main())
```

### AI Agent Integration

```python
class DevOpsAssistant:
    """AI assistant with GitLab integration"""
    
    async def handle_build_request(self, user_message: str):
        """Process natural language build requests"""
        
        # Parse intent
        if "deploy" in user_message.lower():
            # Extract project and environment
            project = self.extract_project(user_message)
            env = self.extract_environment(user_message)
            
            # Trigger deployment pipeline
            result = await self.mcp.execute_tool("gitlab.trigger_build", {
                "project": f"{project}-deploy",
                "branch": env,
                "parameters": {
                    "ENVIRONMENT": env,
                    "VERSION": "latest"
                }
            })
            
            return f"Deployment started for {project} to {env}. Monitor at: {result['url']}"
        
        elif "failed builds" in user_message.lower():
            # List recent failures
            failures = await self.mcp.execute_tool("gitlab.list_failed_builds", {
                "project": self.extract_project(user_message),
                "limit": 5
            })
            
            response = "Recent failed builds:\n"
            for build in failures["builds"]:
                response += f"\n- {build['id']}: {build['failure_reason']}"
                response += f"\n  Failed at: {build['failed_stage']}"
            
            return response
```

## Testing Your Integration

### Unit Tests

```go
// pkg/adapters/gitlab/adapter_test.go
package gitlab_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
    
    "github.com/developer-mesh/developer-mesh/pkg/adapters/gitlab"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

type mockGitLabClient struct {
    mock.Mock
}

func (m *mockGitLabClient) TriggerJob(ctx context.Context, name string, params map[string]string) (*gitlabJob, error) {
    args := m.Called(ctx, name, params)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*gitlabJob), args.Error(1)
}

func TestGitLabAdapter_TriggerBuild(t *testing.T) {
    // Setup
    mockClient := new(mockGitLabClient)
    adapter := &gitlab.GitLabAdapter{
        client: mockClient,
        logger: testLogger,
        metrics: testMetrics,
    }
    
    // Test data
    req := &models.BuildRequest{
        ProjectID: "my-app",
        Branch:    "main",
        Commit:    "abc123",
    }
    
    expectedJob := &gitlabJob{
        Name:      "my-app",
        Number:    42,
        Result:    "SUCCESS",
        Timestamp: time.Now().Unix() * 1000,
        URL:       "https://gitlab.example.com/job/my-app/42",
    }
    
    // Expectations
    mockClient.On("TriggerJob", mock.Anything, "my-app", map[string]string{
        "BRANCH": "main",
        "COMMIT": "abc123",
    }).Return(expectedJob, nil)
    
    // Execute
    build, err := adapter.TriggerBuild(context.Background(), req)
    
    // Assertions
    assert.NoError(t, err)
    assert.NotNil(t, build)
    assert.Equal(t, "my-app-42", build.ID)
    assert.Equal(t, models.BuildStatusSucceeded, build.Status)
    assert.Equal(t, expectedJob.URL, build.URL)
    
    mockClient.AssertExpectations(t)
}
```

### Integration Tests

```go
// tests/integration/gitlab_test.go
// +build integration

package integration_test

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/suite"
    
    "github.com/developer-mesh/developer-mesh/pkg/adapters/gitlab"
    "github.com/developer-mesh/developer-mesh/pkg/models"
)

type GitLabIntegrationSuite struct {
    suite.Suite
    adapter *gitlab.GitLabAdapter
    ctx     context.Context
}

func (s *GitLabIntegrationSuite) SetupSuite() {
    // Initialize with test GitLab instance
    config := &gitlab.Config{
        URL:      getEnvOrSkip(s.T(), "TEST_GITLAB_URL"),
        Username: getEnvOrSkip(s.T(), "TEST_GITLAB_USER"),
        Token:    getEnvOrSkip(s.T(), "TEST_GITLAB_TOKEN"),
    }
    
    adapter, err := gitlab.NewGitLabAdapter(config, testLogger, testMetrics)
    s.Require().NoError(err)
    
    s.adapter = adapter
    s.ctx = context.Background()
}

func (s *GitLabIntegrationSuite) TestFullBuildLifecycle() {
    // Trigger build
    req := &models.BuildRequest{
        ProjectID: "test-project",
        Branch:    "main",
        Commit:    "HEAD",
    }
    
    build, err := s.adapter.TriggerBuild(s.ctx, req)
    s.Require().NoError(err)
    s.Require().NotNil(build)
    
    // Wait for build to complete
    timeout := time.After(5 * time.Minute)
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-timeout:
            s.Fail("Build timed out")
        case <-ticker.C:
            status, err := s.adapter.GetBuild(s.ctx, build.ID)
            s.Require().NoError(err)
            
            if status.Status != models.BuildStatusRunning {
                s.Equal(models.BuildStatusSucceeded, status.Status)
                s.NotNil(status.FinishedAt)
                s.NotZero(status.Duration)
                return
            }
        }
    }
}

func TestGitLabIntegrationSuite(t *testing.T) {
    suite.Run(t, new(GitLabIntegrationSuite))
}
```

## Best Practices

### 1. Error Handling
```go
// Define custom errors
type GitLabError struct {
    Code    string
    Message string
    Details map[string]interface{}
}

func (e *GitLabError) Error() string {
    return fmt.Sprintf("gitlab error %s: %s", e.Code, e.Message)
}

// Use error wrapping
if err != nil {
    return nil, fmt.Errorf("trigger build: %w", &GitLabError{
        Code:    "BUILD_TRIGGER_FAILED",
        Message: "Failed to trigger GitLab build",
        Details: map[string]interface{}{
            "project": req.ProjectID,
            "error":   err.Error(),
        },
    })
}
```

### 2. Observability
```go
// Add comprehensive logging
a.logger.Info("triggering build",
    "project", req.ProjectID,
    "branch", req.Branch,
    "commit", req.Commit,
    "trace_id", ctx.Value("trace_id"),
)

// Record metrics
a.metrics.Histogram("gitlab_api_latency", latency, map[string]string{
    "operation": "trigger_build",
    "status":    "success",
})

// Add tracing
ctx, span := a.tracer.Start(ctx, "gitlab.trigger_build",
    trace.WithAttributes(
        attribute.String("project", req.ProjectID),
        attribute.String("branch", req.Branch),
    ),
)
defer span.End()
```

### 3. Security
```go
// Validate inputs
if err := validateProjectID(req.ProjectID); err != nil {
    return nil, fmt.Errorf("invalid project ID: %w", err)
}

// Sanitize parameters
params := make(map[string]string)
for k, v := range req.Parameters {
    params[sanitizeKey(k)] = sanitizeValue(v)
}

// Use secrets management
token, err := a.secretManager.GetSecret(ctx, "gitlab-token")
if err != nil {
    return nil, fmt.Errorf("get token: %w", err)
}
```

### 4. Performance
```go
// Use connection pooling
var clientPool = &sync.Pool{
    New: func() interface{} {
        return &http.Client{
            Timeout: 30 * time.Second,
            Transport: &http.Transport{
                MaxIdleConns:        100,
                MaxIdleConnsPerHost: 10,
            },
        }
    },
}

// Cache frequently accessed data
type gitlabCache struct {
    projects sync.Map
    builds   *lru.Cache
}

// Batch operations when possible
func (a *GitLabAdapter) BatchTriggerBuilds(ctx context.Context, requests []*models.BuildRequest) ([]*models.Build, error) {
    // Process in parallel with concurrency limit
    sem := make(chan struct{}, 10)
    results := make([]*models.Build, len(requests))
    
    g, ctx := errgroup.WithContext(ctx)
    for i, req := range requests {
        i, req := i, req // Capture loop variables
        
        g.Go(func() error {
            sem <- struct{}{}
            defer func() { <-sem }()
            
            build, err := a.TriggerBuild(ctx, req)
            if err != nil {
                return err
            }
            results[i] = build
            return nil
        })
    }
    
    if err := g.Wait(); err != nil {
        return nil, err
    }
    
    return results, nil
}
```

## Production Checklist

- [ ] Comprehensive unit test coverage (>80%)
- [ ] Integration tests with real tool instance
- [ ] Load testing for performance validation
- [ ] Security review (authentication, authorization, input validation)
- [ ] Observability (logging, metrics, tracing)
- [ ] Error handling and recovery
- [ ] Documentation (API reference, examples, troubleshooting)
- [ ] Configuration validation and defaults
- [ ] Graceful shutdown handling
- [ ] Health check endpoints
- [ ] Rate limiting and backpressure
- [ ] Circuit breaker for external calls
- [ ] Webhook signature verification
- [ ] Idempotency for critical operations
- [ ] Monitoring dashboards and alerts

## Next Steps

1. **Explore Examples**: Review the GitHub adapter implementation
2. **API Documentation**: Document your tool's MCP interface
3. **Share**: Contribute your adapter to the community
4. **Support**: Join our Discord for help and discussions

---

*For more examples and support, visit [github.com/developer-mesh/developer-mesh](https://github.com/developer-mesh/developer-mesh)*
