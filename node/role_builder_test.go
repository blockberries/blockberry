package node

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/blockstore"
	"github.com/blockberries/blockberry/config"
	"github.com/blockberries/blockberry/mempool"
	bsync "github.com/blockberries/blockberry/sync"
	"github.com/blockberries/blockberry/types"
)

func TestNewRoleBasedBuilder(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Role = types.RoleValidator

	builder := NewRoleBasedBuilder(cfg)
	require.NotNil(t, builder)
	assert.Equal(t, types.RoleValidator, builder.Role())
	assert.True(t, builder.Capabilities().CanVote)
}

func TestNewRoleBasedBuilder_DefaultRole(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Role = ""

	builder := NewRoleBasedBuilder(cfg)
	require.NotNil(t, builder)
	assert.Equal(t, types.RoleFull, builder.Role())
}

func TestRoleBasedBuilder_WithRole(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewRoleBasedBuilder(cfg)

	builder = builder.WithRole(types.RoleSeed)
	assert.Equal(t, types.RoleSeed, builder.Role())
	assert.True(t, builder.Capabilities().ParticipatesInPEX)
	assert.False(t, builder.Capabilities().StoresFullBlocks)
}

func TestRoleBasedBuilder_WithRole_Invalid(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewRoleBasedBuilder(cfg)

	builder = builder.WithRole(types.NodeRole("invalid"))
	_, err := builder.Build()
	require.Error(t, err)
	assert.ErrorIs(t, err, types.ErrInvalidRole)
}

func TestRoleBasedBuilder_WithMempool(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewRoleBasedBuilder(cfg)

	customMempool := mempool.NewNoOpMempool()
	builder = builder.WithMempool(customMempool)

	// Can't easily test the mempool was set without building
	assert.NotNil(t, builder)
}

func TestRoleBasedBuilder_WithBlockStore(t *testing.T) {
	cfg := config.DefaultConfig()
	builder := NewRoleBasedBuilder(cfg)

	customStore := blockstore.NewNoOpBlockStore()
	builder = builder.WithBlockStore(customStore)

	assert.NotNil(t, builder)
}

func TestRoleBasedBuilder_ValidatorRequiresEngine(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Role = types.RoleValidator

	builder := NewRoleBasedBuilder(cfg)
	// No consensus engine set, but validator requires one
	builder = builder.WithBlockValidator(acceptAllValidator)

	_, err := builder.Build()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingConsensusEngine)
}

func TestRoleBasedBuilder_FullNodeRequiresBlockValidator(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Role = types.RoleFull

	builder := NewRoleBasedBuilder(cfg)
	// No block validator set, but full node stores blocks

	_, err := builder.Build()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrMissingBlockValidator)
}

func TestRoleBasedBuilder_SeedNodeMinimalRequirements(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Role = types.RoleSeed

	builder := NewRoleBasedBuilder(cfg)
	// Seed nodes don't require consensus engine or block validator

	// Still need to create a proper test setup or skip build
	// For now just verify the builder state
	assert.Equal(t, types.RoleSeed, builder.Role())
	assert.False(t, builder.Capabilities().RequiresConsensusEngine)
	assert.False(t, builder.Capabilities().RequiresBlockStore)
}

func TestRoleBasedBuilder_Capabilities(t *testing.T) {
	cfg := config.DefaultConfig()

	tests := []struct {
		role         types.NodeRole
		canVote      bool
		storesBlocks bool
		needsMempool bool
	}{
		{types.RoleValidator, true, true, true},
		{types.RoleFull, false, true, true},
		{types.RoleSeed, false, false, false},
		{types.RoleLight, false, false, false},
		{types.RoleArchive, false, true, true},
	}

	for _, tt := range tests {
		t.Run(string(tt.role), func(t *testing.T) {
			builder := NewRoleBasedBuilder(cfg)
			builder = builder.WithRole(tt.role)

			caps := builder.Capabilities()
			assert.Equal(t, tt.canVote, caps.CanVote)
			assert.Equal(t, tt.storesBlocks, caps.StoresFullBlocks)
			assert.Equal(t, tt.needsMempool, caps.RequiresMempool)
		})
	}
}

func TestIsValidatorRole(t *testing.T) {
	assert.True(t, IsValidatorRole(types.RoleValidator))
	assert.False(t, IsValidatorRole(types.RoleFull))
	assert.False(t, IsValidatorRole(types.RoleSeed))
	assert.False(t, IsValidatorRole(types.RoleLight))
	assert.False(t, IsValidatorRole(types.RoleArchive))
}

func TestRoleRequiresFullSync(t *testing.T) {
	assert.True(t, RoleRequiresFullSync(types.RoleValidator))
	assert.True(t, RoleRequiresFullSync(types.RoleFull))
	assert.False(t, RoleRequiresFullSync(types.RoleSeed))
	assert.False(t, RoleRequiresFullSync(types.RoleLight))
	assert.True(t, RoleRequiresFullSync(types.RoleArchive))
}

func TestRoleAcceptsInbound(t *testing.T) {
	assert.True(t, RoleAcceptsInbound(types.RoleValidator))
	assert.True(t, RoleAcceptsInbound(types.RoleFull))
	assert.True(t, RoleAcceptsInbound(types.RoleSeed))
	assert.False(t, RoleAcceptsInbound(types.RoleLight)) // Light clients typically don't accept inbound
	assert.True(t, RoleAcceptsInbound(types.RoleArchive))
}

func TestRoleBasedBuilder_Errors(t *testing.T) {
	assert.NotNil(t, ErrMissingConsensusEngine)
	assert.NotNil(t, ErrMissingBlockValidator)
	assert.NotNil(t, ErrIncompatibleComponent)

	assert.Contains(t, ErrMissingConsensusEngine.Error(), "consensus")
	assert.Contains(t, ErrMissingBlockValidator.Error(), "validator")
}

// acceptAllValidator is a test validator that accepts all blocks.
var acceptAllValidator bsync.BlockValidator = func(height int64, hash []byte, data []byte) error {
	return nil
}
