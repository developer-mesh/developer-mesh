package search

import (
	"math"
	"strings"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
)

// RelevanceRanker ranks search results based on multiple factors
type RelevanceRanker struct {
	logger observability.Logger
}

// NewRelevanceRanker creates a new relevance ranker
func NewRelevanceRanker(logger observability.Logger) *RelevanceRanker {
	if logger == nil {
		logger = observability.NewLogger("relevance-ranker")
	}

	return &RelevanceRanker{
		logger: logger,
	}
}

// Rank calculates a relevance score for a search result
// Score is a weighted combination of:
// - Semantic similarity (60% weight)
// - Keyword matching (20% weight)
// - Recency (10% weight)
// - Popularity signals (10% weight)
func (r *RelevanceRanker) Rank(query string, result *PackageSearchResult) float64 {
	if result == nil || result.Release == nil {
		return 0.0
	}

	// Base score from semantic similarity (0.0 to 1.0)
	semanticScore := result.Similarity * 0.6

	// Keyword matching score
	keywordScore := r.calculateKeywordScore(query, result.Release) * 0.2

	// Recency score (newer releases rank higher)
	recencyScore := r.calculateRecencyScore(result.Release) * 0.1

	// Popularity score (based on signals like breaking changes, etc.)
	popularityScore := r.calculatePopularityScore(result.Release) * 0.1

	// Combine scores
	totalScore := semanticScore + keywordScore + recencyScore + popularityScore

	r.logger.Debug("Calculated relevance score", map[string]interface{}{
		"package":          result.Release.PackageName,
		"version":          result.Release.Version,
		"semantic_score":   semanticScore,
		"keyword_score":    keywordScore,
		"recency_score":    recencyScore,
		"popularity_score": popularityScore,
		"total_score":      totalScore,
	})

	return totalScore
}

// calculateKeywordScore calculates a score based on keyword matching
func (r *RelevanceRanker) calculateKeywordScore(query string, release *models.PackageRelease) float64 {
	queryWords := strings.Fields(strings.ToLower(query))
	if len(queryWords) == 0 {
		return 0.0
	}

	matchCount := 0
	totalWords := len(queryWords)

	// Build searchable text from release
	searchableText := strings.ToLower(release.PackageName)
	if release.Description != nil {
		searchableText += " " + strings.ToLower(*release.Description)
	}
	if release.ReleaseNotes != nil {
		searchableText += " " + strings.ToLower(*release.ReleaseNotes)
	}

	// Count matches
	for _, word := range queryWords {
		if len(word) < 3 {
			// Skip very short words
			continue
		}
		if strings.Contains(searchableText, word) {
			matchCount++
		}
	}

	// Calculate percentage of matched words
	score := float64(matchCount) / float64(totalWords)

	// Boost score if package name contains query
	if strings.Contains(strings.ToLower(release.PackageName), strings.ToLower(query)) {
		score = math.Min(score+0.3, 1.0)
	}

	return score
}

// calculateRecencyScore calculates a score based on release recency
func (r *RelevanceRanker) calculateRecencyScore(release *models.PackageRelease) float64 {
	// Score decays over time using exponential decay
	// Recent releases (< 30 days) get close to 1.0
	// Older releases gradually decrease

	// Calculate seconds since publication
	now := time.Now()
	secondsSincePublished := now.Sub(release.PublishedAt).Seconds()

	// Use exponential decay with half-life of 90 days
	halfLifeDays := 90.0 * 24 * 60 * 60 // 90 days in seconds
	decayRate := math.Log(2) / halfLifeDays

	// Calculate score (1.0 for new, approaches 0 for very old)
	score := math.Exp(-decayRate * secondsSincePublished)

	// Clamp between 0 and 1
	return math.Max(0.0, math.Min(1.0, score))
}

// calculatePopularityScore calculates a score based on popularity signals
func (r *RelevanceRanker) calculatePopularityScore(release *models.PackageRelease) float64 {
	score := 0.5 // Base score

	// Penalize breaking changes slightly (users may want stable versions)
	if release.IsBreakingChange {
		score -= 0.2
	}

	// Boost if it has good documentation
	if release.DocumentationURL != nil && *release.DocumentationURL != "" {
		score += 0.1
	}

	// Boost if it has a homepage
	if release.Homepage != nil && *release.Homepage != "" {
		score += 0.1
	}

	// Boost if it has a license
	if release.License != nil && *release.License != "" {
		score += 0.1
	}

	// Penalize prereleases
	if release.Prerelease != nil && *release.Prerelease != "" {
		score -= 0.15
	}

	// Clamp between 0 and 1
	return math.Max(0.0, math.Min(1.0, score))
}

// RankResults ranks a list of search results in place
func (r *RelevanceRanker) RankResults(query string, results []*PackageSearchResult) {
	for _, result := range results {
		result.Score = r.Rank(query, result)
	}
}
