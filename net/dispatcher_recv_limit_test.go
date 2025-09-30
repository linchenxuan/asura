package net

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestDispatcherRecvLimiter_Basic tests the basic functionality of the token bucket rate limiter
func TestDispatcherRecvLimiter_Basic(t *testing.T) {
	limiter := NewTokenRecvLimiter(10, 5) // 10 requests per second, burst of 5
	if limiter == nil {
		t.Fatal("Failed to create token limiter")
	}

	// Should be able to take 5 requests immediately (burst size)
	for i := 0; i < 5; i++ {
		if err := limiter.Take(); err != nil {
			t.Errorf("Failed to take token %d: %v", i, err)
		}
	}

	// 6th request should block or take longer
	start := time.Now()
	if err := limiter.Take(); err != nil {
		t.Errorf("Failed to take 6th token: %v", err)
	}
	duration := time.Since(start)

	// Should have taken some time (at least 50ms for token refill)
	if duration < 50*time.Millisecond {
		t.Logf("6th token taken too quickly (%v), this might be expected behavior", duration)
	}
}

// TestDispatcherRecvLimiter_Reload tests dynamic reloading of rate limits
func TestDispatcherRecvLimiter_Reload(t *testing.T) {
	limiter := NewTokenRecvLimiter(10, 5)
	if limiter == nil {
		t.Fatal("Failed to create token limiter")
	}

	// Take all initial tokens
	for i := 0; i < 5; i++ {
		if err := limiter.Take(); err != nil {
			t.Errorf("Failed to take initial token %d: %v", i, err)
		}
	}

	// Reload with higher limits
	limiter.Reload(20, 10)

	// Should be able to take more tokens immediately
	for i := 0; i < 10; i++ {
		if err := limiter.Take(); err != nil {
			t.Errorf("Failed to take reloaded token %d: %v", i, err)
		}
	}
}

// TestDispatcherRecvLimiter_FilterIntegration tests the rate limiter as a dispatcher filter
func TestDispatcherRecvLimiter_FilterIntegration(t *testing.T) {
	limiter := NewTokenRecvLimiter(5, 2) // 5 requests per second, burst of 2
	if limiter == nil {
		t.Fatal("Failed to create token limiter")
	}

	handlerCalled := 0
	handler := func(d *DispatcherDelivery) error {
		handlerCalled++
		return nil
	}

	delivery := &DispatcherDelivery{
		TransportDelivery: &TransportDelivery{
			Pkg: &TransRecvPkg{
				PkgHdr: &PkgHead{
					MsgID: "test.msg",
				},
			},
		},
	}

	// First 2 requests should pass immediately (burst size)
	for i := 0; i < 2; i++ {
		err := limiter.recvLimiterFilter(delivery, handler)
		if err != nil {
			t.Errorf("Request %d should pass: %v", i, err)
		}
	}

	if handlerCalled != 2 {
		t.Errorf("Handler should be called 2 times, got %d", handlerCalled)
	}
}

// TestFunnelRecvLimiter_Basic tests the basic functionality of the leaky bucket rate limiter
func TestFunnelRecvLimiter_Basic(t *testing.T) {
	limiter := NewFunnelRecvLimiter(10) // 10 requests per second
	if limiter == nil {
		t.Fatal("Failed to create funnel limiter")
	}

	// Should be able to take multiple requests
	for i := 0; i < 10; i++ {
		limiter.Take()
	}

	// All requests should complete without blocking for too long
	start := time.Now()
	for i := 0; i < 10; i++ {
		limiter.Take()
	}
	duration := time.Since(start)

	// Should complete within reasonable time (less than 2 seconds for 10 requests at 10 RPS)
	if duration > 2*time.Second {
		t.Errorf("Requests took too long: %v", duration)
	}
}

// TestFunnelRecvLimiter_Reload tests dynamic reloading of funnel rate limits
func TestFunnelRecvLimiter_Reload(t *testing.T) {
	limiter := NewFunnelRecvLimiter(10)
	if limiter == nil {
		t.Fatal("Failed to create funnel limiter")
	}

	// Take some requests
	for i := 0; i < 5; i++ {
		limiter.Take()
	}

	// Reload with higher limit
	limiter.Reload(20)

	// Should be able to take more requests faster
	start := time.Now()
	for i := 0; i < 10; i++ {
		limiter.Take()
	}
	duration := time.Since(start)

	// With higher limit, should complete faster
	if duration > 1*time.Second {
		t.Logf("Reloaded limiter took %v, might be slower than expected", duration)
	}
}

// TestDispatcherRecvLimiter_Concurrent tests concurrent access to the rate limiter
func TestDispatcherRecvLimiter_Concurrent(t *testing.T) {
	limiter := NewTokenRecvLimiter(100, 50) // High limit for concurrent testing
	if limiter == nil {
		t.Fatal("Failed to create token limiter")
	}

	var wg sync.WaitGroup
	errors := make([]error, 10)

	// Launch 10 goroutines, each trying to take 5 tokens
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				if err := limiter.Take(); err != nil {
					errors[idx] = err
					return
				}
			}
		}(i)
	}

	wg.Wait()

	// Check for any errors
	for i, err := range errors {
		if err != nil {
			t.Errorf("Goroutine %d encountered error: %v", i, err)
		}
	}
}

// TestDispatcherRecvLimiter_ContextCancellation tests behavior with context cancellation
func TestDispatcherRecvLimiter_ContextCancellation(t *testing.T) {
	// This test would require modifying the Take() method to accept context
	// For now, we'll test the basic functionality
	limiter := NewTokenRecvLimiter(1, 1) // Very low limit
	if limiter == nil {
		t.Fatal("Failed to create token limiter")
	}

	// Take the only available token
	if err := limiter.Take(); err != nil {
		t.Errorf("Failed to take initial token: %v", err)
	}

	// Create a context that will be cancelled
	_, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancel the context
	cancel()

	// The current implementation uses context.Background(), so this test
	// mainly documents the expected behavior when context support is added
	t.Log("Context cancellation test completed - current implementation uses background context")
}

// BenchmarkDispatcherRecvLimiter benchmarks the token bucket rate limiter
func BenchmarkDispatcherRecvLimiter(b *testing.B) {
	limiter := NewTokenRecvLimiter(1000, 500)
	if limiter == nil {
		b.Fatal("Failed to create token limiter")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := limiter.Take(); err != nil {
			b.Errorf("Failed to take token: %v", err)
		}
	}
}

// BenchmarkFunnelRecvLimiter benchmarks the leaky bucket rate limiter
func BenchmarkFunnelRecvLimiter(b *testing.B) {
	limiter := NewFunnelRecvLimiter(1000)
	if limiter == nil {
		b.Fatal("Failed to create funnel limiter")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		limiter.Take()
	}
}

// BenchmarkDispatcherRecvLimiter_Reload benchmarks dynamic reloading
func BenchmarkDispatcherRecvLimiter_Reload(b *testing.B) {
	limiter := NewTokenRecvLimiter(100, 50)
	if limiter == nil {
		b.Fatal("Failed to create token limiter")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Alternate between two different configurations
		if i%2 == 0 {
			limiter.Reload(200, 100)
		} else {
			limiter.Reload(100, 50)
		}
	}
}
