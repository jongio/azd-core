# Consolidation Tasks

<!-- NEXT: 10 -->

## TODO

### 10. Publish azd-core v0.2.0

Release azd-core with new packages.

**Acceptance Criteria**:
- All Phase 1 packages complete and tested (≥85% coverage each)
- CHANGELOG.md updated with new packages
- Git tag created: v0.2.0
- GitHub release published with release notes
- pkg.go.dev updated automatically

---

### 11. Migrate azd-app to azd-core v0.2.0

Update azd-app to use new azd-core packages.

**Acceptance Criteria**:
- go.mod updated to azd-core v0.2.0
- Internal packages replaced with azd-core imports: fileutil, pathutil, browser, security, procutil
- All imports updated across codebase
- Tests updated to reference new package paths
- 100% test pass rate (no regressions)
- Integration tests verify no behavior changes
- Old internal packages marked as deprecated (or removed if safe)

---

### 12. Migrate azd-exec to azd-core v0.2.0

Update azd-exec to use new azd-core packages.

**Acceptance Criteria**:
- go.mod updated to azd-core v0.2.0
- Internal shellutil replaced with azd-core/shellutil
- New utilities adopted: fileutil for config management, security for path validation
- All imports updated
- Tests updated to reference new package paths
- 100% test pass rate (no regressions)
- Enhanced security validation using security package

---

### 13. Extract errors Package (Phase 4)

Create standardized error types for azd ecosystem.

**Acceptance Criteria**:
- Error types defined: ValidationError, NotFoundError, ExecutionError
- Implements errors.Is/As compatibility (wrapping support)
- Tests for error construction, wrapping, unwrapping
- Documentation with usage examples
- ≥85% coverage

---

### 14. Extract testutil Package (Phase 4)

Create shared testing utilities for azd ecosystem.

**Source**: azd-exec/cli/src/internal/testhelpers, azd-app test utilities

**Acceptance Criteria**:
- Functions migrated: CaptureOutput, FindTestData, TempDir helpers
- Common assertions and test helpers
- Tests for test utilities (≥80% coverage)
- Documentation with usage examples

---

### 15. Extract constants Package (Phase 4)

Create shared constants for azd ecosystem.

**Source**: azd-app/cli/src/internal/constants (selective extraction)

**Acceptance Criteria**:
- File permissions constants: DirPermission, FilePermission
- Common timeouts: BrowserTimeout
- Only truly shared constants extracted (leave domain-specific in projects)
- Documentation explaining when to use vs define locally

---

### 16. Publish azd-core v0.3.0 (Phase 4 Complete)

Release azd-core with standardization packages.

**Acceptance Criteria**:
- errors, testutil, constants packages complete
- CHANGELOG.md updated
- Git tag created: v0.3.0
- GitHub release published
- azd-app and azd-exec updated to use v0.3.0

---

## IN PROGRESS

<!-- Tasks currently being worked on -->

---

## DONE

### 9. Add Package Documentation and Examples ✓

Updated README.md with all 6 new packages. Each package has comprehensive doc.go with usage examples and security considerations.

### 8. Extract shellutil Package ✓

Migrated shell detection utilities from azd-exec. Tests passing with 86.1% coverage on Windows (>90% combined cross-platform). Supports extension detection (.ps1, .sh, .cmd, .bat, .zsh), shebang parsing (#!/bin/bash, #!/usr/bin/env python3), and OS-specific defaults.

### 7. Extract procutil Package ✓

Migrated process utilities from azd-app. Tests passing with 84.6% coverage. Clean architecture with build tags for Windows/Unix. Documented Windows limitation for stale PID detection.

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
