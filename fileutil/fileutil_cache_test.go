// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestIsCacheValid(t *testing.T) {
	tests := []struct {
		name     string
		metadata CacheMetadata
		opts     CacheOptions
		want     bool
	}{
		{
			name: "valid - within TTL",
			metadata: CacheMetadata{
				CachedAt: time.Now().Add(-1 * time.Hour),
				Version:  "1.0.0",
			},
			opts: CacheOptions{TTL: 24 * time.Hour, Version: "1.0.0"},
			want: true,
		},
		{
			name: "invalid - TTL expired",
			metadata: CacheMetadata{
				CachedAt: time.Now().Add(-48 * time.Hour),
				Version:  "1.0.0",
			},
			opts: CacheOptions{TTL: 24 * time.Hour, Version: "1.0.0"},
			want: false,
		},
		{
			name: "invalid - version mismatch",
			metadata: CacheMetadata{
				CachedAt: time.Now(),
				Version:  "1.0.0",
			},
			opts: CacheOptions{TTL: 24 * time.Hour, Version: "2.0.0"},
			want: false,
		},
		{
			name: "valid - no TTL check",
			metadata: CacheMetadata{
				CachedAt: time.Now().Add(-100 * time.Hour),
				Version:  "1.0.0",
			},
			opts: CacheOptions{TTL: 0, Version: "1.0.0"},
			want: true,
		},
		{
			name: "valid - no version check",
			metadata: CacheMetadata{
				CachedAt: time.Now(),
				Version:  "1.0.0",
			},
			opts: CacheOptions{TTL: 24 * time.Hour, Version: ""},
			want: true,
		},
		{
			name: "valid - no checks",
			metadata: CacheMetadata{
				CachedAt: time.Now().Add(-1000 * time.Hour),
				Version:  "",
			},
			opts: CacheOptions{},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.metadata.IsCacheValid(tt.opts)
			if got != tt.want {
				t.Errorf("IsCacheValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestLoadCacheJSON(t *testing.T) {
	t.Run("cache file does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		var target struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}

		valid, err := LoadCacheJSON(path, &target, CacheOptions{TTL: 24 * time.Hour})
		if err != nil {
			t.Errorf("LoadCacheJSON() error = %v, want nil", err)
		}
		if valid {
			t.Error("LoadCacheJSON() valid = true, want false for nonexistent file")
		}
	})

	t.Run("valid cache file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		// Create a valid cache file
		cacheData := struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}{
			Cache: CacheMetadata{CachedAt: time.Now(), Version: "1.0.0"},
			Data:  "test data",
		}
		data, _ := json.Marshal(cacheData)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		var target struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}

		valid, err := LoadCacheJSON(path, &target, CacheOptions{TTL: 24 * time.Hour, Version: "1.0.0"})
		if err != nil {
			t.Errorf("LoadCacheJSON() error = %v, want nil", err)
		}
		if !valid {
			t.Error("LoadCacheJSON() valid = false, want true")
		}
		if target.Data != "test data" {
			t.Errorf("LoadCacheJSON() data = %q, want %q", target.Data, "test data")
		}
	})

	t.Run("expired cache file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		// Create an expired cache file
		cacheData := struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}{
			Cache: CacheMetadata{CachedAt: time.Now().Add(-48 * time.Hour), Version: "1.0.0"},
			Data:  "test data",
		}
		data, _ := json.Marshal(cacheData)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		var target struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}

		valid, err := LoadCacheJSON(path, &target, CacheOptions{TTL: 24 * time.Hour})
		if err != nil {
			t.Errorf("LoadCacheJSON() error = %v, want nil", err)
		}
		if valid {
			t.Error("LoadCacheJSON() valid = true, want false for expired cache")
		}
	})

	t.Run("version mismatch", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		cacheData := struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}{
			Cache: CacheMetadata{CachedAt: time.Now(), Version: "1.0.0"},
			Data:  "test data",
		}
		data, _ := json.Marshal(cacheData)
		if err := os.WriteFile(path, data, 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		var target struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}

		valid, err := LoadCacheJSON(path, &target, CacheOptions{Version: "2.0.0"})
		if err != nil {
			t.Errorf("LoadCacheJSON() error = %v, want nil", err)
		}
		if valid {
			t.Error("LoadCacheJSON() valid = true, want false for version mismatch")
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		if err := os.WriteFile(path, []byte("not json"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		var target struct {
			Cache CacheMetadata `json:"_cache"`
			Data  string        `json:"data"`
		}

		_, err := LoadCacheJSON(path, &target, CacheOptions{})
		if err == nil {
			t.Error("LoadCacheJSON() error = nil, want error for invalid JSON")
		}
	})
}

func TestSaveCacheJSON(t *testing.T) {
	t.Run("saves cache with metadata", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		data := map[string]string{"key": "value"}
		err := SaveCacheJSON(path, data, "1.0.0")
		if err != nil {
			t.Fatalf("SaveCacheJSON() error = %v", err)
		}

		// Read and verify
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read cache file: %v", err)
		}

		var result struct {
			Cache CacheMetadata          `json:"_cache"`
			Data  map[string]interface{} `json:"data"`
		}
		if err := json.Unmarshal(content, &result); err != nil {
			t.Fatalf("Failed to parse cache file: %v", err)
		}

		if result.Cache.Version != "1.0.0" {
			t.Errorf("Version = %q, want %q", result.Cache.Version, "1.0.0")
		}
		if time.Since(result.Cache.CachedAt) > time.Minute {
			t.Errorf("CachedAt is too old: %v", result.Cache.CachedAt)
		}
		if result.Data["key"] != "value" {
			t.Errorf("Data[key] = %q, want %q", result.Data["key"], "value")
		}
	})

	t.Run("requires parent directories to exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "subdir", "nested", "cache.json")

		// Should fail if parent directory doesn't exist
		err := SaveCacheJSON(path, "test", "")
		if err == nil {
			t.Error("SaveCacheJSON() should fail when parent directory doesn't exist")
		}

		// Create parent directories first
		if err := EnsureDir(filepath.Dir(path)); err != nil {
			t.Fatalf("EnsureDir() error = %v", err)
		}

		// Now should succeed
		err = SaveCacheJSON(path, "test", "")
		if err != nil {
			t.Fatalf("SaveCacheJSON() error = %v", err)
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Error("Cache file was not created")
		}
	})
}

func TestClearCache(t *testing.T) {
	t.Run("removes existing file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		// Create file
		if err := os.WriteFile(path, []byte("{}"), 0644); err != nil {
			t.Fatalf("Failed to create test file: %v", err)
		}

		err := ClearCache(path)
		if err != nil {
			t.Errorf("ClearCache() error = %v", err)
		}

		if _, err := os.Stat(path); err == nil {
			t.Error("Cache file still exists after ClearCache()")
		}
	})

	t.Run("no error for nonexistent file", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "nonexistent.json")

		err := ClearCache(path)
		if err != nil {
			t.Errorf("ClearCache() error = %v, want nil for nonexistent file", err)
		}
	})
}

func TestCacheEntry(t *testing.T) {
	t.Run("round trip with CacheEntry", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := filepath.Join(tmpDir, "cache.json")

		// Save using SaveCacheJSON
		originalData := []string{"item1", "item2", "item3"}
		err := SaveCacheJSON(path, originalData, "v1.0")
		if err != nil {
			t.Fatalf("SaveCacheJSON() error = %v", err)
		}

		// Load using LoadCacheJSON
		type MyCache struct {
			Cache CacheMetadata `json:"_cache"`
			Data  []string      `json:"data"`
		}
		var loaded MyCache
		valid, err := LoadCacheJSON(path, &loaded, CacheOptions{Version: "v1.0", TTL: time.Hour})
		if err != nil {
			t.Fatalf("LoadCacheJSON() error = %v", err)
		}
		if !valid {
			t.Error("LoadCacheJSON() valid = false, want true")
		}

		if len(loaded.Data) != 3 {
			t.Errorf("loaded.Data length = %d, want 3", len(loaded.Data))
		}
		if loaded.Cache.Version != "v1.0" {
			t.Errorf("loaded.Cache.Version = %q, want %q", loaded.Cache.Version, "v1.0")
		}
	})
}
