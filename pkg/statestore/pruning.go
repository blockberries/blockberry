package statestore

import (
	"errors"
	"sync"
	"time"
)

// State pruning errors.
var (
	ErrStatePruningDisabled   = errors.New("state pruning is disabled")
	ErrStatePruningInProgress = errors.New("state pruning already in progress")
	ErrInvalidPruneVersion    = errors.New("invalid prune version")
	ErrStatePrunerNotActive   = errors.New("state pruner not running")
)

// StatePruneConfig defines configuration for state pruning.
type StatePruneConfig struct {
	// KeepRecent is the number of recent versions to always keep.
	KeepRecent int64 `toml:"keep_recent"`

	// KeepEvery specifies the interval for keeping checkpoint versions.
	// For example, KeepEvery=1000 means versions 1000, 2000, etc. are kept.
	// Set to 0 to disable checkpoint keeping.
	KeepEvery int64 `toml:"keep_every"`

	// Interval is how often to run automatic state pruning.
	// Set to 0 to disable automatic pruning.
	Interval time.Duration `toml:"interval"`

	// Enabled indicates whether state pruning is enabled.
	Enabled bool `toml:"enabled"`
}

// DefaultStatePruneConfig returns sensible defaults for state pruning.
func DefaultStatePruneConfig() *StatePruneConfig {
	return &StatePruneConfig{
		KeepRecent: 100,  // Keep last 100 versions
		KeepEvery:  1000, // Keep every 1000th version
		Interval:   time.Hour,
		Enabled:    true,
	}
}

// ArchiveStatePruneConfig returns config for archive nodes (no pruning).
func ArchiveStatePruneConfig() *StatePruneConfig {
	return &StatePruneConfig{
		Enabled: false,
	}
}

// Validate checks if the state prune configuration is valid.
func (c *StatePruneConfig) Validate() error {
	if c == nil {
		return nil
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

// ShouldKeep returns true if the version should be kept.
func (c *StatePruneConfig) ShouldKeep(version, currentVersion int64) bool {
	if c == nil || !c.Enabled {
		return true
	}

	// Always keep recent versions
	if c.KeepRecent > 0 && version > currentVersion-c.KeepRecent {
		return true
	}

	// Keep checkpoint versions
	if c.KeepEvery > 0 && version%c.KeepEvery == 0 {
		return true
	}

	return false
}

// CalculatePruneTarget returns the version below which versions can be pruned.
func (c *StatePruneConfig) CalculatePruneTarget(currentVersion int64) int64 {
	if c == nil || !c.Enabled {
		return 0
	}

	target := currentVersion - c.KeepRecent
	if target < 1 {
		return 0
	}
	return target
}

// StatePruneResult contains information about a state pruning operation.
type StatePruneResult struct {
	// PrunedCount is the number of versions pruned.
	PrunedCount int64

	// OldestVersion is the oldest version after pruning.
	OldestVersion int64

	// NewestVersion is the newest version (unchanged).
	NewestVersion int64

	// Duration is how long the pruning took.
	Duration time.Duration
}

// PrunableStateStore extends StateStore with pruning capabilities.
type PrunableStateStore interface {
	StateStore

	// PruneVersions removes old versions of the state.
	// Versions that should be kept according to the config are preserved.
	PruneVersions(beforeVersion int64) (*StatePruneResult, error)

	// AvailableVersions returns the range of available versions.
	AvailableVersions() (oldest, newest int64)

	// StatePruneConfig returns the current state pruning configuration.
	StatePruneConfig() *StatePruneConfig

	// SetStatePruneConfig updates the state pruning configuration.
	SetStatePruneConfig(cfg *StatePruneConfig)
}

// BackgroundStatePruner runs automatic state pruning in the background.
type BackgroundStatePruner struct {
	store    PrunableStateStore
	cfg      *StatePruneConfig
	stopCh   chan struct{}
	doneCh   chan struct{}
	running  bool
	mu       sync.Mutex

	// Callbacks
	onPrune func(*StatePruneResult)
	onError func(error)
}

// NewBackgroundStatePruner creates a new background state pruner.
func NewBackgroundStatePruner(store PrunableStateStore, cfg *StatePruneConfig) *BackgroundStatePruner {
	return &BackgroundStatePruner{
		store:  store,
		cfg:    cfg,
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}
}

// SetOnPrune sets a callback invoked after successful pruning.
func (p *BackgroundStatePruner) SetOnPrune(fn func(*StatePruneResult)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onPrune = fn
}

// SetOnError sets a callback invoked when pruning fails.
func (p *BackgroundStatePruner) SetOnError(fn func(error)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.onError = fn
}

// Start begins the background pruning loop.
func (p *BackgroundStatePruner) Start() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return errors.New("state pruner already running")
	}

	if p.cfg == nil || !p.cfg.Enabled || p.cfg.Interval <= 0 {
		return ErrStatePruningDisabled
	}

	p.running = true
	p.stopCh = make(chan struct{})
	p.doneCh = make(chan struct{})

	go p.pruneLoop()

	return nil
}

// Stop stops the background pruning loop.
func (p *BackgroundStatePruner) Stop() error {
	p.mu.Lock()
	if !p.running {
		p.mu.Unlock()
		return ErrStatePrunerNotActive
	}
	p.running = false
	close(p.stopCh)
	p.mu.Unlock()

	<-p.doneCh
	return nil
}

// IsRunning returns true if the pruner is running.
func (p *BackgroundStatePruner) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.running
}

// PruneNow triggers an immediate prune operation.
func (p *BackgroundStatePruner) PruneNow() (*StatePruneResult, error) {
	return p.doPrune()
}

func (p *BackgroundStatePruner) pruneLoop() {
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

func (p *BackgroundStatePruner) doPrune() (*StatePruneResult, error) {
	_, currentVersion := p.store.AvailableVersions()
	if currentVersion == 0 {
		return &StatePruneResult{}, nil
	}

	target := p.cfg.CalculatePruneTarget(currentVersion)
	if target <= 0 {
		return &StatePruneResult{}, nil
	}

	return p.store.PruneVersions(target)
}
