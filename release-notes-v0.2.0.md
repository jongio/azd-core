# azd-core v0.2.0 - Test Infrastructure and CLI Output Release

## Overview

This release introduces **2 essential packages** that transform how azd extensions are built and tested: **testutil** for reliable test infrastructure and **cliout** for professional CLI output formatting. These packages complete the core utilities extraction from azd-exec and azd-app, establishing comprehensive standardization across the azd ecosystem.

This release (v0.2.0) focuses on two critical developer experience areas:
- **Testing Excellence**: Standardized test helpers enable consistent, reliable CLI testing
- **CLI Consistency**: Unified output formatting ensures professional user experience

Combined with the new **Extension Patterns Guide**, this release provides everything needed to build production-quality azd extensions from day one.

## What's New

### Testing Utilities (testutil package)

**Coverage**: 83.3% (38 test cases)

The `testutil` package provides essential testing utilities extracted from azd-exec and enhanced for universal applicability. These helpers solve common CLI testing challenges with reliable, well-tested implementations.

**Key Functions:**

#### `CaptureOutput` - CLI Command Testing
Capture stdout during function execution for testing CLI commands. Uses goroutine-based reading for reliable capture even when functions panic or exit early.

```go
func TestVersionCommand(t *testing.T) {
    output := testutil.CaptureOutput(t, func() error {
        return versionCmd.Execute()
    })
    
    if !testutil.Contains(output, "version 1.0.0") {
        t.Errorf("expected version in output, got: %s", output)
    }
}
```

#### `FindTestData` - Test Fixture Location
Locate test fixture directories with flexible path searching. Automatically searches upward from current directory to find test data, making tests work regardless of execution directory.

```go
func TestScriptExecution(t *testing.T) {
    // Finds tests/fixtures relative to repo root
    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
    scriptPath := filepath.Join(fixturesDir, "deploy.sh")
    
    // Run test with fixture
    result := runScript(scriptPath)
    if result.ExitCode != 0 {
        t.Error("script execution failed")
    }
}
```

#### `TempDir` - Isolated Test Environments
Create temporary directories with automatic cleanup via `t.Cleanup()`. Safe for parallel tests, cross-platform, and guarantees cleanup even on test failure.

```go
func TestFileOperations(t *testing.T) {
    tmpDir := testutil.TempDir(t)
    // Automatically cleaned up after test
    
    configPath := filepath.Join(tmpDir, "config.json")
    writeConfig(configPath, testConfig)
    
    loaded := loadConfig(configPath)
    // Test assertions...
}
```

#### `Contains` - String Containment Helper
Convenience helper for string containment checks. Simple, readable alternative to `strings.Contains` with better test failure messages.

```go
func TestErrorMessage(t *testing.T) {
    err := validateInput("")
    if err == nil {
        t.Fatal("expected error")
    }
    
    if !testutil.Contains(err.Error(), "cannot be empty") {
        t.Errorf("unexpected error: %v", err)
    }
}
```

**Features:**
- Proper test line reporting via `t.Helper()` in all functions
- Automatic cleanup of temporary resources
- Cross-platform path handling (Windows and Unix)
- Reliable stdout capture with goroutine-based reading
- Thread-safe for parallel test execution

**Benefits:**
- **Eliminates Duplicate Test Helpers**: No more copying test utilities between projects
- **Reliable CLI Testing**: Battle-tested stdout capture handles edge cases
- **Flexible Fixture Location**: Tests work from any directory (IDE, CLI, CI)
- **Automatic Cleanup**: No manual cleanup code, no leaked temp directories
- **Consistent Testing Patterns**: All azd extensions test the same way

---

### CLI Output Formatting (cliout package)

**Coverage**: 94.9% (43 test cases)

The `cliout` package provides comprehensive CLI output formatting extracted from azd-app. This package enables professional, consistent CLI user experience across all azd extensions with minimal effort.

**Output Formats:**

#### `FormatDefault` - Human-Readable Output
Rich text output with ANSI colors, Unicode symbols, and visual structure. Automatically detects terminal capabilities and falls back to ASCII when needed.

#### `FormatJSON` - Machine-Readable Output
Structured JSON output for automation, scripting, and CI/CD pipelines. All output functions support JSON mode.

**Key Functions:**

#### Status Messages
```go
cliout.Success("Deployment completed in %s", duration)
// ✔ Deployment completed in 2m15s (default mode)
// {"level":"success","message":"Deployment completed in 2m15s"} (JSON mode)

cliout.Error("Failed to connect to %s: %v", host, err)
// ✘ Failed to connect to api.example.com: connection refused

cliout.Warning("API key will expire in %d days", days)
// ⚠ API key will expire in 7 days

cliout.Info("Processing %d items", count)
// ℹ Processing 42 items
```

#### Section Headers
```go
cliout.Header("Deployment Configuration")
// ═══════════════════════════════════════
// Deployment Configuration
// ═══════════════════════════════════════

cliout.Section("🚀", "Starting deployment")
// 🚀 Starting deployment
//    ────────────────────
```

#### Lists and Labels
```go
cliout.Bullet("Service: %s", name)
// • Service: web-api

cliout.Label("Status", "Running")
//   Status: Running
```

#### Tables
```go
headers := []string{"Name", "Status", "Port"}
rows := []cliout.TableRow{
    {"Name": "web", "Status": "running", "Port": "8080"},
    {"Name": "api", "Status": "stopped", "Port": "3000"},
}
cliout.Table(headers, rows)
// Name  Status   Port
// ────  ───────  ────
// web   running  8080
// api   stopped  3000
```

#### Progress Indicators
```go
progress := cliout.ProgressBar(7, 10, 40)
fmt.Println(progress)
// [████████████████████────────────────────] 70%
```

#### Interactive Prompts
```go
if cliout.Confirm("Deploy to production?") {
    // User confirmed (y/Y entered)
    deploy()
}
// In JSON mode, returns true without prompting (CI/CD friendly)
```

#### Hybrid Output (Default/JSON)
```go
data := map[string]interface{}{
    "status": "success",
    "deployed": 5,
    "failed": 0,
}

cliout.Print(data, func() {
    cliout.Success("Deployed %d services", 5)
})
// Default mode: ✔ Deployed 5 services
// JSON mode: {"status":"success","deployed":5,"failed":0}
```

**Format Management:**
```go
// Set output format
if err := cliout.SetFormat("json"); err != nil {
    log.Fatal(err)
}

// Check current format
if cliout.IsJSON() {
    // Skip interactive prompts
}

// Get format for custom logic
format := cliout.GetFormat()
```

**Orchestration Mode:**
```go
// Skip headers when composing subcommands
cliout.SetOrchestrated(true)
// Now Header() calls are no-ops

// Useful for workflows that call multiple commands
runWorkflow()

cliout.SetOrchestrated(false)
```

**Features:**
- **Cross-Platform Unicode Detection**: Windows Terminal, VS Code, PowerShell, ConEmu
- **ASCII Fallback**: Automatic fallback for legacy terminals (old cmd.exe)
- **Consistent Color Scheme**: Green (success), red (error), yellow (warning), blue (info)
- **Non-Interactive Mode**: JSON mode skips prompts for CI/CD
- **Orchestration Support**: Skip headers when composing commands
- **Rich Visual Elements**: Tables, progress bars, sections, bullets
- **Thread-Safe**: Safe for concurrent use from multiple goroutines

**Benefits:**
- **Brand Consistency**: All azd extensions look and feel the same
- **Professional UX**: Rich formatting out of the box
- **Automation Ready**: JSON mode for scripting and CI/CD
- **Cross-Platform**: Handles terminal differences automatically
- **Composable**: Orchestration mode for complex workflows
- **Battle-Tested**: 848 lines of tests covering edge cases

---

## Integration Benefits

### ✅ azd-exec Integration (Complete)

**testutil Migration:**
- Removed internal `testhelpers` package (100 lines deleted)
- Migrated to `azd-core/testutil` for all test utilities
- Enhanced test reliability with standardized helpers
- All tests pass with zero regressions

**cliout Integration:**
- Enhanced version command with formatted output and JSON mode
- Added colored info messages for listen command
- Improved Key Vault warnings with cliout.Warning
- Enhanced error messages with cliout.Error
- All 65 tests pass, backward compatible

**Total Impact:**
- **~100 lines removed** (testhelpers package)
- **Enhanced CLI output** with colors, icons, JSON mode
- **Improved developer experience** for testing
- **Zero breaking changes** for users

---

### ✅ azd-app Integration (Complete)

**testutil Adoption:**
- Added `testutil.CaptureOutput` for CLI command testing
- Enhanced logs tests with `testutil.Contains` (13 assertions)
- Created version command tests using CaptureOutput
- Improved test reliability and readability
- 5 new tests added, all 30+ tests pass

**cliout Migration:**
- Migrated 30 files from `internal/output` to `azd-core/cliout`
- Reduced `internal/output` to thin wrapper (125 lines) + progress tracking
- Deleted `output_test.go` (tests now in azd-core)
- Maintained CLI output compatibility (zero visual changes)
- All 35 tests pass, build and runtime verified

**Total Impact:**
- **~550 lines reduced** (output package minimized, tests in azd-core)
- **Enhanced test coverage** with standardized helpers
- **Maintained compatibility** (zero breaking changes)
- **Improved maintainability** (shared tests in azd-core)

---

## Extension Patterns Guide

**New Documentation Resource**: [docs/extension-patterns.md](docs/extension-patterns.md)

A comprehensive 1,056-line guide documenting best practices for building azd extensions, with 26 code examples covering:

### 1. Version Management Pattern
How to implement version information with build-time ldflags injection, including ExtensionID for registry integration.

```go
// Example: Extension version management
package version

var Version = "0.0.0-dev"      // -X flag: version
var BuildDate = "unknown"       // -X flag: build date
var GitCommit = "unknown"       // -X flag: git commit

const ExtensionID = "exec"     // Must match extension.yaml
const Name = "azd-exec"        // Human-readable name
```

### 2. Logging Pattern
Recommendations for structured logging with `log/slog`, including component-based loggers and context propagation.

### 3. Extension Structure Best Practices
Standard project layout, package organization, configuration management, and release process.

### 4. Testing Patterns
How to use `azd-core/testutil` for CLI command testing, test fixture management, and table-driven tests.

### 5. CLI Output Patterns
Comprehensive guide to using `azd-core/cliout` for status messages, tables, progress indicators, and hybrid output.

### 6. Error Handling Patterns
Structured errors, error wrapping, user-friendly messages, and validation errors.

**Benefits:**
- **Faster Onboarding**: New extensions follow established patterns
- **Consistency**: All extensions structured similarly
- **Best Practices**: Lessons learned from azd-exec and azd-app
- **Code Examples**: 26 real-world examples to copy-paste

---

## Quality Metrics

### Test Coverage by Package

| Package | Coverage | Tests | Lines | Status |
|---------|----------|-------|-------|--------|
| **testutil** | **83.3%** | 38 | 162 | ✅ Production-ready |
| **cliout** | **94.9%** | 43 | 464 | ✅ Production-ready |

### Coverage Details

**testutil** (83.3%):
- CaptureOutput: 100% (edge cases: panic, early return, errors)
- FindTestData: 85% (path traversal, missing directories)
- TempDir: 100% (cleanup, failure handling)
- Contains: 100% (basic helper)
- Platform-specific logic tested on Windows and Unix

**cliout** (94.9%):
- Format management: 100% (format validation, switching)
- Status messages: 100% (Success, Error, Warning, Info)
- Unicode detection: 95% (Windows Terminal, VS Code, PowerShell, ConEmu)
- Table rendering: 100% (column width, alignment, edge cases)
- Progress bars: 100% (boundaries, formatting)
- Interactive prompts: 100% (JSON mode, user input)
- Orchestration mode: 100% (header suppression)

### Integration Test Results

**azd-exec:**
- ✅ All 65 tests pass with azd-core/testutil and cliout
- ✅ Build verified (clean compilation)
- ✅ Runtime tested (version, listen, exec commands)
- ✅ Zero regressions

**azd-app:**
- ✅ All 35 package tests pass
- ✅ CLI output identical to pre-migration
- ✅ JSON mode tested and verified
- ✅ Build and runtime verified

---

## Total Impact

### Code Reduction
- **azd-exec**: ~100 lines (testhelpers package deleted)
- **azd-app**: ~550 lines (output package minimized to wrapper)
- **Total**: **~650 lines eliminated** across projects

### Standardization Achieved
- ✅ **Unified Testing**: All extensions use same test helpers
- ✅ **Consistent CLI UX**: All extensions have same look and feel
- ✅ **JSON Mode**: Automation-ready output across extensions
- ✅ **Documentation**: Extension patterns guide for best practices

### Quality Improvements
- ✅ **Battle-Tested Helpers**: 81 test cases across testutil and cliout
- ✅ **Cross-Platform**: Windows and Unix compatibility verified
- ✅ **Edge Case Handling**: Panic recovery, missing files, terminal detection
- ✅ **Thread-Safe**: Safe for parallel tests and concurrent use

### Developer Experience
- ✅ **Faster Testing**: CaptureOutput and FindTestData simplify CLI tests
- ✅ **Professional Output**: cliout provides rich formatting out of the box
- ✅ **Better Documentation**: Extension patterns guide accelerates development
- ✅ **Reduced Duplication**: No more copying test helpers or output code

---

## Migration Guide

### Adopting testutil in Your Extension

#### Step 1: Add Dependency
```bash
go get github.com/jongio/azd-core/testutil@v0.2.0
```

#### Step 2: Migrate CLI Command Tests
**Before:**
```go
func TestVersionCommand(t *testing.T) {
    // Manually capture stdout
    old := os.Stdout
    r, w, _ := os.Pipe()
    os.Stdout = w
    
    versionCmd.Execute()
    
    w.Close()
    os.Stdout = old
    
    var buf bytes.Buffer
    io.Copy(&buf, r)
    output := buf.String()
    
    if !strings.Contains(output, "version") {
        t.Error("expected version in output")
    }
}
```

**After:**
```go
import "github.com/jongio/azd-core/testutil"

func TestVersionCommand(t *testing.T) {
    output := testutil.CaptureOutput(t, func() error {
        return versionCmd.Execute()
    })
    
    if !testutil.Contains(output, "version") {
        t.Error("expected version in output")
    }
}
```

#### Step 3: Migrate Test Fixture Handling
**Before:**
```go
func TestScript(t *testing.T) {
    wd, _ := os.Getwd()
    fixturesDir := filepath.Join(wd, "..", "..", "tests", "fixtures")
    if _, err := os.Stat(fixturesDir); err != nil {
        // Try alternative path...
    }
    // ...
}
```

**After:**
```go
func TestScript(t *testing.T) {
    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
    // Automatically finds directory from repo root
}
```

#### Step 4: Replace Temporary Directory Creation
**Before:**
```go
func TestFileOps(t *testing.T) {
    tmpDir, err := os.MkdirTemp("", "test-*")
    if err != nil {
        t.Fatal(err)
    }
    defer os.RemoveAll(tmpDir) // Manual cleanup
    // ...
}
```

**After:**
```go
func TestFileOps(t *testing.T) {
    tmpDir := testutil.TempDir(t)
    // Automatic cleanup via t.Cleanup()
}
```

---

### Adopting cliout in Your Extension

#### Step 1: Add Dependency
```bash
go get github.com/jongio/azd-core/cliout@v0.2.0
```

#### Step 2: Replace fmt.Printf with Status Messages
**Before:**
```go
fmt.Printf("✔ Deployment succeeded\n")
fmt.Fprintf(os.Stderr, "✘ Deployment failed: %v\n", err)
```

**After:**
```go
import "github.com/jongio/azd-core/cliout"

cliout.Success("Deployment succeeded")
cliout.Error("Deployment failed: %v", err)
```

#### Step 3: Add JSON Output Mode
**Before:**
```go
// Only text output
fmt.Printf("Status: %s\n", status)
```

**After:**
```go
// Support both text and JSON
data := map[string]string{"status": status}
cliout.Print(data, func() {
    cliout.Info("Status: %s", status)
})

// Or configure globally
if outputFormat == "json" {
    cliout.SetFormat("json")
}
```

#### Step 4: Add Tables for Structured Data
**Before:**
```go
fmt.Printf("Name\tStatus\tPort\n")
for _, svc := range services {
    fmt.Printf("%s\t%s\t%d\n", svc.Name, svc.Status, svc.Port)
}
```

**After:**
```go
headers := []string{"Name", "Status", "Port"}
var rows []cliout.TableRow
for _, svc := range services {
    rows = append(rows, cliout.TableRow{
        "Name":   svc.Name,
        "Status": svc.Status,
        "Port":   fmt.Sprint(svc.Port),
    })
}
cliout.Table(headers, rows)
```

#### Step 5: Add Progress Indicators
```go
for i := 0; i < total; i++ {
    processItem(items[i])
    
    progress := cliout.ProgressBar(i+1, total, 40)
    fmt.Printf("\r%s", progress)
}
fmt.Println() // Newline after progress
```

#### Step 6: Make Interactive Prompts CI-Friendly
**Before:**
```go
fmt.Print("Continue? (y/n): ")
var response string
fmt.Scanln(&response)
if response != "y" && response != "Y" {
    return
}
```

**After:**
```go
if !cliout.Confirm("Continue?") {
    return
}
// In JSON mode (CI/CD), returns true without prompting
```

---

### Complete Example: Adding Both Packages

```go
package main

import (
    "context"
    "github.com/jongio/azd-core/cliout"
    "github.com/jongio/azd-core/testutil"
    "testing"
)

// Production code
func deployServices(ctx context.Context, services []string) error {
    cliout.Header("Deploying Services")
    
    for i, svc := range services {
        cliout.Info("Deploying %s...", svc)
        
        if err := deploy(ctx, svc); err != nil {
            cliout.Error("Failed to deploy %s: %v", svc, err)
            return err
        }
        
        progress := cliout.ProgressBar(i+1, len(services), 40)
        fmt.Printf("\r%s", progress)
    }
    
    cliout.Success("Deployed %d services", len(services))
    return nil
}

// Test code
func TestDeployServices(t *testing.T) {
    output := testutil.CaptureOutput(t, func() error {
        services := []string{"web", "api"}
        return deployServices(context.Background(), services)
    })
    
    // Verify output
    if !testutil.Contains(output, "Deployed 2 services") {
        t.Errorf("unexpected output: %s", output)
    }
}

func TestWithTestData(t *testing.T) {
    // Load test fixtures
    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
    config := loadConfig(filepath.Join(fixturesDir, "config.json"))
    
    // Create temporary directory for outputs
    tmpDir := testutil.TempDir(t)
    
    // Run test with fixtures
    result := runDeployment(config, tmpDir)
    
    // Assertions...
}
```

---

## Get Started

### Installation

```bash
# Get latest version
go get github.com/jongio/azd-core@v0.2.0

# Or get specific packages
go get github.com/jongio/azd-core/testutil@v0.2.0
go get github.com/jongio/azd-core/cliout@v0.2.0
```

### Quick Start with testutil

```go
import (
    "testing"
    "github.com/jongio/azd-core/testutil"
)

func TestCLICommand(t *testing.T) {
    // Capture command output
    output := testutil.CaptureOutput(t, func() error {
        return runCommand()
    })
    
    // Verify output
    if !testutil.Contains(output, "success") {
        t.Error("expected success message")
    }
}

func TestWithFixtures(t *testing.T) {
    // Find test data
    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
    
    // Create temp directory
    tmpDir := testutil.TempDir(t)
    
    // Run test...
}
```

### Quick Start with cliout

```go
import "github.com/jongio/azd-core/cliout"

func main() {
    // Set format from flag
    if *outputFormat == "json" {
        cliout.SetFormat("json")
    }
    
    // Print status messages
    cliout.Info("Processing %d items...", count)
    
    if err := process(); err != nil {
        cliout.Error("Processing failed: %v", err)
        os.Exit(1)
    }
    
    cliout.Success("Processing complete")
}
```

### Documentation

- **API Documentation**: https://pkg.go.dev/github.com/jongio/azd-core@v0.2.0
- **Extension Patterns Guide**: [docs/extension-patterns.md](docs/extension-patterns.md)
- **Repository**: https://github.com/jongio/azd-core
- **Issues**: https://github.com/jongio/azd-core/issues
- **Contributing**: [CONTRIBUTING.md](CONTRIBUTING.md)
- **Security**: [SECURITY.md](SECURITY.md)

---

## What's Next

### v0.4.0 Candidates (Under Consideration)

Based on usage patterns and feedback, future releases may include:

- **errors package**: Standardized error types (ValidationError, NotFoundError, ExecutionError)
- **constants package**: Common timeout values, error limits, network defaults
- **Additional testutil helpers**: Assertion helpers, mock utilities
- **cliout enhancements**: Custom color schemes, advanced table formatting

### Feedback Welcome

Have suggestions for v0.4.0? Open an issue or discussion at:
https://github.com/jongio/azd-core/issues

---

## Acknowledgments

Special thanks to the azd-exec and azd-app teams for:
- Identifying extraction opportunities through the azd-core extraction spec
- Thoroughly testing migrations and providing feedback
- Contributing patterns to the Extension Patterns Guide
- Achieving ~650 lines of code reduction across projects

This release represents the culmination of Tasks 1-8 in the azd-core extraction project, delivering comprehensive testing and CLI utilities for the entire azd ecosystem.

---

## Full Package List (8 packages)

### v0.2.0 Packages
- **testutil** (NEW) - Test utilities for CLI testing
- **cliout** (NEW) - CLI output formatting with JSON mode
- **fileutil** - File system utilities (89% coverage)
- **pathutil** - PATH management (83% coverage)
- **browser** - Browser launching (77% coverage)
- **security** - Security validation (87% coverage)
- **procutil** - Process detection (81% coverage)
- **shellutil** - Shell detection (85% coverage)

### v0.1.0 Packages
- **env** - Environment variable resolution
- **keyvault** - Azure Key Vault integration

See [CHANGELOG.md](CHANGELOG.md) for complete version history.
