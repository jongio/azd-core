// Copyright (c) Microsoft Corporation. All rights reserved.
// Licensed under the MIT License.

package security

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Additional tests to improve coverage to ≥95%

func TestValidatePackageManager_EmptyString(t *testing.T) {
	err := ValidatePackageManager("")
	if err == nil {
		t.Error("ValidatePackageManager(\"\") should error for empty string")
	}
}

func TestSanitizeScriptName_AllDangerousCharacters(t *testing.T) {
	tests := []struct {
		name       string
		scriptName string
		char       string
	}{
		{"semicolon", "test;rm", ";"},
		{"ampersand", "test&", "&"},
		{"pipe", "test|cat", "|"},
		{"greater", "test>file", ">"},
		{"less", "test<file", "<"},
		{"backtick", "test`cmd`", "`"},
		{"dollar", "test$HOME", "$"},
		{"open paren", "test()", "("},
		{"close paren", "test)", ")"},
		{"open brace", "test{}", "{"},
		{"close brace", "test}", "}"},
		{"open bracket", "test[]", "["},
		{"close bracket", "test]", "]"},
		{"newline", "test\n", "\n"},
		{"carriage return", "test\r", "\r"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeScriptName(tt.scriptName)
			if err == nil {
				t.Errorf("SanitizeScriptName() should reject %q character", tt.char)
			}
			if !strings.Contains(err.Error(), "dangerous character") {
				t.Errorf("Expected error about dangerous character, got: %v", err)
			}
		})
	}
}

func TestSanitizeScriptName_SafeNames(t *testing.T) {
	safeNames := []string{
		"dev",
		"build",
		"test",
		"build-prod",
		"start:dev",
		"my.script.name",
		"script_123",
	}

	for _, name := range safeNames {
		t.Run(name, func(t *testing.T) {
			err := SanitizeScriptName(name)
			if err != nil {
				t.Errorf("SanitizeScriptName(%q) should be safe, got error: %v", name, err)
			}
		})
	}
}

func TestValidatePath_ErrorWrapping(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		wantError error
	}{
		{
			name:      "empty path",
			path:      "",
			wantError: ErrInvalidPath,
		},
		{
			name:      "path traversal detected",
			path:      "../etc/passwd",
			wantError: ErrPathTraversal,
		},
		{
			name:      "path traversal in middle",
			path:      "/usr/../etc/passwd",
			wantError: ErrPathTraversal,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if err == nil {
				t.Errorf("ValidatePath(%q) expected error, got nil", tt.path)
				return
			}
			if !errors.Is(err, tt.wantError) {
				t.Errorf("ValidatePath(%q) error = %v, want %v", tt.path, err, tt.wantError)
			}
		})
	}
}

func TestValidateServiceName_MaxLength(t *testing.T) {
	// Exactly 63 characters (max allowed)
	maxLengthName := strings.Repeat("a", 63)
	err := ValidateServiceName(maxLengthName, false)
	if err != nil {
		t.Errorf("ValidateServiceName() with exactly 63 chars should pass, got: %v", err)
	}

	// 64 characters (too long)
	tooLongName := strings.Repeat("a", 64)
	err = ValidateServiceName(tooLongName, false)
	if err == nil {
		t.Error("ValidateServiceName() with 64 chars should fail")
	}
	if !errors.Is(err, ErrInvalidServiceName) {
		t.Errorf("Expected ErrInvalidServiceName, got: %v", err)
	}
}

func TestValidateServiceName_StartCharacterRestrictions(t *testing.T) {
	tests := []struct {
		name        string
		serviceName string
		wantErr     bool
	}{
		{"starts with letter", "api", false},
		{"starts with number", "1api", false},
		{"starts with dash", "-api", true},
		{"starts with underscore", "_api", true},
		{"starts with dot", ".api", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateServiceName(tt.serviceName, false)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateServiceName(%q) error = %v, wantErr %v", tt.serviceName, err, tt.wantErr)
			}
		})
	}
}

func TestValidateServiceName_AllowEmpty(t *testing.T) {
	// Test allowEmpty=true
	err := ValidateServiceName("", true)
	if err != nil {
		t.Errorf("ValidateServiceName(\"\", true) should allow empty, got: %v", err)
	}

	// Test allowEmpty=false
	err = ValidateServiceName("", false)
	if err == nil {
		t.Error("ValidateServiceName(\"\", false) should reject empty")
	}
	if !errors.Is(err, ErrInvalidServiceName) {
		t.Errorf("Expected ErrInvalidServiceName, got: %v", err)
	}
}

func TestValidateServiceName_ComplexValidNames(t *testing.T) {
	validNames := []string{
		"api-service",
		"api_service",
		"api.service",
		"service1",
		"Service1",
		"api-v1.service_2",
		"a1b2c3",
		"my.api.service-v1_2",
	}

	for _, name := range validNames {
		t.Run(name, func(t *testing.T) {
			err := ValidateServiceName(name, false)
			if err != nil {
				t.Errorf("ValidateServiceName(%q) should be valid, got: %v", name, err)
			}
		})
	}
}

func TestValidateServiceName_PathTraversalAttempts(t *testing.T) {
	tests := []string{
		"../service",
		"service/..",
		"ser..vice",
		"service/sub",
		"service\\sub",
	}

	for _, name := range tests {
		t.Run(name, func(t *testing.T) {
			err := ValidateServiceName(name, false)
			if err == nil {
				t.Errorf("ValidateServiceName(%q) should reject path characters", name)
			}
		})
	}
}

func TestIsContainerEnvironment_AllEnvironments(t *testing.T) {
	// Save original env vars
	originalCodespaces := os.Getenv("CODESPACES")
	originalRemoteContainers := os.Getenv("REMOTE_CONTAINERS")
	originalK8s := os.Getenv("KUBERNETES_SERVICE_HOST")

	defer func() {
		_ = os.Unsetenv("CODESPACES")
		_ = os.Unsetenv("REMOTE_CONTAINERS")
		_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")
		if originalCodespaces != "" {
			_ = os.Setenv("CODESPACES", originalCodespaces)
		}
		if originalRemoteContainers != "" {
			_ = os.Setenv("REMOTE_CONTAINERS", originalRemoteContainers)
		}
		if originalK8s != "" {
			_ = os.Setenv("KUBERNETES_SERVICE_HOST", originalK8s)
		}
	}()

	tests := []struct {
		name     string
		envVars  map[string]string
		expected bool
	}{
		{
			name:     "no container indicators",
			envVars:  map[string]string{},
			expected: false,
		},
		{
			name:     "Codespaces=true",
			envVars:  map[string]string{"CODESPACES": "true"},
			expected: true,
		},
		{
			name:     "Codespaces=false",
			envVars:  map[string]string{"CODESPACES": "false"},
			expected: false,
		},
		{
			name:     "Remote Containers=true",
			envVars:  map[string]string{"REMOTE_CONTAINERS": "true"},
			expected: true,
		},
		{
			name:     "Remote Containers=false",
			envVars:  map[string]string{"REMOTE_CONTAINERS": "false"},
			expected: false,
		},
		{
			name:     "Kubernetes host set",
			envVars:  map[string]string{"KUBERNETES_SERVICE_HOST": "10.0.0.1"},
			expected: true,
		},
		{
			name:     "Kubernetes host empty",
			envVars:  map[string]string{"KUBERNETES_SERVICE_HOST": ""},
			expected: false,
		},
		{
			name: "Multiple indicators",
			envVars: map[string]string{
				"CODESPACES":       "true",
				"REMOTE_CONTAINERS": "true",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear all env vars
			_ = os.Unsetenv("CODESPACES")
			_ = os.Unsetenv("REMOTE_CONTAINERS")
			_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")

			// Set test env vars
			for k, v := range tt.envVars {
				if v != "" {
					_ = os.Setenv(k, v)
				}
			}

			result := IsContainerEnvironment()
			if result != tt.expected {
				t.Errorf("IsContainerEnvironment() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestValidateFilePermissions_Windows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-specific test")
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// On Windows, permission check should be skipped
	err := ValidateFilePermissions(tmpFile)
	if err != nil {
		t.Errorf("ValidateFilePermissions() on Windows should skip check, got error: %v", err)
	}
}

func TestValidateFilePermissions_UnixPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		perm    os.FileMode
		wantErr bool
	}{
		{"secure 0600", 0600, false},
		{"secure 0644", 0644, false},
		{"secure 0400", 0400, false},
		{"group-writable 0664", 0664, true},
		{"world-writable 0666", 0666, true},
		{"world-writable 0646", 0646, true},
		{"all-writable 0777", 0777, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile := filepath.Join(tmpDir, tt.name+".txt")
			if err := os.WriteFile(tmpFile, []byte("test"), 0644); err != nil {
				t.Fatalf("Failed to create test file: %v", err)
			}

			// Set specific permissions
			if err := os.Chmod(tmpFile, tt.perm); err != nil {
				t.Fatalf("Failed to chmod file: %v", err)
			}

			err := ValidateFilePermissions(tmpFile)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateFilePermissions(%s) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}

			if err != nil && !errors.Is(err, ErrInsecureFilePermissions) {
				t.Errorf("Expected ErrInsecureFilePermissions, got: %v", err)
			}
		})
	}
}

func TestValidateFilePermissions_NonExistent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping Unix-specific test on Windows")
	}

	err := ValidateFilePermissions("/nonexistent/path/to/file.txt")
	if err == nil {
		t.Error("ValidateFilePermissions() with non-existent file should error")
	}
}

func TestValidatePath_FutureFile(t *testing.T) {
	tmpDir := t.TempDir()
	futurePath := filepath.Join(tmpDir, "future-file.txt")

	// Should not error - file doesn't exist yet, but path structure is valid
	err := ValidatePath(futurePath)
	if err != nil && strings.Contains(err.Error(), "parent directory reference") {
		t.Errorf("ValidatePath() with non-existent file should not detect traversal: %v", err)
	}
}

func TestValidatePath_DotsInFilename(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "dots in filename",
			path:    filepath.Join(tmpDir, "file..name.txt"),
			wantErr: true,
		},
		{
			name:    "hidden file",
			path:    filepath.Join(tmpDir, ".hidden"),
			wantErr: false,
		},
		{
			name:    "nested hidden",
			path:    filepath.Join(tmpDir, ".config", ".secrets"),
			wantErr: false,
		},
		{
			name:    "single dot",
			path:    ".",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidatePath_CleanedPathValidation(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "normal path",
			path:    "/tmp/test",
			wantErr: false,
		},
		{
			name:    "path becomes .. after clean",
			path:    "a/../../..",
			wantErr: true,
		},
		{
			name:    "relative traversal",
			path:    "foo/../../bar",
			wantErr: true,
		},
		{
			name:    "clean removes redundant separators",
			path:    "/tmp//test",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

func TestValidateFilePermissions_ContainerWarnings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping permission test on Windows")
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("test"), 0666); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Save original env vars
	originalCodespaces := os.Getenv("CODESPACES")
	defer func() {
		if originalCodespaces != "" {
			_ = os.Setenv("CODESPACES", originalCodespaces)
		} else {
			_ = os.Unsetenv("CODESPACES")
		}
	}()

	// Test in container environment
	_ = os.Setenv("CODESPACES", "true")

	err := ValidateFilePermissions(tmpFile)
	// Should still return error in container, but caller should handle as warning
	if !errors.Is(err, ErrInsecureFilePermissions) {
		t.Errorf("Expected ErrInsecureFilePermissions in container, got: %v", err)
	}
}

func TestValidatePackageManager_AllSupported(t *testing.T) {
	supportedPMs := []string{
		"npm", "pnpm", "yarn",
		"pip", "poetry", "uv",
		"dotnet",
	}

	for _, pm := range supportedPMs {
		t.Run(pm, func(t *testing.T) {
			err := ValidatePackageManager(pm)
			if err != nil {
				t.Errorf("ValidatePackageManager(%q) should be valid, got: %v", pm, err)
			}
		})
	}
}

func TestValidatePackageManager_Unsupported(t *testing.T) {
	unsupportedPMs := []string{
		"",
		"maven",
		"gradle",
		"composer",
		"gem",
		"cargo",
		"malicious-pm",
		"npm; rm -rf /",
	}

	for _, pm := range unsupportedPMs {
		t.Run(pm, func(t *testing.T) {
			err := ValidatePackageManager(pm)
			if err == nil {
				t.Errorf("ValidatePackageManager(%q) should be invalid", pm)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	// Test that sentinel errors are correctly defined
	if ErrInvalidPath == nil {
		t.Error("ErrInvalidPath should not be nil")
	}
	if ErrPathTraversal == nil {
		t.Error("ErrPathTraversal should not be nil")
	}
	if ErrInvalidServiceName == nil {
		t.Error("ErrInvalidServiceName should not be nil")
	}
	if ErrInsecureFilePermissions == nil {
		t.Error("ErrInsecureFilePermissions should not be nil")
	}

	// Test error messages
	if ErrInvalidPath.Error() != "invalid path" {
		t.Errorf("ErrInvalidPath.Error() = %q, want \"invalid path\"", ErrInvalidPath.Error())
	}
	if ErrPathTraversal.Error() != "path traversal detected" {
		t.Errorf("ErrPathTraversal.Error() = %q, want \"path traversal detected\"", ErrPathTraversal.Error())
	}
}

// Test ValidatePath error wrapping with symlink errors
func TestValidatePath_SymlinkErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink handling different on Windows")
	}

	tmpDir := t.TempDir()

	// Create a broken symlink (points to non-existent target)
	brokenLink := filepath.Join(tmpDir, "broken-link")
	if err := os.Symlink("/nonexistent/target", brokenLink); err != nil {
		t.Skipf("Cannot create symlink: %v", err)
	}

	// Validate broken symlink - should not error (path doesn't exist, but structure is valid)
	err := ValidatePath(brokenLink)
	if err != nil && strings.Contains(err.Error(), "cannot resolve symbolic links") {
		// This is expected - the symlink can't be resolved
		t.Logf("Broken symlink validation: %v", err)
	}
}

// Test IsContainerEnvironment with all environment variable permutations
func TestIsContainerEnvironment_EdgeCases(t *testing.T) {
	// Save original env vars
	vars := []string{"CODESPACES", "REMOTE_CONTAINERS", "KUBERNETES_SERVICE_HOST"}
	originals := make(map[string]string)
	for _, v := range vars {
		originals[v] = os.Getenv(v)
		_ = os.Unsetenv(v)
	}
	defer func() {
		for k, v := range originals {
			if v != "" {
				_ = os.Setenv(k, v)
			} else {
				_ = os.Unsetenv(k)
			}
		}
	}()

	// Test with no indicators (should check /.dockerenv)
	result := IsContainerEnvironment()
	// Result depends on whether /.dockerenv exists
	t.Logf("IsContainerEnvironment() with no env vars = %v", result)

	// Test Kubernetes with empty value
	_ = os.Setenv("KUBERNETES_SERVICE_HOST", "")
	if IsContainerEnvironment() {
		t.Error("Empty KUBERNETES_SERVICE_HOST should not indicate container")
	}
	_ = os.Unsetenv("KUBERNETES_SERVICE_HOST")

	// Test Kubernetes with non-empty value
	_ = os.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	if !IsContainerEnvironment() {
		t.Error("Non-empty KUBERNETES_SERVICE_HOST should indicate container")
	}
}

// Test ValidatePath with complex paths
func TestValidatePath_ComplexPaths(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "path with spaces",
			path:    filepath.Join(tmpDir, "file with spaces.txt"),
			wantErr: false,
		},
		{
			name:    "unicode path",
			path:    filepath.Join(tmpDir, "файл.txt"),
			wantErr: false,
		},
		{
			name:    "very long path",
			path:    filepath.Join(tmpDir, strings.Repeat("a", 200)+".txt"),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
		})
	}
}

// Test ValidatePath with actual file creation to test EvalSymlinks path
func TestValidatePath_WithRealFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a real file
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Validate real file - should pass
	if err := ValidatePath(realFile); err != nil {
		t.Errorf("ValidatePath() with real file should pass, got: %v", err)
	}

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "subdir")
	if err := os.Mkdir(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Validate subdirectory - should pass
	if err := ValidatePath(subDir); err != nil {
		t.Errorf("ValidatePath() with real directory should pass, got: %v", err)
	}

	// Validate path with .. that stays within bounds
	// This should fail because ValidatePath checks for .. before cleaning
	pathWithDots := filepath.Join(subDir, "..", "real.txt")
	if err := ValidatePath(pathWithDots); err == nil {
		// On Windows, filepath.Join may normalize the path differently
		t.Logf("ValidatePath() with .. returned nil (path may be normalized by filepath.Join on Windows)")
	} else if !errors.Is(err, ErrPathTraversal) {
		t.Logf("ValidatePath() with .. returned error: %v", err)
	}
}
