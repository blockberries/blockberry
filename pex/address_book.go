package pex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
)

// AddressEntry represents a peer address in the address book.
type AddressEntry struct {
	// Multiaddr is the multiaddress of the peer.
	Multiaddr string `json:"multiaddr"`

	// NodeID is the node identifier.
	NodeID string `json:"node_id"`

	// LastSeen is the Unix timestamp when the peer was last seen.
	LastSeen int64 `json:"last_seen"`

	// Latency is the last measured latency in milliseconds.
	Latency int32 `json:"latency"`

	// IsSeed indicates if this peer is a seed node.
	IsSeed bool `json:"is_seed"`

	// LastAttempt is the Unix timestamp of the last connection attempt.
	LastAttempt int64 `json:"last_attempt"`

	// AttemptCount is the number of consecutive failed connection attempts.
	AttemptCount int `json:"attempt_count"`
}

// SeedNode represents a configured seed node.
type SeedNode struct {
	Multiaddr string `json:"multiaddr"`
}

// addressBookData represents the JSON structure for persistence.
type addressBookData struct {
	Peers []AddressEntry `json:"peers"`
	Seeds []SeedNode     `json:"seeds"`
}

// AddressBook manages known peer addresses with persistence.
type AddressBook struct {
	peers        map[peer.ID]*AddressEntry
	seeds        []SeedNode
	path         string
	maxAddresses int // Maximum total addresses to store (0 = unlimited)
	mu           sync.RWMutex
}

// NewAddressBook creates a new AddressBook with no address limit.
func NewAddressBook(path string) *AddressBook {
	return NewAddressBookWithLimit(path, 0)
}

// NewAddressBookWithLimit creates a new AddressBook with a maximum address limit.
// A limit of 0 means unlimited addresses.
func NewAddressBookWithLimit(path string, maxAddresses int) *AddressBook {
	return &AddressBook{
		peers:        make(map[peer.ID]*AddressEntry),
		seeds:        nil,
		path:         path,
		maxAddresses: maxAddresses,
	}
}

// Load loads the address book from disk.
func (ab *AddressBook) Load() error {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if ab.path == "" {
		return nil
	}

	data, err := os.ReadFile(ab.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading address book: %w", err)
	}

	var book addressBookData
	if err := json.Unmarshal(data, &book); err != nil {
		return fmt.Errorf("parsing address book: %w", err)
	}

	ab.peers = make(map[peer.ID]*AddressEntry)
	for i := range book.Peers {
		entry := book.Peers[i]
		if entry.NodeID == "" {
			continue
		}
		peerID := peer.ID(entry.NodeID)
		ab.peers[peerID] = &entry
	}

	ab.seeds = book.Seeds

	return nil
}

// Save persists the address book to disk.
func (ab *AddressBook) Save() error {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	if ab.path == "" {
		return nil
	}

	var book addressBookData
	book.Seeds = ab.seeds

	for _, entry := range ab.peers {
		book.Peers = append(book.Peers, *entry)
	}

	data, err := json.MarshalIndent(book, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling address book: %w", err)
	}

	dir := filepath.Dir(ab.path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating address book directory: %w", err)
	}

	tmpPath := ab.path + ".tmp"
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		return fmt.Errorf("writing address book: %w", err)
	}

	if err := os.Rename(tmpPath, ab.path); err != nil {
		return fmt.Errorf("renaming address book: %w", err)
	}

	return nil
}

// AddPeer adds or updates a peer in the address book.
// If the address book is at capacity, the oldest non-seed peer is evicted.
func (ab *AddressBook) AddPeer(peerID peer.ID, multiaddr string, nodeID string, latency int32) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	now := time.Now().Unix()

	if entry, ok := ab.peers[peerID]; ok {
		// Update existing peer
		entry.Multiaddr = multiaddr
		entry.NodeID = nodeID
		entry.LastSeen = now
		entry.Latency = latency
		entry.AttemptCount = 0
	} else {
		// Adding a new peer - check capacity first
		if ab.maxAddresses > 0 && len(ab.peers) >= ab.maxAddresses {
			ab.evictOldestLocked()
		}

		ab.peers[peerID] = &AddressEntry{
			Multiaddr: multiaddr,
			NodeID:    nodeID,
			LastSeen:  now,
			Latency:   latency,
			IsSeed:    false,
		}
	}
}

// evictOldestLocked removes the oldest non-seed peer from the address book.
// Must be called with mu held.
func (ab *AddressBook) evictOldestLocked() {
	var oldestID peer.ID
	var oldestTime int64 = 1<<63 - 1 // Max int64

	for id, entry := range ab.peers {
		// Don't evict seed nodes
		if entry.IsSeed {
			continue
		}
		if entry.LastSeen < oldestTime {
			oldestTime = entry.LastSeen
			oldestID = id
		}
	}

	if oldestID != "" {
		delete(ab.peers, oldestID)
	}
}

// MaxAddresses returns the maximum number of addresses allowed in the book.
// Returns 0 if unlimited.
func (ab *AddressBook) MaxAddresses() int {
	ab.mu.RLock()
	defer ab.mu.RUnlock()
	return ab.maxAddresses
}

// SetMaxAddresses sets the maximum number of addresses allowed in the book.
// If the new limit is lower than the current size, oldest peers are evicted.
func (ab *AddressBook) SetMaxAddresses(max int) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	ab.maxAddresses = max

	// Evict peers if over the new limit
	if max > 0 {
		for len(ab.peers) > max {
			ab.evictOldestLocked()
		}
	}
}

// RemovePeer removes a peer from the address book.
func (ab *AddressBook) RemovePeer(peerID peer.ID) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	delete(ab.peers, peerID)
}

// GetPeer retrieves a peer entry from the address book.
func (ab *AddressBook) GetPeer(peerID peer.ID) (*AddressEntry, bool) {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	entry, ok := ab.peers[peerID]
	if !ok {
		return nil, false
	}
	entryCopy := *entry
	return &entryCopy, true
}

// HasPeer checks if a peer exists in the address book.
func (ab *AddressBook) HasPeer(peerID peer.ID) bool {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	_, ok := ab.peers[peerID]
	return ok
}

// GetPeers returns all peers in the address book.
func (ab *AddressBook) GetPeers() map[peer.ID]*AddressEntry {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	result := make(map[peer.ID]*AddressEntry, len(ab.peers))
	for id, entry := range ab.peers {
		entryCopy := *entry
		result[id] = &entryCopy
	}
	return result
}

// Size returns the number of peers in the address book.
func (ab *AddressBook) Size() int {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	return len(ab.peers)
}

// MarkSeed marks a peer as a seed node.
func (ab *AddressBook) MarkSeed(peerID peer.ID) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if entry, ok := ab.peers[peerID]; ok {
		entry.IsSeed = true
	}
}

// UpdateLastSeen updates the last seen timestamp for a peer.
func (ab *AddressBook) UpdateLastSeen(peerID peer.ID) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if entry, ok := ab.peers[peerID]; ok {
		entry.LastSeen = time.Now().Unix()
	}
}

// UpdateLatency updates the latency for a peer.
func (ab *AddressBook) UpdateLatency(peerID peer.ID, latency int32) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if entry, ok := ab.peers[peerID]; ok {
		entry.Latency = latency
	}
}

// RecordAttempt records a connection attempt for a peer.
func (ab *AddressBook) RecordAttempt(peerID peer.ID) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if entry, ok := ab.peers[peerID]; ok {
		entry.LastAttempt = time.Now().Unix()
		entry.AttemptCount++
	}
}

// ResetAttempts resets the attempt count for a peer (on successful connection).
func (ab *AddressBook) ResetAttempts(peerID peer.ID) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	if entry, ok := ab.peers[peerID]; ok {
		entry.AttemptCount = 0
	}
}

// GetPeersForExchange returns peers suitable for peer exchange.
// Filters by lastSeenAfter timestamp and returns up to max peers.
// Peers are sorted by last seen time (most recent first).
func (ab *AddressBook) GetPeersForExchange(lastSeenAfter int64, max int) []*AddressEntry {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	var candidates []*AddressEntry
	for _, entry := range ab.peers {
		if entry.LastSeen >= lastSeenAfter {
			entryCopy := *entry
			candidates = append(candidates, &entryCopy)
		}
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].LastSeen > candidates[j].LastSeen
	})

	return candidates[:min(len(candidates), max)]
}

// GetPeersToConnect returns peers that are good candidates for connection.
// Excludes peers with too many recent failed attempts.
func (ab *AddressBook) GetPeersToConnect(exclude map[peer.ID]bool, max int) []*AddressEntry {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	now := time.Now().Unix()
	maxBackoff := int64(3600)

	candidates := make([]*AddressEntry, 0, len(ab.peers))
	for peerID, entry := range ab.peers {
		if exclude != nil && exclude[peerID] {
			continue
		}

		if entry.AttemptCount > 0 {
			backoff := min(int64(1<<entry.AttemptCount), maxBackoff)
			if now-entry.LastAttempt < backoff {
				continue
			}
		}

		entryCopy := *entry
		candidates = append(candidates, &entryCopy)
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Latency != candidates[j].Latency {
			return candidates[i].Latency < candidates[j].Latency
		}
		return candidates[i].LastSeen > candidates[j].LastSeen
	})

	return candidates[:min(len(candidates), max)]
}

// AddSeeds adds seed nodes to the address book.
func (ab *AddressBook) AddSeeds(seeds []string) {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	for _, addr := range seeds {
		ab.seeds = append(ab.seeds, SeedNode{Multiaddr: addr})
	}
}

// GetSeeds returns the configured seed nodes.
func (ab *AddressBook) GetSeeds() []SeedNode {
	ab.mu.RLock()
	defer ab.mu.RUnlock()

	result := make([]SeedNode, len(ab.seeds))
	copy(result, ab.seeds)
	return result
}

// Prune removes stale peers from the address book.
// Peers not seen since maxAge are removed (unless they are seeds).
func (ab *AddressBook) Prune(maxAge time.Duration) int {
	ab.mu.Lock()
	defer ab.mu.Unlock()

	cutoff := time.Now().Add(-maxAge).Unix()
	pruned := 0

	for peerID, entry := range ab.peers {
		if entry.IsSeed {
			continue
		}
		if entry.LastSeen < cutoff {
			delete(ab.peers, peerID)
			pruned++
		}
	}

	return pruned
}
