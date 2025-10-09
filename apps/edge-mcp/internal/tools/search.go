package tools

import (
	"sort"
	"strings"
)

// SearchOptions defines the criteria for searching tools
type SearchOptions struct {
	// Keyword to search for in tool names and descriptions
	Keyword string `json:"keyword,omitempty"`

	// Category to filter by
	Category string `json:"category,omitempty"`

	// Tags to filter by (must have all specified tags)
	Tags []string `json:"tags,omitempty"`

	// InputType to filter by I/O compatibility
	InputType *DataType `json:"input_type,omitempty"`

	// OutputType to filter by I/O compatibility
	OutputType *DataType `json:"output_type,omitempty"`

	// MaxResults limits the number of results returned
	MaxResults int `json:"max_results,omitempty"`

	// MinRelevanceScore filters out results below this score (0-100)
	MinRelevanceScore float64 `json:"min_relevance_score,omitempty"`

	// EnableFuzzyMatch enables fuzzy matching for tool names
	EnableFuzzyMatch bool `json:"enable_fuzzy_match,omitempty"`

	// FuzzyMaxDistance sets the maximum Levenshtein distance for fuzzy matching
	FuzzyMaxDistance int `json:"fuzzy_max_distance,omitempty"`
}

// SearchResult represents a search result with relevance scoring
type SearchResult struct {
	Tool           ToolDefinition `json:"tool"`
	RelevanceScore float64        `json:"relevance_score"` // Score from 0-100
	MatchDetails   MatchDetails   `json:"match_details"`
}

// MatchDetails provides information about why a tool matched
type MatchDetails struct {
	NameMatch        bool   `json:"name_match"`        // Matched on name
	DescriptionMatch bool   `json:"description_match"` // Matched on description
	CategoryMatch    bool   `json:"category_match"`    // Matched on category
	TagsMatch        bool   `json:"tags_match"`        // Matched on tags
	IOMatch          bool   `json:"io_match"`          // Matched on I/O types
	FuzzyMatch       bool   `json:"fuzzy_match"`       // Matched via fuzzy matching
	FuzzyDistance    int    `json:"fuzzy_distance"`    // Levenshtein distance (if fuzzy)
	KeywordPositions []int  `json:"keyword_positions"` // Positions where keyword was found
	Explanation      string `json:"explanation"`       // Human-readable explanation
}

// Search performs a comprehensive search across all registered tools
func (r *Registry) Search(opts SearchOptions) []SearchResult {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// Set defaults
	if opts.FuzzyMaxDistance == 0 {
		opts.FuzzyMaxDistance = 3 // Default max distance for fuzzy matching
	}

	results := make([]SearchResult, 0)

	// Iterate through all tools
	for _, tool := range r.tools {
		score, match := r.scoreToolMatch(tool, opts)

		// Skip if below minimum relevance score
		if opts.MinRelevanceScore > 0 && score < opts.MinRelevanceScore {
			continue
		}

		// Skip if no match
		if score == 0 {
			continue
		}

		results = append(results, SearchResult{
			Tool:           tool,
			RelevanceScore: score,
			MatchDetails:   match,
		})
	}

	// Sort by relevance score (descending)
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	// Apply max results limit
	if opts.MaxResults > 0 && len(results) > opts.MaxResults {
		results = results[:opts.MaxResults]
	}

	return results
}

// scoreToolMatch calculates the relevance score for a tool based on search options
func (r *Registry) scoreToolMatch(tool ToolDefinition, opts SearchOptions) (float64, MatchDetails) {
	var score float64
	match := MatchDetails{}

	// Track if at least one criterion is specified
	hasAnyCriteria := false

	// 1. Category matching (30 points if specified and matches)
	if opts.Category != "" {
		hasAnyCriteria = true
		if tool.Category == opts.Category {
			score += 30.0
			match.CategoryMatch = true
		} else {
			// No match on category - this is a required filter
			return 0, match
		}
	}

	// 2. Tags matching (20 points if all tags match)
	if len(opts.Tags) > 0 {
		hasAnyCriteria = true
		if hasAllTags(tool.Tags, opts.Tags) {
			score += 20.0
			match.TagsMatch = true
		} else {
			// No match on tags - this is a required filter
			return 0, match
		}
	}

	// 3. I/O type matching (15 points for input, 15 points for output)
	if opts.InputType != nil {
		hasAnyCriteria = true
		if tool.IOCompatibility != nil && matchDataType(tool.IOCompatibility.InputType, *opts.InputType) {
			score += 15.0
			match.IOMatch = true
		} else {
			// No match on input type - this is a required filter
			return 0, match
		}
	}

	if opts.OutputType != nil {
		hasAnyCriteria = true
		if tool.IOCompatibility != nil && matchDataType(tool.IOCompatibility.OutputType, *opts.OutputType) {
			score += 15.0
			match.IOMatch = true
		} else {
			// No match on output type - this is a required filter
			return 0, match
		}
	}

	// 4. Keyword matching (up to 20 points based on where it's found)
	if opts.Keyword != "" {
		hasAnyCriteria = true
		keywordScore, keywordMatch := r.scoreKeywordMatch(tool, opts.Keyword)
		score += keywordScore
		match.NameMatch = keywordMatch.NameMatch
		match.DescriptionMatch = keywordMatch.DescriptionMatch
		match.KeywordPositions = keywordMatch.KeywordPositions

		// If keyword is specified but not found, try fuzzy matching
		if keywordScore == 0 && opts.EnableFuzzyMatch {
			fuzzyScore, fuzzyDist := r.scoreFuzzyMatch(tool.Name, opts.Keyword, opts.FuzzyMaxDistance)
			if fuzzyScore > 0 {
				score += fuzzyScore
				match.FuzzyMatch = true
				match.FuzzyDistance = fuzzyDist
			}
		}

		// If keyword is specified but nothing matched, return 0
		if keywordScore == 0 && !match.FuzzyMatch {
			return 0, match
		}
	}

	// If no criteria specified, return 0
	if !hasAnyCriteria {
		return 0, match
	}

	// Generate explanation
	match.Explanation = r.generateMatchExplanation(match, score)

	return score, match
}

// scoreKeywordMatch scores how well a keyword matches a tool
func (r *Registry) scoreKeywordMatch(tool ToolDefinition, keyword string) (float64, MatchDetails) {
	var score float64
	match := MatchDetails{
		KeywordPositions: make([]int, 0),
	}

	keyword = strings.ToLower(keyword)

	// Check name (exact match: 20 points, contains: 15 points)
	nameLower := strings.ToLower(tool.Name)
	if nameLower == keyword {
		score += 20.0
		match.NameMatch = true
		match.KeywordPositions = append(match.KeywordPositions, 0)
	} else if strings.Contains(nameLower, keyword) {
		score += 15.0
		match.NameMatch = true
		match.KeywordPositions = append(match.KeywordPositions, strings.Index(nameLower, keyword))
	}

	// Check description (contains: 10 points, multiple occurrences: +2 per occurrence)
	descLower := strings.ToLower(tool.Description)
	if strings.Contains(descLower, keyword) {
		occurrences := strings.Count(descLower, keyword)
		score += 10.0 + float64(min(occurrences-1, 5)*2) // Cap at 5 extra occurrences
		match.DescriptionMatch = true

		// Find all positions
		pos := 0
		for {
			idx := strings.Index(descLower[pos:], keyword)
			if idx == -1 {
				break
			}
			match.KeywordPositions = append(match.KeywordPositions, pos+idx)
			pos += idx + len(keyword)
		}
	}

	return score, match
}

// scoreFuzzyMatch calculates a fuzzy match score using Levenshtein distance
func (r *Registry) scoreFuzzyMatch(toolName, keyword string, maxDistance int) (float64, int) {
	toolNameLower := strings.ToLower(toolName)
	keywordLower := strings.ToLower(keyword)

	distance := levenshteinDistance(toolNameLower, keywordLower)

	// If distance exceeds max, no match
	if distance > maxDistance {
		return 0, distance
	}

	// Score based on how close the match is
	// Distance 1: 15 points, Distance 2: 10 points, Distance 3: 5 points
	score := float64(maxDistance-distance+1) * 5.0

	return score, distance
}

// matchDataType checks if two data types match
func matchDataType(dt1, dt2 DataType) bool {
	// If format is specified, it must match
	if dt2.Format != "" && dt1.Format != dt2.Format {
		return false
	}

	// If schema is specified, it must match
	if dt2.Schema != "" && dt1.Schema != dt2.Schema {
		return false
	}

	// If content type is specified, it must match
	if dt2.ContentType != "" && dt1.ContentType != dt2.ContentType {
		return false
	}

	// All specified fields match
	return true
}

// generateMatchExplanation generates a human-readable explanation of the match
func (r *Registry) generateMatchExplanation(match MatchDetails, score float64) string {
	var parts []string

	if match.CategoryMatch {
		parts = append(parts, "category match")
	}
	if match.TagsMatch {
		parts = append(parts, "tags match")
	}
	if match.IOMatch {
		parts = append(parts, "I/O types match")
	}
	if match.NameMatch {
		parts = append(parts, "name match")
	}
	if match.DescriptionMatch {
		parts = append(parts, "description match")
	}
	if match.FuzzyMatch {
		parts = append(parts, "fuzzy name match")
	}

	if len(parts) == 0 {
		return "no match"
	}

	explanation := strings.Join(parts, ", ")
	return explanation + " (score: " + formatScore(score) + ")"
}

// formatScore formats a score for display
func formatScore(score float64) string {
	return strings.TrimSuffix(strings.TrimSuffix(sprintf("%.1f", score), "0"), ".")
}

// sprintf is a simple sprintf implementation
func sprintf(format string, a ...interface{}) string {
	// For now, just convert to string - in production use fmt.Sprintf
	return strings.Replace(format, "%.1f", toString(a[0].(float64)), 1)
}

// toString converts float to string with 1 decimal place
func toString(f float64) string {
	s := ""
	i := int(f)
	d := int((f - float64(i)) * 10)
	s = itoa(i) + "." + itoa(d)
	return s
}

// itoa converts int to string
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}
	digits := []byte{}
	for i > 0 {
		digits = append([]byte{byte(i%10) + '0'}, digits...)
		i /= 10
	}
	if negative {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}

	// Initialize first row and column
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}

	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 1
			if s1[i-1] == s2[j-1] {
				cost = 0
			}

			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}

	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of two or more integers
func min(values ...int) int {
	if len(values) == 0 {
		return 0
	}

	result := values[0]
	for _, v := range values[1:] {
		if v < result {
			result = v
		}
	}
	return result
}

// SearchByKeyword is a convenience method for simple keyword searches
func (r *Registry) SearchByKeyword(keyword string, maxResults int) []SearchResult {
	return r.Search(SearchOptions{
		Keyword:          keyword,
		MaxResults:       maxResults,
		EnableFuzzyMatch: true,
	})
}

// SearchByCategory is a convenience method for category-based searches
func (r *Registry) SearchByCategory(category string, maxResults int) []SearchResult {
	return r.Search(SearchOptions{
		Category:   category,
		MaxResults: maxResults,
	})
}

// SearchByTags is a convenience method for tag-based searches
func (r *Registry) SearchByTagsSearch(tags []string, maxResults int) []SearchResult {
	return r.Search(SearchOptions{
		Tags:       tags,
		MaxResults: maxResults,
	})
}

// SearchByIOType searches for tools by input/output type compatibility
func (r *Registry) SearchByIOType(inputType, outputType *DataType, maxResults int) []SearchResult {
	return r.Search(SearchOptions{
		InputType:  inputType,
		OutputType: outputType,
		MaxResults: maxResults,
	})
}

// GetToolSuggestions returns tool suggestions based on a partial name or description
func (r *Registry) GetToolSuggestions(partial string, maxResults int) []SearchResult {
	return r.Search(SearchOptions{
		Keyword:          partial,
		MaxResults:       maxResults,
		EnableFuzzyMatch: true,
		FuzzyMaxDistance: 3,
	})
}
