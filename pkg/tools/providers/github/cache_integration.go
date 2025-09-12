package github

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// CachedExecutor wraps a handler's Execute method with caching logic
type CachedExecutor struct {
	provider *GitHubProvider
	handler  ToolHandler
	cacheKey func(params map[string]interface{}) string
	cacheTTL time.Duration
	readOnly bool // Whether this is a read-only operation that can be cached
}

// NewCachedExecutor creates a new cached executor for a handler
func NewCachedExecutor(
	provider *GitHubProvider,
	handler ToolHandler,
	keyBuilder func(params map[string]interface{}) string,
	ttl time.Duration,
	readOnly bool,
) *CachedExecutor {
	return &CachedExecutor{
		provider: provider,
		handler:  handler,
		cacheKey: keyBuilder,
		cacheTTL: ttl,
		readOnly: readOnly,
	}
}

// Execute runs the handler with caching logic
func (ce *CachedExecutor) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	// Only cache read-only operations when caching is enabled
	if !ce.readOnly || !ce.provider.cacheEnabled || ce.provider.cache == nil {
		return ce.handler.Execute(ctx, params)
	}

	// Build cache key
	key := ce.cacheKey(params)

	// Try to get from cache
	if cached, found := ce.provider.cache.Get(key); found {
		ce.provider.logger.Debug("Cache hit", map[string]interface{}{
			"operation": ce.handler.GetDefinition().Name,
			"key":       key,
		})

		// Convert cached value back to ToolResult
		if result, ok := cached.(*ToolResult); ok {
			return result, nil
		}
	}

	// Cache miss - execute the handler
	ce.provider.logger.Debug("Cache miss", map[string]interface{}{
		"operation": ce.handler.GetDefinition().Name,
		"key":       key,
	})

	result, err := ce.handler.Execute(ctx, params)
	if err != nil {
		return result, err
	}

	// Cache successful results only
	if result != nil && !result.IsError {
		ce.provider.cache.Set(key, result, ce.cacheTTL)
		ce.provider.logger.Debug("Cached result", map[string]interface{}{
			"operation": ce.handler.GetDefinition().Name,
			"key":       key,
			"ttl":       ce.cacheTTL.String(),
		})
	}

	return result, nil
}

// GetDefinition returns the underlying handler's definition
func (ce *CachedExecutor) GetDefinition() ToolDefinition {
	return ce.handler.GetDefinition()
}

// Example cache key builders for common operations

// RepositoryCacheKeyBuilder builds cache keys for repository operations
func RepositoryCacheKeyBuilder(operation string) func(params map[string]interface{}) string {
	return func(params map[string]interface{}) string {
		owner := extractString(params, "owner")
		repo := extractString(params, "repo")

		// Include pagination in key if present
		page := extractInt(params, "page")
		perPage := extractInt(params, "per_page")

		if page > 0 || perPage > 0 {
			return fmt.Sprintf("github:repo:%s:%s:%s:p%d:pp%d", owner, repo, operation, page, perPage)
		}

		return BuildRepositoryCacheKey(owner, repo, operation)
	}
}

// IssueCacheKeyBuilder builds cache keys for issue operations
func IssueCacheKeyBuilder(operation string) func(params map[string]interface{}) string {
	return func(params map[string]interface{}) string {
		owner := extractString(params, "owner")
		repo := extractString(params, "repo")
		issueNum := extractInt(params, "issue_number")

		if issueNum > 0 {
			return BuildIssueCacheKey(owner, repo, issueNum, operation)
		}

		// For list operations
		state := extractString(params, "state")
		labels := extractString(params, "labels")
		page := extractInt(params, "page")

		return fmt.Sprintf("github:issues:%s:%s:%s:%s:%s:p%d",
			owner, repo, operation, state, labels, page)
	}
}

// PullRequestCacheKeyBuilder builds cache keys for PR operations
func PullRequestCacheKeyBuilder(operation string) func(params map[string]interface{}) string {
	return func(params map[string]interface{}) string {
		owner := extractString(params, "owner")
		repo := extractString(params, "repo")
		prNum := extractInt(params, "pull_number")

		if prNum > 0 {
			return BuildPullRequestCacheKey(owner, repo, prNum, operation)
		}

		// For list operations
		state := extractString(params, "state")
		page := extractInt(params, "page")

		return fmt.Sprintf("github:prs:%s:%s:%s:%s:p%d",
			owner, repo, operation, state, page)
	}
}

// SearchCacheKeyBuilder builds cache keys for search operations
func SearchCacheKeyBuilder(searchType string) func(params map[string]interface{}) string {
	return func(params map[string]interface{}) string {
		query := extractString(params, "query")
		page := extractInt(params, "page")
		sort := extractString(params, "sort")
		order := extractString(params, "order")

		// Create a hash of the query to keep key length reasonable
		queryHash := fmt.Sprintf("%x", query)
		if len(queryHash) > 20 {
			queryHash = queryHash[:20]
		}

		return fmt.Sprintf("github:search:%s:%s:%s:%s:p%d",
			searchType, queryHash, sort, order, page)
	}
}

// WrapWithCache wraps a handler with caching for read operations
func WrapWithCache(provider *GitHubProvider, handler ToolHandler) ToolHandler {
	def := handler.GetDefinition()

	// Determine if this is a read-only operation
	readOnly := isReadOnlyOperation(def.Name)

	// Determine TTL based on operation type
	ttl := GetRecommendedTTL(getOperationType(def.Name))

	// Build appropriate key builder
	var keyBuilder func(params map[string]interface{}) string

	switch {
	case contains(def.Name, "repository") || contains(def.Name, "repo"):
		keyBuilder = RepositoryCacheKeyBuilder(def.Name)
	case contains(def.Name, "issue"):
		keyBuilder = IssueCacheKeyBuilder(def.Name)
	case contains(def.Name, "pull"):
		keyBuilder = PullRequestCacheKeyBuilder(def.Name)
	case contains(def.Name, "search"):
		searchType := getSearchType(def.Name)
		keyBuilder = SearchCacheKeyBuilder(searchType)
	default:
		// Generic key builder
		keyBuilder = func(params map[string]interface{}) string {
			data, _ := json.Marshal(params)
			return fmt.Sprintf("github:%s:%x", def.Name, data)
		}
	}

	return NewCachedExecutor(provider, handler, keyBuilder, ttl, readOnly)
}

// Helper functions

func isReadOnlyOperation(name string) bool {
	readOnlyPrefixes := []string{
		"get_", "list_", "search_", "find_",
	}

	for _, prefix := range readOnlyPrefixes {
		if contains(name, prefix) {
			return true
		}
	}

	return false
}

func getOperationType(name string) string {
	switch {
	case contains(name, "notification"):
		return "notifications"
	case contains(name, "workflow"):
		return "workflow_runs"
	case contains(name, "issue"):
		return "issues"
	case contains(name, "pull"):
		return "pulls"
	case contains(name, "commit"):
		return "commits"
	case contains(name, "branch"):
		return "branches"
	case contains(name, "repository") || contains(name, "repo"):
		return "repositories"
	case contains(name, "organization") || contains(name, "org"):
		return "organizations"
	case contains(name, "user"):
		return "users"
	case contains(name, "team"):
		return "teams"
	default:
		return ""
	}
}

func getSearchType(name string) string {
	switch {
	case contains(name, "repository"):
		return "repositories"
	case contains(name, "code"):
		return "code"
	case contains(name, "issue"):
		return "issues"
	case contains(name, "pull"):
		return "pulls"
	case contains(name, "user"):
		return "users"
	case contains(name, "org"):
		return "orgs"
	default:
		return "general"
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
