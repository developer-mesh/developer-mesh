package mcp

import (
	"fmt"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
)

// ErrorTemplates provides standardized error messages and recovery suggestions
type ErrorTemplates struct{}

// NewErrorTemplates creates a new error templates manager
func NewErrorTemplates() *ErrorTemplates {
	return &ErrorTemplates{}
}

// Protocol Errors

// ProtocolVersionMismatch creates error for protocol version incompatibility
func (et *ErrorTemplates) ProtocolVersionMismatch(clientVersion, serverVersion string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeVersionMismatch,
		"Protocol version mismatch",
	).
		WithDetails(fmt.Sprintf("Client requested version %s but server supports %s", clientVersion, serverVersion)).
		WithSeverity(models.SeverityError).
		WithOperation("initialize").
		WithSuggestion(fmt.Sprintf("Update your client to use protocol version %s", serverVersion)).
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "upgrade_client",
				Description: "Upgrade your MCP client to support the latest protocol version",
			},
			{
				Order:       2,
				Action:      "check_compatibility",
				Description: "Check the MCP protocol documentation for version compatibility matrix",
			},
		}).
		WithDocumentation("https://developer-mesh.io/docs/mcp-protocol-versions")
}

// InvalidRequest creates error for malformed requests
func (et *ErrorTemplates) InvalidRequest(details string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeProtocolError,
		"Invalid request format",
	).
		WithDetails(details).
		WithSeverity(models.SeverityError).
		WithSuggestion("Ensure request follows JSON-RPC 2.0 specification with required fields").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "validate_json",
				Description: "Validate that the request is valid JSON",
			},
			{
				Order:       2,
				Action:      "check_required_fields",
				Description: "Ensure 'jsonrpc', 'method', and 'id' fields are present",
			},
			{
				Order:       3,
				Action:      "verify_method",
				Description: "Verify the method name matches supported MCP operations",
			},
		}).
		WithDocumentation("https://developer-mesh.io/docs/mcp-protocol")
}

// UninitializedSession creates error for operations before initialization
func (et *ErrorTemplates) UninitializedSession() *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodePreconditionFail,
		"Session not initialized",
	).
		WithDetails("The initialize method must be called before any other operations").
		WithSeverity(models.SeverityError).
		WithSuggestion("Call the 'initialize' method first to establish the session").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "send_initialize",
				Description: "Send an 'initialize' request with protocol version and client info",
				Parameters: map[string]interface{}{
					"method": "initialize",
					"params": map[string]interface{}{
						"protocolVersion": "2025-06-18",
						"clientInfo": map[string]string{
							"name":    "your-client",
							"version": "1.0.0",
						},
					},
				},
			},
			{
				Order:       2,
				Action:      "send_initialized",
				Description: "After receiving initialize response, send 'initialized' notification",
				Parameters: map[string]interface{}{
					"method": "initialized",
					"params": map[string]interface{}{},
				},
			},
		}).
		WithDocumentation("https://developer-mesh.io/docs/mcp-session-lifecycle")
}

// Authentication Errors

// AuthenticationFailed creates error for failed authentication
func (et *ErrorTemplates) AuthenticationFailed(reason string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeAuthFailed,
		"Authentication failed",
	).
		WithDetails(reason).
		WithSeverity(models.SeverityError).
		WithSuggestion("Verify your API key or credentials are correct").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "check_api_key",
				Description: "Verify the API key is correct and not expired",
			},
			{
				Order:       2,
				Action:      "check_headers",
				Description: "Ensure the Authorization header is properly formatted (Bearer <token>)",
			},
			{
				Order:       3,
				Action:      "regenerate_key",
				Description: "If the key is invalid, regenerate it from the DevMesh dashboard",
			},
		}).
		WithDocumentation("https://developer-mesh.io/docs/authentication")
}

// PermissionDenied creates error for insufficient permissions
func (et *ErrorTemplates) PermissionDenied(resource, operation string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodePermissionDenied,
		"Permission denied",
	).
		WithDetails(fmt.Sprintf("You do not have permission to %s on %s", operation, resource)).
		WithSeverity(models.SeverityError).
		WithResource(models.ResourceInfo{
			Type: resource,
		}).
		WithSuggestion("Contact your administrator to request the required permissions").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "check_permissions",
				Description: "Verify your user has the required permissions in the DevMesh dashboard",
			},
			{
				Order:       2,
				Action:      "request_access",
				Description: "Request access from your team administrator",
			},
			{
				Order:       3,
				Action:      "use_alternative",
				Description: "Use a read-only alternative if available",
				Optional:    true,
			},
		}).
		WithDocumentation("https://developer-mesh.io/docs/permissions")
}

// Tool Execution Errors

// ToolNotFound creates error when a tool doesn't exist
func (et *ErrorTemplates) ToolNotFound(toolName string, availableCategories []string) *models.ErrorResponse {
	err := models.NewErrorResponse(
		models.ErrorCodeNotFound,
		fmt.Sprintf("Tool '%s' not found", toolName),
	).
		WithSeverity(models.SeverityError).
		WithOperation(fmt.Sprintf("tools/call:%s", toolName)).
		WithSuggestion("Use 'tools/list' to see available tools or check the tool name spelling").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "list_tools",
				Description: "Call tools/list to see all available tools",
				Tool:        "tools/list",
			},
			{
				Order:       2,
				Action:      "search_similar",
				Description: "Search for tools with similar names or in the same category",
			},
			{
				Order:       3,
				Action:      "check_spelling",
				Description: "Verify the tool name spelling and format",
			},
		}).
		WithMetadata("tool_name", toolName).
		WithDocumentation("https://developer-mesh.io/docs/available-tools")

	// Add alternative tools suggestion if categories are provided
	if len(availableCategories) > 0 {
		_ = err.WithMetadata("available_categories", availableCategories)
		_ = err.WithSuggestion(fmt.Sprintf("Tool '%s' not found. Browse tools in categories: %v", toolName, availableCategories))
	}

	return err
}

// ToolExecutionFailed creates error when tool execution fails
func (et *ErrorTemplates) ToolExecutionFailed(toolName string, err error, alternativeTools []string) *models.ErrorResponse {
	errResp := models.NewErrorResponse(
		models.ErrorCodeIntegrationError,
		fmt.Sprintf("Tool '%s' execution failed", toolName),
	).
		WithDetails(err.Error()).
		WithSeverity(models.SeverityError).
		WithOperation(fmt.Sprintf("tools/call:%s", toolName)).
		WithSuggestion("Check tool parameters and try again, or use an alternative tool").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "validate_parameters",
				Description: "Verify all required parameters are provided with correct types",
			},
			{
				Order:       2,
				Action:      "check_tool_status",
				Description: "Check if the underlying service is available",
			},
			{
				Order:       3,
				Action:      "retry_with_backoff",
				Description: "Retry the operation after a brief delay",
			},
		}).
		WithMetadata("tool_name", toolName).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:    true,
			MaxAttempts:  3,
			BackoffType:  "exponential",
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		})

	// Add alternative tools if provided
	if len(alternativeTools) > 0 {
		errResp.AlternativeTools = alternativeTools
		var altToolsStep models.RecoveryStep
		if len(alternativeTools) == 1 {
			altToolsStep = models.RecoveryStep{
				Order:       4,
				Action:      "use_alternative",
				Description: fmt.Sprintf("Try using alternative tool: %s", alternativeTools[0]),
				Tool:        alternativeTools[0],
				Optional:    true,
			}
		} else {
			altToolsStep = models.RecoveryStep{
				Order:       4,
				Action:      "use_alternative",
				Description: fmt.Sprintf("Try using one of these alternative tools: %v", alternativeTools),
				Optional:    true,
			}
		}
		errResp.RecoverySteps = append(errResp.RecoverySteps, altToolsStep)
	}

	return errResp
}

// ParameterValidationFailed creates error for invalid parameters
func (et *ErrorTemplates) ParameterValidationFailed(toolName, paramName, issue string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeValidation,
		fmt.Sprintf("Parameter validation failed for '%s'", paramName),
	).
		WithDetails(issue).
		WithSeverity(models.SeverityWarning).
		WithOperation(fmt.Sprintf("tools/call:%s", toolName)).
		WithSuggestion("Check the parameter type and format against the tool's input schema").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "check_schema",
				Description: "Review the tool's inputSchema to see required format and constraints",
				Tool:        "tools/list",
			},
			{
				Order:       2,
				Action:      "fix_parameter",
				Description: fmt.Sprintf("Correct the value for parameter '%s'", paramName),
			},
			{
				Order:       3,
				Action:      "retry",
				Description: "Retry the tool call with corrected parameters",
			},
		}).
		WithMetadata("tool_name", toolName).
		WithMetadata("parameter_name", paramName).
		WithDocumentation("https://developer-mesh.io/docs/tool-parameters")
}

// Rate Limit Errors

// RateLimitExceeded creates error when rate limit is hit
func (et *ErrorTemplates) RateLimitExceeded(limit, remaining int, resetTime time.Time, scope string) *models.ErrorResponse {
	retryAfter := time.Until(resetTime)
	if retryAfter < 0 {
		retryAfter = 1 * time.Second
	}

	return models.NewErrorResponse(
		models.ErrorCodeRateLimit,
		"Rate limit exceeded",
	).
		WithDetails(fmt.Sprintf("You have exceeded the rate limit for %s operations", scope)).
		WithSeverity(models.SeverityWarning).
		WithRetryAfter(retryAfter).
		WithRateLimitInfo(models.RateLimitInfo{
			Limit:     limit,
			Remaining: remaining,
			Reset:     resetTime,
			Scope:     scope,
		}).
		WithSuggestion(fmt.Sprintf("Wait %v before retrying, or reduce request frequency", retryAfter.Round(time.Second))).
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "wait",
				Description: fmt.Sprintf("Wait until %s for rate limit to reset", resetTime.Format(time.RFC3339)),
			},
			{
				Order:       2,
				Action:      "implement_backoff",
				Description: "Implement exponential backoff in your retry logic",
			},
			{
				Order:       3,
				Action:      "batch_requests",
				Description: "Batch multiple operations into single requests where possible",
				Optional:    true,
			},
		}).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:      true,
			MaxAttempts:    5,
			BackoffType:    "exponential",
			InitialDelay:   retryAfter,
			MaxDelay:       5 * time.Minute,
			RetryCondition: "after_rate_limit_reset",
		}).
		WithDocumentation("https://developer-mesh.io/docs/rate-limits")
}

// Network and Timeout Errors

// OperationTimeout creates error when operation times out
func (et *ErrorTemplates) OperationTimeout(operation string, timeout time.Duration) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeTimeout,
		fmt.Sprintf("Operation '%s' timed out", operation),
	).
		WithDetails(fmt.Sprintf("Operation exceeded maximum timeout of %v", timeout)).
		WithSeverity(models.SeverityError).
		WithOperation(operation).
		WithSuggestion("Try again with a longer timeout or reduce the request size").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "reduce_scope",
				Description: "Reduce the scope of the operation (e.g., request fewer items)",
			},
			{
				Order:       2,
				Action:      "increase_timeout",
				Description: "Increase the timeout value if your client supports it",
				Optional:    true,
			},
			{
				Order:       3,
				Action:      "retry",
				Description: "Retry the operation as it may succeed on subsequent attempts",
			},
		}).
		WithMetadata("timeout_seconds", timeout.Seconds()).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:    true,
			MaxAttempts:  3,
			BackoffType:  "exponential",
			InitialDelay: 2 * time.Second,
			MaxDelay:     30 * time.Second,
		})
}

// ServiceUnavailable creates error when external service is down
func (et *ErrorTemplates) ServiceUnavailable(service string, alternativeTools []string) *models.ErrorResponse {
	err := models.NewErrorResponse(
		models.ErrorCodeServiceOffline,
		fmt.Sprintf("Service '%s' is temporarily unavailable", service),
	).
		WithSeverity(models.SeverityCritical).
		WithSuggestion("The service is experiencing issues. Please try again later or use an alternative service").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "retry_later",
				Description: "Wait a few minutes and retry the operation",
			},
			{
				Order:       2,
				Action:      "check_status",
				Description: "Check the service status page for known issues",
			},
		}).
		WithMetadata("service", service).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:    true,
			MaxAttempts:  5,
			BackoffType:  "exponential",
			InitialDelay: 5 * time.Second,
			MaxDelay:     2 * time.Minute,
		}).
		WithDocumentation("https://developer-mesh.io/status")

	// Add alternative tools if provided
	if len(alternativeTools) > 0 {
		err.AlternativeTools = alternativeTools
		err.RecoverySteps = append(err.RecoverySteps, models.RecoveryStep{
			Order:       3,
			Action:      "use_alternative",
			Description: fmt.Sprintf("Use alternative tools while service is unavailable: %v", alternativeTools),
			Optional:    true,
		})
	}

	return err
}

// UpstreamError creates error when upstream service returns error
func (et *ErrorTemplates) UpstreamError(service string, statusCode int, errorMsg string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeUpstreamError,
		fmt.Sprintf("Upstream service '%s' returned error", service),
	).
		WithDetails(fmt.Sprintf("HTTP %d: %s", statusCode, errorMsg)).
		WithSeverity(models.SeverityError).
		WithSuggestion("The external service encountered an error. Check service status or try again later").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "check_parameters",
				Description: "Verify request parameters are valid for the external service",
			},
			{
				Order:       2,
				Action:      "check_permissions",
				Description: "Ensure you have proper permissions in the external service",
			},
			{
				Order:       3,
				Action:      "retry",
				Description: "Retry the operation if it's a transient error",
			},
		}).
		WithMetadata("service", service).
		WithMetadata("status_code", statusCode).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:      statusCode >= 500 || statusCode == 429, // Retry server errors and rate limits
			MaxAttempts:    3,
			BackoffType:    "exponential",
			InitialDelay:   1 * time.Second,
			MaxDelay:       15 * time.Second,
			RetryCondition: "if_transient_error",
		})
}

// Resource Errors

// ResourceNotFound creates error when resource doesn't exist
func (et *ErrorTemplates) ResourceNotFound(resourceType, resourceID string, suggestedTools []string) *models.ErrorResponse {
	err := models.NewErrorResponse(
		models.ErrorCodeNotFound,
		fmt.Sprintf("%s not found", resourceType),
	).
		WithDetails(fmt.Sprintf("No %s with identifier '%s' exists", resourceType, resourceID)).
		WithSeverity(models.SeverityError).
		WithResource(models.ResourceInfo{
			Type: resourceType,
			ID:   resourceID,
		}).
		WithSuggestion(fmt.Sprintf("Verify that the %s exists and you have access to it", resourceType)).
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "verify_id",
				Description: fmt.Sprintf("Check that the %s identifier '%s' is correct", resourceType, resourceID),
			},
			{
				Order:       2,
				Action:      "check_access",
				Description: "Verify you have permission to access this resource",
			},
		})

	// Add tool suggestions for discovering resources
	if len(suggestedTools) > 0 {
		err.AlternativeTools = suggestedTools
		err.RecoverySteps = append(err.RecoverySteps, models.RecoveryStep{
			Order:       3,
			Action:      "list_resources",
			Description: fmt.Sprintf("Use tools to list available %ss: %v", resourceType, suggestedTools),
			Optional:    true,
		})
	}

	return err
}

// Conflict creates error when resource state conflicts
func (et *ErrorTemplates) Conflict(resourceType, resourceID, reason string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeConflict,
		"Resource conflict",
	).
		WithDetails(reason).
		WithSeverity(models.SeverityError).
		WithResource(models.ResourceInfo{
			Type: resourceType,
			ID:   resourceID,
		}).
		WithSuggestion("Refresh the resource state and try again with updated data").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "fetch_latest",
				Description: "Fetch the latest version of the resource",
			},
			{
				Order:       2,
				Action:      "resolve_conflict",
				Description: "Resolve the conflict by updating your changes based on latest state",
			},
			{
				Order:       3,
				Action:      "retry",
				Description: "Retry the operation with resolved data",
			},
		})
}

// System Errors

// InternalError creates error for internal server errors
func (et *ErrorTemplates) InternalError(operation string, err error) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeInternal,
		"Internal server error",
	).
		WithDetails(err.Error()).
		WithSeverity(models.SeverityCritical).
		WithOperation(operation).
		WithSuggestion("An unexpected error occurred. Please try again or contact support if the issue persists").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "retry",
				Description: "Retry the operation as it may be a transient error",
			},
			{
				Order:       2,
				Action:      "contact_support",
				Description: "If the error persists, contact support with the request ID",
			},
		}).
		WithRetryStrategy(models.RetryStrategy{
			Retryable:    true,
			MaxAttempts:  3,
			BackoffType:  "exponential",
			InitialDelay: 1 * time.Second,
			MaxDelay:     10 * time.Second,
		}).
		WithDocumentation("https://developer-mesh.io/support")
}

// ConfigurationError creates error for configuration issues
func (et *ErrorTemplates) ConfigurationError(setting, issue string) *models.ErrorResponse {
	return models.NewErrorResponse(
		models.ErrorCodeConfigError,
		"Configuration error",
	).
		WithDetails(fmt.Sprintf("Invalid configuration for '%s': %s", setting, issue)).
		WithSeverity(models.SeverityCritical).
		WithSuggestion("Check your configuration settings and ensure all required values are provided").
		WithRecoverySteps([]models.RecoveryStep{
			{
				Order:       1,
				Action:      "check_config",
				Description: "Review the configuration file or environment variables",
			},
			{
				Order:       2,
				Action:      "validate_values",
				Description: "Ensure all configuration values are valid and properly formatted",
			},
			{
				Order:       3,
				Action:      "restart_service",
				Description: "Restart the service after correcting the configuration",
			},
		}).
		WithMetadata("setting", setting).
		WithDocumentation("https://developer-mesh.io/docs/configuration")
}
