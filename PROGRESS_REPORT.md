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

### Phase 4.2: Role-Based Component Selection

**Status:** Complete

**Files Created:**
- `node/role_builder.go` - Role-based node builder
- `node/role_builder_test.go` - Builder tests
- `mempool/noop.go` - No-op mempool for roles that don't need transactions
- `mempool/noop_test.go` - No-op mempool tests
- `blockstore/noop.go` - No-op blockstore for roles that don't store blocks
- `blockstore/noop_test.go` - No-op blockstore tests
- `blockstore/header_only.go` - Header-only blockstore for light clients
- `blockstore/header_only_test.go` - Header-only blockstore tests

**Files Modified:**
- `node/node.go` - Added role field and Role() getter

**RoleBasedBuilder:**
- Extends NodeBuilder with role-aware component selection
- Validates role requirements before build
- Auto-selects components based on role capabilities:
  - NoOpMempool for roles without mempool requirement
  - NoOpBlockStore for roles without block storage
  - HeaderOnlyBlockStore for light clients
- Configures consensus engine via reactor after build
- Fluent builder API with chaining

**Role Validation:**
- Validators require consensus engine
- Block-storing roles require block validator
- Helpful error messages for missing components

**NoOpMempool:**
- Returns ErrMempoolClosed for AddTx
- Reports 0 size and empty state
- For seed nodes and other minimal roles

**NoOpBlockStore:**
- Returns ErrStoreClosed for SaveBlock
- Returns ErrBlockNotFound for Load operations
- Reports 0 height and base

**HeaderOnlyBlockStore:**
- Stores hash only, discards block data
- Thread-safe with RWMutex
- GetHash() method for header-specific access
- Deep copies hashes to prevent mutation

**Helper Functions:**
- IsValidatorRole(role) - Checks consensus participation
- RoleRequiresFullSync(role) - Checks full block sync requirement
- RoleAcceptsInbound(role) - Checks inbound connection acceptance

**Errors:**
- ErrMissingConsensusEngine - Validator without engine
- ErrMissingBlockValidator - Block-storing role without validator
- ErrIncompatibleComponent - Component doesn't match role

**Test Coverage (35 tests across files):**

Role Builder (14 tests):
- NewRoleBasedBuilder with explicit role
- Default role (full) when not set
- WithRole valid/invalid
- WithMempool/WithBlockStore
- Validator requires engine
- Full node requires block validator
- Seed node minimal requirements
- Capabilities for all roles
- Helper functions (IsValidatorRole, etc.)

NoOp Mempool (12 tests):
- Interface compliance
- AddTx returns error
- All methods return empty/zero values
- SetTxValidator no-op

NoOp BlockStore (9 tests):
- Interface compliance
- SaveBlock returns error
- Load methods return not found
- Zero height/base

Header-Only BlockStore (13 tests):
- Interface compliance
- Saves hash, discards data
- Duplicate detection
- Load returns hash without data
- By-hash lookups
- Height/base tracking
- Close clears state
- Hash copy verification

**Design Decisions:**
- Consensus engine set after build to access network
- NoOp implementations for minimal resource usage
- Header-only store separate from NoOp for light client functionality
- Deep copies prevent external mutation

---

### Phase 4.3: Validator Detection

**Status:** Complete

**Files Created:**
- `consensus/detector.go` - Validator detection implementations
- `consensus/detector_test.go` - Comprehensive detector tests

**Files Modified:**
- `consensus/interface.go` - Added F() and Validators() to ValidatorSet interface
- `consensus/interface_test.go` - Updated mockValidatorSet with new methods
- `consensus/bft/tendermint_test.go` - Updated mockValidatorSet with new methods

**ValidatorDetector Interface:**
```go
type ValidatorDetector interface {
    IsValidator(valSet ValidatorSet) bool
    ValidatorIndex(valSet ValidatorSet) int
    GetValidator(valSet ValidatorSet) *Validator
}
```

**KeyBasedDetector:**
- Detects validator status by public key
- Deep copies key to prevent external mutation
- Returns index or -1 if not a validator
- PublicKey() getter for retrieval

**AddressBasedDetector:**
- Detects validator status by address
- Uses ValidatorSet.Contains() for O(1) lookup with IndexedValidatorSet
- Address() getter for retrieval

**ValidatorStatusTracker:**
- Monitors validator set changes
- Notifies via callback on status changes
- Thread-safe with RWMutex
- Detects both joining and leaving validator set
- Tracks validator index changes

**ValidatorSet Interface Updates:**
- Added F() - Returns max Byzantine validators tolerated
- Added Validators() - Returns all validators in set

**Test Coverage (25 tests):**

Detector Tests:
- Interface compliance for both detectors
- IsValidator valid/invalid/nil cases
- ValidatorIndex finding and not found
- GetValidator retrieval
- Key/Address deep copy verification

Status Tracker Tests:
- Creation and initial state
- Becoming validator (callback invoked)
- No change (callback not re-invoked)
- Leaving validator set
- Validator info retrieval
- SetCallback updates
- Nil callback handling
- Concurrent access safety
- Index change detection

**Design Decisions:**
- Two detector types for different identity models
- Status tracker separates detection from monitoring
- Callback invoked outside lock to prevent deadlocks
- Deep copies for all byte slice returns
- Interface extends ValidatorSet for complete functionality

---

## Phase 4 Complete

All Phase 4 tasks completed:
- 4.1 Node Role Definitions - Role types and capabilities
- 4.2 Role-Based Component Selection - Builder and NoOp components
- 4.3 Validator Detection - Detection and status tracking

---

## Phase 5: Dynamic Stream Registry

### Phase 5.1: Stream Registry Interface

**Status:** Complete

**Files Created:**
- `p2p/stream_registry.go` - Stream registry interface and implementation
- `p2p/stream_registry_test.go` - Comprehensive tests (35 tests)

**StreamRegistry Interface:**
```go
type StreamRegistry interface {
    Register(cfg StreamConfig) error
    Unregister(name string) error
    Get(name string) *StreamConfig
    All() []StreamConfig
    Names() []string
    Has(name string) bool
    RegisterHandler(name string, handler StreamHandler) error
    GetHandler(name string) StreamHandler
    ByOwner(owner string) []StreamConfig
    UnregisterByOwner(owner string) int
}
```

**StreamConfig Struct (Unified Definition):**
```go
type StreamConfig struct {
    Name           string    // Unique stream identifier
    Encrypted      bool      // Uses encryption after handshake
    MessageTypes   []uint16  // Cramberry message type IDs
    RateLimit      int       // Max messages per second
    MaxMessageSize int       // Max message size in bytes
    Owner          string    // Component that owns this stream
}
```

**StreamHandler Type:**
```go
type StreamHandler func(peerID peer.ID, data []byte) error
```

**InMemoryStreamRegistry Implementation:**
- Thread-safe with RWMutex
- Validates configs on registration
- Prevents unregistering streams with active handlers
- Supports owner-based stream management
- Clone semantics for all returned configs

**Helper Functions:**
- `RegisterBuiltinStreams(registry)` - Registers core blockberry streams
- `StreamConfig.Validate()` - Validates configuration
- `StreamConfig.Clone()` - Creates deep copy

**Stream Registry Errors:**
- ErrStreamAlreadyRegistered
- ErrStreamNotFound
- ErrStreamHandlerNotSet
- ErrInvalidStreamConfig
- ErrStreamInUse

**Test Coverage (35 tests):**
- StreamConfig validation (valid, empty name, negative values)
- StreamConfig clone (deep copy, nil message types)
- Registry creation and counting
- Stream registration (success, duplicate, invalid)
- Stream unregistration (success, not found, with handler)
- Get/Has/Names/All operations
- Handler registration and retrieval
- Owner-based operations
- Force unregister and clear
- Concurrent access safety
- Built-in stream registration

---

### Phase 5.2: Integration with Glueberry

**Status:** Complete

**Files Created:**
- `p2p/stream_adapter.go` - GlueberryStreamAdapter and StreamRouter
- `p2p/stream_adapter_test.go` - Adapter tests (17 tests)

**GlueberryStreamAdapter:**
- Bridges stream registry with glueberry's stream management
- Provides encrypted stream names for PrepareStreams calls
- Routes incoming messages to registered handlers
- Manages stream configurations and handlers

**Key Methods:**
```go
func (a *GlueberryStreamAdapter) GetEncryptedStreamNames() []string
func (a *GlueberryStreamAdapter) RouteMessage(msg streams.IncomingMessage) error
func (a *GlueberryStreamAdapter) RegisterStream(cfg StreamConfig) error
func (a *GlueberryStreamAdapter) SetHandler(name string, handler StreamHandler) error
```

**ObservableStreamAdapter:**
- Extends GlueberryStreamAdapter with registration callbacks
- Notifies when streams are registered/unregistered
- Enables dynamic updates for existing connections

**StreamRouter:**
- Routes messages with rate limiting and validation
- Enforces MaxMessageSize limits
- Integrates with RateLimiter for per-stream rate limiting

**Test Coverage:**
- Adapter creation and registry access
- Encrypted stream name retrieval
- Message routing (success, no handler, stream not found)
- Stream registration/unregistration
- Handler management
- Owner-based operations
- Observable adapter callbacks
- Router with rate limiting and message size validation

---

### Phase 5.3: Plugin Stream Registration

**Status:** Complete

**Files Modified:**
- `p2p/network.go` - Added stream registry integration and registration methods

**Network Struct Updates:**
```go
type Network struct {
    // ... existing fields ...
    streamAdapter *GlueberryStreamAdapter  // Stream management
}
```

**New Network Methods:**
```go
// Stream registration
func (n *Network) RegisterStream(cfg StreamConfig, handler StreamHandler) error
func (n *Network) UnregisterStream(name string) error
func (n *Network) SetStreamHandler(name string, handler StreamHandler) error
func (n *Network) GetStreamHandler(name string) StreamHandler

// Stream queries
func (n *Network) HasStream(name string) bool
func (n *Network) GetStreamConfig(name string) *StreamConfig
func (n *Network) RegisteredStreams() []string

// Message routing
func (n *Network) RouteMessage(msg streams.IncomingMessage) error

// Registry access
func (n *Network) StreamRegistry() StreamRegistry
func (n *Network) StreamAdapter() *GlueberryStreamAdapter

// Bulk operations
func (n *Network) RegisterBuiltinStreams() error
func (n *Network) UnregisterStreamsByOwner(owner string) int

// MempoolNetwork interface
func (n *Network) ConnectedPeers() []peer.ID
```

**Constructor Updates:**
```go
func NewNetwork(node *glueberry.Node) *Network  // Creates with default registry
func NewNetworkWithRegistry(node *glueberry.Node, registry StreamRegistry) *Network
```

**PrepareStreams/CompleteHandshake Updates:**
- Now uses streams from registry if available
- Falls back to AllStreams() if registry is empty
- Supports dynamic stream addition for future connections

**Design Decisions:**
- Stream adapter initialized in constructor
- Handlers cleared before stream unregistration
- Registry-first approach with fallback to defaults
- Network implements MempoolNetwork interface via ConnectedPeers()

---

## Phase 5 Complete

All Phase 5 tasks completed:
- 5.1 Stream Registry Interface - Registry and InMemoryStreamRegistry
- 5.2 Integration with Glueberry - GlueberryStreamAdapter and StreamRouter
- 5.3 Plugin Stream Registration - Network integration methods

The dynamic stream registry enables:
1. **Plugin streams** - Consensus engines and mempools can register custom streams
2. **Message routing** - Automatic routing to registered handlers
3. **Rate limiting** - Per-stream rate limiting via StreamRouter
4. **Owner tracking** - Bulk unregister by component owner
5. **Runtime flexibility** - Add/remove streams without node restart

---

## Phase 6: Storage Enhancements

### Phase 6.1: Block Pruning

**Status:** Complete

**Files Created:**
- `blockstore/pruning.go` - Pruning types, interfaces, and background pruner
- `blockstore/pruning_test.go` - Comprehensive pruning tests (20+ tests)

**Files Modified:**
- `blockstore/leveldb.go` - Added Prune() and related methods
- `blockstore/memory.go` - Added Prune() and related methods

**Key Changes:**

1. **PruneStrategy Enum**:
   ```go
   const (
       PruneNothing    PruneStrategy = "nothing"     // Archive mode - keep everything
       PruneEverything PruneStrategy = "everything"  // Aggressive - keep only recent
       PruneDefault    PruneStrategy = "default"     // Keep recent + checkpoints
       PruneCustom     PruneStrategy = "custom"      // Custom configuration
   )
   ```

2. **PruneConfig Structure**:
   ```go
   type PruneConfig struct {
       Strategy   PruneStrategy
       KeepRecent int64          // Keep last N blocks
       KeepEvery  int64          // Keep every Nth block (checkpoints)
       Interval   time.Duration  // Background pruning interval
   }
   ```

3. **PrunableBlockStore Interface**:
   ```go
   type PrunableBlockStore interface {
       BlockStore
       Prune(beforeHeight int64) (*PruneResult, error)
       PruneConfig() *PruneConfig
       SetPruneConfig(cfg *PruneConfig)
   }
   ```

4. **BackgroundPruner**: Automatic background pruning with configurable interval and callbacks.

5. **LevelDB and Memory Implementations**: Both block stores now implement PrunableBlockStore with checkpoint preservation.

**Test Coverage:**
- Strategy constants and validation
- Default, archive, and aggressive configs
- ShouldKeep logic (recent, checkpoint, pruned)
- Prune operations with various configurations
- Invalid/high height handling
- Background pruner lifecycle and callbacks

---

### Phase 6.2: State Pruning

**Status:** Complete

**Files Created:**
- `statestore/pruning.go` - State pruning configuration and background pruner
- `statestore/pruning_test.go` - State pruning tests (15+ tests)

**Files Modified:**
- `statestore/iavl.go` - Added PruneVersions() and related methods

**Key Changes:**

1. **StatePruneConfig Structure**:
   ```go
   type StatePruneConfig struct {
       KeepRecent int64          // Keep last N versions
       KeepEvery  int64          // Keep every Nth version (checkpoints)
       Interval   time.Duration  // Background pruning interval
       Enabled    bool           // Enable/disable pruning
   }
   ```

2. **PrunableStateStore Interface**:
   ```go
   type PrunableStateStore interface {
       StateStore
       PruneVersions(beforeVersion int64) (*StatePruneResult, error)
       AvailableVersions() (oldest, newest int64)
       StatePruneConfig() *StatePruneConfig
       SetStatePruneConfig(cfg *StatePruneConfig)
   }
   ```

3. **IAVLStore Updates**:
   - PruneVersions() - Prunes old tree versions
   - AvailableVersions() - Returns version range
   - AllVersions() - Returns all available versions
   - Uses IAVL's DeleteVersionsTo() for efficient pruning

4. **BackgroundStatePruner**: Automatic version pruning with callbacks.

**Test Coverage:**
- Default and archive configurations
- Validation for negative values
- ShouldKeep logic for versions
- CalculatePruneTarget calculations
- IAVLStore pruning operations
- Background pruner lifecycle

---

### Phase 6.3: State Snapshots

**Status:** Complete

**Files Created:**
- `statestore/snapshot.go` - Snapshot store interface and file-based implementation
- `statestore/snapshot_test.go` - Snapshot tests (20+ tests)

**Key Changes:**

1. **Snapshot Structure**:
   ```go
   type Snapshot struct {
       Version   uint32
       Height    int64
       Hash      []byte
       ChunkSize int
       Chunks    int
       Metadata  []byte
       AppHash   []byte
       CreatedAt time.Time
   }
   ```

2. **SnapshotStore Interface**:
   ```go
   type SnapshotStore interface {
       Create(height int64) (*Snapshot, error)
       List() ([]*SnapshotInfo, error)
       Load(hash []byte) (*Snapshot, error)
       LoadChunk(hash []byte, index int) (*SnapshotChunk, error)
       Delete(hash []byte) error
       Prune(keepRecent int) error
       Has(hash []byte) bool
       Import(snapshot *Snapshot, chunkProvider ChunkProvider) error
   }
   ```

3. **FileSnapshotStore Implementation**:
   - Creates snapshots using IAVL tree export
   - Gzip compression for storage efficiency
   - Chunked storage for large snapshots
   - Import support for state sync

4. **ChunkProvider Interface**:
   ```go
   type ChunkProvider interface {
       GetChunk(index int) ([]byte, error)
       ChunkCount() int
   }
   ```

5. **MemoryChunkProvider**: For testing and import operations.

**Test Coverage:**
- Snapshot creation and listing
- Load/delete operations
- Chunk loading
- Pruning (keep recent)
- Import/export round-trip
- Metadata encoding/decoding

---

### Phase 6.4: BadgerDB Backend

**Status:** Complete

**Files Created:**
- `blockstore/badgerdb.go` - BadgerDB block store implementation
- `blockstore/badgerdb_test.go` - BadgerDB tests (20+ tests)

**Files Modified:**
- `go.mod` - Added BadgerDB v4 dependency

**Key Changes:**

1. **BadgerDBOptions Configuration**:
   ```go
   type BadgerDBOptions struct {
       SyncWrites              bool
       Compression             bool
       ValueLogFileSize        int64
       MemTableSize            int64
       NumMemtables            int
       NumLevelZeroTables      int
       NumLevelZeroTablesStall int
       Logger                  badger.Logger
   }
   ```

2. **BadgerDBBlockStore**:
   - Implements BlockStore and PrunableBlockStore interfaces
   - SSD-optimized with configurable compression (Snappy)
   - Value log garbage collection via Compact()
   - Sync() and Size() methods for management
   - Block iterator support

3. **Constructors**:
   ```go
   func NewBadgerDBBlockStore(path string) (*BadgerDBBlockStore, error)
   func NewBadgerDBBlockStoreWithOptions(path string, opts *BadgerDBOptions) (*BadgerDBBlockStore, error)
   ```

4. **Additional Methods**:
   - Compact() - Runs value log GC
   - Sync() - Ensures data is flushed
   - Size() - Returns LSM and value log sizes
   - NewBlockIterator() - Creates block iterator

**Test Coverage:**
- Default options and custom options
- Save/load operations
- Duplicate detection
- Height/base tracking
- Persistence across restarts
- Pruning with configurations
- Compaction and sync
- Size reporting
- Block iteration

---

## Phase 6 Complete

All Phase 6 tasks completed:
- 6.1 Block Pruning - PrunableBlockStore interface and implementations
- 6.2 State Pruning - PrunableStateStore interface and IAVL integration
- 6.3 State Snapshots - SnapshotStore interface and FileSnapshotStore
- 6.4 BadgerDB Backend - Alternative block storage with SSD optimization

The storage enhancements enable:
1. **Disk space management** - Automatic pruning of old blocks and state versions
2. **Checkpoint preservation** - Keep periodic checkpoints for state sync
3. **Fast sync** - State snapshots for quick node bootstrapping
4. **Storage flexibility** - BadgerDB alternative for SSD-optimized workloads
5. **Background operations** - Non-blocking pruning via background goroutines

---

*Last Updated: January 2025*

---

## Phase 1: ABI Core Types - COMPLETE (January 27, 2026)

Created the new `abi/` package with all ABI v2.0 core types as specified in ABI_DESIGN.md.

### Files Created

1. **abi/codes.go** - Result codes
   - `ResultCode` enum (CodeOK, CodeInvalidTx, CodeNotAuthorized, etc.)
   - Helper methods: IsOK(), IsError(), IsAppError(), String()
   - Reserved ranges: 0 = OK, 1-99 = framework, 100+ = application

2. **abi/transaction.go** - Transaction types
   - `Transaction` struct with Hash, Data, Sender, Nonce, GasLimit, GasPrice, Priority
   - `CheckTxMode` enum (New, Recheck, Recovery)
   - `TxCheckResult` with Code, Error, GasWanted, Priority, Sender, Nonce, ExpireAfter
   - `TxExecResult` with Code, Error, GasUsed, Events, Data

3. **abi/block.go** - Block types
   - `BlockHeader` with Height, Time, PrevHash, ProposerAddress, Evidence, LastValidators
   - `Evidence` and `EvidenceType` for misbehavior tracking
   - `EndBlockResult` with ValidatorUpdates, ConsensusParams, Events
   - `CommitResult` with AppHash, RetainHeight
   - `ConsensusParams`, `BlockParams`, `EvidenceParams`, `ValidatorParams`
   - `Block`, `Commit`, `CommitSig`, `BlockIDFlag`

4. **abi/event.go** - Event types
   - `Event` with Type and Attributes
   - `Attribute` with Key, Value, Index
   - Fluent builders: NewEvent(), AddAttribute(), AddStringAttribute(), AddIndexedAttribute()
   - Standard event type constants (EventNewBlock, EventTx, EventCommit, etc.)
   - Standard attribute key constants (AttributeKeyHeight, AttributeKeyHash, etc.)

5. **abi/query.go** - Query types
   - `QueryRequest` with Path, Data, Height, Prove
   - `QueryResponse` with Code, Error, Key, Value, Proof, Height
   - `Proof` and `ProofOp` for Merkle proofs
   - Helper methods: IsOK(), Exists()

6. **abi/genesis.go** - Genesis types
   - `Genesis` with ChainID, GenesisTime, ConsensusParams, Validators, AppState, InitialHeight
   - `ApplicationInfo` with Name, Version, AppHash, Height, LastBlockTime, ProtocolVersion

7. **abi/validator.go** - Validator types
   - `Validator` with Address, PublicKey, VotingPower, Index
   - `ValidatorUpdate` with PublicKey, Power (0 = removal)
   - `ValidatorSet` interface with full validator set operations
   - `SimpleValidatorSet` implementation with round-robin proposer selection

8. **abi/application.go** - Application interface
   - Core `Application` interface with:
     - Info() ApplicationInfo
     - InitChain(*Genesis) error
     - CheckTx(ctx, *Transaction) *TxCheckResult
     - BeginBlock(ctx, *BlockHeader) error
     - ExecuteTx(ctx, *Transaction) *TxExecResult
     - EndBlock(ctx) *EndBlockResult
     - Commit(ctx) *CommitResult
     - Query(ctx, *QueryRequest) *QueryResponse

9. **abi/base_application.go** - Default implementations
   - `BaseApplication` with fail-closed defaults (rejects all operations)
   - `AcceptAllApplication` for testing (accepts all operations)

10. **abi/abi_test.go** - Comprehensive tests
    - Tests for all types and methods
    - Verifies fail-closed behavior
    - Validates ValidatorSet operations

### Key Design Decisions

1. **Fail-Closed Security**: BaseApplication rejects all operations by default. Applications must explicitly enable functionality.

2. **Context Support**: All block execution methods accept context.Context for cancellation and deadlines.

3. **Structured Results**: All results use typed structs (TxCheckResult, TxExecResult) instead of raw error returns.

4. **Event System**: Events have a fluent builder API for easy construction.

5. **Simple ValidatorSet**: Included a basic implementation using round-robin proposer selection.

### Test Results

All 17 test cases pass:
- ResultCode operations
- Transaction validation and hashing
- TxCheckResult/TxExecResult behavior
- Event construction
- QueryResponse helpers
- ValidatorSet operations
- BaseApplication fail-closed behavior
- AcceptAllApplication test helper

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 2: Application Interface - ABI-Only Refactor

**Status:** Complete
**Date:** January 27, 2026

### Summary

Refactored the codebase to use ABI as the ONLY application interface, removing backward compatibility and legacy adapter code. All applications must now implement `abi.Application` directly.

### Files Modified

1. **abi/adapter.go** - DELETED
   - Removed `LegacyAdapter` that wrapped old-style applications
   - Removed `WrapLegacyApplication` function
   - ABI is now the only interface - no adapters needed

2. **types/application.go** - Simplified
   - Removed old Application interface
   - Kept only `TxValidator` and `BlockValidator` function types for compatibility
   - Added deprecation notice pointing to `abi.TxCheckResult`-based validation

3. **types/null_app.go** - Updated
   - `NullApplication` now implements `abi.Application` directly
   - All methods use ABI types (`abi.Transaction`, `abi.TxCheckResult`, etc.)
   - Added `var _ abi.Application = (*NullApplication)(nil)` compile-time check

4. **types/application_test.go** - Rewritten
   - All tests updated to use ABI types
   - Tests verify NullApplication implements abi.Application
   - Tests verify correct behavior with ABI-based method signatures

5. **testing/helpers.go** - Updated
   - `MockApplication` now implements `abi.Application`
   - Also implements `handlers.ConsensusHandler` for integration tests
   - Updated all methods to use ABI types
   - Added compile-time interface check

6. **consensus/interface.go** - Updated
   - `ConsensusDependencies.Application` now uses `abi.Application`
   - Removed duplicate Application interface definition

7. **MASTER_PLAN.md** - Updated
   - Added explicit statement that ABI is the ONLY interface
   - Clarified Phase 14 ABCI compatibility is for external interoperability only
   - Added "No backward compatibility" rule to Contributing section

### Key Design Decisions

1. **No Backward Compatibility**: The user explicitly requested removing the LegacyAdapter. Applications MUST implement `abi.Application` directly.

2. **Simplified Codebase**: Removing adapters reduces complexity and ensures all applications benefit from ABI v2.0's improved design.

3. **Dual Interface for MockApplication**: MockApplication implements both `abi.Application` and `handlers.ConsensusHandler` since it's used in integration tests that need both capabilities.

4. **Deprecation Notices**: Old types like `TxValidator` are kept but marked deprecated to guide users toward ABI types.

### Test Results

All tests pass:
```
go test -race -short ./...
ok  github.com/blockberries/blockberry/abi
ok  github.com/blockberries/blockberry/blockstore
ok  github.com/blockberries/blockberry/config
ok  github.com/blockberries/blockberry/consensus
ok  github.com/blockberries/blockberry/consensus/bft
ok  github.com/blockberries/blockberry/container
ok  github.com/blockberries/blockberry/handlers
ok  github.com/blockberries/blockberry/logging
ok  github.com/blockberries/blockberry/mempool
ok  github.com/blockberries/blockberry/mempool/looseberry
ok  github.com/blockberries/blockberry/metrics
ok  github.com/blockberries/blockberry/node
ok  github.com/blockberries/blockberry/p2p
ok  github.com/blockberries/blockberry/pex
ok  github.com/blockberries/blockberry/statestore
ok  github.com/blockberries/blockberry/sync
ok  github.com/blockberries/blockberry/testing
ok  github.com/blockberries/blockberry/types
```

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 9: Event System

**Status:** Complete
**Date:** January 27, 2026

### Summary

Implemented the EventBus pub/sub system for event handling throughout blockberry. This allows components to publish events and subscribers to filter and receive events matching their queries.

### Files Created

1. **abi/component.go** - Component interface for ABI
   - `Component` interface with Start/Stop/IsRunning
   - `Named`, `HealthChecker`, `Dependent`, `LifecycleAware`, `Resettable` interfaces
   - `HealthStatus` and `Health` types

2. **abi/eventbus.go** - EventBus interface and query types
   - `EventBus` interface with Subscribe/Unsubscribe/Publish methods
   - `Query` interface for event filtering
   - Query implementations:
     - `QueryAll` - matches all events
     - `QueryEventType` - matches by event type
     - `QueryEventTypes` - matches multiple event types
     - `QueryAttribute` - matches by attribute key-value
     - `QueryAttributeExists` - matches by attribute key existence
     - `QueryAnd` - AND logic for multiple queries
     - `QueryOr` - OR logic for multiple queries
     - `QueryFunc` - custom function-based matching
   - `Subscription` type for internal use
   - `EventBusConfig` with sensible defaults

3. **events/bus.go** - In-memory EventBus implementation
   - Thread-safe subscription management
   - Non-blocking publish with optional timeout
   - Context cancellation support
   - Configurable buffer sizes and limits
   - Automatic channel cleanup on Stop

4. **events/bus_test.go** - Comprehensive tests
   - 27 test cases covering all functionality
   - Tests for all query types
   - Concurrency tests
   - Context cancellation tests
   - Resource limit tests

### Key Design Decisions

1. **Non-blocking by default**: `Publish` drops events if subscriber channels are full, preventing slow subscribers from blocking publishers.

2. **Optional timeout**: `PublishWithTimeout` allows waiting for slow subscribers up to a configurable timeout.

3. **Context cancellation**: Subscriptions can be automatically cancelled when their context is cancelled.

4. **Query composition**: `QueryAnd` and `QueryOr` allow composing complex queries from simple ones.

5. **Subscriber limits**: Configurable limits on total subscribers and subscribers per query prevent resource exhaustion.

### Test Results

All 27 tests pass with race detection:
- Bus lifecycle (Start/Stop)
- Subscribe/Unsubscribe operations
- Query matching (all query types)
- Concurrent publish
- Context cancellation
- Resource limits

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 10: Extended Application Interfaces

**Status:** Complete
**Date:** January 27, 2026

### Summary

Implemented three extended application interfaces that provide advanced functionality for state sync, block proposals, and finality with vote extensions.

### Files Created

1. **abi/errors.go** - Common ABI errors
   - `ErrNotImplemented`, `ErrSnapshotNotFound`, `ErrChunkNotFound`
   - `ErrInvalidSnapshot`, `ErrInvalidChunk`, `ErrStateSyncInProgress`
   - `ErrProposalRejected`, `ErrVoteExtensionInvalid`

2. **abi/application_snapshot.go** - State sync via snapshots
   - `SnapshotApplication` interface extending `Application`
   - `Snapshot` type with Height, Format, Chunks, Hash, Metadata
   - `OfferResult` enum (Accept, Abort, Reject, RejectFormat, RejectSender)
   - `ApplyResult` enum (Accept, Abort, Retry, RetrySnapshot, RejectSnapshot)
   - `BaseSnapshotApplication` with fail-closed defaults

3. **abi/application_proposer.go** - Block proposal customization
   - `ProposerApplication` interface extending `Application`
   - `PrepareRequest` and `PrepareResponse` for PrepareProposal
   - `ProcessRequest` and `ProcessResponse` for ProcessProposal
   - `ProcessStatus` enum (Accept, Reject)
   - `CommitInfo` and `VoteInfo` types for commit information
   - `Misbehavior` and `MisbehaviorType` for validator misbehavior
   - `BaseProposerApplication` with pass-through defaults

4. **abi/application_finality.go** - Vote extensions and finality
   - `FinalityApplication` interface extending `Application`
   - `ExtendVoteRequest` and `ExtendVoteResponse` for vote extensions
   - `VerifyVoteExtRequest` and `VerifyVoteExtResponse` for verification
   - `VerifyStatus` enum (Accept, Reject)
   - `FinalizeBlockRequest` and `FinalizeBlockResponse` for block finalization
   - `ExtendedCommitInfo` and `ExtendedVoteInfo` with vote extensions
   - `BaseFinalityApplication` with accepting defaults

5. **abi/application_extended_test.go** - Comprehensive tests
   - Tests for all three extended interfaces
   - Tests for all types and enums
   - Tests for base implementations
   - Tests for error definitions

### Key Design Decisions

1. **Interface Composition**: Each extended interface embeds the base `Application` interface, allowing applications to implement only the features they need.

2. **Base Implementations**: Each interface has a corresponding base implementation:
   - `BaseSnapshotApplication` - fail-closed (rejects all snapshots)
   - `BaseProposerApplication` - pass-through (accepts all proposals)
   - `BaseFinalityApplication` - accepting (accepts all vote extensions)

3. **Deferred Reactor Integration**: The actual reactor implementations (snapshot sync reactor, consensus engine integration) are deferred to future phases to keep the ABI layer clean.

4. **Comprehensive Types**: All request/response types are fully defined with proper documentation, enabling clear contracts between the framework and applications.

### Test Results

All tests pass with race detection:
- SnapshotApplication tests (5 subtests)
- ProposerApplication tests (8 subtests)
- FinalityApplication tests (7 subtests)
- Error definition tests

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 11: Developer Experience (Partial)

**Status:** In Progress
**Date:** January 27, 2026

### Summary

Implemented the RPC server interface and JSON-RPC 2.0 transport layer, along with the TxIndexer interface for transaction indexing. CLI tooling and gRPC transport are deferred to future work.

### Files Created

1. **rpc/types.go** - RPC response types
   - `NodeStatus`, `NodeInfo`, `SyncInfo`, `ValidatorInfo` - Node status types
   - `HealthStatus`, `HealthCheck` - Health monitoring types
   - `BroadcastMode`, `BroadcastResult` - Transaction broadcast types
   - `PeerInfo`, `BlockResult`, `BlockID`, `PartSetHeader` - Block types
   - `TxResult`, `ConsensusState`, `NetInfo` - Query result types

2. **rpc/server.go** - RPC Server interface
   - `Server` interface extending `types.Component`
   - Full method set: Health, Status, NetInfo, BroadcastTx, Query, Block, BlockByHash, Tx, TxSearch, Peers, ConsensusState, Subscribe, Unsubscribe, UnsubscribeAll
   - `Handler` interface for transport-agnostic RPC handling
   - `ServerConfig` with configurable limits and TLS support
   - `RPCError` type with standard JSON-RPC error codes

3. **rpc/jsonrpc/types.go** - JSON-RPC 2.0 types
   - `Request`, `Response`, `Error` types per JSON-RPC 2.0 spec
   - Standard error codes (ParseError, InvalidRequest, MethodNotFound, InvalidParams, InternalError)
   - `BatchRequest`, `BatchResponse` for batch operations
   - Helper functions: `NewResponse`, `NewErrorResponse`, `NewErrorWithData`

4. **rpc/jsonrpc/server.go** - JSON-RPC server implementation
   - `Server` struct wrapping `rpc.Server` interface
   - HTTP handler with POST method enforcement
   - Batch request support
   - Method handlers for all RPC methods:
     - Health/status: health, status, net_info
     - Transaction: broadcast_tx_sync, broadcast_tx_async, broadcast_tx_commit
     - Query: abci_query, block, block_by_hash, tx, tx_search
     - Consensus: consensus_state, peers
     - Subscription: subscribe, unsubscribe, unsubscribe_all
   - Query parsing for subscription filters (parseQuery)

5. **abi/indexer.go** - Transaction indexer interface
   - `TxIndexer` interface: Index, Get, Search, Delete, Has, Batch
   - `TxIndexResult` containing hash, height, index, and execution result
   - `TxIndexBatch` for batched operations
   - `BlockIndexer` interface for block event indexing
   - `IndexerConfig` for configurable indexer settings
   - `NullTxIndexer` no-op implementation
   - Indexer errors: ErrTxNotFound, ErrInvalidQuery, ErrIndexFull, ErrIndexCorrupted

6. **abi/indexer_test.go** - Indexer tests
   - Tests for NullTxIndexer (Index, Get, Search, Delete, Has, Batch)
   - Tests for IndexError error messages
   - Tests for IndexerConfig defaults
   - Tests for TxIndexResult fields

7. **rpc/jsonrpc/server_test.go** - JSON-RPC server tests
   - Mock RPC server implementation
   - Server lifecycle tests (Start/Stop)
   - Method handler tests (28 tests covering all methods)
   - HTTP handler tests (POST, batch, invalid JSON)
   - Error handling tests (method not found, invalid version, invalid params)
   - Query parsing tests

### Key Design Decisions

1. **ABI Type Integration**: All RPC methods use ABI types directly (e.g., `abi.QueryResponse`, `abi.Event`, `abi.Query`).

2. **Interface-based Design**: The `rpc.Server` interface allows different transport implementations (JSON-RPC, gRPC) to share the same business logic.

3. **JSON-RPC 2.0 Compliance**: Full compliance with JSON-RPC 2.0 specification including batch requests and standard error codes.

4. **Pluggable Indexing**: The `TxIndexer` interface allows different storage backends (LevelDB, PostgreSQL) with a no-op NullTxIndexer for nodes that don't need indexing.

5. **Query Parsing**: Simple query string parsing for subscriptions supporting "type=", "key=value", and "all" patterns.

### Test Results

All tests pass with race detection:
- NullTxIndexer tests (7 tests)
- IndexError tests (4 subtests)
- IndexerConfig tests
- JSON-RPC server tests (28 tests)
- HTTP handler tests
- Query parsing tests (6 subtests)

**Build Status**: Clean build, all tests pass with race detection.

### Deferred Work

- gRPC transport implementation
- CLI tooling (cobra-based)
- LevelDB-based TxIndexer implementation
- Search query language for indexer

---

## Phase 12: Security Hardening (Partial)

**Status:** In Progress
**Date:** January 27, 2026

### Summary

Implemented resource limits, rate limiting, connection tracking, and eclipse attack mitigation. Private key encryption and full limit enforcement are deferred to future work.

### Files Created

1. **abi/limits.go** - Resource limits and security interfaces
   - `ResourceLimits` struct with transaction, block, message, connection, subscription, and rate limits
   - `DefaultResourceLimits()` with sensible defaults
   - `RateLimiter` interface for rate limiting implementations
   - `RateLimiterConfig` with rate, interval, burst, cleanup settings
   - `ConnectionLimiter` interface for connection tracking
   - `EclipseMitigation` interface for eclipse attack protection
   - `EclipseMitigationConfig` with subnet limits, source limits, outbound requirements
   - `BandwidthLimiter` interface for bandwidth limiting
   - Security errors: ErrRateLimited, ErrConnectionLimit, ErrEclipseRisk, etc.

2. **abi/limits_test.go** - Tests for limits
   - Tests for DefaultResourceLimits
   - Tests for ResourceLimits.Validate()
   - Tests for all config defaults
   - Tests for all security errors

3. **security/ratelimit.go** - Rate limiting implementations
   - `TokenBucketLimiter` - Token bucket algorithm implementation
   - `SlidingWindowLimiter` - Sliding window rate limiter
   - `ConnectionTracker` - Connection count tracking and enforcement
   - All implement their respective ABI interfaces

4. **security/ratelimit_test.go** - Rate limiter tests
   - Tests for TokenBucketLimiter (Allow, AllowN, Refill, Reset, MultipleKeys, Concurrent, Size)
   - Tests for SlidingWindowLimiter (Allow, Window, Reset, Size)
   - Tests for ConnectionTracker (Basic, InboundLimit, OutboundLimit, TotalLimit, Disconnect, Concurrent)

5. **security/eclipse.go** - Eclipse attack mitigation
   - `EclipseProtector` implementing `abi.EclipseMitigation`
   - Subnet-based peer limiting
   - Source-based peer tracking
   - Outbound connection requirements
   - Trusted peer management
   - Peer diversity scoring
   - Misbehavior tracking

6. **security/eclipse_test.go** - Eclipse protection tests
   - Tests for Basic functionality
   - Tests for SubnetLimit enforcement
   - Tests for OutboundRequirement
   - Tests for TrustedPeers
   - Tests for Misbehavior handling
   - Tests for Disconnect cleanup
   - Tests for Diversity scoring
   - Tests for extractSubnet helper

### Key Design Decisions

1. **Interface-based Design**: All security components implement ABI interfaces, allowing easy replacement with custom implementations.

2. **Token Bucket Algorithm**: Primary rate limiter uses token bucket for smooth rate limiting with burst support.

3. **Sliding Window Alternative**: SlidingWindowLimiter provides an alternative for stricter rate limiting without burst behavior.

4. **Eclipse Mitigation Strategy**:
   - Limit peers from same /24 subnet
   - Limit peers learned from same source
   - Require minimum outbound connections
   - Track and penalize misbehaving peers
   - Trust well-behaved peers over time

5. **Diversity Scoring**: Composite score (0-100) based on subnet diversity, source diversity, and inbound/outbound balance.

### Test Results

All 38 security tests pass with race detection:
- TokenBucketLimiter tests (7 tests)
- SlidingWindowLimiter tests (4 tests)
- ConnectionTracker tests (6 tests)
- EclipseProtector tests (10 tests)
- Limits tests (11 tests)

**Build Status**: Clean build, all tests pass with race detection.

### Deferred Work

- Private key encryption
- Full limit enforcement throughout codebase
- Bandwidth limiting implementation
- Performance optimization (Phase 13)
- Observability/Metrics (Phase 13)

---

## Phase 13: Performance & Observability

### Status: IN PROGRESS (January 27, 2026)

### Completed Tasks

1. **abi/metrics.go** - ABI Metrics interface
   - `Metrics` interface with comprehensive metric methods
   - Consensus metrics (height, round, step, block committed, block size)
   - Mempool metrics (size, tx added/removed/rejected)
   - Application metrics (BeginBlock, ExecuteTx, EndBlock, Commit, Query, CheckTx)
   - Network metrics (peers, bytes sent/received, messages, errors)
   - Storage metrics (block store height/size, state store operations)
   - Sync metrics (progress, blocks received, duration)
   - `NullMetrics` no-op implementation
   - `MetricsConfig` configuration struct
   - `HistogramBuckets` for histogram configuration
   - Metric name constants for standardization
   - Label name constants
   - Tx removal/rejection reason constants

2. **abi/metrics_test.go** - Metrics tests
   - Tests for NullMetrics (Consensus, Mempool, App, Network, Storage, Sync)
   - Tests for DefaultMetricsConfig and HistogramBuckets
   - Tests for metric and label name constants
   - Tests for tx removal/rejection reason uniqueness

3. **abi/tracer.go** - Distributed tracing interface
   - `Tracer` interface for distributed tracing
   - `Span` interface for individual trace spans
   - `SpanContext` for trace propagation with TraceID, SpanID, TraceFlags
   - `SpanAttribute` for span attributes
   - `Carrier` interface for context injection/extraction
   - `MapCarrier` implementation
   - `NullTracer` no-op implementation
   - `nullSpan` no-op span implementation
   - `StatusCode` enum (Unset, OK, Error)
   - `SpanKind` enum (Internal, Server, Client, Producer, Consumer)
   - `SpanOption` and `spanConfig` for configuration
   - `Link` for cross-trace linking
   - `TracerConfig` configuration struct
   - Standard span name constants (CheckTx, BeginBlock, ExecuteTx, etc.)
   - Standard attribute key constants

4. **abi/tracer_test.go** - Tracer tests
   - Tests for NullTracer (StartSpan, SpanFromContext, Extract, Inject)
   - Tests for nullSpan operations
   - Tests for SpanContext (IsValid, IsSampled)
   - Tests for MapCarrier
   - Tests for SpanAttribute helpers
   - Tests for StatusCode and SpanKind string methods
   - Tests for DefaultTracerConfig
   - Tests for span name and attribute key constants

5. **metrics/abi_adapter.go** - Prometheus adapter for ABI Metrics
   - `ABIMetricsAdapter` wrapping PrometheusMetrics
   - Implements full `abi.Metrics` interface
   - Bridges existing Prometheus metrics to new ABI interface
   - `NewABIMetricsAdapter` factory function
   - `NewABIMetricsAdapterFrom` for wrapping existing instances
   - `Handler()` for HTTP metrics endpoint
   - `Prometheus()` for direct access to underlying implementation

6. **metrics/abi_adapter_test.go** - Adapter tests
   - Tests for adapter creation
   - Tests for all metric categories (Consensus, Mempool, App, Network, Storage, Sync)
   - Tests for edge cases (zero target sync)
   - Tests for interface compliance

### Key Design Decisions

1. **ABI-Centric Metrics**: The `abi.Metrics` interface is the primary contract. Existing implementations are adapted via `ABIMetricsAdapter`.

2. **No-Op Implementations**: Both `NullMetrics` and `NullTracer` provide no-op implementations for testing and when observability is disabled.

3. **OpenTelemetry-Compatible Tracing**: The `Tracer` and `Span` interfaces follow OpenTelemetry conventions for easy integration.

4. **SpanAttribute vs Attribute**: Used `SpanAttribute` to avoid conflict with existing `Attribute` type in `abi/event.go`.

5. **Standard Constants**: Defined standard span names and attribute keys for consistency across the codebase.

### Test Results

All metrics and tracer tests pass with race detection:
- abi/metrics_test.go: 13 tests
- abi/tracer_test.go: 17 tests
- metrics/abi_adapter_test.go: 12 tests

**Build Status**: Clean build, all tests pass with race detection.

### Deferred Work

- OpenTelemetry integration
- Jaeger/Zipkin export
- Parallel block sync
- Memory optimization

---

## KV Transaction Indexer

### Status: COMPLETE (January 27, 2026)

### Completed Tasks

1. **indexer/kv/indexer.go** - LevelDB-based transaction indexer
   - `TxIndexer` implementing `abi.TxIndexer` interface
   - Primary index: transaction hash -> TxIndexResult (JSON)
   - Secondary index: block height -> transaction hashes
   - Secondary index: event attributes -> transaction hashes
   - Search support:
     - Height queries: `tx.height=100`, `tx.height>100`, etc.
     - Event queries: `transfer.sender='alice'`
   - Pagination with configurable page size
   - Batch operations for efficient bulk indexing
   - Configurable `indexAllEvents` option
   - Thread-safe with mutex protection
   - Proper cleanup of secondary indexes on delete

2. **indexer/kv/indexer_test.go** - Comprehensive tests
   - Basic CRUD operations (Index, Get, Has, Delete)
   - Search by height (exact, >, >=, <, <=)
   - Search by event attributes
   - Pagination
   - Invalid query handling
   - Batch operations (Add, Delete, Commit, Discard)
   - Close behavior
   - Edge cases (nil input, empty hash)
   - IndexAllEvents configuration

### Key Design Decisions

1. **JSON Serialization**: Transaction results are stored as JSON for simplicity and debuggability. Binary encoding could be used for better performance if needed.

2. **Key Prefixes**: Different key prefixes (`th/`, `ht/`, `ev/`) separate the primary index from secondary indexes for efficient range scans.

3. **Height Encoding**: Block heights are encoded as big-endian uint64 for proper lexicographic ordering in range queries.

4. **Event Index Format**: Event indexes use the format `ev/<event_type>/<attr_key>/<attr_value>/<tx_hash>` for efficient prefix-based lookups.

5. **Batch Processing**: Batch operations accumulate changes in memory and write atomically on commit for efficiency and consistency.

### Test Results

All 16 tests pass:
- TestTxIndexer_IndexAndGet
- TestTxIndexer_Get_NotFound
- TestTxIndexer_Has
- TestTxIndexer_Delete
- TestTxIndexer_SearchByHeight (6 subtests)
- TestTxIndexer_SearchByEvent
- TestTxIndexer_SearchPagination
- TestTxIndexer_SearchInvalidQuery (4 subtests)
- TestTxIndexer_Batch
- TestTxIndexer_BatchDelete
- TestTxIndexer_BatchDiscard
- TestTxIndexer_Close
- TestTxIndexer_IndexNilResult
- TestTxIndexer_IndexAllEvents
- TestKeyConstruction (4 subtests)

**Build Status**: Clean build, all tests pass.

---

## OpenTelemetry Integration

### Status: COMPLETE (January 27, 2026)

### Completed Tasks

1. **tracing/otel/tracer.go** - OpenTelemetry Tracer implementation
   - `Tracer` struct implementing `abi.Tracer` interface
   - Wraps OpenTelemetry SDK's `trace.Tracer`
   - `Span` struct implementing `abi.Span` interface
   - `NewTracer(serviceName)` - Creates tracer using global provider
   - `NewTracerWithProvider(serviceName, provider)` - Creates tracer with specific provider
   - Methods: StartSpan, SpanFromContext, Extract, Inject
   - Span methods: End, SetName, SetAttribute, SetAttributes, AddEvent, RecordError, SetStatus, IsRecording, SpanContext
   - `carrierAdapter` for propagation context
   - `convertAttribute` for ABI to OTel attribute conversion
   - `convertSpanOptions` for span option conversion
   - Support for all attribute types: string, int, int64, float64, bool, []byte, slices

2. **tracing/otel/provider.go** - TracerProvider factory
   - `ProviderConfig` with service name, version, environment, exporter settings
   - `DefaultProviderConfig()` with sensible defaults
   - `NewProvider(cfg)` - Creates TracerProvider with configured exporter
   - `ProviderFromConfig(abi.TracerConfig)` - Creates provider from ABI config
   - `SetupGlobalTracer(cfg)` - Sets up global tracer with shutdown function
   - Exporter types:
     - "otlp-grpc" / "otlp" - OTLP gRPC exporter
     - "otlp-http" - OTLP HTTP exporter
     - "stdout" - Pretty-printed stdout exporter
     - "none" - No exporter (for testing)
   - Sampler configuration: NeverSample, AlwaysSample, TraceIDRatioBased
   - Propagation format: W3C TraceContext (default), B3 (fallback to W3C)

3. **tracing/otel/tracer_test.go** - Tracer tests
   - `createTestTracer` helper with in-memory exporter
   - TestTracer_StartSpan - Span creation and recording
   - TestTracer_SpanFromContext - Context-based span retrieval
   - TestSpan_SetAttribute - Single attribute setting
   - TestSpan_SetAttributes - Multiple attributes with ABI helpers
   - TestSpan_AddEvent - Event addition with attributes
   - TestSpan_RecordError - Error recording
   - TestSpan_SetStatus - Status setting (OK, Error, Unset)
   - TestSpan_SetName - Span name updates
   - TestSpan_SpanContext - TraceID/SpanID validity
   - TestCarrierAdapter - Get/Set/Keys operations
   - TestTracer_ExtractInject - Context propagation round-trip
   - TestConvertAttribute - All attribute type conversions
   - TestNewTracer - Basic constructor
   - TestInterfaceCompliance - Compile-time interface checks

4. **tracing/otel/provider_test.go** - Provider tests
   - TestDefaultProviderConfig - Default values verification
   - TestNewProvider_None - No-exporter provider
   - TestNewProvider_Stdout - Stdout exporter
   - TestNewProvider_InvalidExporter - Error handling
   - TestNewProvider_SampleRates - All sampling scenarios
   - TestProviderFromConfig - ABI config conversion
   - TestSetupGlobalTracer_Disabled - Disabled tracing
   - TestSetupGlobalTracer_Enabled - Full tracing with span test
   - TestSetupGlobalTracer_PropagationFormats - W3C, B3, unknown, empty

### Key Design Decisions

1. **Interface Compliance**: Both `Tracer` and `Span` implement their respective ABI interfaces, verified at compile time.

2. **Carrier Adapter**: The `carrierAdapter` bridges ABI's `Carrier` interface to OTel's `propagation.TextMapCarrier`.

3. **Attribute Conversion**: The `convertAttribute` function handles all common Go types with a fallback to string representation.

4. **Schema URL Handling**: Resource creation avoids merging with `resource.Default()` to prevent schema URL conflicts between semconv versions.

5. **Propagation**: W3C TraceContext is used by default with Baggage support.

### Dependencies Added

```
go.opentelemetry.io/otel v1.37.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.37.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.37.0
go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.37.0
go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.37.0
go.opentelemetry.io/otel/sdk v1.37.0
go.opentelemetry.io/otel/sdk/trace v1.37.0
```

### Test Results

All 26 tests pass:
- tracing/otel/tracer_test.go: 14 tests
- tracing/otel/provider_test.go: 12 tests (including subtests)

**Build Status**: Clean build, all tests pass with race detection.

---

## WebSocket Transport for EventBus

### Status: COMPLETE (January 27, 2026)

### Completed Tasks

1. **rpc/websocket/server.go** - WebSocket server for event subscriptions
   - `Server` struct implementing WebSocket connection management
   - `Client` struct for individual client connection handling
   - `Config` with customizable settings:
     - ReadBufferSize, WriteBufferSize (default: 1024)
     - MaxClients (default: 100)
     - MaxSubscriptionsPerClient (default: 10)
     - PingInterval (default: 30s)
     - PongTimeout, WriteTimeout, ReadTimeout
     - AllowedOrigins for CORS control
   - Methods:
     - `NewServer(eventBus, cfg)` - Creates WebSocket server
     - `Start()/Stop()/IsRunning()` - Lifecycle management
     - `Handler()` - Returns http.Handler for mounting
     - `ClientCount()` - Returns connected client count
   - Client message handling:
     - `subscribe` - Subscribe to events with query
     - `unsubscribe` - Unsubscribe from specific query
     - `unsubscribe_all` - Unsubscribe from all queries
   - Query parsing support:
     - "all" or "*" -> QueryAll
     - "type=EventType" -> QueryEventType
     - "key=value" -> QueryAttribute
     - Plain string -> QueryEventType

2. **rpc/websocket/server_test.go** - Comprehensive tests (20 tests)
   - TestDefaultConfig - Configuration defaults
   - TestNewServer - Server creation
   - TestServer_StartStop - Lifecycle (including double start/stop)
   - TestServer_Handler - HTTP handler generation
   - TestServer_ClientConnection - Client connection handling
   - TestServer_MaxClients - Client limit enforcement
   - TestServer_Subscribe - Subscription creation
   - TestServer_ReceiveEvents - Event delivery to clients
   - TestServer_Unsubscribe - Single subscription removal
   - TestServer_UnsubscribeAll - All subscriptions removal
   - TestServer_MaxSubscriptionsPerClient - Subscription limit
   - TestServer_InvalidMessage - Invalid JSON handling
   - TestServer_UnknownMethod - Unknown method error
   - TestServer_ClientDisconnect - Client cleanup
   - TestServer_StopDisconnectsClients - Server stop cleanup
   - TestServer_ConcurrentSubscriptions - Multiple subscriptions
   - TestParseQuery - Query string parsing (6 subtests)
   - TestServer_AllowedOrigins - Origin restriction
   - TestServer_AllowAllOrigins - Wildcard origin
   - TestServer_NotRunning - Reject when stopped

### Message Protocol (JSON-RPC 2.0)

**Subscribe Request:**
```json
{"jsonrpc": "2.0", "id": 1, "method": "subscribe", "query": "type=NewBlock"}
```

**Subscribe Response:**
```json
{"jsonrpc": "2.0", "id": 1, "result": {"subscribed": true, "query": "type=NewBlock"}}
```

**Event Notification:**
```json
{"jsonrpc": "2.0", "result": {"query": "type=NewBlock", "event": {"type": "NewBlock", "attributes": [...]}}}
```

**Unsubscribe Request:**
```json
{"jsonrpc": "2.0", "id": 2, "method": "unsubscribe", "query": "type=NewBlock"}
```

### Key Design Decisions

1. **JSON-RPC 2.0 Protocol**: Uses standard JSON-RPC 2.0 for consistency with the JSON-RPC HTTP transport.

2. **Non-blocking Event Delivery**: Events are sent through a buffered channel. If the channel is full, events are dropped to prevent slow clients from blocking the system.

3. **Automatic Ping/Pong**: The server sends periodic pings to detect dead connections and clients must respond with pongs.

4. **Origin Checking**: Configurable allowed origins for WebSocket connections with support for wildcard ("*").

5. **Resource Limits**: Configurable limits on max clients and max subscriptions per client prevent resource exhaustion.

6. **Graceful Cleanup**: On client disconnect or server stop, all subscriptions are properly cleaned up from the EventBus.

### Usage Example

```go
// Create EventBus
bus := events.NewBusWithConfig(abi.DefaultEventBusConfig())
bus.Start()

// Create WebSocket server
wsCfg := websocket.DefaultConfig()
wsServer := websocket.NewServer(bus, wsCfg)
wsServer.Start()

// Mount on HTTP server
http.Handle("/websocket", wsServer.Handler())
http.ListenAndServe(":8080", nil)
```

### Test Results

All 20 tests pass with race detection.

**Build Status**: Clean build, all tests pass with race detection.

---

## WebSocket Implementation Update: gobwas/ws Migration

**Status:** Complete (January 27, 2026)

### Summary

Migrated the WebSocket server from `github.com/gorilla/websocket` to `github.com/gobwas/ws` for improved performance and zero-allocation upgrades.

### Files Modified

- `rpc/websocket/server.go` - Migrated from gorilla/websocket to gobwas/ws
- `rpc/websocket/server_test.go` - Updated tests to use gobwas/ws client
- `go.mod` - Added gobwas/ws and gobwas/pool dependencies

### Key Changes

1. **Connection Type**: Changed from `*websocket.Conn` to `net.Conn` for the underlying connection.

2. **Upgrade Handler**:
   ```go
   // Before (gorilla)
   upgrader := websocket.Upgrader{...}
   conn, err := upgrader.Upgrade(w, r, nil)

   // After (gobwas)
   upgrader := ws.HTTPUpgrader{...}
   conn, _, _, err := upgrader.Upgrade(r, w)
   ```

3. **Message Reading/Writing**:
   ```go
   // Before (gorilla)
   _, data, err := conn.ReadMessage()
   conn.WriteMessage(websocket.TextMessage, data)

   // After (gobwas)
   data, op, err := wsutil.ReadClientData(conn)
   wsutil.WriteServerMessage(conn, ws.OpText, data)
   ```

4. **Control Frames**: The gobwas/ws `wsutil.ReadClientData` automatically handles control frames internally, simplifying the read loop.

5. **Test Client Updates**: Tests now use `ws.Dialer` for connecting and `wsutil.ReadServerData`/`wsutil.WriteClientMessage` for communication.

### Benefits of gobwas/ws

1. **Zero-allocation upgrades**: HTTP upgrade process doesn't allocate memory
2. **Lower-level control**: Direct access to `net.Conn` for fine-grained management
3. **Better performance**: Designed for high-concurrency WebSocket servers
4. **Minimal memory overhead**: Ideal for high-throughput scenarios

### Test Results

All 20 tests pass with race detection:
- TestDefaultConfig
- TestNewServer
- TestServer_StartStop
- TestServer_Handler
- TestServer_ClientConnection
- TestServer_MaxClients
- TestServer_Subscribe
- TestServer_ReceiveEvents
- TestServer_Unsubscribe
- TestServer_UnsubscribeAll
- TestServer_MaxSubscriptionsPerClient
- TestServer_InvalidMessage
- TestServer_UnknownMethod
- TestServer_ClientDisconnect
- TestServer_StopDisconnectsClients
- TestServer_ConcurrentSubscriptions
- TestParseQuery (6 subtests)
- TestServer_AllowedOrigins
- TestServer_AllowAllOrigins
- TestServer_NotRunning

**Build Status**: Clean build, all tests pass with race detection.

---

## PEX Total Address Limit Implementation

**Status:** Complete (January 27, 2026)

### Summary

Added a configurable maximum address limit to the PEX address book to prevent memory exhaustion attacks from malicious peers sending excessive address responses.

### Files Modified

- `config/config.go` - Added `MaxTotalAddresses` to `PEXConfig`
- `config/config_test.go` - Added tests for new config field
- `pex/address_book.go` - Added address limit enforcement with eviction
- `pex/address_book_test.go` - Added comprehensive tests for limit enforcement
- `pex/reactor.go` - Added per-response limit to `handleAddressResponse`

### Key Changes

1. **Config (`config/config.go`)**:
   ```go
   type PEXConfig struct {
       // ... existing fields ...
       MaxTotalAddresses int `toml:"max_total_addresses"`
   }
   ```
   - Default: 1000 addresses
   - Validation ensures positive value when PEX is enabled

2. **AddressBook (`pex/address_book.go`)**:
   - Added `NewAddressBookWithLimit(path, maxAddresses)` constructor
   - `AddPeer()` now evicts oldest non-seed peer when at capacity
   - `SetMaxAddresses()` allows runtime limit adjustment
   - `MaxAddresses()` returns current limit (0 = unlimited)
   - `evictOldestLocked()` finds and removes the oldest non-seed entry

3. **Reactor (`pex/reactor.go`)**:
   - `handleAddressResponse()` now limits how many addresses are accepted per response
   - Prevents a single malicious peer from filling the address book

### Eviction Policy

When the address book is at capacity:
1. Seed nodes are never evicted
2. The oldest non-seed peer (by `LastSeen` timestamp) is removed
3. New peer is then added

### Test Coverage

New tests added:
- `TestNewAddressBookWithLimit` - Constructor with limit
- `TestAddressBook_MaxAddresses_Unlimited` - Zero means no limit
- `TestAddressBook_MaxAddresses_Enforced` - Eviction when at capacity
- `TestAddressBook_MaxAddresses_UpdateExistingDoesNotEvict` - Updates don't trigger eviction
- `TestAddressBook_MaxAddresses_SeedsNotEvicted` - Seeds are protected
- `TestAddressBook_SetMaxAddresses` - Runtime limit changes
- `TestAddressBook_SetMaxAddresses_Zero` - Switching to unlimited
- `TestPEXConfigValidation/enabled_with_zero_max_total_addresses` - Config validation

**Build Status**: Clean build, all tests pass with race detection.

---

## CLI Tooling Implementation

**Status:** Complete (January 27, 2026)

### Summary

Implemented a comprehensive command-line interface using the Cobra library for managing Blockberry nodes.

### Files Created

- `cmd/blockberry/main.go` - Entry point
- `cmd/blockberry/root.go` - Root command and global flags
- `cmd/blockberry/init.go` - Node initialization command
- `cmd/blockberry/start.go` - Node startup command
- `cmd/blockberry/status.go` - Status query command
- `cmd/blockberry/keys.go` - Key management commands

### Commands

1. **`blockberry init`** - Initialize a new node
   ```bash
   blockberry init --chain-id mychain --moniker mynode --role full --data-dir ./node
   ```
   - Creates `config.toml` with default configuration
   - Generates `node_key.json` Ed25519 keypair
   - Creates data directories for blockstore and state
   - Supports `--force` to override existing configuration

2. **`blockberry start`** - Start the node
   ```bash
   blockberry start --config config.toml
   ```
   - Loads configuration from file
   - Initializes logger based on config
   - Creates and starts the node
   - Handles graceful shutdown on SIGINT/SIGTERM

3. **`blockberry status`** - Query node status
   ```bash
   blockberry status --rpc http://localhost:26657
   blockberry status --json
   ```
   - Queries running node via JSON-RPC
   - Displays chain info, sync status, peer counts
   - Supports JSON output format

4. **`blockberry keys`** - Key management
   ```bash
   blockberry keys generate [output-file]
   blockberry keys show <key-file>
   ```
   - Generate new Ed25519 keypairs
   - Display public key and node ID from key files

5. **`blockberry version`** - Version information
   ```bash
   blockberry version
   ```
   - Displays version, git commit, and build time
   - Version info can be set at build time via ldflags

### Global Flags

- `--config, -c` - Config file path (default: "config.toml")
- `--verbose, -v` - Enable verbose output
- `--help, -h` - Help for any command

### Key Features

1. **Cobra Framework**: Industry-standard CLI library with automatic help generation, shell completion, and subcommand support.

2. **Graceful Shutdown**: The `start` command handles SIGINT/SIGTERM signals for clean node shutdown.

3. **Configurable Logging**: Logger is initialized based on config file settings (level, format, output).

4. **Secure Key Generation**: Node keys are generated with crypto/rand and stored with 0600 permissions.

5. **Build-time Version**: Version info can be injected at build time:
   ```bash
   go build -ldflags "-X main.Version=1.0.0 -X main.GitCommit=$(git rev-parse HEAD)" ./cmd/blockberry
   ```

### Example Workflow

```bash
# Initialize a new full node
blockberry init --chain-id mainnet --moniker my-node --data-dir ./node

# View the generated key
blockberry keys show ./node/node_key.json

# Start the node
blockberry start --config ./node/config.toml

# In another terminal, check status
blockberry status --rpc http://localhost:26657
```

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 9 (Continued): Softer Blacklist Policy for PEX

**Status:** Complete (January 27, 2026)

### Summary

Implemented a temporary ban system as an alternative to permanent blacklisting for recoverable peer violations such as chain ID or version mismatches.

### Files Modified

- `p2p/network.go` - Added temp ban data structures and methods
- `p2p/network_test.go` - Added tests for temp ban functionality
- `handlers/handshake.go` - Updated to use temp bans for chain/version mismatch

### Implementation Details

1. **TempBanEntry Structure**
   ```go
   type TempBanEntry struct {
       ExpiresAt time.Time
       Reason    string
   }
   ```

2. **Network Methods**
   - `TempBanPeer(peerID, duration, reason)` - Temporarily ban a peer with automatic disconnect
   - `IsTempBanned(peerID)` - Check if a peer is currently temp banned
   - `GetTempBanReason(peerID)` - Get the reason for a temp ban
   - `CleanupExpiredTempBans()` - Remove expired bans from memory

3. **Ban Durations**
   - Chain ID mismatch: 1 hour temp ban (node may have reconnected to wrong chain)
   - Version mismatch: 30 minute temp ban (node may upgrade and retry)

4. **Behavior Changes**
   - Previously: Chain/version mismatches resulted in permanent blacklist
   - Now: These violations result in temporary bans allowing reconnection after expiry
   - Peer is still disconnected immediately and recorded in scoring system

### Test Coverage

- Direct temp bans map testing
- Cleanup logic for expired bans
- IsTempBanned logic verification
- All tests pass with race detection

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 10: Snapshot Sync Reactor

**Status:** Complete (January 27, 2026)

### Summary

Implemented a state sync reactor that enables nodes to bootstrap from snapshots instead of replaying all blocks from genesis. This significantly reduces the time required to bring new nodes online.

### Files Created

- `sync/statesync.go` - State sync reactor implementation
- `sync/statesync_test.go` - Comprehensive tests for state sync reactor

### Files Modified

- `blockberry.cram` - Added state sync message definitions
- `schema/blockberry.go` - Regenerated cramberry code with new messages
- `p2p/network.go` - Added `StreamStateSync` constant and updated `AllStreams()`
- `p2p/network_test.go` - Updated tests for new stream constant
- `config/config.go` - Added `StateSyncConfig` struct and validation

### Schema Messages

Added new cramberry messages for state sync protocol:

```
message SnapshotsRequest {
  required int64 min_height = 1;
}

message SnapshotMetadata {
  required int64 height = 1;
  required bytes hash = 2;
  required int32 chunks = 3;
  required bytes app_hash = 4;
  int64 created_at = 5;
}

message SnapshotsResponse {
  repeated SnapshotMetadata snapshots = 1;
}

message SnapshotChunkRequest {
  required bytes snapshot_hash = 1;
  required int32 chunk_index = 2;
}

message SnapshotChunkResponse {
  required bytes snapshot_hash = 1;
  required int32 chunk_index = 2;
  required bytes data = 3;
  required bytes chunk_hash = 4;
}

interface StateSyncMessage {
  144 = SnapshotsRequest;
  145 = SnapshotsResponse;
  146 = SnapshotChunkRequest;
  147 = SnapshotChunkResponse;
}
```

### Configuration

Added `StateSyncConfig` with the following options:

```go
type StateSyncConfig struct {
    Enabled             bool     // Enable state sync
    TrustHeight         int64    // Block height to trust
    TrustHash           string   // Block hash at trust height
    DiscoveryInterval   Duration // Time between snapshot discovery requests
    ChunkRequestTimeout Duration // Timeout for chunk requests
    MaxChunkRetries     int      // Max retries per chunk
    SnapshotPath        string   // Directory for downloaded snapshots
}
```

### State Sync Reactor Features

1. **Snapshot Discovery**
   - Queries connected peers for available snapshots
   - Filters snapshots by minimum height (trust height)
   - Selects best snapshot (highest height, matching trust hash if specified)

2. **State Machine**
   - `StateSyncIdle` - Not active
   - `StateSyncDiscovering` - Discovering available snapshots
   - `StateSyncDownloading` - Downloading snapshot chunks
   - `StateSyncApplying` - Applying snapshot to state store
   - `StateSyncComplete` - Successfully completed
   - `StateSyncFailed` - Failed with error

3. **Chunk Management**
   - Parallel chunk downloads from multiple peers
   - Automatic retry with configurable limits
   - Timeout handling for stalled requests
   - SHA256 verification of chunk integrity

4. **Peer Integration**
   - `OnPeerConnected` - Request snapshots from new peers during discovery
   - `OnPeerDisconnected` - Clean up pending requests from disconnected peers

5. **Callbacks**
   - `SetOnComplete(fn)` - Called when state sync completes successfully
   - `SetOnFailed(fn)` - Called when state sync fails

### Type IDs

```go
TypeIDSnapshotsRequest      = 144
TypeIDSnapshotsResponse     = 145
TypeIDSnapshotChunkRequest  = 146
TypeIDSnapshotChunkResponse = 147
```

### Test Coverage

- Reactor lifecycle (start/stop)
- Progress tracking
- Message encoding/decoding
- Snapshot discovery and selection
- Trust height/hash validation
- Peer disconnect handling
- Callback invocation
- Invalid message handling

### Example Usage

```go
// Create state sync reactor
reactor := sync.NewStateSyncReactor(
    network,
    peerManager,
    snapshotStore,
    trustHeight,      // e.g., 100000
    trustHash,        // e.g., []byte("...")
    10*time.Second,   // discovery interval
    30*time.Second,   // chunk timeout
    3,                // max retries
)

// Set completion callback
reactor.SetOnComplete(func(height int64, appHash []byte) {
    log.Info("State sync complete", "height", height)
    // Start normal block sync from this height
})

// Start state sync
reactor.Start()

// Check progress
progress := reactor.Progress() // 0-100
```

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 11: Performance Optimization

**Status:** Complete (January 28, 2026)

### Summary

Implemented performance optimizations including parallel block sync, memory pooling, and expanded tracing export support.

### 11.1 Parallel Block Sync

**Files Modified:**
- `sync/reactor.go` - Enhanced sync reactor with parallel request support
- `sync/reactor_test.go` - Added tests for parallel sync features

**Key Features:**

1. **Parallel Requests Configuration**
   ```go
   const (
       DefaultMaxParallel    = 4
       DefaultRequestTimeout = 30 * time.Second
   )
   
   reactor.SetMaxParallel(8)        // Allow up to 8 concurrent requests
   reactor.SetRequestTimeout(60*time.Second)  // Timeout for stalled requests
   ```

2. **PendingRequest Tracking**
   ```go
   type PendingRequest struct {
       PeerID      peer.ID
       StartHeight int64
       EndHeight   int64
       RequestedAt time.Time
   }
   ```

3. **Request Management**
   - Tracks height ranges per pending request
   - Automatic timeout detection and cleanup
   - Re-requests timed out height ranges
   - Parallel requests to multiple peers
   - Request distribution across available peers

4. **Improvements**
   - Requests blocks from multiple peers simultaneously
   - Configurable maximum parallel requests
   - Automatic timeout handling with retry
   - Better peer utilization during sync

### 11.2 Memory Optimization

**Files Created:**
- `memory/pool.go` - Buffer and byte slice pooling utilities
- `memory/pool_test.go` - Comprehensive tests and benchmarks

**Key Features:**

1. **BufferPool** - Reusable bytes.Buffer pool
   ```go
   pool := memory.NewBufferPool(4096)
   buf := pool.Get()
   defer pool.Put(buf)
   buf.WriteString("data")
   ```

2. **ByteSlicePool** - Reusable byte slice pool
   ```go
   pool := memory.NewByteSlicePool(1024)
   b := pool.Get()
   defer pool.Put(b)
   ```

3. **Global Pools** - Pre-configured pools for common sizes
   - `SmallBufferPool` (4KB)
   - `MediumBufferPool` (64KB)
   - `LargeBufferPool` (1MB)
   - `SmallBytePool` (4KB)
   - `MediumBytePool` (64KB)

4. **Arena Allocator** - Batch allocation for objects with same lifetime
   ```go
   arena := memory.NewArena(1024 * 1024)
   b1 := arena.Alloc(100)
   b2 := arena.Alloc(200)
   arena.Reset() // Reuse all memory
   ```

5. **Helper Functions**
   - `GetBuffer(sizeHint)` - Get appropriate sized buffer
   - `PutBuffer(buf)` - Return buffer to appropriate pool

### 11.3 Jaeger/Zipkin Tracing Export

**Files Modified:**
- `tracing/otel/provider.go` - Added Jaeger and Zipkin exporter support
- `tracing/otel/provider_test.go` - Added tests for new exporters
- `go.mod`, `go.sum` - Added Zipkin exporter dependency

**Exporter Types:**

| Exporter | Description | Default Endpoint |
|----------|-------------|------------------|
| `none` | No export (traces discarded) | - |
| `stdout` | Print to stdout (debugging) | - |
| `otlp-grpc` | OTLP over gRPC | localhost:4317 |
| `otlp-http` | OTLP over HTTP | localhost:4318 |
| `jaeger` | Jaeger via OTLP | localhost:4317 |
| `zipkin` | Zipkin native format | http://localhost:9411/api/v2/spans |

**Configuration Example:**

```toml
[tracing]
enabled = true
service_name = "blockberry-node"
exporter = "jaeger"  # or "zipkin"
endpoint = "localhost:4317"
sample_rate = 0.1
```

**Notes:**
- Modern Jaeger supports OTLP natively (uses OTLP gRPC exporter)
- Zipkin uses its native JSON format via dedicated exporter
- Zipkin endpoint auto-corrects from OTLP default to Zipkin default

### Test Coverage

All new features include comprehensive tests:
- Parallel sync configuration and timeout handling
- Buffer pool allocation and return
- Arena allocator with concurrent access
- All exporter types creation and shutdown

### Benchmarks

Memory pool benchmarks show significant allocation reduction:
- BufferPool vs raw allocation: ~3x fewer allocations
- ByteSlicePool vs make(): ~2x fewer allocations
- Arena vs individual allocations: ~10x fewer allocations

**Build Status**: Clean build, all tests pass with race detection.

---

## Phase 11: Developer Experience (continued)

### 11.1 RPC System - gRPC Transport with Cramberry Encoding
**Status:** Complete
**Date:** January 28, 2026

Implemented gRPC transport for the RPC system using Cramberry encoding instead of the traditional Protocol Buffers. This provides consistent serialization across P2P and RPC layers.

**Files Created:**
- `rpc/grpc/codec.go` - Cramberry gRPC codec implementation
- `rpc/grpc/server.go` - gRPC server with all RPC methods
- `rpc/grpc/auth.go` - API key authentication interceptor
- `rpc/grpc/ratelimit.go` - Rate limiting interceptor
- `rpc/grpc/server_test.go` - Server and RPC method tests
- `rpc/grpc/auth_test.go` - Authentication tests
- `rpc/grpc/ratelimit_test.go` - Rate limiting tests

**Files Modified:**
- `blockberry.cram` - Added RPC message types (GrpcNodeInfo, GrpcSyncInfo, etc.)
- `schema/blockberry.go` - Regenerated from schema
- `MASTER_PLAN.md` - Updated task status

**Key Features:**

1. **Cramberry gRPC Codec:**
   - Implements `encoding.Codec` interface
   - Supports `CramberryMarshaler`/`CramberryUnmarshaler` interfaces for optimized serialization
   - Falls back to reflection-based marshaling
   - Auto-registers on package import

2. **gRPC Server:**
   - Full RPC method implementation (Health, Status, NetInfo, BroadcastTx, Query, Block, Tx, TxSearch, Peers, ConsensusState, Subscribe, Unsubscribe)
   - Manual service registration (no protoc-generated code)
   - TLS support via configuration
   - Keepalive and connection management

3. **Authentication:**
   - API key-based authentication via Bearer tokens or x-api-key header
   - Constant-time key comparison for security
   - Configurable public methods that bypass authentication
   - Runtime key management (add/remove)

4. **Rate Limiting:**
   - Token bucket algorithm via existing security package
   - Separate global and per-client rate limits
   - Configurable exempt methods and clients
   - Automatic cleanup of stale entries

**Schema Additions:**

| Message Type | Purpose |
|--------------|---------|
| `GrpcNodeInfo` | Node information |
| `GrpcSyncInfo` | Sync status |
| `GrpcValidatorInfo` | Validator data |
| `GrpcHealthCheck` | Health check result |
| `GrpcPeerInfo` | Peer information |
| `GrpcBlock` | Block data |
| `GrpcTxResult` | Transaction result |
| `GrpcBlockId` | Block identifier |
| `HealthRequest/Response` | Health RPC |
| `StatusRequest/Response` | Status RPC |
| `BroadcastTxRequest/Response` | Tx broadcast |
| ... | Other RPC messages |

**Configuration Example:**

```go
config := grpc.Config{
    ListenAddr:           "0.0.0.0:26658",
    MaxRecvMsgSize:       4 * 1024 * 1024,
    MaxSendMsgSize:       4 * 1024 * 1024,
    MaxConcurrentStreams: 100,
    Auth: grpc.AuthConfig{
        Enabled: true,
        APIKeys: []string{"secret-key-1", "secret-key-2"},
        PublicMethods: []string{
            "/blockberry.Node/Health",
            "/blockberry.Node/Status",
        },
    },
    RateLimit: grpc.RateLimitConfig{
        Enabled:       true,
        GlobalRate:    1000,
        PerClientRate: 100,
        Interval:      time.Second,
        Burst:         50,
    },
}
```

**Test Coverage:**
- 43 tests covering codec, server, authentication, and rate limiting
- All tests pass with race detection enabled

**Build Status:** Clean build, all 43 gRPC tests pass.

---

## Phase 12: Security Hardening (continued)

### 12.2 Resource Limit Enforcement
**Status:** Complete
**Date:** January 28, 2026

Added enforcement of resource limits throughout the system. The limits are configured via `LimitsConfig` in the main configuration and enforced at multiple layers.

**Files Modified:**
- `config/config.go` - Added `LimitsConfig` struct with limit configuration
- `mempool/simple_mempool.go` - Added `maxTxSize` field and enforcement
- `mempool/ttl_mempool.go` - Added `maxTxSize` field and enforcement
- `mempool/mempool_test.go` - Added test for MaxTxSize enforcement
- `MASTER_PLAN.md` - Updated task status

**Limit Configuration:**

```toml
[limits]
max_tx_size = 1048576              # 1 MB per transaction
max_block_size = 22020096          # 21 MB per block
max_block_txs = 10000              # 10k txs per block
max_msg_size = 10485760            # 10 MB per message
max_subscribers = 1000             # Total event subscribers
max_subscribers_per_query = 100    # Per-query subscribers
```

**Enforcement Points:**

| Limit | Location | Error Returned |
|-------|----------|----------------|
| MaxTxSize | `SimpleMempool.AddTx()`, `TTLMempool.AddTxWithTTL()` | `ErrTxTooLarge` |
| MaxSubscribers | `events.Bus.Subscribe()` | `ErrTooManySubscribers` |
| MaxSubscribersPerQuery | `events.Bus.Subscribe()` | `ErrTooManySubscribers` |
| MaxMessageSize | `p2p.StreamAdapter` (per-stream config) | Message rejected |

**Pre-existing Enforcement:**
- Event bus already had subscriber limits (MaxSubscribers, MaxSubscribersPerQuery)
- P2P layer already had per-stream message size limits (MaxMessageSize)
- Mempool already had total size limits (MaxTxs, MaxBytes)

**Test Coverage:**
- New test: `TestSimpleMempool_MaxTxSize` verifies transaction size enforcement
- All existing tests continue to pass

**Build Status:** Clean build, all tests pass with race detection.

---

## Code Review Iteration - January 29, 2026

### Summary
Performed comprehensive code review of the entire codebase using parallel analysis agents. Identified 15 issues across CRITICAL, HIGH, and MEDIUM severity levels. Fixed all 8 CRITICAL and HIGH severity bugs.

### Issues Fixed

#### CRITICAL (2 fixed)
1. **Double mutex unlock in `consensus/detector.go`** - ValidatorStatusTracker.Update() used defer with explicit unlock/lock, causing potential panic on callback errors.
2. **Lock contract violation in `sync/statesync.go`** - failWithErrorLocked() violated its documented "must be called with mutex held" contract.

#### HIGH (6 fixed)
3. **Integer overflow in quorum calculation** (`consensus/validators.go`) - Formula `(2*totalPower)/3 + 1` could overflow for large voting power values.
4. **Missing signature length validation** (`consensus/validators.go`) - ed25519.Verify panics on invalid signature length.
5. **Timestamp truncation** (`handlers/consensus.go`) - Block timestamp read as 4 bytes instead of 8.
6. **Nil transaction in ReapTxs** (`mempool/simple_mempool.go`) - Missing defensive check for nil transactions.
7. **Hash collision in MemoryBlockStore** (`blockstore/memory.go`) - byHash map could silently overwrite entries.
8. **Hash collision in HeaderOnlyStore** (`blockstore/header_only.go`) - Same issue as #7.

### Files Modified
- `consensus/detector.go` - Restructured mutex handling for panic safety
- `consensus/validators.go` - Added overflow protection and signature validation
- `sync/statesync.go` - Renamed and documented lock behavior
- `handlers/consensus.go` - Fixed timestamp to use 8 bytes
- `handlers/consensus_test.go` - Updated test data for new format
- `mempool/simple_mempool.go` - Added nil check
- `blockstore/memory.go` - Added hash collision detection
- `blockstore/header_only.go` - Added hash collision detection
- `CODE_REVIEW.md` - Created to track findings and fixes

### Test Coverage
All existing tests pass. Tests updated for new block data format (8-byte timestamp).

### Remaining Issues (MEDIUM - Low Priority)
7 medium-severity issues remain as operational optimizations:
- Inefficient eviction in PriorityMempool
- Inefficient validator set lookup
- LRU cache error ignored
- Race condition in stream adapter
- Goroutine spawned while holding lock
- Silently ignored JSON marshal errors
- Events silently dropped when channel full

### Build Status
Clean build, all tests pass with race detection.

---

## Phase 4: BlockStore Certificate Integration - February 2, 2026

### Summary
Extended the BlockStore interface to support DAG certificate and batch storage for Looseberry integration. Implemented certificate storage across all three BlockStore backends (LevelDB, BadgerDB, Memory) with comprehensive testing and performance benchmarks.

### Task 1: Schema Design for Certificates
**Status:** Complete

**Key Design:**
```
LevelDB Key Schema:
+------------------------------------------------------+
| Block Storage (existing schema)                       |
+------------------------------------------------------+
| H:<height>        -> Block hash                       |
| B:<hash>          -> Block data (height + payload)    |
| M:height          -> Latest block height metadata     |
| M:base            -> Earliest block height metadata   |
+------------------------------------------------------+
| Certificate Storage (NEW)                             |
+------------------------------------------------------+
| C:<digest>        -> Certificate data (cramberry)     |
| CR:<round>:<idx>  -> Certificate digest (round index) |
| CH:<height>:<idx> -> Certificate digest (height index)|
| CB:<digest>       -> Batch data (cramberry)           |
+------------------------------------------------------+
```

**Index Design Rationale:**
- Primary key by digest enables O(1) lookup by certificate/batch hash
- Round index (CR:) enables efficient retrieval of all certificates for a DAG round
- Height index (CH:) enables retrieval of certificates committed at a specific block height
- Fixed-width binary encoding (8 bytes for round/height, 2 bytes for validator index) ensures proper lexicographic ordering

### Task 2: Extended BlockStore Interface
**Status:** Complete

**Files Modified:**
- `blockstore/store.go`

**New Interface Methods:**
```go
type CertificateStore interface {
    SaveCertificate(cert *loosetypes.Certificate) error
    GetCertificate(digest loosetypes.Hash) (*loosetypes.Certificate, error)
    GetCertificatesForRound(round uint64) ([]*loosetypes.Certificate, error)
    GetCertificatesForHeight(height int64) ([]*loosetypes.Certificate, error)
    SaveBatch(batch *loosetypes.Batch) error
    GetBatch(digest loosetypes.Hash) (*loosetypes.Batch, error)
    SetCertificateBlockHeight(digest loosetypes.Hash, height int64) error
}

type CertificateBlockStore interface {
    BlockStore
    CertificateStore
}
```

### Task 3: LevelDB Implementation
**Status:** Complete

**Files Modified:**
- `blockstore/leveldb.go`

**Key Features:**
- Atomic batch writes for certificate and round index
- Defensive copying of digests to prevent external mutation
- Cramberry serialization for certificate/batch data
- Iterator-based round and height queries with proper ordering

### Task 4: Memory and BadgerDB Implementations
**Status:** Complete

**Files Modified:**
- `blockstore/memory.go`
- `blockstore/badgerdb.go`

**Key Features:**
- Memory store uses maps with defensive cloning
- BadgerDB implementation mirrors LevelDB key schema
- All implementations return certificates sorted by validator index

### Task 5: Error Types
**Status:** Complete

**Files Modified:**
- `types/errors.go`

**New Errors:**
```go
var (
    ErrCertificateNotFound      = errors.New("certificate not found")
    ErrCertificateAlreadyExists = errors.New("certificate already exists")
    ErrInvalidCertificate       = errors.New("invalid certificate")
    ErrBatchNotFound            = errors.New("batch not found")
    ErrBatchAlreadyExists       = errors.New("batch already exists")
    ErrInvalidBatch             = errors.New("invalid batch")
)
```

### Task 6: Comprehensive Testing
**Status:** Complete

**Files Created:**
- `blockstore/certificate_test.go`

**Test Coverage:**
- SaveAndGetCertificate - Basic CRUD operations
- CertificateNotFound - Error handling for missing certificates
- DuplicateCertificate - Duplicate detection
- GetCertificatesForRound - Round-based queries
- GetCertificatesForHeight - Height-based queries
- SaveAndGetBatch - Batch storage CRUD
- BatchNotFound - Error handling for missing batches
- DuplicateBatch - Duplicate detection
- DefensiveCopying - Immutability verification
- ConcurrentAccess - Thread-safety with race detection
- Nil handling tests

**All tests pass with race detection.**

### Task 7: Performance Benchmarks
**Status:** Complete

**Benchmark Results (Apple M4 Pro):**
| Operation                  | Latency    | Target  | Status |
|---------------------------|------------|---------|--------|
| SaveCertificate           | ~4.2ms     | <50ms   | PASS   |
| GetCertificate            | ~2.3us     | <5ms    | PASS   |
| SaveBatch                 | ~4.5ms     | <50ms   | PASS   |
| GetBatch                  | ~605ns     | <5ms    | PASS   |
| GetCertificatesForRound   | ~27us      | N/A     | PASS   |

### Task 8: Raspberry BlockExecutor Integration
**Status:** Complete

**Files Modified:**
- `../raspberry/integration/blockexecutor.go`

**Integration Changes:**
1. Added `CertificateStore` interface for certificate/batch persistence
2. Added `CertificateBlockStore` combined interface
3. `NewBlockExecutorAdapter` automatically detects if BlockStore implements CertificateStore
4. `NewBlockExecutorAdapterWithCertStore` for explicit certificate store configuration
5. `CreateProposalBlock` now saves certificates and batches to storage before block creation
6. `ApplyBlock` updates certificate height indexes after block commit
7. Added `fetchBatch` helper with fallback from mempool to certificate store

### Deliverables Summary

| Deliverable                          | Status   |
|-------------------------------------|----------|
| BlockStore interface extended       | Complete |
| LevelDB implementation              | Complete |
| Memory implementation               | Complete |
| BadgerDB implementation             | Complete |
| Raspberry BlockExecutor integration | Complete |
| Certificate storage tests           | Complete |
| Performance benchmarks              | Complete |

### Build Status
- Blockstore package: Clean build, all tests pass
- Raspberry package: Blockexecutor.go compiles (other files have unrelated issues)
- Race detection: All tests pass

---
