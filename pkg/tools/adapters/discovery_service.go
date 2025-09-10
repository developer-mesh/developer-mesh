package adapters

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/security"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/net/html"
)

// DiscoveryService handles intelligent OpenAPI specification discovery
type DiscoveryService struct {
	logger           observability.Logger
	httpClient       *http.Client
	validator        *tools.URLValidator
	formatDetector   *FormatDetector
	hintDiscovery    *HintBasedDiscovery
	learningService  *LearningDiscoveryService
	webhookExtractor *WebhookExtractor
	sanitizer        *OpenAPISanitizer
}

// DiscoveryHint provides optional user-provided hints for discovering APIs
type DiscoveryHint struct {
	CommonPaths []string          // Additional paths to try
	Subdomains  []string          // Additional subdomains to try
	Headers     map[string]string // Headers to send during discovery
}

// NewDiscoveryService creates a new discovery service
func NewDiscoveryService(logger observability.Logger) *DiscoveryService {
	// Increased timeout to 60s to handle large OpenAPI specs like GitHub
	httpClient := &http.Client{
		Timeout: 60 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}
	validator := tools.NewURLValidator()
	formatDetector := NewFormatDetector(httpClient)
	hintDiscovery := NewHintBasedDiscovery(formatDetector, validator)
	learningService := NewLearningDiscoveryService(NewInMemoryPatternStore())
	webhookExtractor := NewWebhookExtractor()

	return &DiscoveryService{
		logger:           logger,
		httpClient:       httpClient,
		validator:        validator,
		formatDetector:   formatDetector,
		hintDiscovery:    hintDiscovery,
		learningService:  learningService,
		webhookExtractor: webhookExtractor,
		sanitizer:        NewOpenAPISanitizer(logger),
	}
}

// NewDiscoveryServiceWithOptions creates a new discovery service with custom options
func NewDiscoveryServiceWithOptions(logger observability.Logger, httpClient *http.Client, validator *tools.URLValidator) *DiscoveryService {
	return NewDiscoveryServiceWithStore(logger, httpClient, validator, nil)
}

// NewDiscoveryServiceWithStore creates a new discovery service with custom pattern store
func NewDiscoveryServiceWithStore(logger observability.Logger, httpClient *http.Client, validator *tools.URLValidator, patternStore DiscoveryPatternStore) *DiscoveryService {
	if httpClient == nil {
		// Increased timeout to 60s to handle large OpenAPI specs like GitHub
		httpClient = &http.Client{
			Timeout: 60 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		}
	}
	// Don't create a validator if nil is explicitly passed (for testing)
	// validator == nil means no URL validation (test mode)

	// Initialize enhanced features
	formatDetector := NewFormatDetector(httpClient)
	hintDiscovery := NewHintBasedDiscovery(formatDetector, validator)
	webhookExtractor := NewWebhookExtractor()

	// Use provided pattern store or default to in-memory
	if patternStore == nil {
		patternStore = NewInMemoryPatternStore()
	}
	learningService := NewLearningDiscoveryService(patternStore)

	return &DiscoveryService{
		logger:           logger,
		httpClient:       httpClient,
		validator:        validator,
		formatDetector:   formatDetector,
		hintDiscovery:    hintDiscovery,
		learningService:  learningService,
		webhookExtractor: webhookExtractor,
		sanitizer:        NewOpenAPISanitizer(logger),
	}
}

// DiscoverOpenAPISpec discovers OpenAPI specification for a given tool
func (s *DiscoveryService) DiscoverOpenAPISpec(ctx context.Context, config tools.ToolConfig) (*tools.DiscoveryResult, error) {
	result := &tools.DiscoveryResult{
		Status:         tools.DiscoveryStatusFailed,
		DiscoveredURLs: []string{},
		Metadata:       make(map[string]interface{}),
	}

	// Strategy 1: Direct OpenAPI URL if provided
	if config.OpenAPIURL != "" {
		spec, err := s.fetchAndParseSpec(ctx, config.OpenAPIURL, config.Credential)
		if err == nil {
			result.Status = tools.DiscoveryStatusSuccess
			result.OpenAPISpec = spec
			result.SpecURL = config.OpenAPIURL
			result.DiscoveredURLs = append(result.DiscoveredURLs, config.OpenAPIURL)

			// Extract webhook configuration
			if s.webhookExtractor != nil {
				result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(spec, config.ID)
			}

			return result, nil
		}
		s.logger.Debug("Failed to fetch from configured OpenAPI URL", map[string]interface{}{
			"url":   config.OpenAPIURL,
			"error": err.Error(),
		})
	}

	// Try hint-based discovery first if hints are available
	hints, err := ExtractHintsFromConfig(config)
	if err == nil && hints != nil && (hints.OpenAPIURL != "" || len(hints.CustomPaths) > 0) {
		hintResult, err := s.hintDiscovery.DiscoverWithHints(ctx, config, hints)
		if err == nil && hintResult.Status == tools.DiscoveryStatusSuccess {
			// Extract webhook configuration from hint-based discovery
			if s.webhookExtractor != nil && hintResult.OpenAPISpec != nil {
				hintResult.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(hintResult.OpenAPISpec, config.ID)
			}

			// Learn from successful hint-based discovery
			if s.learningService != nil {
				if err := s.learningService.LearnFromSuccess(config, hintResult); err != nil {
					s.logger.Warn("Failed to record discovery pattern", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}
			return hintResult, nil
		}
	}

	// Get learned paths from previous successful discoveries
	var learnedPaths []string
	if s.learningService != nil {
		learnedPaths = s.learningService.GetSuggestedPaths(config.BaseURL)
	}

	// Get user-provided hints if any (optional)
	discoveryHints := s.getUserProvidedHints(config)

	// Strategy 2: Try common OpenAPI paths
	openAPIPaths := s.getCommonOpenAPIPaths()

	// Prioritize learned paths, then hints, then common paths
	if len(learnedPaths) > 0 {
		openAPIPaths = append(learnedPaths, openAPIPaths...)
	}
	if len(discoveryHints.CommonPaths) > 0 {
		// User provided additional paths to try
		openAPIPaths = append(discoveryHints.CommonPaths, openAPIPaths...)
	}
	for _, path := range openAPIPaths {
		fullURL := s.buildURL(config.BaseURL, path)
		if s.validator != nil {
			if err := s.validator.ValidateURL(ctx, fullURL); err != nil {
				s.logger.Debug("URL validation failed", map[string]interface{}{
					"url":   fullURL,
					"error": err.Error(),
				})
				continue
			}
		}

		// First try as OpenAPI
		spec, err := s.fetchAndParseSpec(ctx, fullURL, config.Credential)
		if err == nil {
			result.Status = tools.DiscoveryStatusSuccess
			result.OpenAPISpec = spec
			result.SpecURL = fullURL
			result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)
			result.Metadata["api_format"] = "openapi"

			// Extract webhook configuration
			if s.webhookExtractor != nil {
				result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(spec, config.ID)
			}

			// Learn from success
			if s.learningService != nil {
				if err := s.learningService.LearnFromSuccess(config, result); err != nil {
					s.logger.Warn("Failed to record discovery pattern", map[string]interface{}{
						"error": err.Error(),
					})
				}
			}
			return result, nil
		}

		// Try format detection if OpenAPI parsing failed
		if s.formatDetector != nil {
			content, fetchErr := s.fetchContent(ctx, fullURL, config.Credential)
			if fetchErr == nil {
				format, _ := s.formatDetector.DetectFormat(content)
				if format != FormatUnknown && format != FormatOpenAPI3 {
					// Try to convert to OpenAPI
					convertedSpec, convErr := s.formatDetector.ConvertToOpenAPI(content, format, config.BaseURL)
					if convErr == nil {
						result.Status = tools.DiscoveryStatusSuccess
						result.OpenAPISpec = convertedSpec
						result.SpecURL = fullURL
						result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)
						result.Metadata["api_format"] = string(format)
						result.Metadata["converted_from"] = string(format)

						// Extract webhook configuration
						if s.webhookExtractor != nil {
							result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(convertedSpec, config.ID)
						}

						// Learn from success
						if err := s.learningService.LearnFromSuccess(config, result); err != nil {
							s.logger.Warn("Failed to record discovery pattern", map[string]interface{}{
								"error": err.Error(),
							})
						}
						return result, nil
					}
				}
			}
		}
		s.logger.Debug("Failed to fetch spec", map[string]interface{}{
			"url":   fullURL,
			"error": err.Error(),
		})
		result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)
	}

	// Strategy 3: Try subdomain discovery
	subdomains := s.getCommonSubdomains()
	if len(discoveryHints.Subdomains) > 0 {
		// User provided additional subdomains to try
		subdomains = append(discoveryHints.Subdomains, subdomains...)
	}
	for _, subdomain := range subdomains {
		subdomainURL := s.applySubdomain(config.BaseURL, subdomain)
		if subdomainURL == "" {
			continue
		}

		for _, path := range openAPIPaths[:5] { // Try first few paths
			fullURL := s.buildURL(subdomainURL, path)
			if s.validator != nil {
				if err := s.validator.ValidateURL(ctx, fullURL); err != nil {
					continue
				}
			}

			spec, err := s.fetchAndParseSpec(ctx, fullURL, config.Credential)
			if err == nil {
				result.Status = tools.DiscoveryStatusSuccess
				result.OpenAPISpec = spec
				result.SpecURL = fullURL
				result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)

				// Extract webhook configuration
				if s.webhookExtractor != nil {
					result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(spec, config.ID)
				}

				return result, nil
			}
		}
	}

	// Strategy 4: Parse HTML for API documentation links
	htmlLinks, err := s.discoverFromHTML(ctx, config.BaseURL, config.Credential)
	if err == nil && len(htmlLinks) > 0 {
		for _, link := range htmlLinks {
			if s.validator != nil {
				if err := s.validator.ValidateURL(ctx, link); err != nil {
					continue
				}
			}

			spec, err := s.fetchAndParseSpec(ctx, link, config.Credential)
			if err == nil {
				result.Status = tools.DiscoveryStatusSuccess
				result.OpenAPISpec = spec
				result.SpecURL = link
				result.DiscoveredURLs = append(result.DiscoveredURLs, link)

				// Extract webhook configuration
				if s.webhookExtractor != nil {
					result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(spec, config.ID)
				}

				return result, nil
			}
			result.DiscoveredURLs = append(result.DiscoveredURLs, link)
		}
	}

	// Strategy 5: Try well-known paths
	wellKnownPaths := s.getWellKnownPaths()
	for _, path := range wellKnownPaths {
		fullURL := s.buildURL(config.BaseURL, path)
		if s.validator != nil {
			if err := s.validator.ValidateURL(ctx, fullURL); err != nil {
				continue
			}
		}

		// Check if it's a JSON document that might be OpenAPI
		if s.checkForOpenAPIDocument(ctx, fullURL, config.Credential) {
			spec, err := s.fetchAndParseSpec(ctx, fullURL, config.Credential)
			if err == nil {
				result.Status = tools.DiscoveryStatusSuccess
				result.OpenAPISpec = spec
				result.SpecURL = fullURL
				result.DiscoveredURLs = append(result.DiscoveredURLs, fullURL)

				// Extract webhook configuration
				if s.webhookExtractor != nil {
					result.WebhookConfig = s.webhookExtractor.ExtractWebhookConfig(spec, config.ID)
				}

				return result, nil
			}
		}
	}

	// If we discovered some URLs but couldn't parse them, it's partial success
	if len(result.DiscoveredURLs) > 0 {
		result.Status = tools.DiscoveryStatusPartial
		result.RequiresManual = true
		result.SuggestedActions = append(result.SuggestedActions,
			"Review discovered URLs and provide the correct OpenAPI specification URL",
			"Check if the API requires special authentication for accessing documentation",
		)
	} else {
		result.Status = tools.DiscoveryStatusManualNeeded
		result.RequiresManual = true
		result.SuggestedActions = append(result.SuggestedActions,
			"Manually provide the OpenAPI specification URL",
			"Contact the API provider for documentation",
			"Check if the API uses a different specification format (RAML, API Blueprint, etc.)",
		)
	}

	return result, nil
}

// getUserProvidedHints extracts user-provided discovery hints from config
func (s *DiscoveryService) getUserProvidedHints(config tools.ToolConfig) DiscoveryHint {
	hint := DiscoveryHint{}

	// Check if user provided discovery hints in config
	if config.Config != nil {
		if paths, ok := config.Config["discovery_paths"].([]string); ok {
			hint.CommonPaths = paths
		}
		if subdomains, ok := config.Config["discovery_subdomains"].([]string); ok {
			hint.Subdomains = subdomains
		}
	}

	return hint
}

// getCommonOpenAPIPaths returns common OpenAPI specification paths
func (s *DiscoveryService) getCommonOpenAPIPaths() []string {
	return []string{
		// Generic paths
		"/openapi.json",
		"/openapi.yaml",
		"/openapi.yml",
		"/swagger.json",
		"/swagger.yaml",
		"/api-docs",
		"/api-docs.json",
		"/api/swagger.json",
		"/v2/api-docs",
		"/v3/api-docs",
		"/api/v1/openapi.json",
		"/api/v2/openapi.json",
		"/api/v3/openapi.json",
		"/swagger/v1/swagger.json",
		"/swagger/v2/swagger.json",
		"/swagger-ui/swagger.json",
		"/docs/api.json",
		"/docs/openapi.json",
		"/documentation/api.json",
		"/spec/openapi.json",
		"/specification/openapi.json",
		"/.well-known/openapi.json",
		"/api-specification.json",

		// Service-specific paths based on research
		// Harness.io
		"/gateway/api/openapi.json",
		"/ng/api/openapi.json",
		"/ccm/api/openapi.json",

		// SonarQube
		"/api/webservices/list",
		"/web_api/api/webservices",

		// JFrog Artifactory
		"/artifactory/api/openapi.json",
		"/artifactory/api/swagger.json",

		// Nexus Repository
		"/service/rest/swagger.json",

		// Kubernetes-style APIs
		"/openapi/v2",
		"/openapi/v3",

		// Additional common patterns
		"/rest/api/swagger.json",
		"/rest/swagger.json",
		"/api/swagger",
		"/api/openapi",
		"/v1/swagger",
		"/v1/openapi",

		// Framework-specific patterns
		"/swagger-resources",
		"/api-docs/swagger.json",
		"/api/api-docs",
		"/apidocs/swagger.json",
		"/api/definition",
		"/api/spec",
		"/api/v1/spec",
		"/api/v1/swagger",
		"/docs/swagger.json",
		"/swagger/docs/v1",
		"/swagger/docs/v2",

		// Cloud/Enterprise patterns
		"/api/management/v1/openapi.json",
		"/platform/api/openapi.json",
		"/enterprise/api/swagger.json",
		"/cloud/api/openapi.json",

		// API Gateway patterns
		"/gateway/swagger.json",
		"/api-gateway/swagger.json",
		"/proxy/api/swagger.json",
	}
}

// getCommonSubdomains returns common API subdomains
func (s *DiscoveryService) getCommonSubdomains() []string {
	return []string{
		"api",
		"docs",
		"apidocs",
		"api-docs",
		"developer",
		"developers",
		"dev",
		"public-api",
		"open-api",
		"openapi",
	}
}

// getWellKnownPaths returns well-known paths to check
func (s *DiscoveryService) getWellKnownPaths() []string {
	return []string{
		"/.well-known/openapi.json",
		"/.well-known/api-documentation",
		"/api",
		"/api/v1",
		"/api/v2",
		"/rest",
		"/graphql", // Some tools expose GraphQL schemas
	}
}

// buildURL builds a complete URL
func (s *DiscoveryService) buildURL(baseURL, path string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return baseURL + path
}

// applySubdomain applies a subdomain to a URL
func (s *DiscoveryService) applySubdomain(baseURL, subdomain string) string {
	u, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}

	// Split the host into parts
	parts := strings.Split(u.Host, ".")

	// If it's a bare domain (e.g., "example.com"), we can add a subdomain
	if len(parts) == 2 {
		u.Host = subdomain + "." + u.Host
		return u.String()
	}

	// If it already has subdomain(s), check if the first one matches
	if len(parts) > 2 && parts[0] == subdomain {
		return baseURL
	}

	// Replace the first subdomain with the new one
	if len(parts) > 2 {
		parts[0] = subdomain
		u.Host = strings.Join(parts, ".")
		return u.String()
	}

	// Single part domain (e.g., "localhost"), can't add subdomain
	return ""
}

// fetchContent fetches raw content from a URL
func (s *DiscoveryService) fetchContent(ctx context.Context, url string, creds *models.TokenCredential) ([]byte, error) {
	// Always validate URL to prevent SSRF attacks
	validator := security.NewURLValidator()
	// In test mode (when s.validator is nil), allow localhost for testing
	if s.validator == nil {
		validator.AllowLocalhost = true
		validator.AllowPrivateNetworks = true
	}
	validatedURL, err := validator.ValidateAndSanitizeURL(url)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, "GET", validatedURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication if provided
	if creds != nil {
		switch creds.Type {
		case "token":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
		case "api_key":
			if creds.HeaderName != "" {
				req.Header.Set(creds.HeaderName, creds.Token)
			} else {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
			}
		case "basic":
			req.SetBasicAuth(creds.Username, creds.Password)
		case "header":
			if creds.HeaderName != "" {
				req.Header.Set(creds.HeaderName, creds.Token)
			}
		}
	}

	// Common headers
	req.Header.Set("Accept", "application/json, application/yaml, text/yaml")
	req.Header.Set("User-Agent", "DevOps-MCP/1.0 OpenAPI-Discovery")

	// Make request
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			s.logger.Debugf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Read body with size limit
	// Increased limit to 20MB to support large OpenAPI specs like GitHub
	body, err := io.ReadAll(io.LimitReader(resp.Body, 20*1024*1024)) // 20MB limit
	if err != nil {
		return nil, err
	}

	return body, nil
}

// fetchAndParseSpec fetches and parses an OpenAPI specification
func (s *DiscoveryService) fetchAndParseSpec(ctx context.Context, specURL string, creds *models.TokenCredential) (*openapi3.T, error) {
	// Fetch content
	body, err := s.fetchContent(ctx, specURL, creds)
	if err != nil {
		return nil, err
	}

	// Pre-process JSON if sanitizer is available
	if s.sanitizer != nil {
		sanitized, fixes := s.sanitizer.SanitizeJSON(body)
		if len(fixes) > 0 {
			s.logger.Info("Applied JSON sanitization fixes", map[string]interface{}{
				"url":   specURL,
				"fixes": fixes,
			})
		}
		body = sanitized
	}

	// Parse OpenAPI spec
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = false // Security

	// Try to parse
	spec, err := loader.LoadFromData(body)
	if err != nil {
		// Try YAML if JSON failed
		// Note: kin-openapi handles both JSON and YAML
		return nil, fmt.Errorf("failed to parse OpenAPI spec: %w", err)
	}

	// Validate with sanitization
	if s.sanitizer != nil {
		// This will attempt to fix validation errors and continue even if some remain
		if err := s.sanitizer.ValidateAndSanitize(ctx, spec); err != nil {
			// This should rarely happen as ValidateAndSanitize returns nil for partial success
			s.logger.Warn("OpenAPI spec validation failed even after sanitization", map[string]interface{}{
				"url":   specURL,
				"error": err.Error(),
			})
		}
	} else {
		// Fallback to standard validation
		if err := spec.Validate(ctx); err != nil {
			return nil, fmt.Errorf("invalid OpenAPI spec: %w", err)
		}
	}

	return spec, nil
}

// checkForOpenAPIDocument checks if a URL might contain an OpenAPI document
func (s *DiscoveryService) checkForOpenAPIDocument(ctx context.Context, url string, creds *models.TokenCredential) bool {
	// Always validate URL to prevent SSRF attacks
	validator := security.NewURLValidator()
	// In test mode (when s.validator is nil), allow localhost for testing
	if s.validator == nil {
		validator.AllowLocalhost = true
		validator.AllowPrivateNetworks = true
	}
	validatedURL, err := validator.ValidateAndSanitizeURL(url)
	if err != nil {
		return false
	}

	req, err := http.NewRequestWithContext(ctx, "HEAD", validatedURL, nil)
	if err != nil {
		return false
	}

	// Add authentication
	if creds != nil {
		switch creds.Type {
		case "token":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
		case "api_key":
			if creds.HeaderName != "" {
				req.Header.Set(creds.HeaderName, creds.Token)
			} else {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
			}
		case "basic":
			req.SetBasicAuth(creds.Username, creds.Password)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			s.logger.Debugf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	// Check content type
	contentType := resp.Header.Get("Content-Type")
	return strings.Contains(contentType, "json") ||
		strings.Contains(contentType, "yaml") ||
		strings.Contains(contentType, "openapi")
}

// discoverFromHTML discovers API documentation links from HTML pages
func (s *DiscoveryService) discoverFromHTML(ctx context.Context, baseURL string, creds *models.TokenCredential) ([]string, error) {
	// Always validate URL to prevent SSRF attacks
	validator := security.NewURLValidator()
	// In test mode (when s.validator is nil), allow localhost for testing
	if s.validator == nil {
		validator.AllowLocalhost = true
		validator.AllowPrivateNetworks = true
	}
	validatedURL, err := validator.ValidateAndSanitizeURL(baseURL)
	if err != nil {
		return nil, fmt.Errorf("invalid base URL: %w", err)
	}

	// Fetch the homepage
	req, err := http.NewRequestWithContext(ctx, "GET", validatedURL, nil)
	if err != nil {
		return nil, err
	}

	// Add authentication
	if creds != nil {
		switch creds.Type {
		case "token":
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
		case "api_key":
			if creds.HeaderName != "" {
				req.Header.Set(creds.HeaderName, creds.Token)
			} else {
				req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", creds.Token))
			}
		case "basic":
			req.SetBasicAuth(creds.Username, creds.Password)
		}
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			// Log error but don't fail the operation
			s.logger.Debugf("failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	// Parse HTML
	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, err
	}

	// Look for API documentation links
	var links []string
	var crawler func(*html.Node)
	crawler = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, attr := range n.Attr {
				if attr.Key == "href" {
					href := attr.Val
					// Check if it might be API documentation
					if s.isAPIDocLink(href) {
						// Make absolute URL
						if !strings.HasPrefix(href, "http") {
							href = s.buildURL(baseURL, href)
						}
						links = append(links, href)
					}
					break
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			crawler(c)
		}
	}
	crawler(doc)

	return links, nil
}

// isAPIDocLink checks if a link might be API documentation
func (s *DiscoveryService) isAPIDocLink(href string) bool {
	href = strings.ToLower(href)

	// Keywords that suggest API documentation
	keywords := []string{
		"api", "swagger", "openapi", "docs", "documentation",
		"developer", "reference", "rest", "spec", "specification",
	}

	for _, keyword := range keywords {
		if strings.Contains(href, keyword) {
			return true
		}
	}

	return false
}
