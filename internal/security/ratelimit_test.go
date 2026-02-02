package security

import (
	"sync"
	"testing"
	"time"

	"github.com/blockberries/blockberry/pkg/abi"
)

func TestTokenBucketLimiter_Allow(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0, // Disable cleanup for testing
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	// Should allow up to burst size
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test") {
			t.Errorf("Allow() returned false on request %d, expected true", i+1)
		}
	}

	// Should reject after burst is exhausted
	if limiter.Allow("test") {
		t.Error("Allow() returned true after burst exhausted, expected false")
	}
}

func TestTokenBucketLimiter_AllowN(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	// Should allow bulk requests within burst
	if !limiter.AllowN("test", 5) {
		t.Error("AllowN(5) returned false, expected true")
	}

	if !limiter.AllowN("test", 5) {
		t.Error("AllowN(5) second call returned false, expected true")
	}

	// Should reject when burst exceeded
	if limiter.AllowN("test", 1) {
		t.Error("AllowN(1) returned true after burst exhausted")
	}
}

func TestTokenBucketLimiter_Refill(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            100, // 100 per second = 10 per 100ms
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	// Exhaust burst
	for i := 0; i < 10; i++ {
		limiter.Allow("test")
	}

	// Wait for refill (100ms should give ~10 tokens at 100/s)
	time.Sleep(150 * time.Millisecond)

	// Should be able to make requests again
	if !limiter.Allow("test") {
		t.Error("Allow() returned false after refill, expected true")
	}
}

func TestTokenBucketLimiter_Reset(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	// Exhaust burst
	for i := 0; i < 10; i++ {
		limiter.Allow("test")
	}

	// Reset
	limiter.Reset("test")

	// Should have full burst again
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test") {
			t.Errorf("Allow() returned false after reset on request %d", i+1)
		}
	}
}

func TestTokenBucketLimiter_MultipleKeys(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           5,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	// Keys should be independent
	for i := 0; i < 5; i++ {
		if !limiter.Allow("key1") {
			t.Errorf("Allow(key1) failed on request %d", i+1)
		}
		if !limiter.Allow("key2") {
			t.Errorf("Allow(key2) failed on request %d", i+1)
		}
	}

	// Both should be exhausted independently
	if limiter.Allow("key1") {
		t.Error("Allow(key1) should be exhausted")
	}
	if limiter.Allow("key2") {
		t.Error("Allow(key2) should be exhausted")
	}
}

func TestTokenBucketLimiter_Concurrent(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            1000,
		Interval:        time.Second,
		Burst:           100,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	var wg sync.WaitGroup
	allowed := make(chan bool, 200)

	// Concurrent requests
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			allowed <- limiter.Allow("concurrent")
		}()
	}

	wg.Wait()
	close(allowed)

	// Count allowed requests
	count := 0
	for a := range allowed {
		if a {
			count++
		}
	}

	// Should allow approximately burst size
	if count > 110 { // Allow some slack for timing
		t.Errorf("Allowed %d requests, expected ~100", count)
	}
	if count < 90 {
		t.Errorf("Allowed only %d requests, expected ~100", count)
	}
}

func TestTokenBucketLimiter_Size(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewTokenBucketLimiter(cfg)
	defer limiter.Close()

	if limiter.Size() != 0 {
		t.Errorf("Size() = %d, want 0", limiter.Size())
	}

	limiter.Allow("key1")
	limiter.Allow("key2")
	limiter.Allow("key3")

	if limiter.Size() != 3 {
		t.Errorf("Size() = %d, want 3", limiter.Size())
	}

	limiter.Reset("key1")
	if limiter.Size() != 2 {
		t.Errorf("Size() = %d after reset, want 2", limiter.Size())
	}
}

func TestConnectionTracker_Basic(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         10,
		MaxInboundPeers:  7,
		MaxOutboundPeers: 3,
	}

	tracker := NewConnectionTracker(limits)

	// Initially should allow connections
	if !tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = false initially, want true")
	}
	if !tracker.CanDialOutbound() {
		t.Error("CanDialOutbound() = false initially, want true")
	}

	if tracker.TotalCount() != 0 {
		t.Errorf("TotalCount() = %d, want 0", tracker.TotalCount())
	}
}

func TestConnectionTracker_InboundLimit(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         10,
		MaxInboundPeers:  3,
		MaxOutboundPeers: 7,
	}

	tracker := NewConnectionTracker(limits)

	// Add inbound connections up to limit
	for i := 0; i < 3; i++ {
		tracker.OnConnect([]byte{byte(i)}, true)
	}

	if tracker.InboundCount() != 3 {
		t.Errorf("InboundCount() = %d, want 3", tracker.InboundCount())
	}

	// Should not accept more inbound
	if tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = true at limit, want false")
	}

	// Should still accept outbound
	if !tracker.CanDialOutbound() {
		t.Error("CanDialOutbound() = false, want true")
	}
}

func TestConnectionTracker_OutboundLimit(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         10,
		MaxInboundPeers:  7,
		MaxOutboundPeers: 3,
	}

	tracker := NewConnectionTracker(limits)

	// Add outbound connections up to limit
	for i := 0; i < 3; i++ {
		tracker.OnConnect([]byte{byte(i)}, false)
	}

	if tracker.OutboundCount() != 3 {
		t.Errorf("OutboundCount() = %d, want 3", tracker.OutboundCount())
	}

	// Should not dial more outbound
	if tracker.CanDialOutbound() {
		t.Error("CanDialOutbound() = true at limit, want false")
	}

	// Should still accept inbound
	if !tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = false, want true")
	}
}

func TestConnectionTracker_TotalLimit(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         5,
		MaxInboundPeers:  4,
		MaxOutboundPeers: 4,
	}

	tracker := NewConnectionTracker(limits)

	// Add 3 inbound + 2 outbound = 5 total
	for i := 0; i < 3; i++ {
		tracker.OnConnect([]byte{byte(i)}, true)
	}
	for i := 3; i < 5; i++ {
		tracker.OnConnect([]byte{byte(i)}, false)
	}

	if tracker.TotalCount() != 5 {
		t.Errorf("TotalCount() = %d, want 5", tracker.TotalCount())
	}

	// Should not accept any more connections
	if tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = true at total limit")
	}
	if tracker.CanDialOutbound() {
		t.Error("CanDialOutbound() = true at total limit")
	}
}

func TestConnectionTracker_Disconnect(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         5,
		MaxInboundPeers:  3,
		MaxOutboundPeers: 2,
	}

	tracker := NewConnectionTracker(limits)

	// Fill up inbound
	for i := 0; i < 3; i++ {
		tracker.OnConnect([]byte{byte(i)}, true)
	}

	if tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = true at limit")
	}

	// Disconnect one
	tracker.OnDisconnect([]byte{1})

	if tracker.InboundCount() != 2 {
		t.Errorf("InboundCount() = %d after disconnect, want 2", tracker.InboundCount())
	}

	// Should accept inbound again
	if !tracker.CanAcceptInbound() {
		t.Error("CanAcceptInbound() = false after disconnect, want true")
	}
}

func TestConnectionTracker_Concurrent(t *testing.T) {
	limits := abi.ResourceLimits{
		MaxPeers:         100,
		MaxInboundPeers:  50,
		MaxOutboundPeers: 50,
	}

	tracker := NewConnectionTracker(limits)

	var wg sync.WaitGroup

	// Concurrent connects
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.OnConnect([]byte{byte(id)}, id%2 == 0)
		}(i)
	}

	wg.Wait()

	if tracker.TotalCount() != 100 {
		t.Errorf("TotalCount() = %d after concurrent connects, want 100", tracker.TotalCount())
	}

	// Concurrent disconnects
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			tracker.OnDisconnect([]byte{byte(id)})
		}(i)
	}

	wg.Wait()

	if tracker.TotalCount() != 50 {
		t.Errorf("TotalCount() = %d after concurrent disconnects, want 50", tracker.TotalCount())
	}
}

func TestSlidingWindowLimiter_Allow(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewSlidingWindowLimiter(cfg)
	defer limiter.Close()

	// Should allow up to rate limit
	for i := 0; i < 10; i++ {
		if !limiter.Allow("test") {
			t.Errorf("Allow() returned false on request %d, expected true", i+1)
		}
	}

	// Should reject after rate limit
	if limiter.Allow("test") {
		t.Error("Allow() returned true after rate exhausted")
	}
}

func TestSlidingWindowLimiter_Window(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        100 * time.Millisecond, // Short interval for testing
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewSlidingWindowLimiter(cfg)
	defer limiter.Close()

	// Exhaust rate
	for i := 0; i < 10; i++ {
		limiter.Allow("test")
	}

	// Wait for window to slide
	time.Sleep(150 * time.Millisecond)

	// Should allow again
	if !limiter.Allow("test") {
		t.Error("Allow() returned false after window slide")
	}
}

func TestSlidingWindowLimiter_Reset(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            5,
		Interval:        time.Second,
		Burst:           5,
		CleanupInterval: 0,
	}

	limiter := NewSlidingWindowLimiter(cfg)
	defer limiter.Close()

	// Exhaust rate
	for i := 0; i < 5; i++ {
		limiter.Allow("test")
	}

	// Reset
	limiter.Reset("test")

	// Should allow again
	for i := 0; i < 5; i++ {
		if !limiter.Allow("test") {
			t.Errorf("Allow() returned false after reset on request %d", i+1)
		}
	}
}

func TestSlidingWindowLimiter_Size(t *testing.T) {
	cfg := abi.RateLimiterConfig{
		Rate:            10,
		Interval:        time.Second,
		Burst:           10,
		CleanupInterval: 0,
	}

	limiter := NewSlidingWindowLimiter(cfg)
	defer limiter.Close()

	if limiter.Size() != 0 {
		t.Errorf("Size() = %d, want 0", limiter.Size())
	}

	limiter.Allow("key1")
	limiter.Allow("key2")

	if limiter.Size() != 2 {
		t.Errorf("Size() = %d, want 2", limiter.Size())
	}
}
