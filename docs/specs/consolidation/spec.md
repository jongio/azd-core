---
title: azd-core Package Consolidation
status: in-progress
created: 2026-01-09
updated: 2026-01-09
epic: consolidation
priority: P1
version: v0.2.0
---

# azd-core Package Consolidation Spec

## Status Update (2026-01-10)

**Current Phase**: v0.2.0 Ready for Publication

**Completed**:
- ✅ 6 core utility packages created and tested (77-89% coverage)
- ✅ azd-exec integrated with azd-core/shellutil (349 lines duplicate code removed)
- ✅ gopsutil v4.24.12 adopted for reliable cross-platform process detection
- ✅ go.work configured for local development/testing
- ✅ Documentation complete (README.md + doc.go per package)

**In Progress**:
- ⏳ azd-app integration analysis (determining if beneficial)

**Blocking v0.2.0 Publication**:
1. Complete azd-app integration analysis
2. Create CHANGELOG.md
3. Git tag and GitHub release

**v0.2.0 Scope** (6 packages):
- fileutil, pathutil, browser, security, procutil, shellutil

**Deferred to v0.3.0** (Phase 4):
- errors, testutil, constants packages (standardization phase per migration plan)

---

## Summary

Extract common utilities from azd-app and azd-exec into azd-core to reduce code duplication, improve maintainability, and provide standardized utilities for the azd extension ecosystem.

## Background

Currently, azd-app and azd-exec both depend on azd-core (which provides `env` and `keyvault` packages), but each project maintains significant duplicated utility code for:
- File/path operations
- Security validation  
- Browser launching
- Process utilities
- Shell detection
- Testing infrastructure
- Output formatting
- Error handling

This duplication leads to:
- Maintenance burden (bug fixes must be applied to multiple codebases)
- Inconsistent behavior across extensions
- Missed optimization opportunities
- Higher barrier to creating new azd extensions

## Goals

1. **Reduce Duplication**: Consolidate common code into azd-core
2. **Standardize Behavior**: Ensure consistent patterns across azd extensions
3. **Enable Extension Development**: Provide reusable building blocks for new extensions
4. **Maintain Backward Compatibility**: Existing functionality in azd-app and azd-exec must continue working
5. **Improve Test Coverage**: Centralize testing of common utilities

## Non-Goals

- Extracting azd-app-specific logic (dashboard, orchestrator, MCP server)
- Extracting azd-exec-specific logic (command execution flow)
- Breaking changes to existing azd-app or azd-exec APIs
- Creating a monolithic "kitchen sink" library

## Packages to Extract

### Priority 1: High-Value, General Purpose

#### 1. `fileutil` - File System Utilities

**Source**: azd-app/cli/src/internal/fileutil

**Capabilities**:
- Atomic file writes (JSON and raw bytes) with retry logic
- JSON read/write with graceful handling of missing files
- Directory creation with secure permissions
- File existence checks (single, any, all patterns)
- File extension detection
- Text containment checks with security validation

**Why Extract**:
- Used by both azd-app and azd-exec (azd-exec currently lacks atomic write patterns)
- Critical for safe configuration file management
- Prevents race conditions in concurrent environments
- Security-conscious path handling

**Test Coverage Required**: ≥90% (including error paths, race conditions, permission scenarios)

---

#### 2. `pathutil` - PATH Management Utilities

**Source**: azd-app/cli/src/internal/pathutil

**Capabilities**:
- Refresh PATH from system (Windows registry, Unix profiles)
- Find tools in PATH with cross-platform executable detection
- Search common system directories for missing tools
- Installation suggestions for missing dependencies

**Why Extract**:
- Common pattern for extensions that detect/install dependencies
- Cross-platform complexity benefits from centralization
- azd-exec could use for shell detection fallbacks

**Test Coverage Required**: ≥85% (cross-platform scenarios)

---

#### 3. `browser` - Cross-Platform Browser Launching

**Source**: azd-app/cli/src/internal/browser

**Capabilities**:
- Launch URLs in system default browser
- Cross-platform command construction (Windows/macOS/Linux)
- Non-blocking launch with timeout
- Target options (default, system, none)
- URL validation (http/https only)

**Why Extract**:
- Common pattern for extensions with web UIs
- Security-conscious (validates URLs, prevents command injection)
- Well-tested cross-platform implementation

**Test Coverage Required**: ≥80% (cross-platform scenarios)

---

#### 4. `security` - Security Validation Utilities

**Source**: azd-app/cli/src/internal/security

**Capabilities**:
- Path validation (traversal attack prevention)
- Symbolic link resolution and validation
- Service name validation (DNS-safe, container-safe)
- Package manager name validation
- Script name sanitization (shell metacharacter detection)
- Container environment detection
- File permission validation (world-writable detection)

**Why Extract**:
- Security critical - centralization ensures consistent protection
- Both azd-app and azd-exec handle user-provided paths and names
- Prevents SSRF, path traversal, command injection attacks

**Test Coverage Required**: ≥95% (security-critical code requires extensive testing)

---

#### 5. `procutil` - Process Utilities

**Source**: azd-app/cli/src/internal/procutil

**Capabilities**:
- Cross-platform process running check
- Handles Windows vs Unix differences in Signal(0)
- Documented limitations (Windows stale PID caveat)

**Why Extract**:
- Common pattern for extensions managing child processes
- Cross-platform complexity benefits from centralization
- azd-app uses for service lifecycle management

**Test Coverage Required**: ≥85% (cross-platform scenarios, edge cases)

---

#### 6. `shellutil` - Shell Detection & Command Building

**Source**: azd-exec/cli/src/internal/executor (shell_detection.go, constants.go)

**Capabilities**:
- Detect shell from file extension (.ps1, .sh, .cmd, .zsh)
- Parse shebang lines (#!/bin/bash, #!/usr/bin/env python3)
- OS-specific default shell detection
- Shell identifier constants (bash, pwsh, cmd, zsh, sh)

**Why Extract**:
- Core capability for any extension executing scripts
- azd-app executor could use for consistency
- Well-tested cross-platform implementation

**Test Coverage Required**: ≥90% (shebang parsing, extension detection, OS defaults)

---

### Priority 2: Standardization Opportunities

#### 7. `errors` - Structured Error Types

**Source**: Multiple packages in azd-app and azd-exec

**Capabilities**:
- Validation errors with context
- Not found errors (scripts, configs, tools)
- Execution errors with exit codes
- Wrapping support (errors.Is/As compatible)

**Why Extract**:
- Improves error handling consistency
- Better error messages for users
- Enables programmatic error detection (errors.Is checks)

**Example Error Types**:
```go
type ValidationError struct {
    Field   string
    Value   string
    Message string
    Cause   error
}

type NotFoundError struct {
    Resource string
    Path     string
}

type ExecutionError struct {
    Command  string
    ExitCode int
    Output   string
    Cause    error
}
```

**Test Coverage Required**: ≥85%

---

#### 8. `testutil` - Testing Utilities

**Source**: azd-exec/cli/src/internal/testhelpers, azd-app test utilities

**Capabilities**:
- Output capture (stdout/stderr redirection)
- Test resource discovery (finding test data files)
- Temporary directory management
- Common test assertions and helpers

**Why Extract**:
- Reduces test infrastructure duplication
- Consistent testing patterns across extensions
- Makes extension development easier

**Test Coverage Required**: ≥80%

---

#### 9. `constants` - Shared Constants

**Source**: azd-app/cli/src/internal/constants

**Capabilities**:
- File permissions (DirPermission, FilePermission)
- Common timeouts (BrowserTimeout, etc.)
- Buffer sizes
- Status/state constants

**Why Extract**:
- Prevents magic numbers
- Ensures consistent behavior
- Single source of truth for shared values

**Rationale for Selection**:
- Only extract **truly shared** constants
- Leave domain-specific constants in their packages
- Focus on file system, security, and timeout values

**Test Coverage Required**: N/A (constants)

---

### Priority 3: Consider for Future Extraction

These packages are valuable but either:
- Too specialized for azd-core (better suited for a separate shared library)
- Need further evaluation for general-purpose use
- May have overlap with existing Go ecosystem libraries

#### 10. `output` - CLI Output Formatting *(Consider)*

**Source**: azd-app/cli/src/internal/output

**Rationale for Deferral**:
- azd-exec has minimal output needs currently
- Could use existing CLI libraries (e.g., github.com/fatih/color)
- Evaluate if azd-core should own UI concerns vs domain logic

---

#### 11. `yamlutil` - YAML Manipulation *(Consider)*

**Source**: azd-app/cli/src/internal/yamlutil

**Rationale for Deferral**:
- Highly specialized for azure.yaml editing
- May not be needed by all extensions
- Consider separate azd-extensions-common library if multiple extensions need this

---

#### 12. `docker` - Docker Client Abstraction *(Consider)*

**Source**: azd-app/cli/src/internal/docker

**Rationale for Deferral**:
- Container-specific, not needed by azd-exec
- Consider separate library or keep in azd-app
- Evaluate broader container orchestration needs

---

#### 13. `cache` - Cache Management *(Consider)*

**Source**: azd-app/cli/src/internal/cache

**Rationale for Deferral**:
- Currently only used by azd-app
- Evaluate if azd-exec has caching needs
- May be too specialized for azd-core

---

## Migration Plan

### Phase 1: Foundation (Priority 1 packages)

**Packages**: fileutil, pathutil, browser, security, procutil, shellutil

**Steps**:
1. Create package structure in azd-core
2. Copy source code with full git history (use `git log --follow`)
3. Add comprehensive tests (≥85% coverage for each package)
4. Add package-level documentation and examples
5. Publish azd-core v0.2.0

**Success Criteria**:
- All tests pass
- Coverage ≥85% per package
- Documentation complete
- No breaking changes to azd-core v0.1.x API

---

### Phase 2: Adoption in azd-app

**Steps**:
1. Update azd-app to depend on azd-core v0.2.0
2. Replace internal packages with azd-core imports
3. Update tests to reference new package paths
4. Verify 100% test pass rate
5. Verify no behavior changes (integration tests)

**Success Criteria**:
- All azd-app tests pass
- No regressions in functionality
- Reduced LOC in azd-app (internal packages removed)

---

### Phase 3: Adoption in azd-exec

**Steps**:
1. Update azd-exec to depend on azd-core v0.2.0
2. Replace internal packages with azd-core imports (shellutil)
3. Add new utilities from azd-core (fileutil, pathutil, security)
4. Update tests to reference new package paths
5. Verify 100% test pass rate

**Success Criteria**:
- All azd-exec tests pass
- Enhanced security validation using security package
- Reduced LOC in azd-exec

---

### Phase 4: Standardization (Priority 2 packages)

**Packages**: errors, testutil, constants

**Steps**:
1. Create package structure in azd-core
2. Define standardized error types
3. Migrate test utilities
4. Extract shared constants (selective, not all)
5. Publish azd-core v0.2.0
6. Update azd-app and azd-exec to use new packages

**Success Criteria**:
- Consistent error handling across extensions
- Shared test infrastructure reduces duplication
- Magic numbers eliminated

---

## Technical Design

### Package Structure

```
azd-core/
├── env/              (existing)
├── keyvault/         (existing)
├── fileutil/         (new)
│   ├── fileutil.go
│   ├── fileutil_test.go
│   └── doc.go
├── pathutil/         (new)
│   ├── pathutil.go
│   ├── pathutil_test.go
│   └── doc.go
├── browser/          (new)
│   ├── browser.go
│   ├── browser_test.go
│   └── doc.go
├── security/         (new)
│   ├── validation.go
│   ├── validation_test.go
│   ├── fuzz_test.go
│   └── doc.go
├── procutil/         (new)
│   ├── procutil.go
│   ├── procutil_test.go
│   └── doc.go
├── shellutil/        (new)
│   ├── detection.go
│   ├── detection_test.go
│   ├── constants.go
│   └── doc.go
├── errors/           (new, Phase 4)
│   ├── errors.go
│   └── errors_test.go
├── testutil/         (new, Phase 4)
│   ├── testutil.go
│   └── testutil_test.go
└── constants/        (new, Phase 4)
    └── constants.go
```

### Versioning Strategy

- **Current**: azd-core v0.1.0 (env, keyvault packages)
- **v0.2.0 (This Release)**: Comprehensive utility and standardization packages

**v0.2.0 Contents** (9 packages):

*Core Utilities:*
- fileutil: Atomic file operations, JSON handling
- pathutil: PATH management, tool discovery
- browser: Cross-platform browser launching
- security: Path validation, input sanitization
- procutil: Process detection (using gopsutil v4.24.12)
- shellutil: Shell detection from extension/shebang

*Standardization:*
- errors: Standardized error types (ValidationError, NotFoundError, ExecutionError)
- testutil: Shared testing utilities (CaptureOutput, FindTestData, TempDir helpers)
- constants: Shared constants (file permissions, timeouts)

**Integration Status**:
- azd-exec: ✅ Integrated (uses shellutil)
- azd-app: ⏳ In progress

Use semantic versioning:
- **MAJOR**: Breaking changes to existing packages
- **MINOR**: New packages added
- **PATCH**: Bug fixes, documentation updates

### Dependency Management

**Principle**: azd-core should have **minimal external dependencies**

**Current Dependencies** (v0.2.0):
- github.com/Azure/azure-sdk-for-go/sdk/azidentity
- github.com/Azure/azure-sdk-for-go/sdk/security/keyvault/azsecrets
- github.com/shirou/gopsutil/v4 v4.24.12 (for procutil only)

**Rationale for gopsutil**:
- Eliminates Windows stale PID limitation
- Uses native platform APIs (Windows: OpenProcess, Linux: /proc, macOS: sysctl)
- Production-grade, well-maintained library
- Acceptable ~1-2MB binary size increase for CLI tooling

**Prohibited**:
- Large framework dependencies (cobra, viper, etc.)
- OS-specific packages unless absolutely necessary
- Deprecated or unmaintained packages

**Rationale**:
- Keep azd-core lightweight
- Minimize dependency conflicts in consuming projects
- Reduce security surface area

---

## Testing Requirements

### Coverage Targets

- **Security-critical packages** (security): ≥95%
- **Core utilities** (fileutil, shellutil): ≥90%
- **Cross-platform packages** (pathutil, procutil, browser): ≥85%
- **Other packages**: ≥80%

### Test Types Required

1. **Unit Tests**: All exported functions
2. **Integration Tests**: File I/O, process execution
3. **Cross-Platform Tests**: Windows/Linux/macOS (via CI)
4. **Fuzz Tests**: Security-critical input validation (security package)
5. **Error Path Tests**: All error conditions covered

### CI Requirements

- Run on Linux, Windows, macOS
- Go versions: 1.25.5+ (match azd-app/azd-exec)
- Static analysis: golangci-lint, gosec
- Coverage reporting: codecov

---

## Documentation Requirements

Each package must include:

1. **Package-level doc.go**:
   - Purpose and scope
   - Usage examples
   - Security considerations (if applicable)

2. **Function/Type Comments**:
   - Clear description
   - Parameter documentation
   - Return value documentation
   - Error conditions
   - Example usage (for complex functions)

3. **README.md Update**:
   - Add new package to "Packages" section
   - Include usage examples
   - Link to pkg.go.dev documentation

---

## Security Considerations

### Path Validation (security package)

- **MUST** validate all user-provided paths before file operations
- **MUST** resolve symbolic links to prevent link-based attacks
- **MUST** detect and prevent path traversal (`..` sequences)
- **MUST** document path validation requirements in consuming packages

### Input Sanitization

- Service names: alphanumeric start, DNS-safe characters only
- Script names: no shell metacharacters
- Package managers: allowlist only

### File Permissions

- Directories: 0750 (rwxr-x---)
- Files: 0644 (rw-r--r--)
- Detect and warn on world-writable files

### Command Execution

- Shell detection must prevent command injection
- No user input directly interpolated into shell commands
- Validate all executables before running

---

## Risks & Mitigations

### Risk 1: Breaking Changes During Migration

**Impact**: azd-app or azd-exec functionality breaks

**Mitigation**:
- Maintain 100% test coverage during migration
- Run integration tests before and after
- Use feature flags for gradual rollout
- Keep internal packages during transition period (deprecate later)

### Risk 2: Version Skew (azd-app and azd-exec use different azd-core versions)

**Impact**: Inconsistent behavior across extensions

**Mitigation**:
- Document version compatibility in README
- Use go.work for local development
- CI checks for version mismatches
- Publish coordinated releases (azd-core → azd-app → azd-exec)

### Risk 3: Over-Extraction (Kitchen Sink Problem)

**Impact**: azd-core becomes bloated, hard to maintain

**Mitigation**:
- Strict inclusion criteria (must be used by ≥2 projects)
- Regular review of package usage
- Consider separate azd-extensions-common library for specialized utilities
- Keep domain logic in consuming projects

### Risk 4: Test Coverage Gaps

**Impact**: Bugs in common utilities affect all extensions

**Mitigation**:
- Mandatory coverage targets (≥85%)
- Fuzz testing for security-critical code
- Cross-platform CI testing
- Code review for all azd-core changes

---

## Success Metrics

### Quantitative

- **Code Reduction**: ≥30% LOC reduction in azd-app and azd-exec internal packages
- **Test Coverage**: ≥85% for azd-core
- **Build Time**: No significant increase (<10%)
- **Dependency Count**: azd-core maintains ≤5 external dependencies

### Qualitative

- **Developer Experience**: Easier to create new azd extensions
- **Consistency**: Common utilities behave identically across extensions
- **Maintainability**: Bug fixes applied once, benefit all extensions
- **Documentation**: Clear examples and API docs for all packages

---

## Timeline

### Phase 1: Foundation (4-6 weeks)

- Week 1-2: Extract fileutil, pathutil, browser
- Week 3-4: Extract security, procutil, shellutil
- Week 5: Testing, documentation, examples
- Week 6: azd-core v0.2.0 release

### Phase 2: azd-app Adoption (2-3 weeks)

- Week 1: Migration, testing
- Week 2: Integration testing, bug fixes
- Week 3: azd-app release with azd-core v0.2.0

### Phase 3: azd-exec Adoption (1-2 weeks)

- Week 1: Migration, testing
- Week 2: azd-exec release with azd-core v0.2.0

### Phase 4: Standardization (2-3 weeks)

- Week 1: Extract errors, testutil, constants
- Week 2: Testing, adoption in azd-app and azd-exec
- Week 3: azd-core v0.2.0 release

**Total Estimated Duration**: 9-14 weeks

---

## Open Questions

1. **Should we create a separate `azd-extensions-common` library for specialized utilities?**
   - Candidates: yamlutil, docker, cache, output
   - Pro: Keeps azd-core focused on general-purpose utilities
   - Con: Additional package to maintain

2. **Should we extract output formatting utilities?**
   - azd-exec has minimal needs
   - Could use existing CLI libraries (fatih/color, etc.)
   - Decide based on future extension requirements

3. **How do we handle breaking changes to extracted packages?**
   - Semantic versioning (MAJOR bump)
   - Deprecation period (1-2 minor versions)
   - Communication plan (CHANGELOG, GitHub discussions)

4. **Should azd-core packages depend on each other?**
   - Example: fileutil uses security.ValidatePath
   - Pro: Enforces security best practices
   - Con: Increases coupling
   - **Recommendation**: Allow internal dependencies, document in architecture

---

## References

- [azd-core Repository](https://github.com/jongio/azd-core)
- [azd-app Repository](https://github.com/jongio/azd-app)
- [azd-exec Repository](https://github.com/jongio/azd-exec)
- [Azure Developer CLI Extensions](https://github.com/Azure/azure-dev)
- [Go Project Layout](https://github.com/golang-standards/project-layout)
