package types

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNodeRole_String(t *testing.T) {
	tests := []struct {
		role     NodeRole
		expected string
	}{
		{RoleValidator, "validator"},
		{RoleFull, "full"},
		{RoleSeed, "seed"},
		{RoleLight, "light"},
		{RoleArchive, "archive"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, tt.role.String())
	}
}

func TestNodeRole_IsValid(t *testing.T) {
	tests := []struct {
		role    NodeRole
		isValid bool
	}{
		{RoleValidator, true},
		{RoleFull, true},
		{RoleSeed, true},
		{RoleLight, true},
		{RoleArchive, true},
		{NodeRole(""), false},
		{NodeRole("invalid"), false},
		{NodeRole("VALIDATOR"), false}, // Case sensitive
	}

	for _, tt := range tests {
		assert.Equal(t, tt.isValid, tt.role.IsValid(), "role=%q", tt.role)
	}
}

func TestNodeRole_Capabilities(t *testing.T) {
	// Test validator capabilities
	validatorCaps := RoleValidator.Capabilities()
	assert.True(t, validatorCaps.CanPropose)
	assert.True(t, validatorCaps.CanVote)
	assert.True(t, validatorCaps.StoresFullBlocks)
	assert.True(t, validatorCaps.StoresState)
	assert.True(t, validatorCaps.RequiresConsensusEngine)
	assert.True(t, validatorCaps.RequiresMempool)
	assert.False(t, validatorCaps.GossipsTransactions) // Uses DAG

	// Test full node capabilities
	fullCaps := RoleFull.Capabilities()
	assert.False(t, fullCaps.CanPropose)
	assert.False(t, fullCaps.CanVote)
	assert.True(t, fullCaps.StoresFullBlocks)
	assert.True(t, fullCaps.StoresState)
	assert.False(t, fullCaps.RequiresConsensusEngine)
	assert.True(t, fullCaps.GossipsTransactions)

	// Test seed node capabilities
	seedCaps := RoleSeed.Capabilities()
	assert.False(t, seedCaps.CanPropose)
	assert.False(t, seedCaps.CanVote)
	assert.False(t, seedCaps.StoresFullBlocks)
	assert.False(t, seedCaps.StoresState)
	assert.True(t, seedCaps.ParticipatesInPEX)
	assert.False(t, seedCaps.RequiresBlockStore)

	// Test light node capabilities
	lightCaps := RoleLight.Capabilities()
	assert.False(t, lightCaps.CanPropose)
	assert.False(t, lightCaps.StoresFullBlocks)
	assert.False(t, lightCaps.StoresState)
	assert.False(t, lightCaps.AcceptsInboundConnections)
	assert.True(t, lightCaps.RequiresBlockStore) // For headers

	// Test archive node capabilities
	archiveCaps := RoleArchive.Capabilities()
	assert.False(t, archiveCaps.CanPropose)
	assert.True(t, archiveCaps.StoresFullBlocks)
	assert.True(t, archiveCaps.StoresState)
	assert.False(t, archiveCaps.Prunes) // Never prunes
}

func TestNodeRole_Capabilities_InvalidRole(t *testing.T) {
	caps := NodeRole("invalid").Capabilities()
	// Should return empty capabilities
	assert.False(t, caps.CanPropose)
	assert.False(t, caps.CanVote)
	assert.False(t, caps.StoresFullBlocks)
}

func TestParseRole(t *testing.T) {
	tests := []struct {
		input    string
		expected NodeRole
		hasError bool
	}{
		{"validator", RoleValidator, false},
		{"full", RoleFull, false},
		{"seed", RoleSeed, false},
		{"light", RoleLight, false},
		{"archive", RoleArchive, false},
		{"", "", true},
		{"invalid", "", true},
		{"VALIDATOR", "", true}, // Case sensitive
	}

	for _, tt := range tests {
		role, err := ParseRole(tt.input)
		if tt.hasError {
			require.Error(t, err, "input=%q", tt.input)
			assert.ErrorIs(t, err, ErrInvalidRole)
		} else {
			require.NoError(t, err, "input=%q", tt.input)
			assert.Equal(t, tt.expected, role)
		}
	}
}

func TestValidateRole(t *testing.T) {
	// Valid roles
	for _, role := range AllRoles() {
		err := ValidateRole(role)
		require.NoError(t, err, "role=%q", role)
	}

	// Invalid roles
	err := ValidateRole(NodeRole("invalid"))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRole)

	err = ValidateRole(NodeRole(""))
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidRole)
}

func TestAllRoles(t *testing.T) {
	roles := AllRoles()
	require.Len(t, roles, 5)

	// Verify all roles are valid
	for _, role := range roles {
		assert.True(t, role.IsValid())
	}

	// Verify specific roles exist
	assert.Contains(t, roles, RoleValidator)
	assert.Contains(t, roles, RoleFull)
	assert.Contains(t, roles, RoleSeed)
	assert.Contains(t, roles, RoleLight)
	assert.Contains(t, roles, RoleArchive)
}

func TestDefaultRole(t *testing.T) {
	role := DefaultRole()
	assert.Equal(t, RoleFull, role)
	assert.True(t, role.IsValid())
}

func TestRoleRequiresComponent(t *testing.T) {
	tests := []struct {
		role      NodeRole
		component string
		required  bool
	}{
		// Validator requires everything
		{RoleValidator, "consensus", true},
		{RoleValidator, "mempool", true},
		{RoleValidator, "blockstore", true},
		{RoleValidator, "statestore", true},

		// Full node doesn't require consensus
		{RoleFull, "consensus", false},
		{RoleFull, "mempool", true},
		{RoleFull, "blockstore", true},
		{RoleFull, "statestore", true},

		// Seed node requires minimal components
		{RoleSeed, "consensus", false},
		{RoleSeed, "mempool", false},
		{RoleSeed, "blockstore", false},
		{RoleSeed, "statestore", false},

		// Light node requires only blockstore (for headers)
		{RoleLight, "consensus", false},
		{RoleLight, "mempool", false},
		{RoleLight, "blockstore", true},
		{RoleLight, "statestore", false},

		// Archive requires most components (except consensus)
		{RoleArchive, "consensus", false},
		{RoleArchive, "mempool", true},
		{RoleArchive, "blockstore", true},
		{RoleArchive, "statestore", true},

		// Unknown component
		{RoleValidator, "unknown", false},
	}

	for _, tt := range tests {
		result := RoleRequiresComponent(tt.role, tt.component)
		assert.Equal(t, tt.required, result, "role=%q, component=%q", tt.role, tt.component)
	}
}

func TestRoleCapabilities_AllRolesHaveCapabilities(t *testing.T) {
	for _, role := range AllRoles() {
		caps := role.Capabilities()
		// At minimum, all roles should have some capability defined
		// This verifies roleCapabilities map is complete
		_, exists := roleCapabilities[role]
		assert.True(t, exists, "role %q missing from roleCapabilities", role)

		// All roles participate in PEX
		assert.True(t, caps.ParticipatesInPEX, "role %q should participate in PEX", role)
	}
}

func TestRoleCapabilities_ValidatorCanConsensus(t *testing.T) {
	caps := RoleValidator.Capabilities()
	assert.True(t, caps.CanPropose, "validators must be able to propose")
	assert.True(t, caps.CanVote, "validators must be able to vote")
}

func TestRoleCapabilities_NonValidatorsCantConsensus(t *testing.T) {
	nonValidators := []NodeRole{RoleFull, RoleSeed, RoleLight, RoleArchive}
	for _, role := range nonValidators {
		caps := role.Capabilities()
		assert.False(t, caps.CanPropose, "role %q should not propose", role)
		assert.False(t, caps.CanVote, "role %q should not vote", role)
	}
}

func TestRoleCapabilities_ArchiveNeverPrunes(t *testing.T) {
	caps := RoleArchive.Capabilities()
	assert.False(t, caps.Prunes, "archive nodes must never prune")
}

func TestRoleCapabilities_SeedMinimalStorage(t *testing.T) {
	caps := RoleSeed.Capabilities()
	assert.False(t, caps.StoresFullBlocks, "seed nodes should not store blocks")
	assert.False(t, caps.StoresState, "seed nodes should not store state")
	assert.False(t, caps.RequiresBlockStore, "seed nodes should not require block store")
	assert.False(t, caps.RequiresStateStore, "seed nodes should not require state store")
}

func TestRoleErrors(t *testing.T) {
	// Verify errors are properly defined
	assert.NotNil(t, ErrInvalidRole)
	assert.NotNil(t, ErrRoleCapabilityMismatch)
	assert.NotNil(t, ErrRoleChangeNotAllowed)

	// Verify error messages
	assert.Contains(t, ErrInvalidRole.Error(), "invalid")
	assert.Contains(t, ErrRoleCapabilityMismatch.Error(), "capability")
	assert.Contains(t, ErrRoleChangeNotAllowed.Error(), "change")
}
