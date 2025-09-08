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
	tool                 *models.DynamicTool
	specCache            repository.OpenAPICacheRepository
	httpClient           *http.Client
	authenticator        *tools.DynamicAuthenticator
	encryptionSvc        *security.EncryptionService
	logger               observability.Logger
	router               routers.Router
	operationResolver    *tools.OperationResolver
	permissionDiscoverer *tools.PermissionDiscoverer
	resourceResolver     *tools.ResourceScopeResolver
	allowedOperations    map[string]bool      // Cache of allowed operations based on permissions
	resourceScope        *tools.ResourceScope // Resource scope for this tool
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

	// Create resource scope resolver
	resourceResolver := tools.NewResourceScopeResolver(logger)

	// Extract resource scope from tool name
	resourceScope := resourceResolver.ExtractResourceScopeFromToolName(tool.ToolName)

	return &DynamicToolAdapter{
		tool:                 tool,
		specCache:            specCache,
		httpClient:           httpClient,
		authenticator:        tools.NewDynamicAuthenticator(),
		encryptionSvc:        encryptionSvc,
		logger:               logger,
		operationResolver:    tools.NewOperationResolver(logger),
		permissionDiscoverer: tools.NewPermissionDiscoverer(logger),
		resourceResolver:     resourceResolver,
		allowedOperations:    make(map[string]bool),
		resourceScope:        resourceScope,
	}, nil
}

// ListActions returns available actions from the OpenAPI spec
func (a *DynamicToolAdapter) ListActions(ctx context.Context) ([]models.ToolAction, error) {
	// Get the OpenAPI spec
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	// STEP 1: Apply resource scope filtering FIRST
	// This ensures each tool only sees operations relevant to its resource type
	var operationsToConsider map[string]*openapi3.Operation
	if a.resourceResolver != nil && a.resourceScope != nil {
		operationsToConsider = a.resourceResolver.FilterOperationsByScope(spec, a.resourceScope)
		a.logger.Info("Applied resource scope filtering", map[string]interface{}{
			"tool_name":      a.tool.ToolName,
			"resource_type":  a.resourceScope.ResourceType,
			"filtered_count": len(operationsToConsider),
		})
	} else {
		// No resource scope, get all operations
		operationsToConsider = a.getAllOperationsFromSpec(spec)
	}

	var actions []models.ToolAction

	// Iterate through resource-scoped operations
	for opID, operation := range operationsToConsider {
		// Find the path and method for this operation
		path, method := a.findPathAndMethod(spec, operation)
		if path == "" || method == "" {
			continue
		}

		// Create action ID - use simplified name if resource scope is defined
		actionID := opID
		if a.resourceScope != nil && a.resourceResolver != nil {
			// Use simplified action name for better AI agent usability
			actionID = a.resourceResolver.GetSimplifiedActionName(operation, a.resourceScope.ResourceType)
		}

		// STEP 2: Check if this operation is allowed based on discovered permissions
		// If we have permission info, filter. Otherwise, include all.
		if len(a.allowedOperations) > 0 {
			allowed, exists := a.allowedOperations[opID]
			if exists && !allowed {
				a.logger.Debug("Filtering out operation due to permissions", map[string]interface{}{
					"operation_id": opID,
					"action_id":    actionID,
					"path":         path,
					"method":       method,
				})
				continue
			}
		}

		// Extract parameters
		var parameters []models.ActionParameter

		// Get pathItem for parameter extraction
		pathItem := spec.Paths.Find(path)
		if pathItem != nil {
			// Path parameters
			for _, param := range pathItem.Parameters {
				if param.Value != nil {
					parameters = append(parameters, a.convertParameter(param.Value))
				}
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

	// Find the operation with parameter context for better resolution
	operation, path, method, err := a.findOperationWithContext(spec, actionID, params)
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

	// If we're using passthrough auth, we should also discover permissions with it
	if a.permissionDiscoverer != nil && passthroughAuth != nil {
		a.discoverPassthroughPermissions(ctx, passthroughAuth, passthroughConfig)
	}

	// Get the OpenAPI spec
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	// Find the operation with parameter context for better resolution
	operation, path, method, err := a.findOperationWithContext(spec, actionID, params)
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

		// Build operation mappings for intelligent resolution (even for cached specs)
		// IMPORTANT: Only build mappings for resource-scoped operations
		if a.operationResolver != nil {
			a.logger.Info("Building operation mappings from cached spec", map[string]interface{}{
				"tool_name": a.tool.ToolName,
			})

			// If we have a resource scope, only build mappings for those operations
			if a.resourceResolver != nil && a.resourceScope != nil {
				scopedOps := a.resourceResolver.FilterOperationsByScope(spec, a.resourceScope)
				// Build a temporary spec with only scoped operations
				scopedSpec := &openapi3.T{
					Paths: openapi3.NewPaths(),
				}
				for _, operation := range scopedOps {
					path, method := a.findPathAndMethod(spec, operation)
					if path != "" && method != "" {
						pathItem := scopedSpec.Paths.Find(path)
						if pathItem == nil {
							pathItem = &openapi3.PathItem{}
							scopedSpec.Paths.Set(path, pathItem)
						}
						pathItem.SetOperation(method, operation)
					}
				}
				err = a.operationResolver.BuildOperationMappings(scopedSpec, a.tool.ToolName)
			} else {
				err = a.operationResolver.BuildOperationMappings(spec, a.tool.ToolName)
			}

			if err != nil {
				a.logger.Warn("Failed to build operation mappings from cached spec", map[string]interface{}{
					"error": err.Error(),
				})
			} else {
				a.logger.Info("Successfully built operation mappings from cached spec", map[string]interface{}{
					"tool_name": a.tool.ToolName,
				})
			}
		} else {
			a.logger.Warn("OperationResolver is nil", map[string]interface{}{
				"tool_name": a.tool.ToolName,
			})
		}

		// Create router for operation lookup (even for cached specs)
		router, err := gorillamux.NewRouter(spec)
		if err == nil {
			a.router = router
		}

		// Discover and cache permissions for filtering (if credentials are available)
		a.discoverAndCachePermissions(ctx, spec)

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
		// Increased to 50MB to handle large OpenAPI specs like GitHub's (11.45MB)
		limitedReader := io.LimitReader(resp.Body, 50*1024*1024) // 50MB limit
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

		// Build operation mappings for intelligent resolution
		// IMPORTANT: Only build mappings for resource-scoped operations
		if a.operationResolver != nil {
			// If we have a resource scope, only build mappings for those operations
			if a.resourceResolver != nil && a.resourceScope != nil {
				scopedOps := a.resourceResolver.FilterOperationsByScope(spec, a.resourceScope)
				// Build a temporary spec with only scoped operations
				scopedSpec := &openapi3.T{
					Paths: openapi3.NewPaths(),
				}
				for _, operation := range scopedOps {
					path, method := a.findPathAndMethod(spec, operation)
					if path != "" && method != "" {
						pathItem := scopedSpec.Paths.Find(path)
						if pathItem == nil {
							pathItem = &openapi3.PathItem{}
							scopedSpec.Paths.Set(path, pathItem)
						}
						pathItem.SetOperation(method, operation)
					}
				}
				err = a.operationResolver.BuildOperationMappings(scopedSpec, a.tool.ToolName)
			} else {
				err = a.operationResolver.BuildOperationMappings(spec, a.tool.ToolName)
			}

			if err != nil {
				a.logger.Warn("Failed to build operation mappings", map[string]interface{}{
					"error": err.Error(),
				})
			}
		}

		// Create router for operation lookup
		router, err := gorillamux.NewRouter(spec)
		if err == nil {
			a.router = router
		}

		// Discover and cache permissions for filtering (if credentials are available)
		a.discoverAndCachePermissions(ctx, spec)

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

// findOperationWithContext finds an operation by ID or path/method with parameter context
func (a *DynamicToolAdapter) findOperationWithContext(spec *openapi3.T, actionID string, params map[string]interface{}) (*openapi3.Operation, string, string, error) {
	return a.findOperation(spec, actionID, params)
}

// findOperation finds an operation by ID or path/method
func (a *DynamicToolAdapter) findOperation(spec *openapi3.T, actionID string, context map[string]interface{}) (*openapi3.Operation, string, string, error) {
	// Log the search attempt
	a.logger.Debug("Searching for operation", map[string]interface{}{
		"action_id": actionID,
		"tool_name": a.tool.ToolName,
	})

	// Check direct mapping first for known problematic cases
	// This bypasses the OperationResolver's transformations
	// Now all operations come with consistent hyphen formatting
	directMap := map[string]string{
		"actions-list-repo-workflows":      "actions/list-repo-workflows",
		"actions-create-workflow-dispatch": "actions/create-workflow-dispatch",
	}

	a.logger.Info("Checking direct mapping", map[string]interface{}{
		"action_id": actionID,
		"has_match": directMap[actionID] != "",
	})

	if mapped, ok := directMap[actionID]; ok {
		// Try the mapped operation ID
		if spec.Paths != nil {
			for path, pathItem := range spec.Paths.Map() {
				for method, operation := range pathItem.Operations() {
					if operation != nil && operation.OperationID == mapped {
						a.logger.Debug("Found operation by direct mapping", map[string]interface{}{
							"original": actionID,
							"mapped":   mapped,
							"path":     path,
							"method":   method,
						})
						return operation, path, method, nil
					}
				}
			}
		}
	}

	// Use the OperationResolver for intelligent operation resolution
	if a.operationResolver != nil {
		// Add resource type to context if we have it
		// This helps the resolver prioritize operations from the correct resource
		if a.resourceScope != nil && a.resourceScope.ResourceType != "" {
			if context == nil {
				context = make(map[string]interface{})
			}
			context["__resource_type"] = a.resourceScope.ResourceType
		}

		a.logger.Info("Using OperationResolver", map[string]interface{}{
			"action_id": actionID,
			"context":   context,
		})

		// Use the provided context (parameters) for better resolution
		// This helps the resolver understand what operation is being requested
		// based on the parameters provided (e.g., "owner", "repo" suggests GitHub operations)

		// Try to resolve the operation
		resolvedOp, err := a.operationResolver.ResolveOperation(actionID, context)
		if resolvedOp != nil {
			a.logger.Info("OperationResolver result", map[string]interface{}{
				"action_id":    actionID,
				"resolved":     true,
				"operation_id": resolvedOp.OperationID,
				"path":         resolvedOp.Path,
				"method":       resolvedOp.Method,
				"error":        err,
			})
		} else {
			a.logger.Info("OperationResolver result", map[string]interface{}{
				"action_id": actionID,
				"resolved":  false,
				"error":     err,
			})
		}
		if err == nil && resolvedOp != nil {
			// Find the actual operation in the spec
			if spec.Paths != nil {
				for path, pathItem := range spec.Paths.Map() {
					for method, operation := range pathItem.Operations() {
						if operation != nil && operation.OperationID == resolvedOp.OperationID {
							a.logger.Debug("Found operation via resolver", map[string]interface{}{
								"action_id":    actionID,
								"operation_id": resolvedOp.OperationID,
								"path":         path,
								"method":       method,
							})
							return operation, path, method, nil
						}
					}
				}
			}
		}
	}

	// Try exact match first
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

		// Try to find the parameter value
		// First check direct params, then check nested in "parameters" map
		value, exists := params[param.Name]
		if !exists {
			// Check if parameter is nested in "parameters" map (common for MCP tools)
			if paramsMap, ok := params["parameters"].(map[string]interface{}); ok {
				value, exists = paramsMap[param.Name]
			}
		}

		if !exists && param.Required {
			return nil, fmt.Errorf("required parameter missing: %s", param.Name)
		}

		if exists {
			switch param.In {
			case "path":
				// For path parameters like {ref}, handle special encoding for Git refs
				pathValue := fmt.Sprintf("%v", value)
				// For Git refs, we need to URL encode but preserve the forward slashes
				if param.Name == "ref" && strings.HasPrefix(pathValue, "refs/") {
					// Remove "refs/" prefix for the path parameter
					pathValue = strings.TrimPrefix(pathValue, "refs/")
				}
				urlPath = strings.ReplaceAll(urlPath, "{"+param.Name+"}", pathValue)
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
	if operation.RequestBody != nil {
		// Collect all parameters that aren't path, query, or header parameters
		bodyParams := make(map[string]interface{})
		processedParams := make(map[string]bool)

		// Mark path, query, and header parameters as processed
		for _, paramRef := range operation.Parameters {
			if paramRef.Value != nil {
				processedParams[paramRef.Value.Name] = true
			}
		}

		// Special handling for GitHub-style tools from Edge MCP that have a nested "parameters" map
		// Only unwrap if we have specific GitHub tool indicators
		shouldUnwrapParameters := false
		if a.tool != nil && a.tool.ToolName != "" {
			// Check if this is a GitHub tool that uses the nested parameters pattern
			if strings.HasPrefix(a.tool.ToolName, "github_") || strings.Contains(a.tool.ToolName, "_github_") {
				// Check if we have a "parameters" field that needs unwrapping
				if _, hasParameters := params["parameters"]; hasParameters {
					shouldUnwrapParameters = true
				}
			}
		}

		if shouldUnwrapParameters {
			if parametersMap, exists := params["parameters"]; exists {
				if paramsAsMap, ok := parametersMap.(map[string]interface{}); ok {
					// Use the contents of the "parameters" map as the request body
					bodyParams = paramsAsMap
					processedParams["parameters"] = true
				}
			}
		} else {
			// Check if there's a "body" parameter that represents the entire request body
			if bodyParam, exists := params["body"]; exists {
				// If "body" exists and is a map, it might be the entire request body
				if bodyMap, ok := bodyParam.(map[string]interface{}); ok {
					// Use it as the base for bodyParams
					bodyParams = bodyMap
					processedParams["body"] = true
				}
			}

			// Add all unprocessed parameters to the request body
			// This handles both the case where "body" is a field in the request body
			// and the case where parameters should be collected into the request body
			for key, value := range params {
				if !processedParams[key] {
					bodyParams[key] = value
				}
			}
		}

		// Log the parameters being sent in the request body
		a.logger.Info("Building request body", map[string]interface{}{
			"method":         method,
			"path":           path,
			"params":         params,
			"bodyParams":     bodyParams,
			"processedCount": len(bodyParams),
		})

		// Only create body if we have parameters
		if len(bodyParams) > 0 {
			bodyData, err := json.Marshal(bodyParams)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal request body: %w", err)
			}
			body = bytes.NewReader(bodyData)
			headers.Set("Content-Type", "application/json")

			// Log the JSON being sent
			a.logger.Info("Request body JSON", map[string]interface{}{
				"json": string(bodyData),
			})
		}
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

// getAllOperationsFromSpec extracts all operations from the OpenAPI spec
func (a *DynamicToolAdapter) getAllOperationsFromSpec(spec *openapi3.T) map[string]*openapi3.Operation {
	operations := make(map[string]*openapi3.Operation)

	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				if operation != nil {
					opID := operation.OperationID
					if opID == "" {
						opID = fmt.Sprintf("%s_%s", strings.ToLower(method), strings.ReplaceAll(path, "/", "_"))
					}
					operations[opID] = operation
				}
			}
		}
	}

	return operations
}

// findPathAndMethod finds the path and method for a given operation
func (a *DynamicToolAdapter) findPathAndMethod(spec *openapi3.T, targetOp *openapi3.Operation) (string, string) {
	if spec.Paths != nil {
		for path, pathItem := range spec.Paths.Map() {
			for method, operation := range pathItem.Operations() {
				if operation == targetOp {
					return path, method
				}
			}
		}
	}
	return "", ""
}

// discoverAndCachePermissions discovers permissions for the current token and caches allowed operations
func (a *DynamicToolAdapter) discoverAndCachePermissions(ctx context.Context, spec *openapi3.T) {
	if a.permissionDiscoverer == nil {
		a.logger.Debug("Permission discoverer not initialized", map[string]interface{}{})
		return
	}

	// Only discover permissions if we have credentials
	if len(a.tool.CredentialsEncrypted) == 0 {
		a.logger.Debug("No credentials available for permission discovery", map[string]interface{}{})
		return
	}

	// Decrypt credentials to get the token
	creds := &models.TokenCredential{}
	err := a.encryptionSvc.DecryptJSON(string(a.tool.CredentialsEncrypted), a.tool.TenantID, creds)
	if err != nil {
		a.logger.Warn("Failed to decrypt credentials for permission discovery", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Get the token and auth type
	token := ""
	authType := "bearer" // default
	if creds.Token != "" {
		token = creds.Token
		authType = creds.Type
	} else if creds.Password != "" {
		// Some APIs use password field for API keys
		token = creds.Password
		authType = "api_key"
	}

	if token == "" {
		a.logger.Debug("No token available for permission discovery", map[string]interface{}{})
		return
	}

	// Discover permissions
	a.logger.Info("Discovering permissions for dynamic tool", map[string]interface{}{
		"tool_name": a.tool.ToolName,
		"base_url":  a.tool.BaseURL,
		"auth_type": authType,
	})

	discoveredPerms, err := a.permissionDiscoverer.DiscoverPermissions(ctx, a.tool.BaseURL, token, authType)
	if err != nil {
		a.logger.Warn("Failed to discover permissions", map[string]interface{}{
			"error": err.Error(),
		})
		// Don't filter if discovery fails - allow all operations
		return
	}

	// Log discovered permissions
	a.logger.Info("Discovered permissions", map[string]interface{}{
		"tool_name":    a.tool.ToolName,
		"scopes_count": len(discoveredPerms.Scopes),
		"scopes":       discoveredPerms.Scopes,
		"headers":      discoveredPerms.RawHeaders,
	})

	// Filter operations based on discovered permissions
	a.allowedOperations = a.permissionDiscoverer.FilterOperationsByPermissions(spec, discoveredPerms)

	// Log filtering results
	allowedCount := 0
	for _, allowed := range a.allowedOperations {
		if allowed {
			allowedCount++
		}
	}

	a.logger.Info("Permission-based filtering applied", map[string]interface{}{
		"tool_name":        a.tool.ToolName,
		"total_operations": len(a.allowedOperations),
		"allowed_count":    allowedCount,
	})
}

// DiscoverAndUpdatePermissions allows external callers to trigger permission discovery
// This is useful when credentials change or for testing
func (a *DynamicToolAdapter) DiscoverAndUpdatePermissions(ctx context.Context) error {
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		return fmt.Errorf("failed to get OpenAPI spec: %w", err)
	}

	a.discoverAndCachePermissions(ctx, spec)
	return nil
}

// discoverPassthroughPermissions discovers permissions using passthrough credentials
func (a *DynamicToolAdapter) discoverPassthroughPermissions(
	ctx context.Context,
	passthroughAuth *models.PassthroughAuthBundle,
	passthroughConfig *models.EnhancedPassthroughConfig,
) {
	if a.permissionDiscoverer == nil {
		return
	}

	// Get the spec first
	spec, err := a.getOpenAPISpec(ctx)
	if err != nil {
		a.logger.Warn("Failed to get spec for passthrough permission discovery", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Extract token from passthrough bundle
	token := ""
	authType := "bearer"

	// Check for tool-specific credential
	if cred := passthroughAuth.GetCredentialForTool(a.tool.ToolName); cred != nil {
		token = cred.Token
		if token == "" {
			token = cred.KeyValue
		}
		authType = cred.Type
	} else if oauthToken := passthroughAuth.GetOAuthTokenForTool(a.tool.ToolName); oauthToken != nil {
		token = oauthToken.AccessToken
		authType = "oauth2"
	} else if sessionToken, ok := passthroughAuth.SessionTokens[a.tool.ToolName]; ok {
		token = sessionToken
		authType = "session"
	}

	if token == "" {
		a.logger.Debug("No passthrough token available for permission discovery", map[string]interface{}{})
		return
	}

	// Discover permissions with passthrough token
	a.logger.Info("Discovering permissions with passthrough auth", map[string]interface{}{
		"tool_name": a.tool.ToolName,
		"auth_type": authType,
	})

	discoveredPerms, err := a.permissionDiscoverer.DiscoverPermissions(ctx, a.tool.BaseURL, token, authType)
	if err != nil {
		a.logger.Warn("Failed to discover passthrough permissions", map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	// Update allowed operations
	a.allowedOperations = a.permissionDiscoverer.FilterOperationsByPermissions(spec, discoveredPerms)

	allowedCount := 0
	for _, allowed := range a.allowedOperations {
		if allowed {
			allowedCount++
		}
	}

	a.logger.Info("Passthrough permission filtering applied", map[string]interface{}{
		"tool_name":        a.tool.ToolName,
		"total_operations": len(a.allowedOperations),
		"allowed_count":    allowedCount,
		"scopes":           discoveredPerms.Scopes,
	})
}
