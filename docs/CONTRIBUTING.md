# Contributing to Blockberry

Thank you for your interest in contributing to Blockberry! This document provides guidelines and instructions for contributing.

## Table of Contents

- [Code of Conduct](#code-of-conduct)
- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Code Style](#code-style)
- [Pull Request Process](#pull-request-process)
- [Issue Guidelines](#issue-guidelines)
- [Release Process](#release-process)

## Code of Conduct

Please be respectful and constructive in all interactions. We welcome contributors of all backgrounds and experience levels.

## Getting Started

### Prerequisites

- Go 1.22 or later
- Make
- Git
- LevelDB (for block storage)

### Fork and Clone

1. Fork the repository on GitHub
2. Clone your fork:
   ```bash
   git clone https://github.com/YOUR-USERNAME/blockberry.git
   cd blockberry
   ```
3. Add upstream remote:
   ```bash
   git remote add upstream https://github.com/blockberries/blockberry.git
   ```

## Development Setup

### Initial Setup

```bash
# Install Go dependencies
go mod download

# Verify setup
make check
```

### Project Structure

```
blockberry/
├── node/          # Main Node coordinator and lifecycle
├── mempool/       # Transaction pool implementations (simple, priority, TTL)
├── blockstore/    # Block storage interface and LevelDB implementation
├── statestore/    # IAVL-based merkleized state storage
├── pex/           # Peer exchange protocol
├── sync/          # Block synchronization reactor
├── handlers/      # Message handlers (handshake, transactions, blocks)
├── p2p/           # Peer management, scoring, and rate limiting
├── metrics/       # Prometheus metrics collection
├── logging/       # Structured logging with slog
├── types/         # Common types, errors, and validation
├── config/        # Configuration loading and validation
├── schema/        # Generated cramberry message types
├── testing/       # Integration test helpers
└── examples/      # Example implementations
```

### Available Make Targets

```bash
make help           # Show all available targets
make build          # Build all packages
make test           # Run tests (skips integration)
make test-race      # Run tests with race detection
make test-integration  # Run integration tests
make lint           # Run golangci-lint
make check          # Run all checks (format, vet, lint, test)
make coverage       # Generate coverage report
make bench          # Run benchmarks
make clean          # Clean build artifacts
```

## Making Changes

### Branch Naming

Use descriptive branch names:
- `feature/add-state-sync`
- `fix/mempool-eviction`
- `docs/update-api-reference`
- `refactor/simplify-handshake`

### Commit Messages

Follow conventional commit format:

```
type(scope): short description

Longer description if needed. Explain the motivation
for the change and contrast with previous behavior.

Fixes #123
```

**Types:**
- `feat`: New feature
- `fix`: Bug fix
- `docs`: Documentation only
- `style`: Code style (formatting, no logic change)
- `refactor`: Code refactoring
- `perf`: Performance improvement
- `test`: Adding or updating tests
- `chore`: Maintenance tasks

**Examples:**
```
feat(mempool): add TTL-based transaction expiration

Add TTLMempool implementation that automatically removes
expired transactions. Configurable default TTL and
per-transaction TTL support via AddTxWithTTL.

Fixes #45
```

```
fix(sync): handle pruned peer gracefully

Peers that have pruned old blocks now return appropriate
error instead of crashing when syncing node requests
blocks below their base height.

Fixes #78
```

## Testing

### Running Tests

```bash
# Run all unit tests
make test

# Run with race detection
make test-race

# Run integration tests
make test-integration

# Run specific package tests
go test -v ./mempool/...

# Run specific test
go test -v ./mempool -run TestPriorityMempool

# Run with coverage
make coverage
```

### Writing Tests

1. **Test file naming**: `*_test.go` in the same package
2. **Test function naming**: `TestFunctionName`, `TestType_Method`
3. **Table-driven tests** for multiple cases
4. **Benchmark naming**: `BenchmarkFunctionName`

```go
func TestMempool_AddTx(t *testing.T) {
    tests := []struct {
        name    string
        setup   func() *SimpleMempool
        tx      []byte
        wantErr error
    }{
        {
            name: "add valid transaction",
            setup: func() *SimpleMempool {
                return NewSimpleMempool(SimpleMempoolConfig{MaxTxs: 100})
            },
            tx:      []byte("valid tx"),
            wantErr: nil,
        },
        {
            name: "mempool full",
            setup: func() *SimpleMempool {
                mp := NewSimpleMempool(SimpleMempoolConfig{MaxTxs: 1})
                mp.AddTx([]byte("existing"))
                return mp
            },
            tx:      []byte("new tx"),
            wantErr: types.ErrMempoolFull,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mp := tt.setup()
            err := mp.AddTx(tt.tx)
            if !errors.Is(err, tt.wantErr) {
                t.Errorf("AddTx() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}
```

### Test Coverage

Aim for high coverage on:
- Public APIs (AddTx, ReapTxs, etc.)
- Edge cases (nil, empty, max values)
- Error paths
- Concurrent access (race detection)
- Message encoding/decoding

### Integration Tests

Integration tests verify multi-node scenarios:

```bash
# Run 10-node PEX integration test
make test-integration
```

## Code Style

### Go Code Style

- Follow [Effective Go](https://golang.org/doc/effective_go)
- Use `gofmt` (enforced by CI)
- Use `golangci-lint` (run `make lint`)

**Key points:**
- Export only necessary symbols
- Document all exported types/functions
- Handle all errors
- Avoid global state
- Use context for cancellation
- Use RWMutex for thread-safe maps

### Naming Conventions

```go
// Public types: PascalCase
type PriorityMempool struct {}

// Public functions: PascalCase
func NewPriorityMempool(cfg PriorityMempoolConfig) *PriorityMempool

// Private types/functions: camelCase
type mempoolTx struct {}
func (m *PriorityMempool) evictLowest() bool

// Constants: PascalCase for public, camelCase for private
const MaxKnownTxsPerPeer = 20000
const defaultCleanupInterval = 5 * time.Minute
```

### Error Handling

```go
// Use sentinel errors for known conditions
var ErrMempoolFull = errors.New("mempool is full")

// Wrap errors with context
return fmt.Errorf("add transaction: %w", err)

// Use typed errors for rich context
return &ValidationError{
    Field:  "height",
    Value:  height,
    Reason: "must be positive",
}
```

### Documentation

```go
// Package mempool provides transaction pool implementations.
package mempool

// PriorityMempool is a mempool that orders transactions by priority.
// Transactions with higher priority are returned first by ReapTxs.
// When the mempool is full, lowest priority transactions are evicted.
//
// The priority function is configurable via SetPriorityFunc.
// Default priority is based on transaction size (smaller = higher priority).
type PriorityMempool struct {
    // ...
}
```

## Pull Request Process

### Before Submitting

1. **Sync with upstream**:
   ```bash
   git fetch upstream
   git rebase upstream/main
   ```

2. **Run all checks**:
   ```bash
   make check
   ```

3. **Update documentation** if needed

4. **Add tests** for new functionality

### PR Description Template

```markdown
## Summary
Brief description of changes.

## Motivation
Why is this change needed?

## Changes
- Change 1
- Change 2

## Testing
How was this tested?

## Breaking Changes
List any breaking changes (if applicable).

## Related Issues
Fixes #123
Related to #456
```

### Review Process

1. PRs require at least one approval
2. CI must pass (tests, lint, build)
3. Address all review comments
4. Squash commits if requested
5. Maintainer will merge when ready

### After Merge

- Delete your feature branch
- Update your local main:
  ```bash
  git checkout main
  git pull upstream main
  ```

## Issue Guidelines

### Bug Reports

Include:
- Blockberry version
- Go version (`go version`)
- Operating system
- Minimal reproduction case
- Expected vs actual behavior
- Stack trace (if applicable)

### Feature Requests

Include:
- Use case description
- Proposed API (if applicable)
- Alternative approaches considered
- Impact on existing functionality

### Labels

- `bug`: Something isn't working
- `enhancement`: New feature or request
- `documentation`: Documentation improvements
- `good first issue`: Good for newcomers
- `help wanted`: Extra attention needed
- `priority:high/medium/low`: Priority level
- `breaking`: Breaking change

## Release Process

### Version Numbering

We use [Semantic Versioning](https://semver.org/):
- **MAJOR**: Breaking changes
- **MINOR**: New features (backward compatible)
- **PATCH**: Bug fixes (backward compatible)

### Release Checklist

1. Update CHANGELOG.md
2. Update version references
3. Run full test suite
4. Create release tag
5. Update documentation

### Changelog Format

```markdown
## [1.1.0] - 2026-01-26

### Added
- TTL-based mempool with automatic transaction expiration

### Changed
- Improved peer scoring algorithm

### Fixed
- Memory leak in peer state tracking (#78)

### Deprecated
- V1 sync protocol (use V2)

### Security
- Rate limiting prevents DoS attacks
```

## Getting Help

- **Questions**: Open a GitHub Discussion
- **Bugs**: Open a GitHub Issue
- **Security**: See SECURITY.md for reporting vulnerabilities

## Recognition

Contributors are recognized in:
- CHANGELOG.md (for the release)
- GitHub contributors list

Thank you for contributing to Blockberry!
