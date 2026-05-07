# configdrift

A tool to detect drift between canonical configuration sources and live environments.

> #### ⚠️ **[Important]**
> This project is currently in the **active development phase**. Configuration schemas, and features are subject to frequent changes and may not be stable for production use.

## Getting Started

### Prerequisites
- Go 1.26+
- `golangci-lint` (for linting)

### Setup
We use a `Makefile` to manage the development environment. To set up your local environment (including Git hooks):
```bash
make setup
```

To see all available commands, run:
```bash
make help
```

## Development

### Building
Build the binary with injected version metadata:
```bash
make build
./bin/configdrift --version
```

### Testing
Run the test suite:
```bash
make test
```

Generate coverage report:
```bash
make coverage
```

### Linting & Formatting
```bash
make lint
make fmt
```

## Architecture
- `cmd/configdrift`: Entry point.
- `internal/config`: Configuration parsing and policy management.
- `internal/source`: Adapters for canonical sources (Git, S3, Local).
- `internal/target`: Adapters for live targets (Docker, K8s, SSH).
- `internal/diff`: Core logic for calculating configuration drift.
- `internal/version`: Build-time metadata.
