// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cache

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{
		Dir:     dir,
		TTL:     time.Hour,
		Version: "1.0",
	})

	if m == nil {
		t.Fatal("NewManager() returned nil")
	}
	if m.dir != dir {
		t.Errorf("dir = %q, want %q", m.dir, dir)
	}
	if m.ttl != time.Hour {
		t.Errorf("ttl = %v, want %v", m.ttl, time.Hour)
	}
	if m.version != "1.0" {
		t.Errorf("version = %q, want %q", m.version, "1.0")
	}
}

func TestSetAndGetRoundtrip(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour, Version: "1.0"})

	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	input := payload{Name: "test", Count: 42}
	if err := m.Set("mykey", input); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var output payload
	ok, err := m.Get("mykey", &output)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatal("Get() returned false, want true")
	}
	if output.Name != input.Name || output.Count != input.Count {
		t.Errorf("Get() = %+v, want %+v", output, input)
	}
}

func TestGetMissingKey(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	var result string
	ok, err := m.Get("nonexistent", &result)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Error("Get() returned true for missing key, want false")
	}
}

func TestGetExpiredEntry(t *testing.T) {
	dir := t.TempDir()
	// Use a very short TTL
	m := NewManager(Options{Dir: dir, TTL: 1 * time.Millisecond, Version: "1.0"})

	if err := m.Set("expire-me", "hello"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Poll until TTL expires instead of sleeping a fixed duration
	deadline := time.Now().Add(100 * time.Millisecond)
	for time.Now().Before(deadline) {
		var probe string
		ok, _ := m.Get("expire-me", &probe)
		if !ok {
			break
		}
		runtime.Gosched()
	}

	var result string
	ok, err := m.Get("expire-me", &result)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Error("Get() returned true for expired entry, want false")
	}
}

func TestGetWrongVersion(t *testing.T) {
	dir := t.TempDir()

	// Write with version "1.0"
	m1 := NewManager(Options{Dir: dir, TTL: time.Hour, Version: "1.0"})
	if err := m1.Set("versioned", "data"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Read with version "2.0"
	m2 := NewManager(Options{Dir: dir, TTL: time.Hour, Version: "2.0"})
	var result string
	ok, err := m2.Get("versioned", &result)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Error("Get() returned true for wrong version, want false")
	}
}

func TestInvalidate(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	if err := m.Set("remove-me", "value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	if err := m.Invalidate("remove-me"); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}

	var result string
	ok, err := m.Get("remove-me", &result)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if ok {
		t.Error("Get() returned true after Invalidate, want false")
	}
}

func TestInvalidateNonexistent(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	// Should not error for missing key
	if err := m.Invalidate("does-not-exist"); err != nil {
		t.Fatalf("Invalidate() error = %v", err)
	}
}

func TestClear(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	// Set multiple entries
	for _, key := range []string{"a", "b", "c"} {
		if err := m.Set(key, key+"_value"); err != nil {
			t.Fatalf("Set(%q) error = %v", key, err)
		}
	}

	if err := m.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}

	// All entries should be gone
	for _, key := range []string{"a", "b", "c"} {
		var result string
		ok, err := m.Get(key, &result)
		if err != nil {
			t.Fatalf("Get(%q) error = %v", key, err)
		}
		if ok {
			t.Errorf("Get(%q) returned true after Clear, want false", key)
		}
	}
}

func TestClearNonexistentDir(t *testing.T) {
	m := NewManager(Options{Dir: filepath.Join(t.TempDir(), "nonexistent"), TTL: time.Hour})
	// Should not error
	if err := m.Clear(); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
}

func TestGetStats(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	// Initial stats
	stats := m.GetStats()
	if stats.Hits != 0 || stats.Misses != 0 || stats.Errors != 0 {
		t.Errorf("initial stats = %+v, want all zeros", stats)
	}

	// Miss
	var s string
	_, _ = m.Get("missing", &s)
	stats = m.GetStats()
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}

	// Set then Hit
	_ = m.Set("exists", "val")
	_, _ = m.Get("exists", &s)
	stats = m.GetStats()
	if stats.Hits != 1 {
		t.Errorf("Hits = %d, want 1", stats.Hits)
	}
	if stats.Misses != 1 {
		t.Errorf("Misses = %d, want 1", stats.Misses)
	}
}

func TestHashFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.txt")
	if err := os.WriteFile(f, []byte("hello world"), 0o644); err != nil {
		t.Fatal(err)
	}

	hash1, err := HashFile(f)
	if err != nil {
		t.Fatalf("HashFile() error = %v", err)
	}
	if hash1 == "" {
		t.Fatal("HashFile() returned empty string")
	}
	// SHA256 hex string should be 64 chars
	if len(hash1) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash1))
	}

	// Same file -> same hash
	hash2, err := HashFile(f)
	if err != nil {
		t.Fatalf("HashFile() error = %v", err)
	}
	if hash1 != hash2 {
		t.Errorf("HashFile() not deterministic: %q != %q", hash1, hash2)
	}

	// Different content -> different hash
	f2 := filepath.Join(dir, "test2.txt")
	if err := os.WriteFile(f2, []byte("different"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash3, err := HashFile(f2)
	if err != nil {
		t.Fatalf("HashFile() error = %v", err)
	}
	if hash1 == hash3 {
		t.Error("HashFile() same hash for different content")
	}
}

func TestHashFileNonexistent(t *testing.T) {
	_, err := HashFile(filepath.Join(t.TempDir(), "nope.txt"))
	if err == nil {
		t.Error("HashFile() expected error for nonexistent file")
	}
}

func TestConcurrentGetSet(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour, Version: "1.0"})

	const goroutines = 20
	const iterations = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				key := "key"
				if id%2 == 0 {
					_ = m.Set(key, map[string]int{"id": id, "iter": i})
				} else {
					var result map[string]int
					_, _ = m.Get(key, &result)
				}
			}
		}(g)
	}

	wg.Wait()

	// Verify stats are consistent (no negative numbers, totals make sense)
	stats := m.GetStats()
	if stats.Hits < 0 || stats.Misses < 0 || stats.Errors < 0 {
		t.Errorf("negative stats: %+v", stats)
	}
}

func TestGetInvalidJSON(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	// Write a file with invalid JSON content
	keyPath := m.keyPath("bad-json")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	if err := os.WriteFile(keyPath, []byte("not valid json!!!"), 0o644); err != nil {
		t.Fatalf("failed to write bad json: %v", err)
	}

	var result string
	ok, err := m.Get("bad-json", &result)
	if ok {
		t.Error("Get() returned true for invalid JSON, want false")
	}
	if err == nil {
		t.Error("Get() expected error for invalid JSON")
	}

	stats := m.GetStats()
	if stats.Errors != 1 {
		t.Errorf("Errors = %d, want 1", stats.Errors)
	}
}

func TestGetInvalidDataField(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(Options{Dir: dir, TTL: time.Hour})

	// Write a valid envelope but with data that can't unmarshal to the target type
	keyPath := m.keyPath("bad-data")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("failed to create dir: %v", err)
	}
	envelope := `{"_cache":{"cachedAt":"` + time.Now().Format(time.RFC3339) + `","version":""},"data":"not-an-object"}`
	if err := os.WriteFile(keyPath, []byte(envelope), 0o644); err != nil {
		t.Fatalf("failed to write bad data: %v", err)
	}

	var result map[string]int
	ok, err := m.Get("bad-data", &result)
	if ok {
		t.Error("Get() returned true for bad data field, want false")
	}
	if err == nil {
		t.Error("Get() expected error for unmarshal failure")
	}

	stats := m.GetStats()
	if stats.Errors < 1 {
		t.Errorf("Errors = %d, want >= 1", stats.Errors)
	}
}

func TestSanitizeKey(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"simple", "simple"},
		{"with spaces", "with_spaces"},
		{"path/to/thing", "path_to_thing"},
		{"special!@#chars", "special___chars"},
		{"dots.and-dashes", "dots.and-dashes"},
	}
	for _, tt := range tests {
		got := sanitizeKey(tt.input)
		if got != tt.want {
			t.Errorf("sanitizeKey(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
