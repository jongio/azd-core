package version

import (
	"encoding/json"
	"fmt"

	"github.com/jongio/azd-core/cliout"
	"github.com/spf13/cobra"
)

// NewCommand creates a version command that displays extension version info.
// outputFormat is an optional pointer to a global output format flag (e.g. "json").
// If nil, defaults to human-readable output.
func NewCommand(info *Info, outputFormat *string) *cobra.Command {
	var quiet bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: fmt.Sprintf("Display %s version information", info.Name),
		RunE: func(cmd *cobra.Command, args []string) error {
			format := ""
			if outputFormat != nil {
				format = *outputFormat
			}

			if format == "json" {
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
	cmd.Flags().BoolVarP(&quiet, "quiet", "q", false, "Only print version number")
	return cmd
}
