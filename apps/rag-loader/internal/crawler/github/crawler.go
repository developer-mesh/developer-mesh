// Package github implements the GitHub data source crawler for the RAG loader
package github

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/go-github/v62/github"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/developer-mesh/developer-mesh/pkg/rag/interfaces"
	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// Crawler implements the DataSource interface for GitHub repositories
type Crawler struct {
	client   *github.Client
	config   Config
	tenantID uuid.UUID
}

// Config holds the configuration for the GitHub crawler
type Config struct {
	Owner           string   `json:"owner"`
	Repo            string   `json:"repo"`
	Branch          string   `json:"branch"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	Token           string   `json:"token"`    // GitHub personal access token
	BaseURL         string   `json:"base_url"` // GitHub Enterprise base URL (optional, defaults to github.com)
}

// NewCrawler creates a new GitHub crawler from configuration
func NewCrawler(tenantID uuid.UUID, config Config) (*Crawler, error) {
	// Create HTTP client with comprehensive timeout configuration
	httpClient := &http.Client{
		Timeout: 60 * time.Second, // Overall request timeout
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSHandshakeTimeout: 10 * time.Second,
			DialContext: (&net.Dialer{
				Timeout:   10 * time.Second, // Connection timeout
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ResponseHeaderTimeout: 30 * time.Second, // Time to receive response headers
		},
	}

	// Create GitHub client with timeout-configured HTTP client
	var tc *github.Client
	var err error

	if config.Token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: config.Token})
		// Wrap the timeout-configured HTTP client with OAuth2
		ctx := context.WithValue(context.Background(), oauth2.HTTPClient, httpClient)
		oauthClient := oauth2.NewClient(ctx, ts)
		tc, err = createGitHubClient(oauthClient, config.BaseURL)
	} else {
		tc, err = createGitHubClient(httpClient, config.BaseURL)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &Crawler{
		client:   tc,
		config:   config,
		tenantID: tenantID,
	}, nil
}

// createGitHubClient creates a GitHub client with support for GitHub Enterprise
func createGitHubClient(httpClient *http.Client, baseURL string) (*github.Client, error) {
	// Default to github.com if no base URL provided
	if baseURL == "" || baseURL == "https://github.com" || baseURL == "https://api.github.com" {
		return github.NewClient(httpClient), nil
	}

	// For GitHub Enterprise, we need both the base URL and upload URL
	// The go-github library expects:
	// - baseURL: https://github.company.com/api/v3/ (with trailing slash)
	// - uploadURL: https://github.company.com/api/uploads/ (with trailing slash)

	// Normalize the base URL
	baseURL = strings.TrimSuffix(baseURL, "/")

	// Construct API URL
	apiURL := baseURL
	if !strings.HasSuffix(apiURL, "/api/v3") {
		apiURL = apiURL + "/api/v3/"
	} else {
		apiURL = apiURL + "/"
	}

	// Construct upload URL
	uploadURL := strings.TrimSuffix(baseURL, "/api/v3")
	if !strings.HasSuffix(uploadURL, "/api/uploads") {
		uploadURL = uploadURL + "/api/uploads/"
	} else {
		uploadURL = uploadURL + "/"
	}

	// Create client with Enterprise URLs
	client, err := github.NewClient(httpClient).WithEnterpriseURLs(apiURL, uploadURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub Enterprise client: %w", err)
	}

	return client, nil
}

// ID returns a unique identifier for this source
func (c *Crawler) ID() string {
	return fmt.Sprintf("github_%s_%s", c.config.Owner, c.config.Repo)
}

// Type returns the type of data source
func (c *Crawler) Type() string {
	return models.SourceTypeGitHub
}

// Fetch retrieves documents from the GitHub repository
func (c *Crawler) Fetch(ctx context.Context, since *time.Time) ([]*models.Document, error) {
	var documents []*models.Document

	// Get repository tree
	branch := c.config.Branch
	if branch == "" {
		branch = "main"
	}

	tree, resp, err := c.client.Git.GetTree(ctx, c.config.Owner, c.config.Repo, branch, true)
	if err != nil {
		if resp != nil && resp.StatusCode == 404 {
			// Try 'master' as fallback
			tree, _, err = c.client.Git.GetTree(ctx, c.config.Owner, c.config.Repo, "master", true)
			if err != nil {
				return nil, fmt.Errorf("failed to get repository tree: %w", err)
			}
			branch = "master"
		} else {
			return nil, fmt.Errorf("failed to get repository tree: %w", err)
		}
	}

	// Process each file in the tree
	for _, entry := range tree.Entries {
		if entry.Type != nil && *entry.Type == "blob" && entry.Path != nil {
			// Apply file filtering
			if !c.shouldProcess(*entry.Path) {
				continue
			}

			// Fetch file content
			doc, err := c.fetchFile(ctx, entry, branch)
			if err != nil {
				// Log error but continue processing other files
				continue
			}

			documents = append(documents, doc)
		}
	}

	return documents, nil
}

// fetchFile retrieves a single file from the repository
func (c *Crawler) fetchFile(ctx context.Context, entry *github.TreeEntry, branch string) (*models.Document, error) {
	// Get file content
	fileContent, _, _, err := c.client.Repositories.GetContents(
		ctx,
		c.config.Owner,
		c.config.Repo,
		*entry.Path,
		&github.RepositoryContentGetOptions{Ref: branch},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get file content: %w", err)
	}

	// Decode content
	content, err := fileContent.GetContent()
	if err != nil {
		return nil, fmt.Errorf("failed to decode file content: %w", err)
	}

	// Create content hash for deduplication
	hash := sha256.Sum256([]byte(content))
	hashStr := hex.EncodeToString(hash[:])

	// Build document
	doc := &models.Document{
		ID:          uuid.New(),
		TenantID:    c.tenantID,
		SourceID:    c.ID(),
		SourceType:  models.SourceTypeGitHub,
		URL:         fileContent.GetHTMLURL(),
		Title:       *entry.Path,
		Content:     content,
		ContentHash: hashStr,
		Metadata: map[string]interface{}{
			"path":   *entry.Path,
			"sha":    *entry.SHA,
			"size":   *entry.Size,
			"branch": branch,
			"owner":  c.config.Owner,
			"repo":   c.config.Repo,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Calculate scoring components
	doc.BaseScore = c.calculateBaseScore(doc)
	doc.FreshnessScore = 1.0 // New content is always fresh
	doc.AuthorityScore = 0.5 // Default authority
	doc.PopularityScore = 0.5
	doc.QualityScore = c.calculateQualityScore(doc)

	// Calculate final importance score (weighted average)
	doc.ImportanceScore = (doc.BaseScore*0.3 +
		doc.FreshnessScore*0.2 +
		doc.AuthorityScore*0.2 +
		doc.PopularityScore*0.2 +
		doc.QualityScore*0.1)

	return doc, nil
}

// calculateBaseScore determines the inherent importance of a document
func (c *Crawler) calculateBaseScore(doc *models.Document) float64 {
	score := 0.5 // Base score

	// Boost README files
	if strings.ToLower(filepath.Base(doc.Title)) == "readme.md" {
		score += 0.3
	}

	// Boost documentation files
	if strings.Contains(strings.ToLower(doc.Title), "doc") {
		score += 0.2
	}

	// Boost main source directories
	if strings.HasPrefix(doc.Title, "pkg/") || strings.HasPrefix(doc.Title, "src/") {
		score += 0.1
	}

	// Boost API and interface files
	if strings.Contains(strings.ToLower(doc.Title), "interface") ||
		strings.Contains(strings.ToLower(doc.Title), "api") {
		score += 0.15
	}

	return math.Min(score, 1.0)
}

// calculateQualityScore assesses the quality of the document
func (c *Crawler) calculateQualityScore(doc *models.Document) float64 {
	score := 0.5

	// Length indicators (well-documented files tend to be longer)
	contentLen := len(doc.Content)
	if contentLen > 500 && contentLen < 50000 { // Sweet spot
		score += 0.2
	} else if contentLen > 50000 { // Too long might be less useful
		score += 0.1
	}

	// Has proper structure (for markdown)
	if strings.HasSuffix(doc.Title, ".md") {
		if strings.Contains(doc.Content, "#") { // Has headers
			score += 0.1
		}
		if strings.Contains(doc.Content, "```") { // Has code blocks
			score += 0.1
		}
	}

	// Code files with comments
	if strings.HasSuffix(doc.Title, ".go") || strings.HasSuffix(doc.Title, ".js") {
		if strings.Contains(doc.Content, "//") || strings.Contains(doc.Content, "/*") {
			score += 0.1
		}
	}

	return math.Min(score, 1.0)
}

// shouldProcess determines if a file should be processed based on patterns
func (c *Crawler) shouldProcess(path string) bool {
	// Check exclude patterns first
	for _, pattern := range c.config.ExcludePatterns {
		matched, err := filepath.Match(pattern, path)
		if err == nil && matched {
			return false
		}
		// Also check if the path contains the pattern (for directory exclusions)
		if strings.Contains(path, strings.TrimSuffix(pattern, "/**")) {
			return false
		}
	}

	// If no include patterns specified, include everything not excluded
	if len(c.config.IncludePatterns) == 0 {
		return true
	}

	// Check include patterns
	for _, pattern := range c.config.IncludePatterns {
		matched, err := filepath.Match(pattern, filepath.Base(path))
		if err == nil && matched {
			return true
		}
	}

	return false
}

// GetMetadata returns source-specific metadata
func (c *Crawler) GetMetadata() map[string]interface{} {
	return map[string]interface{}{
		"owner":            c.config.Owner,
		"repo":             c.config.Repo,
		"branch":           c.config.Branch,
		"include_patterns": c.config.IncludePatterns,
		"exclude_patterns": c.config.ExcludePatterns,
	}
}

// Validate checks if the source configuration is valid
func (c *Crawler) Validate() error {
	if c.config.Owner == "" {
		return fmt.Errorf("owner is required")
	}
	if c.config.Repo == "" {
		return fmt.Errorf("repo is required")
	}
	return nil
}

// HealthCheck verifies the source is accessible and working
func (c *Crawler) HealthCheck(ctx context.Context) error {
	_, _, err := c.client.Repositories.Get(ctx, c.config.Owner, c.config.Repo)
	if err != nil {
		return fmt.Errorf("failed to access repository: %w", err)
	}
	return nil
}

// ParseRepoConfig parses the repository configuration from a map
func ParseRepoConfig(config map[string]interface{}) (Config, error) {
	repoConfig := Config{}

	// Required: owner
	if owner, ok := config["owner"].(string); ok {
		repoConfig.Owner = owner
	} else {
		return repoConfig, fmt.Errorf("owner is required for github source")
	}

	// Required: repo
	if repo, ok := config["repo"].(string); ok {
		repoConfig.Repo = repo
	} else {
		return repoConfig, fmt.Errorf("repo is required for github source")
	}

	// Required: token
	if token, ok := config["token"].(string); ok {
		repoConfig.Token = token
	} else {
		return repoConfig, fmt.Errorf("token is required for github source")
	}

	// Optional: branch
	if branch, ok := config["branch"].(string); ok {
		repoConfig.Branch = branch
	}

	// Optional: base_url (for GitHub Enterprise)
	if baseURL, ok := config["base_url"].(string); ok {
		repoConfig.BaseURL = strings.TrimSpace(baseURL)
	}

	// Optional: include_patterns
	if patterns, ok := config["include_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if patternStr, ok := p.(string); ok {
				repoConfig.IncludePatterns = append(repoConfig.IncludePatterns, patternStr)
			}
		}
	}

	// Optional: exclude_patterns
	if patterns, ok := config["exclude_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if patternStr, ok := p.(string); ok {
				repoConfig.ExcludePatterns = append(repoConfig.ExcludePatterns, patternStr)
			}
		}
	}

	return repoConfig, nil
}

// Ensure Crawler implements interfaces.DataSource
var _ interfaces.DataSource = (*Crawler)(nil)
