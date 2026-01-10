// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
)

// IsProcessRunning checks if a process with the given PID is running.
// Works cross-platform (Windows and Unix).
//
// KNOWN LIMITATION (Windows): On Windows, this function may return true for
// stale PIDs because os.FindProcess always succeeds and Signal(0) is not
// fully supported. For production use requiring high reliability, consider
// using Windows API (OpenProcess) or github.com/shirou/gopsutil.
func IsProcessRunning(pid int) bool {
	if pid <= 0 {
		return false
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	return checkProcessRunning(process)
}

// checkProcessRunning is implemented in platform-specific files:
// - procutil_windows.go for Windows
// - procutil_unix.go for Unix-like systems

