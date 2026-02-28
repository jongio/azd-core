package azdextutil

import (
	"fmt"

	"golang.org/x/time/rate"
)

// RateLimiter wraps golang.org/x/time/rate to provide token-bucket
// rate limiting for MCP tool calls.
type RateLimiter struct {
	limiter *rate.Limiter
}

// NewRateLimiter creates a rate limiter with the specified burst size and refill rate.
// For example, NewRateLimiter(10, 1.0) allows 10 burst calls and refills 1 token/second.
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		limiter: rate.NewLimiter(rate.Limit(refillRate), int(maxTokens)),
	}
}

// Allow returns true if a request is allowed, consuming one token.
func (r *RateLimiter) Allow() bool {
	return r.limiter.Allow()
}

// CheckRateLimit checks the rate limiter and returns an error if the limit is exceeded.
func (r *RateLimiter) CheckRateLimit(toolName string) error {
	if !r.limiter.Allow() {
		return fmt.Errorf("rate limit exceeded for tool %q, please wait before retrying", toolName)
	}
	return nil
}
