package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// DynamicToolAdapter handles execution of dynamically discovered tools
type DynamicToolAdapter struct {
	tool          *models.DynamicTool
	specCache     repository.OpenAPICacheRepository
	httpClient    *http.Client
	authenticator *tools.DynamicAuthenticator
	encryptionSvc *security.EncryptionService
	logger        observability.Logger
	router        routers.Router
}

// NewDynamicToolAdapter creates a new adapter for a dynamic tool
func NewDynamicToolAdapter(
	tool *models.DynamicTool,
	specCache repository.OpenAPICacheRepository,
	encryptionSvc *security.EncryptionService,
	logger observability.Logger,
) (*DynamicToolAdapter, error) {
	// Create HTTP client with timeout
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	// TODO: Apply retry policy if configured
	// if tool.RetryPolicy != nil {
	//     Wrap client with retry logic
	// }

	return &DynamicToolAdapter{
		tool:          tool,
		specCache:     specCache,
		httpClient:    httpClient,
		authenticator: tools.NewDynamicAuthenticator(),
		encryptionSvc: encryptionSvc,
		logger:        logger,
	}, nil
}

// ListActions returns available actions from the OpenAPI spec
func (a *DynamicToolAdapter) ListActions(ctx context.Context) ([]models.ToolAction, error) {
	// Get the OpenAPI spec
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	var actions []models.ToolAction

	// Iterate through all paths and operations
	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				if operation == nil {
					continue
				}

				// Create action ID
				actionID := operation.OperationID
				if actionID == "" {
					actionID = fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ReplaceAll(path, "/", "_"))
				}

				// Extract parameters
				var parameters []models.ActionParameter

				// Path parameters
				for _, param := range pathItem.Parameters {
					if param.Value != nil {
						parameters = append(parameters, a.convertParameter(param.Value))
					}
				}

				// Operation parameters
				for _, param := range operation.Parameters {
					if param.Value != nil {
						parameters = append(parameters, a.convertParameter(param.Value))
					}
				}

				// Request body as parameter
				if operation.RequestBody != nil && operation.RequestBody.Value != nil {
					if content, ok := operation.RequestBody.Value.Content["application/json"]; ok {
						param := models.ActionParameter{
							Name:        "body",
							In:          "body",
							Required:    operation.RequestBody.Value.Required,
							Description: operation.RequestBody.Value.Description,
							Type:        "object",
						}
						if content.Schema != nil && content.Schema.Value != nil {
							param.Description = content.Schema.Value.Description
						}
						parameters = append(parameters, param)
					}
				}

				// Create action
				action := models.ToolAction{
					ID:          actionID,
					Name:        operation.Summary,
					Description: operation.Description,
					Method:      method,
					Path:        path,
					Parameters:  parameters,
				}

				if action.Name == "" {
					action.Name = actionID
				}

				actions = append(actions, action)
			}
		}
	}

	return actions, nil
}

// ExecuteAction executes a specific action
func (a *DynamicToolAdapter) ExecuteAction(ctx context.Context, actionID string, params map[string]interface{}) (*models.ToolExecutionResponse, error) {
	startTime := time.Now()

	// Get the OpenAPI spec
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	// Find the operation
	operation, path, method, err := a.findOperation(spec, actionID)
	if err != nil {
		return nil, err
	}

	// Build the request
	req, err := a.buildRequest(ctx, spec, operation, path, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Decrypt and apply authentication
	if err := a.applyAuthentication(req); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Execute the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return &models.ToolExecutionResponse{
			Success:    false,
			Error:      err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
			ExecutedAt: startTime,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response based on content type
	var responseBody interface{}
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(body, &responseBody); err != nil {
			responseBody = string(body)
		}
	} else {
		responseBody = string(body)
	}

	// Build response
	response := &models.ToolExecutionResponse{
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       responseBody,
		Duration:   time.Since(startTime).Milliseconds(),
		ExecutedAt: startTime,
	}

	if !response.Success {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return response, nil
}

// ExecuteWithPassthrough executes an action with passthrough authentication
func (a *DynamicToolAdapter) ExecuteWithPassthrough(
	ctx context.Context,
	actionID string,
	params map[string]interface{},
	passthroughAuth *models.PassthroughAuthBundle,
	passthroughConfig *models.EnhancedPassthroughConfig,
) (*models.ToolExecutionResponse, error) {
	startTime := time.Now()

	// Get the OpenAPI spec
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	// Find the operation
	operation, path, method, err := a.findOperation(spec, actionID)
	if err != nil {
		return nil, err
	}

	// Build the request
	req, err := a.buildRequest(ctx, spec, operation, path, method, params)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Apply authentication with passthrough support
	if err := a.applyAuthenticationWithPassthrough(req, passthroughAuth, passthroughConfig); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Execute the request
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return &models.ToolExecutionResponse{
			Success:    false,
			Error:      err.Error(),
			Duration:   time.Since(startTime).Milliseconds(),
			ExecutedAt: startTime,
		}, nil
	}
	defer func() { _ = resp.Body.Close() }()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Parse response based on content type
	var responseBody interface{}
	contentType := resp.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		if err := json.Unmarshal(body, &responseBody); err != nil {
			responseBody = string(body)
		}
	} else {
		responseBody = string(body)
	}

	// Build response
	response := &models.ToolExecutionResponse{
		Success:    resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Headers:    resp.Header,
		Body:       responseBody,
		Duration:   time.Since(startTime).Milliseconds(),
		ExecutedAt: startTime,
	}

	if !response.Success {
		response.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return response, nil
}

// applyAuthenticationWithPassthrough applies authentication with passthrough support
func (a *DynamicToolAdapter) applyAuthenticationWithPassthrough(
	req *http.Request,
	passthroughAuth *models.PassthroughAuthBundle,
	passthroughConfig *models.EnhancedPassthroughConfig,
) error {
	// Determine authentication strategy
	usePassthrough := false

	if passthroughConfig != nil {
		switch passthroughConfig.Mode {
		case "required":
			if passthroughAuth == nil {
				return fmt.Errorf("passthrough authentication is required but not provided")
			}
			usePassthrough = true
		case "optional":
			if passthroughAuth != nil {
				usePassthrough = true
			}
		case "disabled":
			usePassthrough = false
		case "hybrid":
			// Hybrid mode: use passthrough if available, otherwise fall back
			usePassthrough = (passthroughAuth != nil)
		}
	}

	if usePassthrough && passthroughAuth != nil {
		// Create passthrough authenticator
		passthroughAuthenticator := tools.NewPassthroughAuthenticator(a.logger, nil)

		// Apply passthrough authentication
		if err := passthroughAuthenticator.ApplyPassthroughAuth(req, a.tool.ToolName, passthroughConfig, passthroughAuth); err != nil {
			// Check if fallback is allowed
			if passthroughConfig != nil && passthroughConfig.FallbackToService {
				a.logger.Warn("Passthrough auth failed, falling back to stored credentials", map[string]interface{}{
					"error": err.Error(),
				})
				return a.applyAuthentication(req)
			}
			return err
		}
		return nil
	}

	// Use stored credentials
	return a.applyAuthentication(req)
}

// getOpenAPISpec retrieves the OpenAPI spec from cache or fetches it
func (a *DynamicToolAdapter) getOpenAPISpec(ctx context.Context) (*openapi3.T, error) {
	// Get spec URL from tool config
	specURL, ok := a.tool.Config["spec_url"].(string)
	if !ok || specURL == "" {
		a.logger.Error("No OpenAPI spec URL in tool configuration", map[string]interface{}{
			"tool_name": a.tool.ToolName,
			"tool_id":   a.tool.ID,
		})
		return nil, fmt.Errorf("no OpenAPI spec URL found in tool configuration")
	}

	// Try cache first
	spec, err := a.specCache.Get(ctx, specURL)
	if err == nil {
		a.logger.Debug("Loaded OpenAPI spec from cache", map[string]interface{}{
			"tool_name": a.tool.ToolName,
			"spec_url":  specURL,
		})
		return spec, nil
	}

	a.logger.Info("Spec not in cache, fetching", map[string]interface{}{
		"tool_name": a.tool.ToolName,
		"spec_url":  specURL,
		"cache_err": err.Error(),
	})

	// Implement retry logic with exponential backoff
	var lastErr error
	maxRetries := 3
	baseDelay := time.Second

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<uint(attempt-1))
			a.logger.Debug("Retrying spec fetch", map[string]interface{}{
				"attempt": attempt + 1,
				"delay":   delay.String(),
			})
			time.Sleep(delay)
		}

		// Create request with timeout
		reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "GET", specURL, nil)
		if err != nil {
			lastErr = err
			continue
		}

		resp, err := a.httpClient.Do(req)
		if err != nil {
			lastErr = err
			a.logger.Warn("Failed to fetch spec", map[string]interface{}{
				"attempt": attempt + 1,
				"error":   err.Error(),
			})
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("HTTP %d", resp.StatusCode)
			a.logger.Warn("Spec fetch returned error status", map[string]interface{}{
				"status_code": resp.StatusCode,
				"attempt":     attempt + 1,
			})
			continue
		}

		// Read response body with size limit for large specs
		limitedReader := io.LimitReader(resp.Body, 10*1024*1024) // 10MB limit
		bodyData, err := io.ReadAll(limitedReader)
		if err != nil {
			lastErr = fmt.Errorf("failed to read spec: %w", err)
			continue
		}

		// Parse spec
		loader := openapi3.NewLoader()
		loader.IsExternalRefsAllowed = true
		spec, err = loader.LoadFromData(bodyData)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse spec: %w", err)
			a.logger.Warn("Failed to parse OpenAPI spec", map[string]interface{}{
				"attempt": attempt + 1,
				"error":   err.Error(),
				"size":    len(bodyData),
			})
			// Even if parsing fails, try to cache raw data
			// Some operations might still work
			continue
		}

		// Success! Cache the spec
		if err := a.specCache.Set(ctx, specURL, spec, 24*time.Hour); err != nil {
			a.logger.Warn("Failed to cache OpenAPI spec", map[string]interface{}{
				"url":   specURL,
				"error": err.Error(),
			})
		}

		a.logger.Info("Successfully fetched and cached OpenAPI spec", map[string]interface{}{
			"tool_name":   a.tool.ToolName,
			"spec_url":    specURL,
			"paths_count": len(spec.Paths.Map()),
		})

		// Create router for operation lookup
		router, err := gorillamux.NewRouter(spec)
		if err == nil {
			a.router = router
		}

		return spec, nil
	}

	// All retries failed
	a.logger.Error("Failed to fetch OpenAPI spec after retries", map[string]interface{}{
		"tool_name": a.tool.ToolName,
		"spec_url":  specURL,
		"attempts":  maxRetries,
		"last_err":  lastErr,
	})

	return nil, fmt.Errorf("failed to fetch OpenAPI spec after %d attempts: %w", maxRetries, lastErr)
}

// findOperation finds an operation by ID or path/method
func (a *DynamicToolAdapter) findOperation(spec *openapi3.T, actionID string) (*openapi3.Operation, string, string, error) {
	// Log the search attempt
	a.logger.Debug("Searching for operation", map[string]interface{}{
		"action_id": actionID,
		"tool_name": a.tool.ToolName,
	})

	// Normalize action ID - handle both slash and hyphen formats
	// e.g., "repos/get-content" or "repos-get-content"
	normalizedID := strings.ReplaceAll(actionID, "/", "-")
	alternativeID := strings.ReplaceAll(actionID, "-", "/")

	// First try by operation ID (exact match and normalized variants)
	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				if operation != nil && operation.OperationID != "" {
					// Try exact match
					if operation.OperationID == actionID {
						a.logger.Debug("Found operation by exact ID", map[string]interface{}{
							"operation_id": operation.OperationID,
							"path":         path,
							"method":       method,
						})
						return operation, path, method, nil
					}
					// Try normalized match
					if operation.OperationID == normalizedID || operation.OperationID == alternativeID {
						a.logger.Debug("Found operation by normalized ID", map[string]interface{}{
							"operation_id": operation.OperationID,
							"path":         path,
							"method":       method,
						})
						return operation, path, method, nil
					}
				}
			}
		}
	}

	// Try parsing as method_path format
	parts := strings.SplitN(actionID, "_", 2)
	if len(parts) == 2 {
		method := strings.ToUpper(parts[0])
		path := "/" + strings.ReplaceAll(parts[1], "_", "/")

		if spec.Paths != nil {
			if pathItem := spec.Paths.Find(path); pathItem != nil {
				if operation := pathItem.GetOperation(method); operation != nil {
					a.logger.Debug("Found operation by method_path format", map[string]interface{}{
						"path":   path,
						"method": method,
					})
					return operation, path, method, nil
				}
			}
		}
	}

	// Log available operations for debugging
	availableOps := []string{}
	if spec.Paths != nil {
		for _, pathItem := range spec.Paths.Map() {
			for _, operation := range pathItem.Operations() {
				if operation != nil && operation.OperationID != "" {
					availableOps = append(availableOps, operation.OperationID)
					if len(availableOps) >= 10 {
						break // Limit logging to first 10
					}
				}
			}
		}
	}

	a.logger.Error("Operation not found", map[string]interface{}{
		"action_id":   actionID,
		"tool_name":   a.tool.ToolName,
		"total_paths": len(spec.Paths.Map()),
		"sample_ops":  availableOps,
	})

	return nil, "", "", fmt.Errorf("operation not found: %s", actionID)
}

// buildRequest builds an HTTP request from OpenAPI operation
func (a *DynamicToolAdapter) buildRequest(
	ctx context.Context,
	spec *openapi3.T,
	operation *openapi3.Operation,
	path string,
	method string,
	params map[string]interface{},
) (*http.Request, error) {
	// Get base URL
	baseURL := a.tool.BaseURL
	if len(spec.Servers) > 0 {
		baseURL = spec.Servers[0].URL
	}

	// Build URL with path parameters
	urlPath := path
	queryParams := url.Values{}
	headers := http.Header{}

	// Process parameters
	for _, paramRef := range operation.Parameters {
		if paramRef.Value == nil {
			continue
		}
		param := paramRef.Value

		value, exists := params[param.Name]
		if !exists && param.Required {
			return nil, fmt.Errorf("required parameter missing: %s", param.Name)
		}

		if exists {
			switch param.In {
			case "path":
				urlPath = strings.ReplaceAll(urlPath, "{"+param.Name+"}", fmt.Sprintf("%v", value))
			case "query":
				queryParams.Set(param.Name, fmt.Sprintf("%v", value))
			case "header":
				headers.Set(param.Name, fmt.Sprintf("%v", value))
			}
		}
	}

	// Build full URL
	fullURL := baseURL + urlPath
	if len(queryParams) > 0 {
		fullURL += "?" + queryParams.Encode()
	}

	// Handle request body
	var body io.Reader
	if operation.RequestBody != nil && params["body"] != nil {
		bodyData, err := json.Marshal(params["body"])
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(bodyData)
		headers.Set("Content-Type", "application/json")
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, method, fullURL, body)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header = headers

	return req, nil
}

// applyAuthentication applies authentication to the request
func (a *DynamicToolAdapter) applyAuthentication(req *http.Request) error {
	// Decrypt credentials if encrypted
	var creds *models.TokenCredential

	if len(a.tool.CredentialsEncrypted) > 0 {
		// Use DecryptJSON since credentials were encrypted with EncryptJSON
		creds = &models.TokenCredential{}
		err := a.encryptionSvc.DecryptJSON(string(a.tool.CredentialsEncrypted), a.tool.TenantID, creds)
		if err != nil {
			return fmt.Errorf("failed to decrypt credentials: %w", err)
		}
	}

	// Apply authentication
	if creds != nil {
		if err := a.authenticator.ApplyAuthentication(req, creds); err != nil {
			return err
		}
	}

	return nil
}

// convertParameter converts OpenAPI parameter to action parameter
func (a *DynamicToolAdapter) convertParameter(param *openapi3.Parameter) models.ActionParameter {
	actionParam := models.ActionParameter{
		Name:        param.Name,
		In:          param.In,
		Required:    param.Required,
		Description: param.Description,
	}

	if param.Schema != nil && param.Schema.Value != nil {
		// Handle type which might be a string or array of strings
		if param.Schema.Value.Type != nil {
			if len(*param.Schema.Value.Type) > 0 {
				actionParam.Type = (*param.Schema.Value.Type)[0]
			}
		}
		if param.Schema.Value.Default != nil {
			actionParam.Default = param.Schema.Value.Default
		}
	}

	return actionParam
}
