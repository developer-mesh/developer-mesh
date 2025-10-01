package mcp

import (
	"errors"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

func TestErrorTemplates_ToolNotFound(t *testing.T) {
	et := NewErrorTemplates()
	categories := []string{"repository", "issues", "ci/cd"}

	err := et.ToolNotFound("nonexistent_tool", categories)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeNotFound {
		t.Errorf("Expected error code NOT_FOUND, got %s", err.Code)
	}

	if err.Suggestion == "" {
		t.Error("Expected suggestion to be present")
	}

	if len(err.RecoverySteps) == 0 {
		t.Error("Expected recovery steps to be present")
	}

	if err.Operation == "" {
		t.Error("Expected operation to be set")
	}

	// Verify AI-friendly features
	hasListTools := false
	for _, step := range err.RecoverySteps {
		if step.Tool == "tools/list" {
			hasListTools = true
			break
		}
	}
	if !hasListTools {
		t.Error("Expected tools/list in recovery steps")
	}
}

func TestErrorTemplates_ToolExecutionFailed(t *testing.T) {
	et := NewErrorTemplates()
	underlyingErr := errors.New("connection timeout")
	alternatives := []string{"alternative_tool_1", "alternative_tool_2"}

	err := et.ToolExecutionFailed("test_tool", underlyingErr, alternatives)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeIntegrationError {
		t.Errorf("Expected error code INTEGRATION_ERROR, got %s", err.Code)
	}

	if len(err.AlternativeTools) != 2 {
		t.Errorf("Expected 2 alternative tools, got %d", len(err.AlternativeTools))
	}

	if err.RetryStrategy == nil {
		t.Error("Expected retry strategy to be present")
	}

	if !err.RetryStrategy.Retryable {
		t.Error("Expected error to be retryable")
	}

	// Verify alternative tools are suggested in recovery steps
	hasAlternative := false
	for _, step := range err.RecoverySteps {
		if step.Action == "use_alternative" {
			hasAlternative = true
			break
		}
	}
	if !hasAlternative {
		t.Error("Expected alternative tool step in recovery steps")
	}
}

func TestErrorTemplates_RateLimitExceeded(t *testing.T) {
	et := NewErrorTemplates()
	resetTime := time.Now().Add(5 * time.Minute)

	err := et.RateLimitExceeded(100, 0, resetTime, "api_calls")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeRateLimit {
		t.Errorf("Expected error code RATE_LIMIT, got %s", err.Code)
	}

	if err.RateLimitInfo == nil {
		t.Fatal("Expected rate limit info to be present")
	}

	if err.RateLimitInfo.Limit != 100 {
		t.Errorf("Expected limit 100, got %d", err.RateLimitInfo.Limit)
	}

	if err.RetryAfter == nil {
		t.Error("Expected retry_after to be present")
	}

	if err.RetryStrategy == nil || !err.RetryStrategy.Retryable {
		t.Error("Expected retryable strategy")
	}

	// Verify AI-friendly guidance
	if len(err.RecoverySteps) == 0 {
		t.Error("Expected recovery steps with wait/backoff guidance")
	}
}

func TestErrorTemplates_AuthenticationFailed(t *testing.T) {
	et := NewErrorTemplates()

	err := et.AuthenticationFailed("Invalid API key format")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeAuthFailed {
		t.Errorf("Expected error code AUTH_FAILED, got %s", err.Code)
	}

	if err.Suggestion == "" {
		t.Error("Expected suggestion for credential verification")
	}

	if len(err.RecoverySteps) < 2 {
		t.Error("Expected multiple recovery steps for auth errors")
	}

	// Verify practical guidance
	hasCheckCredentials := false
	for _, step := range err.RecoverySteps {
		if step.Action == "check_api_key" || step.Action == "check_headers" {
			hasCheckCredentials = true
			break
		}
	}
	if !hasCheckCredentials {
		t.Error("Expected credential check in recovery steps")
	}
}

func TestErrorTemplates_OperationTimeout(t *testing.T) {
	et := NewErrorTemplates()

	err := et.OperationTimeout("fetch_data", 30*time.Second)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeTimeout {
		t.Errorf("Expected error code TIMEOUT, got %s", err.Code)
	}

	if err.RetryStrategy == nil {
		t.Error("Expected retry strategy")
	}

	if err.RetryStrategy.MaxAttempts == 0 {
		t.Error("Expected max retry attempts to be specified")
	}

	// Verify timeout metadata
	if err.Metadata == nil {
		t.Fatal("Expected metadata to be present")
	}

	if _, ok := err.Metadata["timeout_seconds"]; !ok {
		t.Error("Expected timeout_seconds in metadata")
	}
}

func TestErrorTemplates_ServiceUnavailable(t *testing.T) {
	et := NewErrorTemplates()
	alternatives := []string{"backup_service_tool"}

	err := et.ServiceUnavailable("github", alternatives)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeServiceOffline {
		t.Errorf("Expected error code SERVICE_OFFLINE, got %s", err.Code)
	}

	if err.Severity != models.SeverityCritical {
		t.Error("Expected critical severity for service unavailable")
	}

	if len(err.AlternativeTools) != 1 {
		t.Error("Expected alternative tools to be present")
	}

	if err.RetryStrategy == nil || !err.RetryStrategy.Retryable {
		t.Error("Expected retryable with backoff")
	}
}

func TestErrorTemplates_ResourceNotFound(t *testing.T) {
	et := NewErrorTemplates()
	suggestedTools := []string{"list_repositories", "search_repositories"}

	err := et.ResourceNotFound("repository", "nonexistent/repo", suggestedTools)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeNotFound {
		t.Errorf("Expected error code NOT_FOUND, got %s", err.Code)
	}

	if err.Resource == nil {
		t.Fatal("Expected resource info to be present")
	}

	if err.Resource.Type != "repository" {
		t.Errorf("Expected resource type 'repository', got %s", err.Resource.Type)
	}

	if err.Resource.ID != "nonexistent/repo" {
		t.Error("Expected resource ID to match")
	}

	if len(err.AlternativeTools) != 2 {
		t.Error("Expected suggested tools to be in alternative tools")
	}
}

func TestErrorTemplates_ValidationError(t *testing.T) {
	et := NewErrorTemplates()

	err := et.ParameterValidationFailed("my_tool", "repo_name", "must not be empty")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeValidation {
		t.Errorf("Expected error code VALIDATION_ERROR, got %s", err.Code)
	}

	if err.Severity != models.SeverityWarning {
		t.Error("Expected warning severity for validation errors")
	}

	if err.Metadata == nil {
		t.Fatal("Expected metadata")
	}

	if toolName, ok := err.Metadata["tool_name"]; !ok || toolName != "my_tool" {
		t.Error("Expected tool_name in metadata")
	}

	if paramName, ok := err.Metadata["parameter_name"]; !ok || paramName != "repo_name" {
		t.Error("Expected parameter_name in metadata")
	}
}

func TestErrorTemplates_ToMCPError(t *testing.T) {
	et := NewErrorTemplates()

	errResp := et.ToolNotFound("test_tool", []string{"category1"})
	mcpErr := ToMCPError(errResp)

	if mcpErr == nil {
		t.Fatal("Expected MCP error, got nil")
	}

	if mcpErr.Code == 0 {
		t.Error("Expected non-zero error code")
	}

	if mcpErr.Message == "" {
		t.Error("Expected error message")
	}

	if mcpErr.Data == nil {
		t.Fatal("Expected error data")
	}

	// Verify AI-friendly fields in data
	data := mcpErr.Data.(map[string]interface{})

	if _, ok := data["suggestion"]; !ok {
		t.Error("Expected suggestion in MCP error data")
	}

	if _, ok := data["recovery_steps"]; !ok {
		t.Error("Expected recovery_steps in MCP error data")
	}

	if _, ok := data["code"]; !ok {
		t.Error("Expected error code in MCP error data")
	}

	if _, ok := data["category"]; !ok {
		t.Error("Expected category in MCP error data")
	}
}

func TestErrorTemplates_ProtocolVersionMismatch(t *testing.T) {
	et := NewErrorTemplates()

	err := et.ProtocolVersionMismatch("1.0.0", "2.0.0")

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	if err.Code != models.ErrorCodeVersionMismatch {
		t.Errorf("Expected error code VERSION_MISMATCH, got %s", err.Code)
	}

	if err.Documentation == "" {
		t.Error("Expected documentation link")
	}

	// Verify actionable recovery steps
	hasUpgradeStep := false
	for _, step := range err.RecoverySteps {
		if step.Action == "upgrade_client" {
			hasUpgradeStep = true
			break
		}
	}
	if !hasUpgradeStep {
		t.Error("Expected upgrade_client in recovery steps")
	}
}
