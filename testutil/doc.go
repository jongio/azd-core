// Package testutil provides common testing utilities for azd extensions.
//
// This package includes helpers for:
//   - Capturing stdout during test execution (CaptureOutput)
//   - Locating test fixture directories (FindTestData)
//   - Creating temporary directories with automatic cleanup (TempDir)
//   - Common string assertions (Contains)
//
// All functions use t.Helper() for proper test line reporting.
//
// Example usage:
//
//	import (
//	    "testing"
//	    "github.com/jongio/azd-core/testutil"
//	)
//
//	func TestCommand(t *testing.T) {
//	    // Capture stdout from a command
//	    output := testutil.CaptureOutput(t, func() error {
//	        return runCommand()
//	    })
//
//	    // Check output contains expected text
//	    if !testutil.Contains(output, "success") {
//	        t.Error("expected success message")
//	    }
//	}
//
//	func TestWithFixtures(t *testing.T) {
//	    // Find test data directory
//	    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
//
//	    // Create temporary directory for test outputs
//	    tmpDir := testutil.TempDir(t)
//	    // tmpDir is automatically cleaned up after test
//	}
package testutil
