CONTRIBUTING.md:

# Contributing to gpu-orchestrator

Thank you for your interest in contributing! Here's how you can help:

## Getting Started

1. **Fork** the repository
2. **Clone** your fork locally
3. **Create** a feature branch: `git checkout -b feature/your-feature-name`

## Development Setup

```bash
# Install dependencies
go mod download

# Run tests
make test

# Build locally
make build

# Run controller locally
make run
```

## Code Standards

- **Format**: Run `go fmt ./...`
- **Vet**: Run `go vet ./...`
- **Lint**: Run `make lint` (requires golangci-lint)
- **Tests**: All new features must have unit tests
- **Comments**: Export all public symbols with comments

## Testing

```bash
# Run all tests
make test

# Run tests with coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test
go test -run TestName ./package/...
```

## Commits

- Use clear, descriptive commit messages
- Reference issues where applicable: `Fixes #123`
- Keep commits focused on a single concern

## Pull Requests

1. **Describe** your changes clearly
2. **Test** thoroughly before submitting
3. **Reference** any related issues
4. **Review** the CI results

## Reporting Issues

Please include:
- Kubernetes version
- Go version
- Detailed steps to reproduce
- Expected vs actual behavior

## License

All contributions are under Apache License 2.0.

Thank you for contributing!
