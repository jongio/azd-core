package azdextutil

import (
	"fmt"
	"sync"
	"time"
)

// RateLimiter implements a token bucket rate limiter for MCP tool calls.
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter creates a rate limiter with the specified max tokens and refill rate.
// For example, NewRateLimiter(10, 1.0) allows 10 burst calls and refills 1 token/second.
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Allow returns true if a request is allowed, consuming one token.
func (r *RateLimiter) Allow() bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(r.lastRefill).Seconds()
	r.tokens += elapsed * r.refillRate
	if r.tokens > r.maxTokens {
		r.tokens = r.maxTokens
	}
	r.lastRefill = now

	if r.tokens >= 1 {
		r.tokens--
		return true
	}
	return false
}

// CheckRateLimit checks the rate limiter and returns an error if the limit is exceeded.
func (r *RateLimiter) CheckRateLimit(toolName string) error {
	if !r.Allow() {
		return fmt.Errorf("rate limit exceeded for tool %q, please wait before retrying", toolName)
	}
	return nil
}
