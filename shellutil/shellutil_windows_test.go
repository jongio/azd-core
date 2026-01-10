//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package shellutil

import (
	"testing"
)

func TestDetectShellPowerShellOnWindows(t *testing.T) {
	// On Windows, .ps1 files should use powershell (not pwsh)
	got := DetectShell("script.ps1")
	if got != ShellPowerShell {
		t.Errorf("DetectShell(.ps1) on Windows = %q, want %q", got, ShellPowerShell)
	}
}

func TestDetectShellDefaultOnWindows(t *testing.T) {
	// On Windows, unknown extensions should default to cmd
	got := DetectShell("script.unknown")
	if got != ShellCmd {
		t.Errorf("DetectShell(unknown) on Windows = %q, want %q", got, ShellCmd)
	}
}
