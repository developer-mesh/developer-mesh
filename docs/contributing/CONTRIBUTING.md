<!-- SOURCE VERIFICATION
Last Verified: 2025-08-11 14:38:56
Verification Script: update-docs-parallel.sh
Batch: aa
-->

# Contributing to Developer Mesh

Thank you for your interest in contributing to Developer Mesh! This document provides guidelines for contributing to the project.

## 📋 Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Workflow](#development-workflow)
- [Pull Request Process](#pull-request-process)
- [Coding Standards](#coding-standards)
- [Testing Guidelines](#testing-guidelines)
- [Documentation](#documentation)
- [Communication](#communication)

## 🤝 Code of Conduct

We are committed to providing a welcoming and inclusive environment. All participants are expected to:

- Be respectful and considerate
- Welcome newcomers and help them get started
- Focus on what is best for the community
- Show empathy towards other community members

Harassment, offensive behavior, or discrimination of any kind will not be tolerated.

## 🚀 Getting Started

### Prerequisites

- Go 1.24
- Docker and Docker Compose
- PostgreSQL 14+ with pgvector extension
- Redis 8+ (for Redis Streams)
- Make
- Git
- golang-migrate (for database migrations)

### Development Environment Setup

1. **Fork and Clone**
   ```bash
   # Fork the repository on GitHub, then:
   git clone https://github.com/YOUR-USERNAME/developer-mesh.git
   cd developer-mesh
   
   # Add upstream remote
   git remote add upstream https://github.com/developer-mesh/developer-mesh.git
   ```

2. **Create a Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

3. **Set Up Local Environment**
   ```bash
   # Copy environment template
   cp .env.example .env
   # Edit .env with your settings
   
   # Set up development environment
   make dev-setup
   
   # Install dependencies (handled by Go workspace)
   go work sync
   
   # Start Docker services (PostgreSQL, Redis, etc.)
   make dev
   
   # Run database migrations
   make migrate-up-docker
   ```

4. **Build and Run**
   ```bash
   # Build all services
   make build
   
   # Run services (in separate terminals)
   make run-edge-mcp
   make run-rest-api
   make run-worker
   ```

## 💻 Development Workflow

### 1. Making Changes

- Create a feature branch from `main`
- Make your changes following our coding standards
- Write/update tests for your changes
- Update documentation as needed
- Commit with clear, descriptive messages

### 2. Commit Messages

Follow the [Conventional Commits](https://www.conventionalcommits.org/) specification:

```
<type>(<scope>): <subject>

<body>

<footer>
```

Types:
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation changes
- `style`: Code style changes (formatting, etc.)
- `refactor`: Code refactoring
- `test`: Test additions or modifications
- `chore`: Maintenance tasks

Example:
```
feat(api): add cross-model embedding search

- Implement POST /api/embeddings/search/cross-model
- Add support for searching across different embedding models
- Include dimension normalization for compatibility

Closes #123
```

### 3. Testing Your Changes

```bash
# Run unit tests (excludes integration and Redis-dependent tests)
make test

# Run tests with Docker services (Redis/PostgreSQL)
make test-with-services

# Run specific package tests
go test ./pkg/adapters/...

# Run E2E tests against local services
make test-e2e-local

# Check test coverage
make test-coverage

# Generate HTML coverage report
make test-coverage-html
```

## 🔄 Pull Request Process

1. **Before Submitting**
   - Ensure all tests pass: `make test`
   - Run pre-commit checks: `make pre-commit`
   - Format code: `make fmt`
   - Run linters: `make lint`
   - Update documentation if needed
   - Rebase on latest `main` branch

2. **PR Guidelines**
   - Use a descriptive title following commit message conventions
   - Fill out the PR template completely
   - Link related issues using keywords (Fixes #123, Closes #456)
   - Keep PRs focused - one feature/fix per PR
   - Add screenshots/examples for UI changes

3. **Review Process**
   - Address reviewer feedback promptly
   - Push additional commits (don't force-push during review)
   - Re-request review after making changes
   - Once approved, the PR will be squash-merged

## 📝 Coding Standards

### Go Code Style

We follow standard Go conventions and use automated tooling:

```bash
# Format code (excludes .claude templates which are not valid Go)
make fmt

# Run linters
make lint

# Install development tools
make install-tools
```

### Key Guidelines

1. **Package Design**
   - Keep packages focused and cohesive
   - Minimize package dependencies
   - Use interfaces for abstraction
   - Follow SOLID principles

2. **Error Handling**
   ```go
   // Good: Wrap errors with context
   if err != nil {
       return fmt.Errorf("failed to create context: %w", err)
   }
   
   // Use custom error types for API responses
   return &APIError{
       Code:    "CONTEXT_NOT_FOUND",
       Message: "Context not found",
       Status:  http.StatusNotFound,
   }
   ```

3. **Documentation**
   ```go
   // Package adapters provides implementations for external service integrations.
   // It follows the adapter pattern to isolate external dependencies.
   package adapters
   
   // GitHubAdapter provides GitHub API integration capabilities.
   // It implements the ToolAdapter interface for GitHub-specific operations.
   type GitHubAdapter struct {
       client *github.Client
       config GitHubConfig
   }
   ```

4. **Testing**
   - Write table-driven tests
   - Use testify for assertions
   - Mock external dependencies
   - Aim for >80% coverage

### Project Structure

```
devops-mcp/
├── apps/                    # Application modules (Go workspace)
│   ├── edge-mcp/           # MCP protocol WebSocket server
│   │   ├── cmd/            # Entry points
│   │   └── internal/       # Private packages
│   ├── rest-api/           # REST API service
│   ├── worker/             # Redis Streams event processor
│   ├── rag-loader/         # RAG code indexing and loading
│   └── mockserver/         # Mock server for testing
├── pkg/                    # Shared packages (public API)
│   ├── adapters/           # External adapters (AWS, MCP, etc.)
│   ├── models/             # Data models
│   ├── redis/              # Redis Streams client
│   ├── services/           # Business logic services
│   └── ...
├── configs/                # Configuration files
├── migrations/             # Database migrations
├── docs/                   # Documentation
├── scripts/                # Utility scripts
├── test/                   # E2E and integration tests
└── go.work                 # Go workspace file
```

## 🧪 Testing Guidelines

### Test Organization

```
package_test.go          # Unit tests
package_integration_test.go  # Integration tests
package_benchmark_test.go    # Benchmarks
testdata/                # Test fixtures
mocks/                   # Generated mocks
```

### Writing Tests

```go
func TestContextManager_CreateContext(t *testing.T) {
    tests := []struct {
        name    string
        input   *models.Context
        want    *models.Context
        wantErr bool
    }{
        {
            name: "valid context",
            input: &models.Context{
                Name: "test-context",
                Type: "conversation",
            },
            want: &models.Context{
                ID:   "generated-id",
                Name: "test-context",
                Type: "conversation",
            },
            wantErr: false,
        },
        // More test cases...
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Mock Generation

```bash
# Generate mocks for interfaces
go generate ./...

# Most mocks are defined in test files directly using testify/mock
# Example: see pkg/adapters/aws/aws_adapter_test.go
```

## 📚 Documentation

### Documentation Requirements

1. **Code Documentation**
   - All exported types, functions, and packages must have godoc comments
   - Include examples for complex functionality
   - Document any non-obvious behavior

2. **API Documentation**
   - Update OpenAPI/Swagger specs for API changes
   - Include request/response examples
   - Document error responses

3. **Architecture Documentation**
   - Update diagrams for structural changes
   - Document design decisions in ADRs (Architecture Decision Records)
   - Keep README files current

### Writing Documentation

```markdown
# Feature Name

## Overview
Brief description of the feature.

## Usage
How to use the feature with examples.

## Configuration
Required configuration with examples.

## API Reference
Detailed API documentation if applicable.

## Troubleshooting
Common issues and solutions.
```

## 💬 Communication

### Getting Help

- **GitHub Issues**: Bug reports, feature requests, questions
- **Pull Requests**: Code reviews and implementation discussions
- **Discussions**: General discussions and ideas

### Reporting Issues

When reporting issues, please include:
- Clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Environment details (OS, Go version, etc.)
- Relevant logs or error messages

## 🎯 Areas for Contribution

### Good First Issues

Look for issues labeled `good first issue` for beginner-friendly tasks:
- Documentation improvements
- Test coverage additions
- Simple bug fixes
- Code cleanup

### Feature Requests

Check issues labeled `enhancement` for feature ideas:
- New tool integrations
- API improvements
- Performance optimizations
- UI/UX enhancements

### Current Priorities

- Redis Streams integration (completed)
- Dynamic tools implementation
- MCP (Model Context Protocol) compliance
- Multi-agent orchestration
- Security hardening
- Test coverage expansion (target: 85%)

## 🙏 Thank You!

Your contributions make Developer Mesh better for everyone. We appreciate your time and effort!

