.PHONY: all build test test-race lint generate clean fmt vet

# Default target
all: build

# Build all packages
build:
	go build ./...

# Run unit tests (skips integration tests)
test:
	go test -short ./...

# Run tests with race detection (skips integration tests)
test-race:
	go test -race -short -v ./...

# Run integration tests
test-integration:
	go test -race -v -timeout 300s ./testing/

# Run linter
lint:
	golangci-lint run ./...

# Format code
fmt:
	go fmt ./...

# Run go vet
vet:
	go vet ./...

# Generate cramberry code from schema
generate:
	cramberry generate -lang go -out ./schema ./blockberry.cram

# Clean build artifacts
clean:
	go clean ./...
	rm -f coverage.out coverage.html

# Run tests with coverage
coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Tidy dependencies
tidy:
	go mod tidy

# Check everything (format, vet, lint, test)
check: fmt vet lint test-race
