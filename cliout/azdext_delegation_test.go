package cliout

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// clearInteractiveEnv removes every environment variable that
// azdext.DetectInteractive consults so a test starts from a known state. The
// suite itself runs under CI and often under an AI agent host, both of which
// would otherwise make CanPrompt report false for the wrong reason.
func clearInteractiveEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"AZD_NO_PROMPT",
		"CI", "GITHUB_ACTIONS", "TF_BUILD", "JENKINS_URL", "GITLAB_CI",
		"CIRCLECI", "TRAVIS", "BUILDKITE", "CODEBUILD_BUILD_ID",
		"CLAUDE_CODE", "CLAUDE_CODE_ENTRYPOINT", "GITHUB_COPILOT_CLI",
		"GH_COPILOT", "GEMINI_CLI", "GEMINI_CLI_NO_RELAUNCH", "OPENCODE",
		"AZURE_DEV_AGENT_TYPE",
		"NO_COLOR", "FORCE_COLOR",
	} {
		t.Setenv(k, "")
	}
}

// withColor restores the package color flag after a test mutates it.
func withColor(t *testing.T, disabled bool) {
	t.Helper()
	mu.Lock()
	prev := noColor
	noColor = disabled
	mu.Unlock()
	t.Cleanup(func() {
		mu.Lock()
		noColor = prev
		mu.Unlock()
	})
}

func TestStyledStripsAnsiWhenColorDisabled(t *testing.T) {
	withColor(t, true)

	got := styled(Bold + Cyan + "hello" + Reset)
	if got != "hello" {
		t.Errorf("expected escapes stripped, got %q", got)
	}

	if got := styled("plain"); got != "plain" {
		t.Errorf("plain text should pass through, got %q", got)
	}
}

func TestStyledPreservesAnsiWhenColorEnabled(t *testing.T) {
	withColor(t, false)

	in := Bold + "hello" + Reset
	if got := styled(in); got != in {
		t.Errorf("expected escapes preserved, got %q", got)
	}
}

func TestPrintersRespectNoColor(t *testing.T) {
	withColor(t, true)

	output := captureOutput(t, func() {
		Success("done %s", "now")
		Header("Title")
	})

	if strings.Contains(output, "\x1b[") {
		t.Errorf("expected no ANSI escapes in output, got %q", output)
	}
	if !strings.Contains(output, "done now") {
		t.Errorf("expected message text retained, got %q", output)
	}
}

func TestPrintersEmitColorWhenEnabled(t *testing.T) {
	withColor(t, false)

	output := captureOutput(t, func() {
		Success("done")
	})

	if !strings.Contains(output, "\x1b[") {
		t.Errorf("expected ANSI escapes in output, got %q", output)
	}
}

// NoColor and ForceColor are documented as overriding the environment probe.
// Before delegation they were inert because nothing consulted getNoColor.
func TestNoColorOverrideActuallySuppressesOutput(t *testing.T) {
	withColor(t, false)

	NoColor()
	suppressed := captureOutput(t, func() { Success("x") })
	if strings.Contains(suppressed, "\x1b[") {
		t.Errorf("NoColor should suppress escapes, got %q", suppressed)
	}

	ForceColor()
	colored := captureOutput(t, func() { Success("x") })
	if !strings.Contains(colored, "\x1b[") {
		t.Errorf("ForceColor should restore escapes, got %q", colored)
	}
}

func TestNewAzdextOutputFormatMapping(t *testing.T) {
	if newAzdextOutput(FormatJSON).IsJSON() != true {
		t.Error("FormatJSON should map to the SDK JSON format")
	}
	if newAzdextOutput(FormatDefault).IsJSON() != false {
		t.Error("FormatDefault should map to the SDK default format")
	}
}

func TestPrintJSONDelegatesToAzdext(t *testing.T) {
	type payload struct {
		Name  string `json:"name"`
		Count int    `json:"count"`
	}

	output := captureOutput(t, func() {
		if err := PrintJSON(payload{Name: "web", Count: 2}); err != nil {
			t.Errorf("PrintJSON returned error: %v", err)
		}
	})

	var got payload
	if err := json.Unmarshal([]byte(output), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v (%q)", err, output)
	}
	if got.Name != "web" || got.Count != 2 {
		t.Errorf("round trip mismatch: %+v", got)
	}
	// The SDK encoder keeps the two space indent azd-core used before.
	if !strings.Contains(output, "\n  \"name\"") {
		t.Errorf("expected two space indent, got %q", output)
	}
}

func TestPrintJSONReturnsErrorForUnencodableValue(t *testing.T) {
	captureOutput(t, func() {
		if err := PrintJSON(make(chan int)); err == nil {
			t.Error("expected an encoding error for a channel value")
		}
	})
}

func TestTableSubstitutesMissingKeys(t *testing.T) {
	headers := []string{"Name", "Status", "Port"}
	rows := []TableRow{
		{"Name": "web", "Port": "8080"}, // Status intentionally absent
	}

	output := captureOutput(t, func() { Table(headers, rows) })

	if !strings.Contains(output, "web") || !strings.Contains(output, "8080") {
		t.Errorf("expected present cells rendered, got %q", output)
	}
	if strings.Contains(output, "map[") {
		t.Errorf("row map leaked into output: %q", output)
	}
}

func TestTableEmitsJSONInJSONMode(t *testing.T) {
	prev := GetFormat()
	t.Cleanup(func() { _ = SetFormat(string(prev)) })
	if err := SetFormat("json"); err != nil {
		t.Fatal(err)
	}

	output := captureOutput(t, func() {
		Table([]string{"Name"}, []TableRow{{"Name": "web"}})
	})

	var rows []map[string]string
	if err := json.Unmarshal([]byte(output), &rows); err != nil {
		t.Fatalf("expected JSON rows in JSON mode, got %q (%v)", output, err)
	}
	if len(rows) != 1 || rows[0]["Name"] != "web" {
		t.Errorf("unexpected JSON rows: %+v", rows)
	}
}

func TestConfirmDeclinesWhenPromptingIsImpossible(t *testing.T) {
	clearInteractiveEnv(t)
	t.Setenv("AZD_NO_PROMPT", "true")

	prev := GetFormat()
	t.Cleanup(func() { _ = SetFormat(string(prev)) })
	if err := SetFormat("default"); err != nil {
		t.Fatal(err)
	}

	if azdext.DetectInteractive().CanPrompt() {
		t.Fatal("test precondition failed: prompting should be disabled")
	}

	done := make(chan bool, 1)
	go func() { done <- Confirm("proceed?") }()

	select {
	case got := <-done:
		if got {
			t.Error("Confirm should decline when it cannot prompt")
		}
	case <-time.After(time.Second):
		t.Fatal("Confirm blocked on stdin instead of declining")
	}
}

func TestConfirmStillAssumesYesInJSONMode(t *testing.T) {
	prev := GetFormat()
	t.Cleanup(func() { _ = SetFormat(string(prev)) })
	if err := SetFormat("json"); err != nil {
		t.Fatal(err)
	}

	if !Confirm("proceed?") {
		t.Error("JSON mode should keep assuming yes")
	}
}
