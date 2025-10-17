<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:41:25
Verification Script: update-docs-parallel.sh
Batch: ab
-->

# Agent Integration Examples

> **Purpose**: Complete working examples of AI agent integrations with the Developer Mesh platform
> **Audience**: Developers implementing AI agents with practical, runnable code examples
> **Scope**: Full implementations including GPT-4, Claude, Bedrock, and custom agents

## Table of Contents

1. [Basic Agent Examples](#basic-agent-examples)
2. [OpenAI GPT-4 Agent](#openai-gpt-4-agent)
3. [Anthropic Claude Agent](#anthropic-claude-agent)
4. [AWS Bedrock Multi-Model Agent](#aws-bedrock-multi-model-agent)
5. [Specialized Agents](#specialized-agents)
6. [Multi-Agent Collaboration](#multi-agent-collaboration)
7. [Production Examples](#production-examples)

## Basic Agent Examples

### 1. Simple Echo Agent

A minimal agent that demonstrates the basic structure.

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/gorilla/websocket" <!-- Source: pkg/models/websocket/binary.go -->
)

type EchoAgent struct {
    ID       string
    conn     *websocket.Conn <!-- Source: pkg/models/websocket/binary.go -->
    msgChan  chan Message
}

type Message struct {
    ID     string      `json:"id"`
    Type   string      `json:"type"`
    Method string      `json:"method"`
    Params interface{} `json:"params,omitempty"`
    Result interface{} `json:"result,omitempty"`
}

func main() {
    agent := &EchoAgent{
        ID:      "echo-agent-001",
        msgChan: make(chan Message, 100),
    }

    // Connect to MCP server
    serverURL := os.Getenv("MCP_SERVER_URL")
    if serverURL == "" {
        serverURL = "ws://localhost:8080/ws"
    }

    conn, _, err := websocket.DefaultDialer.Dial(serverURL, nil) <!-- Source: pkg/models/websocket/binary.go -->
    if err != nil {
        log.Fatal("Failed to connect:", err)
    }
    agent.conn = conn

    // Register agent
    if err := agent.register(); err != nil {
        log.Fatal("Failed to register:", err)
    }

    // Start message handlers
    go agent.readMessages()
    go agent.processMessages()

    // Wait for interrupt
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    <-sigChan

    log.Println("Shutting down...")
    agent.conn.Close()
}

func (a *EchoAgent) register() error {
    msg := Message{
        ID:     generateID(),
        Type:   "request",
        Method: "agent.register",
        Params: map[string]interface{}{
            "agent_id":   a.ID,
            "agent_type": "echo",
            "capabilities": []map[string]interface{}{
                {
                    "name":       "echo",
                    "confidence": 1.0,
                    "description": "Echoes any input",
                },
            },
        },
    }

    return a.conn.WriteJSON(msg)
}

func (a *EchoAgent) readMessages() {
    for {
        var msg Message
        if err := a.conn.ReadJSON(&msg); err != nil {
            log.Println("Read error:", err)
            return
        }
        a.msgChan <- msg
    }
}

func (a *EchoAgent) processMessages() {
    for msg := range a.msgChan {
        switch msg.Method {
        case "task.assigned":
            a.handleTask(msg)
        case "ping":
            a.handlePing(msg)
        }
    }
}

func (a *EchoAgent) handleTask(msg Message) {
    // Echo the task content
    params := msg.Params.(map[string]interface{})
    
    result := Message{
        ID:     generateID(),
        Type:   "request",
        Method: "task.complete",
        Params: map[string]interface{}{
            "task_id": params["task_id"],
            "agent_id": a.ID,
            "result": map[string]interface{}{
                "echo": params["content"],
                "timestamp": time.Now().Unix(),
            },
        },
    }

    a.conn.WriteJSON(result)
}

func (a *EchoAgent) handlePing(msg Message) {
    pong := Message{
        ID:     msg.ID,
        Type:   "response",
        Method: "pong",
    }
    a.conn.WriteJSON(pong)
}

func generateID() string {
    return fmt.Sprintf("%d", time.Now().UnixNano())
}
```

### 2. Health Monitoring Agent

An agent that monitors its own health and reports metrics.

```go
package main

import (
    "context"
    "runtime"
    "time"

    "github.com/shirou/gopsutil/v3/cpu"
    "github.com/shirou/gopsutil/v3/mem"
)

type HealthAgent struct {
    *BaseAgent
    metrics *AgentMetrics
}

type AgentMetrics struct {
    CPUUsage      float64
    MemoryUsage   float64
    ActiveTasks   int
    CompletedTasks int
    FailedTasks   int
    Uptime        time.Duration
    startTime     time.Time
}

func NewHealthAgent() *HealthAgent {
    return &HealthAgent{
        BaseAgent: NewBaseAgent("health-monitor-001", "monitoring"),
        metrics: &AgentMetrics{
            startTime: time.Now(),
        },
    }
}

func (h *HealthAgent) Start(ctx context.Context) error {
    // Register with capabilities
    if err := h.Register([]Capability{
        {
            Name:       "system_monitoring",
            Confidence: 0.95,
            Specialties: []string{"cpu", "memory", "performance"},
        },
        {
            Name:       "health_check",
            Confidence: 0.98,
            Specialties: []string{"availability", "latency", "errors"},
        },
    }); err != nil {
        return err
    }

    // Start health reporting
    go h.reportHealth(ctx)

    // Start metric collection
    go h.collectMetrics(ctx)

    return h.ProcessMessages(ctx)
}

func (h *HealthAgent) reportHealth(ctx context.Context) {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            h.sendHealthReport()
        case <-ctx.Done():
            return
        }
    }
}

func (h *HealthAgent) collectMetrics(ctx context.Context) {
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()

    for {
        select {
        case <-ticker.C:
            h.updateMetrics()
        case <-ctx.Done():
            return
        }
    }
}

func (h *HealthAgent) updateMetrics() {
    // CPU usage
    cpuPercent, _ := cpu.Percent(time.Second, false)
    if len(cpuPercent) > 0 {
        h.metrics.CPUUsage = cpuPercent[0]
    }

    // Memory usage
    vmStat, _ := mem.VirtualMemory()
    h.metrics.MemoryUsage = vmStat.UsedPercent

    // Uptime
    h.metrics.Uptime = time.Since(h.metrics.startTime)
}

func (h *HealthAgent) sendHealthReport() {
    report := map[string]interface{}{
        "agent_id": h.ID,
        "status":   h.getHealthStatus(),
        "metrics": map[string]interface{}{
            "cpu_usage_percent":    h.metrics.CPUUsage,
            "memory_usage_percent": h.metrics.MemoryUsage,
            "active_tasks":         h.metrics.ActiveTasks,
            "completed_tasks":      h.metrics.CompletedTasks,
            "failed_tasks":         h.metrics.FailedTasks,
            "uptime_seconds":       h.metrics.Uptime.Seconds(),
        },
        "workload": map[string]interface{}{
            "current_load":    h.calculateLoad(),
            "queue_depth":     h.metrics.ActiveTasks,
            "processing_rate": h.calculateProcessingRate(),
        },
    }

    msg := Message{
        ID:     generateID(),
        Type:   "request",
        Method: "agent.heartbeat",
        Params: report,
    }

    h.SendMessage(msg)
}

func (h *HealthAgent) getHealthStatus() string {
    if h.metrics.CPUUsage > 90 || h.metrics.MemoryUsage > 90 {
        return "degraded"
    }
    if h.metrics.FailedTasks > h.metrics.CompletedTasks * 0.1 {
        return "unhealthy"
    }
    return "healthy"
}

func (h *HealthAgent) calculateLoad() float64 {
    // Simple load calculation
    return (h.metrics.CPUUsage + h.metrics.MemoryUsage) / 200.0
}

func (h *HealthAgent) calculateProcessingRate() float64 {
    if h.metrics.Uptime.Seconds() == 0 {
        return 0
    }
    return float64(h.metrics.CompletedTasks) / h.metrics.Uptime.Seconds()
}
```

## OpenAI GPT-4 Agent

### Complete GPT-4 Code Review Agent

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "strings"

    "github.com/sashabaranov/go-openai"
)

type GPT4CodeReviewAgent struct {
    *BaseAgent
    client *openai.Client
    config CodeReviewConfig
}

type CodeReviewConfig struct {
    Model            string
    Temperature      float32
    MaxTokens        int
    SystemPrompt     string
    ReviewCategories []string
}

func NewGPT4CodeReviewAgent() *GPT4CodeReviewAgent {
    apiKey := os.Getenv("OPENAI_API_KEY")
    if apiKey == "" {
        log.Fatal("OPENAI_API_KEY not set")
    }

    return &GPT4CodeReviewAgent{
        BaseAgent: NewBaseAgent("gpt4-code-reviewer-001", "code-review"),
        client:    openai.NewClient(apiKey),
        config: CodeReviewConfig{
            Model:       openai.GPT4TurboPreview,
            Temperature: 0.3,
            MaxTokens:   4000,
            SystemPrompt: `You are an expert code reviewer specializing in:
- Security vulnerabilities (OWASP Top 10, injection attacks, authentication issues)
- Performance optimization (algorithmic complexity, memory usage, bottlenecks)
- Code quality (readability, maintainability, best practices)
- Error handling and edge cases
- Concurrency issues and race conditions

Provide detailed, actionable feedback with specific line numbers and code examples.`,
            ReviewCategories: []string{
                "security",
                "performance",
                "quality",
                "errors",
                "concurrency",
            },
        },
    }
}

func (g *GPT4CodeReviewAgent) Start(ctx context.Context) error {
    // Register capabilities
    if err := g.Register([]Capability{
        {
            Name:        "code_review",
            Confidence:  0.92,
            Languages:   []string{"go", "python", "javascript", "typescript", "java"},
            Specialties: g.config.ReviewCategories,
        },
        {
            Name:        "security_analysis",
            Confidence:  0.88,
            Specialties: []string{"owasp", "authentication", "authorization", "injection"},
        },
        {
            Name:        "performance_analysis",
            Confidence:  0.85,
            Specialties: []string{"complexity", "memory", "cpu", "bottlenecks"},
        },
    }); err != nil {
        return err
    }

    return g.ProcessMessages(ctx)
}

func (g *GPT4CodeReviewAgent) HandleTask(ctx context.Context, task Task) error {
    switch task.Type {
    case "code_review":
        return g.performCodeReview(ctx, task)
    case "security_audit":
        return g.performSecurityAudit(ctx, task)
    default:
        return fmt.Errorf("unsupported task type: %s", task.Type)
    }
}

func (g *GPT4CodeReviewAgent) performCodeReview(ctx context.Context, task Task) error {
    // Extract code from task
    codeData := task.Context.(map[string]interface{})
    code := codeData["code"].(string)
    language := codeData["language"].(string)
    
    // Prepare review prompt
    prompt := fmt.Sprintf(`Review the following %s code:

\`\`\`%s
%s
\`\`\`

Provide a comprehensive review covering:
1. Security vulnerabilities
2. Performance issues
3. Code quality and best practices
4. Error handling
5. Potential bugs or edge cases

Format your response as JSON with the following structure:
{
  "summary": "overall assessment",
  "score": 0-100,
  "findings": [
    {
      "type": "security|performance|quality|error|bug",
      "severity": "critical|high|medium|low",
      "line": line_number,
      "description": "detailed description",
      "suggestion": "how to fix",
      "code_example": "fixed code snippet"
    }
  ],
  "metrics": {
    "complexity": "score",
    "maintainability": "score",
    "test_coverage": "estimated percentage"
  }
}`, language, language, code)

    // Call GPT-4
    resp, err := g.client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
        Model:       g.config.Model,
        Temperature: g.config.Temperature,
        MaxTokens:   g.config.MaxTokens,
        Messages: []openai.ChatCompletionMessage{
            {
                Role:    openai.ChatMessageRoleSystem,
                Content: g.config.SystemPrompt,
            },
            {
                Role:    openai.ChatMessageRoleUser,
                Content: prompt,
            },
        },
    })

    if err != nil {
        return fmt.Errorf("GPT-4 API error: %w", err)
    }

    // Parse response
    var review CodeReview
    content := resp.Choices[0].Message.Content
    if err := json.Unmarshal([]byte(content), &review); err != nil {
        // Try to extract JSON from response
        start := strings.Index(content, "{")
        end := strings.LastIndex(content, "}")
        if start >= 0 && end > start {
            jsonStr := content[start:end+1]
            if err := json.Unmarshal([]byte(jsonStr), &review); err != nil {
                return fmt.Errorf("failed to parse review: %w", err)
            }
        } else {
            return fmt.Errorf("failed to parse review: %w", err)
        }
    }

    // Send result
    return g.CompleteTask(task.ID, map[string]interface{}{
        "review":     review,
        "model":      g.config.Model,
        "tokens_used": resp.Usage.TotalTokens,
        "cost_usd":   g.calculateCost(resp.Usage),
    })
}

func (g *GPT4CodeReviewAgent) performSecurityAudit(ctx context.Context, task Task) error {
    // Specialized security-focused review
    securityPrompt := `Focus exclusively on security vulnerabilities:
- SQL/NoSQL injection
- XSS (Cross-Site Scripting)
- CSRF (Cross-Site Request Forgery)
- Authentication/Authorization flaws
- Sensitive data exposure
- Cryptography misuse
- Insecure dependencies
- Path traversal
- Command injection
- XXE (XML External Entity) attacks`

    // Similar implementation with security-specific prompting
    // ...
}

func (g *GPT4CodeReviewAgent) calculateCost(usage openai.Usage) float64 {
    // GPT-4 Turbo pricing (as of 2024)
    inputCost := float64(usage.PromptTokens) * 0.01 / 1000
    outputCost := float64(usage.CompletionTokens) * 0.03 / 1000
    return inputCost + outputCost
}

type CodeReview struct {
    Summary  string     `json:"summary"`
    Score    int        `json:"score"`
    Findings []Finding  `json:"findings"`
    Metrics  Metrics    `json:"metrics"`
}

type Finding struct {
    Type        string `json:"type"`
    Severity    string `json:"severity"`
    Line        int    `json:"line"`
    Description string `json:"description"`
    Suggestion  string `json:"suggestion"`
    CodeExample string `json:"code_example,omitempty"`
}

type Metrics struct {
    Complexity      string `json:"complexity"`
    Maintainability string `json:"maintainability"`
    TestCoverage    string `json:"test_coverage"`
}
```

## Anthropic Claude Agent

### Claude 3 Documentation Agent

```go
package main

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "time"
)

type ClaudeDocAgent struct {
    *BaseAgent
    apiKey    string
    model     string
    maxTokens int
}

type ClaudeRequest struct {
    Model     string    `json:"model"`
    Messages  []Message `json:"messages"`
    MaxTokens int       `json:"max_tokens"`
    Temperature float32 `json:"temperature,omitempty"`
    System    string    `json:"system,omitempty"`
}

type ClaudeMessage struct {
    Role    string `json:"role"`
    Content string `json:"content"`
}

type ClaudeResponse struct {
    Content []struct {
        Text string `json:"text"`
    } `json:"content"`
    Usage struct {
        InputTokens  int `json:"input_tokens"`
        OutputTokens int `json:"output_tokens"`
    } `json:"usage"`
}

func NewClaudeDocAgent() *ClaudeDocAgent {
    apiKey := os.Getenv("ANTHROPIC_API_KEY")
    if apiKey == "" {
        log.Fatal("ANTHROPIC_API_KEY not set")
    }

    return &ClaudeDocAgent{
        BaseAgent: NewBaseAgent("claude-doc-agent-001", "documentation"),
        apiKey:    apiKey,
        model:     "claude-3-opus-20240229",
        maxTokens: 4000,
    }
}

func (c *ClaudeDocAgent) Start(ctx context.Context) error {
    // Register capabilities
    if err := c.Register([]Capability{
        {
            Name:        "documentation_generation",
            Confidence:  0.95,
            Specialties: []string{"api-docs", "readme", "guides", "tutorials"},
        },
        {
            Name:        "code_explanation",
            Confidence:  0.93,
            Specialties: []string{"architecture", "design-patterns", "algorithms"},
        },
        {
            Name:        "technical_writing",
            Confidence:  0.90,
            Specialties: []string{"clarity", "structure", "examples"},
        },
    }); err != nil {
        return err
    }

    return c.ProcessMessages(ctx)
}

func (c *ClaudeDocAgent) HandleTask(ctx context.Context, task Task) error {
    switch task.Type {
    case "generate_docs":
        return c.generateDocumentation(ctx, task)
    case "explain_code":
        return c.explainCode(ctx, task)
    case "improve_docs":
        return c.improveDocumentation(ctx, task)
    default:
        return fmt.Errorf("unsupported task type: %s", task.Type)
    }
}

func (c *ClaudeDocAgent) generateDocumentation(ctx context.Context, task Task) error {
    data := task.Context.(map[string]interface{})
    codeFiles := data["files"].([]interface{})
    docType := data["doc_type"].(string)
    
    // Build comprehensive prompt
    prompt := c.buildDocPrompt(codeFiles, docType)
    
    // Call Claude API
    response, err := c.callClaude(ctx, prompt, "You are an expert technical writer specializing in creating clear, comprehensive, and user-friendly documentation for software projects.")
    if err != nil {
        return err
    }
    
    // Parse and structure the documentation
    docs := c.structureDocumentation(response.Content[0].Text, docType)
    
    // Complete task with documentation
    return c.CompleteTask(task.ID, map[string]interface{}{
        "documentation": docs,
        "doc_type":      docType,
        "model":         c.model,
        "tokens_used":   response.Usage.InputTokens + response.Usage.OutputTokens,
    })
}

func (c *ClaudeDocAgent) explainCode(ctx context.Context, task Task) error {
    data := task.Context.(map[string]interface{})
    code := data["code"].(string)
    language := data["language"].(string)
    level := data["explanation_level"].(string) // beginner, intermediate, expert
    
    prompt := fmt.Sprintf(`Explain this %s code at a %s level:

\`\`\`%s
%s
\`\`\`

Provide:
1. High-level overview
2. Step-by-step explanation
3. Key concepts used
4. Potential improvements
5. Common use cases`, language, level, language, code)
    
    response, err := c.callClaude(ctx, prompt, "You are an expert programmer and teacher who excels at explaining complex code in simple terms.")
    if err != nil {
        return err
    }
    
    return c.CompleteTask(task.ID, map[string]interface{}{
        "explanation": response.Content[0].Text,
        "model":       c.model,
        "tokens_used": response.Usage.InputTokens + response.Usage.OutputTokens,
    })
}

func (c *ClaudeDocAgent) callClaude(ctx context.Context, prompt, system string) (*ClaudeResponse, error) {
    request := ClaudeRequest{
        Model:     c.model,
        MaxTokens: c.maxTokens,
        System:    system,
        Messages: []ClaudeMessage{
            {
                Role:    "user",
                Content: prompt,
            },
        },
    }
    
    jsonData, err := json.Marshal(request)
    if err != nil {
        return nil, err
    }
    
    req, err := http.NewRequestWithContext(ctx, "POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("anthropic-version", "2023-06-01")
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, err
    }
    
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("Claude API error: %s", string(body))
    }
    
    var claudeResp ClaudeResponse
    if err := json.Unmarshal(body, &claudeResp); err != nil {
        return nil, err
    }
    
    return &claudeResp, nil
}

func (c *ClaudeDocAgent) buildDocPrompt(codeFiles []interface{}, docType string) string {
    var prompt strings.Builder
    
    prompt.WriteString(fmt.Sprintf("Generate %s documentation for the following code:\n\n", docType))
    
    for _, file := range codeFiles {
        fileData := file.(map[string]interface{})
        prompt.WriteString(fmt.Sprintf("File: %s\n```%s\n%s\n```\n\n", 
            fileData["name"], 
            fileData["language"], 
            fileData["content"]))
    }
    
    switch docType {
    case "api":
        prompt.WriteString("\nGenerate OpenAPI 3.0 specification with detailed endpoint documentation.")
    case "readme":
        prompt.WriteString("\nGenerate a comprehensive README.md with installation, usage, examples, and API reference.")
    case "architecture":
        prompt.WriteString("\nGenerate architectural documentation with diagrams (mermaid), design decisions, and component interactions.")
    }
    
    return prompt.String()
}

func (c *ClaudeDocAgent) structureDocumentation(content, docType string) interface{} {
    // Parse and structure based on doc type
    switch docType {
    case "api":
        // Parse OpenAPI spec
        var spec map[string]interface{}
        json.Unmarshal([]byte(content), &spec)
        return spec
    case "readme":
        // Structure markdown sections
        return map[string]interface{}{
            "content": content,
            "sections": c.extractMarkdownSections(content),
        }
    default:
        return content
    }
}
```

## AWS Bedrock Multi-Model Agent

### Bedrock Agent with Model Selection

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "log"
    
    "github.com/aws/aws-sdk-go-v2/config"
    "github.com/aws/aws-sdk-go-v2/service/bedrock"
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockMultiModelAgent struct {
    *BaseAgent
    client        *bedrockruntime.Client
    modelSelector *ModelSelector
    models        map[string]ModelConfig
}

type ModelConfig struct {
    ModelID      string
    Provider     string
    Capabilities []string
    CostPerToken float64
    MaxTokens    int
}

type ModelSelector struct {
    strategy SelectionStrategy
}

type SelectionStrategy interface {
    SelectModel(task Task, models map[string]ModelConfig) (string, error)
}

func NewBedrockMultiModelAgent(ctx context.Context) (*BedrockMultiModelAgent, error) {
    cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
    if err != nil {
        return nil, err
    }
    
    return &BedrockMultiModelAgent{
        BaseAgent: NewBaseAgent("bedrock-multi-001", "multi-model"),
        client:    bedrockruntime.NewFromConfig(cfg),
        modelSelector: &ModelSelector{
            strategy: &CapabilityBasedStrategy{},
        },
        models: map[string]ModelConfig{
            "claude-3": {
                ModelID:      "anthropic.claude-3-sonnet-20240229-v1:0",
                Provider:     "anthropic",
                Capabilities: []string{"reasoning", "analysis", "writing"},
                CostPerToken: 0.003,
                MaxTokens:    4096,
            },
            "llama2-70b": {
                ModelID:      "meta.llama2-70b-chat-v1",
                Provider:     "meta",
                Capabilities: []string{"chat", "general", "code"},
                CostPerToken: 0.00195,
                MaxTokens:    4096,
            },
            "titan-text": {
                ModelID:      "amazon.titan-text-express-v1",
                Provider:     "amazon",
                Capabilities: []string{"summarization", "qa", "extraction"},
                CostPerToken: 0.0008,
                MaxTokens:    8192,
            },
            "cohere-command": {
                ModelID:      "cohere.command-text-v14",
                Provider:     "cohere",
                Capabilities: []string{"generation", "classification"},
                CostPerToken: 0.0015,
                MaxTokens:    4096,
            },
        },
    }, nil
}

func (b *BedrockMultiModelAgent) Start(ctx context.Context) error {
    // Register with multi-model capabilities
    var allCapabilities []Capability
    
    for _, model := range b.models {
        for _, cap := range model.Capabilities {
            allCapabilities = append(allCapabilities, Capability{
                Name:       cap,
                Confidence: 0.85,
                Model:      model.ModelID,
            })
        }
    }
    
    if err := b.Register(allCapabilities); err != nil {
        return err
    }
    
    return b.ProcessMessages(ctx)
}

func (b *BedrockMultiModelAgent) HandleTask(ctx context.Context, task Task) error {
    // Select best model for task
    modelID, err := b.modelSelector.SelectModel(task, b.models)
    if err != nil {
        return err
    }
    
    model := b.models[modelID]
    log.Printf("Selected model %s for task %s", model.ModelID, task.ID)
    
    // Route to appropriate handler
    switch model.Provider {
    case "anthropic":
        return b.handleClaudeTask(ctx, task, model)
    case "meta":
        return b.handleLlamaTask(ctx, task, model)
    case "amazon":
        return b.handleTitanTask(ctx, task, model)
    case "cohere":
        return b.handleCohereTask(ctx, task, model)
    default:
        return fmt.Errorf("unsupported provider: %s", model.Provider)
    }
}

func (b *BedrockMultiModelAgent) handleClaudeTask(ctx context.Context, task Task, model ModelConfig) error {
    // Claude-specific request format
    request := map[string]interface{}{
        "anthropic_version": "bedrock-2023-05-31",
        "max_tokens":        model.MaxTokens,
        "messages": []map[string]string{
            {
                "role":    "user",
                "content": b.buildPrompt(task),
            },
        },
    }
    
    return b.invokeModel(ctx, model.ModelID, request, task)
}

func (b *BedrockMultiModelAgent) handleLlamaTask(ctx context.Context, task Task, model ModelConfig) error {
    // Llama-specific request format
    prompt := fmt.Sprintf("[INST] %s [/INST]", b.buildPrompt(task))
    
    request := map[string]interface{}{
        "prompt":      prompt,
        "max_gen_len": model.MaxTokens,
        "temperature": 0.7,
        "top_p":       0.9,
    }
    
    return b.invokeModel(ctx, model.ModelID, request, task)
}

func (b *BedrockMultiModelAgent) invokeModel(ctx context.Context, modelID string, request interface{}, task Task) error {
    jsonBytes, err := json.Marshal(request)
    if err != nil {
        return err
    }
    
    input := &bedrockruntime.InvokeModelInput{
        ModelId:     &modelID,
        Body:        jsonBytes,
        ContentType: aws.String("application/json"),
    }
    
    resp, err := b.client.InvokeModel(ctx, input)
    if err != nil {
        return err
    }
    
    // Parse response based on model
    result := b.parseModelResponse(modelID, resp.Body)
    
    // Track costs
    tokensUsed := b.estimateTokens(request, result)
    cost := tokensUsed * b.models[modelID].CostPerToken / 1000
    
    return b.CompleteTask(task.ID, map[string]interface{}{
        "result":      result,
        "model":       modelID,
        "tokens_used": tokensUsed,
        "cost_usd":    cost,
    })
}

// Capability-based model selection strategy
type CapabilityBasedStrategy struct{}

func (s *CapabilityBasedStrategy) SelectModel(task Task, models map[string]ModelConfig) (string, error) {
    var bestModel string
    bestScore := 0.0
    
    for modelID, config := range models {
        score := s.scoreModel(task, config)
        if score > bestScore {
            bestScore = score
            bestModel = modelID
        }
    }
    
    if bestModel == "" {
        return "", fmt.Errorf("no suitable model found for task")
    }
    
    return bestModel, nil
}

func (s *CapabilityBasedStrategy) scoreModel(task Task, model ModelConfig) float64 {
    score := 0.0
    
    // Check capability match
    for _, cap := range model.Capabilities {
        if s.taskRequiresCapability(task, cap) {
            score += 1.0
        }
    }
    
    // Factor in cost (inverse relationship)
    score += 1.0 / model.CostPerToken
    
    // Factor in context window if needed
    if task.EstimatedTokens > 4096 && model.MaxTokens > 4096 {
        score += 0.5
    }
    
    return score
}
```

## Specialized Agents

### 1. Security Scanning Agent

```go
package main

import (
    "context"
    "fmt"
    "regexp"
    "strings"
)

type SecurityScanAgent struct {
    *BaseAgent
    scanners []SecurityScanner
    rules    *SecurityRules
}

type SecurityScanner interface {
    Scan(code string, language string) []SecurityIssue
}

type SecurityIssue struct {
    Type        string
    Severity    string
    Line        int
    Column      int
    Description string
    CWE         string
    OWASP       string
    Fix         string
}

type SecurityRules struct {
    Patterns map[string][]*SecurityPattern
}

type SecurityPattern struct {
    Name     string
    Pattern  *regexp.Regexp
    Severity string
    CWE      string
    Message  string
    Fix      string
}

func NewSecurityScanAgent() *SecurityScanAgent {
    return &SecurityScanAgent{
        BaseAgent: NewBaseAgent("security-scanner-001", "security"),
        scanners: []SecurityScanner{
            &SQLInjectionScanner{},
            &XSSScanner{},
            &CryptoScanner{},
            &AuthScanner{},
            &PathTraversalScanner{},
        },
        rules: loadSecurityRules(),
    }
}

// SQL Injection Scanner
type SQLInjectionScanner struct{}

func (s *SQLInjectionScanner) Scan(code string, language string) []SecurityIssue {
    var issues []SecurityIssue
    
    patterns := map[string]*regexp.Regexp{
        "string_concat": regexp.MustCompile(`(?i)(query|execute|prepare)\s*\(\s*['"\x60].*?\+.*?['"\x60]`),
        "format_string": regexp.MustCompile(`(?i)fmt\.Sprintf\s*\(\s*['"\x60].*?(SELECT|INSERT|UPDATE|DELETE).*?['"\x60].*?,.*?\)`),
        "raw_query":     regexp.MustCompile(`(?i)\.Raw\s*\(\s*['"\x60].*?\$\{.*?\}.*?['"\x60]`),
    }
    
    lines := strings.Split(code, "\n")
    for lineNum, line := range lines {
        for patternName, pattern := range patterns {
            if matches := pattern.FindStringSubmatch(line); matches != nil {
                issues = append(issues, SecurityIssue{
                    Type:        "SQL Injection",
                    Severity:    "critical",
                    Line:        lineNum + 1,
                    Description: fmt.Sprintf("Potential SQL injection via %s", patternName),
                    CWE:         "CWE-89",
                    OWASP:       "A03:2021",
                    Fix:         "Use parameterized queries or prepared statements",
                })
            }
        }
    }
    
    return issues
}

// Cryptography Scanner
type CryptoScanner struct{}

func (c *CryptoScanner) Scan(code string, language string) []SecurityIssue {
    var issues []SecurityIssue
    
    weakAlgorithms := map[string]string{
        "MD5":    "Use SHA-256 or stronger",
        "SHA1":   "Use SHA-256 or stronger",
        "DES":    "Use AES-256",
        "RC4":    "Use AES-256-GCM",
        "ECB":    "Use CBC or GCM mode",
    }
    
    for algo, fix := range weakAlgorithms {
        pattern := regexp.MustCompile(`(?i)\b` + algo + `\b`)
        if matches := pattern.FindAllStringIndex(code, -1); matches != nil {
            for _, match := range matches {
                lineNum := strings.Count(code[:match[0]], "\n") + 1
                issues = append(issues, SecurityIssue{
                    Type:        "Weak Cryptography",
                    Severity:    "high",
                    Line:        lineNum,
                    Description: fmt.Sprintf("Weak cryptographic algorithm: %s", algo),
                    CWE:         "CWE-327",
                    OWASP:       "A02:2021",
                    Fix:         fix,
                })
            }
        }
    }
    
    return issues
}

func (s *SecurityScanAgent) HandleTask(ctx context.Context, task Task) error {
    data := task.Context.(map[string]interface{})
    code := data["code"].(string)
    language := data["language"].(string)
    
    // Run all scanners
    var allIssues []SecurityIssue
    for _, scanner := range s.scanners {
        issues := scanner.Scan(code, language)
        allIssues = append(allIssues, issues...)
    }
    
    // Apply additional rules
    ruleIssues := s.applyRules(code, language)
    allIssues = append(allIssues, ruleIssues...)
    
    // Generate report
    report := s.generateSecurityReport(allIssues)
    
    return s.CompleteTask(task.ID, map[string]interface{}{
        "report":       report,
        "issues":       allIssues,
        "total_issues": len(allIssues),
        "critical":     s.countBySeverity(allIssues, "critical"),
        "high":         s.countBySeverity(allIssues, "high"),
        "medium":       s.countBySeverity(allIssues, "medium"),
        "low":          s.countBySeverity(allIssues, "low"),
    })
}
```

### 2. Performance Profiling Agent

```go
package main

import (
    "context"
    "go/ast"
    "go/parser"
    "go/token"
    "strings"
)

type PerformanceAgent struct {
    *BaseAgent
    analyzer *PerformanceAnalyzer
}

type PerformanceAnalyzer struct {
    complexityThreshold int
    checks              []PerformanceCheck
}

type PerformanceCheck interface {
    Check(file *ast.File) []PerformanceIssue
}

type PerformanceIssue struct {
    Type        string
    Location    string
    Description string
    Impact      string
    Suggestion  string
    Complexity  int
}

func NewPerformanceAgent() *PerformanceAgent {
    return &PerformanceAgent{
        BaseAgent: NewBaseAgent("performance-profiler-001", "performance"),
        analyzer: &PerformanceAnalyzer{
            complexityThreshold: 10,
            checks: []PerformanceCheck{
                &ComplexityCheck{},
                &AllocationCheck{},
                &LoopCheck{},
                &ConcurrencyCheck{},
            },
        },
    }
}

// Cyclomatic Complexity Check
type ComplexityCheck struct{}

func (c *ComplexityCheck) Check(file *ast.File) []PerformanceIssue {
    var issues []PerformanceIssue
    
    ast.Inspect(file, func(n ast.Node) bool {
        switch fn := n.(type) {
        case *ast.FuncDecl:
            complexity := c.calculateComplexity(fn)
            if complexity > 10 {
                issues = append(issues, PerformanceIssue{
                    Type:        "High Complexity",
                    Location:    fn.Name.Name,
                    Description: fmt.Sprintf("Function has cyclomatic complexity of %d", complexity),
                    Impact:      "Hard to maintain and test, potential performance issues",
                    Suggestion:  "Refactor into smaller functions",
                    Complexity:  complexity,
                })
            }
        }
        return true
    })
    
    return issues
}

func (c *ComplexityCheck) calculateComplexity(fn *ast.FuncDecl) int {
    complexity := 1 // Base complexity
    
    ast.Inspect(fn, func(n ast.Node) bool {
        switch n.(type) {
        case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt:
            complexity++
        case *ast.CaseClause:
            complexity++
        }
        return true
    })
    
    return complexity
}

// Memory Allocation Check
type AllocationCheck struct{}

func (a *AllocationCheck) Check(file *ast.File) []PerformanceIssue {
    var issues []PerformanceIssue
    
    ast.Inspect(file, func(n ast.Node) bool {
        switch node := n.(type) {
        case *ast.CallExpr:
            if a.isAllocationInLoop(node) {
                issues = append(issues, PerformanceIssue{
                    Type:        "Loop Allocation",
                    Location:    a.getLocation(node),
                    Description: "Memory allocation inside loop",
                    Impact:      "High GC pressure, performance degradation",
                    Suggestion:  "Pre-allocate outside loop or use sync.Pool",
                })
            }
        }
        return true
    })
    
    return issues
}

func (p *PerformanceAgent) HandleTask(ctx context.Context, task Task) error {
    data := task.Context.(map[string]interface{})
    code := data["code"].(string)
    
    // Parse Go code
    fset := token.NewFileSet()
    file, err := parser.ParseFile(fset, "performance.go", code, parser.ParseComments)
    if err != nil {
        return fmt.Errorf("parse error: %w", err)
    }
    
    // Run all checks
    var allIssues []PerformanceIssue
    for _, check := range p.analyzer.checks {
        issues := check.Check(file)
        allIssues = append(allIssues, issues...)
    }
    
    // Generate optimization recommendations
    recommendations := p.generateOptimizations(allIssues)
    
    return p.CompleteTask(task.ID, map[string]interface{}{
        "issues":          allIssues,
        "recommendations": recommendations,
        "summary":         p.generateSummary(allIssues),
    })
}
```

## Multi-Agent Collaboration

### MapReduce Code Analysis System

```go
package main

import (
    "context"
    "sync"
)

type CodeAnalysisCoordinator struct {
    *BaseAgent
    agents map[string]Agent
}

func NewCodeAnalysisCoordinator() *CodeAnalysisCoordinator {
    return &CodeAnalysisCoordinator{
        BaseAgent: NewBaseAgent("coordinator-001", "coordinator"),
        agents: map[string]Agent{
            "security":     NewSecurityScanAgent(),
            "performance":  NewPerformanceAgent(),
            "quality":      NewCodeQualityAgent(),
            "dependencies": NewDependencyAgent(),
        },
    }
}

func (c *CodeAnalysisCoordinator) HandleTask(ctx context.Context, task Task) error {
    // Map phase - distribute subtasks
    subtasks := c.createSubtasks(task)
    
    // Execute subtasks in parallel
    results := make(chan SubTaskResult, len(subtasks))
    var wg sync.WaitGroup
    
    for agentType, subtask := range subtasks {
        wg.Add(1)
        go func(aType string, st SubTask) {
            defer wg.Done()
            
            agent := c.agents[aType]
            result, err := c.executeSubtask(ctx, agent, st)
            results <- SubTaskResult{
                AgentType: aType,
                Result:    result,
                Error:     err,
            }
        }(agentType, subtask)
    }
    
    // Wait for all subtasks
    wg.Wait()
    close(results)
    
    // Reduce phase - combine results
    finalReport := c.reduceResults(results)
    
    return c.CompleteTask(task.ID, finalReport)
}

func (c *CodeAnalysisCoordinator) createSubtasks(task Task) map[string]SubTask {
    data := task.Context.(map[string]interface{})
    
    return map[string]SubTask{
        "security": {
            Type:    "security_scan",
            Context: data,
        },
        "performance": {
            Type:    "performance_analysis",
            Context: data,
        },
        "quality": {
            Type:    "quality_check",
            Context: data,
        },
        "dependencies": {
            Type:    "dependency_audit",
            Context: data,
        },
    }
}

func (c *CodeAnalysisCoordinator) reduceResults(results chan SubTaskResult) map[string]interface{} {
    report := map[string]interface{}{
        "timestamp": time.Now(),
        "analyses":  map[string]interface{}{},
        "summary":   map[string]interface{}{},
    }
    
    var totalIssues int
    severityCounts := map[string]int{}
    
    for result := range results {
        if result.Error != nil {
            report["analyses"].(map[string]interface{})[result.AgentType] = map[string]interface{}{
                "error": result.Error.Error(),
            }
            continue
        }
        
        report["analyses"].(map[string]interface{})[result.AgentType] = result.Result
        
        // Aggregate metrics
        if issues, ok := result.Result["issues"].([]interface{}); ok {
            totalIssues += len(issues)
            for _, issue := range issues {
                if severity, ok := issue.(map[string]interface{})["severity"].(string); ok {
                    severityCounts[severity]++
                }
            }
        }
    }
    
    report["summary"] = map[string]interface{}{
        "total_issues":    totalIssues,
        "severity_counts": severityCounts,
        "risk_score":      c.calculateRiskScore(severityCounts),
    }
    
    return report
}
```

## Production Examples

### 1. Complete Production Agent with All Features

```go
package main

import (
    "context"
    "time"
    
    "github.com/prometheus/client_golang/prometheus"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

type ProductionAgent struct {
    *BaseAgent
    config      *Config
    metrics     *AgentMetrics
    tracer      trace.Tracer
    limiter     *RateLimiter
    circuit     *CircuitBreaker
    cache       *Cache
}

func NewProductionAgent(config *Config) (*ProductionAgent, error) {
    tracer := otel.Tracer("production-agent")
    
    return &ProductionAgent{
        BaseAgent: NewBaseAgent(config.AgentID, config.AgentType),
        config:    config,
        metrics:   NewAgentMetrics(),
        tracer:    tracer,
        limiter:   NewRateLimiter(config.RateLimit),
        circuit:   NewCircuitBreaker(config.CircuitBreaker),
        cache:     NewCache(config.Cache),
    }, nil
}

func (p *ProductionAgent) Start(ctx context.Context) error {
    // Initialize with retries
    err := p.initializeWithRetry(ctx, 3)
    if err != nil {
        return err
    }
    
    // Start background workers
    go p.metricsReporter(ctx)
    go p.healthChecker(ctx)
    go p.cacheManager(ctx)
    
    // Main message processing loop
    return p.processMessagesWithRecovery(ctx)
}

func (p *ProductionAgent) processMessagesWithRecovery(ctx context.Context) error {
    for {
        select {
        case <-ctx.Done():
            return ctx.Err()
        default:
            if err := p.ProcessMessages(ctx); err != nil {
                p.handleError(err)
                if p.shouldReconnect(err) {
                    if err := p.reconnect(ctx); err != nil {
                        return err
                    }
                }
            }
        }
    }
}

func (p *ProductionAgent) HandleTask(ctx context.Context, task Task) error {
    // Start span
    ctx, span := p.tracer.Start(ctx, "handle_task",
        trace.WithAttributes(
            attribute.String("task.id", task.ID),
            attribute.String("task.type", task.Type),
        ),
    )
    defer span.End()
    
    // Check rate limit
    if !p.limiter.Allow(task.Type) {
        p.metrics.rateLimited.Inc()
        return ErrRateLimited
    }
    
    // Check circuit breaker
    if !p.circuit.Allow() {
        p.metrics.circuitOpen.Inc()
        return ErrCircuitOpen
    }
    
    // Check cache
    if result, found := p.cache.Get(task.CacheKey()); found {
        p.metrics.cacheHits.Inc()
        return p.CompleteTask(task.ID, result)
    }
    
    // Process task
    start := time.Now()
    result, err := p.processTask(ctx, task)
    duration := time.Since(start)
    
    // Record metrics
    p.metrics.taskDuration.WithLabelValues(task.Type).Observe(duration.Seconds())
    
    if err != nil {
        p.circuit.RecordFailure()
        p.metrics.taskErrors.WithLabelValues(task.Type).Inc()
        span.RecordError(err)
        return err
    }
    
    p.circuit.RecordSuccess()
    p.metrics.taskCompleted.WithLabelValues(task.Type).Inc()
    
    // Cache result
    p.cache.Set(task.CacheKey(), result, p.config.CacheTTL)
    
    return p.CompleteTask(task.ID, result)
}

// Graceful shutdown
func (p *ProductionAgent) Shutdown(ctx context.Context) error {
    log.Info("Starting graceful shutdown")
    
    // Stop accepting new tasks
    p.SetState(StateDraining)
    
    // Wait for active tasks to complete
    timeout := time.NewTimer(30 * time.Second)
    ticker := time.NewTicker(1 * time.Second)
    defer timeout.Stop()
    defer ticker.Stop()
    
    for {
        select {
        case <-timeout.C:
            return ErrShutdownTimeout
        case <-ticker.C:
            if p.metrics.activeTasks.Load() == 0 {
                goto shutdown
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }
    
shutdown:
    // Deregister agent
    if err := p.Deregister(); err != nil {
        log.Error("Failed to deregister:", err)
    }
    
    // Flush metrics
    p.metrics.Flush()
    
    // Close connections
    p.conn.Close()
    
    log.Info("Graceful shutdown completed")
    return nil
}

// Production metrics
type AgentMetrics struct {
    taskCompleted *prometheus.CounterVec
    taskErrors    *prometheus.CounterVec
    taskDuration  *prometheus.HistogramVec
    activeTasks   atomic.Int64
    cacheHits     prometheus.Counter
    rateLimited   prometheus.Counter
    circuitOpen   prometheus.Counter
}

func NewAgentMetrics() *AgentMetrics {
    return &AgentMetrics{
        taskCompleted: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "agent_tasks_completed_total",
                Help: "Total completed tasks",
            },
            []string{"task_type"},
        ),
        taskErrors: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "agent_tasks_errors_total",
                Help: "Total task errors",
            },
            []string{"task_type"},
        ),
        taskDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "agent_task_duration_seconds",
                Help:    "Task processing duration",
                Buckets: []float64{0.1, 0.5, 1, 2, 5, 10, 30},
            },
            []string{"task_type"},
        ),
        cacheHits: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "agent_cache_hits_total",
                Help: "Total cache hits",
            },
        ),
        rateLimited: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "agent_rate_limited_total",
                Help: "Total rate limited requests",
            },
        ),
        circuitOpen: prometheus.NewCounter(
            prometheus.CounterOpts{
                Name: "agent_circuit_open_total",
                Help: "Total circuit breaker rejections",
            },
        ),
    }
}
```

### 2. Docker Deployment Example

```dockerfile
# Multi-stage build for production agent
FROM golang:1.21-alpine AS builder

WORKDIR /app

# Install dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build with optimizations
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-w -s" \
    -o agent \
    ./cmd/agent

# Final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy binary
COPY --from=builder /app/agent .

# Copy config
COPY --from=builder /app/configs/production.yaml ./configs/

# Health check
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
    CMD ["/root/agent", "health"]

# Run as non-root
USER nobody

EXPOSE 8080

CMD ["./agent", "start", "--config", "./configs/production.yaml"]
```

### 3. Kubernetes Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ai-agent-fleet
  namespace: mcp
spec:
  replicas: 5
  selector:
    matchLabels:
      app: ai-agent
  template:
    metadata:
      labels:
        app: ai-agent
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: ai-agent
      containers:
      - name: agent
        image: your-registry/ai-agent:v1.2.0
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: MCP_SERVER_URL
          value: "wss://edge-mcp.mcp.svc.cluster.local:8080 (MCP Server)/ws"
        - name: AGENT_ID
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-credentials
              key: api-key
        - name: AWS_REGION
          value: "us-east-1"
        resources:
          requests:
            memory: "512Mi"
            cpu: "500m"
          limits:
            memory: "2Gi"
            cpu: "2"
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
        volumeMounts:
        - name: config
          mountPath: /etc/agent
          readOnly: true
      volumes:
      - name: config
        configMap:
          name: agent-config
---
apiVersion: v1
kind: Service
metadata:
  name: ai-agent-metrics
  namespace: mcp
spec:
  selector:
    app: ai-agent
  ports:
  - port: 8080
    targetPort: 8080
    name: metrics
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: ai-agent-hpa
  namespace: mcp
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: ai-agent-fleet
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
  - type: Pods
    pods:
      metric:
        name: agent_active_tasks
      target:
        type: AverageValue
        averageValue: "10"
```

## Testing Your Agent

### Integration Test Example

```go
package main

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAgentIntegration(t *testing.T) {
    ctx := context.Background()
    
    // Start test MCP server
    server := StartTestServer(t)
    defer server.Stop()
    
    // Create and start agent
    agent := NewTestAgent()
    go agent.Start(ctx)
    
    // Wait for registration
    require.Eventually(t, func() bool {
        return server.IsAgentRegistered(agent.ID)
    }, 10*time.Second, 100*time.Millisecond)
    
    // Test task assignment
    task := Task{
        ID:   "test-task-001",
        Type: "test",
        Context: map[string]interface{}{
            "action": "echo",
            "data":   "hello world",
        },
    }
    
    server.AssignTask(agent.ID, task)
    
    // Wait for completion
    var result TaskResult
    require.Eventually(t, func() bool {
        r, err := server.GetTaskResult(task.ID)
        if err == nil {
            result = r
            return true
        }
        return false
    }, 5*time.Second, 100*time.Millisecond)
    
    // Verify result
    assert.Equal(t, "completed", result.Status)
    assert.Equal(t, "hello world", result.Data["echo"])
}

func TestAgentReconnection(t *testing.T) {
    ctx := context.Background()
    
    server := StartTestServer(t)
    agent := NewTestAgent()
    
    // Start agent
    go agent.Start(ctx)
    
    // Wait for connection
    require.Eventually(t, func() bool {
        return server.IsAgentConnected(agent.ID)
    }, 5*time.Second, 100*time.Millisecond)
    
    // Simulate connection drop
    server.DisconnectAgent(agent.ID)
    
    // Verify reconnection
    require.Eventually(t, func() bool {
        return server.IsAgentConnected(agent.ID)
    }, 10*time.Second, 100*time.Millisecond)
}
```

## Best Practices

1. **Error Handling**: Always implement comprehensive error handling and recovery
2. **Monitoring**: Include metrics, logging, and tracing from the start
3. **Testing**: Write integration tests for all agent behaviors
4. **Security**: Use secure connections, validate inputs, implement rate limiting
5. **Performance**: Use caching, connection pooling, and efficient serialization
6. **Deployment**: Use health checks, graceful shutdown, and proper resource limits

## Next Steps

1. Review [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details <!-- Source: pkg/models/websocket/binary.go -->
2. See [Agent SDK Guide](./agent-sdk-guide.md) for SDK usage
3. Check [Agent Integration Troubleshooting](./agent-integration-troubleshooting.md) for debugging
4. Explore [Building Custom AI Agents](./building-custom-ai-agents.md) for advanced patterns

## Resources

- [Example Agent Repository](https://github.com/developer-mesh/developer-mesh-agents)
- [Agent Testing Framework](https://github.com/developer-mesh/mcp-agent-test)
- [Production Agent Templates](https://github.com/developer-mesh/mcp-agent-templates)
