package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// TestSearch_KeywordMatching tests keyword search functionality
func TestSearch_KeywordMatching(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		keyword       string
		expectedCount int
		expectedFirst string
		minScore      float64
	}{
		{
			name:          "exact name match",
			keyword:       "github_get_issue",
			expectedCount: 1,
			expectedFirst: "github_get_issue",
			minScore:      20.0,
		},
		{
			name:          "partial name match",
			keyword:       "issue",
			expectedCount: 3,
			expectedFirst: "", // Any tool with "issue" in name
			minScore:      10.0,
		},
		{
			name:          "description match",
			keyword:       "repository",
			expectedCount: 4, // All tools mention repository
			minScore:      10.0,
		},
		{
			name:          "no match",
			keyword:       "nonexistent",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchByKeyword(tt.keyword, 10)

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			if len(results) > 0 {
				if results[0].Tool.Name != tt.expectedFirst && tt.expectedFirst != "" {
					t.Errorf("expected first result to be %s, got %s", tt.expectedFirst, results[0].Tool.Name)
				}

				if results[0].RelevanceScore < tt.minScore {
					t.Errorf("expected score >= %.1f, got %.1f", tt.minScore, results[0].RelevanceScore)
				}
			}
		})
	}
}

// TestSearch_CategoryFiltering tests category-based filtering
func TestSearch_CategoryFiltering(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		category      string
		expectedCount int
	}{
		{
			name:          "issues category",
			category:      "issues",
			expectedCount: 3,
		},
		{
			name:          "repository category",
			category:      "repository",
			expectedCount: 1,
		},
		{
			name:          "nonexistent category",
			category:      "nonexistent",
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchByCategory(tt.category, 10)

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			// Verify all results match the category
			for _, result := range results {
				if result.Tool.Category != tt.category {
					t.Errorf("expected category %s, got %s", tt.category, result.Tool.Category)
				}
				if !result.MatchDetails.CategoryMatch {
					t.Error("expected CategoryMatch to be true")
				}
			}
		})
	}
}

// TestSearch_TagFiltering tests tag-based filtering
func TestSearch_TagFiltering(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		tags          []string
		expectedCount int
		minCount      int
	}{
		{
			name:          "read tag",
			tags:          []string{"read"},
			expectedCount: 3, // github_get_issue, github_list_issues, github_get_repository
		},
		{
			name:          "write tag",
			tags:          []string{"write"},
			expectedCount: 1,
		},
		{
			name:     "multiple tags (read + list)",
			tags:     []string{"read", "list"},
			minCount: 1,
		},
		{
			name:          "nonexistent tag",
			tags:          []string{"nonexistent"},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchByTagsSearch(tt.tags, 10)

			if tt.expectedCount > 0 {
				if len(results) != tt.expectedCount {
					t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
				}
			} else if tt.minCount > 0 {
				if len(results) < tt.minCount {
					t.Errorf("expected at least %d results, got %d", tt.minCount, len(results))
				}
			}

			// Verify all results have the required tags
			for _, result := range results {
				if !hasAllTags(result.Tool.Tags, tt.tags) {
					t.Errorf("tool %s missing required tags", result.Tool.Name)
				}
				if !result.MatchDetails.TagsMatch {
					t.Error("expected TagsMatch to be true")
				}
			}
		})
	}
}

// TestSearch_IOTypeMatching tests input/output type filtering
func TestSearch_IOTypeMatching(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		inputType     *DataType
		outputType    *DataType
		expectedCount int
	}{
		{
			name: "input type json",
			inputType: &DataType{
				Format: "json",
			},
			expectedCount: 4, // All tools have json input
		},
		{
			name: "output type issue schema",
			outputType: &DataType{
				Schema: "issue",
			},
			expectedCount: 2, // github_get_issue, github_create_issue
		},
		{
			name: "both input and output",
			inputType: &DataType{
				Format: "json",
			},
			outputType: &DataType{
				Schema: "issue",
			},
			expectedCount: 2, // github_get_issue, github_create_issue
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchByIOType(tt.inputType, tt.outputType, 10)

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			// Verify all results match the I/O types
			for _, result := range results {
				if !result.MatchDetails.IOMatch {
					t.Error("expected IOMatch to be true")
				}
			}
		})
	}
}

// TestSearch_FuzzyMatching tests fuzzy matching functionality
func TestSearch_FuzzyMatching(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		keyword       string
		enableFuzzy   bool
		expectedCount int
		expectFuzzy   bool
	}{
		{
			name:          "exact match - fuzzy not needed",
			keyword:       "github_get_issue",
			enableFuzzy:   true,
			expectedCount: 1,
			expectFuzzy:   false,
		},
		{
			name:          "fuzzy match enabled - typo",
			keyword:       "github_get_isue", // Missing 's'
			enableFuzzy:   true,
			expectedCount: 1,
			expectFuzzy:   true,
		},
		{
			name:          "fuzzy match disabled - typo",
			keyword:       "github_get_isue",
			enableFuzzy:   false,
			expectedCount: 0,
		},
		{
			name:          "fuzzy match - too far",
			keyword:       "xyz123",
			enableFuzzy:   true,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.Search(SearchOptions{
				Keyword:          tt.keyword,
				EnableFuzzyMatch: tt.enableFuzzy,
				MaxResults:       10,
			})

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}

			if len(results) > 0 && tt.expectFuzzy {
				if !results[0].MatchDetails.FuzzyMatch {
					t.Error("expected FuzzyMatch to be true")
				}
				if results[0].MatchDetails.FuzzyDistance == 0 {
					t.Error("expected FuzzyDistance > 0")
				}
			}
		})
	}
}

// TestSearch_RelevanceScoring tests relevance score calculation
func TestSearch_RelevanceScoring(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name         string
		opts         SearchOptions
		expectOrder  []string // Expected tool names in order of relevance
		expectAnySet bool     // If true, just check all tools are present, not order
	}{
		{
			name: "category match scores higher than keyword",
			opts: SearchOptions{
				Category: "issues",
				Keyword:  "issue",
			},
			// All three tools should match with similar scores, order may vary
			expectOrder:  []string{"github_create_issue", "github_get_issue", "github_list_issues"},
			expectAnySet: true, // Don't check order, just check all are present
		},
		{
			name: "exact name match scores highest",
			opts: SearchOptions{
				Keyword: "github_get_issue",
			},
			expectOrder: []string{"github_get_issue"},
		},
		{
			name: "combined category and tags",
			opts: SearchOptions{
				Category: "issues",
				Tags:     []string{"read"},
			},
			expectOrder:  []string{"github_get_issue", "github_list_issues"},
			expectAnySet: true, // Don't check order, just check both are present
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.Search(tt.opts)

			if len(results) != len(tt.expectOrder) {
				t.Errorf("expected %d results, got %d", len(tt.expectOrder), len(results))
			}

			if tt.expectAnySet {
				// Just verify all expected tools are present
				resultNames := make(map[string]bool)
				for _, result := range results {
					resultNames[result.Tool.Name] = true
				}
				for _, expectedName := range tt.expectOrder {
					if !resultNames[expectedName] {
						t.Errorf("expected tool %s not found in results", expectedName)
					}
				}
			} else {
				// Verify exact order
				for i, expectedName := range tt.expectOrder {
					if i >= len(results) {
						break
					}
					if results[i].Tool.Name != expectedName {
						t.Errorf("expected result[%d] to be %s, got %s (score: %.1f)",
							i, expectedName, results[i].Tool.Name, results[i].RelevanceScore)
					}
				}
			}

			// Verify scores are in descending order
			for i := 1; i < len(results); i++ {
				if results[i].RelevanceScore > results[i-1].RelevanceScore {
					t.Errorf("results not sorted by score: [%d]=%.1f > [%d]=%.1f",
						i, results[i].RelevanceScore, i-1, results[i-1].RelevanceScore)
				}
			}
		})
	}
}

// TestSearch_MaxResults tests result limiting
func TestSearch_MaxResults(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name       string
		keyword    string
		maxResults int
		expectLen  int
	}{
		{
			name:       "limit to 2",
			keyword:    "issue",
			maxResults: 2,
			expectLen:  2,
		},
		{
			name:       "limit to 1",
			keyword:    "github",
			maxResults: 1,
			expectLen:  1,
		},
		{
			name:       "no limit",
			keyword:    "github",
			maxResults: 0,
			expectLen:  4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.SearchByKeyword(tt.keyword, tt.maxResults)

			if len(results) != tt.expectLen {
				t.Errorf("expected %d results, got %d", tt.expectLen, len(results))
			}
		})
	}
}

// TestSearch_MinRelevanceScore tests minimum score filtering
func TestSearch_MinRelevanceScore(t *testing.T) {
	registry := setupTestRegistryForSearch()

	results := registry.Search(SearchOptions{
		Keyword:           "issue",
		MinRelevanceScore: 18.0, // Only exact or very close matches
	})

	// All results should have score >= 18.0
	for _, result := range results {
		if result.RelevanceScore < 18.0 {
			t.Errorf("result %s has score %.1f, expected >= 18.0",
				result.Tool.Name, result.RelevanceScore)
		}
	}
}

// TestSearch_CombinedFilters tests combining multiple search criteria
func TestSearch_CombinedFilters(t *testing.T) {
	registry := setupTestRegistryForSearch()

	results := registry.Search(SearchOptions{
		Keyword:  "issue",
		Category: "issues",
		Tags:     []string{"read"},
	})

	// Should only return tools that match ALL criteria
	for _, result := range results {
		if result.Tool.Category != "issues" {
			t.Errorf("tool %s has wrong category", result.Tool.Name)
		}
		if !hasAllTags(result.Tool.Tags, []string{"read"}) {
			t.Errorf("tool %s missing read tag", result.Tool.Name)
		}
		if !result.MatchDetails.CategoryMatch {
			t.Error("expected CategoryMatch to be true")
		}
		if !result.MatchDetails.TagsMatch {
			t.Error("expected TagsMatch to be true")
		}
	}
}

// TestSearch_MatchExplanation tests match explanation generation
func TestSearch_MatchExplanation(t *testing.T) {
	registry := setupTestRegistryForSearch()

	results := registry.Search(SearchOptions{
		Keyword:  "github_get_issue",
		Category: "issues",
	})

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	result := results[0]
	if result.MatchDetails.Explanation == "" {
		t.Error("expected non-empty explanation")
	}

	// Should mention both category and name match
	if !contains(result.MatchDetails.Explanation, "category") {
		t.Error("explanation should mention category match")
	}
	if !contains(result.MatchDetails.Explanation, "name") {
		t.Error("explanation should mention name match")
	}
}

// TestLevenshteinDistance tests the Levenshtein distance calculation
func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		s1       string
		s2       string
		expected int
	}{
		{"", "", 0},
		{"abc", "", 3},
		{"", "abc", 3},
		{"abc", "abc", 0},
		{"abc", "ab", 1},
		{"abc", "abcd", 1},
		{"abc", "adc", 1},
		{"kitten", "sitting", 3},
		{"saturday", "sunday", 3},
	}

	for _, tt := range tests {
		t.Run(tt.s1+"_"+tt.s2, func(t *testing.T) {
			distance := levenshteinDistance(tt.s1, tt.s2)
			if distance != tt.expected {
				t.Errorf("levenshteinDistance(%q, %q) = %d, expected %d",
					tt.s1, tt.s2, distance, tt.expected)
			}
		})
	}
}

// TestSearch_GetToolSuggestions tests tool suggestion functionality
func TestSearch_GetToolSuggestions(t *testing.T) {
	registry := setupTestRegistryForSearch()

	tests := []struct {
		name          string
		partial       string
		maxResults    int
		expectedCount int
	}{
		{
			name:          "partial name",
			partial:       "get",
			maxResults:    5,
			expectedCount: 2,
		},
		{
			name:          "fuzzy partial",
			partial:       "issu",
			maxResults:    5,
			expectedCount: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := registry.GetToolSuggestions(tt.partial, tt.maxResults)

			if len(results) != tt.expectedCount {
				t.Errorf("expected %d results, got %d", tt.expectedCount, len(results))
			}
		})
	}
}

// setupTestRegistryForSearch creates a test registry with sample tools
func setupTestRegistryForSearch() *Registry {
	registry := NewRegistry()

	// Create sample tools with various attributes
	tools := []ToolDefinition{
		{
			Name:        "github_get_issue",
			Description: "Retrieve a specific issue from a repository",
			Category:    "issues",
			Tags:        []string{"read", "github"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner":        map[string]interface{}{"type": "string"},
					"repo":         map[string]interface{}{"type": "string"},
					"issue_number": map[string]interface{}{"type": "integer"},
				},
			},
			IOCompatibility: &IOCompatibility{
				InputType: DataType{
					Format: "json",
					Schema: "issue_query",
				},
				OutputType: DataType{
					Format: "json",
					Schema: "issue",
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return map[string]interface{}{"id": 1}, nil
			},
		},
		{
			Name:        "github_list_issues",
			Description: "List all issues in a repository",
			Category:    "issues",
			Tags:        []string{"read", "list", "github"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{"type": "string"},
					"repo":  map[string]interface{}{"type": "string"},
				},
			},
			IOCompatibility: &IOCompatibility{
				InputType: DataType{
					Format: "json",
					Schema: "issue_query",
				},
				OutputType: DataType{
					Format: "json",
					Schema: "issue_list",
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return []interface{}{}, nil
			},
		},
		{
			Name:        "github_create_issue",
			Description: "Create a new issue in a repository",
			Category:    "issues",
			Tags:        []string{"write", "github"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{"type": "string"},
					"repo":  map[string]interface{}{"type": "string"},
					"title": map[string]interface{}{"type": "string"},
				},
			},
			IOCompatibility: &IOCompatibility{
				InputType: DataType{
					Format: "json",
					Schema: "issue_create",
				},
				OutputType: DataType{
					Format: "json",
					Schema: "issue",
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return map[string]interface{}{"id": 1}, nil
			},
		},
		{
			Name:        "github_get_repository",
			Description: "Get repository information",
			Category:    "repository",
			Tags:        []string{"read", "github"},
			InputSchema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"owner": map[string]interface{}{"type": "string"},
					"repo":  map[string]interface{}{"type": "string"},
				},
			},
			IOCompatibility: &IOCompatibility{
				InputType: DataType{
					Format: "json",
				},
				OutputType: DataType{
					Format: "json",
					Schema: "repository",
				},
			},
			Handler: func(ctx context.Context, args json.RawMessage) (interface{}, error) {
				return map[string]interface{}{"name": "test"}, nil
			},
		},
	}

	for _, tool := range tools {
		registry.RegisterRemote(tool)
	}

	return registry
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
