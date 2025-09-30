package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"
)

// JFrogArtifactoryHandler handles comprehensive mock Artifactory API requests
func JFrogArtifactoryHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock Artifactory request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Check authentication
	apiKey := r.Header.Get("X-JFrog-Art-Api")
	authHeader := r.Header.Get("Authorization")
	if apiKey == "" && authHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"}); err != nil {
			log.Printf("Failed to encode error response: %v", err)
		}
		return
	}

	// Router for different endpoints
	switch {
	case strings.Contains(r.URL.Path, "/api/system/ping"):
		handlePing(w, r)
	case strings.Contains(r.URL.Path, "/api/security/apiKey"):
		handleAPIKeyInfo(w, r)
	case strings.Contains(r.URL.Path, "/api/security/users"):
		handleUsers(w, r)
	case strings.Contains(r.URL.Path, "/api/repositories"):
		handleRepositories(w, r)
	case strings.Contains(r.URL.Path, "/api/search/aql"):
		handleAQL(w, r)
	case strings.Contains(r.URL.Path, "/api/storage"):
		handleStorage(w, r)
	case strings.Contains(r.URL.Path, "/access/api/v1/projects"):
		handleProjects(w, r)
	case strings.Contains(r.URL.Path, "/api/v2/security/permissions"):
		handlePermissions(w, r)
	case strings.Contains(r.URL.Path, "/api/builds"):
		handleBuilds(w, r)
	default:
		handleDefaultArtifactory(w, r)
	}
}

// JFrogXrayHandler handles comprehensive mock Xray API requests
func JFrogXrayHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock Xray request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Check authentication
	apiKey := r.Header.Get("X-JFrog-Art-Api")
	authHeader := r.Header.Get("Authorization")
	if apiKey == "" && authHeader == "" {
		w.WriteHeader(http.StatusUnauthorized)
		if err := json.NewEncoder(w).Encode(map[string]string{"error": "Authentication required"}); err != nil {
			log.Printf("Failed to encode error response: %v", err)
		}
		return
	}

	// Router for different endpoints
	switch {
	case strings.Contains(r.URL.Path, "/xray/api/v1/system/version"):
		handleXrayVersion(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/summary/artifact"):
		handleArtifactSummary(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/scan/artifact"):
		handleArtifactScan(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/scan/build"):
		handleBuildScan(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/scan/status"):
		handleScanStatus(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/component"):
		handleComponents(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/reports"):
		handleReports(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v2/metrics"):
		handleMetrics(w, r)
	case strings.Contains(r.URL.Path, "/xray/api/v1/dependencyGraph"):
		handleDependencyGraph(w, r)
	default:
		handleDefaultXray(w, r)
	}
}

// Artifactory endpoint handlers
func handlePing(w http.ResponseWriter, r *http.Request) {
	if err := json.NewEncoder(w).Encode(map[string]string{"status": "OK"}); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleAPIKeyInfo(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"apiKey":   "test-api-key-1234",
		"username": "test-user",
		"scope":    "member-of-groups:readers,developers",
		"expires":  time.Now().Add(30 * 24 * time.Hour).Unix(),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleUsers(w http.ResponseWriter, r *http.Request) {
	if strings.Contains(r.URL.Path, "/test-user") {
		response := map[string]interface{}{
			"name":         "test-user",
			"email":        "test@example.com",
			"admin":        false,
			"groups":       []string{"readers", "developers"},
			"realm":        "internal",
			"lastLoggedIn": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	} else {
		response := map[string]interface{}{
			"users": []map[string]interface{}{
				{
					"name":  "test-user",
					"email": "test@example.com",
					"admin": false,
				},
				{
					"name":  "admin-user",
					"email": "admin@example.com",
					"admin": true,
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func handleRepositories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		response := []map[string]interface{}{
			{
				"key":         "libs-release-local",
				"type":        "LOCAL",
				"packageType": "Generic",
				"description": "Local repository for release artifacts",
			},
			{
				"key":         "maven-central",
				"type":        "REMOTE",
				"packageType": "Maven",
				"url":         "https://repo1.maven.org/maven2",
				"description": "Maven Central remote repository",
			},
			{
				"key":          "libs-release",
				"type":         "VIRTUAL",
				"packageType":  "Generic",
				"repositories": []string{"libs-release-local", "maven-central"},
				"description":  "Virtual repository aggregating release repositories",
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	case "PUT", "POST":
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "Repository created successfully"}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func handleAQL(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"results": []map[string]interface{}{
			{
				"repo":        "libs-release-local",
				"path":        "com/example/app/1.0.0",
				"name":        "app-1.0.0.jar",
				"type":        "file",
				"size":        10485760,
				"created":     time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
				"created_by":  "test-user",
				"modified":    time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
				"modified_by": "test-user",
				"actual_md5":  "abcd1234abcd1234abcd1234abcd1234",
				"actual_sha1": "abcd1234abcd1234abcd1234abcd1234abcd1234",
			},
			{
				"repo":        "libs-release-local",
				"path":        "com/example/lib/2.0.0",
				"name":        "lib-2.0.0.jar",
				"type":        "file",
				"size":        5242880,
				"created":     time.Now().Add(-14 * 24 * time.Hour).Format(time.RFC3339),
				"created_by":  "test-user",
				"modified":    time.Now().Add(-10 * 24 * time.Hour).Format(time.RFC3339),
				"modified_by": "test-user",
			},
		},
		"range": map[string]interface{}{
			"start_pos": 0,
			"end_pos":   2,
			"total":     2,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleStorage(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"uri":          "http://localhost/artifactory/api/storage/libs-release-local/com/example/app/1.0.0/app-1.0.0.jar",
		"downloadUri":  "http://localhost/artifactory/libs-release-local/com/example/app/1.0.0/app-1.0.0.jar",
		"repo":         "libs-release-local",
		"path":         "/com/example/app/1.0.0/app-1.0.0.jar",
		"created":      time.Now().Add(-7 * 24 * time.Hour).Format(time.RFC3339),
		"createdBy":    "test-user",
		"lastModified": time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		"modifiedBy":   "test-user",
		"lastUpdated":  time.Now().Add(-2 * 24 * time.Hour).Format(time.RFC3339),
		"size":         "10485760",
		"mimeType":     "application/java-archive",
		"checksums": map[string]interface{}{
			"sha1":   "abcd1234abcd1234abcd1234abcd1234abcd1234",
			"md5":    "abcd1234abcd1234abcd1234abcd1234",
			"sha256": "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleProjects(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		response := map[string]interface{}{
			"projects": []map[string]interface{}{
				{
					"project_key":  "proj-1",
					"display_name": "Project One",
					"description":  "First test project",
					"admin_privileges": map[string]interface{}{
						"manage_members":   true,
						"manage_resources": true,
						"index_resources":  true,
					},
					"storage_quota_bytes": 10737418240, // 10GB
					"soft_limit":          false,
				},
			},
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	case "POST", "PUT":
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(map[string]string{"status": "Project created successfully"}); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func handlePermissions(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"permissions": []map[string]interface{}{
			{
				"name": "read-permission",
				"repo": map[string]interface{}{
					"repositories": []string{"libs-release-local"},
					"actions": map[string][]string{
						"users": {"test-user"},
					},
				},
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleBuilds(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"builds": []map[string]interface{}{
			{
				"uri":         "/build-1",
				"lastStarted": time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
			},
			{
				"uri":         "/build-2",
				"lastStarted": time.Now().Add(-48 * time.Hour).Format(time.RFC3339),
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleDefaultArtifactory(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Mock Artifactory response for %s", r.URL.Path),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

// Xray endpoint handlers
func handleXrayVersion(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"xray_version":  "3.82.0",
		"xray_revision": "abc123def",
		"xray_license":  "OSS",
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleArtifactSummary(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"artifacts": []map[string]interface{}{
			{
				"general": map[string]interface{}{
					"component_id": "gav://com.example:app:1.0.0",
					"name":         "app",
					"path":         "libs-release-local/com/example/app/1.0.0/app-1.0.0.jar",
					"pkg_type":     "Maven",
					"sha256":       "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
				},
				"issues": []map[string]interface{}{
					{
						"summary":    "High severity vulnerability",
						"severity":   "High",
						"issue_type": "security",
						"provider":   "JFrog",
						"cves": []map[string]interface{}{
							{
								"cve":            "CVE-2023-1234",
								"cvss_v3_score":  "7.5",
								"cvss_v3_vector": "CVSS:3.1/AV:N/AC:L/PR:N/UI:N/S:U/C:H/I:N/A:N",
							},
						},
						"components": []map[string]interface{}{
							{
								"component_id":   "gav://org.apache.logging.log4j:log4j-core:2.14.1",
								"fixed_versions": []string{"2.17.1", "2.18.0"},
							},
						},
					},
				},
				"licenses": []map[string]interface{}{
					{
						"license":    "Apache-2.0",
						"full_name":  "Apache License 2.0",
						"components": []string{"gav://com.example:app:1.0.0"},
					},
				},
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleArtifactScan(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"scan_id": "scan-12345",
		"status":  "running",
		"created": time.Now().Format(time.RFC3339),
		"progress": map[string]interface{}{
			"scanned_artifacts": 5,
			"total_artifacts":   10,
			"percentage":        50,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleBuildScan(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"scan_id":   "build-scan-67890",
		"status":    "completed",
		"created":   time.Now().Add(-30 * time.Minute).Format(time.RFC3339),
		"completed": time.Now().Format(time.RFC3339),
		"summary": map[string]interface{}{
			"total_violations":    3,
			"security_violations": 2,
			"license_violations":  1,
			"operational_risks":   0,
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleScanStatus(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":    "completed",
		"created":   time.Now().Add(-10 * time.Minute).Format(time.RFC3339),
		"completed": time.Now().Format(time.RFC3339),
		"findings": map[string]interface{}{
			"security": map[string]interface{}{
				"critical": 0,
				"high":     2,
				"medium":   5,
				"low":      10,
			},
			"license": map[string]interface{}{
				"banned":      0,
				"permissive":  15,
				"restrictive": 1,
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleComponents(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"component_details": []map[string]interface{}{
			{
				"component_id": "gav://org.apache.logging.log4j:log4j-core:2.14.1",
				"package_type": "Maven",
				"vulnerabilities": []map[string]interface{}{
					{
						"cve":           "CVE-2021-44228",
						"severity":      "Critical",
						"cvss_v3_score": "10.0",
						"summary":       "Log4Shell vulnerability",
					},
				},
				"licenses": []string{"Apache-2.0"},
			},
		},
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleReports(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		// Create report
		response := map[string]interface{}{
			"report_id": "report-abc123",
			"status":    "pending",
			"created":   time.Now().Format(time.RFC3339),
			"type":      "vulnerability",
		}
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	} else {
		// List reports
		response := map[string]interface{}{
			"reports": []map[string]interface{}{
				{
					"report_id": "report-abc123",
					"name":      "Vulnerability Report",
					"type":      "vulnerability",
					"status":    "completed",
					"created":   time.Now().Add(-24 * time.Hour).Format(time.RFC3339),
					"completed": time.Now().Add(-23 * time.Hour).Format(time.RFC3339),
				},
			},
			"total_count": 1,
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("Failed to encode response: %v", err)
		}
	}
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"violations": map[string]interface{}{
			"security": map[string]interface{}{
				"critical": 0,
				"high":     5,
				"medium":   12,
				"low":      30,
			},
			"license": map[string]interface{}{
				"banned":  0,
				"unknown": 3,
			},
		},
		"scans": map[string]interface{}{
			"total":    150,
			"last_24h": 25,
			"last_7d":  120,
		},
		"components": map[string]interface{}{
			"total":      500,
			"vulnerable": 45,
			"outdated":   120,
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleDependencyGraph(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"artifact": "gav://com.example:app:1.0.0",
		"dependencies": []map[string]interface{}{
			{
				"component_id": "gav://org.springframework:spring-core:5.3.0",
				"direct":       true,
				"dependencies": []map[string]interface{}{
					{
						"component_id": "gav://org.springframework:spring-jcl:5.3.0",
						"direct":       false,
					},
				},
			},
			{
				"component_id": "gav://org.apache.logging.log4j:log4j-core:2.17.1",
				"direct":       true,
				"dependencies": []map[string]interface{}{},
			},
		},
		"total_dependencies":      15,
		"direct_dependencies":     5,
		"transitive_dependencies": 10,
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}

func handleDefaultXray(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"success":   true,
		"message":   fmt.Sprintf("Mock Xray response for %s", r.URL.Path),
		"timestamp": time.Now().Format(time.RFC3339),
	}
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to encode response: %v", err)
	}
}
