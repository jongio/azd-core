// Package cmdutil provides generic command execution utilities including
// running commands with timeouts, capturing output, and monitoring output line-by-line.
package cmdutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

// DefaultTimeout is the default timeout for command execution.
const DefaultTimeout = 30 * time.Minute

// OutputLineHandler is a callback for processing output lines in real-time.
type OutputLineHandler func(line string)

// RunCommand runs a command and waits for it to complete.
// stdout and stderr go to os.Stdout and os.Stderr.
func RunCommand(ctx context.Context, name string, args []string, dir string) error {
	return RunWithContext(ctx, name, args, dir)
}

// RunWithContext runs a command with the given context for cancellation.
// The command inherits environment variables, stdout, stderr, and stdin from the parent process.
func RunWithContext(ctx context.Context, name string, args []string, dir string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	return cmd.Run()
}

// RunWithTimeout runs a command with a timeout.
func RunWithTimeout(name string, args []string, dir string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	return RunWithContext(ctx, name, args, dir)
}

// RunCommandWithOutput runs a command and returns its combined output.
// The command inherits environment variables from the parent process.
func RunCommandWithOutput(ctx context.Context, name string, args []string, dir string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("command failed: %w", err)
	}

	return output, nil
}

// StartCommand starts a command without waiting for it to complete.
// Returns the started Cmd for the caller to manage.
func StartCommand(ctx context.Context, name string, args []string, dir string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return cmd, nil
}

// StartCommandWithOutputMonitoring starts a command and calls handler for each line of output.
// Output is also written to os.Stdout/os.Stderr in real-time.
// The caller is responsible for calling cmd.Wait() on the returned Cmd.
func StartCommandWithOutputMonitoring(ctx context.Context, name string, args []string, dir string, handler OutputLineHandler) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Stdin = os.Stdin
	cmd.Env = os.Environ()

	cmd.Stdout = &lineWriter{output: os.Stdout, handler: handler}
	cmd.Stderr = &lineWriter{output: os.Stderr, handler: handler}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start command: %w", err)
	}

	return cmd, nil
}
