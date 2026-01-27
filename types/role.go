package types

import (
	"errors"
	"fmt"
)

// NodeRole defines the role of a node in the network.
// The role determines which components are active and what capabilities the node has.
type NodeRole string

const (
	// RoleValidator is a consensus-participating validator node.
	// Validators propose blocks, vote in consensus, and maintain full state.
	RoleValidator NodeRole = "validator"

	// RoleFull is a full node that stores all data but doesn't participate in consensus.
	// Full nodes verify blocks, maintain state, and serve as reliable data sources.
	RoleFull NodeRole = "full"

	// RoleSeed is a seed node for peer discovery.
	// Seed nodes facilitate peer exchange but don't store blocks or state.
	RoleSeed NodeRole = "seed"

	// RoleLight is a light client that only stores headers.
	// Light clients verify headers and proofs without storing full blocks.
	RoleLight NodeRole = "light"

	// RoleArchive is a full node that never prunes historical data.
	// Archive nodes maintain complete history for queries and analysis.
	RoleArchive NodeRole = "archive"
)

// Role-related errors.
var (
	// ErrInvalidRole is returned when a role string is not recognized.
	ErrInvalidRole = errors.New("invalid node role")

	// ErrRoleCapabilityMismatch is returned when a node attempts an action
	// its role doesn't support.
	ErrRoleCapabilityMismatch = errors.New("role capability mismatch")

	// ErrRoleChangeNotAllowed is returned when a role change is not permitted.
	ErrRoleChangeNotAllowed = errors.New("role change not allowed")
)

// String returns the role as a string.
func (r NodeRole) String() string {
	return string(r)
}

// IsValid returns true if the role is a recognized value.
func (r NodeRole) IsValid() bool {
	switch r {
	case RoleValidator, RoleFull, RoleSeed, RoleLight, RoleArchive:
		return true
	default:
		return false
	}
}

// Capabilities returns the capabilities for this role.
func (r NodeRole) Capabilities() RoleCapabilities {
	caps, ok := roleCapabilities[r]
	if !ok {
		return RoleCapabilities{}
	}
	return caps
}

// RoleCapabilities defines what capabilities each role has.
// These determine which components are activated and what actions are permitted.
type RoleCapabilities struct {
	// CanPropose indicates if this role can propose blocks.
	CanPropose bool

	// CanVote indicates if this role can vote in consensus.
	CanVote bool

	// StoresFullBlocks indicates if this role stores full block data.
	StoresFullBlocks bool

	// StoresState indicates if this role stores application state.
	StoresState bool

	// ParticipatesInPEX indicates if this role exchanges peer addresses.
	ParticipatesInPEX bool

	// GossipsTransactions indicates if this role gossips transactions.
	// Validators may use DAG-based transaction propagation instead.
	GossipsTransactions bool

	// AcceptsInboundConnections indicates if this role accepts incoming connections.
	AcceptsInboundConnections bool

	// Prunes indicates if this role prunes old data.
	// Archive nodes never prune.
	Prunes bool

	// RequiresConsensusEngine indicates if this role needs a consensus engine.
	RequiresConsensusEngine bool

	// RequiresMempool indicates if this role needs a mempool.
	RequiresMempool bool

	// RequiresBlockStore indicates if this role needs a block store.
	RequiresBlockStore bool

	// RequiresStateStore indicates if this role needs a state store.
	RequiresStateStore bool
}

// roleCapabilities maps each role to its capabilities.
var roleCapabilities = map[NodeRole]RoleCapabilities{
	RoleValidator: {
		CanPropose:                true,
		CanVote:                   true,
		StoresFullBlocks:          true,
		StoresState:               true,
		ParticipatesInPEX:         true,
		GossipsTransactions:       false, // Validators may use DAG
		AcceptsInboundConnections: true,
		Prunes:                    true,
		RequiresConsensusEngine:   true,
		RequiresMempool:           true,
		RequiresBlockStore:        true,
		RequiresStateStore:        true,
	},
	RoleFull: {
		CanPropose:                false,
		CanVote:                   false,
		StoresFullBlocks:          true,
		StoresState:               true,
		ParticipatesInPEX:         true,
		GossipsTransactions:       true,
		AcceptsInboundConnections: true,
		Prunes:                    true,
		RequiresConsensusEngine:   false, // Uses NullConsensus
		RequiresMempool:           true,
		RequiresBlockStore:        true,
		RequiresStateStore:        true,
	},
	RoleSeed: {
		CanPropose:                false,
		CanVote:                   false,
		StoresFullBlocks:          false,
		StoresState:               false,
		ParticipatesInPEX:         true, // Primary purpose
		GossipsTransactions:       false,
		AcceptsInboundConnections: true,
		Prunes:                    false, // Nothing to prune
		RequiresConsensusEngine:   false,
		RequiresMempool:           false,
		RequiresBlockStore:        false,
		RequiresStateStore:        false,
	},
	RoleLight: {
		CanPropose:                false,
		CanVote:                   false,
		StoresFullBlocks:          false, // Only headers
		StoresState:               false, // Verifies proofs
		ParticipatesInPEX:         true,
		GossipsTransactions:       false,
		AcceptsInboundConnections: false, // Typically outbound only
		Prunes:                    true,
		RequiresConsensusEngine:   false,
		RequiresMempool:           false,
		RequiresBlockStore:        true, // For headers
		RequiresStateStore:        false,
	},
	RoleArchive: {
		CanPropose:                false,
		CanVote:                   false,
		StoresFullBlocks:          true,
		StoresState:               true,
		ParticipatesInPEX:         true,
		GossipsTransactions:       true,
		AcceptsInboundConnections: true,
		Prunes:                    false, // Never prune
		RequiresConsensusEngine:   false,
		RequiresMempool:           true,
		RequiresBlockStore:        true,
		RequiresStateStore:        true,
	},
}

// AllRoles returns all valid node roles.
func AllRoles() []NodeRole {
	return []NodeRole{
		RoleValidator,
		RoleFull,
		RoleSeed,
		RoleLight,
		RoleArchive,
	}
}

// ParseRole parses a string into a NodeRole.
// Returns an error if the string is not a valid role.
func ParseRole(s string) (NodeRole, error) {
	role := NodeRole(s)
	if !role.IsValid() {
		return "", fmt.Errorf("%w: %q", ErrInvalidRole, s)
	}
	return role, nil
}

// ValidateRole validates a role configuration.
// Returns an error if the role is invalid.
func ValidateRole(role NodeRole) error {
	if !role.IsValid() {
		return fmt.Errorf("%w: %q", ErrInvalidRole, role)
	}
	return nil
}

// RoleRequiresComponent returns true if the given role requires the specified component.
func RoleRequiresComponent(role NodeRole, component string) bool {
	caps := role.Capabilities()
	switch component {
	case "consensus":
		return caps.RequiresConsensusEngine
	case "mempool":
		return caps.RequiresMempool
	case "blockstore":
		return caps.RequiresBlockStore
	case "statestore":
		return caps.RequiresStateStore
	default:
		return false
	}
}

// DefaultRole returns the default node role (full node).
func DefaultRole() NodeRole {
	return RoleFull
}
