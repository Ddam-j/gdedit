# Development Setup

## Go Version

This project requires **Go 1.24.x** or later. The baseline is defined in `go.mod` using the `go` directive.

## Continuous Integration

The CI pipeline uses GitHub Actions with the official `actions/setup-go` action. The Go version is read automatically from the `go.mod` file:

```yaml
- uses: actions/setup-go@v5
  with:
    go-version-file: go.mod
```

This approach ensures that CI always uses the same Go version as the development environment, avoiding version drift between local development and automated builds.

## Getting Started

1. Install Go 1.24.x from [go.dev](https://go.dev/dl/)
2. Clone the repository
3. Run `go mod download` to fetch dependencies
4. Run `go build` to verify the setup
