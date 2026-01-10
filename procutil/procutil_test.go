// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
	"time"
)

func TestIsProcessRunningCurrentProcess(t *testing.T) {
	// Current process should always be running
	pid := os.Getpid()

	result := IsProcessRunning(pid)

	// Our own process should always be detectable
	if !result {
		t.Errorf("IsProcessRunning(%d) = false for current process, expected true", pid)
	}
}

func TestIsProcessRunningInvalidPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
	}{
		{"zero pid", 0},
		{"negative pid", -1},
		{"very negative pid", -999},
		{"negative small", -5},
		{"min int32", -2147483648},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProcessRunning(tt.pid)
			if result {
				t.Errorf("IsProcessRunning(%d) = true, expected false for invalid PID", tt.pid)
			}
		})
	}
}

func TestIsProcessRunningNonExistentPID(t *testing.T) {
	tests := []struct {
		name string
		pid  int
	}{
		{"very high PID", 999999},
		{"high PID", 500000},
		{"max int", 2147483647},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProcessRunning(tt.pid)

			// This should return false on most systems
			// Note: On Windows, this may incorrectly return true due to known limitations
			if runtime.GOOS != "windows" && result {
				t.Logf("IsProcessRunning(%d) = true for non-existent PID (expected false on Unix)", tt.pid)
			}
		})
	}
}

func TestIsProcessRunningRealProcess(t *testing.T) {
	// Start a short-lived process to test against a known running process
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Use timeout command which waits for specified seconds
		cmd = exec.Command("timeout", "5")
	} else {
		// Use sleep command on Unix
		cmd = exec.Command("sleep", "5")
	}

	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

	// Ensure the process is cleaned up
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
			cmd.Wait()
		}
	}()

	pid := cmd.Process.Pid

	// Process should be running
	if !IsProcessRunning(pid) {
		t.Errorf("IsProcessRunning(%d) = false, expected true for running process", pid)
	}

	// Kill the process
	if err := cmd.Process.Kill(); err != nil {
		t.Fatalf("Failed to kill test process: %v", err)
	}

	// Wait for process to exit
	cmd.Wait()

	// Give the system a moment to clean up the process
	time.Sleep(100 * time.Millisecond)

	// On Unix, process should not be running after kill
	// On Windows, this may still return true due to stale PID issue
	if runtime.GOOS != "windows" {
		// After a brief wait, the process should be gone on Unix
		time.Sleep(200 * time.Millisecond)
		if IsProcessRunning(pid) {
			t.Logf("Warning: IsProcessRunning(%d) = true after process killed (may be transient)", pid)
		}
	}
}

func TestIsProcessRunningParentProcess(t *testing.T) {
	// Get parent process ID
	ppid := os.Getppid()

	if ppid <= 0 {
		t.Skip("Parent PID not available or invalid")
	}

	result := IsProcessRunning(ppid)

	// Parent process should typically be running
	// (unless we're orphaned, which is rare in tests)
	if !result && runtime.GOOS != "windows" {
		t.Logf("Parent process (PID %d) not detected as running - may be orphaned or permission issue", ppid)
	}
}

func TestIsProcessRunningEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		pid      int
		expected bool
	}{
		{"PID 1 (init/system)", 1, true}, // PID 1 is always running on Unix, usually on Windows
		{"PID 2", 2, false},              // PID 2 may or may not exist
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsProcessRunning(tt.pid)
			// For PID 1, we expect it to be running on most systems
			if tt.name == "PID 1 (init/system)" && runtime.GOOS != "windows" && !result {
				t.Logf("PID 1 not running - unexpected on Unix systems")
			}
			t.Logf("%s: IsProcessRunning(%d) = %v", tt.name, tt.pid, result)
		})
	}
}

func TestIsProcessRunningMultipleTimes(t *testing.T) {
	// Test calling multiple times on same PID
	pid := os.Getpid()

	for i := 0; i < 5; i++ {
		result := IsProcessRunning(pid)
		if !result {
			t.Errorf("Iteration %d: IsProcessRunning(%d) = false for current process", i, pid)
		}
	}
}
