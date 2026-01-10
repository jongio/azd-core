# Consolidation Tasks

<!-- NEXT: 12 -->

## TODO

### 13. Extract errors Package

Create standardized error types for azd ecosystem.

**Acceptance Criteria**:
- Error types defined: ValidationError, NotFoundError, ExecutionError
- Implements errors.Is/As compatibility (wrapping support)
- Tests for error construction, wrapping, unwrapping
- Documentation with usage examples
- ≥85% coverage

---

### 14. Extract testutil Package

Create shared testing utilities for azd ecosystem.

**Source**: azd-exec/cli/src/internal/testhelpers, azd-app test utilities

**Acceptance Criteria**:
- Functions migrated: CaptureOutput, FindTestData, TempDir helpers
- Common assertions and test helpers
- Tests for test utilities (≥80% coverage)
- Documentation with usage examples

---

### 15. Extract constants Package

Create shared constants for azd ecosystem.

**Source**: azd-app/cli/src/internal/constants (selective extraction)

**Acceptance Criteria**:
- File permissions constants: DirPermission, FilePermission
- Common timeouts: BrowserTimeout
- Only truly shared constants extracted (leave domain-specific in projects)
- Documentation explaining when to use vs define locally

---

### 16. Publish azd-core v0.2.0

Release azd-core v0.2.0 with all 9 packages.

**Status**: Ready after all packages complete

**What's in v0.2.0** (9 packages):
- Core utilities: fileutil, pathutil, browser, security, procutil (with gopsutil), shellutil
- Standardization: errors, testutil, constants
- azd-exec integration complete
- azd-app integration complete
- Full documentation and examples

**Acceptance Criteria**:
- ✅ 6 core utility packages complete and tested (77-89% coverage)
- ✅ Dependencies: github.com/shirou/gopsutil/v4 v4.24.12
- ✅ azd-exec integrated and tested
- ⏳ azd-app integrated and tested
- ⏳ errors package complete (≥85% coverage)
- ⏳ testutil package complete (≥80% coverage)
- ⏳ constants package complete
- ⏳ CHANGELOG.md created
- ⏳ Git tag created: v0.2.0
- ⏳ GitHub release published
- ⏳ pkg.go.dev updated automatically

---

## IN PROGRESS

### 12. Integrate azd-app with azd-core v0.2.0

Analyze and integrate azd-core utilities into azd-app where beneficial.

**Status**: Analyzing azd-app codebase

**Note**: azd-app doesn't have exact duplicates of azd-core packages. This task identifies opportunities to improve code quality/security using azd-core utilities.

**Acceptance Criteria**:
- ✅ go.work linking local azd-core for testing
- ⏳ Code analysis complete (identify integration points)
- ⏳ azd-core imports added where beneficial
- ⏳ Tests updated/added for integrations
- ⏳ 100% test pass rate (no regressions)
- ⏳ Integration improves code quality/security

---

## DONE

### 11. Integrate azd-exec with azd-core v0.2.0 ✓

Successfully integrated azd-exec to use azd-core/shellutil.

**Completed**:
- ✅ Replaced internal shell_detection.go with azd-core/shellutil
- ✅ Updated executor.go to use shellutil.DetectShell
- ✅ Updated tests to use shellutil.ReadShebang  
- ✅ Removed 349 lines of duplicate code (shell_detection.go, detect_shell_test.go)
- ✅ All tests passing (executor, version, testhelpers)
- ✅ azd-exec builds successfully

**Files Modified**:
- executor/executor.go: Added shellutil import, replaced detectShell method
- executor/constants.go: Shell constants now reference shellutil constants
- executor/command_shell_test.go: Updated to use shellutil.ReadShebang

**Files Deleted**:
- shell_detection.go (162 lines)
- detect_shell_test.go (187 lines)

---

### 10. Create azd-core v0.2.0 Packages ✓

Created 6 new utility packages in azd-core.

**Completed**:
- ✅ fileutil: Atomic file operations, JSON handling (77.7% coverage)
- ✅ pathutil: PATH management, tool discovery (78.3% coverage)
- ✅ browser: Cross-platform browser launching (81.8% coverage)
- ✅ security: Path validation, input sanitization (80% coverage)
- ✅ procutil: Process detection using gopsutil (88.9% coverage)
- ✅ shellutil: Shell detection from extension/shebang (86.1% coverage)
- ✅ Full documentation (README.md + doc.go for each package)
- ✅ All tests passing
- ✅ go.work configured for local testing

---

### 9. Add Package Documentation and Examples ✓

Updated README.md with all 6 new packages. Each package has comprehensive doc.go with usage examples and security considerations.

### 8. Extract shellutil Package ✓

Migrated shell detection utilities from azd-exec. Tests passing with 86.1% coverage on Windows (>90% combined cross-platform). Supports extension detection (.ps1, .sh, .cmd, .bat, .zsh), shebang parsing (#!/bin/bash, #!/usr/bin/env python3), and OS-specific defaults.

### 7. Extract procutil Package ✓

Migrated process utilities from azd-app using gopsutil v4.24.12. Tests passing with 88.9% coverage. Eliminated Windows stale PID limitation by using native platform APIs (Windows: OpenProcess, Linux: /proc, macOS: sysctl).

### 6. Extract security Package ✓

Extracted as dependency for fileutil in task 3. Complete with 80% coverage. Provides path validation, service name validation, input sanitization, and permission checks.

### 5. Extract browser Package ✓

Migrated browser launching utilities from azd-app. Tests passing with 81.8% coverage. Cross-platform support for Windows/macOS/Linux with URL validation and timeout.

### 4. Extract pathutil Package ✓

Migrated PATH management utilities from azd-app. Tests passing with 78.3% coverage (88% platform-specific). Installation suggestions for 22+ tools including npm, python, docker, azd.

### 3. Extract fileutil Package ✓

Migrated file system utilities from azd-app. Tests passing with 77.7% coverage. Atomic writes, JSON handling, secure file operations with security.ValidatePath integration.

### 2. Setup azd-core Package Structure (Phase 1) ✓

Created 6 package directories (fileutil, pathutil, browser, security, procutil, shellutil) with scaffolding files (package.go, package_test.go, doc.go). All compile successfully.

### 1. Review and Finalize Spec ✓

Spec created at docs/specs/consolidation/spec.md with Priority 1 packages confirmed: fileutil, pathutil, browser, security, procutil, shellutil. Ready for implementation.
