package azdextutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
