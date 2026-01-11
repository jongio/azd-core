# azd-app Integration Analysis - Task 12

**Date**: January 10, 2026  
**Status**: Complete  
**Decision**: Proceed with targeted integration  

## Executive Summary

Analysis complete. azd-app already uses azd-core/security extensively (21+ files). Integration opportunities identified for:
- **fileutil**: Replace 3 atomic write patterns with fileutil.AtomicWriteJSON/AtomicWriteFile
- **pathutil**: Limited opportunity (1 use case for FindToolInPath)
- **procutil**: Not needed (container-focused, no direct PID checks)
- **browser**: Not currently implemented (dashboard auto-opens via VS Code Simple Browser)
- **shellutil**: Not needed (uses azd-exec extension for shell detection)

**Recommendation**: Proceed with fileutil integration only. Document browser/pathutil for future consideration.

---

## Current azd-core Integration Status

### ✅ Already Integrated: security package

**Current Usage** (21 files):
- `yamlutil/`: ValidatePath for azure.yaml operations (2 files)
- `testing/`: ValidatePath for coverage reports, test configs (5 files)
- `service/`: ValidatePath for detector, environment, port, parser operations (7 files)
- `installer/`: ValidatePath for dependency installation (1 file)
- `runner/`: ValidatePath for service execution (1 file)
- `detector/`: ValidatePath + fileutil wrappers (3 files)
- `commands/`: ValidatePath for core helpers, generate (2 files)

**Integration Quality**: Excellent
- Consistent usage pattern: validate path, then `#nosec G304` comment before os.ReadFile
- Security-first approach embedded in codebase
- No custom path validation implementations found

---

## Integration Opportunities by Package

### 1. fileutil - HIGH PRIORITY ⭐

#### Current State: Custom Atomic Write Patterns (3 locations)

**Pattern 1: cache/reqs_cache.go (Lines 259-276)**
```go
// Current implementation
data, err := json.MarshalIndent(cache, "", "  ")
tempFile := cacheFile + ".tmp"
if err := os.WriteFile(tempFile, data, 0600); err != nil {
    return fmt.Errorf("failed to write cache file: %w", err)
}
if err := os.Rename(tempFile, cacheFile); err != nil {
    _ = os.Remove(tempFile)
    return fmt.Errorf("failed to save cache file: %w", err)
}
```

**Issues**:
- Manual temp file cleanup required
- No sync/flush before rename (azd-core has this for CI reliability)
- No retry logic for rename failures (azd-core has 5 retries with backoff)

**Pattern 2: config/notifications.go (Lines 176-192)**
```go
// Current implementation
data, err := json.MarshalIndent(prefs, "", "  ")
tempPath := prefsPath + ".tmp"
if err := os.WriteFile(tempPath, data, 0644); err != nil {
    return fmt.Errorf("failed to write temp preferences file: %w", err)
}
if err := os.Rename(tempPath, prefsPath); err != nil {
    os.Remove(tempPath) // Clean up temp file on error
    return fmt.Errorf("failed to save notification preferences: %w", err)
}
```

**Issues**: Same as Pattern 1

**Pattern 3: config/config.go (Lines 86-92)**
```go
// Current implementation - NON-ATOMIC
data, err := json.MarshalIndent(config, "", "  ")
if err := os.WriteFile(configPath, data, 0644); err != nil {
    return fmt.Errorf("failed to write config file: %w", err)
}
```

**Issues**:
- Not atomic! Direct write without temp file
- File can be left in corrupt state if process crashes mid-write
- Critical config file vulnerability

#### Recommended Integration

**Replace with azd-core/fileutil**:

1. **cache/reqs_cache.go** (SaveResults method):
   ```go
   // Replace lines 259-276 with:
   if err := fileutil.AtomicWriteJSON(cacheFile, cache); err != nil {
       return fmt.Errorf("failed to save cache file: %w", err)
   }
   ```
   - Removes 17 lines of manual temp file handling
   - Gains sync/flush, retry logic, proper cleanup
   - Safer for CI environments (azd-core tested this extensively)

2. **config/notifications.go** (Save method):
   ```go
   // Replace lines 176-192 with:
   if err := fileutil.AtomicWriteJSON(prefsPath, prefs); err != nil {
       return fmt.Errorf("failed to save notification preferences: %w", err)
   }
   ```
   - Removes 16 lines of manual temp file handling
   - Note: Must handle mutex locking separately (fileutil doesn't know about prefs.mu)

3. **config/config.go** (Save method):
   ```go
   // Replace lines 86-92 with:
   if err := fileutil.AtomicWriteJSON(configPath, config); err != nil {
       return fmt.Errorf("failed to write config file: %w", err)
   }
   ```
   - **CRITICAL FIX**: Makes config.json writes atomic
   - Prevents corruption from crashes/kills

**Additional fileutil Usage**:

4. **detector/detector.go** already uses fileutil!
   ```go
   import "github.com/jongio/azd-core/fileutil"
   
   func fileExistsInDir(dir string, filename string) bool {
       return fileutil.FileExists(dir, filename)
   }
   ```
   - Shows existing pattern: azd-app wraps azd-core utilities
   - Can extend wrappers: fileExistsAny, filesExistAll, hasFileWithExt, containsTextInFile

5. **service/detector.go** has custom implementations:
   ```go
   // Lines 324-365: Custom fileExists, containsText functions
   func fileExists(dir string, filename string) bool {
       _, err := os.Stat(filepath.Join(dir, filename))
       return err == nil
   }
   
   func containsText(filePath string, text string) bool {
       data, err := os.ReadFile(filePath)  // No path validation!
       if err != nil {
           return false
       }
       return strings.Contains(string(data), text)
   }
   ```
   
   **Issues**:
   - No path validation (security.ValidatePath)
   - Duplicate of fileutil.ContainsText
   
   **Fix**: Replace with fileutil.ContainsText (validates paths)

**Summary**:
- Replace 3 atomic write patterns → saves ~50 lines, improves reliability
- Fix 1 non-atomic write → prevents config corruption
- Replace 2 custom helpers → improves security (path validation)

**Impact**: Medium effort, high value - critical reliability improvements

---

### 2. pathutil - LOW PRIORITY

#### Current State: exec.LookPath Usage (8+ locations)

**Usage Pattern**:
```go
// installer/installer.go lines 202, 295
if _, err := exec.LookPath("uv"); err != nil {
    // fallback to pip
}

// executor/hooks.go lines 180-190
if _, err := exec.LookPath(ShellPwsh); err == nil {
    return ShellPwsh
}

// notify/notify_*.go (Linux, macOS)
_, err := exec.LookPath("notify-send")
_, err := exec.LookPath("osascript")
```

**Analysis**:
- All uses are for tool existence checks, not installation guidance
- azd-app doesn't currently provide "install missing tool" suggestions via pathutil
- Could use pathutil.FindToolInPath + pathutil.GetInstallSuggestion for better UX

#### Potential Integration

**Option A**: Enhanced error messages for missing tools
```go
// installer/installer.go - when uv not found
if _, err := exec.LookPath("uv"); err != nil {
    if !output.IsJSON() && progressWriter == nil {
        output.ItemWarning("uv not found")
        output.Hint(pathutil.GetInstallSuggestion("uv"))
    }
    return setupWithPip(projectDir, progressWriter)
}
```

**Option B**: Tool discovery in system paths
```go
// Check PATH first, then common install locations
toolPath := pathutil.FindToolInPath("uv")
if toolPath == "" {
    toolPath = pathutil.SearchToolInSystemPath("uv")
}
```

**Recommendation**: Low priority - current exec.LookPath works fine. Consider for UX enhancement in future release.

---

### 3. procutil - NOT NEEDED

#### Current State: No Direct Process Checking

**Analysis**:
- azd-app tracks services via **container orchestration**, not PIDs
- Registry stores PID but only for display (serviceinfo.go line 86):
  ```go
  type ServiceInfo struct {
      PID int `json:"pid,omitempty"`  // For display only
  }
  ```
- Process termination uses `taskkill /T` on Windows, `process.Kill()` on Unix
- No process.IsRunning() checks needed

**Container-Focused Architecture**:
```go
// service/container_runner.go line 94
if client.IsRunning(container.ID) {  // Docker container, not PID
    // ...
}
```

**Recommendation**: No integration needed. azd-app architecture doesn't require PID-based process monitoring.

---

### 4. browser - DEFERRED (Not Currently Implemented)

#### Current State: No Browser Launching Code

**Analysis**:
- Dashboard opens automatically via **VS Code Simple Browser** (extension host integration)
- No `cmd /c start`, `xdg-open`, or `osascript` commands found
- Config exists for future use:
  ```go
  // config/config.go line 29
  Browser string `json:"browser,omitempty"` // Browser target: default, system, none
  ```
- Constants defined but unused:
  ```go
  // constants/constants.go line 34-35
  DefaultBrowserTimeout = 5 * time.Second
  ```

**Future Integration Path**:
If azd-app adds browser launching (e.g., for standalone mode without VS Code):
```go
import "github.com/jongio/azd-core/browser"

browserTarget := config.GetDashboardBrowser()  // Already exists
if browserTarget == "" {
    browserTarget = "default"
}

err := browser.Launch(browser.LaunchOptions{
    URL:     dashboardURL,
    Target:  browser.Target(browserTarget),
    Timeout: constants.DefaultBrowserTimeout,
})
```

**Recommendation**: Document for future use. Not needed for current VS Code-based architecture.

---

### 5. shellutil - NOT NEEDED

#### Current State: Uses azd-exec Extension

**Analysis**:
- Shell detection delegated to **azd-exec extension** (executor package)
- No internal shell detection code in azd-app
- Clean separation of concerns:
  - azd-exec: Shell detection, script execution
  - azd-app: Service orchestration, dashboard

**Architecture**:
```
azd-app → azd-exec extension → azd-core/shellutil
```

**Recommendation**: No direct integration needed. azd-exec handles shell detection.

---

## Implementation Plan

### Phase 1: fileutil Integration (RECOMMENDED)

**Priority**: High - Critical reliability improvements

**Changes**:

1. **cache/reqs_cache.go**
   - Import: Add `"github.com/jongio/azd-core/fileutil"`
   - SaveResults method (lines 259-276): Replace with `fileutil.AtomicWriteJSON`
   - Test: Verify cache persistence, corruption resistance

2. **config/notifications.go**
   - Import: Add `"github.com/jongio/azd-core/fileutil"`
   - Save method (lines 176-192): Replace with `fileutil.AtomicWriteJSON`
   - **Important**: Lock handling - marshal data inside RLock, then write outside lock
   - Test: Verify concurrent writes, notification preferences persistence

3. **config/config.go**
   - Import: Add `"github.com/jongio/azd-core/fileutil"`
   - Save method (lines 86-92): Replace with `fileutil.AtomicWriteJSON`
   - **CRITICAL**: Fixes non-atomic config writes
   - Test: Verify config.json integrity under process crashes

4. **service/detector.go**
   - Replace `fileExists` function with `fileutil.FileExists`
   - Replace `containsText` function with `fileutil.ContainsText`
   - Benefit: Adds path validation security

**Testing Requirements**:
- ✅ Existing tests should pass (drop-in replacements)
- ✅ Add corruption resistance tests (kill process during write)
- ✅ Verify .tmp file cleanup on failure
- ✅ Check concurrent write safety

**Risk**: Low - fileutil API matches current patterns exactly

---

### Phase 2: pathutil Enhancement (OPTIONAL)

**Priority**: Low - UX improvement, not critical

**Changes**:

1. **installer/installer.go**
   - Add install suggestions when tools not found (uv, poetry, etc.)
   - Use pathutil.GetInstallSuggestion() for user guidance

2. **commands/reqs.go**
   - Use pathutil.FindToolInPath for tool discovery
   - Provide better error messages with installation links

**Testing Requirements**:
- ✅ Verify install suggestions are helpful and accurate
- ✅ Test on Windows/macOS/Linux for path differences

**Risk**: Very low - additive changes only

---

## Testing Strategy

### Pre-Integration Validation

1. **Baseline Tests**: Run full azd-app test suite
   ```powershell
   cd c:\code\azd-app\cli
   go test ./...
   ```

2. **Integration Tests**: Run with azd-core via go.work
   ```powershell
   # Already configured in c:\code\azd-app\go.work
   go test ./src/internal/cache/...
   go test ./src/internal/config/...
   ```

### Post-Integration Validation

1. **Unit Tests**: Verify atomic write behavior
   - Test concurrent writes to same file
   - Test process crash during write (use signals)
   - Test disk full conditions

2. **Integration Tests**: End-to-end scenarios
   - Run azd-app dashboard with multiple services
   - Verify cache persistence across restarts
   - Verify config.json survives process crashes

3. **Regression Tests**: Ensure no breaking changes
   - All existing tests pass
   - Config file format unchanged
   - Cache file format unchanged

---

## Files to Modify

### Minimal Integration (fileutil only)

```
c:\code\azd-app\cli\src\internal\cache\reqs_cache.go        (SaveResults method)
c:\code\azd-app\cli\src\internal\config\notifications.go    (Save method)
c:\code\azd-app\cli\src\internal\config\config.go           (Save method)
c:\code\azd-app\cli\src\internal\service\detector.go        (fileExists, containsText)
```

**Total**: 4 files, ~5 method replacements, ~60 lines removed, ~10 lines added

**Net Impact**: -50 lines, improved reliability, better security

---

## Benefits Analysis

### fileutil Integration

**Code Quality**:
- ✅ Remove ~50 lines of duplicate temp file handling
- ✅ Consistent atomic write pattern across codebase
- ✅ Leverage battle-tested code (77.7% coverage, CI-hardened)

**Reliability**:
- ✅ Fix critical bug: config.go non-atomic writes
- ✅ Add sync/flush before rename (prevents CI flakiness)
- ✅ Add retry logic for rename failures (5 attempts with backoff)

**Security**:
- ✅ Add path validation to detector.go helpers
- ✅ Use security.ValidatePath before all file reads

**Maintainability**:
- ✅ Reduce maintenance burden (atomic write logic in one place)
- ✅ Bug fixes in azd-core benefit all consumers

### pathutil Integration (Optional)

**User Experience**:
- ✅ Better error messages when tools missing
- ✅ Installation guidance with direct links
- ✅ Discovery of tools in non-standard locations

**Code Quality**:
- ✅ Remove duplicate exec.LookPath patterns
- ✅ Centralize tool discovery logic

---

## Risks and Mitigation

### Risk 1: Breaking Config File Format

**Mitigation**: 
- fileutil.AtomicWriteJSON uses same json.MarshalIndent(data, "", "  ")
- Config format 100% identical to current implementation
- Verify with file diff before/after integration

### Risk 2: Performance Regression

**Mitigation**:
- Atomic writes add ~1ms for sync/flush (negligible)
- Retry logic only triggers on failure (rare)
- Benchmark before/after if needed

### Risk 3: Test Failures

**Mitigation**:
- Run full test suite before any changes (baseline)
- Use go.work for local azd-core testing
- Fix tests incrementally, file by file

---

## Decision Matrix

| Package    | Priority | Effort | Value | Decision             |
|------------|----------|--------|-------|----------------------|
| fileutil   | HIGH     | Low    | High  | ✅ Proceed           |
| security   | N/A      | N/A    | N/A   | ✅ Already integrated|
| pathutil   | LOW      | Low    | Low   | 📋 Document for future|
| procutil   | N/A      | N/A    | N/A   | ❌ Not needed        |
| browser    | N/A      | N/A    | N/A   | 📋 Document for future|
| shellutil  | N/A      | N/A    | N/A   | ❌ Not needed        |

---

## Conclusion

**Integration Decision**: ✅ Proceed with fileutil integration

**Rationale**:
1. **Critical reliability fix**: config.go non-atomic writes are a bug
2. **High value, low risk**: Drop-in API replacements, well-tested
3. **Already integrated**: azd-app uses azd-core/security extensively, pattern established
4. **Consistent with project goals**: Reduce code duplication, improve quality

**Next Steps**:
1. Update go.mod to require azd-core v0.2.0 (when published)
2. Implement fileutil integration (4 files)
3. Run full test suite
4. Update CHANGELOG.md
5. Submit PR for review

**Deferred**:
- pathutil: Consider for v0.3.0 when adding UX improvements
- browser: Consider when adding standalone mode (no VS Code)
- procutil, shellutil: Not needed for current architecture

---

## Appendix: Code Examples

### Example 1: cache/reqs_cache.go Integration

**Before** (17 lines):
```go
data, err := json.MarshalIndent(cache, "", "  ")
if err != nil {
    return fmt.Errorf("failed to marshal cache data: %w", err)
}

cacheFile := filepath.Join(cm.cacheDir, "reqs_cache.json")
tempFile := cacheFile + ".tmp"
if err := os.WriteFile(tempFile, data, 0600); err != nil {
    return fmt.Errorf("failed to write cache file: %w", err)
}

if err := os.Rename(tempFile, cacheFile); err != nil {
    _ = os.Remove(tempFile)
    return fmt.Errorf("failed to save cache file: %w", err)
}

return nil
```

**After** (5 lines):
```go
cacheFile := filepath.Join(cm.cacheDir, "reqs_cache.json")
if err := fileutil.AtomicWriteJSON(cacheFile, cache); err != nil {
    return fmt.Errorf("failed to save cache file: %w", err)
}
return nil
```

**Improvements**:
- -12 lines of code
- Adds sync/flush before rename
- Adds 5 retry attempts with backoff
- Proper temp file cleanup on all error paths
- Uses CreateTemp for unique temp filename

---

### Example 2: config/config.go Critical Fix

**Before** (7 lines, NON-ATOMIC):
```go
data, err := json.MarshalIndent(config, "", "  ")
if err != nil {
    return fmt.Errorf("failed to serialize config: %w", err)
}

if err := os.WriteFile(configPath, data, 0644); err != nil {
    return fmt.Errorf("failed to write config file: %w", err)
}
```

**After** (3 lines, ATOMIC):
```go
if err := fileutil.AtomicWriteJSON(configPath, config); err != nil {
    return fmt.Errorf("failed to write config file: %w", err)
}
```

**Critical Fix**:
- ❌ Before: Direct write can corrupt file on crash
- ✅ After: Atomic write guarantees valid config.json or no change

---

### Example 3: service/detector.go Security Fix

**Before** (5 lines, NO PATH VALIDATION):
```go
func containsText(filePath string, text string) bool {
    data, err := os.ReadFile(filePath)  // Vulnerable to path traversal!
    if err != nil {
        return false
    }
    return strings.Contains(string(data), text)
}
```

**After** (1 line, PATH VALIDATED):
```go
func containsText(filePath string, text string) bool {
    return fileutil.ContainsText(filePath, text)  // Validates path with security.ValidatePath
}
```

**Security Fix**:
- ❌ Before: No path validation, potential traversal attack
- ✅ After: fileutil.ContainsText validates path before reading

---

## Sign-off

**Analysis Completed**: January 10, 2026  
**Analyst**: GitHub Copilot  
**Recommendation**: Proceed with fileutil integration  
**Estimated Effort**: 4-6 hours (implementation + testing)  
**Risk Level**: Low  
**Value**: High (critical reliability improvements)
