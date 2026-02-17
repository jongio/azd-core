## [0.4.3] - 2026-02-17

- chore: Go 1.26.0, copilot skills, and release coordination (#13) (9713d31)

## [0.4.2] - 2026-02-13

- fix: update release workflow to use bump type (patch/minor/major) (9310829)
- Implement mTLS-based authentication server for Azure credentials (#11) (14570d7)

# Changelog

All notable changes to azd-core will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

#### New URL Validation Package
- **urlutil**: RFC-compliant HTTP/HTTPS URL validation and parsing
  - `Validate(rawURL)` - Comprehensive URL validation using `net/url.Parse`
  - `ValidateHTTPSOnly(rawURL)` - Enforce HTTPS-only for production (allows localhost HTTP)
  - `Parse(rawURL)` - Parse and normalize URLs with validation
  - `NormalizeScheme(rawURL, defaultScheme)` - Ensure URL has proper protocol prefix
  - **Validation Rules**:
    - Protocol must be http:// or https:// (rejects ftp://, file://, javascript://, etc.)
    - URL must have valid host/domain (rejects "http://", "https://")
    - URL length limit of 2048 characters (RFC 2616 standard)
    - Whitespace trimming before validation
  - **Security Features**:
    - Prevents protocol injection attacks
    - Host presence validation
    - DoS prevention via length limits
    - HTTPS enforcement with localhost exception
  - **Test Coverage**: 97.1% (60+ test cases)

#### Environment Package Extensions
- **env**: Pattern-based environment variable extraction helpers
  - `FilterByPrefix(envVars, prefix)` - Filter environment variables by prefix (case-insensitive)
  - `FilterByPrefixSlice(envSlice, prefix)` - Filter KEY=VALUE slices by prefix
  - `ExtractPattern(envVars, opts)` - Extract with prefix/suffix matching and key transformation
  - `PatternOptions` struct for configurable extraction:
    - Prefix/suffix matching (case-insensitive)
    - Optional prefix/suffix trimming
    - Custom key transformation functions
    - Value validation callbacks
  - `NormalizeServiceName(envVarName)` - Convert env var naming to service naming (MY_API → my-api)
  - **Use Cases**:
    - Extract all AZURE_* environment variables
    - Extract SERVICE_*_URL variables for service discovery
    - Extract SERVICE_*_CUSTOM_DOMAIN with service name normalization
    - Filter environment for specific contexts
  - **Test Coverage**: 100.0% (40+ test cases)

### Quality Metrics
- **urlutil**: 97.1% coverage (60 tests, 170 lines)
- **env extensions**: 100.0% coverage (40 tests, 150 lines)
- **Combined**: 100+ test cases ensuring reliability
- All tests pass with zero regressions

### Benefits
- **Code Reduction**: Enables removal of 200-310 lines of duplicated code from azd-app and azd-exec
- **Standardization**: Unified URL validation and environment parsing patterns
- **Quality**: Battle-tested helpers with extensive edge case coverage
- **Security**: Prevents protocol injection, validates hosts, enforces HTTPS for production

### Documentation
- Comprehensive README updates with urlutil section
- Enhanced env package documentation with pattern extraction examples
- Full godoc for all public functions
- Security considerations documented

---

## [0.2.0] - 2026-01-10

### Added

#### New Test Infrastructure Package
- **testutil**: Common testing utilities for CLI testing and test fixture management
  - `CaptureOutput(t, fn)` - Capture stdout during function execution for testing CLI commands
  - `FindTestData(t, subdirs...)` - Locate test fixture directories with flexible path searching
  - `TempDir(t)` - Create temporary directories with automatic cleanup via t.Cleanup()
  - `Contains(s, substr)` - Convenience helper for string containment checks
  - Proper test line reporting via `t.Helper()` in all functions
  - Cross-platform path handling (Windows and Unix)
  - Thread-safe for parallel test execution
  - **Test Coverage**: 83.3% (38 test cases)

#### New CLI Output Package
- **cliout**: Structured CLI output formatting with cross-platform terminal support
  - **Status Messages**: `Success`, `Error`, `Warning`, `Info` with colored icons
  - **Section Headers**: `Header`, `Section` with visual separators
  - **Lists**: `Bullet`, `Label` for structured content
  - **Tables**: `Table` with automatic column width calculation
  - **Progress**: `ProgressBar` for visual progress indicators
  - **Interactive**: `Confirm` for yes/no prompts (CI-friendly in JSON mode)
  - **Hybrid Output**: `Print`, `PrintJSON` for format-agnostic output
  - **Format Management**: `SetFormat`, `GetFormat`, `IsJSON` for output control
  - **Orchestration Mode**: `SetOrchestrated` to skip headers in composed workflows
  - Cross-platform Unicode detection (Windows Terminal, VS Code, PowerShell, ConEmu)
  - ASCII fallback for legacy terminals
  - Consistent color scheme (green success, red error, yellow warning, blue info)
  - Non-interactive mode for CI/CD pipelines
  - Thread-safe for concurrent use
  - **Test Coverage**: 94.9% (43 test cases)

#### New Documentation
- **Extension Patterns Guide**: Comprehensive guide for building azd extensions (1,056 lines)
  - Version management pattern with ldflags injection
  - Logging pattern with structured logging (log/slog)
  - Extension structure best practices
  - Testing patterns using testutil
  - CLI output patterns using cliout
  - Error handling patterns
  - 26 code examples from azd-exec and azd-app

### Integration Benefits

#### azd-exec Integration (Complete)
- **testutil migration**: Removed internal testhelpers package (100 lines deleted)
  - Migrated all tests to azd-core/testutil
  - Enhanced test reliability with standardized helpers
  - All 65 tests pass with zero regressions
- **cliout integration**: Enhanced CLI output formatting
  - Version command with formatted output and JSON mode
  - Colored info messages for listen command
  - Improved Key Vault warnings
  - Enhanced error messages
  - Backward compatible, all tests pass
- **Total Impact**: ~100 lines removed, enhanced CLI UX

#### azd-app Integration (Complete)
- **testutil adoption**: Enhanced test infrastructure
  - Added CaptureOutput for CLI command testing
  - Enhanced logs tests with Contains (13 assertions)
  - Created version command tests
  - 5 new tests added, all 30+ tests pass
- **cliout migration**: Migrated from internal/output to azd-core/cliout
  - 30 files migrated to use azd-core/cliout
  - Reduced internal/output to thin wrapper (125 lines) + progress tracking
  - Deleted output_test.go (tests now in azd-core)
  - All 35 tests pass, CLI output identical to pre-migration
- **Total Impact**: ~550 lines reduced, zero breaking changes

### Quality Metrics
- **testutil**: 83.3% coverage (38 tests, 162 lines)
- **cliout**: 94.9% coverage (43 tests, 464 lines)
- **Combined**: 81 test cases ensuring reliability
- **Integration testing**: All azd-exec (65 tests) and azd-app (35 tests) pass

### Total Impact
- **Code Reduction**: ~650 lines eliminated across azd-exec and azd-app
- **Standardization**: Unified testing and CLI output patterns
- **Quality**: Battle-tested helpers with extensive edge case coverage
- **Developer Experience**: Faster testing, professional output, better documentation

### Documentation
- Extension Patterns Guide published in docs/extension-patterns.md
- README updated with links to patterns guide
- Comprehensive API documentation for testutil and cliout
- Migration guide in release notes

---

## [0.1.0] - Initial Release

### Added
- Initial project setup
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
