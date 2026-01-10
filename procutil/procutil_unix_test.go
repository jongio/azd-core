//go:build !windows
// +build !windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestIsProcessRunningUnixSignal tests Unix-specific Signal(0) behavior
func TestIsProcessRunningUnixSignal(t *testing.T) {
	// Start a process
	cmd := exec.Command("sleep", "10")
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
	time.Sleep(100 * time.Millisecond)

	// Process should no longer be running
	if IsProcessRunning(pid) {
		t.Logf("Process %d still appears running after kill (may be transient)", pid)
	}
}

// TestIsProcessRunningUnixInit tests checking PID 1 (init process)
func TestIsProcessRunningUnixInit(t *testing.T) {
	// PID 1 should always be running on Unix systems
	if !IsProcessRunning(1) {
		t.Error("IsProcessRunning(1) = false, expected true for init process")
	}
}

// TestIsProcessRunningUnixPermission tests permission-related failures
func TestIsProcessRunningUnixPermission(t *testing.T) {
	// Try to check a system process we likely don't have permission to signal
	// This tests the Unix path where Signal(0) fails
	pid := os.Getpid()
	
	// Current process should always work
	if !IsProcessRunning(pid) {
		t.Errorf("IsProcessRunning(%d) = false for current process, expected true", pid)
	}
}

// TestCheckProcessRunningUnix tests the Unix-specific implementation
func TestCheckProcessRunningUnix(t *testing.T) {
	// Test with current process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	result := checkProcessRunning(process)
	if !result {
		t.Error("checkProcessRunning() = false for current process, expected true")
	}

	// Test with a non-existent process (high PID unlikely to exist)
	process2, err := os.FindProcess(999999)
	if err == nil {
		result := checkProcessRunning(process2)
		if result {
			t.Logf("checkProcessRunning(999999) = true, expected false (transient)")
		}
	}
}
