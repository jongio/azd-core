# Extension Development Patterns

## Overview

This guide documents proven patterns for building azd extensions, extracted from real-world implementations in [azd-exec](https://github.com/jongio/azd-exec) and [azd-app](https://github.com/jongio/azd-app). These patterns ensure consistency, maintainability, and professional user experience across the azd ecosystem.

**Why patterns matter:**
- **Consistency**: Users get the same experience across all azd extensions
- **Quality**: Proven solutions to common problems (Unicode detection, testing, error handling)
- **Speed**: Start building features immediately without reinventing infrastructure
- **Maintenance**: Standardized code is easier to debug and evolve

---

## Version Management Pattern

### The Pattern

All azd extensions should manage version information using build-time ldflags injection. This allows version metadata to be set during the build process without modifying source code.

### Implementation

**File: `internal/version/version.go`**
```go
// Package version provides version information for the azd extension.
// Version information is set at build time via ldflags.
package version

// Version is the current version of the extension.
// It follows semantic versioning (e.g., "1.0.0").
// Set at build time via:
//   go build -ldflags "-X path/to/version.Version=1.0.0"
var Version = "0.0.0-dev"

// BuildDate is the UTC timestamp of the build.
// Set at build time via:
//   go build -ldflags "-X path/to/version.BuildDate=2026-01-10T12:00:00Z"
var BuildDate = "unknown"

// GitCommit is the git SHA used for the build.
// Set at build time via:
//   go build -ldflags "-X path/to/version.GitCommit=abc123def456"
var GitCommit = "unknown"

// ExtensionID is the unique identifier for this extension.
// This ID is used by the azd extension registry and must match extension.yaml.
const ExtensionID = "your.extension.id"

// Name is the human-readable name of the extension.
const Name = "Your Extension"
```

### Build Script Pattern

**File: `build.ps1` (PowerShell build script)**
```powershell
# Get version from extension.yaml
$yamlContent = Get-Content "extension.yaml" -Raw
if ($yamlContent -match 'version:\s*(\S+)') {
    $VERSION = $matches[1]
} else {
    $VERSION = "0.0.0-dev"
}

# Get git metadata
$COMMIT = git rev-parse HEAD 2>$null
if ($LASTEXITCODE -ne 0) { $COMMIT = "unknown" }
$BUILD_DATE = (Get-Date -Format "yyyy-MM-ddTHH:mm:ssZ")

# Build with ldflags
$APP_PATH = "github.com/yourorg/yourext/cli/src/internal/version"
$ldflags = "-s -w -X '$APP_PATH.Version=$VERSION' -X '$APP_PATH.BuildDate=$BUILD_DATE' -X '$APP_PATH.GitCommit=$COMMIT'"

go build -ldflags="$ldflags" -o bin/yourext.exe ./src/cmd/yourext
```

### Why This Pattern?

✅ **Single Source of Truth**: Version comes from `extension.yaml`, not hardcoded  
✅ **Build Metadata**: Capture git commit and build time automatically  
✅ **Developer Experience**: Default `0.0.0-dev` clearly indicates local builds  
✅ **Registry Integration**: ExtensionID matches extension.yaml for proper registration

### Common Pitfalls

❌ **Hardcoding versions** in source code (requires code changes for releases)  
❌ **Forgetting `-X` prefix** in ldflags (won't set variables)  
❌ **Wrong package path** in ldflags (variables remain at default values)  
❌ **Missing quotes** around ldflags with spaces (build fails)

### Usage in Version Command

```go
package commands

import (
    "fmt"
    "github.com/yourorg/yourext/cli/src/internal/version"
    "github.com/jongio/azd-core/cliout"
)

func versionCommand() error {
    // Support JSON output
    if cliout.IsJSON() {
        data := map[string]string{
            "version":   version.Version,
            "commit":    version.GitCommit,
            "buildDate": version.BuildDate,
        }
        return cliout.PrintJSON(data)
    }

    // Human-readable output
    cliout.Header(version.Name)
    cliout.Label("Version", version.Version)
    cliout.Label("Commit", version.GitCommit)
    cliout.Label("Build Date", version.BuildDate)
    return nil
}
```

**References:**
- azd-exec: [internal/version/version.go](https://github.com/jongio/azd-exec/blob/main/cli/src/internal/version/version.go)
- azd-exec: [build.ps1](https://github.com/jongio/azd-exec/blob/main/cli/build.ps1)

---

## Logging Pattern

### The Pattern

Use structured logging built on Go's standard `log/slog` (Go 1.21+) with component-based context propagation. This enables filtering by component, service, or operation in production debugging.

### Implementation

**File: `internal/logging/logger.go`**
```go
package logging

import (
    "io"
    "log/slog"
    "os"
)

type Level int

const (
    LevelDebug Level = iota
    LevelInfo
    LevelWarn
    LevelError
)

var (
    globalLogger *slog.Logger
    currentLevel = LevelInfo
    isStructured = false
)

// Logger provides component-scoped logging with context propagation.
type Logger struct {
    slogger   *slog.Logger
    component string
}

// NewLogger creates a logger with a component context.
func NewLogger(component string) *Logger {
    return &Logger{
        slogger:   globalLogger.With("component", component),
        component: component,
    }
}

// WithService returns a new logger with service context.
func (l *Logger) WithService(service string) *Logger {
    return &Logger{
        slogger:   l.slogger.With("service", service),
        component: l.component,
    }
}

// WithOperation returns a new logger with operation context.
func (l *Logger) WithOperation(operation string) *Logger {
    return &Logger{
        slogger:   l.slogger.With("operation", operation),
        component: l.component,
    }
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...any) {
    if IsDebugEnabled() {
        l.slogger.Debug(msg, args...)
    }
}

// Info logs an info message.
func (l *Logger) Info(msg string, args ...any) {
    l.slogger.Info(msg, args...)
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...any) {
    l.slogger.Warn(msg, args...)
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...any) {
    l.slogger.Error(msg, args...)
}

// SetupLogger configures the global logger.
func SetupLogger(debug, structured bool) {
    var level slog.Level
    if debug {
        level = slog.LevelDebug
        currentLevel = LevelDebug
    } else {
        level = slog.LevelInfo
        currentLevel = LevelInfo
    }

    isStructured = structured
    opts := &slog.HandlerOptions{Level: level}

    var handler slog.Handler
    if structured {
        handler = slog.NewJSONHandler(os.Stderr, opts)
    } else {
        handler = slog.NewTextHandler(os.Stderr, opts)
    }

    globalLogger = slog.New(handler)
    slog.SetDefault(globalLogger)
}

// IsDebugEnabled returns true if debug logging is enabled.
func IsDebugEnabled() bool {
    return currentLevel == LevelDebug || os.Getenv("DEBUG") == "true"
}
```

### Usage Pattern

```go
package executor

import "yourext/internal/logging"

type Executor struct {
    logger *logging.Logger
}

func NewExecutor() *Executor {
    return &Executor{
        logger: logging.NewLogger("executor"),
    }
}

func (e *Executor) RunScript(service, script string) error {
    // Add service context for filtering
    logger := e.logger.WithService(service)
    
    logger.Info("starting script execution",
        "script", script,
        "cwd", e.workingDir,
    )
    
    // Add operation context
    opLogger := logger.WithOperation("keyvault-resolve")
    opLogger.Debug("resolving Key Vault references",
        "count", len(refs),
    )
    
    return nil
}
```

### Why This Pattern?

✅ **Standard Library**: No external logging dependencies (Go 1.21+)  
✅ **Structured Output**: JSON mode for log aggregation systems  
✅ **Context Propagation**: Filter logs by component/service/operation  
✅ **Debug Control**: Enable via environment variable or flag

### When to Log vs Display

| Use Case | Tool | Reason |
|----------|------|--------|
| **User-facing status** | `cliout.Success/Info/Warning/Error` | Colored, formatted for humans |
| **Debug information** | `logger.Debug()` | Hidden unless debug mode enabled |
| **Operation tracking** | `logger.Info()` | Audit trail for troubleshooting |
| **Error details** | `logger.Error()` + `cliout.Error()` | Log full context, show summary to user |

### Common Pitfalls

❌ **Logging to stdout** (conflicts with CLI output)  
❌ **Using `fmt.Println` for debugging** (can't be disabled)  
❌ **Logging sensitive data** (passwords, tokens, secrets)  
❌ **Too much Info logging** (noise in production logs)

**References:**
- azd-app: [internal/logging/logger.go](https://github.com/jongio/azd-app/blob/main/cli/src/internal/logging/logger.go)
- Pattern: Log internal operations, display user-facing messages

---

## Extension Structure Pattern

### Recommended Directory Layout

```
your-extension/
├── cli/                          # Extension CLI code
│   ├── bin/                      # Compiled binaries (gitignored)
│   ├── src/
│   │   ├── cmd/
│   │   │   └── yourext/          # Main package
│   │   │       ├── main.go       # Entry point
│   │   │       └── commands/     # Command implementations
│   │   │           ├── root.go
│   │   │           ├── version.go
│   │   │           └── ...
│   │   └── internal/             # Internal packages (not importable)
│   │       ├── version/          # Version management
│   │       ├── logging/          # Logging setup (optional)
│   │       ├── executor/         # Core business logic
│   │       └── ...
│   ├── tests/                    # Integration test fixtures
│   │   └── projects/
│   ├── build.ps1                 # Build script (PowerShell)
│   ├── build.sh                  # Build script (Bash)
│   ├── extension.yaml            # Extension metadata
│   └── go.mod
├── docs/                         # Documentation
│   ├── specs/                    # Design specs
│   └── cli-reference.md
├── .github/
│   └── workflows/
│       ├── ci.yml                # CI pipeline
│       └── release.yml           # Release automation
├── README.md
├── LICENSE
└── CONTRIBUTING.md
```

### Package Organization Principles

**1. Use `internal/` for Extension-Specific Code**

Code in `internal/` cannot be imported by other projects. Use this for:
- Extension business logic
- Internal utilities specific to your extension
- Version management (not reusable across extensions)

**2. Use `cmd/` for Command Entry Points**

- `cmd/yourext/main.go` - Entry point, minimal logic
- `cmd/yourext/commands/` - Command implementations using Cobra or similar

**3. Use `github.com/jongio/azd-core` for Shared Utilities**

Import from azd-core instead of duplicating:
- `azd-core/testutil` - Test helpers
- `azd-core/cliout` - CLI output formatting
- `azd-core/env` - Environment variable resolution
- `azd-core/keyvault` - Key Vault integration
- `azd-core/shellutil` - Shell detection
- And more...

### Example main.go Structure

```go
package main

import (
    "os"
    "yourext/src/cmd/yourext/commands"
    "yourext/src/internal/logging"
    "github.com/jongio/azd-core/cliout"
)

func main() {
    // Initialize logging
    debug := os.Getenv("DEBUG") == "true"
    logging.SetupLogger(debug, false)

    // Initialize CLI output
    format := os.Getenv("OUTPUT_FORMAT")
    if format == "" {
        format = "default"
    }
    if err := cliout.SetFormat(format); err != nil {
        cliout.Error("invalid output format: %s", err)
        os.Exit(1)
    }

    // Execute root command
    if err := commands.Execute(); err != nil {
        cliout.Error("command failed: %s", err)
        os.Exit(1)
    }
}
```

### Why This Structure?

✅ **Clear Boundaries**: `internal/` for private, azd-core for shared  
✅ **Importability**: Only `cmd/` and public packages can be imported  
✅ **Discoverability**: Consistent structure across extensions  
✅ **Modularity**: Easy to extract reusable components to azd-core later

**References:**
- azd-exec: [cli/src structure](https://github.com/jongio/azd-exec/tree/main/cli/src)
- azd-app: [cli/src structure](https://github.com/jongio/azd-app/tree/main/cli/src)

---

## Testing Patterns

### Using testutil for Reliable Tests

azd-core provides `testutil` with battle-tested helpers for common testing scenarios.

### Pattern 1: Capturing CLI Output

**Use Case**: Test commands that print to stdout

```go
package commands

import (
    "testing"
    "github.com/jongio/azd-core/testutil"
)

func TestVersionCommand(t *testing.T) {
    // Capture output from command execution
    output := testutil.CaptureOutput(t, func() error {
        return versionCommand()
    })

    // Assert on captured output
    if !testutil.Contains(output, "Version:") {
        t.Errorf("expected version output, got: %s", output)
    }
    if !testutil.Contains(output, "1.0.0") {
        t.Errorf("expected version 1.0.0 in output")
    }
}
```

**Why `CaptureOutput`?**
- ✅ Properly restores stdout after test (even if panic/error)
- ✅ Uses goroutines to avoid blocking on pipe reads
- ✅ Marks test helper with `t.Helper()` for accurate failure line numbers
- ✅ Thread-safe for parallel tests

### Pattern 2: Locating Test Fixtures

**Use Case**: Find test data directories regardless of where test runs from

```go
func TestScriptExecution(t *testing.T) {
    // FindTestData searches up the directory tree
    projectsDir := testutil.FindTestData(t, "tests", "projects")
    
    scriptPath := filepath.Join(projectsDir, "simple", "hello.sh")
    
    // Run test against fixture
    result, err := executor.Run(scriptPath)
    if err != nil {
        t.Fatalf("execution failed: %v", err)
    }
    
    if !testutil.Contains(result.Output, "Hello") {
        t.Errorf("expected greeting in output")
    }
}
```

**Why `FindTestData`?**
- ✅ Works regardless of `go test` working directory
- ✅ Searches up to 5 levels for flexibility
- ✅ Clear error messages when fixtures not found
- ✅ Cross-platform path handling (Windows/Unix)

### Pattern 3: Temporary Directories

**Use Case**: Create isolated test environments

```go
func TestConfigWrite(t *testing.T) {
    // Create temp directory with automatic cleanup
    tmpDir := testutil.TempDir(t)
    
    configPath := filepath.Join(tmpDir, "config.json")
    
    // Write test file
    err := writeConfig(configPath, testConfig)
    if err != nil {
        t.Fatalf("failed to write config: %v", err)
    }
    
    // Read back and verify
    config, err := readConfig(configPath)
    if err != nil {
        t.Fatalf("failed to read config: %v", err)
    }
    
    // tmpDir automatically cleaned up via t.Cleanup()
}
```

**Why `TempDir`?**
- ✅ Automatic cleanup via `t.Cleanup()` (even if test fails)
- ✅ Unique directory per test (no conflicts in parallel tests)
- ✅ Proper permissions (0755 on Unix)
- ✅ No manual cleanup code needed

### Pattern 4: Table-Driven Tests

Combine testutil with table-driven tests for comprehensive coverage:

```go
func TestShellDetection(t *testing.T) {
    tests := []struct {
        name     string
        filename string
        expected string
    }{
        {"PowerShell script", "deploy.ps1", "pwsh"},
        {"Bash script", "build.sh", "bash"},
        {"Batch file", "setup.cmd", "cmd"},
        {"Python script", "app.py", "python"},
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            shell := detectShell(tt.filename)
            if shell != tt.expected {
                t.Errorf("expected %s, got %s", tt.expected, shell)
            }
        })
    }
}
```

### Common Testing Pitfalls

❌ **Not using `t.Helper()`** in test helpers (wrong failure line numbers)  
❌ **Forgetting to restore stdout** after capture (breaks subsequent tests)  
❌ **Hardcoded fixture paths** (breaks when running from different directories)  
❌ **Manual temp directory cleanup** (leaks if test panics)  
❌ **Testing stdout and stderr simultaneously** (can deadlock without goroutines)

**References:**
- azd-core/testutil: [testutil.go](https://github.com/jongio/azd-core/blob/main/testutil/testutil.go)
- azd-exec tests: [executor_test.go](https://github.com/jongio/azd-exec/blob/main/cli/src/internal/executor/executor_coverage_test.go)
- azd-app tests: [logs_test.go](https://github.com/jongio/azd-app/blob/main/cli/src/dashboard/commands/logs_test.go)

---

## CLI Output Patterns

### Using cliout for Consistent UX

azd-core provides `cliout` for professional, consistent CLI output across all azd extensions.

### Pattern 1: Status Messages

```go
package commands

import "github.com/jongio/azd-core/cliout"

func deployCommand() error {
    cliout.Info("Starting deployment...")
    
    // Perform deployment
    if err := deploy(); err != nil {
        cliout.Error("Deployment failed: %s", err)
        return err
    }
    
    cliout.Success("Deployment completed successfully!")
    cliout.Info("Application available at: https://myapp.azurewebsites.net")
    
    return nil
}

func validateConfig(config *Config) error {
    if config.Port < 1024 {
        cliout.Warning("Port %d requires elevated privileges", config.Port)
    }
    
    if config.Timeout > 300 {
        cliout.Warning("Timeout of %d seconds is very high", config.Timeout)
    }
    
    return nil
}
```

**Output Colors** (automatically disabled on unsupported terminals):
- `Success` → Green with ✓ icon
- `Error` → Red with ✗ icon
- `Warning` → Yellow with ⚠ icon
- `Info` → Blue with ℹ icon

### Pattern 2: Sections and Headers

```go
func statusCommand() error {
    cliout.Header("Application Status")
    
    cliout.Section("🌐", "Services")
    cliout.Bullet("web: running on port 8080")
    cliout.Bullet("api: running on port 3000")
    cliout.Bullet("db: connected")
    
    cliout.Section("📊", "Metrics")
    cliout.Label("CPU Usage", "45%")
    cliout.Label("Memory", "2.3 GB / 4 GB")
    cliout.Label("Uptime", "3 days 5 hours")
    
    return nil
}
```

### Pattern 3: Tables

```go
func listCommand() error {
    headers := []string{"Name", "Status", "Port", "Uptime"}
    rows := []cliout.TableRow{
        {"Name": "web", "Status": "running", "Port": "8080", "Uptime": "3d 5h"},
        {"Name": "api", "Status": "running", "Port": "3000", "Uptime": "3d 5h"},
        {"Name": "worker", "Status": "stopped", "Port": "-", "Uptime": "-"},
    }
    
    cliout.Table(headers, rows)
    return nil
}
```

**Output:**
```
┌────────┬─────────┬──────┬────────┐
│ Name   │ Status  │ Port │ Uptime │
├────────┼─────────┼──────┼────────┤
│ web    │ running │ 8080 │ 3d 5h  │
│ api    │ running │ 3000 │ 3d 5h  │
│ worker │ stopped │ -    │ -      │
└────────┴─────────┴──────┴────────┘
```

### Pattern 4: JSON Output Mode

Support automation and scripting with `--output json`:

```go
func statusCommand() error {
    data := map[string]interface{}{
        "services": []map[string]string{
            {"name": "web", "status": "running", "port": "8080"},
            {"name": "api", "status": "running", "port": "3000"},
        },
        "metrics": map[string]string{
            "cpu":    "45%",
            "memory": "2.3 GB / 4 GB",
            "uptime": "3 days 5 hours",
        },
    }
    
    // Hybrid output: JSON if --output json, formatted otherwise
    return cliout.Print(data, func() {
        cliout.Header("Application Status")
        cliout.Section("🌐", "Services")
        cliout.Bullet("web: running on port 8080")
        cliout.Bullet("api: running on port 3000")
        // ... formatted output ...
    })
}
```

**Usage:**
```bash
# Human-readable
$ yourext status

# Machine-readable
$ yourext status --output json
{"services":[{"name":"web","status":"running","port":"8080"}],...}
```

### Pattern 5: Interactive Prompts

```go
func deleteCommand() error {
    if !cliout.Confirm("Are you sure you want to delete all data?") {
        cliout.Info("Operation cancelled")
        return nil
    }
    
    cliout.Info("Deleting data...")
    // Perform deletion
    cliout.Success("Data deleted successfully")
    return nil
}
```

**Behavior:**
- **Default mode**: Prompts user for y/n input
- **JSON mode**: Always returns `true` (non-interactive)
- **CI/CD**: Set `OUTPUT_FORMAT=json` to skip prompts

### Pattern 6: Orchestration Mode

When composing multiple commands in a workflow, suppress duplicate headers:

```go
func workflowCommand() error {
    // Run subcommands without duplicate headers
    cliout.SetOrchestrated(true)
    defer cliout.SetOrchestrated(false)
    
    if err := buildCommand(); err != nil {
        return err
    }
    if err := testCommand(); err != nil {
        return err
    }
    if err := deployCommand(); err != nil {
        return err
    }
    
    return nil
}
```

### Unicode Detection

cliout automatically detects terminal capabilities:

**Supported Terminals** (Unicode symbols):
- Windows Terminal
- VS Code integrated terminal
- PowerShell 7+
- ConEmu
- iTerm2, Terminal.app (macOS)
- Most Linux terminals

**Legacy Terminals** (ASCII fallback):
- Old Windows cmd.exe (pre-Windows 10)
- Terminals without UTF-8 support

**Detection:** Checks `WT_SESSION`, `TERM_PROGRAM`, `ConEmuPID` environment variables

### Common CLI Output Pitfalls

❌ **Using `fmt.Printf` directly** (no color, no JSON mode)  
❌ **Hardcoding ANSI escape codes** (breaks on legacy terminals)  
❌ **Forgetting JSON mode** (breaks automation/scripting)  
❌ **Interactive prompts in CI/CD** (hangs pipelines)  
❌ **Logging to stdout** (conflicts with CLI output, breaks JSON parsing)

**References:**
- azd-core/cliout: [cliout.go](https://github.com/jongio/azd-core/blob/main/cliout/cliout.go)
- azd-exec version command: [version.go](https://github.com/jongio/azd-exec/blob/main/cli/src/cmd/exec/commands/version.go)
- azd-app commands: [dashboard/commands/](https://github.com/jongio/azd-app/tree/main/cli/src/dashboard/commands)

---

## Error Handling Pattern

### The Pattern

Standardized error types improve consistency, testability, and user experience. Use typed errors for programmatic handling and clear error messages for users.

### Standard Error Types

**1. Validation Errors**

```go
package executor

import "fmt"

// ValidationError indicates that input validation failed.
type ValidationError struct {
    Field  string
    Reason string
}

func (e *ValidationError) Error() string {
    return fmt.Sprintf("validation error for %s: %s", e.Field, e.Reason)
}

// Usage
func validateConfig(cfg *Config) error {
    if cfg.Port < 0 || cfg.Port > 65535 {
        return &ValidationError{
            Field:  "port",
            Reason: "must be between 0 and 65535",
        }
    }
    return nil
}
```

**2. Not Found Errors**

```go
// ScriptNotFoundError indicates a script file was not found.
type ScriptNotFoundError struct {
    Path string
}

func (e *ScriptNotFoundError) Error() string {
    return fmt.Sprintf("script not found: %s", e.Path)
}

// Usage
func loadScript(path string) error {
    if !fileExists(path) {
        return &ScriptNotFoundError{Path: path}
    }
    return nil
}
```

**3. Execution Errors**

```go
// ExecutionError indicates a command failed with an exit code.
type ExecutionError struct {
    Command  string
    ExitCode int
    Stderr   string
}

func (e *ExecutionError) Error() string {
    msg := fmt.Sprintf("command failed with exit code %d: %s", e.ExitCode, e.Command)
    if e.Stderr != "" {
        msg += fmt.Sprintf("\nstderr: %s", e.Stderr)
    }
    return msg
}

// Usage
func runCommand(cmd string) error {
    exitCode, stderr := execute(cmd)
    if exitCode != 0 {
        return &ExecutionError{
            Command:  cmd,
            ExitCode: exitCode,
            Stderr:   stderr,
        }
    }
    return nil
}
```

### Error Handling Best Practices

**1. Separate Logging from Display**

```go
func deployCommand() error {
    logger := logging.NewLogger("deploy")
    
    err := performDeployment()
    if err != nil {
        // Log full details (stack trace, context, etc.)
        logger.Error("deployment failed",
            "error", err,
            "service", serviceName,
            "environment", env,
            "timestamp", time.Now(),
        )
        
        // Show user-friendly message
        cliout.Error("Deployment failed: %s", err)
        
        // Provide helpful context
        if errors.Is(err, ErrNetworkTimeout) {
            cliout.Info("Check your network connection and try again")
        }
        
        return err
    }
    
    return nil
}
```

**2. Type Assertions for Specific Handling**

```go
func handleError(err error) {
    switch e := err.(type) {
    case *ValidationError:
        cliout.Error("Invalid %s: %s", e.Field, e.Reason)
        cliout.Info("Run 'yourext validate --help' for more information")
        
    case *ScriptNotFoundError:
        cliout.Error("Script not found: %s", e.Path)
        cliout.Info("Make sure the file exists and the path is correct")
        
    case *ExecutionError:
        cliout.Error("Execution failed with exit code %d", e.ExitCode)
        if e.Stderr != "" {
            cliout.Info("Error output:\n%s", e.Stderr)
        }
        
    default:
        cliout.Error("An unexpected error occurred: %s", err)
    }
}
```

**3. Wrapping Errors with Context**

```go
import "fmt"

func loadConfiguration(path string) error {
    data, err := os.ReadFile(path)
    if err != nil {
        return fmt.Errorf("failed to load configuration from %s: %w", path, err)
    }
    
    var config Config
    if err := json.Unmarshal(data, &config); err != nil {
        return fmt.Errorf("failed to parse configuration: %w", err)
    }
    
    if err := validateConfig(&config); err != nil {
        return fmt.Errorf("configuration validation failed: %w", err)
    }
    
    return nil
}

// Error chain provides context:
// "configuration validation failed: validation error for port: must be between 0 and 65535"
```

**4. Error Testing**

```go
func TestValidateConfig(t *testing.T) {
    cfg := &Config{Port: -1}
    
    err := validateConfig(cfg)
    if err == nil {
        t.Fatal("expected validation error, got nil")
    }
    
    var valErr *ValidationError
    if !errors.As(err, &valErr) {
        t.Fatalf("expected ValidationError, got %T", err)
    }
    
    if valErr.Field != "port" {
        t.Errorf("expected field 'port', got %s", valErr.Field)
    }
}
```

### User-Facing Error Messages

**Good Error Messages:**
✅ Specific: "Port 99999 is invalid: must be between 0 and 65535"  
✅ Actionable: "Script not found: deploy.sh. Create it or check the path."  
✅ Contextual: "Deployment failed: network timeout. Check your internet connection."

**Bad Error Messages:**
❌ Vague: "Invalid input"  
❌ Technical: "error: syscall.ENOENT"  
❌ No guidance: "Operation failed"

### Common Error Handling Pitfalls

❌ **Swallowing errors** (`if err != nil { return nil }`)  
❌ **Generic error types** (always returning `fmt.Errorf`)  
❌ **Logging and returning** the same error (duplicates in logs)  
❌ **Exposing stack traces to users** (scary, not helpful)  
❌ **No error type checking** (can't handle specific cases)

**References:**
- azd-exec errors: [errors.go](https://github.com/jongio/azd-exec/blob/main/cli/src/internal/executor/errors.go)
- Go error handling: [Working with Errors in Go 1.13](https://go.dev/blog/go1.13-errors)

---

## Summary

### Quick Reference

| Pattern | azd-core Package | Key Benefit |
|---------|------------------|-------------|
| **Version Management** | Build-time ldflags | Single source of truth |
| **Logging** | `log/slog` (stdlib) | Structured debugging |
| **CLI Output** | `azd-core/cliout` | Brand consistency, JSON mode |
| **Testing** | `azd-core/testutil` | Reliable, maintainable tests |
| **Error Handling** | Typed errors | Better UX, testability |

### Implementation Checklist

**New Extension Checklist:**
- [ ] Create `internal/version/version.go` with build-time variables
- [ ] Add build script (`build.ps1`/`build.sh`) with ldflags injection
- [ ] Use `azd-core/cliout` for all CLI output
- [ ] Support `--output json` for automation
- [ ] Use `azd-core/testutil` for test helpers
- [ ] Create `tests/` directory with fixtures
- [ ] Define typed errors for validation, not found, execution
- [ ] Setup structured logging with `log/slog`
- [ ] Follow `cmd/` and `internal/` package structure
- [ ] Import from `azd-core` instead of duplicating utilities

**Quality Checks:**
- [ ] All commands support JSON output mode
- [ ] Tests use `testutil.CaptureOutput` for CLI testing
- [ ] Errors are typed and user-friendly
- [ ] Version command shows version/commit/build date
- [ ] Debug logging available via environment variable
- [ ] Unicode detection works on Windows/Mac/Linux
- [ ] No hardcoded ANSI escape codes
- [ ] No `fmt.Printf` for user-facing output

---

## Additional Resources

**azd-core Packages:**
- [testutil](https://github.com/jongio/azd-core/tree/main/testutil) - Test utilities
- [cliout](https://github.com/jongio/azd-core/tree/main/cliout) - CLI output formatting
- [env](https://github.com/jongio/azd-core/tree/main/env) - Environment variable resolution
- [keyvault](https://github.com/jongio/azd-core/tree/main/keyvault) - Azure Key Vault integration
- [shellutil](https://github.com/jongio/azd-core/tree/main/shellutil) - Shell detection
- [fileutil](https://github.com/jongio/azd-core/tree/main/fileutil) - File system utilities
- [pathutil](https://github.com/jongio/azd-core/tree/main/pathutil) - PATH management
- [security](https://github.com/jongio/azd-core/tree/main/security) - Security validation

**Example Extensions:**
- [azd-exec](https://github.com/jongio/azd-exec) - Script execution extension
- [azd-app](https://github.com/jongio/azd-app) - Multi-service application management

**Go Resources:**
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Standard Go Project Layout](https://github.com/golang-standards/project-layout)
