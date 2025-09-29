# Artifactory & Xray Workflow Examples

This document provides comprehensive examples of common DevOps workflows using the Artifactory and Xray providers in Developer Mesh. These examples demonstrate real-world scenarios that AI agents can execute autonomously.

## Table of Contents

- [Authentication Setup](#authentication-setup)
- [Basic Workflows](#basic-workflows)
- [CI/CD Integration Workflows](#cicd-integration-workflows)
- [Security and Compliance Workflows](#security-and-compliance-workflows)
- [Advanced Workflows](#advanced-workflows)
- [Troubleshooting](#troubleshooting)

## Authentication Setup

Both Artifactory and Xray providers support unified JFrog Platform authentication. The providers automatically detect the appropriate authentication method based on your credentials.

### Authentication Methods

1. **API Key Authentication** (Legacy)
   ```
   X-JFrog-Art-Api: your-api-key-here
   ```

2. **Access Token Authentication** (Recommended)
   ```
   Authorization: Bearer your-access-token-here
   ```

3. **Reference Token Authentication**
   ```
   Authorization: Bearer your-reference-token-here
   ```

### Configuration

Configure authentication via context or provider configuration:

```json
{
  "providers": {
    "artifactory": {
      "baseURL": "https://mycompany.jfrog.io/artifactory",
      "authType": "api_key",
      "credentials": {
        "apiKey": "your-api-key-or-token"
      }
    },
    "xray": {
      "baseURL": "https://mycompany.jfrog.io/xray",
      "authType": "api_key",
      "credentials": {
        "apiKey": "your-api-key-or-token"
      }
    }
  }
}
```

**Note:** The same credentials work for both Artifactory and Xray when using JFrog Platform.

## Basic Workflows

### 1. Repository Setup and Configuration

**Scenario:** Set up a complete repository structure for a new project

```json
{
  "workflow": "repository_setup",
  "steps": [
    {
      "name": "Create local Maven repository",
      "provider": "artifactory",
      "action": "repos/create",
      "parameters": {
        "repoKey": "myproject-maven-local",
        "rclass": "local",
        "packageType": "maven",
        "description": "Local Maven repository for myproject",
        "handleReleases": true,
        "handleSnapshots": false
      }
    },
    {
      "name": "Create Maven Central remote repository",
      "provider": "artifactory",
      "action": "repos/create",
      "parameters": {
        "repoKey": "maven-central-remote",
        "rclass": "remote",
        "packageType": "maven",
        "url": "https://repo.maven.apache.org/maven2",
        "description": "Proxy for Maven Central"
      }
    },
    {
      "name": "Create virtual Maven repository",
      "provider": "artifactory",
      "action": "repos/create",
      "parameters": {
        "repoKey": "myproject-maven-virtual",
        "rclass": "virtual",
        "packageType": "maven",
        "repositories": ["myproject-maven-local", "maven-central-remote"],
        "defaultDeploymentRepo": "myproject-maven-local"
      }
    },
    {
      "name": "Create security watch for new repositories",
      "provider": "xray",
      "action": "watches/create",
      "parameters": {
        "name": "myproject-watch",
        "description": "Monitor myproject repositories for security issues",
        "repositories": ["myproject-maven-local"],
        "policies": ["default-security-policy"],
        "watch_recipients": ["security@mycompany.com"]
      }
    }
  ]
}
```

### 2. Artifact Management and Properties

**Scenario:** Upload artifacts with metadata and scan for vulnerabilities

```json
{
  "workflow": "artifact_upload_scan",
  "steps": [
    {
      "name": "Upload JAR with build properties",
      "provider": "artifactory",
      "action": "artifacts/upload",
      "parameters": {
        "repoKey": "myproject-maven-local",
        "itemPath": "com/mycompany/myapp/1.0.0/myapp-1.0.0.jar",
        "properties": {
          "build.name": "myapp",
          "build.number": "123",
          "vcs.revision": "abc123def",
          "quality.gate": "pending"
        }
      }
    },
    {
      "name": "Scan uploaded artifact",
      "provider": "xray",
      "action": "scan/artifact",
      "parameters": {
        "componentId": "maven://myproject-maven-local/com.mycompany:myapp:1.0.0",
        "watch": "myproject-watch"
      }
    },
    {
      "name": "Get scan results",
      "provider": "xray",
      "action": "summary/artifact",
      "parameters": {
        "paths": ["default/myproject-maven-local/com/mycompany/myapp/1.0.0/myapp-1.0.0.jar"],
        "include_licenses": true
      }
    },
    {
      "name": "Update quality gate based on scan",
      "provider": "artifactory",
      "action": "artifacts/properties/set",
      "parameters": {
        "repoKey": "myproject-maven-local",
        "itemPath": "com/mycompany/myapp/1.0.0/myapp-1.0.0.jar",
        "properties": {
          "quality.gate": "passed",
          "scan.date": "2025-01-28T10:30:00Z"
        }
      }
    }
  ]
}
```

## CI/CD Integration Workflows

### 3. Build Promotion Pipeline

**Scenario:** Promote artifacts through development → staging → production environments

```json
{
  "workflow": "build_promotion_pipeline",
  "steps": [
    {
      "name": "Upload build information",
      "provider": "artifactory",
      "action": "builds/upload",
      "parameters": {
        "buildInfo": {
          "name": "myapp",
          "number": "123",
          "started": "2025-01-28T10:00:00.000Z",
          "modules": [{
            "id": "com.mycompany:myapp:1.0.0",
            "artifacts": [{
              "type": "jar",
              "sha1": "abc123def456",
              "name": "myapp-1.0.0.jar"
            }]
          }]
        }
      }
    },
    {
      "name": "Scan entire build",
      "provider": "xray",
      "action": "scan/build",
      "parameters": {
        "buildName": "myapp",
        "buildNumber": "123"
      }
    },
    {
      "name": "Check for critical violations",
      "provider": "xray",
      "action": "violations/list",
      "parameters": {
        "type": "security",
        "severity": "critical",
        "watch_name": "myproject-watch"
      }
    },
    {
      "name": "Promote to staging (if no critical issues)",
      "provider": "artifactory",
      "action": "builds/promote",
      "parameters": {
        "buildName": "myapp",
        "buildNumber": "123",
        "targetRepo": "myproject-staging-local",
        "status": "Staged",
        "comment": "Promoted to staging after security scan"
      }
    },
    {
      "name": "Final promotion to production",
      "provider": "artifactory",
      "action": "builds/promote",
      "parameters": {
        "buildName": "myapp",
        "buildNumber": "123",
        "targetRepo": "myproject-prod-local",
        "status": "Released",
        "comment": "Released to production"
      }
    }
  ]
}
```

### 4. Docker Image Security Pipeline

**Scenario:** Complete Docker image security workflow from build to deployment

```json
{
  "workflow": "docker_security_pipeline",
  "steps": [
    {
      "name": "Create Docker repositories if not exist",
      "provider": "artifactory",
      "action": "repos/create",
      "parameters": {
        "repoKey": "docker-dev-local",
        "rclass": "local",
        "packageType": "docker",
        "description": "Docker images for development"
      }
    },
    {
      "name": "Scan Docker image",
      "provider": "xray",
      "action": "scan/artifact",
      "parameters": {
        "componentId": "docker://docker-dev-local/myapp:latest",
        "include_licenses": true
      }
    },
    {
      "name": "Get detailed scan results",
      "provider": "xray",
      "action": "summary/artifact",
      "parameters": {
        "paths": ["default/docker-dev-local/myapp/latest"],
        "include_licenses": true
      }
    },
    {
      "name": "Check for vulnerabilities in base images",
      "provider": "xray",
      "action": "components/searchByCves",
      "parameters": {
        "cves": ["CVE-2021-44228", "CVE-2021-45046"]
      }
    },
    {
      "name": "Generate compliance report",
      "provider": "xray",
      "action": "reports/vulnerability",
      "parameters": {
        "name": "Docker-Security-Report",
        "type": "repository",
        "repositories": ["docker-dev-local"],
        "format": "json"
      }
    }
  ]
}
```

## Security and Compliance Workflows

### 5. Vulnerability Management

**Scenario:** Comprehensive vulnerability assessment and remediation workflow

```json
{
  "workflow": "vulnerability_management",
  "steps": [
    {
      "name": "Search for Log4j vulnerabilities",
      "provider": "xray",
      "action": "components/searchByCves",
      "parameters": {
        "cves": ["CVE-2021-44228", "CVE-2021-45046", "CVE-2021-45105", "CVE-2021-44832"]
      }
    },
    {
      "name": "Get detailed component analysis",
      "provider": "xray",
      "action": "components/findByName",
      "parameters": {
        "component_name": "log4j",
        "include_fixed_versions": true
      }
    },
    {
      "name": "Find all affected artifacts",
      "provider": "artifactory",
      "action": "search/aql",
      "parameters": {
        "query": "items.find({\"@build.name\": {\"$match\": \"*\"}, \"name\": {\"$match\": \"*log4j*\"}})"
      }
    },
    {
      "name": "Create ignore rule for false positives",
      "provider": "xray",
      "action": "ignore-rules/create",
      "parameters": {
        "vulnerability": "CVE-2021-44228",
        "component": "org.apache.logging.log4j:log4j-core:2.17.0",
        "reason": "Fixed version deployed",
        "expiry_date": "2025-12-31T23:59:59Z"
      }
    },
    {
      "name": "Generate vulnerability report",
      "provider": "xray",
      "action": "reports/vulnerability",
      "parameters": {
        "name": "Log4j-Vulnerability-Assessment",
        "type": "global",
        "filters": {
          "cve": ["CVE-2021-44228", "CVE-2021-45046"],
          "severity": ["critical", "high"]
        },
        "format": "pdf"
      }
    }
  ]
}
```

### 6. License Compliance Workflow

**Scenario:** Comprehensive license compliance monitoring and reporting

```json
{
  "workflow": "license_compliance",
  "steps": [
    {
      "name": "Create license compliance policy",
      "provider": "xray",
      "action": "policies/create",
      "parameters": {
        "name": "license-compliance-policy",
        "type": "license",
        "rules": [{
          "criteria": "banned_licenses",
          "value": ["GPL-3.0", "AGPL-3.0"],
          "action": "block"
        }]
      }
    },
    {
      "name": "Set up license monitoring watch",
      "provider": "xray",
      "action": "watches/create",
      "parameters": {
        "name": "license-compliance-watch",
        "description": "Monitor for license violations",
        "repositories": ["*-local"],
        "policies": ["license-compliance-policy"],
        "watch_recipients": ["legal@mycompany.com"]
      }
    },
    {
      "name": "Generate license report",
      "provider": "xray",
      "action": "reports/license",
      "parameters": {
        "name": "License-Compliance-Report",
        "type": "repository",
        "repositories": ["myproject-maven-local", "docker-dev-local"],
        "format": "csv"
      }
    },
    {
      "name": "Export SBOM for compliance",
      "provider": "xray",
      "action": "reports/sbom",
      "parameters": {
        "name": "Production-SBOM-2025",
        "type": "repository",
        "repositories": ["myproject-prod-local"],
        "format": "spdx"
      }
    }
  ]
}
```

## Advanced Workflows

### 7. Dependency Graph Analysis

**Scenario:** Deep dependency analysis for impact assessment

```json
{
  "workflow": "dependency_analysis",
  "steps": [
    {
      "name": "Get artifact dependency graph",
      "provider": "xray",
      "action": "graph/artifact",
      "parameters": {
        "componentId": "maven://myproject-maven-local/com.mycompany:myapp:1.0.0"
      }
    },
    {
      "name": "Compare dependency graphs",
      "provider": "xray",
      "action": "graph/compareArtifacts",
      "parameters": {
        "source_component": "maven://myproject-maven-local/com.mycompany:myapp:1.0.0",
        "target_component": "maven://myproject-maven-local/com.mycompany:myapp:1.1.0"
      }
    },
    {
      "name": "Analyze component impact",
      "provider": "xray",
      "action": "components/impact",
      "parameters": {
        "component_id": "maven:org.springframework:spring-core:5.3.0"
      }
    },
    {
      "name": "Search for component versions",
      "provider": "xray",
      "action": "components/versions",
      "parameters": {
        "component_name": "springframework",
        "package_type": "maven"
      }
    }
  ]
}
```

### 8. Automated Cleanup and Retention

**Scenario:** Automated cleanup based on security scan results and retention policies

```json
{
  "workflow": "automated_cleanup",
  "steps": [
    {
      "name": "Find old artifacts with critical vulnerabilities",
      "provider": "artifactory",
      "action": "search/aql",
      "parameters": {
        "query": "items.find({\"created\": {\"$lt\": \"2024-01-01\"}, \"@quality.gate\": \"failed\"})"
      }
    },
    {
      "name": "Get security summary for old artifacts",
      "provider": "xray",
      "action": "summary/artifact",
      "parameters": {
        "paths": ["${previous_step.results}"],
        "include_licenses": false
      }
    },
    {
      "name": "Delete artifacts with critical issues",
      "provider": "artifactory",
      "action": "artifacts/delete",
      "parameters": {
        "repoKey": "myproject-maven-local",
        "itemPath": "${artifacts_with_critical_issues}",
        "dry": false
      }
    },
    {
      "name": "Generate cleanup report",
      "provider": "xray",
      "action": "reports/operational_risk",
      "parameters": {
        "name": "Cleanup-Report",
        "type": "repository",
        "repositories": ["myproject-maven-local"],
        "format": "json"
      }
    }
  ]
}
```

## Troubleshooting

### Common Error Scenarios and Resolutions

#### Authentication Issues

**Error:** `401 Unauthorized`

**Resolution Workflow:**
```json
{
  "troubleshooting": "auth_issues",
  "steps": [
    {
      "name": "Verify current user identity",
      "provider": "artifactory",
      "action": "internal/current-user"
    },
    {
      "name": "Check available features",
      "provider": "artifactory",
      "action": "internal/available-features"
    },
    {
      "name": "Verify Xray accessibility",
      "provider": "xray",
      "action": "system/ping"
    }
  ]
}
```

#### Permission Issues

**Error:** `403 Forbidden`

**Resolution Workflow:**
```json
{
  "troubleshooting": "permission_issues",
  "steps": [
    {
      "name": "List available repositories",
      "provider": "artifactory",
      "action": "repos/list"
    },
    {
      "name": "Check user permissions",
      "provider": "artifactory",
      "action": "security/permissions/list"
    },
    {
      "name": "List accessible watches",
      "provider": "xray",
      "action": "watches/list"
    }
  ]
}
```

#### Scan Issues

**Error:** `scan_in_progress` or `no_xray_data`

**Resolution Workflow:**
```json
{
  "troubleshooting": "scan_issues",
  "steps": [
    {
      "name": "Check scan status",
      "provider": "xray",
      "action": "scan/status",
      "parameters": {
        "scan_id": "${scan_id}"
      }
    },
    {
      "name": "Verify artifact exists",
      "provider": "artifactory",
      "action": "artifacts/info",
      "parameters": {
        "repoKey": "${repo}",
        "itemPath": "${path}"
      }
    },
    {
      "name": "Check Xray system health",
      "provider": "xray",
      "action": "system/ping"
    }
  ]
}
```

### Best Practices

1. **Always verify authentication first** using `internal/current-user` operations
2. **Check service health** before starting complex workflows
3. **Use dry-run mode** for destructive operations when available
4. **Implement proper error handling** with retry logic for scan operations
5. **Cache scan results** to avoid unnecessary re-scanning
6. **Use watches and policies** for continuous monitoring rather than manual scans
7. **Filter operations** based on discovered permissions to avoid 403 errors
8. **Schedule reports** rather than generating them on-demand for better performance

### Rate Limiting Considerations

- **Scanning operations:** Limited to 30 requests/minute
- **Report generation:** Limited to 10 requests/minute
- **General API operations:** 60 requests/minute for Artifactory, 60 requests/minute for Xray
- **Bulk operations:** Use batch endpoints when available

### Integration Notes

- Both providers share the same authentication when using JFrog Platform
- Component IDs must match between Artifactory storage paths and Xray scan paths
- Build information must be uploaded to Artifactory before Xray can scan builds
- Watch policies must exist before creating watches that reference them
- Reports are generated asynchronously - use status checks for completion