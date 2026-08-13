// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package fileutil

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestAtomicWriteFile_NonExistentDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodir", "deep", "file.txt")
	err := AtomicWriteFile(path, []byte("hello"), 0o644)
	if err == nil {
		t.Fatal("expected error writing to non-existent directory, got nil")
	}
}

func TestAtomicWriteFile_LargePayload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "large.bin")
	data := make([]byte, 2*1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := AtomicWriteFile(path, data, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() large payload error: %v", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(got) != len(data) {
		t.Fatalf("expected %d bytes, got %d", len(data), len(got))
	}
}

func TestAtomicWriteFile_Overwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overwrite.txt")
	if err := AtomicWriteFile(path, []byte("first"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AtomicWriteFile(path, []byte("second"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "second" {
		t.Fatalf("expected 'second', got %q", string(got))
	}
}

func TestAtomicWriteFile_EmptyData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "empty.txt")
	if err := AtomicWriteFile(path, []byte{}, 0o644); err != nil {
		t.Fatalf("AtomicWriteFile() empty data error: %v", err)
	}
	got, _ := os.ReadFile(path)
	if len(got) != 0 {
		t.Fatalf("expected empty file, got %d bytes", len(got))
	}
}

func TestAtomicWriteJSON_NestedStruct(t *testing.T) {
	type Inner struct {
		Value int `json:"value"`
	}
	type Outer struct {
		Name  string `json:"name"`
		Inner Inner  `json:"inner"`
	}
	path := filepath.Join(t.TempDir(), "nested.json")
	data := Outer{Name: "test", Inner: Inner{Value: 42}}
	if err := AtomicWriteJSON(path, data); err != nil {
		t.Fatalf("AtomicWriteJSON() error: %v", err)
	}
	raw, _ := os.ReadFile(path)
	var result Outer
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if result.Name != "test" || result.Inner.Value != 42 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestAtomicWriteJSON_InvalidMarshal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "invalid.json")
	err := AtomicWriteJSON(path, make(chan int))
	if err == nil {
		t.Fatal("expected error for unmarshalable type, got nil")
	}
	if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
		t.Error("file should not exist after marshal failure")
	}
}

func TestAtomicWriteFile_ConcurrentWrites(t *testing.T) {
	dir := t.TempDir()
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			path := filepath.Join(dir, "concurrent.txt")
			data := []byte("writer-" + string(rune('A'+n)))
			_ = AtomicWriteFile(path, data, 0o644)
		}(i)
	}
	wg.Wait()
	got, err := os.ReadFile(filepath.Join(dir, "concurrent.txt"))
	if err != nil {
		t.Fatalf("ReadFile() after concurrent writes: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("file is empty after concurrent writes")
	}
}

func TestAtomicWriteFile_TempFileCleanup(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "cleanup.txt")
	if err := AtomicWriteFile(path, []byte("data"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "cleanup.txt" {
			t.Errorf("unexpected file left behind: %s", e.Name())
		}
	}
}
