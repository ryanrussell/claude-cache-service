.PHONY: all build test clean lint fmt pre-commit install-hooks run docker-build docker-run

# Variables
BINARY_NAME=claude-cache-service
DOCKER_IMAGE=claude-cache-service:latest
GO_FILES=$(shell find . -name '*.go' -type f)

# Default target
all: lint test build

# Build the binary
build:
	@echo "Building $(BINARY_NAME)..."
	@go build -ldflags="-s -w" -o $(BINARY_NAME) cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -func=coverage.out

# Run tests with coverage report
test-coverage:
	@echo "Running tests with coverage..."
	@go test -v -race -coverprofile=coverage.out ./...
	@go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -f $(BINARY_NAME)
	@rm -f coverage.out coverage.html
	@rm -rf dist/

# Format code
fmt:
	@echo "Formatting code..."
	@gofmt -l -w $(GO_FILES)
	@goimports -w $(GO_FILES)

# Run linter
lint: fmt
	@echo "Running linter..."
	@golangci-lint run

# Run pre-commit checks
pre-commit:
	@echo "Running pre-commit checks..."
	@pre-commit run --all-files

# Install git hooks
install-hooks:
	@echo "Installing git hooks..."
	@pre-commit install
	@pre-commit install --hook-type commit-msg
	@echo "Git hooks installed successfully!"

# Run the server
run: build
	@echo "Starting $(BINARY_NAME)..."
	@./$(BINARY_NAME)

# Development mode with hot reload (requires air)
dev:
	@which air > /dev/null || (echo "Installing air..." && go install github.com/cosmtrek/air@latest)
	@air

# Docker build
docker-build:
	@echo "Building Docker image..."
	@docker build -t $(DOCKER_IMAGE) .

# Docker run
docker-run: docker-build
	@echo "Running Docker container..."
	@docker run --rm -p 8080:8080 -v $$(pwd)/cache:/app/cache $(DOCKER_IMAGE)

# Install dependencies
deps:
	@echo "Installing dependencies..."
	@go mod download
	@go mod tidy

# Check for security vulnerabilities
security:
	@echo "Checking for security vulnerabilities..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securego/gosec/v2/cmd/gosec@latest)
	@gosec ./...

# Generate API documentation
docs:
	@echo "Generating API documentation..."
	@which swag > /dev/null || (echo "Installing swag..." && go install github.com/swaggo/swag/cmd/swag@latest)
	@swag init -g cmd/server/main.go

# Full check before committing
check: fmt lint test security
	@echo "All checks passed!"

# Show help
help:
	@echo "Available targets:"
	@echo "  make build          - Build the binary"
	@echo "  make test           - Run tests"
	@echo "  make test-coverage  - Run tests with coverage report"
	@echo "  make clean          - Clean build artifacts"
	@echo "  make fmt            - Format code"
	@echo "  make lint           - Run linter"
	@echo "  make pre-commit     - Run pre-commit checks"
	@echo "  make install-hooks  - Install git hooks"
	@echo "  make run            - Build and run the server"
	@echo "  make dev            - Run in development mode with hot reload"
	@echo "  make docker-build   - Build Docker image"
	@echo "  make docker-run     - Run Docker container"
	@echo "  make deps           - Install dependencies"
	@echo "  make security       - Check for security vulnerabilities"
	@echo "  make docs           - Generate API documentation"
	@echo "  make check          - Run all checks (fmt, lint, test, security)"
	@echo "  make help           - Show this help message"