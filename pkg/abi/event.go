package abi

// Event represents an application event emitted during transaction execution
// or block processing. Events are used for indexing and external monitoring.
type Event struct {
	// Type is the event type identifier (e.g., "transfer", "delegate").
	Type string

	// Attributes are the key-value pairs associated with this event.
	Attributes []Attribute
}

// NewEvent creates a new event with the given type.
func NewEvent(eventType string) Event {
	return Event{Type: eventType}
}

// AddAttribute adds an attribute to the event and returns the event for chaining.
func (e Event) AddAttribute(key string, value []byte) Event {
	e.Attributes = append(e.Attributes, Attribute{Key: key, Value: value})
	return e
}

// AddStringAttribute adds a string attribute to the event.
func (e Event) AddStringAttribute(key, value string) Event {
	return e.AddAttribute(key, []byte(value))
}

// AddIndexedAttribute adds an indexed attribute to the event.
func (e Event) AddIndexedAttribute(key string, value []byte) Event {
	e.Attributes = append(e.Attributes, Attribute{Key: key, Value: value, Index: true})
	return e
}

// Attribute represents a key-value pair within an event.
type Attribute struct {
	// Key is the attribute name.
	Key string

	// Value is the attribute value.
	Value []byte

	// Index indicates whether this attribute should be indexed for queries.
	Index bool
}

// StringValue returns the attribute value as a string.
func (a Attribute) StringValue() string {
	return string(a.Value)
}

// Common event types used throughout the system.
const (
	// Block events
	EventNewBlock       = "NewBlock"
	EventNewBlockHeader = "NewBlockHeader"
	EventCommit         = "Commit"

	// Transaction events
	EventTx        = "Tx"
	EventTxAdded   = "TxAdded"
	EventTxRemoved = "TxRemoved"

	// Consensus events
	EventVote                = "Vote"
	EventValidatorSetUpdates = "ValidatorSetUpdates"
	EventEvidence            = "Evidence"

	// Network events
	EventPeerConnected    = "PeerConnected"
	EventPeerDisconnected = "PeerDisconnected"
	EventPeerMisbehavior  = "PeerMisbehavior"

	// Sync events
	EventSyncStarted   = "SyncStarted"
	EventSyncProgress  = "SyncProgress"
	EventSyncCompleted = "SyncCompleted"
)

// Common attribute keys used in events.
const (
	AttributeKeyHeight    = "height"
	AttributeKeyHash      = "hash"
	AttributeKeyTxHash    = "tx.hash"
	AttributeKeySender    = "sender"
	AttributeKeyRecipient = "recipient"
	AttributeKeyAmount    = "amount"
	AttributeKeyValidator = "validator"
	AttributeKeyPeerID    = "peer_id"
	AttributeKeyReason    = "reason"
)
