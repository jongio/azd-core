// Package shellutil provides shell detection and command building utilities.
//
// This package offers robust shell detection from multiple sources including
// file extensions, shebang lines, and OS-specific defaults. It provides
// shell identifier constants for consistent shell identification across
// Azure Developer CLI tools and extensions.
//
// # Key Features
//
// - Detect shell from file extension (.ps1 → pwsh, .sh → bash, .cmd → cmd)
// - Parse shebang lines (#!/bin/bash, #!/usr/bin/env python3)
// - OS-specific default shell detection (Windows → cmd, Unix → bash)
// - Shell identifier constants (ShellBash, ShellPwsh, ShellCmd, ShellZsh, ShellSh)
// - Cross-platform PowerShell handling (powershell on Windows, pwsh elsewhere)
//
// # Shell Detection Priority
//
// The DetectShell function uses the following priority for shell detection:
//  1. File extension (.ps1, .sh, .cmd, .bat, .zsh)
//  2. Shebang line (if present and parseable)
//  3. OS-specific default (cmd on Windows, bash on Unix)
//
// # Shebang Parsing
//
// The ReadShebang function supports both direct and env-based shebangs:
//   - #!/bin/bash → bash
//   - #!/usr/bin/env python3 → python3
//   - #!/usr/bin/zsh → zsh
//   - #!/bin/sh → sh
//   - #! /bin/bash (with space) → bash
//
// # Supported Shells
//
// The package recognizes these shells through constants:
//   - ShellBash (bash - Bourne Again Shell, Unix default)
//   - ShellSh (sh - POSIX shell)
//   - ShellZsh (zsh - Z Shell)
//   - ShellPwsh (pwsh - PowerShell Core, cross-platform)
//   - ShellPowerShell (powershell - Windows PowerShell 5.1)
//   - ShellCmd (cmd - Windows Command Prompt)
//
// Additionally, shebang parsing recognizes interpreters like:
//   - python, python3 (Python scripts)
//   - node (Node.js scripts)
//   - ruby, perl, php, etc.
//
// # Example Usage
//
// Basic shell detection from file path:
//
//	shell := shellutil.DetectShell("deploy.sh")
//	// Returns: "bash"
//
//	shell := shellutil.DetectShell("deploy.ps1")
//	// Returns: "powershell" on Windows, "pwsh" elsewhere
//
//	shell := shellutil.DetectShell("install.bat")
//	// Returns: "cmd"
//
// Detect shell from a script without extension (checks shebang):
//
//	shell := shellutil.DetectShell("script")
//	// If script starts with #!/bin/bash, returns: "bash"
//	// If no shebang, returns OS default: "cmd" on Windows, "bash" on Unix
//
// Read shebang directly:
//
//	shell := shellutil.ReadShebang("myscript")
//	// If file starts with #!/usr/bin/env python3, returns: "python3"
//	// If no shebang or file doesn't exist, returns: ""
//
// Use constants for comparison:
//
//	shell := shellutil.DetectShell(scriptPath)
//	switch shell {
//	case shellutil.ShellBash, shellutil.ShellSh:
//	    // Handle POSIX shell
//	case shellutil.ShellPwsh, shellutil.ShellPowerShell:
//	    // Handle PowerShell
//	case shellutil.ShellCmd:
//	    // Handle Windows cmd
//	case shellutil.ShellZsh:
//	    // Handle zsh
//	}
//
// # Extension Mapping
//
// File extension to shell mapping:
//   - .ps1 → powershell (Windows) or pwsh (Unix/macOS)
//   - .sh → bash
//   - .zsh → zsh
//   - .cmd, .bat → cmd
//   - (no extension) → shebang detection, then OS default
//
// # OS-Specific Behavior
//
// Windows:
//   - .ps1 scripts use "powershell" (Windows PowerShell 5.1)
//   - Default shell for scripts without extension: "cmd"
//   - Batch files (.cmd, .bat) use "cmd"
//
// Unix/macOS:
//   - .ps1 scripts use "pwsh" (PowerShell Core)
//   - Default shell for scripts without extension: "bash"
//   - Shell scripts (.sh) use "bash"
//
// # Error Handling
//
// - ReadShebang returns empty string ("") for errors (file not found, permission denied, etc.)
// - DetectShell always returns a valid shell name (falls back to OS default)
// - No panics or fatal errors - functions are safe for all inputs
//
// # Security Considerations
//
// - File paths are validated by callers before passing to these functions
// - Shebang parsing reads only the first line (limited buffer)
// - File descriptors are properly closed even on errors
// - Debug output (when AZD_DEBUG=true) goes to stderr only
//
// # Testing
//
// The package includes comprehensive tests covering:
//   - Extension-based detection (.ps1, .sh, .cmd, .bat, .zsh)
//   - Shebang parsing (various formats)
//   - OS-specific behavior (Windows vs Unix)
//   - Error cases (missing files, permission errors)
//   - Edge cases (empty files, malformed shebangs, etc.)
package shellutil
