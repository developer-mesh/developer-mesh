package artifactory

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// AQLQueryBuilder provides a structured way to build AQL (Artifactory Query Language) queries
// This helps AI agents construct valid AQL queries without syntax errors
type AQLQueryBuilder struct {
	findType      string                   // "items", "builds", "entries"
	criteria      []map[string]interface{} // List of criteria to be ANDed together
	includeFields []string                 // Fields to include in response
	sortBy        []SortField              // Sort specifications
	limit         int                      // Maximum results
	offset        int                      // Result offset for pagination
}

// SortField represents a field to sort by
type SortField struct {
	Field string
	Asc   bool // true for ascending, false for descending
}

// NewAQLQueryBuilder creates a new AQL query builder
// Default find type is "items" as it's the most common
func NewAQLQueryBuilder() *AQLQueryBuilder {
	return &AQLQueryBuilder{
		findType:      "items",
		criteria:      []map[string]interface{}{},
		includeFields: []string{},
		sortBy:        []SortField{},
		limit:         100, // Default reasonable limit
		offset:        0,
	}
}

// FindItems sets the query to find items (artifacts)
func (b *AQLQueryBuilder) FindItems() *AQLQueryBuilder {
	b.findType = "items"
	return b
}

// FindBuilds sets the query to find builds
func (b *AQLQueryBuilder) FindBuilds() *AQLQueryBuilder {
	b.findType = "builds"
	return b
}

// FindEntries sets the query to find archive entries
func (b *AQLQueryBuilder) FindEntries() *AQLQueryBuilder {
	b.findType = "entries"
	return b
}

// FindItemsByName adds a name pattern criterion
func (b *AQLQueryBuilder) FindItemsByName(pattern string) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"name": map[string]string{"$match": pattern},
	})
	return b
}

// FindItemsByRepo adds a repository criterion
func (b *AQLQueryBuilder) FindItemsByRepo(repoKey string) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"repo": repoKey,
	})
	return b
}

// FindItemsByPath adds a path pattern criterion
func (b *AQLQueryBuilder) FindItemsByPath(pathPattern string) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"path": map[string]string{"$match": pathPattern},
	})
	return b
}

// FindItemsByProperty adds a property criterion
func (b *AQLQueryBuilder) FindItemsByProperty(key, value string) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"@" + key: value,
	})
	return b
}

// FindItemsByChecksum adds a checksum criterion
func (b *AQLQueryBuilder) FindItemsByChecksum(checksumType, checksum string) *AQLQueryBuilder {
	field := "actual_" + checksumType // actual_sha1, actual_sha256, actual_md5
	b.criteria = append(b.criteria, map[string]interface{}{
		field: checksum,
	})
	return b
}

// FindItemsBySize adds size criteria
func (b *AQLQueryBuilder) FindItemsBySize(operator string, bytes int64) *AQLQueryBuilder {
	// operator can be "$gt", "$gte", "$lt", "$lte", "$eq"
	b.criteria = append(b.criteria, map[string]interface{}{
		"size": map[string]int64{operator: bytes},
	})
	return b
}

// FindItemsModifiedSince adds a modified time criterion
func (b *AQLQueryBuilder) FindItemsModifiedSince(since time.Time) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"modified": map[string]string{"$gt": since.Format("2006-01-02T15:04:05.000Z")},
	})
	return b
}

// FindItemsCreatedBefore adds a created time criterion
func (b *AQLQueryBuilder) FindItemsCreatedBefore(before time.Time) *AQLQueryBuilder {
	b.criteria = append(b.criteria, map[string]interface{}{
		"created": map[string]string{"$lt": before.Format("2006-01-02T15:04:05.000Z")},
	})
	return b
}

// FindItemsByType adds a file type criterion
func (b *AQLQueryBuilder) FindItemsByType(fileType string) *AQLQueryBuilder {
	// fileType can be "file" or "folder"
	b.criteria = append(b.criteria, map[string]interface{}{
		"type": fileType,
	})
	return b
}

// Include adds fields to include in the response
func (b *AQLQueryBuilder) Include(fields ...string) *AQLQueryBuilder {
	b.includeFields = append(b.includeFields, fields...)
	return b
}

// Sort adds a sort specification
func (b *AQLQueryBuilder) Sort(field string, ascending bool) *AQLQueryBuilder {
	b.sortBy = append(b.sortBy, SortField{Field: field, Asc: ascending})
	return b
}

// Limit sets the maximum number of results
func (b *AQLQueryBuilder) Limit(n int) *AQLQueryBuilder {
	b.limit = n
	return b
}

// Offset sets the result offset for pagination
func (b *AQLQueryBuilder) Offset(n int) *AQLQueryBuilder {
	b.offset = n
	return b
}

// AddCustomCriterion adds a custom criterion for advanced use cases
func (b *AQLQueryBuilder) AddCustomCriterion(criterion map[string]interface{}) *AQLQueryBuilder {
	b.criteria = append(b.criteria, criterion)
	return b
}

// Build constructs the final AQL query string
func (b *AQLQueryBuilder) Build() (string, error) {
	if err := b.validate(); err != nil {
		return "", err
	}

	var queryParts []string

	// Start with find type
	queryParts = append(queryParts, b.findType+".find(")

	// Add criteria
	if len(b.criteria) > 0 {
		// Combine all criteria with AND logic (all must match)
		criteriaJSON, err := json.Marshal(map[string]interface{}{
			"$and": b.criteria,
		})
		if err != nil {
			return "", fmt.Errorf("failed to marshal criteria: %w", err)
		}
		queryParts = append(queryParts, string(criteriaJSON))
	} else {
		// No criteria means find all
		queryParts = append(queryParts, "{}")
	}

	queryParts = append(queryParts, ")")

	// Add include fields
	if len(b.includeFields) > 0 {
		includeList := strings.Join(wrapQuotes(b.includeFields), ", ")
		queryParts = append(queryParts, ".include("+includeList+")")
	}

	// Add sort
	if len(b.sortBy) > 0 {
		sortSpecs := []string{}
		for _, sort := range b.sortBy {
			order := "asc"
			if !sort.Asc {
				order = "desc"
			}
			sortSpecs = append(sortSpecs, fmt.Sprintf(`{"$%s": ["%s"]}`, order, sort.Field))
		}
		queryParts = append(queryParts, ".sort({"+strings.Join(sortSpecs, ", ")+"})")
	}

	// Add limit
	if b.limit > 0 {
		queryParts = append(queryParts, fmt.Sprintf(".limit(%d)", b.limit))
	}

	// Add offset
	if b.offset > 0 {
		queryParts = append(queryParts, fmt.Sprintf(".offset(%d)", b.offset))
	}

	return strings.Join(queryParts, ""), nil
}

// BuildSimple creates a simplified query without complex JSON
// This is useful for simple queries where readability is important
func (b *AQLQueryBuilder) BuildSimple() (string, error) {
	if err := b.validate(); err != nil {
		return "", err
	}

	// For simple queries with one criterion, use a more readable format
	if len(b.criteria) == 1 && len(b.includeFields) == 0 && len(b.sortBy) == 0 {
		criterion := b.criteria[0]

		// Try to build a simple string representation
		var parts []string
		parts = append(parts, b.findType+".find({")

		criteriaStrings := []string{}
		for key, value := range criterion {
			switch v := value.(type) {
			case string:
				criteriaStrings = append(criteriaStrings, fmt.Sprintf(`"%s": "%s"`, key, v))
			case map[string]interface{}:
				// Handle operators like $match, $gt, etc.
				for op, opVal := range v {
					criteriaStrings = append(criteriaStrings, fmt.Sprintf(`"%s": {"%s": "%v"}`, key, op, opVal))
				}
			default:
				criteriaStrings = append(criteriaStrings, fmt.Sprintf(`"%s": %v`, key, value))
			}
		}

		parts = append(parts, strings.Join(criteriaStrings, ", "))
		parts = append(parts, "})")

		// Only add limit if it's different from the default
		if b.limit > 0 && b.limit != 100 {
			parts = append(parts, fmt.Sprintf(".limit(%d)", b.limit))
		}

		// Add offset if specified
		if b.offset > 0 {
			parts = append(parts, fmt.Sprintf(".offset(%d)", b.offset))
		}

		return strings.Join(parts, ""), nil
	}

	// For complex queries, use the standard Build method
	return b.Build()
}

// validate checks if the query builder state is valid
func (b *AQLQueryBuilder) validate() error {
	// Validate find type
	validTypes := map[string]bool{"items": true, "builds": true, "entries": true}
	if !validTypes[b.findType] {
		return fmt.Errorf("invalid find type: %s (must be items, builds, or entries)", b.findType)
	}

	// Validate limit
	if b.limit < 0 {
		return fmt.Errorf("limit cannot be negative")
	}
	if b.limit > 10000 {
		return fmt.Errorf("limit exceeds maximum allowed value of 10000")
	}

	// Validate offset
	if b.offset < 0 {
		return fmt.Errorf("offset cannot be negative")
	}

	// Validate sort fields
	for _, sort := range b.sortBy {
		if sort.Field == "" {
			return fmt.Errorf("sort field cannot be empty")
		}
	}

	// Validate include fields
	for _, field := range b.includeFields {
		if field == "" {
			return fmt.Errorf("include field cannot be empty")
		}
	}

	return nil
}

// wrapQuotes wraps each string in double quotes
func wrapQuotes(strs []string) []string {
	quoted := make([]string, len(strs))
	for i, s := range strs {
		quoted[i] = `"` + s + `"`
	}
	return quoted
}

// GetCommonAQLExamples returns examples of how to use the query builder
func GetCommonAQLExamples() map[string]func() *AQLQueryBuilder {
	return map[string]func() *AQLQueryBuilder{
		"Find all JARs in libs-release": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByRepo("libs-release").
				FindItemsByName("*.jar")
		},
		"Find large files modified recently": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsBySize("$gt", 100*1024*1024).               // > 100MB
				FindItemsModifiedSince(time.Now().AddDate(0, 0, -7)) // Last 7 days
		},
		"Find artifacts by checksum": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByChecksum("sha256", "abc123def456")
		},
		"Find Docker images with specific property": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByRepo("docker-local").
				FindItemsByProperty("docker.manifest", "true")
		},
		"Find and sort by size": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByRepo("generic-local").
				Sort("size", false). // Descending (largest first)
				Limit(20)
		},
		"Paginated search": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByRepo("npm-local").
				Limit(50).
				Offset(100) // Skip first 100, get next 50
		},
		"Include specific fields": func() *AQLQueryBuilder {
			return NewAQLQueryBuilder().
				FindItemsByName("*.zip").
				Include("name", "repo", "path", "size", "created", "modified", "sha256")
		},
	}
}

// ValidateAQLQuery performs basic validation on a raw AQL query string
func ValidateAQLQuery(query string) error {
	query = strings.TrimSpace(query)

	if query == "" {
		return fmt.Errorf("query cannot be empty")
	}

	// Check for basic structure
	if !strings.Contains(query, ".find(") {
		return fmt.Errorf("invalid AQL query: must contain .find() method")
	}

	// Check for valid find types
	validPrefixes := []string{"items.find(", "builds.find(", "entries.find("}
	hasValidPrefix := false
	for _, prefix := range validPrefixes {
		if strings.HasPrefix(query, prefix) {
			hasValidPrefix = true
			break
		}
	}
	if !hasValidPrefix {
		return fmt.Errorf("invalid AQL query: must start with items.find(), builds.find(), or entries.find()")
	}

	// Check for balanced parentheses
	openCount := strings.Count(query, "(")
	closeCount := strings.Count(query, ")")
	if openCount != closeCount {
		return fmt.Errorf("invalid AQL query: unbalanced parentheses")
	}

	// Check for common syntax errors first (before balanced check)
	if strings.Contains(query, ",,") {
		return fmt.Errorf("invalid AQL query: double comma detected")
	}

	if strings.Contains(query, "{{") || strings.Contains(query, "}}") {
		return fmt.Errorf("invalid AQL query: double braces detected")
	}

	// Check for balanced braces
	openBrace := strings.Count(query, "{")
	closeBrace := strings.Count(query, "}")
	if openBrace != closeBrace {
		return fmt.Errorf("invalid AQL query: unbalanced braces")
	}

	return nil
}
