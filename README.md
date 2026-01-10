# azd-core

[![Go Reference](https://pkg.go.dev/badge/github.com/jongio/azd-core.svg)](https://pkg.go.dev/github.com/jongio/azd-core)
[![Go Report Card](https://goreportcard.com/badge/github.com/jongio/azd-core)](https://goreportcard.com/report/github.com/jongio/azd-core)
[![CI](https://github.com/jongio/azd-core/workflows/CI/badge.svg)](https://github.com/jongio/azd-core/actions/workflows/ci.yml)
[![codecov](https://codecov.io/gh/jongio/azd-core/branch/main/graph/badge.svg)](https://codecov.io/gh/jongio/azd-core)

Common reusable Go modules for building Azure Developer CLI (azd) extensions and tooling.

## Overview

`azd-core` provides shared utilities extracted from the Azure Developer CLI to support building azd extensions, custom CLI tools, and automation scripts. The goal is to enable developers to create azd-compatible tools without duplicating common logic or pulling in the entire azd runtime.

This library includes:
- **Environment Management**: Environment variable resolution and Key Vault integration
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

## Packages

### `env`
Environment variable utilities for converting between maps and slices, resolving references, and applying transformations.

**Key Functions:**
- `ResolveMap` - Resolve references in environment maps
- `ResolveSlice` - Resolve references in environment slices (`[]string`)
- `MapToSlice` / `SliceToMap` - Convert between formats
- `HasKeyVaultReferences` - Detect Key Vault references in environment data

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
Cross-platform process detection utilities.

**Key Functions:**
- `IsProcessRunning` - Check if process with given PID is running

**Features:**
- Cross-platform implementation (Windows and Unix)
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
