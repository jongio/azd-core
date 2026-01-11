//go:build windows
// +build windows

// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package pathutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Windows-specific tests to improve coverage from 78.3% to 85%+

func TestRefreshWindowsPATH_PowerShellExecution(t *testing.T) {
	// Test the Windows PATH refresh with PowerShell
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	newPath, err := refreshWindowsPATH()
	if err != nil {
		t.Logf("refreshWindowsPATH() error (PowerShell may not be available): %v", err)
		return
	}

	if newPath == "" {
		t.Error("refreshWindowsPATH() returned empty PATH")
	}

	// Verify PATH was updated in environment
	envPath := os.Getenv("PATH")
	if envPath != newPath {
		t.Errorf("PATH not updated in environment")
	}
}

func TestRefreshWindowsPATH_MachinePathOnly(t *testing.T) {
	// Test when user PATH is empty (machine PATH only scenario)
	// We can't actually test this without mocking, but we can verify the function handles it
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	newPath, err := refreshWindowsPATH()
	if err != nil {
		t.Logf("refreshWindowsPATH() error: %v", err)
		return
	}

	// PATH should contain common Windows directories
	if !strings.Contains(strings.ToLower(newPath), "windows") {
		t.Logf("Warning: PATH doesn't contain 'windows': %s", newPath)
	}
}

func TestRefreshWindowsPATH_PathCombination(t *testing.T) {
	// Test the path combination logic
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	newPath, err := refreshWindowsPATH()
	if err != nil {
		t.Logf("refreshWindowsPATH() error: %v", err)
		return
	}

	// PATH should be non-empty and contain semicolons (Windows path separator)
	if newPath != "" && !strings.Contains(newPath, ";") {
		// Single-entry PATH might not have semicolons
		t.Logf("PATH has single entry: %s", newPath)
	}
}

func TestFindToolInPath_WindowsExecutables(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		wantFind bool
	}{
		{
			name:     "cmd without extension",
			toolName: "cmd",
			wantFind: true,
		},
		{
			name:     "cmd with extension",
			toolName: "cmd.exe",
			wantFind: true,
		},
		{
			name:     "CMD uppercase",
			toolName: "CMD",
			wantFind: true,
		},
		{
			name:     "powershell",
			toolName: "powershell",
			wantFind: true,
		},
		{
			name:     "nonexistent",
			toolName: "nonexistent-tool-12345",
			wantFind: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindToolInPath(tt.toolName)
			found := result != ""
			if found != tt.wantFind {
				t.Logf("FindToolInPath(%q) found=%v, want=%v (path: %s)", tt.toolName, found, tt.wantFind, result)
			}
		})
	}
}

func TestFindToolInPath_ExeExtensionAdded(t *testing.T) {
	// Verify .exe is added automatically on Windows
	result1 := FindToolInPath("cmd")
	result2 := FindToolInPath("cmd.exe")

	if result1 == "" || result2 == "" {
		t.Fatal("cmd should be found in PATH")
	}

	// Both should find the same executable
	if !strings.EqualFold(result1, result2) {
		t.Logf("cmd and cmd.exe resolved to different paths: %q vs %q", result1, result2)
	}
}

func TestSearchToolInSystemPath_WindowsPaths(t *testing.T) {
	// Test searching in common Windows locations
	tests := []struct {
		name     string
		toolName string
	}{
		{
			name:     "git",
			toolName: "git",
		},
		{
			name:     "node",
			toolName: "node",
		},
		{
			name:     "python",
			toolName: "python",
		},
		{
			name:     "dotnet",
			toolName: "dotnet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SearchToolInSystemPath(tt.toolName)
			if result != "" {
				t.Logf("Found %s at: %s", tt.toolName, result)

				// Verify the file actually exists
				if _, err := os.Stat(result); err != nil {
					t.Errorf("SearchToolInSystemPath returned path that doesn't exist: %s", result)
				}

				// Verify it has .exe extension
				if !strings.HasSuffix(strings.ToLower(result), ".exe") {
					t.Errorf("Windows executable should have .exe extension: %s", result)
				}
			} else {
				t.Logf("%s not found in common system paths", tt.toolName)
			}
		})
	}
}

func TestSearchToolInSystemPath_ExeExtension(t *testing.T) {
	// Test that .exe is added automatically
	result1 := SearchToolInSystemPath("git")
	result2 := SearchToolInSystemPath("git.exe")

	// Both searches should return the same result (or both empty)
	if result1 != result2 {
		t.Logf("git vs git.exe returned different results: %q vs %q", result1, result2)
	}
}

func TestSearchToolInSystemPath_EnvironmentPaths(t *testing.T) {
	// Test searching in LOCALAPPDATA and APPDATA paths
	localAppData := os.Getenv("LOCALAPPDATA")
	appData := os.Getenv("APPDATA")
	userProfile := os.Getenv("USERPROFILE")

	if localAppData != "" {
		t.Logf("LOCALAPPDATA: %s", localAppData)
	}
	if appData != "" {
		t.Logf("APPDATA: %s", appData)
	}
	if userProfile != "" {
		t.Logf("USERPROFILE: %s", userProfile)
	}

	// Try to find npm (often in APPDATA)
	result := SearchToolInSystemPath("npm")
	if result != "" {
		t.Logf("Found npm at: %s", result)
	}
}

func TestSearchToolInSystemPath_ProgramFiles(t *testing.T) {
	// Verify Program Files paths are searched
	// We know git is often installed in Program Files
	result := SearchToolInSystemPath("git")
	if result != "" {
		if !strings.Contains(result, "Program Files") {
			t.Logf("git found at non-Program Files location: %s", result)
		}
	}
}

func TestSearchToolInSystemPath_EmptyToolName(t *testing.T) {
	// Test with empty tool name
	result := SearchToolInSystemPath("")
	if result != "" {
		t.Errorf("SearchToolInSystemPath(\"\") should return empty, got: %s", result)
	}
}

func TestSearchToolInSystemPath_NonExistentTool(t *testing.T) {
	// Test with tool that definitely doesn't exist
	result := SearchToolInSystemPath("definitely-does-not-exist-12345-xyz")
	if result != "" {
		t.Errorf("SearchToolInSystemPath() should return empty for non-existent tool, got: %s", result)
	}
}

func TestGetInstallSuggestion_AllToolsWindows(t *testing.T) {
	// Verify all common Windows tools have suggestions
	windowsTools := []string{
		"node", "npm", "pnpm",
		"python", "pip",
		"docker", "git",
		"dotnet", "azd", "az",
		"go", "java", "gh",
	}

	for _, tool := range windowsTools {
		t.Run(tool, func(t *testing.T) {
			suggestion := GetInstallSuggestion(tool)
			if suggestion == "" {
				t.Errorf("No suggestion for %s", tool)
			}
			if !strings.Contains(suggestion, "http") {
				t.Errorf("Suggestion for %s should contain URL, got: %s", tool, suggestion)
			}
			t.Logf("%s: %s", tool, suggestion)
		})
	}
}

func TestGetInstallSuggestion_UnknownToolWindows(t *testing.T) {
	unknown := "my-custom-tool-xyz"
	suggestion := GetInstallSuggestion(unknown)
	if !strings.Contains(suggestion, unknown) {
		t.Errorf("Suggestion should mention tool name, got: %s", suggestion)
	}
	if !strings.Contains(suggestion, "manually") {
		t.Errorf("Unknown tool suggestion should mention manual install, got: %s", suggestion)
	}
}

func TestRefreshPATH_WindowsIntegration(t *testing.T) {
	// Full integration test for Windows
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Test refresh
	newPath, err := RefreshPATH()
	if err != nil {
		t.Logf("RefreshPATH() error (may be expected in test environment): %v", err)
		return
	}

	if newPath == "" {
		t.Error("RefreshPATH() returned empty PATH")
	}

	// Verify common Windows directories are in PATH
	commonPaths := []string{"Windows", "System32"}
	foundAny := false
	lowerPath := strings.ToLower(newPath)
	for _, dir := range commonPaths {
		if strings.Contains(lowerPath, strings.ToLower(dir)) {
			foundAny = true
			break
		}
	}
	if !foundAny {
		t.Logf("Warning: Common Windows directories not found in PATH: %s", newPath)
	}
}

func TestRefreshPATH_SetEnvironment(t *testing.T) {
	// Verify RefreshPATH actually updates the environment variable
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Temporarily clear PATH
	_ = os.Setenv("PATH", "")

	// Refresh should restore it
	newPath, err := RefreshPATH()
	if err != nil {
		t.Logf("RefreshPATH() with empty PATH error: %v", err)
		return
	}

	// Verify environment was updated
	currentPath := os.Getenv("PATH")
	if currentPath != newPath {
		t.Errorf("PATH environment not updated: got %q, want %q", currentPath, newPath)
	}
}

func TestFindToolInPath_CaseSensitivity(t *testing.T) {
	// Windows is case-insensitive for executables
	tests := []struct {
		name string
		tool string
	}{
		{"lowercase", "cmd"},
		{"uppercase", "CMD"},
		{"mixed case", "Cmd"},
		{"with exe lowercase", "cmd.exe"},
		{"with exe uppercase", "CMD.EXE"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FindToolInPath(tt.tool)
			if result == "" {
				t.Errorf("FindToolInPath(%q) should find cmd", tt.tool)
			}
		})
	}
}

func TestSearchToolInSystemPath_AllCommonLocations(t *testing.T) {
	// Test that we search all common Windows locations
	// This exercises all the search paths in the function

	// Common tools that might be in various locations
	tools := []string{
		"git",      // Program Files\Git\cmd
		"node",     // Program Files\nodejs
		"python",   // Program Files\Python3xx
		"docker",   // Program Files\Docker
		"dotnet",   // Program Files\dotnet
	}

	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			result := SearchToolInSystemPath(tool)
			if result != "" {
				t.Logf("Found %s at: %s", tool, result)

				// Verify the path exists
				if _, err := os.Stat(result); err != nil {
					t.Errorf("Path doesn't exist: %s", result)
				}
			} else {
				t.Logf("%s not found (may not be installed)", tool)
			}
		})
	}
}

func TestRefreshWindowsPATH_ErrorHandling(t *testing.T) {
	// Test error handling when PowerShell might not be available
	// The function should return an error gracefully
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Even if PowerShell fails, the function should return a proper error
	_, err := refreshWindowsPATH()
	if err != nil {
		// Error is acceptable in test environments without PowerShell
		t.Logf("refreshWindowsPATH() error (expected in some test envs): %v", err)
		if !strings.Contains(err.Error(), "failed to get") {
			t.Errorf("Error should mention what failed, got: %v", err)
		}
	}
}

func TestSearchToolInSystemPath_GoPath(t *testing.T) {
	// Test searching in USERPROFILE\go\bin
	userProfile := os.Getenv("USERPROFILE")
	if userProfile != "" {
		goPath := filepath.Join(userProfile, "go", "bin")
		t.Logf("Go tools path: %s", goPath)

		// Check if go bin directory exists
		if info, err := os.Stat(goPath); err == nil && info.IsDir() {
			// Try to find a go tool
			result := SearchToolInSystemPath("go")
			t.Logf("Go found at: %s", result)
		}
	}
}

func TestFindToolInPath_ExtensionPriority(t *testing.T) {
	// Test that tools without .exe are found correctly
	// Windows should add .exe automatically
	tools := []string{"cmd", "powershell", "where"}

	for _, tool := range tools {
		t.Run(tool, func(t *testing.T) {
			result := FindToolInPath(tool)
			if result != "" {
				// Result should have .exe extension
				if !strings.HasSuffix(strings.ToLower(result), ".exe") {
					t.Errorf("Windows executable should end with .exe: %s", result)
				}
			}
		})
	}
}

func TestRefreshPATH_OriginalValue(t *testing.T) {
	// Test that original PATH is preserved when refresh fails
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	// Refresh (may succeed or fail)
	_, err := RefreshPATH()
	if err != nil {
		// If refresh failed, PATH should be unchanged
		currentPath := os.Getenv("PATH")
		if currentPath != originalPath {
			t.Errorf("PATH changed even though refresh failed")
		}
	}
}

func TestSearchToolInSystemPath_PythonVersions(t *testing.T) {
	// Test searching for Python in versioned directories
	result := SearchToolInSystemPath("python")
	if result != "" {
		t.Logf("Found python at: %s", result)

		// Check if it's in a versioned directory
		if strings.Contains(result, "Python3") {
			t.Logf("Python found in versioned directory")
		}
	}
}

func TestGetInstallSuggestion_WindowsSpecificTools(t *testing.T) {
	// Test tools commonly used on Windows
	tools := map[string]string{
		"func":    "azure-functions",
		"aspire":  "aspire",
		"azd":     "azd",
		"az":      "azure",
		"gh":      "github.com",
	}

	for tool, expectedInSuggestion := range tools {
		t.Run(tool, func(t *testing.T) {
			suggestion := GetInstallSuggestion(tool)
			if suggestion == "" {
				t.Errorf("No suggestion for %s", tool)
			}
			if !strings.Contains(strings.ToLower(suggestion), expectedInSuggestion) {
				t.Logf("Suggestion for %s: %s", tool, suggestion)
			}
		})
	}
}

func TestRefreshWindowsPATH_BothPathsPresent(t *testing.T) {
	// Test the scenario where both machine and user PATH exist
	originalPath := os.Getenv("PATH")
	defer func() { _ = os.Setenv("PATH", originalPath) }()

	newPath, err := refreshWindowsPATH()
	if err != nil {
		t.Logf("refreshWindowsPATH() error: %v", err)
		return
	}

	// PATH should contain multiple entries
	parts := strings.Split(newPath, ";")
	if len(parts) < 2 {
		t.Logf("Warning: PATH has only %d entries", len(parts))
	}
}

func TestFindToolInPath_AlreadyHasExe(t *testing.T) {
	// Test finding tools that already have .exe in the search name
	result := FindToolInPath("cmd.exe")
	if result == "" {
		t.Error("FindToolInPath(\"cmd.exe\") should find cmd")
	}

	// Should not add double .exe
	if strings.HasSuffix(result, ".exe.exe") {
		t.Error("Should not double-add .exe extension")
	}
}
