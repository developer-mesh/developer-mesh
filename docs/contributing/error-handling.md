# Error Handling Guide

## Overview

DevMesh implements a comprehensive, AI-friendly error handling system designed to provide clear, actionable error messages with recovery guidance. This guide covers common error scenarios, recovery strategies, and best practices for handling errors in the DevMesh platform.

## Error Response Structure

All errors in DevMesh follow a standardized `ErrorResponse` structure with rich metadata:

```json
{
  "code": "TOOL_NOT_FOUND",
  "message": "Tool 'github_invalid_tool' not found",
  "category": "RESOURCE",
  "severity": "ERROR",
  "timestamp": "2025-09-30T12:34:56Z",
  "operation": "tools/call:github_invalid_tool",
  "suggestion": "Use 'tools/list' to see available tools or check the tool name spelling",
  "recovery_steps": [
    {
      "order": 1,
      "action": "list_tools",
      "description": "Call tools/list to see all available tools",
      "tool": "tools/list"
    },
    {
      "order": 2,
      "action": "search_similar",
      "description": "Search for tools with similar names or in the same category"
    }
  ],
  "alternative_tools": ["github_list_repositories", "github_search_repositories"],
  "metadata": {
    "tool_name": "github_invalid_tool",
    "available_categories": ["repository", "issues", "pulls"]
  }
}
```

### Key Fields

- **code**: Standardized error code (e.g., `RATE_LIMIT`, `AUTH_FAILED`, `NOT_FOUND`)
- **message**: Human-readable error message
- **category**: Error category (`AUTHENTICATION`, `RESOURCE`, `RATE_LIMIT`, etc.)
- **severity**: Error severity (`INFO`, `WARNING`, `ERROR`, `CRITICAL`, `FATAL`)
- **suggestion**: Immediate actionable suggestion
- **recovery_steps**: Ordered list of recovery actions
- **retry_strategy**: Retry guidance with backoff configuration
- **alternative_tools**: Alternative tools to try
- **metadata**: Additional context-specific information

## Common Error Scenarios

### 1. Authentication Errors

#### Scenario: Invalid API Key

**Error Response:**
```json
{
  "code": "AUTH_FAILED",
  "message": "Authentication failed",
  "details": "API key is invalid or expired",
  "category": "AUTHENTICATION",
  "severity": "ERROR",
  "suggestion": "Verify your API key or credentials are correct",
  "recovery_steps": [
    {
      "order": 1,
      "action": "check_api_key",
      "description": "Verify the API key is correct and not expired"
    },
    {
      "order": 2,
      "action": "check_headers",
      "description": "Ensure the Authorization header is properly formatted (Bearer <token>)"
    },
    {
      "order": 3,
      "action": "regenerate_key",
      "description": "If the key is invalid, regenerate it from the DevMesh dashboard"
    }
  ]
}
```

**Recovery Example (Go):**
```go
func handleAuthError(err *models.ErrorResponse) error {
    if err.Code == models.ErrorCodeAuthFailed {
        // Log the error with context
        log.Printf("Authentication failed: %s", err.Message)

        // Check if API key is set
        apiKey := os.Getenv("DEVMESH_API_KEY")
        if apiKey == "" {
            return fmt.Errorf("API key not set: set DEVMESH_API_KEY environment variable")
        }

        // Verify API key format
        if !isValidAPIKeyFormat(apiKey) {
            return fmt.Errorf("API key format invalid: should match ^[a-zA-Z0-9_-]+$")
        }

        // Suggest regenerating key
        return fmt.Errorf("authentication failed: regenerate API key from dashboard")
    }
    return nil
}

func isValidAPIKeyFormat(key string) bool {
    matched, _ := regexp.MatchString(`^[a-zA-Z0-9_-]+$`, key)
    return matched
}
```

**Recovery Example (Python):**
```python
def handle_auth_error(error_response):
    """Handle authentication errors with recovery steps."""
    if error_response.get('code') == 'AUTH_FAILED':
        # Check environment variable
        api_key = os.getenv('DEVMESH_API_KEY')
        if not api_key:
            raise ValueError("API key not set: set DEVMESH_API_KEY environment variable")

        # Follow recovery steps
        for step in error_response.get('recovery_steps', []):
            if step['action'] == 'check_api_key':
                # Validate key format
                if not re.match(r'^[a-zA-Z0-9_-]+$', api_key):
                    raise ValueError(f"Invalid API key format: {step['description']}")
            elif step['action'] == 'regenerate_key':
                print(f"Action needed: {step['description']}")

        raise AuthenticationError(error_response.get('message'))
```

#### Scenario: Insufficient Permissions

**Error Response:**
```json
{
  "code": "PERMISSION_DENIED",
  "message": "Permission denied",
  "details": "You do not have permission to delete on repository",
  "category": "AUTHENTICATION",
  "severity": "ERROR",
  "resource": {
    "type": "repository",
    "name": "my-org/my-repo"
  },
  "suggestion": "Contact your administrator to request the required permissions"
}
```

**Recovery Example:**
```go
func handlePermissionError(err *models.ErrorResponse, operation string) error {
    if err.Code == models.ErrorCodePermissionDenied {
        resourceType := ""
        if err.Resource != nil {
            resourceType = err.Resource.Type
        }

        // Check if there are alternative read-only operations
        if strings.Contains(operation, "delete") || strings.Contains(operation, "update") {
            // Suggest read-only alternatives
            readOnlyOp := strings.Replace(operation, "delete", "get", 1)
            readOnlyOp = strings.Replace(readOnlyOp, "update", "get", 1)

            return fmt.Errorf("permission denied for %s on %s: try read-only operation %s instead",
                operation, resourceType, readOnlyOp)
        }

        return fmt.Errorf("permission denied: contact administrator for %s access", resourceType)
    }
    return nil
}
```

### 2. Rate Limit Errors

#### Scenario: Rate Limit Exceeded

**Error Response:**
```json
{
  "code": "RATE_LIMIT",
  "message": "Rate limit exceeded",
  "details": "You have exceeded the rate limit for GitHub operations",
  "category": "RATE_LIMIT",
  "severity": "WARNING",
  "retry_after": 3600000000000,
  "rate_limit_info": {
    "limit": 5000,
    "remaining": 0,
    "reset": "2025-09-30T13:34:56Z",
    "window": 3600000000000,
    "scope": "GitHub"
  },
  "suggestion": "Wait 1h0m0s before retrying, or reduce request frequency",
  "retry_strategy": {
    "retryable": true,
    "max_attempts": 5,
    "backoff_type": "exponential",
    "initial_delay": 3600000000000,
    "max_delay": 300000000000,
    "retry_condition": "after_rate_limit_reset"
  }
}
```

**Recovery Example (Go with Exponential Backoff):**
```go
func executeWithRateLimitRetry(ctx context.Context, operation func() error) error {
    maxAttempts := 5
    baseDelay := 1 * time.Second

    for attempt := 0; attempt < maxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        // Check if it's a rate limit error
        var errResp *models.ErrorResponse
        if errors.As(err, &errResp) && errResp.Code == models.ErrorCodeRateLimit {
            // Use retry_after from error if available
            var waitDuration time.Duration
            if errResp.RetryAfter != nil {
                waitDuration = *errResp.RetryAfter
            } else {
                // Exponential backoff: 1s, 2s, 4s, 8s, 16s
                waitDuration = baseDelay * time.Duration(1<<uint(attempt))
                if errResp.RetryStrategy != nil && waitDuration > errResp.RetryStrategy.MaxDelay {
                    waitDuration = errResp.RetryStrategy.MaxDelay
                }
            }

            log.Printf("Rate limit hit (attempt %d/%d), waiting %v",
                attempt+1, maxAttempts, waitDuration)

            select {
            case <-time.After(waitDuration):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        } else {
            // Not a rate limit error, return immediately
            return err
        }
    }

    return fmt.Errorf("operation failed after %d attempts due to rate limiting", maxAttempts)
}

// Usage example
err := executeWithRateLimitRetry(ctx, func() error {
    return callGitHubTool("github_list_repositories", params)
})
```

**Recovery Example (Python with Retry Decorator):**
```python
import time
import functools
from typing import Callable, Any

def retry_on_rate_limit(max_attempts=5):
    """Decorator to retry operations on rate limit errors."""
    def decorator(func: Callable) -> Callable:
        @functools.wraps(func)
        def wrapper(*args, **kwargs) -> Any:
            base_delay = 1.0

            for attempt in range(max_attempts):
                try:
                    return func(*args, **kwargs)
                except RateLimitError as e:
                    if attempt == max_attempts - 1:
                        raise

                    # Use retry_after from error if available
                    wait_seconds = e.retry_after or (base_delay * (2 ** attempt))

                    # Cap at max_delay from retry strategy
                    if hasattr(e, 'retry_strategy') and e.retry_strategy.get('max_delay'):
                        wait_seconds = min(wait_seconds, e.retry_strategy['max_delay'])

                    print(f"Rate limit hit (attempt {attempt+1}/{max_attempts}), "
                          f"waiting {wait_seconds}s")
                    time.sleep(wait_seconds)

            raise Exception(f"Operation failed after {max_attempts} attempts")

        return wrapper
    return decorator

# Usage
@retry_on_rate_limit(max_attempts=5)
def list_repositories(params):
    return call_github_tool("github_list_repositories", params)
```

### 3. Tool Execution Errors

#### Scenario: Tool Not Found

**Error Response:**
```json
{
  "code": "NOT_FOUND",
  "message": "Tool 'github_get_repositry' not found",
  "category": "RESOURCE",
  "severity": "ERROR",
  "operation": "tools/call:github_get_repositry",
  "suggestion": "Use 'tools/list' to see available tools or check the tool name spelling",
  "alternative_tools": ["github_get_repository", "github_list_repositories", "github_search_repositories"],
  "recovery_steps": [
    {
      "order": 1,
      "action": "list_tools",
      "description": "Call tools/list to see all available tools",
      "tool": "tools/list"
    },
    {
      "order": 2,
      "action": "search_similar",
      "description": "Search for tools with similar names or in the same category"
    },
    {
      "order": 3,
      "action": "check_spelling",
      "description": "Verify the tool name spelling and format"
    }
  ],
  "metadata": {
    "tool_name": "github_get_repositry",
    "available_categories": ["repository", "issues", "pulls", "ci/cd"]
  }
}
```

**Recovery Example (Fuzzy Matching):**
```go
import "github.com/lithammer/fuzzysearch/fuzzy"

func findSimilarTool(toolName string, availableTools []string) (string, bool) {
    // Try exact match first
    for _, tool := range availableTools {
        if strings.EqualFold(tool, toolName) {
            return tool, true
        }
    }

    // Try fuzzy matching
    matches := fuzzy.RankFind(toolName, availableTools)
    if len(matches) > 0 && matches[0].Distance < 3 {
        return matches[0].Target, true
    }

    return "", false
}

func handleToolNotFound(err *models.ErrorResponse) error {
    if err.Code == models.ErrorCodeNotFound {
        toolName, _ := err.Metadata["tool_name"].(string)

        // Check if alternative tools are suggested
        if len(err.AlternativeTools) > 0 {
            log.Printf("Tool '%s' not found. Did you mean one of these?", toolName)
            for _, alt := range err.AlternativeTools {
                log.Printf("  - %s", alt)
            }

            // Auto-correct to most similar tool if confidence is high
            if len(err.AlternativeTools) == 1 {
                return fmt.Errorf("tool '%s' not found, use '%s' instead",
                    toolName, err.AlternativeTools[0])
            }
        }

        return fmt.Errorf("tool not found: %s", err.Suggestion)
    }
    return nil
}
```

#### Scenario: Parameter Validation Failed

**Error Response:**
```json
{
  "code": "VALIDATION_ERROR",
  "message": "Parameter validation failed for 'issue_number'",
  "details": "Expected integer, got string",
  "category": "VALIDATION",
  "severity": "WARNING",
  "operation": "tools/call:github_get_issue",
  "suggestion": "Check the parameter type and format against the tool's input schema",
  "recovery_steps": [
    {
      "order": 1,
      "action": "check_schema",
      "description": "Review the tool's inputSchema to see required format and constraints",
      "tool": "tools/list"
    },
    {
      "order": 2,
      "action": "fix_parameter",
      "description": "Correct the value for parameter 'issue_number'"
    },
    {
      "order": 3,
      "action": "retry",
      "description": "Retry the tool call with corrected parameters"
    }
  ],
  "metadata": {
    "tool_name": "github_get_issue",
    "parameter_name": "issue_number"
  }
}
```

**Recovery Example (Parameter Validation):**
```go
func validateAndFixParameters(toolName string, params map[string]interface{}, schema map[string]interface{}) (map[string]interface{}, error) {
    fixed := make(map[string]interface{})

    for key, value := range params {
        schemaType, ok := schema[key].(string)
        if !ok {
            fixed[key] = value
            continue
        }

        // Type conversion/validation
        switch schemaType {
        case "integer":
            switch v := value.(type) {
            case float64:
                fixed[key] = int(v)
            case string:
                intVal, err := strconv.Atoi(v)
                if err != nil {
                    return nil, fmt.Errorf("parameter '%s': cannot convert '%v' to integer", key, v)
                }
                fixed[key] = intVal
            case int:
                fixed[key] = v
            default:
                return nil, fmt.Errorf("parameter '%s': expected integer, got %T", key, v)
            }
        case "string":
            fixed[key] = fmt.Sprintf("%v", value)
        case "boolean":
            boolVal, ok := value.(bool)
            if !ok {
                return nil, fmt.Errorf("parameter '%s': expected boolean, got %T", key, value)
            }
            fixed[key] = boolVal
        default:
            fixed[key] = value
        }
    }

    return fixed, nil
}
```

### 4. Network and Timeout Errors

#### Scenario: Operation Timeout

**Error Response:**
```json
{
  "code": "TIMEOUT",
  "message": "Operation 'github_list_commits' timed out",
  "details": "Operation exceeded maximum timeout of 30s",
  "category": "NETWORK",
  "severity": "ERROR",
  "operation": "github_list_commits",
  "suggestion": "Try again with a longer timeout or reduce the request size",
  "retry_strategy": {
    "retryable": true,
    "max_attempts": 3,
    "backoff_type": "exponential",
    "initial_delay": 2000000000,
    "max_delay": 30000000000
  },
  "metadata": {
    "timeout_seconds": 30
  }
}
```

**Recovery Example (Context with Timeout):**
```go
func executeWithTimeout(ctx context.Context, operation string, timeout time.Duration, fn func(context.Context) error) error {
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    resultCh := make(chan error, 1)

    go func() {
        resultCh <- fn(ctx)
    }()

    select {
    case err := <-resultCh:
        return err
    case <-ctx.Done():
        if ctx.Err() == context.DeadlineExceeded {
            return &models.ErrorResponse{
                Code:      models.ErrorCodeTimeout,
                Message:   fmt.Sprintf("Operation '%s' timed out", operation),
                Details:   fmt.Sprintf("Operation exceeded maximum timeout of %v", timeout),
                Category:  models.CategoryNetwork,
                Severity:  models.SeverityError,
                Operation: operation,
                Suggestion: "Try again with a longer timeout or reduce the request size",
                Metadata: map[string]interface{}{
                    "timeout_seconds": timeout.Seconds(),
                },
            }
        }
        return ctx.Err()
    }
}

// Usage with progressive timeout increase
func executeWithProgressiveTimeout(ctx context.Context, operation string, fn func(context.Context) error) error {
    timeouts := []time.Duration{30 * time.Second, 60 * time.Second, 120 * time.Second}

    for i, timeout := range timeouts {
        log.Printf("Attempting %s with timeout %v (attempt %d/%d)", operation, timeout, i+1, len(timeouts))

        err := executeWithTimeout(ctx, operation, timeout, fn)
        if err == nil {
            return nil
        }

        var errResp *models.ErrorResponse
        if errors.As(err, &errResp) && errResp.Code == models.ErrorCodeTimeout {
            if i < len(timeouts)-1 {
                log.Printf("Timeout occurred, retrying with longer timeout...")
                continue
            }
        }

        return err
    }

    return fmt.Errorf("operation failed after trying all timeout durations")
}
```

#### Scenario: Service Unavailable

**Error Response:**
```json
{
  "code": "SERVICE_OFFLINE",
  "message": "Service 'GitHub' is temporarily unavailable",
  "category": "NETWORK",
  "severity": "CRITICAL",
  "suggestion": "The service is experiencing issues. Please try again later or use an alternative service",
  "alternative_tools": ["harness_gitops_applications_list"],
  "retry_strategy": {
    "retryable": true,
    "max_attempts": 5,
    "backoff_type": "exponential",
    "initial_delay": 5000000000,
    "max_delay": 120000000000
  },
  "metadata": {
    "service": "GitHub"
  }
}
```

**Recovery Example (Circuit Breaker Pattern):**
```go
type CircuitBreaker struct {
    maxFailures    int
    resetTimeout   time.Duration
    failures       int
    lastFailTime   time.Time
    state          string // "closed", "open", "half-open"
    mu             sync.RWMutex
}

func NewCircuitBreaker(maxFailures int, resetTimeout time.Duration) *CircuitBreaker {
    return &CircuitBreaker{
        maxFailures:  maxFailures,
        resetTimeout: resetTimeout,
        state:        "closed",
    }
}

func (cb *CircuitBreaker) Execute(fn func() error) error {
    cb.mu.Lock()

    // Check if we should transition from open to half-open
    if cb.state == "open" && time.Since(cb.lastFailTime) > cb.resetTimeout {
        cb.state = "half-open"
        cb.failures = 0
    }

    if cb.state == "open" {
        cb.mu.Unlock()
        return fmt.Errorf("circuit breaker is open, service unavailable")
    }

    cb.mu.Unlock()

    // Execute the function
    err := fn()

    cb.mu.Lock()
    defer cb.mu.Unlock()

    if err != nil {
        // Check if it's a service unavailable error
        var errResp *models.ErrorResponse
        if errors.As(err, &errResp) && errResp.Code == models.ErrorCodeServiceOffline {
            cb.failures++
            cb.lastFailTime = time.Now()

            if cb.failures >= cb.maxFailures {
                cb.state = "open"
                log.Printf("Circuit breaker opened after %d failures", cb.failures)
            }
        }
        return err
    }

    // Success - reset circuit breaker
    if cb.state == "half-open" {
        cb.state = "closed"
        cb.failures = 0
        log.Printf("Circuit breaker closed after successful call")
    }

    return nil
}
```

### 5. Resource Errors

#### Scenario: Resource Not Found

**Error Response:**
```json
{
  "code": "NOT_FOUND",
  "message": "repository not found",
  "details": "No repository with identifier 'myorg/nonexistent-repo' exists",
  "category": "RESOURCE",
  "severity": "ERROR",
  "resource": {
    "type": "repository",
    "id": "myorg/nonexistent-repo"
  },
  "suggestion": "Verify that the repository exists and you have access to it",
  "alternative_tools": ["github_list_repositories", "github_search_repositories"],
  "recovery_steps": [
    {
      "order": 1,
      "action": "verify_id",
      "description": "Check that the repository identifier 'myorg/nonexistent-repo' is correct"
    },
    {
      "order": 2,
      "action": "check_access",
      "description": "Verify you have permission to access this resource"
    },
    {
      "order": 3,
      "action": "list_resources",
      "description": "Use tools to list available repositorys",
      "optional": true
    }
  ]
}
```

**Recovery Example:**
```go
func handleResourceNotFound(err *models.ErrorResponse, resourceType string) error {
    if err.Code != models.ErrorCodeNotFound {
        return err
    }

    // Try alternative discovery tools
    if len(err.AlternativeTools) > 0 {
        log.Printf("Resource not found. Trying discovery with tools: %v", err.AlternativeTools)

        for _, tool := range err.AlternativeTools {
            if strings.Contains(tool, "list") || strings.Contains(tool, "search") {
                // Use the listing/search tool to find the resource
                log.Printf("Suggestion: Use '%s' to find available %ss", tool, resourceType)

                return fmt.Errorf("resource not found: use '%s' to discover available %ss",
                    tool, resourceType)
            }
        }
    }

    return fmt.Errorf("%s not found: %s", resourceType, err.Suggestion)
}
```

## Retry Strategy Recommendations

### 1. Exponential Backoff Strategy

**When to Use:**
- Rate limit errors
- Timeout errors
- Temporary network failures
- Service unavailable errors

**Implementation:**
```go
type ExponentialBackoff struct {
    InitialDelay time.Duration
    MaxDelay     time.Duration
    Multiplier   float64
    MaxAttempts  int
}

func (eb *ExponentialBackoff) Execute(ctx context.Context, operation func() error) error {
    delay := eb.InitialDelay

    for attempt := 0; attempt < eb.MaxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        // Check if error is retryable
        var errResp *models.ErrorResponse
        if !errors.As(err, &errResp) || !errResp.IsRetryable() {
            return err
        }

        if attempt == eb.MaxAttempts-1 {
            return fmt.Errorf("max attempts reached: %w", err)
        }

        // Add jitter to prevent thundering herd
        jitter := time.Duration(rand.Float64() * float64(delay) * 0.1)
        waitTime := delay + jitter

        log.Printf("Attempt %d failed, retrying in %v", attempt+1, waitTime)

        select {
        case <-time.After(waitTime):
            delay = time.Duration(float64(delay) * eb.Multiplier)
            if delay > eb.MaxDelay {
                delay = eb.MaxDelay
            }
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return fmt.Errorf("operation failed after %d attempts", eb.MaxAttempts)
}

// Usage
backoff := &ExponentialBackoff{
    InitialDelay: 1 * time.Second,
    MaxDelay:     30 * time.Second,
    Multiplier:   2.0,
    MaxAttempts:  5,
}

err := backoff.Execute(ctx, func() error {
    return callTool("github_list_repositories", params)
})
```

### 2. Fixed Interval Retry

**When to Use:**
- Known fixed rate limits
- Scheduled maintenance windows
- Predictable service patterns

**Implementation:**
```go
func retryWithFixedInterval(ctx context.Context, interval time.Duration, maxAttempts int, operation func() error) error {
    for attempt := 0; attempt < maxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        if attempt < maxAttempts-1 {
            select {
            case <-time.After(interval):
                continue
            case <-ctx.Done():
                return ctx.Err()
            }
        }
    }

    return fmt.Errorf("operation failed after %d attempts", maxAttempts)
}
```

### 3. Adaptive Retry Strategy

**When to Use:**
- Complex scenarios with multiple error types
- When error responses include retry guidance

**Implementation:**
```go
func retryWithAdaptiveStrategy(ctx context.Context, operation func() error) error {
    maxAttempts := 5

    for attempt := 0; attempt < maxAttempts; attempt++ {
        err := operation()
        if err == nil {
            return nil
        }

        var errResp *models.ErrorResponse
        if !errors.As(err, &errResp) {
            return err // Not a structured error, don't retry
        }

        // Check if error is retryable
        if !errResp.IsRetryable() {
            return err
        }

        if attempt == maxAttempts-1 {
            return fmt.Errorf("max attempts reached: %w", err)
        }

        // Use retry strategy from error response
        var waitTime time.Duration
        if errResp.RetryAfter != nil {
            waitTime = *errResp.RetryAfter
        } else if errResp.RetryStrategy != nil {
            // Calculate backoff based on strategy
            if errResp.RetryStrategy.BackoffType == "exponential" {
                waitTime = errResp.RetryStrategy.InitialDelay * time.Duration(1<<uint(attempt))
                if waitTime > errResp.RetryStrategy.MaxDelay {
                    waitTime = errResp.RetryStrategy.MaxDelay
                }
            } else {
                waitTime = errResp.RetryStrategy.InitialDelay
            }
        } else {
            waitTime = time.Duration(1<<uint(attempt)) * time.Second
        }

        log.Printf("Retrying after %v (attempt %d/%d): %s",
            waitTime, attempt+1, maxAttempts, errResp.Message)

        select {
        case <-time.After(waitTime):
            continue
        case <-ctx.Done():
            return ctx.Err()
        }
    }

    return fmt.Errorf("operation failed after %d attempts", maxAttempts)
}
```

## Error Handling Best Practices

### 1. Always Check Error Codes

Don't rely solely on error messages. Use standardized error codes for programmatic handling:

```go
func handleError(err error) {
    var errResp *models.ErrorResponse
    if !errors.As(err, &errResp) {
        // Not a structured error
        log.Printf("Unexpected error: %v", err)
        return
    }

    // Handle based on error code
    switch errResp.Code {
    case models.ErrorCodeRateLimit:
        handleRateLimit(errResp)
    case models.ErrorCodeAuthFailed:
        handleAuthError(errResp)
    case models.ErrorCodeNotFound:
        handleNotFound(errResp)
    case models.ErrorCodeTimeout:
        handleTimeout(errResp)
    default:
        log.Printf("Unhandled error code %s: %s", errResp.Code, errResp.Message)
    }
}
```

### 2. Implement Proper Logging

Log errors with full context for debugging:

```go
func logError(err *models.ErrorResponse, context map[string]interface{}) {
    logData := map[string]interface{}{
        "error_code":     string(err.Code),
        "error_message":  err.Message,
        "error_category": string(err.Category),
        "error_severity": string(err.Severity),
        "operation":      err.Operation,
    }

    // Add custom context
    for k, v := range context {
        logData[k] = v
    }

    // Add metadata from error
    for k, v := range err.Metadata {
        logData[fmt.Sprintf("error_%s", k)] = v
    }

    logger.Error("Operation failed", logData)
}
```

### 3. Follow Recovery Steps

Use the structured recovery steps provided in errors:

```go
func executeRecoverySteps(err *models.ErrorResponse) error {
    if len(err.RecoverySteps) == 0 {
        return fmt.Errorf("no recovery steps available")
    }

    log.Printf("Attempting recovery for error: %s", err.Message)

    for _, step := range err.RecoverySteps {
        log.Printf("Step %d: %s - %s", step.Order, step.Action, step.Description)

        // Execute recovery action
        switch step.Action {
        case "list_tools":
            if step.Tool != "" {
                log.Printf("  Suggested tool: %s", step.Tool)
            }
        case "check_api_key":
            log.Printf("  Checking API key configuration...")
            // Implement key validation
        case "retry":
            log.Printf("  Will retry operation...")
            return nil // Signal to retry
        case "use_alternative":
            if !step.Optional {
                log.Printf("  Alternative required: %v", err.AlternativeTools)
            }
        }

        if step.Optional {
            log.Printf("  (Optional step)")
        }
    }

    return nil
}
```

### 4. Use Alternative Tools

When a tool fails, try suggested alternatives:

```go
func executeWithAlternatives(toolName string, params map[string]interface{}, alternatives []string) (interface{}, error) {
    // Try primary tool
    result, err := executeTool(toolName, params)
    if err == nil {
        return result, nil
    }

    // Check for alternative tools in error
    var errResp *models.ErrorResponse
    if errors.As(err, &errResp) && len(errResp.AlternativeTools) > 0 {
        alternatives = errResp.AlternativeTools
    }

    // Try alternatives
    for _, altTool := range alternatives {
        log.Printf("Primary tool '%s' failed, trying alternative '%s'", toolName, altTool)

        result, err := executeTool(altTool, params)
        if err == nil {
            log.Printf("Alternative tool '%s' succeeded", altTool)
            return result, nil
        }
    }

    return nil, fmt.Errorf("all tools failed, including alternatives")
}
```

### 5. Implement Timeout Management

Always use contexts with timeouts for external operations:

```go
func safeToolExecution(ctx context.Context, toolName string, params map[string]interface{}, timeout time.Duration) (interface{}, error) {
    // Create timeout context
    ctx, cancel := context.WithTimeout(ctx, timeout)
    defer cancel()

    // Execute with timeout
    resultCh := make(chan toolResult, 1)

    go func() {
        result, err := executeTool(toolName, params)
        resultCh <- toolResult{result: result, err: err}
    }()

    select {
    case res := <-resultCh:
        return res.result, res.err
    case <-ctx.Done():
        if ctx.Err() == context.DeadlineExceeded {
            return nil, &models.ErrorResponse{
                Code:     models.ErrorCodeTimeout,
                Message:  fmt.Sprintf("Tool '%s' execution timed out", toolName),
                Category: models.CategoryNetwork,
                Severity: models.SeverityError,
                Metadata: map[string]interface{}{
                    "tool_name":       toolName,
                    "timeout_seconds": timeout.Seconds(),
                },
            }
        }
        return nil, ctx.Err()
    }
}

type toolResult struct {
    result interface{}
    err    error
}
```

### 6. Error Severity Handling

Handle errors differently based on severity:

```go
func handleBySeverity(err *models.ErrorResponse) {
    switch err.Severity {
    case models.SeverityInfo:
        // Log for information
        log.Info(err.Message)

    case models.SeverityWarning:
        // Log warning but continue operation
        log.Warn(err.Message)
        // Possibly notify user

    case models.SeverityError:
        // Log error and stop current operation
        log.Error(err.Message)
        // Attempt recovery if possible

    case models.SeverityCritical:
        // Log critical error, stop operation, alert team
        log.Error(err.Message)
        sendAlert(err)

    case models.SeverityFatal:
        // Log fatal error, initiate graceful shutdown
        log.Fatal(err.Message)
        initiateShutdown()
    }
}
```

### 7. AI Agent Best Practices

For AI agents using DevMesh tools:

```python
class DevMeshErrorHandler:
    """Error handler optimized for AI agents."""

    def handle_error(self, error_response):
        """Handle DevMesh error with AI-friendly recovery."""

        # 1. Extract key information
        code = error_response.get('code')
        suggestion = error_response.get('suggestion')
        recovery_steps = error_response.get('recovery_steps', [])
        alternative_tools = error_response.get('alternative_tools', [])

        # 2. Log structured information for AI context
        self.log_for_ai({
            'error_code': code,
            'suggestion': suggestion,
            'recovery_steps': [step['description'] for step in recovery_steps],
            'alternatives': alternative_tools
        })

        # 3. Attempt automatic recovery
        if code == 'TOOL_NOT_FOUND' and alternative_tools:
            # Try the most similar tool automatically
            return self.retry_with_alternative(alternative_tools[0])

        elif code == 'RATE_LIMIT':
            # Implement intelligent backoff
            retry_after = error_response.get('retry_after')
            return self.schedule_retry(retry_after)

        elif code == 'VALIDATION_ERROR':
            # Attempt parameter correction
            param_name = error_response.get('metadata', {}).get('parameter_name')
            return self.fix_parameter(param_name, error_response)

        # 4. If automatic recovery not possible, return actionable message
        return self.create_ai_friendly_message(error_response)

    def create_ai_friendly_message(self, error_response):
        """Create a clear message for AI agent to understand and act on."""
        message_parts = [
            f"Error: {error_response.get('message')}",
            f"Suggestion: {error_response.get('suggestion')}"
        ]

        if error_response.get('recovery_steps'):
            message_parts.append("Recovery steps:")
            for step in error_response['recovery_steps']:
                message_parts.append(f"  {step['order']}. {step['description']}")

        if error_response.get('alternative_tools'):
            message_parts.append(f"Alternative tools: {', '.join(error_response['alternative_tools'])}")

        return '\n'.join(message_parts)
```

## Summary

DevMesh's error handling system is designed to be:

1. **Comprehensive**: All errors include detailed context and metadata
2. **Actionable**: Every error includes recovery suggestions and steps
3. **Retryable**: Clear retry strategies with exponential backoff
4. **AI-Friendly**: Structured format with alternative tools and next steps
5. **Standardized**: Consistent error codes and categories across the platform

By following these patterns and practices, you can build robust integrations that gracefully handle errors and provide excellent user experience.

## Additional Resources

- [Error Taxonomy Reference](../pkg/models/errors.go)
- [Error Templates Documentation](../apps/edge-mcp/internal/mcp/error_templates.go)
- [MCP Protocol Error Handling](https://developer-mesh.io/docs/mcp-protocol)
- [API Error Reference](https://developer-mesh.io/api/errors)
