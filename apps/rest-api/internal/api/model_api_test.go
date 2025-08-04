package api_test

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	api "github.com/developer-mesh/developer-mesh/apps/rest-api/internal/api"
	"github.com/developer-mesh/developer-mesh/apps/rest-api/internal/repository"

	"github.com/developer-mesh/developer-mesh/pkg/models"
	"github.com/developer-mesh/developer-mesh/pkg/testutil"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockModelRepository mocks repository.ModelRepository
// Implements testify's Mock for all methods
// All methods must match the ModelRepository interface
// Used for unit testing ModelAPI

type MockModelRepository struct {
	mock.Mock
}

func (m *MockModelRepository) CreateModel(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}
func (m *MockModelRepository) ListModels(ctx context.Context, tenantID string) ([]*models.Model, error) {
	args := m.Called(ctx, tenantID)
	return args.Get(0).([]*models.Model), args.Error(1)
}
func (m *MockModelRepository) UpdateModel(ctx context.Context, model *models.Model) error {
	args := m.Called(ctx, model)
	return args.Error(0)
}
func (m *MockModelRepository) GetModelByID(ctx context.Context, tenantID, modelID string) (*models.Model, error) {
	args := m.Called(ctx, tenantID, modelID)
	return args.Get(0).(*models.Model), args.Error(1)
}

func (m *MockModelRepository) DeleteModel(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockModelRepository) SearchModels(ctx context.Context, tenantID, query string, limit, offset int) ([]*models.Model, error) {
	args := m.Called(ctx, tenantID, query, limit, offset)
	return args.Get(0).([]*models.Model), args.Error(1)
}

// Helper to set up Gin and handler
func setupModelAPI(repo repository.ModelRepository, withTenant bool) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	if withTenant {
		r.Use(func(c *gin.Context) {
			c.Set("user", map[string]any{"tenant_id": testutil.TestTenantIDString()})
			c.Next()
		})
	}
	a := api.NewModelAPI(repo)
	a.RegisterRoutes(r.Group("/"))
	return r
}

func TestCreateModel_Success(t *testing.T) {
	repo := new(MockModelRepository)
	repo.On("CreateModel", mock.Anything, mock.AnythingOfType("*models.Model")).Return(nil)

	r := setupModelAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Model"}`)
	req, _ := http.NewRequest("POST", "/models", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusCreated, w.Code)
	repo.AssertCalled(t, "CreateModel", mock.Anything, mock.AnythingOfType("*models.Model"))
}

func TestCreateModel_MissingTenant(t *testing.T) {
	repo := new(MockModelRepository)
	r := setupModelAPI(repo, false)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Test Model"}`)
	req, _ := http.NewRequest("POST", "/models", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestListModels_Success(t *testing.T) {
	repo := new(MockModelRepository)
	modelsList := []*models.Model{{ID: "m1", TenantID: testutil.TestTenantIDString(), Name: "Model1"}}
	repo.On("ListModels", mock.Anything, testutil.TestTenantIDString()).Return(modelsList, nil)

	r := setupModelAPI(repo, true)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/models", nil)

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestUpdateModel_NotFound(t *testing.T) {
	repo := new(MockModelRepository)
	repo.On("GetModelByID", mock.Anything, testutil.TestTenantIDString(), "m1").Return((*models.Model)(nil), errors.New("not found"))

	r := setupModelAPI(repo, true)
	w := httptest.NewRecorder()
	body := []byte(`{"name":"Updated"}`)
	req, _ := http.NewRequest("PUT", "/models/m1", bytes.NewBuffer(body))

	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// More tests can be added for error cases, unauthorized, etc.
