package healthcheck

import (
	"sync"
	"time"

	"github.com/sony/gobreaker"
)

// CircuitBreakerManager encapsulates circuit breaker creation and state queries.
type CircuitBreakerManager struct {
	mu       sync.Mutex
	breakers map[string]*gobreaker.CircuitBreaker
	enabled  bool
	maxFail  int
	timeout  time.Duration
}

func newCircuitBreakerManager(enabled bool, maxFailures int, timeout time.Duration) *CircuitBreakerManager {
	return &CircuitBreakerManager{
		breakers: make(map[string]*gobreaker.CircuitBreaker),
		enabled:  enabled,
		maxFail:  maxFailures,
		timeout:  timeout,
	}
}

// GetOrCreate returns the circuit breaker for the given key, creating one if needed.
// Returns nil if circuit breaking is disabled.
func (m *CircuitBreakerManager) GetOrCreate(key string) *gobreaker.CircuitBreaker {
	if !m.enabled {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cb, ok := m.breakers[key]; ok {
		return cb
	}
	cb := gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        key,
		MaxRequests: 1,
		Timeout:     m.timeout,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return int(counts.ConsecutiveFailures) >= m.maxFail
		},
	})
	m.breakers[key] = cb
	return cb
}

// State returns the current state of the circuit breaker for the given key.
func (m *CircuitBreakerManager) State(key string) gobreaker.State {
	if !m.enabled {
		return gobreaker.StateClosed
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if cb, ok := m.breakers[key]; ok {
		return cb.State()
	}
	return gobreaker.StateClosed
}
