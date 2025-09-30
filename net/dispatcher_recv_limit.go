// Package net provides network communication components for the Asura framework,
// including message routing, dispatching, and transport mechanisms.
package net

import (
	"context"
	"sync/atomic"

	"go.uber.org/ratelimit"
	"golang.org/x/time/rate"
)

// DispatcherRecvLimiter implements a token bucket rate limiting algorithm
// for controlling the rate at which messages are processed by the dispatcher.
// This helps protect the system from overload and ensures consistent performance
// under varying load conditions.
//
// The implementation uses atomic operations to safely update the limiter
// configuration at runtime without causing race conditions.

type DispatcherRecvLimiter struct {
	// limiter holds a pointer to a rate.Limiter from golang.org/x/time/rate
	// which implements the token bucket algorithm
	limiter atomic.Pointer[rate.Limiter]
}

// NewTokenRecvLimiter creates a new token bucket-based rate limiter.
//
// Parameters:
// - limit: The maximum number of requests allowed per second (RPS)
// - burst: The maximum burst size of requests that can be processed at once
//
// Returns:
// A pointer to a new DispatcherRecvLimiter instance, or nil if creation failed
//
// Example usage:
// limiter := NewTokenRecvLimiter(100, 10) // 100 requests per second with a burst of 10

func NewTokenRecvLimiter(limit int, burst int) *DispatcherRecvLimiter {
	limiter := rate.NewLimiter(rate.Limit(limit), burst)
	if limiter == nil {
		return nil
	}
	self := &DispatcherRecvLimiter{}
	self.limiter.Store(limiter)
	return self
}

// Take blocks until a token is available, ensuring that the rate limit is respected.
// This method should be called before processing each message to enforce rate limiting.
//
// Returns:
// An error if the context is canceled, or nil if a token was successfully acquired

func (l *DispatcherRecvLimiter) Take() error {
	return l.limiter.Load().Wait(context.Background())
}

// Reload updates the rate limiter configuration at runtime.
// This allows for dynamic adjustment of rate limits without restarting the service.
//
// Parameters:
// - limit: The new maximum number of requests allowed per second (RPS)
// - burst: The new maximum burst size of requests

func (l *DispatcherRecvLimiter) Reload(limit int, burst int) {
	limiter := rate.NewLimiter(rate.Limit(limit), burst)
	if limiter == nil {
		return
	}
	l.limiter.Store(limiter)
}

// recvLimiterFilter implements a dispatcher filter that applies rate limiting
// to incoming messages. It serves as an integration point between the rate limiter
// and the dispatcher's filter chain.
//
// Parameters:
// - d: A pointer to the DispatcherDelivery containing the message to filter
// - f: The next filter handle function in the chain
//
// Returns:
// An error if rate limiting fails, or the result of the next filter in the chain

func (l *DispatcherRecvLimiter) recvLimiterFilter(d *DispatcherDelivery, f DispatcherFilterHandleFunc) error {
	if err := l.Take(); err != nil {
		return err
	}
	return f(d)
}

// FunnelRecvLimiter implements a leaky bucket rate limiting algorithm
// using Uber's ratelimit package. This alternative rate limiting strategy
// provides more deterministic behavior for controlling request rates.
//
// Similar to DispatcherRecvLimiter, it uses atomic operations for thread-safety.

type FunnelRecvLimiter struct {
	// limiter holds a pointer to a ratelimit.Limiter from go.uber.org/ratelimit
	// which implements the leaky bucket algorithm
	limiter atomic.Pointer[ratelimit.Limiter]
}

// NewFunnelRecvLimiter creates a new leaky bucket-based rate limiter.
//
// Parameters:
// - limit: The maximum number of requests allowed per second (RPS)
//
// Returns:
// A pointer to a new FunnelRecvLimiter instance, or nil if creation failed

func NewFunnelRecvLimiter(limit int) *FunnelRecvLimiter {
	limiter := ratelimit.New(limit)
	if limiter == nil {
		return nil
	}
	self := &FunnelRecvLimiter{}
	self.limiter.Store(&limiter)
	return self
}

// Take blocks until the rate limit allows the next request to be processed.
// This method is similar to DispatcherRecvLimiter.Take but uses the leaky bucket algorithm.

func (l *FunnelRecvLimiter) Take() {
	_ = (*l.limiter.Load()).Take()
}

// Reload updates the leaky bucket rate limiter configuration at runtime.
//
// Parameters:
// - limit: The new maximum number of requests allowed per second (RPS)

func (l *FunnelRecvLimiter) Reload(limit int) {
	limiter := ratelimit.New(limit)
	if limiter == nil {
		return
	}
	l.limiter.Store(&limiter)
}
