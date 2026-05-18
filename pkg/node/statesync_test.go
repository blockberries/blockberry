package node

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/pkg/statestore"
)

// stubSnapshotStore is a minimal SnapshotStore for runner-construction
// tests. The runner itself doesn't invoke any method until it starts,
// and Start is what we drive in tests selectively.
type stubSnapshotStore struct{}

func (stubSnapshotStore) Create(int64) (*statestore.Snapshot, error)                { return nil, nil }
func (stubSnapshotStore) List() ([]*statestore.SnapshotInfo, error)                 { return nil, nil }
func (stubSnapshotStore) Load([]byte) (*statestore.Snapshot, error)                 { return nil, nil }
func (stubSnapshotStore) LoadChunk([]byte, int) (*statestore.SnapshotChunk, error)  { return nil, nil }
func (stubSnapshotStore) Delete([]byte) error                                       { return nil }
func (stubSnapshotStore) Prune(int) error                                           { return nil }
func (stubSnapshotStore) Has([]byte) bool                                           { return false }
func (stubSnapshotStore) Import(*statestore.Snapshot, statestore.ChunkProvider) error {
	return nil
}

func TestStateSyncOptions_DefaultsApplied(t *testing.T) {
	o := (StateSyncOptions{}).withDefaults()
	require.Equal(t, 5*time.Second, o.DiscoveryInterval)
	require.Equal(t, 10*time.Second, o.ChunkRequestTimeout)
	require.Equal(t, 3, o.MaxChunkRetries)
}

func TestStateSyncOptions_RespectsExplicitValues(t *testing.T) {
	o := StateSyncOptions{
		DiscoveryInterval:   2 * time.Second,
		ChunkRequestTimeout: 30 * time.Second,
		MaxChunkRetries:     10,
	}.withDefaults()
	require.Equal(t, 2*time.Second, o.DiscoveryInterval)
	require.Equal(t, 30*time.Second, o.ChunkRequestTimeout)
	require.Equal(t, 10, o.MaxChunkRetries)
}

func TestNode_NewStateSyncRunner_ReturnsUsableRunner(t *testing.T) {
	cfg := testConfig(t)
	node, err := NewNodeBuilder(cfg).Build()
	require.NoError(t, err)

	r := node.NewStateSyncRunner(stubSnapshotStore{}, StateSyncOptions{
		TrustHeight: 100,
		TrustHash:   []byte{0xAA},
	})
	require.NotNil(t, r)

	// Before Start: not running, zero progress.
	require.False(t, r.IsRunning())
	require.Equal(t, 0, r.Progress())

	// Callbacks can be wired without side effects.
	r.SetOnComplete(func(int64, []byte) {})
	r.SetOnFailed(func(error) {})

	// Start transitions to running. The node isn't actually started so
	// the runner won't make network progress, but Start itself just
	// flips state and launches the loop goroutine.
	require.NoError(t, r.Start())
	require.True(t, r.IsRunning())

	// Stop returns cleanly.
	require.NoError(t, r.Stop())
	require.False(t, r.IsRunning())
}

func TestStateSyncStreamName_IsTheExportedConstant(t *testing.T) {
	// The exported constant is what raspberry uses when registering
	// the stream handler. Confirm it matches the well-known wire name.
	require.Equal(t, "statesync", StateSyncStreamName)
}
