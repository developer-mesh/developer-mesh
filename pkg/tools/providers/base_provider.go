package providers

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

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// BaseProvider provides common functionality for all providers
type BaseProvider struct {
	name              string
	version           string
	baseURL           string
	httpClient        *http.Client
	logger            observability.Logger
	config            ProviderConfig
	operationMappings map[string]OperationMapping
}

// NewBaseProvider creates a new base provider
func NewBaseProvider(name, version, baseURL string, logger observability.Logger) *BaseProvider {
	return &BaseProvider{
		name:    name,
		version: version,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger:            logger,
		operationMappings: make(map[string]OperationMapping),
	}
}

// GetProviderName returns the provider name
func (p *BaseProvider) GetProviderName() string {
	return p.name
}

// GetSupportedVersions returns supported API versions
func (p *BaseProvider) GetSupportedVersions() []string {
	return []string{p.version}
}

// GetDefaultConfiguration returns the default configuration
func (p *BaseProvider) GetDefaultConfiguration() ProviderConfig {
	return p.config
}

// SetConfiguration sets the provider configuration
func (p *BaseProvider) SetConfiguration(config ProviderConfig) {
	p.config = config
	if config.BaseURL != "" {
		p.baseURL = config.BaseURL
	}
	if config.Timeout > 0 {
		p.httpClient.Timeout = config.Timeout
	}
}

// SetOperationMappings sets the operation mappings for the provider
func (p *BaseProvider) SetOperationMappings(mappings map[string]OperationMapping) {
	p.operationMappings = mappings
}

// GetLogger returns the logger instance
func (p *BaseProvider) GetLogger() observability.Logger {
	return p.logger
}

// HealthCheck performs a basic health check
func (p *BaseProvider) HealthCheck(ctx context.Context) error {
	healthEndpoint := p.config.HealthEndpoint
	if healthEndpoint == "" {
		// Default health check - just try to reach the base URL
		healthEndpoint = p.baseURL
	}

	req, err := http.NewRequestWithContext(ctx, "GET", healthEndpoint, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode >= 500 {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}

// Close cleans up resources
func (p *BaseProvider) Close() error {
	// Base implementation doesn't have resources to clean up
	return nil
}

// Execute executes an operation using the operation mappings
func (p *BaseProvider) Execute(ctx context.Context, operation string, params map[string]interface{}) (interface{}, error) {
	mapping, exists := p.operationMappings[operation]
	if !exists {
		return nil, fmt.Errorf("operation %s not found", operation)
	}

	// Build path with parameters
	path := mapping.PathTemplate
	queryParams := make(map[string]string)

	// For Harness provider, try to extract accountIdentifier from PAT token if missing
	if p.name == "harness" {
		if _, hasAccount := params["accountIdentifier"]; !hasAccount {
			// Try to extract from context credentials
			if pctx, ok := FromContext(ctx); ok && pctx.Credentials != nil {
				apiKey := pctx.Credentials.APIKey
				if apiKey == "" {
					apiKey = pctx.Credentials.Token
				}
				// Check if it's a PAT token format: pat.ACCOUNT_ID.xxx
				if strings.HasPrefix(apiKey, "pat.") {
					parts := strings.Split(apiKey, ".")
					if len(parts) >= 3 {
						params["accountIdentifier"] = parts[1]
						if p.logger != nil {
							p.logger.Debug("Extracted account ID from PAT token", map[string]interface{}{
								"token_format": "pat.***",
							})
						}
					}
				}
			}
		}
	}

	// Replace path parameters
	for _, param := range mapping.RequiredParams {
		placeholder := "{" + param + "}"
		if value, ok := params[param]; ok {
			if strings.Contains(path, placeholder) {
				path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", value))
			}
		} else if strings.Contains(path, placeholder) {
			// For Harness, provide sensible defaults for common parameters
			switch param {
			case "org":
				// Try "default" as a fallback for org
				path = strings.ReplaceAll(path, placeholder, "default")
				params[param] = "default" // Add to params for body if needed
			case "project":
				// Try "default" as a fallback for project
				path = strings.ReplaceAll(path, placeholder, "default")
				params[param] = "default" // Add to params for body if needed
			default:
				// Return error for other missing required params
				return nil, fmt.Errorf("required parameter '%s' not provided for operation", param)
			}
		}
	}

	// For GET requests, collect query parameters
	if mapping.Method == "GET" || mapping.Method == "HEAD" {
		// Add optional parameters as query params
		for _, param := range mapping.OptionalParams {
			if value, ok := params[param]; ok {
				queryParams[param] = fmt.Sprintf("%v", value)
			}
		}

		// Also check for common pagination parameters even if not in OptionalParams
		for _, param := range []string{"per_page", "page", "limit", "offset", "sort", "direction"} {
			if value, ok := params[param]; ok {
				queryParams[param] = fmt.Sprintf("%v", value)
			}
		}

		// Build query string with proper URL encoding
		if len(queryParams) > 0 {
			values := url.Values{}
			for k, v := range queryParams {
				values.Add(k, v)
			}
			path = path + "?" + values.Encode()
		}
	}

	// Prepare body for POST/PUT/PATCH methods
	var body interface{}
	if mapping.Method == "POST" || mapping.Method == "PUT" || mapping.Method == "PATCH" {
		// Special handling for certain endpoints that need specific body structures
		if strings.Contains(mapping.PathTemplate, "/recommendation/overview/list") {
			// CCM recommendations endpoint expects a CCMRecommendationFilterProperties object
			// Send a properly structured filter to get all recommendations
			body = map[string]interface{}{
				"perspectiveFilters": []interface{}{},
			}
			// accountIdentifier is a required query parameter for this endpoint
			if accountID, ok := params["accountIdentifier"]; ok {
				queryParams["accountIdentifier"] = fmt.Sprintf("%v", accountID)
				// Remove from body params since it goes in query
				delete(params, "accountIdentifier")
			} else {
				// Check if it's in the required params list
				for _, reqParam := range mapping.RequiredParams {
					if reqParam == "accountIdentifier" {
						return nil, fmt.Errorf("accountIdentifier is required for CCM recommendations")
					}
				}
			}
		} else if strings.Contains(mapping.PathTemplate, "/anomaly") {
			// CCM anomalies endpoint also expects a specific structure
			body = map[string]interface{}{}
			if accountID, ok := params["accountIdentifier"]; ok {
				queryParams["accountIdentifier"] = fmt.Sprintf("%v", accountID)
			}
		} else {
			// Default behavior for other endpoints
			body = params
		}

		// Build query string for POST requests that need query params
		if len(queryParams) > 0 {
			values := url.Values{}
			for k, v := range queryParams {
				values.Add(k, v)
			}
			path = path + "?" + values.Encode()
		}
	}

	// Execute HTTP request
	resp, err := p.ExecuteHTTPRequest(ctx, mapping.Method, path, body, nil)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	// Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check for non-2xx status codes
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		// Try to parse error response
		var errorResult map[string]interface{}
		if err := json.Unmarshal(responseBody, &errorResult); err != nil {
			// If not JSON, return the raw response
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(responseBody))
		}
		// Return the parsed error
		return errorResult, nil
	}

	// Parse JSON response
	var result interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		// Check if it's an HTML error page
		bodyStr := string(responseBody)
		if strings.Contains(bodyStr, "<html") || strings.Contains(bodyStr, "<!DOCTYPE") {
			return nil, fmt.Errorf("received HTML response instead of JSON (status %d)", resp.StatusCode)
		}
		// For other parse errors, include a snippet of the response
		snippet := bodyStr
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		return nil, fmt.Errorf("failed to parse response: %w (response: %s)", err, snippet)
	}

	return result, nil
}

// ExecuteHTTPRequest executes an HTTP request with authentication and error handling
func (p *BaseProvider) ExecuteHTTPRequest(ctx context.Context, method, path string, body interface{}, headers map[string]string) (*http.Response, error) {
	url := p.buildURL(path)

	var bodyReader io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set default headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Apply default headers from config
	for k, v := range p.config.DefaultHeaders {
		req.Header.Set(k, v)
	}

	// Apply request-specific headers
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Apply authentication
	if err := p.applyAuthentication(ctx, req); err != nil {
		return nil, fmt.Errorf("failed to apply authentication: %w", err)
	}

	// Execute request with retry logic if configured
	if p.config.RetryPolicy != nil {
		return p.executeWithRetry(ctx, req)
	}

	return p.httpClient.Do(req)
}

// buildURL constructs the full URL from base URL and path
func (p *BaseProvider) buildURL(path string) string {
	baseURL := strings.TrimRight(p.baseURL, "/")
	path = strings.TrimLeft(path, "/")
	return fmt.Sprintf("%s/%s", baseURL, path)
}

// applyAuthentication applies authentication to the request
func (p *BaseProvider) applyAuthentication(ctx context.Context, req *http.Request) error {
	pctx, ok := FromContext(ctx)
	if !ok || pctx.Credentials == nil {
		if p.logger != nil {
			p.logger.Error("No credentials in context", map[string]interface{}{
				"has_context": ok,
				"context_nil": pctx == nil,
				"provider":    p.name,
				"auth_type":   p.config.AuthType,
			})
		}
		return fmt.Errorf("no credentials found in context")
	}

	// Log credential status
	if p.logger != nil {
		p.logger.Info("Applying authentication", map[string]interface{}{
			"provider":     p.name,
			"auth_type":    p.config.AuthType,
			"has_token":    pctx.Credentials.Token != "",
			"token_len":    len(pctx.Credentials.Token),
			"has_api_key":  pctx.Credentials.APIKey != "",
			"has_username": pctx.Credentials.Username != "",
			"url":          req.URL.String(),
		})
	}

	switch p.config.AuthType {
	case "bearer":
		if pctx.Credentials.Token != "" {
			authHeader := "Bearer " + pctx.Credentials.Token
			req.Header.Set("Authorization", authHeader)
			// Log more details (without exposing full token)
			if p.logger != nil {
				tokenPreview := ""
				if len(pctx.Credentials.Token) > 10 {
					tokenPreview = pctx.Credentials.Token[:10] + "..."
				}
				p.logger.Info("Applied bearer token auth", map[string]interface{}{
					"provider":      p.name,
					"token_len":     len(pctx.Credentials.Token),
					"token_preview": tokenPreview,
					"auth_len":      len(authHeader),
					"url":           req.URL.String(),
					"method":        req.Method,
				})
			}
		} else if pctx.Credentials.APIKey != "" {
			req.Header.Set("Authorization", "Bearer "+pctx.Credentials.APIKey)
			if p.logger != nil {
				p.logger.Debug("Applied bearer API key auth", map[string]interface{}{
					"key_len": len(pctx.Credentials.APIKey),
					"url":     req.URL.String(),
				})
			}
		} else {
			return fmt.Errorf("bearer token required but not provided")
		}
	case "api_key":
		if pctx.Credentials.APIKey != "" {
			// Some APIs use X-API-Key, others use different headers
			switch p.name {
			case "harness":
				// Harness uses lowercase x-api-key
				req.Header.Set("x-api-key", pctx.Credentials.APIKey)
			case "nexus":
				// Nexus uses NX-APIKEY
				req.Header.Set("Authorization", "NX-APIKEY "+pctx.Credentials.APIKey)
			default:
				req.Header.Set("X-API-Key", pctx.Credentials.APIKey)
			}
		} else {
			return fmt.Errorf("API key required but not provided")
		}
	case "basic":
		if pctx.Credentials.Username != "" && pctx.Credentials.Password != "" {
			req.SetBasicAuth(pctx.Credentials.Username, pctx.Credentials.Password)
		} else {
			return fmt.Errorf("username and password required but not provided")
		}
	case "oauth2":
		if pctx.Credentials.AccessToken != "" {
			req.Header.Set("Authorization", "Bearer "+pctx.Credentials.AccessToken)
		} else {
			return fmt.Errorf("OAuth2 access token required but not provided")
		}
	default:
		// No authentication or custom authentication
		return nil
	}

	return nil
}

// executeWithRetry executes a request with retry logic
func (p *BaseProvider) executeWithRetry(ctx context.Context, req *http.Request) (*http.Response, error) {
	policy := p.config.RetryPolicy
	if policy == nil {
		return p.httpClient.Do(req)
	}

	var lastErr error
	delay := policy.InitialDelay

	for attempt := 0; attempt <= policy.MaxRetries; attempt++ {
		if attempt > 0 {
			// Wait before retry
			select {
			case <-time.After(delay):
			case <-ctx.Done():
				return nil, ctx.Err()
			}

			// Increase delay for next attempt
			delay = time.Duration(float64(delay) * policy.Multiplier)
			if delay > policy.MaxDelay {
				delay = policy.MaxDelay
			}
		}

		// Clone the request for retry
		reqClone := req.Clone(ctx)
		if req.Body != nil {
			// Re-read body if necessary
			if seeker, ok := req.Body.(io.Seeker); ok {
				_, err := seeker.Seek(0, io.SeekStart)
				if err != nil {
					return nil, fmt.Errorf("failed to reset request body: %w", err)
				}
			}
		}

		resp, err := p.httpClient.Do(reqClone)
		if err != nil {
			lastErr = err
			if !policy.RetryOnTimeout {
				return nil, err
			}
			continue
		}

		// Check if we should retry based on status code
		if !p.shouldRetry(resp, policy) {
			return resp, nil
		}

		_ = resp.Body.Close()
		lastErr = fmt.Errorf("request failed with status %d", resp.StatusCode)
	}

	return nil, fmt.Errorf("request failed after %d retries: %w", policy.MaxRetries, lastErr)
}

// shouldRetry determines if a request should be retried based on the response
func (p *BaseProvider) shouldRetry(resp *http.Response, policy *RetryPolicy) bool {
	// Retry on rate limit if configured
	if policy.RetryOnRateLimit && resp.StatusCode == 429 {
		return true
	}

	// Retry on server errors (5xx)
	if resp.StatusCode >= 500 && resp.StatusCode < 600 {
		return true
	}

	// Check specific error codes
	for _, code := range policy.RetryableErrors {
		if fmt.Sprintf("%d", resp.StatusCode) == code {
			return true
		}
	}

	return false
}

// ParseJSONResponse parses a JSON response from the provider
func (p *BaseProvider) ParseJSONResponse(resp *http.Response, target interface{}) error {
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return &ProviderError{
			Provider:    p.name,
			Code:        fmt.Sprintf("HTTP_%d", resp.StatusCode),
			Message:     fmt.Sprintf("request failed with status %d: %s", resp.StatusCode, string(body)),
			StatusCode:  resp.StatusCode,
			IsRetryable: resp.StatusCode >= 500 || resp.StatusCode == 429,
		}
	}

	if target != nil {
		decoder := json.NewDecoder(resp.Body)
		if err := decoder.Decode(target); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// BuildPath builds a path with parameter substitution
func (p *BaseProvider) BuildPath(template string, params map[string]interface{}) string {
	path := template
	for key, value := range params {
		placeholder := fmt.Sprintf("{%s}", key)
		path = strings.ReplaceAll(path, placeholder, fmt.Sprintf("%v", value))
	}
	return path
}

// ExtractPathParams extracts path parameters from the params map
func (p *BaseProvider) ExtractPathParams(template string, params map[string]interface{}) (pathParams map[string]string, remainingParams map[string]interface{}) {
	pathParams = make(map[string]string)
	remainingParams = make(map[string]interface{})

	// Find all placeholders in the template
	placeholders := make(map[string]bool)
	for i := 0; i < len(template); i++ {
		if template[i] == '{' {
			end := strings.Index(template[i:], "}")
			if end > 0 {
				placeholder := template[i+1 : i+end]
				placeholders[placeholder] = true
			}
		}
	}

	// Separate path params from other params
	for key, value := range params {
		if placeholders[key] {
			pathParams[key] = fmt.Sprintf("%v", value)
		} else {
			remainingParams[key] = value
		}
	}

	return pathParams, remainingParams
}
