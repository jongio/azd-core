package cmdutil

import (
	"context"
	"io"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestRunCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// "go version" works cross-platform
	err := RunCommand(ctx, "go", []string{"version"}, "")
	if err != nil {
		t.Errorf("RunCommand() error = %v, want nil", err)
	}
}

func TestRunCommandInvalidCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := RunCommand(ctx, "nonexistent-command-xyz-123", []string{}, "")
	if err == nil {
		t.Error("RunCommand() with invalid command should fail")
	}
}

func TestRunWithContext(t *testing.T) {
	ctx := context.Background()

	err := RunWithContext(ctx, "go", []string{"version"}, "")
	if err != nil {
		t.Errorf("RunWithContext() error = %v, want nil", err)
	}
}

func TestRunWithContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd.exe"
		args = []string{"/c", "timeout", "10"}
	} else {
		name = "sleep"
		args = []string{"10"}
	}

	err := RunWithContext(ctx, name, args, "")
	if err == nil {
		t.Error("RunWithContext() with canceled context should fail")
	}
}

func TestRunWithTimeoutCompletes(t *testing.T) {
	err := RunWithTimeout("go", []string{"version"}, "", 5*time.Second)
	if err != nil {
		t.Errorf("RunWithTimeout() error = %v, want nil", err)
	}
}

func TestRunWithTimeoutExceeded(t *testing.T) {
	var name string
	var args []string
	if runtime.GOOS == "windows" {
		name = "cmd.exe"
		args = []string{"/c", "timeout", "10"}
	} else {
		name = "sleep"
		args = []string{"10"}
	}

	err := RunWithTimeout(name, args, "", 100*time.Millisecond)
	if err == nil {
		t.Error("RunWithTimeout() should timeout")
	}
}

func TestRunCommandWithOutput(t *testing.T) {
	ctx := context.Background()

	output, err := RunCommandWithOutput(ctx, "go", []string{"version"}, "")
	if err != nil {
		t.Fatalf("RunCommandWithOutput() error = %v, want nil", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if !strings.Contains(outputStr, "go version") {
		t.Errorf("RunCommandWithOutput() output = %q, want to contain %q", outputStr, "go version")
	}
}

func TestRunCommandWithOutputInvalidCommand(t *testing.T) {
	ctx := context.Background()

	_, err := RunCommandWithOutput(ctx, "nonexistent-command-xyz-123", []string{}, "")
	if err == nil {
		t.Error("RunCommandWithOutput() with invalid command should fail")
	}
}

func TestStartCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd, err := StartCommand(ctx, "go", []string{"version"}, "")
	if err != nil {
		t.Fatalf("StartCommand() error = %v, want nil", err)
	}

	if cmd == nil {
		t.Fatal("StartCommand() returned nil cmd")
	}

	if cmd.Process == nil {
		t.Fatal("StartCommand() process not started")
	}

	// Wait for it to finish
	_ = cmd.Wait()
}

func TestStartCommandInvalidCommand(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd, err := StartCommand(ctx, "nonexistent-command-xyz-123", []string{}, "")
	if err == nil {
		t.Error("StartCommand() with invalid command should fail")
	}
	if cmd != nil {
		t.Error("StartCommand() should return nil cmd on error")
	}
}

func TestLineWriter(t *testing.T) {
	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	lw := &lineWriter{
		output:  io.Discard,
		handler: handler,
	}

	// Write complete line
	_, err := lw.Write([]byte("line 1\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if len(lines) != 1 || lines[0] != "line 1" {
		t.Errorf("lines = %v, want [\"line 1\"]", lines)
	}

	// Write multiple lines at once
	_, err = lw.Write([]byte("line 2\nline 3\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	if len(lines) != 3 {
		t.Fatalf("len(lines) = %d, want 3", len(lines))
	}
	if lines[1] != "line 2" || lines[2] != "line 3" {
		t.Errorf("lines = %v", lines)
	}
}

func TestLineWriterIncomplete(t *testing.T) {
	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	lw := &lineWriter{
		output:  io.Discard,
		handler: handler,
	}

	// Write incomplete line
	_, err := lw.Write([]byte("partial"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(lines) != 0 {
		t.Fatalf("len(lines) = %d, want 0", len(lines))
	}

	// Complete the line
	_, err = lw.Write([]byte(" line\n"))
	if err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if len(lines) != 1 || lines[0] != "partial line" {
		t.Errorf("lines = %v, want [\"partial line\"]", lines)
	}
}

func TestLineWriterFlush(t *testing.T) {
	var lines []string
	handler := func(line string) {
		lines = append(lines, line)
	}

	lw := newLineWriter(handler)

	_, _ = lw.Write([]byte("no newline"))
	if len(lines) != 0 {
		t.Fatal("expected no lines before flush")
	}

	lw.Flush()
	if len(lines) != 1 || lines[0] != "no newline" {
		t.Errorf("after Flush, lines = %v, want [\"no newline\"]", lines)
	}
}

func TestExecuteHookSimple(t *testing.T) {
	ctx := context.Background()
	hook := HookConfig{
		Run:   "go version",
		Shell: GetDefaultShell(),
	}

	err := ExecuteHook(ctx, hook, ".")
	if err != nil {
		t.Errorf("ExecuteHook() error = %v, want nil", err)
	}
}

func TestExecuteHookEmpty(t *testing.T) {
	ctx := context.Background()
	hook := HookConfig{Run: ""}

	err := ExecuteHook(ctx, hook, ".")
	if err != nil {
		t.Errorf("ExecuteHook() with empty Run should return nil, got: %v", err)
	}
}

func TestExecuteHookContinueOnError(t *testing.T) {
	ctx := context.Background()

	failCmd := "exit 1"
	if runtime.GOOS == "windows" {
		failCmd = "exit /b 1"
	}

	hook := HookConfig{
		Run:             failCmd,
		Shell:           GetDefaultShell(),
		ContinueOnError: true,
	}

	err := ExecuteHook(ctx, hook, ".")
	if err != nil {
		t.Errorf("ExecuteHook() with ContinueOnError should not fail, got: %v", err)
	}
}

func TestExecuteHookFailure(t *testing.T) {
	ctx := context.Background()

	failCmd := "exit 1"
	if runtime.GOOS == "windows" {
		failCmd = "exit /b 1"
	}

	hook := HookConfig{
		Run:             failCmd,
		Shell:           GetDefaultShell(),
		ContinueOnError: false,
	}

	err := ExecuteHook(ctx, hook, ".")
	if err == nil {
		t.Error("ExecuteHook() should fail when ContinueOnError is false")
	}
}

func TestGetDefaultShell(t *testing.T) {
	shell := GetDefaultShell()
	if shell == "" {
		t.Error("GetDefaultShell() returned empty string")
	}
}
