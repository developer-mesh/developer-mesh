package search

import (
	"context"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockEmbeddingService is a mock embedding service for testing
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, req interface{}) (interface{}, error) {
	args := m.Called(ctx, req)
	return args.Get(0), args.Error(1)
}

func TestRelevanceRanker_CalculateKeywordScore(t *testing.T) {
	ranker := NewRelevanceRanker(nil)

	tests := []struct {
		name     string
		query    string
		release  *models.PackageRelease
		expected float64
		minScore float64
	}{
		{
			name:  "exact package name match",
			query: "my-package",
			release: &models.PackageRelease{
				PackageName: "my-package",
				Description: strPtr("A test package"),
			},
			minScore: 0.8, // Should be high due to package name match
		},
		{
			name:  "partial package name match",
			query: "package",
			release: &models.PackageRelease{
				PackageName: "my-awesome-package",
				Description: strPtr("A test package"),
			},
			minScore: 0.5,
		},
		{
			name:  "description match",
			query: "test database",
			release: &models.PackageRelease{
				PackageName: "mydb",
				Description: strPtr("A test database for development"),
			},
			minScore: 0.4,
		},
		{
			name:  "no match",
			query: "unrelated",
			release: &models.PackageRelease{
				PackageName: "my-package",
				Description: strPtr("A test package"),
			},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ranker.calculateKeywordScore(tt.query, tt.release)

			if tt.expected > 0 {
				assert.Equal(t, tt.expected, score)
			} else {
				assert.GreaterOrEqual(t, score, tt.minScore)
			}
		})
	}
}

func TestRelevanceRanker_CalculateRecencyScore(t *testing.T) {
	ranker := NewRelevanceRanker(nil)

	now := time.Now()

	tests := []struct {
		name     string
		release  *models.PackageRelease
		expected string // "high", "medium", "low"
	}{
		{
			name: "recent release",
			release: &models.PackageRelease{
				PublishedAt: now.AddDate(0, 0, -7), // 7 days ago
			},
			expected: "high",
		},
		{
			name: "medium age release",
			release: &models.PackageRelease{
				PublishedAt: now.AddDate(0, -3, 0), // 3 months ago
			},
			expected: "medium",
		},
		{
			name: "old release",
			release: &models.PackageRelease{
				PublishedAt: now.AddDate(-2, 0, 0), // 2 years ago
			},
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ranker.calculateRecencyScore(tt.release)

			switch tt.expected {
			case "high":
				assert.Greater(t, score, 0.7)
			case "medium":
				assert.Greater(t, score, 0.3)
				assert.Less(t, score, 0.7)
			case "low":
				assert.Less(t, score, 0.3)
			}
		})
	}
}

func TestRelevanceRanker_CalculatePopularityScore(t *testing.T) {
	ranker := NewRelevanceRanker(nil)

	tests := []struct {
		name     string
		release  *models.PackageRelease
		expected string // "high", "low"
	}{
		{
			name: "well-documented package",
			release: &models.PackageRelease{
				DocumentationURL: strPtr("https://docs.example.com"),
				Homepage:         strPtr("https://example.com"),
				License:          strPtr("MIT"),
			},
			expected: "high",
		},
		{
			name: "breaking change",
			release: &models.PackageRelease{
				IsBreakingChange: true,
			},
			expected: "low",
		},
		{
			name: "prerelease version",
			release: &models.PackageRelease{
				Prerelease: strPtr("beta.1"),
			},
			expected: "low",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := ranker.calculatePopularityScore(tt.release)

			switch tt.expected {
			case "high":
				assert.Greater(t, score, 0.5)
			case "low":
				assert.Less(t, score, 0.5)
			}

			// Score should always be between 0 and 1
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestRelevanceRanker_Rank(t *testing.T) {
	ranker := NewRelevanceRanker(nil)

	now := time.Now()

	result := &PackageSearchResult{
		Release: &models.PackageRelease{
			PackageName:      "test-package",
			Version:          "1.0.0",
			Description:      strPtr("A test package for unit testing"),
			PublishedAt:      now.AddDate(0, 0, -7), // Recent
			License:          strPtr("MIT"),
			Homepage:         strPtr("https://example.com"),
			DocumentationURL: strPtr("https://docs.example.com"),
			IsBreakingChange: false,
		},
		Similarity: 0.85, // High semantic similarity
	}

	score := ranker.Rank("test package", result)

	// Score should be high for a well-matched, recent, well-documented package
	assert.Greater(t, score, 0.6)
	assert.LessOrEqual(t, score, 1.0)

	// Test that breaking change reduces score
	result.Release.IsBreakingChange = true
	scoreWithBreaking := ranker.Rank("test package", result)
	assert.Less(t, scoreWithBreaking, score)
}

func TestExtractMatchedKeywords(t *testing.T) {
	service := &PackageSearchService{
		logger: nil,
	}

	release := &models.PackageRelease{
		PackageName: "my-awesome-package",
		Description: strPtr("A package for testing and development"),
	}

	keywords := service.extractMatchedKeywords("awesome testing", release)

	// Should match "awesome" and "testing"
	assert.Contains(t, keywords, "awesome")
	assert.Contains(t, keywords, "testing")
	assert.GreaterOrEqual(t, len(keywords), 2)
}

func TestGenerateHighlights(t *testing.T) {
	service := &PackageSearchService{
		logger: nil,
	}

	tests := []struct {
		name     string
		release  *models.PackageRelease
		expected int // minimum number of highlights
	}{
		{
			name: "basic release",
			release: &models.PackageRelease{
				PackageName: "test-package",
				Version:     "1.0.0",
				Description: strPtr("A test package"),
			},
			expected: 2, // Name + description
		},
		{
			name: "breaking change release",
			release: &models.PackageRelease{
				PackageName:      "test-package",
				Version:          "2.0.0",
				IsBreakingChange: true,
			},
			expected: 2, // Name + breaking change indicator
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			highlights := service.generateHighlights("test", tt.release)
			assert.GreaterOrEqual(t, len(highlights), tt.expected)
		})
	}
}

func TestDependencyGraph_Structure(t *testing.T) {
	// Test that dependency graph has correct structure
	graph := &DependencyGraph{
		Root: &DependencyNode{
			PackageName: "root-package",
			Version:     "1.0.0",
			Children:    []*DependencyNode{},
		},
		Nodes: make(map[string]*DependencyNode),
	}

	// Add child dependency
	childNode := &DependencyNode{
		PackageName: "child-package",
		Version:     "^1.0.0",
		Children:    []*DependencyNode{},
	}

	graph.Root.Children = append(graph.Root.Children, childNode)
	graph.Nodes["root-package"] = graph.Root
	graph.Nodes["child-package"] = childNode

	assert.Equal(t, "root-package", graph.Root.PackageName)
	assert.Len(t, graph.Root.Children, 1)
	assert.Equal(t, "child-package", graph.Root.Children[0].PackageName)
	assert.Len(t, graph.Nodes, 2)
}

// Helper function
func strPtr(s string) *string {
	return &s
}

// Benchmark tests
func BenchmarkRelevanceRanker_Rank(b *testing.B) {
	ranker := NewRelevanceRanker(nil)
	now := time.Now()

	result := &PackageSearchResult{
		Release: &models.PackageRelease{
			PackageName:      "test-package",
			Version:          "1.0.0",
			Description:      strPtr("A test package for benchmarking"),
			PublishedAt:      now,
			License:          strPtr("MIT"),
			IsBreakingChange: false,
		},
		Similarity: 0.85,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ranker.Rank("test package", result)
	}
}

func BenchmarkExtractMatchedKeywords(b *testing.B) {
	service := &PackageSearchService{
		logger: nil,
	}

	release := &models.PackageRelease{
		PackageName: "my-awesome-package",
		Description: strPtr("A package for testing and development with many features"),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		service.extractMatchedKeywords("awesome testing features", release)
	}
}
