package healthcheck

import "sync"

// EndpointCache provides thread-safe caching of resolved health-check endpoints.
type EndpointCache struct {
	mu    sync.RWMutex
	cache map[string]string
}

func newEndpointCache() *EndpointCache {
	return &EndpointCache{
		cache: make(map[string]string),
	}
}

// Get returns the cached endpoint for the key and whether it was found.
func (c *EndpointCache) Get(key string) (string, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	v, ok := c.cache[key]
	return v, ok
}

// Set stores a resolved endpoint.
func (c *EndpointCache) Set(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = value
}

// Invalidate removes the cached endpoint for the key.
func (c *EndpointCache) Invalidate(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.cache, key)
}

// SetNone marks a key as having no valid endpoint.
func (c *EndpointCache) SetNone(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache[key] = endpointCacheNone
}

// IsNone returns true if the cached value indicates no valid endpoint.
func (c *EndpointCache) IsNone(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache[key] == endpointCacheNone
}
