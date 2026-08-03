// Package version provides shared version metadata and a reusable version
// command for azd extensions, so each extension does not reimplement the same
// boilerplate.
//
// [Info] holds the values normally injected at build time through ldflags.
// [NewCommand] turns an [Info] into a cobra command that prints either a
// human-readable summary, the bare version with --quiet, or the full struct as
// JSON.
//
// Typical wiring, alongside a root command from
// [github.com/azure/azure-dev/cli/azd/pkg/azdext.NewExtensionRootCommand]:
//
//	var Info = version.New("jongio.azd.example", "azd example")
//
//	rootCmd, extCtx := azdext.NewExtensionRootCommand(opts)
//	rootCmd.AddCommand(version.NewCommand(Info, &extCtx.OutputFormat))
//
// That form declares the supported --output values to azd, which drives shell
// completion, help text, extension metadata, and parse-time validation.
//
// Two things are configurable because hardcoding either one breaks real
// extensions:
//
//   - [WithQuietShorthand] changes or removes the -q shorthand on --quiet.
//     cobra panics during flag parsing when a subcommand shorthand collides
//     with an inherited persistent flag, so an extension that already uses -q
//     must pass "".
//
//   - [WithOutputFlag] names the flag that supplies the outputFormat pointer,
//     or disables the declaration with an empty name. An extension that reads
//     its own output flag rather than azd's --output should disable it, so azd
//     does not validate a flag the command never reads.
package version
