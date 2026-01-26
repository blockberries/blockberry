.PHONY: all build test test-short test-race test-integration bench lint fmt vet generate clean
.PHONY: coverage tidy deps verify check ci examples help

# Go parameters
GO := go
GOFLAGS := -v
TESTFLAGS := -race -coverprofile=coverage.out -covermode=atomic
BENCHFLAGS := -bench=. -benchmem -benchtime=3s

# Package paths
PKG := ./...

# Default target
all: fmt vet lint test build

## Build targets

build: ## Build all packages
	$(GO) build $(GOFLAGS) $(PKG)

## Test targets

test: ## Run tests (skips integration tests)
	$(GO) test -short $(PKG)

test-short: ## Run tests without race detection (faster)
	$(GO) test -short $(PKG)

test-race: ## Run tests with race detection
	$(GO) test -race -short -v $(PKG)

test-integration: ## Run integration tests
	$(GO) test -race -v -timeout 300s ./testing/

test-all: ## Run all tests including integration
	$(GO) test -race -v -timeout 300s $(PKG)

bench: ## Run benchmarks
	$(GO) test $(BENCHFLAGS) $(PKG)

coverage: ## Generate coverage report
	$(GO) test $(TESTFLAGS) $(PKG)
	$(GO) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

## Code quality targets

fmt: ## Format code
	$(GO) fmt $(PKG)
	@echo "Code formatted"

vet: ## Run go vet
	$(GO) vet $(PKG)

lint: ## Run golangci-lint
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run $(PKG); \
	else \
		echo "golangci-lint not installed, skipping..."; \
		echo "Install with: go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest"; \
	fi

## Generation targets

generate: ## Generate cramberry code from schema
	cramberry generate -lang go -out ./schema ./blockberry.cram

## Utility targets

clean: ## Clean build artifacts
	$(GO) clean -cache -testcache
	rm -f coverage.out coverage.html

tidy: ## Tidy go.mod
	$(GO) mod tidy

deps: ## Download dependencies
	$(GO) mod download

verify: ## Verify dependencies
	$(GO) mod verify

## Development helpers

check: fmt vet lint test-race ## Run all checks (format, vet, lint, test)

ci: ## Run CI pipeline locally
	@echo "Running CI pipeline..."
	$(MAKE) fmt
	$(MAKE) vet
	$(MAKE) test-race
	$(MAKE) build
	@echo "CI pipeline complete"

## Example targets

examples: ## Run example applications
	@echo "\n=== Simple Node Example ==="
	@$(GO) run ./examples/simple_node/
	@echo "\n=== Custom Mempool Example ==="
	@$(GO) run ./examples/custom_mempool/
	@echo "\n=== Mock Consensus Example ==="
	@$(GO) run ./examples/mock_consensus/

example-simple: ## Run simple node example
	@$(GO) run ./examples/simple_node/

example-mempool: ## Run custom mempool example
	@$(GO) run ./examples/custom_mempool/

example-consensus: ## Run mock consensus example
	@$(GO) run ./examples/mock_consensus/

## Help

help: ## Show this help
	@echo "Blockberry Makefile"
	@echo ""
	@echo "Usage: make [target]"
	@echo ""
	@echo "Targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2}'
