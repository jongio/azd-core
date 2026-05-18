package healthcheck

import (
	"sync"
	"time"

	"github.com/sony/gobreaker"
	"golang.org/x/time/rate"
)

// CircuitBreakerManager manages per-service circuit breakers.
type CircuitBreakerManager struct {
	breakers        map[string]*gobreaker.CircuitBreaker
	mu              sync.RWMutex
	enabled         bool
	failureThreshold int
	timeout         time.Duration
}

// NewCircuitBreakerManager creates a new CircuitBreakerManager.
func NewCircuitBreakerManager(enabled bool, failureThreshold int, timeout time.Duration) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers:        make(map[string]*gobreaker.CircuitBreaker),
		enabled:         enabled,
		failureThreshold: failureThreshold,
		timeout:         timeout,
	}
}

// GetOrCreate gets or creates a circuit breaker for a service.
func (m *CircuitBreakerManager) GetOrCreate(serviceName string) *gobreaker.CircuitBreaker {
	if m == nil || !m.enabled {
		return nil
	}

	m.mu.RLock()
	breaker, exists := m.breakers[serviceName]
	m.mu.RUnlock()

	if exists {
		return breaker
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if breaker, exists := m.breakers[serviceName]; exists {
		return breaker
	}

	settings := gobreaker.Settings{
		Name:        serviceName,
		MaxRequests: 3,
		Interval:    m.timeout,
		Timeout:     m.timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			if m.failureThreshold < 0 {
				return false
			}
			failureRatio := float64(counts.TotalFailures) / float64(counts.Requests)
			return counts.Requests >= uint32(m.failureThreshold) && failureRatio >= 0.6 //nolint:gosec // G115: safe conversion, failureThreshold is bounded
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			if metricsEnabled.Load() {
				recordCircuitBreakerState(name, to)
			}
		},
	}

	breaker = gobreaker.NewCircuitBreaker(settings)
	m.breakers[serviceName] = breaker
	return breaker
}

// RateLimiterManager manages per-service rate limiters.
type RateLimiterManager struct {
	limiters map[string]*rate.Limiter
	mu       sync.RWMutex
	limit    int
}

// NewRateLimiterManager creates a new RateLimiterManager.
func NewRateLimiterManager(limit int) *RateLimiterManager {
	return &RateLimiterManager{
		limiters: make(map[string]*rate.Limiter),
		limit:    limit,
	}
}

// GetOrCreate gets or creates a rate limiter for a service.
func (m *RateLimiterManager) GetOrCreate(serviceName string) *rate.Limiter {
	if m == nil || m.limit <= 0 {
		return nil
	}

	m.mu.RLock()
	limiter, exists := m.limiters[serviceName]
	m.mu.RUnlock()

	if exists {
		return limiter
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if limiter, exists := m.limiters[serviceName]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(m.limit), m.limit*2)
	m.limiters[serviceName] = limiter

	return limiter
}

// EndpointCache caches discovered health check endpoints per service.
type EndpointCache struct {
	cache map[string]string
	mu    sync.RWMutex
}

// NewEndpointCache creates a new EndpointCache.
func NewEndpointCache() *EndpointCache {
	return &EndpointCache{
		cache: make(map[string]string),
	}
}

// Get returns the cached endpoint for a service key. Returns empty string and false if not cached.
func (c *EndpointCache) Get(key string) (string, bool) {
	if c == nil {
		return "", false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	endpoint, exists := c.cache[key]
	return endpoint, exists
}

// Set stores an endpoint for a service key.
func (c *EndpointCache) Set(key, endpoint string) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = endpoint
}
