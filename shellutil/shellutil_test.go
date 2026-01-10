// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package shellutil

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Package shellutil provides cross-platform shell detection utilities.
// Test coverage expectations:
//   - Windows platform: ~86% (due to Unix-specific code paths)
//   - Unix platforms: ~86% (due to Windows-specific code paths)
//   - Combined coverage across all platforms: >90%
//
// This is standard practice for cross-platform Go packages where
// platform-specific code paths cannot all be tested on a single OS.

func TestDetectShell(t *testing.T) {
	// Get expected PowerShell name based on OS
	expectedPwsh := ShellPowerShell
	if runtime.GOOS != osWindows {
		expectedPwsh = ShellPwsh
	}

	tests := []struct {
		name       string
		scriptPath string
		want       string
	}{
		{
			name:       "PowerShell script",
			scriptPath: "test.ps1",
			want:       expectedPwsh,
		},
		{
			name:       "Bash script",
			scriptPath: "test.sh",
			want:       ShellBash,
		},
		{
			name:       "Cmd script",
			scriptPath: "test.cmd",
			want:       ShellCmd,
		},
		{
			name:       "Batch script",
			scriptPath: "test.bat",
			want:       ShellCmd,
		},
		{
			name:       "Zsh script",
			scriptPath: "test.zsh",
			want:       ShellZsh,
		},
		{
			name:       "No extension",
			scriptPath: "script",
			want: func() string {
				if runtime.GOOS == osWindows {
					return ShellCmd
				}
				return ShellBash
			}(),
		},
		{
			name:       "Uppercase extension",
			scriptPath: "test.PS1",
			want:       expectedPwsh,
		},
		{
			name:       "Mixed case extension",
			scriptPath: "test.Sh",
			want:       ShellBash,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectShell(tt.scriptPath)
			if got != tt.want {
				t.Errorf("DetectShell(%q) = %q, want %q", tt.scriptPath, got, tt.want)
			}
		})
	}
}

func TestReadShebang(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Bash shebang",
			content: "#!/bin/bash\necho hello",
			want:    "bash",
		},
		{
			name:    "Sh shebang",
			content: "#!/bin/sh\necho hello",
			want:    "sh",
		},
		{
			name:    "Python shebang",
			content: "#!/usr/bin/env python3\nprint('hello')",
			want:    "python3",
		},
		{
			name:    "Zsh shebang",
			content: "#!/usr/bin/zsh\necho hello",
			want:    "zsh",
		},
		{
			name:    "No shebang",
			content: "echo hello",
			want:    "",
		},
		{
			name:    "Empty file",
			content: "",
			want:    "",
		},
		{
			name:    "Shebang with spaces",
			content: "#! /bin/bash\necho hello",
			want:    "bash",
		},
		{
			name:    "Node shebang",
			content: "#!/usr/bin/env node\nconsole.log('test')",
			want:    "node",
		},
		{
			name:    "Python2 shebang",
			content: "#!/usr/bin/python\nprint 'test'",
			want:    "python",
		},
		{
			name:    "Pwsh shebang",
			content: "#!/usr/bin/env pwsh\nWrite-Host 'test'",
			want:    "pwsh",
		},
		{
			name:    "Comment not shebang",
			content: "# This is a comment\necho hello",
			want:    "",
		},
		{
			name:    "Shebang without newline (EOF)",
			content: "#!/bin/bash",
			want:    "bash",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "script-*.sh")
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				_ = os.Remove(tmpFile.Name())
			}()

			if _, err := tmpFile.WriteString(tt.content); err != nil {
				t.Fatal(err)
			}
			_ = tmpFile.Close()

			got := ReadShebang(tmpFile.Name())
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPowerShellName(t *testing.T) {
	// Test PowerShell name selection based on OS
	got := DetectShell("test.ps1")

	if runtime.GOOS == osWindows {
		if got != ShellPowerShell {
			t.Errorf("DetectShell(.ps1) on Windows = %q, want %q", got, ShellPowerShell)
		}
	} else {
		if got != ShellPwsh {
			t.Errorf("DetectShell(.ps1) on non-Windows = %q, want %q", got, ShellPwsh)
		}
	}
}

func TestDetectShellWithShebang(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		content  string
		filename string
		want     string
	}{
		{
			name:     "Bash shebang overrides .txt extension",
			content:  "#!/bin/bash\necho hello",
			filename: "script.txt",
			want:     "bash",
		},
		{
			name:     "Python shebang",
			content:  "#!/usr/bin/env python3\nprint('hello')",
			filename: "script",
			want:     "python3",
		},
		{
			name:     "Zsh shebang",
			content:  "#!/usr/bin/zsh\necho hello",
			filename: "script",
			want:     "zsh",
		},
		{
			name:     "Node shebang overrides unknown extension",
			content:  "#!/usr/bin/env node\nconsole.log('test')",
			filename: "script.js",
			want:     "node",
		},
		{
			name:     "Sh shebang",
			content:  "#!/bin/sh\necho test",
			filename: "script",
			want:     "sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}

			got := DetectShell(scriptPath)
			if got != tt.want {
				t.Errorf("DetectShell(%q) = %q, want %q", scriptPath, got, tt.want)
			}
		})
	}
}

func TestReadShebangFileNotFound(t *testing.T) {
	got := ReadShebang("nonexistent-file.sh")

	if got != "" {
		t.Errorf("ReadShebang(nonexistent) = %q, want empty string", got)
	}
}

func TestReadShebangFilePermissionError(t *testing.T) {
	// Create a temp file
	tmpFile, err := os.CreateTemp("", "script-*.sh")
	if err != nil {
		t.Fatal(err)
	}
	tmpPath := tmpFile.Name()

	// Write content
	if _, err := tmpFile.WriteString("#!/bin/bash\necho test"); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	defer os.Remove(tmpPath)

	// On Unix, we can test permission errors
	if runtime.GOOS != osWindows {
		// Remove read permission
		if err := os.Chmod(tmpPath, 0000); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(tmpPath, 0644) // Restore for cleanup

		got := ReadShebang(tmpPath)
		if got != "" {
			t.Errorf("ReadShebang(unreadable) = %q, want empty string", got)
		}
	}
}

func TestShellConstants(t *testing.T) {
	// Test that constants have expected values
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"ShellBash", ShellBash, "bash"},
		{"ShellCmd", ShellCmd, "cmd"},
		{"ShellPowerShell", ShellPowerShell, "powershell"},
		{"ShellPwsh", ShellPwsh, "pwsh"},
		{"ShellSh", ShellSh, "sh"},
		{"ShellZsh", ShellZsh, "zsh"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.value != tt.want {
				t.Errorf("%s = %q, want %q", tt.name, tt.value, tt.want)
			}
		})
	}
}

func TestDetectShellExtensionPriority(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a .sh file with a python shebang
	// Extension should take priority
	scriptPath := filepath.Join(tmpDir, "test.sh")
	content := "#!/usr/bin/env python3\nprint('hello')"
	if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got := DetectShell(scriptPath)
	// .sh extension should win, returning bash
	if got != ShellBash {
		t.Errorf("DetectShell(.sh with python shebang) = %q, want %q (extension priority)", got, ShellBash)
	}
}

func TestReadShebangWithEnvVariants(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "env with multiple args",
			content: "#!/usr/bin/env -S python3 -u\nprint('test')",
			want:    "-S", // This tests current behavior - takes first arg after env
		},
		{
			name:    "env with bash",
			content: "#!/usr/bin/env bash\necho test",
			want:    "bash",
		},
		{
			name:    "env with zsh",
			content: "#!/usr/bin/env zsh\necho test",
			want:    "zsh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadShebangEdgeCases(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Shebang with trailing spaces",
			content: "#!/bin/bash   \necho test",
			want:    "bash",
		},
		{
			name:    "Shebang with tabs",
			content: "#!/bin/bash\t\t\necho test",
			want:    "bash",
		},
		{
			name:    "Shebang only (no newline at all)",
			content: "#!/bin/sh",
			want:    "sh",
		},
		{
			name:    "Shebang with Windows line endings",
			content: "#!/bin/bash\r\necho test",
			want:    "bash",
		},
		{
			name:    "Just shebang prefix with no path",
			content: "#!\necho test",
			want:    "",
		},
		{
			name:    "Shebang with only spaces after",
			content: "#!   \necho test",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0600); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadShebangWithDebugMode(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with shebang
	scriptPath := filepath.Join(tmpDir, "test.sh")
	if err := os.WriteFile(scriptPath, []byte("#!/bin/bash\necho test"), 0600); err != nil {
		t.Fatal(err)
	}

	// Enable debug mode
	originalDebug := os.Getenv(EnvVarDebug)
	defer os.Setenv(EnvVarDebug, originalDebug)

	os.Setenv(EnvVarDebug, "true")

	// Read shebang - this exercises the debug path
	// The debug output goes to stderr, which we don't capture in this test
	// but this ensures the code path is covered
	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() with debug = %q, want bash", got)
	}
}

func TestDetectShellOSDefaults(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with unknown extension - should fall back to OS default
	scriptPath := filepath.Join(tmpDir, "script.unknown")
	if err := os.WriteFile(scriptPath, []byte("echo test"), 0600); err != nil {
		t.Fatal(err)
	}

	got := DetectShell(scriptPath)

	if runtime.GOOS == osWindows {
		if got != ShellCmd {
			t.Errorf("DetectShell(unknown ext) on Windows = %q, want %q", got, ShellCmd)
		}
	} else {
		if got != ShellBash {
			t.Errorf("DetectShell(unknown ext) on Unix = %q, want %q", got, ShellBash)
		}
	}
}

func TestReadShebangLargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with shebang followed by lots of content
	scriptPath := filepath.Join(tmpDir, "large.sh")
	content := "#!/bin/bash\n" + strings.Repeat("echo test\n", 1000)
	if err := os.WriteFile(scriptPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang(large file) = %q, want bash", got)
	}
}

func TestReadShebangBinaryFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with binary content (no valid shebang)
	scriptPath := filepath.Join(tmpDir, "binary")
	binaryContent := []byte{0xFF, 0xFE, 0x00, 0x01, 0x02}
	if err := os.WriteFile(scriptPath, binaryContent, 0600); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(scriptPath)
	// Binary file shouldn't have valid shebang
	if got != "" {
		t.Errorf("ReadShebang(binary file) = %q, want empty string", got)
	}
}

func TestReadShebangSingleByteFile(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file with only one byte (less than shebang prefix)
	scriptPath := filepath.Join(tmpDir, "single")
	if err := os.WriteFile(scriptPath, []byte("#"), 0600); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(scriptPath)
	// Too short to be a valid shebang
	if got != "" {
		t.Errorf("ReadShebang(single byte) = %q, want empty string", got)
	}
}
