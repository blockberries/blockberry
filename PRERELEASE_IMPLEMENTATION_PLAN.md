# Blockberry Prerelease Implementation Plan

This document provides a comprehensive, prioritized implementation plan for preparing Blockberry for production release. Blockberry is the core node framework that integrates all other Blockberries modules.

**Current Status**: 85% production ready (claimed 95%)
**Target**: v1.0.0 stable release with full production safety

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Critical Blockers](#critical-blockers)
3. [Phase 1: Critical Fixes](#phase-1-critical-fixes)
4. [Phase 2: Memory & Safety](#phase-2-memory--safety)
5. [Phase 3: Integration Hardening](#phase-3-integration-hardening)
6. [Phase 4: Observability](#phase-4-observability)
7. [Phase 5: Features & Polish](#phase-5-features--polish)
8. [Testing Requirements](#testing-requirements)
9. [Release Checklist](#release-checklist)

---

## Executive Summary

Blockberry is a well-architected modular blockchain node framework with good test coverage (22 test files, ~6,000 lines). However, the code review identified **2 critical blockers**, **4 race conditions**, and several memory management issues not captured in the existing ROADMAP.

### Current Assessment

| Aspect | Status | Notes |
|--------|--------|-------|
| Architecture | A | Clean modular design |
| Core Functionality | 95% | Mostly complete |
| Safety/Correctness | 75% | Critical bugs found |
| Memory Management | 70% | Unbounded growth |
| Integration | 80% | Version pinning needed |
| Observability | 40% | No metrics |

### Production Blockers

| Issue | Severity | Effort | Location |
|-------|----------|--------|----------|
| Proof.Verify() stubbed | CRITICAL | 2-3 days | statestore/store.go |
| Outbound detection hardcoded | CRITICAL | 4 hours | node/node.go |
| Peer state memory leak | HIGH | 1 day | p2p/peer_manager.go |
| Race conditions (4) | HIGH | 2 days | Multiple files |

### Dependencies

- **Glueberry**: P2P networking (requires connection direction API)
- **Cramberry**: Message serialization (requires security fixes)
- **Looseberry**: Advanced mempool (optional, requires major fixes)

---

## Critical Blockers

### Blocker 1: Proof.Verify() Always Returns False

**Location**: `statestore/store.go:70-77`
**Priority**: CRITICAL
**Blocks**: Light clients, cross-chain verification, state proofs

```go
// CURRENT (STUBBED)
func (p *Proof) Verify(rootHash []byte) (bool, error) {
    return false, nil  // ALWAYS FAILS!
}
```

This breaks any functionality requiring state proof verification.

### Blocker 2: Outbound Connection Detection Hardcoded

**Location**: `node/node.go:376`
**Priority**: CRITICAL
**Blocks**: Peer limit enforcement, connection management

```go
// CURRENT (HARDCODED)
isOutbound := true  // Should query Glueberry
```

All connections appear as outbound, breaking inbound/outbound limits.

---

## Phase 1: Critical Fixes

Focus: Fix the two production blockers.

### P1-1: ICS23 Proof Verification Implementation

**Priority**: CRITICAL
**Effort**: 2-3 days
**Location**: `statestore/store.go`

#### Implementation

```go
// statestore/store.go

import (
    ics23 "github.com/cosmos/ics23/go"
)

// Proof represents a state proof that can be verified against a root hash.
type Proof struct {
    Key       []byte
    Value     []byte
    ProofOps  *ics23.CommitmentProof
    Version   int64
}

// Verify checks that this proof is valid against the given root hash.
// For existence proofs, it verifies the key-value pair exists.
// For non-existence proofs, it verifies the key does not exist.
func (p *Proof) Verify(rootHash []byte) (bool, error) {
    if p.ProofOps == nil {
        return false, ErrInvalidProof
    }

    // Get the proof spec for IAVL trees
    spec := ics23.IavlSpec

    // Determine proof type and verify
    switch proof := p.ProofOps.Proof.(type) {
    case *ics23.CommitmentProof_Exist:
        // Existence proof - verify key/value exists at root
        return ics23.VerifyMembership(spec, rootHash, p.ProofOps, p.Key, p.Value)

    case *ics23.CommitmentProof_Nonexist:
        // Non-existence proof - verify key does not exist
        return ics23.VerifyNonMembership(spec, rootHash, p.ProofOps, p.Key)

    case *ics23.CommitmentProof_Batch:
        // Batch proof - verify all entries
        return p.verifyBatch(spec, rootHash, proof.Batch)

    default:
        return false, fmt.Errorf("unknown proof type: %T", proof)
    }
}

func (p *Proof) verifyBatch(spec *ics23.ProofSpec, rootHash []byte, batch *ics23.BatchProof) (bool, error) {
    for _, entry := range batch.Entries {
        switch e := entry.Proof.(type) {
        case *ics23.BatchEntry_Exist:
            if valid, err := ics23.VerifyMembership(spec, rootHash,
                &ics23.CommitmentProof{Proof: &ics23.CommitmentProof_Exist{Exist: e.Exist}},
                e.Exist.Key, e.Exist.Value); !valid || err != nil {
                return false, err
            }
        case *ics23.BatchEntry_Nonexist:
            if valid, err := ics23.VerifyNonMembership(spec, rootHash,
                &ics23.CommitmentProof{Proof: &ics23.CommitmentProof_Nonexist{Nonexist: e.Nonexist}},
                e.Nonexist.Key); !valid || err != nil {
                return false, err
            }
        }
    }
    return true, nil
}

// GetProof returns a proof for the given key at the specified version.
func (s *StateStore) GetProof(key []byte, version int64) (*Proof, error) {
    s.mu.RLock()
    defer s.mu.RUnlock()

    tree, err := s.treeAt(version)
    if err != nil {
        return nil, err
    }

    value, proof, err := tree.GetWithProof(key)
    if err != nil {
        return nil, fmt.Errorf("get proof: %w", err)
    }

    // Convert IAVL proof to ICS23 format
    ics23Proof, err := s.convertToICS23(proof, key, value)
    if err != nil {
        return nil, fmt.Errorf("convert proof: %w", err)
    }

    return &Proof{
        Key:      key,
        Value:    value,
        ProofOps: ics23Proof,
        Version:  version,
    }, nil
}

func (s *StateStore) convertToICS23(iavlProof *iavl.RangeProof, key, value []byte) (*ics23.CommitmentProof, error) {
    if value != nil {
        // Existence proof
        existProof, err := iavl.ConvertExistenceProof(iavlProof, key, value)
        if err != nil {
            return nil, err
        }
        return &ics23.CommitmentProof{
            Proof: &ics23.CommitmentProof_Exist{Exist: existProof},
        }, nil
    }

    // Non-existence proof
    nonExistProof, err := iavl.ConvertNonExistenceProof(iavlProof, key)
    if err != nil {
        return nil, err
    }
    return &ics23.CommitmentProof{
        Proof: &ics23.CommitmentProof_Nonexist{Nonexist: nonExistProof},
    }, nil
}
```

#### Tests Required

```go
func TestProof_VerifyExistence(t *testing.T) {
    store := NewStateStore(cfg)

    // Set some values
    store.Set([]byte("key1"), []byte("value1"))
    store.Commit()

    // Get proof
    proof, err := store.GetProof([]byte("key1"), 1)
    require.NoError(t, err)

    // Get root hash
    rootHash := store.RootHash()

    // Verify proof
    valid, err := proof.Verify(rootHash)
    require.NoError(t, err)
    require.True(t, valid)
}

func TestProof_VerifyNonExistence(t *testing.T) {
    store := NewStateStore(cfg)

    // Set some values but not the key we'll prove
    store.Set([]byte("key1"), []byte("value1"))
    store.Commit()

    // Get non-existence proof
    proof, err := store.GetProof([]byte("nonexistent"), 1)
    require.NoError(t, err)
    require.Nil(t, proof.Value)

    // Verify proof
    valid, err := proof.Verify(store.RootHash())
    require.NoError(t, err)
    require.True(t, valid)
}

func TestProof_RejectsInvalidProof(t *testing.T) {
    store := NewStateStore(cfg)

    store.Set([]byte("key1"), []byte("value1"))
    store.Commit()

    proof, _ := store.GetProof([]byte("key1"), 1)

    // Tamper with the value
    proof.Value = []byte("tampered")

    valid, _ := proof.Verify(store.RootHash())
    require.False(t, valid)
}
```

---

### P1-2: Fix Outbound Connection Detection

**Priority**: CRITICAL
**Effort**: 4 hours
**Location**: `node/node.go:376`
**Dependency**: Requires Glueberry `IsOutbound()` API (Phase 3 in Glueberry plan)

#### Implementation

```go
// node/node.go

func (n *Node) handleConnected(event glueberry.ConnectionEvent) {
    peerID := event.PeerID

    // Query actual connection direction from Glueberry
    isOutbound, err := n.glueNode.IsOutbound(peerID)
    if err != nil {
        n.logger.Warn("failed to determine connection direction, assuming inbound",
            "peer", peerID,
            "error", err,
        )
        isOutbound = false
    }

    // Register with peer manager
    n.peerManager.AddPeer(peerID, isOutbound)

    // Enforce peer limits
    if isOutbound {
        if n.peerManager.OutboundCount() > n.cfg.Network.MaxOutbound {
            n.logger.Info("outbound peer limit reached, disconnecting",
                "peer", peerID,
            )
            n.glueNode.Disconnect(peerID)
            return
        }
    } else {
        if n.peerManager.InboundCount() > n.cfg.Network.MaxInbound {
            n.logger.Info("inbound peer limit reached, disconnecting",
                "peer", peerID,
            )
            n.glueNode.Disconnect(peerID)
            return
        }
    }

    // Continue with handshake...
}
```

#### Fallback (if Glueberry API not ready)

```go
// Temporary solution: Track locally initiated connections

type Node struct {
    // ... existing fields
    pendingOutbound sync.Map // peerID -> bool
}

func (n *Node) Connect(peerID peer.ID) error {
    // Mark as outbound before connecting
    n.pendingOutbound.Store(peerID, true)

    if err := n.glueNode.Connect(peerID); err != nil {
        n.pendingOutbound.Delete(peerID)
        return err
    }
    return nil
}

func (n *Node) handleConnected(event glueberry.ConnectionEvent) {
    peerID := event.PeerID

    // Check if we initiated this connection
    _, isOutbound := n.pendingOutbound.LoadAndDelete(peerID)

    n.peerManager.AddPeer(peerID, isOutbound)
    // ...
}
```

---

## Phase 2: Memory & Safety

Focus: Fix memory leaks and race conditions.

### P2-1: Peer State Memory Leak

**Priority**: HIGH
**Location**: `p2p/peer_manager.go`
**Effort**: 1 day

#### Problem

```go
// CURRENT - Maps grow unbounded
type PeerState struct {
    KnownTxs   map[types.Hash]struct{}  // Never pruned!
    KnownBlocks map[types.Hash]struct{} // Never pruned!
}
```

After 30 days at moderate load: ~259M entries consuming >4GB memory.

#### Solution

```go
// p2p/peer_state.go

import (
    lru "github.com/hashicorp/golang-lru/v2"
)

const (
    MaxKnownTxsPerPeer    = 10000
    MaxKnownBlocksPerPeer = 1000
)

type PeerState struct {
    mu sync.RWMutex

    PeerID       peer.ID
    IsOutbound   bool
    ConnectedAt  time.Time
    LastSeen     time.Time

    // Use LRU caches instead of unbounded maps
    knownTxs    *lru.Cache[types.Hash, struct{}]
    knownBlocks *lru.Cache[types.Hash, struct{}]

    // Penalty tracking
    PenaltyScore int
    LastPenalty  time.Time
}

func NewPeerState(peerID peer.ID, isOutbound bool) *PeerState {
    txCache, _ := lru.New[types.Hash, struct{}](MaxKnownTxsPerPeer)
    blockCache, _ := lru.New[types.Hash, struct{}](MaxKnownBlocksPerPeer)

    return &PeerState{
        PeerID:       peerID,
        IsOutbound:   isOutbound,
        ConnectedAt:  time.Now(),
        LastSeen:     time.Now(),
        knownTxs:     txCache,
        knownBlocks:  blockCache,
    }
}

func (ps *PeerState) MarkTxKnown(hash types.Hash) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    ps.knownTxs.Add(hash, struct{}{})
}

func (ps *PeerState) HasTx(hash types.Hash) bool {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    return ps.knownTxs.Contains(hash)
}

func (ps *PeerState) MarkBlockKnown(hash types.Hash) {
    ps.mu.Lock()
    defer ps.mu.Unlock()
    ps.knownBlocks.Add(hash, struct{}{})
}

func (ps *PeerState) HasBlock(hash types.Hash) bool {
    ps.mu.RLock()
    defer ps.mu.RUnlock()
    return ps.knownBlocks.Contains(hash)
}
```

---

### P2-2: Race Conditions (4 Locations)

**Priority**: HIGH
**Effort**: 2 days

#### Race 1: UpdateLatency Use-After-Free

**Location**: `p2p/peer_manager.go`

```go
// CURRENT (RACE)
func (pm *PeerManager) UpdateLatency(peerID peer.ID, latency time.Duration) {
    peer := pm.peers[peerID]  // Read without lock
    peer.Latency = latency     // Write without lock
}

// FIX
func (pm *PeerManager) UpdateLatency(peerID peer.ID, latency time.Duration) {
    pm.mu.Lock()
    defer pm.mu.Unlock()

    peer, ok := pm.peers[peerID]
    if !ok {
        return
    }

    peer.mu.Lock()
    peer.Latency = latency
    peer.LastSeen = time.Now()
    peer.mu.Unlock()
}
```

#### Race 2: Concurrent Peer State Access

**Location**: `handlers/transactions.go`

```go
// CURRENT (RACE)
func (h *TransactionsHandler) HandleTx(peerID peer.ID, tx *types.Transaction) {
    state := h.peerManager.GetState(peerID)
    if state.KnownTxs[tx.Hash] {  // Concurrent read
        return
    }
    state.KnownTxs[tx.Hash] = struct{}{}  // Concurrent write
}

// FIX - Use atomic methods on PeerState
func (h *TransactionsHandler) HandleTx(peerID peer.ID, tx *types.Transaction) {
    state := h.peerManager.GetState(peerID)
    if state == nil {
        return
    }

    if state.HasTx(tx.Hash) {  // Thread-safe
        return
    }
    state.MarkTxKnown(tx.Hash)  // Thread-safe
}
```

#### Race 3: Peer Map Iteration During Modification

**Location**: `p2p/peer_manager.go`

```go
// CURRENT (RACE)
func (pm *PeerManager) BroadcastTx(tx *types.Transaction) {
    for peerID, state := range pm.peers {  // Iteration
        if !state.HasTx(tx.Hash) {
            pm.sendTx(peerID, tx)  // May disconnect and modify pm.peers
        }
    }
}

// FIX - Copy peer list first
func (pm *PeerManager) BroadcastTx(tx *types.Transaction) {
    pm.mu.RLock()
    peers := make([]peer.ID, 0, len(pm.peers))
    for peerID := range pm.peers {
        peers = append(peers, peerID)
    }
    pm.mu.RUnlock()

    for _, peerID := range peers {
        state := pm.GetState(peerID)
        if state == nil {
            continue  // Peer disconnected
        }
        if !state.HasTx(tx.Hash) {
            pm.sendTx(peerID, tx)
        }
    }
}
```

#### Race 4: Channel Double-Close

**Location**: `handlers/blocksync.go`

```go
// CURRENT (PANIC RISK)
func (h *BlockSyncHandler) Stop() {
    close(h.stopCh)  // May panic if called twice
}

// FIX
type BlockSyncHandler struct {
    stopCh   chan struct{}
    stopOnce sync.Once
}

func (h *BlockSyncHandler) Stop() {
    h.stopOnce.Do(func() {
        close(h.stopCh)
    })
}
```

#### Add Race Detection Tests

```go
func TestPeerManager_ConcurrentAccess(t *testing.T) {
    pm := NewPeerManager(cfg)

    var wg sync.WaitGroup
    for i := 0; i < 100; i++ {
        wg.Add(3)

        // Concurrent adds
        go func(i int) {
            defer wg.Done()
            pm.AddPeer(peer.ID(fmt.Sprintf("peer-%d", i)), true)
        }(i)

        // Concurrent updates
        go func(i int) {
            defer wg.Done()
            pm.UpdateLatency(peer.ID(fmt.Sprintf("peer-%d", i)), time.Millisecond)
        }(i)

        // Concurrent removes
        go func(i int) {
            defer wg.Done()
            pm.RemovePeer(peer.ID(fmt.Sprintf("peer-%d", i/2)))
        }(i)
    }

    wg.Wait()
}
```

---

### P2-3: Input Validation

**Priority**: HIGH
**Effort**: 4 hours

Add validation for heights, sizes, and other inputs:

```go
// types/validation.go

const (
    MaxBlockSize     = 100 * 1024 * 1024  // 100 MB
    MaxTxSize        = 1 * 1024 * 1024    // 1 MB
    MaxMessageSize   = 10 * 1024 * 1024   // 10 MB
    MaxChainIDLength = 50
)

func ValidateHeight(height int64) error {
    if height < 0 {
        return fmt.Errorf("negative height: %d", height)
    }
    if height > 1<<53 {  // Max safe JavaScript integer
        return fmt.Errorf("height too large: %d", height)
    }
    return nil
}

func ValidateBlock(block *Block) error {
    if block == nil {
        return errors.New("nil block")
    }

    if err := ValidateHeight(block.Header.Height); err != nil {
        return fmt.Errorf("invalid height: %w", err)
    }

    size := block.Size()
    if size > MaxBlockSize {
        return fmt.Errorf("block too large: %d > %d", size, MaxBlockSize)
    }

    if block.Header.Time.After(time.Now().Add(10 * time.Minute)) {
        return errors.New("block time too far in future")
    }

    return nil
}

func ValidateTransaction(tx *Transaction) error {
    if tx == nil {
        return errors.New("nil transaction")
    }

    if len(tx.Data) > MaxTxSize {
        return fmt.Errorf("transaction too large: %d > %d", len(tx.Data), MaxTxSize)
    }

    return nil
}
```

Apply validation at message boundaries:

```go
// handlers/blocks.go

func (h *BlockHandler) HandleBlock(peerID peer.ID, data []byte) error {
    // Size check before unmarshal
    if len(data) > types.MaxBlockSize {
        h.peerManager.Penalize(peerID, PenaltyOversizedMessage)
        return ErrOversizedMessage
    }

    var block types.Block
    if err := cramberry.Unmarshal(data, &block); err != nil {
        h.peerManager.Penalize(peerID, PenaltyInvalidMessage)
        return err
    }

    // Validate after unmarshal
    if err := types.ValidateBlock(&block); err != nil {
        h.peerManager.Penalize(peerID, PenaltyInvalidBlock)
        return err
    }

    return h.processBlock(&block)
}
```

---

## Phase 3: Integration Hardening

Focus: Ensure robust integration with other Blockberries modules.

### P3-1: Version Pinning

**Priority**: HIGH
**Effort**: 2 hours

Update `go.mod`:

```go
// go.mod

module github.com/blockberries/blockberry

go 1.21

require (
    github.com/blockberries/glueberry v1.0.0  // Pin specific version
    github.com/blockberries/cramberry v1.1.0  // Pin specific version
    github.com/cosmos/iavl v1.0.0
    github.com/cosmos/ics23/go v0.10.0
)
```

### P3-2: Message Validation After Unmarshal

**Priority**: HIGH
**Effort**: 1 day

Add validation layer:

```go
// handlers/validation.go

// MessageValidator validates messages after deserialization.
type MessageValidator struct {
    maxMessageSize int
    maxArrayLength int
}

func (v *MessageValidator) ValidateHandshakeMessage(msg *schema.HandshakeMessage) error {
    switch m := msg.(type) {
    case *schema.HelloRequest:
        return v.validateHelloRequest(m)
    case *schema.HelloResponse:
        return v.validateHelloResponse(m)
    default:
        return fmt.Errorf("unknown handshake message type: %T", msg)
    }
}

func (v *MessageValidator) validateHelloRequest(req *schema.HelloRequest) error {
    if req.NodeId == nil || len(*req.NodeId) != 32 {
        return errors.New("invalid node ID length")
    }
    if req.ChainId == nil || len(*req.ChainId) > types.MaxChainIDLength {
        return errors.New("invalid chain ID")
    }
    if req.Version == nil {
        return errors.New("missing version")
    }
    return nil
}
```

### P3-3: Error Context Preservation

**Priority**: MEDIUM
**Effort**: 4 hours

Wrap errors with context at boundaries:

```go
// handlers/blocks.go

func (h *BlockHandler) HandleBlock(peerID peer.ID, data []byte) error {
    block, err := h.unmarshalBlock(data)
    if err != nil {
        return &BlockError{
            PeerID:  peerID,
            Message: "failed to unmarshal block",
            Cause:   err,
        }
    }

    if err := h.processBlock(block); err != nil {
        return &BlockError{
            PeerID:  peerID,
            Height:  block.Header.Height,
            Hash:    block.Hash(),
            Message: "failed to process block",
            Cause:   err,
        }
    }

    return nil
}

type BlockError struct {
    PeerID  peer.ID
    Height  int64
    Hash    types.Hash
    Message string
    Cause   error
}

func (e *BlockError) Error() string {
    return fmt.Sprintf("block error from %s at height %d (%x): %s: %v",
        e.PeerID, e.Height, e.Hash[:8], e.Message, e.Cause)
}

func (e *BlockError) Unwrap() error { return e.Cause }
```

---

## Phase 4: Observability

Focus: Add metrics and structured logging for production operations.

### P4-1: Prometheus Metrics

**Priority**: HIGH
**Effort**: 3-4 days

```go
// metrics/metrics.go

type Metrics struct {
    // Peer metrics
    PeersTotal         *prometheus.GaugeVec   // labels: direction (inbound/outbound)
    PeerConnections    *prometheus.CounterVec // labels: result (success/failure)
    PeerDisconnections *prometheus.CounterVec // labels: reason

    // Block metrics
    BlockHeight    prometheus.Gauge
    BlocksReceived prometheus.Counter
    BlocksProposed prometheus.Counter
    BlockLatency   prometheus.Histogram

    // Transaction metrics
    MempoolSize    prometheus.Gauge
    MempoolBytes   prometheus.Gauge
    TxsReceived    prometheus.Counter
    TxsProposed    prometheus.Counter
    TxsRejected    *prometheus.CounterVec // labels: reason

    // Sync metrics
    SyncState      *prometheus.GaugeVec // labels: state
    SyncProgress   prometheus.Gauge
    SyncPeerHeight prometheus.Gauge

    // Message metrics
    MessagesReceived *prometheus.CounterVec // labels: stream
    MessagesSent     *prometheus.CounterVec // labels: stream
    MessageErrors    *prometheus.CounterVec // labels: stream, error_type
}

func NewMetrics(namespace string) *Metrics {
    return &Metrics{
        PeersTotal: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{
                Namespace: namespace,
                Name:      "peers_total",
                Help:      "Total number of connected peers",
            },
            []string{"direction"},
        ),
        BlockHeight: prometheus.NewGauge(
            prometheus.GaugeOpts{
                Namespace: namespace,
                Name:      "block_height",
                Help:      "Current block height",
            },
        ),
        // ... etc
    }
}
```

### P4-2: Structured Logging

**Priority**: HIGH
**Effort**: 2 days

```go
// logging/logger.go

type Logger interface {
    Debug(msg string, fields ...Field)
    Info(msg string, fields ...Field)
    Warn(msg string, fields ...Field)
    Error(msg string, fields ...Field)
}

type Field struct {
    Key   string
    Value interface{}
}

func PeerID(id peer.ID) Field { return Field{"peer_id", id.String()} }
func Height(h int64) Field    { return Field{"height", h} }
func Hash(h types.Hash) Field { return Field{"hash", h.String()} }
func Err(e error) Field       { return Field{"error", e.Error()} }
func Duration(d time.Duration) Field { return Field{"duration_ms", d.Milliseconds()} }

// Usage:
// n.logger.Info("block received",
//     logging.PeerID(peerID),
//     logging.Height(block.Height),
//     logging.Hash(block.Hash()),
// )
```

---

## Phase 5: Features & Polish

Focus: Implement high-priority ROADMAP items.

### P5-1: Priority-Based Mempool

**Priority**: HIGH (from ROADMAP)
**Effort**: 3-4 days

```go
// mempool/priority_mempool.go

type PriorityMempool struct {
    mu sync.RWMutex

    txs      map[types.Hash]*mempoolTx
    heap     *txHeap
    maxSize  int
    maxBytes int64

    currentBytes int64
    priorityFunc func(tx []byte) int64
}

type mempoolTx struct {
    tx        *types.Transaction
    priority  int64
    addedAt   time.Time
    heapIndex int
}

// Add adds a transaction to the mempool with computed priority.
func (m *PriorityMempool) Add(tx *types.Transaction) error {
    m.mu.Lock()
    defer m.mu.Unlock()

    hash := tx.Hash()
    if _, exists := m.txs[hash]; exists {
        return ErrTxAlreadyExists
    }

    // Compute priority
    priority := m.priorityFunc(tx.Data)

    mtx := &mempoolTx{
        tx:       tx,
        priority: priority,
        addedAt:  time.Now(),
    }

    // Check if we need to evict
    for m.shouldEvict(tx) {
        evicted := m.evictLowest()
        if evicted == nil {
            return ErrMempoolFull
        }
    }

    m.txs[hash] = mtx
    heap.Push(m.heap, mtx)
    m.currentBytes += int64(len(tx.Data))

    return nil
}

// ReapMaxBytes returns transactions up to maxBytes, highest priority first.
func (m *PriorityMempool) ReapMaxBytes(maxBytes int64) []*types.Transaction {
    m.mu.RLock()
    defer m.mu.RUnlock()

    var result []*types.Transaction
    var totalBytes int64

    // Copy and sort by priority (heap order)
    sorted := m.heap.Sorted()

    for _, mtx := range sorted {
        txBytes := int64(len(mtx.tx.Data))
        if totalBytes+txBytes > maxBytes {
            break
        }
        result = append(result, mtx.tx)
        totalBytes += txBytes
    }

    return result
}
```

### P5-2: Transaction Expiration/TTL

**Priority**: HIGH (from ROADMAP)
**Effort**: 1 day

```go
// mempool/ttl.go

type TTLMempool struct {
    *PriorityMempool
    defaultTTL      time.Duration
    cleanupInterval time.Duration
    stopCh          chan struct{}
}

func NewTTLMempool(cfg MempoolConfig) *TTLMempool {
    m := &TTLMempool{
        PriorityMempool: NewPriorityMempool(cfg),
        defaultTTL:      cfg.DefaultTTL,
        cleanupInterval: cfg.CleanupInterval,
        stopCh:          make(chan struct{}),
    }

    go m.cleanupLoop()
    return m
}

func (m *TTLMempool) cleanupLoop() {
    ticker := time.NewTicker(m.cleanupInterval)
    defer ticker.Stop()

    for {
        select {
        case <-m.stopCh:
            return
        case <-ticker.C:
            m.removeExpired()
        }
    }
}

func (m *TTLMempool) removeExpired() {
    m.mu.Lock()
    defer m.mu.Unlock()

    now := time.Now()
    cutoff := now.Add(-m.defaultTTL)

    var expired []types.Hash
    for hash, mtx := range m.txs {
        if mtx.addedAt.Before(cutoff) {
            expired = append(expired, hash)
        }
    }

    for _, hash := range expired {
        m.removeLocked(hash)
    }

    if len(expired) > 0 {
        m.metrics.TxsExpired.Add(float64(len(expired)))
    }
}
```

### P5-3: Rate Limiting

**Priority**: HIGH (from ROADMAP)
**Effort**: 1 day

```go
// network/rate_limiter.go

type RateLimiter struct {
    mu sync.RWMutex

    perPeer  map[peer.ID]*peerLimiter
    defaults RateLimits
}

type RateLimits struct {
    TxPerSecond      int
    BlocksPerSecond  int
    PexPerMinute     int
    BytesPerSecond   int64
}

type peerLimiter struct {
    txLimiter    *rate.Limiter
    blockLimiter *rate.Limiter
    pexLimiter   *rate.Limiter
    byteLimiter  *rate.Limiter
}

func (rl *RateLimiter) Allow(peerID peer.ID, msgType string, size int) bool {
    rl.mu.RLock()
    limiter := rl.perPeer[peerID]
    rl.mu.RUnlock()

    if limiter == nil {
        limiter = rl.createLimiter(peerID)
    }

    // Check message-type specific limit
    var allowed bool
    switch msgType {
    case "transactions":
        allowed = limiter.txLimiter.Allow()
    case "blocks":
        allowed = limiter.blockLimiter.Allow()
    case "pex":
        allowed = limiter.pexLimiter.Allow()
    default:
        allowed = true
    }

    if !allowed {
        return false
    }

    // Check byte limit
    return limiter.byteLimiter.AllowN(time.Now(), size)
}
```

---

## Testing Requirements

### Unit Test Coverage Targets

| Package | Current | Target |
|---------|---------|--------|
| statestore | 80% | 95% |
| p2p | 70% | 85% |
| handlers | 75% | 85% |
| mempool | 85% | 90% |
| blockstore | 90% | 95% |
| node | 65% | 80% |

### Race Condition Tests

```bash
# Run all tests with race detector
go test -race -v ./...

# Run concurrent access stress tests
go test -race -v -run TestConcurrent ./...
```

### Integration Tests

```go
func TestThreeNodeCluster(t *testing.T) {
    // Start 3 nodes
    // Connect them in a triangle
    // Propose a block on node 1
    // Verify all nodes receive it
    // Verify state consistency
}

func TestNodeRecovery(t *testing.T) {
    // Start node, add blocks
    // Stop node
    // Restart node
    // Verify state recovered correctly
}

func TestNetworkPartition(t *testing.T) {
    // Start 4 nodes
    // Partition into 2 groups
    // Verify both groups continue (or halt depending on consensus)
    // Heal partition
    // Verify reconciliation
}
```

---

## Release Checklist

### Pre-Release (Must Complete)

- [ ] Both critical blockers fixed
- [ ] All race conditions fixed
- [ ] Memory leaks fixed
- [ ] Input validation added
- [ ] Integration version pinning done
- [ ] All tests pass with -race
- [ ] Integration tests pass
- [ ] No known security vulnerabilities
- [ ] Metrics implementation complete
- [ ] Documentation updated

### Release Process

1. [ ] Create release branch `release/v1.0.0`
2. [ ] Run full test suite
3. [ ] Run 24-hour stability test
4. [ ] Update version constants
5. [ ] Generate CHANGELOG
6. [ ] Tag release `v1.0.0`
7. [ ] Create GitHub release

### Post-Release

- [ ] Monitor for issues
- [ ] Address critical bugs in patch releases
- [ ] Plan v1.1.0 based on ROADMAP

---

## Implementation Schedule

| Phase | Duration | Items |
|-------|----------|-------|
| Phase 1 | Week 1 | Critical blockers (proof, outbound) |
| Phase 2 | Week 2 | Memory leaks, race conditions, validation |
| Phase 3 | Week 3 | Integration hardening |
| Phase 4 | Week 4 | Observability (metrics, logging) |
| Phase 5 | Week 5-6 | Features (priority mempool, TTL, rate limiting) |
| Testing | Week 7 | Full regression, integration, stability |

**Total Estimated Time**: 7 weeks to production readiness

---

*Last Updated: January 2026*
*Target Release: v1.0.0*
