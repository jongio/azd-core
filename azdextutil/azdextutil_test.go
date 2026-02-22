package azdextutil

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/spf13/cobra"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, 1.0)

	// Should allow burst
	if !rl.Allow() {
		t.Error("expected first call to be allowed")
	}
	if !rl.Allow() {
		t.Error("expected second call to be allowed")
	}
	if !rl.Allow() {
		t.Error("expected third call to be allowed")
	}

	// Should deny after burst exhausted
	if rl.Allow() {
		t.Error("expected fourth call to be denied")
	}
}

func TestRateLimiter_CheckRateLimit(t *testing.T) {
	rl := NewRateLimiter(1, 0.0)

	if err := rl.CheckRateLimit("test_tool"); err != nil {
		t.Errorf("expected first call to pass: %v", err)
	}

	if err := rl.CheckRateLimit("test_tool"); err == nil {
		t.Error("expected second call to fail with rate limit")
	}
}

// Deprecated: Tests deprecated ValidatePath function. Keep for backwards compatibility.
func TestValidatePath_TraversalBlocked(t *testing.T) {
	_, err := ValidatePath("../../../etc/passwd")
	if err == nil {
		t.Error("expected path traversal to be blocked")
	}
}

// Deprecated: Tests deprecated ValidatePath function. Keep for backwards compatibility.
func TestValidatePath_EmptyBlocked(t *testing.T) {
	_, err := ValidatePath("")
	if err == nil {
		t.Error("expected empty path to be blocked")
	}
}

// Deprecated: Tests deprecated ValidatePath function. Keep for backwards compatibility.
func TestValidatePath_AllowedBase(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	// Current directory should be allowed when it's the base
	result, err := ValidatePath(".", cwd)
	if err != nil {
		t.Errorf("expected current dir to be allowed: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
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

// Deprecated: Tests deprecated SetupTracingFromEnv/GetTraceContext functions. Keep for backwards compatibility.
func TestSetupTracingFromEnv(t *testing.T) {
	ctx := context.Background()

	// No trace context
	ctx2 := SetupTracingFromEnv(ctx)
	if tc := GetTraceContext(ctx2); tc != nil {
		t.Error("expected no trace context when TRACEPARENT not set")
	}

	// With trace context
	t.Setenv("TRACEPARENT", "00-abc123-def456-01")

	ctx3 := SetupTracingFromEnv(ctx)
	tc := GetTraceContext(ctx3)
	if tc == nil {
		t.Fatal("expected trace context to be set")
	}
	if tc.TraceParent != "00-abc123-def456-01" {
		t.Errorf("expected traceparent %q, got %q", "00-abc123-def456-01", tc.TraceParent)
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

// Deprecated: Tests deprecated GenerateMetadataFromCobra function. Keep for backwards compatibility.
func TestGenerateMetadataFromCobra(t *testing.T) {
	root := &cobra.Command{Use: "myext"}
	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the extension",
	})

	meta := GenerateMetadataFromCobra("1.0", "myext", root)
	if meta.SchemaVersion != "1.0" {
		t.Errorf("expected schema version 1.0, got %s", meta.SchemaVersion)
	}
	if meta.ID != "myext" {
		t.Errorf("expected ID myext, got %s", meta.ID)
	}
	if len(meta.Commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(meta.Commands))
	}
	if meta.Commands[0].Short != "Run the extension" {
		t.Errorf("unexpected short: %s", meta.Commands[0].Short)
	}
}

// Deprecated: Tests deprecated NewMetadataCommand function. Keep for backwards compatibility.
func TestNewMetadataCommand(t *testing.T) {
	rootProvider := func() *cobra.Command {
		root := &cobra.Command{Use: "testext"}
		root.AddCommand(&cobra.Command{Use: "hello", Short: "Say hello"})
		return root
	}

	cmd := NewMetadataCommand("testext", rootProvider)
	if !cmd.Hidden {
		t.Error("expected metadata command to be hidden")
	}

	var buf bytes.Buffer
	cmd.SetOut(&buf)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("metadata command failed: %v", err)
	}

	var meta ExtensionMetadata
	if err := json.Unmarshal(buf.Bytes(), &meta); err != nil {
		t.Fatalf("failed to parse metadata JSON: %v", err)
	}
	if meta.ID != "testext" {
		t.Errorf("expected ID testext, got %s", meta.ID)
	}
}

func TestGetArgsMap_NilArgs(t *testing.T) {
	req := mcp.CallToolRequest{}
	args := GetArgsMap(req)
	if len(args) != 0 {
		t.Error("expected empty map for nil args")
	}
}

func TestGetStringParam(t *testing.T) {
	args := map[string]interface{}{"key": "value", "num": 42}

	val, ok := GetStringParam(args, "key")
	if !ok || val != "value" {
		t.Errorf("expected 'value', got %q (ok=%v)", val, ok)
	}

	_, ok = GetStringParam(args, "num")
	if ok {
		t.Error("expected false for non-string value")
	}

	_, ok = GetStringParam(args, "missing")
	if ok {
		t.Error("expected false for missing key")
	}
}
