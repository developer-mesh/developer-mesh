package scoring

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/developer-mesh/developer-mesh/pkg/rag/models"
)

func TestNewDefaultScorer(t *testing.T) {
	scorer := NewDefaultScorer()
	assert.NotNil(t, scorer)
	assert.Equal(t, 0.3, scorer.weights.Base)
	assert.Equal(t, 0.2, scorer.weights.Freshness)
	assert.Equal(t, 0.2, scorer.weights.Authority)
	assert.Equal(t, 0.2, scorer.weights.Popularity)
	assert.Equal(t, 0.1, scorer.weights.Quality)
}

func TestCalculateFreshnessScore(t *testing.T) {
	scorer := NewDefaultScorer()

	tests := []struct {
		name        string
		updatedAt   time.Time
		expectedMin float64
		expectedMax float64
		description string
	}{
		{
			name:        "Today",
			updatedAt:   time.Now(),
			expectedMin: 0.9,
			expectedMax: 1.0,
			description: "Very fresh content",
		},
		{
			name:        "Last Week",
			updatedAt:   time.Now().AddDate(0, 0, -5),
			expectedMin: 0.85,
			expectedMax: 0.95,
			description: "Recent content",
		},
		{
			name:        "Last Month",
			updatedAt:   time.Now().AddDate(0, -1, 0),
			expectedMin: 0.4,
			expectedMax: 0.8,
			description: "Month-old content",
		},
		{
			name:        "Last Quarter",
			updatedAt:   time.Now().AddDate(0, -2, 0),
			expectedMin: 0.4,
			expectedMax: 0.6,
			description: "Quarter-old content",
		},
		{
			name:        "Old",
			updatedAt:   time.Now().AddDate(-1, 0, 0),
			expectedMin: 0.2,
			expectedMax: 0.4,
			description: "Year-old content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.CalculateFreshnessScore(tt.updatedAt)
			assert.GreaterOrEqual(t, score, tt.expectedMin, tt.description)
			assert.LessOrEqual(t, score, tt.expectedMax, tt.description)
		})
	}
}

func TestCalculateQualityScore(t *testing.T) {
	scorer := NewDefaultScorer()

	tests := []struct {
		name        string
		content     string
		title       string
		expectedMin float64
		expectedMax float64
	}{
		{
			name:        "Short content",
			content:     "Short",
			title:       "Title",
			expectedMin: 0.3,
			expectedMax: 0.5,
		},
		{
			name:        "Medium content",
			content:     generateContent(600),
			title:       "Good Title",
			expectedMin: 0.6,
			expectedMax: 0.8,
		},
		{
			name:        "Long content",
			content:     generateContent(1500),
			title:       "Comprehensive Title",
			expectedMin: 0.7,
			expectedMax: 1.0,
		},
		{
			name:        "No title",
			content:     generateContent(800),
			title:       "",
			expectedMin: 0.5,
			expectedMax: 0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.CalculateQualityScore(tt.content, tt.title)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, tt.expectedMax)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestCalculateImportance(t *testing.T) {
	scorer := NewDefaultScorer()

	tests := []struct {
		name      string
		doc       *models.Document
		expected  float64
		tolerance float64
	}{
		{
			name: "High importance document",
			doc: &models.Document{
				BaseScore:       0.8,
				FreshnessScore:  0.9,
				AuthorityScore:  0.7,
				PopularityScore: 0.6,
				QualityScore:    0.8,
				UpdatedAt:       time.Now(),
			},
			expected:  0.76, // Approximate
			tolerance: 0.05,
		},
		{
			name: "Low importance document",
			doc: &models.Document{
				BaseScore:       0.3,
				FreshnessScore:  0.4,
				AuthorityScore:  0.3,
				PopularityScore: 0.4,
				QualityScore:    0.3,
				UpdatedAt:       time.Now().AddDate(-1, 0, 0),
			},
			expected:  0.2, // Approximate with time decay
			tolerance: 0.1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.CalculateImportance(tt.doc)
			assert.InDelta(t, tt.expected, score, tt.tolerance)
			assert.GreaterOrEqual(t, score, 0.0)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestApplySourceBoost(t *testing.T) {
	scorer := NewDefaultScorer()

	tests := []struct {
		name         string
		sourceType   string
		initialScore float64
		expectedMin  float64
	}{
		{
			name:         "GitHub",
			sourceType:   "github",
			initialScore: 0.5,
			expectedMin:  0.5,
		},
		{
			name:         "Confluence",
			sourceType:   "confluence",
			initialScore: 0.5,
			expectedMin:  0.55,
		},
		{
			name:         "Jira",
			sourceType:   "jira",
			initialScore: 0.5,
			expectedMin:  0.45,
		},
		{
			name:         "Web",
			sourceType:   "web",
			initialScore: 0.5,
			expectedMin:  0.4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := scorer.ApplySourceBoost(tt.sourceType, tt.initialScore)
			assert.GreaterOrEqual(t, score, tt.expectedMin)
			assert.LessOrEqual(t, score, 1.0)
		})
	}
}

func TestScoreDocument(t *testing.T) {
	scorer := NewDefaultScorer()

	doc := &models.Document{
		ID:         uuid.New(),
		TenantID:   uuid.New(),
		SourceID:   "test-source",
		SourceType: "github",
		Title:      "README.md",
		Content:    generateContent(1000),
		UpdatedAt:  time.Now(),
		BaseScore:  0.7,
	}

	// Score the document
	scorer.ScoreDocument(doc)

	// Verify all scores are calculated
	assert.Greater(t, doc.FreshnessScore, 0.0)
	assert.Greater(t, doc.QualityScore, 0.0)
	assert.Greater(t, doc.ImportanceScore, 0.0)

	// Verify scores are in valid range
	assert.LessOrEqual(t, doc.FreshnessScore, 1.0)
	assert.LessOrEqual(t, doc.QualityScore, 1.0)
	assert.LessOrEqual(t, doc.ImportanceScore, 1.0)
}

func TestTimeDecay(t *testing.T) {
	scorer := NewDefaultScorer()

	tests := []struct {
		name         string
		updatedAt    time.Time
		initialScore float64
		maxDecay     float64 // Maximum decay percentage
	}{
		{
			name:         "Recent (no decay)",
			updatedAt:    time.Now().AddDate(0, 0, -3),
			initialScore: 1.0,
			maxDecay:     0.0,
		},
		{
			name:         "Month old (small decay)",
			updatedAt:    time.Now().AddDate(0, -1, 0),
			initialScore: 1.0,
			maxDecay:     0.2, // 5% decay = 0.05, so max 0.2
		},
		{
			name:         "Quarter old (medium decay)",
			updatedAt:    time.Now().AddDate(0, -3, 0),
			initialScore: 1.0,
			maxDecay:     0.4, // 20% decay = 0.2, so max 0.4
		},
		{
			name:         "Half year old (significant decay)",
			updatedAt:    time.Now().AddDate(0, -6, 0),
			initialScore: 1.0,
			maxDecay:     0.6, // 40% decay = 0.4, so max 0.6
		},
		{
			name:         "Year old (major decay)",
			updatedAt:    time.Now().AddDate(-1, 0, 0),
			initialScore: 1.0,
			maxDecay:     0.7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decayedScore := scorer.applyTimeDecay(tt.updatedAt, tt.initialScore)
			expectedMin := tt.initialScore * (1.0 - tt.maxDecay)
			assert.GreaterOrEqual(t, decayedScore, expectedMin)
			assert.LessOrEqual(t, decayedScore, tt.initialScore)
		})
	}
}

func TestCustomWeights(t *testing.T) {
	customWeights := ScoringWeights{
		Base:       0.4,
		Freshness:  0.3,
		Authority:  0.1,
		Popularity: 0.1,
		Quality:    0.1,
	}

	scorer := NewScorer(customWeights)

	doc := &models.Document{
		BaseScore:       0.8,
		FreshnessScore:  0.9,
		AuthorityScore:  0.5,
		PopularityScore: 0.5,
		QualityScore:    0.6,
		UpdatedAt:       time.Now(),
	}

	score := scorer.CalculateImportance(doc)

	// With high base and freshness weights, and high base/freshness scores
	// the overall score should be relatively high
	assert.Greater(t, score, 0.6)
	assert.LessOrEqual(t, score, 1.0)
}

// Helper function to generate content of specified length
func generateContent(length int) string {
	content := ""
	word := "word "
	for len(content) < length {
		content += word
	}
	return content[:length]
}
