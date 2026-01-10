// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"github.com/shirou/gopsutil/v4/process"
)

// IsProcessRunning checks if a process with the given PID is running.
// Works cross-platform (Windows, Linux, macOS, FreeBSD, OpenBSD, Solaris, AIX).
//
// This function uses github.com/shirou/gopsutil for reliable cross-platform
// process detection, including proper handling of stale PIDs on Windows.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	// Create a process handle
	proc, err := process.NewProcess(int32(pid))
	if err != nil {
		// Process doesn't exist or can't be accessed
		return false
	}

	// Check if process is running
	// gopsutil handles platform differences correctly
	isRunning, err := proc.IsRunning()
	if err != nil {
		// Error checking status, assume not running
		return false
	}

	return isRunning
}

