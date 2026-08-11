package env

import (
	"context"
	"strings"
	"testing"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// recordingRunner captures the argv it was asked to run so a test can assert
// how the azd CLI was invoked.
type recordingRunner struct {
	args   []string
	output []byte
}

func (r *recordingRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.args = append([]string{name}, args...)
	if r.output == nil {
		return []byte("{}"), nil
	}
	return r.output, nil
}

// TestSDKLoaderCannotTargetNamedEnvironment pins the reason this package keeps
// its own loader instead of delegating to azdext.LoadAzdEnvironment.
//
// The SDK function runs "azd env get-values" with no -e flag, so it always
// reads whichever environment azd currently considers default. This package
// exists specifically to honor an explicit -e, because azd injects the default
// environment into the extension process and only then passes -e through.
// Swapping in the SDK function would make "-e staging" silently read the
// default environment.
func TestSDKLoaderCannotTargetNamedEnvironment(t *testing.T) {
	runner := &recordingRunner{}
	prev := SetCommandRunner(runner)
	t.Cleanup(func() { SetCommandRunner(prev) })

	if _, err := GetAzdEnvironmentValues(context.Background(), "staging"); err != nil {
		t.Fatalf("GetAzdEnvironmentValues returned error: %v", err)
	}

	joined := strings.Join(runner.args, " ")
	if !strings.Contains(joined, "-e staging") {
		t.Errorf("expected the environment name to be passed through, got %q", joined)
	}
	if !strings.Contains(joined, "--output json") {
		t.Errorf("expected the JSON output path to be preferred, got %q", joined)
	}
}

// TestNamedEnvironmentValidationRejectsInjection pins the allowlist the SDK
// loader has no equivalent for. The environment name reaches an exec argv.
func TestNamedEnvironmentValidationRejectsInjection(t *testing.T) {
	runner := &recordingRunner{}
	prev := SetCommandRunner(runner)
	t.Cleanup(func() { SetCommandRunner(prev) })

	for _, name := range []string{"", "a b", "a;rm -rf /", "a$(id)", "a`id`", "a|b", "a&b"} {
		if _, err := GetAzdEnvironmentValues(context.Background(), name); err == nil {
			t.Errorf("expected %q to be rejected as an environment name", name)
		}
	}
	if runner.args != nil {
		t.Errorf("a rejected name must never reach the command runner, got %v", runner.args)
	}
}

// TestParserDivergesFromSDK pins the two cases where this package's KEY=VALUE
// parser deliberately differs from azdext.ParseEnvironmentVariables. Both
// differences change the value a caller receives, so the SDK parser is not a
// drop-in replacement. If the SDK later matches this behavior, this test
// starts failing and the delegation becomes safe.
func TestParserDivergesFromSDK(t *testing.T) {
	tests := []struct {
		name  string
		line  string
		key   string
		local string
		sdk   string
	}{
		{
			name:  "single quotes are stripped locally but kept by the SDK",
			line:  "KEY='single'",
			key:   "KEY",
			local: "single",
			sdk:   "'single'",
		},
		{
			name:  "leading whitespace is preserved locally but trimmed by the SDK",
			line:  "KEY=  padded",
			key:   "KEY",
			local: "  padded",
			sdk:   "padded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			local, err := ParseKeyValueFormat([]byte(tt.line))
			if err != nil {
				t.Fatalf("ParseKeyValueFormat returned error: %v", err)
			}
			if got := local[tt.key]; got != tt.local {
				t.Errorf("local parser: got %q, want %q", got, tt.local)
			}

			sdk := azdext.ParseEnvironmentVariables([]string{tt.line})
			if got := sdk[tt.key]; got != tt.sdk {
				t.Errorf("SDK parser changed behavior: got %q, want %q. "+
					"Re-evaluate whether ParseKeyValueFormat can now delegate.", got, tt.sdk)
			}
		})
	}
}

// TestParserAgreesWithSDKOnCommonForms documents the cases where the two
// parsers already match, so the divergence above is understood as narrow rather
// than as a wholesale reimplementation.
func TestParserAgreesWithSDKOnCommonForms(t *testing.T) {
	lines := []string{
		"KEY=value",
		`KEY="quoted"`,
		"KEY=a=b=c",
		"KEY=trailing   ",
		"EMPTY=",
		"=noKey",
		"# comment",
		"",
		`KEY="  inner pad  "`,
	}

	for _, line := range lines {
		local, err := ParseKeyValueFormat([]byte(line))
		if err != nil {
			t.Fatalf("ParseKeyValueFormat(%q) returned error: %v", line, err)
		}
		sdk := azdext.ParseEnvironmentVariables([]string{line})

		if len(local) != len(sdk) {
			t.Errorf("%q: entry count differs, local=%v sdk=%v", line, local, sdk)
			continue
		}
		for k, v := range local {
			if sdk[k] != v {
				t.Errorf("%q: key %q differs, local=%q sdk=%q", line, k, v, sdk[k])
			}
		}
	}
}
