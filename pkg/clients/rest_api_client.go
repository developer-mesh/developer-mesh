package clients

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RESTAPIClient defines the interface for interacting with the REST API
type RESTAPIClient interface {
	// ListTools returns all available tools for a tenant
	ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error)
	
	// GetTool returns details for a specific tool
	GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error)
	
	// ExecuteTool executes a tool action
	ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error)
	
	// GetToolHealth checks the health status of a tool
	GetToolHealth(ctx context.Context, tenantID, toolID string) (*models.HealthStatus, error)
}

// restAPIClient implements the RESTAPIClient interface
type restAPIClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     observability.Logger
	
	// Cache for tool list with TTL
	cacheMutex sync.RWMutex
	toolCache  map[string]*toolCacheEntry
}

type toolCacheEntry struct {
	tools     []*models.DynamicTool
	cachedAt  time.Time
	cacheTTL  time.Duration
}

// RESTClientConfig holds configuration for the REST API client
type RESTClientConfig struct {
	BaseURL         string
	APIKey          string
	Timeout         time.Duration
	MaxIdleConns    int
	MaxConnsPerHost int
	CacheTTL        time.Duration
	Logger          observability.Logger
}

// NewRESTAPIClient creates a new REST API client with configuration
func NewRESTAPIClient(config RESTClientConfig) RESTAPIClient {
	// Set defaults
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.MaxIdleConns == 0 {
		config.MaxIdleConns = 100
	}
	if config.MaxConnsPerHost == 0 {
		config.MaxConnsPerHost = 10
	}
	if config.CacheTTL == 0 {
		config.CacheTTL = 30 * time.Second
	}
	
	// Create HTTP client with connection pooling
	transport := &http.Transport{
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
	}
	
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   config.Timeout,
	}
	
	return &restAPIClient{
		baseURL:    config.BaseURL,
		apiKey:     config.APIKey,
		httpClient: httpClient,
		logger:     config.Logger,
		toolCache:  make(map[string]*toolCacheEntry),
	}
}

// ListTools retrieves all tools for a tenant
func (c *restAPIClient) ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error) {
	// Check cache first
	c.cacheMutex.RLock()
	if entry, exists := c.toolCache[tenantID]; exists {
		if time.Since(entry.cachedAt) < entry.cacheTTL {
			c.cacheMutex.RUnlock()
			c.logger.Debug("Returning cached tool list", map[string]interface{}{
				"tenant_id": tenantID,
				"tool_count": len(entry.tools),
			})
			return entry.tools, nil
		}
	}
	c.cacheMutex.RUnlock()
	
	// Build request
	url := fmt.Sprintf("%s/api/v1/tools", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Set headers
	c.setHeaders(req, tenantID)
	
	// Execute request
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()
	
	// Parse response
	var tools []*models.DynamicTool
	if err := json.NewDecoder(resp.Body).Decode(&tools); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	// Update cache
	c.cacheMutex.Lock()
	c.toolCache[tenantID] = &toolCacheEntry{
		tools:    tools,
		cachedAt: time.Now(),
		cacheTTL: 30 * time.Second,
	}
	c.cacheMutex.Unlock()
	
	c.logger.Info("Retrieved tools from REST API", map[string]interface{}{
		"tenant_id":  tenantID,
		"tool_count": len(tools),
	})
	
	return tools, nil
}

// GetTool retrieves a specific tool
func (c *restAPIClient) GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error) {
	url := fmt.Sprintf("%s/api/v1/tools/%s", c.baseURL, toolID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setHeaders(req, tenantID)
	
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()
	
	var tool models.DynamicTool
	if err := json.NewDecoder(resp.Body).Decode(&tool); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &tool, nil
}

// ExecuteTool executes a tool action
func (c *restAPIClient) ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error) {
	url := fmt.Sprintf("%s/api/v1/tools/%s/execute/%s", c.baseURL, toolID, action)
	
	// Prepare request body
	body, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setHeaders(req, tenantID)
	req.Header.Set("Content-Type", "application/json")
	
	// Clear cache on execution (tool state might change)
	c.invalidateCache(tenantID)
	
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()
	
	var result models.ToolExecutionResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	c.logger.Info("Executed tool via REST API", map[string]interface{}{
		"tenant_id": tenantID,
		"tool_id":   toolID,
		"action":    action,
		"success":   result.Success,
	})
	
	return &result, nil
}

// GetToolHealth checks tool health status
func (c *restAPIClient) GetToolHealth(ctx context.Context, tenantID, toolID string) (*models.HealthStatus, error) {
	url := fmt.Sprintf("%s/api/v1/tools/%s/health", c.baseURL, toolID)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	c.setHeaders(req, tenantID)
	
	resp, err := c.doRequest(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()
	
	var health models.HealthStatus
	if err := json.NewDecoder(resp.Body).Decode(&health); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	
	return &health, nil
}

// setHeaders sets common headers for all requests
func (c *restAPIClient) setHeaders(req *http.Request, tenantID string) {
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Tenant-ID", tenantID)
	req.Header.Set("User-Agent", "MCP-Server/1.0")
}

// doRequest executes an HTTP request with retry logic and error handling
func (c *restAPIClient) doRequest(req *http.Request) (*http.Response, error) {
	maxRetries := 3
	baseDelay := 100 * time.Millisecond
	maxDelay := 5 * time.Second
	
	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		// Clone the request for retry (body might be consumed)
		reqCopy := req.Clone(req.Context())
		if req.Body != nil {
			// For retries, we need to reset the body
			body, err := io.ReadAll(req.Body)
			if err != nil {
				return nil, fmt.Errorf("failed to read request body: %w", err)
			}
			reqCopy.Body = io.NopCloser(bytes.NewReader(body))
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		
		resp, err := c.httpClient.Do(reqCopy)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			
			// Network errors are retryable
			if attempt < maxRetries {
				delay := c.calculateBackoff(attempt, baseDelay, maxDelay)
				c.logger.Warn("Request failed, retrying", map[string]interface{}{
					"attempt":     attempt + 1,
					"max_retries": maxRetries,
					"delay_ms":    delay.Milliseconds(),
					"error":       err.Error(),
				})
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		}
		
		// Check status code
		if resp.StatusCode >= 500 {
			// Server errors are retryable
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
			
			if attempt < maxRetries {
				delay := c.calculateBackoff(attempt, baseDelay, maxDelay)
				c.logger.Warn("Server error, retrying", map[string]interface{}{
					"status_code": resp.StatusCode,
					"attempt":     attempt + 1,
					"max_retries": maxRetries,
					"delay_ms":    delay.Milliseconds(),
				})
				time.Sleep(delay)
				continue
			}
			return nil, lastErr
		} else if resp.StatusCode >= 400 {
			// Client errors are not retryable
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}
		
		// Success
		return resp, nil
	}
	
	return nil, lastErr
}

// calculateBackoff calculates exponential backoff with jitter
func (c *restAPIClient) calculateBackoff(attempt int, baseDelay, maxDelay time.Duration) time.Duration {
	// Exponential backoff: baseDelay * 2^attempt
	delay := baseDelay * time.Duration(1<<uint(attempt))
	
	// Add jitter (±25%)
	jitter := time.Duration(float64(delay) * 0.25 * (0.5 - float64(time.Now().UnixNano()%100)/100.0))
	delay += jitter
	
	// Cap at maxDelay
	if delay > maxDelay {
		delay = maxDelay
	}
	
	return delay
}

// invalidateCache removes cached data for a tenant
func (c *restAPIClient) invalidateCache(tenantID string) {
	c.cacheMutex.Lock()
	delete(c.toolCache, tenantID)
	c.cacheMutex.Unlock()
}
