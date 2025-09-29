package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// SetupHandlers configures the HTTP handlers for the mock server
func SetupHandlers(mux *http.ServeMux) {
	// If no mux is provided, use the default one for backward compatibility
	if mux == nil {
		mux = http.DefaultServeMux
	}

	// GitHub API mock
	mux.HandleFunc("/mock-github/", GitHubHandler)

	// Harness API mock
	mux.HandleFunc("/mock-harness/", HarnessHandler)

	// SonarQube API mock
	mux.HandleFunc("/mock-sonarqube/", SonarQubeHandler)

	// Artifactory API mock - use comprehensive handler from jfrog.go
	mux.HandleFunc("/mock-artifactory/", JFrogArtifactoryHandler)

	// Xray API mock - use comprehensive handler from jfrog.go
	mux.HandleFunc("/mock-xray/", JFrogXrayHandler)

	// Mock webhook endpoints
	mux.HandleFunc("/api/v1/webhook/github", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Mock GitHub webhook received: %s", r.Method)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(`{"status":"ok"}`)); err != nil {
			log.Printf("Failed to write webhook response: %v", err)
		}
	})
}

// HarnessHandler handles mock Harness API requests
func HarnessHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock Harness request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Special handling for health endpoint
	if r.URL.Path == "/mock-harness/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle pipelines endpoint
	if r.URL.Path == "/mock-harness/pipelines" {
		response := map[string]interface{}{
			"pipelines": []map[string]interface{}{
				{
					"id":   "pipeline1",
					"name": "Deploy to Production",
					"type": "deployment",
				},
				{
					"id":   "pipeline2",
					"name": "Run Integration Tests",
					"type": "build",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success": true,
		"message": "Mock Harness response",
		"result": map[string]interface{}{
			"id":        "mock-execution-id",
			"status":    "SUCCESS",
			"timestamp": time.Now().Format(time.RFC3339),
		},
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}

// SonarQubeHandler handles mock SonarQube API requests
func SonarQubeHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock SonarQube request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Special handling for health endpoint
	if r.URL.Path == "/mock-sonarqube/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle quality gate endpoint
	if strings.Contains(r.URL.Path, "/qualitygates/project_status") {
		response := map[string]interface{}{
			"projectStatus": map[string]interface{}{
				"status": "OK",
				"conditions": []map[string]interface{}{
					{
						"status":         "OK",
						"metricKey":      "bugs",
						"comparator":     "LT",
						"errorThreshold": "10",
						"actualValue":    "0",
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle issues search endpoint
	if strings.Contains(r.URL.Path, "/issues/search") {
		response := map[string]interface{}{
			"total": 5,
			"issues": []map[string]interface{}{
				{
					"key":       "issue1",
					"component": "project:file.java",
					"severity":  "MAJOR",
					"message":   "Fix this code smell",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success":   true,
		"message":   "Mock SonarQube response",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}

// ArtifactoryHandler handles mock Artifactory API requests
func ArtifactoryHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock Artifactory request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Special handling for health endpoint
	if r.URL.Path == "/mock-artifactory/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle storage endpoint
	if strings.Contains(r.URL.Path, "/storage/") {
		response := map[string]interface{}{
			"repo":    "libs-release-local",
			"path":    "/com/example/app/1.0.0/app-1.0.0.jar",
			"created": time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
			"size":    "10485760",
			"checksums": map[string]interface{}{
				"md5":  "abcd1234abcd1234abcd1234abcd1234",
				"sha1": "abcd1234abcd1234abcd1234abcd1234abcd1234",
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle search endpoint
	if strings.Contains(r.URL.Path, "/search/") {
		response := map[string]interface{}{
			"results": []map[string]interface{}{
				{
					"uri":     "libs-release-local/com/example/app/1.0.0/app-1.0.0.jar",
					"size":    "10485760",
					"created": time.Now().AddDate(0, -1, 0).Format(time.RFC3339),
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success":   true,
		"message":   "Mock Artifactory response",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}

// XrayHandler handles mock Xray API requests
func XrayHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("Mock Xray request: %s %s", r.Method, r.URL.Path)
	w.Header().Set("Content-Type", "application/json")

	// Special handling for health endpoint
	if r.URL.Path == "/mock-xray/health" {
		response := map[string]interface{}{
			"status":    "ok",
			"timestamp": time.Now().Format(time.RFC3339),
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle vulnerabilities endpoint
	if strings.Contains(r.URL.Path, "/vulnerabilities") {
		response := map[string]interface{}{
			"total": 3,
			"vulnerabilities": []map[string]interface{}{
				{
					"id":          "CVE-2023-1234",
					"severity":    "HIGH",
					"summary":     "Security vulnerability in component",
					"description": "This is a serious vulnerability that should be fixed",
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Handle licenses endpoint
	if strings.Contains(r.URL.Path, "/licenses") {
		response := map[string]interface{}{
			"licenses": []map[string]interface{}{
				{
					"name": "Apache-2.0",
					"components": []map[string]interface{}{
						{
							"name":    "org.apache.commons:commons-lang3",
							"version": "3.12.0",
						},
					},
				},
			},
		}
		_ = json.NewEncoder(w).Encode(response)
		return
	}

	// Default response for other endpoints
	response := map[string]interface{}{
		"success":   true,
		"message":   "Mock Xray response",
		"timestamp": time.Now().Format(time.RFC3339),
	}
	_ = json.NewEncoder(w).Encode(response)
}
