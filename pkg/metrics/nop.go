package metrics

import (
	"time"
)

// NopMetrics is a no-op implementation of the Metrics interface.
// Use this when metrics collection is disabled.
type NopMetrics struct{}

// NewNopMetrics creates a new NopMetrics instance.
func NewNopMetrics() *NopMetrics {
	return &NopMetrics{}
}

// Peer metrics (no-op)

func (m *NopMetrics) SetPeersTotal(direction string, count int) {}
func (m *NopMetrics) IncPeerConnections(result string)          {}
func (m *NopMetrics) IncPeerDisconnections(reason string)       {}

// Block metrics (no-op)

func (m *NopMetrics) SetBlockHeight(height int64)               {}
func (m *NopMetrics) IncBlocksReceived()                        {}
func (m *NopMetrics) IncBlocksProposed()                        {}
func (m *NopMetrics) ObserveBlockLatency(latency time.Duration) {}
func (m *NopMetrics) SetBlockSize(size int)                     {}

// Transaction metrics (no-op)

func (m *NopMetrics) SetMempoolSize(size int)      {}
func (m *NopMetrics) SetMempoolBytes(bytes int64)  {}
func (m *NopMetrics) IncTxsReceived()              {}
func (m *NopMetrics) IncTxsProposed()              {}
func (m *NopMetrics) IncTxsRejected(reason string) {}
func (m *NopMetrics) IncTxsEvicted(reason string)  {}

// Sync metrics (no-op)

func (m *NopMetrics) SetSyncState(state string)                  {}
func (m *NopMetrics) SetSyncProgress(progress float64)           {}
func (m *NopMetrics) SetSyncPeerHeight(height int64)             {}
func (m *NopMetrics) IncSyncBlocksReceived(count int)            {}
func (m *NopMetrics) ObserveSyncDuration(duration time.Duration) {}

// Message metrics (no-op)

func (m *NopMetrics) IncMessagesReceived(stream string)          {}
func (m *NopMetrics) IncMessagesSent(stream string)              {}
func (m *NopMetrics) IncMessageErrors(stream, errorType string)  {}
func (m *NopMetrics) ObserveMessageSize(stream string, size int) {}

// Latency metrics (no-op)

func (m *NopMetrics) SetPeerLatency(peerID string, latency time.Duration) {}
func (m *NopMetrics) ObservePeerRTT(latency time.Duration)                {}

// State store metrics (no-op)

func (m *NopMetrics) SetStateStoreVersion(version int64)                        {}
func (m *NopMetrics) SetStateStoreSize(size int64)                              {}
func (m *NopMetrics) IncStateStoreGets()                                        {}
func (m *NopMetrics) IncStateStoreSets()                                        {}
func (m *NopMetrics) IncStateStoreDeletes()                                     {}
func (m *NopMetrics) ObserveStateStoreLatency(op string, latency time.Duration) {}

// Handler returns nil since there's nothing to serve.
func (m *NopMetrics) Handler() any {
	return nil
}
