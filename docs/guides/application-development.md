# Application Development Guide

Comprehensive guide to building blockchain applications with Blockberry's ABI v2.0.

## Table of Contents

- [Introduction](#introduction)
- [ABI v2.0 Overview](#abi-v20-overview)
- [Application Interface](#application-interface)
- [Transaction Lifecycle](#transaction-lifecycle)
- [State Management](#state-management)
- [Event Emission](#event-emission)
- [Validator Updates](#validator-updates)
- [Query Handling](#query-handling)
- [Testing](#testing)
- [Best Practices](#best-practices)

## Introduction

The Application Binary Interface (ABI) v2.0 is the contract between your blockchain application and the Blockberry framework. This guide provides comprehensive coverage of application development.

## ABI v2.0 Overview

ABI v2.0 uses a callback-based design where the framework calls your application at well-defined points in the block lifecycle.

**Key Principles:**

- Fail-closed security by default
- Explicit state management
- Event-driven architecture
- Type-safe interfaces

## Application Interface

The core `Application` interface defines all required methods. See [API Reference](../../API_REFERENCE.md#pkgabi) for complete details.

## Transaction Lifecycle

1. **CheckTx** - Mempool validation (concurrent-safe)
2. **BeginBlock** - Block initiation
3. **ExecuteTx** - Transaction execution (sequential)
4. **EndBlock** - Block finalization
5. **Commit** - State persistence

## State Management

Use `StateStore` for merkleized state:

```go
// Set values
app.state.Set([]byte("key"), []byte("value"))

// Commit changes
hash, version, err := app.state.Commit()

```text

## Best Practices

1. Use `BaseApplication` for fail-closed defaults
2. Always validate inputs in CheckTx
3. Keep ExecuteTx deterministic
4. Emit events for indexing
5. Test concurrent CheckTx calls
6. Profile performance with benchmarks

## Next Steps

- [State Management](../concepts/state-management.md)
- [ABI Concepts](../concepts/abi.md)
- [Testing Guide](testing.md)
