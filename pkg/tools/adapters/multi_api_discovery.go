package adapters

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools"
	"github.com/getkin/kin-openapi/openapi3"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// MultiAPIDiscoveryResult contains all discovered APIs from a portal
type MultiAPIDiscoveryResult struct {
	BaseURL         string                `json:"base_url"`
	DiscoveredAPIs  []APIDefinition       `json:"discovered_apis"`
	Status          tools.DiscoveryStatus `json:"status"`
	DiscoveryMethod string                `json:"discovery_method"`
	Timestamp       time.Time             `json:"timestamp"`
	Errors          []string              `json:"errors,omitempty"`
}

// APIDefinition represents a single discovered API
type APIDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	SpecURL     string      `json:"spec_url"`
	Version     string      `json:"version"`
	Category    string      `json:"category"`
	OpenAPISpec *openapi3.T `json:"-"` // Don't serialize the full spec
	Format      APIFormat   `json:"format"`
	Discovered  time.Time   `json:"discovered"`
}

// PortalPattern defines patterns for known API portal types
type PortalPattern struct {
	Name              string
	IdentifierPattern *regexp.Regexp // Pattern to identify this portal type
	SpecPatterns      []string       // URL patterns for API specs
	CategoryExtractor func(string) string
}

// Common portal patterns for major platforms
var portalPatterns = []PortalPattern{
	{
		Name:              "Harness",
		IdentifierPattern: regexp.MustCompile(`harness\.io`),
		SpecPatterns: []string{
			"/api/*/swagger.json",
			"/api/*/openapi.json",
			"/gateway/*/api/swagger.json",
			"/docs/api/*/spec.yaml",
		},
		CategoryExtractor: extractHarnessCategory,
	},
	{
		Name:              "AWS",
		IdentifierPattern: regexp.MustCompile(`(amazonaws\.com|aws\.amazon\.com)`),
		SpecPatterns: []string{
			"/*/latest/APIReference/API_*.json",
			"/*/api-reference.json",
			"/*/swagger.json",
		},
		CategoryExtractor: extractAWSCategory,
	},
	{
		Name:              "Azure",
		IdentifierPattern: regexp.MustCompile(`(azure\.microsoft\.com|azure\.com)`),
		SpecPatterns: []string{
			"/*/swagger/*.json",
			"/*/api-docs/*.json",
			"/specifications/*/resource-manager/*/swagger/*.json",
		},
		CategoryExtractor: extractAzureCategory,
	},
	{
		Name:              "Google Cloud",
		IdentifierPattern: regexp.MustCompile(`(googleapis\.com|cloud\.google\.com)`),
		SpecPatterns: []string{
			"/*/v*/swagger.json",
			"/*/discovery/rest",
			"/$discovery/rest?version=*",
		},
		CategoryExtractor: extractGoogleCategory,
	},
	{
		Name:              "Kubernetes",
		IdentifierPattern: regexp.MustCompile(`(kubernetes\.io|k8s\.io)`),
		SpecPatterns: []string{
			"/openapi/v2",
			"/swagger.json",
			"/apis/*/swagger.json",
		},
		CategoryExtractor: extractK8sCategory,
	},
	{
		Name:              "Generic",
		IdentifierPattern: regexp.MustCompile(`.+`), // Matches everything
		SpecPatterns: []string{
			"/openapi.json",
			"/openapi.yaml",
			"/swagger.json",
			"/swagger.yaml",
			"/api-docs",
			"/api/swagger.json",
			"/v*/api-docs",
			"/api/v*/docs",
			"/docs/api/swagger.json",
			"/api-documentation.json",
		},
		CategoryExtractor: extractGenericCategory,
	},
}

// MultiAPIDiscoveryService handles discovery of multiple APIs from portals
type MultiAPIDiscoveryService struct {
	logger         observability.Logger
	httpClient     *http.Client
	formatDetector *FormatDetector
	concurrency    int
	timeout        time.Duration
}

// NewMultiAPIDiscoveryService creates a new multi-API discovery service
func NewMultiAPIDiscoveryService(logger observability.Logger) *MultiAPIDiscoveryService {
	httpClient := &http.Client{
		Timeout: 30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			return nil
		},
	}

	return &MultiAPIDiscoveryService{
		logger:         logger,
		httpClient:     httpClient,
		formatDetector: NewFormatDetector(httpClient),
		concurrency:    5, // Limit concurrent API spec fetches
		timeout:        5 * time.Minute,
	}
}

// DiscoverMultipleAPIs discovers all APIs from a portal URL
func (s *MultiAPIDiscoveryService) DiscoverMultipleAPIs(ctx context.Context, portalURL string) (*MultiAPIDiscoveryResult, error) {
	result := &MultiAPIDiscoveryResult{
		BaseURL:        portalURL,
		DiscoveredAPIs: []APIDefinition{},
		Status:         tools.DiscoveryStatusPartial,
		Timestamp:      time.Now(),
		Errors:         []string{},
	}

	// Detect portal type
	portalType := s.detectPortalType(portalURL)
	result.DiscoveryMethod = portalType.Name

	s.logger.Info("Starting multi-API discovery", map[string]interface{}{
		"portal_url":  portalURL,
		"portal_type": portalType.Name,
	})

	// Try multiple discovery strategies in parallel
	var wg sync.WaitGroup
	discoveries := make(chan APIDefinition, 100)
	errors := make(chan error, 10)

	// Strategy 1: Pattern-based discovery
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.discoverByPatterns(ctx, portalURL, portalType, discoveries, errors)
	}()

	// Strategy 2: HTML crawling for API links
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.discoverByCrawling(ctx, portalURL, discoveries, errors)
	}()

	// Strategy 3: Well-known paths
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.discoverByWellKnownPaths(ctx, portalURL, discoveries, errors)
	}()

	// Strategy 4: API catalog endpoints
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.discoverByCatalogEndpoints(ctx, portalURL, discoveries, errors)
	}()

	// Wait for all strategies to complete
	go func() {
		wg.Wait()
		close(discoveries)
		close(errors)
	}()

	// Collect results
	discoveredURLs := make(map[string]bool)
	for {
		select {
		case api, ok := <-discoveries:
			if !ok {
				discoveries = nil
			} else if !discoveredURLs[api.SpecURL] {
				discoveredURLs[api.SpecURL] = true
				result.DiscoveredAPIs = append(result.DiscoveredAPIs, api)
			}
		case err, ok := <-errors:
			if !ok {
				errors = nil
			} else {
				result.Errors = append(result.Errors, err.Error())
			}
		case <-ctx.Done():
			result.Status = tools.DiscoveryStatusFailed
			result.Errors = append(result.Errors, "Discovery timeout")
			return result, ctx.Err()
		}

		if discoveries == nil && errors == nil {
			break
		}
	}

	// Determine final status
	if len(result.DiscoveredAPIs) > 0 {
		result.Status = tools.DiscoveryStatusSuccess
	} else if len(result.Errors) > 0 {
		result.Status = tools.DiscoveryStatusPartial
	} else {
		result.Status = tools.DiscoveryStatusManualNeeded
	}

	s.logger.Info("Multi-API discovery completed", map[string]interface{}{
		"portal_url": portalURL,
		"apis_found": len(result.DiscoveredAPIs),
		"status":     result.Status,
		"errors":     len(result.Errors),
	})

	return result, nil
}

// detectPortalType identifies the type of API portal
func (s *MultiAPIDiscoveryService) detectPortalType(portalURL string) *PortalPattern {
	for i := range portalPatterns {
		pattern := &portalPatterns[i]
		if pattern.IdentifierPattern.MatchString(portalURL) {
			return pattern
		}
	}
	// Return generic pattern as fallback
	return &portalPatterns[len(portalPatterns)-1]
}

// discoverByPatterns uses portal-specific patterns to find APIs
func (s *MultiAPIDiscoveryService) discoverByPatterns(ctx context.Context, baseURL string, portal *PortalPattern, discoveries chan<- APIDefinition, errors chan<- error) {
	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		errors <- fmt.Errorf("invalid base URL: %w", err)
		return
	}

	// Try each pattern
	for _, pattern := range portal.SpecPatterns {
		// Handle wildcard patterns
		if strings.Contains(pattern, "*") {
			s.discoverWithWildcard(ctx, parsedBase, pattern, portal, discoveries, errors)
		} else {
			// Direct path
			specURL := parsedBase.Scheme + "://" + parsedBase.Host + pattern
			s.tryDiscoverAPI(ctx, specURL, portal.CategoryExtractor, discoveries)
		}
	}
}

// discoverByCrawling crawls HTML pages to find API documentation links
func (s *MultiAPIDiscoveryService) discoverByCrawling(ctx context.Context, portalURL string, discoveries chan<- APIDefinition, errors chan<- error) {
	req, err := http.NewRequestWithContext(ctx, "GET", portalURL, nil)
	if err != nil {
		errors <- err
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		errors <- err
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		errors <- err
		return
	}

	// Look for API documentation links
	apiLinks := s.findAPILinks(doc, portalURL)

	// Process each link
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.concurrency)

	for _, link := range apiLinks {
		wg.Add(1)
		go func(apiLink string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			s.tryDiscoverAPI(ctx, apiLink, extractGenericCategory, discoveries)
		}(link)
	}

	wg.Wait()
}

// findAPILinks extracts potential API documentation links from HTML
func (s *MultiAPIDiscoveryService) findAPILinks(doc *goquery.Document, baseURL string) []string {
	links := make(map[string]bool)
	parsedBase, _ := url.Parse(baseURL)

	// Patterns that indicate API documentation
	apiPatterns := []string{
		"api", "swagger", "openapi", "docs", "documentation",
		"reference", "spec", "schema", "rest", "graphql",
	}

	doc.Find("a[href]").Each(func(i int, sel *goquery.Selection) {
		href, exists := sel.Attr("href")
		if !exists {
			return
		}

		text := strings.ToLower(sel.Text())
		href = strings.ToLower(href)

		// Check if link text or href contains API-related keywords
		isAPILink := false
		for _, pattern := range apiPatterns {
			if strings.Contains(text, pattern) || strings.Contains(href, pattern) {
				isAPILink = true
				break
			}
		}

		if isAPILink {
			// Resolve relative URLs
			if !strings.HasPrefix(href, "http") {
				if parsedBase != nil {
					parsedHref, err := parsedBase.Parse(href)
					if err == nil {
						href = parsedHref.String()
					}
				}
			}
			links[href] = true
		}
	})

	// Convert map to slice
	result := make([]string, 0, len(links))
	for link := range links {
		result = append(result, link)
	}

	return result
}

// discoverByWellKnownPaths tries common API documentation paths
func (s *MultiAPIDiscoveryService) discoverByWellKnownPaths(ctx context.Context, baseURL string, discoveries chan<- APIDefinition, errors chan<- error) {
	wellKnownPaths := []string{
		"/api-docs/index.html",
		"/api/catalog",
		"/api/registry",
		"/apis",
		"/.well-known/openapi.json",
		"/public/openapi.json",
		"/static/swagger.json",
		"/api-explorer",
		"/developer/apis",
		"/platform/apis",
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		errors <- err
		return
	}

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, s.concurrency)

	for _, path := range wellKnownPaths {
		wg.Add(1)
		go func(p string) {
			defer wg.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			specURL := parsedBase.Scheme + "://" + parsedBase.Host + p
			s.tryDiscoverAPI(ctx, specURL, extractGenericCategory, discoveries)
		}(path)
	}

	wg.Wait()
}

// discoverByCatalogEndpoints tries to find API catalog or registry endpoints
func (s *MultiAPIDiscoveryService) discoverByCatalogEndpoints(ctx context.Context, baseURL string, discoveries chan<- APIDefinition, errors chan<- error) {
	catalogEndpoints := []string{
		"/api/catalog/list",
		"/api/registry/services",
		"/apis/list",
		"/api-catalog.json",
		"/service-registry.json",
	}

	parsedBase, err := url.Parse(baseURL)
	if err != nil {
		errors <- err
		return
	}

	for _, endpoint := range catalogEndpoints {
		catalogURL := parsedBase.Scheme + "://" + parsedBase.Host + endpoint

		req, err := http.NewRequestWithContext(ctx, "GET", catalogURL, nil)
		if err != nil {
			continue
		}

		resp, err := s.httpClient.Do(req)
		if err != nil {
			continue
		}
		defer func() { _ = resp.Body.Close() }()

		if resp.StatusCode == http.StatusOK {
			// Try to parse as JSON catalog
			var catalog interface{}
			if err := json.NewDecoder(resp.Body).Decode(&catalog); err == nil {
				s.parseAPICatalog(ctx, catalog, parsedBase, discoveries)
			}
		}
	}
}

// parseAPICatalog parses various catalog formats
func (s *MultiAPIDiscoveryService) parseAPICatalog(ctx context.Context, catalog interface{}, baseURL *url.URL, discoveries chan<- APIDefinition) {
	switch v := catalog.(type) {
	case map[string]interface{}:
		// Look for arrays of APIs
		for key, value := range v {
			if arr, ok := value.([]interface{}); ok && (key == "apis" || key == "services" || key == "endpoints") {
				for _, item := range arr {
					if apiMap, ok := item.(map[string]interface{}); ok {
						s.parseAPIEntry(ctx, apiMap, baseURL, discoveries)
					}
				}
			}
		}
	case []interface{}:
		// Direct array of APIs
		for _, item := range v {
			if apiMap, ok := item.(map[string]interface{}); ok {
				s.parseAPIEntry(ctx, apiMap, baseURL, discoveries)
			}
		}
	}
}

// parseAPIEntry extracts API information from a catalog entry
func (s *MultiAPIDiscoveryService) parseAPIEntry(ctx context.Context, entry map[string]interface{}, baseURL *url.URL, discoveries chan<- APIDefinition) {
	// Look for spec URL
	var specURL string
	for _, key := range []string{"spec_url", "specUrl", "swagger_url", "swaggerUrl", "openapi_url", "openapiUrl", "url", "href"} {
		if val, ok := entry[key].(string); ok {
			specURL = val
			break
		}
	}

	if specURL != "" {
		// Resolve relative URLs
		if !strings.HasPrefix(specURL, "http") {
			parsed, err := baseURL.Parse(specURL)
			if err == nil {
				specURL = parsed.String()
			}
		}

		s.tryDiscoverAPI(ctx, specURL, extractGenericCategory, discoveries)
	}
}

// tryDiscoverAPI attempts to fetch and validate an API specification
func (s *MultiAPIDiscoveryService) tryDiscoverAPI(ctx context.Context, specURL string, categoryExtractor func(string) string, discoveries chan<- APIDefinition) {
	req, err := http.NewRequestWithContext(ctx, "GET", specURL, nil)
	if err != nil {
		return
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return
	}

	// Read content
	content, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024)) // 10MB limit
	if err != nil {
		return
	}

	// Detect format
	format, err := s.formatDetector.DetectFormat(content)
	if err != nil || format == FormatUnknown {
		return
	}

	// Convert to OpenAPI if needed
	spec, err := s.formatDetector.ConvertToOpenAPI(content, format, specURL)
	if err != nil {
		return
	}

	// Extract API information
	api := APIDefinition{
		Name:        s.extractAPIName(spec, specURL),
		Description: s.extractAPIDescription(spec),
		SpecURL:     specURL,
		Version:     s.extractAPIVersion(spec),
		Category:    categoryExtractor(specURL),
		OpenAPISpec: spec,
		Format:      format,
		Discovered:  time.Now(),
	}

	discoveries <- api
}

// discoverWithWildcard handles URL patterns with wildcards
func (s *MultiAPIDiscoveryService) discoverWithWildcard(ctx context.Context, baseURL *url.URL, pattern string, portal *PortalPattern, discoveries chan<- APIDefinition, errors chan<- error) {
	// This is a simplified implementation
	// In a real system, you might want to:
	// 1. Query a service discovery endpoint
	// 2. Parse a sitemap
	// 3. Use the portal's API to list services

	// For now, try common replacements
	commonReplacements := []string{
		"v1", "v2", "v3",
		"platform", "core", "admin",
		"public", "private", "internal",
		"rest", "graphql",
	}

	for _, replacement := range commonReplacements {
		specURL := baseURL.Scheme + "://" + baseURL.Host + strings.ReplaceAll(pattern, "*", replacement)
		s.tryDiscoverAPI(ctx, specURL, portal.CategoryExtractor, discoveries)
	}
}

// Helper functions to extract information from OpenAPI specs
func (s *MultiAPIDiscoveryService) extractAPIName(spec *openapi3.T, fallbackURL string) string {
	if spec != nil && spec.Info != nil && spec.Info.Title != "" {
		return spec.Info.Title
	}

	// Extract from URL as fallback
	parts := strings.Split(fallbackURL, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" && !strings.Contains(parts[i], ".") {
			return cases.Title(language.English).String(parts[i])
		}
	}

	return "Unknown API"
}

func (s *MultiAPIDiscoveryService) extractAPIDescription(spec *openapi3.T) string {
	if spec != nil && spec.Info != nil {
		return spec.Info.Description
	}
	return ""
}

func (s *MultiAPIDiscoveryService) extractAPIVersion(spec *openapi3.T) string {
	if spec != nil && spec.Info != nil {
		return spec.Info.Version
	}
	return "1.0.0"
}

// Category extractors for different platforms
func extractHarnessCategory(specURL string) string {
	if strings.Contains(specURL, "/platform/") {
		return "Platform"
	} else if strings.Contains(specURL, "/chaos/") {
		return "Chaos Engineering"
	} else if strings.Contains(specURL, "/ci/") {
		return "CI/CD"
	} else if strings.Contains(specURL, "/ff/") {
		return "Feature Flags"
	}
	return "Core"
}

func extractAWSCategory(specURL string) string {
	// Extract service name from URL
	parts := strings.Split(specURL, "/")
	for i, part := range parts {
		if part == "latest" && i > 0 {
			return strings.ToUpper(parts[i-1])
		}
	}
	return "AWS Service"
}

func extractAzureCategory(specURL string) string {
	if strings.Contains(specURL, "/resource-manager/") {
		return "Resource Manager"
	} else if strings.Contains(specURL, "/data-plane/") {
		return "Data Plane"
	}
	return "Azure Service"
}

func extractGoogleCategory(specURL string) string {
	// Extract from path like /compute/v1/
	parts := strings.Split(specURL, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, "v") && i > 0 {
			return cases.Title(language.English).String(parts[i-1])
		}
	}
	return "Google Cloud Service"
}

func extractK8sCategory(specURL string) string {
	if strings.Contains(specURL, "/apis/") {
		// Extract API group
		parts := strings.Split(specURL, "/")
		for i, part := range parts {
			if part == "apis" && i+1 < len(parts) {
				return parts[i+1]
			}
		}
	}
	return "Core"
}

func extractGenericCategory(specURL string) string {
	// Try to extract from common patterns
	if strings.Contains(specURL, "/admin/") {
		return "Admin"
	} else if strings.Contains(specURL, "/public/") {
		return "Public"
	} else if strings.Contains(specURL, "/internal/") {
		return "Internal"
	} else if strings.Contains(specURL, "/v1/") || strings.Contains(specURL, "/v2/") {
		return "Core"
	}
	return "General"
}
