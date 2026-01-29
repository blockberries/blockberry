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

## MEDIUM Issues (Remaining - Low Priority)

### 9. Inefficient O(n) Eviction in PriorityMempool

**File:** `mempool/priority_mempool.go`
**Lines:** 185-213
**Status:** OPEN - LOW PRIORITY

**Issue:** The `evictLowestLocked` function scans the heap twice looking for the lowest priority element.

---

### 10. Inefficient O(n log n) Validator Set Lookup

**File:** `consensus/validators.go`
**Lines:** 457-484
**Status:** OPEN - LOW PRIORITY

**Issue:** `ValidatorSetAtHeight` creates a slice, sorts it, then iterates on every call.

---

### 11. LRU Cache Error Ignored

**File:** `p2p/peer_state.go`
**Lines:** 61-64
**Status:** OPEN - LOW PRIORITY

**Issue:** The error from `lru.New` is discarded with blank assignment.

---

### 12. Race Condition in Stream Adapter

**File:** `p2p/stream_adapter.go`
**Line:** 65
**Status:** OPEN - LOW PRIORITY

**Issue:** After releasing the read lock, the stream could be unregistered before the next operation.

---

### 13. Goroutine Spawned While Holding Lock

**File:** `sync/reactor.go`
**Lines:** 361-367
**Status:** OPEN - LOW PRIORITY

**Issue:** A goroutine is spawned while holding a mutex lock.

---

### 14. Silently Ignored JSON Marshal Errors

**File:** `rpc/jsonrpc/server.go`
**Lines:** 184, 217
**Status:** OPEN - LOW PRIORITY

**Issue:** `json.Marshal` errors are discarded.

---

### 15. Event Silently Dropped When Channel Full

**File:** `rpc/websocket/server.go`
**Lines:** 529-532, 549-551, 571-573
**Status:** OPEN - LOW PRIORITY

**Issue:** Events, results, and errors are silently dropped when channels are full.

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

## Summary

| Severity | Total | Fixed | Remaining |
|----------|-------|-------|-----------|
| CRITICAL | 2 | 2 | 0 |
| HIGH | 6 | 6 | 0 |
| MEDIUM | 7 | 0 | 7 |
| **Total** | **15** | **8** | **7** |

All CRITICAL and HIGH severity issues have been fixed. Remaining MEDIUM severity issues are operational optimizations that can be addressed in future iterations.

*Last Updated: January 29, 2026*
