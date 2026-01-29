package mempool

import (
	"container/heap"
	"sync"
	"time"

	"github.com/blockberries/blockberry/types"
)

// PriorityFunc calculates the priority of a transaction.
// Higher values indicate higher priority.
type PriorityFunc func(tx []byte) int64

// DefaultPriorityFunc returns a constant priority (FIFO behavior).
func DefaultPriorityFunc(_ []byte) int64 {
	return 0
}

// SizePriorityFunc returns priority inversely proportional to transaction size.
// Smaller transactions get higher priority.
func SizePriorityFunc(tx []byte) int64 {
	if len(tx) == 0 {
		return 0
	}
	// Inverse of size, scaled to avoid division issues
	return int64(1_000_000 / len(tx))
}

// mempoolTx represents a transaction in the priority mempool.
type mempoolTx struct {
	hash      []byte
	tx        []byte
	priority  int64
	addedAt   time.Time
	heapIndex int // Index in the heap, maintained by heap.Interface
}

// txHeap implements heap.Interface for priority-based ordering.
type txHeap []*mempoolTx

func (h txHeap) Len() int           { return len(h) }
func (h txHeap) Less(i, j int) bool { return h[i].priority > h[j].priority } // Max-heap: higher priority first
func (h txHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *txHeap) Push(x any) {
	n := len(*h)
	item := x.(*mempoolTx)
	item.heapIndex = n
	*h = append(*h, item)
}

func (h *txHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // Avoid memory leak
	item.heapIndex = -1
	*h = old[0 : n-1]
	return item
}

// PriorityMempool implements Mempool with priority-based transaction ordering.
// Transactions are returned in order of priority (highest first).
// When the mempool is full, lowest priority transactions are evicted.
type PriorityMempool struct {
	mu sync.RWMutex

	// txs maps transaction hash (as string) to mempoolTx
	txs map[string]*mempoolTx

	// heap maintains transactions ordered by priority
	heap txHeap

	// Configuration
	maxTxs   int
	maxBytes int64

	// Priority calculation function
	priorityFunc PriorityFunc

	// Transaction validation
	validator TxValidator

	// Current state
	sizeBytes int64
}

// PriorityMempoolConfig holds configuration for priority mempool.
type PriorityMempoolConfig struct {
	MaxTxs       int
	MaxBytes     int64
	PriorityFunc PriorityFunc
}

// NewPriorityMempool creates a new priority-based mempool.
func NewPriorityMempool(cfg PriorityMempoolConfig) *PriorityMempool {
	pf := cfg.PriorityFunc
	if pf == nil {
		pf = DefaultPriorityFunc
	}

	m := &PriorityMempool{
		txs:          make(map[string]*mempoolTx),
		heap:         make(txHeap, 0),
		maxTxs:       cfg.MaxTxs,
		maxBytes:     cfg.MaxBytes,
		priorityFunc: pf,
	}

	heap.Init(&m.heap)
	return m
}

// AddTx adds a transaction to the mempool with computed priority.
// The transaction is validated before being added.
// If no validator is set, DefaultTxValidator (reject all) is used.
func (m *PriorityMempool) AddTx(tx []byte) error {
	if tx == nil {
		return types.ErrInvalidTx
	}

	hash := types.HashTx(tx)
	hashKey := string(hash)
	txSize := int64(len(tx))

	m.mu.Lock()
	defer m.mu.Unlock()

	// Check if already exists
	if _, exists := m.txs[hashKey]; exists {
		return types.ErrTxAlreadyExists
	}

	// Validate transaction using validator or default (fail-closed)
	validator := m.validator
	if validator == nil {
		validator = DefaultTxValidator
	}
	if err := validator(tx); err != nil {
		return err
	}

	// Calculate priority for the new transaction
	priority := m.priorityFunc(tx)

	// Check if we need to evict transactions
	if m.maxTxs > 0 && len(m.txs) >= m.maxTxs {
		// Try to evict lowest priority transaction
		if !m.evictLowestLocked(priority, txSize) {
			return types.ErrMempoolFull
		}
	}

	// Check byte capacity
	for m.maxBytes > 0 && m.sizeBytes+txSize > m.maxBytes {
		// Try to evict to make room
		if !m.evictLowestLocked(priority, txSize) {
			return types.ErrMempoolFull
		}
	}

	// Add transaction
	mtx := &mempoolTx{
		hash:     hash,
		tx:       tx,
		priority: priority,
		addedAt:  time.Now(),
	}

	m.txs[hashKey] = mtx
	heap.Push(&m.heap, mtx)
	m.sizeBytes += txSize

	return nil
}

// evictLowestLocked evicts the lowest priority transaction if it has lower priority
// than the given priority. Returns true if a transaction was evicted.
// Caller must hold the lock.
//
// Note: For a max-heap, finding the minimum requires O(n) scan since the minimum
// can be anywhere in the leaf nodes. For high-throughput systems, consider using
// a dual-heap or indexed priority queue data structure.
func (m *PriorityMempool) evictLowestLocked(newPriority int64, _ int64) bool {
	n := len(m.heap)
	if n == 0 {
		return false
	}

	// Find the lowest priority transaction in a single O(n) pass.
	// The minimum in a max-heap must be among the leaves (indices n/2 to n-1),
	// but we scan all elements to be safe in case of edge cases.
	lowestIdx := 0
	lowestPriority := m.heap[0].priority

	for i := 1; i < n; i++ {
		if m.heap[i].priority < lowestPriority {
			lowestPriority = m.heap[i].priority
			lowestIdx = i
		}
	}

	// Only evict if the new transaction has higher priority
	if newPriority <= lowestPriority {
		return false
	}

	// Evict the lowest priority transaction
	evicted := m.heap[lowestIdx]
	heap.Remove(&m.heap, lowestIdx)
	delete(m.txs, string(evicted.hash))
	m.sizeBytes -= int64(len(evicted.tx))

	return true
}

// RemoveTxs removes transactions by their hashes.
func (m *PriorityMempool) RemoveTxs(hashes [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, hash := range hashes {
		hashKey := string(hash)
		if mtx, exists := m.txs[hashKey]; exists {
			m.sizeBytes -= int64(len(mtx.tx))
			heap.Remove(&m.heap, mtx.heapIndex)
			delete(m.txs, hashKey)
		}
	}
}

// ReapTxs returns up to maxBytes worth of transactions in priority order.
// Highest priority transactions are returned first.
func (m *PriorityMempool) ReapTxs(maxBytes int64) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxBytes <= 0 {
		maxBytes = m.maxBytes
	}

	if len(m.heap) == 0 {
		return nil
	}

	// Create a sorted copy of transactions by priority
	sorted := make([]*mempoolTx, len(m.heap))
	copy(sorted, m.heap)

	// Sort by priority (highest first) using simple bubble sort
	for i := range sorted {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].priority > sorted[i].priority {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	result := make([][]byte, 0, len(sorted))
	var totalBytes int64

	for _, mtx := range sorted {
		txSize := int64(len(mtx.tx))
		if maxBytes > 0 && totalBytes+txSize > maxBytes {
			break
		}
		result = append(result, mtx.tx)
		totalBytes += txSize
	}

	return result
}

// HasTx checks if a transaction with the given hash exists.
func (m *PriorityMempool) HasTx(hash []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, exists := m.txs[string(hash)]
	return exists
}

// GetTx retrieves a transaction by its hash.
func (m *PriorityMempool) GetTx(hash []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return nil, types.ErrTxNotFound
	}
	return mtx.tx, nil
}

// Size returns the number of transactions in the mempool.
func (m *PriorityMempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.txs)
}

// SizeBytes returns the total size in bytes of all transactions.
func (m *PriorityMempool) SizeBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sizeBytes
}

// Flush removes all transactions from the mempool.
func (m *PriorityMempool) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.txs = make(map[string]*mempoolTx)
	m.heap = make(txHeap, 0)
	m.sizeBytes = 0
}

// TxHashes returns all transaction hashes in the mempool.
func (m *PriorityMempool) TxHashes() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([][]byte, 0, len(m.txs))
	for _, mtx := range m.txs {
		result = append(result, mtx.hash)
	}
	return result
}

// GetPriority returns the priority of a transaction by its hash.
// Returns 0 if the transaction doesn't exist.
func (m *PriorityMempool) GetPriority(hash []byte) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return 0
	}
	return mtx.priority
}

// HighestPriority returns the highest priority value in the mempool.
// Returns 0 if the mempool is empty.
func (m *PriorityMempool) HighestPriority() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.heap) == 0 {
		return 0
	}
	return m.heap[0].priority
}

// SetPriorityFunc updates the priority function.
// Note: This does not recalculate priorities for existing transactions.
func (m *PriorityMempool) SetPriorityFunc(fn PriorityFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if fn == nil {
		fn = DefaultPriorityFunc
	}
	m.priorityFunc = fn
}

// SetTxValidator sets the transaction validation function.
func (m *PriorityMempool) SetTxValidator(validator TxValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validator = validator
}
