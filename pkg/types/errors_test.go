package types

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestErrorsAreDistinct(t *testing.T) {
	// Ensure all errors are distinct from each other
	allErrors := []error{
		// Peer errors
		ErrPeerNotFound,
		ErrPeerBlacklisted,
		ErrPeerAlreadyConnected,
		ErrMaxPeersReached,
		// Block errors
		ErrBlockNotFound,
		ErrBlockAlreadyExists,
		ErrInvalidBlockHeight,
		ErrInvalidBlockHash,
		// Transaction errors
		ErrTxNotFound,
		ErrTxAlreadyExists,
		ErrInvalidTx,
		ErrTxTooLarge,
		// Mempool errors
		ErrMempoolFull,
		ErrMempoolClosed,
		// Connection errors
		ErrChainIDMismatch,
		ErrVersionMismatch,
		ErrHandshakeFailed,
		ErrHandshakeTimeout,
		ErrConnectionClosed,
		ErrNotConnected,
		// Message errors
		ErrInvalidMessage,
		ErrUnknownMessageType,
		ErrMessageTooLarge,
		// State errors
		ErrKeyNotFound,
		ErrStoreClosed,
		ErrInvalidProof,
		// Sync errors
		ErrAlreadySyncing,
		ErrNotSyncing,
		ErrSyncFailed,
		// Node errors
		ErrNodeNotStarted,
		ErrNodeAlreadyStarted,
		ErrNodeStopped,
	}

	for i, err1 := range allErrors {
		for j, err2 := range allErrors {
			if i != j {
				require.NotEqual(t, err1, err2, "errors at index %d and %d should be distinct", i, j)
				require.False(t, errors.Is(err1, err2), "errors.Is(%v, %v) should be false", err1, err2)
			}
		}
	}
}

func TestErrorsIsCompatible(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		target error
	}{
		{"ErrPeerNotFound", ErrPeerNotFound, ErrPeerNotFound},
		{"ErrBlockNotFound", ErrBlockNotFound, ErrBlockNotFound},
		{"ErrTxNotFound", ErrTxNotFound, ErrTxNotFound},
		{"ErrMempoolFull", ErrMempoolFull, ErrMempoolFull},
		{"ErrChainIDMismatch", ErrChainIDMismatch, ErrChainIDMismatch},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.True(t, errors.Is(tt.err, tt.target))
		})
	}
}

func TestWrappedErrorsIs(t *testing.T) {
	// Test that wrapped errors work with errors.Is
	wrapped := fmt.Errorf("context: %w", ErrPeerNotFound)
	require.True(t, errors.Is(wrapped, ErrPeerNotFound))
	require.False(t, errors.Is(wrapped, ErrBlockNotFound))

	// Double wrapped
	doubleWrapped := fmt.Errorf("more context: %w", wrapped)
	require.True(t, errors.Is(doubleWrapped, ErrPeerNotFound))
}

func TestErrorMessages(t *testing.T) {
	// Ensure error messages are meaningful
	tests := []struct {
		err      error
		contains string
	}{
		{ErrPeerNotFound, "peer"},
		{ErrBlockNotFound, "block"},
		{ErrTxNotFound, "transaction"},
		{ErrMempoolFull, "mempool"},
		{ErrChainIDMismatch, "chain"},
		{ErrVersionMismatch, "version"},
		{ErrHandshakeFailed, "handshake"},
		{ErrInvalidMessage, "message"},
		{ErrKeyNotFound, "key"},
		{ErrStoreClosed, "store"},
		{ErrNodeNotStarted, "node"},
	}

	for _, tt := range tests {
		t.Run(tt.err.Error(), func(t *testing.T) {
			require.Contains(t, tt.err.Error(), tt.contains)
		})
	}
}
