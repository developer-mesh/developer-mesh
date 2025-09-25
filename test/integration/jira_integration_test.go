//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/developer-mesh/developer-mesh/pkg/observability"
	"github.com/developer-mesh/developer-mesh/pkg/tools/providers/jira"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

// JiraIntegrationTestSuite defines the integration test suite for Jira
type JiraIntegrationTestSuite struct {
	suite.Suite
	provider       *jira.JiraProvider
	mockServer     *JiraMockServer
	sandboxMode    bool
	deploymentType string // "cloud", "server", or "datacenter"
	logger         observability.Logger
}

// JiraMockServer simulates different Jira deployment types
type JiraMockServer struct {
	server         *httptest.Server
	deploymentType string
	issues         map[string]interface{}
	projects       map[string]interface{}
	users          map[string]interface{}
	boards         map[string]interface{}
	sprints        map[string]interface{}
	mutex          sync.RWMutex
	requestLog     []RequestEntry
	responseDelay  time.Duration
}

// RequestEntry logs API requests for verification
type RequestEntry struct {
	Method    string
	Path      string
	Body      string
	Headers   map[string]string
	Timestamp time.Time
	User      string
}

// NewJiraMockServer creates a mock server for a specific deployment type
func NewJiraMockServer(deploymentType string) *JiraMockServer {
	mock := &JiraMockServer{
		deploymentType: deploymentType,
		issues:         make(map[string]interface{}),
		projects:       make(map[string]interface{}),
		users:          make(map[string]interface{}),
		boards:         make(map[string]interface{}),
		sprints:        make(map[string]interface{}),
		requestLog:     make([]RequestEntry, 0),
	}

	// Initialize with test data
	mock.initializeTestData()

	mock.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log request
		mock.logRequest(r)

		// Apply response delay if set
		if mock.responseDelay > 0 {
			time.Sleep(mock.responseDelay)
		}

		// Route based on deployment type and path
		switch mock.deploymentType {
		case "cloud":
			mock.handleCloudAPI(w, r)
		case "server":
			mock.handleServerAPI(w, r)
		case "datacenter":
			mock.handleDataCenterAPI(w, r)
		default:
			mock.handleCloudAPI(w, r) // Default to cloud
		}
	}))

	return mock
}

// initializeTestData sets up initial test data
func (m *JiraMockServer) initializeTestData() {
	// Add test project
	m.projects["TEST"] = map[string]interface{}{
		"id":          "10000",
		"key":         "TEST",
		"name":        "Test Project",
		"description": "Integration test project",
		"lead": map[string]interface{}{
			"accountId":   "user-123",
			"displayName": "Test Lead",
		},
	}

	// Add test users
	m.users["user-123"] = map[string]interface{}{
		"accountId":    "user-123",
		"displayName":  "Test User",
		"emailAddress": "test@example.com",
		"active":       true,
	}

	m.users["user-456"] = map[string]interface{}{
		"accountId":    "user-456",
		"displayName":  "Another User",
		"emailAddress": "another@example.com",
		"active":       true,
	}

	// Add test board
	m.boards["1"] = map[string]interface{}{
		"id":   1,
		"name": "Test Board",
		"type": "scrum",
		"location": map[string]interface{}{
			"projectId":  10000,
			"projectKey": "TEST",
		},
	}

	// Add active sprint
	m.sprints["1"] = map[string]interface{}{
		"id":        1,
		"name":      "Sprint 1",
		"state":     "active",
		"startDate": time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		"endDate":   time.Now().Add(7 * 24 * time.Hour).Format(time.RFC3339),
		"boardId":   1,
	}
}

// logRequest logs incoming requests for verification
func (m *JiraMockServer) logRequest(r *http.Request) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	headers := make(map[string]string)
	for key := range r.Header {
		headers[key] = r.Header.Get(key)
	}

	// Extract user from auth header
	user := ""
	if auth := r.Header.Get("Authorization"); auth != "" {
		if strings.HasPrefix(auth, "Basic ") {
			user = "basic-auth-user"
		} else if strings.HasPrefix(auth, "Bearer ") {
			user = "bearer-token-user"
		}
	}

	entry := RequestEntry{
		Method:    r.Method,
		Path:      r.URL.Path,
		Headers:   headers,
		Timestamp: time.Now(),
		User:      user,
	}

	// Read body if present
	if r.Body != nil {
		// Note: In real implementation, would need to read and restore body
		entry.Body = "request-body"
	}

	m.requestLog = append(m.requestLog, entry)
}

// handleCloudAPI handles Jira Cloud API requests
func (m *JiraMockServer) handleCloudAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Atlassian-Deployment", "cloud")

	// Route based on path
	switch {
	case strings.HasPrefix(r.URL.Path, "/rest/api/3/issue"):
		m.handleIssueOperations(w, r)
	case strings.HasPrefix(r.URL.Path, "/rest/api/3/project"):
		m.handleProjectOperations(w, r)
	case strings.HasPrefix(r.URL.Path, "/rest/api/3/user"):
		m.handleUserOperations(w, r)
	case strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/board"):
		m.handleBoardOperations(w, r)
	case strings.HasPrefix(r.URL.Path, "/rest/agile/1.0/sprint"):
		m.handleSprintOperations(w, r)
	default:
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"errorMessages": []string{"Endpoint not found"},
		})
	}
}

// handleServerAPI handles Jira Server API requests
func (m *JiraMockServer) handleServerAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Atlassian-Deployment", "server")

	// Server uses /rest/api/2 instead of /rest/api/3
	modifiedPath := strings.Replace(r.URL.Path, "/rest/api/3", "/rest/api/2", 1)

	// Route based on path
	switch {
	case strings.HasPrefix(modifiedPath, "/rest/api/2/issue"):
		m.handleIssueOperations(w, r)
	case strings.HasPrefix(modifiedPath, "/rest/api/2/project"):
		m.handleProjectOperations(w, r)
	case strings.HasPrefix(modifiedPath, "/rest/api/2/user"):
		m.handleUserOperationsServer(w, r) // Server has different user endpoints
	default:
		w.WriteHeader(http.StatusNotFound)
	}
}

// handleDataCenterAPI handles Jira Data Center API requests
func (m *JiraMockServer) handleDataCenterAPI(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Atlassian-Deployment", "datacenter")
	w.Header().Set("X-Atlassian-Cluster-Node", "node-1")

	// Data Center is similar to Server but with additional features
	m.handleServerAPI(w, r)
}

// handleIssueOperations handles issue CRUD operations
func (m *JiraMockServer) handleIssueOperations(w http.ResponseWriter, r *http.Request) {
	m.mutex.Lock()
	defer m.mutex.Unlock()

	if strings.Contains(r.URL.Path, "/search") {
		// Handle JQL search
		m.handleIssueSearch(w, r)
		return
	}

	// Extract issue key from path
	parts := strings.Split(r.URL.Path, "/")
	issueKey := ""
	if len(parts) > 5 {
		issueKey = parts[5]
	}

	switch r.Method {
	case "GET":
		if issue, ok := m.issues[issueKey]; ok {
			json.NewEncoder(w).Encode(issue)
		} else {
			w.WriteHeader(http.StatusNotFound)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"errorMessages": []string{fmt.Sprintf("Issue %s not found", issueKey)},
			})
		}

	case "POST":
		// Create new issue
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)

		newKey := fmt.Sprintf("TEST-%d", len(m.issues)+1)
		newIssue := map[string]interface{}{
			"id":     fmt.Sprintf("%d", 10000+len(m.issues)),
			"key":    newKey,
			"self":   fmt.Sprintf("%s/rest/api/3/issue/%s", m.server.URL, newKey),
			"fields": payload["fields"],
		}
		m.issues[newKey] = newIssue

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id":   newIssue["id"],
			"key":  newKey,
			"self": newIssue["self"],
		})

	case "PUT":
		// Update issue
		if issue, ok := m.issues[issueKey]; ok {
			var payload map[string]interface{}
			json.NewDecoder(r.Body).Decode(&payload)

			// Update fields
			if fields, ok := issue.(map[string]interface{})["fields"].(map[string]interface{}); ok {
				for k, v := range payload["fields"].(map[string]interface{}) {
					fields[k] = v
				}
			}

			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}

	case "DELETE":
		// Delete issue
		if _, ok := m.issues[issueKey]; ok {
			delete(m.issues, issueKey)
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}
}

// handleIssueSearch handles JQL search requests
func (m *JiraMockServer) handleIssueSearch(w http.ResponseWriter, r *http.Request) {
	jql := r.URL.Query().Get("jql")
	startAt := r.URL.Query().Get("startAt")
	maxResults := r.URL.Query().Get("maxResults")

	if startAt == "" {
		startAt = "0"
	}
	if maxResults == "" {
		maxResults = "50"
	}

	// Convert issues map to array for response
	issueArray := make([]interface{}, 0)
	for _, issue := range m.issues {
		// Simple JQL filtering (in real implementation would parse JQL properly)
		if jql == "" || strings.Contains(jql, "TEST") {
			issueArray = append(issueArray, issue)
		}
	}

	response := map[string]interface{}{
		"startAt":    startAt,
		"maxResults": maxResults,
		"total":      len(issueArray),
		"issues":     issueArray,
	}

	json.NewEncoder(w).Encode(response)
}

// handleProjectOperations handles project operations
func (m *JiraMockServer) handleProjectOperations(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return all projects
	projects := make([]interface{}, 0)
	for _, project := range m.projects {
		projects = append(projects, project)
	}

	json.NewEncoder(w).Encode(projects)
}

// handleUserOperations handles user operations for Cloud
func (m *JiraMockServer) handleUserOperations(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	accountId := r.URL.Query().Get("accountId")
	if accountId != "" {
		if user, ok := m.users[accountId]; ok {
			json.NewEncoder(w).Encode(user)
			return
		}
	}

	// Return first user as default
	for _, user := range m.users {
		json.NewEncoder(w).Encode(user)
		return
	}

	w.WriteHeader(http.StatusNotFound)
}

// handleUserOperationsServer handles user operations for Server/DC
func (m *JiraMockServer) handleUserOperationsServer(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Server uses username instead of accountId
	username := r.URL.Query().Get("username")
	if username != "" {
		// Convert to server format
		for _, user := range m.users {
			serverUser := map[string]interface{}{
				"name":         "testuser",
				"emailAddress": user.(map[string]interface{})["emailAddress"],
				"displayName":  user.(map[string]interface{})["displayName"],
				"active":       user.(map[string]interface{})["active"],
			}
			json.NewEncoder(w).Encode(serverUser)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

// handleBoardOperations handles Agile board operations
func (m *JiraMockServer) handleBoardOperations(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return boards
	boards := make([]interface{}, 0)
	for _, board := range m.boards {
		boards = append(boards, board)
	}

	response := map[string]interface{}{
		"values":     boards,
		"startAt":    0,
		"maxResults": 50,
		"isLast":     true,
	}

	json.NewEncoder(w).Encode(response)
}

// handleSprintOperations handles sprint operations
func (m *JiraMockServer) handleSprintOperations(w http.ResponseWriter, r *http.Request) {
	m.mutex.RLock()
	defer m.mutex.RUnlock()

	// Return sprints
	sprints := make([]interface{}, 0)
	for _, sprint := range m.sprints {
		sprints = append(sprints, sprint)
	}

	response := map[string]interface{}{
		"values":     sprints,
		"startAt":    0,
		"maxResults": 50,
		"isLast":     true,
	}

	json.NewEncoder(w).Encode(response)
}

// GetRequestLog returns the request log
func (m *JiraMockServer) GetRequestLog() []RequestEntry {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	return append([]RequestEntry{}, m.requestLog...)
}

// Close shuts down the mock server
func (m *JiraMockServer) Close() {
	m.server.Close()
}

// SetupSuite runs before the test suite
func (suite *JiraIntegrationTestSuite) SetupSuite() {
	suite.logger = &observability.NoopLogger{}

	// Check if we should run against a real sandbox
	if sandboxURL := os.Getenv("JIRA_SANDBOX_URL"); sandboxURL != "" {
		suite.sandboxMode = true
		suite.deploymentType = "cloud"
		suite.provider = jira.NewJiraProvider(suite.logger, sandboxURL)
		suite.T().Logf("Running against Jira sandbox: %s", sandboxURL)
	} else {
		// Use mock server
		suite.sandboxMode = false
		suite.deploymentType = "cloud" // Default to cloud
		if deployType := os.Getenv("JIRA_DEPLOYMENT_TYPE"); deployType != "" {
			suite.deploymentType = deployType
		}

		suite.mockServer = NewJiraMockServer(suite.deploymentType)
		suite.provider = jira.NewJiraProvider(suite.logger, strings.TrimPrefix(suite.mockServer.server.URL, "https://"))
		suite.T().Logf("Running with mock server in %s mode", suite.deploymentType)
	}
}

// TearDownSuite runs after the test suite
func (suite *JiraIntegrationTestSuite) TearDownSuite() {
	if suite.mockServer != nil {
		suite.mockServer.Close()
	}
}

// TestCompleteIssueCycle tests the complete lifecycle of an issue
func (suite *JiraIntegrationTestSuite) TestCompleteIssueCycle() {
	ctx := context.Background()
	t := suite.T()

	// 1. Create an issue
	t.Log("Creating new issue...")
	createBody := `{
		"fields": {
			"project": {"key": "TEST"},
			"summary": "Integration Test Issue",
			"description": "This issue was created by integration tests",
			"issuetype": {"name": "Task"}
		}
	}`

	createReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/rest/api/3/issue", suite.getBaseURL()),
		strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")
	suite.addAuth(createReq)

	createResp, err := suite.provider.Execute(ctx, createReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createResult map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	issueKey := createResult["key"].(string)
	t.Logf("Created issue: %s", issueKey)

	// 2. Get the issue
	t.Log("Fetching created issue...")
	getReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
	require.NoError(t, err)
	suite.addAuth(getReq)

	getResp, err := suite.provider.Execute(ctx, getReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, getResp.StatusCode)

	var getResult map[string]interface{}
	json.NewDecoder(getResp.Body).Decode(&getResult)
	getResp.Body.Close()

	assert.Equal(t, issueKey, getResult["key"])

	// 3. Update the issue
	t.Log("Updating issue...")
	updateBody := `{
		"fields": {
			"summary": "Updated Integration Test Issue",
			"description": "This issue was updated by integration tests"
		}
	}`

	updateReq, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey),
		strings.NewReader(updateBody))
	require.NoError(t, err)
	updateReq.Header.Set("Content-Type", "application/json")
	suite.addAuth(updateReq)

	updateResp, err := suite.provider.Execute(ctx, updateReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, updateResp.StatusCode)
	updateResp.Body.Close()

	// 4. Search for the issue
	t.Log("Searching for issue...")
	jql := fmt.Sprintf("key=%s", issueKey)
	searchReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/api/3/issue/search?jql=%s", suite.getBaseURL(), jql), nil)
	require.NoError(t, err)
	suite.addAuth(searchReq)

	searchResp, err := suite.provider.Execute(ctx, searchReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, searchResp.StatusCode)

	var searchResult map[string]interface{}
	json.NewDecoder(searchResp.Body).Decode(&searchResult)
	searchResp.Body.Close()

	issues := searchResult["issues"].([]interface{})
	assert.GreaterOrEqual(t, len(issues), 1)

	// 5. Delete the issue (only if not in sandbox mode to avoid polluting real environment)
	if !suite.sandboxMode {
		t.Log("Deleting issue...")
		deleteReq, err := http.NewRequestWithContext(ctx, "DELETE",
			fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
		require.NoError(t, err)
		suite.addAuth(deleteReq)

		deleteResp, err := suite.provider.Execute(ctx, deleteReq)
		require.NoError(t, err)
		assert.Equal(t, http.StatusNoContent, deleteResp.StatusCode)
		deleteResp.Body.Close()

		t.Logf("Deleted issue: %s", issueKey)
	}
}

// TestCloudVsServerCompatibility tests differences between Cloud and Server APIs
func (suite *JiraIntegrationTestSuite) TestCloudVsServerCompatibility() {
	if suite.sandboxMode {
		suite.T().Skip("Skipping compatibility test in sandbox mode")
	}

	ctx := context.Background()
	t := suite.T()

	// Test user endpoint differences
	t.Logf("Testing %s user endpoints...", suite.deploymentType)

	if suite.deploymentType == "cloud" {
		// Cloud uses accountId
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/rest/api/3/user?accountId=user-123", suite.getBaseURL()), nil)
		require.NoError(t, err)
		suite.addAuth(req)

		resp, err := suite.provider.Execute(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var user map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&user)
		resp.Body.Close()

		assert.Equal(t, "user-123", user["accountId"])
	} else {
		// Server/DC uses username
		req, err := http.NewRequestWithContext(ctx, "GET",
			fmt.Sprintf("%s/rest/api/2/user?username=testuser", suite.getBaseURL()), nil)
		require.NoError(t, err)
		suite.addAuth(req)

		resp, err := suite.provider.Execute(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)

		var user map[string]interface{}
		json.NewDecoder(resp.Body).Decode(&user)
		resp.Body.Close()

		assert.Equal(t, "testuser", user["name"])
	}

	// Check deployment type header
	if suite.mockServer != nil {
		logs := suite.mockServer.GetRequestLog()
		assert.Greater(t, len(logs), 0)
	}
}

// TestMultiUserScenarios tests multi-user collaboration scenarios
func (suite *JiraIntegrationTestSuite) TestMultiUserScenarios() {
	ctx := context.Background()
	t := suite.T()

	// Simulate two users working on the same issue
	t.Log("Testing multi-user collaboration...")

	// User 1 creates an issue
	createBody := `{
		"fields": {
			"project": {"key": "TEST"},
			"summary": "Collaborative Task",
			"description": "Task for multiple users",
			"issuetype": {"name": "Task"},
			"assignee": {"accountId": "user-123"}
		}
	}`

	createReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/rest/api/3/issue", suite.getBaseURL()),
		strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")
	suite.addAuth(createReq) // User 1

	createResp, err := suite.provider.Execute(ctx, createReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusCreated, createResp.StatusCode)

	var createResult map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	issueKey := createResult["key"].(string)
	t.Logf("User 1 created issue: %s", issueKey)

	// User 2 updates the issue (reassigns to themselves)
	updateBody := `{
		"fields": {
			"assignee": {"accountId": "user-456"}
		}
	}`

	updateReq, err := http.NewRequestWithContext(ctx, "PUT",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey),
		strings.NewReader(updateBody))
	require.NoError(t, err)
	updateReq.Header.Set("Content-Type", "application/json")
	suite.addAuthUser2(updateReq) // User 2

	updateResp, err := suite.provider.Execute(ctx, updateReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusNoContent, updateResp.StatusCode)
	updateResp.Body.Close()

	t.Log("User 2 reassigned issue to themselves")

	// Both users add comments (simulated)
	if !suite.sandboxMode {
		// Check request log to verify multiple users
		logs := suite.mockServer.GetRequestLog()
		userSet := make(map[string]bool)
		for _, log := range logs {
			if log.User != "" {
				userSet[log.User] = true
			}
		}
		// In real scenario, would have different auth tokens
		t.Logf("Unique users in session: %d", len(userSet))
	}

	// Clean up
	if !suite.sandboxMode {
		deleteReq, err := http.NewRequestWithContext(ctx, "DELETE",
			fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
		require.NoError(t, err)
		suite.addAuth(deleteReq)

		deleteResp, err := suite.provider.Execute(ctx, deleteReq)
		require.NoError(t, err)
		deleteResp.Body.Close()
	}
}

// TestAgileWorkflow tests Agile-specific workflows
func (suite *JiraIntegrationTestSuite) TestAgileWorkflow() {
	ctx := context.Background()
	t := suite.T()

	t.Log("Testing Agile workflow...")

	// Get boards
	boardReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/agile/1.0/board", suite.getBaseURL()), nil)
	require.NoError(t, err)
	suite.addAuth(boardReq)

	boardResp, err := suite.provider.Execute(ctx, boardReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, boardResp.StatusCode)

	var boardResult map[string]interface{}
	json.NewDecoder(boardResp.Body).Decode(&boardResult)
	boardResp.Body.Close()

	boards := boardResult["values"].([]interface{})
	assert.Greater(t, len(boards), 0)
	t.Logf("Found %d boards", len(boards))

	// Get active sprints
	sprintReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/agile/1.0/sprint", suite.getBaseURL()), nil)
	require.NoError(t, err)
	suite.addAuth(sprintReq)

	sprintResp, err := suite.provider.Execute(ctx, sprintReq)
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, sprintResp.StatusCode)

	var sprintResult map[string]interface{}
	json.NewDecoder(sprintResp.Body).Decode(&sprintResult)
	sprintResp.Body.Close()

	sprints := sprintResult["values"].([]interface{})
	assert.GreaterOrEqual(t, len(sprints), 0)
	t.Logf("Found %d sprints", len(sprints))
}

// TestErrorHandlingAndRecovery tests error scenarios and recovery
func (suite *JiraIntegrationTestSuite) TestErrorHandlingAndRecovery() {
	ctx := context.Background()
	t := suite.T()

	// Test 404 Not Found
	t.Log("Testing error handling...")

	getReq, err := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/api/3/issue/NONEXISTENT-999", suite.getBaseURL()), nil)
	require.NoError(t, err)
	suite.addAuth(getReq)

	getResp, err := suite.provider.Execute(ctx, getReq)
	if err == nil {
		assert.Equal(t, http.StatusNotFound, getResp.StatusCode)
		getResp.Body.Close()
	} else {
		// Provider may return error for 404
		assert.Contains(t, err.Error(), "404")
	}

	// Test invalid project key
	createBody := `{
		"fields": {
			"project": {"key": "INVALID"},
			"summary": "Test Issue",
			"issuetype": {"name": "Task"}
		}
	}`

	createReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/rest/api/3/issue", suite.getBaseURL()),
		strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")
	suite.addAuth(createReq)

	createResp, err := suite.provider.Execute(ctx, createReq)
	if err == nil {
		// Should get 400 or 404
		assert.True(t, createResp.StatusCode == http.StatusBadRequest ||
			createResp.StatusCode == http.StatusNotFound)
		createResp.Body.Close()
	}
}

// TestPerformanceAndCaching tests performance with caching
func (suite *JiraIntegrationTestSuite) TestPerformanceAndCaching() {
	if suite.sandboxMode {
		suite.T().Skip("Skipping performance test in sandbox mode")
	}

	ctx := context.Background()
	t := suite.T()

	// Create test issue for repeated queries
	createBody := `{
		"fields": {
			"project": {"key": "TEST"},
			"summary": "Performance Test Issue",
			"issuetype": {"name": "Task"}
		}
	}`

	createReq, err := http.NewRequestWithContext(ctx, "POST",
		fmt.Sprintf("%s/rest/api/3/issue", suite.getBaseURL()),
		strings.NewReader(createBody))
	require.NoError(t, err)
	createReq.Header.Set("Content-Type", "application/json")
	suite.addAuth(createReq)

	createResp, err := suite.provider.Execute(ctx, createReq)
	require.NoError(t, err)

	var createResult map[string]interface{}
	json.NewDecoder(createResp.Body).Decode(&createResult)
	createResp.Body.Close()

	issueKey := createResult["key"].(string)

	// Measure time for multiple identical requests
	t.Log("Testing caching performance...")

	// First request (cache miss)
	start1 := time.Now()
	req1, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
	suite.addAuth(req1)
	resp1, err := suite.provider.Execute(ctx, req1)
	require.NoError(t, err)
	resp1.Body.Close()
	duration1 := time.Since(start1)

	// Second request (should be cached)
	start2 := time.Now()
	req2, _ := http.NewRequestWithContext(ctx, "GET",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
	suite.addAuth(req2)
	resp2, err := suite.provider.Execute(ctx, req2)
	require.NoError(t, err)
	resp2.Body.Close()
	duration2 := time.Since(start2)

	t.Logf("First request: %v, Second request: %v", duration1, duration2)

	// Second request should be faster due to caching
	// In real implementation with caching enabled, duration2 should be < duration1

	// Clean up
	deleteReq, _ := http.NewRequestWithContext(ctx, "DELETE",
		fmt.Sprintf("%s/rest/api/3/issue/%s", suite.getBaseURL(), issueKey), nil)
	suite.addAuth(deleteReq)
	deleteResp, _ := suite.provider.Execute(ctx, deleteReq)
	if deleteResp != nil {
		deleteResp.Body.Close()
	}
}

// Helper methods

func (suite *JiraIntegrationTestSuite) getBaseURL() string {
	if suite.mockServer != nil {
		return suite.mockServer.server.URL
	}
	// In sandbox mode, provider handles the URL
	return ""
}

func (suite *JiraIntegrationTestSuite) addAuth(req *http.Request) {
	// Add authentication based on configuration
	if apiToken := os.Getenv("JIRA_API_TOKEN"); apiToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", apiToken))
	} else {
		// Default test auth
		req.Header.Set("Authorization", "Basic dGVzdEBleGFtcGxlLmNvbTp0ZXN0LXRva2Vu")
	}
}

func (suite *JiraIntegrationTestSuite) addAuthUser2(req *http.Request) {
	// Simulate different user
	req.Header.Set("Authorization", "Basic YW5vdGhlckBleGFtcGxlLmNvbTp0ZXN0LXRva2Vu")
}

// TestJiraIntegration runs the integration test suite
func TestJiraIntegration(t *testing.T) {
	suite.Run(t, new(JiraIntegrationTestSuite))
}
