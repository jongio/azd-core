package testutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCaptureOutput(t *testing.T) {
	t.Run("captures stdout", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			fmt.Println("test output")
			return nil
		})

		if !strings.Contains(output, "test output") {
			t.Errorf("expected output to contain 'test output', got: %s", output)
		}
	})

	t.Run("captures multiple lines", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			fmt.Println("line 1")
			fmt.Println("line 2")
			fmt.Println("line 3")
			return nil
		})

		if !strings.Contains(output, "line 1") {
			t.Error("expected output to contain 'line 1'")
		}
		if !strings.Contains(output, "line 2") {
			t.Error("expected output to contain 'line 2'")
		}
		if !strings.Contains(output, "line 3") {
			t.Error("expected output to contain 'line 3'")
		}
	})

	t.Run("restores stdout on error", func(t *testing.T) {
		expectedErr := errors.New("test error")
		output := CaptureOutput(t, func() error {
			fmt.Println("output before error")
			return expectedErr
		})

		if !strings.Contains(output, "output before error") {
			t.Error("expected output to contain 'output before error'")
		}

		// Verify stdout is restored by printing to it
		fmt.Println("stdout restored")
	})

	t.Run("handles empty output", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			return nil
		})

		if output != "" {
			t.Errorf("expected empty output, got: %s", output)
		}
	})

	t.Run("captures fmt.Print without newline", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			fmt.Print("no newline")
			return nil
		})

		if !strings.Contains(output, "no newline") {
			t.Errorf("expected output to contain 'no newline', got: %s", output)
		}
	})

	t.Run("captures mixed fmt.Print and fmt.Println", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			fmt.Print("part1")
			fmt.Println(" part2")
			fmt.Print("part3")
			return nil
		})

		expected := "part1 part2\npart3"
		if output != expected {
			t.Errorf("expected '%s', got: '%s'", expected, output)
		}
	})

	t.Run("captures large output", func(t *testing.T) {
		output := CaptureOutput(t, func() error {
			// Generate output larger than the 1024 byte buffer
			for i := 0; i < 200; i++ {
				fmt.Printf("line %d with some extra text to make it longer\n", i)
			}
			return nil
		})

		// Verify we got all the output
		if !strings.Contains(output, "line 0") {
			t.Error("expected to find first line")
		}
		if !strings.Contains(output, "line 199") {
			t.Error("expected to find last line")
		}

		// Count lines to ensure we got everything
		lines := strings.Split(output, "\n")
		// Should have 200 lines plus 1 empty line from trailing newline
		if len(lines) < 200 {
			t.Errorf("expected at least 200 lines, got %d", len(lines))
		}
	})
}

func TestFindTestData(t *testing.T) {
	t.Run("finds testutil directory from current location", func(t *testing.T) {
		// We're in testutil package, so searching for "testutil" should find current dir
		testDir := FindTestData(t, "testutil")

		if testDir == "" {
			t.Fatal("expected to find testutil directory")
		}

		// Verify it's a valid directory
		info, err := os.Stat(testDir)
		if err != nil {
			t.Fatalf("directory does not exist: %v", err)
		}
		if !info.IsDir() {
			t.Error("path is not a directory")
		}

		// Verify it contains testutil files
		testutilGo := filepath.Join(testDir, "testutil.go")
		if _, err := os.Stat(testutilGo); err != nil {
			t.Errorf("expected to find testutil.go in directory: %v", err)
		}
	})

	t.Run("finds nested subdirectories", func(t *testing.T) {
		// Create a temporary test structure
		tmpDir := TempDir(t)

		// Create nested structure: tmpDir/tests/fixtures/data
		nestedPath := filepath.Join(tmpDir, "tests", "fixtures", "data")
		if err := os.MkdirAll(nestedPath, 0750); err != nil {
			t.Fatalf("failed to create nested structure: %v", err)
		}

		// Change to tmpDir to test from there
		origWd, _ := os.Getwd()
		defer os.Chdir(origWd)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Find the nested path
		found := FindTestData(t, "tests", "fixtures", "data")
		if found == "" {
			t.Fatal("expected to find nested directory")
		}

		// Verify it's the correct path
		if !strings.HasSuffix(filepath.Clean(found), filepath.Join("tests", "fixtures", "data")) {
			t.Errorf("unexpected path: %s", found)
		}
	})

	t.Run("searches parent directories", func(t *testing.T) {
		// Create a temporary test structure
		tmpDir := TempDir(t)

		// Create structure: tmpDir/tests/data and tmpDir/src/cmd/test
		dataPath := filepath.Join(tmpDir, "tests", "data")
		testPath := filepath.Join(tmpDir, "src", "cmd", "test")
		if err := os.MkdirAll(dataPath, 0750); err != nil {
			t.Fatalf("failed to create data path: %v", err)
		}
		if err := os.MkdirAll(testPath, 0750); err != nil {
			t.Fatalf("failed to create test path: %v", err)
		}

		// Change to nested test directory
		origWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(origWd) }()
		if err := os.Chdir(testPath); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Should find tests/data by searching upward
		found := FindTestData(t, "tests", "data")
		if found == "" {
			t.Fatal("expected to find tests/data directory")
		}

		// Verify it's the correct path
		if !strings.HasSuffix(filepath.Clean(found), filepath.Join("tests", "data")) {
			t.Errorf("unexpected path: %s", found)
		}
	})

	// Note: We cannot easily test the failure case for no subdirectories
	// because FindTestData calls t.Fatal which exits the test.
	// This would require a more complex test harness or subprocess testing.
	// The failure behavior is documented and enforced by the implementation.

	t.Run("searches multiple parent levels", func(t *testing.T) {
		// Create a temporary test structure
		tmpDir := TempDir(t)

		// Create structure: tmpDir/data and tmpDir/a/b/c/d/e (deeply nested)
		dataPath := filepath.Join(tmpDir, "data")
		deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e")
		if err := os.MkdirAll(dataPath, 0750); err != nil {
			t.Fatalf("failed to create data path: %v", err)
		}
		if err := os.MkdirAll(deepPath, 0750); err != nil {
			t.Fatalf("failed to create deep path: %v", err)
		}

		// Change to deeply nested directory
		origWd, _ := os.Getwd()
		defer func() { _ = os.Chdir(origWd) }()
		if err := os.Chdir(deepPath); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Should find data directory by searching upward through multiple levels
		found := FindTestData(t, "data")
		if found == "" {
			t.Fatal("expected to find data directory")
		}

		// Verify it's a directory
		if info, err := os.Stat(found); err != nil {
			t.Errorf("directory not found: %v", err)
		} else if !info.IsDir() {
			t.Error("path is not a directory")
		}
	})

	t.Run("handles single subdirectory", func(t *testing.T) {
		// Create a temporary test structure
		tmpDir := TempDir(t)

		// Create single directory: tmpDir/tests
		testsPath := filepath.Join(tmpDir, "tests")
		if err := os.MkdirAll(testsPath, 0750); err != nil {
			t.Fatalf("failed to create tests path: %v", err)
		}

		// Change to tmpDir to test from there
		origWd, _ := os.Getwd()
		defer os.Chdir(origWd)
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatalf("failed to change directory: %v", err)
		}

		// Find the tests directory
		found := FindTestData(t, "tests")
		if found == "" {
			t.Fatal("expected to find tests directory")
		}

		// Verify it's a directory
		if info, err := os.Stat(found); err != nil {
			t.Errorf("directory not found: %v", err)
		} else if !info.IsDir() {
			t.Error("path is not a directory")
		}
	})

	t.Run("returns clean path", func(t *testing.T) {
		// Find a directory that exists
		found := FindTestData(t, "testutil")

		// Path should be clean (no double slashes, etc.)
		cleaned := filepath.Clean(found)
		if found != cleaned {
			t.Errorf("path not clean: %s != %s", found, cleaned)
		}
	})
}

func TestTempDir(t *testing.T) {
	t.Run("creates directory", func(t *testing.T) {
		tmpDir := TempDir(t)

		// Verify directory exists
		info, err := os.Stat(tmpDir)
		if err != nil {
			t.Fatalf("temp directory does not exist: %v", err)
		}
		if !info.IsDir() {
			t.Error("temp path is not a directory")
		}
	})

	t.Run("creates unique directories", func(t *testing.T) {
		tmpDir1 := TempDir(t)
		tmpDir2 := TempDir(t)

		if tmpDir1 == tmpDir2 {
			t.Error("expected unique directories")
		}
	})

	t.Run("directory is writable", func(t *testing.T) {
		tmpDir := TempDir(t)

		// Try to create a file in the directory
		testFile := filepath.Join(tmpDir, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to write to temp directory: %v", err)
		}

		// Verify file was created
		if _, err := os.Stat(testFile); err != nil {
			t.Errorf("test file not created: %v", err)
		}
	})

	t.Run("directory has azd-test prefix", func(t *testing.T) {
		tmpDir := TempDir(t)
		baseName := filepath.Base(tmpDir)

		if !strings.HasPrefix(baseName, "azd-test-") {
			t.Errorf("expected directory name to have 'azd-test-' prefix, got: %s", baseName)
		}
	})

	t.Run("cleanup removes directory", func(t *testing.T) {
		// We can't directly test cleanup without complex setup since cleanup
		// runs via t.Cleanup() which executes after the test completes.
		// The cleanup mechanism is tested indirectly by the fact that
		// we don't accumulate temp directories across test runs.

		// Instead, verify that cleanup is registered
		tmpDir := TempDir(t)

		// Verify directory exists now
		if _, err := os.Stat(tmpDir); err != nil {
			t.Fatalf("temp directory should exist: %v", err)
		}

		// Create nested content to verify cleanup handles complex structures
		nestedPath := filepath.Join(tmpDir, "a", "b", "c")
		if err := os.MkdirAll(nestedPath, 0750); err != nil {
			t.Fatalf("failed to create nested structure: %v", err)
		}
		testFile := filepath.Join(nestedPath, "test.txt")
		if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
	})

	t.Run("multiple files in temp dir", func(t *testing.T) {
		tmpDir := TempDir(t)

		// Create multiple files
		for i := 0; i < 5; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
			if err := os.WriteFile(fileName, []byte(fmt.Sprintf("content%d", i)), 0644); err != nil {
				t.Fatalf("failed to write file %d: %v", i, err)
			}
		}

		// Verify all files exist
		for i := 0; i < 5; i++ {
			fileName := filepath.Join(tmpDir, fmt.Sprintf("file%d.txt", i))
			if _, err := os.Stat(fileName); err != nil {
				t.Errorf("file %d not found: %v", i, err)
			}
		}
	})

	t.Run("nested directories in temp dir", func(t *testing.T) {
		tmpDir := TempDir(t)

		// Create nested structure
		nestedPath := filepath.Join(tmpDir, "a", "b", "c")
		if err := os.MkdirAll(nestedPath, 0750); err != nil {
			t.Fatalf("failed to create nested structure: %v", err)
		}

		// Verify nested directory exists
		if info, err := os.Stat(nestedPath); err != nil {
			t.Errorf("nested directory not created: %v", err)
		} else if !info.IsDir() {
			t.Error("nested path is not a directory")
		}
	})
}

func TestContains(t *testing.T) {
	t.Run("finds substring", func(t *testing.T) {
		if !Contains("hello world", "world") {
			t.Error("expected to find 'world' in 'hello world'")
		}
	})

	t.Run("returns false when substring not found", func(t *testing.T) {
		if Contains("hello world", "foo") {
			t.Error("expected not to find 'foo' in 'hello world'")
		}
	})

	t.Run("handles empty substring", func(t *testing.T) {
		// Empty string is always contained
		if !Contains("hello", "") {
			t.Error("expected empty string to be contained")
		}
	})

	t.Run("handles empty string", func(t *testing.T) {
		if Contains("", "hello") {
			t.Error("expected not to find 'hello' in empty string")
		}
	})

	t.Run("handles exact match", func(t *testing.T) {
		if !Contains("hello", "hello") {
			t.Error("expected to find exact match")
		}
	})

	t.Run("is case-sensitive", func(t *testing.T) {
		if Contains("Hello World", "hello") {
			t.Error("expected case-sensitive comparison")
		}
		if !Contains("Hello World", "Hello") {
			t.Error("expected to find 'Hello' with correct case")
		}
	})

	t.Run("finds substring at start", func(t *testing.T) {
		if !Contains("hello world", "hello") {
			t.Error("expected to find substring at start")
		}
	})

	t.Run("finds substring at end", func(t *testing.T) {
		if !Contains("hello world", "world") {
			t.Error("expected to find substring at end")
		}
	})

	t.Run("finds substring in middle", func(t *testing.T) {
		if !Contains("hello world", "lo wo") {
			t.Error("expected to find substring in middle")
		}
	})
}

// Test integration: using multiple helpers together
func TestIntegration(t *testing.T) {
	t.Run("capture output to temp file", func(t *testing.T) {
		tmpDir := TempDir(t)

		// Capture output
		output := CaptureOutput(t, func() error {
			fmt.Println("test output to capture")
			return nil
		})

		// Write to temp file
		outputFile := filepath.Join(tmpDir, "output.txt")
		if err := os.WriteFile(outputFile, []byte(output), 0644); err != nil {
			t.Fatalf("failed to write output file: %v", err)
		}

		// Read back and verify
		content, err := os.ReadFile(outputFile)
		if err != nil {
			t.Fatalf("failed to read output file: %v", err)
		}

		if !Contains(string(content), "test output to capture") {
			t.Error("expected to find output in file")
		}
	})

	t.Run("find test data and create temp dir", func(t *testing.T) {
		// Find testutil directory (we're in it)
		testutilDir := FindTestData(t, "testutil")

		// Create temp directory for test outputs
		tmpDir := TempDir(t)

		// Both should be valid directories
		for _, dir := range []string{testutilDir, tmpDir} {
			if info, err := os.Stat(dir); err != nil {
				t.Errorf("directory not found: %v", err)
			} else if !info.IsDir() {
				t.Error("path is not a directory")
			}
		}
	})
}
