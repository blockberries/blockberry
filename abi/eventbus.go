package abi

import (
	"context"
	"time"
)

// EventBus provides pub/sub for system events.
// Subscribers receive events that match their query through a channel.
// The EventBus is thread-safe and can handle multiple concurrent publishers
// and subscribers.
type EventBus interface {
	Component

	// Subscribe creates a subscription for events matching the query.
	// Returns a channel that will receive matching events.
	// The subscriber string identifies this subscription for later unsubscribe.
	// The channel is closed when the subscription is cancelled or the bus stops.
	Subscribe(ctx context.Context, subscriber string, query Query) (<-chan Event, error)

	// Unsubscribe removes a specific subscription.
	// The subscriber and query must match a previous Subscribe call.
	Unsubscribe(ctx context.Context, subscriber string, query Query) error

	// UnsubscribeAll removes all subscriptions for a subscriber.
	UnsubscribeAll(ctx context.Context, subscriber string) error

	// Publish sends an event to all matching subscribers.
	// This is non-blocking; if a subscriber's channel is full, the event may be dropped.
	Publish(ctx context.Context, event Event) error

	// PublishWithTimeout sends an event with a timeout for slow subscribers.
	// If a subscriber's channel is full, it waits up to timeout before dropping.
	PublishWithTimeout(ctx context.Context, event Event, timeout time.Duration) error

	// NumSubscribers returns the total number of active subscriptions.
	NumSubscribers() int

	// NumSubscribersForQuery returns the number of subscribers for a specific query.
	NumSubscribersForQuery(query Query) int
}

// Query filters events for subscription matching.
// Implementations determine which events a subscriber receives.
type Query interface {
	// Matches returns true if the event should be delivered to this subscriber.
	Matches(event Event) bool

	// String returns a string representation of the query for debugging.
	String() string
}

// QueryAll matches all events.
type QueryAll struct{}

// Matches always returns true.
func (q QueryAll) Matches(event Event) bool {
	return true
}

// String returns the query representation.
func (q QueryAll) String() string {
	return "all"
}

// QueryEventType matches events by their type.
type QueryEventType struct {
	EventType string
}

// Matches returns true if the event type matches.
func (q QueryEventType) Matches(event Event) bool {
	return event.Type == q.EventType
}

// String returns the query representation.
func (q QueryEventType) String() string {
	return "type=" + q.EventType
}

// QueryEventTypes matches events by multiple types.
type QueryEventTypes struct {
	EventTypes []string
}

// Matches returns true if the event type is in the list.
func (q QueryEventTypes) Matches(event Event) bool {
	for _, t := range q.EventTypes {
		if event.Type == t {
			return true
		}
	}
	return false
}

// String returns the query representation.
func (q QueryEventTypes) String() string {
	if len(q.EventTypes) == 0 {
		return "types=[]"
	}
	result := "types=["
	for i, t := range q.EventTypes {
		if i > 0 {
			result += ","
		}
		result += t
	}
	return result + "]"
}

// QueryFunc allows using a function as a query.
type QueryFunc struct {
	Fn          func(Event) bool
	Description string
}

// Matches calls the function.
func (q QueryFunc) Matches(event Event) bool {
	if q.Fn == nil {
		return false
	}
	return q.Fn(event)
}

// String returns the description.
func (q QueryFunc) String() string {
	if q.Description == "" {
		return "func"
	}
	return q.Description
}

// QueryAnd combines multiple queries with AND logic.
type QueryAnd struct {
	Queries []Query
}

// Matches returns true if all queries match.
func (q QueryAnd) Matches(event Event) bool {
	for _, query := range q.Queries {
		if !query.Matches(event) {
			return false
		}
	}
	return true
}

// String returns the query representation.
func (q QueryAnd) String() string {
	if len(q.Queries) == 0 {
		return "and()"
	}
	result := "and("
	for i, query := range q.Queries {
		if i > 0 {
			result += ","
		}
		result += query.String()
	}
	return result + ")"
}

// QueryOr combines multiple queries with OR logic.
type QueryOr struct {
	Queries []Query
}

// Matches returns true if any query matches.
func (q QueryOr) Matches(event Event) bool {
	for _, query := range q.Queries {
		if query.Matches(event) {
			return true
		}
	}
	return false
}

// String returns the query representation.
func (q QueryOr) String() string {
	if len(q.Queries) == 0 {
		return "or()"
	}
	result := "or("
	for i, query := range q.Queries {
		if i > 0 {
			result += ","
		}
		result += query.String()
	}
	return result + ")"
}

// QueryAttribute matches events that have a specific attribute key-value pair.
type QueryAttribute struct {
	Key   string
	Value string
}

// Matches returns true if the event has the matching attribute.
func (q QueryAttribute) Matches(event Event) bool {
	for _, attr := range event.Attributes {
		if attr.Key == q.Key && attr.StringValue() == q.Value {
			return true
		}
	}
	return false
}

// String returns the query representation.
func (q QueryAttribute) String() string {
	return q.Key + "=" + q.Value
}

// QueryAttributeExists matches events that have a specific attribute key.
type QueryAttributeExists struct {
	Key string
}

// Matches returns true if the event has the attribute key.
func (q QueryAttributeExists) Matches(event Event) bool {
	for _, attr := range event.Attributes {
		if attr.Key == q.Key {
			return true
		}
	}
	return false
}

// String returns the query representation.
func (q QueryAttributeExists) String() string {
	return "exists(" + q.Key + ")"
}

// Subscription represents an active event subscription.
// This is used internally by EventBus implementations.
type Subscription struct {
	// Subscriber is the unique identifier for this subscriber.
	Subscriber string

	// Query filters which events this subscription receives.
	Query Query

	// Channel receives matching events.
	Channel chan Event

	// Cancelled indicates if this subscription has been cancelled.
	Cancelled bool
}

// EventBusConfig contains configuration for EventBus implementations.
type EventBusConfig struct {
	// BufferSize is the channel buffer size for each subscription.
	// Larger buffers reduce the chance of dropped events but use more memory.
	// Default: 100
	BufferSize int

	// PublishTimeout is the default timeout for PublishWithTimeout.
	// Default: 100ms
	PublishTimeout time.Duration

	// MaxSubscribers is the maximum number of total subscriptions allowed.
	// 0 means unlimited.
	// Default: 0
	MaxSubscribers int

	// MaxSubscribersPerQuery is the max subscriptions for a single query.
	// 0 means unlimited.
	// Default: 0
	MaxSubscribersPerQuery int
}

// DefaultEventBusConfig returns sensible defaults for EventBusConfig.
func DefaultEventBusConfig() EventBusConfig {
	return EventBusConfig{
		BufferSize:             100,
		PublishTimeout:         100 * time.Millisecond,
		MaxSubscribers:         0,
		MaxSubscribersPerQuery: 0,
	}
}
