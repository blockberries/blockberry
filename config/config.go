package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

// NodeRole defines the role of a node in the network.
type NodeRole string

// Node role constants.
const (
	// RoleValidator is a node that participates in consensus.
	RoleValidator NodeRole = "validator"

	// RoleFull is a full node that stores all data but doesn't participate in consensus.
	RoleFull NodeRole = "full"

	// RoleSeed is a seed node that helps peers discover each other.
	RoleSeed NodeRole = "seed"

	// RoleLight is a light client that only stores block headers.
	RoleLight NodeRole = "light"
)

// ValidRoles contains all valid node roles.
var ValidRoles = []NodeRole{RoleValidator, RoleFull, RoleSeed, RoleLight}

// IsValid returns true if the role is valid.
func (r NodeRole) IsValid() bool {
	for _, valid := range ValidRoles {
		if r == valid {
			return true
		}
	}
	return false
}

// Config is the main configuration for a blockberry node.
type Config struct {
	Node         NodeConfig         `toml:"node"`
	Network      NetworkConfig      `toml:"network"`
	Role         NodeRole           `toml:"role"`
	Handlers     HandlersConfig     `toml:"handlers"`
	PEX          PEXConfig          `toml:"pex"`
	Mempool      MempoolConfig      `toml:"mempool"`
	BlockStore   BlockStoreConfig   `toml:"blockstore"`
	StateStore   StateStoreConfig   `toml:"statestore"`
	Housekeeping HousekeepingConfig `toml:"housekeeping"`
	Metrics      MetricsConfig      `toml:"metrics"`
	Logging      LoggingConfig      `toml:"logging"`
}

// NodeConfig contains node identity and chain configuration.
type NodeConfig struct {
	// ChainID is the unique identifier for the blockchain network.
	ChainID string `toml:"chain_id"`

	// ProtocolVersion is the protocol version supported by this node.
	ProtocolVersion int32 `toml:"protocol_version"`

	// PrivateKeyPath is the path to the node's Ed25519 private key file.
	PrivateKeyPath string `toml:"private_key_path"`
}

// NetworkConfig contains P2P networking configuration.
type NetworkConfig struct {
	// ListenAddrs are the multiaddrs to listen on for incoming connections.
	ListenAddrs []string `toml:"listen_addrs"`

	// MaxInboundPeers is the maximum number of inbound peer connections.
	MaxInboundPeers int `toml:"max_inbound_peers"`

	// MaxOutboundPeers is the maximum number of outbound peer connections.
	MaxOutboundPeers int `toml:"max_outbound_peers"`

	// HandshakeTimeout is the maximum time allowed to complete a handshake.
	HandshakeTimeout Duration `toml:"handshake_timeout"`

	// DialTimeout is the maximum time allowed for dialing a peer.
	DialTimeout Duration `toml:"dial_timeout"`

	// AddressBookPath is the path to persist the address book.
	AddressBookPath string `toml:"address_book_path"`

	// Seeds contains seed node configuration.
	Seeds SeedsConfig `toml:"seeds"`
}

// SeedsConfig contains seed node configuration.
type SeedsConfig struct {
	// Addrs are the multiaddrs of seed nodes for bootstrap.
	Addrs []string `toml:"addrs"`
}

// PEXConfig contains peer exchange configuration.
type PEXConfig struct {
	// Enabled determines whether peer exchange is active.
	Enabled bool `toml:"enabled"`

	// RequestInterval is the time between peer exchange requests.
	RequestInterval Duration `toml:"request_interval"`

	// MaxAddressesPerResponse is the maximum addresses to return in a PEX response.
	MaxAddressesPerResponse int `toml:"max_addresses_per_response"`
}

// MempoolConfig contains transaction mempool configuration.
type MempoolConfig struct {
	// MaxTxs is the maximum number of transactions in the mempool.
	MaxTxs int `toml:"max_txs"`

	// MaxBytes is the maximum total size of transactions in the mempool.
	MaxBytes int64 `toml:"max_bytes"`

	// CacheSize is the size of the recent transaction hash cache.
	CacheSize int `toml:"cache_size"`
}

// BlockStoreConfig contains block storage configuration.
type BlockStoreConfig struct {
	// Backend is the storage backend to use ("leveldb" or "badgerdb").
	Backend string `toml:"backend"`

	// Path is the directory path for block storage.
	Path string `toml:"path"`
}

// StateStoreConfig contains state storage configuration.
type StateStoreConfig struct {
	// Path is the directory path for state storage.
	Path string `toml:"path"`

	// CacheSize is the IAVL node cache size.
	CacheSize int `toml:"cache_size"`
}

// HousekeepingConfig contains housekeeping configuration.
type HousekeepingConfig struct {
	// LatencyProbeInterval is the time between latency probe messages.
	LatencyProbeInterval Duration `toml:"latency_probe_interval"`
}

// HandlersConfig contains configuration for message handlers.
type HandlersConfig struct {
	// Transactions contains transaction handler configuration.
	Transactions TransactionsHandlerConfig `toml:"transactions"`

	// Blocks contains block handler configuration.
	Blocks BlocksHandlerConfig `toml:"blocks"`

	// Sync contains block sync handler configuration.
	Sync SyncHandlerConfig `toml:"sync"`
}

// TransactionsHandlerConfig contains transaction handler configuration.
type TransactionsHandlerConfig struct {
	// RequestInterval is the time between transaction gossip requests.
	RequestInterval Duration `toml:"request_interval"`

	// BatchSize is the maximum number of transactions to request at once.
	BatchSize int32 `toml:"batch_size"`

	// MaxPending is the maximum number of pending transaction requests per peer.
	MaxPending int `toml:"max_pending"`

	// MaxPendingAge is the maximum age of a pending request before cleanup.
	MaxPendingAge Duration `toml:"max_pending_age"`
}

// BlocksHandlerConfig contains block handler configuration.
type BlocksHandlerConfig struct {
	// MaxBlockSize is the maximum allowed block size in bytes.
	MaxBlockSize int64 `toml:"max_block_size"`
}

// SyncHandlerConfig contains block sync handler configuration.
type SyncHandlerConfig struct {
	// SyncInterval is the time between sync status checks.
	SyncInterval Duration `toml:"sync_interval"`

	// BatchSize is the number of blocks to request per batch during sync.
	BatchSize int32 `toml:"batch_size"`

	// MaxPendingBatches is the maximum number of pending block batches.
	MaxPendingBatches int `toml:"max_pending_batches"`
}

// MetricsConfig contains metrics configuration.
type MetricsConfig struct {
	// Enabled determines whether metrics collection is active.
	Enabled bool `toml:"enabled"`

	// Namespace is the Prometheus metrics namespace prefix.
	Namespace string `toml:"namespace"`

	// ListenAddr is the address to serve metrics on (e.g., ":9090").
	ListenAddr string `toml:"listen_addr"`
}

// LoggingConfig contains logging configuration.
type LoggingConfig struct {
	// Level is the minimum log level ("debug", "info", "warn", "error").
	Level string `toml:"level"`

	// Format is the log output format ("text" or "json").
	Format string `toml:"format"`

	// Output is the log output destination ("stdout", "stderr", or a file path).
	Output string `toml:"output"`
}

// Duration is a wrapper around time.Duration for TOML unmarshaling.
type Duration time.Duration

// UnmarshalText implements encoding.TextUnmarshaler for Duration.
func (d *Duration) UnmarshalText(text []byte) error {
	duration, err := time.ParseDuration(string(text))
	if err != nil {
		return err
	}
	*d = Duration(duration)
	return nil
}

// MarshalText implements encoding.TextMarshaler for Duration.
func (d Duration) MarshalText() ([]byte, error) {
	return []byte(time.Duration(d).String()), nil
}

// Duration returns the underlying time.Duration.
func (d Duration) Duration() time.Duration {
	return time.Duration(d)
}

// DefaultConfig returns a Config with sensible default values.
func DefaultConfig() *Config {
	return &Config{
		Node: NodeConfig{
			ChainID:         "blockberry-testnet-1",
			ProtocolVersion: 1,
			PrivateKeyPath:  "node_key.json",
		},
		Network: NetworkConfig{
			ListenAddrs:      []string{"/ip4/0.0.0.0/tcp/26656"},
			MaxInboundPeers:  40,
			MaxOutboundPeers: 10,
			HandshakeTimeout: Duration(30 * time.Second),
			DialTimeout:      Duration(3 * time.Second),
			AddressBookPath:  "addrbook.json",
			Seeds: SeedsConfig{
				Addrs: []string{},
			},
		},
		Role: RoleFull, // Default to full node
		Handlers: HandlersConfig{
			Transactions: TransactionsHandlerConfig{
				RequestInterval: Duration(5 * time.Second),
				BatchSize:       100,
				MaxPending:      1000,
				MaxPendingAge:   Duration(60 * time.Second),
			},
			Blocks: BlocksHandlerConfig{
				MaxBlockSize: 22020096, // ~21MB
			},
			Sync: SyncHandlerConfig{
				SyncInterval:      Duration(5 * time.Second),
				BatchSize:         100,
				MaxPendingBatches: 10,
			},
		},
		PEX: PEXConfig{
			Enabled:                 true,
			RequestInterval:         Duration(30 * time.Second),
			MaxAddressesPerResponse: 100,
		},
		Mempool: MempoolConfig{
			MaxTxs:    5000,
			MaxBytes:  1073741824, // 1GB
			CacheSize: 10000,
		},
		BlockStore: BlockStoreConfig{
			Backend: "leveldb",
			Path:    "data/blockstore",
		},
		StateStore: StateStoreConfig{
			Path:      "data/state",
			CacheSize: 10000,
		},
		Housekeeping: HousekeepingConfig{
			LatencyProbeInterval: Duration(60 * time.Second),
		},
		Metrics: MetricsConfig{
			Enabled:    false,
			Namespace:  "blockberry",
			ListenAddr: ":9090",
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			Output: "stderr",
		},
	}
}

// LoadConfig loads configuration from a TOML file.
// Missing values are filled with defaults.
func LoadConfig(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := toml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validating config: %w", err)
	}

	return cfg, nil
}

// Validation errors.
var (
	ErrEmptyChainID                 = errors.New("chain_id cannot be empty")
	ErrInvalidProtocolVersion       = errors.New("protocol_version must be positive")
	ErrEmptyPrivateKeyPath          = errors.New("private_key_path cannot be empty")
	ErrNoListenAddrs                = errors.New("at least one listen address is required")
	ErrInvalidMaxInboundPeers       = errors.New("max_inbound_peers must be non-negative")
	ErrInvalidMaxOutboundPeers      = errors.New("max_outbound_peers must be non-negative")
	ErrInvalidHandshakeTimeout      = errors.New("handshake_timeout must be positive")
	ErrInvalidDialTimeout           = errors.New("dial_timeout must be positive")
	ErrEmptyAddressBookPath         = errors.New("address_book_path cannot be empty")
	ErrInvalidNodeRole              = errors.New("role must be one of: validator, full, seed, light")
	ErrInvalidTxRequestInterval     = errors.New("transactions request_interval must be positive")
	ErrInvalidTxBatchSize           = errors.New("transactions batch_size must be positive")
	ErrInvalidTxMaxPending          = errors.New("transactions max_pending must be non-negative")
	ErrInvalidTxMaxPendingAge       = errors.New("transactions max_pending_age must be positive")
	ErrInvalidBlocksMaxSize         = errors.New("blocks max_block_size must be positive")
	ErrInvalidSyncInterval          = errors.New("sync sync_interval must be positive")
	ErrInvalidSyncBatchSize         = errors.New("sync batch_size must be positive")
	ErrInvalidSyncMaxPendingBatches = errors.New("sync max_pending_batches must be positive")
	ErrInvalidRequestInterval       = errors.New("request_interval must be positive when pex is enabled")
	ErrInvalidMaxAddresses          = errors.New("max_addresses_per_response must be positive when pex is enabled")
	ErrInvalidMaxTxs                = errors.New("max_txs must be positive")
	ErrInvalidMaxBytes              = errors.New("max_bytes must be positive")
	ErrInvalidMempoolCacheSize      = errors.New("mempool cache_size must be non-negative")
	ErrInvalidBlockStoreBackend     = errors.New("blockstore backend must be 'leveldb' or 'badgerdb'")
	ErrEmptyBlockStorePath          = errors.New("blockstore path cannot be empty")
	ErrEmptyStateStorePath          = errors.New("statestore path cannot be empty")
	ErrInvalidStateCacheSize        = errors.New("statestore cache_size must be non-negative")
	ErrInvalidLatencyInterval       = errors.New("latency_probe_interval must be positive")
	ErrEmptyMetricsNamespace        = errors.New("metrics namespace cannot be empty when enabled")
	ErrEmptyMetricsListenAddr       = errors.New("metrics listen_addr cannot be empty when enabled")
	ErrInvalidLogLevel              = errors.New("log level must be one of: debug, info, warn, error")
	ErrInvalidLogFormat             = errors.New("log format must be 'text' or 'json'")
	ErrEmptyLogOutput               = errors.New("log output cannot be empty")
)

// Validate checks the configuration for errors.
func (c *Config) Validate() error {
	if err := c.Node.Validate(); err != nil {
		return fmt.Errorf("node config: %w", err)
	}
	if err := c.Network.Validate(); err != nil {
		return fmt.Errorf("network config: %w", err)
	}
	if !c.Role.IsValid() {
		return fmt.Errorf("config: %w", ErrInvalidNodeRole)
	}
	if err := c.Handlers.Validate(); err != nil {
		return fmt.Errorf("handlers config: %w", err)
	}
	if err := c.PEX.Validate(); err != nil {
		return fmt.Errorf("pex config: %w", err)
	}
	if err := c.Mempool.Validate(); err != nil {
		return fmt.Errorf("mempool config: %w", err)
	}
	if err := c.BlockStore.Validate(); err != nil {
		return fmt.Errorf("blockstore config: %w", err)
	}
	if err := c.StateStore.Validate(); err != nil {
		return fmt.Errorf("statestore config: %w", err)
	}
	if err := c.Housekeeping.Validate(); err != nil {
		return fmt.Errorf("housekeeping config: %w", err)
	}
	if err := c.Metrics.Validate(); err != nil {
		return fmt.Errorf("metrics config: %w", err)
	}
	if err := c.Logging.Validate(); err != nil {
		return fmt.Errorf("logging config: %w", err)
	}
	return nil
}

// Validate checks the node configuration for errors.
func (c *NodeConfig) Validate() error {
	if c.ChainID == "" {
		return ErrEmptyChainID
	}
	if c.ProtocolVersion <= 0 {
		return ErrInvalidProtocolVersion
	}
	if c.PrivateKeyPath == "" {
		return ErrEmptyPrivateKeyPath
	}
	return nil
}

// Validate checks the network configuration for errors.
func (c *NetworkConfig) Validate() error {
	if len(c.ListenAddrs) == 0 {
		return ErrNoListenAddrs
	}
	if c.MaxInboundPeers < 0 {
		return ErrInvalidMaxInboundPeers
	}
	if c.MaxOutboundPeers < 0 {
		return ErrInvalidMaxOutboundPeers
	}
	if c.HandshakeTimeout.Duration() <= 0 {
		return ErrInvalidHandshakeTimeout
	}
	if c.DialTimeout.Duration() <= 0 {
		return ErrInvalidDialTimeout
	}
	if c.AddressBookPath == "" {
		return ErrEmptyAddressBookPath
	}
	return nil
}

// Validate checks the PEX configuration for errors.
func (c *PEXConfig) Validate() error {
	if c.Enabled {
		if c.RequestInterval.Duration() <= 0 {
			return ErrInvalidRequestInterval
		}
		if c.MaxAddressesPerResponse <= 0 {
			return ErrInvalidMaxAddresses
		}
	}
	return nil
}

// Validate checks the mempool configuration for errors.
func (c *MempoolConfig) Validate() error {
	if c.MaxTxs <= 0 {
		return ErrInvalidMaxTxs
	}
	if c.MaxBytes <= 0 {
		return ErrInvalidMaxBytes
	}
	if c.CacheSize < 0 {
		return ErrInvalidMempoolCacheSize
	}
	return nil
}

// Validate checks the block store configuration for errors.
func (c *BlockStoreConfig) Validate() error {
	if c.Backend != "leveldb" && c.Backend != "badgerdb" {
		return ErrInvalidBlockStoreBackend
	}
	if c.Path == "" {
		return ErrEmptyBlockStorePath
	}
	return nil
}

// Validate checks the state store configuration for errors.
func (c *StateStoreConfig) Validate() error {
	if c.Path == "" {
		return ErrEmptyStateStorePath
	}
	if c.CacheSize < 0 {
		return ErrInvalidStateCacheSize
	}
	return nil
}

// Validate checks the housekeeping configuration for errors.
func (c *HousekeepingConfig) Validate() error {
	if c.LatencyProbeInterval.Duration() <= 0 {
		return ErrInvalidLatencyInterval
	}
	return nil
}

// Validate checks the handlers configuration for errors.
func (c *HandlersConfig) Validate() error {
	if err := c.Transactions.Validate(); err != nil {
		return fmt.Errorf("transactions: %w", err)
	}
	if err := c.Blocks.Validate(); err != nil {
		return fmt.Errorf("blocks: %w", err)
	}
	if err := c.Sync.Validate(); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	return nil
}

// Validate checks the transactions handler configuration for errors.
func (c *TransactionsHandlerConfig) Validate() error {
	if c.RequestInterval.Duration() <= 0 {
		return ErrInvalidTxRequestInterval
	}
	if c.BatchSize <= 0 {
		return ErrInvalidTxBatchSize
	}
	if c.MaxPending < 0 {
		return ErrInvalidTxMaxPending
	}
	if c.MaxPendingAge.Duration() <= 0 {
		return ErrInvalidTxMaxPendingAge
	}
	return nil
}

// Validate checks the blocks handler configuration for errors.
func (c *BlocksHandlerConfig) Validate() error {
	if c.MaxBlockSize <= 0 {
		return ErrInvalidBlocksMaxSize
	}
	return nil
}

// Validate checks the sync handler configuration for errors.
func (c *SyncHandlerConfig) Validate() error {
	if c.SyncInterval.Duration() <= 0 {
		return ErrInvalidSyncInterval
	}
	if c.BatchSize <= 0 {
		return ErrInvalidSyncBatchSize
	}
	if c.MaxPendingBatches <= 0 {
		return ErrInvalidSyncMaxPendingBatches
	}
	return nil
}

// Validate checks the metrics configuration for errors.
func (c *MetricsConfig) Validate() error {
	if c.Enabled {
		if c.Namespace == "" {
			return ErrEmptyMetricsNamespace
		}
		if c.ListenAddr == "" {
			return ErrEmptyMetricsListenAddr
		}
	}
	return nil
}

// Validate checks the logging configuration for errors.
func (c *LoggingConfig) Validate() error {
	switch c.Level {
	case "debug", "info", "warn", "error":
		// Valid levels
	default:
		return ErrInvalidLogLevel
	}

	switch c.Format {
	case "text", "json":
		// Valid formats
	default:
		return ErrInvalidLogFormat
	}

	if c.Output == "" {
		return ErrEmptyLogOutput
	}

	return nil
}

// WriteConfigFile writes the configuration to a TOML file.
func WriteConfigFile(path string, cfg *Config) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating config file: %w", err)
	}
	defer f.Close()

	encoder := toml.NewEncoder(f)
	if err := encoder.Encode(cfg); err != nil {
		return fmt.Errorf("encoding config: %w", err)
	}

	return nil
}

// EnsureDataDirs creates the data directories specified in the configuration.
func (c *Config) EnsureDataDirs() error {
	dirs := []string{
		filepath.Dir(c.Node.PrivateKeyPath),
		filepath.Dir(c.Network.AddressBookPath),
		c.BlockStore.Path,
		c.StateStore.Path,
	}

	for _, dir := range dirs {
		if dir == "" || dir == "." {
			continue
		}
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}

	return nil
}
