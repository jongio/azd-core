// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/jongio/azd-core/fileutil"
)

// Options configures a cache Manager.
type Options struct {
	Dir     string        // Directory to store cache files
	TTL     time.Duration // Time-to-live for cache entries
	Version string        // Cache version (entries with different version are invalidated)
}

// Stats tracks cache hit/miss statistics.
type Stats struct {
	Hits   int
	Misses int
	Errors int
}

// cacheEnvelope is the on-disk format wrapping cached data with metadata.
type cacheEnvelope struct {
	Metadata fileutil.CacheMetadata `json:"_cache"`
	Data     json.RawMessage        `json:"data"`
}

var keySanitizer = regexp.MustCompile(`[^a-zA-Z0-9_\-.]`)

// Manager provides thread-safe file-based caching with TTL and version support.
type Manager struct {
	dir     string
	ttl     time.Duration
	version string
	mu      sync.RWMutex
	statsMu sync.Mutex
	stats   Stats
}

// NewManager creates a new cache manager.
func NewManager(opts Options) *Manager {
	return &Manager{
		dir:     opts.Dir,
		ttl:     opts.TTL,
		version: opts.Version,
	}
}

// Get loads a cached value by key. Returns true if cache is valid.
// target must be a pointer to the type to unmarshal into.
func (m *Manager) Get(key string, target interface{}) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	path := m.keyPath(key)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			m.recordMiss()
			return false, nil
		}
		m.recordError()
		return false, fmt.Errorf("failed to read cache file: %w", err)
	}

	var env cacheEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		m.recordError()
		return false, fmt.Errorf("failed to parse cache file: %w", err)
	}

	// Check version
	if m.version != "" && env.Metadata.Version != m.version {
		m.recordMiss()
		return false, nil
	}

	// Check TTL
	if m.ttl > 0 && time.Since(env.Metadata.CachedAt) > m.ttl {
		m.recordMiss()
		return false, nil
	}

	if err := json.Unmarshal(env.Data, target); err != nil {
		m.recordError()
		return false, fmt.Errorf("failed to unmarshal cached data: %w", err)
	}

	m.recordHit()
	return true, nil
}

// Set stores a value in the cache.
func (m *Manager) Set(key string, data interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := fileutil.EnsureDir(m.dir); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	rawData, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	env := cacheEnvelope{
		Metadata: fileutil.CacheMetadata{
			CachedAt: time.Now(),
			Version:  m.version,
		},
		Data: rawData,
	}

	path := m.keyPath(key)
	return fileutil.AtomicWriteJSON(path, env)
}

// Invalidate removes a specific cache entry.
func (m *Manager) Invalidate(key string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	path := m.keyPath(key)
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove cache entry: %w", err)
	}
	return nil
}

// Clear removes all cache entries in the cache directory.
func (m *Manager) Clear() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		path := filepath.Join(m.dir, entry.Name())
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to remove cache file %s: %w", entry.Name(), err)
		}
	}

	return nil
}

// GetStats returns cache hit/miss statistics.
func (m *Manager) GetStats() Stats {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	return m.stats
}

// HashFile computes SHA256 hash of a file for cache invalidation.
func HashFile(path string) (string, error) {
	// #nosec G304 -- caller controls the path
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to hash file: %w", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// sanitizeKey replaces non-alphanumeric characters in a cache key.
func sanitizeKey(key string) string {
	return keySanitizer.ReplaceAllString(key, "_")
}

// keyPath returns the file path for a cache key.
func (m *Manager) keyPath(key string) string {
	return filepath.Join(m.dir, sanitizeKey(key)+".json")
}

func (m *Manager) recordHit() {
	m.statsMu.Lock()
	m.stats.Hits++
	m.statsMu.Unlock()
}

func (m *Manager) recordMiss() {
	m.statsMu.Lock()
	m.stats.Misses++
	m.statsMu.Unlock()
}

func (m *Manager) recordError() {
	m.statsMu.Lock()
	m.stats.Errors++
	m.statsMu.Unlock()
}
