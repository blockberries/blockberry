# Blockberry Documentation Hub

Welcome to the Blockberry documentation! This directory contains comprehensive guides, tutorials, and reference materials for building blockchain applications with Blockberry.

## Quick Navigation

| Section | Description | Start Here |
|---------|-------------|------------|
| **[Getting Started](getting-started/)** | Installation, quickstart, and basic configuration | [Installation Guide](getting-started/installation.md) |
| **[Tutorials](tutorials/)** | Step-by-step guides for common tasks | [Custom Mempool](tutorials/custom-mempool.md) |
| **[Guides](guides/)** | In-depth development guides | [Application Development](guides/application-development.md) |
| **[Concepts](concepts/)** | Core concepts and architecture | [Architecture Overview](concepts/architecture.md) |
| **[Reference](reference/)** | Technical reference documentation | [Configuration Reference](reference/configuration.md) |
| **[Examples](examples/)** | Complete code examples | [Simple Node](examples/simple-node.md) |

## Documentation Index

### Getting Started

New to Blockberry? Start here.

- **[Installation Guide](getting-started/installation.md)** - Install Blockberry and dependencies
- **[5-Minute Quickstart](getting-started/quickstart.md)** - Get a node running quickly
- **[First Application](getting-started/first-application.md)** - Build your first blockchain app
- **[Configuration Guide](getting-started/configuration.md)** - Understanding configuration options

### Tutorials

Learn by doing with step-by-step tutorials.

- **[Building a Custom Mempool](tutorials/custom-mempool.md)** - Implement custom transaction ordering logic
- **[Integrating Consensus](tutorials/consensus-integration.md)** - Add your consensus engine
- **[Custom Storage Backend](tutorials/storage-backend.md)** - Implement alternative storage
- **[Transaction Lifecycle](tutorials/transaction-lifecycle.md)** - Deep dive into transaction flow

### Developer Guides

Comprehensive guides for building production applications.

- **[Application Development](guides/application-development.md)** - Complete ABI v2.0 development guide
- **[Consensus Engines](guides/consensus-engines.md)** - Implementing consensus protocols
- **[Testing Strategies](guides/testing.md)** - Unit, integration, and benchmark testing
- **[Performance Benchmarking](guides/benchmarking.md)** - Measuring and optimizing performance
- **[Deployment Guide](guides/deployment.md)** - Production deployment best practices
- **[Monitoring & Observability](guides/monitoring.md)** - Metrics, tracing, and logging setup
- **[Security Best Practices](guides/security.md)** - Security hardening guide
- **[Contributing Guide](guides/contributing.md)** - How to contribute to Blockberry

### Core Concepts

Understand the architecture and design philosophy.

- **[Architecture Overview](concepts/architecture.md)** - System architecture and components
- **[Application Binary Interface](concepts/abi.md)** - ABI v2.0 design philosophy
- **[Consensus Layer](concepts/consensus.md)** - Pluggable consensus architecture
- **[Mempool Implementations](concepts/mempool.md)** - Transaction pool strategies
- **[P2P Networking](concepts/p2p-networking.md)** - Secure networking with Glueberry
- **[Storage Layer](concepts/storage.md)** - Block and state storage
- **[State Management](concepts/state-management.md)** - IAVL merkle trees
- **[Node Roles](concepts/node-roles.md)** - Validator, full, seed, light, archive

### Technical Reference

Detailed technical documentation.

- **[CLI Reference](reference/cli.md)** - Command-line interface documentation
- **[Configuration Reference](reference/configuration.md)** - Complete configuration options
- **[RPC API Reference](reference/rpc-api.md)** - gRPC, JSON-RPC, WebSocket APIs
- **[Message Protocol](reference/message-protocol.md)** - P2P message specifications
- **[Error Codes](reference/error-codes.md)** - Error codes and troubleshooting
- **[Glossary](reference/glossary.md)** - Terminology and definitions

### Code Examples

Complete, runnable examples.

- **[Simple Node](examples/simple-node.md)** - Basic node setup example
- **[Custom Mempool](examples/custom-mempool.md)** - Priority-based mempool
- **[Consensus Engine](examples/consensus-engine.md)** - Mock consensus implementation
- **[Full Application](examples/full-application.md)** - Complete blockchain application

## Documentation Structure

```text
docs/
├── index.md                    # Documentation home
├── README.md                   # This file (hub)
├── getting-started/            # Getting started guides
│   ├── installation.md
│   ├── quickstart.md
│   ├── first-application.md
│   └── configuration.md
├── tutorials/                  # Step-by-step tutorials
│   ├── custom-mempool.md
│   ├── consensus-integration.md
│   ├── storage-backend.md
│   └── transaction-lifecycle.md
├── guides/                     # Developer guides
│   ├── application-development.md
│   ├── consensus-engines.md
│   ├── testing.md
│   ├── benchmarking.md
│   ├── deployment.md
│   ├── monitoring.md
│   ├── security.md
│   └── contributing.md
├── concepts/                   # Core concepts
│   ├── architecture.md
│   ├── abi.md
│   ├── consensus.md
│   ├── mempool.md
│   ├── p2p-networking.md
│   ├── storage.md
│   ├── state-management.md
│   └── node-roles.md
├── reference/                  # Reference documentation
│   ├── cli.md
│   ├── configuration.md
│   ├── rpc-api.md
│   ├── message-protocol.md
│   ├── error-codes.md
│   └── glossary.md
└── examples/                   # Code examples
    ├── simple-node.md
    ├── custom-mempool.md
    ├── consensus-engine.md
    └── full-application.md

```text

## Additional Resources

### Primary Documentation

- **[Main README](../README.md)** - Project overview and features
- **[Architecture Documentation](../ARCHITECTURE.md)** - Detailed architecture analysis
- **[API Reference](../API_REFERENCE.md)** - Complete API documentation
- **[ABI Design](../ABI_DESIGN.md)** - Application Binary Interface specification
- **[Changelog](../CHANGELOG.md)** - Version history and release notes

### External Links

- **[GitHub Repository](https://github.com/blockberries/blockberry)** - Source code
- **[pkg.go.dev](https://pkg.go.dev/github.com/blockberries/blockberry)** - Go package documentation
- **[GitHub Issues](https://github.com/blockberries/blockberry/issues)** - Bug reports and feature requests
- **[GitHub Discussions](https://github.com/blockberries/blockberry/discussions)** - Community Q&A

### Related Projects

- **[Glueberry](https://github.com/blockberries/glueberry)** - P2P networking library
- **[Cramberry](https://github.com/blockberries/cramberry)** - Binary serialization library
- **[Looseberry](https://github.com/blockberries/looseberry)** - DAG mempool (optional)

## Learning Paths

### For Beginners

1. Read the [Installation Guide](getting-started/installation.md)
2. Follow the [5-Minute Quickstart](getting-started/quickstart.md)
3. Build [Your First Application](getting-started/first-application.md)
4. Explore the [Simple Node Example](examples/simple-node.md)
5. Understand the [Architecture](concepts/architecture.md)

### For Application Developers

1. Review the [ABI Specification](concepts/abi.md)
2. Study the [Application Development Guide](guides/application-development.md)
3. Learn about [State Management](concepts/state-management.md)
4. Follow the [Full Application Example](examples/full-application.md)
5. Read [Testing Strategies](guides/testing.md)

### For Infrastructure Engineers

1. Understand [P2P Networking](concepts/p2p-networking.md)
2. Learn [Deployment Best Practices](guides/deployment.md)
3. Configure [Monitoring & Observability](guides/monitoring.md)
4. Review [Security Best Practices](guides/security.md)
5. Study [Node Roles](concepts/node-roles.md)

### For Consensus Developers

1. Understand the [Consensus Layer](concepts/consensus.md)
2. Follow the [Consensus Integration Tutorial](tutorials/consensus-integration.md)
3. Read the [Consensus Engines Guide](guides/consensus-engines.md)
4. Examine the [Consensus Engine Example](examples/consensus-engine.md)
5. Review the [Message Protocol](reference/message-protocol.md)

## Contributing to Documentation

Found an error? Want to improve the docs? See our [Contributing Guide](guides/contributing.md).

Documentation contributions are welcome:

- Fix typos and errors
- Add missing examples
- Improve clarity
- Add diagrams
- Expand tutorials

## Version Information

- **Documentation Version**: Matches Blockberry v0.1.7
- **Last Updated**: 2026-02-02
- **Go Version**: 1.25.6+

## Getting Help

- **Questions**: Use [GitHub Discussions](https://github.com/blockberries/blockberry/discussions)
- **Bugs**: Report on [GitHub Issues](https://github.com/blockberries/blockberry/issues)
- **Security**: Email security issues to the maintainers privately

## License

Documentation is licensed under [Apache 2.0](../LICENSE), same as the project.

---

**Ready to start?** Head to the [installation guide](getting-started/installation.md) or jump straight to the [quickstart](getting-started/quickstart.md)!
