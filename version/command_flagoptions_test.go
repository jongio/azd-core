package version

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
	"github.com/spf13/cobra"
)

func TestNewCommand_RegistersOutputFlagOptions(t *testing.T) {
	format := OutputFormatDefault
	cmd := NewCommand(New("jongio.azd.test", "azd test"), &format)

	raw, ok := cmd.Annotations["azdext.allowed-values/"+OutputFlagName]
	if !ok {
		t.Fatalf("expected an allowed-values annotation for --%s, got annotations %v",
			OutputFlagName, cmd.Annotations)
	}

	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("allowed-values annotation is not a JSON string array: %v", err)
	}

	want := []string{OutputFormatDefault, OutputFormatJSON}
	if len(values) != len(want) {
		t.Fatalf("allowed values = %v, want %v", values, want)
	}
	for i := range want {
		if values[i] != want[i] {
			t.Fatalf("allowed values = %v, want %v", values, want)
		}
	}
}

func TestNewCommand_RegistersOutputUsage(t *testing.T) {
	format := OutputFormatDefault
	cmd := NewCommand(New("jongio.azd.test", "azd test"), &format)

	got := cmd.Annotations["azdext.usage/"+OutputFlagName]
	if got == "" {
		t.Fatal("expected a usage annotation for --output")
	}
	if !strings.Contains(got, "version") {
		t.Errorf("usage annotation = %q, want it to mention version", got)
	}
}

// TestVersionCommand_RejectsUnsupportedOutput exercises the annotation end to
// end through a real SDK root, which is where the validation actually runs.
func TestVersionCommand_RejectsUnsupportedOutput(t *testing.T) {
	root, extCtx := azdext.NewExtensionRootCommand(azdext.ExtensionCommandOptions{
		Name:    "test",
		Version: "0.0.0-dev",
	})
	root.AddCommand(NewCommand(New("jongio.azd.test", "azd test"), &extCtx.OutputFormat))
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})

	root.SetArgs([]string{"version", "--output", "table"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected --output table to be rejected")
	}
	if !strings.Contains(err.Error(), "table") {
		t.Errorf("error = %q, want it to name the rejected value", err)
	}
	if !strings.Contains(err.Error(), OutputFormatJSON) {
		t.Errorf("error = %q, want it to list the supported values", err)
	}
}

func TestVersionCommand_AcceptsSupportedOutput(t *testing.T) {
	for _, format := range []string{OutputFormatDefault, OutputFormatJSON} {
		t.Run(format, func(t *testing.T) {
			root, extCtx := azdext.NewExtensionRootCommand(azdext.ExtensionCommandOptions{
				Name:    "test",
				Version: "0.0.0-dev",
			})
			root.AddCommand(NewCommand(New("jongio.azd.test", "azd test"), &extCtx.OutputFormat))
			root.SetOut(&bytes.Buffer{})
			root.SetErr(&bytes.Buffer{})

			root.SetArgs([]string{"version", "--output", format})
			if err := root.Execute(); err != nil {
				t.Fatalf("--output %s was rejected: %v", format, err)
			}
		})
	}
}

// TestVersionCommand_UsageListsSupportedValues covers the help rendering path,
// which reads the same annotation through the root's wrapped UsageFunc. It has
// to go through Execute because cobra only merges inherited persistent flags
// into cmd.Flags() during ParseFlags, and the override lookup needs them there.
func TestVersionCommand_UsageListsSupportedValues(t *testing.T) {
	root, extCtx := azdext.NewExtensionRootCommand(azdext.ExtensionCommandOptions{
		Name:    "test",
		Version: "0.0.0-dev",
	})
	root.AddCommand(NewCommand(New("jongio.azd.test", "azd test"), &extCtx.OutputFormat))

	var out bytes.Buffer
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"version", "--help"})
	if err := root.Execute(); err != nil {
		t.Fatalf("version --help failed: %v", err)
	}

	usage := out.String()
	if !strings.Contains(usage, "supported:") {
		t.Errorf("usage output does not list supported values:\n%s", usage)
	}
	if !strings.Contains(usage, OutputFormatJSON) {
		t.Errorf("usage output does not mention %q:\n%s", OutputFormatJSON, usage)
	}
	if !strings.Contains(usage, "The output format for version information") {
		t.Errorf("usage output does not use the per-command usage text:\n%s", usage)
	}
}

// TestNewCommand_QuietShorthandCollision is a regression test for a crash:
// cobra panics during flag parsing when a subcommand shorthand collides with
// one inherited from a parent persistent flag. azd-rest binds -q to --query,
// so the default -q on --quiet took the whole extension down on every
// invocation.
func TestNewCommand_QuietShorthandCollision(t *testing.T) {
	newRoot := func() *cobra.Command {
		root, _ := azdext.NewExtensionRootCommand(azdext.ExtensionCommandOptions{
			Name:    "test",
			Version: "0.0.0-dev",
		})
		var query string
		root.PersistentFlags().StringVarP(&query, "query", "q", "", "A query")
		return root
	}

	t.Run("default shorthand panics", func(t *testing.T) {
		root := newRoot()
		format := OutputFormatDefault
		root.AddCommand(NewCommand(New("jongio.azd.test", "azd test"), &format))
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"version"})

		defer func() {
			if recover() == nil {
				t.Fatal("expected a shorthand collision panic; if cobra stopped panicking, " +
					"WithQuietShorthand may no longer be needed")
			}
		}()
		_ = root.Execute()
	})

	t.Run("empty shorthand is safe", func(t *testing.T) {
		root := newRoot()
		format := OutputFormatDefault
		root.AddCommand(NewCommand(New("jongio.azd.test", "azd test"), &format, WithQuietShorthand("")))
		root.SetOut(&bytes.Buffer{})
		root.SetErr(&bytes.Buffer{})
		root.SetArgs([]string{"version"})

		if err := root.Execute(); err != nil {
			t.Fatalf("version failed: %v", err)
		}
	})
}

func TestWithQuietShorthand(t *testing.T) {
	tests := []struct {
		name      string
		opts      []Option
		wantShort string
	}{
		{"default", nil, "q"},
		{"disabled", []Option{WithQuietShorthand("")}, ""},
		{"renamed", []Option{WithQuietShorthand("Q")}, "Q"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			format := OutputFormatDefault
			cmd := NewCommand(New("jongio.azd.test", "azd test"), &format, tt.opts...)
			flag := cmd.Flags().Lookup("quiet")
			if flag == nil {
				t.Fatal("--quiet was not registered")
			}
			if flag.Shorthand != tt.wantShort {
				t.Errorf("shorthand = %q, want %q", flag.Shorthand, tt.wantShort)
			}
		})
	}
}

func TestWithOutputFlag(t *testing.T) {
	format := OutputFormatDefault

	t.Run("disabled skips the declaration", func(t *testing.T) {
		cmd := NewCommand(New("jongio.azd.test", "azd test"), &format, WithOutputFlag(""))
		for key := range cmd.Annotations {
			if strings.HasPrefix(key, "azdext.") {
				t.Errorf("expected no azdext annotations, found %q", key)
			}
		}
	})

	t.Run("renamed flag and values", func(t *testing.T) {
		cmd := NewCommand(New("jongio.azd.test", "azd test"), &format,
			WithOutputFlag("format", "auto", OutputFormatJSON))

		raw, ok := cmd.Annotations["azdext.allowed-values/format"]
		if !ok {
			t.Fatalf("expected an allowed-values annotation for --format, got %v", cmd.Annotations)
		}
		var values []string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			t.Fatalf("annotation is not a JSON array: %v", err)
		}
		if len(values) != 2 || values[0] != "auto" || values[1] != OutputFormatJSON {
			t.Errorf("allowed values = %v, want [auto json]", values)
		}
		if _, ok := cmd.Annotations["azdext.allowed-values/"+OutputFlagName]; ok {
			t.Error("the default output flag should not also be declared")
		}
	})
}
