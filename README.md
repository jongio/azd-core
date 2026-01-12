# azd-core

[![Go Reference](https://pkg.go.dev/badge/github.com/jongio/azd-core.svg)](https://pkg.go.dev/github.com/jongio/azd-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/jongio/azd-core)](https://goreportcard.com/report/github.com/jongio/azd-core)
[![CI](https://github.com/jongio/azd-core/workflows/CI/badge.svg)](https://github.com/jongio/azd-core/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jongio/azd-core/branch/main/graph/badge.svg)](https://codecov.io/gh/jongio/azd-core)

Common reusable Go modules for building Azure Developer CLI (azd) extensions and tooling.

## Overview

`azd-core` provides shared utilities extracted from the Azure Developer CLI to support building azd extensions, custom CLI tools, and automation scripts. The goal is to enable developers to create azd-compatible tools without duplicating common logic or pulling in the entire azd runtime.

This library includes:
- **URL Validation**: RFC-compliant HTTP/HTTPS URL validation and parsing
- **Environment Management**: Environment variable resolution, pattern extraction, and Key Vault integration
- **File System Utilities**: Atomic writes, JSON handling, secure file operations
- **Path Management**: Tool discovery, PATH manipulation, installation suggestions
- **Process Utilities**: Cross-platform process detection and management
- **Shell Detection**: Script type detection from extensions, shebangs, and OS defaults
- **Browser Launching**: Secure cross-platform URL opening
- **Security Validation**: Path traversal prevention, input sanitization, permission checks

## Installation

```bash
go get github.com/jongio/azd-core
```

Or add specific packages to your `go.mod`:

```bash
go get github.com/jongio/azd-core/urlutil
go get github.com/jongio/azd-core/testutil
go get github.com/jongio/azd-core/cliout
go get github.com/jongio/azd-core/env
go get github.com/jongio/azd-core/keyvault
go get github.com/jongio/azd-core/fileutil
go get github.com/jongio/azd-core/pathutil
go get github.com/jongio/azd-core/browser
go get github.com/jongio/azd-core/security
go get github.com/jongio/azd-core/procutil
go get github.com/jongio/azd-core/shellutil
```

## Documentation

Full API documentation is available at [pkg.go.dev/github.com/jongio/azd-core](https://pkg.go.dev/github.com/jongio/azd-core).

**Extension Development:**
- [Extension Patterns Guide](docs/extension-patterns.md) - Comprehensive patterns and best practices for building azd extensions

**Migration Guides:**
- [URL Validation and Environment Patterns Migration](docs/migration-urlutil-env.md) - Migrate from custom validation to azd-core utilities

## Packages

### `urlutil`
URL validation and parsing utilities with RFC-compliant validation.

**Key Functions:**
- `Validate` - Comprehensive HTTP/HTTPS URL validation using `net/url.Parse`
- `ValidateHTTPSOnly` - Enforce HTTPS-only for production (allows localhost HTTP)
- `Parse` - Parse and normalize URLs with validation
- `NormalizeScheme` - Ensure URL has http:// or https:// prefix

**Validation Rules:**
- Protocol must be http:// or https:// (rejects ftp://, file://, javascript://, etc.)
- URL must have a valid host/domain (rejects "http://", "https://")
- URL must not exceed 2048 characters (RFC 2616 practical limit)
- Uses `net/url.Parse` for RFC 3986 compliant parsing
- Whitespace is trimmed before validation

**Security Features:**
- Prevents protocol injection (javascript:, file:, data: URLs)
- Validates host presence to prevent malformed URLs
- Length limits prevent DoS via extremely long URLs
- HTTPS enforcement for production with localhost exception

**Example:**
```go
import "github.com/jongio/azd-core/urlutil"

// Validate custom URL from configuration
if err := urlutil.Validate(customURL); err != nil {
    return fmt.Errorf("invalid custom URL: %w", err)
}

// Enforce HTTPS for production endpoints (allows localhost HTTP)
if err := urlutil.ValidateHTTPSOnly(apiEndpoint); err != nil {
    return fmt.Errorf("production endpoint must use HTTPS: %w", err)
}

// Parse and normalize URL
parsed, err := urlutil.Parse(userProvidedURL)
if err != nil {
    return err
}
fmt.Printf("Accessing: %s://%s\n", parsed.Scheme, parsed.Host)

// Add default scheme if missing
normalized := urlutil.NormalizeScheme("example.com", "https")
// Returns: "https://example.com"
```

### `testutil`
Common testing utilities for writing reliable tests in azd extensions.

**Key Functions:**
- `CaptureOutput` - Capture stdout during function execution for testing CLI commands
- `FindTestData` - Locate test fixture directories with flexible path searching
- `TempDir` - Create temporary directories with automatic cleanup via t.Cleanup()
- `Contains` - Convenience helper for string containment checks

**Features:**
- Proper test line reporting via t.Helper() in all functions
- Automatic cleanup of temporary resources
- Cross-platform path handling
- Reliable stdout capture with goroutine-based reading

**Example:**
```go
import "github.com/jongio/azd-core/testutil"

func TestCLICommand(t *testing.T) {
    // Capture command output
    output := testutil.CaptureOutput(t, func() error {
        return runCommand()
    })
    
    if !testutil.Contains(output, "success") {
        t.Error("expected success message")
    }
}

func TestWithFixtures(t *testing.T) {
    // Find test data directory
    fixturesDir := testutil.FindTestData(t, "tests", "fixtures")
    
    // Create temporary directory for outputs
    tmpDir := testutil.TempDir(t)
    // Automatically cleaned up after test
}
```

### `cliout`
Structured CLI output formatting with cross-platform terminal support and multiple output formats.

**Key Functions:**
- `Success` / `Error` / `Warning` / `Info` - Colored status messages with icons
- `Header` / `Section` - Formatted section headers
- `Table` - Simple table rendering with automatic column width calculation
- `ProgressBar` - Visual progress indicators
- `Confirm` - Interactive yes/no prompts (non-interactive in JSON mode)
- `Print` - Hybrid output (JSON or formatted text)

**Output Formats:**
- `FormatDefault` - Human-readable text with ANSI colors and Unicode symbols
- `FormatJSON` - Structured JSON for automation and scripting

**Example:**
```go
import "github.com/jongio/azd-core/cliout"

// Set output format
if err := cliout.SetFormat("json"); err != nil {
    log.Fatal(err)
}

// Print status messages
cliout.Success("Deployment completed successfully")
cliout.Error("Failed to connect: %s", err)
cliout.Warning("This feature is deprecated")
cliout.Info("Processing %d items", count)

// Create tables
headers := []string{"Name", "Status", "Port"}
rows := []cliout.TableRow{
    {"Name": "web", "Status": "running", "Port": "8080"},
    {"Name": "api", "Status": "stopped", "Port": "3000"},
}
cliout.Table(headers, rows)

// Hybrid output (JSON mode or formatted)
data := map[string]interface{}{"status": "success", "count": 42}
cliout.Print(data, func() {
    cliout.Success("Processed %d items", 42)
})

// Interactive prompts
if cliout.Confirm("Do you want to continue?") {
    // User confirmed (always true in JSON mode)
}

// Orchestration mode for subcommands
cliout.SetOrchestrated(true)
// Now CommandHeader() calls are skipped
```

### `env`
Environment variable utilities for converting between maps and slices, resolving references, and applying transformations.

**Key Functions:**
- `ResolveMap` / `ResolveSlice` - Resolve Key Vault references in environment variables
- `MapToSlice` / `SliceToMap` - Convert between map and slice formats
- `HasKeyVaultReferences` - Detect Key Vault references in environment data
- `FilterByPrefix` / `FilterByPrefixSlice` - Filter environment variables by prefix (case-insensitive)
- `ExtractPattern` - Extract environment variables matching prefix/suffix with key transformation
- `NormalizeServiceName` - Convert environment variable naming to service naming (MY_API → my-api)

**Pattern Extraction Features:**
- Case-insensitive prefix/suffix matching
- Optional prefix/suffix trimming from result keys
- Custom key transformation functions
- Value validation with callback functions
- Useful for extracting service URLs, Azure variables, custom domain configs

**Example:**
```go
import "github.com/jongio/azd-core/env"

// Filter by prefix (case-insensitive)
envVars := map[string]string{
    "AZURE_TENANT_ID": "xyz",
    "AZURE_CLIENT_ID": "abc",
    "DATABASE_URL": "postgres://...",
}
azureVars := env.FilterByPrefix(envVars, "AZURE_")
// Returns: {"AZURE_TENANT_ID": "xyz", "AZURE_CLIENT_ID": "abc"}

// Extract service URLs with normalization
serviceEnv := map[string]string{
    "SERVICE_MY_API_URL": "https://api.example.com",
    "SERVICE_WEB_APP_URL": "https://web.example.com",
    "SERVICE_DB_HOST": "db.example.com",
}
urls, _ := env.ExtractPattern(serviceEnv, env.PatternOptions{
    Prefix:       "SERVICE_",
    Suffix:       "_URL",
    TrimPrefix:   true,
    TrimSuffix:   true,
    KeyTransform: env.NormalizeServiceName, // MY_API → my-api
})
// Returns: {"my-api": "https://api.example.com", "web-app": "https://web.example.com"}
```

**Key Vault Resolution:**

### `keyvault`
Azure Key Vault reference detection and resolution for environment variables.

**Supported Formats:**
- `@Microsoft.KeyVault(SecretUri=https://...)`
- `@Microsoft.KeyVault(VaultName=...;SecretName=...;SecretVersion=...)`
- `akvs://<subscription-id>/<vault-name>/<secret-name>[/<version>]`

**Features:**
- Uses `azidentity.DefaultAzureCredential` for authentication
- Thread-safe client caching
- Configurable error handling (fail-fast or graceful degradation)
- SSRF protection and validation

### `fileutil`
File system utilities with atomic operations, JSON handling, and secure file detection.

**Key Functions:**
- `AtomicWriteJSON` / `AtomicWriteFile` - Write files atomically with retry logic
- `ReadJSON` - Read JSON with graceful missing file handling
- `EnsureDir` - Create directories with secure permissions (0750)
- `FileExists` / `FileExistsAny` / `FilesExistAll` - File existence checks
- `HasFileWithExt` / `HasAnyFileWithExts` - Extension-based file detection
- `ContainsText` / `ContainsTextInFile` - Search file contents

**Features:**
- Atomic writes prevent partial/corrupt files
- Retry logic for transient filesystem errors
- Secure permissions (directories: 0750, files: 0644)
- Path traversal protection via `security.ValidatePath`

### `pathutil`
PATH environment variable management and tool discovery utilities.

**Key Functions:**
- `RefreshPATH` - Refresh PATH from system (Windows registry, Unix environment)
- `FindToolInPath` - Search PATH for executables (auto .exe handling on Windows)
- `SearchToolInSystemPath` - Search common installation directories
- `GetInstallSuggestion` - Get installation URLs for 22+ popular tools

**Features:**
- Cross-platform PATH refresh (Windows PowerShell registry read, Unix environment)
- Automatic .exe extension handling on Windows
- Common install directory search (Program Files, /usr/local/bin, Homebrew, etc.)
- Installation suggestions for npm, python, docker, azd, and more

### `browser`
Cross-platform browser launching with URL validation and timeout support.

**Key Functions:**
- `Launch` - Open URL in system default browser (non-blocking)
- `ResolveTarget` - Resolve browser target (default, system, none)
- `ValidTargets` / `IsValid` - Target validation
- `GetTargetDisplayName` / `FormatValidTargets` - Display formatting

**Features:**
- Cross-platform support (Windows cmd/start, macOS open, Linux xdg-open)
- URL validation (http/https only for security)
- Non-blocking launch with configurable timeout
- Context-based cancellation
- Graceful error handling (warnings only, non-critical)

### `security`
Security validation utilities for path traversal prevention, input sanitization, and permission checks.

**Key Functions:**
- `ValidatePath` - Prevent path traversal attacks (detects `..`, resolves symlinks)
- `ValidateServiceName` - Validate service names (DNS-safe, container-safe)
- `ValidatePackageManager` - Allowlist-based package manager validation
- `SanitizeScriptName` - Detect shell metacharacters
- `IsContainerEnvironment` - Detect Codespaces, Dev Containers, Docker, Kubernetes
- `ValidateFilePermissions` - Detect world-writable files (Unix only)

**Features:**
- Path traversal attack prevention
- Symbolic link resolution and validation
- Service name validation (alphanumeric start, DNS label limits)
- Shell metacharacter detection
- Container environment detection
- World-writable file detection (security warning)

### `procutil`
Cross-platform process detection utilities using gopsutil for reliable cross-platform behavior.

**Key Functions:**
- `IsProcessRunning` - Check if process with given PID is running

**Features:**
- Cross-platform support (Windows, Linux, macOS, BSD, Solaris, AIX)
- Reliable Windows process detection (no stale PID issues)
- Uses platform-native APIs (Windows: OpenProcess, Linux: /proc, macOS: sysctl)
- Powered by github.com/shirou/gopsutil/v4
- Uses Signal(0) on Unix for accurate detection
- Windows fallback with documented limitations (stale PID may return true)
- Invalid PID handling (≤0 returns false)

### `shellutil`
Shell detection from file extensions, shebangs, and OS defaults.

**Key Functions:**
- `DetectShell` - Auto-detect shell from extension, shebang, or OS default
- `ReadShebang` - Parse shebang line to extract interpreter

**Shell Constants:**
- `ShellBash`, `ShellSh`, `ShellZsh` - Unix shells
- `ShellPwsh`, `ShellPowerShell` - PowerShell variants
- `ShellCmd` - Windows Command Prompt

**Features:**
- Extension detection (.ps1 → pwsh, .sh → bash, .cmd → cmd, etc.)
- Shebang parsing (#!/bin/bash, #!/usr/bin/env python3, etc.)
- OS-specific defaults (Windows: cmd, Unix: bash)
- Graceful error handling (falls back to OS default)

## Usage Examples

### Resolve Key Vault References in Environment

```go
package main

import (
    "context"
    "os"

    "github.com/jongio/azd-core/env"
    "github.com/jongio/azd-core/keyvault"
)

func main() {
    // Create resolver
    resolver, err := keyvault.NewKeyVaultResolver()
    if err != nil {
        panic(err)
    }

    // Resolve from environment map
    envMap := map[string]string{
        "DATABASE_PASSWORD": "@Microsoft.KeyVault(VaultName=myvault;SecretName=db-pass)",
        "API_ENDPOINT":      "https://api.example.com",
    }

    resolved, warnings, err := env.ResolveMap(
        context.Background(),
        envMap,
        resolver,
        keyvault.ResolveEnvironmentOptions{},
    )
    if err != nil {
        panic(err)
    }

    // Handle warnings
    for _, w := range warnings {
        os.Stderr.WriteString("warning: " + w.Err.Error() + "\n")
    }

    // Use resolved environment
    os.Setenv("DATABASE_PASSWORD", resolved["DATABASE_PASSWORD"])
}
```

### Atomic File Writing

```go
import "github.com/jongio/azd-core/fileutil"

// Write JSON atomically (prevents partial/corrupt files)
data := map[string]interface{}{
    "version": "1.0",
    "config":  map[string]string{"key": "value"},
}
err := fileutil.AtomicWriteJSON("config.json", data)
```

### Tool Discovery

```go
import (
    "fmt"
    "github.com/jongio/azd-core/pathutil"
)

// Find a tool in PATH
if nodePath := pathutil.FindToolInPath("node"); nodePath != "" {
    fmt.Printf("Node.js found at: %s\n", nodePath)
} else {
    fmt.Println(pathutil.GetInstallSuggestion("node"))
}

// Search common system directories
if dockerPath := pathutil.SearchToolInSystemPath("docker"); dockerPath != "" {
    fmt.Printf("Docker found at: %s\n", dockerPath)
}
```

### Secure Path Validation

```go
import "github.com/jongio/azd-core/security"

// Validate user-provided path (prevents path traversal)
if err := security.ValidatePath(userPath); err != nil {
    return fmt.Errorf("invalid path: %w", err)
}

// Validate service name (DNS-safe, container-safe)
if err := security.ValidateServiceName(name, false); err != nil {
    return fmt.Errorf("invalid service name: %w", err)
}
```

### Shell Detection

```go
import "github.com/jongio/azd-core/shellutil"

// Auto-detect shell from script
shell := shellutil.DetectShell("deploy.sh")  // Returns "bash"
shell = shellutil.DetectShell("setup.ps1")   // Returns "pwsh" or "powershell"
shell = shellutil.DetectShell("build.cmd")   // Returns "cmd"

// Read shebang to detect interpreter
if shebang := shellutil.ReadShebang("script.py"); shebang != "" {
    fmt.Printf("Interpreter: %s\n", shebang)  // "python3"
}
```

### Browser Launch

```go
import (
    "github.com/jongio/azd-core/browser"
    "time"
)

// Open URL in default browser
err := browser.Launch(browser.LaunchOptions{
    URL:     "https://example.com",
    Target:  browser.TargetDefault,
    Timeout: 5 * time.Second,
})
```

### Process Detection

```go
import "github.com/jongio/azd-core/procutil"

// Check if process is running
if procutil.IsProcessRunning(pid) {
    fmt.Println("Process is running")
}
```

## Authentication

The `keyvault` package uses `azidentity.DefaultAzureCredential`, supporting:
- Environment variables (`AZURE_TENANT_ID`, `AZURE_CLIENT_ID`, `AZURE_CLIENT_SECRET`)
- Managed identity (Azure VM, App Service, Container Apps, etc.)
- Azure CLI (`az login`)
- Azure PowerShell
- Interactive browser authentication

No global state is maintained, and client caching is thread-safe.

## Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

Tests are offline-only and use mocks for Azure SDK interactions.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on contributing to this project.

## Security

See [SECURITY.md](SECURITY.md) for information on reporting security vulnerabilities.

## License

This project is licensed under the MIT License. See [LICENSE](LICENSE).
