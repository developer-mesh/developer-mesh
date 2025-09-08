package tools

import (
	"fmt"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// OperationResolver intelligently resolves operation IDs from various input formats
type OperationResolver struct {
	logger observability.Logger
	// Cache of operation mappings for quick lookup
	operationCache map[string]*ResolvedOperation
	// Reverse mapping from simple names to full operation IDs
	simpleNameMap map[string][]string
}

// ResolvedOperation contains details about an OpenAPI operation
type ResolvedOperation struct {
	OperationID string
	Path        string
	Method      string
	Tags        []string
	Summary     string
	SimpleName  string // Extracted simple name like "get", "list", "create"
}

// NewOperationResolver creates a new operation resolver
func NewOperationResolver(logger observability.Logger) *OperationResolver {
	return &OperationResolver{
		logger:         logger,
		operationCache: make(map[string]*ResolvedOperation),
		simpleNameMap:  make(map[string][]string),
	}
}

// BuildOperationMappings analyzes an OpenAPI spec and builds operation mappings
func (r *OperationResolver) BuildOperationMappings(spec *openapi3.T, toolName string) error {
	if spec == nil || spec.Paths == nil {
		return fmt.Errorf("invalid OpenAPI spec")
	}

	// Clear existing mappings for this tool
	r.operationCache = make(map[string]*ResolvedOperation)
	r.simpleNameMap = make(map[string][]string)

	// Iterate through all paths and operations
	for path, pathItem := range spec.Paths.Map() {
		for method, operation := range pathItem.Operations() {
			if operation == nil {
				continue
			}

			// Create operation info
			info := &ResolvedOperation{
				OperationID: operation.OperationID,
				Path:        path,
				Method:      method,
				Summary:     operation.Summary,
			}

			// Extract tags
			if operation.Tags != nil {
				info.Tags = operation.Tags
			}

			// Generate various keys for this operation
			keys := r.generateOperationKeys(operation.OperationID, path, method, info.Tags, toolName)

			// Extract simple name from operation ID
			info.SimpleName = r.extractSimpleName(operation.OperationID)

			// Store in cache with all possible keys
			for _, key := range keys {
				r.operationCache[key] = info
			}

			// Map simple name to operation IDs
			if info.SimpleName != "" {
				r.simpleNameMap[info.SimpleName] = append(r.simpleNameMap[info.SimpleName], operation.OperationID)
			}

			r.logger.Debug("Mapped operation", map[string]interface{}{
				"operation_id": operation.OperationID,
				"simple_name":  info.SimpleName,
				"path":         path,
				"method":       method,
				"keys":         keys,
			})
		}
	}

	r.logger.Info("Built operation mappings", map[string]interface{}{
		"tool_name":        toolName,
		"total_operations": len(r.operationCache),
		"simple_names":     len(r.simpleNameMap),
	})

	return nil
}

// ResolveOperation finds the best matching operation for a given action
func (r *OperationResolver) ResolveOperation(action string, context map[string]interface{}) (*ResolvedOperation, error) {
	// Clean and normalize the action
	action = strings.TrimSpace(strings.ToLower(action))

	r.logger.Debug("Resolving operation", map[string]interface{}{
		"action":  action,
		"context": context,
	})

	// Strategy 1: Direct lookup
	if info, ok := r.operationCache[action]; ok {
		r.logger.Debug("Found operation by direct lookup", map[string]interface{}{
			"action":       action,
			"operation_id": info.OperationID,
		})
		return info, nil
	}

	// Strategy 2: Try with context hints (e.g., resource type from params)
	if contextualOp := r.resolveWithContext(action, context); contextualOp != nil {
		return contextualOp, nil
	}

	// Strategy 3: Simple name lookup
	if candidates, ok := r.simpleNameMap[action]; ok && len(candidates) > 0 {
		// If only one candidate, use it
		if len(candidates) == 1 {
			if info, ok := r.operationCache[candidates[0]]; ok {
				r.logger.Debug("Found operation by simple name", map[string]interface{}{
					"action":       action,
					"operation_id": info.OperationID,
				})
				return info, nil
			}
		}

		// Multiple candidates - try to disambiguate with context
		if bestMatch := r.disambiguateWithContext(candidates, context); bestMatch != nil {
			return bestMatch, nil
		}

		// Return first candidate as fallback
		if info, ok := r.operationCache[candidates[0]]; ok {
			r.logger.Warn("Multiple operations match, using first", map[string]interface{}{
				"action":     action,
				"candidates": candidates,
				"selected":   candidates[0],
			})
			return info, nil
		}
	}

	// Strategy 4: Fuzzy matching
	if fuzzyMatch := r.fuzzyMatch(action); fuzzyMatch != nil {
		return fuzzyMatch, nil
	}

	// No match found - return helpful error
	availableOps := r.getAvailableOperations()
	return nil, fmt.Errorf("operation '%s' not found. Available operations: %v", action, availableOps)
}

// resolveWithContext attempts to resolve an operation using context hints
func (r *OperationResolver) resolveWithContext(action string, context map[string]interface{}) *ResolvedOperation {
	// Check for resource hints in context
	resourceHints := []string{}

	// First check if parameters are nested
	var params map[string]interface{}
	if p, ok := context["parameters"].(map[string]interface{}); ok {
		params = p
	} else {
		params = context
	}

	// Check for common parameter names that indicate resource type
	paramNamesToCheck := []string{"owner", "repo", "org", "user", "issue_number", "pull_number", "repository"}
	for _, param := range paramNamesToCheck {
		if _, exists := params[param]; exists {
			resource := r.inferResourceFromParam(param)
			if resource != "" && !contains(resourceHints, resource) {
				resourceHints = append(resourceHints, resource)
			}
		}
	}

	// Special handling for issues_create which needs both repo context
	if action == "create" && hasParam(params, "title") && hasParam(params, "body") {
		if hasParam(params, "owner") && hasParam(params, "repo") {
			// This is likely an issue creation
			resourceHints = append([]string{"issues"}, resourceHints...)
		}
	}

	// Try combining resource hints with action
	for _, resource := range resourceHints {
		// Try different formats
		candidates := []string{
			fmt.Sprintf("%s/%s", resource, action), // repos/get
			fmt.Sprintf("%s-%s", resource, action), // repos-get
			fmt.Sprintf("%s_%s", resource, action), // repos_get
			fmt.Sprintf("%s%s", resource, action),  // reposget
			fmt.Sprintf("%s.%s", resource, action), // repos.get
		}

		for _, candidate := range candidates {
			if info, ok := r.operationCache[candidate]; ok {
				r.logger.Debug("Found operation with context", map[string]interface{}{
					"action":       action,
					"resource":     resource,
					"operation_id": info.OperationID,
					"params":       params,
				})
				return info
			}
		}
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, value string) bool {
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}

// hasParam checks if a parameter exists in a map
func hasParam(params map[string]interface{}, key string) bool {
	_, exists := params[key]
	return exists
}

// disambiguateWithContext selects the best operation from multiple candidates using context
func (r *OperationResolver) disambiguateWithContext(candidates []string, context map[string]interface{}) *ResolvedOperation {
	// Score each candidate based on parameter matches
	bestScore := 0
	var bestMatch *ResolvedOperation

	// Extract resource type from context if available
	resourceType := ""
	if rt, ok := context["__resource_type"].(string); ok {
		resourceType = rt
	}

	for _, candidate := range candidates {
		if info, ok := r.operationCache[candidate]; ok {
			score := r.scoreOperation(info, context)

			// CRITICAL: Prioritize operations that start with the primary resource type
			// This prevents "repos/list-pull-requests-associated-with-commit" from being chosen
			// over "pulls/list-for-repo" when tool is "github_pulls"
			if resourceType != "" {
				// Check if the operation ID starts with the resource type
				if strings.HasPrefix(strings.ToLower(candidate), resourceType+"/") ||
					strings.HasPrefix(strings.ToLower(candidate), resourceType+"-") {
					score += 1000 // Very heavy weight for matching resource type
					r.logger.Debug("Boosting score for resource type match", map[string]interface{}{
						"candidate":     candidate,
						"resource_type": resourceType,
						"boost":         1000,
					})
				} else if strings.Contains(strings.ToLower(candidate), "/"+resourceType+"/") {
					// Secondary match - operation contains resource in path
					score += 50
				}
			}

			if score > bestScore {
				bestScore = score
				bestMatch = info
			}
		}
	}

	if bestMatch != nil && bestScore > 0 {
		r.logger.Debug("Disambiguated operation with context", map[string]interface{}{
			"candidates":    candidates,
			"selected":      bestMatch.OperationID,
			"score":         bestScore,
			"resource_type": resourceType,
		})
		return bestMatch
	}

	return nil
}

// scoreOperation scores how well an operation matches the provided context
func (r *OperationResolver) scoreOperation(info *ResolvedOperation, context map[string]interface{}) int {
	score := 0

	// Check if path contains parameter names from context
	for param := range context {
		// Skip internal parameters
		if strings.HasPrefix(param, "__") {
			continue
		}

		// Check for nested parameters (e.g., parameters.ref)
		if paramsMap, ok := context["parameters"].(map[string]interface{}); ok {
			for nestedParam := range paramsMap {
				// CRITICAL: For Git operations, strongly prefer operations that match the parameter type
				// If we have "ref" parameter, we want create-ref, not create-commit or create-blob
				if nestedParam == "ref" && strings.Contains(strings.ToLower(info.OperationID), "ref") {
					score += 100 // Very high score for ref parameter matching ref operation
					r.logger.Debug("Boosting score for ref parameter match", map[string]interface{}{
						"operation_id": info.OperationID,
						"boost":        100,
					})
				}
				if nestedParam == "tree" && strings.Contains(strings.ToLower(info.OperationID), "tree") {
					score += 100
				}
				if nestedParam == "content" && strings.Contains(strings.ToLower(info.OperationID), "blob") {
					score += 100
				}
			}
		}

		// Direct parameter matching
		if param == "ref" && strings.Contains(strings.ToLower(info.OperationID), "ref") {
			score += 100 // Very high score for ref parameter matching ref operation
		}

		if strings.Contains(info.Path, "{"+param+"}") {
			score += 10 // High score for exact parameter match in path
		}
		if strings.Contains(strings.ToLower(info.OperationID), param) {
			score += 5 // Medium score for operation ID containing param
		}
	}

	// Check for resource type indicators
	if strings.Contains(info.Path, "/repos/") && context["repo"] != nil {
		score += 8
	}
	if strings.Contains(info.Path, "/issues/") && context["issue_number"] != nil {
		score += 8
	}
	if strings.Contains(info.Path, "/pulls/") && context["pull_number"] != nil {
		score += 8
	}
	if strings.Contains(info.Path, "/git/refs") && context["ref"] != nil {
		score += 50 // High score for git refs endpoint when ref parameter present
	}

	return score
}

// fuzzyMatch attempts to find an operation using fuzzy matching
func (r *OperationResolver) fuzzyMatch(action string) *ResolvedOperation {
	// Try common variations
	// Special handling for GitHub Actions operations to preserve hyphenated action names
	var variations []string
	if strings.HasPrefix(action, "actions-") || strings.HasPrefix(action, "actions/") {
		// For actions operations, only replace the first separator
		// e.g., "actions-list-repo-workflows" -> "actions/list-repo-workflows"
		if strings.HasPrefix(action, "actions-") {
			variations = []string{
				action,
				strings.Replace(action, "actions-", "actions/", 1),
			}
		} else {
			variations = []string{
				action,
				strings.Replace(action, "actions/", "actions-", 1),
			}
		}
	} else {
		// Standard variations for other operations
		// Be more conservative with replacements to avoid over-transformation
		variations = []string{
			action,
			strings.ReplaceAll(action, "_", "-"),
			strings.ReplaceAll(action, "-", "_"),
			strings.ReplaceAll(action, "_", "/"),
		}

		// For hyphenated operations, try replacing just the first hyphen with slash
		// This handles cases like "repos-get" -> "repos/get" without breaking
		// compound names like "list-repo-workflows"
		if strings.Contains(action, "-") {
			// Count hyphens to decide replacement strategy
			hyphenCount := strings.Count(action, "-")
			if hyphenCount == 1 {
				// Single hyphen: replace with slash (e.g., "repos-get" -> "repos/get")
				variations = append(variations, strings.ReplaceAll(action, "-", "/"))
			} else {
				// Multiple hyphens: only replace the first one
				// (e.g., "repos-list-for-user" -> "repos/list-for-user")
				idx := strings.Index(action, "-")
				if idx > 0 {
					variations = append(variations, action[:idx]+"/"+action[idx+1:])
				}
			}
		}
	}

	for _, variation := range variations {
		// Check if any operation ID contains this variation
		for opID, info := range r.operationCache {
			if strings.Contains(strings.ToLower(opID), variation) {
				r.logger.Debug("Found operation by fuzzy match", map[string]interface{}{
					"action":       action,
					"variation":    variation,
					"operation_id": info.OperationID,
				})
				return info
			}
		}
	}

	return nil
}

// generateOperationKeys generates all possible keys for an operation
func (r *OperationResolver) generateOperationKeys(operationID, path, method string, tags []string, toolName string) []string {
	keys := []string{}

	// Add operation ID as-is
	if operationID != "" {
		keys = append(keys, operationID)
		keys = append(keys, strings.ToLower(operationID))

		// Add variations
		keys = append(keys, strings.ReplaceAll(operationID, "/", "-"))
		keys = append(keys, strings.ReplaceAll(operationID, "/", "_"))
		keys = append(keys, strings.ReplaceAll(operationID, "-", "_"))
	}

	// Add method + path combinations
	pathParts := strings.Split(strings.Trim(path, "/"), "/")
	if len(pathParts) > 0 {
		// Use last significant path part
		lastPart := ""
		for i := len(pathParts) - 1; i >= 0; i-- {
			if !strings.HasPrefix(pathParts[i], "{") {
				lastPart = pathParts[i]
				break
			}
		}
		if lastPart != "" {
			keys = append(keys, fmt.Sprintf("%s_%s", strings.ToLower(method), lastPart))
			keys = append(keys, fmt.Sprintf("%s-%s", strings.ToLower(method), lastPart))
		}
	}

	// Add tag-based keys
	for _, tag := range tags {
		tagLower := strings.ToLower(tag)
		if operationID != "" {
			// Extract operation name from ID
			parts := strings.Split(operationID, "/")
			if len(parts) > 1 {
				keys = append(keys, fmt.Sprintf("%s_%s", tagLower, parts[len(parts)-1]))
			}
		}
	}

	return keys
}

// extractSimpleName extracts a simple action name from an operation ID
func (r *OperationResolver) extractSimpleName(operationID string) string {
	if operationID == "" {
		return ""
	}

	// Handle different formats
	// "repos/get" -> "get"
	// "pulls/list-for-repo" -> "list"
	// "get-repos" -> "get"
	// "getRepos" -> "get"
	// "repos-get-content" -> "get-content"

	// Split by common delimiters
	parts := strings.FieldsFunc(operationID, func(r rune) bool {
		return r == '/' || r == '-' || r == '_' || r == '.'
	})

	if len(parts) == 0 {
		return ""
	}

	// Common action verbs to look for
	actionVerbs := []string{
		"get", "list", "create", "update", "delete", "patch",
		"post", "put", "remove", "add", "set", "fetch",
		"search", "find", "query", "read", "write",
	}

	// Check each part for action verbs
	for _, part := range parts {
		partLower := strings.ToLower(part)
		for _, verb := range actionVerbs {
			if partLower == verb {
				return verb
			}
			// Check if part starts with verb (e.g., "getAll" -> "get")
			if strings.HasPrefix(partLower, verb) {
				return verb
			}
		}
	}

	// If no verb found, try the last part
	return strings.ToLower(parts[len(parts)-1])
}

// inferResourceFromParam infers a resource type from a parameter name
func (r *OperationResolver) inferResourceFromParam(param string) string {
	switch param {
	case "owner", "repo", "repository":
		return "repos"
	case "org", "organization":
		return "orgs"
	case "user", "username":
		return "users"
	case "issue_number", "issue_id":
		return "issues"
	case "pull_number", "pr_number":
		return "pulls"
	case "gist_id":
		return "gists"
	case "team_id", "team_slug":
		return "teams"
	default:
		// Try to extract resource from parameter name
		if strings.HasSuffix(param, "_id") || strings.HasSuffix(param, "_number") {
			resource := strings.TrimSuffix(strings.TrimSuffix(param, "_id"), "_number")
			return resource + "s" // Pluralize
		}
		return ""
	}
}

// getAvailableOperations returns a list of available operation IDs
func (r *OperationResolver) getAvailableOperations() []string {
	operations := make([]string, 0, len(r.operationCache))
	seen := make(map[string]bool)

	for _, info := range r.operationCache {
		if !seen[info.OperationID] {
			operations = append(operations, info.OperationID)
			seen[info.OperationID] = true
		}

		// Limit to 20 for readability
		if len(operations) >= 20 {
			operations = append(operations, "...")
			break
		}
	}

	return operations
}

// GetOperationSchema returns the full operation info for a resolved operation
func (r *OperationResolver) GetOperationSchema(operationID string) (*ResolvedOperation, error) {
	if info, ok := r.operationCache[operationID]; ok {
		return info, nil
	}
	return nil, fmt.Errorf("operation not found: %s", operationID)
}
