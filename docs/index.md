# Blockberry Documentation

Welcome to the comprehensive documentation for Blockberry, a modular blockchain node framework for Go.

## What is Blockberry?

Blockberry is a production-ready blockchain framework that provides the infrastructure layer for building custom blockchain networks. It is **not** a consensus engine or smart contract platform—instead, it offers composable building blocks that allow you to:

- Implement custom consensus algorithms (BFT, DAG, PoS, or hybrid approaches)
- Define application-specific transaction validation and execution logic
- Choose storage backends optimized for your use case
- Deploy various node roles (validators, full nodes, seeds, light clients, archives)
- Monitor and observe your network with comprehensive metrics and tracing

## Quick Links

- **New to Blockberry?** Start with the [5-minute quickstart](getting-started/quickstart.md)
- **Building an application?** See the [application development guide](guides/application-development.md)
- **Need API docs?** Check the [API Reference](../API_REFERENCE.md)
- **Understanding architecture?** Read the [architecture overview](concepts/architecture.md)

## Documentation Structure

### Getting Started

Perfect for newcomers who want to set up and run their first Blockberry node.

- [Installation Guide](getting-started/installation.md) - Prerequisites and installation steps
- [Quickstart](getting-started/quickstart.md) - Get a node running in 5 minutes
- [First Application](getting-started/first-application.md) - Build your first blockchain application
- [Configuration](getting-started/configuration.md) - Understanding and customizing configuration

### Tutorials

Step-by-step guides for common tasks and integrations.

- [Building a Custom Mempool](tutorials/custom-mempool.md) - Implement custom transaction ordering
- [Integrating Consensus](tutorials/consensus-integration.md) - Add your consensus engine
- [Custom Storage Backend](tutorials/storage-backend.md) - Implement alternative storage
- [Transaction Lifecycle](tutorials/transaction-lifecycle.md) - Deep dive into transaction flow

### Guides

In-depth guides for developers building production applications.

- [Application Development](guides/application-development.md) - Complete ABI v2.0 guide
- [Consensus Engines](guides/consensus-engines.md) - Implementing consensus protocols
- [Testing Strategies](guides/testing.md) - Unit, integration, and benchmark testing
- [Performance Benchmarking](guides/benchmarking.md) - Measuring and optimizing performance
- [Deployment](guides/deployment.md) - Production deployment best practices
- [Monitoring & Observability](guides/monitoring.md) - Metrics, tracing, and logging
- [Security Best Practices](guides/security.md) - Security hardening guide
- [Contributing](guides/contributing.md) - How to contribute to Blockberry

### Concepts

Understand the core concepts and design decisions behind Blockberry.

- [Architecture Overview](concepts/architecture.md) - System architecture and components
- [Application Binary Interface](concepts/abi.md) - ABI v2.0 design and philosophy
- [Consensus Layer](concepts/consensus.md) - Pluggable consensus architecture
- [Mempool Implementations](concepts/mempool.md) - Transaction pool strategies
- [P2P Networking](concepts/p2p-networking.md) - Secure networking layer
- [Storage Layer](concepts/storage.md) - Block and state storage
- [State Management](concepts/state-management.md) - IAVL merkle trees
- [Node Roles](concepts/node-roles.md) - Validator, full, seed, light, archive nodes

### Reference

Technical reference documentation.

- [CLI Reference](reference/cli.md) - Command-line interface documentation
- [Configuration Reference](reference/configuration.md) - Complete configuration options
- [RPC API Reference](reference/rpc-api.md) - gRPC, JSON-RPC, and WebSocket APIs
- [Message Protocol](reference/message-protocol.md) - P2P message specifications
- [Error Codes](reference/error-codes.md) - Error codes and troubleshooting
- [Glossary](reference/glossary.md) - Terminology and definitions

### Examples

Complete, runnable code examples.

- [Simple Node](examples/simple-node.md) - Basic node setup
- [Custom Mempool](examples/custom-mempool.md) - Priority-based mempool example
- [Consensus Engine](examples/consensus-engine.md) - Mock consensus implementation
- [Full Application](examples/full-application.md) - Complete blockchain application

## Additional Resources

- [GitHub Repository](https://github.com/blockberries/blockberry)
- [API Documentation](https://pkg.go.dev/github.com/blockberries/blockberry)
- [Architecture Analysis](../ARCHITECTURE.md)
- [Changelog](../CHANGELOG.md)

## Getting Help

- **Issues**: Report bugs or request features on [GitHub Issues](https://github.com/blockberries/blockberry/issues)
- **Discussions**: Ask questions on [GitHub Discussions](https://github.com/blockberries/blockberry/discussions)
- **Security**: Report security vulnerabilities privately to the maintainers

## Documentation Conventions

Throughout this documentation, you'll see these conventions:

- **Code blocks** show Go code examples
- **Console blocks** show command-line operations
- **Note blocks** highlight important information
- **Warning blocks** indicate potential pitfalls

## Version Information

This documentation corresponds to:

- **Blockberry Version**: v0.1.7
- **Go Version**: 1.25.6+
- **Last Updated**: 2026-02-02

## License

Blockberry is licensed under the Apache License 2.0. See the [LICENSE](../LICENSE) file for details.

---

**Ready to get started?** Head to the [installation guide](getting-started/installation.md) or jump straight to the [quickstart](getting-started/quickstart.md).
