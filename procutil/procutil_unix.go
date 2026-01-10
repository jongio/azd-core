//go:build !windows
// +build !windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package procutil

import (
	"os"
	"syscall"
)

// checkProcessRunning performs Unix-specific process running check
func checkProcessRunning(process *os.Process) bool {
	// On Unix-like systems, use signal 0 to check existence
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}

	return true
}
