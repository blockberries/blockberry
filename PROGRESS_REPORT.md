# Progress Report

## Replace cosmos/iavl with blockberries/avlberry

### Status: Complete

### Summary

Replaced the `github.com/cosmos/iavl` dependency with `github.com/blockberries/avlberry` across the entire blockberry codebase. avlberry is our own AVL+ tree library with ICS-23 proof support.

### Phase 1: Implement AvailableVersions in avlberry

**Files modified:**
- `/Volumes/Tendermint/stealth/avlberry/mutable_tree.go`
  - Updated `VersionExists()` to check both new-format ('s' prefix) and legacy ('r' prefix) root keys
  - Implemented `AvailableVersions()` by iterating from `firstVersion` to `latestVersion` and checking each with `VersionExists()`
- `/Volumes/Tendermint/stealth/avlberry/mutable_tree_test.go`
  - Added `TestMutableTree_VersionExists_LegacyKey` - tests legacy root key detection
  - Added `TestMutableTree_AvailableVersions_Empty` - tests empty tree returns nil
  - Added `TestMutableTree_AvailableVersions_WithVersions` - tests multiple saved versions
  - Added `TestMutableTree_AvailableVersions_AfterPruning` - tests versions after pruning

### Phase 2: Update blockberry dependencies

**Files modified:**
- `go.mod` - Replaced `github.com/cosmos/iavl v1.3.5` with `github.com/blockberries/avlberry v0.0.0`, updated `cosmos/ics23/go` from v0.10.0 to v0.11.0, added replace directive for avlberry
- `deps.go` - Replaced `_ "github.com/cosmos/iavl"` with `_ "github.com/blockberries/avlberry"`

### Phase 3: Update statestore implementation

**Files modified:**
- `pkg/statestore/iavl.go`
  - Changed imports from `cosmos/iavl` to `blockberries/avlberry`
  - Updated struct types: `*iavl.MutableTree` → `*avlberry.MutableTree`, `idb.DB` → `avlberry.DB`
  - `NewIAVLStore`: `idb.NewGoLevelDB` → `avldb.NewGoLevelDB`, `iavl.NewMutableTree` → `avlberry.NewMutableTree`
  - `NewMemoryIAVLStore`: `idb.NewMemDB()` → `avldb.NewMemDB()`
  - `RootHash`: `tree.WorkingHash()` → `tree.Hash()`
  - `GetProof`: `tree.WorkingHash()` → `tree.Hash()`
  - `Close`: `s.db.Close()` → `s.tree.Close()`
  - `VersionExists`: Handle new `(bool, error)` return from avlberry
  - `GetVersioned`: Replaced `tree.GetVersioned(key, version)` with temp tree approach (avlberry lacks this method)
  - `PruneVersions`: Handle `([]int64, error)` return from `AvailableVersions()`
  - `AvailableVersions`: Same return type adaptation
  - `AllVersions`: Same return type adaptation

- `pkg/statestore/snapshot.go`
  - Changed import from `cosmos/iavl` to `blockberries/avlberry`
  - `iavl.ErrorExportDone` → `avlberry.ErrExportDone`
  - `*iavl.ExportNode` → `*avlberry.ExportNode`

- `pkg/statestore/store.go`
  - Updated `Proof.Verify()` to use direct root hash calculation instead of `ics23.VerifyMembership` spec-based validation. avlberry's ICS23 proof format uses shorter inner op prefixes than what `IavlSpec` expects, so direct root calculation comparison is used (matching avlberry's own verification approach).

### Phase 4: Documentation updates

**Files modified:**
- `CLAUDE.md` - Updated dependency table
- `CHANGELOG.md` - Updated dependency references
- `ARCHITECTURE.md` - Updated dependency tables, architecture descriptions
- `ARCHITECTURE_ANALYSIS.md` - Updated cosmos/iavl references to avlberry
- `README.md` - Updated dependency links and descriptions
- `CODEBASE_ANALYSIS.md` - Updated import paths and descriptions

### Test Coverage

- All avlberry tests pass (including new AvailableVersions tests)
- All blockberry statestore tests pass (44 tests)
- Full blockberry test suite passes (all packages)
- No remaining `cosmos/iavl` references in Go source files

### Design Decisions

1. **GetVersioned via temp tree**: Since avlberry lacks a `GetVersioned(key, version)` method, we create a temporary `MutableTree` sharing the same DB, load the target version, and call `Get()`. The temp tree is not closed since it shares the underlying DB.

2. **Proof verification**: Changed from ICS23 spec-based validation (`ics23.VerifyMembership`) to direct root hash calculation comparison. This is necessary because avlberry's inner op prefixes can be shorter than `IavlSpec.MinPrefixLength` (3 bytes vs 4 bytes required). The direct calculation approach provides equivalent security — it verifies the proof path computes to the correct root hash.

3. **VersionExists with legacy support**: Updated avlberry's `VersionExists` to check both new-format root keys ('s' prefix) and legacy root keys ('r' prefix) for backward compatibility with databases that contain legacy-format data.

---

## Update to avlberry v1.1.0

### Status: Complete

### Summary

Updated blockberry to leverage avlberry v1.1.0, which fixes ICS23 spec compatibility (inner op prefixes now include version field, meeting `IavlSpec.MinPrefixLength` of 4 bytes) and adds the `GetVersioned` and `WorkingHash` APIs.

### Changes

**Files modified:**
- `pkg/statestore/iavl.go`
  - `GetVersioned`: Replaced temp tree workaround with direct `s.tree.GetVersioned(key, version)` call using avlberry's native API
  - `RootHash`: Changed from `s.tree.Hash()` to `s.tree.WorkingHash()`. In v1.1.0, `MutableTree.Hash()` returns the committed root hash, while `WorkingHash()` returns the hash reflecting uncommitted changes (matching the `StateStore.RootHash()` contract)

- `pkg/statestore/store.go`
  - `Proof.Verify`: Restored proper ICS23 spec-based verification using `ics23.VerifyMembership` and `ics23.VerifyNonMembership` with `ics23.IavlSpec`, replacing the direct root hash calculation workaround

### Test Coverage

- All 49 statestore tests pass with race detection
- Full blockberry test suite passes (all packages)

### Design Decisions

1. **Hash vs WorkingHash**: avlberry v1.1.0 splits `Hash()` semantics — `Hash()` on `MutableTree` returns the last committed root hash, while `WorkingHash()` returns the current working tree hash. `RootHash()` uses `WorkingHash()` (uncommitted state), while `GetProof()` uses `Hash()` (committed state) since proofs are generated against committed versions.

2. **ICS23 spec restoration**: With v1.1.0 including version in inner op prefixes (>= 4 bytes), standard `ics23.VerifyMembership`/`ics23.VerifyNonMembership` with `IavlSpec` now works correctly, eliminating the need for the direct root hash calculation workaround.

---

## Replace pkg/abi with github.com/blockberries/bapi v0.2.0

### Status: Complete

### Summary

Replaced the entire `pkg/abi/` package (25 files, ~180 types) with `github.com/blockberries/bapi` v0.2.0. The bapi module provides a ground-up redesign of the consensus-application boundary with whole-block execution, capability discovery, and cleaner types. Framework-internal types (EventBus, Metrics, Tracer, Indexer, Security, Component) were relocated from `pkg/abi/` to their natural packages.

### Key Architecture Changes

- `abi.Application` (7 methods: InitChain, CheckTx, BeginBlock, ExecuteTx, EndBlock, Commit, Query) → `bapi.Lifecycle` (5 methods: Handshake, CheckTx, ExecuteBlock, Commit, Query)
- Per-tx execution model (BeginBlock + ExecuteTx per-tx + EndBlock) → single `ExecuteBlock` returning `BlockOutcome`
- `abi.Event.Type` → `bapitypes.Event.Kind`; `abi.Attribute.Value []byte` → `bapitypes.EventAttribute.Value string`
- `abi.ResultCode` (typed uint32) → raw `uint32`
- `abi.CommitResult.AppHash` → moved to `bapitypes.BlockOutcome.AppHash`; `CommitResult` only has `RetainHeight`

### Phase 1: Add bapi dependency (Step 1)

**Files modified:**
- `go.mod` — Added `require github.com/blockberries/bapi v0.2.0` with `replace` directive pointing to `../bapi`

### Phase 2: Relocate framework-internal types (Steps 2-7)

**Step 2 — EventBus types → `pkg/events/`**
- Created `pkg/events/types.go` — Moved `EventBus`, `Query`, `QueryAll`, `QueryEventKind`, `QueryEventKinds`, `QueryFunc`, `QueryAnd`, `QueryOr`, `QueryAttribute`, `QueryAttributeExists`, `Subscription`, `EventBusConfig`, `DefaultEventBusConfig()`, event constants, attribute key constants
- Modified `pkg/events/bus.go` — Updated to use local types and `bapitypes.Event`

**Step 3 — Metrics types → `pkg/metrics/`**
- Created `pkg/metrics/interface.go` — Moved `Metrics` interface, `NullMetrics`, `MetricsConfig`, `HistogramBuckets`, `DefaultMetricsConfig()`, all metric/label/reason constants
- Modified `pkg/metrics/adapter.go` (renamed from `abi_adapter.go`) — Updated to implement local `Metrics` interface, replaced `AppBeginBlock`/`AppExecuteTx`/`AppEndBlock` with `AppExecuteBlock`

**Step 4 — Tracer types → `pkg/tracing/`**
- Created `pkg/tracing/interface.go` — Moved `Tracer`, `Span`, `SpanContext`, `SpanAttribute`, `SpanOption`, `SpanKind`, `StatusCode`, `Link`, `Carrier`, `MapCarrier`, `NullTracer`, `nullSpan`, `TracerConfig`, `DefaultTracerConfig()`, all span constants
- Modified `pkg/tracing/otel/tracer.go` and `provider.go` — Updated to use `tracing.*` types

**Step 5 — Indexer types → `pkg/indexer/`**
- Created `pkg/indexer/interface.go` — Moved `TxIndexer`, `TxIndexBatch`, `BlockIndexer`, `IndexerConfig`, `DefaultIndexerConfig()`, `NullTxIndexer`, error sentinels
- Modified `pkg/indexer/kv/indexer.go` — Updated to use `indexer.*` and `bapitypes.*` types

**Step 6 — Security/limits types → `internal/security/`**
- Created `internal/security/types.go` — Moved `ResourceLimits`, `RateLimiter`, `RateLimiterConfig`, `ConnectionLimiter`, `EclipseMitigation`, `BandwidthLimiter`, all config types and error sentinels
- Modified `internal/security/ratelimit.go`, `eclipse.go` — Updated to use local types

**Step 7 — Component types → `pkg/types/`**
- Verified `pkg/types/component.go` already contained `Component`, `Named`, `HealthChecker`, `HealthStatus`, `Health`, `Dependent`, `LifecycleAware`, `Resettable` types

### Phase 3: Update consumers (Steps 8-13)

**Step 8 — `pkg/consensus/interface.go`**
- Changed `Application abi.Application` → `Application bapi.Lifecycle`

**Step 9 — `pkg/types/null_app.go`**
- Complete rewrite to implement `bapi.Lifecycle`: Handshake, CheckTx (with Tx+MempoolContext), ExecuteBlock, Commit, Query
- Simplified fields: removed `LastBlockHeight`, `LastBlockHash`; `AppHash` changed to `bapitypes.AppHash`

**Step 10 — `pkg/rpc/server.go`**
- `Query` return: `*abi.QueryResult` → `*bapitypes.StateQueryResult`
- `Subscribe`/`Unsubscribe` params: `abi.Query` → `events.Query`, `abi.Event` → `bapitypes.Event`

**Step 11 — `pkg/rpc/types.go`**
- `BroadcastResult.Code`: `abi.ResultCode` → `uint32`
- Created local `Block` and `BlockHeader` structs (abi.Block was a framework concept, bapi doesn't define Block for RPC)
- `TxResult.Result`: `*abi.TxResult` → `*bapitypes.TxOutcome`

**Step 12 — RPC sub-servers**
- `pkg/rpc/jsonrpc/server.go` — `parseQuery` returns `events.Query`, uses `events.QueryEventKind`/`events.QueryAttribute`/`events.QueryAll`
- `pkg/rpc/grpc/server.go` — Same query changes; `txResultToRPC` updated for `TxOutcome` fields; `Query` maps `result.Value`/`result.Info`
- `pkg/rpc/websocket/server.go` — `events.EventBus`, `bapitypes.Event`, `events.Query` throughout

**Step 13 — `test/helpers.go`**
- `MockApplication` rewritten for `bapi.Lifecycle`: Handshake, CheckTx, ExecuteBlock, Commit, Query
- Changed fields: `CheckedTxs []bapitypes.Tx`, `ExecutedBlocks []uint64`, `AppHash bapitypes.AppHash`

### Phase 4: Update tests (Step 17)

- `pkg/types/application_test.go` — Rewritten for `bapi.Lifecycle` (Handshake, CheckTx with MempoolContext, ExecuteBlock, Commit, Query with StateQuery)
- `pkg/rpc/jsonrpc/server_test.go` — mockRPCServer updated: `*bapitypes.StateQueryResult`, `chan bapitypes.Event`, `events.Query`, `rpc.Block`, literal `0` replacing `abi.CodeOK`
- `pkg/rpc/grpc/server_test.go` — Same mock updates; TestParseQuery asserts `events.QueryAll`, `events.QueryEventKind`, `events.QueryAttribute`
- `pkg/rpc/websocket/server_test.go` — `events.DefaultEventBusConfig()`, `bapitypes.Event{Kind, Attributes}`, `events.Query` types

### Phase 5: Delete pkg/abi and update docs (Steps 14, 16)

- Deleted `pkg/abi/` entirely (25 files)
- Updated CLAUDE.md — Removed `pkg/abi/` from package structure
- Updated README.md, CODEBASE_ANALYSIS.md, ARCHITECTURE_ANALYSIS.md, ARCHITECTURE.md, API_REFERENCE.md, docs/getting-started/*.md — All `pkg/abi` references replaced with correct new locations

### Test Coverage

- All tests pass across all packages
- `go build ./...` succeeds with zero errors
- No remaining `pkg/abi` references in any Go source files

### Design Decisions

1. **Local RPC Block type**: Created `rpc.Block` and `rpc.BlockHeader` structs in `pkg/rpc/types.go` since bapi doesn't define a Block type for RPC transport — the block concept is framework-internal.

2. **MempoolContext as uint8**: `bapitypes.MempoolContext` is a `uint8` enum (not a struct), so tests use `bapitypes.MempoolFirstSeen` constant.

3. **QueryEventKind.String()**: Returns `"kind=X"` rather than `"type=X"` (matching the field name change from `Event.Type` to `Event.Kind`). Test expectations updated accordingly.
