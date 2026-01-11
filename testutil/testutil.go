// Package testutil provides common testing utilities for azd extensions.
// It includes helpers for capturing output, locating test resources, creating
// temporary directories, and common test assertions.
package testutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CaptureOutput captures stdout during function execution.
// It redirects os.Stdout to a pipe, executes the function, and returns the captured output.
// The original stdout is always restored, even if the function returns an error.
// This is useful for testing commands that write to stdout.
//
// Example:
//
//	output := testutil.CaptureOutput(t, func() error {
//	    fmt.Println("test output")
//	    return nil
//	})
//	if !strings.Contains(output, "test output") {
//	    t.Error("expected output not found")
//	}
func CaptureOutput(t *testing.T, fn func() error) string {
	t.Helper()

	// Save original stdout
	origStdout := os.Stdout

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Failed to create pipe: %v", err)
	}

	// Replace stdout
	os.Stdout = w

	// Channel for output (buffered to avoid goroutine leak)
	outCh := make(chan string, 1)
	go func() {
		var output strings.Builder
		buf := make([]byte, 1024)
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				output.Write(buf[:n])
			}
			if readErr != nil {
				break
			}
		}
		outCh <- output.String()
	}()

	// Execute function
	fnErr := fn()

	// Close write end and restore stdout
	if err := w.Close(); err != nil {
		t.Logf("Failed to close pipe writer: %v", err)
	}
	os.Stdout = origStdout

	// Get output
	output := <-outCh

	if fnErr != nil {
		t.Logf("Command error: %v", fnErr)
	}

	return output
}

// FindTestData finds a test data directory relative to the current working directory.
// It accepts variadic subdirectory names to construct the path (e.g., "tests", "projects").
// It searches common locations relative to typical test directory structures and returns
// the first valid path found. This helper is useful for integration tests that need to
// locate test fixtures.
//
// Example:
//
//	testsDir := testutil.FindTestData(t, "tests", "projects")
//	scriptPath := filepath.Join(testsDir, "hello-world", "script.sh")
func FindTestData(t *testing.T, subdirs ...string) string {
	t.Helper()

	if len(subdirs) == 0 {
		t.Fatal("FindTestData requires at least one subdirectory")
	}

	// Get current working directory
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get working directory: %v", err)
	}

	// Build the target path from subdirectories
	targetPath := filepath.Join(subdirs...)

	// Try multiple possible paths relative to common test locations
	possiblePaths := []string{
		filepath.Join(cwd, targetPath),                                  // From current dir
		filepath.Join(cwd, "..", targetPath),                            // From parent dir
		filepath.Join(cwd, "..", "..", targetPath),                      // From grandparent
		filepath.Join(cwd, "..", "..", "..", targetPath),                // From great-grandparent
		filepath.Join(cwd, "..", "..", "..", "..", targetPath),          // From great-great-grandparent
		filepath.Join(cwd, "..", "..", "..", "..", "..", targetPath),    // From 5th ancestor
	}

	for _, testDir := range possiblePaths {
		testDir = filepath.Clean(testDir)
		if info, err := os.Stat(testDir); err == nil && info.IsDir() {
			return testDir
		}
	}

	t.Fatalf("Test data directory not found: %s (searched from %s)", targetPath, cwd)
	return ""
}

// TempDir creates a temporary directory for testing with automatic cleanup.
// The directory is created with secure permissions (0750) and is automatically
// removed when the test completes via t.Cleanup().
//
// Example:
//
//	tmpDir := testutil.TempDir(t)
//	configPath := filepath.Join(tmpDir, "config.json")
//	// Directory is automatically cleaned up after test
func TempDir(t *testing.T) string {
	t.Helper()

	tmpDir, err := os.MkdirTemp("", "azd-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}

	// Register cleanup
	t.Cleanup(func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			t.Logf("Failed to clean up temp directory %s: %v", tmpDir, err)
		}
	})

	return tmpDir
}

// Contains checks if a string contains a substring.
// This is a convenience helper for common test assertions.
//
// Example:
//
//	if !testutil.Contains(output, "expected text") {
//	    t.Error("output does not contain expected text")
//	}
func Contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
