package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/developer-mesh/developer-mesh/pkg/search"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// PackageSearchHandler handles package search API endpoints
type PackageSearchHandler struct {
	searchService *search.PackageSearchService
}

// NewPackageSearchHandler creates a new package search handler
func NewPackageSearchHandler(searchService *search.PackageSearchService) *PackageSearchHandler {
	return &PackageSearchHandler{
		searchService: searchService,
	}
}

// PackageSearchRequest represents a package search request
type PackageSearchRequest struct {
	Query           string   `json:"query" binding:"required"`
	PackageTypes    []string `json:"package_types,omitempty"`
	Repositories    []string `json:"repositories,omitempty"`
	VersionRange    string   `json:"version_range,omitempty"`
	IncludeBreaking bool     `json:"include_breaking"`
	OnlyLatest      bool     `json:"only_latest"`
	MinSimilarity   float64  `json:"min_similarity,omitempty"`
	Limit           int      `json:"limit,omitempty"`
	Offset          int      `json:"offset,omitempty"`
}

// PackageSearchResponse represents a package search response
type PackageSearchResponse struct {
	Results    []*PackageSearchResultResponse `json:"results"`
	TotalCount int                            `json:"total_count"`
	Query      string                         `json:"query"`
	Limit      int                            `json:"limit"`
	Offset     int                            `json:"offset"`
}

// PackageSearchResultResponse represents a single search result
type PackageSearchResultResponse struct {
	Package         PackageInfo `json:"package"`
	Similarity      float64     `json:"similarity"`
	Score           float64     `json:"score"`
	MatchedKeywords []string    `json:"matched_keywords,omitempty"`
	Highlights      []string    `json:"highlights,omitempty"`
}

// PackageInfo represents package information in the response
type PackageInfo struct {
	ID               uuid.UUID              `json:"id"`
	PackageName      string                 `json:"package_name"`
	Version          string                 `json:"version"`
	PackageType      string                 `json:"package_type"`
	Repository       string                 `json:"repository"`
	Description      *string                `json:"description,omitempty"`
	License          *string                `json:"license,omitempty"`
	Homepage         *string                `json:"homepage,omitempty"`
	PublishedAt      string                 `json:"published_at"`
	IsBreakingChange bool                   `json:"is_breaking_change"`
	ArtifactoryPath  *string                `json:"artifactory_path,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
}

// PackageHistoryResponse represents package version history
type PackageHistoryResponse struct {
	PackageName string                 `json:"package_name"`
	Versions    []*VersionInfoResponse `json:"versions"`
	TotalCount  int                    `json:"total_count"`
}

// VersionInfoResponse represents a single version in history
type VersionInfoResponse struct {
	ID               uuid.UUID `json:"id"`
	Version          string    `json:"version"`
	PublishedAt      string    `json:"published_at"`
	IsBreakingChange bool      `json:"is_breaking_change"`
	Prerelease       *string   `json:"prerelease,omitempty"`
	PackageType      string    `json:"package_type"`
}

// DependencyGraphResponse represents a dependency graph
type DependencyGraphResponse struct {
	PackageName string                  `json:"package_name"`
	Version     string                  `json:"version"`
	Root        *DependencyNodeResponse `json:"root"`
	TotalNodes  int                     `json:"total_nodes"`
}

// DependencyNodeResponse represents a node in the dependency graph
type DependencyNodeResponse struct {
	PackageName     string                    `json:"package_name"`
	Version         string                    `json:"version"`
	ResolvedVersion *string                   `json:"resolved_version,omitempty"`
	DependencyType  *string                   `json:"dependency_type,omitempty"`
	Children        []*DependencyNodeResponse `json:"children,omitempty"`
}

// SearchPackages godoc
// @Summary Search packages semantically
// @Description Performs semantic search across package releases using embeddings
// @Tags packages
// @Accept json
// @Produce json
// @Param request body PackageSearchRequest true "Search parameters"
// @Param tenant_id query string true "Tenant ID"
// @Success 200 {object} PackageSearchResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /packages/search [post]
func (h *PackageSearchHandler) SearchPackages(c *gin.Context) {
	// Get tenant ID from context or query parameter
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		// Try to get from auth context
		if tenantID, exists := c.Get("tenant_id"); exists {
			tenantIDStr = tenantID.(string)
		}
	}

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "MISSING_TENANT_ID",
			"message": "tenant_id is required in query parameter or auth context",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_TENANT_ID",
			"message": "tenant_id must be a valid UUID",
		})
		return
	}

	// Parse request body
	var req PackageSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "INVALID_REQUEST",
			"message": err.Error(),
		})
		return
	}

	// Get agent ID from context if available
	agentID := ""
	if agentIDVal, exists := c.Get("agent_id"); exists {
		agentID = agentIDVal.(string)
	}

	// Build search query
	query := search.PackageSearchQuery{
		Query:           req.Query,
		PackageTypes:    req.PackageTypes,
		Repositories:    req.Repositories,
		VersionRange:    req.VersionRange,
		IncludeBreaking: req.IncludeBreaking,
		OnlyLatest:      req.OnlyLatest,
		MinSimilarity:   req.MinSimilarity,
		Limit:           req.Limit,
		Offset:          req.Offset,
		TenantID:        tenantID,
		AgentID:         agentID,
	}

	// Perform search
	results, err := h.searchService.Search(c.Request.Context(), query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "SEARCH_FAILED",
			"message": err.Error(),
		})
		return
	}

	// Convert results to response
	responseResults := make([]*PackageSearchResultResponse, 0, len(results))
	for _, result := range results {
		responseResults = append(responseResults, &PackageSearchResultResponse{
			Package: PackageInfo{
				ID:               result.Release.ID,
				PackageName:      result.Release.PackageName,
				Version:          result.Release.Version,
				PackageType:      result.Release.PackageType,
				Repository:       result.Release.RepositoryName,
				Description:      result.Release.Description,
				License:          result.Release.License,
				Homepage:         result.Release.Homepage,
				PublishedAt:      result.Release.PublishedAt.Format("2006-01-02T15:04:05Z07:00"),
				IsBreakingChange: result.Release.IsBreakingChange,
				ArtifactoryPath:  result.Release.ArtifactoryPath,
				Metadata:         result.Release.Metadata,
			},
			Similarity:      result.Similarity,
			Score:           result.Score,
			MatchedKeywords: result.MatchedKeywords,
			Highlights:      result.Highlights,
		})
	}

	c.JSON(http.StatusOK, PackageSearchResponse{
		Results:    responseResults,
		TotalCount: len(results),
		Query:      req.Query,
		Limit:      query.Limit,
		Offset:     query.Offset,
	})
}

// GetPackageHistory godoc
// @Summary Get package version history
// @Description Retrieves version history for a package
// @Tags packages
// @Produce json
// @Param tenant_id query string true "Tenant ID"
// @Param package_name path string true "Package name"
// @Param limit query int false "Maximum number of versions to return" default(50)
// @Success 200 {object} PackageHistoryResponse
// @Failure 400 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /packages/{package_name}/history [get]
func (h *PackageSearchHandler) GetPackageHistory(c *gin.Context) {
	// Get tenant ID
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		if tenantID, exists := c.Get("tenant_id"); exists {
			tenantIDStr = tenantID.(string)
		}
	}

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "tenant_id is required",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "tenant_id must be a valid UUID",
		})
		return
	}

	// Get package name from path
	packageName := c.Param("package_name")
	if packageName == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "package_name is required in path",
		})
		return
	}

	// Get limit from query parameter
	limitStr := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}

	// Fetch package history
	versions, err := h.searchService.GetPackageHistory(c.Request.Context(), tenantID, packageName, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Code:    ErrInternalServer,
			Message: err.Error(),
		})
		return
	}

	// Convert to response
	versionResponses := make([]*VersionInfoResponse, 0, len(versions))
	for _, v := range versions {
		versionResponses = append(versionResponses, &VersionInfoResponse{
			ID:               v.ID,
			Version:          v.Version,
			PublishedAt:      v.PublishedAt.(string),
			IsBreakingChange: v.IsBreakingChange,
			Prerelease:       v.Prerelease,
			PackageType:      v.PackageType,
		})
	}

	c.JSON(http.StatusOK, PackageHistoryResponse{
		PackageName: packageName,
		Versions:    versionResponses,
		TotalCount:  len(versionResponses),
	})
}

// GetDependencyGraph godoc
// @Summary Get dependency graph
// @Description Retrieves dependency graph for a package
// @Tags packages
// @Produce json
// @Param tenant_id query string true "Tenant ID"
// @Param package_name path string true "Package name"
// @Param version path string true "Package version"
// @Param depth query int false "Maximum depth to traverse" default(3)
// @Success 200 {object} DependencyGraphResponse
// @Failure 400 {object} ErrorResponse
// @Failure 404 {object} ErrorResponse
// @Failure 500 {object} ErrorResponse
// @Router /packages/{package_name}/{version}/dependencies [get]
func (h *PackageSearchHandler) GetDependencyGraph(c *gin.Context) {
	// Get tenant ID
	tenantIDStr := c.Query("tenant_id")
	if tenantIDStr == "" {
		if tenantID, exists := c.Get("tenant_id"); exists {
			tenantIDStr = tenantID.(string)
		}
	}

	if tenantIDStr == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "tenant_id is required",
		})
		return
	}

	tenantID, err := uuid.Parse(tenantIDStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "tenant_id must be a valid UUID",
		})
		return
	}

	// Get package name and version from path
	packageName := c.Param("package_name")
	version := c.Param("version")

	if packageName == "" || version == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Code:    ErrBadRequest,
			Message: "package_name and version are required",
		})
		return
	}

	// Get depth from query parameter
	depthStr := c.DefaultQuery("depth", "3")
	depth, err := strconv.Atoi(depthStr)
	if err != nil || depth <= 0 {
		depth = 3
	}

	// Build dependency graph
	graph, err := h.searchService.GetDependencyGraph(c.Request.Context(), tenantID, packageName, version, depth)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			c.JSON(http.StatusNotFound, ErrorResponse{
				Code:    ErrNotFound,
				Message: err.Error(),
			})
		} else {
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Code:    ErrInternalServer,
				Message: err.Error(),
			})
		}
		return
	}

	// Convert to response
	rootNode := convertDependencyNode(graph.Root)

	c.JSON(http.StatusOK, DependencyGraphResponse{
		PackageName: packageName,
		Version:     version,
		Root:        rootNode,
		TotalNodes:  len(graph.Nodes),
	})
}

// convertDependencyNode converts internal dependency node to response format
func convertDependencyNode(node *search.DependencyNode) *DependencyNodeResponse {
	if node == nil {
		return nil
	}

	response := &DependencyNodeResponse{
		PackageName:     node.PackageName,
		Version:         node.Version,
		ResolvedVersion: node.ResolvedVersion,
		DependencyType:  node.DependencyType,
		Children:        make([]*DependencyNodeResponse, 0, len(node.Children)),
	}

	for _, child := range node.Children {
		response.Children = append(response.Children, convertDependencyNode(child))
	}

	return response
}

// RegisterRoutes registers package search routes
func (h *PackageSearchHandler) RegisterRoutes(router *gin.RouterGroup) {
	packages := router.Group("/packages")
	{
		packages.POST("/search", h.SearchPackages)
		packages.GET("/:package_name/history", h.GetPackageHistory)
		packages.GET("/:package_name/:version/dependencies", h.GetDependencyGraph)
	}
}
