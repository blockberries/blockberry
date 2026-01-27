package consensus

import (
	"bytes"
	"sync"
)

// ValidatorDetector determines if this node is a validator in the current validator set.
// This is used for role-based behavior and dynamic role switching.
type ValidatorDetector interface {
	// IsValidator returns true if this node is in the given validator set.
	IsValidator(valSet ValidatorSet) bool

	// ValidatorIndex returns this node's validator index, or -1 if not a validator.
	ValidatorIndex(valSet ValidatorSet) int

	// GetValidator returns this node's validator info if it's in the set, nil otherwise.
	GetValidator(valSet ValidatorSet) *Validator
}

// KeyBasedDetector uses the node's public key to detect validator status.
// This is the standard detection method where validator identity is tied to keys.
type KeyBasedDetector struct {
	publicKey []byte
}

// NewKeyBasedDetector creates a new key-based validator detector.
func NewKeyBasedDetector(publicKey []byte) *KeyBasedDetector {
	keyCopy := make([]byte, len(publicKey))
	copy(keyCopy, publicKey)
	return &KeyBasedDetector{
		publicKey: keyCopy,
	}
}

// IsValidator returns true if this node's public key is in the validator set.
func (d *KeyBasedDetector) IsValidator(valSet ValidatorSet) bool {
	if valSet == nil {
		return false
	}
	return d.ValidatorIndex(valSet) >= 0
}

// ValidatorIndex returns this node's validator index, or -1 if not a validator.
func (d *KeyBasedDetector) ValidatorIndex(valSet ValidatorSet) int {
	if valSet == nil {
		return -1
	}

	validators := valSet.Validators()
	for i, val := range validators {
		if bytes.Equal(val.PublicKey, d.publicKey) {
			return i
		}
	}
	return -1
}

// GetValidator returns this node's validator info if it's in the set.
func (d *KeyBasedDetector) GetValidator(valSet ValidatorSet) *Validator {
	if valSet == nil {
		return nil
	}

	for _, val := range valSet.Validators() {
		if bytes.Equal(val.PublicKey, d.publicKey) {
			return val
		}
	}
	return nil
}

// PublicKey returns the detector's public key.
func (d *KeyBasedDetector) PublicKey() []byte {
	keyCopy := make([]byte, len(d.publicKey))
	copy(keyCopy, d.publicKey)
	return keyCopy
}

// AddressBasedDetector uses the node's address to detect validator status.
// Use this when validator identity is tied to addresses rather than public keys.
type AddressBasedDetector struct {
	address []byte
}

// NewAddressBasedDetector creates a new address-based validator detector.
func NewAddressBasedDetector(address []byte) *AddressBasedDetector {
	addrCopy := make([]byte, len(address))
	copy(addrCopy, address)
	return &AddressBasedDetector{
		address: addrCopy,
	}
}

// IsValidator returns true if this node's address is in the validator set.
func (d *AddressBasedDetector) IsValidator(valSet ValidatorSet) bool {
	if valSet == nil {
		return false
	}
	return valSet.Contains(d.address)
}

// ValidatorIndex returns this node's validator index, or -1 if not a validator.
func (d *AddressBasedDetector) ValidatorIndex(valSet ValidatorSet) int {
	if valSet == nil {
		return -1
	}

	validators := valSet.Validators()
	for i, val := range validators {
		if bytes.Equal(val.Address, d.address) {
			return i
		}
	}
	return -1
}

// GetValidator returns this node's validator info if it's in the set.
func (d *AddressBasedDetector) GetValidator(valSet ValidatorSet) *Validator {
	if valSet == nil {
		return nil
	}
	return valSet.GetByAddress(d.address)
}

// Address returns the detector's address.
func (d *AddressBasedDetector) Address() []byte {
	addrCopy := make([]byte, len(d.address))
	copy(addrCopy, d.address)
	return addrCopy
}

// ValidatorStatusCallback is called when validator status changes.
type ValidatorStatusCallback func(isValidator bool, index int, validator *Validator)

// ValidatorStatusTracker monitors validator set changes and notifies on status changes.
// It maintains the current validator status and detects when the node joins or leaves
// the validator set.
type ValidatorStatusTracker struct {
	detector      ValidatorDetector
	callback      ValidatorStatusCallback
	lastStatus    bool
	lastIndex     int
	lastValidator *Validator
	mu            sync.RWMutex
}

// NewValidatorStatusTracker creates a new validator status tracker.
func NewValidatorStatusTracker(detector ValidatorDetector, callback ValidatorStatusCallback) *ValidatorStatusTracker {
	return &ValidatorStatusTracker{
		detector:   detector,
		callback:   callback,
		lastStatus: false,
		lastIndex:  -1,
	}
}

// Update checks the validator set for status changes.
// If the status changed, the callback is invoked.
// Returns true if the status changed.
func (t *ValidatorStatusTracker) Update(valSet ValidatorSet) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	isValidator := t.detector.IsValidator(valSet)
	index := t.detector.ValidatorIndex(valSet)
	var validator *Validator
	if isValidator {
		validator = t.detector.GetValidator(valSet)
	}

	// Check if status changed
	statusChanged := isValidator != t.lastStatus || index != t.lastIndex

	if statusChanged {
		t.lastStatus = isValidator
		t.lastIndex = index
		t.lastValidator = validator

		if t.callback != nil {
			// Invoke callback outside of lock to prevent deadlocks
			// Copy values to avoid race conditions
			cb := t.callback
			t.mu.Unlock()
			cb(isValidator, index, validator)
			t.mu.Lock()
		}
	}

	return statusChanged
}

// IsValidator returns the current validator status.
func (t *ValidatorStatusTracker) IsValidator() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastStatus
}

// ValidatorIndex returns the current validator index, or -1 if not a validator.
func (t *ValidatorStatusTracker) ValidatorIndex() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastIndex
}

// Validator returns the current validator info, or nil if not a validator.
func (t *ValidatorStatusTracker) Validator() *Validator {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastValidator
}

// SetCallback sets or updates the status change callback.
func (t *ValidatorStatusTracker) SetCallback(callback ValidatorStatusCallback) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.callback = callback
}

// Verify interface compliance.
var (
	_ ValidatorDetector = (*KeyBasedDetector)(nil)
	_ ValidatorDetector = (*AddressBasedDetector)(nil)
)
