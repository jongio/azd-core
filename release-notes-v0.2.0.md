# azd-core v0.2.0 - Core Utilities Release

## Overview

This release introduces **6 production-ready utility packages** for building Azure Developer CLI (azd) extensions and tooling. These packages provide shared utilities extracted from azd to enable developers to create azd-compatible tools without duplicating common logic.

## What's New

### Core Utility Packages (6 packages)

#### 1. **fileutil** - File System Utilities (89% coverage)
- Atomic file operations with retry logic
- JSON handling with graceful error handling
- Secure file detection and permissions (0750 for dirs, 0644 for files)
- Path traversal protection

**Key Functions:**
- `AtomicWriteJSON` / `AtomicWriteFile` - Prevent partial/corrupt files
- `ReadJSON` - Graceful missing file handling
- `EnsureDir` - Secure directory creation
- `FileExists` / `FileExistsAny` / `FilesExistAll`
- `HasFileWithExt` / `HasAnyFileWithExts`
- `ContainsText` / `ContainsTextInFile`

#### 2. **pathutil** - PATH Management (83% coverage)
- PATH environment variable refresh (Windows registry, Unix environment)
- Tool discovery with auto .exe handling on Windows
- Installation suggestions for 22+ popular tools

**Key Functions:**
- `RefreshPATH` - Refresh PATH from system
- `FindToolInPath` - Search PATH for executables
- `SearchToolInSystemPath` - Search common install directories
- `GetInstallSuggestion` - Installation URLs for npm, python, docker, azd, etc.

#### 3. **browser** - Browser Launching (77% coverage)
- Cross-platform URL opening (Windows cmd/start, macOS open, Linux xdg-open)
- URL validation (http/https only for security)
- Non-blocking launch with configurable timeout
- Context-based cancellation

**Key Functions:**
- `Launch` - Open URL in system default browser
- `ResolveTarget` - Resolve browser target (default, system, none)

#### 4. **security** - Security Validation (87% coverage)
- Path traversal attack prevention
- Input sanitization and validation
- Container environment detection
- World-writable file detection (Unix)

**Key Functions:**
- `ValidatePath` - Prevent path traversal (detects `..`, resolves symlinks)
- `ValidateServiceName` - DNS-safe, container-safe validation
- `ValidatePackageManager` - Allowlist-based validation
- `SanitizeScriptName` - Detect shell metacharacters
- `IsContainerEnvironment` - Detect Codespaces, Dev Containers, Docker, K8s
- `ValidateFilePermissions` - Detect world-writable files

#### 5. **procutil** - Process Detection (81% coverage)
- Cross-platform process detection using gopsutil
- Reliable PID checking (Windows, Linux, macOS, BSD, Solaris, AIX)
- Platform-native APIs (Windows: OpenProcess, Linux: /proc, macOS: sysctl)

**Key Functions:**
- `IsProcessRunning` - Check if process with PID is running

**Dependencies:**
- `github.com/shirou/gopsutil/v4 v4.25.12`

#### 6. **shellutil** - Shell Detection (85% coverage)
- Auto-detect shell from extension, shebang, or OS default
- Support for bash, sh, zsh, pwsh, powershell, cmd
- Extension detection (.ps1 → pwsh, .sh → bash, .cmd → cmd)
- Shebang parsing (#!/bin/bash, #!/usr/bin/env python3)

**Key Functions:**
- `DetectShell` - Auto-detect shell type
- `ReadShebang` - Parse shebang line

## Integration Benefits

### ✅ azd-exec Integration (Complete)
- Integrated **shellutil** for shell detection and command building
- **349 lines of duplicate code removed**
- Improved reliability with standardized shell detection
- Enhanced cross-platform compatibility

### ✅ azd-app Integration (Complete)
- Integrated **fileutil** for atomic file operations
- **50 lines of duplicate code removed**
- **Critical Bug Fix**: Fixed config file corruption in azd-app
  - Atomic writes prevent partial/corrupt config files
  - Retry logic handles transient filesystem errors
  - Secure permissions prevent unauthorized access
- Enhanced security with path traversal protection

## Quality Metrics

### Test Coverage
- **fileutil**: 89%
- **pathutil**: 83%
- **browser**: 77%
- **security**: 87%
- **procutil**: 81%
- **shellutil**: 85%

### Total Impact
- ✅ **6 core utility packages** published and production-ready
- ✅ **399 lines of code** removed from dependent projects
- ✅ **1 critical bug** fixed in azd-app config handling
- ✅ **77-89% test coverage** across all packages
- ✅ **Zero breaking changes** - fully backward compatible

## Installation

```bash
go get github.com/jongio/azd-core@v0.2.0
```

Or add specific packages:

```bash
go get github.com/jongio/azd-core/fileutil@v0.2.0
go get github.com/jongio/azd-core/pathutil@v0.2.0
go get github.com/jongio/azd-core/browser@v0.2.0
go get github.com/jongio/azd-core/security@v0.2.0
go get github.com/jongio/azd-core/procutil@v0.2.0
go get github.com/jongio/azd-core/shellutil@v0.2.0
```

## Documentation

- **API Documentation**: https://pkg.go.dev/github.com/jongio/azd-core@v0.2.0
- **Repository**: https://github.com/jongio/azd-core
- **Issues**: https://github.com/jongio/azd-core/issues
- **Contributing**: [CONTRIBUTING.md](https://github.com/jongio/azd-core/blob/main/CONTRIBUTING.md)
- **Security**: [SECURITY.md](https://github.com/jongio/azd-core/blob/main/SECURITY.md)

## What's Next

Future releases will include additional packages from the consolidation roadmap:
- `env` - Environment variable resolution
- `keyvault` - Azure Key Vault integration
- Additional utilities as needed

See [CHANGELOG.md](https://github.com/jongio/azd-core/blob/main/CHANGELOG.md) for full details.
