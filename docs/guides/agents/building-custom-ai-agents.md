<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:32:40
Verification Script: update-docs-parallel.sh
Batch: ac
-->

# Building Custom AI Agents Guide

> **Purpose**: Comprehensive guide for building custom AI agents from scratch
> **Audience**: Developers creating specialized AI agents for the MCP platform
> **Scope**: Agent architecture, implementation patterns, best practices

## Overview

This guide walks through the complete process of building custom AI agents for the Developer Mesh platform. You'll learn how to design, implement, test, and deploy agents that can leverage various AI models and collaborate with other agents.

## Agent Architecture

### Core Components

```
┌────────────────────────────────────────────────────────────┐
│                    Custom AI Agent                          │
├────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │    Core     │  │  Capability  │  │  Communication  │  │
│  │   Engine    │  │   Manager    │  │     Layer       │  │
│  │             │  │              │  │                 │  │
│  │ - Identity  │  │ - Skills     │  │ - WebSocket     │  │ <!-- Source: pkg/models/websocket/binary.go -->
│  │ - State     │  │ - Models     │  │ - Protocol      │  │
│  │ - Lifecycle │  │ - Routing    │  │ - Messages      │  │
│  └─────────────┘  └──────────────┘  └─────────────────┘  │
├────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌──────────────┐  ┌─────────────────┐  │
│  │    Task     │  │   Context    │  │  Observability  │  │
│  │  Processor  │  │   Manager    │  │     Layer       │  │
│  │             │  │              │  │                 │  │
│  │ - Queue     │  │ - Memory     │  │ - Metrics       │  │
│  │ - Executor  │  │ - State      │  │ - Logging       │  │
│  │ - Results   │  │ - History    │  │ - Tracing       │  │
│  └─────────────┘  └──────────────┘  └─────────────────┘  │
└────────────────────────────────────────────────────────────┘
```

## Project Setup

### 1. Initialize Agent Project

```bash
# Create project structure
mkdir my-custom-agent
cd my-custom-agent

# Initialize Go module
go mod init github.com/yourorg/my-custom-agent

# Create directory structure
mkdir -p {cmd/agent,internal/{agent,tasks,models,capabilities},pkg/{client,config},configs,scripts,deployments}

# Initialize git
git init
echo "# My Custom AI Agent" > README.md
```

### 2. Project Structure

```
my-custom-agent/
├── cmd/
│   └── agent/
│       └── main.go          # Entry point
├── internal/
│   ├── agent/
│   │   ├── agent.go         # Core agent implementation
│   │   ├── lifecycle.go     # Lifecycle management
│   │   └── state.go         # State management
│   ├── tasks/
│   │   ├── processor.go     # Task processing
│   │   ├── queue.go         # Task queue
│   │   └── handlers/        # Task type handlers
│   ├── models/
│   │   ├── integration.go   # AI model integration
│   │   └── providers/       # Model providers
│   └── capabilities/
│       ├── manager.go       # Capability management
│       └── registry.go      # Capability registry
├── pkg/
│   ├── client/
│   │   └── websocket.go     # WebSocket client <!-- Source: pkg/models/websocket/binary.go -->
│   └── config/
│       └── config.go        # Configuration
├── configs/
│   └── agent.yaml           # Default configuration
├── scripts/
│   ├── build.sh            # Build script
│   └── test.sh             # Test script
└── deployments/
    ├── Dockerfile          # Container image
    └── k8s/                # Kubernetes manifests
```

## Core Implementation

### 1. Agent Core

```go
// internal/agent/agent.go
package agent

import (
    "context"
    "sync"
    "time"
    
    "github.com/google/uuid"
    "github.com/yourorg/my-custom-agent/internal/capabilities"
    "github.com/yourorg/my-custom-agent/internal/tasks"
    "github.com/yourorg/my-custom-agent/pkg/client"
    "github.com/yourorg/my-custom-agent/pkg/config"
)

// Agent represents a custom AI agent
type Agent struct {
    // Identity
    ID          string
    Name        string
    Type        string
    Version     string
    
    // Core components
    config      *config.Config
    client      *client.WebSocketClient <!-- Source: pkg/models/websocket/binary.go -->
    capabilities *capabilities.Manager
    taskProcessor *tasks.Processor
    
    // State
    state       *State
    stateMu     sync.RWMutex
    
    // Lifecycle
    ctx         context.Context
    cancel      context.CancelFunc
    started     time.Time
    
    // Channels
    errorChan   chan error
    shutdownChan chan struct{}
}

// NewAgent creates a new agent instance
func NewAgent(cfg *config.Config) (*Agent, error) {
    agentID := uuid.New().String()
    ctx, cancel := context.WithCancel(context.Background())
    
    // Initialize capabilities
    capManager := capabilities.NewManager()
    if err := capManager.LoadFromConfig(cfg.Capabilities); err != nil {
        cancel()
        return nil, fmt.Errorf("failed to load capabilities: %w", err)
    }
    
    // Initialize task processor
    taskProc := tasks.NewProcessor(
        tasks.WithQueueSize(cfg.TaskQueueSize),
        tasks.WithWorkers(cfg.WorkerCount),
        tasks.WithTimeout(cfg.TaskTimeout),
    )
    
    agent := &Agent{
        ID:           agentID,
        Name:         cfg.AgentName,
        Type:         cfg.AgentType,
        Version:      cfg.Version,
        config:       cfg,
        capabilities: capManager,
        taskProcessor: taskProc,
        state:        NewState(),
        ctx:          ctx,
        cancel:       cancel,
        errorChan:    make(chan error, 10),
        shutdownChan: make(chan struct{}),
    }
    
    return agent, nil
}

// Start initializes and starts the agent
func (a *Agent) Start() error {
    a.stateMu.Lock()
    if a.state.Status != StatusCreated {
        a.stateMu.Unlock()
        return fmt.Errorf("agent already started")
    }
    a.state.Status = StatusStarting
    a.started = time.Now()
    a.stateMu.Unlock()
    
    // Connect to MCP server
    wsClient, err := client.NewWebSocketClient(a.config.ServerURL, a.config.APIKey) <!-- Source: pkg/models/websocket/binary.go -->
    if err != nil {
        return fmt.Errorf("failed to create WebSocket client: %w", err) <!-- Source: pkg/models/websocket/binary.go -->
    }
    
    a.client = wsClient
    
    // Register with server
    if err := a.register(); err != nil {
        return fmt.Errorf("registration failed: %w", err)
    }
    
    // Start components
    go a.taskProcessor.Start(a.ctx)
    go a.heartbeatLoop()
    go a.messageHandler()
    go a.errorHandler()
    
    a.stateMu.Lock()
    a.state.Status = StatusRunning
    a.stateMu.Unlock()
    
    log.Info("Agent started successfully", 
        zap.String("agent_id", a.ID),
        zap.String("name", a.Name),
    )
    
    return nil
}

// Stop gracefully shuts down the agent
func (a *Agent) Stop() error {
    a.stateMu.Lock()
    if a.state.Status != StatusRunning {
        a.stateMu.Unlock()
        return fmt.Errorf("agent not running")
    }
    a.state.Status = StatusStopping
    a.stateMu.Unlock()
    
    log.Info("Stopping agent", zap.String("agent_id", a.ID))
    
    // Deregister from server
    if err := a.deregister(); err != nil {
        log.Error("Deregistration failed", zap.Error(err))
    }
    
    // Stop components
    a.cancel()
    
    // Wait for graceful shutdown
    select {
    case <-a.shutdownChan:
        log.Info("Agent stopped gracefully")
    case <-time.After(30 * time.Second):
        log.Warn("Agent stop timeout")
    }
    
    a.stateMu.Lock()
    a.state.Status = StatusStopped
    a.stateMu.Unlock()
    
    return nil
}
```

### 2. Capability System

```go
// internal/capabilities/manager.go
package capabilities

import (
    "fmt"
    "sync"
)

// Capability represents an agent capability
type Capability struct {
    Name        string              `json:"name"`
    Version     string              `json:"version"`
    Description string              `json:"description"`
    Type        CapabilityType      `json:"type"`
    Confidence  float64             `json:"confidence"`
    Parameters  map[string]interface{} `json:"parameters"`
    Constraints CapabilityConstraints  `json:"constraints"`
}

type CapabilityType string

const (
    CapabilityTypeAnalysis   CapabilityType = "analysis"
    CapabilityTypeGeneration CapabilityType = "generation"
    CapabilityTypeProcessing CapabilityType = "processing"
    CapabilityTypeSpecialized CapabilityType = "specialized"
)

// Manager manages agent capabilities
type Manager struct {
    capabilities map[string]*Capability
    mu          sync.RWMutex
    validators  map[CapabilityType]Validator
}

// NewManager creates a new capability manager
func NewManager() *Manager {
    return &Manager{
        capabilities: make(map[string]*Capability),
        validators:   initializeValidators(),
    }
}

// Register adds a new capability
func (m *Manager) Register(cap *Capability) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    
    // Validate capability
    if validator, exists := m.validators[cap.Type]; exists {
        if err := validator.Validate(cap); err != nil {
            return fmt.Errorf("capability validation failed: %w", err)
        }
    }
    
    // Check for conflicts
    if existing, exists := m.capabilities[cap.Name]; exists {
        if existing.Version >= cap.Version {
            return fmt.Errorf("capability %s already registered with version %s", 
                cap.Name, existing.Version)
        }
    }
    
    m.capabilities[cap.Name] = cap
    log.Info("Capability registered", 
        zap.String("name", cap.Name),
        zap.String("type", string(cap.Type)),
    )
    
    return nil
}

// Define specific capabilities
func (m *Manager) DefineCapabilities() error {
    // Code analysis capability
    codeAnalysis := &Capability{
        Name:        "code_analysis",
        Version:     "1.0.0",
        Description: "Analyzes code for quality, security, and performance",
        Type:        CapabilityTypeAnalysis,
        Confidence:  0.92,
        Parameters: map[string]interface{}{
            "languages": []string{"go", "python", "javascript"},
            "max_file_size": 1048576, // 1MB
            "timeout": "5m",
        },
        Constraints: CapabilityConstraints{
            RequiredMemory: "512Mi",
            RequiredCPU:    "250m",
            MaxConcurrent:  10,
        },
    }
    
    if err := m.Register(codeAnalysis); err != nil {
        return err
    }
    
    // Documentation generation capability
    docGen := &Capability{
        Name:        "documentation_generation",
        Version:     "1.0.0",
        Description: "Generates documentation from code and comments",
        Type:        CapabilityTypeGeneration,
        Confidence:  0.88,
        Parameters: map[string]interface{}{
            "formats": []string{"markdown", "html", "pdf"},
            "templates": []string{"api", "readme", "guide"},
        },
        Constraints: CapabilityConstraints{
            RequiredMemory: "256Mi",
            RequiredCPU:    "100m",
            MaxConcurrent:  5,
        },
    }
    
    if err := m.Register(docGen); err != nil {
        return err
    }
    
    // Add more capabilities...
    
    return nil
}

// Match finds capabilities that match requirements
func (m *Manager) Match(requirements []Requirement) ([]*Capability, error) {
    m.mu.RLock()
    defer m.mu.RUnlock()
    
    var matched []*Capability
    
    for _, req := range requirements {
        for _, cap := range m.capabilities {
            if m.matchesRequirement(cap, req) {
                matched = append(matched, cap)
            }
        }
    }
    
    if len(matched) == 0 {
        return nil, ErrNoMatchingCapabilities
    }
    
    // Sort by confidence
    sort.Slice(matched, func(i, j int) bool {
        return matched[i].Confidence > matched[j].Confidence
    })
    
    return matched, nil
}
```

### 3. Task Processing

```go
// internal/tasks/processor.go
package tasks

import (
    "context"
    "fmt"
    "sync"
    "time"
)

// Task represents a unit of work
type Task struct {
    ID          string                 `json:"id"`
    Type        string                 `json:"type"`
    Priority    Priority               `json:"priority"`
    Payload     interface{}            `json:"payload"`
    Metadata    map[string]interface{} `json:"metadata"`
    CreatedAt   time.Time             `json:"created_at"`
    StartedAt   *time.Time            `json:"started_at,omitempty"`
    CompletedAt *time.Time            `json:"completed_at,omitempty"`
    Status      TaskStatus            `json:"status"`
    Result      *TaskResult           `json:"result,omitempty"`
    Error       *TaskError            `json:"error,omitempty"`
}

// Processor handles task execution
type Processor struct {
    queue       *TaskQueue
    handlers    map[string]Handler
    workers     int
    workerPool  chan struct{}
    metrics     *TaskMetrics
    mu          sync.RWMutex
}

// Handler processes a specific task type
type Handler interface {
    Handle(ctx context.Context, task *Task) (*TaskResult, error)
    CanHandle(taskType string) bool
}

// NewProcessor creates a new task processor
func NewProcessor(opts ...ProcessorOption) *Processor {
    cfg := defaultProcessorConfig()
    for _, opt := range opts {
        opt(cfg)
    }
    
    return &Processor{
        queue:      NewTaskQueue(cfg.queueSize),
        handlers:   make(map[string]Handler),
        workers:    cfg.workers,
        workerPool: make(chan struct{}, cfg.workers),
        metrics:    NewTaskMetrics(),
    }
}

// RegisterHandler registers a task handler
func (p *Processor) RegisterHandler(taskType string, handler Handler) error {
    p.mu.Lock()
    defer p.mu.Unlock()
    
    if _, exists := p.handlers[taskType]; exists {
        return fmt.Errorf("handler already registered for type: %s", taskType)
    }
    
    p.handlers[taskType] = handler
    log.Info("Task handler registered", zap.String("type", taskType))
    
    return nil
}

// Start begins processing tasks
func (p *Processor) Start(ctx context.Context) {
    log.Info("Starting task processor", zap.Int("workers", p.workers))
    
    // Initialize worker pool
    for i := 0; i < p.workers; i++ {
        p.workerPool <- struct{}{}
    }
    
    // Start processing loop
    for {
        select {
        case <-ctx.Done():
            log.Info("Task processor stopping")
            return
            
        default:
            task := p.queue.Dequeue(ctx)
            if task == nil {
                continue
            }
            
            // Acquire worker
            <-p.workerPool
            
            // Process task asynchronously
            go p.processTask(ctx, task)
        }
    }
}

// Process a single task
func (p *Processor) processTask(ctx context.Context, task *Task) {
    defer func() {
        // Return worker to pool
        p.workerPool <- struct{}{}
        
        // Recover from panics
        if r := recover(); r != nil {
            log.Error("Task processing panic",
                zap.String("task_id", task.ID),
                zap.Any("panic", r),
            )
            task.Error = &TaskError{
                Code:    "PANIC",
                Message: fmt.Sprintf("Task processing panic: %v", r),
            }
            task.Status = TaskStatusFailed
            p.metrics.RecordFailure(task.Type)
        }
    }()
    
    // Start timing
    startTime := time.Now()
    task.StartedAt = &startTime
    task.Status = TaskStatusProcessing
    
    // Update metrics
    p.metrics.RecordStart(task.Type)
    
    // Find handler
    p.mu.RLock()
    handler, exists := p.handlers[task.Type]
    p.mu.RUnlock()
    
    if !exists {
        task.Error = &TaskError{
            Code:    "NO_HANDLER",
            Message: fmt.Sprintf("No handler for task type: %s", task.Type),
        }
        task.Status = TaskStatusFailed
        p.metrics.RecordFailure(task.Type)
        return
    }
    
    // Execute handler
    taskCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    result, err := handler.Handle(taskCtx, task)
    
    // Update task
    completedTime := time.Now()
    task.CompletedAt = &completedTime
    
    if err != nil {
        task.Error = &TaskError{
            Code:    "PROCESSING_ERROR",
            Message: err.Error(),
        }
        task.Status = TaskStatusFailed
        p.metrics.RecordFailure(task.Type)
        
        log.Error("Task processing failed",
            zap.String("task_id", task.ID),
            zap.String("type", task.Type),
            zap.Error(err),
        )
    } else {
        task.Result = result
        task.Status = TaskStatusCompleted
        p.metrics.RecordSuccess(task.Type, time.Since(startTime))
        
        log.Info("Task completed",
            zap.String("task_id", task.ID),
            zap.String("type", task.Type),
            zap.Duration("duration", time.Since(startTime)),
        )
    }
}
```

### 4. Model Integration

```go
// internal/models/integration.go
package models

import (
    "context"
    "fmt"
    
    "github.com/yourorg/my-custom-agent/internal/models/providers"
)

// ModelProvider interface for AI model providers
type ModelProvider interface {
    Name() string
    Generate(ctx context.Context, prompt string, opts GenerateOptions) (*Response, error)
    Embed(ctx context.Context, text string) ([]float32, error)
    HealthCheck(ctx context.Context) error
}

// ModelManager manages multiple AI model providers
type ModelManager struct {
    providers map[string]ModelProvider
    primary   string
    fallbacks []string
}

// NewModelManager creates a new model manager
func NewModelManager(config ModelConfig) (*ModelManager, error) {
    mm := &ModelManager{
        providers: make(map[string]ModelProvider),
        primary:   config.Primary,
        fallbacks: config.Fallbacks,
    }
    
    // Initialize providers
    for name, cfg := range config.Providers {
        provider, err := createProvider(name, cfg)
        if err != nil {
            return nil, fmt.Errorf("failed to create provider %s: %w", name, err)
        }
        mm.providers[name] = provider
    }
    
    return mm, nil
}

// Create provider based on type
func createProvider(name string, config ProviderConfig) (ModelProvider, error) {
    switch config.Type {
    case "openai":
        return providers.NewOpenAIProvider(config)
    case "anthropic":
        return providers.NewAnthropicProvider(config)
    case "bedrock":
        return providers.NewBedrockProvider(config)
    case "custom":
        return providers.NewCustomProvider(config)
    default:
        return nil, fmt.Errorf("unknown provider type: %s", config.Type)
    }
}

// Generate text using primary or fallback providers
func (m *ModelManager) Generate(ctx context.Context, prompt string, opts GenerateOptions) (*Response, error) {
    // Try primary provider
    if provider, exists := m.providers[m.primary]; exists {
        resp, err := provider.Generate(ctx, prompt, opts)
        if err == nil {
            return resp, nil
        }
        log.Warn("Primary provider failed", 
            zap.String("provider", m.primary),
            zap.Error(err),
        )
    }
    
    // Try fallbacks
    for _, fallback := range m.fallbacks {
        if provider, exists := m.providers[fallback]; exists {
            resp, err := provider.Generate(ctx, prompt, opts)
            if err == nil {
                return resp, nil
            }
            log.Warn("Fallback provider failed",
                zap.String("provider", fallback),
                zap.Error(err),
            )
        }
    }
    
    return nil, fmt.Errorf("all providers failed")
}

// Example provider implementation
// internal/models/providers/bedrock.go
package providers

import (
    "context"
    "encoding/json"
    
    "github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
)

type BedrockProvider struct {
    client  *bedrockruntime.Client
    modelID string
}

func NewBedrockProvider(config ProviderConfig) (*BedrockProvider, error) {
    cfg, err := awsconfig.LoadDefaultConfig(context.Background())
    if err != nil {
        return nil, fmt.Errorf("failed to load AWS config: %w", err)
    }
    
    return &BedrockProvider{
        client:  bedrockruntime.NewFromConfig(cfg),
        modelID: config.ModelID,
    }, nil
}

func (b *BedrockProvider) Generate(ctx context.Context, prompt string, opts GenerateOptions) (*Response, error) {
    // Build request based on model
    requestBody := b.buildRequest(prompt, opts)
    
    input := &bedrockruntime.InvokeModelInput{
        ModelId:     &b.modelID,
        Body:        requestBody,
        ContentType: aws.String("application/json"),
    }
    
    output, err := b.client.InvokeModel(ctx, input)
    if err != nil {
        return nil, fmt.Errorf("bedrock invoke failed: %w", err)
    }
    
    // Parse response based on model
    return b.parseResponse(output.Body)
}
```

### 5. Communication Layer

```go
// pkg/client/websocket.go <!-- Source: pkg/models/websocket/binary.go -->
package client

import (
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "sync"
    "time"
    
    "github.com/gorilla/websocket" <!-- Source: pkg/models/websocket/binary.go -->
)

// WebSocketClient handles WebSocket communication <!-- Source: pkg/models/websocket/binary.go -->
type WebSocketClient struct { <!-- Source: pkg/models/websocket/binary.go -->
    url      string
    conn     *websocket.Conn <!-- Source: pkg/models/websocket/binary.go -->
    apiKey   string
    
    // Channels
    sendChan chan Message
    recvChan chan Message
    errChan  chan error
    
    // State
    connected bool
    mu       sync.RWMutex
    
    // Handlers
    handlers map[MessageType]MessageHandler
}

// Message represents a WebSocket message <!-- Source: pkg/models/websocket/binary.go -->
type Message struct {
    Type      MessageType    `json:"type"`
    ID        string        `json:"id"`
    Timestamp time.Time     `json:"timestamp"`
    Payload   interface{}   `json:"payload"`
}

// NewWebSocketClient creates a new WebSocket client <!-- Source: pkg/models/websocket/binary.go -->
func NewWebSocketClient(url, apiKey string) (*WebSocketClient, error) { <!-- Source: pkg/models/websocket/binary.go -->
    client := &WebSocketClient{ <!-- Source: pkg/models/websocket/binary.go -->
        url:      url,
        apiKey:   apiKey,
        sendChan: make(chan Message, 100),
        recvChan: make(chan Message, 100),
        errChan:  make(chan error, 10),
        handlers: make(map[MessageType]MessageHandler),
    }
    
    // Register default handlers
    client.registerDefaultHandlers()
    
    return client, nil
}

// Connect establishes WebSocket connection <!-- Source: pkg/models/websocket/binary.go -->
func (c *WebSocketClient) Connect() error { <!-- Source: pkg/models/websocket/binary.go -->
    headers := http.Header{
        "Authorization": []string{fmt.Sprintf("Bearer %s", c.apiKey)},
    }
    
    conn, _, err := websocket.DefaultDialer.Dial(c.url, headers) <!-- Source: pkg/models/websocket/binary.go -->
    if err != nil {
        return fmt.Errorf("websocket dial failed: %w", err) <!-- Source: pkg/models/websocket/binary.go -->
    }
    
    c.mu.Lock()
    c.conn = conn
    c.connected = true
    c.mu.Unlock()
    
    // Start read/write pumps
    go c.readPump()
    go c.writePump()
    
    log.Info("WebSocket connected", zap.String("url", c.url)) <!-- Source: pkg/models/websocket/binary.go -->
    
    return nil
}

// Send sends a message
func (c *WebSocketClient) Send(msg Message) error { <!-- Source: pkg/models/websocket/binary.go -->
    c.mu.RLock()
    if !c.connected {
        c.mu.RUnlock()
        return ErrNotConnected
    }
    c.mu.RUnlock()
    
    select {
    case c.sendChan <- msg:
        return nil
    case <-time.After(5 * time.Second):
        return ErrSendTimeout
    }
}

// Read pump
func (c *WebSocketClient) readPump() { <!-- Source: pkg/models/websocket/binary.go -->
    defer func() {
        c.mu.Lock()
        c.connected = false
        c.mu.Unlock()
        c.conn.Close()
    }()
    
    c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
    c.conn.SetPongHandler(func(string) error {
        c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
        return nil
    })
    
    for {
        var msg Message
        err := c.conn.ReadJSON(&msg)
        if err != nil {
            if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) { <!-- Source: pkg/models/websocket/binary.go -->
                log.Error("WebSocket read error", zap.Error(err)) <!-- Source: pkg/models/websocket/binary.go -->
            }
            c.errChan <- err
            return
        }
        
        // Handle message
        if handler, exists := c.handlers[msg.Type]; exists {
            go handler(msg)
        } else {
            c.recvChan <- msg
        }
    }
}

// Write pump
func (c *WebSocketClient) writePump() { <!-- Source: pkg/models/websocket/binary.go -->
    ticker := time.NewTicker(30 * time.Second)
    defer func() {
        ticker.Stop()
        c.conn.Close()
    }()
    
    for {
        select {
        case msg, ok := <-c.sendChan:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if !ok {
                c.conn.WriteMessage(websocket.CloseMessage, []byte{}) <!-- Source: pkg/models/websocket/binary.go -->
                return
            }
            
            if err := c.conn.WriteJSON(msg); err != nil {
                log.Error("WebSocket write error", zap.Error(err)) <!-- Source: pkg/models/websocket/binary.go -->
                return
            }
            
        case <-ticker.C:
            c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
            if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil { <!-- Source: pkg/models/websocket/binary.go -->
                return
            }
        }
    }
}
```

## Advanced Features

### 1. Context Management

```go
// internal/agent/context.go
package agent

import (
    "sync"
    "time"
)

// ContextManager manages agent context and memory
type ContextManager struct {
    shortTermMemory *ShortTermMemory
    longTermMemory  *LongTermMemory
    workingMemory   *WorkingMemory
    mu              sync.RWMutex
}

// ShortTermMemory holds recent interactions
type ShortTermMemory struct {
    capacity int
    items    []MemoryItem
    index    map[string]int
}

// LongTermMemory persists important information
type LongTermMemory struct {
    store    MemoryStore
    indexer  MemoryIndexer
}

// WorkingMemory holds current task context
type WorkingMemory struct {
    taskContext  map[string]interface{}
    activeGoals  []Goal
    constraints  []Constraint
}

// Remember stores information in appropriate memory
func (c *ContextManager) Remember(item MemoryItem) {
    c.mu.Lock()
    defer c.mu.Unlock()
    
    // Determine memory placement
    if item.Importance > 0.8 || item.Permanent {
        c.longTermMemory.Store(item)
    }
    
    // Always store in short-term
    c.shortTermMemory.Add(item)
    
    // Update working memory if relevant
    if item.RelevantToCurrentTask() {
        c.workingMemory.Update(item)
    }
}

// Recall retrieves relevant memories
func (c *ContextManager) Recall(query Query) []MemoryItem {
    c.mu.RLock()
    defer c.mu.RUnlock()
    
    // Search all memory types
    results := []MemoryItem{}
    
    // Working memory (highest priority)
    if items := c.workingMemory.Search(query); len(items) > 0 {
        results = append(results, items...)
    }
    
    // Short-term memory
    if items := c.shortTermMemory.Search(query); len(items) > 0 {
        results = append(results, items...)
    }
    
    // Long-term memory
    if items := c.longTermMemory.Search(query); len(items) > 0 {
        results = append(results, items...)
    }
    
    // Rank by relevance
    return c.rankByRelevance(results, query)
}
```

### 2. Learning System

```go
// internal/agent/learning.go
package agent

import (
    "context"
    "time"
)

// LearningSystem enables agent self-improvement
type LearningSystem struct {
    experienceBuffer *ExperienceBuffer
    policyNetwork    *PolicyNetwork
    valueNetwork     *ValueNetwork
    optimizer        *Optimizer
}

// Experience represents a learning experience
type Experience struct {
    State       State
    Action      Action
    Reward      float64
    NextState   State
    Done        bool
    Metadata    map[string]interface{}
}

// Learn from experience
func (l *LearningSystem) Learn(exp Experience) {
    // Store experience
    l.experienceBuffer.Add(exp)
    
    // Periodic training
    if l.experienceBuffer.Size() >= l.batchSize {
        batch := l.experienceBuffer.Sample(l.batchSize)
        loss := l.train(batch)
        
        log.Info("Learning update",
            zap.Float64("loss", loss),
            zap.Int("experiences", l.experienceBuffer.Size()),
        )
    }
}

// Improve capability based on feedback
func (l *LearningSystem) ImproveCapability(capability string, feedback Feedback) {
    // Analyze feedback
    analysis := l.analyzeFeedback(feedback)
    
    // Generate improvement plan
    plan := l.generateImprovementPlan(capability, analysis)
    
    // Apply improvements
    for _, improvement := range plan.Improvements {
        l.applyImprovement(capability, improvement)
    }
}
```

### 3. Collaboration Interface

```go
// internal/agent/collaboration.go
package agent

import (
    "context"
    "sync"
)

// CollaborationManager handles multi-agent collaboration
type CollaborationManager struct {
    agent       *Agent
    peers       map[string]PeerAgent
    protocols   map[string]CollaborationProtocol
    sessions    map[string]*CollaborationSession
    mu          sync.RWMutex
}

// PeerAgent represents another agent
type PeerAgent struct {
    ID           string
    Capabilities []Capability
    Status       AgentStatus
    Reputation   float64
}

// InitiateCollaboration starts a collaboration session
func (c *CollaborationManager) InitiateCollaboration(
    ctx context.Context,
    task CollaborativeTask,
) (*CollaborationSession, error) {
    // Find suitable peers
    peers := c.findSuitablePeers(task.Requirements)
    if len(peers) == 0 {
        return nil, ErrNoPeersAvailable
    }
    
    // Create session
    session := &CollaborationSession{
        ID:       generateSessionID(),
        Task:     task,
        Peers:    peers,
        Status:   SessionStatusInitializing,
        Created:  time.Now(),
    }
    
    // Initialize protocol
    protocol := c.selectProtocol(task.Type)
    
    // Send invitations
    for _, peer := range peers {
        invitation := CollaborationInvitation{
            SessionID: session.ID,
            Task:      task,
            Role:      c.assignRole(peer, task),
            Protocol:  protocol.Name(),
        }
        
        if err := c.sendInvitation(peer, invitation); err != nil {
            log.Warn("Failed to invite peer",
                zap.String("peer_id", peer.ID),
                zap.Error(err),
            )
        }
    }
    
    c.mu.Lock()
    c.sessions[session.ID] = session
    c.mu.Unlock()
    
    return session, nil
}

// HandleCollaborationRequest processes incoming collaboration requests
func (c *CollaborationManager) HandleCollaborationRequest(
    ctx context.Context,
    invitation CollaborationInvitation,
) error {
    // Evaluate invitation
    if !c.shouldAccept(invitation) {
        return c.declineInvitation(invitation)
    }
    
    // Accept and join session
    return c.joinSession(ctx, invitation)
}
```

## Testing

### 1. Unit Tests

```go
// internal/agent/agent_test.go
package agent

import (
    "context"
    "testing"
    "time"
    
    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)

func TestAgentLifecycle(t *testing.T) {
    // Create test config
    cfg := &config.Config{
        AgentName: "test-agent",
        AgentType: "test",
        ServerURL: "ws://localhost:8080/ws",
        APIKey:    "test-key",
    }
    
    // Create agent
    agent, err := NewAgent(cfg)
    require.NoError(t, err)
    assert.NotEmpty(t, agent.ID)
    
    // Test start
    err = agent.Start()
    assert.NoError(t, err)
    assert.Equal(t, StatusRunning, agent.GetStatus())
    
    // Test stop
    err = agent.Stop()
    assert.NoError(t, err)
    assert.Equal(t, StatusStopped, agent.GetStatus())
}

func TestCapabilityRegistration(t *testing.T) {
    manager := capabilities.NewManager()
    
    // Register capability
    cap := &capabilities.Capability{
        Name:       "test-capability",
        Version:    "1.0.0",
        Type:       capabilities.CapabilityTypeAnalysis,
        Confidence: 0.9,
    }
    
    err := manager.Register(cap)
    assert.NoError(t, err)
    
    // Test duplicate registration
    err = manager.Register(cap)
    assert.Error(t, err)
    
    // Test capability matching
    requirements := []capabilities.Requirement{
        {Type: capabilities.CapabilityTypeAnalysis},
    }
    
    matched, err := manager.Match(requirements)
    assert.NoError(t, err)
    assert.Len(t, matched, 1)
    assert.Equal(t, "test-capability", matched[0].Name)
}
```

### 2. Integration Tests

```go
// test/integration/agent_integration_test.go
package integration

import (
    "context"
    "testing"
    "time"
)

func TestAgentMCPIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Start test MCP server
    server := startTestMCPServer(t)
    defer server.Stop()
    
    // Create and start agent
    agent := createTestAgent(t, server.URL)
    err := agent.Start()
    require.NoError(t, err)
    defer agent.Stop()
    
    // Wait for registration
    assert.Eventually(t, func() bool {
        return server.IsAgentRegistered(agent.ID)
    }, 10*time.Second, 100*time.Millisecond)
    
    // Send test task
    task := &Task{
        ID:   "test-task-1",
        Type: "test",
        Payload: map[string]string{
            "action": "echo",
            "message": "Hello, World!",
        },
    }
    
    server.SendTask(agent.ID, task)
    
    // Wait for completion
    assert.Eventually(t, func() bool {
        result := server.GetTaskResult(task.ID)
        return result != nil && result.Status == "completed"
    }, 30*time.Second, 100*time.Millisecond)
}
```

## Deployment

### 1. Docker Configuration

```dockerfile
# Dockerfile
FROM golang:1.23-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o agent ./cmd/agent

FROM alpine:latest

RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app

COPY --from=builder /build/agent .
COPY --from=builder /build/configs ./configs

ENV TZ=UTC
EXPOSE 8080

USER nobody:nobody
ENTRYPOINT ["./agent"]
```

### 2. Kubernetes Deployment

```yaml
# deployments/k8s/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: custom-ai-agent
  namespace: mcp-agents
spec:
  replicas: 3
  selector:
    matchLabels:
      app: custom-ai-agent
  template:
    metadata:
      labels:
        app: custom-ai-agent
      annotations:
        prometheus.io/scrape: "true"
        prometheus.io/port: "8080"
        prometheus.io/path: "/metrics"
    spec:
      serviceAccountName: ai-agent
      containers:
      - name: agent
        image: your-registry/custom-ai-agent:latest
        ports:
        - containerPort: 8080
          name: metrics
        env:
        - name: AGENT_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: MCP_SERVER_URL
          value: "wss://edge-mcp.mcp.svc.cluster.local:8080/ws"
        - name: MCP_API_KEY
          valueFrom:
            secretKeyRef:
              name: agent-credentials
              key: api-key
        resources:
          requests:
            memory: "512Mi"
            cpu: "250m"
          limits:
            memory: "1Gi"
            cpu: "1"
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

## Best Practices

### 1. Error Handling
- Use structured errors with context
- Implement retry mechanisms with backoff
- Log errors with appropriate levels
- Graceful degradation on failures

### 2. Performance
- Profile code regularly
- Optimize hot paths
- Use appropriate data structures
- Implement caching where beneficial

### 3. Security
- Validate all inputs
- Use secure communication (TLS)
- Implement rate limiting
- Regular security audits

### 4. Observability
- Comprehensive logging
- Detailed metrics
- Distributed tracing
- Health checks

## Next Steps

1. Review [Agent WebSocket Protocol](./agent-websocket-protocol.md) for protocol details <!-- Source: pkg/models/websocket/binary.go -->
2. Explore [Agent Integration Examples](./agent-integration-examples.md) for patterns
3. See [Agent SDK Guide](./agent-sdk-guide.md) for SDK usage
4. Check [Agent Integration Troubleshooting](./agent-integration-troubleshooting.md)

## Resources

- [Go Best Practices](https://go.dev/doc/effective_go)
- [WebSocket RFC](https://tools.ietf.org/html/rfc6455) <!-- Source: pkg/models/websocket/binary.go -->
- [Kubernetes Operators](https://kubernetes.io/docs/concepts/extend-kubernetes/operator/)
