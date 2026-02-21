package version

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

func TestNew_Defaults(t *testing.T) {
	info := New("test.ext", "Test Ext")
	if info.Version != "0.0.0-dev" {
		t.Errorf("expected Version '0.0.0-dev', got %q", info.Version)
	}
	if info.BuildDate != "unknown" {
		t.Errorf("expected BuildDate 'unknown', got %q", info.BuildDate)
	}
	if info.GitCommit != "unknown" {
		t.Errorf("expected GitCommit 'unknown', got %q", info.GitCommit)
	}
}

func TestNew_SetsIDAndName(t *testing.T) {
	info := New("jongio.azd.exec", "azd exec")
	if info.ExtensionID != "jongio.azd.exec" {
		t.Errorf("expected ExtensionID 'jongio.azd.exec', got %q", info.ExtensionID)
	}
	if info.Name != "azd exec" {
		t.Errorf("expected Name 'azd exec', got %q", info.Name)
	}
}

func TestInfo_String(t *testing.T) {
	info := &Info{
		Version:   "1.2.3",
		BuildDate: "2024-01-01",
		GitCommit: "abc123",
		Name:      "azd exec",
	}
	got := info.String()
	expected := "azd exec version 1.2.3 (commit: abc123, built: 2024-01-01)"
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// captureStdout captures stdout during fn execution.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestNewCommand_HumanReadable(t *testing.T) {
	info := New("jongio.azd.exec", "azd exec")
	cmd := NewCommand(info, nil)
	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	for _, want := range []string{"Version", "Build Date", "Git Commit", "Extension ID"} {
		if !strings.Contains(output, want) {
			t.Errorf("expected output to contain %q, got:\n%s", want, output)
		}
	}
}

func TestNewCommand_JSON(t *testing.T) {
	info := New("jongio.azd.exec", "azd exec")
	format := "json"
	cmd := NewCommand(info, &format)
	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	var parsed Info
	if err := json.Unmarshal([]byte(output), &parsed); err != nil {
		t.Fatalf("expected valid JSON, got error: %v\noutput: %s", err, output)
	}
	if parsed.ExtensionID != "jongio.azd.exec" {
		t.Errorf("expected extensionId 'jongio.azd.exec', got %q", parsed.ExtensionID)
	}
	if parsed.Version != "0.0.0-dev" {
		t.Errorf("expected version '0.0.0-dev', got %q", parsed.Version)
	}
}

func TestNewCommand_Quiet(t *testing.T) {
	info := New("jongio.azd.exec", "azd exec")
	cmd := NewCommand(info, nil)
	cmd.SetArgs([]string{"--quiet"})
	output := captureStdout(t, func() {
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	trimmed := strings.TrimSpace(output)
	if trimmed != "0.0.0-dev" {
		t.Errorf("expected '0.0.0-dev', got %q", trimmed)
	}
}
