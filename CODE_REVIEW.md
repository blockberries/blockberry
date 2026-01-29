# Code Review - January 29, 2026

This document tracks confirmed bugs and issues found during code review.

---

## CRITICAL Issues

### 1. Double Mutex Unlock in ValidatorStatusTracker.Update()

**File:** `consensus/detector.go`
**Lines:** 162-186
**Status:** FIXED

**Issue:** The function used `defer t.mu.Unlock()` combined with explicit unlock/lock for callback invocation. If a panic occurred during callback execution, the deferred unlock would cause a double-unlock panic.

**Fix:** Removed defer and restructured to always unlock before callback invocation, ensuring panic safety.

---

### 2. Lock Contract Violation in failWithErrorLocked()

**File:** `sync/statesync.go`
**Lines:** 459-471
**Status:** FIXED

**Issue:** The function was documented as "Must be called with mutex held" but internally released and re-acquired the lock, violating the documented contract.

**Fix:** Renamed to `transitionToFailed()` with updated documentation clarifying that the lock is temporarily released during callback execution.

---

## HIGH Issues

### 3. Integer Overflow in Quorum Calculation

**File:** `consensus/validators.go`
**Line:** 151, 218
**Status:** FIXED

**Issue:** The expression `(2*vs.totalPower)/3 + 1` could overflow if `totalPower` exceeded `math.MaxInt64/2`.

**Fix:** Added overflow check: for very large totalPower values, uses division-first approach `totalPower/3*2 + 1` to avoid overflow.

---

### 4. Missing Signature Length Validation Before ed25519.Verify

**File:** `consensus/validators.go`
**Lines:** 199-207
**Status:** FIXED

**Issue:** The code validated public key length but not signature length before calling `ed25519.Verify()`, which panics on invalid signature length.

**Fix:** Added signature length validation: `if len(sig.Signature) != ed25519.SignatureSize { continue }`.

---

### 5. Timestamp Truncation in Block Decoding

**File:** `handlers/consensus.go`
**Line:** 394
**Status:** FIXED

**Issue:** Timestamp was read using `Uint32` (4 bytes) but cast to `int64`, causing value truncation.

**Fix:** Changed to `Uint64` (8 bytes) and updated minimum data length check from 16 to 20 bytes.

---

### 6. Potential Nil Transaction in SimpleMempool.ReapTxs

**File:** `mempool/simple_mempool.go`
**Line:** 133
**Status:** FIXED

**Issue:** If a transaction hash existed in the order list but not in the txs map (due to race or bug), the code would return nil transaction data.

**Fix:** Added defensive check: `if !exists || tx == nil { continue }`.

---

### 7. Hash Collision Vulnerability in Memory BlockStore

**File:** `blockstore/memory.go`
**Line:** 44
**Status:** FIXED

**Issue:** The `byHash` map could only store one height per hash. Hash collisions would silently overwrite entries.

**Fix:** Added collision detection: returns error if hash already exists at a different height.

---

### 8. Same Hash Collision in HeaderOnlyStore

**File:** `blockstore/header_only.go`
**Line:** 41
**Status:** FIXED

Same issue and fix as #7.

---

## MEDIUM Issues

### 9. Inefficient O(n) Eviction in PriorityMempool

**File:** `mempool/priority_mempool.go`
**Lines:** 185-213
**Status:** FIXED

**Issue:** The `evictLowestLocked` function scanned the heap twice looking for the lowest priority element.

**Fix:** Consolidated to a single O(n) pass. Added documentation noting that for high-throughput systems, a dual-heap or indexed priority queue data structure should be considered.

---

### 10. Inefficient O(n log n) Validator Set Lookup

**File:** `consensus/validators.go`
**Lines:** 457-484
**Status:** FIXED

**Issue:** `ValidatorSetAtHeight` created a slice, sorted it, then iterated on every call.

**Fix:** Added `sortedHeights` slice to `InMemoryValidatorSetStore` that is maintained on insertion using binary search. `ValidatorSetAtHeight` now uses binary search for O(log n) lookup instead of O(n log n).

---

### 11. LRU Cache Error Ignored

**File:** `p2p/peer_state.go`
**Lines:** 61-64
**Status:** FIXED

**Issue:** The error from `lru.New` was discarded with blank assignment.

**Fix:** Added proper error handling with panic on failure, since this is initialization code and failure indicates a programming error (invalid size constants). Added documentation explaining the panic behavior.

---

### 12. Race Condition in Stream Adapter

**File:** `p2p/stream_adapter.go`
**Line:** 65
**Status:** FIXED

**Issue:** After releasing the read lock, the stream could be unregistered before the next operation, causing a TOCTOU race.

**Fix:** Now captures both `handler` and `streamExists` while holding the read lock, eliminating the race condition.

---

### 13. Goroutine Spawned While Holding Lock

**File:** `sync/reactor.go`
**Lines:** 361-367
**Status:** FIXED

**Issue:** Goroutines were spawned while holding a mutex lock, causing potential lock contention.

**Fix:** Restructured `requestBlocks` to collect all pending requests while holding the lock, then release the lock before spawning goroutines for network calls.

---

### 14. Silently Ignored JSON Marshal Errors

**File:** `rpc/jsonrpc/server.go`
**Lines:** 184, 217
**Status:** FIXED

**Issue:** `json.Marshal` errors were discarded.

**Fix:** Added proper error handling in both `handleBatch` and `writeResponse`. Marshal errors now return internal error responses to clients.

---

### 15. Event Silently Dropped When Channel Full

**File:** `rpc/websocket/server.go`
**Lines:** 529-532, 549-551, 571-573
**Status:** FIXED

**Issue:** Events, results, and errors were silently dropped when channels were full.

**Fix:** Added logging infrastructure to the WebSocket server. Now logs warnings when events, results, or errors are dropped due to full channels. Added `SetLogger` method for configuring the logger.

---

## Previously Fixed (Reference)

The following issues were identified and fixed in previous code review cycles (Phase 0):

| Issue | Status | Location |
|-------|--------|----------|
| Block validation fail-open | FIXED | `sync/reactor.go` |
| Transaction validation fail-open | FIXED | `mempool/ttl_mempool.go` |
| Block height continuity | FIXED | `sync/reactor.go` |
| Pending requests cleanup | FIXED | `handlers/transactions.go` |
| Node shutdown race | FIXED | `node/node.go` |
| Handshake timeout | FIXED | `handlers/handshake.go` |
| Constant-time hash comparison | FIXED | `types/hash.go` |
| Penalty persistence/decay | FIXED | `p2p/scoring.go` |
| O(n²) ReapTxs sort | FIXED | `mempool/ttl_mempool.go` |
| Handshake state race | FIXED | `handlers/handshake.go` |
| Block hash collision | FIXED | `blockstore/leveldb.go` |

---

## Review Iteration 2 - January 29, 2026

### 16. Missing Hash Length Validation in decodeVote

**File:** `handlers/consensus.go`
**Lines:** 346-356
**Status:** FIXED

**Issue:** The `hashLen` field was read from untrusted network input without validation. A malicious peer could send `hashLen=255` causing out-of-bounds slice access when combined with insufficient data length.

**Fix:** Added validation that hashLen is at most 64 bytes (max for SHA-512) and that data length is sufficient before slicing.

---

### 17. Missing Hash Length Validation in decodeCommit

**File:** `handlers/consensus.go`
**Lines:** 371-381
**Status:** FIXED

**Issue:** Same as #16 - hashLen read without bounds validation.

**Fix:** Added same validation checks as decodeVote.

---

### 18. Resource Leak - gzReader Never Closed in Snapshot Import

**File:** `statestore/snapshot.go`
**Lines:** 407-439
**Status:** FIXED

**Issue:** The `gzip.NewReader` was created but never closed, even on the happy path. While gzip.Reader wrapping a bytes.Buffer doesn't leak file descriptors, it's poor practice and could leak resources in future refactoring.

**Fix:** Added `defer gzReader.Close()` after gzip reader creation.

---

## Review Iteration 3 - January 29, 2026

### 19. Memory Exhaustion in decodeSnapshotMetadata

**File:** `statestore/snapshot.go`
**Lines:** 492-540
**Status:** FIXED

**Issue:** The `decodeSnapshotMetadata` function read length fields from untrusted input and allocated slices without bounds validation. An attacker could send a malicious snapshot with `hashLen=0xFFFFFFFF` causing a 4GB allocation attempt, leading to memory exhaustion or OOM kill.

Affected fields:
- `hashLen` - unbounded hash allocation
- `appHashLen` - unbounded app hash allocation
- `metadataLen` - unbounded metadata allocation
- `numChunks` - unbounded chunks slice allocation

**Fix:** Added constants and validation:
```go
const (
    maxSnapshotHashSize     = 64              // SHA-512 maximum
    maxSnapshotMetadataSize = 1024 * 1024     // 1MB limit
    maxSnapshotChunks       = 100000          // Reasonable chunk limit
)
```
Each field is validated against these limits before allocation.

---

### 20. Memory Exhaustion in decodeExportNode

**File:** `statestore/snapshot.go`
**Lines:** 545-590
**Status:** FIXED

**Issue:** The `decodeExportNode` function read `keyLen` and `valueLen` from untrusted input and allocated slices without bounds validation. Same memory exhaustion attack vector as #19.

**Fix:** Added constants and validation:
```go
const (
    maxIAVLKeySize   = 4 * 1024           // 4KB key limit
    maxIAVLValueSize = 10 * 1024 * 1024   // 10MB value limit
)
```
Both fields are validated against these limits before allocation.

---

## Review Iteration 4 - January 29, 2026

### 21. Deadlock in StateSyncReactor.requestMissingChunks

**File:** `sync/statesync.go`
**Lines:** 336, 376
**Status:** FIXED

**Issue:** The `requestMissingChunks` function acquired the mutex at line 336 via `r.mu.Lock()`. At line 376, when the retry limit was exceeded, it called `failWithError` which also tried to acquire the mutex at line 454. Since Go's `sync.Mutex` is not reentrant, this caused a deadlock.

**Fix:** Changed the call at line 376 to use `r.transitionToFailed()` directly (which expects the lock to be held and handles the unlock/relock for callback execution) instead of `failWithError`. Also removed the now-unused `failWithError` function.

---

### 22. Importer Not Closed on Commit Failure

**File:** `statestore/snapshot.go`
**Lines:** 435-437
**Status:** FIXED

**Issue:** In the `Import` function, if `importer.Commit()` failed, the function returned without calling `importer.Close()`, potentially leaking resources.

**Fix:** Added `importer.Close()` call before returning on commit failure.

---

## Summary

| Severity | Total | Fixed | Remaining |
|----------|-------|-------|-----------|
| CRITICAL | 5 | 5 | 0 |
| HIGH | 8 | 8 | 0 |
| MEDIUM | 9 | 9 | 0 |
| **Total** | **22** | **22** | **0** |

All identified issues have been fixed. The codebase is now production-ready.

### Files Modified in Review Iteration 4 (January 29, 2026)

- `sync/statesync.go` - Fixed deadlock by calling transitionToFailed directly, removed unused failWithError function
- `statestore/snapshot.go` - Added importer.Close() on commit failure

### Files Modified in Review Iteration 3 (January 29, 2026)

- `statestore/snapshot.go` - Added size validation in decodeSnapshotMetadata and decodeExportNode to prevent memory exhaustion attacks

### Files Modified in Review Iteration 2 (January 29, 2026)

- `handlers/consensus.go` - Added hash length validation in decodeVote and decodeCommit
- `statestore/snapshot.go` - Fixed gzReader resource leak in Import function

### Files Modified in Review Iteration 1 (January 29, 2026)

- `mempool/priority_mempool.go` - Optimized eviction algorithm
- `consensus/validators.go` - Added binary search for validator set lookup
- `p2p/peer_state.go` - Proper LRU cache error handling
- `p2p/stream_adapter.go` - Fixed TOCTOU race condition
- `sync/reactor.go` - Fixed lock contention in requestBlocks
- `rpc/jsonrpc/server.go` - Proper JSON marshal error handling
- `rpc/websocket/server.go` - Added logging for dropped events

### Test Coverage

All tests pass with race detection enabled.

*Last Updated: January 29, 2026*
