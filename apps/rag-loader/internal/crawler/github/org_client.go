// Package github implements the GitHub data source crawler for the RAG loader
package github

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/v62/github"
	"github.com/google/uuid"
	"golang.org/x/oauth2"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// OrgConfig holds the configuration for GitHub organization-wide crawling
type OrgConfig struct {
	Org             string   `json:"org"`
	Token           string   `json:"token"`
	Repos           []string `json:"repos"`            // Optional: specific repos, empty = all repos
	IncludeArchived bool     `json:"include_archived"` // Default: false
	IncludeForks    bool     `json:"include_forks"`    // Default: false
	IncludePatterns []string `json:"include_patterns"` // File patterns to include
	ExcludePatterns []string `json:"exclude_patterns"` // File patterns to exclude
	Branch          string   `json:"branch"`           // Default branch to use (empty = repo default)
	BaseURL         string   `json:"base_url"`         // GitHub Enterprise base URL (optional, defaults to github.com)
}

// OrgClient handles GitHub organization operations
type OrgClient struct {
	client *github.Client
	logger observability.Logger
}

// NewOrgClient creates a new GitHub organization client
func NewOrgClient(token string, baseURL string, logger observability.Logger) (*OrgClient, error) {
	var httpClient *http.Client
	if token != "" {
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		httpClient = oauth2.NewClient(context.Background(), ts)
	} else {
		httpClient = nil
	}

	tc, err := createGitHubClient(httpClient, baseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to create GitHub client: %w", err)
	}

	return &OrgClient{
		client: tc,
		logger: logger,
	}, nil
}

// ListRepositories lists all repositories in an organization with filtering
func (c *OrgClient) ListRepositories(ctx context.Context, config OrgConfig) ([]string, error) {
	c.logger.Info("Listing repositories for organization", map[string]interface{}{
		"org":              config.Org,
		"include_archived": config.IncludeArchived,
		"include_forks":    config.IncludeForks,
		"repo_filter":      len(config.Repos),
	})

	// If specific repos are configured, use those
	if len(config.Repos) > 0 {
		c.logger.Info("Using configured repository list", map[string]interface{}{
			"org":        config.Org,
			"repo_count": len(config.Repos),
		})
		return config.Repos, nil
	}

	// Otherwise, discover all repos in the org
	var allRepos []string
	opts := &github.RepositoryListByOrgOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		repos, resp, err := c.client.Repositories.ListByOrg(ctx, config.Org, opts)
		if err != nil {
			return nil, fmt.Errorf("failed to list repositories for org %s: %w", config.Org, err)
		}

		for _, repo := range repos {
			// Apply filters
			if !c.shouldIncludeRepo(repo, config) {
				c.logger.Debug("Skipping repository", map[string]interface{}{
					"repo":     repo.GetName(),
					"archived": repo.GetArchived(),
					"fork":     repo.GetFork(),
				})
				continue
			}

			allRepos = append(allRepos, repo.GetName())
		}

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	c.logger.Info("Repository discovery complete", map[string]interface{}{
		"org":        config.Org,
		"repo_count": len(allRepos),
	})

	return allRepos, nil
}

// shouldIncludeRepo determines if a repository should be included based on filters
func (c *OrgClient) shouldIncludeRepo(repo *github.Repository, config OrgConfig) bool {
	// Filter archived repos
	if repo.GetArchived() && !config.IncludeArchived {
		return false
	}

	// Filter forked repos
	if repo.GetFork() && !config.IncludeForks {
		return false
	}

	return true
}

// GetDefaultBranch gets the default branch for a repository
func (c *OrgClient) GetDefaultBranch(ctx context.Context, org, repo string) (string, error) {
	repository, _, err := c.client.Repositories.Get(ctx, org, repo)
	if err != nil {
		return "", fmt.Errorf("failed to get repository %s/%s: %w", org, repo, err)
	}

	defaultBranch := repository.GetDefaultBranch()
	if defaultBranch == "" {
		defaultBranch = "main" // Fallback
	}

	return defaultBranch, nil
}

// CreateCrawlers creates GitHubCrawler instances for all repositories in an organization
func (c *OrgClient) CreateCrawlers(ctx context.Context, tenantID uuid.UUID, orgConfig OrgConfig) ([]*Crawler, error) {
	// List repositories
	repos, err := c.ListRepositories(ctx, orgConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	if len(repos) == 0 {
		c.logger.Warn("No repositories found for organization", map[string]interface{}{
			"org": orgConfig.Org,
		})
		return nil, nil
	}

	// Create crawlers for each repository
	var crawlers []*Crawler
	for _, repoName := range repos {
		// Get default branch if not specified
		branch := orgConfig.Branch
		if branch == "" {
			branch, err = c.GetDefaultBranch(ctx, orgConfig.Org, repoName)
			if err != nil {
				c.logger.Error("Failed to get default branch, using 'main'", map[string]interface{}{
					"org":   orgConfig.Org,
					"repo":  repoName,
					"error": err.Error(),
				})
				branch = "main"
			}
		}

		// Create crawler config
		crawlerConfig := Config{
			Owner:           orgConfig.Org,
			Repo:            repoName,
			Branch:          branch,
			IncludePatterns: orgConfig.IncludePatterns,
			ExcludePatterns: orgConfig.ExcludePatterns,
			Token:           orgConfig.Token,
			BaseURL:         orgConfig.BaseURL,
		}

		// Create crawler
		crawler, err := NewCrawler(tenantID, crawlerConfig)
		if err != nil {
			c.logger.Error("Failed to create crawler for repository", map[string]interface{}{
				"org":   orgConfig.Org,
				"repo":  repoName,
				"error": err.Error(),
			})
			continue
		}

		crawlers = append(crawlers, crawler)

		c.logger.Debug("Created crawler for repository", map[string]interface{}{
			"org":    orgConfig.Org,
			"repo":   repoName,
			"branch": branch,
		})
	}

	c.logger.Info("Created crawlers for organization", map[string]interface{}{
		"org":           orgConfig.Org,
		"crawler_count": len(crawlers),
	})

	return crawlers, nil
}

// ValidateOrgAccess validates that the token has access to the organization
func (c *OrgClient) ValidateOrgAccess(ctx context.Context, org string) error {
	_, _, err := c.client.Organizations.Get(ctx, org)
	if err != nil {
		return fmt.Errorf("failed to access organization %s: %w", org, err)
	}
	return nil
}

// ParseOrgConfig parses the organization configuration from a map
func ParseOrgConfig(config map[string]interface{}) (OrgConfig, error) {
	orgConfig := OrgConfig{
		IncludeArchived: false, // Default to not including archived
		IncludeForks:    false, // Default to not including forks
	}

	// Required: org
	if org, ok := config["org"].(string); ok {
		orgConfig.Org = org
	} else {
		return orgConfig, fmt.Errorf("org is required for github_org source")
	}

	// Required: token
	if token, ok := config["token"].(string); ok {
		orgConfig.Token = token
	} else {
		return orgConfig, fmt.Errorf("token is required for github_org source")
	}

	// Optional: base_url (for GitHub Enterprise)
	if baseURL, ok := config["base_url"].(string); ok {
		orgConfig.BaseURL = strings.TrimSpace(baseURL)
	}

	// Optional: repos
	if repos, ok := config["repos"].([]interface{}); ok {
		for _, r := range repos {
			if repoStr, ok := r.(string); ok {
				orgConfig.Repos = append(orgConfig.Repos, repoStr)
			}
		}
	}

	// Optional: include_archived
	if includeArchived, ok := config["include_archived"].(bool); ok {
		orgConfig.IncludeArchived = includeArchived
	}

	// Optional: include_forks
	if includeForks, ok := config["include_forks"].(bool); ok {
		orgConfig.IncludeForks = includeForks
	}

	// Optional: include_patterns
	if patterns, ok := config["include_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if patternStr, ok := p.(string); ok {
				orgConfig.IncludePatterns = append(orgConfig.IncludePatterns, patternStr)
			}
		}
	}

	// Optional: exclude_patterns
	if patterns, ok := config["exclude_patterns"].([]interface{}); ok {
		for _, p := range patterns {
			if patternStr, ok := p.(string); ok {
				orgConfig.ExcludePatterns = append(orgConfig.ExcludePatterns, patternStr)
			}
		}
	}

	// Optional: branch
	if branch, ok := config["branch"].(string); ok {
		orgConfig.Branch = strings.TrimSpace(branch)
	}

	return orgConfig, nil
}
