package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentRepository mocks repository.AgentRepository
type MockAgentRepository struct {
	mock.Mock
}

func (m *MockAgentRepository) CreateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}
func (m *MockAgentRepository) ListAgents(ctx context.Context, tenantID string) ([]*models.Agent, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Agent), args.Error(1)
}
func (m *MockAgentRepository) GetAgentByID(ctx context.Context, tenantID, id string) (*models.Agent, error) {
	args := m.Called(ctx, tenantID, id)
	return args.Get(0).(*models.Agent), args.Error(1)
}
func (m *MockAgentRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	args := m.Called(ctx, agent)
	return args.Error(0)
}

func (m *MockAgentRepository) DeleteAgent(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// Helper to set up Gin and handler
func setupAgentAPI(repo repository.AgentRepository, withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withTenant {
		r.Use(func(c *gin.Context) {
			c.Set("user", map[string]interface{}{"tenant_id": testutil.TestTenantIDString()})
			c.Next()
		})
	}
	a := NewAgentAPI(repo)
	a.RegisterRoutes(r.Group("/"))
	return r
}

func TestCreateAgent_Success(t *testing.T) {
	repo := new(MockAgentRepository)
	repo.On("CreateAgent", mock.Anything, mock.AnythingOfType("*models.Agent")).Return(nil)

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Agent"}`)
	req, _ := http.NewRequest("POST", "/agents", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertCalled(t, "CreateAgent", mock.Anything, mock.AnythingOfType("*models.Agent"))
}

func TestCreateAgent_MissingTenant(t *testing.T) {
	repo := new(MockAgentRepository)
	r := setupAgentAPI(repo, false)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Agent"}`)
	req, _ := http.NewRequest("POST", "/agents", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListAgents_Success(t *testing.T) {
	repo := new(MockAgentRepository)
	agents := []*models.Agent{{ID: "a1", TenantID: testutil.TestTenantID, Name: "Agent1"}}
	repo.On("ListAgents", mock.Anything, testutil.TestTenantIDString()).Return(agents, nil)

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/agents", nil)

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateAgent_NotFound(t *testing.T) {
	repo := new(MockAgentRepository)
	repo.On("GetAgentByID", mock.Anything, testutil.TestTenantIDString(), "a1").Return((*models.Agent)(nil), errors.New("not found"))

	r := setupAgentAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Updated"}`)
	req, _ := http.NewRequest("PUT", "/agents/a1", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// More tests can be added for error cases, unauthorized, etc.
