package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/tools"
)

// DiscoveryHintType represents the type of hint provided
type DiscoveryHintType string

const (
	HintTypeOpenAPIURL  DiscoveryHintType = "openapi_url"
	HintTypeAuthHeader  DiscoveryHintType = "auth_header"
	HintTypeCustomPath  DiscoveryHintType = "custom_path"
	HintTypeAPIFormat   DiscoveryHintType = "api_format"
	HintTypeExampleCall DiscoveryHintType = "example_call"
	HintTypeDocURL      DiscoveryHintType = "doc_url"
	HintTypeAPIKey      DiscoveryHintType = "api_key_location"
)

// DiscoveryHints provides user-supplied hints for API discovery
type DiscoveryHints struct {
	// Direct OpenAPI URL if known
	OpenAPIURL string `json:"openapi_url,omitempty"`

	// Authentication hints
	AuthHeaders map[string]string `json:"auth_headers,omitempty"`
	APIKeyName  string            `json:"api_key_name,omitempty"`
	APIKeyIn    string            `json:"api_key_in,omitempty"` // header, query, cookie

	// Custom paths to try
	CustomPaths []string `json:"custom_paths,omitempty"`

	// API format hint (openapi, swagger, custom, etc.)
	APIFormat string `json:"api_format,omitempty"`

	// Example API call that works
	ExampleEndpoint string            `json:"example_endpoint,omitempty"`
	ExampleMethod   string            `json:"example_method,omitempty"`
	ExampleHeaders  map[string]string `json:"example_headers,omitempty"`

	// Documentation URL for manual reference
	DocumentationURL string `json:"documentation_url,omitempty"`

	// Response format hints
	ResponseFormat string `json:"response_format,omitempty"` // json, xml, etc.

	// Version information
	APIVersion string `json:"api_version,omitempty"`

	// Rate limiting hints
	RateLimitHeader string `json:"rate_limit_header,omitempty"`

	// Pagination hints
	PaginationStyle string `json:"pagination_style,omitempty"` // offset, cursor, page
	PageParam       string `json:"page_param,omitempty"`
	LimitParam      string `json:"limit_param,omitempty"`
}

// HintBasedDiscovery enhances discovery with user-provided hints
type HintBasedDiscovery struct {
	detector  *FormatDetector
	validator *tools.URLValidator
}

// NewHintBasedDiscovery creates a new hint-based discovery service
func NewHintBasedDiscovery(detector *FormatDetector, validator *tools.URLValidator) *HintBasedDiscovery {
	return &HintBasedDiscovery{
		detector:  detector,
		validator: validator,
	}
}

// DiscoverWithHints attempts discovery using user-provided hints
func (h *HintBasedDiscovery) DiscoverWithHints(ctx context.Context, config tools.ToolConfig, hints *DiscoveryHints) (*tools.DiscoveryResult, error) {
	result := &tools.DiscoveryResult{
		Status:         tools.DiscoveryStatusFailed,
		DiscoveredURLs: []string{},
		Metadata:       make(map[string]interface{}),
	}

	// Store hints in metadata for future reference
	result.Metadata["hints"] = hints

	// Store example endpoint if provided
	if hints.ExampleEndpoint != "" {
		result.Metadata["example_endpoint"] = hints.ExampleEndpoint
	}

	// Store documentation URL if provided
	if hints.DocumentationURL != "" {
		result.Metadata["documentation_url"] = hints.DocumentationURL
	}

	// Try direct OpenAPI URL if provided
	if hints.OpenAPIURL != "" {
		// Basic URL validation - ensure it's an absolute URL
		parsedURL, err := url.Parse(hints.OpenAPIURL)
		if err != nil || !parsedURL.IsAbs() || parsedURL.Scheme == "" || parsedURL.Host == "" {
			// Invalid URL
			result.Status = tools.DiscoveryStatusFailed
			result.SuggestedActions = append(result.SuggestedActions, "The provided OpenAPI URL is invalid or not absolute")
			return result, nil
		}

		// Validate and fetch the OpenAPI spec
		if h.validator == nil || h.validator.ValidateURL(ctx, hints.OpenAPIURL) == nil {
			result.DiscoveredURLs = append(result.DiscoveredURLs, hints.OpenAPIURL)
			result.Status = tools.DiscoveryStatusSuccess
			result.SpecURL = hints.OpenAPIURL

			// Fetch and process the OpenAPI spec
			if h.detector != nil && h.detector.httpClient != nil {
				req, err := http.NewRequestWithContext(ctx, "GET", hints.OpenAPIURL, nil)
				if err == nil {
					// Add auth headers if provided
					for key, value := range hints.AuthHeaders {
						req.Header.Set(key, value)
					}

					// Add credentials from config if provided
					if config.Credential != nil {
						cred := config.Credential
						if cred.Type == "token" && cred.Token != "" {
							req.Header.Set("Authorization", "Bearer "+cred.Token)
						} else if cred.Type == "api_key" && cred.Token != "" && cred.HeaderName != "" {
							req.Header.Set(cred.HeaderName, cred.Token)
						} else if cred.Type == "basic" && cred.Username != "" && cred.Password != "" {
							req.SetBasicAuth(cred.Username, cred.Password)
						}
					}

					resp, err := h.detector.httpClient.Do(req)
					if err == nil && resp.StatusCode == http.StatusOK {
						defer func() { _ = resp.Body.Close() }()
						content, err := io.ReadAll(resp.Body)
						if err == nil {
							// Detect format
							format, _ := h.detector.DetectFormat(content)
							if format != FormatUnknown {
								result.Metadata["api_format"] = string(format)
								// Convert to OpenAPI if needed
								spec, err := h.detector.ConvertToOpenAPI(content, format, config.BaseURL)
								if err == nil {
									result.OpenAPISpec = spec
									if format != FormatOpenAPI3 {
										result.Metadata["converted_from"] = string(format)
									}
								}
							}
						}
					}
				}
			}
			return result, nil
		}
	}

	// Try custom paths if provided
	if len(hints.CustomPaths) > 0 {
		for _, path := range hints.CustomPaths {
			fullURL := strings.TrimRight(config.BaseURL, "/") + "/" + strings.TrimLeft(path, "/")
			result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)

			// Try to fetch and detect format
			if h.detector != nil && h.detector.httpClient != nil {
				req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
				if err == nil {
					// Add auth headers if provided
					for key, value := range hints.AuthHeaders {
						req.Header.Set(key, value)
					}

					// Add credentials from config if provided
					if config.Credential != nil {
						cred := config.Credential
						if cred.Type == "token" && cred.Token != "" {
							req.Header.Set("Authorization", "Bearer "+cred.Token)
						} else if cred.Type == "api_key" && cred.Token != "" && cred.HeaderName != "" {
							req.Header.Set(cred.HeaderName, cred.Token)
						} else if cred.Type == "basic" && cred.Username != "" && cred.Password != "" {
							req.SetBasicAuth(cred.Username, cred.Password)
						}
					}

					resp, err := h.detector.httpClient.Do(req)
					if err == nil && resp.StatusCode == http.StatusOK {
						defer func() { _ = resp.Body.Close() }()
						content, err := io.ReadAll(resp.Body)
						if err == nil {
							// Detect format
							format, _ := h.detector.DetectFormat(content)
							if format != FormatUnknown || hints.APIFormat != "" {
								// Use hint format if detection failed
								if format == FormatUnknown && hints.APIFormat != "" {
									format = APIFormat(hints.APIFormat)
								}
								result.Metadata["api_format"] = string(format)
								// Convert to OpenAPI if needed
								spec, err := h.detector.ConvertToOpenAPI(content, format, config.BaseURL)
								if err == nil {
									result.OpenAPISpec = spec
									result.Status = tools.DiscoveryStatusSuccess
									result.SpecURL = fullURL
									if format != FormatOpenAPI3 {
										result.Metadata["converted_from"] = string(format)
									}
									return result, nil
								}
							}
						}
					}
				}
			}
		}
	}

	// Try example endpoint to understand API structure
	if hints.ExampleEndpoint != "" {
		apiInfo, err := h.analyzeExampleEndpoint(ctx, config.BaseURL, hints)
		if err == nil {
			result.Metadata["api_info"] = apiInfo
			result.Status = tools.DiscoveryStatusPartial
		}
	}

	// Generate suggested actions based on hints
	result.SuggestedActions = h.generateSuggestedActions(hints)

	// If no hints were effective, set status to manual needed
	if result.Status == tools.DiscoveryStatusFailed && len(hints.OpenAPIURL) == 0 && len(hints.CustomPaths) == 0 {
		result.Status = tools.DiscoveryStatusManualNeeded
		result.RequiresManual = true

		// Add default suggestions when no hints provided
		if len(result.SuggestedActions) == 0 {
			result.SuggestedActions = append(result.SuggestedActions,
				"Try providing the OpenAPI specification URL",
				"Check the API documentation for specification links",
				"Provide custom paths where the API specification might be located",
			)
		}
	}

	return result, nil
}

// analyzeExampleEndpoint analyzes an example API endpoint to understand the API
func (h *HintBasedDiscovery) analyzeExampleEndpoint(ctx context.Context, baseURL string, hints *DiscoveryHints) (map[string]interface{}, error) {
	// This would make a request to the example endpoint and analyze:
	// - Response structure
	// - Headers
	// - Authentication method
	// - Rate limit information

	apiInfo := map[string]interface{}{
		"base_url":         baseURL,
		"example_endpoint": hints.ExampleEndpoint,
		"method":           hints.ExampleMethod,
		"response_format":  hints.ResponseFormat,
	}

	// Analyze authentication from example
	if len(hints.ExampleHeaders) > 0 {
		authInfo := h.analyzeAuthHeaders(hints.ExampleHeaders)
		apiInfo["auth_info"] = authInfo
	}

	return apiInfo, nil
}

// analyzeAuthHeaders analyzes headers to determine authentication method
func (h *HintBasedDiscovery) analyzeAuthHeaders(headers map[string]string) map[string]interface{} {
	authInfo := map[string]interface{}{}

	for key, value := range headers {
		lowerKey := strings.ToLower(key)

		// Bearer token
		if lowerKey == "authorization" && strings.HasPrefix(strings.ToLower(value), "bearer ") {
			authInfo["type"] = "bearer"
			authInfo["header"] = key
		}

		// Basic auth
		if lowerKey == "authorization" && strings.HasPrefix(strings.ToLower(value), "basic ") {
			authInfo["type"] = "basic"
			authInfo["header"] = key
		}

		// API key in header
		if strings.Contains(lowerKey, "api") && strings.Contains(lowerKey, "key") {
			authInfo["type"] = "apikey"
			authInfo["header"] = key
			authInfo["in"] = "header"
		}

		// Custom token headers
		if strings.Contains(lowerKey, "token") || strings.Contains(lowerKey, "auth") {
			authInfo["type"] = "custom"
			authInfo["header"] = key
		}
	}

	return authInfo
}

// generateSuggestedActions generates suggested actions based on hints
func (h *HintBasedDiscovery) generateSuggestedActions(hints *DiscoveryHints) []string {
	actions := []string{}

	if hints.DocumentationURL != "" {
		actions = append(actions, fmt.Sprintf("Check the API documentation at %s for OpenAPI/Swagger specification links", hints.DocumentationURL))
	}

	if hints.APIFormat != "" && hints.APIFormat != "openapi" {
		actions = append(actions, fmt.Sprintf("The API uses %s format. Consider converting to OpenAPI format", hints.APIFormat))
	}

	if hints.APIKeyName != "" {
		actions = append(actions, fmt.Sprintf("Configure API authentication using %s in %s", hints.APIKeyName, hints.APIKeyIn))
	}

	if len(hints.CustomPaths) > 0 {
		actions = append(actions, "Try the custom paths provided in the hints")
	}

	return actions
}

// ExtractHintsFromConfig extracts discovery hints from tool config
func ExtractHintsFromConfig(config tools.ToolConfig) (*DiscoveryHints, error) {
	hints := &DiscoveryHints{}

	if hintsData, ok := config.Config["discovery_hints"].(map[string]interface{}); ok {
		hintsJSON, err := json.Marshal(hintsData)
		if err != nil {
			return nil, err
		}

		if err := json.Unmarshal(hintsJSON, hints); err != nil {
			return nil, err
		}
	}

	// Also check for individual hint fields in config
	if openAPIURL, ok := config.Config["hint_openapi_url"].(string); ok {
		hints.OpenAPIURL = openAPIURL
	}

	if customPaths, ok := config.Config["hint_custom_paths"].([]interface{}); ok {
		for _, path := range customPaths {
			if p, ok := path.(string); ok {
				hints.CustomPaths = append(hints.CustomPaths, p)
			}
		}
	}

	if apiFormat, ok := config.Config["hint_api_format"].(string); ok {
		hints.APIFormat = apiFormat
	}

	return hints, nil
}

// SaveDiscoveryResults saves successful discovery results for future use
func SaveDiscoveryResults(config *tools.ToolConfig, result *tools.DiscoveryResult) {
	if result.Status == tools.DiscoveryStatusSuccess && result.SpecURL != "" {
		// Save the discovered OpenAPI URL for future use
		if config.Config == nil {
			config.Config = make(map[string]interface{})
		}
		config.Config["discovered_openapi_url"] = result.SpecURL
		config.Config["discovery_timestamp"] = result.Metadata["timestamp"]
	}
}
