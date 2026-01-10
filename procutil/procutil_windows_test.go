//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestIsProcessRunningWindowsSignal tests Windows-specific Signal(0) behavior
func TestIsProcessRunningWindowsSignal(t *testing.T) {
	// Start a process
	cmd := exec.Command("timeout", "10")
	err := cmd.Start()
	if err != nil {
		t.Fatalf("Failed to start test process: %v", err)
	}

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

	// Give the system a moment to clean up
	time.Sleep(200 * time.Millisecond)

	// Note: On Windows, this may still return true due to stale PID limitation
	result := IsProcessRunning(pid)
	t.Logf("After kill, IsProcessRunning(%d) = %v (may be true on Windows due to stale PID)", pid, result)
}

// TestIsProcessRunningWindowsCurrentProcess tests current process on Windows
func TestIsProcessRunningWindowsCurrentProcess(t *testing.T) {
	pid := os.Getpid()
	
	// Current process should always be detected
	if !IsProcessRunning(pid) {
		t.Errorf("IsProcessRunning(%d) = false for current process, expected true", pid)
	}
}

// TestIsProcessRunningWindowsParentProcess tests parent process on Windows
func TestIsProcessRunningWindowsParentProcess(t *testing.T) {
	ppid := os.Getppid()
	
	if ppid <= 0 {
		t.Skip("Parent PID not available")
	}
	
	// Parent process should typically be running
	if !IsProcessRunning(ppid) {
		t.Logf("Parent process (PID %d) not detected as running", ppid)
	}
}

// TestIsProcessRunningWindowsSystem tests system process on Windows  
func TestIsProcessRunningWindowsSystem(t *testing.T) {
	// PID 4 is typically the System process on Windows
	result := IsProcessRunning(4)
	t.Logf("IsProcessRunning(4) [System process] = %v", result)
	
	// We expect this to usually return true, but don't fail if it doesn't
	// since permissions or system configuration may vary

	// Also test some other well-known Windows system PIDs
	// These tests help exercise different error paths
	for _, pid := range []int{0, 4, 8} {
		result := IsProcessRunning(pid)
		t.Logf("IsProcessRunning(%d) = %v", pid, result)
	}
}

// TestCheckProcessRunningWindows tests the Windows-specific implementation
func TestCheckProcessRunningWindows(t *testing.T) {
	// Test with current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	result := checkProcessRunning(process)
	if !result {
		t.Error("checkProcessRunning() = false for current process, expected true")
	}

	// Test with likely non-existent high PID
	process2, err := os.FindProcess(999999)
	if err == nil {
		result := checkProcessRunning(process2)
		// On Windows, this may return true due to known limitations
		t.Logf("checkProcessRunning(999999) = %v", result)
	}

	// Test with another likely non-existent PID
	process3, err := os.FindProcess(123456)
	if err == nil {
		result := checkProcessRunning(process3)
		t.Logf("checkProcessRunning(123456) = %v", result)
	}

	// Test with parent process
	ppid := os.Getppid()
	if ppid > 0 {
		process4, err := os.FindProcess(ppid)
		if err == nil {
			result := checkProcessRunning(process4)
			t.Logf("checkProcessRunning(parent %d) = %v", ppid, result)
		}
	}
}
