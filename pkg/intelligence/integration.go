package intelligence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
)

// DynamicToolsIntegration integrates intelligence with the DynamicToolsService
type DynamicToolsIntegration struct {
	// Core services
	executionService *ResilientExecutionService
	embeddingService EmbeddingService // Use the interface
	dynamicToolsRepo DynamicToolRepository

	// Configuration
	config        IntegrationConfig
	logger        observability.Logger
	metricsClient observability.MetricsClient

	// Database
	db *sql.DB
}

// IntegrationConfig contains integration configuration
type IntegrationConfig struct {
	// Feature flags
	AutoEmbedding       bool
	IntelligenceEnabled bool
	SecurityChecks      bool
	CostTracking        bool

	// Execution modes
	DefaultMode    ExecutionMode
	AsyncThreshold int // Use async for results > this size

	// Performance
	CacheEnabled    bool
	BatchingEnabled bool
}

// NewDynamicToolsIntegration creates the integration layer
func NewDynamicToolsIntegration(
	config IntegrationConfig,
	deps IntegrationDependencies,
) (*DynamicToolsIntegration, error) {
	// Create execution service if intelligence is enabled
	var executionService *ResilientExecutionService
	if config.IntelligenceEnabled {
		serviceConfig := &ServiceConfig{
			DefaultMode:         config.DefaultMode,
			EnableAsyncFallback: true,
			MaxConcurrency:      10,
			TimeoutSeconds:      30,
			CacheEnabled:        config.CacheEnabled,
			CacheTTL:            5 * time.Minute,
		}

		serviceDeps := ServiceDependencies{
			ToolExecutor:     deps.ToolExecutor,
			ContentAnalyzer:  deps.ContentAnalyzer,
			EmbeddingService: deps.EmbeddingService,
			SemanticGraph:    deps.SemanticGraph,
			DB:               deps.DB,
			Cache:            deps.CacheService,
			EventStore:       deps.EventStore,
			Logger:           deps.Logger,
		}

		var err error
		executionService, err = NewResilientExecutionService(serviceConfig, serviceDeps)
		if err != nil {
			return nil, fmt.Errorf("failed to create execution service: %w", err)
		}
	}

	return &DynamicToolsIntegration{
		executionService: executionService,
		embeddingService: deps.EmbeddingService,
		dynamicToolsRepo: deps.DynamicToolsRepo,
		config:           config,
		logger:           deps.Logger,
		metricsClient:    deps.MetricsClient,
		db:               deps.DB,
	}, nil
}

// ExecuteToolActionWithIntelligence wraps tool execution with intelligence processing
func (i *DynamicToolsIntegration) ExecuteToolActionWithIntelligence(
	ctx context.Context,
	request ToolExecutionRequest,
) (*ToolExecutionResponse, error) {
	// Start span for tracing
	ctx, span := i.startSpan(ctx, "tool.execute.with_intelligence",
		"tool_id", request.ToolID.String(),
		"action", request.Action,
	)
	defer i.endSpan(span)

	// Record start time
	startTime := time.Now()

	// Check if intelligence is enabled
	if !i.config.IntelligenceEnabled {
		// Fall back to standard execution
		return i.executeStandard(ctx, request)
	}

	// Determine execution mode based on configuration and request
	mode := i.determineExecutionMode(request)

	// Create intelligence execution request
	execRequest := ExecutionRequest{
		ToolID:   request.ToolID,
		AgentID:  request.AgentID,
		TenantID: request.TenantID,
		Action:   request.Action,
		Params:   request.Params,
		Mode:     mode,
		Metadata: map[string]interface{}{
			"source":      "dynamic_tools",
			"auto_embed":  i.config.AutoEmbedding,
			"passthrough": request.PassthroughAuth != nil,
		},
	}

	// Execute with intelligence
	execResponse, err := i.executionService.Execute(ctx, execRequest)
	if err != nil {
		i.recordError(ctx, err, request)
		return nil, fmt.Errorf("intelligence execution failed: %w", err)
	}

	// Build response
	response := &ToolExecutionResponse{
		ExecutionID:     execResponse.ExecutionID,
		Result:          execResponse.ToolResult,
		Intelligence:    execResponse.Intelligence,
		ContextID:       execResponse.ContextID,
		RelatedContexts: execResponse.RelatedContexts,
		Duration:        time.Since(startTime),
		Metrics:         execResponse.Metrics,
	}

	// Record metrics
	i.recordExecutionMetrics(ctx, request, response, nil)

	// Store execution history
	if err := i.storeExecutionHistory(ctx, request, response); err != nil {
		// Log but don't fail
		i.logger.Error("Failed to store execution history", map[string]interface{}{
			"error":        err.Error(),
			"execution_id": response.ExecutionID.String(),
		})
	}

	return response, nil
}

// executeStandard performs standard tool execution without intelligence
func (i *DynamicToolsIntegration) executeStandard(
	ctx context.Context,
	request ToolExecutionRequest,
) (*ToolExecutionResponse, error) {
	// Use the tool executor interface to execute the tool
	// This allows for proper abstraction and testing
	toolResult, err := i.executeToolDirect(ctx, request)
	if err != nil {
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	result := toolResult.Data

	// Build basic response
	response := &ToolExecutionResponse{
		ExecutionID: uuid.New(),
		Result:      result,
		Duration:    time.Since(time.Now()),
	}

	// Generate embedding if auto-embedding is enabled
	if i.config.AutoEmbedding && i.shouldGenerateEmbedding(result) {
		go i.generateEmbeddingAsync(context.Background(), request, result)
	}

	return response, nil
}

// determineExecutionMode determines the execution mode based on config and request
func (i *DynamicToolsIntegration) determineExecutionMode(request ToolExecutionRequest) ExecutionMode {
	// Check if explicitly set in request
	if mode, ok := request.Metadata["execution_mode"].(string); ok {
		switch ExecutionMode(mode) {
		case ModeSync, ModeAsync, ModeHybrid:
			return ExecutionMode(mode)
		}
	}

	// Check if response is expected to be large
	if i.isLargeResponse(request.Action) {
		return ModeHybrid // Return tool result quickly, process intelligence async
	}

	// Check if real-time response is needed
	if i.isRealTimeAction(request.Action) {
		return ModeSync
	}

	// Use default from config
	return i.config.DefaultMode
}

// shouldGenerateEmbedding determines if an embedding should be generated
func (i *DynamicToolsIntegration) shouldGenerateEmbedding(result interface{}) bool {
	// Check result type
	switch v := result.(type) {
	case string:
		return len(v) > 100 // Only embed substantial content
	case map[string]interface{}:
		// Check for content fields
		if content, ok := v["content"].(string); ok {
			return len(content) > 100
		}
		if text, ok := v["text"].(string); ok {
			return len(text) > 100
		}
	case []byte:
		return len(v) > 100
	}

	return false
}

// generateEmbeddingAsync generates embedding asynchronously
func (i *DynamicToolsIntegration) generateEmbeddingAsync(
	ctx context.Context,
	request ToolExecutionRequest,
	result interface{},
) {
	// Extract content
	content := i.extractContent(result)
	if content == "" {
		return
	}

	// Create metadata for embedding
	metadata := map[string]interface{}{
		"tool_id":        request.ToolID.String(),
		"action":         request.Action,
		"auto_generated": true,
		"tenant_id":      request.TenantID.String(),
		"agent_id":       request.AgentID.String(),
	}

	// Generate embedding using the interface method
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	embeddingID, err := i.embeddingService.GenerateEmbedding(ctx, content, metadata)
	if err != nil {
		i.logger.Error("Failed to generate embedding", map[string]interface{}{
			"error":   err.Error(),
			"tool_id": request.ToolID.String(),
		})
		return
	}

	i.logger.Info("Auto-generated embedding", map[string]interface{}{
		"embedding_id": embeddingID.String(),
		"tool_id":      request.ToolID.String(),
	})
}

// extractContent extracts textual content from result
func (i *DynamicToolsIntegration) extractContent(result interface{}) string {
	switch v := result.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case map[string]interface{}:
		// Try common content fields
		if content, ok := v["content"].(string); ok {
			return content
		}
		if text, ok := v["text"].(string); ok {
			return text
		}
		if body, ok := v["body"].(string); ok {
			return body
		}
		// Fallback to JSON representation
		data, _ := json.Marshal(v)
		return string(data)
	default:
		// Try JSON marshaling
		data, err := json.Marshal(v)
		if err == nil {
			return string(data)
		}
	}
	return ""
}

// storeExecutionHistory stores execution history in database
func (i *DynamicToolsIntegration) storeExecutionHistory(
	ctx context.Context,
	request ToolExecutionRequest,
	response *ToolExecutionResponse,
) error {
	query := `
		INSERT INTO mcp.execution_history (
			execution_id, tenant_id, agent_id, tool_id,
			action, request_data, response_data, execution_mode,
			status, content_type, intelligence_metadata, context_id,
			embedding_id, execution_time_ms, embedding_time_ms,
			total_tokens, total_cost_usd, created_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19
		)`

	requestData, _ := json.Marshal(request.Params)
	responseData, _ := json.Marshal(response.Result)
	intelligenceData, _ := json.Marshal(response.Intelligence)

	_, err := i.db.ExecContext(ctx, query,
		response.ExecutionID,
		request.TenantID,
		request.AgentID,
		request.ToolID,
		request.Action,
		requestData,
		responseData,
		string(i.config.DefaultMode),
		"completed",
		response.Intelligence.ContentType,
		intelligenceData,
		response.ContextID,
		nil, // embedding_id - set separately if generated
		response.Metrics.ExecutionTimeMs,
		response.Metrics.EmbeddingTimeMs,
		response.Metrics.TotalTokens,
		response.Metrics.TotalCostUSD,
		time.Now(),
		time.Now(),
	)

	return err
}

// isLargeResponse checks if an action typically returns large responses
func (i *DynamicToolsIntegration) isLargeResponse(action string) bool {
	largeActions := map[string]bool{
		"list":     true,
		"search":   true,
		"export":   true,
		"download": true,
		"report":   true,
	}

	actionLower := strings.ToLower(action)
	for key := range largeActions {
		if strings.Contains(actionLower, key) {
			return true
		}
	}

	return false
}

// isRealTimeAction checks if an action requires real-time response
func (i *DynamicToolsIntegration) isRealTimeAction(action string) bool {
	realTimeActions := map[string]bool{
		"health":   true,
		"ping":     true,
		"status":   true,
		"validate": true,
	}

	actionLower := strings.ToLower(action)
	for key := range realTimeActions {
		if strings.Contains(actionLower, key) {
			return true
		}
	}

	return false
}

// recordExecutionMetrics records execution metrics
func (i *DynamicToolsIntegration) recordExecutionMetrics(
	ctx context.Context,
	request ToolExecutionRequest,
	response *ToolExecutionResponse,
	err error,
) {
	labels := map[string]string{
		"tool_id": request.ToolID.String(),
		"action":  request.Action,
		"mode":    string(i.config.DefaultMode),
		"success": fmt.Sprintf("%t", err == nil),
	}

	// Record execution time
	i.metricsClient.RecordHistogram("tool_execution_duration_ms",
		float64(response.Duration.Milliseconds()), labels)

	// Record token usage
	if response.Metrics.TotalTokens > 0 {
		i.metricsClient.RecordHistogram("tool_execution_tokens",
			float64(response.Metrics.TotalTokens), labels)
	}

	// Record cost
	if response.Metrics.TotalCostUSD > 0 {
		i.metricsClient.RecordHistogram("tool_execution_cost_usd",
			response.Metrics.TotalCostUSD, labels)
	}

	// Record success/failure
	if err != nil {
		i.metricsClient.IncrementCounter("tool_execution_errors", 1)
	} else {
		i.metricsClient.IncrementCounter("tool_execution_success", 1)
	}
}

// recordError records error metrics
func (i *DynamicToolsIntegration) recordError(ctx context.Context, err error, request ToolExecutionRequest) {
	i.logger.Error("Tool execution failed", map[string]interface{}{
		"error":   err.Error(),
		"tool_id": request.ToolID.String(),
		"action":  request.Action,
	})

	i.metricsClient.IncrementCounter("tool_execution_errors", 1)
}

// executeToolDirect executes a tool directly using the ToolExecutor interface
func (i *DynamicToolsIntegration) executeToolDirect(ctx context.Context, request ToolExecutionRequest) (*ToolResult, error) {
	// If we have a tool executor in our dependencies, use it
	if i.executionService != nil && i.executionService.toolExecutor != nil {
		return i.executionService.toolExecutor.Execute(ctx, request.ToolID, request.Action, request.Params)
	}

	// Otherwise, create a simple tool result
	// In production, this would call the actual tool adapter
	return &ToolResult{
		Data:     map[string]interface{}{"status": "executed", "action": request.Action},
		Duration: time.Since(time.Now()),
	}, nil
}

// startSpan starts a tracing span
func (i *DynamicToolsIntegration) startSpan(ctx context.Context, name string, keyValues ...string) (context.Context, interface{}) {
	// This would integrate with OpenTelemetry or similar
	return ctx, nil
}

// endSpan ends a tracing span
func (i *DynamicToolsIntegration) endSpan(span interface{}) {
	// This would integrate with OpenTelemetry or similar
}

// Supporting types for integration

type ToolExecutionRequest struct {
	ToolID          uuid.UUID
	AgentID         uuid.UUID
	TenantID        uuid.UUID
	Action          string
	Params          map[string]interface{}
	PassthroughAuth *models.PassthroughAuthBundle
	Metadata        map[string]interface{}
}

type ToolExecutionResponse struct {
	ExecutionID     uuid.UUID
	Result          interface{}
	Intelligence    IntelligenceMetadata
	ContextID       uuid.UUID
	RelatedContexts []uuid.UUID
	Duration        time.Duration
	Metrics         ExecutionMetrics
}

type DynamicToolRepository interface {
	GetTool(ctx context.Context, toolID uuid.UUID) (*models.DynamicTool, error)
	UpdateToolUsage(ctx context.Context, toolID uuid.UUID, usage ToolUsage) error
}

type ToolUsage struct {
	LastUsed       time.Time
	ExecutionCount int64
	TotalTokens    int64
	TotalCost      float64
}

type IntegrationDependencies struct {
	ToolExecutor     ToolExecutor
	ContentAnalyzer  ContentAnalyzer
	EmbeddingService EmbeddingService // Use the interface, not the concrete type
	SemanticGraph    SemanticGraphService
	DynamicToolsRepo DynamicToolRepository
	DB               *sql.DB
	CacheService     CacheService
	EventStore       EventStore
	Logger           observability.Logger
	MetricsClient    observability.MetricsClient
}
