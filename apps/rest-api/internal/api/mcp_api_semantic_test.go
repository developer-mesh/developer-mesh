package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/repository"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockSemanticContextManager is a mock implementation of repository.SemanticContextManager
type MockSemanticContextManager struct {
	mock.Mock
}

func (m *MockSemanticContextManager) CreateContext(ctx context.Context, req *repository.CreateContextRequest) (*repository.Context, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Context), args.Error(1)
}

func (m *MockSemanticContextManager) GetContext(ctx context.Context, contextID string, opts *repository.RetrievalOptions) (*repository.Context, error) {
	args := m.Called(ctx, contextID, opts)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Context), args.Error(1)
}

func (m *MockSemanticContextManager) UpdateContext(ctx context.Context, contextID string, update *repository.ContextUpdate) error {
	args := m.Called(ctx, contextID, update)
	return args.Error(0)
}

func (m *MockSemanticContextManager) DeleteContext(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockSemanticContextManager) SearchContext(ctx context.Context, query string, contextID string, limit int) ([]*repository.ContextItem, error) {
	args := m.Called(ctx, query, contextID, limit)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*repository.ContextItem), args.Error(1)
}

func (m *MockSemanticContextManager) CompactContext(ctx context.Context, contextID string, strategy repository.CompactionStrategy) error {
	args := m.Called(ctx, contextID, strategy)
	return args.Error(0)
}

func (m *MockSemanticContextManager) GetRelevantContext(ctx context.Context, contextID string, query string, maxTokens int) (*repository.Context, error) {
	args := m.Called(ctx, contextID, query, maxTokens)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*repository.Context), args.Error(1)
}

func (m *MockSemanticContextManager) PromoteToHot(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockSemanticContextManager) ArchiveToCold(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

func (m *MockSemanticContextManager) AuditContextAccess(ctx context.Context, contextID string, operation string) error {
	args := m.Called(ctx, contextID, operation)
	return args.Error(0)
}

func (m *MockSemanticContextManager) ValidateContextIntegrity(ctx context.Context, contextID string) error {
	args := m.Called(ctx, contextID)
	return args.Error(0)
}

// TestGetContextWithSemanticRetrieval tests the enhanced getContext endpoint with semantic retrieval
func TestGetContextWithSemanticRetrieval(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		contextID          string
		queryParams        string
		setupMocks         func(*MockContextManager, *MockSemanticContextManager)
		expectedStatusCode int
		expectedSemantic   bool
	}{
		{
			name:        "semantic retrieval with relevant_to parameter",
			contextID:   "ctx-123",
			queryParams: "?relevant_to=authentication%20errors&max_tokens=2000",
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				relevantCtx := &repository.Context{
					ID:   "ctx-123",
					Name: "Relevant Context",
				}
				mscm.On("GetRelevantContext", mock.Anything, "ctx-123", "authentication errors", 2000).Return(relevantCtx, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedSemantic:   true,
		},
		{
			name:        "fallback to regular retrieval without relevant_to",
			contextID:   "ctx-456",
			queryParams: "",
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				regularCtx := &models.Context{
					ID:   "ctx-456",
					Name: "Regular Context",
				}
				mcm.On("GetContext", mock.Anything, "ctx-456").Return(regularCtx, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedSemantic:   false,
		},
		{
			name:        "semantic retrieval error handling",
			contextID:   "ctx-error",
			queryParams: "?relevant_to=test",
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				mscm.On("GetRelevantContext", mock.Anything, "ctx-error", "test", 4000).Return(nil, assert.AnError)
			},
			expectedStatusCode: http.StatusInternalServerError,
			expectedSemantic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtxMgr := new(MockContextManager)
			mockSemanticMgr := new(MockSemanticContextManager)

			tt.setupMocks(mockCtxMgr, mockSemanticMgr)

			api := NewMCPAPI(mockCtxMgr)
			api.SetSemanticContextManager(mockSemanticMgr)

			router := gin.Default()
			router.GET("/mcp/context/:id", api.getContext)

			req, _ := http.NewRequest("GET", "/mcp/context/"+tt.contextID+tt.queryParams, nil)
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatusCode, resp.Code)
			if tt.expectedStatusCode == http.StatusOK {
				assert.Contains(t, resp.Body.String(), `"semantic":`)
			}

			mockCtxMgr.AssertExpectations(t)
			mockSemanticMgr.AssertExpectations(t)
		})
	}
}

// TestCompactContext tests the new compactContext endpoint
func TestCompactContext(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		contextID          string
		requestBody        string
		setupMocks         func(*MockSemanticContextManager)
		expectedStatusCode int
		withSemanticMgr    bool
	}{
		{
			name:        "successful compaction with tool_clear strategy",
			contextID:   "ctx-123",
			requestBody: `{"strategy": "tool_clear"}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				mscm.On("CompactContext", mock.Anything, "ctx-123", repository.CompactionToolClear).Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			withSemanticMgr:    true,
		},
		{
			name:        "successful compaction with summarize strategy",
			contextID:   "ctx-456",
			requestBody: `{"strategy": "summarize"}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				mscm.On("CompactContext", mock.Anything, "ctx-456", repository.CompactionSummarize).Return(nil)
			},
			expectedStatusCode: http.StatusOK,
			withSemanticMgr:    true,
		},
		{
			name:        "invalid strategy",
			contextID:   "ctx-789",
			requestBody: `{"strategy": "invalid_strategy"}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				// No mock setup needed - validation fails before calling
			},
			expectedStatusCode: http.StatusBadRequest,
			withSemanticMgr:    true,
		},
		{
			name:        "missing strategy",
			contextID:   "ctx-error",
			requestBody: `{}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				// No mock setup needed
			},
			expectedStatusCode: http.StatusBadRequest,
			withSemanticMgr:    true,
		},
		{
			name:        "semantic manager not available",
			contextID:   "ctx-no-mgr",
			requestBody: `{"strategy": "tool_clear"}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				// No mock setup needed
			},
			expectedStatusCode: http.StatusNotImplemented,
			withSemanticMgr:    false,
		},
		{
			name:        "compaction error",
			contextID:   "ctx-err",
			requestBody: `{"strategy": "prune"}`,
			setupMocks: func(mscm *MockSemanticContextManager) {
				mscm.On("CompactContext", mock.Anything, "ctx-err", repository.CompactionPrune).Return(assert.AnError)
			},
			expectedStatusCode: http.StatusInternalServerError,
			withSemanticMgr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtxMgr := new(MockContextManager)
			mockSemanticMgr := new(MockSemanticContextManager)

			tt.setupMocks(mockSemanticMgr)

			api := NewMCPAPI(mockCtxMgr)
			if tt.withSemanticMgr {
				api.SetSemanticContextManager(mockSemanticMgr)
			}

			router := gin.Default()
			router.POST("/mcp/context/:id/compact", api.compactContext)

			req, _ := http.NewRequest("POST", "/mcp/context/"+tt.contextID+"/compact", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatusCode, resp.Code)

			if tt.expectedStatusCode == http.StatusOK {
				assert.Contains(t, resp.Body.String(), `"status":"compacted"`)
				assert.Contains(t, resp.Body.String(), tt.contextID)
			}

			mockSemanticMgr.AssertExpectations(t)
		})
	}
}

// TestSearchContextWithSemantic tests the enhanced searchContext endpoint
func TestSearchContextWithSemantic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name               string
		contextID          string
		requestBody        string
		setupMocks         func(*MockContextManager, *MockSemanticContextManager)
		expectedStatusCode int
		expectedSemantic   bool
	}{
		{
			name:        "semantic search enabled",
			contextID:   "ctx-123",
			requestBody: `{"query": "authentication error", "semantic": true, "limit": 5}`,
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				results := []*repository.ContextItem{
					{ID: "item-1", ContextID: "ctx-123", Content: "Error with auth", Type: "user"},
					{ID: "item-2", ContextID: "ctx-123", Content: "Let me help with authentication", Type: "assistant"},
				}
				mscm.On("SearchContext", mock.Anything, "authentication error", "ctx-123", 5).Return(results, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedSemantic:   true,
		},
		{
			name:        "regular text search",
			contextID:   "ctx-456",
			requestBody: `{"query": "test query", "semantic": false}`,
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				results := []models.ContextItem{
					{Role: "user", Content: "test query result"},
				}
				mcm.On("SearchInContext", mock.Anything, "ctx-456", "test query").Return(results, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedSemantic:   false,
		},
		{
			name:        "semantic search with default limit",
			contextID:   "ctx-789",
			requestBody: `{"query": "error handling", "semantic": true}`,
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				results := []*repository.ContextItem{}
				mscm.On("SearchContext", mock.Anything, "error handling", "ctx-789", 10).Return(results, nil)
			},
			expectedStatusCode: http.StatusOK,
			expectedSemantic:   true,
		},
		{
			name:        "missing query parameter",
			contextID:   "ctx-err",
			requestBody: `{"semantic": true}`,
			setupMocks: func(mcm *MockContextManager, mscm *MockSemanticContextManager) {
				// No mock setup needed - validation fails
			},
			expectedStatusCode: http.StatusBadRequest,
			expectedSemantic:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockCtxMgr := new(MockContextManager)
			mockSemanticMgr := new(MockSemanticContextManager)

			tt.setupMocks(mockCtxMgr, mockSemanticMgr)

			api := NewMCPAPI(mockCtxMgr)
			api.SetSemanticContextManager(mockSemanticMgr)

			router := gin.Default()
			router.POST("/mcp/context/:id/search", api.searchContext)

			req, _ := http.NewRequest("POST", "/mcp/context/"+tt.contextID+"/search", strings.NewReader(tt.requestBody))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			assert.Equal(t, tt.expectedStatusCode, resp.Code)

			if tt.expectedStatusCode == http.StatusOK {
				assert.Contains(t, resp.Body.String(), `"semantic":`)
				assert.Contains(t, resp.Body.String(), `"count":`)
			}

			mockCtxMgr.AssertExpectations(t)
			mockSemanticMgr.AssertExpectations(t)
		})
	}
}
