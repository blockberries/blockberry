package metrics

import (
	"time"
)

// Metrics defines the interface for collecting node metrics.
// All methods are designed to be thread-safe and non-blocking.
type Metrics interface {
	// Peer metrics
	SetPeersTotal(direction string, count int)
	IncPeerConnections(result string)
	IncPeerDisconnections(reason string)

	// Block metrics
	SetBlockHeight(height int64)
	IncBlocksReceived()
	IncBlocksProposed()
	ObserveBlockLatency(latency time.Duration)
	SetBlockSize(size int)

	// Transaction metrics
	SetMempoolSize(size int)
	SetMempoolBytes(bytes int64)
	IncTxsReceived()
	IncTxsProposed()
	IncTxsRejected(reason string)
	IncTxsEvicted(reason string)

	// Sync metrics
	SetSyncState(state string)
	SetSyncProgress(progress float64)
	SetSyncPeerHeight(height int64)
	IncSyncBlocksReceived(count int)
	ObserveSyncDuration(duration time.Duration)

	// Message metrics
	IncMessagesReceived(stream string)
	IncMessagesSent(stream string)
	IncMessageErrors(stream, errorType string)
	ObserveMessageSize(stream string, size int)

	// Latency metrics
	SetPeerLatency(peerID string, latency time.Duration)
	ObservePeerRTT(latency time.Duration)

	// State store metrics
	SetStateStoreVersion(version int64)
	SetStateStoreSize(size int64)
	IncStateStoreGets()
	IncStateStoreSets()
	IncStateStoreDeletes()
	ObserveStateStoreLatency(op string, latency time.Duration)

	// HTTP handler (for serving metrics)
	Handler() any
}

// Direction labels for peer connections.
const (
	DirectionInbound  = "inbound"
	DirectionOutbound = "outbound"
)

// Connection result labels.
const (
	ResultSuccess = "success"
	ResultFailure = "failure"
)

// Sync state labels.
const (
	SyncStateSynced  = "synced"
	SyncStateSyncing = "syncing"
)

// Disconnection reason labels.
const (
	ReasonPenalized    = "penalized"
	ReasonTimeout      = "timeout"
	ReasonProtocol     = "protocol"
	ReasonRemote       = "remote"
	ReasonShutdown     = "shutdown"
	ReasonLimitReached = "limit_reached"
)

// Transaction rejection reason labels.
const (
	ReasonMempoolFull   = "mempool_full"
	ReasonTxTooLarge    = "tx_too_large"
	ReasonTxInvalid     = "tx_invalid"
	ReasonTxDuplicate   = "tx_duplicate"
	ReasonTxExpired     = "tx_expired"
	ReasonTxLowPriority = "tx_low_priority"
)

// Transaction eviction reason labels.
const (
	EvictionTTLExpired  = "ttl_expired"
	EvictionLowPriority = "low_priority"
	EvictionMempoolFull = "mempool_full"
)

// Message error type labels.
const (
	ErrorUnmarshal = "unmarshal"
	ErrorValidate  = "validate"
	ErrorTimeout   = "timeout"
	ErrorUnknown   = "unknown"
)

// Stream name labels.
const (
	StreamHandshake    = "handshake"
	StreamPEX          = "pex"
	StreamTransactions = "transactions"
	StreamBlockSync    = "blocksync"
	StreamBlocks       = "blocks"
	StreamConsensus    = "consensus"
	StreamHousekeeping = "housekeeping"
)

// State store operation labels.
const (
	OpGet    = "get"
	OpSet    = "set"
	OpDelete = "delete"
	OpProof  = "proof"
	OpCommit = "commit"
)
