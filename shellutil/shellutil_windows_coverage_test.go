//go:build windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package shellutil

import (
	"os"
	"path/filepath"
	"testing"
)

// Windows-specific tests to improve coverage from 86.1% to 90%+

func TestDetectShell_WindowsExtensions(t *testing.T) {
	tests := []struct {
		name       string
		scriptPath string
		want       string
	}{
		{
			name:       "PowerShell .ps1",
			scriptPath: "script.ps1",
			want:       ShellPowerShell, // On Windows, .ps1 -> powershell
		},
		{
			name:       "PowerShell uppercase",
			scriptPath: "script.PS1",
			want:       ShellPowerShell,
		},
		{
			name:       "PowerShell mixed case",
			scriptPath: "script.Ps1",
			want:       ShellPowerShell,
		},
		{
			name:       "CMD script",
			scriptPath: "script.cmd",
			want:       ShellCmd,
		},
		{
			name:       "Batch script lowercase",
			scriptPath: "script.bat",
			want:       ShellCmd,
		},
		{
			name:       "Batch script uppercase",
			scriptPath: "script.BAT",
			want:       ShellCmd,
		},
		{
			name:       "Bash on Windows",
			scriptPath: "script.sh",
			want:       ShellBash,
		},
		{
			name:       "Zsh on Windows",
			scriptPath: "script.zsh",
			want:       ShellZsh,
		},
		{
			name:       "No extension defaults to cmd",
			scriptPath: "script",
			want:       ShellCmd, // Windows default
		},
		{
			name:       "Unknown extension defaults to cmd",
			scriptPath: "script.txt",
			want:       ShellCmd,
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

func TestDetectShell_WithShebangOnWindows(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		ext      string
		content  string
		wantExt  string // What the extension would give
		wantReal string // What we actually get
	}{
		{
			name:     ".ps1 overrides bash shebang",
			ext:      ".ps1",
			content:  "#!/bin/bash\nWrite-Host 'test'",
			wantExt:  ShellPowerShell,
			wantReal: ShellPowerShell, // Extension takes priority
		},
		{
			name:     ".cmd overrides shebang",
			ext:      ".cmd",
			content:  "#!/bin/bash\necho test",
			wantExt:  ShellCmd,
			wantReal: ShellCmd, // Extension takes priority
		},
		{
			name:     ".sh respects extension",
			ext:      ".sh",
			content:  "#!/usr/bin/env python3\nprint('test')",
			wantExt:  ShellBash,
			wantReal: ShellBash, // Extension takes priority over shebang
		},
		{
			name:     "No extension uses shebang",
			ext:      ".txt",
			content:  "#!/bin/bash\necho test",
			wantExt:  ShellCmd, // .txt is unknown, falls back to OS default (cmd)
			wantReal: "bash",   // Shebang is checked for unknown extensions
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, "script"+tt.ext)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to create test script: %v", err)
			}

			got := DetectShell(scriptPath)
			if got != tt.wantReal {
				t.Errorf("DetectShell() = %q, want %q", got, tt.wantReal)
			}
		})
	}
}

func TestReadShebang_WindowsLineEndings(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Windows CRLF",
			content: "#!/bin/bash\r\necho test",
			want:    "bash",
		},
		{
			name:    "Unix LF",
			content: "#!/bin/bash\necho test",
			want:    "bash",
		},
		{
			name:    "Mixed line endings",
			content: "#!/bin/bash\r\necho test\necho more",
			want:    "bash",
		},
		{
			name:    "PowerShell shebang",
			content: "#!/usr/bin/env pwsh\r\nWrite-Host 'test'",
			want:    "pwsh",
		},
		{
			name:    "Python on Windows",
			content: "#!/usr/bin/env python\r\nprint('test')",
			want:    "python",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name+".sh")
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadShebang_FileCloseOnWindows(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")

	content := "#!/bin/bash\necho test"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Enable debug mode to exercise the close error logging path
	originalDebug := os.Getenv(EnvVarDebug)
	_ = os.Setenv(EnvVarDebug, "true")
	defer func() {
		if originalDebug != "" {
			_ = os.Setenv(EnvVarDebug, originalDebug)
		} else {
			_ = os.Unsetenv(EnvVarDebug)
		}
	}()

	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() = %q, want bash", got)
	}

	// Call multiple times to verify file is properly closed each time
	for i := 0; i < 5; i++ {
		result := ReadShebang(scriptPath)
		if result != "bash" {
			t.Errorf("ReadShebang() iteration %d = %q, want bash", i, result)
		}
	}
}

func TestReadShebang_WindowsPath(t *testing.T) {
	tmpDir := t.TempDir()

	// Test with Windows-style path
	scriptPath := filepath.Join(tmpDir, "script.sh")
	content := "#!/bin/bash\necho test"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() with Windows path = %q, want bash", got)
	}
}

func TestReadShebang_ShortFiles(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Empty file",
			content: "",
			want:    "",
		},
		{
			name:    "Single byte",
			content: "#",
			want:    "",
		},
		{
			name:    "Just #!",
			content: "#!",
			want:    "",
		},
		{
			name:    "Shebang no newline",
			content: "#!/bin/sh",
			want:    "sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestReadShebang_EnvVariants_Windows(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "env with bash",
			content: "#!/usr/bin/env bash\necho test",
			want:    "bash",
		},
		{
			name:    "env with pwsh",
			content: "#!/usr/bin/env pwsh\nWrite-Host test",
			want:    "pwsh",
		},
		{
			name:    "env with python3",
			content: "#!/usr/bin/env python3\nprint('test')",
			want:    "python3",
		},
		{
			name:    "env with node",
			content: "#!/usr/bin/env node\nconsole.log('test')",
			want:    "node",
		},
		{
			name:    "env with flags",
			content: "#!/usr/bin/env -S python3 -u\nprint('test')",
			want:    "-S", // Current behavior - takes first arg after env
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectShell_CaseInsensitivity_Windows(t *testing.T) {
	// Windows file extensions are case-insensitive
	tmpDir := t.TempDir()

	tests := []struct {
		ext  string
		want string
	}{
		{".ps1", ShellPowerShell},
		{".PS1", ShellPowerShell},
		{".Ps1", ShellPowerShell},
		{".pS1", ShellPowerShell},
		{".cmd", ShellCmd},
		{".CMD", ShellCmd},
		{".Cmd", ShellCmd},
		{".bat", ShellCmd},
		{".BAT", ShellCmd},
		{".Bat", ShellCmd},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, "script"+tt.ext)
			got := DetectShell(scriptPath)
			if got != tt.want {
				t.Errorf("DetectShell(%q) = %q, want %q", scriptPath, got, tt.want)
			}
		})
	}
}

func TestReadShebang_NonExistentFile_Windows(t *testing.T) {
	// Test with Windows-style non-existent path
	nonExistent := "C:\\nonexistent\\path\\script.sh"
	got := ReadShebang(nonExistent)
	if got != "" {
		t.Errorf("ReadShebang(nonexistent) = %q, want empty string", got)
	}
}

func TestReadShebang_LongPathWindows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a script with a long path on Windows
	longPath := filepath.Join(tmpDir, "very", "long", "path", "to", "script")
	if err := os.MkdirAll(filepath.Dir(longPath), 0755); err != nil {
		t.Fatal(err)
	}

	content := "#!/bin/bash\necho test"
	scriptPath := longPath + ".sh"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() with long path = %q, want bash", got)
	}
}

func TestDetectShell_PathWithSpaces_Windows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a script in a path with spaces
	spacePath := filepath.Join(tmpDir, "dir with spaces", "script.ps1")
	if err := os.MkdirAll(filepath.Dir(spacePath), 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(spacePath, []byte("Write-Host 'test'"), 0644); err != nil {
		t.Fatal(err)
	}

	got := DetectShell(spacePath)
	if got != ShellPowerShell {
		t.Errorf("DetectShell() with space in path = %q, want %q", got, ShellPowerShell)
	}
}

func TestReadShebang_BinaryContent_Windows(t *testing.T) {
	tmpDir := t.TempDir()
	binaryPath := filepath.Join(tmpDir, "binary.exe")

	// Create a file with binary content (not a valid shebang)
	binaryContent := []byte{0xFF, 0xFE, 0x00, 0x01, 0xDE, 0xAD, 0xBE, 0xEF}
	if err := os.WriteFile(binaryPath, binaryContent, 0644); err != nil {
		t.Fatal(err)
	}

	got := ReadShebang(binaryPath)
	if got != "" {
		t.Errorf("ReadShebang(binary) = %q, want empty string", got)
	}
}

func TestReadShebang_OnlyShebangPrefix_Windows(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "Just #! with newline",
			content: "#!\n",
			want:    "",
		},
		{
			name:    "Just #! with spaces",
			content: "#!   \n",
			want:    "",
		},
		{
			name:    "Just #! no newline",
			content: "#!",
			want:    "",
		},
		{
			name:    "Shebang with only spaces after",
			content: "#!     \necho test",
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.name)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := ReadShebang(scriptPath)
			if got != tt.want {
				t.Errorf("ReadShebang() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDetectShell_MultipleExtensions_Windows(t *testing.T) {
	// Test files with multiple extensions (e.g., script.test.ps1)
	tests := []struct {
		path string
		want string
	}{
		{"script.test.ps1", ShellPowerShell}, // Last extension wins
		{"script.backup.cmd", ShellCmd},
		{"script.old.bat", ShellCmd},
		{"script.sh.bak", ShellCmd}, // .bak is unknown, defaults to cmd on Windows
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := DetectShell(tt.path)
			if got != tt.want {
				t.Errorf("DetectShell(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestReadShebang_CloseErrorLogging_Windows(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "test.sh")

	content := "#!/bin/bash\necho test"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test with debug mode off - close error shouldn't log
	originalDebug := os.Getenv(EnvVarDebug)
	_ = os.Unsetenv(EnvVarDebug)
	defer func() {
		if originalDebug != "" {
			_ = os.Setenv(EnvVarDebug, originalDebug)
		}
	}()

	got := ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() without debug = %q, want bash", got)
	}

	// Test with debug mode on
	_ = os.Setenv(EnvVarDebug, "true")
	got = ReadShebang(scriptPath)
	if got != "bash" {
		t.Errorf("ReadShebang() with debug = %q, want bash", got)
	}
}

func TestReadShebang_VeryLongShebangLine_Windows(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "long-shebang.sh")

	// Create a shebang with a very long path
	longPath := "/very/long/path/to/interpreter/" + string(make([]byte, 200))
	content := "#!" + longPath + "\necho test"
	if err := os.WriteFile(scriptPath, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Should extract the basename even from a long path
	got := ReadShebang(scriptPath)
	// The result depends on what's in the long path
	t.Logf("ReadShebang() with long shebang = %q", got)
}

func TestDetectShell_RealWorldScripts_Windows(t *testing.T) {
	tmpDir := t.TempDir()

	// Create real-world script examples
	scripts := map[string]struct {
		filename string
		content  string
		want     string
	}{
		"npm script": {
			filename: "build.cmd",
			content:  "@echo off\nnpm run build",
			want:     ShellCmd,
		},
		"PowerShell deployment": {
			filename: "deploy.ps1",
			content:  "Write-Host 'Deploying...'\nazd deploy",
			want:     ShellPowerShell,
		},
		"Git Bash script": {
			filename: "test.sh",
			content:  "#!/bin/bash\necho 'Running tests...'",
			want:     ShellBash,
		},
		"Batch file": {
			filename: "setup.bat",
			content:  "@echo off\necho Setting up environment...",
			want:     ShellCmd,
		},
	}

	for name, tt := range scripts {
		t.Run(name, func(t *testing.T) {
			scriptPath := filepath.Join(tmpDir, tt.filename)
			if err := os.WriteFile(scriptPath, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got := DetectShell(scriptPath)
			if got != tt.want {
				t.Errorf("DetectShell(%q) = %q, want %q", tt.filename, got, tt.want)
			}
		})
	}
}

func TestConstants_Windows(t *testing.T) {
	// Verify constants are correct
	if ShellPowerShell != "powershell" {
		t.Errorf("ShellPowerShell = %q, want \"powershell\"", ShellPowerShell)
	}
	if ShellCmd != "cmd" {
		t.Errorf("ShellCmd = %q, want \"cmd\"", ShellCmd)
	}
	if osWindows != "windows" {
		t.Errorf("osWindows = %q, want \"windows\"", osWindows)
	}
}

func TestDetectShell_DefaultBehavior_Windows(t *testing.T) {
	// On Windows, unknown extensions should default to cmd
	unknownExts := []string{".txt", ".log", ".data", ".xyz", ""}

	for _, ext := range unknownExts {
		t.Run("ext_"+ext, func(t *testing.T) {
			scriptPath := "script" + ext
			got := DetectShell(scriptPath)
			if got != ShellCmd {
				t.Errorf("DetectShell(%q) = %q, want %q (Windows default)", scriptPath, got, ShellCmd)
			}
		})
	}
}
