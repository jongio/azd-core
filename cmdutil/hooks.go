package cmdutil

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Shell type constants for platform-specific shell detection.
const (
	ShellSh         = "sh"
	ShellBash       = "bash"
	ShellPwsh       = "pwsh"
	ShellPowerShell = "powershell"
	ShellCmd        = "cmd"
	ShellZsh        = "zsh"
)

// HookConfig defines a lifecycle hook to execute.
type HookConfig struct {
	Run             string   // Command to run
	Shell           string   // Shell to use (bash, sh, pwsh, cmd, zsh) - auto-detected if empty
	ContinueOnError bool     // Don't fail on non-zero exit
	Interactive     bool     // Pass through stdin
	Env             []string // Additional environment variables (KEY=VALUE format)
}

// ExecuteHook runs a lifecycle hook command with the specified shell.
func ExecuteHook(ctx context.Context, hook HookConfig, dir string) error {
	if hook.Run == "" {
		return nil
	}

	shell := hook.Shell
	if shell == "" {
		shell = GetDefaultShell()
	}

	cmd := prepareHookCommand(ctx, shell, hook.Run, dir, hook.Env)
	configureCommandIO(cmd, hook.Interactive)

	err := cmd.Run()
	if err != nil {
		if hook.ContinueOnError {
			return nil
		}
		return fmt.Errorf("hook failed: %w", err)
	}

	return nil
}

// GetDefaultShell returns the default shell for the current platform.
func GetDefaultShell() string {
	if runtime.GOOS == "windows" {
		if _, err := exec.LookPath(ShellPwsh); err == nil {
			return ShellPwsh
		}
		if _, err := exec.LookPath(ShellPowerShell); err == nil {
			return ShellPowerShell
		}
		return ShellCmd
	}
	if _, err := exec.LookPath(ShellBash); err == nil {
		return ShellBash
	}
	return ShellSh
}

// isScriptFilePath checks if the run value appears to be a path to a script file
// rather than inline commands.
func isScriptFilePath(script string) bool {
	trimmed := strings.TrimSpace(script)
	if trimmed == "" {
		return false
	}

	pathPrefixes := []string{"./", "../", "/", ".\\", "..\\"}
	for _, prefix := range pathPrefixes {
		if strings.HasPrefix(trimmed, prefix) {
			parts := strings.Fields(trimmed)
			return len(parts) == 1
		}
	}
	return false
}

// prepareHookCommand prepares the command based on the shell and script.
func prepareHookCommand(ctx context.Context, shell, script, workingDir string, envVars []string) *exec.Cmd {
	var cmd *exec.Cmd

	shellLower := strings.ToLower(shell)
	isFilePath := isScriptFilePath(script)

	switch {
	case strings.Contains(shellLower, "pwsh") || strings.Contains(shellLower, "powershell"):
		if isFilePath && strings.HasSuffix(strings.ToLower(strings.TrimSpace(script)), ".ps1") {
			cmd = exec.CommandContext(ctx, shell, "-File", strings.TrimSpace(script))
		} else {
			wrappedScript := fmt.Sprintf("[Console]::OutputEncoding = [System.Text.Encoding]::UTF8; %s", script)
			cmd = exec.CommandContext(ctx, shell, "-Command", wrappedScript)
		}
	case strings.Contains(shellLower, "cmd"):
		cmd = exec.CommandContext(ctx, shell, "/c", script)
	default:
		if isFilePath {
			cmd = exec.CommandContext(ctx, shell, script)
		} else {
			cmd = exec.CommandContext(ctx, shell, "-c", script)
		}
	}

	cmd.Dir = workingDir
	cmd.Env = os.Environ()

	if len(envVars) > 0 {
		cmd.Env = append(cmd.Env, envVars...)
	}

	return cmd
}

// configureCommandIO configures stdin, stdout, and stderr for the command.
func configureCommandIO(cmd *exec.Cmd, interactive bool) {
	if interactive {
		cmd.Stdin = os.Stdin
	} else {
		cmd.Stdin = nil
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
}
