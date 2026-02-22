package azdextutil

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// The azdext version produces the correct framework schema (extensions.ExtensionCommandMetadata).
//
// ExtensionMetadata represents the metadata output for an azd extension.
type ExtensionMetadata struct {
	SchemaVersion string            `json:"schemaVersion"`
	ID            string            `json:"id"`
	Commands      []CommandMetadata `json:"commands"`
	Configuration *ConfigMetadata   `json:"configuration,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// CommandMetadata describes a single command in the extension.
type CommandMetadata struct {
	Name        []string          `json:"name"`
	Short       string            `json:"short"`
	Long        string            `json:"long,omitempty"`
	Usage       string            `json:"usage,omitempty"`
	Examples    []ExampleMetadata `json:"examples,omitempty"`
	Args        []ArgMetadata     `json:"args,omitempty"`
	Flags       []FlagMetadata    `json:"flags,omitempty"`
	Subcommands []CommandMetadata `json:"subcommands,omitempty"`
	Hidden      bool              `json:"hidden,omitempty"`
	Aliases     []string          `json:"aliases,omitempty"`
	Deprecated  string            `json:"deprecated,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// ExampleMetadata describes a usage example for a command.
type ExampleMetadata struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Command     string `json:"command,omitempty"`
	Usage       string `json:"usage,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// ArgMetadata describes a positional argument for a command.
type ArgMetadata struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Required    bool     `json:"required,omitempty"`
	Variadic    bool     `json:"variadic,omitempty"`
	ValidValues []string `json:"validValues,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// FlagMetadata describes a flag for a command.
type FlagMetadata struct {
	Name        string `json:"name"`
	Shorthand   string `json:"shorthand,omitempty"`
	Description string `json:"description"`
	Type        string `json:"type"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required,omitempty"`
	Hidden      bool   `json:"hidden,omitempty"`
	Deprecated  string `json:"deprecated,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// ConfigMetadata describes configuration for the extension.
type ConfigMetadata struct {
	EnvironmentVariables []EnvVarMetadata `json:"environmentVariables,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
//
// EnvVarMetadata describes an environment variable used by the extension.
type EnvVarMetadata struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default,omitempty"`
	Example     string `json:"example,omitempty"`
}

// Deprecated: Use azdext.GenerateExtensionMetadata() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// The azdext version produces the correct framework schema (extensions.ExtensionCommandMetadata).
//
// GenerateMetadataFromCobra generates ExtensionMetadata by introspecting a Cobra command tree.
// This is a lightweight alternative to azdext.GenerateExtensionMetadata that doesn't
// require the azure-dev/cli/azd dependency.
func GenerateMetadataFromCobra(schemaVersion, extensionID string, rootCmd *cobra.Command) *ExtensionMetadata {
	return &ExtensionMetadata{
		SchemaVersion: schemaVersion,
		ID:            extensionID,
		Commands:      generateCommands(rootCmd),
	}
}

// Deprecated: Use azdext.NewMetadataCommand() from github.com/azure/azure-dev/cli/azd/pkg/azdext instead.
// The azdext version produces the correct framework schema (extensions.ExtensionCommandMetadata).
//
// NewMetadataCommand creates a standard hidden metadata command that outputs
// extension metadata as JSON to stdout.
func NewMetadataCommand(extensionID string, rootCmdProvider func() *cobra.Command) *cobra.Command {
	return &cobra.Command{
		Use:    "metadata",
		Short:  "Generate extension metadata",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := rootCmdProvider()
			metadata := GenerateMetadataFromCobra("1.0", extensionID, root)
			data, err := json.MarshalIndent(metadata, "", "  ")
			if err != nil {
				return fmt.Errorf("failed to marshal metadata: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func generateCommands(cmd *cobra.Command) []CommandMetadata {
	var commands []CommandMetadata
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" || child.Name() == "completion" {
			continue
		}
		commands = append(commands, generateCommand(cmd, child))
	}
	return commands
}

func generateCommand(parent *cobra.Command, cmd *cobra.Command) CommandMetadata {
	meta := CommandMetadata{
		Name:  buildCommandPath(parent, cmd),
		Short: cmd.Short,
		Long:  cmd.Long,
		Usage: cmd.UseLine(),
	}

	// Collect flags
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		meta.Flags = append(meta.Flags, FlagMetadata{
			Name:        f.Name,
			Shorthand:   f.Shorthand,
			Description: f.Usage,
			Type:        f.Value.Type(),
			Default:     f.DefValue,
		})
	})

	// Collect subcommands
	for _, child := range cmd.Commands() {
		if child.Hidden || child.Name() == "help" {
			continue
		}
		meta.Subcommands = append(meta.Subcommands, generateCommand(cmd, child))
	}

	return meta
}

func buildCommandPath(parent *cobra.Command, cmd *cobra.Command) []string {
	if parent != nil && parent.Name() != "" && parent.HasParent() {
		return append(buildCommandPath(parent.Parent(), parent), cmd.Name())
	}
	return []string{cmd.Name()}
}
