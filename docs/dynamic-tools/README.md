# Dynamic Tools Documentation

> Zero-code integration for any tool with an OpenAPI specification

## 📚 Documentation Structure

### For Users

#### Getting Started
- **[Registration Guide](./registration-guide.md)** - Comprehensive guide for registering tools
- **[API Reference](./reference/api.md)** - Complete API documentation

#### Tool-Specific Examples
Quick links to popular tool configurations:
- [Snyk Registration](./registration-guide.md#snyk-registration)
- [SonarQube Registration](./registration-guide.md#sonarqube-registration)
- [Harness.io Registration](./registration-guide.md#harnessio-registration)
- [Kubernetes Registration](./registration-guide.md#kubernetes-registration)
- [Prometheus Registration](./registration-guide.md#prometheus-registration)
- [Dynatrace Registration](./registration-guide.md#dynatrace-registration)
- [Datadog Registration](./registration-guide.md#datadog-registration)
- [Docker Hub Registration](./registration-guide.md#docker-hub-registration)
- [Artifactory Registration](./registration-guide.md#jfrog-artifactory-registration)
- [Argo CD Registration](./registration-guide.md#argo-cd-registration)
- [Istio Registration](./registration-guide.md#istio-registration)
- [GitHub Registration](./registration-guide.md#github-registration)
- [GitLab Registration](./registration-guide.md#gitlab-registration)
- [Jira Registration](./registration-guide.md#jira-registration)

### For Developers

#### Validation Reports
Internal validation documents for accuracy verification:
- [General Validation](./validation/general.md)
- [Dynatrace & Datadog](./validation/dynatrace-datadog.md)
- [Docker Hub & Artifactory](./validation/dockerhub-artifactory.md)
- [Argo CD & Istio](./validation/argocd-istio.md)

## 🚀 Quick Start

### 1. Check if Your Tool is Supported
Any tool that provides an OpenAPI 3.0+ specification can be integrated. Common tools include:
- CI/CD platforms (Jenkins, Harness, CircleCI, Argo CD)
- Security scanners (Snyk, SonarQube, Checkmarx)
- Container registries (Docker Hub, Harbor, Artifactory)
- Monitoring tools (Datadog, Dynatrace, New Relic)
- Cloud platforms (AWS, GCP, Azure via their APIs)
- Issue trackers (Jira, Linear, Asana)

### 2. Find Your Tool's OpenAPI Spec
Most modern tools provide OpenAPI/Swagger specifications:
```bash
# Common OpenAPI paths
https://api.example.com/openapi.json
https://api.example.com/swagger.json
https://api.example.com/v3/api-docs
https://api.example.com/api/v1/openapi
```

### 3. Register Your Tool
```bash
curl -X POST http://localhost:8081/api/v1/tools \
  -H "Authorization: Bearer $API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-tool",
    "base_url": "https://api.example.com",
    "openapi_url": "https://api.example.com/openapi.json",
    "auth_type": "bearer",
    "credential": {
      "type": "token",
      "token": "your-api-token"
    }
  }'
```

## 📖 Documentation Categories

### User Documentation
- **[Registration Guide](./registration-guide.md)** - Step-by-step registration instructions
- **[API Reference](./reference/api.md)** - Complete API documentation
- **[Examples](./examples/)** - Tool-specific configuration examples

### Technical Documentation
- **[Architecture](../architecture/dynamic-tools-architecture.md)** - System design
- **[Authentication](./reference/authentication.md)** - Auth methods and flows
- **[Discovery](./reference/discovery.md)** - Automatic API discovery

### Validation & Testing
- **[Validation Reports](./validation/)** - Internal accuracy verification
- **[Testing Guide](./reference/testing.md)** - How to test integrations

## 🔑 Key Features

### Zero-Code Integration
- Automatic API discovery from OpenAPI specs
- Dynamic authentication handling
- Intelligent operation resolution
- No code changes required

### Advanced Capabilities
- **Operation Resolution**: Maps simple actions to complex OpenAPI operations
- **Multi-Auth Support**: Bearer, API Key, Basic, OAuth2, Custom Headers
- **Discovery Engine**: Automatically finds API specifications
- **Health Monitoring**: Built-in health checks and monitoring
- **User Token Passthrough**: Use personal tokens instead of service accounts

### Security
- Per-tenant credential encryption (AES-256-GCM)
- Audit logging for all operations
- Rate limiting and quota management
- Token rotation support

## 📚 Additional Resources

### Related Documentation
- [System Architecture](../architecture/system-overview.md)
- [Security Guide](../operations/SECURITY.md)
- [Troubleshooting](../troubleshooting/dynamic-tools.md)

### External Resources
- [OpenAPI Specification](https://swagger.io/specification/)
- [API Authentication Best Practices](https://swagger.io/docs/specification/authentication/)

## 🤝 Contributing

To add documentation for a new tool:
1. Research the tool's authentication methods
2. Test the registration process
3. Add examples to the registration guide
4. Create a validation report
5. Submit a PR with your additions

## 📞 Support

Having issues with tool registration?
1. Check the [Registration Guide](./registration-guide.md)
2. Review [Troubleshooting](../troubleshooting/dynamic-tools.md)
3. Search [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
4. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)

---

*Last updated: August 2025*