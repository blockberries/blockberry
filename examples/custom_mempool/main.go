// Package main demonstrates using a custom mempool implementation.
//
// This example shows how to implement a priority-based mempool that
// orders transactions by a fee field in the transaction data.
//
// Usage:
//
//	go run main.go
package main

import (
	"container/heap"
	"encoding/binary"
	"fmt"
	"sort"
	"sync"

	"github.com/blockberries/blockberry/pkg/config"
	"github.com/blockberries/blockberry/pkg/mempool"
	"github.com/blockberries/blockberry/pkg/node"
	bsync "github.com/blockberries/blockberry/internal/sync"
	"github.com/blockberries/blockberry/pkg/types"
)

func main() {
	// Create custom priority mempool
	priorityPool := NewPriorityMempool(1000, 10*1024*1024) // 1000 txs, 10MB

	// Create node configuration
	cfg := config.DefaultConfig()
	if err := cfg.EnsureDataDirs(); err != nil {
		fmt.Printf("Error creating data dirs: %v\n", err)
		return
	}

	// Create node with custom mempool.
	// Note: AcceptAllBlockValidator is for demonstration only.
	// Production code should implement proper block validation.
	n, err := node.NewNode(cfg,
		node.WithMempool(priorityPool),
		node.WithBlockValidator(bsync.AcceptAllBlockValidator),
	)
	if err != nil {
		fmt.Printf("Error creating node: %v\n", err)
		return
	}

	fmt.Printf("Node created with priority mempool\n")
	fmt.Printf("Peer ID: %s\n", n.PeerID())

	// Demonstrate the priority mempool behavior
	demonstratePriorityOrder(priorityPool)
}

// demonstratePriorityOrder shows how transactions are ordered by fee.
func demonstratePriorityOrder(mp *PriorityMempool) {
	fmt.Println("\n--- Priority Mempool Demo ---")

	// Add transactions with different fees
	// Transaction format: [8 bytes fee][rest is payload]
	txs := []struct {
		fee     uint64
		payload string
	}{
		{100, "low fee tx"},
		{500, "medium fee tx"},
		{1000, "high fee tx"},
		{200, "another low fee tx"},
	}

	for _, tx := range txs {
		data := makeTx(tx.fee, tx.payload)
		if err := mp.AddTx(data); err != nil {
			fmt.Printf("Error adding tx: %v\n", err)
		}
	}

	fmt.Printf("Added %d transactions\n", mp.Size())

	// Reap transactions - they should come out in fee order (highest first)
	fmt.Println("\nReaping transactions (should be ordered by fee, highest first):")
	reaped := mp.ReapTxs(1024 * 1024) // 1MB limit
	for i, tx := range reaped {
		fee, payload := parseTx(tx)
		fmt.Printf("  %d. fee=%d payload=%q\n", i+1, fee, payload)
	}
}

// makeTx creates a transaction with fee prefix.
func makeTx(fee uint64, payload string) []byte {
	data := make([]byte, 8+len(payload))
	binary.BigEndian.PutUint64(data[:8], fee)
	copy(data[8:], payload)
	return data
}

// parseTx extracts fee and payload from transaction.
func parseTx(tx []byte) (uint64, string) {
	if len(tx) < 8 {
		return 0, ""
	}
	fee := binary.BigEndian.Uint64(tx[:8])
	return fee, string(tx[8:])
}

// PriorityMempool is a mempool that orders transactions by fee.
type PriorityMempool struct {
	txs       map[string]*priorityTx // hash -> tx
	heap      priorityHeap           // for ordering
	maxTxs    int
	maxBytes  int64
	sizeBytes int64
	validator mempool.TxValidator
	mu        sync.RWMutex
}

type priorityTx struct {
	hash  string
	data  []byte
	fee   uint64
	index int // heap index
}

// NewPriorityMempool creates a new priority-based mempool.
func NewPriorityMempool(maxTxs int, maxBytes int64) *PriorityMempool {
	return &PriorityMempool{
		txs:      make(map[string]*priorityTx),
		heap:     make(priorityHeap, 0),
		maxTxs:   maxTxs,
		maxBytes: maxBytes,
	}
}

// AddTx adds a transaction to the mempool.
func (m *PriorityMempool) AddTx(tx []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hash := string(types.HashTx(tx))

	// Check for duplicate
	if _, exists := m.txs[hash]; exists {
		return types.ErrTxAlreadyExists
	}

	// Check limits
	if len(m.txs) >= m.maxTxs {
		return types.ErrMempoolFull
	}
	if m.sizeBytes+int64(len(tx)) > m.maxBytes {
		return types.ErrMempoolFull
	}

	// Extract fee from transaction (first 8 bytes)
	fee := uint64(0)
	if len(tx) >= 8 {
		fee = binary.BigEndian.Uint64(tx[:8])
	}

	// Add to mempool
	ptx := &priorityTx{
		hash: hash,
		data: tx,
		fee:  fee,
	}
	m.txs[hash] = ptx
	heap.Push(&m.heap, ptx)
	m.sizeBytes += int64(len(tx))

	return nil
}

// RemoveTxs removes transactions by their hashes.
func (m *PriorityMempool) RemoveTxs(hashes [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, h := range hashes {
		hash := string(h)
		if ptx, ok := m.txs[hash]; ok {
			m.sizeBytes -= int64(len(ptx.data))
			delete(m.txs, hash)
			heap.Remove(&m.heap, ptx.index)
		}
	}
}

// ReapTxs returns transactions ordered by fee (highest first).
func (m *PriorityMempool) ReapTxs(maxBytes int64) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Build sorted list from heap
	sorted := make([]*priorityTx, len(m.heap))
	copy(sorted, m.heap)

	// Sort by fee descending
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].fee > sorted[j].fee
	})

	// Collect up to maxBytes
	var result [][]byte
	var totalBytes int64
	for _, ptx := range sorted {
		if totalBytes+int64(len(ptx.data)) > maxBytes {
			break
		}
		result = append(result, ptx.data)
		totalBytes += int64(len(ptx.data))
	}

	return result
}

// HasTx checks if a transaction exists.
func (m *PriorityMempool) HasTx(hash []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, exists := m.txs[string(hash)]
	return exists
}

// GetTx retrieves a transaction by hash.
func (m *PriorityMempool) GetTx(hash []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if ptx, ok := m.txs[string(hash)]; ok {
		return ptx.data, nil
	}
	return nil, types.ErrTxNotFound
}

// Size returns the number of transactions.
func (m *PriorityMempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.txs)
}

// SizeBytes returns total size in bytes.
func (m *PriorityMempool) SizeBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.sizeBytes
}

// Flush removes all transactions.
func (m *PriorityMempool) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.txs = make(map[string]*priorityTx)
	m.heap = make(priorityHeap, 0)
	m.sizeBytes = 0
}

// TxHashes returns all transaction hashes.
func (m *PriorityMempool) TxHashes() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hashes := make([][]byte, 0, len(m.txs))
	for hash := range m.txs {
		hashes = append(hashes, []byte(hash))
	}
	return hashes
}

// SetTxValidator sets the transaction validation function.
func (m *PriorityMempool) SetTxValidator(validator mempool.TxValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validator = validator
}

// Verify interface compliance
var _ mempool.Mempool = (*PriorityMempool)(nil)

// priorityHeap implements heap.Interface for priority ordering.
type priorityHeap []*priorityTx

func (h priorityHeap) Len() int           { return len(h) }
func (h priorityHeap) Less(i, j int) bool { return h[i].fee > h[j].fee } // Higher fee = higher priority
func (h priorityHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *priorityHeap) Push(x any) {
	n := len(*h)
	item := x.(*priorityTx)
	item.index = n
	*h = append(*h, item)
}

func (h *priorityHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*h = old[0 : n-1]
	return item
}
