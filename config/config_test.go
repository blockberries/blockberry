package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/blockberries/blockberry/types"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	require.NotNil(t, cfg)

	// Node defaults
	require.Equal(t, "blockberry-testnet-1", cfg.Node.ChainID)
	require.Equal(t, int32(1), cfg.Node.ProtocolVersion)
	require.Equal(t, "node_key.json", cfg.Node.PrivateKeyPath)

	// Network defaults
	require.Equal(t, []string{"/ip4/0.0.0.0/tcp/26656"}, cfg.Network.ListenAddrs)
	require.Equal(t, 40, cfg.Network.MaxInboundPeers)
	require.Equal(t, 10, cfg.Network.MaxOutboundPeers)
	require.Equal(t, 30*time.Second, cfg.Network.HandshakeTimeout.Duration())
	require.Equal(t, 3*time.Second, cfg.Network.DialTimeout.Duration())
	require.Equal(t, "addrbook.json", cfg.Network.AddressBookPath)

	// PEX defaults
	require.True(t, cfg.PEX.Enabled)
	require.Equal(t, 30*time.Second, cfg.PEX.RequestInterval.Duration())
	require.Equal(t, 100, cfg.PEX.MaxAddressesPerResponse)
	require.Equal(t, 1000, cfg.PEX.MaxTotalAddresses)

	// Mempool defaults
	require.Equal(t, 5000, cfg.Mempool.MaxTxs)
	require.Equal(t, int64(1073741824), cfg.Mempool.MaxBytes)
	require.Equal(t, 10000, cfg.Mempool.CacheSize)

	// BlockStore defaults
	require.Equal(t, "leveldb", cfg.BlockStore.Backend)
	require.Equal(t, "data/blockstore", cfg.BlockStore.Path)

	// StateStore defaults
	require.Equal(t, "data/state", cfg.StateStore.Path)
	require.Equal(t, 10000, cfg.StateStore.CacheSize)

	// Housekeeping defaults
	require.Equal(t, 60*time.Second, cfg.Housekeeping.LatencyProbeInterval.Duration())

	// Role defaults
	require.Equal(t, RoleFull, cfg.Role)

	// Handlers defaults
	require.Equal(t, 5*time.Second, cfg.Handlers.Transactions.RequestInterval.Duration())
	require.Equal(t, int32(100), cfg.Handlers.Transactions.BatchSize)
	require.Equal(t, 1000, cfg.Handlers.Transactions.MaxPending)
	require.Equal(t, 60*time.Second, cfg.Handlers.Transactions.MaxPendingAge.Duration())
	require.Equal(t, int64(22020096), cfg.Handlers.Blocks.MaxBlockSize)
	require.Equal(t, 5*time.Second, cfg.Handlers.Sync.SyncInterval.Duration())
	require.Equal(t, int32(100), cfg.Handlers.Sync.BatchSize)
	require.Equal(t, 10, cfg.Handlers.Sync.MaxPendingBatches)
}

func TestDefaultConfigValidates(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	require.NoError(t, err)
}

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[node]
chain_id = "my-test-chain"
protocol_version = 2
private_key_path = "keys/node.json"

[network]
listen_addrs = ["/ip4/0.0.0.0/tcp/9000", "/ip4/0.0.0.0/tcp/9001"]
max_inbound_peers = 50
max_outbound_peers = 20
handshake_timeout = "60s"
dial_timeout = "5s"
address_book_path = "data/addrbook.json"

[network.seeds]
addrs = ["/ip4/192.168.1.1/tcp/26656/p2p/12D3KooWTest"]

[pex]
enabled = true
request_interval = "45s"
max_addresses_per_response = 50

[mempool]
max_txs = 10000
max_bytes = 2147483648
cache_size = 20000

[blockstore]
backend = "badgerdb"
path = "data/blocks"

[statestore]
path = "data/state"
cache_size = 50000

[housekeeping]
latency_probe_interval = "120s"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Verify loaded values
	require.Equal(t, "my-test-chain", cfg.Node.ChainID)
	require.Equal(t, int32(2), cfg.Node.ProtocolVersion)
	require.Equal(t, "keys/node.json", cfg.Node.PrivateKeyPath)

	require.Equal(t, []string{"/ip4/0.0.0.0/tcp/9000", "/ip4/0.0.0.0/tcp/9001"}, cfg.Network.ListenAddrs)
	require.Equal(t, 50, cfg.Network.MaxInboundPeers)
	require.Equal(t, 20, cfg.Network.MaxOutboundPeers)
	require.Equal(t, 60*time.Second, cfg.Network.HandshakeTimeout.Duration())
	require.Equal(t, 5*time.Second, cfg.Network.DialTimeout.Duration())
	require.Equal(t, "data/addrbook.json", cfg.Network.AddressBookPath)
	require.Equal(t, []string{"/ip4/192.168.1.1/tcp/26656/p2p/12D3KooWTest"}, cfg.Network.Seeds.Addrs)

	require.True(t, cfg.PEX.Enabled)
	require.Equal(t, 45*time.Second, cfg.PEX.RequestInterval.Duration())
	require.Equal(t, 50, cfg.PEX.MaxAddressesPerResponse)

	require.Equal(t, 10000, cfg.Mempool.MaxTxs)
	require.Equal(t, int64(2147483648), cfg.Mempool.MaxBytes)
	require.Equal(t, 20000, cfg.Mempool.CacheSize)

	require.Equal(t, "badgerdb", cfg.BlockStore.Backend)
	require.Equal(t, "data/blocks", cfg.BlockStore.Path)

	require.Equal(t, "data/state", cfg.StateStore.Path)
	require.Equal(t, 50000, cfg.StateStore.CacheSize)

	require.Equal(t, 120*time.Second, cfg.Housekeeping.LatencyProbeInterval.Duration())
}

func TestLoadConfigPartial(t *testing.T) {
	// Test that missing values get defaults
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	configContent := `
[node]
chain_id = "partial-chain"
protocol_version = 1
private_key_path = "node.key"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	cfg, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Custom values
	require.Equal(t, "partial-chain", cfg.Node.ChainID)

	// Default values should be applied
	require.Equal(t, 40, cfg.Network.MaxInboundPeers)
	require.Equal(t, 5000, cfg.Mempool.MaxTxs)
	require.Equal(t, "leveldb", cfg.BlockStore.Backend)
}

func TestLoadConfigFileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.toml")
	require.Error(t, err)
}

func TestLoadConfigInvalidTOML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	err := os.WriteFile(configPath, []byte("invalid toml {{{{"), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	require.Error(t, err)
}

func TestLoadConfigValidationError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.toml")

	// Empty chain_id should fail validation
	configContent := `
[node]
chain_id = ""
protocol_version = 1
private_key_path = "node.key"
`

	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	_, err = LoadConfig(configPath)
	require.Error(t, err)
	require.ErrorIs(t, err, ErrEmptyChainID)
}

func TestNodeConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     NodeConfig
		wantErr error
	}{
		{
			name: "valid config",
			cfg: NodeConfig{
				ChainID:         "test-chain",
				ProtocolVersion: 1,
				PrivateKeyPath:  "node.key",
			},
			wantErr: nil,
		},
		{
			name: "empty chain_id",
			cfg: NodeConfig{
				ChainID:         "",
				ProtocolVersion: 1,
				PrivateKeyPath:  "node.key",
			},
			wantErr: ErrEmptyChainID,
		},
		{
			name: "zero protocol_version",
			cfg: NodeConfig{
				ChainID:         "test-chain",
				ProtocolVersion: 0,
				PrivateKeyPath:  "node.key",
			},
			wantErr: ErrInvalidProtocolVersion,
		},
		{
			name: "negative protocol_version",
			cfg: NodeConfig{
				ChainID:         "test-chain",
				ProtocolVersion: -1,
				PrivateKeyPath:  "node.key",
			},
			wantErr: ErrInvalidProtocolVersion,
		},
		{
			name: "empty private_key_path",
			cfg: NodeConfig{
				ChainID:         "test-chain",
				ProtocolVersion: 1,
				PrivateKeyPath:  "",
			},
			wantErr: ErrEmptyPrivateKeyPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestNetworkConfigValidation(t *testing.T) {
	validConfig := func() NetworkConfig {
		return NetworkConfig{
			ListenAddrs:      []string{"/ip4/0.0.0.0/tcp/26656"},
			MaxInboundPeers:  40,
			MaxOutboundPeers: 10,
			HandshakeTimeout: Duration(30 * time.Second),
			DialTimeout:      Duration(3 * time.Second),
			AddressBookPath:  "addrbook.json",
		}
	}

	tests := []struct {
		name    string
		modify  func(*NetworkConfig)
		wantErr error
	}{
		{
			name:    "valid config",
			modify:  func(c *NetworkConfig) {},
			wantErr: nil,
		},
		{
			name:    "no listen addrs",
			modify:  func(c *NetworkConfig) { c.ListenAddrs = []string{} },
			wantErr: ErrNoListenAddrs,
		},
		{
			name:    "negative max_inbound_peers",
			modify:  func(c *NetworkConfig) { c.MaxInboundPeers = -1 },
			wantErr: ErrInvalidMaxInboundPeers,
		},
		{
			name:    "negative max_outbound_peers",
			modify:  func(c *NetworkConfig) { c.MaxOutboundPeers = -1 },
			wantErr: ErrInvalidMaxOutboundPeers,
		},
		{
			name:    "zero handshake_timeout",
			modify:  func(c *NetworkConfig) { c.HandshakeTimeout = Duration(0) },
			wantErr: ErrInvalidHandshakeTimeout,
		},
		{
			name:    "zero dial_timeout",
			modify:  func(c *NetworkConfig) { c.DialTimeout = Duration(0) },
			wantErr: ErrInvalidDialTimeout,
		},
		{
			name:    "empty address_book_path",
			modify:  func(c *NetworkConfig) { c.AddressBookPath = "" },
			wantErr: ErrEmptyAddressBookPath,
		},
		{
			name:    "zero peers allowed",
			modify:  func(c *NetworkConfig) { c.MaxInboundPeers = 0; c.MaxOutboundPeers = 0 },
			wantErr: nil, // Zero peers is valid (node might be isolated)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.modify(&cfg)
			err := cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestPEXConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     PEXConfig
		wantErr error
	}{
		{
			name: "valid enabled config",
			cfg: PEXConfig{
				Enabled:                 true,
				RequestInterval:         Duration(30 * time.Second),
				MaxAddressesPerResponse: 100,
				MaxTotalAddresses:       1000,
			},
			wantErr: nil,
		},
		{
			name: "disabled config - no validation needed",
			cfg: PEXConfig{
				Enabled:                 false,
				RequestInterval:         Duration(0),
				MaxAddressesPerResponse: 0,
				MaxTotalAddresses:       0,
			},
			wantErr: nil,
		},
		{
			name: "enabled with zero interval",
			cfg: PEXConfig{
				Enabled:                 true,
				RequestInterval:         Duration(0),
				MaxAddressesPerResponse: 100,
				MaxTotalAddresses:       1000,
			},
			wantErr: ErrInvalidRequestInterval,
		},
		{
			name: "enabled with zero max addresses per response",
			cfg: PEXConfig{
				Enabled:                 true,
				RequestInterval:         Duration(30 * time.Second),
				MaxAddressesPerResponse: 0,
				MaxTotalAddresses:       1000,
			},
			wantErr: ErrInvalidMaxAddresses,
		},
		{
			name: "enabled with zero max total addresses",
			cfg: PEXConfig{
				Enabled:                 true,
				RequestInterval:         Duration(30 * time.Second),
				MaxAddressesPerResponse: 100,
				MaxTotalAddresses:       0,
			},
			wantErr: ErrInvalidMaxTotalAddresses,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMempoolConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     MempoolConfig
		wantErr error
	}{
		{
			name: "valid config",
			cfg: MempoolConfig{
				MaxTxs:    5000,
				MaxBytes:  1073741824,
				CacheSize: 10000,
			},
			wantErr: nil,
		},
		{
			name: "zero max_txs",
			cfg: MempoolConfig{
				MaxTxs:    0,
				MaxBytes:  1073741824,
				CacheSize: 10000,
			},
			wantErr: ErrInvalidMaxTxs,
		},
		{
			name: "zero max_bytes",
			cfg: MempoolConfig{
				MaxTxs:    5000,
				MaxBytes:  0,
				CacheSize: 10000,
			},
			wantErr: ErrInvalidMaxBytes,
		},
		{
			name: "negative cache_size",
			cfg: MempoolConfig{
				MaxTxs:    5000,
				MaxBytes:  1073741824,
				CacheSize: -1,
			},
			wantErr: ErrInvalidMempoolCacheSize,
		},
		{
			name: "zero cache_size is valid",
			cfg: MempoolConfig{
				MaxTxs:    5000,
				MaxBytes:  1073741824,
				CacheSize: 0,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestBlockStoreConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     BlockStoreConfig
		wantErr error
	}{
		{
			name: "valid leveldb config",
			cfg: BlockStoreConfig{
				Backend: "leveldb",
				Path:    "data/blockstore",
			},
			wantErr: nil,
		},
		{
			name: "valid badgerdb config",
			cfg: BlockStoreConfig{
				Backend: "badgerdb",
				Path:    "data/blockstore",
			},
			wantErr: nil,
		},
		{
			name: "invalid backend",
			cfg: BlockStoreConfig{
				Backend: "mysql",
				Path:    "data/blockstore",
			},
			wantErr: ErrInvalidBlockStoreBackend,
		},
		{
			name: "empty path",
			cfg: BlockStoreConfig{
				Backend: "leveldb",
				Path:    "",
			},
			wantErr: ErrEmptyBlockStorePath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestStateStoreConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     StateStoreConfig
		wantErr error
	}{
		{
			name: "valid config",
			cfg: StateStoreConfig{
				Path:      "data/state",
				CacheSize: 10000,
			},
			wantErr: nil,
		},
		{
			name: "empty path",
			cfg: StateStoreConfig{
				Path:      "",
				CacheSize: 10000,
			},
			wantErr: ErrEmptyStateStorePath,
		},
		{
			name: "negative cache_size",
			cfg: StateStoreConfig{
				Path:      "data/state",
				CacheSize: -1,
			},
			wantErr: ErrInvalidStateCacheSize,
		},
		{
			name: "zero cache_size is valid",
			cfg: StateStoreConfig{
				Path:      "data/state",
				CacheSize: 0,
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestHousekeepingConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     HousekeepingConfig
		wantErr error
	}{
		{
			name: "valid config",
			cfg: HousekeepingConfig{
				LatencyProbeInterval: Duration(60 * time.Second),
			},
			wantErr: nil,
		},
		{
			name: "zero interval",
			cfg: HousekeepingConfig{
				LatencyProbeInterval: Duration(0),
			},
			wantErr: ErrInvalidLatencyInterval,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestWriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.toml")

	cfg := DefaultConfig()
	cfg.Node.ChainID = "written-chain"
	cfg.Network.MaxInboundPeers = 100

	err := WriteConfigFile(configPath, cfg)
	require.NoError(t, err)

	// Verify file was created
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load and verify
	loadedCfg, err := LoadConfig(configPath)
	require.NoError(t, err)
	require.Equal(t, "written-chain", loadedCfg.Node.ChainID)
	require.Equal(t, 100, loadedCfg.Network.MaxInboundPeers)
}

func TestDuration(t *testing.T) {
	t.Run("unmarshal valid duration", func(t *testing.T) {
		var d Duration
		err := d.UnmarshalText([]byte("30s"))
		require.NoError(t, err)
		require.Equal(t, 30*time.Second, d.Duration())
	})

	t.Run("unmarshal complex duration", func(t *testing.T) {
		var d Duration
		err := d.UnmarshalText([]byte("1h30m45s"))
		require.NoError(t, err)
		require.Equal(t, time.Hour+30*time.Minute+45*time.Second, d.Duration())
	})

	t.Run("unmarshal invalid duration", func(t *testing.T) {
		var d Duration
		err := d.UnmarshalText([]byte("invalid"))
		require.Error(t, err)
	})

	t.Run("marshal duration", func(t *testing.T) {
		d := Duration(2*time.Hour + 30*time.Minute)
		text, err := d.MarshalText()
		require.NoError(t, err)
		require.Equal(t, "2h30m0s", string(text))
	})
}

func TestEnsureDataDirs(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := DefaultConfig()
	cfg.Node.PrivateKeyPath = filepath.Join(tmpDir, "keys", "node.key")
	cfg.Network.AddressBookPath = filepath.Join(tmpDir, "data", "addrbook.json")
	cfg.BlockStore.Path = filepath.Join(tmpDir, "data", "blockstore")
	cfg.StateStore.Path = filepath.Join(tmpDir, "data", "state")

	err := cfg.EnsureDataDirs()
	require.NoError(t, err)

	// Verify directories were created
	_, err = os.Stat(filepath.Join(tmpDir, "keys"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "data"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "data", "blockstore"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(tmpDir, "data", "state"))
	require.NoError(t, err)
}

func TestNodeRole(t *testing.T) {
	t.Run("valid roles", func(t *testing.T) {
		require.True(t, RoleValidator.IsValid())
		require.True(t, RoleFull.IsValid())
		require.True(t, RoleSeed.IsValid())
		require.True(t, RoleLight.IsValid())
	})

	t.Run("invalid role", func(t *testing.T) {
		require.False(t, NodeRole("invalid").IsValid())
		require.False(t, NodeRole("").IsValid())
	})

	t.Run("role validation in config", func(t *testing.T) {
		cfg := DefaultConfig()

		// Valid roles
		for _, role := range types.AllRoles() {
			cfg.Role = role
			err := cfg.Validate()
			require.NoError(t, err, "role %s should be valid", role)
		}

		// Invalid role
		cfg.Role = "invalid"
		err := cfg.Validate()
		require.ErrorIs(t, err, ErrInvalidNodeRole)
	})
}

func TestHandlersConfigValidation(t *testing.T) {
	t.Run("valid config", func(t *testing.T) {
		cfg := HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(5 * time.Second),
				BatchSize:       100,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{
				MaxBlockSize: 1024 * 1024,
			},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(5 * time.Second),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		}
		require.NoError(t, cfg.Validate())
	})

	t.Run("invalid transactions request_interval", func(t *testing.T) {
		cfg := HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(0),
				BatchSize:       100,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{MaxBlockSize: 1024},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(5 * time.Second),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		}
		err := cfg.Validate()
		require.ErrorIs(t, err, ErrInvalidTxRequestInterval)
	})

	t.Run("invalid transactions batch_size", func(t *testing.T) {
		cfg := HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(5 * time.Second),
				BatchSize:       0,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{MaxBlockSize: 1024},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(5 * time.Second),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		}
		err := cfg.Validate()
		require.ErrorIs(t, err, ErrInvalidTxBatchSize)
	})

	t.Run("invalid blocks max_block_size", func(t *testing.T) {
		cfg := HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(5 * time.Second),
				BatchSize:       100,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{MaxBlockSize: 0},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(5 * time.Second),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		}
		err := cfg.Validate()
		require.ErrorIs(t, err, ErrInvalidBlocksMaxSize)
	})

	t.Run("invalid sync sync_interval", func(t *testing.T) {
		cfg := HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(5 * time.Second),
				BatchSize:       100,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{MaxBlockSize: 1024},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(0),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		}
		err := cfg.Validate()
		require.ErrorIs(t, err, ErrInvalidSyncInterval)
	})
}
