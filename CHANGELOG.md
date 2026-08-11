## [0.6.0] - 2026-08-11

- feat: align azd-core with the azdext SDK for v0.6.0 (#160) (4c89ed9)
- docs: point auth guidance at azd credentials instead of az CLI (#159) (366931f)
- deps: update dependencies to latest (#154) (d68422a)
- deps: update Go dependencies (#153) (f066ee7)
- deps: update dependencies to latest (#151) (398dbde)
- deps: update all dependencies to latest versions (#150) (4fbc98f)
- deps: upgrade all dependencies to latest (#144) (4ed2b13)
- deps: upgrade all dependencies to latest (#143) (4a6521f)
- deps: upgrade all dependencies to latest (#137) (08ff448)
- deps: upgrade all dependencies to latest (#133) (d0589fe)
- deps: upgrade all dependencies to latest (#131) (d55b350)
- deps: upgrade all dependencies to latest (#127) (7aaa990)
- deps: update all dependencies to latest (#126) (af5c10a)
- chore: satisfy lint after dependency upgrades (#125) (f4726d4)
- deps: upgrade all dependencies to latest (#124) (4aa0c68)
- deps: bump Go version badge to 1.26.4 (#123) (a58ddbc)
- deps: update all dependencies to latest (#122) (4355723)
- chore(deps): bump softprops/action-gh-release from 2.5.0 to 3.0.1 (#121) (f7280b8)
- chore(deps): bump actions/setup-go from 6.3.0 to 6.5.0 (#120) (9cd1f2e)
- ci: remove duplicate concurrency key in govulncheck workflow (f2597c1)
- ci(deps): bump checkout to v7.0.0, codecov-action to v7.0.0, golangci-lint-action to v9.2.1 (882a2c6)
- deps: upgrade all dependencies to latest (#119) (c20c941)
- fix: consolidate pagination limits, credential logging, code decomposition, and architecture improvements (#108) (bbd6c41)
- perf: fix mutex contention, double deserialization, allocation, and goroutine leak (#105) (df8d571)
- fix: extract constants, add package docs, and replace flaky time.Sleep in tests (#101) (7deea1d)
- fix(ci): harden CodeQL analysis and update release workflow actions (#98) (d70c185)
- test: improve coverage for cmdutil, cliout, registry, fileutil, keyvault and fix test races (#102) (c6eb9b7)
- refactor: modernize error handling and concurrency patterns (#100) (07c3c35)
- fix: address preflight lint findings (gosec, goconst, stale nolint) (#99) (5b5a683)
- fix: retain healthcheck panic diagnostics (#97) (a410150)
- refactor: replace deprecated os.IsNotExist and interface{} with modern Go idioms (#61) (4787efa)
- fix(deps): update outdated Go dependencies (#50) (68bf4c7)

## [0.5.7] - 2026-03-16

- chore(deps): bump golang.org/x/time from 0.14.0 to 0.15.0 (#33) (f984c86)
- feat: dispatch-parity quality improvements (#34) (0880335)

## [0.5.6] - 2026-03-12

- ci: optimize GitHub Actions workflows (#30) (4e95aa0)

## [0.5.5] - 2026-03-03

- fix: replace DefaultAzureCredential with resilient credential chain (#28) (22fcaea)

## [0.5.3] - 2026-02-28

- chore: deprecate azdextutil helpers in favor of azdext SDK helpers (#22) (02cc93a)

## [0.5.2] - 2026-02-23

- feat: add azdextutil shared library for azd extension framework (#20) (39a75ce)
- fix: use gh api for release dispatch (fixes fine-grained PAT 403) (6a09e7a)

## [0.5.1] - 2026-02-22

- refactor: extract shared packages for azd extensions (#18) (dd4e15d)

## [0.5.0] - 2026-02-18

- chore: bump version to 0.4.2 (#16) (8afab35)
- fix: handle nil cases in TestGetClient_CachingBehavior (dcdf451)
- fix: correct indentation for Go version in CodeQL workflow (100df81)
- chore: bump version to 0.4.4 (#15) (32f4c78)
- chore: bump version to 0.4.3 (#14) (61a99e8)
- fix: handle missing EXTENSIONS_DISPATCH_TOKEN in release workflow (dab40e5)
- chore: Go 1.26.0, copilot skills, and release coordination (#13) (500ee97)
- chore: bump version to 0.4.2 (#12) (7c275ce)
- fix: update release workflow to use bump type (patch/minor/major) (14c29a6)

## [0.4.2] - 2026-02-18

- fix: handle nil cases in TestGetClient_CachingBehavior (dcdf451)
- fix: correct indentation for Go version in CodeQL workflow (100df81)
- chore: bump version to 0.4.4 (#15) (32f4c78)
- chore: bump version to 0.4.3 (#14) (61a99e8)
- fix: handle missing EXTENSIONS_DISPATCH_TOKEN in release workflow (dab40e5)
- chore: Go 1.26.0, copilot skills, and release coordination (#13) (500ee97)
- chore: bump version to 0.4.2 (#12) (7c275ce)
- fix: update release workflow to use bump type (patch/minor/major) (14c29a6)

## [0.4.4] - 2026-02-17



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
