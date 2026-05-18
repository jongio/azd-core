// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package fileutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jongio/azd-core/security"
)

// File permissions
const (
	// DirPermission is the default permission for creating directories (rwxr-x---)
	DirPermission = 0750
	// FilePermission is the default permission for creating files (rw-r--r--)
	FilePermission = 0644
)

// atomicWrite writes data to a file atomically via a temp file in the same
// directory, followed by a rename. perm controls the final file permissions.
func atomicWrite(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmpFile, err := os.CreateTemp(dir, filepath.Base(path)+".tmp.*")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer func() { _ = tmpFile.Close() }()

	if _, err := tmpFile.Write(data); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := tmpFile.Sync(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to sync temp file: %w", err)
	}

	if err := tmpFile.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to close temp file: %w", err)
	}

	if err := os.Chmod(tmpPath, perm); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("failed to set file permissions: %w", err)
	}

	if err := renameWithRetry(tmpPath, path); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	return nil
}

// renameWithRetry performs os.Rename with retries and linear backoff to
// mitigate transient rename races on Windows and CI environments.
func renameWithRetry(src, dst string) error {
	var err error
	for attempt := 0; attempt < 5; attempt++ {
		err = os.Rename(src, dst)
		if err == nil {
			return nil
		}
		if attempt < 4 {
			delay := time.Duration(20*(attempt+1)) * time.Millisecond // 20ms, 40ms, 60ms, 80ms
			time.Sleep(delay)
		}
	}
	return fmt.Errorf("failed to rename temp file: %w", err)
}

// AtomicWriteJSON writes data as JSON to a file atomically.
// It writes to a temporary file first, then renames it to the target path.
// This ensures the file is never left in a partial/corrupt state.
func AtomicWriteJSON(path string, data any) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return atomicWrite(path, jsonData, FilePermission)
}

// AtomicWriteFile writes raw bytes to a file atomically.
// It writes to a temporary file first, then renames it to the target path.
// This ensures the file is never left in a partial/corrupt state.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	return atomicWrite(path, data, perm)
}

// ReadJSON reads JSON from a file into the target interface.
// Returns nil error if file doesn't exist (target unchanged).
func ReadJSON(path string, target any) error {
	data, err := os.ReadFile(path) // #nosec G304 -- Path provided by internal callers, not direct user input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil // File doesn't exist, not an error
		}
		return fmt.Errorf("failed to read file: %w", err)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("failed to parse JSON: %w", err)
	}

	return nil
}

// EnsureDir creates a directory if it doesn't exist.
func EnsureDir(path string) error {
	if err := os.MkdirAll(path, DirPermission); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	return nil
}

// FileExists checks if a file exists in a directory.
// Returns true if the file exists, false otherwise.
func FileExists(dir string, filename string) bool {
	_, err := os.Stat(filepath.Join(dir, filename))
	return err == nil
}

// HasFileWithExt checks if any file with the given extension exists in the directory.
// ext should include the dot (e.g., ".csproj")
func HasFileWithExt(dir string, ext string) bool {
	pattern := filepath.Join(dir, "*"+ext)
	matches, _ := filepath.Glob(pattern)
	return len(matches) > 0
}

// ContainsText checks if a file contains the specified text.
// Returns false if file doesn't exist, can't be read, or validation fails.
func ContainsText(filePath string, text string) bool {
	// Validate path before reading
	if err := security.ValidatePath(filePath); err != nil {
		return false
	}

	// #nosec G304 -- Path validated by security.ValidatePath
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false
	}
	return strings.Contains(string(data), text)
}

// FileExistsAny checks if any of the given filenames exist in the directory.
func FileExistsAny(dir string, filenames ...string) bool {
	for _, filename := range filenames {
		if FileExists(dir, filename) {
			return true
		}
	}
	return false
}

// FilesExistAll checks if all of the given filenames exist in the directory.
func FilesExistAll(dir string, filenames ...string) bool {
	for _, filename := range filenames {
		if !FileExists(dir, filename) {
			return false
		}
	}
	return true
}

// ContainsTextInFile checks if file contains text at the specified path.
// Convenience function combining filepath.Join and ContainsText.
// The filename must be a relative path; absolute paths are rejected to
// prevent directory-escape attacks.
func ContainsTextInFile(dir string, filename string, text string) bool {
	if filepath.IsAbs(filename) {
		return false
	}
	joined := filepath.Join(dir, filename)
	// Validate that the resolved path stays within dir
	if _, err := security.ValidatePathWithinBases(joined, dir); err != nil {
		return false
	}
	return ContainsText(joined, text)
}

// HasAnyFileWithExts checks if any file with any of the given extensions exists.
func HasAnyFileWithExts(dir string, exts ...string) bool {
	for _, ext := range exts {
		if HasFileWithExt(dir, ext) {
			return true
		}
	}
	return false
}

// CacheOptions configures cache behavior for LoadCacheJSON.
type CacheOptions struct {
	// TTL is the time-to-live for the cache. If the cache is older than this, it's considered invalid.
	TTL time.Duration
	// Version is used to invalidate the cache when it changes (e.g., app version).
	Version string
}

// CacheMetadata is embedded in cached data to track validity.
type CacheMetadata struct {
	// CachedAt is when the cache was created.
	CachedAt time.Time `json:"cachedAt"`
	// Version is the version string when the cache was created.
	Version string `json:"version,omitempty"`
}

// CacheEntry wraps cached data with metadata.
// Use this structure when saving cache data:
//
//	entry := fileutil.CacheEntry{
//	    Metadata: fileutil.CacheMetadata{CachedAt: time.Now(), Version: "1.0.0"},
//	    Data:     myData,
//	}
//	fileutil.SaveCacheJSON(path, entry)
type CacheEntry struct {
	Metadata CacheMetadata `json:"_cache"`
	Data     any           `json:"data"`
}

// IsCacheValid checks if a cache entry is still valid according to the options.
func (m CacheMetadata) IsCacheValid(opts CacheOptions) bool {
	// Check TTL
	if opts.TTL > 0 && time.Since(m.CachedAt) > opts.TTL {
		return false
	}

	// Check version
	if opts.Version != "" && m.Version != opts.Version {
		return false
	}

	return true
}

// LoadCacheJSON loads a JSON cache file if it exists and is valid.
// Returns:
//   - valid=true if cache was loaded and is valid
//   - valid=false if cache doesn't exist, is expired, or version mismatched
//   - error only for actual read/parse errors (not for missing files)
//
// The target should be a pointer to a CacheEntry or a struct containing CacheMetadata.
//
// Example:
//
//	type MyCache struct {
//	    Metadata fileutil.CacheMetadata `json:"_cache"`
//	    Items    []string               `json:"items"`
//	}
//	var cache MyCache
//	valid, err := fileutil.LoadCacheJSON(path, &cache, fileutil.CacheOptions{TTL: 24*time.Hour})
//	if err != nil {
//	    return err
//	}
//	if !valid {
//	    // Rebuild cache
//	}
func LoadCacheJSON(path string, target any, opts CacheOptions) (valid bool, err error) {
	data, err := os.ReadFile(path) // #nosec G304 -- Cache path constructed internally, not from user input
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil // Cache doesn't exist, not an error
		}
		return false, fmt.Errorf("failed to read cache file: %w", err)
	}

	// Validate metadata first (lightweight partial unmarshal) to avoid
	// deserializing the full target when the cache is expired.
	var metaWrapper struct {
		Cache CacheMetadata `json:"_cache"`
	}
	if err := json.Unmarshal(data, &metaWrapper); err != nil {
		return false, fmt.Errorf("failed to parse cache JSON: %w", err)
	}
	if !metaWrapper.Cache.IsCacheValid(opts) {
		return false, nil // Cache is invalid (expired or version mismatch)
	}

	// Cache is valid - deserialize the full target.
	if err := json.Unmarshal(data, target); err != nil {
		return false, fmt.Errorf("failed to parse cache JSON: %w", err)
	}

	return true, nil
}

// SaveCacheJSON saves data to a JSON cache file with metadata.
// It wraps the data in a CacheEntry with the current timestamp.
//
// Example:
//
//	err := fileutil.SaveCacheJSON(path, myData, "1.0.0")
func SaveCacheJSON(path string, data any, version string) error {
	entry := struct {
		Cache CacheMetadata `json:"_cache"`
		Data  any           `json:"data"`
	}{
		Cache: CacheMetadata{
			CachedAt: time.Now(),
			Version:  version,
		},
		Data: data,
	}

	return AtomicWriteJSON(path, entry)
}

// ClearCache removes a cache file if it exists.
// Returns nil if the file doesn't exist.
func ClearCache(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("failed to clear cache: %w", err)
	}
	return nil
}
