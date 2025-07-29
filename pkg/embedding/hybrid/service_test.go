package hybrid

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/developer-mesh/developer-mesh/pkg/auth"
	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockEmbeddingService is a mock implementation of EmbeddingService
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, text, contentType, model string) (*EmbeddingVector, error) {
	args := m.Called(ctx, text, contentType, model)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*EmbeddingVector), args.Error(1)
}

func TestNewHybridSearchService(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mockEmbeddingService := new(MockEmbeddingService)

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				DB:               db,
				EmbeddingService: mockEmbeddingService,
			},
			wantErr: false,
		},
		{
			name: "missing DB",
			config: &Config{
				EmbeddingService: mockEmbeddingService,
			},
			wantErr: true,
		},
		{
			name: "missing embedding service",
			config: &Config{
				DB: db,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, err := NewHybridSearchService(tt.config)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, service)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, service)
			}
		})
	}
}

func TestHybridSearchService_Search(t *testing.T) {
	db, dbMock, err := sqlmock.New()
	require.NoError(t, err)
	defer func() { _ = db.Close() }()

	mockEmbeddingService := new(MockEmbeddingService)
	logger := observability.NewLogger("test")
	metrics := observability.NewMetricsClient()

	service, err := NewHybridSearchService(&Config{
		DB:               db,
		EmbeddingService: mockEmbeddingService,
		Logger:           logger,
		Metrics:          metrics,
		MaxConcurrency:   1,
	})
	require.NoError(t, err)

	tenantID := uuid.New()
	ctx := auth.WithTenantID(context.Background(), tenantID)

	t.Run("successful search", func(t *testing.T) {
		query := "test query"
		embedding := &EmbeddingVector{
			Vector: []float32{0.1, 0.2, 0.3},
		}

		// Mock embedding generation
		mockEmbeddingService.On("GenerateEmbedding", mock.Anything, query, "search_query", "").
			Return(embedding, nil).Once()

		// Mock statistics update
		dbMock.ExpectExec("SELECT update_embedding_statistics").
			WithArgs(tenantID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		// Mock statistics retrieval
		dbMock.ExpectQuery("SELECT total_documents, avg_document_length").
			WithArgs(tenantID).
			WillReturnRows(sqlmock.NewRows([]string{"total_documents", "avg_document_length"}).
				AddRow(1000, 100.0))

		// Mock vector search
		vectorRows := sqlmock.NewRows([]string{
			"id", "content_id", "content", "content_type",
			"metadata", "model_name", "created_at", "similarity",
		}).AddRow(
			uuid.New(), "content1", "Test content", "text",
			[]byte(`{"key": "value"}`), "text-embedding-3-small", time.Now(), 0.9,
		)

		dbMock.ExpectQuery("SELECT(.+)FROM embeddings e(.+)ORDER BY similarity DESC").
			WithArgs(
				pq.Array(embedding.Vector),
				tenantID,
				float32(0.5),
			).
			WillReturnRows(vectorRows)

		// Mock keyword search
		keywordRows := sqlmock.NewRows([]string{
			"id", "content_id", "content", "content_type",
			"metadata", "model_name", "created_at", "score",
		}).AddRow(
			uuid.New(), "content2", "Test keyword content", "text",
			[]byte(`{"key": "value2"}`), "text-embedding-3-small", time.Now(), 5.2,
		)

		dbMock.ExpectQuery("SELECT(.+)FROM embeddings e(.+)ORDER BY score DESC").
			WithArgs(
				pq.Array([]string{"test", "query"}),
				100.0,
				1000,
				tenantID,
				"test & query",
			).
			WillReturnRows(keywordRows)

		opts := &SearchOptions{
			Limit:         10,
			VectorWeight:  0.7,
			KeywordWeight: 0.3,
			MinSimilarity: 0.5,
		}

		results, err := service.Search(ctx, query, opts)
		assert.NoError(t, err)
		assert.NotNil(t, results)
		assert.Greater(t, len(results.Results), 0)
		assert.Equal(t, query, results.Query)
		assert.Greater(t, results.SearchTime, float64(0))

		mockEmbeddingService.AssertExpectations(t)
		assert.NoError(t, dbMock.ExpectationsWereMet())
	})

	t.Run("empty query", func(t *testing.T) {
		results, err := service.Search(ctx, "", nil)
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "query cannot be empty")
	})

	t.Run("missing tenant ID", func(t *testing.T) {
		results, err := service.Search(context.Background(), "test", nil)
		assert.Error(t, err)
		assert.Nil(t, results)
		assert.Contains(t, err.Error(), "tenant ID not found")
	})
}

func TestHybridSearchService_tokenizeQuery(t *testing.T) {
	service := &HybridSearchService{}

	tests := []struct {
		name     string
		query    string
		expected []string
	}{
		{
			name:     "simple query",
			query:    "test query search",
			expected: []string{"test", "query", "search"},
		},
		{
			name:     "query with stop words",
			query:    "the quick brown fox and the lazy dog",
			expected: []string{"quick", "brown", "fox", "lazy", "dog"},
		},
		{
			name:     "query with punctuation",
			query:    "hello, world! how are you?",
			expected: []string{"hello", "world", "how", "you"},
		},
		{
			name:     "empty query",
			query:    "",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.tokenizeQuery(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHybridSearchService_highlightKeywords(t *testing.T) {
	service := &HybridSearchService{}

	tests := []struct {
		name     string
		content  string
		keywords []string
		expected string
	}{
		{
			name:     "single keyword",
			content:  "This is a test content",
			keywords: []string{"test"},
			expected: "This is a <mark>test</mark> content",
		},
		{
			name:     "multiple keywords",
			content:  "The quick brown fox jumps",
			keywords: []string{"quick", "fox"},
			expected: "The <mark>quick</mark> brown <mark>fox</mark> jumps",
		},
		{
			name:     "case insensitive",
			content:  "This is a TEST content",
			keywords: []string{"test"},
			expected: "This is a <mark>TEST</mark> content",
		},
		{
			name:     "no matches",
			content:  "This is a sample content",
			keywords: []string{"missing"},
			expected: "This is a sample content",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := service.highlightKeywords(tt.content, tt.keywords)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHybridSearchService_fuseResults(t *testing.T) {
	service := &HybridSearchService{}

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()

	vectorResults := []*SearchResult{
		{
			ID:          id1,
			VectorScore: 0.9,
			ContentID:   "content1",
		},
		{
			ID:          id2,
			VectorScore: 0.8,
			ContentID:   "content2",
		},
	}

	keywordResults := []*SearchResult{
		{
			ID:           id2,
			KeywordScore: 5.0,
			ContentID:    "content2",
		},
		{
			ID:           id3,
			KeywordScore: 4.0,
			ContentID:    "content3",
		},
	}

	opts := &SearchOptions{
		VectorWeight:  0.7,
		KeywordWeight: 0.3,
		FusionK:       60,
	}

	results := service.fuseResults(vectorResults, keywordResults, opts)

	assert.Len(t, results, 3)

	// Check that results are sorted by hybrid score
	for i := 0; i < len(results)-1; i++ {
		assert.GreaterOrEqual(t, results[i].HybridScore, results[i+1].HybridScore)
	}

	// Check that duplicate was merged
	for _, r := range results {
		if r.ID == id2 {
			assert.Greater(t, r.VectorScore, float32(0))
			assert.Greater(t, r.KeywordScore, float32(0))
		}
	}
}

func TestHybridSearchService_buildFilters(t *testing.T) {
	service := &HybridSearchService{}

	tests := []struct {
		name     string
		opts     *SearchOptions
		expected int // expected number of filters
	}{
		{
			name:     "no filters",
			opts:     &SearchOptions{},
			expected: 0,
		},
		{
			name: "agent ID filter",
			opts: &SearchOptions{
				AgentID: &[]uuid.UUID{uuid.New()}[0],
			},
			expected: 1,
		},
		{
			name: "multiple filters",
			opts: &SearchOptions{
				AgentID:       &[]uuid.UUID{uuid.New()}[0],
				ContentTypes:  []string{"text", "code"},
				SearchModels:  []string{"model1"},
				ExcludeModels: []string{"model2"},
				DateFrom:      &[]time.Time{time.Now().Add(-24 * time.Hour)}[0],
				DateTo:        &[]time.Time{time.Now()}[0],
				Filters: map[string]interface{}{
					"custom": "value",
				},
			},
			expected: 7,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := []interface{}{}
			argCount := 0
			filters := service.buildFilters(tt.opts, &args, &argCount)

			if tt.expected == 0 {
				assert.Empty(t, filters)
			} else {
				assert.NotEmpty(t, filters)
				assert.Equal(t, tt.expected, len(args))
			}
		})
	}
}
