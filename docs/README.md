# Developer Mesh Documentation

Welcome to the Developer Mesh documentation. This guide provides comprehensive information for users, developers, and operators of the Developer Mesh platform.

## 📖 Documentation Overview

### 🚀 Quick Start
- **[Getting Started](quickstart.md)** - Start here if you're new to Developer Mesh

### 📂 Documentation by Domain

#### Core Features
- **[AI Agents](agents/)** - AI agent integration and orchestration
- **[Authentication](authentication/)** - Security, auth, and access control
- **[Dynamic Tools](dynamic-tools/)** - Zero-code tool integration
- **[Embeddings](embeddings/)** - Vector embeddings and semantic search
- **[MCP Protocol](mcp-protocol/)** - Model Context Protocol implementation

#### Platform
- **[Organizations](organizations/)** - Organization and tenant management
- **[API Reference](api/)** - Complete API documentation
- **[Architecture](architecture/)** - System design and architecture

#### Operations
- **[Deployment](deployment/)** - Deployment and operations guide
- **[Development](development/)** - Developer documentation
- **[Integrations](integrations/)** - External integrations
- **[Troubleshooting](troubleshooting/)** - Problem-solving guides

### 🏗️ New Domain-Based Structure

We've reorganized our documentation following 2025 best practices for domain-driven documentation:

**Each domain includes:**
- `README.md` - Overview and navigation
- `quickstart/` - Getting started quickly
- `guides/` - How-to guides
- `examples/` - Code examples
- `reference/` - API references
- `troubleshooting/` - Problem solving

## 🚀 Quick Links

### Tool Registration
Quick access to tool-specific registration examples:
- [Harness Registration](dynamic-tools/registration-guide.md#harnessio-registration) - CI/CD platform with x-api-key authentication
- [Snyk Registration](dynamic-tools/registration-guide.md#snyk-registration) - Security scanning with required parameters
- [SonarQube Registration](dynamic-tools/registration-guide.md#sonarqube-registration) - Code quality analysis
- [Dynatrace Registration](dynamic-tools/registration-guide.md#dynatrace-registration) - Monitoring with Api-Token prefix
- [Datadog Registration](dynamic-tools/registration-guide.md#datadog-registration) - Observability with dual-key auth
- [Docker Hub Registration](dynamic-tools/registration-guide.md#docker-hub-registration) - Container registry with PAT/JWT auth
- [Artifactory Registration](dynamic-tools/registration-guide.md#jfrog-artifactory-registration) - Artifact repository management
- [Argo CD Registration](dynamic-tools/registration-guide.md#argo-cd-registration) - GitOps continuous delivery
- [Istio Registration](dynamic-tools/registration-guide.md#istio-registration) - Service mesh (via Kubernetes API)
- [GitHub Registration](dynamic-tools/registration-guide.md#github-registration) - Repository management
- [GitLab Registration](dynamic-tools/registration-guide.md#gitlab-registration) - Alternative Git platform
- [Jira Registration](dynamic-tools/registration-guide.md#jira-registration) - Issue tracking
- [Kubernetes Registration](dynamic-tools/registration-guide.md#kubernetes-registration) - Container orchestration
- [Prometheus Registration](dynamic-tools/registration-guide.md#prometheus-registration) - Metrics monitoring
- [More Tools...](dynamic-tools/registration-guide.md#additional-tool-examples)

### For Users
- [Quick Start Guide](quickstart.md)
- [AI Agents](agents/) - Agent integration
- [Dynamic Tools](dynamic-tools/) - Tool registration

### For Developers
- [Development Guide](development/) - Complete dev documentation
- [Architecture](architecture/) - System design
- [API Reference](api/) - API documentation

### For Operators
- [Deployment Guide](deployment/) - Deployment and operations
- [Monitoring](deployment/operations/monitoring.md)
- [Security](deployment/operations/security.md)

## 📚 Documentation Structure (2025 Domain-Driven)

```
docs/
├── agents/                # AI agent domain
├── authentication/        # Auth & security domain
├── dynamic-tools/         # Tool integration domain
├── embeddings/           # Embeddings domain
├── mcp-protocol/         # MCP protocol domain
├── organizations/        # Organization management
├── deployment/           # Deployment & operations
├── development/          # Developer resources
├── api/                  # API documentation
├── architecture/         # System architecture
├── integrations/         # External integrations
└── troubleshooting/      # Problem solving
```

## 🔍 Finding Information

### By Role

**Application Developer**
- Start with [Quick Start](quickstart.md)
- Explore [Dynamic Tools](dynamic-tools/)
- Review [API Reference](api/)

**Platform Developer**
- Read [Architecture](architecture/)
- Set up [Development](development/)
- Check [Contributing](development/contributing/guide.md)

**DevOps Engineer**
- Check [Deployment](deployment/)
- Review [Operations](deployment/operations/guide.md)
- Understand [Monitoring](deployment/operations/monitoring.md)

### By Topic

**Integration**
- [GitHub Integration](integrations/github/)
- [AI Agent Setup](agents/)
- [Custom Tools](dynamic-tools/)

**Embedding & Search**
- [Embeddings Documentation](embeddings/)
- [Embedding API](embeddings/reference/api.md)

**Troubleshooting**
- [General Troubleshooting](troubleshooting/)
- [Debugging Guide](development/debugging.md)

## 📝 Documentation Standards

Our documentation follows these principles:

1. **Clear and Concise**: Easy to understand
2. **Example-Driven**: Real-world code examples
3. **Up-to-Date**: Reflects current implementation
4. **Searchable**: Well-organized and indexed
5. **Accessible**: Written for various skill levels

## 🤝 Contributing to Documentation

Documentation improvements are always welcome! See our [Contributing Guide](../CONTRIBUTING.md) for:

- Documentation style guide
- How to submit documentation PRs
- Building documentation locally

## 📞 Getting Help

Can't find what you need?

1. Search the documentation
2. Check [GitHub Issues](https://github.com/developer-mesh/developer-mesh/issues)
3. Ask in [Discussions](https://github.com/developer-mesh/developer-mesh/discussions)
4. Review [Examples](examples/README.md)

---

*Last updated: August 2025 - Added comprehensive Dynamic Tool Registration Guide with Docker Hub, JFrog Artifactory, Dynatrace, Datadog, Harness.io, Snyk, SonarQube, Kubernetes, Prometheus, Argo CD, and Istio examples*