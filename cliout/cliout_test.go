package cliout

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"runtime"
	"strings"
	"testing"
)

// captureOutput captures stdout during function execution
func captureOutput(t *testing.T, fn func()) string {
	t.Helper()
	
	// Save original stdout
	oldStdout := os.Stdout
	defer func() { os.Stdout = oldStdout }()
	
	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	
	// Replace stdout
	os.Stdout = w
	
	// Channel to signal completion
	done := make(chan string)
	
	// Read from pipe in goroutine
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()
	
	// Execute function
	fn()
	
	// Close writer and wait for reader
	_ = w.Close()
	output := <-done
	
	return output
}

// Test Format Management

func TestSetFormatDefault(t *testing.T) {
	// Reset to default before test
	globalFormat = FormatDefault
	
	err := SetFormat("default")
	if err != nil {
		t.Fatalf("SetFormat(default) failed: %v", err)
	}
	
	if globalFormat != FormatDefault {
		t.Errorf("Expected FormatDefault, got %v", globalFormat)
	}
}

func TestSetFormatJSON(t *testing.T) {
	// Reset to default before test
	globalFormat = FormatDefault
	
	err := SetFormat("json")
	if err != nil {
		t.Fatalf("SetFormat(json) failed: %v", err)
	}
	
	if globalFormat != FormatJSON {
		t.Errorf("Expected FormatJSON, got %v", globalFormat)
	}
	
	// Reset after test
	globalFormat = FormatDefault
}

func TestSetFormatEmpty(t *testing.T) {
	// Reset to JSON
	globalFormat = FormatJSON
	
	err := SetFormat("")
	if err != nil {
		t.Fatalf("SetFormat('') failed: %v", err)
	}
	
	if globalFormat != FormatDefault {
		t.Errorf("Expected FormatDefault for empty string, got %v", globalFormat)
	}
	
	// Reset after test
	globalFormat = FormatDefault
}

func TestSetFormatInvalid(t *testing.T) {
	err := SetFormat("invalid")
	if err == nil {
		t.Fatal("Expected error for invalid format, got nil")
	}
	
	expectedMsg := "invalid output format: invalid"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected error containing %q, got %q", expectedMsg, err.Error())
	}
}

func TestGetFormat(t *testing.T) {
	// Test default format
	globalFormat = FormatDefault
	if GetFormat() != FormatDefault {
		t.Errorf("Expected FormatDefault, got %v", GetFormat())
	}
	
	// Test JSON format
	globalFormat = FormatJSON
	if GetFormat() != FormatJSON {
		t.Errorf("Expected FormatJSON, got %v", GetFormat())
	}
	
	// Reset
	globalFormat = FormatDefault
}

func TestIsJSON(t *testing.T) {
	// Test default format
	globalFormat = FormatDefault
	if IsJSON() {
		t.Error("Expected IsJSON() to return false for default format")
	}
	
	// Test JSON format
	globalFormat = FormatJSON
	if !IsJSON() {
		t.Error("Expected IsJSON() to return true for JSON format")
	}
	
	// Reset
	globalFormat = FormatDefault
}

// Test Orchestration Mode

func TestSetOrchestrated(t *testing.T) {
	// Reset
	orchestratedMode = false
	
	SetOrchestrated(true)
	if !orchestratedMode {
		t.Error("Expected orchestratedMode to be true")
	}
	
	SetOrchestrated(false)
	if orchestratedMode {
		t.Error("Expected orchestratedMode to be false")
	}
}

func TestIsOrchestrated(t *testing.T) {
	// Test false
	orchestratedMode = false
	if IsOrchestrated() {
		t.Error("Expected IsOrchestrated() to return false")
	}
	
	// Test true
	orchestratedMode = true
	if !IsOrchestrated() {
		t.Error("Expected IsOrchestrated() to return true")
	}
	
	// Reset
	orchestratedMode = false
}

// Test Unicode Detection

func TestDetectUnicodeSupport(t *testing.T) {
	// Save original environment
	origEnv := map[string]string{
		"WT_SESSION":                    os.Getenv("WT_SESSION"),
		"TERM_PROGRAM":                  os.Getenv("TERM_PROGRAM"),
		"ConEmuPID":                     os.Getenv("ConEmuPID"),
		"PSModulePath":                  os.Getenv("PSModulePath"),
		"POWERSHELL_DISTRIBUTION_CHANNEL": os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL"),
		"TERM":                          os.Getenv("TERM"),
	}
	defer func() {
		// Restore environment
		for k, v := range origEnv {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
		// Reset detection
		supportsUnicode = detectUnicodeSupport()
	}()
	
	// Clear all relevant environment variables
	_ = os.Unsetenv("WT_SESSION")
	_ = os.Unsetenv("TERM_PROGRAM")
	os.Unsetenv("ConEmuPID")
	os.Unsetenv("PSModulePath")
	os.Unsetenv("POWERSHELL_DISTRIBUTION_CHANNEL")
	os.Unsetenv("TERM")
	
	// Test Windows Terminal
	if runtime.GOOS == "windows" {
		os.Setenv("WT_SESSION", "test-session")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with WT_SESSION on Windows")
		}
		os.Unsetenv("WT_SESSION")
		
		// Test VS Code
		os.Setenv("TERM_PROGRAM", "vscode")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with TERM_PROGRAM=vscode on Windows")
		}
		os.Unsetenv("TERM_PROGRAM")
		
		// Test ConEmu
		os.Setenv("ConEmuPID", "12345")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with ConEmuPID on Windows")
		}
		os.Unsetenv("ConEmuPID")
		
		// Test PowerShell
		os.Setenv("PSModulePath", "/path/to/modules")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with PSModulePath on Windows")
		}
		os.Unsetenv("PSModulePath")
		
		// Test PowerShell distribution channel
		os.Setenv("POWERSHELL_DISTRIBUTION_CHANNEL", "MSI:Windows 10")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with POWERSHELL_DISTRIBUTION_CHANNEL on Windows")
		}
		os.Unsetenv("POWERSHELL_DISTRIBUTION_CHANNEL")
		
		// Test TERM variable
		os.Setenv("TERM", "xterm-256color")
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support with TERM on Windows")
		}
		os.Unsetenv("TERM")
		
		// Test old Windows console (no env vars)
		if detectUnicodeSupport() {
			t.Error("Expected no Unicode support for old Windows console")
		}
	} else {
		// Unix-like systems should always support Unicode
		if !detectUnicodeSupport() {
			t.Error("Expected Unicode support on Unix-like systems")
		}
	}
}

func TestGetIcon(t *testing.T) {
	// Save original value
	origSupportsUnicode := supportsUnicode
	defer func() { supportsUnicode = origSupportsUnicode }()
	
	// Test with Unicode support
	supportsUnicode = true
	if icon := getIcon("✓", "[+]"); icon != "✓" {
		t.Errorf("Expected Unicode icon '✓', got %q", icon)
	}
	
	// Test without Unicode support
	supportsUnicode = false
	if icon := getIcon("✓", "[+]"); icon != "[+]" {
		t.Errorf("Expected ASCII icon '[+]', got %q", icon)
	}
}

// Test Output Functions

func TestSuccess(t *testing.T) {
	output := captureOutput(t, func() {
		Success("Test success message")
	})
	
	if !strings.Contains(output, "Test success message") {
		t.Errorf("Output should contain message, got: %s", output)
	}
}

func TestError(t *testing.T) {
	output := captureOutput(t, func() {
		Error("Test error message")
	})
	
	if !strings.Contains(output, "Test error message") {
		t.Errorf("Output should contain message, got: %s", output)
	}
}

func TestWarning(t *testing.T) {
	output := captureOutput(t, func() {
		Warning("Test warning message")
	})
	
	if !strings.Contains(output, "Test warning message") {
		t.Errorf("Output should contain message, got: %s", output)
	}
}

func TestInfo(t *testing.T) {
	output := captureOutput(t, func() {
		Info("Test info message")
	})
	
	if !strings.Contains(output, "Test info message") {
		t.Errorf("Output should contain message, got: %s", output)
	}
}

func TestHeader(t *testing.T) {
	output := captureOutput(t, func() {
		Header("Test Header")
	})
	
	if !strings.Contains(output, "Test Header") {
		t.Errorf("Output should contain header text, got: %s", output)
	}
	
	// Should contain divider with same length as text
	if !strings.Contains(output, strings.Repeat("=", len("Test Header"))) {
		t.Errorf("Output should contain divider, got: %s", output)
	}
}

func TestCommandHeader(t *testing.T) {
	// Reset format and orchestration
	globalFormat = FormatDefault
	orchestratedMode = false
	
	// Test normal mode
	output := captureOutput(t, func() {
		CommandHeader("test", "description")
	})
	
	if !strings.Contains(output, "azd app test") {
		t.Errorf("Output should contain command name, got: %s", output)
	}
	
	// Test JSON mode (should skip header)
	globalFormat = FormatJSON
	output = captureOutput(t, func() {
		CommandHeader("test", "description")
	})
	
	if strings.Contains(output, "azd app test") {
		t.Errorf("JSON mode should skip header, got: %s", output)
	}
	
	// Test orchestrated mode (should skip header)
	globalFormat = FormatDefault
	orchestratedMode = true
	output = captureOutput(t, func() {
		CommandHeader("test", "description")
	})
	
	if strings.Contains(output, "azd app test") {
		t.Errorf("Orchestrated mode should skip header, got: %s", output)
	}
	
	// Reset
	globalFormat = FormatDefault
	orchestratedMode = false
}

func TestSection(t *testing.T) {
	output := captureOutput(t, func() {
		Section("🔧", "Test Section")
	})
	
	if !strings.Contains(output, "Test Section") {
		t.Errorf("Output should contain section text, got: %s", output)
	}
}

func TestStep(t *testing.T) {
	output := captureOutput(t, func() {
		Step("🔍", "Test step: %s", "searching")
	})
	
	if !strings.Contains(output, "Test step: searching") {
		t.Errorf("Output should contain step message, got: %s", output)
	}
}

func TestItem(t *testing.T) {
	output := captureOutput(t, func() {
		Item("Test item")
	})
	
	if !strings.Contains(output, "Test item") {
		t.Errorf("Output should contain item text, got: %s", output)
	}
	
	// Should be indented
	if !strings.Contains(output, "   ") {
		t.Errorf("Output should be indented, got: %s", output)
	}
}

func TestBullet(t *testing.T) {
	output := captureOutput(t, func() {
		Bullet("Test bullet")
	})
	
	if !strings.Contains(output, "Test bullet") {
		t.Errorf("Output should contain bullet text, got: %s", output)
	}
}

func TestItemSuccess(t *testing.T) {
	output := captureOutput(t, func() {
		ItemSuccess("Test success item")
	})
	
	if !strings.Contains(output, "Test success item") {
		t.Errorf("Output should contain item text, got: %s", output)
	}
}

func TestItemError(t *testing.T) {
	output := captureOutput(t, func() {
		ItemError("Test error item")
	})
	
	if !strings.Contains(output, "Test error item") {
		t.Errorf("Output should contain item text, got: %s", output)
	}
}

func TestItemWarning(t *testing.T) {
	output := captureOutput(t, func() {
		ItemWarning("Test warning item")
	})
	
	if !strings.Contains(output, "Test warning item") {
		t.Errorf("Output should contain item text, got: %s", output)
	}
}

func TestItemInfo(t *testing.T) {
	output := captureOutput(t, func() {
		ItemInfo("Test info item")
	})
	
	if !strings.Contains(output, "Test info item") {
		t.Errorf("Output should contain item text, got: %s", output)
	}
}

func TestDivider(t *testing.T) {
	output := captureOutput(t, func() {
		Divider()
	})
	
	if !strings.Contains(output, "─") {
		t.Errorf("Output should contain divider character, got: %s", output)
	}
}

func TestNewline(t *testing.T) {
	output := captureOutput(t, func() {
		Newline()
	})
	
	if output != "\n" {
		t.Errorf("Expected single newline, got: %q", output)
	}
}

func TestHint(t *testing.T) {
	// Test with no hints
	output := captureOutput(t, func() {
		Hint()
	})
	
	if output != "" {
		t.Errorf("Expected empty output for no hints, got: %s", output)
	}
	
	// Test with single hint
	output = captureOutput(t, func() {
		Hint("Press Ctrl+C to stop")
	})
	
	if !strings.Contains(output, "Press Ctrl+C to stop") {
		t.Errorf("Output should contain hint, got: %s", output)
	}
	
	// Test with multiple hints
	output = captureOutput(t, func() {
		Hint("Hint 1", "Hint 2", "Hint 3")
	})
	
	if !strings.Contains(output, "Hint 1") || !strings.Contains(output, "Hint 2") || !strings.Contains(output, "Hint 3") {
		t.Errorf("Output should contain all hints, got: %s", output)
	}
	
	if !strings.Contains(output, "•") {
		t.Errorf("Output should contain bullet separator, got: %s", output)
	}
}

func TestPhase(t *testing.T) {
	output := captureOutput(t, func() {
		Phase("Installing dependencies...")
	})
	
	if !strings.Contains(output, "Installing dependencies...") {
		t.Errorf("Output should contain phase label, got: %s", output)
	}
}

func TestPlain(t *testing.T) {
	output := captureOutput(t, func() {
		Plain("Plain text: %s", "test")
	})
	
	if !strings.Contains(output, "Plain text: test") {
		t.Errorf("Output should contain plain text, got: %s", output)
	}
}

func TestLabel(t *testing.T) {
	output := captureOutput(t, func() {
		Label("Name", "Value")
	})
	
	if !strings.Contains(output, "Name:") || !strings.Contains(output, "Value") {
		t.Errorf("Output should contain label and value, got: %s", output)
	}
}

func TestLabelColored(t *testing.T) {
	output := captureOutput(t, func() {
		LabelColored("Status", "Running", Green)
	})
	
	if !strings.Contains(output, "Status:") || !strings.Contains(output, "Running") {
		t.Errorf("Output should contain label and value, got: %s", output)
	}
}

// Test String Formatting Functions

func TestHighlight(t *testing.T) {
	result := Highlight("highlighted %s", "text")
	
	if !strings.Contains(result, "highlighted text") {
		t.Errorf("Result should contain formatted text, got: %s", result)
	}
	
	if !strings.Contains(result, Bold) || !strings.Contains(result, Cyan) || !strings.Contains(result, Reset) {
		t.Errorf("Result should contain formatting codes, got: %s", result)
	}
}

func TestEmphasize(t *testing.T) {
	result := Emphasize("emphasized %s", "text")
	
	if !strings.Contains(result, "emphasized text") {
		t.Errorf("Result should contain formatted text, got: %s", result)
	}
	
	if !strings.Contains(result, Bold) || !strings.Contains(result, Reset) {
		t.Errorf("Result should contain formatting codes, got: %s", result)
	}
}

func TestMuted(t *testing.T) {
	result := Muted("muted %s", "text")
	
	if !strings.Contains(result, "muted text") {
		t.Errorf("Result should contain formatted text, got: %s", result)
	}
	
	if !strings.Contains(result, Dim) || !strings.Contains(result, Reset) {
		t.Errorf("Result should contain formatting codes, got: %s", result)
	}
}

func TestURL(t *testing.T) {
	result := URL("https://example.com")
	
	if !strings.Contains(result, "https://example.com") {
		t.Errorf("Result should contain URL, got: %s", result)
	}
	
	if !strings.Contains(result, BrightBlue) || !strings.Contains(result, Reset) {
		t.Errorf("Result should contain formatting codes, got: %s", result)
	}
}

func TestCount(t *testing.T) {
	result := Count(42)
	
	if !strings.Contains(result, "42") {
		t.Errorf("Result should contain count, got: %s", result)
	}
	
	if !strings.Contains(result, Bold) || !strings.Contains(result, Reset) {
		t.Errorf("Result should contain formatting codes, got: %s", result)
	}
}

func TestStatus(t *testing.T) {
	tests := []struct {
		status   string
		expected string
	}{
		{"success", BrightGreen},
		{"ok", BrightGreen},
		{"running", BrightGreen},
		{"healthy", BrightGreen},
		{"warning", BrightYellow},
		{"pending", BrightYellow},
		{"starting", BrightYellow},
		{"error", BrightRed},
		{"failed", BrightRed},
		{"unhealthy", BrightRed},
		{"info", BrightBlue},
		{"unknown", BrightBlue},
		{"other", ""},
	}
	
	for _, tt := range tests {
		result := Status(tt.status)
		if !strings.Contains(result, tt.status) {
			t.Errorf("Status(%s) should contain status text, got: %s", tt.status, result)
		}
		
		if tt.expected != "" && !strings.Contains(result, tt.expected) {
			t.Errorf("Status(%s) should contain color code %s, got: %s", tt.status, tt.expected, result)
		}
	}
}

// Test Progress Bar

func TestProgressBar(t *testing.T) {
	// Test zero total
	if bar := ProgressBar(0, 0, 10); bar != "" {
		t.Errorf("Expected empty string for zero total, got: %s", bar)
	}
	
	// Test 0%
	bar := ProgressBar(0, 100, 10)
	if !strings.Contains(bar, "0%") {
		t.Errorf("Expected 0%%, got: %s", bar)
	}
	
	// Test 50%
	bar = ProgressBar(50, 100, 10)
	if !strings.Contains(bar, "50%") {
		t.Errorf("Expected 50%%, got: %s", bar)
	}
	if !strings.Contains(bar, "█") || !strings.Contains(bar, "░") {
		t.Errorf("Expected filled and empty characters, got: %s", bar)
	}
	
	// Test 100%
	bar = ProgressBar(100, 100, 10)
	if !strings.Contains(bar, "100%") {
		t.Errorf("Expected 100%%, got: %s", bar)
	}
}

// Test Table

func TestTable(t *testing.T) {
	// Test empty rows
	output := captureOutput(t, func() {
		Table([]string{"Name", "Status"}, []TableRow{})
	})
	
	if output != "" {
		t.Errorf("Expected empty output for empty rows, got: %s", output)
	}
	
	// Test with rows
	headers := []string{"Name", "Status", "Port"}
	rows := []TableRow{
		{"Name": "web", "Status": "running", "Port": "8080"},
		{"Name": "api", "Status": "stopped", "Port": "3000"},
	}
	
	output = captureOutput(t, func() {
		Table(headers, rows)
	})
	
	// Should contain headers
	if !strings.Contains(output, "Name") || !strings.Contains(output, "Status") || !strings.Contains(output, "Port") {
		t.Errorf("Output should contain headers, got: %s", output)
	}
	
	// Should contain row data
	if !strings.Contains(output, "web") || !strings.Contains(output, "running") || !strings.Contains(output, "8080") {
		t.Errorf("Output should contain first row, got: %s", output)
	}
	
	if !strings.Contains(output, "api") || !strings.Contains(output, "stopped") || !strings.Contains(output, "3000") {
		t.Errorf("Output should contain second row, got: %s", output)
	}
	
	// Should contain separator
	if !strings.Contains(output, "─") {
		t.Errorf("Output should contain separator, got: %s", output)
	}
}

// Test JSON Output

func TestPrintJSON(t *testing.T) {
	data := map[string]interface{}{
		"status": "success",
		"count":  42,
	}
	
	output := captureOutput(t, func() {
		if err := PrintJSON(data); err != nil {
			t.Errorf("PrintJSON failed: %v", err)
		}
	})
	
	// Should be valid JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
	
	// Should contain expected fields
	if decoded["status"] != "success" {
		t.Errorf("Expected status=success, got: %v", decoded["status"])
	}
	
	if decoded["count"] != float64(42) {
		t.Errorf("Expected count=42, got: %v", decoded["count"])
	}
}

func TestPrintDefault(t *testing.T) {
	// Test default format
	globalFormat = FormatDefault
	called := false
	PrintDefault(func() {
		called = true
	})
	
	if !called {
		t.Error("Formatter should be called in default format")
	}
	
	// Test JSON format
	globalFormat = FormatJSON
	called = false
	PrintDefault(func() {
		called = true
	})
	
	if called {
		t.Error("Formatter should not be called in JSON format")
	}
	
	// Reset
	globalFormat = FormatDefault
}

func TestPrint(t *testing.T) {
	data := map[string]interface{}{"status": "success"}
	
	// Test default format
	globalFormat = FormatDefault
	formatterCalled := false
	
	err := Print(data, func() {
		formatterCalled = true
	})
	
	if err != nil {
		t.Errorf("Print failed: %v", err)
	}
	
	if !formatterCalled {
		t.Error("Formatter should be called in default format")
	}
	
	// Test JSON format
	globalFormat = FormatJSON
	formatterCalled = false
	
	output := captureOutput(t, func() {
		err := Print(data, func() {
			formatterCalled = true
		})
		if err != nil {
			t.Errorf("Print failed: %v", err)
		}
	})
	
	if formatterCalled {
		t.Error("Formatter should not be called in JSON format")
	}
	
	// Should output JSON
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Errorf("Output is not valid JSON: %v", err)
	}
	
	// Reset
	globalFormat = FormatDefault
}

// Test Confirm

func TestConfirmJSONMode(t *testing.T) {
	// In JSON mode, should always return true
	globalFormat = FormatJSON
	
	result := Confirm("Do you want to continue?")
	
	if !result {
		t.Error("Confirm should return true in JSON mode")
	}
	
	// Reset
	globalFormat = FormatDefault
}

// Note: Interactive Confirm testing in default mode would require simulating stdin,
// which is complex. The JSON mode test covers the non-interactive behavior.
