# Consolidation Tasks

<!-- NEXT: 16 -->

## TODO

### 16. Publish azd-core v0.2.0

**Note**: Ready to publish after proper PR/merge workflow.

Release azd-core v0.2.0 with 6 core utility packages and full integration.

**What's in v0.2.0** (6 packages):
- Core utilities: fileutil, pathutil, browser, security, procutil (with gopsutil), shellutil
- All packages tested (77-89% coverage)
- azd-exec: Uses shellutil for shell detection
- azd-app: Uses fileutil (8 files), security (20+ files), pathutil (2 files), browser, procutil
- Full documentation and examples

**Acceptance Criteria**:
- ✅ 6 core utility packages complete and tested (77-89% coverage)
- ✅ Dependencies: github.com/shirou/gopsutil/v4 v4.24.12
- ✅ azd-exec integrated with shellutil
- ✅ azd-app integrated with fileutil, pathutil, security
- ⏳ Create PR from azd-core-int-2 → main  
- ⏳ Merge PR to main
- ⏳ CHANGELOG.md created
- ⏳ Git tag v0.2.0 created from main (not feature branch)
- ⏳ GitHub release published
- ⏳ pkg.go.dev updated automatically

**Note**: Standardization packages (errors, testutil, constants) deferred to Phase 4/v0.3.0 per spec.

---

## IN PROGRESS

---

## DONE

### 18. Enhance azd-app with azd-core/pathutil ✓

Enhanced azd-app tool detection and installation suggestions.

**Completed**:
- ✅ root.go: Enhanced azd.exe finding with pathutil.FindToolInPath + SearchInSystemDirs
- ✅ installer.go: Replaced custom installation suggestions with pathutil.GetInstallationSuggestion  
- ✅ Updated 6 tests to match new URL-based suggestion format
- ✅ All tests passing (installer: 30.023s, notify: 1.054s)
- ✅ Eliminated ~20 lines of duplicate code

**Impact**: More robust tool detection, consistent installation URLs (22+ tools), better UX

---

### 17. Integrate azd-app with azd-core/fileutil ✓

Integrated azd-app with azd-core/fileutil for atomic file operations.

**Completed**:
- ✅ 8 files now use fileutil.AtomicWriteJSON/AtomicWriteFile
- ✅ config.go: Fixed critical non-atomic write bug (config corruption vulnerability)
- ✅ reqs_cache.go, notifications.go: Replaced custom atomic write patterns
- ✅ detector.go: Wrapper functions using fileutil with path validation
- ✅ All tests passing
- ✅ ~50 lines of duplicate code eliminated

**Files Using fileutil**:
- config.go, reqs_cache.go, notifications.go, detector.go (4 core files)
- Plus 4 other files across azd-app codebase

---

### 12. Complete azd-app Integration Analysis ✓

Analyzed azd-app codebase to determine integration opportunities with azd-core utilities.

**Analysis Complete**: See [azd-app-integration-analysis.md](./azd-app-integration-analysis.md)

**Key Findings**:
- ✅ azd-core/security already extensively used (21+ files)
- ✅ fileutil integration opportunity: Replace 3 atomic write patterns + fix 1 critical bug
- ✅ pathutil: Low priority, optional UX enhancement
- ❌ procutil: Not needed (container-focused architecture)
- ❌ browser: Not currently implemented (uses VS Code Simple Browser)
- ❌ shellutil: Not needed (uses azd-exec extension)

**Decision**: Proceed with targeted fileutil integration

**Implementation Plan**:
1. Replace atomic write patterns in cache/reqs_cache.go, config/notifications.go, config/config.go
2. Fix critical non-atomic write bug in config/config.go
3. Replace custom file helpers in service/detector.go with fileutil (adds path validation)
4. Total impact: 4 files, -50 lines, improved reliability + security

**Completed**:
- ✅ go.work linking local azd-core for testing
- ✅ Code analysis complete (documented in azd-app-integration-analysis.md)
- ✅ Integration decision made: Proceed with fileutil integration after v0.2.0 release

---

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


---

## DEFERRED (After All Integration Complete)

### 16. Publish azd-core v0.2.0

**Note**: This task runs LAST, after all integration work is complete.

Release azd-core v0.2.0 with 6 core utility packages.

**What's in v0.2.0** (6 packages):
- Core utilities: fileutil, pathutil, browser, security, procutil (with gopsutil), shellutil
- All packages tested (77-89% coverage)
- azd-exec integration complete (uses shellutil)
- azd-app integration complete (uses fileutil)
- Full documentation and examples

**Acceptance Criteria**:
- ✅ 6 core utility packages complete and tested (77-89% coverage)
- ✅ Dependencies: github.com/shirou/gopsutil/v4 v4.24.12
- ✅ azd-exec integrated and tested
- ✅ azd-app integration complete
- ⏳ CHANGELOG.md created
- ⏳ Git tag created: v0.2.0
- ⏳ GitHub release published
- ⏳ pkg.go.dev updated automatically

**Note**: Standardization packages (errors, testutil, constants) deferred to Phase 4/v0.3.0 per spec.