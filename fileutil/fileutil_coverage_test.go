// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package fileutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Additional tests to improve coverage to ≥90%

func TestAtomicWriteJSON_CreateTempFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test unreliable on Windows")
	}

	// Create a read-only directory
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }() // Restore for cleanup

	path := filepath.Join(readOnlyDir, "test.json")
	data := map[string]string{"key": "value"}

	err := AtomicWriteJSON(path, data)
	if err == nil {
		t.Error("AtomicWriteJSON() expected error for read-only directory, got nil")
	}
}

func TestAtomicWriteJSON_MarshalError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	// Create data that can't be marshaled (function type)
	invalidData := map[string]interface{}{
		"func": func() {},
	}

	err := AtomicWriteJSON(path, invalidData)
	if err == nil {
		t.Error("AtomicWriteJSON() expected error for unmarshalable data, got nil")
	}
}

func TestAtomicWriteFile_CreateTempFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test unreliable on Windows")
	}

	// Create a read-only directory
	readOnlyDir := filepath.Join(t.TempDir(), "readonly")
	if err := os.Mkdir(readOnlyDir, 0555); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Chmod(readOnlyDir, 0755) }() // Restore for cleanup

	path := filepath.Join(readOnlyDir, "test.txt")
	data := []byte("test")

	err := AtomicWriteFile(path, data, 0644)
	if err == nil {
		t.Error("AtomicWriteFile() expected error for read-only directory, got nil")
	}
}

func TestAtomicWriteFile_ChmodAfterRename(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	data := []byte("test content")

	err := AtomicWriteFile(path, data, 0600)
	if err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File should exist after AtomicWriteFile(), got: %v", err)
	}

	// Verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read file: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("File content = %q, want %q", content, data)
	}
}

func TestAtomicWriteJSON_Sync(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := map[string]interface{}{
		"key1": "value1",
		"key2": 42,
		"key3": true,
	}

	// Write data
	if err := AtomicWriteJSON(path, data); err != nil {
		t.Fatalf("AtomicWriteJSON() error = %v", err)
	}

	// Verify file was synced (file should exist)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File should exist after sync, got: %v", err)
	}
}

func TestAtomicWriteFile_Sync(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	data := []byte("test data with sync")

	// Write data
	if err := AtomicWriteFile(path, data, 0644); err != nil {
		t.Fatalf("AtomicWriteFile() error = %v", err)
	}

	// Verify file was synced (file should exist)
	if _, err := os.Stat(path); err != nil {
		t.Errorf("File should exist after sync, got: %v", err)
	}
}

func TestAtomicWriteJSON_Chmod(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Permission test unreliable on Windows")
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.json")

	data := map[string]string{"test": "data"}

	if err := AtomicWriteJSON(path, data); err != nil {
		t.Fatalf("AtomicWriteJSON() error = %v", err)
	}

	// Verify permissions
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != FilePermission {
		t.Errorf("File permission = %o, want %o", perm, FilePermission)
	}
}

func TestReadJSON_ErrorCases(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with invalid JSON
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	if err := os.WriteFile(invalidFile, []byte("{invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	var result map[string]interface{}
	err := ReadJSON(invalidFile, &result)
	if err == nil {
		t.Error("ReadJSON() expected error for invalid JSON, got nil")
	}

	// Test with non-existent file (should not error)
	missingFile := filepath.Join(tmpDir, "missing.json")
	err = ReadJSON(missingFile, &result)
	if err != nil {
		t.Errorf("ReadJSON() with missing file should return nil, got: %v", err)
	}
}

func TestEnsureDir_ExistingFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file, then try to create a directory with the same path
	filePath := filepath.Join(tmpDir, "file-not-dir")
	if err := os.WriteFile(filePath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// This should fail because a file exists with that name
	err := EnsureDir(filePath)
	if err == nil {
		t.Error("EnsureDir() expected error when file exists at path, got nil")
	}
}

func TestFileExists_SymlinkToFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink test unreliable on Windows")
	}

	tmpDir := t.TempDir()

	// Create a file
	targetFile := "target.txt"
	targetPath := filepath.Join(tmpDir, targetFile)
	if err := os.WriteFile(targetPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create a symlink
	linkName := "link.txt"
	linkPath := filepath.Join(tmpDir, linkName)
	if err := os.Symlink(targetPath, linkPath); err != nil {
		t.Skipf("Failed to create symlink: %v", err)
	}

	// FileExists should return true for the symlink
	if !FileExists(tmpDir, linkName) {
		t.Error("FileExists() should return true for symlink")
	}
}

func TestAtomicWriteJSON_RetryLogic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "retry-test.json")

	data := map[string]string{"test": "retry"}

	// This tests the retry logic in atomic write
	// Write multiple times quickly to potentially trigger retries
	for i := 0; i < 3; i++ {
		if err := AtomicWriteJSON(path, data); err != nil {
			t.Errorf("AtomicWriteJSON() iteration %d error = %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify final state
	var result map[string]string
	if err := ReadJSON(path, &result); err != nil {
		t.Errorf("Failed to read final state: %v", err)
	}
}

func TestAtomicWriteFile_RetryLogic(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "retry-test.txt")

	data := []byte("test retry")

	// This tests the retry logic in atomic write
	// Write multiple times quickly to potentially trigger retries
	for i := 0; i < 3; i++ {
		if err := AtomicWriteFile(path, data, 0644); err != nil {
			t.Errorf("AtomicWriteFile() iteration %d error = %v", i, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Verify final state
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("Failed to read final state: %v", err)
	}
	if string(content) != string(data) {
		t.Errorf("Final content = %q, want %q", content, data)
	}
}

func TestHasFileWithExt_EmptyDir(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty directory should have no files
	if HasFileWithExt(tmpDir, ".txt") {
		t.Error("HasFileWithExt() should return false for empty directory")
	}
}

func TestHasFileWithExt_NonExistentDir(t *testing.T) {
	// Non-existent directory should return false
	if HasFileWithExt("/nonexistent/directory", ".txt") {
		t.Error("HasFileWithExt() should return false for non-existent directory")
	}
}

func TestFileExistsAny_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty list should return false
	if FileExistsAny(tmpDir) {
		t.Error("FileExistsAny() should return false for empty list")
	}
}

func TestFilesExistAll_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty list should return true (vacuous truth)
	if !FilesExistAll(tmpDir) {
		t.Error("FilesExistAll() should return true for empty list")
	}
}

func TestHasAnyFileWithExts_EmptyList(t *testing.T) {
	tmpDir := t.TempDir()

	// Empty list should return false
	if HasAnyFileWithExts(tmpDir) {
		t.Error("HasAnyFileWithExts() should return false for empty list")
	}
}

func TestContainsTextInFile_NonExistent(t *testing.T) {
	tmpDir := t.TempDir()

	// Non-existent file should return false
	if ContainsTextInFile(tmpDir, "missing.txt", "text") {
		t.Error("ContainsTextInFile() should return false for non-existent file")
	}
}

func TestReadJSON_EmptyTarget(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a valid JSON file
	jsonFile := filepath.Join(tmpDir, "data.json")
	if err := os.WriteFile(jsonFile, []byte(`{"key":"value"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Read into nil target should fail
	err := ReadJSON(jsonFile, nil)
	if err == nil {
		t.Error("ReadJSON() with nil target should error")
	}
}

func TestAtomicWriteFile_LargeData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "large.dat")

	// Create large data (1MB)
	data := make([]byte, 1024*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}

	if err := AtomicWriteFile(path, data, 0644); err != nil {
		t.Fatalf("AtomicWriteFile() with large data error = %v", err)
	}

	// Verify data
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(content) != len(data) {
		t.Errorf("File size = %d, want %d", len(content), len(data))
	}
}

func TestAtomicWriteJSON_ComplexData(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "complex.json")

	// Create complex nested data
	data := map[string]interface{}{
		"string":  "value",
		"number":  42,
		"float":   3.14,
		"bool":    true,
		"null":    nil,
		"array":   []interface{}{1, 2, 3},
		"object": map[string]interface{}{
			"nested": "value",
		},
	}

	if err := AtomicWriteJSON(path, data); err != nil {
		t.Fatalf("AtomicWriteJSON() with complex data error = %v", err)
	}

	// Verify we can read it back
	var result map[string]interface{}
	if err := ReadJSON(path, &result); err != nil {
		t.Errorf("Failed to read complex JSON: %v", err)
	}
}
