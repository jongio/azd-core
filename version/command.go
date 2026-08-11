package version

import (
	"encoding/json"
	"fmt"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
	"github.com/jongio/azd-core/cliout"
	"github.com/spf13/cobra"
)

// Option customizes the command returned by [NewCommand].
type Option func(*options)

type options struct {
	quietShorthand   string
	outputFlagName   string
	outputFlagValues []string
}

// WithQuietShorthand overrides the single-letter shorthand bound to --quiet,
// which defaults to "q". Pass an empty string to register --quiet with no
// shorthand at all.
//
// This exists because cobra panics when a subcommand's own shorthand collides
// with one it inherits from a parent's persistent flags, and the panic happens
// during flag parsing rather than at construction. An extension that already
// binds -q to one of its own persistent flags must pass "" here, otherwise
// every invocation of the extension crashes.
func WithQuietShorthand(shorthand string) Option {
	return func(o *options) { o.quietShorthand = shorthand }
}

// WithOutputFlag names the flag that supplies the outputFormat pointer passed
// to [NewCommand], along with the values it accepts. It defaults to "output"
// with "default" and "json", matching the flag [azdext.NewExtensionRootCommand]
// registers.
//
// Pass an empty name to skip the [azdext.RegisterFlagOptions] declaration
// entirely. Extensions that bind outputFormat to a flag of their own, with
// their own vocabulary, should do that: declaring the wrong flag name makes
// azd validate and complete a flag the command never reads.
func WithOutputFlag(name string, allowedValues ...string) Option {
	return func(o *options) {
		o.outputFlagName = name
		o.outputFlagValues = allowedValues
	}
}

// NewCommand creates a version command that displays extension version info.
// outputFormat is an optional pointer to a global output format flag (e.g. "json").
// If nil, defaults to human-readable output.
//
// By default the command declares its supported --output values through
// [azdext.RegisterFlagOptions]. When the root command comes from
// [azdext.NewExtensionRootCommand], that declaration gives azd shell
// completion, rejects unsupported values before RunE, and surfaces the
// values in extension metadata. See [WithOutputFlag] to point it at a
// different flag or turn it off.
func NewCommand(info *Info, outputFormat *string, opts ...Option) *cobra.Command {
	cfg := options{
		quietShorthand: "q",
		outputFlagName: OutputFlagName,
		// "default" must stay in the list: the SDK root registers --output
		// with a "default" default value, and omitting it here would reject
		// an explicit --output default.
		outputFlagValues: []string{OutputFormatDefault, OutputFormatJSON},
	}
	for _, apply := range opts {
		apply(&cfg)
	}

	var quiet bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Display %s version information", info.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := ""
			if outputFormat != nil {
				format = *outputFormat
			}

			if format == OutputFormatJSON {
				data, err := json.MarshalIndent(info, "", "  ")
				if err != nil {
					return err
				}
				fmt.Println(string(data))
				return nil
			}

			if quiet {
				fmt.Println(info.Version)
				return nil
			}

			cliout.Header(fmt.Sprintf("%s Version", info.Name))
			cliout.Label("Version", info.Version)
			cliout.Label("Build Date", info.BuildDate)
			cliout.Label("Git Commit", info.GitCommit)
			cliout.Label("Extension ID", info.ExtensionID)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&quiet, "quiet", cfg.quietShorthand, false, "Only print version number")

	if cfg.outputFlagName != "" {
		azdext.RegisterFlagOptions(cmd, azdext.FlagOptions{
			Name:          cfg.outputFlagName,
			AllowedValues: cfg.outputFlagValues,
			Usage:         "The output format for version information",
		})
	}

	return cmd
}
