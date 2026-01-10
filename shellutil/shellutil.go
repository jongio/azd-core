// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package shellutil

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Shell identifiers used for script execution.
// These constants define the supported shell types and are used
// for shell detection and command building.
const (
	// ShellBash is the Bourne Again Shell (default on most Unix systems).
	ShellBash = "bash"

	// ShellCmd is the Windows Command Prompt.
	ShellCmd = "cmd"

	// ShellPowerShell is Windows PowerShell (5.1 and earlier).
	ShellPowerShell = "powershell"

	// ShellPwsh is PowerShell Core (6.0+, cross-platform).
	ShellPwsh = "pwsh"

	// ShellSh is the POSIX shell.
	ShellSh = "sh"

	// ShellZsh is the Z Shell.
	ShellZsh = "zsh"
)

// Operating system identifiers.
const (
	// osWindows identifies the Windows operating system.
	osWindows = "windows"
)

// Environment variable names.
const (
	// EnvVarDebug enables debug output for script execution.
	// When set to "true", execution details are logged to stderr.
	EnvVarDebug = "AZD_DEBUG"
)

// File reading constants for shebang detection.
const (
	// shebangPrefix is the expected start of a shebang line ("#!").
	shebangPrefix = "#!"

	// shebangReadSize is the number of bytes to read for shebang detection.
	// This must be at least len(shebangPrefix) bytes.
	shebangReadSize = len(shebangPrefix)

	// envCommand is the common env wrapper in shebangs (e.g., #!/usr/bin/env bash).
	envCommand = "env"
)

// DetectShell auto-detects the appropriate shell based on the script extension and shebang.
// Detection priority:
//  1. File extension (.ps1, .cmd, .bat, .sh, .zsh)
//  2. Shebang line (#!/bin/bash, #!/usr/bin/env python3, etc.)
//  3. OS-specific default (Windows: cmd, Unix: bash)
//
// Returns the shell command name (e.g., "bash", "pwsh", "cmd").
func DetectShell(scriptPath string) string {
	ext := strings.ToLower(filepath.Ext(scriptPath))

	switch ext {
	case ".ps1":
		if runtime.GOOS == osWindows {
			return ShellPowerShell
		}
		return ShellPwsh
	case ".cmd", ".bat":
		return ShellCmd
	case ".sh":
		return ShellBash
	case ".zsh":
		return ShellZsh
	default:
		// Check shebang line for scripts without recognized extensions
		if shebang := ReadShebang(scriptPath); shebang != "" {
			return shebang
		}

		// Default based on OS (cmd for Windows, bash for Unix)
		if runtime.GOOS == osWindows {
			return ShellCmd
		}
		return ShellBash
	}
}

// ReadShebang reads the shebang line from a script file and extracts the shell name.
// It handles common shebang formats:
//   - #!/bin/bash
//   - #!/usr/bin/env python3
//   - #! /bin/sh
//
// Returns:
//   - Empty string if no shebang is found or file cannot be read
//   - The base name of the shell/interpreter (e.g., "bash", "python3")
func ReadShebang(scriptPath string) string {
	file, err := os.Open(scriptPath) // #nosec G304 - scriptPath is validated by caller
	if err != nil {
		return ""
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			// Log error but don't fail - we may have already read what we needed
			// Only log to stderr if we're in debug mode to avoid noise
			if os.Getenv(EnvVarDebug) == "true" {
				fmt.Fprintf(os.Stderr, "warning: failed to close file %s: %v\n", filepath.Base(scriptPath), closeErr)
			}
		}
	}()

	reader := bufio.NewReader(file)

	// Read first bytes to check for shebang
	buf := make([]byte, shebangReadSize)
	if _, readErr := io.ReadFull(reader, buf); readErr != nil {
		return ""
	}

	if string(buf) != shebangPrefix {
		return ""
	}

	// Read the rest of the line
	line, lineErr := reader.ReadString('\n')
	if lineErr != nil && lineErr != io.EOF {
		return ""
	}

	line = strings.TrimSpace(line)
	parts := strings.Fields(line)
	if len(parts) == 0 {
		return ""
	}

	// Handle "#!/usr/bin/env python3" style shebangs
	if filepath.Base(parts[0]) == envCommand && len(parts) > 1 {
		return filepath.Base(parts[1])
	}

	return filepath.Base(parts[0])
}
