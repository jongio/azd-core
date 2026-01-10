//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
	"strings"
	"syscall"
)

// checkProcessRunning performs Windows-specific process running check
func checkProcessRunning(process *os.Process) bool {
	// Try Signal(0) first - it may work on some Windows versions
	err := process.Signal(syscall.Signal(0))
	if err == nil {
		return true
	}

	// On Windows, Signal(0) is typically not supported.
	// NOTE: This is a known limitation. For fully reliable Windows process
	// detection, use Windows API (OpenProcess with PROCESS_QUERY_LIMITED_INFORMATION)
	// or a library like github.com/shirou/gopsutil.
	// For now, we return true for valid PIDs as a fallback, with the understanding
	// that stale PIDs may incorrectly appear as running.
	errMsg := err.Error()
	if strings.Contains(errMsg, "not supported") || strings.Contains(errMsg, "Access is denied") {
		// Process handle was created, assume process exists
		// This is imperfect but better than failing all Windows checks
		return true
	}

	// Other error - process may not exist
	return false
}
