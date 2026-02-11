// Package events provides an in-memory implementation of the EventBus interface.
package events

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"time"

	bapitypes "github.com/blockberries/bapi/types"
)

// Common errors returned by the EventBus.
var (
	ErrBusNotRunning      = errors.New("event bus is not running")
	ErrBusStopped         = errors.New("event bus has been stopped")
	ErrSubscriberExists   = errors.New("subscriber already exists for this query")
	ErrSubscriberNotFound = errors.New("subscriber not found")
	ErrTooManySubscribers = errors.New("maximum number of subscribers reached")
)

// Bus is an in-memory implementation of EventBus.
// It provides thread-safe pub/sub for system events.
type Bus struct {
	config EventBusConfig

	// subscriptions maps subscriber+query to subscription
	subscriptions map[string]*subscription
	mu            sync.RWMutex

	// running state
	running atomic.Bool
	stopCh  chan struct{}
	wg      sync.WaitGroup
}

// subscription holds a single subscriber's channel and metadata.
type subscription struct {
	subscriber string
	query      Query
	ch         chan bapitypes.Event
	cancelled  atomic.Bool
}

// NewBus creates a new in-memory EventBus with default configuration.
func NewBus() *Bus {
	return NewBusWithConfig(DefaultEventBusConfig())
}

// NewBusWithConfig creates a new in-memory EventBus with the given configuration.
func NewBusWithConfig(config EventBusConfig) *Bus {
	if config.BufferSize <= 0 {
		config.BufferSize = 100
	}
	if config.PublishTimeout <= 0 {
		config.PublishTimeout = 100 * time.Millisecond
	}

	return &Bus{
		config:        config,
		subscriptions: make(map[string]*subscription),
		stopCh:        make(chan struct{}),
	}
}

// subscriptionKey creates a unique key for a subscriber+query pair.
func subscriptionKey(subscriber string, query Query) string {
	return subscriber + ":" + query.String()
}

// Start starts the event bus.
func (b *Bus) Start() error {
	if b.running.Swap(true) {
		return nil // Already running
	}
	b.stopCh = make(chan struct{})
	return nil
}

// Stop stops the event bus and closes all subscription channels.
func (b *Bus) Stop() error {
	if !b.running.Swap(false) {
		return nil // Already stopped
	}

	close(b.stopCh)

	// Close all subscription channels
	b.mu.Lock()
	for _, sub := range b.subscriptions {
		sub.cancelled.Store(true)
		close(sub.ch)
	}
	b.subscriptions = make(map[string]*subscription)
	b.mu.Unlock()

	b.wg.Wait()
	return nil
}

// IsRunning returns true if the event bus is running.
func (b *Bus) IsRunning() bool {
	return b.running.Load()
}

// Subscribe creates a subscription for events matching the query.
func (b *Bus) Subscribe(ctx context.Context, subscriber string, query Query) (<-chan bapitypes.Event, error) {
	if !b.running.Load() {
		return nil, ErrBusNotRunning
	}

	key := subscriptionKey(subscriber, query)

	b.mu.Lock()
	defer b.mu.Unlock()

	// Check if subscription already exists
	if _, exists := b.subscriptions[key]; exists {
		return nil, ErrSubscriberExists
	}

	// Check max subscribers limit
	if b.config.MaxSubscribers > 0 && len(b.subscriptions) >= b.config.MaxSubscribers {
		return nil, ErrTooManySubscribers
	}

	// Check max subscribers per query limit
	if b.config.MaxSubscribersPerQuery > 0 {
		count := 0
		queryStr := query.String()
		for _, sub := range b.subscriptions {
			if sub.query.String() == queryStr {
				count++
			}
		}
		if count >= b.config.MaxSubscribersPerQuery {
			return nil, ErrTooManySubscribers
		}
	}

	// Create new subscription
	sub := &subscription{
		subscriber: subscriber,
		query:      query,
		ch:         make(chan bapitypes.Event, b.config.BufferSize),
	}

	b.subscriptions[key] = sub

	// Handle context cancellation
	if ctx != nil && ctx.Done() != nil {
		b.wg.Add(1)
		go func() {
			defer b.wg.Done()
			select {
			case <-ctx.Done():
				_ = b.Unsubscribe(context.Background(), subscriber, query)
			case <-b.stopCh:
				// Bus stopped
			}
		}()
	}

	return sub.ch, nil
}

// Unsubscribe removes a specific subscription.
func (b *Bus) Unsubscribe(ctx context.Context, subscriber string, query Query) error {
	key := subscriptionKey(subscriber, query)

	b.mu.Lock()
	defer b.mu.Unlock()

	sub, exists := b.subscriptions[key]
	if !exists {
		return ErrSubscriberNotFound
	}

	// Mark as cancelled and close channel
	if !sub.cancelled.Swap(true) {
		close(sub.ch)
	}
	delete(b.subscriptions, key)

	return nil
}

// UnsubscribeAll removes all subscriptions for a subscriber.
func (b *Bus) UnsubscribeAll(ctx context.Context, subscriber string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Find and remove all subscriptions for this subscriber
	toDelete := make([]string, 0)
	for key, sub := range b.subscriptions {
		if sub.subscriber == subscriber {
			if !sub.cancelled.Swap(true) {
				close(sub.ch)
			}
			toDelete = append(toDelete, key)
		}
	}

	for _, key := range toDelete {
		delete(b.subscriptions, key)
	}

	return nil
}

// Publish sends an event to all matching subscribers.
// Non-blocking: if a subscriber's channel is full, the event is dropped for that subscriber.
func (b *Bus) Publish(ctx context.Context, event bapitypes.Event) error {
	if !b.running.Load() {
		return ErrBusNotRunning
	}

	b.mu.RLock()
	defer b.mu.RUnlock()

	for _, sub := range b.subscriptions {
		if sub.cancelled.Load() {
			continue
		}
		if sub.query.Matches(event) {
			// Non-blocking send
			select {
			case sub.ch <- event:
			default:
				// Channel full, drop event for this subscriber
			}
		}
	}

	return nil
}

// PublishWithTimeout sends an event with a timeout for slow subscribers.
func (b *Bus) PublishWithTimeout(ctx context.Context, event bapitypes.Event, timeout time.Duration) error {
	if !b.running.Load() {
		return ErrBusNotRunning
	}

	b.mu.RLock()
	subs := make([]*subscription, 0, len(b.subscriptions))
	for _, sub := range b.subscriptions {
		if !sub.cancelled.Load() && sub.query.Matches(event) {
			subs = append(subs, sub)
		}
	}
	b.mu.RUnlock()

	// Send to each matching subscriber with timeout
	for _, sub := range subs {
		if sub.cancelled.Load() {
			continue
		}

		// Use time.NewTimer instead of time.After to avoid timer leaks
		timer := time.NewTimer(timeout)
		select {
		case sub.ch <- event:
			// Sent successfully - stop timer to prevent leak
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
			// Timeout, drop event for this subscriber
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-b.stopCh:
			timer.Stop()
			return ErrBusStopped
		}
	}

	return nil
}

// NumSubscribers returns the total number of active subscriptions.
func (b *Bus) NumSubscribers() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.subscriptions)
}

// NumSubscribersForQuery returns the number of subscribers for a specific query.
func (b *Bus) NumSubscribersForQuery(query Query) int {
	b.mu.RLock()
	defer b.mu.RUnlock()

	count := 0
	queryStr := query.String()
	for _, sub := range b.subscriptions {
		if sub.query.String() == queryStr {
			count++
		}
	}
	return count
}

// Ensure Bus implements EventBus.
var _ EventBus = (*Bus)(nil)
