// Package cliout provides structured output formatting for CLI commands with
// cross-platform terminal support and multiple output formats.
//
// # Features
//
//   - Multiple output formats (default human-readable and JSON)
//   - ANSI color support with consistent color scheme
//   - Unicode/emoji detection with ASCII fallbacks for legacy terminals
//   - Orchestration mode for composing subcommands
//   - Progress bars, tables, and interactive prompts
//   - Cross-platform terminal detection (Windows Terminal, VS Code, PowerShell, ConEmu)
//
// # Basic Usage
//
//	import "github.com/jongio/azd-core/cliout"
//
//	// Print success message
//	cliout.Success("Operation completed successfully")
//
//	// Print error message
//	cliout.Error("Operation failed: %s", err)
//
//	// Print warning
//	cliout.Warning("This feature is deprecated")
//
//	// Print info message
//	cliout.Info("Processing %d items", count)
//
// # Output Formats
//
// The package supports two output formats:
//   - default: Human-readable text with colors and Unicode symbols
//   - json: Structured JSON output for automation and scripting
//
// Set the output format using SetFormat:
//
//	if err := cliout.SetFormat("json"); err != nil {
//	    log.Fatal(err)
//	}
//
// Check the current format:
//
//	if cliout.IsJSON() {
//	    // Skip interactive prompts
//	}
//
// # Unicode Detection
//
// The package automatically detects terminal Unicode support and falls back to
// ASCII symbols on legacy terminals. Detection includes:
//   - Windows Terminal (via WT_SESSION environment variable)
//   - VS Code integrated terminal (via TERM_PROGRAM environment variable)
//   - PowerShell (via PSModulePath or POWERSHELL_DISTRIBUTION_CHANNEL)
//   - ConEmu (via ConEmuPID environment variable)
//   - Unix-like systems (assumed to support Unicode)
//
// Old Windows Command Prompt (cmd.exe) without these environment variables will
// use ASCII fallback symbols.
//
// # Orchestration Mode
//
// Orchestration mode allows composing multiple commands while suppressing redundant
// headers from subcommands:
//
//	cliout.SetOrchestrated(true)
//	// Now CommandHeader() calls will be skipped
//
// This is useful when building workflows that chain multiple CLI commands together.
//
// # Hybrid Output
//
// The Print function supports hybrid output where you provide both JSON data and
// a formatter function:
//
//	data := map[string]interface{}{"status": "success", "count": 42}
//	err := cliout.Print(data, func() {
//	    cliout.Success("Processed %d items", 42)
//	})
//
// In JSON mode, the data is marshaled to JSON. In default mode, the formatter is called.
//
// # Tables
//
// Create simple tables with automatic column width calculation:
//
//	headers := []string{"Name", "Status", "Port"}
//	rows := []cliout.TableRow{
//	    {"Name": "web", "Status": "running", "Port": "8080"},
//	    {"Name": "api", "Status": "stopped", "Port": "3000"},
//	}
//	cliout.Table(headers, rows)
//
// # Interactive Prompts
//
// The Confirm function prompts for user confirmation:
//
//	if cliout.Confirm("Do you want to continue?") {
//	    // User confirmed
//	}
//
// In JSON mode, Confirm always returns true (non-interactive).
//
// # Progress Indicators
//
// Create simple progress bars:
//
//	bar := cliout.ProgressBar(45, 100, 30)
//	fmt.Println(bar)  // [█████████████░░░░░░░░░░░░░░░░░] 45%
//
// # Color Constants
//
// The package exports ANSI color constants for custom formatting:
//   - Reset, Bold, Dim
//   - Foreground colors: Black, Red, Green, Yellow, Blue, Magenta, Cyan, White, Gray
//   - Bright colors: BrightRed, BrightGreen, BrightYellow, BrightBlue, BrightMagenta, BrightCyan
//
// # Unicode Symbols
//
// Unicode symbols with ASCII fallbacks:
//   - SymbolCheck (✓) / ASCIICheck ([+])
//   - SymbolCross (✗) / ASCIICross ([-])
//   - SymbolWarning (⚠) / ASCIIWarning ([!])
//   - SymbolInfo (ℹ) / ASCIIInfo ([i])
//   - SymbolArrow (→) / ASCIIArrow (->)
//   - SymbolDot (•) / ASCIIDot (*)
//
// # Design Principles
//
//   - No global state except format and orchestration settings
//   - All output goes to stdout (use stderr wrapper if needed)
//   - Consistent color scheme across all azd extensions
//   - Graceful degradation on legacy terminals
//   - JSON mode for automation and scripting scenarios
package cliout
