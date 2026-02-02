package pex

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/stretchr/testify/require"
)

func TestNewAddressBook(t *testing.T) {
	ab := NewAddressBook("test.json")
	require.NotNil(t, ab)
	require.NotNil(t, ab.peers)
	require.Equal(t, "test.json", ab.path)
}

func TestAddressBook_AddAndGetPeer(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	entry, ok := ab.GetPeer(peerID)
	require.True(t, ok)
	require.Equal(t, "/ip4/127.0.0.1/tcp/26656", entry.Multiaddr)
	require.Equal(t, peerID.String(), entry.NodeID)
	require.Equal(t, int32(50), entry.Latency)
	require.False(t, entry.IsSeed)
	require.Greater(t, entry.LastSeen, int64(0))
}

func TestAddressBook_UpdateExistingPeer(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)
	ab.AddPeer(peerID, "/ip4/192.168.1.1/tcp/26656", peerID.String(), 25)

	entry, ok := ab.GetPeer(peerID)
	require.True(t, ok)
	require.Equal(t, "/ip4/192.168.1.1/tcp/26656", entry.Multiaddr)
	require.Equal(t, int32(25), entry.Latency)
}

func TestAddressBook_RemovePeer(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)
	require.True(t, ab.HasPeer(peerID))

	ab.RemovePeer(peerID)
	require.False(t, ab.HasPeer(peerID))

	_, ok := ab.GetPeer(peerID)
	require.False(t, ok)
}

func TestAddressBook_HasPeer(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	require.False(t, ab.HasPeer(peerID))

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)
	require.True(t, ab.HasPeer(peerID))
}

func TestAddressBook_Size(t *testing.T) {
	ab := NewAddressBook("")

	require.Equal(t, 0, ab.Size())

	ab.AddPeer(peer.ID("peer-1"), "/ip4/127.0.0.1/tcp/26656", "peer-1", 50)
	require.Equal(t, 1, ab.Size())

	ab.AddPeer(peer.ID("peer-2"), "/ip4/127.0.0.2/tcp/26656", "peer-2", 60)
	require.Equal(t, 2, ab.Size())

	ab.RemovePeer(peer.ID("peer-1"))
	require.Equal(t, 1, ab.Size())
}

func TestAddressBook_GetPeers(t *testing.T) {
	ab := NewAddressBook("")

	ab.AddPeer(peer.ID("peer-1"), "/ip4/127.0.0.1/tcp/26656", "peer-1", 50)
	ab.AddPeer(peer.ID("peer-2"), "/ip4/127.0.0.2/tcp/26656", "peer-2", 60)

	peers := ab.GetPeers()
	require.Len(t, peers, 2)
}

func TestAddressBook_MarkSeed(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	entry, _ := ab.GetPeer(peerID)
	require.False(t, entry.IsSeed)

	ab.MarkSeed(peerID)

	entry, _ = ab.GetPeer(peerID)
	require.True(t, entry.IsSeed)
}

func TestAddressBook_UpdateLastSeen(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	entry1, _ := ab.GetPeer(peerID)
	time.Sleep(10 * time.Millisecond)

	ab.UpdateLastSeen(peerID)

	entry2, _ := ab.GetPeer(peerID)
	require.GreaterOrEqual(t, entry2.LastSeen, entry1.LastSeen)
}

func TestAddressBook_UpdateLatency(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	ab.UpdateLatency(peerID, 100)

	entry, _ := ab.GetPeer(peerID)
	require.Equal(t, int32(100), entry.Latency)
}

func TestAddressBook_RecordAndResetAttempts(t *testing.T) {
	ab := NewAddressBook("")
	peerID := peer.ID("test-peer-1")

	ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), 50)

	entry, _ := ab.GetPeer(peerID)
	require.Equal(t, 0, entry.AttemptCount)

	ab.RecordAttempt(peerID)
	ab.RecordAttempt(peerID)

	entry, _ = ab.GetPeer(peerID)
	require.Equal(t, 2, entry.AttemptCount)
	require.Greater(t, entry.LastAttempt, int64(0))

	ab.ResetAttempts(peerID)

	entry, _ = ab.GetPeer(peerID)
	require.Equal(t, 0, entry.AttemptCount)
}

func TestAddressBook_GetPeersForExchange(t *testing.T) {
	ab := NewAddressBook("")
	now := time.Now().Unix()

	ab.peers[peer.ID("old-peer")] = &AddressEntry{
		NodeID:   "old-peer",
		LastSeen: now - 3600, // 1 hour ago
		Latency:  50,
	}
	ab.peers[peer.ID("new-peer-1")] = &AddressEntry{
		NodeID:   "new-peer-1",
		LastSeen: now - 60, // 1 minute ago
		Latency:  30,
	}
	ab.peers[peer.ID("new-peer-2")] = &AddressEntry{
		NodeID:   "new-peer-2",
		LastSeen: now - 30, // 30 seconds ago
		Latency:  40,
	}

	t.Run("filters by lastSeen", func(t *testing.T) {
		peers := ab.GetPeersForExchange(now-120, 10)
		require.Len(t, peers, 2)
	})

	t.Run("respects max limit", func(t *testing.T) {
		peers := ab.GetPeersForExchange(now-7200, 1)
		require.Len(t, peers, 1)
	})

	t.Run("sorts by lastSeen descending", func(t *testing.T) {
		peers := ab.GetPeersForExchange(now-7200, 10)
		require.Len(t, peers, 3)
		require.Equal(t, "new-peer-2", peers[0].NodeID)
		require.Equal(t, "new-peer-1", peers[1].NodeID)
		require.Equal(t, "old-peer", peers[2].NodeID)
	})
}

func TestAddressBook_GetPeersToConnect(t *testing.T) {
	ab := NewAddressBook("")
	now := time.Now().Unix()

	ab.peers[peer.ID("failed-peer")] = &AddressEntry{
		NodeID:       "failed-peer",
		LastSeen:     now,
		Latency:      50,
		LastAttempt:  now,
		AttemptCount: 3,
	}
	ab.peers[peer.ID("good-peer")] = &AddressEntry{
		NodeID:       "good-peer",
		LastSeen:     now,
		Latency:      30,
		AttemptCount: 0,
	}
	ab.peers[peer.ID("slow-peer")] = &AddressEntry{
		NodeID:       "slow-peer",
		LastSeen:     now,
		Latency:      100,
		AttemptCount: 0,
	}

	t.Run("excludes peers with recent failures", func(t *testing.T) {
		peers := ab.GetPeersToConnect(nil, 10)
		require.Len(t, peers, 2)
		for _, p := range peers {
			require.NotEqual(t, "failed-peer", p.NodeID)
		}
	})

	t.Run("excludes specified peers", func(t *testing.T) {
		exclude := map[peer.ID]bool{peer.ID("good-peer"): true}
		peers := ab.GetPeersToConnect(exclude, 10)
		require.Len(t, peers, 1)
		require.Equal(t, "slow-peer", peers[0].NodeID)
	})

	t.Run("sorts by latency", func(t *testing.T) {
		peers := ab.GetPeersToConnect(nil, 10)
		require.Len(t, peers, 2)
		require.Equal(t, "good-peer", peers[0].NodeID)
		require.Equal(t, "slow-peer", peers[1].NodeID)
	})

	t.Run("respects max limit", func(t *testing.T) {
		peers := ab.GetPeersToConnect(nil, 1)
		require.Len(t, peers, 1)
	})
}

func TestAddressBook_AddAndGetSeeds(t *testing.T) {
	ab := NewAddressBook("")

	require.Len(t, ab.GetSeeds(), 0)

	ab.AddSeeds([]string{"/ip4/1.2.3.4/tcp/26656", "/ip4/5.6.7.8/tcp/26656"})

	seeds := ab.GetSeeds()
	require.Len(t, seeds, 2)
	require.Equal(t, "/ip4/1.2.3.4/tcp/26656", seeds[0].Multiaddr)
	require.Equal(t, "/ip4/5.6.7.8/tcp/26656", seeds[1].Multiaddr)
}

func TestAddressBook_Prune(t *testing.T) {
	ab := NewAddressBook("")
	now := time.Now().Unix()

	ab.peers[peer.ID("old-peer")] = &AddressEntry{
		NodeID:   "old-peer",
		LastSeen: now - 7200,
	}
	ab.peers[peer.ID("new-peer")] = &AddressEntry{
		NodeID:   "new-peer",
		LastSeen: now,
	}
	ab.peers[peer.ID("old-seed")] = &AddressEntry{
		NodeID:   "old-seed",
		LastSeen: now - 7200,
		IsSeed:   true,
	}

	pruned := ab.Prune(time.Hour)
	require.Equal(t, 1, pruned)
	require.False(t, ab.HasPeer(peer.ID("old-peer")))
	require.True(t, ab.HasPeer(peer.ID("new-peer")))
	require.True(t, ab.HasPeer(peer.ID("old-seed")))
}

func TestAddressBook_SaveAndLoad(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "addrbook.json")

	ab1 := NewAddressBook(path)
	ab1.AddPeer(peer.ID("peer-1"), "/ip4/127.0.0.1/tcp/26656", "peer-1", 50)
	ab1.AddPeer(peer.ID("peer-2"), "/ip4/127.0.0.2/tcp/26656", "peer-2", 60)
	ab1.MarkSeed(peer.ID("peer-1"))
	ab1.AddSeeds([]string{"/ip4/1.2.3.4/tcp/26656"})

	err := ab1.Save()
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NotEmpty(t, data)

	ab2 := NewAddressBook(path)
	err = ab2.Load()
	require.NoError(t, err)

	require.Equal(t, ab1.Size(), ab2.Size())

	entry1, ok := ab2.GetPeer(peer.ID("peer-1"))
	require.True(t, ok)
	require.Equal(t, "/ip4/127.0.0.1/tcp/26656", entry1.Multiaddr)
	require.True(t, entry1.IsSeed)

	entry2, ok := ab2.GetPeer(peer.ID("peer-2"))
	require.True(t, ok)
	require.Equal(t, "/ip4/127.0.0.2/tcp/26656", entry2.Multiaddr)
	require.False(t, entry2.IsSeed)

	seeds := ab2.GetSeeds()
	require.Len(t, seeds, 1)
	require.Equal(t, "/ip4/1.2.3.4/tcp/26656", seeds[0].Multiaddr)
}

func TestAddressBook_LoadNonExistent(t *testing.T) {
	ab := NewAddressBook("/nonexistent/path/addrbook.json")
	err := ab.Load()
	require.NoError(t, err)
	require.Equal(t, 0, ab.Size())
}

func TestAddressBook_SaveWithEmptyPath(t *testing.T) {
	ab := NewAddressBook("")
	ab.AddPeer(peer.ID("peer-1"), "/ip4/127.0.0.1/tcp/26656", "peer-1", 50)

	err := ab.Save()
	require.NoError(t, err)
}

func TestAddressBook_LoadWithEmptyPath(t *testing.T) {
	ab := NewAddressBook("")

	err := ab.Load()
	require.NoError(t, err)
}

func TestAddressBook_Concurrency(t *testing.T) {
	ab := NewAddressBook("")

	done := make(chan bool)

	go func() {
		for i := range 100 {
			peerID := peer.ID(string(rune('A' + i%26)))
			ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), int32(i))
		}
		done <- true
	}()

	go func() {
		for i := range 100 {
			peerID := peer.ID(string(rune('A' + i%26)))
			ab.HasPeer(peerID)
			ab.GetPeer(peerID)
		}
		done <- true
	}()

	go func() {
		for range 100 {
			ab.GetPeers()
			ab.Size()
		}
		done <- true
	}()

	go func() {
		for range 100 {
			ab.GetPeersForExchange(0, 10)
			ab.GetPeersToConnect(nil, 10)
		}
		done <- true
	}()

	for range 4 {
		<-done
	}
}

func TestNewAddressBookWithLimit(t *testing.T) {
	ab := NewAddressBookWithLimit("test.json", 100)
	require.NotNil(t, ab)
	require.Equal(t, 100, ab.MaxAddresses())
}

func TestAddressBook_MaxAddresses_Unlimited(t *testing.T) {
	ab := NewAddressBook("")
	require.Equal(t, 0, ab.MaxAddresses())

	// Should accept unlimited addresses
	for i := range 1000 {
		peerID := peer.ID(string(rune('A' + (i % 26))) + string(rune('0'+(i/26))))
		ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), int32(i))
	}
	require.Equal(t, 1000, ab.Size())
}

func TestAddressBook_MaxAddresses_Enforced(t *testing.T) {
	ab := NewAddressBookWithLimit("", 5)
	now := time.Now().Unix()

	// Add 5 peers
	for i := range 5 {
		peerID := peer.ID(string(rune('A' + i)))
		ab.mu.Lock()
		ab.peers[peerID] = &AddressEntry{
			NodeID:   peerID.String(),
			Multiaddr: "/ip4/127.0.0.1/tcp/26656",
			LastSeen: now - int64(i*100), // Older as i increases
		}
		ab.mu.Unlock()
	}
	require.Equal(t, 5, ab.Size())

	// Add one more - should evict oldest
	newPeerID := peer.ID("F")
	ab.AddPeer(newPeerID, "/ip4/127.0.0.1/tcp/26656", newPeerID.String(), 0)

	require.Equal(t, 5, ab.Size())
	require.True(t, ab.HasPeer(newPeerID))
	// Peer E was oldest (lastSeen = now - 400), should be evicted
	require.False(t, ab.HasPeer(peer.ID("E")))
}

func TestAddressBook_MaxAddresses_UpdateExistingDoesNotEvict(t *testing.T) {
	ab := NewAddressBookWithLimit("", 3)

	// Add 3 peers
	ab.AddPeer(peer.ID("A"), "/ip4/127.0.0.1/tcp/26656", "A", 0)
	ab.AddPeer(peer.ID("B"), "/ip4/127.0.0.2/tcp/26656", "B", 0)
	ab.AddPeer(peer.ID("C"), "/ip4/127.0.0.3/tcp/26656", "C", 0)

	require.Equal(t, 3, ab.Size())

	// Update existing peer - should not evict
	ab.AddPeer(peer.ID("A"), "/ip4/192.168.1.1/tcp/26656", "A", 100)

	require.Equal(t, 3, ab.Size())
	require.True(t, ab.HasPeer(peer.ID("A")))
	require.True(t, ab.HasPeer(peer.ID("B")))
	require.True(t, ab.HasPeer(peer.ID("C")))

	entry, _ := ab.GetPeer(peer.ID("A"))
	require.Equal(t, "/ip4/192.168.1.1/tcp/26656", entry.Multiaddr)
}

func TestAddressBook_MaxAddresses_SeedsNotEvicted(t *testing.T) {
	ab := NewAddressBookWithLimit("", 3)
	now := time.Now().Unix()

	// Add 2 regular peers and 1 seed (oldest)
	ab.mu.Lock()
	ab.peers[peer.ID("seed")] = &AddressEntry{
		NodeID:   "seed",
		Multiaddr: "/ip4/127.0.0.1/tcp/26656",
		LastSeen: now - 1000, // Oldest
		IsSeed:   true,
	}
	ab.peers[peer.ID("peer1")] = &AddressEntry{
		NodeID:   "peer1",
		Multiaddr: "/ip4/127.0.0.2/tcp/26656",
		LastSeen: now - 500,
	}
	ab.peers[peer.ID("peer2")] = &AddressEntry{
		NodeID:   "peer2",
		Multiaddr: "/ip4/127.0.0.3/tcp/26656",
		LastSeen: now - 100,
	}
	ab.mu.Unlock()

	require.Equal(t, 3, ab.Size())

	// Add new peer - should evict peer1 (oldest non-seed), not seed
	ab.AddPeer(peer.ID("peer3"), "/ip4/127.0.0.4/tcp/26656", "peer3", 0)

	require.Equal(t, 3, ab.Size())
	require.True(t, ab.HasPeer(peer.ID("seed"))) // Seed preserved
	require.False(t, ab.HasPeer(peer.ID("peer1"))) // Oldest non-seed evicted
	require.True(t, ab.HasPeer(peer.ID("peer2")))
	require.True(t, ab.HasPeer(peer.ID("peer3")))
}

func TestAddressBook_SetMaxAddresses(t *testing.T) {
	ab := NewAddressBook("")

	// Add 10 peers
	for i := range 10 {
		peerID := peer.ID(string(rune('A' + i)))
		ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), int32(i))
	}
	require.Equal(t, 10, ab.Size())

	// Set limit lower than current size
	ab.SetMaxAddresses(5)
	require.Equal(t, 5, ab.MaxAddresses())
	require.Equal(t, 5, ab.Size())

	// Increasing limit doesn't change current size
	ab.SetMaxAddresses(20)
	require.Equal(t, 20, ab.MaxAddresses())
	require.Equal(t, 5, ab.Size())
}

func TestAddressBook_SetMaxAddresses_Zero(t *testing.T) {
	ab := NewAddressBookWithLimit("", 5)

	// Add 3 peers
	for i := range 3 {
		peerID := peer.ID(string(rune('A' + i)))
		ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), int32(i))
	}

	// Set to unlimited
	ab.SetMaxAddresses(0)
	require.Equal(t, 0, ab.MaxAddresses())
	require.Equal(t, 3, ab.Size())

	// Can now add unlimited
	for i := 3; i < 100; i++ {
		peerID := peer.ID(string(rune('A' + (i % 26))) + string(rune('0'+(i/26))))
		ab.AddPeer(peerID, "/ip4/127.0.0.1/tcp/26656", peerID.String(), int32(i))
	}
	require.Equal(t, 100, ab.Size())
}
