package healthcheck

import (
	"sync"

	"golang.org/x/time/rate"
)

// RateLimiterManager encapsulates per-service rate limiter creation.
type RateLimiterManager struct {
	mu        sync.Mutex
	limiters  map[string]*rate.Limiter
	rateLimit int
}

func newRateLimiterManager(rateLimit int) *RateLimiterManager {
	return &RateLimiterManager{
		limiters:  make(map[string]*rate.Limiter),
		rateLimit: rateLimit,
	}
}

// GetOrCreate returns the rate limiter for the given key, creating one if needed.
// Returns nil if rate limiting is disabled (rateLimit <= 0).
func (m *RateLimiterManager) GetOrCreate(key string) *rate.Limiter {
	if m.rateLimit <= 0 {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if lim, ok := m.limiters[key]; ok {
		return lim
	}
	lim := rate.NewLimiter(rate.Limit(m.rateLimit), m.rateLimit)
	m.limiters[key] = lim
	return lim
}
