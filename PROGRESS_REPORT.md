# Blockberry Implementation Progress Report

This document tracks the implementation progress of the MASTER_PLAN.md phases.

---

## Phase 0: Critical Fixes

### 0.1 Mandatory Block Validation
**Status:** Complete

**Files Modified:**
- `types/errors.go` - Added `ErrNoBlockValidator` and `ErrNoTxValidator` errors
- `sync/reactor.go` - Added `DefaultBlockValidator` (fail-closed) and `AcceptAllBlockValidator` (testing); modified `Start()` to require validator; modified `handleBlocksResponse()` to always use validator
- `sync/reactor_test.go` - Added tests for mandatory validation behavior
- `node/node.go` - Added `WithBlockValidator` option
- `testing/helpers.go` - Set default validator for test nodes
- `examples/simple_node/main.go` - Updated to use `WithBlockValidator`
- `examples/custom_mempool/main.go` - Updated to use `WithBlockValidator`
- `examples/mock_consensus/main.go` - Updated to use `WithBlockValidator`

**Key Changes:**
1. **Fail-Closed Block Validation**: Blocks are now rejected by default if no validator is configured. The `DefaultBlockValidator` returns an error for all blocks, ensuring applications must explicitly provide their own validation logic.

2. **Start() Enforcement**: `SyncReactor.Start()` now returns `ErrNoBlockValidator` if no validator has been set, preventing the node from starting without proper validation.

3. **handleBlocksResponse() Always Validates**: Even if no validator is explicitly set, the code uses `DefaultBlockValidator` to reject blocks, ensuring fail-closed behavior throughout.

4. **WithBlockValidator Option**: Added `node.WithBlockValidator()` option to allow applications to configure block validation when creating a node.

5. **Testing Support**: Added `AcceptAllBlockValidator` for testing scenarios where block validation is not the focus of the test.

**Test Coverage:**
- `TestSyncReactor_StartRequiresValidator` - Verifies Start() fails without validator
- `TestSyncReactor_HandleBlocksResponseWithoutValidator` - Verifies blocks rejected without validator
- `TestDefaultBlockValidator` - Verifies default validator rejects all
- `TestAcceptAllBlockValidator` - Verifies test validator accepts all

**Design Decisions:**
- Used fail-closed approach: without explicit configuration, blocks are rejected
- Separated concerns: validation logic is provided by the application, not hardcoded
- Maintained backward compatibility for tests via `AcceptAllBlockValidator`

---

### 0.2 Mandatory Transaction Validation
**Status:** Complete

**Files Modified:**
- `mempool/mempool.go` - Added `TxValidator` type, `DefaultTxValidator` (fail-closed), `AcceptAllTxValidator` (testing); added `SetTxValidator()` to `Mempool` interface
- `mempool/simple_mempool.go` - Added validator field and `SetTxValidator()` method; updated `AddTx()` to use validator
- `mempool/priority_mempool.go` - Added validator field and `SetTxValidator()` method; updated `AddTx()` to use validator
- `mempool/ttl_mempool.go` - Added validator field and `SetTxValidator()` method; updated `AddTxWithTTL()` to use validator
- `mempool/mempool_test.go` - Updated all tests to set validators
- `mempool/priority_mempool_test.go` - Updated all tests to set validators
- `mempool/ttl_mempool_test.go` - Updated all tests to set validators
- `handlers/transactions_test.go` - Updated tests to set validators on mempools
- `examples/custom_mempool/main.go` - Added `SetTxValidator()` to custom `PriorityMempool` implementation

**Key Changes:**
1. **Fail-Closed Transaction Validation**: Transactions are now rejected by default if no validator is configured. The `DefaultTxValidator` returns `ErrNoTxValidator` for all transactions, ensuring applications must explicitly provide validation logic.

2. **TxValidator Interface**: Added `TxValidator` function type to mempool package:
   ```go
   type TxValidator func(tx []byte) error
   ```

3. **SetTxValidator Method**: Added to `Mempool` interface, allowing applications to configure validation after mempool creation.

4. **All Mempool Implementations Updated**: `SimpleMempool`, `PriorityMempool`, and `TTLMempool` all implement the new validation pattern.

5. **Testing Support**: Added `AcceptAllTxValidator` for testing scenarios where transaction validation is not the focus of the test.

**Test Coverage:**
- `TestDefaultTxValidator` - Verifies default validator rejects all transactions
- `TestAcceptAllTxValidator` - Verifies test validator accepts all transactions
- All existing mempool tests updated to explicitly set validators

**Design Decisions:**
- Used fail-closed approach: without explicit configuration, transactions are rejected
- Consistent with block validation pattern from Phase 0.1
- Validation occurs early in `AddTx()` before capacity checks
- Error wrapping preserves original validation errors while indicating the context

---

### 0.3 Block Height Continuity Check
**Status:** Complete

**Files Modified:**
- `types/errors.go` - Added `ErrNonContiguousBlock` error
- `sync/reactor.go` - Modified `handleBlocksResponse()` to enforce block contiguity
- `sync/reactor_test.go` - Added tests for contiguity validation

**Key Changes:**
1. **Block Contiguity Validation**: Before storing any blocks from a response, the code now verifies that blocks are contiguous and in order. This prevents gaps in the block chain.

2. **Expected Height Enforcement**: Blocks must start at `blockStore.Height() + 1`. If a peer sends blocks starting at a higher height (skipping blocks), the response is rejected.

3. **Peer Penalization**: Peers who send non-contiguous blocks are penalized with `PenaltyProtocolViolation` (50 points) as this indicates protocol misbehavior.

4. **Graceful Handling of Existing Blocks**: If the peer sends blocks we already have, they are gracefully skipped without error, but subsequent blocks must still be contiguous.

5. **Empty Response Handling**: Empty responses are handled gracefully without error.

**Algorithm:**
```go
expectedHeight := blockStore.Height() + 1
for i, block := range resp.Blocks {
    if i == 0 {
        // First block can be one we already have (skip)
        // or must be at expectedHeight
        if height > expectedHeight {
            return ErrNonContiguousBlock  // Gap detected
        }
    } else {
        // Subsequent blocks must be contiguous
        if height != expectedHeight {
            return ErrNonContiguousBlock
        }
    }
    expectedHeight = height + 1
}
```

**Test Coverage:**
- `TestSyncReactor_HandleBlocksResponseContiguous` - Verifies contiguous blocks are accepted
- `TestSyncReactor_HandleBlocksResponseNonContiguous` - Verifies non-contiguous blocks are rejected
- `TestSyncReactor_HandleBlocksResponseWrongStartHeight` - Verifies blocks starting at wrong height are rejected
- `TestSyncReactor_HandleBlocksResponseEmptyResponse` - Verifies empty responses are handled
- `TestSyncReactor_HandleBlocksResponseWithExistingBlocks` - Verifies existing blocks are skipped gracefully

**Design Decisions:**
- Validate contiguity before storing any blocks (fail-fast approach)
- Penalize peers for protocol violations but don't disconnect immediately
- Allow skipping blocks we already have (supports partial retransmission)
- Return error to caller so they can handle appropriately

---

### 0.4 Unbounded Pending Requests Cleanup
**Status:** Complete

**Files Modified:**
- `handlers/transactions.go` - Added `DefaultMaxPendingAge` constant, `maxPendingAge` field, and `cleanupStaleRequests()` method; updated `gossipLoop()` to periodically cleanup stale requests
- `handlers/transactions_test.go` - Added tests for cleanup functionality

**Key Changes:**
1. **Maximum Pending Age Configuration**: Added `DefaultMaxPendingAge = 60 * time.Second` constant and `maxPendingAge` field to `TransactionsReactor` to control how long pending requests are retained before cleanup.

2. **Stale Request Cleanup**: Added `cleanupStaleRequests()` method that iterates through all pending requests and removes those that have exceeded `maxPendingAge`. This prevents unbounded memory growth from requests that never receive responses (e.g., from unresponsive or disconnected peers).

3. **Automatic Cleanup in Gossip Loop**: The `gossipLoop()` now calls `cleanupStaleRequests()` on each tick before requesting transactions from peers, ensuring regular cleanup without additional goroutines.

4. **Peer Cleanup on Empty**: When all pending requests for a peer are removed (either by cleanup or by completion), the peer entry itself is removed from the map to prevent empty map entries from accumulating.

**Algorithm:**
```go
func (r *TransactionsReactor) cleanupStaleRequests() {
    now := time.Now()
    r.mu.Lock()
    defer r.mu.Unlock()

    for peerID, pending := range r.pendingRequests {
        for txHash, requestTime := range pending {
            if now.Sub(requestTime) > r.maxPendingAge {
                delete(pending, txHash)
            }
        }
        if len(pending) == 0 {
            delete(r.pendingRequests, peerID)
        }
    }
}
```

**Test Coverage:**
- `TestTransactionsReactor_DefaultMaxPendingAge` - Verifies default max pending age is set correctly
- `TestTransactionsReactor_CleanupStaleRequests` - Verifies stale requests are removed and fresh ones are kept
- `TestTransactionsReactor_CleanupStaleRequestsEmpty` - Verifies cleanup handles empty state without panic
- `TestTransactionsReactor_CleanupStaleRequestsAllFresh` - Verifies all fresh requests are retained

**Design Decisions:**
- Cleanup runs on the same ticker as gossip to avoid additional goroutines
- Default 60-second timeout balances memory cleanup with allowing slow peers to respond
- Configurable via `maxPendingAge` field for testing and tuning
- Thread-safe implementation using existing mutex

---

### 0.5 Race Condition in Node Shutdown
**Status:** Complete

**Files Modified:**
- `node/node.go` - Added `context` and `sync/atomic` imports; added `DefaultShutdownTimeout` constant; added `stopping` atomic flag; updated `Stop()` method with proper shutdown sequence; updated `handleConnectionEvent()` and `handleMessage()` to check stopping flag
- `node/node_test.go` - Added tests for shutdown behavior

**Key Changes:**
1. **Stopping Flag**: Added `stopping atomic.Bool` field to the Node struct. This flag is set when shutdown begins and prevents the event loop from dispatching to reactors during shutdown.

2. **Shutdown Timeout**: Added `DefaultShutdownTimeout = 5 * time.Second` constant. The Stop() method now uses a timeout when waiting for the event loop to drain, preventing indefinite hangs.

3. **Proper Shutdown Sequence**: Stop() now follows a safe shutdown sequence:
   - Set stopping flag (prevents new message dispatching)
   - Signal event loop to stop
   - Wait for event loop with timeout
   - Stop reactors (now safe since event loop won't call them)
   - Stop network
   - Close stores

4. **Event Loop Protection**: Both `handleConnectionEvent()` and `handleMessage()` now check the stopping flag at the start and return immediately if shutdown is in progress.

**Algorithm:**
```go
func (n *Node) Stop() error {
    // 1. Signal that shutdown is in progress
    n.stopping.Store(true)

    // 2. Signal event loop to stop
    close(n.stopCh)

    // 3. Wait for event loop with timeout
    done := make(chan struct{})
    go func() {
        n.wg.Wait()
        close(done)
    }()

    ctx, cancel := context.WithTimeout(context.Background(), DefaultShutdownTimeout)
    defer cancel()

    select {
    case <-done:
        // Event loop drained cleanly
    case <-ctx.Done():
        // Timeout - proceed with shutdown anyway
    }

    // 4. Now safe to stop reactors
    _ = n.syncReactor.Stop()
    // ...
}
```

**Test Coverage:**
- `TestNode_StopNotStarted` - Verifies Stop() returns error when not started
- `TestNode_StoppingFlag` - Verifies stopping flag can be set and read
- `TestDefaultShutdownTimeout` - Verifies default timeout value

**Design Decisions:**
- Used atomic.Bool for stopping flag to avoid lock contention
- 5-second timeout provides safety against hangs while allowing reasonable drain time
- Event loop checks stopping flag before dispatching to prevent races with reactor.Stop()
- Timeout proceeds with shutdown rather than failing, ensuring node eventually stops

---

### 0.6 Handshake Timeout Enforcement
**Status:** Complete

**Files Modified:**
- `handlers/handshake.go` - Added timeout constants, lifecycle management (Start/Stop), timeout loop, and cleanup method
- `handlers/handshake_test.go` - Added tests for timeout functionality
- `node/node.go` - Added handshake handler Start/Stop calls to node lifecycle
- `go.mod` - Bumped glueberry version to v1.2.3

**Key Changes:**
1. **Timeout Configuration**: Added `DefaultHandshakeTimeout = 30 * time.Second` and `DefaultHandshakeCheckInterval = 5 * time.Second` constants.

2. **Lifecycle Management**: Added `Start()`, `Stop()`, and `IsRunning()` methods to `HandshakeHandler` with proper goroutine management via `sync.WaitGroup`.

3. **Timeout Loop**: Added `timeoutLoop()` goroutine that periodically checks for stale handshakes and cleans them up.

4. **Cleanup Method**: Added `cleanupStaleHandshakes()` that iterates through all handshake states and disconnects peers whose incomplete handshakes have exceeded the timeout.

5. **Node Integration**: Updated `node.go` to start the handshake handler before other reactors and stop it during shutdown.

**Algorithm:**
```go
func (h *HandshakeHandler) cleanupStaleHandshakes() {
    now := time.Now()
    h.mu.Lock()
    defer h.mu.Unlock()

    for peerID, state := range h.states {
        if state.State != StateComplete && now.Sub(state.StartedAt) > h.timeout {
            delete(h.states, peerID)
            if h.network != nil {
                go func(pid peer.ID) {
                    _ = h.network.Disconnect(pid)
                }(peerID)
            }
        }
    }
}
```

**Test Coverage:**
- `TestHandshakeHandler_DefaultTimeouts` - Verifies default timeout values are set
- `TestHandshakeHandler_StartStop` - Verifies lifecycle management works correctly
- `TestHandshakeHandler_CleanupStaleHandshakes` - Verifies stale handshakes are cleaned up while fresh and complete ones are retained
- `TestHandshakeHandler_TimeoutConstants` - Verifies default timeout constants are reasonable

**Design Decisions:**
- 30-second handshake timeout balances allowing slow connections with preventing resource exhaustion
- 5-second check interval provides timely cleanup without excessive CPU usage
- Disconnect calls are made asynchronously to avoid holding the lock during network operations
- Complete handshakes are never cleaned up regardless of age (they represent successful connections)

---

### 0.7 Constant-Time Hash Comparison
**Status:** Complete

**Files Modified:**
- `types/hash.go` - Added `crypto/subtle` import and `HashEqual()` helper function
- `types/hash_test.go` - Added comprehensive tests for `HashEqual()`
- `handlers/blocks.go` - Updated hash comparison to use `HashEqual()`
- `handlers/transactions.go` - Updated hash comparison to use `HashEqual()`
- `sync/reactor.go` - Updated hash comparison to use `HashEqual()`

**Key Changes:**
1. **HashEqual Helper Function**: Added `types.HashEqual(a, b []byte) bool` that uses `crypto/subtle.ConstantTimeCompare()` to perform timing-attack-resistant hash comparisons.

2. **Updated All Hash Comparisons**: Replaced all `string(hash1) != string(hash2)` comparisons with `!types.HashEqual(hash1, hash2)` in:
   - Block hash verification in `handlers/blocks.go`
   - Transaction hash verification in `handlers/transactions.go`
   - Block sync hash verification in `sync/reactor.go`

**Implementation:**
```go
// HashEqual performs a constant-time comparison of two hashes.
// This prevents timing attacks when comparing secret hash values.
// Returns true if the hashes are equal, false otherwise.
func HashEqual(a, b []byte) bool {
    return subtle.ConstantTimeCompare(a, b) == 1
}
```

**Test Coverage:**
- `TestHashEqual` - Comprehensive test covering:
  - Equal hashes
  - Different hashes
  - Empty vs non-empty
  - Nil slices
  - Different lengths
  - Constant-time property verification
- `BenchmarkHashEqual` - Performance benchmark

**Design Decisions:**
- Used `crypto/subtle.ConstantTimeCompare()` which is the standard Go approach for timing-safe comparisons
- Created a helper function to centralize the pattern and make future audits easier
- Function returns `bool` rather than `int` for more idiomatic Go usage
- Function handles nil and empty slices correctly

---

### 0.8 Penalty Persistence and Decay Fix
**Status:** Complete

**Files Modified:**
- `p2p/scoring.go` - Added `PenaltyRecord` struct, `penaltyHistory` map, wall-clock decay methods, and consolidated ban tracking
- `p2p/scoring_test.go` - Added comprehensive tests for penalty persistence and wall-clock decay
- `testing/helpers.go` - Added `AcceptAllTxValidator` to test node mempool

**Key Changes:**
1. **PenaltyRecord Struct**: Added new struct to track penalty state across peer disconnects:
   ```go
   type PenaltyRecord struct {
       Points     int64     // Current raw penalty points (before decay)
       LastDecay  time.Time // Last time decay was calculated
       BanCount   int       // Number of times this peer has been banned
       LastBanEnd time.Time // When the last ban ended (for progressive bans)
   }
   ```

2. **Penalty History Map**: Added `penaltyHistory map[peer.ID]*PenaltyRecord` to `PeerScorer` to persist penalties across peer disconnects.

3. **Wall-Clock Decay**: Implemented `applyDecayLocked()` method that calculates decay based on elapsed wall-clock time since `LastDecay`, rather than requiring periodic decay loop calls. This ensures penalties decay correctly even for disconnected peers.

4. **GetEffectivePenaltyPoints**: New method that returns current penalty points after applying wall-clock-based decay, working for both connected and disconnected peers.

5. **Updated Existing Methods**:
   - `AddPenalty`: Now stores in penaltyHistory and applies decay before adding new points
   - `GetPenaltyPoints`: Falls back to penalty history for disconnected peers
   - `ShouldBan`: Uses `GetEffectivePenaltyPoints` for consistent behavior
   - `GetBanDuration`: Uses consolidated `PenaltyRecord.BanCount`
   - `RecordBan`: Updates consolidated penalty history alongside legacy map
   - `ResetBanCount`: Resets both penalty history and legacy map

**Algorithm:**
```go
func (ps *PeerScorer) applyDecayLocked(record *PenaltyRecord) {
    elapsed := time.Since(record.LastDecay)
    hoursElapsed := int64(elapsed.Hours())
    if hoursElapsed > 0 {
        decay := hoursElapsed * PenaltyDecayRate
        record.Points -= decay
        if record.Points < 0 {
            record.Points = 0
        }
        record.LastDecay = time.Now()
    }
}
```

**Test Coverage:**
- `TestPeerScorer_PenaltyPersistence` - Verifies penalties tracked without connected peer
- `TestPeerScorer_PenaltyRecord` - Verifies record initialization and ban count tracking
- `TestPeerScorer_WallClockDecay` - Verifies decay based on time elapsed, timestamp updates, no negative values
- `TestPeerScorer_GetPenaltyPointsFallback` - Verifies fallback to history for disconnected peers
- `TestPeerScorer_NilPeerManager` - Verifies functionality works without peer manager

**Design Decisions:**
- Wall-clock decay ensures disconnected peers still have penalties decay naturally
- Consolidated `PenaltyRecord` tracks all penalty-related state in one place
- Maintained backward compatibility with legacy `banCounts` map
- Decay applied lazily on access (not background goroutine) for simplicity and correctness
- Nil checks added throughout for robustness when peer manager is not set

---

## Phase 1: Pluggable Architecture Foundation

### 1.1 Component Interface Pattern
**Status:** Complete

**Files Created:**
- `types/component.go` - Base interfaces for pluggable components
- `types/component_test.go` - Comprehensive tests for component interfaces

**Files Modified:**
- `handlers/blocks.go` - Added Start/Stop/IsRunning/Name methods
- `handlers/consensus.go` - Added Start/Stop/IsRunning/Name methods
- `handlers/transactions.go` - Added Name method
- `handlers/handshake.go` - Added Name method
- `handlers/housekeeping.go` - Added Name method
- `sync/reactor.go` - Added Name method
- `pex/reactor.go` - Added Name method

**Key Changes:**
1. **Component Interface**: Base interface for all pluggable components:
   ```go
   type Component interface {
       Start() error
       Stop() error
       IsRunning() bool
   }
   ```

2. **ConfigurableComponent**: For components with configuration validation:
   ```go
   type ConfigurableComponent interface {
       Component
       Validate() error
   }
   ```

3. **LifecycleAware**: For components that need lifecycle hooks:
   ```go
   type LifecycleAware interface {
       OnStart() error
       OnStop() error
   }
   ```

4. **Named Interface**: For component identification:
   ```go
   type Named interface {
       Name() string
   }
   ```

5. **Dependent Interface**: For declaring component dependencies:
   ```go
   type Dependent interface {
       Dependencies() []string
   }
   ```

6. **HealthChecker Interface**: For health monitoring:
   ```go
   type HealthChecker interface {
       HealthCheck() error
   }
   ```

7. **GetComponentInfo Helper**: Extracts metadata from components for introspection.

**Test Coverage:**
- `TestComponent_BasicLifecycle` - Verifies Start/Stop/IsRunning behavior
- `TestConfigurableComponent_Validation` - Verifies Validate() for valid/invalid configs
- `TestGetComponentInfo_*` - Verifies metadata extraction from components
- `TestNamedInterface` - Verifies Name() interface
- `TestDependentInterface` - Verifies Dependencies() interface
- `TestHealthCheckerInterface` - Verifies HealthCheck() for healthy/unhealthy states
- `TestLifecycleAwareInterface` - Verifies OnStart/OnStop hooks

**Design Decisions:**
- Interfaces are composable - components implement only what they need
- All reactors now implement Component for consistent lifecycle management
- Named interface enables better logging and debugging
- GetComponentInfo provides runtime introspection for monitoring
- Existing reactor lifecycle methods preserved for backward compatibility

---

### 1.2 Dependency Injection Container
**Status:** Complete

**Files Created:**
- `container/container.go` - Dependency injection container implementation
- `container/container_test.go` - Comprehensive tests for container

**Files Modified:**
- `node/node.go` - Added container integration methods and component name constants
- `node/node_test.go` - Added tests for container integration
- `p2p/network.go` - Added Name() and IsRunning() methods to implement Component interface

**Key Changes:**

1. **Container Package**: New `container/container.go` provides a dependency injection container:
   ```go
   type Container struct {
       components map[string]*componentEntry
       order      []string  // Startup order (topologically sorted)
       started    bool
       mu         sync.RWMutex
   }
   ```

2. **Component Registration**: Register components with optional dependencies:
   ```go
   c.Register("sync-reactor", syncReactor, "network", "block-reactor")
   ```

3. **Topological Sort**: Uses Kahn's algorithm to compute dependency-respecting startup order.

4. **Lifecycle Management**: StartAll/StopAll methods that:
   - Start components in dependency order
   - Call OnStart for LifecycleAware components
   - Validate ConfigurableComponents
   - Stop components in reverse dependency order
   - Call OnStop for LifecycleAware components

5. **Error Detection**:
   - Circular dependency detection
   - Missing dependency detection
   - Duplicate registration prevention
   - Post-start registration prevention

6. **Node Integration**: Added methods to Node for container access:
   ```go
   func (n *Node) ComponentContainer() (*container.Container, error)
   func (n *Node) GetComponent(name string) (types.Component, error)
   func (n *Node) ComponentNames() []string
   ```

7. **Component Name Constants**: Type-safe component names in node package:
   ```go
   const (
       ComponentNetwork      = "network"
       ComponentHandshake    = "handshake-handler"
       ComponentPEX          = "pex-reactor"
       ComponentHousekeeping = "housekeeping-reactor"
       ComponentTransactions = "transactions-reactor"
       ComponentBlocks       = "block-reactor"
       ComponentConsensus    = "consensus-reactor"
       ComponentSync         = "sync-reactor"
   )
   ```

8. **Network Component**: Added Name() and IsRunning() to p2p.Network to implement Component interface.

**Test Coverage:**
- `TestNew` - Verifies container initialization
- `TestRegister` - Tests registration scenarios (success, duplicates, post-start, with Dependent interface)
- `TestGet/TestMustGet/TestHas` - Tests component retrieval
- `TestNames/TestCount` - Tests introspection methods
- `TestStartAll` - Tests startup (idempotent, dependency order, errors, LifecycleAware, ConfigurableComponent)
- `TestStopAll` - Tests shutdown (idempotent, reverse order, error handling, LifecycleAware)
- `TestCircularDependency` - Tests cycle detection (simple, long, self-dependency)
- `TestMissingDependency` - Tests missing dependency detection
- `TestStartupOrder` - Tests order retrieval
- `TestComponentInfo` - Tests metadata extraction
- `TestConcurrentAccess` - Tests thread safety
- `TestComplexDependencyGraph` - Tests diamond and complex dependency patterns

**Design Decisions:**
- Container is optional - Node still manages its own lifecycle for backward compatibility
- ComponentContainer() returns a pre-configured container for introspection and custom wiring
- Topological sort ensures dependencies start before dependents
- Kahn's algorithm provides O(V+E) complexity and clear cycle detection
- Container can be used standalone for custom node configurations
- All errors are wrapped with context for debugging

---

### 1.3 Callback-Based Extensibility
**Status:** Complete

**Files Created:**
- `types/callbacks.go` - NodeCallbacks struct with event callbacks and safe invocation helpers
- `types/callbacks_test.go` - Comprehensive tests for callbacks

**Files Modified:**
- `node/node.go` - Added callbacks field, SetCallbacks/Callbacks methods, WithCallbacks option, callback invocations in event handlers
- `node/node_test.go` - Added tests for callback methods
- `types/component_test.go` - Fixed formatting

**Key Changes:**

1. **NodeCallbacks Struct**: Defines callbacks for all node events:
   ```go
   type NodeCallbacks struct {
       // Transaction callbacks
       OnTxReceived     func(peerID peer.ID, tx []byte)
       OnTxValidated    func(tx []byte, err error)
       OnTxBroadcast    func(tx []byte, peers []peer.ID)
       OnTxAdded        func(txHash []byte, tx []byte)
       OnTxRemoved      func(txHash []byte, reason string)

       // Block callbacks
       OnBlockReceived  func(peerID peer.ID, height int64, hash, data []byte)
       OnBlockValidated func(height int64, hash []byte, err error)
       OnBlockCommitted func(height int64, hash []byte)
       OnBlockStored    func(height int64, hash []byte)

       // Peer callbacks
       OnPeerConnected    func(peerID peer.ID, isOutbound bool)
       OnPeerHandshaked   func(peerID peer.ID, info *PeerInfo)
       OnPeerDisconnected func(peerID peer.ID)
       OnPeerPenalized    func(peerID peer.ID, points int64, reason string)

       // Consensus callbacks
       OnConsensusMessage func(peerID peer.ID, data []byte)
       OnProposalReady    func(height int64) ([]byte, error)
       OnVoteReady        func(height int64, round int32, voteType VoteType) ([]byte, error)

       // Sync callbacks
       OnSyncStarted   func(startHeight, targetHeight int64)
       OnSyncProgress  func(currentHeight, targetHeight int64)
       OnSyncCompleted func(height int64)
   }
   ```

2. **Safe Invocation Helpers**: Methods like `InvokeTxReceived()`, `InvokePeerConnected()` etc. that check for nil before calling.

3. **Helper Methods**:
   - `DefaultCallbacks()` - Returns empty callbacks struct
   - `Clone()` - Creates a copy of callbacks
   - `Merge()` - Merges callbacks, overwriting with non-nil values

4. **Node Integration**:
   - `WithCallbacks(cb)` option for node creation
   - `SetCallbacks(cb)` method for runtime callback changes
   - `Callbacks()` getter method
   - Event handlers invoke appropriate callbacks

5. **PeerInfo Struct**: For handshake callback:
   ```go
   type PeerInfo struct {
       NodeID          string
       ChainID         string
       ProtocolVersion int32
       Height          int64
   }
   ```

**Test Coverage:**
- `TestVoteTypeConstants` - Verifies vote type constants
- `TestDefaultCallbacks` - Verifies default callbacks are nil
- `TestCallbacks_Clone` - Tests cloning behavior
- `TestCallbacks_Merge` - Tests merging behavior
- `TestCallbacks_SafeInvocation` - Tests nil-safe invocation
- `TestPeerInfo` - Tests PeerInfo struct
- `TestCallbacks_AllInvokers` - Tests all invoker methods
- `TestCallbacks_Nil/SetAndGet/SetNil` - Node callback integration tests
- `TestOption_WithCallbacks` - Tests WithCallbacks option

**Design Decisions:**
- All callbacks are optional (nil-safe)
- Safe invocation helpers prevent nil pointer panics
- Clone and Merge enable callback composition
- Callbacks are invoked at appropriate points in event handling
- Consensus callback allows pluggable consensus engines
- PeerInfo provides handshake details to applications
- Callbacks can be changed at runtime via SetCallbacks

---

### 1.4 Configuration Overhaul
**Status:** Complete

**Files Modified:**
- `config/config.go` - Added NodeRole enum, HandlersConfig, validation
- `config/config_test.go` - Added tests for new config types

**Key Changes:**

1. **NodeRole Enum**: Defines node roles in the network:
   ```go
   type NodeRole string

   const (
       RoleValidator NodeRole = "validator"
       RoleFull      NodeRole = "full"
       RoleSeed      NodeRole = "seed"
       RoleLight     NodeRole = "light"
   )
   ```

2. **HandlersConfig**: Configuration for message handlers (previously hardcoded):
   ```go
   type HandlersConfig struct {
       Transactions TransactionsHandlerConfig `toml:"transactions"`
       Blocks       BlocksHandlerConfig       `toml:"blocks"`
       Sync         SyncHandlerConfig         `toml:"sync"`
   }

   type TransactionsHandlerConfig struct {
       RequestInterval Duration `toml:"request_interval"`
       BatchSize       int32    `toml:"batch_size"`
       MaxPending      int      `toml:"max_pending"`
       MaxPendingAge   Duration `toml:"max_pending_age"`
   }

   type BlocksHandlerConfig struct {
       MaxBlockSize int64 `toml:"max_block_size"`
   }

   type SyncHandlerConfig struct {
       SyncInterval      Duration `toml:"sync_interval"`
       BatchSize         int32    `toml:"batch_size"`
       MaxPendingBatches int      `toml:"max_pending_batches"`
   }
   ```

3. **Config Struct Updates**: Added Role and Handlers fields to main Config.

4. **Validation**: Complete validation for all new configuration fields.

5. **Default Values**: Sensible defaults for all handler configuration.

**Test Coverage:**
- `TestDefaultConfig` - Updated to verify new defaults
- `TestNodeRole` - Tests role validation (valid, invalid, config integration)
- `TestHandlersConfigValidation` - Tests handler config validation

**Design Decisions:**
- NodeRole enables role-based behavior (validators vs. full nodes vs. seeds)
- HandlersConfig centralizes timing values previously hardcoded in node.go
- All new config fields have validation with descriptive error messages
- Backward compatible - existing configs work with new defaults

---

### 1.5 Remove Hardcoded Values
**Status:** Complete (merged with 1.4)

The HandlersConfig added in Phase 1.4 addresses this phase by making configurable:
- Transaction gossip request interval (was hardcoded 5s in node.go:188)
- Transaction batch size (was hardcoded 100 in node.go:189)
- Sync interval (was hardcoded 5s in node.go:224)
- Sync batch size (was hardcoded 100 in node.go:225)
- Max pending transaction requests
- Max pending age for cleanup
- Max block size
- Max pending sync batches

All timing and size values are now in configuration with sensible defaults.

---

### 1.6 Fix Options Pattern
**Status:** Complete

**Files Created:**
- `node/builder.go` - NodeBuilder fluent interface for node construction
- `node/builder_test.go` - Comprehensive tests for builder pattern

**Key Changes:**

1. **NodeBuilder Pattern**: New fluent interface for constructing nodes:
   ```go
   type NodeBuilder struct {
       cfg              *config.Config
       mempool          mempool.Mempool
       blockStore       blockstore.BlockStore
       consensusHandler handlers.ConsensusHandler
       blockValidator   bsync.BlockValidator
       callbacks        *types.NodeCallbacks
       err              error
   }
   ```

2. **Fluent Methods**: Chain-able methods for setting components:
   ```go
   node, err := NewNodeBuilder(cfg).
       WithMempool(customMempool).
       WithBlockStore(customBlockStore).
       WithConsensusHandler(myHandler).
       WithBlockValidator(myValidator).
       WithCallbacks(myCallbacks).
       Build()
   ```

3. **Single-Pass Initialization**: The builder ensures all components are configured before `Build()` is called, preventing the double-apply issues that existed in the Options pattern (lines 186-188 and 254-256 of node.go).

4. **Error Tracking**: Builder tracks errors during configuration. If any error occurs, subsequent method calls are no-ops and `Build()` returns the error.

5. **MustBuild()**: Convenience method that panics on error, useful for tests and simple scenarios where errors are unexpected.

6. **Backward Compatibility**: `NewNodeWithBuilder()` is an alias for `NewNodeBuilder()`, providing clear naming while maintaining existing patterns.

7. **HandlersConfig Integration**: The builder uses `HandlersConfig` values from configuration when creating reactors, ensuring timing values come from config rather than hardcoded defaults.

**Test Coverage:**
- `TestNodeBuilder_Basic` - Verifies basic node creation with all components
- `TestNodeBuilder_WithMempool` - Tests custom mempool injection
- `TestNodeBuilder_WithCallbacks` - Tests callback configuration with invocation verification
- `TestNodeBuilder_WithBlockValidator` - Tests validator injection
- `TestNodeBuilder_WithConsensusHandler` - Tests consensus handler injection
- `TestNodeBuilder_InvalidConfig` - Verifies invalid config returns error
- `TestNodeBuilder_Chaining` - Tests method chaining with multiple options
- `TestNodeBuilder_MustBuild_Panics` - Verifies MustBuild panics on error
- `TestNodeBuilder_MustBuild_Success` - Verifies MustBuild succeeds with valid config
- `TestNewNodeWithBuilder` - Tests the alias function
- `TestNodeBuilder_ConsensusHandlerIsSet` - Verifies handler is retrievable from reactor
- `TestNodeBuilder_ErrorPropagation` - Tests that invalid config doesn't panic during chaining
- `TestNodeBuilder_ConsensusHandlerInterface` - Verifies interface compatibility

**Design Decisions:**
- Builder pattern provides clear single-pass initialization
- Error tracking in builder prevents partial configuration issues
- All methods check for prior errors before proceeding
- Fluent interface enables clean, readable configuration
- MustBuild() provides panic-on-error for simple use cases
- Builder creates all reactors in one pass using config values
- Default components (mempool, blockstore) are created if not explicitly provided
- Consensus handler and block validator are set on reactors during build

---

## Phase 2: Mempool Plugin System

### 2.1 Extended Mempool Interface
**Status:** Complete

**Files Created:**
- `mempool/interface.go` - Extended mempool interfaces for DAG and network-aware mempools
- `mempool/interface_test.go` - Comprehensive tests for extended interfaces

**Key Changes:**

1. **ValidatingMempool Interface**: Extends base Mempool with explicit validation:
   ```go
   type ValidatingMempool interface {
       Mempool
       SetTxValidator(validator TxValidator)
   }
   ```

2. **DAGMempool Interface**: For DAG-based mempools like looseberry:
   ```go
   type DAGMempool interface {
       ValidatingMempool
       types.Component
       ReapCertifiedBatches(maxBytes int64) []CertifiedBatch
       NotifyCommitted(round uint64)
       UpdateValidatorSet(validators ValidatorSet)
       CurrentRound() uint64
       DAGMetrics() *DAGMempoolMetrics
   }
   ```

3. **NetworkAwareMempool Interface**: For mempools needing custom network streams:
   ```go
   type NetworkAwareMempool interface {
       Mempool
       StreamConfigs() []StreamConfig
       SetNetwork(network MempoolNetwork)
   }
   ```

4. **CertifiedBatch Struct**: Represents a batch with a certificate from a DAG mempool:
   ```go
   type CertifiedBatch struct {
       Batch          []byte
       Certificate    []byte
       Round          uint64
       ValidatorIndex uint16
       Hash           []byte
   }
   ```

5. **ValidatorSet Interface**: For validator management:
   ```go
   type ValidatorSet interface {
       Count() int
       GetPublicKey(index uint16) []byte
       F() int
       Quorum() int
       VerifySignature(index uint16, digest []byte, sig []byte) bool
   }
   ```

6. **StreamConfig Struct**: For network stream configuration:
   ```go
   type StreamConfig struct {
       Name           string
       Encrypted      bool
       RateLimit      int
       MaxMessageSize int
       Owner          string
   }
   ```

7. **Additional Interfaces**:
   - `PrioritizedMempool` - For priority-based transaction ordering
   - `ExpirableMempool` - For TTL-based transaction expiration
   - `MempoolIterator` and `IterableMempool` - For transaction iteration

**Test Coverage:**
- Mock implementations for DAGMempool, NetworkAwareMempool, ValidatorSet
- Tests for CertifiedBatch, StreamConfig struct fields
- Tests for PrioritizedMempool and ExpirableMempool
- Interface compliance verification

---

### 2.2 Mempool Factory
**Status:** Complete

**Files Created:**
- `mempool/factory.go` - Mempool factory for plugin registration
- `mempool/factory_test.go` - Comprehensive tests for factory

**Files Modified:**
- `config/config.go` - Added Type, TTL, CleanupInterval to MempoolConfig

**Key Changes:**

1. **MempoolType Constants**:
   ```go
   const (
       TypeSimple     MempoolType = "simple"
       TypePriority   MempoolType = "priority"
       TypeTTL        MempoolType = "ttl"
       TypeLooseberry MempoolType = "looseberry"
   )
   ```

2. **MempoolConstructor Function Type**:
   ```go
   type MempoolConstructor func(cfg *config.MempoolConfig) (Mempool, error)
   ```

3. **Factory Struct**: Registry for mempool implementations:
   ```go
   type Factory struct {
       registry map[MempoolType]MempoolConstructor
       mu       sync.RWMutex
   }
   ```

4. **Factory Methods**:
   - `Register(mempoolType, constructor)` - Register new mempool type
   - `Unregister(mempoolType)` - Remove mempool type
   - `Create(cfg)` - Create mempool instance from config
   - `Has(mempoolType)` - Check if type is registered
   - `Types()` - List registered types

5. **DefaultFactory**: Global factory instance with built-in types registered.

6. **Helper Functions**:
   - `CreateFromConfig(cfg)` - Create mempool using default factory
   - `RegisterMempool(type, constructor)` - Register on default factory

7. **MempoolConfig Updates**:
   ```go
   type MempoolConfig struct {
       Type            string   `toml:"type"`
       MaxTxs          int      `toml:"max_txs"`
       MaxBytes        int64    `toml:"max_bytes"`
       CacheSize       int      `toml:"cache_size"`
       TTL             Duration `toml:"ttl"`
       CleanupInterval Duration `toml:"cleanup_interval"`
   }
   ```

**Test Coverage:**
- `TestNewFactory` - Built-in types registered
- `TestFactory_Register` - Custom mempool registration
- `TestFactory_Unregister` - Type removal
- `TestFactory_Create_*` - Creation for each type
- `TestFactory_Types` - Type listing
- `TestFactory_Override` - Type override
- `TestDefaultFactory` - Global factory
- `TestFactory_ConcurrentAccess` - Thread safety

---

### 2.3 Looseberry Integration
**Status:** Complete

**Files Created:**
- `mempool/looseberry/adapter.go` - Adapter wrapping looseberry.Looseberry
- `mempool/looseberry/validator_adapter.go` - ValidatorSet adapter
- `mempool/looseberry/network_adapter.go` - Network adapter bridging MempoolNetwork to looseberry
- `mempool/looseberry/adapter_test.go` - Comprehensive tests

**Files Modified:**
- `go.mod` - Added looseberry dependency with replace directive

**Key Changes:**

1. **Adapter Struct**: Wraps looseberry.Looseberry to implement blockberry interfaces:
   ```go
   type Adapter struct {
       lb           *looseberry.Looseberry
       cfg          *Config
       network      mempool.MempoolNetwork
       txValidator  mempool.TxValidator
       validatorSet mempool.ValidatorSet
       running      atomic.Bool
       stopCh       chan struct{}
       wg           sync.WaitGroup
       mu           sync.RWMutex
   }
   ```

2. **Interface Implementations**:
   - `mempool.Mempool` - Basic mempool operations
   - `mempool.DAGMempool` - DAG-specific operations
   - `mempool.NetworkAwareMempool` - Custom stream configuration

3. **Config Struct**:
   ```go
   type Config struct {
       ValidatorIndex   uint16
       Signer           Signer
       LooseberryConfig *looseberry.Config
   }
   ```

4. **Signer Interface**: For cryptographic operations:
   ```go
   type Signer interface {
       Sign(digest []byte) ([]byte, error)
       PublicKey() []byte
   }
   ```

5. **ValidatorSet Adapter**: Converts blockberry's ValidatorSet to looseberry's:
   ```go
   type looseberryValidatorSet struct {
       bb mempool.ValidatorSet
   }
   ```

6. **Network Adapter**: Bridges MempoolNetwork to looseberry's Network interface:
   ```go
   type networkAdapter struct {
       network        mempool.MempoolNetwork
       validatorIndex uint16
       validatorSet   *validatorSetAdapter
       // Message channels for each message type
   }
   ```

7. **Stream Configurations**: Three streams for looseberry:
   - `looseberry-batches` - Batch messages (10MB max)
   - `looseberry-headers` - Header messages (1MB max)
   - `looseberry-sync` - Sync messages (50MB max)

8. **Encoding Functions**: Placeholder implementations for cramberry serialization.

**Test Coverage:**
- `TestNewAdapter_*` - Construction tests (nil config, nil looseberry config, success)
- `TestAdapter_StartStop` - Lifecycle tests (start, double start, stop, double stop)
- `TestAdapter_AddTx` - Transaction addition
- `TestAdapter_HasTx` - Transaction lookup
- `TestAdapter_GetTx_NotSupported` - DAG mempool limitation
- `TestAdapter_RemoveTxs_NoOp` - No-op for DAG mempool
- `TestAdapter_Flush` - Mempool clearing
- `TestAdapter_TxHashes` - Hash listing
- `TestAdapter_ReapTxs` - Transaction reaping
- `TestAdapter_ReapCertifiedBatches` - Batch reaping
- `TestAdapter_CurrentRound` - Round tracking
- `TestAdapter_NotifyCommitted` - Commit notification
- `TestAdapter_DAGMetrics` - Metrics retrieval
- `TestAdapter_StreamConfigs` - Stream configuration
- `TestAdapter_SetNetwork` - Network setup
- `TestAdapter_SetTxValidator` - Validator setup
- `TestAdapter_UpdateValidatorSet` - Validator set updates
- `TestAdapter_InterfaceCompliance` - Interface verification

**Design Decisions:**
- Adapter pattern for clean interface separation
- Placeholder encoding functions ready for cramberry integration
- Network adapter bridges different network abstractions
- ValidatorSet adapter handles type conversion between frameworks
- All three blockberry interfaces implemented for full compatibility
- Tests use mock network and validator set for isolation

---

### 2.4 TransactionsReactor Passive Mode
**Status:** Complete

**Files Modified:**
- `handlers/transactions.go` - Added passive mode support
- `handlers/transactions_test.go` - Added passive mode tests

**Key Changes:**

1. **New Struct Fields**:
   ```go
   type TransactionsReactor struct {
       // ...
       dagMempool  mempool.DAGMempool // Non-nil when using a DAG mempool
       passiveMode bool               // True for DAG mempools (no active gossip)
       // ...
   }
   ```

2. **Auto-Detection in Constructor**:
   ```go
   func NewTransactionsReactor(mp mempool.Mempool, ...) *TransactionsReactor {
       r := &TransactionsReactor{...}

       // Auto-detect passive mode based on mempool type
       if dagMp, ok := mp.(mempool.DAGMempool); ok {
           r.dagMempool = dagMp
           r.passiveMode = true
       }

       return r
   }
   ```

3. **Start() Modification**:
   ```go
   func (r *TransactionsReactor) Start() error {
       // ...
       // Skip gossip loop in passive mode - DAG mempool handles propagation
       if !r.passiveMode {
           r.wg.Add(1)
           go r.gossipLoop()
       }
       return nil
   }
   ```

4. **HandleMessage Passive Mode Behavior**:
   - In passive mode, only `TransactionDataResponse` is processed
   - `TransactionsRequest`, `TransactionsResponse`, and `TransactionDataRequest` are ignored
   - Unknown message types still return errors
   ```go
   if r.passiveMode {
       switch typeID {
       case TypeIDTransactionDataResponse:
           return r.handleTransactionDataResponse(peerID, remaining)
       case TypeIDTransactionsRequest, TypeIDTransactionsResponse,
           TypeIDTransactionDataRequest:
           return nil // Ignore gossip messages in passive mode
       default:
           return types.ErrUnknownMessageType
       }
   }
   ```

5. **BroadcastTx Passive Mode**:
   ```go
   func (r *TransactionsReactor) BroadcastTx(tx []byte) error {
       // In passive mode, DAG mempool handles propagation
       if r.passiveMode {
           return nil
       }
       // ... normal broadcast logic
   }
   ```

6. **Helper Methods**:
   - `IsPassiveMode()` - Returns whether reactor is in passive mode
   - `SetPassiveMode(bool)` - Manually enable/disable passive mode
   - `DAGMempool()` - Returns the DAG mempool if configured

**Test Coverage:**
- `TestTransactionsReactor_PassiveModeAutoDetection` - Tests auto-detection with DAG vs simple mempool
- `TestTransactionsReactor_SetPassiveMode` - Tests manual passive mode toggle
- `TestTransactionsReactor_PassiveModeSkipsGossipLoop` - Verifies gossip loop not started
- `TestTransactionsReactor_PassiveModeHandleMessageOnlyDataResponse` - Verifies data responses processed
- `TestTransactionsReactor_PassiveModeIgnoresGossipMessages` - Verifies gossip messages ignored
- `TestTransactionsReactor_PassiveModeUnknownTypeError` - Verifies unknown types still error
- `TestTransactionsReactor_PassiveModeBroadcastTxSkipped` - Verifies broadcast is no-op

**Design Decisions:**
- Auto-detection based on DAGMempool interface type assertion
- Passive mode skips entire gossip loop, not just parts of it
- Still accepts incoming transaction data (for compatibility)
- Ignores gossip-related messages rather than erroring (graceful degradation)
- BroadcastTx returns immediately since DAG mempool handles propagation
- Manual SetPassiveMode() available for testing and edge cases

---

## Phase 3: Consensus Engine Framework

### Phase 3.1: Consensus Engine Interface

**Status:** Complete

**Files Created:**
- `consensus/interface.go` - Core consensus engine interfaces and types
- `consensus/interface_test.go` - Comprehensive tests for interfaces

**Key Interfaces Defined:**

1. **ConsensusEngine** - Main interface for all consensus implementations:
   ```go
   type ConsensusEngine interface {
       types.Component
       Initialize(deps ConsensusDependencies) error
       ProcessBlock(block *Block) error
       GetHeight() int64
       GetRound() int32
       IsValidator() bool
       ValidatorSet() ValidatorSet
   }
   ```

2. **BFTConsensus** - Extended interface for BFT-style consensus:
   ```go
   type BFTConsensus interface {
       ConsensusEngine
       BlockProducer
       HandleProposal(proposal *Proposal) error
       HandleVote(vote *Vote) error
       HandleCommit(commit *Commit) error
       OnTimeout(height int64, round int32, step TimeoutStep) error
   }
   ```

3. **StreamAwareConsensus** - For consensus engines needing custom network streams:
   ```go
   type StreamAwareConsensus interface {
       ConsensusEngine
       StreamConfigs() []StreamConfig
       HandleStreamMessage(stream string, peerID peer.ID, data []byte) error
   }
   ```

4. **BlockProducer** - For engines that can create blocks:
   ```go
   type BlockProducer interface {
       ProduceBlock(height int64) (*Block, error)
       ShouldPropose(height int64, round int32) bool
   }
   ```

5. **ValidatorSet** - Interface for validator management:
   ```go
   type ValidatorSet interface {
       Count() int
       GetByIndex(index uint16) *Validator
       GetByAddress(address []byte) *Validator
       Contains(address []byte) bool
       GetProposer(height int64, round int32) *Validator
       TotalVotingPower() int64
       Quorum() int64
       VerifyCommit(height int64, blockHash []byte, commit *Commit) error
   }
   ```

**Supporting Types:**

- `ConsensusDependencies` - Node components provided to consensus engines
- `ConsensusCallbacks` - Event notifications from consensus to node
- `ConsensusConfig` - Configuration with timeouts and custom params
- `Block`, `Proposal`, `Vote`, `Commit`, `CommitSig` - BFT message types
- `Validator` - Validator information struct
- `StreamConfig` - Custom stream configuration
- `VoteType`, `TimeoutStep` - Type constants

**Safe Callback Invocation Methods:**
- `InvokeOnBlockProposed()` - Safely invoke block proposed callback
- `InvokeOnBlockCommitted()` - Safely invoke block committed callback
- `InvokeOnValidatorSetChanged()` - Safely invoke validator set change callback
- `InvokeOnStateSync()` - Safely invoke state sync callback
- `InvokeOnConsensusError()` - Safely invoke error callback

**Test Coverage:**
- 26 test functions covering all interfaces and types
- Mock implementations for ConsensusEngine, BFTConsensus, StreamAwareConsensus, ValidatorSet
- Callback invocation tests including nil safety
- Type constant verification
- Structure field verification for all types

**Design Decisions:**
- Interfaces embed `types.Component` and `types.Named` for lifecycle and identification
- BFTConsensus composes ConsensusEngine and BlockProducer
- Dependencies passed via Initialize() rather than constructor injection
- Safe callback invocations handle nil receivers and nil function pointers
- StreamConfig includes Owner field for routing to correct component
- ValidatorSet uses uint16 for index (matching BFT limitations)
- VotingPower and ProposerPriority as int64 for future flexibility

---

### Phase 3.2: Consensus Factory

**Status:** Complete

**Files Created:**
- `consensus/factory.go` - Consensus engine factory with registration
- `consensus/factory_test.go` - Factory tests
- `consensus/null.go` - NullConsensus implementation for full nodes
- `consensus/null_test.go` - NullConsensus tests

**Factory Implementation:**

1. **ConsensusType Constants**:
   ```go
   const (
       TypeNone ConsensusType = "none"
   )
   ```

2. **Factory Struct**:
   ```go
   type Factory struct {
       registry map[ConsensusType]ConsensusConstructor
       mu       sync.RWMutex
   }
   ```

3. **Factory Methods**:
   - `NewFactory()` - Creates factory with built-in types registered
   - `Register(type, constructor)` - Registers custom consensus engines
   - `Unregister(type)` - Removes a consensus engine
   - `Create(cfg)` - Creates engine from configuration
   - `Has(type)` - Checks if type is registered
   - `Types()` - Lists all registered types

4. **Convenience Functions**:
   - `DefaultFactory` - Global factory instance
   - `CreateFromConfig(cfg)` - Creates engine using default factory
   - `RegisterConsensus(type, constructor)` - Registers with default factory

**NullConsensus Implementation:**

A no-op consensus engine for full nodes that:
- Receives and stores blocks without participating in consensus
- Tracks height and round from processed blocks
- Invokes OnBlockCommitted callback when blocks are processed
- Returns false for IsValidator() and nil for ValidatorSet()
- Thread-safe with atomic operations for state

**Test Coverage:**
- 13 factory tests covering registration, creation, unregistration, nil handling
- 14 NullConsensus tests covering lifecycle, initialization, block processing
- Concurrent access tests for both factory and NullConsensus
- Interface compliance verification

**Design Decisions:**
- Factory pattern mirrors mempool.Factory for consistency
- Default factory allows easy global registration of custom engines
- NullConsensus useful for light clients and full nodes
- Thread-safe registry with RWMutex
- Atomic operations for NullConsensus state (height, round, running)

---

### Phase 3.3: Consensus Reactor Refactoring

**Status:** Complete

**Files Modified:**
- `handlers/consensus.go` - Refactored to use ConsensusEngine interface
- `handlers/consensus_test.go` - Added comprehensive tests for engine integration

**Key Changes:**

1. **New Constructor for Engine-based Reactor**:
   ```go
   func NewConsensusReactorWithEngine(
       engine consensus.ConsensusEngine,
       network *p2p.Network,
       peerManager *p2p.PeerManager,
   ) *ConsensusReactor
   ```

2. **Engine Management Methods**:
   - `SetEngine(engine)` - Sets the consensus engine
   - `GetEngine()` - Returns current engine
   - `HasCustomStream(name)` - Checks custom stream registration
   - `CustomStreams()` - Lists registered custom streams

3. **Custom Stream Support**:
   - Automatically registers streams from StreamAwareConsensus engines
   - `HandleCustomStreamMessage()` routes to engine's HandleStreamMessage()

4. **BFT Message Routing**:
   - Detects BFTConsensus engines and parses message types
   - Message type constants: MsgTypeProposal, MsgTypeVote, MsgTypeCommit, MsgTypeBlock
   - Routes to HandleProposal(), HandleVote(), HandleCommit(), ProcessBlock()

5. **Message Priority**:
   - Engine takes priority over legacy handler when both are set
   - Non-BFT engines receive raw data in Block.Data
   - BFT engines get parsed proposals, votes, and commits

6. **Backward Compatibility**:
   - Legacy ConsensusHandler interface still supported
   - SetHandler/GetHandler methods marked as deprecated
   - Graceful fallback to handler when no engine is set

**Test Coverage:**
- `TestNewConsensusReactorWithEngine` - Engine-based constructor
- `TestNewConsensusReactorWithEngine_StreamAware` - Stream registration
- `TestConsensusReactor_SetEngine` - Engine management
- `TestConsensusReactor_SetEngine_StreamAware` - Stream updates on engine change
- `TestConsensusReactor_HandleMessageWithEngine` - Non-BFT message handling
- `TestConsensusReactor_HandleMessageEngineOverHandler` - Priority verification
- `TestConsensusReactor_HandleMessageBFTProposal` - Proposal routing
- `TestConsensusReactor_HandleMessageBFTVote` - Vote routing
- `TestConsensusReactor_HandleMessageBFTCommit` - Commit routing
- `TestConsensusReactor_HandleMessageBFTBlock` - Block routing
- `TestConsensusReactor_HandleMessageBFTUnknownType` - Unknown type error
- `TestConsensusReactor_HandleCustomStreamMessage` - Custom stream handling
- `TestConsensusReactor_HandleCustomStreamMessage_UnknownStream` - Unknown stream error
- `TestConsensusReactor_StartStop` - Lifecycle with double-start protection

**Design Decisions:**
- Engine priority over handler for seamless migration path
- Custom streams registered automatically during engine setup
- BFT message parsing with placeholder implementations (TODO: cramberry)
- Thread-safe operations with RWMutex
- Non-BFT engines receive raw data for flexibility

---

### Phase 3.4: Reference BFT Implementation Skeleton

**Status:** Complete

**Files Created:**
- `consensus/bft/tendermint.go` - Reference Tendermint-style BFT implementation
- `consensus/bft/tendermint_test.go` - BFT engine tests
- `consensus/bft/types.go` - Supporting types (PrivValidator, WAL, vote tracking)
- `consensus/bft/types_test.go` - Vote tracking tests

**RoundStep State Machine:**
```
NewHeight -> NewRound -> Propose -> Prevote -> PrevoteWait ->
Precommit -> PrecommitWait -> Commit -> (next height)
```

**TendermintBFT Engine:**
- Implements ConsensusEngine, BFTConsensus, BlockProducer interfaces
- Round-based consensus with locked value mechanism
- Configurable timeouts (propose, prevote, precommit, commit)
- Integration points for:
  - PrivValidator (signing)
  - WAL (crash recovery)
  - TimeoutTicker (timeout management)
  - ValidatorSet (proposer selection)

**Supporting Types:**

1. **PrivValidator Interface**:
   - GetAddress/GetPublicKey for identity
   - SignVote/SignProposal for signing

2. **WAL Interface**:
   - Write/WriteSync for message logging
   - SearchForEndHeight for replay
   - Start/Stop/Flush for lifecycle

3. **HeightVoteSet**:
   - Tracks votes for a specific height
   - Per-round vote sets
   - 2f+1 majority detection

4. **RoundVoteSet**:
   - Tracks votes for a specific round
   - Voting power accumulation
   - Block hash vote counts

5. **TimeoutTicker Interface**:
   - ScheduleTimeout for timeout management
   - Chan for timeout events

**Skeleton Methods (TODO):**
- Block validation and execution
- WAL write operations
- Vote signature verification
- Proposer signature verification
- Equivocation detection
- State sync triggering

**Test Coverage:**
- Config defaults and custom config
- Engine lifecycle (start/stop)
- Initialize with dependencies
- Proposal handling (nil, past/current height)
- Vote/commit handling (nil checks)
- Timeout handling (stale vs current)
- Block production
- Proposer detection
- HeightVoteSet/RoundVoteSet operations
- 2f+1 majority detection
- Interface compliance verification

**Design Decisions:**
- Skeleton design for easy customization
- nolint directive for intentional TODOs
- Comprehensive state machine with all steps
- Modular timeout management via TimeoutTicker
- WAL interface for crash recovery
- Atomic operations for thread-safe state

---

### Phase 3.5: Validator Set Management

**Status:** Complete

**Files Created:**
- `consensus/validators.go` - Validator set implementations and storage
- `consensus/validators_test.go` - Comprehensive validator set tests

**SimpleValidatorSet:**
- Basic ValidatorSet implementation with O(n) lookups
- Thread-safe with RWMutex
- Deep copy semantics for validators (prevents external mutation)
- Validation on construction (nil checks, duplicate detection, voting power)
- Methods: Count, GetByIndex, GetByAddress, Contains, GetProposer, TotalVotingPower, Quorum, F, VerifyCommit, Validators, Copy

**IndexedValidatorSet:**
- Optimized ValidatorSet with O(1) address lookups via hash map
- Embeds SimpleValidatorSet for shared functionality
- Maintains address-to-validator index for fast lookups
- Suitable for larger validator sets

**WeightedProposerSelection:**
- PoS-style proposer selection weighted by voting power
- Priority-based algorithm (validators with more stake propose more often)
- GetProposer finds highest priority validator
- IncrementProposerPriority updates priorities after each round:
  - All validators: priority += votingPower
  - Proposer: priority -= totalPower

**ValidatorSetStore Interface:**
- SaveValidatorSet(height, valSet) - Persist at height
- LoadValidatorSet(height) - Retrieve at specific height
- LatestValidatorSet() - Most recent set
- ValidatorSetAtHeight(height) - Active set at given height (epoch support)

**InMemoryValidatorSetStore:**
- In-memory implementation for testing
- Tracks latest height
- ValidatorSetAtHeight finds most recent set at or before height

**Commit Verification:**
- VerifyCommit validates commits against validator set
- Ed25519 signature verification
- Quorum calculation: 2/3 * totalPower + 1
- Skips absent validators (nil signature)
- Height and block hash validation

**Common Errors:**
- ErrValidatorNotFound
- ErrDuplicateValidator
- ErrInvalidVotingPower
- ErrInvalidPublicKey
- ErrInvalidSignature
- ErrInsufficientQuorum
- ErrValidatorSetNotFound

**Helper Functions:**
- makeCommitSignBytes - Creates canonical sign bytes for commits

**Test Coverage (32 tests):**
- NewSimpleValidatorSet creation and validation
- Empty validator set handling
- Nil validator detection
- Invalid voting power rejection
- Duplicate address detection
- GetByIndex valid/invalid cases
- GetByAddress valid/invalid cases
- Contains membership testing
- GetProposer deterministic selection
- GetProposer empty set handling
- Quorum calculation (2f+1)
- F calculation (Byzantine tolerance)
- VerifyCommit nil/height/hash mismatches
- VerifyCommit valid signatures (Ed25519)
- VerifyCommit insufficient quorum
- Validators deep copy verification
- Copy deep copy verification
- IndexedValidatorSet O(1) lookups
- IndexedValidatorSet copy with index
- WeightedProposerSelection creation/empty
- IncrementProposerPriority weighted selection
- ValidatorSetStore save/load/latest
- ValidatorSetAtHeight epoch support

**Design Decisions:**
- Pointer embedding for IndexedValidatorSet to avoid mutex copy issues
- Deep copy for all public validator accessors (prevents external mutation)
- Standard library slices.Sort for modern Go idioms
- Interface compliance verified via compile-time checks
- Sentinel errors for consistent error handling
- nolint for safe int-to-uint16 conversions (validator count bounded)

---

## Phase 4: Node Role System

### Phase 4.1: Node Role Definitions

**Status:** Complete

**Files Created:**
- `types/role.go` - Node role definitions and capabilities
- `types/role_test.go` - Comprehensive role tests

**Files Modified:**
- `config/config.go` - Updated to use types.NodeRole
- `config/config_test.go` - Updated tests to use types.AllRoles()

**NodeRole Type:**
- String-based type for type safety
- Five roles: validator, full, seed, light, archive
- Validation via IsValid() method
- Capabilities via Capabilities() method

**RoleCapabilities Structure:**
```go
type RoleCapabilities struct {
    CanPropose                bool  // Can propose blocks
    CanVote                   bool  // Can vote in consensus
    StoresFullBlocks          bool  // Stores full block data
    StoresState               bool  // Stores application state
    ParticipatesInPEX         bool  // Exchanges peer addresses
    GossipsTransactions       bool  // Gossips transactions
    AcceptsInboundConnections bool  // Accepts incoming connections
    Prunes                    bool  // Prunes old data
    RequiresConsensusEngine   bool  // Needs consensus engine
    RequiresMempool           bool  // Needs mempool
    RequiresBlockStore        bool  // Needs block store
    RequiresStateStore        bool  // Needs state store
}
```

**Role Capability Matrix:**

| Role | Propose | Vote | Blocks | State | PEX | TxGossip | Inbound | Prunes |
|------|---------|------|--------|-------|-----|----------|---------|--------|
| validator | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ | ✓ | ✓ |
| full | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✓ |
| seed | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✓ | ✗ |
| light | ✗ | ✗ | ✗ | ✗ | ✓ | ✗ | ✗ | ✓ |
| archive | ✗ | ✗ | ✓ | ✓ | ✓ | ✓ | ✓ | ✗ |

**Helper Functions:**
- AllRoles() - Returns all valid roles
- ParseRole(string) - Parses string to role with validation
- ValidateRole(role) - Validates a role
- RoleRequiresComponent(role, component) - Checks component requirements
- DefaultRole() - Returns RoleFull as default

**Config Integration:**
- NodeRole type alias pointing to types.NodeRole
- Re-exported role constants for convenience
- Updated ErrInvalidNodeRole to include archive
- Backward compatible - no breaking changes

**Role-specific Errors:**
- ErrInvalidRole - Unknown role string
- ErrRoleCapabilityMismatch - Action not supported by role
- ErrRoleChangeNotAllowed - Dynamic role change denied

**Test Coverage (16 tests):**
- String conversion
- IsValid for all roles and invalid strings
- Capabilities for each role type
- Invalid role returns empty capabilities
- ParseRole valid and invalid inputs
- ValidateRole success and failure
- AllRoles returns all 5 roles
- DefaultRole returns full
- RoleRequiresComponent for all combinations
- Validator consensus capabilities
- Non-validators cannot consensus
- Archive never prunes
- Seed minimal storage requirements
- Error definitions verification

**Design Decisions:**
- Type alias in config for backward compatibility
- Capabilities struct for extensibility
- All roles participate in PEX for network health
- Validators don't gossip transactions (may use DAG)
- Light nodes don't accept inbound (client only)
- Archive nodes never prune (historical data)

---

*Last Updated: January 2025*
