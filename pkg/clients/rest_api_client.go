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

// minInt returns the smaller of two ints
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// RESTAPIClient defines the interface for interacting with the REST API
type RESTAPIClient interface {
	// ListTools returns all available tools for a tenant
	ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error)

	// GetTool returns details for a specific tool
	GetTool(ctx context.Context, tenantID, toolID string) (*models.DynamicTool, error)

	// ExecuteTool executes a tool action
	ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error)

	// ExecuteToolWithAuth executes a tool action with passthrough authentication
	ExecuteToolWithAuth(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}, passthroughAuth *models.PassthroughAuthBundle) (*models.ToolExecutionResponse, error)

	// GetToolHealth checks the health status of a tool
	GetToolHealth(ctx context.Context, tenantID, toolID string) (*models.HealthStatus, error)

	// GenerateEmbedding generates an embedding for the provided text
	GenerateEmbedding(ctx context.Context, tenantID, agentID, text, model, taskType string) (*models.EmbeddingResponse, error)

	// HealthCheck verifies the REST API is reachable and responding
	HealthCheck(ctx context.Context) error

	// GetMetrics returns client metrics for monitoring
	GetMetrics() ClientMetrics

	// Close gracefully shuts down the client
	Close() error
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

	// Circuit breaker for resilience
	circuitBreaker *CircuitBreaker

	// Metrics for monitoring
	metrics ClientMetrics

	// Enhanced observability
	observabilityManager *ObservabilityManager

	// Shutdown channel
	shutdown chan struct{}
	wg       sync.WaitGroup
}

type toolCacheEntry struct {
	tools    []*models.DynamicTool
	cachedAt time.Time
	cacheTTL time.Duration
}

// ClientMetrics holds metrics for REST API client operations
type ClientMetrics struct {
	TotalRequests       int64
	SuccessfulRequests  int64
	FailedRequests      int64
	CacheHits           int64
	CacheMisses         int64
	CircuitBreakerState string
	LastHealthCheck     time.Time
	Healthy             bool
	Metadata            map[string]interface{} // Additional metadata for extended metrics
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu                   sync.RWMutex
	state                string // "closed", "open", "half-open"
	failures             int
	consecutiveSuccesses int
	lastFailureTime      time.Time
	nextRetryTime        time.Time

	// Configuration
	maxFailures      int
	timeout          time.Duration
	retryTimeout     time.Duration
	successThreshold int
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
	MetricsClient   observability.MetricsClient

	// Circuit breaker configuration
	CircuitBreakerMaxFailures  int
	CircuitBreakerTimeout      time.Duration
	CircuitBreakerRetryTimeout time.Duration

	// Health check configuration
	HealthCheckInterval time.Duration
	HealthCheckTimeout  time.Duration

	// Observability configuration
	ObservabilityConfig ObservabilityConfig
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

	// Set circuit breaker defaults
	if config.CircuitBreakerMaxFailures == 0 {
		config.CircuitBreakerMaxFailures = 5
	}
	if config.CircuitBreakerTimeout == 0 {
		config.CircuitBreakerTimeout = 60 * time.Second
	}
	if config.CircuitBreakerRetryTimeout == 0 {
		config.CircuitBreakerRetryTimeout = 30 * time.Second
	}

	// Initialize observability manager if configured
	var observabilityManager *ObservabilityManager
	if config.ObservabilityConfig.TracingEnabled || config.ObservabilityConfig.MetricsEnabled {
		var err error
		observabilityManager, err = NewObservabilityManager(
			config.ObservabilityConfig,
			config.Logger,
			config.MetricsClient,
		)
		if err != nil {
			config.Logger.Warn("Failed to initialize observability manager", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	client := &restAPIClient{
		baseURL:              config.BaseURL,
		apiKey:               config.APIKey,
		httpClient:           httpClient,
		logger:               config.Logger,
		toolCache:            make(map[string]*toolCacheEntry),
		shutdown:             make(chan struct{}),
		observabilityManager: observabilityManager,
		circuitBreaker: &CircuitBreaker{
			state:            "closed",
			maxFailures:      config.CircuitBreakerMaxFailures,
			timeout:          config.CircuitBreakerTimeout,
			retryTimeout:     config.CircuitBreakerRetryTimeout,
			successThreshold: 2,
		},
	}

	// Start health check goroutine if configured
	if config.HealthCheckInterval > 0 {
		client.wg.Add(1)
		go client.runHealthChecks(config.HealthCheckInterval, config.HealthCheckTimeout)
	}

	return client
}

// ListTools retrieves all tools for a tenant
func (c *restAPIClient) ListTools(ctx context.Context, tenantID string) ([]*models.DynamicTool, error) {
	// Start distributed tracing span
	if c.observabilityManager != nil {
		var span interface{}
		ctx, span = c.observabilityManager.StartSpan(ctx, "ListTools")
		if s, ok := span.(interface{ End() }); ok {
			defer s.End()
		}
	}

	// Record business metric
	if c.observabilityManager != nil {
		c.observabilityManager.RecordBusinessMetric(ctx, "api_request", 1, map[string]string{
			"operation": "ListTools",
			"tenant_id": tenantID,
		})
	}

	startTime := time.Now()
	defer func() {
		// Record operation latency
		if c.observabilityManager != nil {
			c.observabilityManager.RecordSpanMetric(ctx, "operation_latency", time.Since(startTime).Seconds(), "seconds")
		}
	}()

	// Check cache first
	c.cacheMutex.RLock()
	if entry, exists := c.toolCache[tenantID]; exists {
		if time.Since(entry.cachedAt) < entry.cacheTTL {
			c.cacheMutex.RUnlock()
			c.metrics.CacheHits++

			// Record cache hit
			if c.observabilityManager != nil {
				c.observabilityManager.IncrementCounter("cache_hits", 1, map[string]string{
					"cache":     "tool_cache",
					"tenant_id": tenantID,
				})
			}

			c.logger.Debug("Returning cached tool list", map[string]interface{}{
				"tenant_id":  tenantID,
				"tool_count": len(entry.tools),
			})
			return entry.tools, nil
		}
	}
	c.cacheMutex.RUnlock()
	c.metrics.CacheMisses++

	// Record cache miss
	if c.observabilityManager != nil {
		c.observabilityManager.IncrementCounter("cache_misses", 1, map[string]string{
			"cache":     "tool_cache",
			"tenant_id": tenantID,
		})
	}

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

	// Read the response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Log the raw response for debugging
	c.logger.Debug("REST API raw response", map[string]interface{}{
		"tenant_id":      tenantID,
		"response_size":  len(bodyBytes),
		"response_start": string(bodyBytes[:minInt(500, len(bodyBytes))]), // First 500 chars
	})

	// Parse response
	// The REST API returns {"count": N, "tools": [...]}
	var response struct {
		Count int                   `json:"count"`
		Tools []*models.DynamicTool `json:"tools"`
	}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		c.logger.Error("Failed to unmarshal response", map[string]interface{}{
			"error":        err.Error(),
			"response":     string(bodyBytes[:minInt(1000, len(bodyBytes))]),
			"content_type": resp.Header.Get("Content-Type"),
		})
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	tools := response.Tools

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

// ExecuteTool executes a tool action (without passthrough auth)
func (c *restAPIClient) ExecuteTool(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}) (*models.ToolExecutionResponse, error) {
	return c.ExecuteToolWithAuth(ctx, tenantID, toolID, action, params, nil)
}

// ExecuteToolWithAuth executes a tool action with passthrough authentication
func (c *restAPIClient) ExecuteToolWithAuth(ctx context.Context, tenantID, toolID, action string, params map[string]interface{}, passthroughAuth *models.PassthroughAuthBundle) (*models.ToolExecutionResponse, error) {
	// Use the new endpoint that accepts action in the body
	apiURL := fmt.Sprintf("%s/api/v1/tools/%s/execute", c.baseURL, toolID)

	// Prepare request body
	// The params passed here are what should go in the "parameters" field
	// MCP already sends the correct structure, so we just pass it through
	requestBody := map[string]interface{}{
		"action":     action,
		"parameters": params,
	}

	// Add passthrough auth if provided
	if passthroughAuth != nil {
		requestBody["passthrough_auth"] = passthroughAuth
	}

	// Log to help debug parameter flow
	c.logger.Info("REST API client sending request", map[string]interface{}{
		"tool_id":     toolID,
		"action":      action,
		"params":      params,
		"params_keys": getKeys(params),
	})

	body, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal params: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", apiURL, bytes.NewReader(body))
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

// GenerateEmbedding generates an embedding for the provided text
func (c *restAPIClient) GenerateEmbedding(ctx context.Context, tenantID, agentID, text, model, taskType string) (*models.EmbeddingResponse, error) {
	// Start distributed tracing span
	if c.observabilityManager != nil {
		var span interface{}
		ctx, span = c.observabilityManager.StartSpan(ctx, "GenerateEmbedding")
		if s, ok := span.(interface{ End() }); ok {
			defer s.End()
		}
	}

	// Prepare request body
	reqBody := map[string]interface{}{
		"text":      text,
		"agent_id":  agentID,
		"tenant_id": tenantID,
		"task_type": taskType,
	}

	// Add model if specified
	if model != "" {
		reqBody["model"] = model
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/v1/embeddings", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(req, tenantID)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.doRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute embedding request: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	var result models.EmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	c.logger.Info("Generated embedding via REST API", map[string]interface{}{
		"tenant_id":    tenantID,
		"agent_id":     agentID,
		"model":        model,
		"task_type":    taskType,
		"embedding_id": result.EmbeddingID,
	})

	return &result, nil
}

// getKeys returns the keys from a map for logging purposes
func getKeys(m map[string]interface{}) []string {
	if m == nil {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
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
			c.circuitBreaker.recordFailure()

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
		c.circuitBreaker.recordSuccess()
		c.metrics.SuccessfulRequests++
		return resp, nil
	}

	c.metrics.FailedRequests++
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

// HealthCheck verifies the REST API is reachable and responding
func (c *restAPIClient) HealthCheck(ctx context.Context) error {
	// Check circuit breaker state
	if !c.circuitBreaker.canAttempt() {
		c.metrics.Healthy = false
		return fmt.Errorf("circuit breaker is open")
	}

	url := fmt.Sprintf("%s/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	// Use a shorter timeout for health checks
	healthCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req = req.WithContext(healthCtx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.circuitBreaker.recordFailure()
		c.metrics.Healthy = false
		return fmt.Errorf("health check request failed: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close health check response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		c.circuitBreaker.recordFailure()
		c.metrics.Healthy = false
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	c.circuitBreaker.recordSuccess()
	c.metrics.Healthy = true
	c.metrics.LastHealthCheck = time.Now()
	return nil
}

// GetMetrics returns client metrics for monitoring
func (c *restAPIClient) GetMetrics() ClientMetrics {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()

	c.metrics.CircuitBreakerState = c.circuitBreaker.getState()

	// Add observability metrics if available
	if c.observabilityManager != nil {
		c.metrics.Metadata = c.observabilityManager.GetObservabilityMetrics()
	}

	return c.metrics
}

// Close gracefully shuts down the client
func (c *restAPIClient) Close() error {
	close(c.shutdown)
	c.wg.Wait()

	// Shutdown observability manager
	if c.observabilityManager != nil {
		if err := c.observabilityManager.Close(); err != nil {
			c.logger.Warn("Failed to close observability manager", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}

	if c.httpClient != nil {
		if transport, ok := c.httpClient.Transport.(*http.Transport); ok {
			transport.CloseIdleConnections()
		}
	}

	c.logger.Info("REST API client closed", map[string]interface{}{
		"total_requests":      c.metrics.TotalRequests,
		"successful_requests": c.metrics.SuccessfulRequests,
		"failed_requests":     c.metrics.FailedRequests,
		"cache_hits":          c.metrics.CacheHits,
		"cache_misses":        c.metrics.CacheMisses,
	})

	return nil
}

// runHealthChecks runs periodic health checks
func (c *restAPIClient) runHealthChecks(interval, timeout time.Duration) {
	defer c.wg.Done()

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), timeout)
			if err := c.HealthCheck(ctx); err != nil {
				c.logger.Warn("Health check failed", map[string]interface{}{
					"error": err.Error(),
				})
			}
			cancel()
		case <-c.shutdown:
			return
		}
	}
}

// Circuit breaker methods
func (cb *CircuitBreaker) canAttempt() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	switch cb.state {
	case "closed":
		return true
	case "open":
		return time.Now().After(cb.nextRetryTime)
	case "half-open":
		return true
	default:
		return true
	}
}

func (cb *CircuitBreaker) recordSuccess() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures = 0
	cb.consecutiveSuccesses++

	if cb.state == "half-open" && cb.consecutiveSuccesses >= cb.successThreshold {
		cb.state = "closed"
		cb.consecutiveSuccesses = 0
	}
}

func (cb *CircuitBreaker) recordFailure() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.failures++
	cb.consecutiveSuccesses = 0
	cb.lastFailureTime = time.Now()

	if cb.failures >= cb.maxFailures {
		cb.state = "open"
		cb.nextRetryTime = time.Now().Add(cb.retryTimeout)
	}
}

func (cb *CircuitBreaker) getState() string {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	// Check if we should transition from open to half-open
	if cb.state == "open" && time.Now().After(cb.nextRetryTime) {
		cb.mu.RUnlock()
		cb.mu.Lock()
		cb.state = "half-open"
		cb.consecutiveSuccesses = 0
		cb.mu.Unlock()
		cb.mu.RLock()
	}

	return cb.state
}
