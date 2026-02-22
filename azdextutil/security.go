package azdextutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ValidatePath ensures a path is safe to access:
//   - Resolves to absolute path
//   - No path traversal (..)
//   - Must be within allowed base directories
//
// Returns the cleaned absolute path or an error.
func ValidatePath(path string, allowedBases ...string) (string, error) {
	if path == "" {
		return "", fmt.Errorf("path cannot be empty")
	}

	// Resolve to absolute
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Clean the path
	absPath = filepath.Clean(absPath)

	// Check for path traversal in the original input
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("path traversal not allowed")
	}

	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		// File may not exist yet, use the cleaned absolute path
		realPath = absPath
	}

	// If allowed bases specified, validate containment
	if len(allowedBases) > 0 {
		allowed := false
		for _, base := range allowedBases {
			absBase, err := filepath.Abs(base)
			if err != nil {
				continue
			}
			absBase = filepath.Clean(absBase)
			if strings.HasPrefix(realPath, absBase+string(filepath.Separator)) || realPath == absBase {
				allowed = true
				break
			}
		}
		if !allowed {
			return "", fmt.Errorf("path %q is outside allowed directories", filepath.Base(path))
		}
	}

	return realPath, nil
}

// ValidateShellName validates that a shell name is one of the known safe values.
func ValidateShellName(shell string) error {
	validShells := map[string]bool{
		"bash": true, "sh": true, "zsh": true,
		"pwsh": true, "powershell": true, "cmd": true,
	}
	if shell != "" && !validShells[strings.ToLower(shell)] {
		return fmt.Errorf("invalid shell %q: must be one of bash, sh, zsh, pwsh, powershell, cmd", shell)
	}
	return nil
}

// GetProjectDir returns the project directory from the specified environment variable,
// falling back to the current working directory. Validates the path is safe.
func GetProjectDir(envVar string) (string, error) {
	dir := os.Getenv(envVar)
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("failed to get working directory: %w", err)
		}
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve project directory: %w", err)
	}

	return filepath.Clean(absDir), nil
}
