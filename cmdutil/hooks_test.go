// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package cmdutil

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestIsScriptFilePath(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"bash script", "hook.sh", true},
		{"powershell ps1", "hook.ps1", true},
		{"powershell psm1", "hook.psm1", true},
		{"batch cmd", "hook.cmd", true},
		{"batch bat", "hook.bat", true},
		{"python", "hook.py", true},
		{"no extension", "hook", false},
		{"go file", "main.go", false},
		{"empty string", "", false},
		{"dot only", ".", false},
		{"hidden sh", ".hook.sh", true},
		{"path with dirs", "scripts/pre-deploy.sh", true},
		{"uppercase ext", "hook.SH", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isScriptFilePath(tc.path)
			if got != tc.want {
				t.Errorf("isScriptFilePath(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestPrepareHookCommand_ShellSelection(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Shell detection differs on Windows")
	}
	tmpDir := t.TempDir()
	script := filepath.Join(tmpDir, "test.sh")
	os.WriteFile(script, []byte("#!/bin/bash\necho hello"), 0o755)

	cmd := prepareHookCommand(context.Background(), "bash", script, tmpDir, nil)
	if cmd == nil {
		t.Fatal("prepareHookCommand returned nil")
	}
	if cmd.Path == "" {
		t.Error("command Path is empty")
	}
}

func TestPrepareHookCommand_EnvVars(t *testing.T) {
	tmpDir := t.TempDir()
	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(tmpDir, "test.cmd")
		os.WriteFile(script, []byte("@echo off\r\necho hello"), 0o644)
	} else {
		script = filepath.Join(tmpDir, "test.sh")
		os.WriteFile(script, []byte("#!/bin/sh\necho hello"), 0o755)
	}

	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "cmd"
	}
	cmd := prepareHookCommand(context.Background(), shell, script, tmpDir, []string{"HOOK_TEST_VAR=1"})
	if cmd == nil {
		t.Fatal("prepareHookCommand returned nil")
	}
	found := false
	for _, e := range cmd.Env {
		if e == "HOOK_TEST_VAR=1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected HOOK_TEST_VAR to be appended to command env")
	}
}

func TestConfigureCommandIO_Interactive(t *testing.T) {
	tmpDir := t.TempDir()
	var script string
	if runtime.GOOS == "windows" {
		script = filepath.Join(tmpDir, "test.cmd")
		os.WriteFile(script, []byte("@echo off"), 0o644)
	} else {
		script = filepath.Join(tmpDir, "test.sh")
		os.WriteFile(script, []byte("#!/bin/sh"), 0o755)
	}

	shell := "bash"
	if runtime.GOOS == "windows" {
		shell = "cmd"
	}
	cmd := prepareHookCommand(context.Background(), shell, script, tmpDir, nil)
	if cmd == nil {
		t.Fatal("prepareHookCommand returned nil")
	}
	configureCommandIO(cmd, true)
	if cmd.Stdout != os.Stdout {
		t.Error("interactive mode should set Stdout to os.Stdout")
	}
	if cmd.Stderr != os.Stderr {
		t.Error("interactive mode should set Stderr to os.Stderr")
	}
}

func TestExecuteHook_EmptyPath(t *testing.T) {
	err := ExecuteHook(context.Background(), HookConfig{Run: ""}, ".")
	if err != nil {
		t.Errorf("ExecuteHook with empty Run should return nil, got %v", err)
	}
}

func TestExecuteHook_NonExistentScript(t *testing.T) {
	err := ExecuteHook(context.Background(), HookConfig{Run: "/nonexistent/path/hook.sh"}, ".")
	if err == nil {
		t.Error("ExecuteHook with non-existent script should return error")
	}
}
