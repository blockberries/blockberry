package abi

import "errors"

// Common errors used by ABI interfaces.
var (
	// ErrNotImplemented indicates a method is not implemented.
	ErrNotImplemented = errors.New("not implemented")

	// ErrSnapshotNotFound indicates a requested snapshot was not found.
	ErrSnapshotNotFound = errors.New("snapshot not found")

	// ErrChunkNotFound indicates a requested snapshot chunk was not found.
	ErrChunkNotFound = errors.New("chunk not found")

	// ErrInvalidSnapshot indicates a snapshot is invalid or corrupted.
	ErrInvalidSnapshot = errors.New("invalid snapshot")

	// ErrInvalidChunk indicates a snapshot chunk is invalid or corrupted.
	ErrInvalidChunk = errors.New("invalid chunk")

	// ErrStateSyncInProgress indicates state sync is already in progress.
	ErrStateSyncInProgress = errors.New("state sync in progress")

	// ErrProposalRejected indicates a block proposal was rejected.
	ErrProposalRejected = errors.New("proposal rejected")

	// ErrVoteExtensionInvalid indicates a vote extension is invalid.
	ErrVoteExtensionInvalid = errors.New("invalid vote extension")
)
