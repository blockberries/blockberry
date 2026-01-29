package mempool

import (
	"container/heap"
	"sync"
	"time"

	"github.com/blockberries/blockberry/types"
)

// TTLMempool wraps PriorityMempool with time-based transaction expiration.
// Expired transactions are automatically removed during cleanup cycles.
type TTLMempool struct {
	mu sync.RWMutex

	// txs maps transaction hash (as string) to ttlTx
	txs map[string]*ttlTx

	// priorityHeap maintains transactions ordered by priority (max-heap)
	priorityHeap ttlTxHeap

	// Configuration
	maxTxs    int
	maxBytes  int64
	maxTxSize int64 // Maximum size of a single transaction
	ttl       time.Duration

	// Priority calculation function
	priorityFunc PriorityFunc

	// Transaction validation
	validator TxValidator

	// Current state
	sizeBytes int64

	// Cleanup
	cleanupInterval time.Duration
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// ttlTx represents a transaction with expiration time.
type ttlTx struct {
	hash      []byte
	tx        []byte
	priority  int64
	addedAt   time.Time
	expiresAt time.Time
	heapIndex int
}

// ttlTxHeap implements heap.Interface for priority-based ordering.
type ttlTxHeap []*ttlTx

func (h ttlTxHeap) Len() int           { return len(h) }
func (h ttlTxHeap) Less(i, j int) bool { return h[i].priority > h[j].priority }
func (h ttlTxHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].heapIndex = i
	h[j].heapIndex = j
}

func (h *ttlTxHeap) Push(x any) {
	n := len(*h)
	item := x.(*ttlTx)
	item.heapIndex = n
	*h = append(*h, item)
}

func (h *ttlTxHeap) Pop() any {
	old := *h
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.heapIndex = -1
	*h = old[0 : n-1]
	return item
}

// TTLMempoolConfig holds configuration for TTL mempool.
type TTLMempoolConfig struct {
	MaxTxs          int
	MaxBytes        int64
	MaxTxSize       int64 // Maximum size of a single transaction
	TTL             time.Duration
	CleanupInterval time.Duration
	PriorityFunc    PriorityFunc
}

// NewTTLMempool creates a new mempool with TTL-based expiration.
func NewTTLMempool(cfg TTLMempoolConfig) *TTLMempool {
	pf := cfg.PriorityFunc
	if pf == nil {
		pf = DefaultPriorityFunc
	}

	cleanupInterval := cfg.CleanupInterval
	if cleanupInterval <= 0 {
		cleanupInterval = time.Minute
	}

	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}

	m := &TTLMempool{
		txs:             make(map[string]*ttlTx),
		priorityHeap:    make(ttlTxHeap, 0),
		maxTxs:          cfg.MaxTxs,
		maxBytes:        cfg.MaxBytes,
		maxTxSize:       cfg.MaxTxSize,
		ttl:             ttl,
		priorityFunc:    pf,
		cleanupInterval: cleanupInterval,
		stopCh:          make(chan struct{}),
	}

	heap.Init(&m.priorityHeap)
	m.wg.Add(1)
	go m.cleanupLoop()

	return m
}

// cleanupLoop periodically removes expired transactions.
func (m *TTLMempool) cleanupLoop() {
	defer m.wg.Done()

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

// removeExpired removes all expired transactions.
func (m *TTLMempool) removeExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	var expired [][]byte

	for hashKey, mtx := range m.txs {
		if now.After(mtx.expiresAt) {
			expired = append(expired, []byte(hashKey))
		}
	}

	for _, hashKey := range expired {
		if mtx, exists := m.txs[string(hashKey)]; exists {
			m.sizeBytes -= int64(len(mtx.tx))
			heap.Remove(&m.priorityHeap, mtx.heapIndex)
			delete(m.txs, string(hashKey))
		}
	}

	return len(expired)
}

// Stop stops the cleanup goroutine.
func (m *TTLMempool) Stop() {
	close(m.stopCh)
	m.wg.Wait()
}

// AddTx adds a transaction to the mempool with the configured TTL.
func (m *TTLMempool) AddTx(tx []byte) error {
	return m.AddTxWithTTL(tx, m.ttl)
}

// AddTxWithTTL adds a transaction with a custom TTL.
// The transaction is validated before being added.
// If no validator is set, DefaultTxValidator (reject all) is used.
func (m *TTLMempool) AddTxWithTTL(tx []byte, ttl time.Duration) error {
	if tx == nil {
		return types.ErrInvalidTx
	}

	txSize := int64(len(tx))

	// Check individual transaction size limit
	if m.maxTxSize > 0 && txSize > m.maxTxSize {
		return types.ErrTxTooLarge
	}

	hash := types.HashTx(tx)
	hashKey := string(hash)
	now := time.Now()

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

	// Calculate priority
	priority := m.priorityFunc(tx)

	// Check capacity and evict if needed
	if m.maxTxs > 0 && len(m.txs) >= m.maxTxs {
		if !m.evictLowestLocked(priority) {
			return types.ErrMempoolFull
		}
	}

	for m.maxBytes > 0 && m.sizeBytes+txSize > m.maxBytes {
		if !m.evictLowestLocked(priority) {
			return types.ErrMempoolFull
		}
	}

	// Add transaction with defensive copies to prevent external mutation
	mtx := &ttlTx{
		hash:      append([]byte(nil), hash...),
		tx:        append([]byte(nil), tx...),
		priority:  priority,
		addedAt:   now,
		expiresAt: now.Add(ttl),
	}

	m.txs[hashKey] = mtx
	heap.Push(&m.priorityHeap, mtx)
	m.sizeBytes += txSize

	return nil
}

// evictLowestLocked evicts the lowest priority transaction if possible.
func (m *TTLMempool) evictLowestLocked(newPriority int64) bool {
	if len(m.priorityHeap) == 0 {
		return false
	}

	// Find lowest priority transaction
	var lowestIdx int
	lowestPriority := m.priorityHeap[0].priority

	for i := range m.priorityHeap {
		if m.priorityHeap[i].priority < lowestPriority {
			lowestPriority = m.priorityHeap[i].priority
			lowestIdx = i
		}
	}

	if newPriority <= lowestPriority {
		return false
	}

	evicted := m.priorityHeap[lowestIdx]
	heap.Remove(&m.priorityHeap, lowestIdx)
	delete(m.txs, string(evicted.hash))
	m.sizeBytes -= int64(len(evicted.tx))

	return true
}

// RemoveTxs removes transactions by their hashes.
func (m *TTLMempool) RemoveTxs(hashes [][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, hash := range hashes {
		hashKey := string(hash)
		if mtx, exists := m.txs[hashKey]; exists {
			m.sizeBytes -= int64(len(mtx.tx))
			heap.Remove(&m.priorityHeap, mtx.heapIndex)
			delete(m.txs, hashKey)
		}
	}
}

// ReapTxs returns up to maxBytes worth of non-expired transactions in priority order.
func (m *TTLMempool) ReapTxs(maxBytes int64) [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if maxBytes <= 0 {
		maxBytes = m.maxBytes
	}

	if len(m.priorityHeap) == 0 {
		return nil
	}

	now := time.Now()

	// Create sorted copy
	sorted := make([]*ttlTx, 0, len(m.priorityHeap))
	for _, mtx := range m.priorityHeap {
		if !now.After(mtx.expiresAt) {
			sorted = append(sorted, mtx)
		}
	}

	// Sort by priority (highest first)
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
		// Return defensive copy to prevent external mutation
		result = append(result, append([]byte(nil), mtx.tx...))
		totalBytes += txSize
	}

	return result
}

// HasTx checks if a non-expired transaction with the given hash exists.
func (m *TTLMempool) HasTx(hash []byte) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return false
	}
	return !time.Now().After(mtx.expiresAt)
}

// GetTx retrieves a non-expired transaction by its hash.
// Returns a defensive copy to prevent external mutation.
func (m *TTLMempool) GetTx(hash []byte) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return nil, types.ErrTxNotFound
	}
	if time.Now().After(mtx.expiresAt) {
		return nil, types.ErrTxNotFound
	}
	return append([]byte(nil), mtx.tx...), nil
}

// Size returns the number of transactions in the mempool (including expired).
func (m *TTLMempool) Size() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.txs)
}

// SizeActive returns the number of non-expired transactions.
func (m *TTLMempool) SizeActive() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	now := time.Now()
	count := 0
	for _, mtx := range m.txs {
		if !now.After(mtx.expiresAt) {
			count++
		}
	}
	return count
}

// SizeBytes returns the total size in bytes of all transactions.
func (m *TTLMempool) SizeBytes() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.sizeBytes
}

// Flush removes all transactions from the mempool.
func (m *TTLMempool) Flush() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.txs = make(map[string]*ttlTx)
	m.priorityHeap = make(ttlTxHeap, 0)
	m.sizeBytes = 0
}

// TxHashes returns all transaction hashes in the mempool.
// Returns defensive copies to prevent external mutation.
func (m *TTLMempool) TxHashes() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([][]byte, 0, len(m.txs))
	for _, mtx := range m.txs {
		result = append(result, append([]byte(nil), mtx.hash...))
	}
	return result
}

// GetTTL returns the remaining TTL for a transaction.
// Returns 0 if the transaction doesn't exist or is expired.
func (m *TTLMempool) GetTTL(hash []byte) time.Duration {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return 0
	}

	remaining := time.Until(mtx.expiresAt)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// ExtendTTL extends the TTL for a transaction.
// Returns false if the transaction doesn't exist.
func (m *TTLMempool) ExtendTTL(hash []byte, extension time.Duration) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return false
	}

	mtx.expiresAt = mtx.expiresAt.Add(extension)
	return true
}

// SetTTL sets the TTL for a transaction to expire at a specific time.
func (m *TTLMempool) SetTTL(hash []byte, expiresAt time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return false
	}

	mtx.expiresAt = expiresAt
	return true
}

// GetPriority returns the priority of a transaction.
func (m *TTLMempool) GetPriority(hash []byte) int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mtx, exists := m.txs[string(hash)]
	if !exists {
		return 0
	}
	return mtx.priority
}

// SetTxValidator sets the transaction validation function.
func (m *TTLMempool) SetTxValidator(validator TxValidator) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.validator = validator
}
