# Installation Guide

This guide walks you through installing Blockberry and its dependencies.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installing as a Library](#installing-as-a-library)
- [Building from Source](#building-from-source)
- [Installing the CLI Tool](#installing-the-cli-tool)
- [Verifying Installation](#verifying-installation)
- [Troubleshooting](#troubleshooting)
- [Next Steps](#next-steps)

## Prerequisites

### System Requirements

- **Operating System**: Linux, macOS, or Windows (with WSL2)
- **CPU**: 2+ cores recommended
- **RAM**: 4GB minimum, 8GB+ recommended
- **Disk**: 10GB+ free space (varies by blockchain size)
- **Network**: Broadband internet connection

### Required Software

#### Go Programming Language

Blockberry requires **Go 1.25.6 or higher**.

**Check if Go is installed:**

```bash
go version

```text

If you see `go version go1.25.6` or higher, you're good to go.

**Install Go (if needed):**

Visit [https://go.dev/dl/](https://go.dev/dl/) and download the appropriate installer for your platform.

**Verify Go installation:**

```bash
go version
go env GOPATH

```text

Make sure `$GOPATH/bin` is in your `PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin

```text

Add this to your `~/.bashrc`, `~/.zshrc`, or equivalent to make it permanent.

#### Git

Git is required for cloning the repository.

```bash

# Check if Git is installed
git --version

# Install on Ubuntu/Debian
sudo apt-get install git

# Install on macOS (via Homebrew)
brew install git

# Install on Windows
# Download from https://git-scm.com/download/win

```text

### Optional Dependencies

These are optional but recommended for specific features:

#### LevelDB Development Headers

Required if you want to build with LevelDB support (default storage backend).

```bash

# Ubuntu/Debian
sudo apt-get install libleveldb-dev

# macOS (via Homebrew)
brew install leveldb

# Fedora/CentOS
sudo dnf install leveldb-devel

```text

Note: The Go LevelDB implementation (syndtr/goleveldb) is pure Go and doesn't require system LevelDB, but having the headers can improve performance.

#### BadgerDB

BadgerDB is a pure Go implementation and requires no system dependencies. It's installed automatically via Go modules.

## Installing as a Library

To use Blockberry as a library in your Go project:

```bash

# Create a new Go module (if you haven't already)
mkdir my-blockchain
cd my-blockchain
go mod init github.com/yourusername/my-blockchain

# Add Blockberry as a dependency
go get github.com/blockberries/blockberry@latest

```text

This will download Blockberry and all its dependencies into your Go module.

**Verify the installation:**

Create a simple `main.go`:

```go
package main

import (
    "fmt"
    "github.com/blockberries/blockberry/pkg/node"
)

func main() {
    fmt.Println("Blockberry is installed!")
    _ = node.Node{}  // Reference to ensure package is available
}

```text

Run it:

```bash
go run main.go

```text

You should see: `Blockberry is installed!`

## Building from Source

For development or to build from the latest source:

### Clone the Repository

```bash

# Clone the repository
git clone https://github.com/blockberries/blockberry.git
cd blockberry

# Checkout a specific version (optional)
git checkout v0.1.7

# Or use the latest main branch
git checkout main

```text

### Build All Packages

```bash

# Download dependencies
go mod download

# Build all packages
make build

# Or manually
go build ./...

```text

This will build all packages but not create any executables.

### Build the CLI Tool

```bash

# Build the blockberry CLI
make build-cli

# Or manually
go build -o build/blockberry ./cmd/blockberry

# The binary is now in build/blockberry
./build/blockberry version

```text

### Build Examples

```bash

# Build all examples
make examples

# Build specific example
go build -o build/simple_node ./examples/simple_node

```text

### Run Tests

Verify everything works by running the test suite:

```bash

# Run all tests
make test

# Run tests with race detection
make test-race

# Run tests with coverage
make coverage

```text

## Installing the CLI Tool

To install the `blockberry` command-line tool globally:

```bash

# Install from GitHub (latest release)
go install github.com/blockberries/blockberry/cmd/blockberry@latest

# Or install from local source
cd blockberry
go install ./cmd/blockberry

# Or install specific version
go install github.com/blockberries/blockberry/cmd/blockberry@v0.1.7

```text

The `blockberry` binary will be installed to `$GOPATH/bin` (typically `~/go/bin`).

**Make sure `$GOPATH/bin` is in your PATH:**

```bash

# Check if it's in PATH
echo $PATH | grep $(go env GOPATH)/bin

# If not, add it (choose your shell's config file)
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.bashrc
source ~/.bashrc

# Or for zsh
echo 'export PATH=$PATH:$(go env GOPATH)/bin' >> ~/.zshrc
source ~/.zshrc

```text

## Verifying Installation

### Verify the CLI Tool

```bash

# Check version
blockberry version

# Expected output:
# Blockberry v0.1.7
# Go version: go1.25.6
# Built: 2026-02-02

```text

### Verify Library Installation

Create a test program:

```go
package main

import (
    "fmt"
    "github.com/blockberries/blockberry/pkg/abi"
    "github.com/blockberries/blockberry/pkg/config"
    "github.com/blockberries/blockberry/pkg/node"
)

func main() {
    // Create default config
    cfg := config.DefaultConfig()
    cfg.Node.ChainID = "test-chain"

    // Create application
    app := abi.NewBaseApplication()

    // Validate we can create a node
    fmt.Println("✓ Blockberry packages imported successfully")
    fmt.Printf("✓ Can create config: %s\n", cfg.Node.ChainID)
    fmt.Printf("✓ Can create application: %s\n", app.Info().Name)

    // We won't actually start the node in this test
    _, err := node.NewNode(cfg)
    if err != nil {
        fmt.Printf("✓ Node creation works (expected error without proper config): %v\n", err)
    } else {
        fmt.Println("✓ Node creation successful")
    }
}

```text

Run it:

```bash
go run test.go

```text

### Run the Example Node

```bash

# Clone if you haven't already
git clone https://github.com/blockberries/blockberry.git
cd blockberry

# Run the simple example
go run ./examples/simple_node/main.go

# You should see the node start up
# Press Ctrl+C to stop

```text

## Troubleshooting

### Go Version Issues

**Problem**: `go: module requires Go 1.25.6`

**Solution**: Update Go to 1.25.6 or higher. Download from [https://go.dev/dl/](https://go.dev/dl/).

### Module Download Issues

**Problem**: `go: github.com/blockberries/blockberry@latest: module lookup failed`

**Solution**: Check your internet connection and Go proxy settings:

```bash

# Check current proxy setting
go env GOPROXY

# Set to default proxy if needed
go env -w GOPROXY=https://proxy.golang.org,direct

# Clear module cache and retry
go clean -modcache
go get github.com/blockberries/blockberry@latest

```text

### Build Errors

**Problem**: `undefined: some_package.SomeType`

**Solution**: Update dependencies:

```bash
go mod tidy
go mod download

```text

### CGO Errors (LevelDB)

**Problem**: `cgo: C compiler not found`

**Solution**: Install a C compiler:

```bash

# Ubuntu/Debian
sudo apt-get install build-essential

# macOS (install Xcode command line tools)
xcode-select --install

# Windows (use WSL2 or install MinGW)

```text

If you don't want to deal with CGO, you can use BadgerDB (pure Go):

Edit your config.toml:

```toml
[blockstore]
backend = "badgerdb"  # Instead of "leveldb"

```text

### Permission Issues

**Problem**: `permission denied` when running `blockberry`

**Solution**: Ensure the binary is executable:

```bash
chmod +x $(which blockberry)

# Or if built locally
chmod +x ./build/blockberry

```text

### Port Already in Use

**Problem**: `bind: address already in use`

**Solution**: Another process is using the port. Either:

1. Find and stop the other process:

   ```bash

   # Linux/macOS
   lsof -i :26656
   kill -9 <PID>

   ```text

2. Or change the port in your config:

   ```toml
   [network]
   listen_addrs = ["/ip4/0.0.0.0/tcp/26657"]  # Different port

   ```text

### Firewall Issues

**Problem**: Can't connect to peers

**Solution**: Open the P2P port in your firewall:

```bash

# Ubuntu/Debian (ufw)
sudo ufw allow 26656/tcp

# CentOS/Fedora (firewalld)
sudo firewall-cmd --permanent --add-port=26656/tcp
sudo firewall-cmd --reload

# macOS (built-in firewall should allow outbound)

```text

### Data Directory Permissions

**Problem**: `permission denied` writing to data directory

**Solution**: Ensure the data directory is writable:

```bash
mkdir -p data/{blockstore,state}
chmod -R 755 data/

```text

Or run with appropriate permissions:

```bash
sudo chown -R $USER:$USER data/

```text

## Next Steps

Now that you have Blockberry installed, you can:

1. **Quick Start**: Follow the [5-minute quickstart](quickstart.md) to get a node running
2. **Build an Application**: Learn to [build your first blockchain application](first-application.md)
3. **Configure Your Node**: Deep dive into [configuration options](configuration.md)
4. **Explore Examples**: Check out the [examples directory](../../examples/)
5. **Read the Architecture**: Understand the [system architecture](../concepts/architecture.md)

## Keeping Up to Date

### Update to Latest Version

```bash

# For library usage
go get -u github.com/blockberries/blockberry@latest

# For CLI tool
go install github.com/blockberries/blockberry/cmd/blockberry@latest

# Check version
blockberry version

```text

### Follow Releases

Watch the [GitHub repository](https://github.com/blockberries/blockberry) for new releases and security updates.

### Read the Changelog

Check [CHANGELOG.md](../../CHANGELOG.md) for version-specific changes, breaking changes, and migration guides.

---

**Installation complete!** Proceed to the [quickstart guide](quickstart.md) to run your first node.
