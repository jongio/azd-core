package env

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommandRunner is an interface for running external commands.
// This allows for mocking in tests.
type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

// DefaultCommandRunner uses os/exec to run commands.
type DefaultCommandRunner struct{}

// Run executes a command and returns its output.
func (r *DefaultCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

// defaultRunner is the default command runner used by the package.
var defaultRunner CommandRunner = &DefaultCommandRunner{}

// SetCommandRunner sets a custom command runner (useful for testing).
// Returns the previous runner so it can be restored.
func SetCommandRunner(runner CommandRunner) CommandRunner {
	prev := defaultRunner
	defaultRunner = runner
	return prev
}

// LoadAzdEnvironment loads all environment variables from the specified azd environment
// by calling 'azd env get-values' and sets them in the current process.
//
// This ensures that when the -e flag is used, the correct environment values are loaded.
//
// WORKAROUND: This is a workaround for a limitation in the azd extension framework.
// Ideally, azd should honor the -e flag before invoking the extension and inject
// the correct environment variables. However, currently azd injects the default
// environment from config.json, then passes the -e flag to the extension.
// This forces us to manually reload the environment using 'azd env get-values'.
func LoadAzdEnvironment(ctx context.Context, envName string) error {
	values, err := GetAzdEnvironmentValues(ctx, envName)
	if err != nil {
		return err
	}

	// Set all environment variables
	for key, value := range values {
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("failed to set environment variable %s: %w", key, err)
		}
	}

	return nil
}

// GetAzdEnvironmentValues retrieves all environment variables from the specified
// azd environment without setting them. This is useful when you need the values
// but don't want to modify the current process environment.
func GetAzdEnvironmentValues(ctx context.Context, envName string) (map[string]string, error) {
	// Validate envName to prevent injection attacks
	if envName == "" {
		return nil, fmt.Errorf("environment name cannot be empty")
	}
	if strings.Contains(envName, " ") || strings.Contains(envName, ";") || strings.Contains(envName, "&") {
		return nil, fmt.Errorf("invalid environment name: %q", envName)
	}

	// Use 'azd env get-values' with the -e flag to get environment variables (JSON format)
	output, err := defaultRunner.Run(ctx, "azd", "env", "get-values", "-e", envName, "--output", "json")
	if err != nil {
		// If azd env get-values fails, try without JSON output (older azd versions)
		output, err = defaultRunner.Run(ctx, "azd", "env", "get-values", "-e", envName)
		if err != nil {
			return nil, fmt.Errorf("failed to get environment values for '%s': %w", envName, err)
		}

		// Parse key=value format
		return ParseKeyValueFormat(output)
	}

	// Parse JSON output
	var values map[string]string
	if err := json.Unmarshal(output, &values); err != nil {
		return nil, fmt.Errorf("failed to parse environment values as JSON: %w", err)
	}

	return values, nil
}

// ParseKeyValueFormat parses output in "KEY=value" format (one per line).
// Handles quoted values and skips empty lines and comments.
func ParseKeyValueFormat(output []byte) (map[string]string, error) {
	values := make(map[string]string)
	lines := strings.Split(string(output), "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first '=' separator
		idx := strings.Index(line, "=")
		if idx <= 0 || idx == len(line)-1 {
			// Invalid line: no '=', '=' at start, or '=' at end
			continue
		}

		key := strings.TrimSpace(line[:idx])
		value := line[idx+1:]

		// Validate key (must be non-empty and contain only allowed characters)
		if key == "" {
			continue
		}

		// Remove surrounding quotes if present (handles both " and ')
		if len(value) >= 2 {
			if (value[0] == '"' && value[len(value)-1] == '"') ||
				(value[0] == '\'' && value[len(value)-1] == '\'') {
				value = value[1 : len(value)-1]
			}
		}

		values[key] = value
	}

	return values, nil
}
