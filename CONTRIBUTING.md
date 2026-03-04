# Contributing to Den

Thank you for your interest in contributing to Den! This guide will help you get started.

## Development Setup

### Prerequisites

- Go 1.23+
- Docker running locally
- [golangci-lint](https://golangci-lint.run/usage/install/)

### Building from Source

```bash
git clone https://github.com/us/den
cd den
go build -o den ./cmd/den
```

### Running Tests

```bash
# Unit tests
go test ./internal/... -short -v

# With race detector
go test ./internal/... -race -count=1 -v

# Integration tests (requires Docker)
go test ./tests/integration/... -v
```

### Running the Server

```bash
./den serve
# Or with custom config
./den serve --config den.yaml
```

## Pull Request Process

1. Fork the repository and create a feature branch from `main`
2. Branch naming: `feat/description`, `fix/description`, `refactor/description`
3. Write tests for new functionality
4. Ensure all tests pass: `go test ./internal/... -race`
5. Run the linter: `golangci-lint run`
6. Commit using [Conventional Commits](https://www.conventionalcommits.org/):
   - `feat:` new features
   - `fix:` bug fixes
   - `refactor:` code restructuring
   - `docs:` documentation
   - `test:` tests
   - `ci:` CI/CD changes
   - `perf:` performance improvements
7. Open a PR against `main` with a clear description

## Code Style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Use `slog` for structured logging
- Use `context.Context` as the first parameter in functions that do I/O
- Handle errors explicitly — do not ignore them
- Write table-driven tests where appropriate

## Project Structure

```
cmd/den/          — CLI entry point and commands
internal/
  api/            — HTTP handlers, middleware, WebSocket
  config/         — Configuration loading and validation
  engine/         — Core sandbox lifecycle management
  mcp/            — Model Context Protocol server
  runtime/docker/ — Docker runtime implementation
  storage/        — Volume, tmpfs, and S3 storage
  store/          — BoltDB persistence
  pathutil/       — Path validation utilities
pkg/client/       — Go SDK (public API)
sdk/
  typescript/     — TypeScript SDK
  python/         — Python SDK
```

## Reporting Issues

- Use [GitHub Issues](https://github.com/us/den/issues) for bug reports and feature requests
- Include Den version, OS, Docker version, and reproduction steps
- For security vulnerabilities, see [SECURITY.md](SECURITY.md)

## License

By contributing, you agree that your contributions will be licensed under the AGPL-3.0 License.
