# Contributing to DevOps MCP

Thank you for your interest in contributing to DevOps MCP! This document provides guidelines for contributing to the project.

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

- Go 1.24 or higher
- Docker and Docker Compose
- PostgreSQL 14+ with pgvector extension
- Redis 6.2+
- Make
- Git

### Development Environment Setup

1. **Fork and Clone**
   ```bash
   # Fork the repository on GitHub, then:
   git clone https://github.com/YOUR-USERNAME/devops-mcp.git
   cd devops-mcp
   
   # Add upstream remote
   git remote add upstream https://github.com/S-Corkum/devops-mcp.git
   ```

2. **Create a Feature Branch**
   ```bash
   git checkout -b feature/your-feature-name
   # or
   git checkout -b fix/your-bug-fix
   ```

3. **Set Up Local Environment**
   ```bash
   # Copy configuration template
   cp config.yaml.template config.yaml
   
   # Start infrastructure services
   docker-compose -f docker-compose.local.yml up -d
   
   # Install dependencies (handled by Go workspace)
   go work sync
   
   # Run database migrations
   make migrate-local
   ```

4. **Build and Run**
   ```bash
   # Build all services
   make build
   
   # Run services (in separate terminals)
   make run-mcp-server
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
feat(api): add vector similarity search endpoint

- Implement POST /api/v1/vectors/search
- Add support for multiple embedding models
- Include similarity threshold parameter

Closes #123
```

### 3. Testing Your Changes

```bash
# Run unit tests
make test

# Run specific package tests
go test ./pkg/adapters/...

# Run integration tests
make test-integration

# Check test coverage
make test-coverage
```

## 🔄 Pull Request Process

1. **Before Submitting**
   - Ensure all tests pass
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
# Format code
gofmt -w .

# Run linters
make lint

# Run with auto-fix
golangci-lint run --fix
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
│   ├── mcp-server/         # MCP protocol server
│   │   ├── cmd/            # Entry points
│   │   └── internal/       # Private packages
│   ├── rest-api/           # REST API service
│   └── worker/             # Event processor
├── pkg/                    # Shared packages (public API)
│   ├── adapters/           # External adapters
│   ├── models/             # Data models
│   └── ...
├── docs/                   # Documentation
├── scripts/                # Utility scripts
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

# Or manually with mockgen
mockgen -source=pkg/adapters/interfaces.go -destination=pkg/adapters/mocks/mock_adapter.go
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

See our [project board](https://github.com/S-Corkum/devops-mcp/projects) for current priorities and roadmap.

## 🙏 Thank You!

Your contributions make DevOps MCP better for everyone. We appreciate your time and effort!

For any questions not covered here, please open an issue or start a discussion.