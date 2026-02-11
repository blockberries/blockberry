package events

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bapitypes "github.com/blockberries/bapi/types"
)

func TestBus_StartStop(t *testing.T) {
	bus := NewBus()

	// Initially not running
	assert.False(t, bus.IsRunning())

	// Start
	require.NoError(t, bus.Start())
	assert.True(t, bus.IsRunning())

	// Start again (idempotent)
	require.NoError(t, bus.Start())
	assert.True(t, bus.IsRunning())

	// Stop
	require.NoError(t, bus.Stop())
	assert.False(t, bus.IsRunning())

	// Stop again (idempotent)
	require.NoError(t, bus.Stop())
	assert.False(t, bus.IsRunning())
}

func TestBus_SubscribeBeforeStart(t *testing.T) {
	bus := NewBus()

	_, err := bus.Subscribe(context.Background(), "test", QueryAll{})
	assert.Equal(t, ErrBusNotRunning, err)
}

func TestBus_PublishBeforeStart(t *testing.T) {
	bus := NewBus()

	err := bus.Publish(context.Background(), bapitypes.Event{Kind: "test"})
	assert.Equal(t, ErrBusNotRunning, err)
}

func TestBus_SubscribeAndPublish(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe
	ch, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)
	require.NotNil(t, ch)

	// Publish
	event := bapitypes.Event{
		Kind: "TestEvent",
		Attributes: []bapitypes.EventAttribute{
			{Key: "key", Value: "value"},
		},
	}
	require.NoError(t, bus.Publish(context.Background(), event))

	// Receive
	select {
	case received := <-ch:
		assert.Equal(t, "TestEvent", received.Kind)
		assert.Len(t, received.Attributes, 1)
		assert.Equal(t, "key", received.Attributes[0].Key)
		assert.Equal(t, "value", received.Attributes[0].Value)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestBus_QueryEventKind(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe to specific event kind
	query := QueryEventKind{Kind: "Transfer"}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Publish matching event
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Transfer"}))

	// Publish non-matching event
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Delegate"}))

	// Should only receive Transfer
	select {
	case received := <-ch:
		assert.Equal(t, "Transfer", received.Kind)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive another event
	select {
	case <-ch:
		t.Fatal("should not receive Delegate event")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestBus_QueryEventKinds(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe to multiple event kinds
	query := QueryEventKinds{Kinds: []string{"Transfer", "Delegate"}}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Publish matching events
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Transfer"}))
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Delegate"}))
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Other"}))

	// Should receive Transfer and Delegate, not Other
	receivedKinds := make([]string, 0, 2)
	for i := 0; i < 2; i++ {
		select {
		case received := <-ch:
			receivedKinds = append(receivedKinds, received.Kind)
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
	assert.Contains(t, receivedKinds, "Transfer")
	assert.Contains(t, receivedKinds, "Delegate")
}

func TestBus_QueryAttribute(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe to events with specific attribute
	query := QueryAttribute{Key: "sender", Value: "alice"}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Publish matching event
	event1 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	require.NoError(t, bus.Publish(context.Background(), event1))

	// Publish non-matching event
	event2 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "bob"}},
	}
	require.NoError(t, bus.Publish(context.Background(), event2))

	// Should only receive alice's event
	select {
	case received := <-ch:
		assert.Equal(t, "Transfer", received.Kind)
		assert.Equal(t, "alice", received.Attributes[0].Value)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive bob's event
	select {
	case <-ch:
		t.Fatal("should not receive bob's event")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestBus_QueryAnd(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe with AND query
	query := QueryAnd{
		Queries: []Query{
			QueryEventKind{Kind: "Transfer"},
			QueryAttribute{Key: "sender", Value: "alice"},
		},
	}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Publish event matching both conditions
	event1 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	require.NoError(t, bus.Publish(context.Background(), event1))

	// Publish event matching only kind
	event2 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "bob"}},
	}
	require.NoError(t, bus.Publish(context.Background(), event2))

	// Publish event matching only attribute
	event3 := bapitypes.Event{
		Kind:       "Delegate",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	require.NoError(t, bus.Publish(context.Background(), event3))

	// Should only receive first event
	select {
	case received := <-ch:
		assert.Equal(t, "Transfer", received.Kind)
		assert.Equal(t, "alice", received.Attributes[0].Value)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}

	// Should not receive other events
	select {
	case <-ch:
		t.Fatal("should not receive other events")
	case <-time.After(100 * time.Millisecond):
		// Expected
	}
}

func TestBus_QueryOr(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe with OR query
	query := QueryOr{
		Queries: []Query{
			QueryEventKind{Kind: "Transfer"},
			QueryEventKind{Kind: "Delegate"},
		},
	}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Publish matching events
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Transfer"}))
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Delegate"}))
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Other"}))

	// Should receive Transfer and Delegate
	count := 0
	for i := 0; i < 2; i++ {
		select {
		case <-ch:
			count++
		case <-time.After(time.Second):
			t.Fatal("timeout waiting for event")
		}
	}
	assert.Equal(t, 2, count)
}

func TestBus_Unsubscribe(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	query := QueryAll{}
	ch, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)
	assert.Equal(t, 1, bus.NumSubscribers())

	// Unsubscribe
	require.NoError(t, bus.Unsubscribe(context.Background(), "sub1", query))
	assert.Equal(t, 0, bus.NumSubscribers())

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok)

	// Unsubscribe again should fail
	err = bus.Unsubscribe(context.Background(), "sub1", query)
	assert.Equal(t, ErrSubscriberNotFound, err)
}

func TestBus_UnsubscribeAll(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	// Subscribe multiple times
	_, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "sub1", QueryEventKind{Kind: "Test"})
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "sub2", QueryAll{})
	require.NoError(t, err)

	assert.Equal(t, 3, bus.NumSubscribers())

	// Unsubscribe all for sub1
	require.NoError(t, bus.UnsubscribeAll(context.Background(), "sub1"))
	assert.Equal(t, 1, bus.NumSubscribers())
}

func TestBus_DuplicateSubscription(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	query := QueryAll{}
	_, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)

	// Try to subscribe again with same subscriber+query
	_, err = bus.Subscribe(context.Background(), "sub1", query)
	assert.Equal(t, ErrSubscriberExists, err)
}

func TestBus_MaxSubscribers(t *testing.T) {
	config := EventBusConfig{
		BufferSize:     10,
		MaxSubscribers: 2,
	}
	bus := NewBusWithConfig(config)
	require.NoError(t, bus.Start())
	defer bus.Stop()

	_, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)

	_, err = bus.Subscribe(context.Background(), "sub2", QueryAll{})
	require.NoError(t, err)

	// Third subscription should fail
	_, err = bus.Subscribe(context.Background(), "sub3", QueryAll{})
	assert.Equal(t, ErrTooManySubscribers, err)
}

func TestBus_PublishWithTimeout(t *testing.T) {
	bus := NewBusWithConfig(EventBusConfig{BufferSize: 1})
	require.NoError(t, bus.Start())
	defer bus.Stop()

	ch, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)

	// Fill the buffer
	require.NoError(t, bus.Publish(context.Background(), bapitypes.Event{Kind: "Event1"}))

	// This should timeout because buffer is full
	start := time.Now()
	err = bus.PublishWithTimeout(context.Background(), bapitypes.Event{Kind: "Event2"}, 50*time.Millisecond)
	elapsed := time.Since(start)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, elapsed, 50*time.Millisecond)

	// Drain the first event
	<-ch
}

func TestBus_ConcurrentPublish(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	ch, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)

	const numPublishers = 10
	const numEvents = 100

	var wg sync.WaitGroup
	for i := 0; i < numPublishers; i++ {
		wg.Add(1)
		go func(publisherID int) {
			defer wg.Done()
			for j := 0; j < numEvents; j++ {
				event := bapitypes.Event{
					Kind: "Test",
					Attributes: []bapitypes.EventAttribute{
						{Key: "publisher", Value: string(rune('A' + publisherID))},
					},
				}
				_ = bus.Publish(context.Background(), event)
			}
		}(i)
	}

	// Receive events
	received := 0
	done := make(chan struct{})
	go func() {
		for range ch {
			received++
		}
		close(done)
	}()

	wg.Wait()
	time.Sleep(100 * time.Millisecond) // Give time for delivery

	// Stop and close channel
	bus.Stop()
	<-done

	// We should have received some events (may not be all due to buffer drops)
	t.Logf("Received %d of %d events", received, numPublishers*numEvents)
	assert.Greater(t, received, 0)
}

func TestBus_StopClosesChannels(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())

	ch, err := bus.Subscribe(context.Background(), "sub1", QueryAll{})
	require.NoError(t, err)

	// Stop the bus
	require.NoError(t, bus.Stop())

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok)
}

func TestBus_ContextCancellation(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	ctx, cancel := context.WithCancel(context.Background())
	ch, err := bus.Subscribe(ctx, "sub1", QueryAll{})
	require.NoError(t, err)
	assert.Equal(t, 1, bus.NumSubscribers())

	// Cancel context
	cancel()

	// Wait for unsubscription
	time.Sleep(100 * time.Millisecond)
	assert.Equal(t, 0, bus.NumSubscribers())

	// Channel should be closed
	_, ok := <-ch
	assert.False(t, ok)
}

func TestBus_NumSubscribersForQuery(t *testing.T) {
	bus := NewBus()
	require.NoError(t, bus.Start())
	defer bus.Stop()

	query := QueryEventKind{Kind: "Test"}

	_, err := bus.Subscribe(context.Background(), "sub1", query)
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "sub2", query)
	require.NoError(t, err)
	_, err = bus.Subscribe(context.Background(), "sub3", QueryAll{})
	require.NoError(t, err)

	assert.Equal(t, 3, bus.NumSubscribers())
	assert.Equal(t, 2, bus.NumSubscribersForQuery(query))
}

// Test query types directly
func TestQueryAll_Matches(t *testing.T) {
	q := QueryAll{}
	assert.True(t, q.Matches(bapitypes.Event{Kind: "anything"}))
	assert.Equal(t, "all", q.String())
}

func TestQueryEventKind_Matches(t *testing.T) {
	q := QueryEventKind{Kind: "Transfer"}
	assert.True(t, q.Matches(bapitypes.Event{Kind: "Transfer"}))
	assert.False(t, q.Matches(bapitypes.Event{Kind: "Delegate"}))
	assert.Equal(t, "kind=Transfer", q.String())
}

func TestQueryEventKinds_Matches(t *testing.T) {
	q := QueryEventKinds{Kinds: []string{"Transfer", "Delegate"}}
	assert.True(t, q.Matches(bapitypes.Event{Kind: "Transfer"}))
	assert.True(t, q.Matches(bapitypes.Event{Kind: "Delegate"}))
	assert.False(t, q.Matches(bapitypes.Event{Kind: "Other"}))
	assert.Equal(t, "kinds=[Transfer,Delegate]", q.String())

	// Empty
	q2 := QueryEventKinds{}
	assert.False(t, q2.Matches(bapitypes.Event{Kind: "anything"}))
	assert.Equal(t, "kinds=[]", q2.String())
}

func TestQueryFunc_Matches(t *testing.T) {
	q := QueryFunc{
		Fn:          func(e bapitypes.Event) bool { return len(e.Kind) > 5 },
		Description: "kind length > 5",
	}
	assert.True(t, q.Matches(bapitypes.Event{Kind: "Transfer"}))
	assert.False(t, q.Matches(bapitypes.Event{Kind: "Test"}))
	assert.Equal(t, "kind length > 5", q.String())

	// Nil function
	q2 := QueryFunc{}
	assert.False(t, q2.Matches(bapitypes.Event{Kind: "anything"}))
	assert.Equal(t, "func", q2.String())
}

func TestQueryAnd_Matches(t *testing.T) {
	q := QueryAnd{
		Queries: []Query{
			QueryEventKind{Kind: "Transfer"},
			QueryAttributeExists{Key: "sender"},
		},
	}

	// Both conditions met
	e1 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	assert.True(t, q.Matches(e1))

	// Only kind matches
	e2 := bapitypes.Event{Kind: "Transfer"}
	assert.False(t, q.Matches(e2))

	// Only attribute matches
	e3 := bapitypes.Event{
		Kind:       "Delegate",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	assert.False(t, q.Matches(e3))

	// Empty AND matches everything
	q2 := QueryAnd{}
	assert.True(t, q2.Matches(bapitypes.Event{Kind: "anything"}))
}

func TestQueryOr_Matches(t *testing.T) {
	q := QueryOr{
		Queries: []Query{
			QueryEventKind{Kind: "Transfer"},
			QueryEventKind{Kind: "Delegate"},
		},
	}

	assert.True(t, q.Matches(bapitypes.Event{Kind: "Transfer"}))
	assert.True(t, q.Matches(bapitypes.Event{Kind: "Delegate"}))
	assert.False(t, q.Matches(bapitypes.Event{Kind: "Other"}))

	// Empty OR matches nothing
	q2 := QueryOr{}
	assert.False(t, q2.Matches(bapitypes.Event{Kind: "anything"}))
}

func TestQueryAttribute_Matches(t *testing.T) {
	q := QueryAttribute{Key: "sender", Value: "alice"}

	e1 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	assert.True(t, q.Matches(e1))

	e2 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "bob"}},
	}
	assert.False(t, q.Matches(e2))

	e3 := bapitypes.Event{Kind: "Transfer"}
	assert.False(t, q.Matches(e3))

	assert.Equal(t, "sender=alice", q.String())
}

func TestQueryAttributeExists_Matches(t *testing.T) {
	q := QueryAttributeExists{Key: "sender"}

	e1 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "sender", Value: "alice"}},
	}
	assert.True(t, q.Matches(e1))

	e2 := bapitypes.Event{
		Kind:       "Transfer",
		Attributes: []bapitypes.EventAttribute{{Key: "recipient", Value: "bob"}},
	}
	assert.False(t, q.Matches(e2))

	assert.Equal(t, "exists(sender)", q.String())
}
