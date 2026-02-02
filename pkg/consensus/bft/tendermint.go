// Package bft provides a reference Tendermint-style BFT consensus implementation.
//
// This is a skeleton implementation designed as a reference for building
// custom BFT consensus engines. It demonstrates the key concepts:
//
//   - Round-based consensus with propose, prevote, precommit phases
//   - Locking mechanism for safety (2f+1 prevotes)
//   - Commit at 2f+1 precommits
//   - Timeout-based round progression
//   - Write-ahead log for crash recovery
//
// To use this as a starting point for a custom implementation:
//
//  1. Copy this file to your project
//  2. Implement the TODO sections
//  3. Add application-specific block validation
//  4. Configure timeouts and validator management
//
//nolint:unused,staticcheck,govet,gosec // Skeleton implementation with intentional TODOs
package bft

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/blockberries/blockberry/pkg/consensus"
)

// RoundStep represents the current step in a consensus round.
type RoundStep int

// Round step constants following Tendermint's state machine.
const (
	// RoundStepNewHeight is the initial step when entering a new height.
	// The node waits for the start time of the new height.
	RoundStepNewHeight RoundStep = iota

	// RoundStepNewRound is entered when starting a new round.
	// If this node is the proposer, it will propose; otherwise wait for proposal.
	RoundStepNewRound

	// RoundStepPropose is when waiting for or processing the proposal.
	RoundStepPropose

	// RoundStepPrevote is when voting (prevoting) on the proposal.
	RoundStepPrevote

	// RoundStepPrevoteWait is when waiting for more prevotes after seeing 2f+1.
	RoundStepPrevoteWait

	// RoundStepPrecommit is when precommitting after seeing 2f+1 prevotes.
	RoundStepPrecommit

	// RoundStepPrecommitWait is when waiting for more precommits after seeing 2f+1.
	RoundStepPrecommitWait

	// RoundStepCommit is when the block is committed.
	RoundStepCommit
)

// String returns the string representation of a RoundStep.
func (rs RoundStep) String() string {
	switch rs {
	case RoundStepNewHeight:
		return "NewHeight"
	case RoundStepNewRound:
		return "NewRound"
	case RoundStepPropose:
		return "Propose"
	case RoundStepPrevote:
		return "Prevote"
	case RoundStepPrevoteWait:
		return "PrevoteWait"
	case RoundStepPrecommit:
		return "Precommit"
	case RoundStepPrecommitWait:
		return "PrecommitWait"
	case RoundStepCommit:
		return "Commit"
	default:
		return fmt.Sprintf("Unknown(%d)", rs)
	}
}

// BFTConfig contains configuration for the BFT consensus engine.
type BFTConfig struct {
	// TimeoutPropose is how long to wait for a proposal.
	TimeoutPropose time.Duration

	// TimeoutPrevote is how long to wait for prevotes after seeing 2f+1.
	TimeoutPrevote time.Duration

	// TimeoutPrecommit is how long to wait for precommits after seeing 2f+1.
	TimeoutPrecommit time.Duration

	// TimeoutCommit is how long to wait after committing before starting next height.
	TimeoutCommit time.Duration

	// SkipTimeoutCommit if true, starts next height immediately after commit.
	SkipTimeoutCommit bool

	// CreateEmptyBlocks determines if empty blocks should be created.
	CreateEmptyBlocks bool

	// CreateEmptyBlocksInterval is the interval between empty blocks when no txs.
	CreateEmptyBlocksInterval time.Duration
}

// DefaultBFTConfig returns the default BFT configuration.
func DefaultBFTConfig() *BFTConfig {
	return &BFTConfig{
		TimeoutPropose:            3000 * time.Millisecond,
		TimeoutPrevote:            1000 * time.Millisecond,
		TimeoutPrecommit:          1000 * time.Millisecond,
		TimeoutCommit:             1000 * time.Millisecond,
		SkipTimeoutCommit:         false,
		CreateEmptyBlocks:         true,
		CreateEmptyBlocksInterval: 0,
	}
}

// TendermintBFT is a reference Tendermint-style BFT implementation.
// This is a skeleton that demonstrates the structure and key concepts.
// Actual implementations should fill in the TODO sections.
type TendermintBFT struct {
	// Configuration
	cfg *BFTConfig

	// Consensus state
	height      int64
	round       int32
	step        RoundStep
	lockedValue *consensus.Block // Block we're locked on (received 2f+1 prevotes)
	lockedRound int32            // Round we locked in
	validValue  *consensus.Block // Most recently valid block
	validRound  int32            // Round of valid value

	// Current proposal for this round
	proposal *consensus.Proposal

	// Validator information
	validatorSet  consensus.ValidatorSet
	privValidator PrivValidator
	validatorIdx  int // Index in validator set, -1 if not a validator

	// Vote tracking
	prevotes   *HeightVoteSet
	precommits *HeightVoteSet

	// Dependencies
	deps consensus.ConsensusDependencies

	// Write-ahead log for crash recovery
	wal WAL

	// Timeouts
	timeoutTicker TimeoutTicker

	// Lifecycle
	running atomic.Bool
	stopCh  chan struct{}
	wg      sync.WaitGroup

	mu sync.RWMutex
}

// NewTendermintBFT creates a new Tendermint-style BFT consensus engine.
func NewTendermintBFT(cfg *BFTConfig) *TendermintBFT {
	if cfg == nil {
		cfg = DefaultBFTConfig()
	}

	return &TendermintBFT{
		cfg:          cfg,
		validatorIdx: -1,
	}
}

// Name returns the component name.
func (bft *TendermintBFT) Name() string {
	return "tendermint-bft"
}

// IsRunning returns true if the engine is running.
func (bft *TendermintBFT) IsRunning() bool {
	return bft.running.Load()
}

// Start starts the BFT consensus engine.
func (bft *TendermintBFT) Start() error {
	if bft.running.Swap(true) {
		return fmt.Errorf("tendermint-bft already running")
	}

	bft.stopCh = make(chan struct{})

	// TODO: Initialize state from last commit
	// TODO: Start WAL
	// TODO: Replay WAL if needed
	// TODO: Start timeout ticker

	bft.wg.Add(1)
	go bft.mainLoop()

	return nil
}

// Stop stops the BFT consensus engine.
func (bft *TendermintBFT) Stop() error {
	if !bft.running.Swap(false) {
		return nil
	}

	close(bft.stopCh)
	bft.wg.Wait()

	// TODO: Close WAL
	// TODO: Stop timeout ticker

	return nil
}

// Initialize sets up the engine with its dependencies.
func (bft *TendermintBFT) Initialize(deps consensus.ConsensusDependencies) error {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	bft.deps = deps

	// Copy timeouts from config if provided
	if deps.Config != nil {
		if deps.Config.TimeoutPropose > 0 {
			bft.cfg.TimeoutPropose = time.Duration(deps.Config.TimeoutPropose) * time.Millisecond
		}
		if deps.Config.TimeoutPrevote > 0 {
			bft.cfg.TimeoutPrevote = time.Duration(deps.Config.TimeoutPrevote) * time.Millisecond
		}
		if deps.Config.TimeoutPrecommit > 0 {
			bft.cfg.TimeoutPrecommit = time.Duration(deps.Config.TimeoutPrecommit) * time.Millisecond
		}
		if deps.Config.TimeoutCommit > 0 {
			bft.cfg.TimeoutCommit = time.Duration(deps.Config.TimeoutCommit) * time.Millisecond
		}
	}

	return nil
}

// ProcessBlock handles a block received from a peer.
func (bft *TendermintBFT) ProcessBlock(block *consensus.Block) error {
	if block == nil {
		return fmt.Errorf("nil block")
	}

	bft.mu.Lock()
	defer bft.mu.Unlock()

	// TODO: Validate block
	// TODO: Check if this block is for the current height
	// TODO: If ahead, trigger fast sync

	return nil
}

// GetHeight returns the current consensus height.
func (bft *TendermintBFT) GetHeight() int64 {
	bft.mu.RLock()
	defer bft.mu.RUnlock()
	return bft.height
}

// GetRound returns the current consensus round.
func (bft *TendermintBFT) GetRound() int32 {
	bft.mu.RLock()
	defer bft.mu.RUnlock()
	return bft.round
}

// IsValidator returns true if this node is a validator.
func (bft *TendermintBFT) IsValidator() bool {
	bft.mu.RLock()
	defer bft.mu.RUnlock()
	return bft.validatorIdx >= 0
}

// ValidatorSet returns the current validator set.
func (bft *TendermintBFT) ValidatorSet() consensus.ValidatorSet {
	bft.mu.RLock()
	defer bft.mu.RUnlock()
	return bft.validatorSet
}

// HandleProposal processes a block proposal from a proposer.
func (bft *TendermintBFT) HandleProposal(proposal *consensus.Proposal) error {
	if proposal == nil {
		return fmt.Errorf("nil proposal")
	}

	bft.mu.Lock()
	defer bft.mu.Unlock()

	// Ignore proposals for past heights or rounds
	if proposal.Height < bft.height || (proposal.Height == bft.height && proposal.Round < bft.round) {
		return nil
	}

	// Ignore proposals for future heights
	if proposal.Height > bft.height {
		// TODO: Store for later or trigger sync
		return nil
	}

	// TODO: Verify proposer signature
	// TODO: Verify proposer is correct for this round
	// TODO: Verify POLRound matches a valid lock

	// Store the proposal
	bft.proposal = proposal

	// Write to WAL
	if bft.wal != nil {
		// TODO: Write proposal to WAL
	}

	// If we're still in propose step, move to prevote
	if bft.step == RoundStepPropose {
		bft.enterPrevote(bft.height, bft.round)
	}

	return nil
}

// HandleVote processes a consensus vote.
func (bft *TendermintBFT) HandleVote(vote *consensus.Vote) error {
	if vote == nil {
		return fmt.Errorf("nil vote")
	}

	bft.mu.Lock()
	defer bft.mu.Unlock()

	// Ignore votes for past heights
	if vote.Height < bft.height {
		return nil
	}

	// TODO: Verify vote signature
	// TODO: Verify voter is in validator set
	// TODO: Check for equivocation

	// Add vote to the appropriate vote set
	switch vote.Type {
	case consensus.VoteTypePrevote:
		if bft.prevotes != nil {
			added, err := bft.prevotes.AddVote(vote)
			if err != nil {
				return err
			}
			if added {
				bft.onPrevoteAdded(vote)
			}
		}
	case consensus.VoteTypePrecommit:
		if bft.precommits != nil {
			added, err := bft.precommits.AddVote(vote)
			if err != nil {
				return err
			}
			if added {
				bft.onPrecommitAdded(vote)
			}
		}
	}

	return nil
}

// HandleCommit processes a commit message.
func (bft *TendermintBFT) HandleCommit(commit *consensus.Commit) error {
	if commit == nil {
		return fmt.Errorf("nil commit")
	}

	bft.mu.Lock()
	defer bft.mu.Unlock()

	// TODO: Verify commit signatures
	// TODO: If valid, commit the block
	// TODO: If for future height, trigger sync

	return nil
}

// OnTimeout handles consensus timeouts.
func (bft *TendermintBFT) OnTimeout(height int64, round int32, step consensus.TimeoutStep) error {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	// Ignore stale timeouts
	if height != bft.height || round != bft.round {
		return nil
	}

	switch step {
	case consensus.TimeoutStepPropose:
		bft.onTimeoutPropose(height, round)
	case consensus.TimeoutStepPrevote:
		bft.onTimeoutPrevote(height, round)
	case consensus.TimeoutStepPrecommit:
		bft.onTimeoutPrecommit(height, round)
	}

	return nil
}

// ProduceBlock creates a new block for the given height.
func (bft *TendermintBFT) ProduceBlock(height int64) (*consensus.Block, error) {
	bft.mu.RLock()
	defer bft.mu.RUnlock()

	if height != bft.height {
		return nil, fmt.Errorf("cannot produce block for height %d, current height is %d", height, bft.height)
	}

	// TODO: Reap transactions from mempool
	// TODO: Create block with parent hash, timestamp, etc.
	// TODO: Execute transactions and get app hash

	block := &consensus.Block{
		Height: height,
		Round:  bft.round,
		// TODO: Fill in other fields
	}

	return block, nil
}

// ShouldPropose returns true if this node should propose for the round.
func (bft *TendermintBFT) ShouldPropose(height int64, round int32) bool {
	bft.mu.RLock()
	defer bft.mu.RUnlock()

	if bft.validatorSet == nil || bft.validatorIdx < 0 {
		return false
	}

	proposer := bft.validatorSet.GetProposer(height, round)
	if proposer == nil {
		return false
	}

	return proposer.Index == uint16(bft.validatorIdx)
}

// mainLoop is the main consensus loop.
func (bft *TendermintBFT) mainLoop() {
	defer bft.wg.Done()

	for {
		select {
		case <-bft.stopCh:
			return
		default:
			// TODO: Handle events from various sources:
			// - New proposals
			// - New votes
			// - Timeouts
			// - New transactions (for proposal)
			// For now, just sleep to avoid busy loop
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// enterNewRound enters a new consensus round.
func (bft *TendermintBFT) enterNewRound(height int64, round int32) {
	// Already at this height/round
	if bft.height == height && bft.round == round {
		return
	}

	bft.height = height
	bft.round = round
	bft.step = RoundStepNewRound
	bft.proposal = nil

	// Reset vote tracking for new round
	// TODO: Initialize prevotes and precommits HeightVoteSets

	// If we're the proposer, create and broadcast proposal
	if bft.ShouldPropose(height, round) {
		bft.enterPropose(height, round)
	} else {
		// Schedule timeout for waiting for proposal
		bft.scheduleTimeout(bft.cfg.TimeoutPropose, height, round, consensus.TimeoutStepPropose)
		bft.step = RoundStepPropose
	}
}

// enterPropose enters the propose step.
func (bft *TendermintBFT) enterPropose(height int64, round int32) {
	bft.step = RoundStepPropose

	// Create proposal
	var block *consensus.Block
	if bft.validValue != nil && bft.validRound >= 0 {
		// Re-propose the valid value
		block = bft.validValue
	} else {
		// Create a new block
		var err error
		block, err = bft.ProduceBlock(height)
		if err != nil {
			// Failed to produce block, prevote nil
			bft.enterPrevote(height, round)
			return
		}
	}

	// Create and sign proposal
	proposal := &consensus.Proposal{
		Height:   height,
		Round:    round,
		Block:    block,
		POLRound: bft.validRound,
	}

	// TODO: Sign proposal with private validator

	// Broadcast proposal
	// TODO: Broadcast to peers

	// Set our own proposal
	bft.proposal = proposal

	// Write to WAL
	if bft.wal != nil {
		// TODO: Write proposal to WAL
	}

	// Move to prevote
	bft.enterPrevote(height, round)
}

// enterPrevote enters the prevote step.
func (bft *TendermintBFT) enterPrevote(height int64, round int32) {
	bft.step = RoundStepPrevote

	// Determine what to prevote for
	var blockHash []byte

	// If locked, prevote for locked value
	if bft.lockedValue != nil {
		blockHash = bft.lockedValue.Hash
	} else if bft.proposal != nil && bft.proposal.Block != nil {
		// TODO: Validate proposal
		// If valid, prevote for it
		blockHash = bft.proposal.Block.Hash
	}
	// Otherwise, prevote nil (empty blockHash)

	// Create and sign prevote
	vote := &consensus.Vote{
		Type:      consensus.VoteTypePrevote,
		Height:    height,
		Round:     round,
		BlockHash: blockHash,
		// TODO: Set ValidatorIndex and sign
	}

	// TODO: Sign vote with private validator
	// TODO: Broadcast vote to peers

	// Write to WAL
	if bft.wal != nil {
		// TODO: Write vote to WAL
	}

	_ = vote // Suppress unused variable warning for skeleton
}

// enterPrecommit enters the precommit step.
func (bft *TendermintBFT) enterPrecommit(height int64, round int32) {
	bft.step = RoundStepPrecommit

	// Determine what to precommit for
	var blockHash []byte

	// Check if we have 2f+1 prevotes for a block
	if bft.prevotes != nil {
		// TODO: Get the block hash with 2f+1 prevotes
		// If we have it, precommit for it
		// Otherwise, precommit nil
	}

	// Create and sign precommit
	vote := &consensus.Vote{
		Type:      consensus.VoteTypePrecommit,
		Height:    height,
		Round:     round,
		BlockHash: blockHash,
		// TODO: Set ValidatorIndex and sign
	}

	// TODO: Sign vote with private validator
	// TODO: Broadcast vote to peers

	_ = vote // Suppress unused variable warning for skeleton
}

// enterCommit enters the commit step.
func (bft *TendermintBFT) enterCommit(height int64) {
	bft.step = RoundStepCommit

	// TODO: Get the block to commit
	// TODO: Execute the block
	// TODO: Commit to state store
	// TODO: Notify callbacks

	// Schedule timeout to start next height
	if !bft.cfg.SkipTimeoutCommit {
		// TODO: Schedule commit timeout
	}
}

// onPrevoteAdded is called when a prevote is added to the vote set.
func (bft *TendermintBFT) onPrevoteAdded(vote *consensus.Vote) {
	// TODO: Check if we now have 2f+1 prevotes for any block
	// If so, update locked/valid values and potentially move to precommit
	_ = vote
}

// onPrecommitAdded is called when a precommit is added to the vote set.
func (bft *TendermintBFT) onPrecommitAdded(vote *consensus.Vote) {
	// TODO: Check if we now have 2f+1 precommits for any block
	// If so, commit the block
	_ = vote
}

// onTimeoutPropose is called when the propose timeout fires.
func (bft *TendermintBFT) onTimeoutPropose(height int64, round int32) {
	if bft.step == RoundStepPropose {
		// No proposal received, prevote nil
		bft.enterPrevote(height, round)
	}
}

// onTimeoutPrevote is called when the prevote timeout fires.
func (bft *TendermintBFT) onTimeoutPrevote(height int64, round int32) {
	if bft.step == RoundStepPrevoteWait {
		// Move to precommit
		bft.enterPrecommit(height, round)
	}
}

// onTimeoutPrecommit is called when the precommit timeout fires.
func (bft *TendermintBFT) onTimeoutPrecommit(height int64, round int32) {
	if bft.step == RoundStepPrecommitWait {
		// Move to next round
		bft.enterNewRound(height, round+1)
	}
}

// scheduleTimeout schedules a consensus timeout.
func (bft *TendermintBFT) scheduleTimeout(duration time.Duration, height int64, round int32, step consensus.TimeoutStep) {
	if bft.timeoutTicker != nil {
		bft.timeoutTicker.ScheduleTimeout(duration, height, round, step)
	}
}

// SetValidatorSet sets the validator set.
func (bft *TendermintBFT) SetValidatorSet(valSet consensus.ValidatorSet) {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	bft.validatorSet = valSet

	// Update our validator index
	if bft.privValidator != nil && valSet != nil {
		addr := bft.privValidator.GetAddress()
		if validator := valSet.GetByAddress(addr); validator != nil {
			bft.validatorIdx = int(validator.Index)
		} else {
			bft.validatorIdx = -1
		}
	}
}

// SetPrivValidator sets the private validator.
func (bft *TendermintBFT) SetPrivValidator(pv PrivValidator) {
	bft.mu.Lock()
	defer bft.mu.Unlock()

	bft.privValidator = pv

	// Update our validator index
	if pv != nil && bft.validatorSet != nil {
		addr := pv.GetAddress()
		if validator := bft.validatorSet.GetByAddress(addr); validator != nil {
			bft.validatorIdx = int(validator.Index)
		} else {
			bft.validatorIdx = -1
		}
	}
}

// SetWAL sets the write-ahead log.
func (bft *TendermintBFT) SetWAL(wal WAL) {
	bft.mu.Lock()
	defer bft.mu.Unlock()
	bft.wal = wal
}

// SetTimeoutTicker sets the timeout ticker.
func (bft *TendermintBFT) SetTimeoutTicker(ticker TimeoutTicker) {
	bft.mu.Lock()
	defer bft.mu.Unlock()
	bft.timeoutTicker = ticker
}

// Verify TendermintBFT implements the interfaces.
var (
	_ consensus.ConsensusEngine = (*TendermintBFT)(nil)
	_ consensus.BFTConsensus    = (*TendermintBFT)(nil)
	_ consensus.BlockProducer   = (*TendermintBFT)(nil)
)
