package p2p

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllStreams(t *testing.T) {
	streams := AllStreams()

	require.Len(t, streams, 6)
	require.Contains(t, streams, StreamPEX)
	require.Contains(t, streams, StreamTransactions)
	require.Contains(t, streams, StreamBlockSync)
	require.Contains(t, streams, StreamBlocks)
	require.Contains(t, streams, StreamConsensus)
	require.Contains(t, streams, StreamHousekeeping)
}

func TestStreamConstants(t *testing.T) {
	// Verify stream constants are defined
	require.Equal(t, "handshake", StreamHandshake)
	require.Equal(t, "pex", StreamPEX)
	require.Equal(t, "transactions", StreamTransactions)
	require.Equal(t, "blocksync", StreamBlockSync)
	require.Equal(t, "blocks", StreamBlocks)
	require.Equal(t, "consensus", StreamConsensus)
	require.Equal(t, "housekeeping", StreamHousekeeping)
}

func TestNewNetwork(t *testing.T) {
	// NewNetwork requires a glueberry.Node which we can't easily mock,
	// so we skip the actual construction test here.
	// The integration tests would cover this path.
	t.Skip("requires glueberry.Node mock")
}
