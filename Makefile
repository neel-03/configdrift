# build configuration
BINARY_NAME=configdrift
BUILD_DIR=bin
MAIN_PACKAGE=./cmd/configdrift

# versioning
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT=$(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME=$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

# go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOMOD=$(GOCMD) mod
LDFLAGS=-ldflags "-X github.com/neel-03/configdrift/internal/version.Version=$(VERSION) -X github.com/neel-03/configdrift/internal/version.Commit=$(COMMIT) -X github.com/neel-03/configdrift/internal/version.BuildTime=$(BUILD_TIME)"

# tools
GOLINT=$(shell which golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)

.PHONY: all
all: build

.PHONY: help
help: ## show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}'

.PHONY: setup
setup: ## setup dev env (tidy modules and install hooks)
	@echo "Tidying modules..."
	$(GOMOD) tidy
	@echo "Setting up git hooks..."
	@chmod +x scripts/setup-hooks.sh
	@./scripts/setup-hooks.sh

.PHONY: build
build: ## build binary
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p $(BUILD_DIR)
	$(GOBUILD) $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PACKAGE)

.PHONY: test
test: ## run unit tests
	@echo "Running tests..."
	$(GOTEST) -v ./...

.PHONY: coverage
coverage: ## run tests with coverage report
	@echo "Running tests with coverage..."
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -func=coverage.out

.PHONY: lint
lint: ## run linter
	@echo "Running linter..."
	$(GOLINT) run

.PHONY: fmt
fmt: ## format source code
	@echo "Formatting code..."
	$(GOCMD) fmt ./...

.PHONY: clean
clean: ## remove build artifacts
	@echo "Cleaning up..."
	$(GOCLEAN)
	rm -rf $(BUILD_DIR)
	rm -f coverage.out

.PHONY: run
run: build ## build and run the binary
	$(BUILD_DIR)/$(BINARY_NAME) $(ARGS)

.PHONY: go-run
go-run: ## run the application using go run
	$(GOCMD) run $(MAIN_PACKAGE) $(ARGS)

# Development environment
.PHONY: dev
dev: setup build run ## setup, build, and run the application for development
	@echo "Development environment ready. Application is running."
	# The 'run' target already executes the binary. This target orchestrates the setup.
