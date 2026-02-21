//go:build !windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package shellutil

import (
	"testing"
)

func TestDetectShellPwshOnUnix(t *testing.T) {
	// On Unix systems, .ps1 files should use pwsh (not powershell)
	got := DetectShell("script.ps1")
	if got != ShellPwsh {
		t.Errorf("DetectShell(.ps1) on Unix = %q, want %q", got, ShellPwsh)
	}
}

func TestDetectShellDefaultOnUnix(t *testing.T) {
	// On Unix systems, unknown extensions should default to bash
	got := DetectShell("script.unknown")
	if got != ShellBash {
		t.Errorf("DetectShell(unknown) on Unix = %q, want %q", got, ShellBash)
	}
}
