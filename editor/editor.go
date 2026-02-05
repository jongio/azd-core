// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

// Package editor provides utilities for opening files in the user's preferred editor.
// It supports automatic detection of editors on Windows, macOS, and Linux.
package editor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
)

// OpenOptions configures how to open a file in an editor.
type OpenOptions struct {
	// Editor overrides the default editor detection.
	// If empty, uses EDITOR, VISUAL env vars, or auto-detects.
	Editor string

	// WaitForClose blocks until the editor process exits.
	// Defaults to true.
	WaitForClose bool

	// LineNumber opens the file at a specific line (if supported by editor).
	LineNumber int
}

// Open opens the specified file in the user's preferred editor.
// Uses EDITOR or VISUAL environment variables if set, otherwise auto-detects.
func Open(path string) error {
	return OpenWithOptions(path, OpenOptions{WaitForClose: true})
}

// OpenWithOptions opens a file with custom options.
func OpenWithOptions(path string, opts OpenOptions) error {
	editor := opts.Editor
	if editor == "" {
		editor = detectEditor()
	}
	if editor == "" {
		return fmt.Errorf("no editor found; set EDITOR or VISUAL environment variable")
	}

	args := buildEditorArgs(editor, path, opts.LineNumber)

	cmd := exec.Command(editor, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if opts.WaitForClose {
		return cmd.Run()
	}

	return cmd.Start()
}

// detectEditor finds an available editor on the system.
func detectEditor() string {
	// Check environment variables first - validate before using
	if editor := os.Getenv("EDITOR"); editor != "" {
		if validated := validateEditor(editor); validated != "" {
			return validated
		}
	}
	if editor := os.Getenv("VISUAL"); editor != "" {
		if validated := validateEditor(editor); validated != "" {
			return validated
		}
	}

	// Platform-specific detection
	candidates := getEditorCandidates()
	for _, candidate := range candidates {
		if _, err := exec.LookPath(candidate); err == nil {
			return candidate
		}
	}

	return ""
}

// editorNamePattern validates editor names - only alphanumeric, dash, underscore, dot
var editorNamePattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)

// validateEditor validates an editor string from environment variables.
// Returns the validated editor name if safe, empty string otherwise.
// Only allows:
// - Simple command names found in PATH (e.g., "code", "vim")
// - Absolute paths to executables (validated via LookPath)
func validateEditor(editor string) string {
	// Empty is not valid
	if editor == "" {
		return ""
	}

	// Check if it's just a command name (no path separators)
	if !filepath.IsAbs(editor) && !containsPathSeparator(editor) {
		// Simple command name - validate characters and check PATH
		if editorNamePattern.MatchString(editor) {
			if _, err := exec.LookPath(editor); err == nil {
				return editor
			}
		}
		return ""
	}

	// It's a path - must be absolute for security
	if !filepath.IsAbs(editor) {
		return ""
	}

	// Validate the path exists and is executable
	if _, err := exec.LookPath(editor); err == nil {
		return editor
	}

	return ""
}

// containsPathSeparator checks if s contains OS-specific path separators.
func containsPathSeparator(s string) bool {
	for _, c := range s {
		if c == '/' || c == filepath.Separator {
			return true
		}
	}
	return false
}

// getEditorCandidates returns a prioritized list of editors to try.
func getEditorCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		return []string{
			"code",    // VS Code
			"notepad", // Always available
			"notepad++",
		}
	case "darwin":
		return []string{
			"code", // VS Code
			"subl", // Sublime Text
			"mate", // TextMate
			"nano", // Usually available
			"vim",  // Usually available
			"open", // macOS default (opens in default app)
		}
	default: // Linux and others
		return []string{
			"code",     // VS Code
			"subl",     // Sublime Text
			"gedit",    // GNOME
			"kate",     // KDE
			"nano",     // Usually available
			"vim",      // Usually available
			"vi",       // Always available
			"xdg-open", // Opens in default app
		}
	}
}

// buildEditorArgs builds command arguments for the editor.
func buildEditorArgs(editor, path string, lineNumber int) []string {
	args := []string{}

	// Handle line number for common editors
	if lineNumber > 0 {
		switch {
		case contains(editor, "code"):
			// VS Code: code --goto file:line
			args = append(args, "--goto", fmt.Sprintf("%s:%d", path, lineNumber))
			return args
		case contains(editor, "vim"), contains(editor, "vi"), contains(editor, "nvim"):
			// Vim: vim +line file
			args = append(args, fmt.Sprintf("+%d", lineNumber), path)
			return args
		case contains(editor, "nano"):
			// Nano: nano +line file
			args = append(args, fmt.Sprintf("+%d", lineNumber), path)
			return args
		case contains(editor, "subl"):
			// Sublime: subl file:line
			args = append(args, fmt.Sprintf("%s:%d", path, lineNumber))
			return args
		case contains(editor, "notepad++"):
			// Notepad++: notepad++ -n123 file
			args = append(args, fmt.Sprintf("-n%d", lineNumber), path)
			return args
		}
	}

	// Default: just the path
	args = append(args, path)
	return args
}

// contains checks if needle is in haystack (case-insensitive partial match).
func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) &&
		(haystack == needle ||
			len(haystack) > len(needle) &&
				(haystack[len(haystack)-len(needle):] == needle ||
					haystack[:len(needle)] == needle))
}

// GetDefaultEditor returns the name of the detected default editor.
// Returns empty string if no editor is found.
func GetDefaultEditor() string {
	return detectEditor()
}

// IsEditorAvailable checks if a specific editor is available on the system.
func IsEditorAvailable(editor string) bool {
	_, err := exec.LookPath(editor)
	return err == nil
}
