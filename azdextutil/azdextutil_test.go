package azdextutil

import (
	"os"
	"strings"
	"testing"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, 1.0)

	if !rl.Allow() {
		t.Error("expected first call to be allowed")
	}
	if !rl.Allow() {
		t.Error("expected second call to be allowed")
	}
	if !rl.Allow() {
		t.Error("expected third call to be allowed")
	}

	if rl.Allow() {
		t.Error("expected fourth call to be denied after burst")
	}
}

func TestRateLimiter_CheckRateLimit(t *testing.T) {
	rl := NewRateLimiter(1, 0.0)

	if err := rl.CheckRateLimit("test_tool"); err != nil {
		t.Errorf("expected first call to pass: %v", err)
	}

	err := rl.CheckRateLimit("test_tool")
	if err == nil {
		t.Error("expected second call to fail with rate limit")
	}
	if err != nil && !strings.Contains(err.Error(), "test_tool") {
		t.Errorf("error should include tool name, got: %v", err)
	}
}

func TestValidateShellName(t *testing.T) {
	validShells := []string{"bash", "sh", "zsh", "pwsh", "powershell", "cmd", ""}
	for _, s := range validShells {
		if err := ValidateShellName(s); err != nil {
			t.Errorf("expected %q to be valid: %v", s, err)
		}
	}

	if err := ValidateShellName("malicious"); err == nil {
		t.Error("expected invalid shell to be rejected")
	}
}

func TestGetProjectDir_Fallback(t *testing.T) {
	dir, err := GetProjectDir("NONEXISTENT_VAR_12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cwd, _ := os.Getwd()
	if dir != cwd {
		t.Errorf("expected cwd %q, got %q", cwd, dir)
	}
}

func TestGetProjectDir_FromEnv(t *testing.T) {
	t.Setenv("TEST_PROJECT_DIR_12345", t.TempDir())
	dir, err := GetProjectDir("TEST_PROJECT_DIR_12345")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dir == "" {
		t.Error("expected non-empty dir")
	}
}
