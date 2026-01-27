package abi

// QueryRequest represents a request to read application state.
type QueryRequest struct {
	// Path is the query path (e.g., "/accounts/{address}", "/store/key").
	Path string

	// Data contains query-specific data (e.g., serialized key).
	Data []byte

	// Height specifies the historical height to query. 0 means latest.
	Height uint64

	// Prove requests a Merkle proof be included in the response.
	Prove bool
}

// QueryResponse represents the response from a state query.
type QueryResponse struct {
	// Code indicates success (0) or failure (non-zero).
	Code ResultCode

	// Error provides a human-readable error message if Code != 0.
	Error error

	// Key is the key that was queried.
	Key []byte

	// Value is the value at the queried key.
	Value []byte

	// Proof is the Merkle proof (if requested and available).
	Proof *Proof

	// Height is the height at which the query was executed.
	Height uint64

	// Index is the index of the key in the tree (for iteration).
	Index int64
}

// IsOK returns true if the query succeeded.
func (r *QueryResponse) IsOK() bool {
	return r != nil && r.Code.IsOK()
}

// Exists returns true if the key exists (value is not nil).
func (r *QueryResponse) Exists() bool {
	return r != nil && r.Code.IsOK() && r.Value != nil
}

// Proof represents a Merkle proof for verifying state.
type Proof struct {
	// Ops are the proof operations that can be verified.
	Ops []ProofOp
}

// Verify verifies the proof against the given root hash, key, and value.
// This is a stub - real implementation would perform cryptographic verification.
func (p *Proof) Verify(rootHash, key, value []byte) bool {
	// Stub implementation
	return len(p.Ops) > 0
}

// ProofOp represents a single operation in a Merkle proof.
type ProofOp struct {
	// Type identifies the proof operation type (e.g., "iavl:v", "simple:v").
	Type string

	// Key is the key this operation applies to.
	Key []byte

	// Data contains the proof data for this operation.
	Data []byte
}
