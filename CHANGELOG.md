# Changelog

All notable changes to azd-core will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.2.0] - 2026-01-10

### Added

#### New Core Utility Packages
- **fileutil**: File system utilities with atomic operations, JSON handling, and secure file detection
  - `AtomicWriteJSON` / `AtomicWriteFile` - Write files atomically with retry logic to prevent partial/corrupt files
  - `ReadJSON` - Read JSON with graceful missing file handling
  - `EnsureDir` - Create directories with secure permissions (0750)
  - `FileExists` / `FileExistsAny` / `FilesExistAll` - Comprehensive file existence checks
  - `HasFileWithExt` / `HasAnyFileWithExts` - Extension-based file detection
  - `ContainsText` / `ContainsTextInFile` - Search file contents
  - Path traversal protection via `security.ValidatePath`

- **pathutil**: PATH environment variable management and tool discovery utilities
  - `RefreshPATH` - Refresh PATH from system (Windows registry, Unix environment)
  - `FindToolInPath` - Search PATH for executables with automatic .exe handling on Windows
  - `SearchToolInSystemPath` - Search common installation directories
  - `GetInstallSuggestion` - Installation URLs for 22+ popular development tools
  - Cross-platform PATH refresh support

- **browser**: Cross-platform browser launching with URL validation and timeout support
  - `Launch` - Open URLs in system default browser (non-blocking)
  - `ResolveTarget` - Resolve browser target (default, system, none)
  - URL validation (http/https only for security)
  - Context-based cancellation with configurable timeout
  - Graceful error handling

- **security**: Security validation utilities for path traversal prevention and input sanitization
  - `ValidatePath` - Prevent path traversal attacks (detects `..`, resolves symlinks)
  - `ValidateServiceName` - Validate service names (DNS-safe, container-safe)
  - `ValidatePackageManager` - Allowlist-based package manager validation
  - `SanitizeScriptName` - Detect shell metacharacters
  - `IsContainerEnvironment` - Detect Codespaces, Dev Containers, Docker, Kubernetes
  - `ValidateFilePermissions` - Detect world-writable files (Unix only)

- **procutil**: Cross-platform process detection utilities using gopsutil
  - `IsProcessRunning` - Check if process with given PID is running
  - Reliable cross-platform support (Windows, Linux, macOS, BSD, Solaris, AIX)
  - Uses platform-native APIs (Windows: OpenProcess, Linux: /proc, macOS: sysctl)
  - Powered by github.com/shirou/gopsutil/v4 v4.25.12

- **shellutil**: Shell detection from file extensions, shebangs, and OS defaults
  - `DetectShell` - Auto-detect shell from extension, shebang, or OS default
  - `ReadShebang` - Parse shebang line to extract interpreter
  - Support for bash, sh, zsh, pwsh, powershell, cmd
  - Extension detection (.ps1 → pwsh, .sh → bash, .cmd → cmd)
  - Shebang parsing (#!/bin/bash, #!/usr/bin/env python3)
  - OS-specific defaults (Windows: cmd, Unix: bash)

#### Dependencies
- Added `github.com/shirou/gopsutil/v4 v4.25.12` for reliable cross-platform process detection

### Integration Benefits

#### azd-exec Integration (Complete)
- Integrated shellutil for shell detection and command building
- **Code Reduction**: 349 lines of duplicate code removed
- Improved reliability with standardized shell detection logic
- Enhanced cross-platform compatibility

#### azd-app Integration (Complete)
- Integrated fileutil for atomic file operations and JSON handling
- **Code Reduction**: 50 lines of duplicate code removed
- **Critical Bug Fix**: Fixed config file corruption issue in azd-app config module
  - Atomic writes prevent partial/corrupt config files during concurrent operations
  - Retry logic handles transient filesystem errors
  - Secure permissions prevent unauthorized access
- Improved error handling and validation
- Enhanced security with path traversal protection

### Coverage and Quality
- **fileutil**: 89% test coverage
- **pathutil**: 83% test coverage  
- **browser**: 77% test coverage
- **security**: 87% test coverage
- **procutil**: 81% test coverage
- **shellutil**: 85% test coverage
- All packages fully documented with comprehensive examples
- CI/CD with automated testing and coverage reporting via codecov

### Documentation
- Comprehensive README with installation instructions
- API documentation available at pkg.go.dev
- Usage examples for all packages
- Contributing guidelines (CONTRIBUTING.md)
- Security policy (SECURITY.md)
- Code of Conduct (CODE_OF_CONDUCT.md)

### Total Impact
- **6 core utility packages** published and production-ready
- **399 lines of code** removed from dependent projects (azd-exec + azd-app)
- **1 critical bug** fixed in azd-app config handling
- **77-89% test coverage** across all packages
- **Zero breaking changes** - fully backward compatible

---

## Links
- **Repository**: https://github.com/jongio/azd-core
- **Documentation**: https://pkg.go.dev/github.com/jongio/azd-core
- **Issues**: https://github.com/jongio/azd-core/issues
- **Contributing**: https://github.com/jongio/azd-core/blob/main/CONTRIBUTING.md
