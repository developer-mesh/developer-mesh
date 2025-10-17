# Contributing Documentation

This directory contains guidelines and resources for contributing to the Developer Mesh project.

## Contents

### Core Guidelines
- **[CONTRIBUTING.md](CONTRIBUTING.md)** - Main contribution guidelines, code of conduct, development workflow, coding standards, pull request process, and testing requirements

### Development Setup
- **[development-environment.md](development-environment.md)** - Complete guide for setting up your local development environment
- **[debugging-guide.md](debugging-guide.md)** - Troubleshooting and debugging development issues

### Testing Documentation
- **[testing-guide.md](testing-guide.md)** - Comprehensive testing guide covering unit, integration, and e2e tests
- **[testing-uuid-guidelines.md](testing-uuid-guidelines.md)** - Guidelines for handling UUIDs in tests
- **[github-integration-testing.md](github-integration-testing.md)** - Testing GitHub integrations
- **[github-tools-test-plan.md](github-tools-test-plan.md)** - Test plan for GitHub tools

### Authentication & Security
- **[authentication-implementation-guide.md](authentication-implementation-guide.md)** - Implementation guide for authentication features
- **[auth-test-coverage-report.md](auth-test-coverage-report.md)** - Authentication test coverage analysis
- **[github-app-setup.md](github-app-setup.md)** - Setting up GitHub App integration

### Code Quality & Patterns
- **[error-handling.md](error-handling.md)** - Error handling patterns and best practices

### Operations
- **[RELEASE_PIPELINE_FIXES.md](RELEASE_PIPELINE_FIXES.md)** - Release pipeline documentation and fixes

## Quick Links

- [Getting Started](../getting-started/) - Tutorials and initial setup guides
- [Architecture Overview](../architecture/system-overview.md) - System design and architecture
- [API Documentation](../reference/api/) - REST API reference
- [Configuration Guide](../reference/configuration/) - Configuration options

## Getting Started

1. Read the [Contributing Guide](CONTRIBUTING.md)
2. Set up your development environment: `make dev-setup && make dev`
3. Run tests to verify setup: `make test`
4. Review [development-environment.md](development-environment.md) for detailed setup instructions
5. Check the issue tracker for tasks to work on

## Development Workflow

1. **Setup**: Follow [development-environment.md](development-environment.md)
2. **Testing**: Read [testing-guide.md](testing-guide.md)
3. **Debugging**: Use [debugging-guide.md](debugging-guide.md) when issues arise
4. **Code Quality**: Follow patterns in [error-handling.md](error-handling.md)
5. **Pull Requests**: Follow process in [CONTRIBUTING.md](CONTRIBUTING.md)

## Need Help?

- Open an [issue](https://github.com/developer-mesh/developer-mesh/issues) for bugs or feature requests
- Check existing [pull requests](https://github.com/developer-mesh/developer-mesh/pulls) for similar work
- Review the [architecture documentation](../architecture/) for system design questions
- Check the [troubleshooting guide](../troubleshooting/) for common problems
