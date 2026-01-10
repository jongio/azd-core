# Consolidation Tasks

<!-- NEXT: 12 -->

## TODO

### 16. Publish azd-core v0.2.0

Release azd-core v0.2.0 with 6 core utility packages.

**What's in v0.2.0** (6 packages):
- Core utilities: fileutil, pathutil, browser, security, procutil (with gopsutil), shellutil
- All packages tested (77-89% coverage)
- azd-exec integration complete (uses shellutil)
- Full documentation and examples

**Acceptance Criteria**:
- ✅ 6 core utility packages complete and tested (77-89% coverage)
- ✅ Dependencies: github.com/shirou/gopsutil/v4 v4.24.12
- ✅ azd-exec integrated and tested
- ⏳ azd-app analysis complete
- ⏳ CHANGELOG.md created
- ⏳ Git tag created: v0.2.0
- ⏳ GitHub release published
- ⏳ pkg.go.dev updated automatically

**Note**: Standardization packages (errors, testutil, constants) deferred to Phase 4/v0.3.0 per spec.

---

## IN PROGRESS

### 12. Complete azd-app Integration Analysis

Analyze azd-app codebase to determine integration opportunities with azd-core utilities.

**Context**: azd-app doesn't have exact duplicates of azd-core packages. This task identifies where azd-core utilities can improve code quality, security, or reduce custom implementations.

**Analysis Approach**:
1. Review azd-app internal packages (constants, cache, docker, output, yamlutil)
2. Identify opportunities to use azd-core/security for path validation
3. Identify opportunities to use azd-core/procutil for process management
4. Identify opportunities to use azd-core/fileutil for atomic operations
5. Document findings and create follow-up tasks if beneficial integrations found

**Acceptance Criteria**:
- ✅ go.work linking local azd-core for testing
- ⏳ Code analysis complete (document findings)
- ⏳ Integration decision made (proceed with imports OR document why not needed)
- ⏳ If proceeding: azd-core imports added, tests pass, no regressions
- ⏳ If not proceeding: document rationale and close task

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
