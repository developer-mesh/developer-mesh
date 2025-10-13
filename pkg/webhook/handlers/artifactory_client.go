package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// ArtifactoryClient provides methods to interact with JFrog Artifactory REST API
type ArtifactoryClient struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	logger     observability.Logger
}

// NewArtifactoryClient creates a new Artifactory client
func NewArtifactoryClient(baseURL, apiKey string, logger observability.Logger) *ArtifactoryClient {
	if logger == nil {
		logger = observability.NewLogger("artifactory-client")
	}

	return &ArtifactoryClient{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: logger,
	}
}

// ArtifactProperties represents Artifactory artifact properties
type ArtifactProperties struct {
	Repo         string                 `json:"repo"`
	Path         string                 `json:"path"`
	Created      string                 `json:"created"`
	CreatedBy    string                 `json:"createdBy"`
	LastModified string                 `json:"lastModified"`
	ModifiedBy   string                 `json:"modifiedBy"`
	LastUpdated  string                 `json:"lastUpdated"`
	Size         int64                  `json:"size"`
	MimeType     string                 `json:"mimeType"`
	Checksums    ArtifactChecksums      `json:"checksums"`
	URI          string                 `json:"uri"`
	Properties   map[string]interface{} `json:"properties"`
}

// ArtifactChecksums contains artifact checksums
type ArtifactChecksums struct {
	SHA1   string `json:"sha1"`
	MD5    string `json:"md5"`
	SHA256 string `json:"sha256"`
}

// BuildInfo represents Artifactory build information
type BuildInfo struct {
	BuildName     string                 `json:"name"`
	BuildNumber   string                 `json:"number"`
	BuildAgent    BuildAgent             `json:"buildAgent"`
	Started       string                 `json:"started"`
	DurationMs    int64                  `json:"durationMillis"`
	Principal     string                 `json:"principal"`
	ArtifactorURI string                 `json:"artifactoryPrincipal"`
	URL           string                 `json:"url"`
	VCS           []VCSInfo              `json:"vcs"`
	Modules       []BuildModule          `json:"modules"`
	Properties    map[string]interface{} `json:"properties"`
}

// BuildAgent represents the build agent information
type BuildAgent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// VCSInfo represents version control system information
type VCSInfo struct {
	URL      string `json:"url"`
	Revision string `json:"revision"`
	Branch   string `json:"branch"`
}

// BuildModule represents a build module
type BuildModule struct {
	ID           string          `json:"id"`
	Artifacts    []BuildArtifact `json:"artifacts"`
	Dependencies []BuildArtifact `json:"dependencies"`
}

// BuildArtifact represents an artifact in a build
type BuildArtifact struct {
	Type string `json:"type"`
	SHA1 string `json:"sha1"`
	MD5  string `json:"md5"`
	Name string `json:"name"`
}

// GetArtifactProperties fetches properties and metadata for an artifact
func (c *ArtifactoryClient) GetArtifactProperties(ctx context.Context, repoKey, path string) (map[string]interface{}, error) {
	url := fmt.Sprintf("%s/api/storage/%s/%s?properties", c.baseURL, repoKey, path)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set authentication header
	req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artifact properties: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifactory API returned status %d: %s", resp.StatusCode, string(body))
	}

	var properties ArtifactProperties
	if err := json.NewDecoder(resp.Body).Decode(&properties); err != nil {
		return nil, fmt.Errorf("failed to decode artifact properties: %w", err)
	}

	// Convert to map for storage
	result := map[string]interface{}{
		"repo":          properties.Repo,
		"path":          properties.Path,
		"created":       properties.Created,
		"created_by":    properties.CreatedBy,
		"last_modified": properties.LastModified,
		"modified_by":   properties.ModifiedBy,
		"size":          properties.Size,
		"mime_type":     properties.MimeType,
		"checksums": map[string]string{
			"sha1":   properties.Checksums.SHA1,
			"md5":    properties.Checksums.MD5,
			"sha256": properties.Checksums.SHA256,
		},
		"properties": properties.Properties,
	}

	return result, nil
}

// GetBuildInfo fetches build information from Artifactory
func (c *ArtifactoryClient) GetBuildInfo(ctx context.Context, buildName, buildNumber string) (*BuildInfo, error) {
	url := fmt.Sprintf("%s/api/build/%s/%s", c.baseURL, buildName, buildNumber)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch build info: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifactory API returned status %d: %s", resp.StatusCode, string(body))
	}

	var buildInfo BuildInfo
	if err := json.NewDecoder(resp.Body).Decode(&buildInfo); err != nil {
		return nil, fmt.Errorf("failed to decode build info: %w", err)
	}

	return &buildInfo, nil
}

// SearchArtifactsByChecksum searches for artifacts by checksum
func (c *ArtifactoryClient) SearchArtifactsByChecksum(ctx context.Context, checksum string) ([]ArtifactSearchResult, error) {
	url := fmt.Sprintf("%s/api/search/checksum?sha256=%s", c.baseURL, checksum)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artifacts: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifactory API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Results []ArtifactSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return searchResponse.Results, nil
}

// ArtifactSearchResult represents a search result
type ArtifactSearchResult struct {
	Repo string `json:"repo"`
	Path string `json:"path"`
	Name string `json:"name"`
	Type string `json:"type"`
	Size int64  `json:"size"`
}

// SearchArtifactsByProperty searches for artifacts by property
func (c *ArtifactoryClient) SearchArtifactsByProperty(ctx context.Context, key, value string) ([]ArtifactSearchResult, error) {
	url := fmt.Sprintf("%s/api/search/prop?%s=%s", c.baseURL, key, value)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artifacts: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifactory API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Results []ArtifactSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return searchResponse.Results, nil
}

// SearchArtifactsByGAVC searches Maven artifacts by Group/Artifact/Version/Classifier
func (c *ArtifactoryClient) SearchArtifactsByGAVC(ctx context.Context, groupID, artifactID, version string) ([]ArtifactSearchResult, error) {
	url := fmt.Sprintf("%s/api/search/gavc?g=%s&a=%s&v=%s", c.baseURL, groupID, artifactID, version)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-JFrog-Art-Api", c.apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to search artifacts: %w", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			c.logger.Warn("Failed to close response body", map[string]interface{}{
				"error": err.Error(),
			})
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("artifactory API returned status %d: %s", resp.StatusCode, string(body))
	}

	var searchResponse struct {
		Results []ArtifactSearchResult `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&searchResponse); err != nil {
		return nil, fmt.Errorf("failed to decode search results: %w", err)
	}

	return searchResponse.Results, nil
}
