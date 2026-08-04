// Package cliout provides structured output formatting for CLI commands.
// It supports multiple output formats including human-readable text and JSON,
// with consistent styling using ANSI colors and Unicode symbols.
package cliout

import (
	"fmt"
	"os"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/azure/azure-dev/cli/azd/pkg/azdext"
)

// Format represents the output format.
type Format string

const (
	// FormatDefault is the default human-readable format.
	FormatDefault Format = "default"
	// FormatJSON is JSON format.
	FormatJSON Format = "json"
)

const (
	statusSuccess = "success"
	statusRunning = "running"
)

// ANSI color codes for consistent styling
const (
	Reset = "\033[0m"
	Bold  = "\033[1m"
	Dim   = "\033[2m"

	// Foreground colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"
	Gray    = "\033[90m"

	// Bright foreground colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
)

// Unicode symbols for modern CLI output
const (
	SymbolCheck   = "✓"
	SymbolCross   = "✗"
	SymbolWarning = "⚠"
	SymbolInfo    = "ℹ"
	SymbolArrow   = "→"
	SymbolDot     = "•"
	SymbolSpinner = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏" // Spinner frames
)

// ASCII fallback symbols for terminals that don't support Unicode
const (
	ASCIICheck   = "[+]"
	ASCIICross   = "[-]"
	ASCIIWarning = "[!]"
	ASCIIInfo    = "[i]"
	ASCIIArrow   = "->"
	ASCIIDot     = "*"
)

// Emoji icons with ASCII fallbacks
var (
	IconSearch  = "🔍"
	IconTool    = "🔧"
	IconRefresh = "🔄"
	IconPackage = "📦"
	IconPython  = "🐍"
	IconDotnet  = "🔷"
	IconDocker  = "🐳"
	IconCheck   = "📋"
	IconBulb    = "💡"
	IconRocket  = "🚀"
	IconWarning = "⚠️"
	IconError   = "❌"
	IconInfo    = "ℹ️"
	IconFolder  = "📁"
)

// Global output format setting
var globalFormat Format = FormatDefault

// noColor disables all color output. It is seeded from the azdext interactive
// probe so that ANSI escapes are suppressed when stdout is redirected, when
// NO_COLOR is set, and when running under CI or an AI agent. FORCE_COLOR=1
// overrides the terminal check. ForceColor and NoColor override the probe.
var noColor = !azdext.DetectInteractive().CanColorize()

// orchestratedMode indicates if running as part of command orchestration
var orchestratedMode = false

// mu protects global state variables
var mu sync.RWMutex

// ForceColor enables color output regardless of terminal detection.
func ForceColor() {
	mu.Lock()
	noColor = false
	mu.Unlock()
}

// NoColor disables color output.
func NoColor() {
	mu.Lock()
	noColor = true
	mu.Unlock()
}

// SetOrchestrated sets the orchestration mode flag.
// When true, subcommands skip their headers.
func SetOrchestrated(value bool) {
	mu.Lock()
	orchestratedMode = value
	mu.Unlock()
}

// IsOrchestrated returns true if running in orchestrated mode.
func IsOrchestrated() bool {
	mu.RLock()
	defer mu.RUnlock()
	return orchestratedMode
}

// getNoColor returns the current noColor setting (thread-safe).
func getNoColor() bool {
	mu.RLock()
	defer mu.RUnlock()
	return noColor
}

// ansiPattern matches ANSI SGR escape sequences emitted by the color constants
// in this package.
var ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")

// styled returns s unchanged when color is enabled and strips every ANSI escape
// sequence from it when color is disabled. Every helper in this package that
// prints or returns decorated text routes through it, which is what makes
// NoColor and the non-TTY detection actually take effect.
func styled(s string) string {
	if !getNoColor() {
		return s
	}
	return ansiPattern.ReplaceAllString(s, "")
}

// outf is the package replacement for fmt.Printf. It applies color gating.
func outf(format string, args ...any) {
	fmt.Print(styled(fmt.Sprintf(format, args...)))
}

// outln is the package replacement for fmt.Println. It applies color gating.
func outln(args ...any) {
	fmt.Print(styled(fmt.Sprintln(args...)))
}

// supportsUnicode detects if the terminal supports Unicode/emojis
var supportsUnicode = detectUnicodeSupport()

// detectUnicodeSupport checks if the terminal can display Unicode properly
func detectUnicodeSupport() bool {
	// Check Windows version and console
	if runtime.GOOS == "windows" {
		// Windows Terminal, VS Code terminal, and modern PowerShell support Unicode
		term := os.Getenv("TERM_PROGRAM")
		wtSession := os.Getenv("WT_SESSION")

		// Check for Windows Terminal
		if wtSession != "" {
			return true
		}

		// Check for VS Code
		if term == "vscode" {
			return true
		}

		// Check for ConEmu
		if os.Getenv("ConEmuPID") != "" {
			return true
		}

		// PowerShell (any version) generally supports Unicode emojis
		// Check if running in PowerShell
		if os.Getenv("PSModulePath") != "" || os.Getenv("POWERSHELL_DISTRIBUTION_CHANNEL") != "" {
			return true
		}

		// Check TERM environment variable
		if os.Getenv("TERM") != "" {
			return true
		}

		// Default to ASCII for old Windows Console/CMD
		return false
	}

	// Unix-like systems generally support Unicode
	return true
}

// getIcon returns the appropriate icon based on Unicode support
func getIcon(unicode, ascii string) string {
	if supportsUnicode {
		return unicode
	}
	return ascii
}

// SetFormat sets the global output format.
func SetFormat(format string) error {
	mu.Lock()
	defer mu.Unlock()
	switch format {
	case "default", "":
		globalFormat = FormatDefault
	case "json":
		globalFormat = FormatJSON
	default:
		return fmt.Errorf("invalid output format: %s (valid options: default, json)", format)
	}
	return nil
}

// GetFormat returns the current output format.
func GetFormat() Format {
	mu.RLock()
	defer mu.RUnlock()
	return globalFormat
}

// IsJSON returns true if the output format is JSON.
func IsJSON() bool {
	mu.RLock()
	defer mu.RUnlock()
	return globalFormat == FormatJSON
}

// PrintJSON prints data as JSON to stdout.
func PrintJSON(data any) error {
	return newAzdextOutput(FormatJSON).JSON(data)
}

// newAzdextOutput builds an azdext.Output bound to the process stdout/stderr
// for the given cliout format. The SDK type captures its writers at
// construction, so it is built per call rather than cached, which keeps it
// correct when tests swap os.Stdout.
func newAzdextOutput(f Format) *azdext.Output {
	format := azdext.OutputFormatDefault
	if f == FormatJSON {
		format = azdext.OutputFormatJSON
	}
	return azdext.NewOutput(azdext.OutputOptions{
		Format:    format,
		Writer:    os.Stdout,
		ErrWriter: os.Stderr,
	})
}

// PrintDefault prints data in default format using a custom formatter function.
func PrintDefault(formatter func()) {
	if GetFormat() == FormatDefault {
		formatter()
	}
}

// Print outputs data in the configured format.
// For default format, uses the formatter function.
// For JSON format, marshals the data object.
func Print(data any, formatter func()) error {
	if GetFormat() == FormatJSON {
		return PrintJSON(data)
	}
	formatter()
	return nil
}

// Modern CLI output functions with consistent styling

// Header prints a bold header with a divider
func Header(text string) {
	outf("\n%s%s%s\n", Bold, text, Reset)
	outln(strings.Repeat("=", len(text)))
}

// CommandHeader prints a minimal command header.
// Shows just the command name with a short divider.
// Skipped when in orchestrated mode (subcommands don't print headers).
func CommandHeader(command, _ string) {
	if IsJSON() || IsOrchestrated() {
		return
	}
	outln()
	outf("%sazd app %s%s\n", Bold, command, Reset)
	outln(strings.Repeat("─", 30))
	outln()
}

// Section prints a section header
func Section(icon, text string) {
	displayIcon := getIcon(icon, "[>]")
	outf("\n%s%s %s%s\n", Cyan, displayIcon, text, Reset)
}

// Success prints a success message with green checkmark
func Success(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	check := getIcon(SymbolCheck, ASCIICheck)
	outf("%s%s%s %s\n", BrightGreen, check, Reset, msg)
}

// Error prints an error message with red X
func Error(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	cross := getIcon(SymbolCross, ASCIICross)
	outf("%s%s%s %s\n", BrightRed, cross, Reset, msg)
}

// Warning prints a warning message with yellow triangle
func Warning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	warning := getIcon(SymbolWarning, ASCIIWarning)
	outf("%s%s%s  %s\n", BrightYellow, warning, Reset, msg)
}

// Info prints an info message with blue info icon
func Info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	info := getIcon(SymbolInfo, ASCIIInfo)
	outf("%s%s%s  %s\n", BrightBlue, info, Reset, msg)
}

// Step prints a step message with an icon
func Step(icon, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	displayIcon := getIcon(icon, "[*]")
	outf("%s%s%s %s\n", Cyan, displayIcon, Reset, msg)
}

// Item prints an indented item
func Item(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	outf("   %s\n", msg)
}

// Bullet prints a bulleted list item
func Bullet(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	bullet := getIcon(SymbolDot, "*")
	outf("  %s %s\n", bullet, msg)
}

// ItemSuccess prints an indented success item
func ItemSuccess(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	check := getIcon(SymbolCheck, ASCIICheck)
	outf("   %s%s%s %s\n", Green, check, Reset, msg)
}

// ItemError prints an indented error item
func ItemError(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	cross := getIcon(SymbolCross, ASCIICross)
	outf("   %s%s%s %s\n", Red, cross, Reset, msg)
}

// ItemWarning prints an indented warning item
func ItemWarning(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	warning := getIcon(SymbolWarning, ASCIIWarning)
	outf("   %s%s%s  %s\n", Yellow, warning, Reset, msg)
}

// ItemInfo prints an indented info item
func ItemInfo(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	info := getIcon(SymbolInfo, ASCIIInfo)
	outf("   %s%s%s  %s\n", Cyan, info, Reset, msg)
}

// Divider prints a horizontal divider
func Divider() {
	outf("\n%s%s%s\n", Dim, strings.Repeat("─", 50), Reset)
}

// Newline prints a blank line
func Newline() {
	outln()
}

// Hint prints compact hints on a single line with bullet separators.
// Example: Hint("Press Ctrl+C to stop", "Use --web to open browser")
func Hint(hints ...string) {
	if len(hints) == 0 {
		return
	}
	outf("%s%s%s\n", Dim, strings.Join(hints, " • "), Reset)
}

// Phase prints a phase label like "Installing dependencies..." or "Starting services..."
func Phase(label string) {
	outf("%s%s%s\n", Dim, label, Reset)
}

// Plain prints plain text without any formatting.
func Plain(format string, args ...any) {
	outf(format+"\n", args...)
}

// Confirm prompts the user for confirmation and returns true if they confirm.
// Returns true immediately if in JSON mode (non-interactive).
// The prompt displays the message and waits for y/n input.
func Confirm(message string) bool {
	if IsJSON() {
		return true // Non-interactive mode, assume yes
	}
	// Never block on stdin when there is nobody to answer. DetectInteractive
	// covers a redirected stdin or stdout, AZD_NO_PROMPT, CI runners, and AI
	// agent hosts. Declining is the safe default for an unanswered prompt.
	if !azdext.DetectInteractive().CanPrompt() {
		return false
	}
	outf("%s%s%s [y/N]: ", BrightYellow, message, Reset)
	var response string
	if _, err := fmt.Scanln(&response); err != nil {
		return false // On read error, default to no
	}
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// Label prints a label and value pair
func Label(label, value string) {
	outf("   %s%-12s%s %s\n", Dim, label+":", Reset, value)
}

// LabelColored prints a label and colored value pair
func LabelColored(label, value, color string) {
	outf("   %s%-12s%s %s%s%s\n", Dim, label+":", Reset, color, value, Reset)
}

// Highlight prints highlighted text
func Highlight(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return Bold + Cyan + msg + Reset
}

// Emphasize prints emphasized text
func Emphasize(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return Bold + msg + Reset
}

// Muted prints muted/dim text
func Muted(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	return Dim + msg + Reset
}

// URL prints a URL in bright blue
func URL(url string) string {
	return BrightBlue + url + Reset
}

// Count prints a count badge
func Count(n int) string {
	return Bold + fmt.Sprintf("%d", n) + Reset
}

// Status prints a status badge with appropriate color
func Status(status string) string {
	switch strings.ToLower(status) {
	case statusSuccess, "ok", statusRunning, "healthy":
		return BrightGreen + status + Reset
	case "warning", "pending", "starting":
		return BrightYellow + status + Reset
	case "error", "failed", "unhealthy":
		return BrightRed + status + Reset
	case "info", "unknown":
		return BrightBlue + status + Reset
	default:
		return status
	}
}

// ProgressBar prints a simple progress bar
func ProgressBar(current, total int, width int) string {
	if total == 0 {
		return ""
	}
	percent := float64(current) / float64(total)
	filled := int(percent * float64(width))
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	return fmt.Sprintf("[%s] %d%%", bar, int(percent*100))
}

// TableRow represents a row in a table as a map of column header to value.
type TableRow map[string]string

// Table prints a simple table with the given headers and rows.
func Table(headers []string, rows []TableRow) {
	if len(rows) == 0 {
		return
	}

	// Flatten the keyed rows into the positional shape the SDK renderer wants,
	// preserving header order and substituting an empty cell for absent keys.
	cells := make([][]string, 0, len(rows))
	for _, row := range rows {
		cell := make([]string, len(headers))
		for i, header := range headers {
			cell[i] = row[header]
		}
		cells = append(cells, cell)
	}

	newAzdextOutput(GetFormat()).Table(headers, cells)
}
