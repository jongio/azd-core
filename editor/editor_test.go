// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package editor

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectEditor(t *testing.T) {
	// Save and restore EDITOR env var
	originalEditor := os.Getenv("EDITOR")
	originalVisual := os.Getenv("VISUAL")
	defer func() {
		os.Setenv("EDITOR", originalEditor)
		os.Setenv("VISUAL", originalVisual)
	}()

	t.Run("uses EDITOR env var with valid editor", func(t *testing.T) {
		// Set EDITOR to a known valid editor (notepad on Windows, vi on Unix)
		var testEditor string
		if runtime.GOOS == "windows" {
			testEditor = "notepad"
		} else {
			// Try to find a common editor on Unix
			for _, ed := range []string{"vi", "vim", "nano"} {
				if _, err := exec.LookPath(ed); err == nil {
					testEditor = ed
					break
				}
			}
			if testEditor == "" {
				t.Skip("No common editor found on Unix")
			}
		}

		os.Setenv("EDITOR", testEditor)
		os.Setenv("VISUAL", "")

		editor := detectEditor()
		if editor != testEditor {
			t.Errorf("detectEditor() = %q, want %q", editor, testEditor)
		}
	})

	t.Run("rejects invalid EDITOR env var", func(t *testing.T) {
		os.Setenv("EDITOR", "nonexistent-editor-xyz-123")
		os.Setenv("VISUAL", "")

		editor := detectEditor()
		// Should fall back to auto-detection, not use the invalid editor
		if editor == "nonexistent-editor-xyz-123" {
			t.Error("detectEditor() should reject nonexistent editor")
		}
	})

	t.Run("uses VISUAL if EDITOR not set", func(t *testing.T) {
		var testEditor string
		if runtime.GOOS == "windows" {
			testEditor = "notepad"
		} else {
			for _, ed := range []string{"vi", "vim", "nano"} {
				if _, err := exec.LookPath(ed); err == nil {
					testEditor = ed
					break
				}
			}
			if testEditor == "" {
				t.Skip("No common editor found on Unix")
			}
		}

		os.Setenv("EDITOR", "")
		os.Setenv("VISUAL", testEditor)

		editor := detectEditor()
		if editor != testEditor {
			t.Errorf("detectEditor() = %q, want %q", editor, testEditor)
		}
	})

	t.Run("auto-detects editor when env vars not set", func(t *testing.T) {
		os.Setenv("EDITOR", "")
		os.Setenv("VISUAL", "")

		editor := detectEditor()
		// Should find something (notepad on Windows, vim/nano on Unix)
		if runtime.GOOS == "windows" && editor == "" {
			t.Error("detectEditor() should find notepad on Windows")
		}
		// On other platforms, it may or may not find something depending on what's installed
	})
}

func TestGetEditorCandidates(t *testing.T) {
	candidates := getEditorCandidates()
	if len(candidates) == 0 {
		t.Error("getEditorCandidates() returned empty list")
	}

	// Check that code is first choice on all platforms
	if candidates[0] != "code" {
		t.Errorf("getEditorCandidates()[0] = %q, want %q", candidates[0], "code")
	}
}

func TestBuildEditorArgs(t *testing.T) {
	tests := []struct {
		name       string
		editor     string
		path       string
		lineNumber int
		wantLen    int
	}{
		{
			name:       "simple path",
			editor:     "notepad",
			path:       "test.txt",
			lineNumber: 0,
			wantLen:    1,
		},
		{
			name:       "vim with line number",
			editor:     "vim",
			path:       "test.txt",
			lineNumber: 42,
			wantLen:    2,
		},
		{
			name:       "code with line number",
			editor:     "code",
			path:       "test.txt",
			lineNumber: 42,
			wantLen:    2,
		},
		{
			name:       "nano with line number",
			editor:     "nano",
			path:       "test.txt",
			lineNumber: 10,
			wantLen:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := buildEditorArgs(tt.editor, tt.path, tt.lineNumber)
			if len(args) != tt.wantLen {
				t.Errorf("buildEditorArgs() returned %d args, want %d; args=%v", len(args), tt.wantLen, args)
			}
		})
	}
}

func TestGetDefaultEditor(t *testing.T) {
	// Just verify it doesn't panic
	_ = GetDefaultEditor()
}

func TestIsEditorAvailable(t *testing.T) {
	t.Run("notepad on Windows", func(t *testing.T) {
		if runtime.GOOS != "windows" {
			t.Skip("Windows-only test")
		}
		if !IsEditorAvailable("notepad") {
			t.Error("IsEditorAvailable(notepad) = false on Windows")
		}
	})

	t.Run("nonexistent editor", func(t *testing.T) {
		if IsEditorAvailable("nonexistent-editor-xyz-123") {
			t.Error("IsEditorAvailable() should return false for nonexistent editor")
		}
	})
}

func TestOpen(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("test content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	t.Run("fails with invalid EDITOR env var", func(t *testing.T) {
		// Save and restore EDITOR
		original := os.Getenv("EDITOR")
		originalVisual := os.Getenv("VISUAL")
		defer func() {
			os.Setenv("EDITOR", original)
			os.Setenv("VISUAL", originalVisual)
		}()

		// Set to invalid editor - validation will reject it
		os.Setenv("EDITOR", "nonexistent-editor-xyz-123")
		os.Setenv("VISUAL", "")

		// Save auto-detect by clearing PATH temporarily
		originalPath := os.Getenv("PATH")
		os.Setenv("PATH", "")
		defer os.Setenv("PATH", originalPath)

		err := Open(testFile)
		// Should fail because no valid editor is found
		if err == nil {
			t.Error("Open() should fail when no valid editor is available")
		}
	})

	t.Run("OpenWithOptions with custom editor", func(t *testing.T) {
		// Use echo as a "test editor" that just exits
		echoPath := "echo"
		if runtime.GOOS == "windows" {
			// On Windows, use cmd /c echo
			if _, err := exec.LookPath("cmd"); err != nil {
				t.Skip("cmd not available")
			}
			echoPath = "cmd"
		}

		err := OpenWithOptions(testFile, OpenOptions{
			Editor:       echoPath,
			WaitForClose: true,
		})
		// This may or may not work depending on platform, but shouldn't panic
		_ = err
	})
}

func TestContains(t *testing.T) {
	tests := []struct {
		haystack string
		needle   string
		want     bool
	}{
		{"vim", "vim", true},
		{"nvim", "vim", true},
		{"vim.exe", "vim", true},
		{"notepad", "vim", false},
		{"", "vim", false},
		{"vi", "vim", false},
	}

	for _, tt := range tests {
		t.Run(tt.haystack+"_"+tt.needle, func(t *testing.T) {
			got := contains(tt.haystack, tt.needle)
			if got != tt.want {
				t.Errorf("contains(%q, %q) = %v, want %v", tt.haystack, tt.needle, got, tt.want)
			}
		})
	}
}

func TestValidateEditor(t *testing.T) {
	tests := []struct {
		name   string
		editor string
		want   string // expected result (empty = rejected)
	}{
		{
			name:   "empty string",
			editor: "",
			want:   "",
		},
		{
			name:   "invalid characters in name",
			editor: "editor;rm -rf /",
			want:   "",
		},
		{
			name:   "relative path rejected",
			editor: "./editor",
			want:   "",
		},
		{
			name:   "path traversal rejected",
			editor: "../editor",
			want:   "",
		},
		{
			name:   "nonexistent command",
			editor: "nonexistent-editor-xyz",
			want:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := validateEditor(tt.editor)
			if got != tt.want {
				t.Errorf("validateEditor(%q) = %q, want %q", tt.editor, got, tt.want)
			}
		})
	}

	// Test with a valid editor (platform-specific)
	t.Run("valid editor in PATH", func(t *testing.T) {
		var testEditor string
		if runtime.GOOS == "windows" {
			testEditor = "notepad"
		} else {
			for _, ed := range []string{"vi", "sh", "cat"} {
				if _, err := exec.LookPath(ed); err == nil {
					testEditor = ed
					break
				}
			}
			if testEditor == "" {
				t.Skip("No common command found")
			}
		}

		got := validateEditor(testEditor)
		if got != testEditor {
			t.Errorf("validateEditor(%q) = %q, want %q", testEditor, got, testEditor)
		}
	})
}

func TestContainsPathSeparator(t *testing.T) {
	tests := []struct {
		input string
		want  bool
	}{
		{"notepad", false},
		{"notepad.exe", false},
		{"/usr/bin/vim", true},
		{"./editor", true},
		{"some/path", true},
	}

	if runtime.GOOS == "windows" {
		tests = append(tests, struct {
			input string
			want  bool
		}{`C:\Windows\notepad.exe`, true})
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := containsPathSeparator(tt.input)
			if got != tt.want {
				t.Errorf("containsPathSeparator(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
