// Package scoring provides document scoring and ranking functionality
package scoring

import (
	"math"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

// ScoringWeights defines the weights for different scoring components
type ScoringWeights struct {
	Base       float64 // Base importance weight
	Freshness  float64 // Time-based freshness weight
	Authority  float64 // Author/source authority weight
	Popularity float64 // Usage/engagement weight
	Quality    float64 // Content quality weight
}

// DefaultWeights returns the default scoring weights
func DefaultWeights() ScoringWeights {
	return ScoringWeights{
		Base:       0.3,
		Freshness:  0.2,
		Authority:  0.2,
		Popularity: 0.2,
		Quality:    0.1,
	}
}

// Scorer calculates importance scores for documents
type Scorer struct {
	weights ScoringWeights
}

// NewScorer creates a new document scorer
func NewScorer(weights ScoringWeights) *Scorer {
	return &Scorer{
		weights: weights,
	}
}

// NewDefaultScorer creates a scorer with default weights
func NewDefaultScorer() *Scorer {
	return NewScorer(DefaultWeights())
}

// CalculateImportance calculates the final importance score for a document
func (s *Scorer) CalculateImportance(doc *models.Document) float64 {
	importance := doc.BaseScore*s.weights.Base +
		doc.FreshnessScore*s.weights.Freshness +
		doc.AuthorityScore*s.weights.Authority +
		doc.PopularityScore*s.weights.Popularity +
		doc.QualityScore*s.weights.Quality

	// Apply time decay
	importance = s.applyTimeDecay(doc.UpdatedAt, importance)

	// Ensure score is between 0 and 1
	return math.Min(math.Max(importance, 0.0), 1.0)
}

// CalculateFreshnessScore calculates freshness score based on last update time
func (s *Scorer) CalculateFreshnessScore(updatedAt time.Time) float64 {
	daysSinceUpdate := time.Since(updatedAt).Hours() / 24

	if daysSinceUpdate < 1 {
		return 1.0 // Very fresh
	} else if daysSinceUpdate < 7 {
		return 0.9 // Recent (last week)
	} else if daysSinceUpdate < 30 {
		return 0.7 // Last month
	} else if daysSinceUpdate < 90 {
		return 0.5 // Last quarter
	}

	return 0.3 // Older content
}

// CalculateQualityScore calculates quality score based on content characteristics
func (s *Scorer) CalculateQualityScore(content string, title string) float64 {
	score := 0.5 // Base quality score

	contentLength := len(content)

	// Boost for substantial content
	if contentLength > 1000 {
		score += 0.2
	} else if contentLength > 500 {
		score += 0.1
	}

	// Penalize very short content
	if contentLength < 100 {
		score -= 0.2
	}

	// Boost for well-structured content (has title)
	if title != "" && len(title) > 5 {
		score += 0.1
	}

	return math.Min(math.Max(score, 0.0), 1.0)
}

// applyTimeDecay applies time-based decay to the importance score
func (s *Scorer) applyTimeDecay(updatedAt time.Time, score float64) float64 {
	daysSinceUpdate := time.Since(updatedAt).Hours() / 24

	if daysSinceUpdate < 7 {
		return score // No decay for recent content
	} else if daysSinceUpdate < 30 {
		return score * 0.95 // 5% decay for month-old content
	} else if daysSinceUpdate < 90 {
		return score * 0.8 // 20% decay for quarter-old content
	} else if daysSinceUpdate < 180 {
		return score * 0.6 // 40% decay for half-year-old content
	}

	return score * 0.4 // 60% decay for older content
}

// ApplySourceBoost applies source-specific boosting
func (s *Scorer) ApplySourceBoost(sourceType string, score float64) float64 {
	boosts := map[string]float64{
		"github":     1.0,  // No boost
		"confluence": 1.1,  // Documentation is important
		"jira":       0.9,  // Issues are less important
		"web":        0.8,  // External web content
		"local":      1.05, // Local documentation slightly boosted
	}

	boost, exists := boosts[sourceType]
	if !exists {
		boost = 1.0
	}

	return math.Min(score*boost, 1.0)
}

// ScoreDocument calculates all scoring components for a document
func (s *Scorer) ScoreDocument(doc *models.Document) {
	// Calculate individual scores if not already set
	if doc.FreshnessScore == 0 {
		doc.FreshnessScore = s.CalculateFreshnessScore(doc.UpdatedAt)
	}

	if doc.QualityScore == 0 {
		doc.QualityScore = s.CalculateQualityScore(doc.Content, doc.Title)
	}

	// Calculate final importance score
	doc.ImportanceScore = s.CalculateImportance(doc)

	// Apply source-specific boosting
	doc.ImportanceScore = s.ApplySourceBoost(doc.SourceType, doc.ImportanceScore)
}
