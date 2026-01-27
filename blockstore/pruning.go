package blockstore

import (
	"errors"
	"sync"
	"time"
)

// Pruning errors.
var (
	ErrPruningDisabled     = errors.New("pruning is disabled")
	ErrPruningInProgress   = errors.New("pruning already in progress")
	ErrInvalidPruneHeight  = errors.New("invalid prune height")
	ErrPruneHeightTooHigh  = errors.New("prune height exceeds allowed range")
	ErrPrunerAlreadyActive = errors.New("pruner already running")
	ErrPrunerNotActive     = errors.New("pruner not running")
)

// PruneStrategy defines how blocks are pruned.
type PruneStrategy string

// Prune strategy constants.
const (
	// PruneNothing keeps all blocks (archive node).
	PruneNothing PruneStrategy = "nothing"

	// PruneEverything aggressively prunes blocks, keeping only recent.
	PruneEverything PruneStrategy = "everything"

	// PruneDefault keeps recent blocks plus periodic checkpoints.
	PruneDefault PruneStrategy = "default"

	// PruneCustom uses custom configuration.
	PruneCustom PruneStrategy = "custom"
)

// DefaultPruneConfig returns sensible defaults for pruning.
func DefaultPruneConfig() *PruneConfig {
	return &PruneConfig{
		Strategy:   PruneDefault,
		KeepRecent: 1000,   // Keep last 1000 blocks
		KeepEvery:  10000,  // Keep every 10000th block as checkpoint
		Interval:   time.Hour,
	}
}

// ArchivePruneConfig returns config for archive nodes (no pruning).
func ArchivePruneConfig() *PruneConfig {
	return &PruneConfig{
		Strategy: PruneNothing,
	}
}

// AggressivePruneConfig returns config for aggressive pruning.
func AggressivePruneConfig() *PruneConfig {
	return &PruneConfig{
		Strategy:   PruneEverything,
		KeepRecent: 100, // Keep only last 100 blocks
		Interval:   10 * time.Minute,
	}
}

// PruneConfig defines how and when to prune blocks.
type PruneConfig struct {
	// Strategy is the pruning strategy to use.
	Strategy PruneStrategy `toml:"strategy"`

	// KeepRecent is the number of recent blocks to always keep.
	// These blocks will never be pruned regardless of strategy.
	KeepRecent int64 `toml:"keep_recent"`

	// KeepEvery specifies the interval for keeping checkpoint blocks.
	// For example, KeepEvery=10000 means blocks at heights 10000, 20000, etc. are kept.
	// Set to 0 to disable checkpoint keeping.
	// Only used with PruneDefault strategy.
	KeepEvery int64 `toml:"keep_every"`

	// Interval is how often to run automatic pruning.
	// Set to 0 to disable automatic pruning.
	Interval time.Duration `toml:"interval"`
}

// ShouldKeep returns true if the block at the given height should be kept.
func (c *PruneConfig) ShouldKeep(height, currentHeight int64) bool {
	if c == nil || c.Strategy == PruneNothing {
		return true
	}

	// Always keep recent blocks
	if c.KeepRecent > 0 && height > currentHeight-c.KeepRecent {
		return true
	}

	// For default strategy, keep checkpoint blocks
	if c.Strategy == PruneDefault && c.KeepEvery > 0 {
		if height%c.KeepEvery == 0 {
			return true
		}
	}

	return false
}

// CalculatePruneTarget returns the height below which blocks can be pruned.
// Returns 0 if no blocks should be pruned.
func (c *PruneConfig) CalculatePruneTarget(currentHeight int64) int64 {
	if c == nil || c.Strategy == PruneNothing {
		return 0
	}

	target := currentHeight - c.KeepRecent
	if target < 1 {
		return 0
	}
	return target
}

// Validate checks if the pruning configuration is valid.
func (c *PruneConfig) Validate() error {
	if c == nil {
		return nil
	}

	switch c.Strategy {
	case PruneNothing, PruneEverything, PruneDefault, PruneCustom:
		// Valid strategies
	default:
		return errors.New("invalid prune strategy")
	}

	if c.KeepRecent < 0 {
		return errors.New("keep_recent cannot be negative")
	}

	if c.KeepEvery < 0 {
		return errors.New("keep_every cannot be negative")
	}

	if c.Interval < 0 {
		return errors.New("interval cannot be negative")
	}

	return nil
}

// PruneResult contains information about a pruning operation.
type PruneResult struct {
	// PrunedCount is the number of blocks pruned.
	PrunedCount int64

	// NewBase is the new base height after pruning.
	NewBase int64

	// BytesFreed is the approximate bytes freed (if available).
	BytesFreed int64

	// Duration is how long the pruning took.
	Duration time.Duration
}

// PrunableBlockStore extends BlockStore with pruning capabilities.
type PrunableBlockStore interface {
	BlockStore

	// Prune removes blocks before the given height.
	// Blocks at heights that should be kept according to the config are preserved.
	// Returns information about what was pruned.
	Prune(beforeHeight int64) (*PruneResult, error)

	// PruneConfig returns the current pruning configuration.
	// Returns nil if pruning is not configured.
	PruneConfig() *PruneConfig

	// SetPruneConfig updates the pruning configuration.
	SetPruneConfig(cfg *PruneConfig)
}

// BackgroundPruner runs automatic pruning in the background.
type BackgroundPruner struct {
	store    PrunableBlockStore
	cfg      *PruneConfig
	stopCh   chan struct{}
	doneCh   chan struct{}
	running  bool
	mu       sync.Mutex

	// Callbacks
	onPrune func(*PruneResult)
	onError func(error)
}

// NewBackgroundPruner creates a new background pruner.
func NewBackgroundPruner(store PrunableBlockStore, cfg *PruneConfig) *BackgroundPruner {
	return &BackgroundPruner{
		store:  store,
		cfg:    cfg,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// SetOnPrune sets a callback invoked after successful pruning.
func (p *BackgroundPruner) SetOnPrune(fn func(*PruneResult)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onPrune = fn
}

// SetOnError sets a callback invoked when pruning fails.
func (p *BackgroundPruner) SetOnError(fn func(error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onError = fn
}

// Start begins the background pruning loop.
func (p *BackgroundPruner) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return ErrPrunerAlreadyActive
	}

	if p.cfg == nil || p.cfg.Strategy == PruneNothing || p.cfg.Interval <= 0 {
		return ErrPruningDisabled
	}

	p.running = true
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})

	go p.pruneLoop()

	return nil
}

// Stop stops the background pruning loop.
func (p *BackgroundPruner) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return ErrPrunerNotActive
	}
	p.running = false
	close(p.stopCh)
	p.mu.Unlock()

	// Wait for loop to finish
	<-p.doneCh
	return nil
}

// IsRunning returns true if the pruner is running.
func (p *BackgroundPruner) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// PruneNow triggers an immediate prune operation.
func (p *BackgroundPruner) PruneNow() (*PruneResult, error) {
	return p.doPrune()
}

func (p *BackgroundPruner) pruneLoop() {
	defer close(p.doneCh)

	ticker := time.NewTicker(p.cfg.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-p.stopCh:
			return
		case <-ticker.C:
			result, err := p.doPrune()
			if err != nil {
				p.mu.Lock()
				onError := p.onError
				p.mu.Unlock()
				if onError != nil {
					onError(err)
				}
			} else if result.PrunedCount > 0 {
				p.mu.Lock()
				onPrune := p.onPrune
				p.mu.Unlock()
				if onPrune != nil {
					onPrune(result)
				}
			}
		}
	}
}

func (p *BackgroundPruner) doPrune() (*PruneResult, error) {
	currentHeight := p.store.Height()
	if currentHeight == 0 {
		return &PruneResult{}, nil
	}

	target := p.cfg.CalculatePruneTarget(currentHeight)
	if target <= 0 {
		return &PruneResult{}, nil
	}

	return p.store.Prune(target)
}
