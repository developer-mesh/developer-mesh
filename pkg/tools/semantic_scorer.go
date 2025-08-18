package tools

import (
	"regexp"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/getkin/kin-openapi/openapi3"
)

// SemanticScorer provides intelligent operation scoring based on semantic understanding
// This is completely tool-agnostic and works with ANY OpenAPI spec
type SemanticScorer struct {
	logger observability.Logger
}

// NewSemanticScorer creates a new semantic scorer
func NewSemanticScorer(logger observability.Logger) *SemanticScorer {
	return &SemanticScorer{
		logger: logger,
	}
}

// OperationCharacteristics extracts semantic characteristics from an operation
type OperationCharacteristics struct {
	// Complexity score (0-100) - simpler operations are preferred for common actions
	Complexity int

	// Is this a primary CRUD operation?
	IsCRUD   bool
	CRUDType string // "create", "read", "update", "delete", "list"

	// Parameter complexity
	RequiredParamCount int
	OptionalParamCount int
	HasComplexParams   bool // Has nested objects or arrays

	// Path depth (shorter paths often indicate primary operations)
	PathDepth    int
	PathSegments []string

	// Semantic indicators from summary/description
	IsListOperation   bool
	IsSingleResource  bool
	IsSubResource     bool
	IsActionOperation bool // Non-CRUD action like "archive", "publish"

	// Response characteristics
	ReturnsArray  bool
	ReturnsSingle bool

	// Extracted action verb
	PrimaryVerb    string
	SecondaryVerbs []string
}

// ScoreOperation provides a comprehensive score for operation matching
func (s *SemanticScorer) ScoreOperation(
	operation *openapi3.Operation,
	operationID string,
	path string,
	method string,
	action string,
	context map[string]interface{},
) int {
	score := 0

	// Extract operation characteristics
	chars := s.extractCharacteristics(operation, operationID, path, method)

	// 1. Action verb matching with semantic understanding
	actionScore := s.scoreActionMatch(action, chars)
	score += actionScore

	// 2. Complexity scoring - prefer simpler operations for common actions
	complexityScore := s.scoreComplexity(chars, action)
	score += complexityScore

	// 3. Parameter matching with context
	paramScore := s.scoreParameterMatch(operation, context, chars)
	score += paramScore

	// 4. Path pattern scoring
	pathScore := s.scorePathPattern(path, chars, context)
	score += pathScore

	// 5. Response type matching
	responseScore := s.scoreResponseType(operation, action, chars)
	score += responseScore

	// 6. Tag relevance scoring
	tagScore := s.scoreTagRelevance(operation, context)
	score += tagScore

	s.logger.Debug("Semantic scoring complete", map[string]interface{}{
		"operation_id":     operationID,
		"action":           action,
		"total_score":      score,
		"action_score":     actionScore,
		"complexity_score": complexityScore,
		"param_score":      paramScore,
		"path_score":       pathScore,
		"response_score":   responseScore,
		"tag_score":        tagScore,
	})

	return score
}

// extractCharacteristics analyzes an operation to extract semantic characteristics
func (s *SemanticScorer) extractCharacteristics(
	operation *openapi3.Operation,
	operationID string,
	path string,
	method string,
) *OperationCharacteristics {
	chars := &OperationCharacteristics{
		PathSegments: strings.Split(strings.Trim(path, "/"), "/"),
	}

	// Calculate path depth
	chars.PathDepth = len(chars.PathSegments)

	// Analyze operation ID and summary for semantic clues
	combinedText := strings.ToLower(operationID + " " + operation.Summary + " " + operation.Description)

	// Detect CRUD operations
	chars.IsCRUD, chars.CRUDType = s.detectCRUDType(method, combinedText, path)

	// Detect list vs single operations
	chars.IsListOperation = s.detectListOperation(combinedText, path, operation)
	chars.IsSingleResource = s.detectSingleResource(path, operation)

	// Check if this is a sub-resource operation
	chars.IsSubResource = strings.Count(path, "/{") > 2 || strings.Contains(path, "}/") && strings.Count(path, "/") > 3

	// Count parameters
	if operation.Parameters != nil {
		for _, param := range operation.Parameters {
			if param.Value != nil {
				if param.Value.Required {
					chars.RequiredParamCount++
				} else {
					chars.OptionalParamCount++
				}

				// Check for complex parameters
				if param.Value.Schema != nil && param.Value.Schema.Value != nil {
					schemaType := param.Value.Schema.Value.Type
					if schemaType != nil && (schemaType.Is("object") || schemaType.Is("array")) {
						chars.HasComplexParams = true
					}
				}
			}
		}
	}

	// Check request body complexity
	if operation.RequestBody != nil && operation.RequestBody.Value != nil {
		chars.RequiredParamCount++    // Body counts as a required param
		chars.HasComplexParams = true // Bodies are complex by nature
	}

	// Calculate complexity score
	chars.Complexity = chars.RequiredParamCount*10 + chars.OptionalParamCount*5
	if chars.HasComplexParams {
		chars.Complexity += 20
	}
	if chars.IsSubResource {
		chars.Complexity += 15
	}

	// Extract primary verb
	chars.PrimaryVerb = s.extractPrimaryVerb(operationID, combinedText)

	// Analyze response type
	if operation.Responses != nil {
		for status, response := range operation.Responses.Map() {
			if strings.HasPrefix(status, "2") && response.Value != nil {
				if response.Value.Content != nil {
					for _, mediaType := range response.Value.Content {
						if mediaType.Schema != nil && mediaType.Schema.Value != nil {
							schemaType := mediaType.Schema.Value.Type
							if schemaType != nil && schemaType.Is("array") {
								chars.ReturnsArray = true
							} else {
								chars.ReturnsSingle = true
							}
						}
					}
				}
			}
		}
	}

	return chars
}

// detectCRUDType determines if this is a CRUD operation and what type
func (s *SemanticScorer) detectCRUDType(method string, text string, path string) (bool, string) {
	method = strings.ToUpper(method)

	// Method-based detection
	switch method {
	case "GET":
		if strings.Contains(text, "list") || strings.Contains(text, "search") || strings.Contains(text, "find all") {
			return true, "list"
		}
		return true, "read"
	case "POST":
		if strings.Contains(text, "create") || strings.Contains(text, "add") || strings.Contains(text, "new") {
			return true, "create"
		}
	case "PUT", "PATCH":
		return true, "update"
	case "DELETE":
		return true, "delete"
	}

	// Text-based detection for non-standard methods
	if strings.Contains(text, "create") || strings.Contains(text, "add") {
		return true, "create"
	}
	if strings.Contains(text, "list") || strings.Contains(text, "get all") {
		return true, "list"
	}
	if strings.Contains(text, "get") || strings.Contains(text, "fetch") || strings.Contains(text, "retrieve") {
		return true, "read"
	}
	if strings.Contains(text, "update") || strings.Contains(text, "modify") || strings.Contains(text, "edit") {
		return true, "update"
	}
	if strings.Contains(text, "delete") || strings.Contains(text, "remove") {
		return true, "delete"
	}

	return false, ""
}

// detectListOperation determines if this returns multiple items
func (s *SemanticScorer) detectListOperation(text string, path string, operation *openapi3.Operation) bool {
	// Check text indicators
	listIndicators := []string{"list", "all", "search", "find", "query", "browse", "index"}
	for _, indicator := range listIndicators {
		if strings.Contains(text, indicator) {
			return true
		}
	}

	// Check if path ends with a resource name (plural) without an ID
	if !strings.Contains(path, "{") || !strings.HasSuffix(path, "}") {
		pathParts := strings.Split(path, "/")
		if len(pathParts) > 0 {
			lastPart := pathParts[len(pathParts)-1]
			// Common plural patterns
			if strings.HasSuffix(lastPart, "s") || strings.HasSuffix(lastPart, "es") || strings.HasSuffix(lastPart, "ies") {
				return true
			}
		}
	}

	return false
}

// detectSingleResource determines if this operates on a single resource
func (s *SemanticScorer) detectSingleResource(path string, operation *openapi3.Operation) bool {
	// Check if path has an ID parameter at the end
	return strings.HasSuffix(path, "}") && strings.Contains(path, "{") &&
		(strings.Contains(path, "_id}") || strings.Contains(path, "Id}") ||
			strings.Contains(path, "_number}") || strings.Contains(path, "_name}"))
}

// extractPrimaryVerb extracts the main action verb from the operation
func (s *SemanticScorer) extractPrimaryVerb(operationID string, text string) string {
	// Common action verbs in priority order
	verbs := []string{
		"list", "get", "create", "update", "delete", "search",
		"fetch", "retrieve", "add", "modify", "remove", "find",
		"query", "post", "put", "patch", "set", "browse",
	}

	// First check operation ID
	opIDLower := strings.ToLower(operationID)
	for _, verb := range verbs {
		if strings.Contains(opIDLower, verb) {
			return verb
		}
	}

	// Then check text
	for _, verb := range verbs {
		if strings.Contains(text, verb) {
			return verb
		}
	}

	// Extract from operation ID pattern (e.g., "repos/list-for-user" -> "list")
	if strings.Contains(operationID, "/") {
		parts := strings.Split(operationID, "/")
		if len(parts) > 1 {
			// Check second part for verb
			secondPart := strings.ToLower(parts[1])
			for _, verb := range verbs {
				if strings.HasPrefix(secondPart, verb) {
					return verb
				}
			}
		}
	}

	return ""
}

// scoreActionMatch scores how well the action matches the operation
func (s *SemanticScorer) scoreActionMatch(action string, chars *OperationCharacteristics) int {
	score := 0
	actionLower := strings.ToLower(action)

	// Direct match with primary verb
	if chars.PrimaryVerb == actionLower {
		score += 100
	} else if strings.Contains(chars.PrimaryVerb, actionLower) {
		score += 50
	}

	// CRUD type matching
	if chars.IsCRUD {
		switch actionLower {
		case "list", "index":
			if chars.CRUDType == "list" {
				score += 80
			}
		case "get", "read", "fetch", "retrieve":
			switch chars.CRUDType {
			case "read":
				score += 80
			case "list":
				score += 30 // "get" might mean list in some contexts
			}
		case "create", "add", "new", "post":
			if chars.CRUDType == "create" {
				score += 80
			}
		case "update", "edit", "modify", "patch", "put":
			if chars.CRUDType == "update" {
				score += 80
			}
		case "delete", "remove", "destroy":
			if chars.CRUDType == "delete" {
				score += 80
			}
		}
	}

	return score
}

// scoreComplexity scores based on operation complexity
func (s *SemanticScorer) scoreComplexity(chars *OperationCharacteristics, action string) int {
	score := 0

	// For common actions, prefer simpler operations
	commonActions := map[string]bool{
		"list": true, "get": true, "create": true,
		"update": true, "delete": true,
	}

	if commonActions[strings.ToLower(action)] {
		// Inverse complexity scoring - simpler is better
		if chars.Complexity < 20 {
			score += 50
		} else if chars.Complexity < 40 {
			score += 30
		} else if chars.Complexity < 60 {
			score += 10
		}

		// Prefer non-sub-resource operations for common actions
		if !chars.IsSubResource {
			score += 30
		}

		// Prefer operations with fewer required parameters
		if chars.RequiredParamCount <= 2 {
			score += 20
		} else if chars.RequiredParamCount <= 4 {
			score += 10
		}
	}

	return score
}

// scoreParameterMatch scores based on parameter alignment with context
func (s *SemanticScorer) scoreParameterMatch(
	operation *openapi3.Operation,
	context map[string]interface{},
	chars *OperationCharacteristics,
) int {
	score := 0

	if operation.Parameters == nil {
		return score
	}

	// Count how many required parameters are satisfied
	requiredSatisfied := 0
	totalRequired := 0

	for _, param := range operation.Parameters {
		if param.Value != nil && param.Value.Required {
			totalRequired++
			paramName := param.Value.Name

			// Check if context has this parameter
			if _, exists := context[paramName]; exists {
				requiredSatisfied++
				score += 20
			} else {
				// Check for common parameter mappings
				if mapped := s.mapCommonParameters(paramName, context); mapped {
					requiredSatisfied++
					score += 15
				}
			}
		}
	}

	// Perfect match bonus
	if totalRequired > 0 && requiredSatisfied == totalRequired {
		score += 50
	}

	// Penalty for missing required parameters
	if totalRequired > 0 && requiredSatisfied < totalRequired {
		missingRatio := float64(totalRequired-requiredSatisfied) / float64(totalRequired)
		score -= int(missingRatio * 50)
	}

	return score
}

// mapCommonParameters checks for common parameter name variations
func (s *SemanticScorer) mapCommonParameters(paramName string, context map[string]interface{}) bool {
	// Common parameter mappings (generic, not tool-specific)
	mappings := map[string][]string{
		"id":              {"identifier", "ID", "Id", "_id"},
		"name":            {"title", "label", "display_name"},
		"description":     {"desc", "summary", "details"},
		"created_at":      {"created", "creation_date", "timestamp"},
		"updated_at":      {"updated", "modified", "last_modified"},
		"user_id":         {"user", "owner_id", "owner", "author_id", "author"},
		"organization_id": {"org_id", "org", "company_id", "company"},
	}

	// Check direct mappings
	if alternatives, exists := mappings[paramName]; exists {
		for _, alt := range alternatives {
			if _, hasAlt := context[alt]; hasAlt {
				return true
			}
		}
	}

	// Check reverse mappings
	for canonical, alternatives := range mappings {
		for _, alt := range alternatives {
			if alt == paramName {
				if _, hasCanonical := context[canonical]; hasCanonical {
					return true
				}
			}
		}
	}

	return false
}

// scorePathPattern scores based on path patterns
func (s *SemanticScorer) scorePathPattern(path string, chars *OperationCharacteristics, context map[string]interface{}) int {
	score := 0

	// Shorter paths for primary operations
	if chars.PathDepth <= 2 && !chars.IsSubResource {
		score += 20
	}

	// Path parameter alignment
	pathParams := regexp.MustCompile(`\{([^}]+)\}`).FindAllStringSubmatch(path, -1)
	for _, match := range pathParams {
		if len(match) > 1 {
			paramName := match[1]
			if _, exists := context[paramName]; exists {
				score += 15
			}
		}
	}

	return score
}

// scoreResponseType scores based on expected response type
func (s *SemanticScorer) scoreResponseType(operation *openapi3.Operation, action string, chars *OperationCharacteristics) int {
	score := 0
	actionLower := strings.ToLower(action)

	// Match response type with action expectations
	switch actionLower {
	case "list", "search", "find":
		if chars.ReturnsArray {
			score += 30
		}
	case "get", "read", "fetch":
		if chars.ReturnsSingle && !chars.IsListOperation {
			score += 30
		}
	case "create":
		if chars.ReturnsSingle {
			score += 20
		}
	case "delete":
		// Delete often returns no content or simple confirmation
		if !chars.ReturnsArray {
			score += 20
		}
	}

	return score
}

// scoreTagRelevance scores based on operation tags
func (s *SemanticScorer) scoreTagRelevance(operation *openapi3.Operation, context map[string]interface{}) int {
	score := 0

	if len(operation.Tags) == 0 {
		return score
	}

	// Check if context hints at resource type
	resourceType := ""
	if rt, ok := context["__resource_type"].(string); ok {
		resourceType = rt
	}

	// Score based on tag matching
	for _, tag := range operation.Tags {
		tagLower := strings.ToLower(tag)

		// Direct resource type match
		if resourceType != "" && strings.Contains(tagLower, resourceType) {
			score += 40
		}

		// Check for context parameter hints in tags
		for key := range context {
			if strings.Contains(tagLower, strings.ToLower(key)) {
				score += 10
			}
		}
	}

	return score
}
