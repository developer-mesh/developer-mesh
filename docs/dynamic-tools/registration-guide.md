# Dynamic Tool Registration Guide

## Table of Contents
1. [Overview](#overview)
2. [Architecture and Flow](#architecture-and-flow)
3. [Registration Examples](#registration-examples)
   - [GitHub Registration](#github-registration)
   - [GitLab Registration](#gitlab-registration)
   - [Snyk Registration](#snyk-registration)
4. [Authentication Configuration](#authentication-configuration)
5. [Default Parameters](#default-parameters)
6. [Operation Grouping](#operation-grouping)
7. [Troubleshooting](#troubleshooting)
8. [API Reference](#api-reference)

## Overview

The Dynamic Tools API enables integration of external DevOps tools without code changes. This guide provides comprehensive examples and best practices for registering popular tools like GitHub, GitLab, and Snyk with the Developer Mesh platform.

## Architecture and Flow

### Registration Flow

The complete flow from tool registration to API execution:

```mermaid
graph TD
    A[API Request] --> B[Dynamic Tools API Handler]
    B --> C[Dynamic Tools Service]
    C --> D[Tool Configuration Storage]
    D --> E[PostgreSQL Database]
    
    F[Tool Execution Request] --> G[Dynamic Tool Adapter]
    G --> H[Load Configuration]
    H --> I[Apply Default Parameters]
    I --> J[Apply Authentication]
    J --> K[Execute API Call]
    
    style A fill:#e1f5fe
    style K fill:#c8e6c9
```

### Key Components

| Component | File Path | Purpose |
|-----------|-----------|---------|
| **API Handler** | `apps/rest-api/internal/api/dynamic_tools_api.go` | HTTP endpoint handling |
| **Service Layer** | `apps/rest-api/internal/services/dynamic_tools_service.go` | Business logic and validation |
| **Repository** | `pkg/repository/dynamic_tool_repository.go` | Database operations |
| **Tool Adapter** | `pkg/tools/adapters/dynamic_tool_adapter.go` | API execution and parameter handling |
| **Authentication** | `pkg/tools/dynamic_auth.go` | Authentication strategy implementation |
| **Tool Config** | `pkg/tools/plugin.go` | Configuration structures |

### Database Schema

Tools are stored in the `tool_configurations` table with the following key fields:
- `id`: Unique identifier
- `name`: Tool name (must be unique per tenant)
- `base_url`: API base URL
- `openapi_spec`: Cached OpenAPI specification
- `auth_type`: Authentication method (token, bearer, api_key, basic, custom_header)
- `auth_config`: JSON configuration for custom authentication
- `credentials`: Encrypted credentials
- `config`: JSON configuration including default_parameters
- `group_operations`: Boolean flag for operation grouping
- `tenant_id`: Multi-tenant isolation

## Registration Examples

### GitHub Registration

GitHub supports multiple authentication methods. Here are the recommended approaches for 2025:

#### Option 1: Fine-Grained Personal Access Token (Recommended)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "base_url": "https://api.github.com",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "github_pat_11AAAAAA_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "hybrid",
      "default_parameters": {
        "per_page": 100
      }
    },
    "passthrough_config": {
      "mode": "optional",
      "fallback_to_service": true
    }
  }'
```

#### Option 2: Classic Personal Access Token

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-classic",
    "base_url": "https://api.github.com",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "hybrid"
    }
  }'
```

#### Option 3: GitHub App Installation

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github-app",
    "base_url": "https://api.github.com",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "auth_type": "bearer",
    "auth_config": {
      "app_id": "123456",
      "installation_id": "987654321",
      "private_key_path": "/path/to/private-key.pem"
    },
    "credential": {
      "type": "jwt",
      "token": "GENERATED_JWT_TOKEN"
    },
    "group_operations": true
  }'
```

### GitLab Registration

GitLab supports several authentication methods. Here are the current options for 2025:

#### Option 1: Personal Access Token with PRIVATE-TOKEN Header

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gitlab",
    "base_url": "https://gitlab.com/api/v4",
    "openapi_url": "https://gitlab.com/api/v4/openapi.json",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "PRIVATE-TOKEN",
      "header_prefix": ""
    },
    "credential": {
      "type": "token",
      "token": "glpat-xxxxxxxxxxxxxxxxxxxx"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag",
      "default_parameters": {
        "per_page": 100
      }
    }
  }'
```

#### Option 2: OAuth-Compliant Bearer Token

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gitlab-bearer",
    "base_url": "https://gitlab.com/api/v4",
    "openapi_url": "https://gitlab.com/api/v4/openapi.json",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "glpat-xxxxxxxxxxxxxxxxxxxx"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag"
    }
  }'
```

#### Option 3: Project or Group Access Token

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "gitlab-project",
    "base_url": "https://gitlab.com/api/v4",
    "openapi_url": "https://gitlab.com/api/v4/openapi.json",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "glpat-project-xxxxxxxxxxxx"
    },
    "config": {
      "default_parameters": {
        "project_id": "12345678"
      }
    }
  }'
```

### Harness.io Registration

Harness.io's Software Delivery Platform uses OpenAPI v3.0 and requires specific authentication. As of 2025, Harness has moved from bearer tokens to x-api-key header authentication:
- Uses `x-api-key` header (not Bearer token)
- API keys can be for personal accounts or service accounts
- Each API key can have multiple tokens
- Required parameter: `accountIdentifier` for most operations
- Multiple API endpoints for different modules (Platform, Chaos, CI/CD)

#### Option 1: Personal Account API Key (For Development)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "harness",
    "base_url": "https://app.harness.io",
    "openapi_url": "https://apidocs.harness.io/openapi.json",
    "auth_type": "api_key",
    "auth_config": {
      "header_name": "x-api-key"
    },
    "credential": {
      "type": "token",
      "token": "pat.YOUR_ACCOUNT_ID.YOUR_TOKEN_HERE"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag",
      "default_parameters": {
        "accountIdentifier": "YOUR_ACCOUNT_ID",
        "orgIdentifier": "default"
      }
    }
  }'
```

#### Option 2: Service Account API Key (For Automation)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "harness-automation",
    "base_url": "https://app.harness.io",
    "openapi_url": "https://apidocs.harness.io/openapi.json",
    "auth_type": "api_key",
    "auth_config": {
      "header_name": "x-api-key"
    },
    "credential": {
      "type": "token",
      "token": "sat.YOUR_ACCOUNT_ID.YOUR_SERVICE_ACCOUNT_TOKEN"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "hybrid",
      "default_parameters": {
        "accountIdentifier": "YOUR_ACCOUNT_ID",
        "orgIdentifier": "YOUR_ORG_ID",
        "projectIdentifier": "YOUR_PROJECT_ID"
      },
      "service_account": true
    }
  }'
```

#### Option 3: Module-Specific Registration

For specific Harness modules, you can register them separately:

```bash
# For CI/CD Module
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "harness-cicd",
    "base_url": "https://app.harness.io/ng",
    "openapi_url": "https://apidocs.harness.io/ng/api/openapi.json",
    "auth_type": "api_key",
    "auth_config": {
      "header_name": "x-api-key"
    },
    "credential": {
      "type": "token",
      "token": "pat.YOUR_ACCOUNT_ID.YOUR_TOKEN"
    },
    "config": {
      "default_parameters": {
        "accountIdentifier": "YOUR_ACCOUNT_ID"
      },
      "module": "ci-cd"
    }
  }'

# For Cloud Cost Management Module
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "harness-ccm",
    "base_url": "https://app.harness.io/ccm",
    "openapi_url": "https://apidocs.harness.io/ccm/api/openapi.json",
    "auth_type": "api_key",
    "auth_config": {
      "header_name": "x-api-key"
    },
    "credential": {
      "type": "token",
      "token": "pat.YOUR_ACCOUNT_ID.YOUR_TOKEN"
    },
    "config": {
      "default_parameters": {
        "accountIdentifier": "YOUR_ACCOUNT_ID"
      },
      "module": "cloud-cost"
    }
  }'
```

#### Finding Your Harness Account Identifier

1. Log into Harness Platform at https://app.harness.io
2. Click on your profile icon in the top-right corner
3. Select "Account Settings"
4. Copy the Account ID (format: alphanumeric string like `kmpySmUISimoRrJL6NL73w`)
5. For organization-specific operations, also note your Organization ID

#### Creating API Keys and Tokens

1. **Personal Account Token**:
   - Go to Account Settings → Access Control → API Keys
   - Click "+ New API Key"
   - Add one or more tokens to the API key
   - Token format: `pat.<account_id>.<token_value>`

2. **Service Account Token**:
   - Go to Account Settings → Access Control → Service Accounts
   - Create or select a service account
   - Create API key at the desired scope (account/org/project)
   - Token format: `sat.<account_id>.<token_value>`

#### Testing Your Harness Connection

```bash
# Test API connection
curl -X GET "https://app.harness.io/ng/api/user/currentUser" \
  -H "x-api-key: YOUR_API_TOKEN"

# List projects
curl -X GET "https://app.harness.io/ng/api/projects?accountIdentifier=YOUR_ACCOUNT_ID&orgIdentifier=default" \
  -H "x-api-key: YOUR_API_TOKEN"
```

### Snyk Registration

Snyk's REST API is based on JSON:API standard and requires specific authentication and parameters. As of 2025, Snyk requires:
- Enterprise plan for API access (Free/Team plans only get CLI/IDE access)
- Custom authorization header format: `Authorization: token <TOKEN>`
- Required parameters: `org_id` and API `version`
- Rate limit: 1620 requests per minute per API key

#### Option 1: Personal API Token (For Development)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "snyk",
    "base_url": "https://api.snyk.io",
    "openapi_url": "https://api.snyk.io/rest/openapi",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "Authorization",
      "header_prefix": "token"
    },
    "credential": {
      "type": "token",
      "token": "YOUR_SNYK_API_TOKEN"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag",
      "default_parameters": {
        "org_id": "YOUR_SNYK_ORG_ID",
        "version": "2024-10-15"
      },
      "rate_limit": {
        "requests_per_minute": 1620
      }
    }
  }'
```

#### Option 2: Service Account Token (For Automation)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "snyk-automation",
    "base_url": "https://api.snyk.io",
    "openapi_url": "https://api.snyk.io/rest/openapi",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "Authorization",
      "header_prefix": "token"
    },
    "credential": {
      "type": "token",
      "token": "YOUR_SNYK_SERVICE_ACCOUNT_TOKEN"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag",
      "default_parameters": {
        "org_id": "YOUR_SNYK_ORG_ID",
        "version": "2024-10-15"
      },
      "service_account": true
    }
  }'
```

#### Finding Your Snyk Organization ID

1. Log into Snyk web console
2. Navigate to Settings → General
3. Copy the Organization ID (format: `12345678-1234-1234-1234-123456789012`)
4. For Group-level access, use the Group ID instead

### Kubernetes Registration

The Kubernetes API uses bearer token authentication with service accounts. As of 2025:
- Service accounts use signed bearer tokens for authentication
- Tokens can be long-lived (from file) or short-lived (TokenRequest API)
- Supports multiple authentication methods: Bearer tokens, client certificates, OIDC
- API server expects `Authorization: Bearer <token>` header
- OpenAPI specification available at `/openapi/v2` or `/openapi/v3`

#### Option 1: Service Account Bearer Token

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kubernetes",
    "base_url": "https://your-cluster.k8s.local:6443",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_SERVICE_ACCOUNT_TOKEN"
    },
    "config": {
      "discovery_paths": [
        "/openapi/v2",
        "/openapi/v3",
        "/swagger.json"
      ],
      "skip_tls_verify": false,
      "default_parameters": {
        "namespace": "default"
      }
    }
  }'
```

#### Option 2: Using Kubeconfig Context

For clusters accessed via kubectl, extract the token from kubeconfig:

```bash
# Get the token from current context
TOKEN=$(kubectl config view --raw -o jsonpath='{.users[?(@.name=="YOUR_USER")].user.token}')

# Or for service account token
TOKEN=$(kubectl get secret YOUR_SA_SECRET -n kube-system -o jsonpath='{.data.token}' | base64 -d)

# Register with Developer Mesh
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kubernetes-prod",
    "base_url": "https://api.your-cluster.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "'$TOKEN'"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "tag",
      "default_parameters": {
        "namespace": "default",
        "limit": 100
      }
    }
  }'
```

#### Option 3: Client Certificate Authentication

For clusters using client certificates:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "kubernetes-cert",
    "base_url": "https://your-cluster.k8s.local:6443",
    "auth_type": "client_cert",
    "credential": {
      "type": "client_cert",
      "cert_path": "/path/to/client.crt",
      "key_path": "/path/to/client.key",
      "ca_path": "/path/to/ca.crt"
    },
    "config": {
      "discovery_paths": [
        "/openapi/v2"
      ]
    }
  }'
```

#### Creating a Service Account Token

```bash
# Create service account
kubectl create serviceaccount devmesh-sa -n default

# Create ClusterRoleBinding (adjust permissions as needed)
kubectl create clusterrolebinding devmesh-binding \
  --clusterrole=view \
  --serviceaccount=default:devmesh-sa

# Get the token (Kubernetes 1.24+)
kubectl create token devmesh-sa -n default --duration=87600h

# Or for older versions, get from secret
kubectl get secret $(kubectl get sa devmesh-sa -n default -o jsonpath='{.secrets[0].name}') \
  -n default -o jsonpath='{.data.token}' | base64 -d
```

#### Testing Kubernetes Connection

```bash
# Test API connection
curl -k https://your-cluster:6443/api/v1/namespaces \
  -H "Authorization: Bearer YOUR_TOKEN"

# Test with kubectl proxy (for local testing)
kubectl proxy --port=8001 &
curl http://localhost:8001/api/v1/namespaces
```

### Prometheus Registration

Prometheus supports multiple authentication methods. As of 2025:
- Basic authentication (username/password)
- Bearer token authentication
- TLS with client certificates
- OAuth 2.0 (cannot be used with basic_auth)
- No built-in OpenAPI spec, but well-documented REST API

#### Option 1: No Authentication (Local/Development)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prometheus",
    "base_url": "http://localhost:9090",
    "auth_type": "none",
    "config": {
      "endpoints": {
        "query": "/api/v1/query",
        "query_range": "/api/v1/query_range",
        "series": "/api/v1/series",
        "labels": "/api/v1/labels",
        "targets": "/api/v1/targets",
        "rules": "/api/v1/rules",
        "alerts": "/api/v1/alerts"
      },
      "default_parameters": {
        "timeout": "30s"
      }
    }
  }'
```

#### Option 2: Basic Authentication

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prometheus-prod",
    "base_url": "https://prometheus.example.com",
    "auth_type": "basic",
    "credential": {
      "type": "basic",
      "username": "prometheus_user",
      "password": "YOUR_PASSWORD"
    },
    "config": {
      "use_tls": true,
      "default_parameters": {
        "timeout": "30s"
      }
    }
  }'
```

#### Option 3: Bearer Token Authentication

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prometheus-token",
    "base_url": "https://prometheus.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_BEARER_TOKEN"
    },
    "config": {
      "endpoints": {
        "query": "/api/v1/query",
        "query_range": "/api/v1/query_range",
        "remote_write": "/api/v1/write"
      },
      "remote_write": {
        "enabled": true,
        "compression": "snappy"
      }
    }
  }'
```

#### Option 4: Prometheus with Grafana Cloud

For Prometheus hosted on Grafana Cloud:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prometheus-grafana",
    "base_url": "https://prometheus-prod-10-prod-us-central-0.grafana.net",
    "auth_type": "basic",
    "credential": {
      "type": "basic",
      "username": "YOUR_INSTANCE_ID",
      "password": "YOUR_API_KEY"
    },
    "config": {
      "remote_write": {
        "enabled": true,
        "url": "https://prometheus-prod-10-prod-us-central-0.grafana.net/api/prom/push"
      }
    }
  }'
```

#### Creating Custom OpenAPI Spec for Prometheus

Since Prometheus doesn't provide an OpenAPI spec, create one for better integration:

```yaml
# prometheus-openapi.yaml
openapi: 3.0.0
info:
  title: Prometheus API
  version: 2.45.0
servers:
  - url: https://prometheus.example.com
paths:
  /api/v1/query:
    get:
      operationId: query
      parameters:
        - name: query
          in: query
          required: true
          schema:
            type: string
        - name: time
          in: query
          schema:
            type: string
        - name: timeout
          in: query
          schema:
            type: string
  /api/v1/query_range:
    get:
      operationId: queryRange
      parameters:
        - name: query
          in: query
          required: true
          schema:
            type: string
        - name: start
          in: query
          required: true
          schema:
            type: string
        - name: end
          in: query
          required: true
          schema:
            type: string
        - name: step
          in: query
          required: true
          schema:
            type: string
  /api/v1/series:
    get:
      operationId: series
      parameters:
        - name: match[]
          in: query
          required: true
          schema:
            type: array
            items:
              type: string
```

Then register with the custom spec:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "prometheus-custom",
    "base_url": "https://prometheus.example.com",
    "openapi_url": "https://your-hosting/prometheus-openapi.yaml",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_TOKEN"
    }
  }'
```

#### Testing Prometheus Connection

```bash
# Test basic connectivity
curl https://prometheus.example.com/api/v1/query?query=up \
  -H "Authorization: Bearer YOUR_TOKEN"

# Test with basic auth
curl https://prometheus.example.com/api/v1/targets \
  -u username:password
```

### SonarQube Registration

SonarQube's Web API doesn't provide a standard OpenAPI specification file. The API is self-documenting through the `/api/webservices/list` endpoint. As of 2025:
- Uses Bearer authentication: `Authorization: Bearer <TOKEN>`
- Tokens require "Execute Analysis" permission for project analysis
- Web API V2 is gradually replacing V1
- Documentation available at `<SONARQUBE_URL>/web_api`

#### Option 1: User Token with Bearer Authentication

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sonarqube",
    "base_url": "https://sonarqube.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "squ_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    },
    "config": {
      "discovery_paths": [
        "/api/webservices/list",
        "/web_api/api/webservices",
        "/api/openapi.json"
      ],
      "default_parameters": {
        "format": "json"
      }
    }
  }'
```

#### Option 2: Project Analysis Token

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sonarqube-analysis",
    "base_url": "https://sonarqube.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "sqp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    },
    "config": {
      "discovery_paths": [
        "/api/webservices/list"
      ],
      "default_parameters": {
        "projectKey": "YOUR_PROJECT_KEY"
      },
      "project_token": true
    }
  }'
```

#### Option 3: Using Discovery with Fallback

If SonarQube's API discovery fails, the system will attempt format detection and conversion:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "sonarqube-discovery",
    "base_url": "https://sonarqube.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "squ_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
    },
    "config": {
      "discovery_paths": [
        "/api/webservices/list",
        "/web_api/api/webservices",
        "/api/webservices/response-example"
      ],
      "api_version": "web_api_v1",
      "default_parameters": {
        "format": "json"
      },
      "discovery_timeout": "120s"
    }
  }'
```

Note: The system will try to parse SonarQube's proprietary API format from `/api/webservices/list` and convert it to a usable format.

#### Monitoring Token Expiration

SonarQube adds a `SonarQube-Authentication-Token-Expiration` header to API responses. Monitor this header to track token expiration:

```bash
# Test API call to check token expiration
curl -I https://sonarqube.example.com/api/system/status \
  -H "Authorization: Bearer YOUR_TOKEN"
# Look for: SonarQube-Authentication-Token-Expiration: 2025-12-31T23:59:59+0000
```

## Authentication Configuration

### Understanding auth_type and auth_config

The `auth_type` field determines the authentication strategy, while `auth_config` provides additional configuration for custom authentication methods.

#### Standard Authentication Types

| auth_type | Description | Example |
|-----------|-------------|---------|
| `bearer` | Standard Bearer token (RFC 6750) | `Authorization: Bearer TOKEN` |
| `token` | Token without Bearer prefix | `Authorization: TOKEN` |
| `api_key` | API key in custom header | `X-API-Key: TOKEN` |
| `basic` | HTTP Basic Authentication | `Authorization: Basic base64(user:pass)` |
| `custom_header` | Custom header with configurable prefix | Configured via auth_config |

#### Custom Header Configuration

For tools that use non-standard authentication headers, use `auth_type: "custom_header"` with `auth_config`:

```json
{
  "auth_type": "custom_header",
  "auth_config": {
    "header_name": "PRIVATE-TOKEN",
    "header_prefix": ""
  }
}
```

Or with a prefix:

```json
{
  "auth_type": "custom_header",
  "auth_config": {
    "header_name": "Authorization",
    "header_prefix": "token"
  }
}
```

This generates: `Authorization: token YOUR_TOKEN`

### Authentication Flow

1. **Registration**: Auth configuration is stored in database
2. **Tool Loading**: Configuration is loaded from database including auth_config
3. **Execution**: Authentication is applied based on auth_type and auth_config
4. **Credential Merging**: auth_config is merged with credentials for the request

Key implementation in `pkg/tools/adapters/dynamic_tool_adapter.go`:

```go
func (a *DynamicToolAdapter) applyAuthentication(req *http.Request) error {
    // Merge auth_config with credentials
    mergedCreds := make(map[string]interface{})
    
    // First, add auth_config if it exists
    if a.tool.AuthConfig != nil {
        for k, v := range a.tool.AuthConfig {
            mergedCreds[k] = v
        }
    }
    
    // Then add credentials (they take precedence)
    for k, v := range a.creds {
        mergedCreds[k] = v
    }
    
    // Apply authentication based on type
    return ApplyDynamicAuth(req, a.tool.AuthType, mergedCreds)
}
```

## Default Parameters

Default parameters are automatically injected into every API call for a tool. This is useful for:
- Required parameters that are constant (e.g., API version, org ID)
- Default pagination settings
- Common filters or options

### Configuration

Default parameters are configured in the `config` field during registration:

```json
{
  "config": {
    "default_parameters": {
      "org_id": "12345",
      "version": "2025-08-01",
      "per_page": 100
    }
  }
}
```

### Implementation

Default parameters are applied in `pkg/tools/adapters/dynamic_tool_adapter.go`:

```go
func (a *DynamicToolAdapter) buildRequest(operation *OpenAPIOperation, params map[string]interface{}) (*http.Request, error) {
    // Merge default parameters from tool config
    mergedParams := make(map[string]interface{})
    
    // First, add default parameters if they exist
    if a.tool.Config != nil {
        if defaultParams, ok := a.tool.Config["default_parameters"].(map[string]interface{}); ok {
            for k, v := range defaultParams {
                mergedParams[k] = v
            }
        }
    }
    
    // Then overlay the provided parameters (they take precedence)
    for k, v := range params {
        mergedParams[k] = v
    }
    
    // Use mergedParams for building the request...
}
```

### Priority Order

1. User-provided parameters (highest priority)
2. Default parameters from tool configuration
3. OpenAPI specification defaults (lowest priority)

## OpenAPI Discovery

When `openapi_url` is not provided, the system attempts automatic discovery using multiple strategies:

### Discovery Strategies (in order)

1. **Direct OpenAPI URL** - If provided, used directly
2. **Common Path Discovery** - Tries standard paths:
   - `/openapi.json`, `/openapi.yaml`, `/openapi.yml`
   - `/swagger.json`, `/swagger.yaml`
   - `/api-docs`, `/api-docs.json`
   - `/v2/api-docs`, `/v3/api-docs`
   - `/api/v1/openapi.json`, `/api/v2/openapi.json`
   - `/swagger/v1/swagger.json`
   - `/.well-known/openapi.json`
   - Service-specific paths (e.g., `/api/webservices/list` for SonarQube)

3. **Subdomain Discovery** - Tries common subdomains:
   - `api.example.com`
   - `docs.example.com`
   - `developer.example.com`

4. **HTML Parsing** - Scans homepage for API documentation links

5. **Well-Known Paths** - Checks `.well-known` directory

6. **Format Detection** - If not OpenAPI, attempts conversion from:
   - Swagger 2.0
   - RAML
   - API Blueprint
   - Postman Collections

### Providing Discovery Hints

Help the discovery process by providing hints:

```json
{
  "name": "custom-api",
  "base_url": "https://api.example.com",
  "config": {
    "discovery_paths": [
      "/custom/openapi.json",
      "/v1/api-specification",
      "/docs/swagger.yaml"
    ],
    "discovery_subdomains": [
      "api-docs",
      "developers"
    ]
  }
}
```

### Tools Without OpenAPI Specifications

Some tools (like SonarQube) don't provide standard OpenAPI specs. For these:

#### Option 1: Use Discovery Paths

Let the system try to find and convert the API documentation:

```json
{
  "config": {
    "discovery_paths": [
      "/api/webservices/list",  // SonarQube
      "/api/documentation",      // Custom format
      "/help/api"               // Proprietary format
    ]
  }
}
```

#### Option 2: Create Custom OpenAPI Specification

When discovery fails, create a minimal OpenAPI specification for the tool and host it:

```yaml
# minimal-openapi.yaml
openapi: 3.0.0
info:
  title: Custom API
  version: 1.0.0
servers:
  - url: https://api.example.com
paths:
  /api/projects:
    get:
      operationId: listProjects
      parameters:
        - name: page
          in: query
          schema:
            type: integer
        - name: size
          in: query
          schema:
            type: integer
    post:
      operationId: createProject
      requestBody:
        content:
          application/json:
            schema:
              type: object
              properties:
                name:
                  type: string
                description:
                  type: string
```

Then register with the hosted spec URL:

```json
{
  "name": "custom-api",
  "base_url": "https://api.example.com",
  "openapi_url": "https://your-hosting/minimal-openapi.yaml"
}
```

#### Option 3: Custom Format Conversion

If the API uses a proprietary format, the system attempts automatic conversion. Supported formats include:
- WADL (Web Application Description Language)
- RAML (RESTful API Modeling Language)
- API Blueprint
- Postman Collections
- HAR (HTTP Archive) files

The format detector (`pkg/tools/adapters/format_detector.go`) automatically identifies and converts these formats to OpenAPI 3.0.

### Discovery Timeout Configuration

For large APIs or slow servers, configure discovery timeouts:

```json
{
  "config": {
    "discovery_timeout": "120s",  // Default is 60s
    "discovery_retries": 3,        // Number of retry attempts
    "discovery_backoff": "5s"      // Delay between retries
  }
}
```

## Operation Grouping

Large APIs like GitHub and GitLab can have hundreds of operations, making them difficult for AI agents to understand and use effectively. Operation grouping automatically splits these APIs into logical tools.

### How It Works

When `group_operations: true` is set:

1. **Analysis**: The system analyzes the OpenAPI specification
2. **Grouping**: Operations are grouped by:
   - **Tags** (preferred): Uses OpenAPI tags
   - **Paths**: Groups by URL path structure
   - **Hybrid**: Combines tags and paths for optimal grouping
3. **Tool Creation**: Each group becomes a separate tool with a descriptive name
4. **Configuration Inheritance**: All groups inherit the parent tool's configuration

### Example: GitHub with Grouping

```bash
# Register GitHub with operation grouping
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "github",
    "base_url": "https://api.github.com",
    "openapi_url": "https://raw.githubusercontent.com/github/rest-api-description/main/descriptions/api.github.com/api.github.com.json",
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "grouping_strategy": "hybrid",
      "default_parameters": {
        "per_page": 100
      }
    },
    "credential": {
      "type": "token",
      "token": "ghp_xxxxxxxxxxxx"
    }
  }'
```

This creates multiple tools:
- `github_repos` - Repository operations
- `github_issues` - Issue management
- `github_pulls` - Pull request operations
- `github_actions` - GitHub Actions workflows
- `github_users` - User management
- `github_orgs` - Organization management
- And 30+ more...

### Benefits

1. **AI Agent Clarity**: Smaller, focused tools are easier for AI to understand
2. **Better Discovery**: Agents can find relevant operations faster
3. **Reduced Context**: Less information to process per tool
4. **Targeted Permissions**: Can grant access to specific operation groups

## Troubleshooting

### Common Issues and Solutions

#### 1. "Required parameter missing" Errors

**Problem**: API calls fail with missing required parameters like `org_id` or `version`.

**Solution**: Configure default parameters:

```json
{
  "config": {
    "default_parameters": {
      "org_id": "YOUR_ORG_ID",
      "version": "API_VERSION"
    }
  }
}
```

**Verification**: Check that default parameters are being saved and loaded:

```bash
# Check database
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT name, config FROM tool_configurations WHERE name='your-tool';"
```

#### 2. Authentication Failures

**Problem**: 401 or 403 errors when calling APIs.

**Common Causes**:
- Wrong auth_type for the API
- Missing or incorrect auth_config
- Expired or invalid tokens
- Insufficient token permissions

**Solutions**:

For custom headers (GitLab, Snyk):
```json
{
  "auth_type": "custom_header",
  "auth_config": {
    "header_name": "PRIVATE-TOKEN",
    "header_prefix": ""
  }
}
```

For standard Bearer tokens (GitHub):
```json
{
  "auth_type": "bearer",
  "credential": {
    "type": "token",
    "token": "ghp_xxxxxxxxxxxx"
  }
}
```

#### 3. auth_config Not Persisting

**Problem**: auth_config shows as null after registration.

**Solution**: Ensure the service layer is properly marshaling auth_config:

```go
// In dynamic_tools_service.go
var authConfigJSON []byte
if config.AuthConfig != nil {
    authConfigJSON, err = json.Marshal(config.AuthConfig)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal auth_config: %w", err)
    }
}
```

**Verification**:
```bash
# Check if auth_config is saved
psql -h localhost -U devmesh -d devmesh_development \
  -c "SELECT name, auth_config FROM tool_configurations WHERE name='your-tool';"
```

#### 4. Operation Grouping Issues

**Problem**: Tools not being split into groups as expected.

**Solutions**:
1. Verify `group_operations: true` is set
2. Check that the OpenAPI spec has proper tags
3. Try different grouping strategies:
   - `"tag"`: Use only OpenAPI tags
   - `"path"`: Use URL path structure
   - `"hybrid"`: Combine both approaches

#### 5. Tool Not Loading auth_config

**Problem**: Tool executions fail because auth_config is not loaded from database.

**Solution**: Verify the repository layer includes auth_config in SELECT queries:

```go
// In dynamic_tool_repository.go
query := `
    SELECT id, tenant_id, name, base_url, openapi_spec, 
           auth_type, auth_config, credentials, provider, config,
           group_operations, status, created_at, updated_at
    FROM tool_configurations
    WHERE id = $1 AND tenant_id = $2 AND status != 'deleted'
`
```

### Debugging Tools

#### 1. Check Tool Configuration

```bash
# List all tools
curl -X GET http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY"

# Get specific tool details
curl -X GET http://localhost:8081/api/v1/tools/{toolId} \
  -H "Authorization: Bearer YOUR_API_KEY"
```

#### 2. Test Tool Execution

```bash
# Test with debug logging
curl -X POST http://localhost:8081/api/v1/tools/{toolId}/execute/test \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"parameters": {}}'
```

#### 3. Database Inspection

```sql
-- Check tool configuration
SELECT 
    name,
    auth_type,
    auth_config::text,
    config::text,
    group_operations
FROM tool_configurations
WHERE tenant_id = 'YOUR_TENANT_ID';

-- Check grouped tools
SELECT 
    name,
    parent_tool_id,
    config::text
FROM tool_configurations
WHERE parent_tool_id IS NOT NULL;
```

#### 4. Application Logs

```bash
# Check service logs for errors
docker-compose logs -f rest-api | grep -i error

# Check for authentication issues
docker-compose logs -f rest-api | grep -i "auth"
```

## API Reference

### Key Endpoints

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/v1/tools` | POST | Register new tool |
| `/api/v1/tools` | GET | List all tools |
| `/api/v1/tools/{id}` | GET | Get tool details |
| `/api/v1/tools/{id}` | PUT | Update tool configuration |
| `/api/v1/tools/{id}` | DELETE | Remove tool |
| `/api/v1/tools/{id}/execute/{action}` | POST | Execute tool action |
| `/api/v1/tools/{id}/actions` | GET | List available actions |
| `/api/v1/tools/{id}/health` | GET | Check tool health |

### Request/Response Examples

#### Registration Response

```json
{
  "id": "tool_123456",
  "name": "github",
  "base_url": "https://api.github.com",
  "auth_type": "bearer",
  "auth_config": {
    "header_prefix": "Bearer"
  },
  "group_operations": true,
  "status": "active",
  "created_at": "2025-08-20T10:00:00Z"
}
```

#### Grouped Tools Response

```json
{
  "tools": [
    {
      "id": "tool_123457",
      "name": "github_repos",
      "parent_tool_id": "tool_123456",
      "operations_count": 45
    },
    {
      "id": "tool_123458", 
      "name": "github_issues",
      "parent_tool_id": "tool_123456",
      "operations_count": 32
    }
  ]
}
```

## Additional Tool Examples

### Jira Registration

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "jira",
    "base_url": "https://your-domain.atlassian.net",
    "openapi_url": "https://developer.atlassian.com/cloud/jira/platform/swagger-v3.v3.json",
    "auth_type": "basic",
    "credential": {
      "type": "basic",
      "username": "your-email@example.com",
      "password": "YOUR_ATLASSIAN_API_TOKEN"
    },
    "group_operations": true,
    "config": {
      "max_operations_per_group": 50,
      "default_parameters": {
        "maxResults": 100
      }
    }
  }'
```

### Jenkins Registration

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "jenkins",
    "base_url": "https://jenkins.example.com",
    "auth_type": "basic",
    "credential": {
      "type": "basic",
      "username": "jenkins-user",
      "password": "YOUR_JENKINS_API_TOKEN"
    },
    "config": {
      "discovery_paths": [
        "/api/json",
        "/openapi/openapi.json"
      ],
      "default_parameters": {
        "pretty": true
      }
    }
  }'
```

### Dynatrace Registration

Dynatrace offers comprehensive monitoring and observability with OpenAPI-compliant APIs. The platform uses a unique token format consisting of three components separated by dots (.).

#### Prerequisites

1. Generate an API token from your Dynatrace environment:
   - Navigate to **Access Tokens** in your Dynatrace environment
   - Select **Generate new token**
   - Choose required scopes (e.g., `Read entities`, `Read metrics`, `Write metrics`)
   - Copy the generated token immediately (format: `dt0c01.XXXXXXXX.YYYYYYYY`)

2. Identify your environment URL:
   - Production: `https://xyz12345.live.dynatrace.com`
   - Sprint: `https://abc67890.sprint.dynatracelabs.com`
   - Managed: `https://your-domain/e/environment-id`

#### Basic Registration

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dynatrace",
    "base_url": "https://xyz12345.live.dynatrace.com",
    "auth_type": "api_key",
    "auth_config": {
      "header_name": "Authorization",
      "header_prefix": "Api-Token"
    },
    "credential": {
      "type": "token",
      "token": "dt0c01.XXXXXXXX.YYYYYYYY"
    },
    "config": {
      "discovery_paths": [
        "/api/v2/openapi.json",
        "/api/config/v1/spec.json",
        "/api/cluster/v2/openapi.json"
      ],
      "default_parameters": {
        "pageSize": 100,
        "from": "now-24h"
      }
    }
  }'
```

Note: Dynatrace uses `Api-Token` prefix instead of the standard `Bearer` prefix.

#### OAuth2 Registration (Platform Apps)

For OAuth2-based authentication (recommended for platform integrations):

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dynatrace-oauth",
    "base_url": "https://xyz12345.live.dynatrace.com",
    "auth_type": "oauth2",
    "credential": {
      "type": "oauth2",
      "token": "YOUR_BEARER_TOKEN"
    },
    "config": {
      "oauth2_config": {
        "client_id": "YOUR_CLIENT_ID",
        "client_secret": "YOUR_CLIENT_SECRET",
        "token_url": "https://sso.dynatrace.com/oauth2/token",
        "scopes": ["storage:entities:read", "metrics.read", "logs.read"]
      }
    }
  }'
```

#### Accessing API Explorer

To discover available APIs and their OpenAPI specifications:

```bash
# Get API documentation links
curl -H "Authorization: Api-Token dt0c01.XXXXXXXX.YYYYYYYY" \
  https://xyz12345.live.dynatrace.com/api/v2/openapi.json
```

#### Testing the Integration

```bash
# Test with environment API
curl -X POST http://localhost:8081/api/v1/tools/dynatrace/execute/list_entities \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "entitySelector": "type(\"SERVICE\")",
      "pageSize": 10
    }
  }'
```

### Datadog Registration

Datadog uses a dual-key authentication system with separate API keys and Application keys for different operations.

#### Prerequisites

1. Generate API Key:
   - Navigate to **Organization Settings > API Keys**
   - Click **New Key**
   - Copy the generated API key

2. Generate Application Key:
   - Navigate to **Organization Settings > Application Keys**
   - Click **New Key**
   - Copy the generated application key immediately (cannot be retrieved later)

#### Important Authentication Limitation

⚠️ **Note**: Datadog requires two separate authentication headers (`DD-API-KEY` and `DD-APPLICATION-KEY`) for most operations. The current dynamic tools system supports single-header authentication. As a workaround, you have the following options:

**Option 1: API Key Only (Limited Operations)**
```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "datadog-write",
    "base_url": "https://api.datadoghq.com",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "DD-API-KEY"
    },
    "credential": {
      "type": "token",
      "token": "YOUR_DD_API_KEY"
    },
    "config": {
      "note": "Limited to write operations (metrics, events, logs)",
      "default_parameters": {
        "api_version": "v2"
      }
    }
  }'
```

**Option 2: Combined Key Format (Custom Implementation Required)**
Some operations may work with a custom proxy or middleware that combines both keys:
```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "datadog",
    "base_url": "https://api.datadoghq.com",
    "openapi_url": "https://api.datadoghq.com/api/v2/openapi.json",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "DD-API-KEY",
      "note": "Application key must be passed via passthrough authentication or proxy"
    },
    "credential": {
      "type": "token",
      "token": "YOUR_DD_API_KEY"
    },
    "config": {
      "requires_dual_auth": true,
      "application_key": "YOUR_DD_APPLICATION_KEY_FOR_REFERENCE_ONLY",
      "note": "Full API access requires custom middleware to inject DD-APPLICATION-KEY header"
    }
  }'
```

**Option 3: Regional Endpoints**
For region-specific deployments:
```bash
# EU region
"base_url": "https://api.datadoghq.eu"

# US3 region
"base_url": "https://api.us3.datadoghq.com"

# US5 region
"base_url": "https://api.us5.datadoghq.com"

# Government region
"base_url": "https://api.ddog-gov.com"
```

#### Testing Basic Functionality

For operations that only require API key:
```bash
# Submit metrics (API key only)
curl -X POST http://localhost:8081/api/v1/tools/datadog-write/execute/submit_metrics \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "series": [{
        "metric": "test.metric",
        "points": [[1234567890, 20]],
        "type": "gauge",
        "tags": ["test:true"]
      }]
    }
  }'
```

#### Validation Endpoints

To validate your keys:
```bash
# Validate API key (requires API key only)
curl -X GET https://api.datadoghq.com/api/v1/validate \
  -H "DD-API-KEY: YOUR_DD_API_KEY"

# Validate application key (requires both keys)
curl -X GET https://api.datadoghq.com/api/v2/validate_keys \
  -H "DD-API-KEY: YOUR_DD_API_KEY" \
  -H "DD-APPLICATION-KEY: YOUR_DD_APPLICATION_KEY"
```

### Docker Hub Registration

Docker Hub provides multiple APIs for different purposes: the Registry API for image operations and the Hub API for repository management.

#### Prerequisites

1. Create a Personal Access Token (PAT):
   - Sign in to Docker Hub and go to **Account Settings**
   - Select **Personal access tokens** → **Generate new token**
   - Configure permissions (Read, Write, or Delete)
   - Copy the token immediately (cannot be retrieved later)

2. Understand the API endpoints:
   - **Registry API**: `registry-1.docker.io` (image pull/push operations)
   - **Hub API**: `hub.docker.com` (repository management)
   - **Auth endpoint**: `auth.docker.io` (token generation)

#### Important Limitations

⚠️ **Note**: Docker Hub's API authentication has known limitations as of 2025:
- PATs work well for Docker CLI and Registry operations
- Some Hub API endpoints return "403 Forbidden" with PATs
- Full API support may require username/password authentication for certain endpoints

#### Option 1: Docker Hub API (Repository Management)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dockerhub",
    "base_url": "https://hub.docker.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_DOCKER_PAT"
    },
    "config": {
      "discovery_paths": [
        "/v2/openapi.json",
        "/api/v2/openapi.json"
      ],
      "note": "Limited PAT support for some endpoints",
      "default_parameters": {
        "page_size": 100
      }
    }
  }'
```

#### Option 2: Docker Registry API (Image Operations)

For registry operations (pulling/pushing images programmatically):

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "docker-registry",
    "base_url": "https://registry-1.docker.io",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_DOCKER_PAT"
    },
    "config": {
      "auth_endpoint": "https://auth.docker.io/token",
      "service": "registry.docker.io",
      "note": "Requires token from auth.docker.io for each operation"
    }
  }'
```

#### Option 3: JWT Authentication (Hub API Full Access)

For full Hub API access using JWT tokens:

```bash
# First, get JWT token
JWT_TOKEN=$(curl -s -X POST https://hub.docker.com/v2/users/login/ \
  -H "Content-Type: application/json" \
  -d '{"username":"YOUR_USERNAME","password":"YOUR_PASSWORD"}' | jq -r .token)

# Then register with JWT
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "dockerhub-jwt",
    "base_url": "https://hub.docker.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "'$JWT_TOKEN'"
    },
    "config": {
      "note": "JWT tokens expire and need refresh",
      "token_endpoint": "https://hub.docker.com/v2/users/login/"
    }
  }'
```

#### Testing the Integration

```bash
# List repositories (Hub API)
curl -X POST http://localhost:8081/api/v1/tools/dockerhub/execute/list_repos \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "namespace": "YOUR_DOCKER_USERNAME",
      "page_size": 10
    }
  }'
```

### JFrog Artifactory Registration

JFrog Artifactory provides comprehensive REST APIs for artifact management, with multiple authentication options.

#### Prerequisites

1. Generate an Access Token:
   - Navigate to your Artifactory instance
   - Go to **User Profile** → **Generate Access Token**
   - Configure token scope and expiration
   - Save the token securely

2. Alternative: Use API Key (legacy):
   - Go to **User Profile** → **API Key**
   - Generate or regenerate your API key

3. Identify your Artifactory URL:
   - Cloud: `https://yourcompany.jfrog.io`
   - Self-hosted: `https://artifactory.yourcompany.com`

#### Option 1: Bearer Token Authentication (Recommended)

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "artifactory",
    "base_url": "https://yourcompany.jfrog.io",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_ARTIFACTORY_ACCESS_TOKEN"
    },
    "config": {
      "discovery_paths": [
        "/artifactory/api/openapi.json",
        "/artifactory/api/swagger.json",
        "/artifactory/api/application.wadl"
      ],
      "default_parameters": {
        "limit": 100
      }
    }
  }'
```

#### Option 2: API Key Authentication (Legacy)

Using the X-JFrog-Art-Api header:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "artifactory-apikey",
    "base_url": "https://yourcompany.jfrog.io",
    "auth_type": "custom_header",
    "auth_config": {
      "header_name": "X-JFrog-Art-Api"
    },
    "credential": {
      "type": "token",
      "token": "YOUR_ARTIFACTORY_API_KEY"
    },
    "config": {
      "discovery_paths": [
        "/artifactory/api/application.wadl"
      ]
    }
  }'
```

#### Option 3: Basic Authentication

For username/password or username/API key:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "artifactory-basic",
    "base_url": "https://yourcompany.jfrog.io",
    "auth_type": "basic",
    "credential": {
      "type": "basic",
      "username": "YOUR_USERNAME",
      "password": "YOUR_API_KEY_OR_PASSWORD"
    },
    "config": {
      "discovery_paths": [
        "/artifactory/api/application.wadl"
      ]
    }
  }'
```

#### Option 4: Reference Token (Short-lived)

For environments with token length limitations:

```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "artifactory-ref",
    "base_url": "https://yourcompany.jfrog.io",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_REFERENCE_TOKEN"
    },
    "config": {
      "token_type": "reference",
      "note": "Reference tokens are shorter aliases for full access tokens"
    }
  }'
```

#### Testing the Integration

```bash
# Get system info
curl -X POST http://localhost:8081/api/v1/tools/artifactory/execute/system_info \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{}'

# List repositories
curl -X POST http://localhost:8081/api/v1/tools/artifactory/execute/list_repositories \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "type": "local",
      "packageType": "docker"
    }
  }'

# Search artifacts
curl -X POST http://localhost:8081/api/v1/tools/artifactory/execute/search_artifacts \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "name": "*.jar",
      "repos": "libs-release-local"
    }
  }'
```

#### Important Notes

1. **Token Types**: 
   - GUI generates 64-character tokens (works with X-JFrog-Art-Api or Bearer)
   - REST API generates 757-character tokens (Bearer only)
   - Reference tokens are shorter aliases for compatibility

2. **Auto-generated Tokens**: When using JFrog CLI with username/password, it automatically generates a 1-hour access token

3. **Transient Users**: Access tokens can create temporary users that only exist while the token is valid

4. **mTLS Support**: From Artifactory 7.38.4+, client certificate authentication is supported (requires reverse proxy setup)

### Argo CD Registration

Argo CD is a declarative, GitOps continuous delivery tool for Kubernetes that provides a comprehensive REST API secured with JWT authentication.

#### Prerequisites

1. **Generate JWT Token**:
   - For admin access: Exchange username/password for JWT via `/api/v1/session`
   - For automation: Generate project-specific tokens via `/api/v1/projects/{project}/roles/{role}/token`
   - Tokens expire after 24 hours (admin) or configurable duration (project tokens)

2. **Identify your Argo CD server**:
   - Default: `https://argocd-server.argocd.svc.cluster.local` (in-cluster)
   - External: `https://argocd.example.com` (exposed via ingress)
   - Port-forwarded: `http://localhost:8080` (for development)

#### Option 1: Admin Token Registration

First, obtain an admin JWT token:
```bash
# Get admin JWT token
ARGOCD_TOKEN=$(curl -s https://argocd.example.com/api/v1/session \
  -d '{"username":"admin","password":"YOUR_ADMIN_PASSWORD"}' | jq -r .token)

# Register Argo CD with the token
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "argocd",
    "base_url": "https://argocd.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "'$ARGOCD_TOKEN'"
    },
    "config": {
      "discovery_paths": [
        "/swagger.json",
        "/api/v1/swagger.json",
        "/swagger-ui/swagger.json"
      ],
      "token_endpoint": "/api/v1/session",
      "token_expiry": "24h",
      "note": "Admin tokens expire after 24 hours"
    }
  }'
```

#### Option 2: Project-Scoped Token Registration

For production use with limited scope:
```bash
# Generate project-specific token (using argocd CLI)
argocd proj role create-token YOUR_PROJECT YOUR_ROLE --token-only

# Or via API (requires existing authentication)
PROJECT_TOKEN=$(curl -s -X POST \
  https://argocd.example.com/api/v1/projects/YOUR_PROJECT/roles/YOUR_ROLE/token \
  -H "Authorization: Bearer $ADMIN_TOKEN" | jq -r .token)

# Register with project token
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "argocd-project",
    "base_url": "https://argocd.example.com",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "'$PROJECT_TOKEN'"
    },
    "config": {
      "discovery_paths": [
        "/swagger.json",
        "/api/v1/swagger.json"
      ],
      "project": "YOUR_PROJECT",
      "role": "YOUR_ROLE",
      "note": "Limited to project scope"
    }
  }'
```

#### Option 3: SSO/OIDC Token Registration

For organizations using SSO:
```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "argocd-oidc",
    "base_url": "https://argocd.example.com",
    "auth_type": "oauth2",
    "credential": {
      "type": "oauth2",
      "token": "YOUR_OIDC_TOKEN"
    },
    "config": {
      "discovery_paths": [
        "/swagger.json"
      ],
      "oidc_provider": "YOUR_IDP",
      "note": "Token managed by identity provider"
    }
  }'
```

#### Testing the Integration

```bash
# List applications
curl -X POST http://localhost:8081/api/v1/tools/argocd/execute/list_applications \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "project": "default"
    }
  }'

# Get application details
curl -X POST http://localhost:8081/api/v1/tools/argocd/execute/get_application \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "name": "YOUR_APP_NAME"
    }
  }'

# Sync application
curl -X POST http://localhost:8081/api/v1/tools/argocd/execute/sync_application \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "name": "YOUR_APP_NAME",
      "revision": "HEAD",
      "prune": true
    }
  }'
```

#### Accessing Swagger Documentation

The Swagger UI is available at `/swagger-ui`:
```bash
# View API documentation
open https://argocd.example.com/swagger-ui

# Download OpenAPI specification
curl https://argocd.example.com/swagger.json -o argocd-openapi.json
```

### Istio Service Mesh Registration

Istio is a service mesh that provides traffic management, security, and observability. While Istio primarily uses Kubernetes CRDs for configuration, it does expose some APIs for management and telemetry.

#### Prerequisites

1. **Kubernetes Service Account Token**:
   - Istio uses Kubernetes RBAC for authentication
   - Requires service account with appropriate permissions
   - Token can be extracted from service account secret

2. **Identify Istio endpoints**:
   - **Istiod**: Control plane API (typically port 15014)
   - **Pilot**: xDS discovery service (port 15010)
   - **Telemetry**: Metrics and traces (port 15004)

#### Important Note

⚠️ **Istio API Limitations**: Istio doesn't provide a traditional REST API with OpenAPI specification. Instead, it uses:
- Kubernetes Custom Resource Definitions (CRDs) for configuration
- xDS protocol for proxy configuration
- gRPC for internal communication

The examples below show how to interact with Istio through the Kubernetes API.

#### Option 1: Kubernetes API Registration (Istio CRDs)

Register access to Istio through Kubernetes API:
```bash
# Extract service account token
SA_TOKEN=$(kubectl get secret -n istio-system \
  $(kubectl get sa -n istio-system istio-reader-service-account -o jsonpath='{.secrets[0].name}') \
  -o jsonpath='{.data.token}' | base64 -d)

# Register Kubernetes API for Istio management
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "istio-k8s",
    "base_url": "https://YOUR_K8S_API_SERVER",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "'$SA_TOKEN'"
    },
    "config": {
      "discovery_paths": [
        "/openapi/v2",
        "/openapi/v3"
      ],
      "api_groups": [
        "networking.istio.io",
        "security.istio.io",
        "telemetry.istio.io"
      ],
      "note": "Access Istio via Kubernetes API"
    }
  }'
```

#### Option 2: Istiod Debug API Registration

For debugging and monitoring (not for production):
```bash
# Port-forward to Istiod
kubectl port-forward -n istio-system deployment/istiod 15014:15014

# Register Istiod debug endpoints
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "istiod-debug",
    "base_url": "http://localhost:15014",
    "auth_type": "none",
    "config": {
      "endpoints": {
        "debug": "/debug",
        "metrics": "/metrics",
        "config_dump": "/debug/config_dump",
        "clusters": "/debug/clusters",
        "sync": "/debug/syncz"
      },
      "note": "Debug endpoints only - not for production"
    }
  }'
```

#### Option 3: Telemetry API Registration

For accessing Istio telemetry data:
```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "istio-telemetry",
    "base_url": "https://YOUR_PROMETHEUS_SERVER",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "YOUR_PROMETHEUS_TOKEN"
    },
    "config": {
      "metrics_path": "/api/v1/query",
      "queries": {
        "request_rate": "istio_request_total",
        "success_rate": "istio_request_duration_milliseconds_bucket",
        "latency_p99": "histogram_quantile(0.99, istio_request_duration_milliseconds_bucket)"
      }
    }
  }'
```

#### Working with Istio CRDs

Example operations through Kubernetes API:
```bash
# List VirtualServices
curl -X POST http://localhost:8081/api/v1/tools/istio-k8s/execute/list_virtualservices \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "namespace": "default",
      "labelSelector": "app=myapp"
    }
  }'

# Get DestinationRule
curl -X POST http://localhost:8081/api/v1/tools/istio-k8s/execute/get_destinationrule \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "namespace": "default",
      "name": "my-destination-rule"
    }
  }'

# Create AuthorizationPolicy
curl -X POST http://localhost:8081/api/v1/tools/istio-k8s/execute/create_authorizationpolicy \
  -H "Authorization: Bearer YOUR_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "parameters": {
      "namespace": "default",
      "name": "my-auth-policy",
      "spec": {
        "selector": {
          "matchLabels": {
            "app": "myapp"
          }
        },
        "action": "ALLOW",
        "rules": [{
          "from": [{
            "source": {
              "principals": ["cluster.local/ns/default/sa/myservice"]
            }
          }]
        }]
      }
    }
  }'
```

#### Important Considerations

1. **No Traditional REST API**: Istio operates through Kubernetes CRDs and xDS protocols
2. **Authentication**: Uses Kubernetes service account tokens
3. **Configuration**: Managed through Kubernetes resources (VirtualService, DestinationRule, etc.)
4. **Telemetry**: Access metrics through Prometheus or similar backends
5. **Security**: mTLS and JWT validation configured via CRDs

## Best Practices

1. **Always use HTTPS** for production API endpoints
2. **Configure default parameters** for required fields
3. **Enable operation grouping** for large APIs
4. **Use fine-grained tokens** when available (GitHub PATs)
5. **Set appropriate token permissions** for least privilege
6. **Monitor token expiration** and implement rotation
7. **Test authentication** before deploying to production
8. **Document custom parameters** for your organization
9. **Use passthrough authentication** for user-specific actions
10. **Regular security audits** of registered tools and credentials

### Tool-Specific Recommendations

#### For Harness:
- Use service accounts for automation, not personal tokens
- Always include `accountIdentifier` in default parameters
- Consider registering modules separately for better organization
- Enable operation grouping due to large API surface
- Token format includes account ID for added security
- Store tokens securely - they're only shown once when created

#### For Snyk:
- Use service accounts for automation, not personal tokens
- Always specify the API version (use `2024-10-15` unless you need an older version)
- Monitor the 1620 requests/minute rate limit
- Enterprise plan required for API access

#### For SonarQube:
- Use project-specific tokens when possible for least privilege
- Monitor token expiration via response headers
- Consider caching responses as the API can be slow for large projects
- Be aware that Web API V2 is gradually replacing V1

#### For GitHub:
- Prefer fine-grained PATs over classic tokens
- Enable operation grouping due to the large API surface
- Use GitHub Apps for organization-wide automation
- Set appropriate `per_page` default parameter (max 100)

#### For GitLab:
- Personal access tokens expire (max 400 days as of 2025)
- Use project or group tokens for scoped access
- Both `PRIVATE-TOKEN` header and Bearer authentication work
- Enable token rotation for security

#### For Docker Hub:
- PATs work well for Docker CLI and Registry operations
- Some Hub API endpoints have limited PAT support
- Consider JWT authentication for full Hub API access
- Use separate tools for Registry API vs Hub API
- JWT tokens expire and need refresh mechanisms

#### For Artifactory:
- Prefer access tokens over API keys (legacy)
- Use reference tokens for environments with length limitations
- Access tokens can create transient users (no pre-existing user needed)
- Auto-generated tokens via JFrog CLI expire after 1 hour
- Consider mTLS for enhanced security (7.38.4+)

## Security Considerations

1. **Credential Encryption**: All credentials are encrypted using AES-256-GCM
2. **Per-Tenant Isolation**: Tools are isolated by tenant_id
3. **Token Rotation**: Implement regular token rotation policies
4. **Audit Logging**: All tool operations are logged for compliance
5. **Network Security**: Validate TLS certificates and use secure connections
6. **Permission Validation**: Verify token permissions match required operations
7. **Rate Limiting**: Implement rate limits to prevent abuse

## Migration Guide

If migrating from hardcoded tools:

1. **Export existing configurations** from your codebase
2. **Prepare OpenAPI specifications** for each tool
3. **Register tools** using the Dynamic Tools API
4. **Update agent configurations** to use new tool names
5. **Test thoroughly** in a staging environment
6. **Monitor logs** during initial production deployment
7. **Remove legacy code** once migration is verified

## Summary

This guide covered comprehensive examples for registering various DevOps tools with the Developer Mesh platform:

### Quick Reference Table

| Tool | Authentication Type | Special Requirements | Notes |
|------|-------------------|---------------------|-------|
| **GitHub** | Bearer (`Bearer` prefix) | None | Use fine-grained PATs, enable operation grouping |
| **GitLab** | Custom Header (`PRIVATE-TOKEN`) or Bearer | None | Tokens expire (max 400 days) |
| **Harness** | API Key (`x-api-key` header) | `accountIdentifier` parameter | Personal or service account tokens |
| **Snyk** | Custom Header (`token` prefix) | `org_id`, `version` parameters | Enterprise plan required for API |
| **SonarQube** | Bearer | Discovery paths needed | No standard OpenAPI spec |
| **Jira** | Basic Auth | Atlassian API token | Use email + API token |
| **Jenkins** | Basic Auth | API token | Discovery paths recommended |
| **Dynatrace** | API Key (`Api-Token` prefix) or OAuth2 | `pageSize`, `from` parameters | Multiple API versions available |
| **Datadog** | Custom Headers (DD-API-KEY, DD-APPLICATION-KEY) | Regional endpoints | ⚠️ Dual key auth - limited support |
| **Docker Hub** | Bearer or JWT | Multiple endpoints | ⚠️ PAT limitations for some Hub API endpoints |
| **Artifactory** | Bearer, API Key, or Basic | Discovery paths available | Multiple token formats supported |

### Key Takeaways

1. **Authentication Flexibility**: The system supports various authentication methods through `auth_type` and `auth_config`
2. **Default Parameters**: Critical for tools requiring constant parameters (Snyk's `org_id`, API versions)
3. **Discovery Mechanisms**: Automatic discovery works for most tools, with hints and fallbacks for others
4. **Operation Grouping**: Essential for large APIs to improve AI agent comprehension
5. **No Manual Operations**: Tools must provide or have discoverable API specifications

### Common Pitfalls to Avoid

- ❌ Forgetting required default parameters (causes "parameter missing" errors)
- ❌ Using wrong authentication type (results in 401/403 errors)
- ❌ Not saving auth_config for custom headers (authentication fails)
- ❌ Ignoring rate limits (API throttling)
- ❌ Using personal tokens for automation (use service accounts)
- ❌ Not testing discovery before production deployment

### Next Steps

1. Review the [API Reference](./reference/api.md) for detailed endpoint documentation
2. Test tool registration in a development environment first
3. Monitor tool health and usage through the audit logs
4. Implement token rotation policies for production
5. Document your organization's specific tool configurations

---

*Last Updated: 2025-08-20*
*Version: 3.0.0*
*Enhanced with Harness.io, Snyk, and SonarQube examples*