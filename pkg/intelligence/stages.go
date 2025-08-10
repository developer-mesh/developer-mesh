package intelligence

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// executeSecurityStage performs security validation on the request
func (s *ResilientExecutionService) executeSecurityStage(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) error {
	stage := "security"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: req,
	}

	// Serialize request data for security check
	requestData, err := json.Marshal(req.Params)
	if err != nil {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:    stage,
			Status:  StageStatusFailed,
			Error:   err,
			EndTime: &[]time.Time{time.Now()}[0],
		}
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	// Validate content with security layer
	validation, err := s.securityLayer.ValidateContent(ctx, requestData)
	if err != nil {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:    stage,
			Status:  StageStatusFailed,
			Error:   err,
			EndTime: &[]time.Time{time.Now()}[0],
		}
		return fmt.Errorf("security validation failed: %w", err)
	}

	if !validation.Passed {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:       stage,
			Status:     StageStatusFailed,
			Error:      fmt.Errorf("%s", validation.BlockReason),
			OutputData: validation,
			EndTime:    &[]time.Time{time.Now()}[0],
		}
		return fmt.Errorf("security check blocked: %s", validation.BlockReason)
	}

	// Register compensation if needed
	if val, ok := s.compensations.Load(execID); ok {
		compensations := val.([]CompensationFunc)
		compensations = append(compensations, func(ctx context.Context) error {
			// Log security validation reversal
			s.logger.Info("Reversing security validation", map[string]interface{}{"execution_id": execID.String()})
			return nil
		})
		s.compensations.Store(execID, compensations)
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: validation,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return nil
}

// executeCostCheckStage checks if the operation fits within budget
func (s *ResilientExecutionService) executeCostCheckStage(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) error {
	stage := "cost_check"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: req,
	}

	// Estimate costs
	costReq := CostCheckRequest{
		TenantID:        req.TenantID,
		ToolExecution:   true,
		ToolType:        "api_call",
		EmbeddingTokens: 1000, // Estimate
		EmbeddingModel:  "text-embedding-3-small",
	}

	response, err := s.costController.CheckBudget(ctx, costReq)
	if err != nil {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:    stage,
			Status:  StageStatusFailed,
			Error:   err,
			EndTime: &[]time.Time{time.Now()}[0],
		}
		return fmt.Errorf("cost check failed: %w", err)
	}

	if !response.Allowed {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:       stage,
			Status:     StageStatusFailed,
			Error:      fmt.Errorf("%s", response.BlockReason),
			OutputData: response,
			EndTime:    &[]time.Time{time.Now()}[0],
		}
		return fmt.Errorf("operation blocked by cost control: %s", response.BlockReason)
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: response,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return nil
}

// executeToolStage executes the actual tool
func (s *ResilientExecutionService) executeToolStage(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	checkpoint *ExecutionCheckpoint,
) (*ToolResult, error) {
	stage := "tool_execution"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: req,
	}

	startTime := time.Now()

	// Execute tool
	result, err := s.toolExecutor.Execute(ctx, req.ToolID, req.Action, req.Params)
	if err != nil {
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:    stage,
			Status:  StageStatusFailed,
			Error:   err,
			EndTime: &[]time.Time{time.Now()}[0],
		}
		return nil, fmt.Errorf("tool execution failed: %w", err)
	}

	// Calculate duration
	result.Duration = time.Since(startTime)

	// Register compensation
	if val, ok := s.compensations.Load(execID); ok {
		compensations := val.([]CompensationFunc)
		compensations = append(compensations, func(ctx context.Context) error {
			// Potentially reverse tool action
			s.logger.Info("Tool execution compensation", map[string]interface{}{
				"execution_id": execID.String(),
				"tool_id":      req.ToolID.String(),
				"action":       req.Action})
			return nil
		})
		s.compensations.Store(execID, compensations)
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: result,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return result, nil
}

// executeAnalysisStage analyzes the tool result
func (s *ResilientExecutionService) executeAnalysisStage(
	ctx context.Context,
	execID uuid.UUID,
	result *ToolResult,
	checkpoint *ExecutionCheckpoint,
) (*ContentAnalysis, error) {
	stage := "content_analysis"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: result,
	}

	// Convert result to bytes for analysis
	var content []byte
	switch v := result.Data.(type) {
	case string:
		content = []byte(v)
	case []byte:
		content = v
	default:
		data, err := json.Marshal(v)
		if err != nil {
			checkpoint.Stages[stage] = StageCheckpoint{
				Name:    stage,
				Status:  StageStatusFailed,
				Error:   err,
				EndTime: &[]time.Time{time.Now()}[0],
			}
			return nil, fmt.Errorf("failed to marshal result: %w", err)
		}
		content = data
	}

	// Analyze content
	analysis, err := s.contentAnalyzer.Analyze(ctx, content)
	if err != nil {
		// Don't fail on analysis errors
		s.logger.Error("Content analysis failed", map[string]interface{}{"error": err.Error()})
		analysis = &ContentAnalysis{
			ContentType: ContentTypeUnknown,
			Metadata:    &ContentMetadata{Size: len(content)},
		}
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: analysis,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return analysis, nil
}

// executeIntelligenceStage processes intelligence extraction
func (s *ResilientExecutionService) executeIntelligenceStage(
	ctx context.Context,
	execID uuid.UUID,
	result *ToolResult,
	analysis *ContentAnalysis,
	checkpoint *ExecutionCheckpoint,
) (*IntelligenceResult, error) {
	stage := "intelligence_processing"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: map[string]interface{}{
			"result":   result,
			"analysis": analysis,
		},
	}

	startTime := time.Now()

	// Build intelligence metadata
	intelligence := &IntelligenceResult{
		Metadata: IntelligenceMetadata{
			ContentType:    analysis.ContentType,
			Entities:       analysis.Entities,
			Topics:         analysis.Topics,
			Keywords:       analysis.Keywords,
			Summary:        analysis.Summary,
			Language:       analysis.Language,
			Classification: ClassificationPublic,
		},
		EmbeddingDuration: time.Duration(0),
		TokensUsed:        0,
		Cost:              0,
	}

	// Generate embedding if appropriate
	if s.shouldGenerateEmbedding(result.Data, analysis) {
		content := s.extractContent(result.Data)
		metadata := map[string]interface{}{
			"execution_id": execID,
			"content_type": analysis.ContentType,
		}

		embeddingID, err := s.embeddingService.GenerateEmbedding(ctx, content, metadata)
		if err != nil {
			s.logger.Error("Embedding generation failed", map[string]interface{}{"error": err.Error()})
		} else {
			intelligence.EmbeddingID = embeddingID
			intelligence.EmbeddingDuration = time.Since(startTime)

			// Estimate tokens (rough approximation)
			intelligence.TokensUsed = len(content) / 4
			intelligence.Cost = float64(intelligence.TokensUsed) * 0.00002 / 1000
		}
	}

	// Find related contexts
	if intelligence.EmbeddingID != nil {
		related, err := s.semanticGraph.FindRelated(ctx, *intelligence.EmbeddingID, 5)
		if err == nil {
			intelligence.RelatedContexts = related
		}
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: intelligence,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return intelligence, nil
}

// executeSemanticStage updates the semantic graph
func (s *ResilientExecutionService) executeSemanticStage(
	ctx context.Context,
	execID uuid.UUID,
	intelligence *IntelligenceResult,
	checkpoint *ExecutionCheckpoint,
) (uuid.UUID, error) {
	stage := "semantic_graph"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
		InputData: intelligence,
	}

	// Create context ID
	contextID := uuid.New()

	// Add node to semantic graph
	metadata := map[string]interface{}{
		"execution_id":   execID,
		"content_type":   intelligence.Metadata.ContentType,
		"classification": intelligence.Metadata.Classification,
		"language":       intelligence.Metadata.Language,
	}

	if err := s.semanticGraph.AddNode(ctx, contextID, metadata); err != nil {
		s.logger.Error("Failed to add semantic node", map[string]interface{}{"error": err.Error()})
		// Don't fail the operation
	}

	// Create relationships with related contexts
	for _, relatedID := range intelligence.RelatedContexts {
		if err := s.semanticGraph.CreateRelationship(ctx, contextID, relatedID, "similar"); err != nil {
			s.logger.Error("Failed to create relationship", map[string]interface{}{"error": err.Error()})
		}
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: contextID,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return contextID, nil
}

// executePersistenceStage persists the execution results
func (s *ResilientExecutionService) executePersistenceStage(
	ctx context.Context,
	execID uuid.UUID,
	req ExecutionRequest,
	result *ToolResult,
	intelligence *IntelligenceResult,
	contextID uuid.UUID,
	checkpoint *ExecutionCheckpoint,
) error {
	stage := "persistence"
	checkpoint.Stages[stage] = StageCheckpoint{
		Name:      stage,
		Status:    StageStatusRunning,
		StartTime: time.Now(),
	}

	// Serialize data
	requestData, _ := json.Marshal(req.Params)
	responseData, _ := json.Marshal(result.Data)
	intelligenceData, _ := json.Marshal(intelligence.Metadata)

	// Store in database
	query := `
		INSERT INTO mcp.execution_history (
			execution_id, tenant_id, agent_id, tool_id,
			action, request_data, response_data, execution_mode,
			status, content_type, intelligence_metadata, context_id,
			embedding_id, execution_time_ms, embedding_time_ms,
			total_tokens, total_cost_usd, created_at, started_at, completed_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12,
			$13, $14, $15, $16, $17, $18, $19, $20
		)`

	_, err := s.db.ExecContext(ctx, query,
		execID,
		req.TenantID,
		req.AgentID,
		req.ToolID,
		req.Action,
		requestData,
		responseData,
		string(req.Mode),
		"completed",
		string(intelligence.Metadata.ContentType),
		intelligenceData,
		contextID,
		intelligence.EmbeddingID,
		result.Duration.Milliseconds(),
		intelligence.EmbeddingDuration.Milliseconds(),
		intelligence.TokensUsed,
		intelligence.Cost,
		checkpoint.StartTime,
		checkpoint.StartTime,
		time.Now(),
	)

	if err != nil {
		s.logger.Error("Failed to persist execution", map[string]interface{}{"error": err.Error()})
		checkpoint.Stages[stage] = StageCheckpoint{
			Name:    stage,
			Status:  StageStatusFailed,
			Error:   err,
			EndTime: &[]time.Time{time.Now()}[0],
		}
		// Don't fail the operation
		return nil
	}

	// Record cost
	if intelligence.Cost > 0 {
		costRecord := CostRecord{
			ExecutionID:     execID,
			TenantID:        req.TenantID,
			ToolExecution:   true,
			ToolType:        "api_call",
			EmbeddingTokens: intelligence.TokensUsed,
			EmbeddingModel:  "text-embedding-3-small",
		}

		if err := s.costController.RecordCost(ctx, costRecord); err != nil {
			s.logger.Error("Failed to record cost", map[string]interface{}{"error": err.Error()})
		}
	}

	checkpoint.Stages[stage] = StageCheckpoint{
		Name:       stage,
		Status:     StageStatusCompleted,
		OutputData: execID,
		EndTime:    &[]time.Time{time.Now()}[0],
	}

	return nil
}

// Helper methods

func (s *ResilientExecutionService) shouldGenerateEmbedding(data interface{}, analysis *ContentAnalysis) bool {
	// Check if content is substantial enough
	if analysis.Metadata == nil {
		return false
	}

	// Don't embed small content
	if analysis.Metadata.Size < 100 {
		return false
	}

	// Don't embed if it has PII or secrets
	if analysis.Metadata.HasPII || analysis.Metadata.HasSecrets {
		return false
	}

	// Embed certain content types
	switch analysis.ContentType {
	case ContentTypeText, ContentTypeCode, ContentTypeDocumentation, ContentTypeAPI:
		return true
	default:
		return false
	}
}

func (s *ResilientExecutionService) extractContent(data interface{}) string {
	switch v := data.(type) {
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
		// Fallback to JSON
		jsonData, _ := json.Marshal(v)
		return string(jsonData)
	default:
		// Try JSON marshaling
		jsonData, err := json.Marshal(v)
		if err == nil {
			return string(jsonData)
		}
		return fmt.Sprintf("%v", v)
	}
}
